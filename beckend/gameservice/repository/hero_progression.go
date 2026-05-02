package repository

import (
	"database/sql"
	"errors"
	"fmt"
	"strings"
)

const (
	DefaultHeroClassID           = "guardian"
	HeroUnlockSourceRegistration = "registration"
	HeroUnlockSourceHire         = "hire"
	HeroUnlockSourceRecovery     = "recovery"
)

var (
	ErrHeroAlreadyOwned = errors.New("hero already owned")
	ErrNotEnoughGold    = errors.New("not enough gold")
	ErrPlayerNotFound   = errors.New("player not found")
	ErrTavernRequired   = errors.New("tavern required")
)

type PlayerCharacter struct {
	ID          int
	UserID      int
	HeroClassID string
	Image       string
	Level       int
	Exp         int
	MaxExp      int
	MaxEnergy   int
	MaxHealth   int
	Attack      int
	Defense     int
	Mobility    int
	Agility     int
	SightRange  int
	IsRanged    bool
	AttackRange int
	Source      string
}

var knownHeroClassIDs = map[string]struct{}{
	"guardian":  {},
	"berserker": {},
	"ranger":    {},
	"mystic":    {},
}

func NormalizeHeroClassID(heroClassID string) string {
	return strings.ToLower(strings.TrimSpace(heroClassID))
}

func IsKnownHeroClassID(heroClassID string) bool {
	_, ok := knownHeroClassIDs[NormalizeHeroClassID(heroClassID)]
	return ok
}

func ResolveSelectedHeroClass(selectedHeroClassID, characterType string) string {
	if selected := NormalizeHeroClassID(selectedHeroClassID); IsKnownHeroClassID(selected) {
		return selected
	}
	if current := NormalizeHeroClassID(characterType); IsKnownHeroClassID(current) {
		return current
	}
	return DefaultHeroClassID
}

func maxExperienceForLevel(level int) int {
	if threshold, ok := levelThresholds[level]; ok {
		return threshold
	}
	return levelThresholds[1]
}

func scanPlayerCharacter(scan func(dest ...interface{}) error) (*PlayerCharacter, error) {
	var character PlayerCharacter
	err := scan(
		&character.ID,
		&character.UserID,
		&character.HeroClassID,
		&character.Image,
		&character.Level,
		&character.Exp,
		&character.MaxExp,
		&character.MaxEnergy,
		&character.MaxHealth,
		&character.Attack,
		&character.Defense,
		&character.Mobility,
		&character.Agility,
		&character.SightRange,
		&character.IsRanged,
		&character.AttackRange,
		&character.Source,
	)
	if err != nil {
		return nil, err
	}
	return &character, nil
}

func selectPlayerCharacterByIDTx(tx *sql.Tx, userID int, characterID int) (*PlayerCharacter, error) {
	return scanPlayerCharacter(tx.QueryRow(`
		SELECT
			id,
			user_id,
			hero_class_id,
			COALESCE(image, ''),
			level,
			exp,
			max_exp,
			max_energy,
			max_health,
			attack,
			defense,
			mobility,
			agility,
			sight_range,
			is_ranged,
			attack_range,
			COALESCE(source, '')
		FROM player_characters
		WHERE user_id = $1
		  AND id = $2
	`, userID, characterID).Scan)
}

func selectPlayerCharacterByClassTx(tx *sql.Tx, userID int, heroClassID string) (*PlayerCharacter, error) {
	return scanPlayerCharacter(tx.QueryRow(`
		SELECT
			id,
			user_id,
			hero_class_id,
			COALESCE(image, ''),
			level,
			exp,
			max_exp,
			max_energy,
			max_health,
			attack,
			defense,
			mobility,
			agility,
			sight_range,
			is_ranged,
			attack_range,
			COALESCE(source, '')
		FROM player_characters
		WHERE user_id = $1
		  AND hero_class_id = $2
	`, userID, heroClassID).Scan)
}

func CreatePlayerCharacterTx(tx *sql.Tx, userID int, heroClassID string, source string) (*PlayerCharacter, error) {
	heroClassID = ResolveSelectedHeroClass(heroClassID, "")
	source = strings.TrimSpace(source)
	if source == "" {
		source = HeroUnlockSourceRegistration
	}

	stats := ResolveHeroClassStats(heroClassID)
	meta := ResolveHeroClassMeta(heroClassID)
	maxExp := maxExperienceForLevel(1)

	character, err := scanPlayerCharacter(tx.QueryRow(`
		INSERT INTO player_characters (
			user_id,
			hero_class_id,
			image,
			level,
			exp,
			max_exp,
			max_energy,
			max_health,
			attack,
			defense,
			mobility,
			agility,
			sight_range,
			is_ranged,
			attack_range,
			source
		)
		VALUES ($1,$2,$3,1,0,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14)
		RETURNING
			id,
			user_id,
			hero_class_id,
			COALESCE(image, ''),
			level,
			exp,
			max_exp,
			max_energy,
			max_health,
			attack,
			defense,
			mobility,
			agility,
			sight_range,
			is_ranged,
			attack_range,
			COALESCE(source, '')
	`, userID, heroClassID, meta.Image, maxExp, stats.MaxEnergy, stats.MaxHealth, stats.Attack, stats.Defense, stats.Mobility, stats.Agility, stats.SightRange, stats.IsRanged, stats.AttackRange, source).Scan)
	if err != nil {
		return nil, fmt.Errorf("CreatePlayerCharacterTx insert: %w", err)
	}
	return character, nil
}

func EnsurePlayerCharacterTx(tx *sql.Tx, userID int, heroClassID string, source string) (*PlayerCharacter, error) {
	heroClassID = ResolveSelectedHeroClass(heroClassID, "")
	character, err := selectPlayerCharacterByClassTx(tx, userID, heroClassID)
	if err == nil {
		return character, nil
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return nil, fmt.Errorf("EnsurePlayerCharacterTx select: %w", err)
	}
	return CreatePlayerCharacterTx(tx, userID, heroClassID, source)
}

func EnsurePlayerHeroUnlocked(userID int, heroClassID, source string, _ int) error {
	tx, err := DB.Begin()
	if err != nil {
		return fmt.Errorf("EnsurePlayerHeroUnlocked begin tx: %w", err)
	}
	defer tx.Rollback()

	if _, err := EnsurePlayerCharacterTx(tx, userID, heroClassID, source); err != nil {
		return err
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("EnsurePlayerHeroUnlocked commit: %w", err)
	}
	return nil
}

func GetPlayerCharactersMap(userID int) (map[string]int, error) {
	rows, err := DB.Query(`
		SELECT id, hero_class_id
		FROM player_characters
		WHERE user_id = $1
	`, userID)
	if err != nil {
		return nil, fmt.Errorf("GetPlayerCharactersMap query: %w", err)
	}
	defer rows.Close()

	m := make(map[string]int)
	for rows.Next() {
		var id int
		var heroClassID string
		if err := rows.Scan(&id, &heroClassID); err != nil {
			return nil, fmt.Errorf("GetPlayerCharactersMap scan: %w", err)
		}
		if normalized := NormalizeHeroClassID(heroClassID); normalized != "" {
			m[normalized] = id
		}
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("GetPlayerCharactersMap rows: %w", err)
	}
	return m, nil
}

func GetSelectedCharacterForUser(userID int) (*PlayerCharacter, error) {
	tx, err := DB.Begin()
	if err != nil {
		return nil, fmt.Errorf("GetSelectedCharacterForUser begin tx: %w", err)
	}
	defer tx.Rollback()

	character, err := ensureSelectedCharacterForUserTx(tx, userID)
	if err != nil {
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("GetSelectedCharacterForUser commit: %w", err)
	}
	return character, nil
}

func ensureSelectedCharacterForUserTx(tx *sql.Tx, userID int) (*PlayerCharacter, error) {
	var selectedCharacterID sql.NullInt64
	if err := tx.QueryRow(`
		SELECT selected_character_id
		FROM player_profiles
		WHERE user_id = $1
		FOR UPDATE
	`, userID).Scan(&selectedCharacterID); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrPlayerNotFound
		}
		return nil, fmt.Errorf("ensureSelectedCharacterForUserTx lock profile: %w", err)
	}

	if selectedCharacterID.Valid && selectedCharacterID.Int64 > 0 {
		character, err := selectPlayerCharacterByIDTx(tx, userID, int(selectedCharacterID.Int64))
		if err == nil {
			return character, nil
		}
		if !errors.Is(err, sql.ErrNoRows) {
			return nil, fmt.Errorf("ensureSelectedCharacterForUserTx select selected: %w", err)
		}
	}

	character, err := EnsurePlayerCharacterTx(tx, userID, DefaultHeroClassID, HeroUnlockSourceRecovery)
	if err != nil {
		return nil, err
	}

	if _, err := tx.Exec(`
		UPDATE player_profiles
		SET selected_character_id = $1,
			updated_at = CURRENT_TIMESTAMP
		WHERE user_id = $2
	`, character.ID, userID); err != nil {
		return nil, fmt.Errorf("ensureSelectedCharacterForUserTx update selected: %w", err)
	}

	return character, nil
}

func SetSelectedCharacterByHeroClass(userID int, heroClassID string) error {
	heroClassID = NormalizeHeroClassID(heroClassID)
	if !IsKnownHeroClassID(heroClassID) {
		return fmt.Errorf("SetSelectedCharacterByHeroClass: unknown hero class %q", heroClassID)
	}

	tx, err := DB.Begin()
	if err != nil {
		return fmt.Errorf("SetSelectedCharacterByHeroClass begin tx: %w", err)
	}
	defer tx.Rollback()

	character, err := selectPlayerCharacterByClassTx(tx, userID, heroClassID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return ErrHeroAlreadyOwned
		}
		return fmt.Errorf("SetSelectedCharacterByHeroClass select: %w", err)
	}

	result, err := tx.Exec(`
		UPDATE player_profiles
		SET selected_character_id = $1,
			updated_at = CURRENT_TIMESTAMP
		WHERE user_id = $2
	`, character.ID, userID)
	if err != nil {
		return fmt.Errorf("SetSelectedCharacterByHeroClass update: %w", err)
	}

	affected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("SetSelectedCharacterByHeroClass rows affected: %w", err)
	}
	if affected == 0 {
		return ErrPlayerNotFound
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("SetSelectedCharacterByHeroClass commit: %w", err)
	}
	return nil
}

func HireHero(userID int, heroClassID string, unlockPrice int, requiresTavern bool) (int, error) {
	heroClassID = NormalizeHeroClassID(heroClassID)
	if !IsKnownHeroClassID(heroClassID) {
		return 0, fmt.Errorf("HireHero: unknown hero class %q", heroClassID)
	}

	tx, err := DB.Begin()
	if err != nil {
		return 0, fmt.Errorf("HireHero begin tx: %w", err)
	}
	defer tx.Rollback()

	var balance int
	if err := tx.QueryRow(`
		SELECT balance
		FROM player_profiles
		WHERE user_id = $1
		FOR UPDATE
	`, userID).Scan(&balance); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return 0, ErrPlayerNotFound
		}
		return 0, fmt.Errorf("HireHero lock profile: %w", err)
	}

	var alreadyOwned bool
	if err := tx.QueryRow(`
		SELECT EXISTS (
			SELECT 1
			FROM player_characters
			WHERE user_id = $1
			  AND hero_class_id = $2
		)
	`, userID, heroClassID).Scan(&alreadyOwned); err != nil {
		return balance, fmt.Errorf("HireHero check ownership: %w", err)
	}
	if alreadyOwned {
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
	if _, err := tx.Exec(`
		UPDATE player_profiles
		SET balance = $1,
			updated_at = CURRENT_TIMESTAMP
		WHERE user_id = $2
	`, newBalance, userID); err != nil {
		return balance, fmt.Errorf("HireHero deduct balance: %w", err)
	}

	if _, err := CreatePlayerCharacterTx(tx, userID, heroClassID, HeroUnlockSourceHire); err != nil {
		return balance, fmt.Errorf("HireHero create character: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return balance, fmt.Errorf("HireHero commit: %w", err)
	}

	return newBalance, nil
}
