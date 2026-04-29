package repository

import (
	"database/sql"
	"errors"
	"fmt"
	"log"
	"strings"
)

const (
	DefaultHeroClassID           = "adventurer"
	HeroUnlockSourceBackfill     = "backfill"
	HeroUnlockSourceRegistration = "registration"
	HeroUnlockSourceHire         = "hire"
)

var (
	ErrHeroAlreadyOwned = errors.New("hero already owned")
	ErrNotEnoughGold    = errors.New("not enough gold")
	ErrPlayerNotFound   = errors.New("player not found")
	ErrTavernRequired   = errors.New("tavern required")
)

func NormalizeHeroClassID(heroClassID string) string {
	return strings.ToLower(strings.TrimSpace(heroClassID))
}

func ResolveSelectedHeroClass(selectedHeroClassID, characterType string) string {
	if selected := NormalizeHeroClassID(selectedHeroClassID); selected != "" {
		return selected
	}
	if current := NormalizeHeroClassID(characterType); current != "" {
		return current
	}
	return DefaultHeroClassID
}

func EnsurePlayerHeroFoundation() {
	if _, err := DB.Exec(`ALTER TABLE players ADD COLUMN IF NOT EXISTS selected_hero_class_id TEXT`); err != nil {
		log.Fatalf("EnsurePlayerHeroFoundation add selected_hero_class_id: %v", err)
	}

	if _, err := DB.Exec(`
		UPDATE players
		SET selected_hero_class_id = COALESCE(
			NULLIF(BTRIM(selected_hero_class_id), ''),
			NULLIF(BTRIM(character_type), ''),
			'adventurer'
		)
		WHERE selected_hero_class_id IS NULL
		   OR BTRIM(selected_hero_class_id) = ''
	`); err != nil {
		log.Fatalf("EnsurePlayerHeroFoundation backfill selected_hero_class_id: %v", err)
	}

	if _, err := DB.Exec(`ALTER TABLE players ALTER COLUMN selected_hero_class_id SET DEFAULT 'adventurer'`); err != nil {
		log.Fatalf("EnsurePlayerHeroFoundation default selected_hero_class_id: %v", err)
	}

	query := `
		CREATE TABLE IF NOT EXISTS player_heroes (
			user_id INTEGER NOT NULL REFERENCES players(user_id) ON DELETE CASCADE,
			hero_class_id TEXT NOT NULL,
			unlocked_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
			source TEXT NOT NULL DEFAULT 'backfill',
			price_paid INTEGER DEFAULT 0,
			PRIMARY KEY (user_id, hero_class_id),
			CHECK (BTRIM(hero_class_id) <> ''),
			CHECK (price_paid IS NULL OR price_paid >= 0)
		);
		CREATE INDEX IF NOT EXISTS idx_player_heroes_user_id ON player_heroes(user_id);
	`
	if _, err := DB.Exec(query); err != nil {
		log.Fatalf("EnsurePlayerHeroFoundation create player_heroes: %v", err)
	}

	if _, err := DB.Exec(`
		INSERT INTO player_heroes (user_id, hero_class_id, source, price_paid)
		SELECT
			user_id,
			COALESCE(NULLIF(BTRIM(character_type), ''), 'adventurer') AS hero_class_id,
			$1 AS source,
			0 AS price_paid
		FROM players
		ON CONFLICT (user_id, hero_class_id) DO NOTHING
	`, HeroUnlockSourceBackfill); err != nil {
		log.Fatalf("EnsurePlayerHeroFoundation backfill player_heroes: %v", err)
	}
}

func EnsureMatchPlayersHeroSnapshotColumn() {
	if _, err := DB.Exec(`ALTER TABLE match_players ADD COLUMN IF NOT EXISTS character_type TEXT`); err != nil {
		log.Fatalf("EnsureMatchPlayersHeroSnapshotColumn add character_type: %v", err)
	}

	if _, err := DB.Exec(`
		UPDATE match_players mp
		SET character_type = COALESCE(
			NULLIF(BTRIM(mp.character_type), ''),
			NULLIF(BTRIM(p.selected_hero_class_id), ''),
			NULLIF(BTRIM(p.character_type), ''),
			'adventurer'
		)
		FROM players p
		WHERE p.user_id = mp.user_id
		  AND (
			mp.character_type IS NULL
			OR BTRIM(mp.character_type) = ''
		  )
	`); err != nil {
		log.Fatalf("EnsureMatchPlayersHeroSnapshotColumn backfill from players: %v", err)
	}

	if _, err := DB.Exec(`
		UPDATE match_players
		SET character_type = 'adventurer'
		WHERE character_type IS NULL
		   OR BTRIM(character_type) = ''
	`); err != nil {
		log.Fatalf("EnsureMatchPlayersHeroSnapshotColumn backfill default: %v", err)
	}

	if _, err := DB.Exec(`ALTER TABLE match_players ALTER COLUMN character_type SET DEFAULT 'adventurer'`); err != nil {
		log.Fatalf("EnsureMatchPlayersHeroSnapshotColumn default character_type: %v", err)
	}
}

func EnsurePlayerHeroUnlocked(userID int, heroClassID, source string, pricePaid int) error {
	heroClassID = ResolveSelectedHeroClass(heroClassID, "")
	source = strings.TrimSpace(source)
	if source == "" {
		source = HeroUnlockSourceBackfill
	}

	_, err := DB.Exec(`
		INSERT INTO player_heroes (user_id, hero_class_id, source, price_paid)
		VALUES ($1, $2, $3, $4)
		ON CONFLICT (user_id, hero_class_id) DO NOTHING
	`, userID, heroClassID, source, pricePaid)
	if err != nil {
		return fmt.Errorf("EnsurePlayerHeroUnlocked: %w", err)
	}
	return nil
}

func GetPlayerHeroClassIDs(userID int) (map[string]bool, error) {
	rows, err := DB.Query(`
		SELECT hero_class_id
		FROM player_heroes
		WHERE user_id = $1
	`, userID)
	if err != nil {
		return nil, fmt.Errorf("GetPlayerHeroClassIDs query: %w", err)
	}
	defer rows.Close()

	owned := make(map[string]bool)
	for rows.Next() {
		var heroClassID string
		if err := rows.Scan(&heroClassID); err != nil {
			return nil, fmt.Errorf("GetPlayerHeroClassIDs scan: %w", err)
		}
		if normalized := NormalizeHeroClassID(heroClassID); normalized != "" {
			owned[normalized] = true
		}
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("GetPlayerHeroClassIDs rows: %w", err)
	}

	return owned, nil
}

func HireHero(userID int, heroClassID string, unlockPrice int, requiresTavern bool) (int, error) {
	heroClassID = NormalizeHeroClassID(heroClassID)
	if heroClassID == "" {
		return 0, fmt.Errorf("HireHero: hero class id is required")
	}

	tx, err := DB.Begin()
	if err != nil {
		return 0, fmt.Errorf("HireHero begin tx: %w", err)
	}
	defer tx.Rollback()

	var balance int
	var characterType string
	var selectedHeroClassID string
	if err := tx.QueryRow(`
		SELECT
			balance,
			COALESCE(character_type, ''),
			COALESCE(selected_hero_class_id, '')
		FROM players
		WHERE user_id = $1
		FOR UPDATE
	`, userID).Scan(&balance, &characterType, &selectedHeroClassID); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return 0, ErrPlayerNotFound
		}
		return 0, fmt.Errorf("HireHero lock player: %w", err)
	}

	var alreadyOwned bool
	if err := tx.QueryRow(`
		SELECT EXISTS (
			SELECT 1
			FROM player_heroes
			WHERE user_id = $1
			  AND hero_class_id = $2
		)
	`, userID, heroClassID).Scan(&alreadyOwned); err != nil {
		return balance, fmt.Errorf("HireHero check ownership: %w", err)
	}
	currentClassOwned := heroClassID == NormalizeHeroClassID(characterType)
	selectedClassOwned := heroClassID == NormalizeHeroClassID(selectedHeroClassID)
	if alreadyOwned || currentClassOwned || selectedClassOwned {
		return balance, ErrHeroAlreadyOwned
	}

	if requiresTavern {
		if err := ensureBaseBuildingsRowTx(tx, userID); err != nil {
			return balance, fmt.Errorf("HireHero ensure base row: %w", err)
		}

		var tavernLevel int
		if err := tx.QueryRow(`
			SELECT tavern_level
			FROM player_base_buildings
			WHERE user_id = $1
			FOR UPDATE
		`, userID).Scan(&tavernLevel); err != nil {
			return balance, fmt.Errorf("HireHero lock tavern state: %w", err)
		}
		if tavernLevel <= 0 {
			return balance, ErrTavernRequired
		}
	}

	if balance < unlockPrice {
		return balance, ErrNotEnoughGold
	}

	newBalance := balance - unlockPrice
	if _, err := tx.Exec(`UPDATE players SET balance = $1 WHERE user_id = $2`, newBalance, userID); err != nil {
		return balance, fmt.Errorf("HireHero deduct balance: %w", err)
	}

	if _, err := tx.Exec(`
		INSERT INTO player_heroes (user_id, hero_class_id, unlocked_at, source, price_paid)
		VALUES ($1, $2, CURRENT_TIMESTAMP, $3, $4)
	`, userID, heroClassID, HeroUnlockSourceHire, unlockPrice); err != nil {
		return balance, fmt.Errorf("HireHero insert ownership: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return balance, fmt.Errorf("HireHero commit: %w", err)
	}

	return newBalance, nil
}

func SetSelectedHeroClassID(userID int, heroClassID string) error {
	heroClassID = NormalizeHeroClassID(heroClassID)
	if heroClassID == "" {
		return fmt.Errorf("SetSelectedHeroClassID: hero class id is required")
	}

	result, err := DB.Exec(`
		UPDATE players
		SET selected_hero_class_id = $1
		WHERE user_id = $2
	`, heroClassID, userID)
	if err != nil {
		return fmt.Errorf("SetSelectedHeroClassID update: %w", err)
	}

	affected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("SetSelectedHeroClassID rows affected: %w", err)
	}
	if affected == 0 {
		return ErrPlayerNotFound
	}

	return nil
}
