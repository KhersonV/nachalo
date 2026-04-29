package repository

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"log"
)

const (
	ForgeCostWood  = 40
	ForgeCostStone = 30
	ForgeCostIron  = 20

	LibraryCostWood  = 30
	LibraryCostStone = 45
	LibraryCostIron  = 25

	TavernCostWood  = 10
	TavernCostStone = 10
	TavernCostIron  = 10
)

type baseBuildingConfig struct {
	Name              string
	LevelColumn       string
	AlreadyBuiltError string
	CostWood          int
	CostStone         int
	CostIron          int
}

var (
	forgeBuildingConfig = baseBuildingConfig{
		Name:              "Forge",
		LevelColumn:       "forge_level",
		AlreadyBuiltError: "forge already built",
		CostWood:          ForgeCostWood,
		CostStone:         ForgeCostStone,
		CostIron:          ForgeCostIron,
	}
	libraryBuildingConfig = baseBuildingConfig{
		Name:              "Library",
		LevelColumn:       "library_level",
		AlreadyBuiltError: "library already built",
		CostWood:          LibraryCostWood,
		CostStone:         LibraryCostStone,
		CostIron:          LibraryCostIron,
	}
	tavernBuildingConfig = baseBuildingConfig{
		Name:              "Tavern",
		LevelColumn:       "tavern_level",
		AlreadyBuiltError: "tavern already built",
		CostWood:          TavernCostWood,
		CostStone:         TavernCostStone,
		CostIron:          TavernCostIron,
	}
)

func getResourceIDByTypeTx(tx *sql.Tx, resourceType string) (int, error) {
	var id int
	if err := tx.QueryRow(`SELECT id FROM resources WHERE type = $1 LIMIT 1`, resourceType).Scan(&id); err != nil {
		return 0, fmt.Errorf("getResourceIDByTypeTx(%s): %w", resourceType, err)
	}
	return id, nil
}

func getResourceCount(inv map[string]map[string]interface{}, key string) int {
	entry, ok := inv[key]
	if !ok {
		return 0
	}
	raw, ok := entry["item_count"]
	if !ok {
		return 0
	}
	switch v := raw.(type) {
	case float64:
		return int(v)
	case int:
		return v
	case int64:
		return int(v)
	default:
		return 0
	}
}

func setResourceCount(inv map[string]map[string]interface{}, key string, next int, displayName string) {
	if next <= 0 {
		delete(inv, key)
		return
	}

	entry := inv[key]
	if entry == nil {
		entry = map[string]interface{}{}
	}
	if _, exists := entry["name"]; !exists {
		entry["name"] = displayName
	}
	entry["item_count"] = next
	inv[key] = entry
}

func EnsurePlayerBaseBuildingsTable() {
	query := `
    CREATE TABLE IF NOT EXISTS player_base_buildings (
        user_id INTEGER PRIMARY KEY REFERENCES players(user_id) ON DELETE CASCADE,
        forge_level INTEGER NOT NULL DEFAULT 0,
        library_level INTEGER NOT NULL DEFAULT 0,
        tavern_level INTEGER NOT NULL DEFAULT 0,
        created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
        updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
    );
    `
	if _, err := DB.Exec(query); err != nil {
		log.Fatalf("EnsurePlayerBaseBuildingsTable: %v", err)
	}

	if _, err := DB.Exec(`ALTER TABLE player_base_buildings ADD COLUMN IF NOT EXISTS library_level INTEGER NOT NULL DEFAULT 0`); err != nil {
		log.Fatalf("EnsurePlayerBaseBuildingsTable add library_level: %v", err)
	}
	if _, err := DB.Exec(`ALTER TABLE player_base_buildings ADD COLUMN IF NOT EXISTS tavern_level INTEGER NOT NULL DEFAULT 0`); err != nil {
		log.Fatalf("EnsurePlayerBaseBuildingsTable add tavern_level: %v", err)
	}
}

func ensureBaseBuildingsRow(userID int) error {
	if _, err := DB.Exec(`
        INSERT INTO player_base_buildings (user_id)
        VALUES ($1)
        ON CONFLICT (user_id) DO NOTHING
    `, userID); err != nil {
		return fmt.Errorf("ensure base buildings row: %w", err)
	}
	return nil
}

func ensureBaseBuildingsRowTx(tx *sql.Tx, userID int) error {
	if _, err := tx.Exec(`
        INSERT INTO player_base_buildings (user_id)
        VALUES ($1)
        ON CONFLICT (user_id) DO NOTHING
    `, userID); err != nil {
		return fmt.Errorf("ensure base buildings row: %w", err)
	}
	return nil
}

func getBaseBuildingLevel(userID int, cfg baseBuildingConfig) (int, error) {
	if err := ensureBaseBuildingsRow(userID); err != nil {
		return 0, fmt.Errorf("Get%sLevel ensure row: %w", cfg.Name, err)
	}

	var level int
	query := fmt.Sprintf(`SELECT %s FROM player_base_buildings WHERE user_id = $1`, cfg.LevelColumn)
	if err := DB.QueryRow(query, userID).Scan(&level); err != nil {
		return 0, fmt.Errorf("Get%sLevel select: %w", cfg.Name, err)
	}
	return level, nil
}

func GetForgeLevel(userID int) (int, error) {
	return getBaseBuildingLevel(userID, forgeBuildingConfig)
}

func GetLibraryLevel(userID int) (int, error) {
	return getBaseBuildingLevel(userID, libraryBuildingConfig)
}

func GetTavernLevel(userID int) (int, error) {
	return getBaseBuildingLevel(userID, tavernBuildingConfig)
}

func buildBaseBuilding(userID int, cfg baseBuildingConfig) error {
	tx, err := DB.Begin()
	if err != nil {
		return fmt.Errorf("Build%s begin tx: %w", cfg.Name, err)
	}
	defer tx.Rollback()

	if err := ensureBaseBuildingsRowTx(tx, userID); err != nil {
		return fmt.Errorf("Build%s ensure row: %w", cfg.Name, err)
	}

	var level int
	levelQuery := fmt.Sprintf(`SELECT %s FROM player_base_buildings WHERE user_id = $1 FOR UPDATE`, cfg.LevelColumn)
	if err := tx.QueryRow(levelQuery, userID).Scan(&level); err != nil {
		return fmt.Errorf("Build%s lock building row: %w", cfg.Name, err)
	}
	if level > 0 {
		return errors.New(cfg.AlreadyBuiltError)
	}

	var invRaw string
	if err := tx.QueryRow(`SELECT inventory FROM players WHERE user_id = $1 FOR UPDATE`, userID).Scan(&invRaw); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return errors.New("player not found")
		}
		return fmt.Errorf("Build%s lock player inventory: %w", cfg.Name, err)
	}

	inv := make(map[string]map[string]interface{})
	if invRaw != "" && invRaw != "{}" {
		if err := json.Unmarshal([]byte(invRaw), &inv); err != nil {
			inv = make(map[string]map[string]interface{})
		}
	}

	woodID, err := getResourceIDByTypeTx(tx, "wood")
	if err != nil {
		return err
	}
	stoneID, err := getResourceIDByTypeTx(tx, "stone")
	if err != nil {
		return err
	}
	ironID, err := getResourceIDByTypeTx(tx, "iron")
	if err != nil {
		return err
	}

	woodKey := fmt.Sprintf("resource_%d", woodID)
	stoneKey := fmt.Sprintf("resource_%d", stoneID)
	ironKey := fmt.Sprintf("resource_%d", ironID)

	woodHave := getResourceCount(inv, woodKey)
	stoneHave := getResourceCount(inv, stoneKey)
	ironHave := getResourceCount(inv, ironKey)

	if woodHave < cfg.CostWood || stoneHave < cfg.CostStone || ironHave < cfg.CostIron {
		return errors.New("not enough resources")
	}

	setResourceCount(inv, woodKey, woodHave-cfg.CostWood, "wood")
	setResourceCount(inv, stoneKey, stoneHave-cfg.CostStone, "stone")
	setResourceCount(inv, ironKey, ironHave-cfg.CostIron, "iron")

	invBytes, err := json.Marshal(inv)
	if err != nil {
		return fmt.Errorf("Build%s marshal inventory: %w", cfg.Name, err)
	}

	if _, err := tx.Exec(`UPDATE players SET inventory = $1 WHERE user_id = $2`, string(invBytes), userID); err != nil {
		return fmt.Errorf("Build%s update player inventory: %w", cfg.Name, err)
	}

	updateQuery := fmt.Sprintf(`
        UPDATE player_base_buildings
        SET %s = 1,
            updated_at = CURRENT_TIMESTAMP
        WHERE user_id = $1
    `, cfg.LevelColumn)
	if _, err := tx.Exec(updateQuery, userID); err != nil {
		return fmt.Errorf("Build%s set building level: %w", cfg.Name, err)
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("Build%s commit: %w", cfg.Name, err)
	}
	return nil
}

func BuildForge(userID int) error {
	return buildBaseBuilding(userID, forgeBuildingConfig)
}

func BuildLibrary(userID int) error {
	return buildBaseBuilding(userID, libraryBuildingConfig)
}

func BuildTavern(userID int) error {
	return buildBaseBuilding(userID, tavernBuildingConfig)
}
