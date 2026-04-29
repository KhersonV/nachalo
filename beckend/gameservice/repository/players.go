// ====================================
// /gameservice/repository/players.go
// ====================================

package repository

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"strconv"
	"strings"

	"gameservice/models"
)

const defaultProfileImage = "/guardian/guardian.webp"

type CreatePlayerProfileInput struct {
	UserID        int
	Name          string
	Image         string
	CharacterType string
	Balance       int
	Inventory     string
}

// Level thresholds for character progression.
var levelThresholds = map[int]int{
	1:  500,
	2:  2000,
	3:  8000,
	4:  32000,
	5:  128000,
	6:  512000,
	7:  2048000,
	8:  8192000,
	9:  32768000,
	10: 131072000,
}

func CreatePlayerProfile(input CreatePlayerProfileInput) (*models.PlayerResponse, error) {
	if input.UserID == 0 {
		return nil, fmt.Errorf("CreatePlayerProfile: user id is required")
	}
	if strings.TrimSpace(input.Name) == "" {
		input.Name = "Player"
	}
	if strings.TrimSpace(input.Image) == "" {
		input.Image = defaultProfileImage
	}
	if strings.TrimSpace(input.Inventory) == "" {
		input.Inventory = "{}"
	}

	heroClassID := ResolveSelectedHeroClass(input.CharacterType, "")

	tx, err := DB.Begin()
	if err != nil {
		return nil, fmt.Errorf("CreatePlayerProfile begin tx: %w", err)
	}
	defer tx.Rollback()

	if _, err := tx.Exec(`
		INSERT INTO player_profiles (
			user_id,
			name,
			image,
			balance,
			inventory
		)
		VALUES ($1,$2,$3,$4,$5)
	`, input.UserID, input.Name, input.Image, input.Balance, input.Inventory); err != nil {
		return nil, fmt.Errorf("CreatePlayerProfile insert profile: %w", err)
	}

	character, err := CreatePlayerCharacterTx(tx, input.UserID, heroClassID, HeroUnlockSourceRegistration)
	if err != nil {
		return nil, err
	}

	if _, err := tx.Exec(`
		UPDATE player_profiles
		SET selected_character_id = $1,
			updated_at = CURRENT_TIMESTAMP
		WHERE user_id = $2
	`, character.ID, input.UserID); err != nil {
		return nil, fmt.Errorf("CreatePlayerProfile set selected character: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("CreatePlayerProfile commit: %w", err)
	}

	return GetPlayerByUserID(input.UserID)
}

// GetPlayerByUserID returns account-level profile data enriched with the active character.
func GetPlayerByUserID(userID int) (*models.PlayerResponse, error) {
	player := &models.PlayerResponse{}
	query := `
		SELECT
			p.user_id,
			COALESCE(p.name, ''),
			COALESCE(p.image, ''),
			p.selected_character_id,
			p.balance,
			COALESCE(p.inventory, '{}'::jsonb),
			pc.id,
			pc.hero_class_id,
			COALESCE(pc.image, ''),
			pc.level,
			pc.exp,
			pc.max_exp,
			pc.max_energy,
			pc.max_health,
			pc.attack,
			pc.defense,
			pc.mobility,
			pc.agility,
			pc.sight_range,
			pc.is_ranged,
			pc.attack_range
		FROM player_profiles p
		LEFT JOIN player_characters pc ON pc.id = p.selected_character_id
		WHERE p.user_id = $1
	`
	row := DB.QueryRow(query, userID)
	var selectedChar sql.NullInt64
	var inventoryRaw []byte
	var characterID sql.NullInt64
	var heroClass sql.NullString
	var characterImage sql.NullString
	var level sql.NullInt64
	var exp sql.NullInt64
	var maxExp sql.NullInt64
	var maxEnergy sql.NullInt64
	var maxHealth sql.NullInt64
	var attack sql.NullInt64
	var defense sql.NullInt64
	var mobility sql.NullInt64
	var agility sql.NullInt64
	var sightRange sql.NullInt64
	var isRanged sql.NullBool
	var attackRange sql.NullInt64

	err := row.Scan(
		&player.UserID,
		&player.Name,
		&player.Image,
		&selectedChar,
		&player.Balance,
		&inventoryRaw,
		&characterID,
		&heroClass,
		&characterImage,
		&level,
		&exp,
		&maxExp,
		&maxEnergy,
		&maxHealth,
		&attack,
		&defense,
		&mobility,
		&agility,
		&sightRange,
		&isRanged,
		&attackRange,
	)
	if err != nil {
		log.Printf("GetPlayerByUserID: failed to load user_id=%d: %v", userID, err)
		return nil, err
	}

	if len(inventoryRaw) > 0 && json.Valid(inventoryRaw) {
		player.Inventory = string(inventoryRaw)
	} else {
		player.Inventory = "{}"
	}

	if selectedChar.Valid {
		v := int(selectedChar.Int64)
		player.SelectedCharacterID = &v
	}

	resolvedHeroClass := DefaultHeroClassID
	if heroClass.Valid {
		resolvedHeroClass = ResolveSelectedHeroClass(heroClass.String, "")
	}
	player.CharacterType = resolvedHeroClass
	player.SelectedHeroClassID = resolvedHeroClass

	if characterID.Valid {
		v := int(characterID.Int64)
		player.SelectedCharacterID = &v
	}

	if characterImage.Valid && characterImage.String != "" {
		player.Image = characterImage.String
	} else if player.Image == "" {
		player.Image = ResolveHeroClassMeta(resolvedHeroClass).Image
	}

	player.Level = 1
	if level.Valid {
		player.Level = int(level.Int64)
	}
	if exp.Valid {
		player.Experience = int(exp.Int64)
	}
	player.MaxExperience = maxExperienceForLevel(player.Level)
	if maxExp.Valid && maxExp.Int64 > 0 {
		player.MaxExperience = int(maxExp.Int64)
	}

	if maxEnergy.Valid {
		player.Energy = int(maxEnergy.Int64)
		player.MaxEnergy = int(maxEnergy.Int64)
	}
	if maxHealth.Valid {
		player.Health = int(maxHealth.Int64)
		player.MaxHealth = int(maxHealth.Int64)
	}
	if attack.Valid {
		player.Attack = int(attack.Int64)
	}
	if defense.Valid {
		player.Defense = int(defense.Int64)
	}
	if mobility.Valid {
		player.Mobility = int(mobility.Int64)
	}
	if agility.Valid {
		player.Agility = int(agility.Int64)
	}
	if sightRange.Valid {
		player.SightRange = int(sightRange.Int64)
	}
	if isRanged.Valid {
		player.IsRanged = isRanged.Bool
	}
	if attackRange.Valid {
		player.AttackRange = int(attackRange.Int64)
	}

	if !characterID.Valid {
		stats := ResolveHeroClassStats(DefaultHeroClassID)
		meta := ResolveHeroClassMeta(DefaultHeroClassID)
		player.CharacterType = DefaultHeroClassID
		player.SelectedHeroClassID = DefaultHeroClassID
		player.Image = meta.Image
		player.Level = 1
		player.Experience = 0
		player.MaxExperience = maxExperienceForLevel(1)
		player.Energy = stats.MaxEnergy
		player.MaxEnergy = stats.MaxEnergy
		player.Health = stats.MaxHealth
		player.MaxHealth = stats.MaxHealth
		player.Attack = stats.Attack
		player.Defense = stats.Defense
		player.Mobility = stats.Mobility
		player.Agility = stats.Agility
		player.SightRange = stats.SightRange
		player.IsRanged = stats.IsRanged
		player.AttackRange = stats.AttackRange
	}

	return player, nil
}

// UpdatePlayer persists only account-level shared profile state.
func UpdatePlayer(player *models.PlayerResponse) error {
	if _, err := DB.Exec(`
		UPDATE player_profiles
		SET balance = $1,
			inventory = $2,
			updated_at = CURRENT_TIMESTAMP
		WHERE user_id = $3
	`, player.Balance, player.Inventory, player.UserID); err != nil {
		return fmt.Errorf("UpdatePlayer: %w", err)
	}
	return nil
}

// DeleteMatchPlayer removes a player snapshot from a match.
func DeleteMatchPlayer(instanceID string, userID int) error {
	_, err := DB.Exec(`
        DELETE FROM match_players
        WHERE instance_id = $1
          AND user_id = $2
    `, instanceID, userID)
	return err
}

func MarkPlayerDead(instanceID string, userID int) error {
	_, err := DB.Exec(`
        UPDATE match_players
           SET health = 0
         WHERE instance_id = $1
           AND user_id = $2
    `, instanceID, userID)
	return err
}

// AddPlayerExperience applies experience to the active character.
func AddPlayerExperience(userID, exp int) error {
	character, err := GetSelectedCharacterForUser(userID)
	if err != nil {
		return fmt.Errorf("AddPlayerExperience: selected character: %w", err)
	}
	return AddCharacterExperience(character.ID, exp)
}

// AddCharacterExperience adds experience to a concrete character and updates level/max_exp.
func AddCharacterExperience(characterID int, exp int) error {
	tx, err := DB.Begin()
	if err != nil {
		return fmt.Errorf("AddCharacterExperience begin tx: %w", err)
	}
	defer tx.Rollback()

	var level int
	var experience int
	if err := tx.QueryRow(`
		SELECT level, exp
		FROM player_characters
		WHERE id = $1
		FOR UPDATE
	`, characterID).Scan(&level, &experience); err != nil {
		return fmt.Errorf("AddCharacterExperience select: %w", err)
	}

	experience += exp
	for {
		threshold, ok := levelThresholds[level]
		if !ok || experience < threshold {
			break
		}
		level++
		experience -= threshold
	}
	maxExp := maxExperienceForLevel(level)

	if _, err := tx.Exec(`
		UPDATE player_characters
		SET level = $1,
			exp = $2,
			max_exp = $3,
			updated_at = CURRENT_TIMESTAMP
		WHERE id = $4
	`, level, experience, maxExp, characterID); err != nil {
		return fmt.Errorf("AddCharacterExperience update: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("AddCharacterExperience commit: %w", err)
	}
	return nil
}

func AddPlayerRewards(userID int, rewardsData []byte) error {
	log.Printf("Raw rewardsData for user %d: %s", userID, string(rewardsData))

	type rewardEntry struct {
		Type   string `json:"type"`
		Amount int    `json:"amount"`
	}

	rewards := make(map[string]int)
	var rewardsMap map[string]int
	if err := json.Unmarshal(rewardsData, &rewardsMap); err == nil {
		for k, v := range rewardsMap {
			rewards[k] += v
		}
	} else {
		var rewardsList []rewardEntry
		if err := json.Unmarshal(rewardsData, &rewardsList); err != nil {
			return fmt.Errorf("AddPlayerRewards: unmarshal rewards: %w", err)
		}
		for _, r := range rewardsList {
			if r.Type == "" || r.Amount == 0 {
				continue
			}
			rewards[r.Type] += r.Amount
		}
	}

	player, err := GetPlayerByUserID(userID)
	if err != nil {
		return fmt.Errorf("AddPlayerRewards: fetch player: %w", err)
	}

	for key, amount := range rewards {
		if strings.HasPrefix(key, "artifact_") || key == "artifact" {
			continue
		}

		switch key {
		case "balance", "coin", "coins":
			player.Balance += amount

		default:
			var inv map[string]map[string]interface{}
			if err := json.Unmarshal([]byte(player.Inventory), &inv); err != nil {
				inv = make(map[string]map[string]interface{})
			}
			inv[key] = map[string]interface{}{
				"name":       key,
				"item_count": amount,
			}
			b, _ := json.Marshal(inv)
			player.Inventory = string(b)
		}
	}

	if err := UpdatePlayer(player); err != nil {
		return fmt.Errorf("AddPlayerRewards: update player: %w", err)
	}
	return nil
}

// SyncPersistentInventoryFromMatchResources copies the final match resource inventory to the profile inventory.
func SyncPersistentInventoryFromMatchResources(instanceID string, userID int) error {
	matchPlayer, err := GetMatchPlayerByID(instanceID, userID)
	if err != nil {
		return fmt.Errorf("SyncPersistentInventoryFromMatchResources: fetch match player: %w", err)
	}

	finalInv := make(map[string]map[string]interface{})
	if matchPlayer.Inventory != "" && matchPlayer.Inventory != "{}" {
		if err := json.Unmarshal([]byte(matchPlayer.Inventory), &finalInv); err != nil {
			finalInv = make(map[string]map[string]interface{})
		}
	}

	for key := range finalInv {
		if strings.HasPrefix(key, "artifact_") || key == "artifact" {
			delete(finalInv, key)
		}
	}

	player, err := GetPlayerByUserID(userID)
	if err != nil {
		return fmt.Errorf("SyncPersistentInventoryFromMatchResources: fetch player: %w", err)
	}

	b, err := json.Marshal(finalInv)
	if err != nil {
		return fmt.Errorf("SyncPersistentInventoryFromMatchResources: marshal inventory: %w", err)
	}
	player.Inventory = string(b)

	if err := UpdatePlayer(player); err != nil {
		return fmt.Errorf("SyncPersistentInventoryFromMatchResources: update player: %w", err)
	}

	return nil
}

// ConsumePlayerInventoryItem decrements an item from shared profile inventory.
func ConsumePlayerInventoryItem(userID int, itemType string, itemID int, count int) error {
	if count <= 0 {
		count = 1
	}

	player, err := GetPlayerByUserID(userID)
	if err != nil {
		return fmt.Errorf("ConsumePlayerInventoryItem: fetch player: %w", err)
	}

	inv := make(map[string]map[string]interface{})
	if player.Inventory != "" && player.Inventory != "{}" {
		if err := json.Unmarshal([]byte(player.Inventory), &inv); err != nil {
			inv = make(map[string]map[string]interface{})
		}
	}

	key := fmt.Sprintf("%s_%d", itemType, itemID)
	entry, ok := inv[key]
	if !ok {
		return nil
	}

	current := 0
	switch v := entry["item_count"].(type) {
	case float64:
		current = int(v)
	case int:
		current = v
	case int64:
		current = int(v)
	case string:
		parsed, parseErr := strconv.Atoi(v)
		if parseErr == nil {
			current = parsed
		}
	default:
		current = 0
	}

	next := current - count
	if next > 0 {
		entry["item_count"] = next
		inv[key] = entry
	} else {
		delete(inv, key)
	}

	b, err := json.Marshal(inv)
	if err != nil {
		return fmt.Errorf("ConsumePlayerInventoryItem: marshal inventory: %w", err)
	}
	player.Inventory = string(b)

	if err := UpdatePlayer(player); err != nil {
		return fmt.Errorf("ConsumePlayerInventoryItem: update player: %w", err)
	}
	return nil
}

func IsPlayerNotFound(err error) bool {
	return errors.Is(err, sql.ErrNoRows) || errors.Is(err, ErrPlayerNotFound)
}
