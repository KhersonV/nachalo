package repository

import (
    "database/sql"
    "encoding/json"
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
}

func GetForgeLevel(userID int) (int, error) {
    if _, err := DB.Exec(`
        INSERT INTO player_base_buildings (user_id, forge_level, library_level)
        VALUES ($1, 0, 0)
        ON CONFLICT (user_id) DO NOTHING
    `, userID); err != nil {
        return 0, fmt.Errorf("GetForgeLevel ensure row: %w", err)
    }

    var level int
    if err := DB.QueryRow(`SELECT forge_level FROM player_base_buildings WHERE user_id = $1`, userID).Scan(&level); err != nil {
        return 0, fmt.Errorf("GetForgeLevel select: %w", err)
    }
    return level, nil
}

func GetLibraryLevel(userID int) (int, error) {
    if _, err := DB.Exec(`
        INSERT INTO player_base_buildings (user_id, forge_level, library_level)
        VALUES ($1, 0, 0)
        ON CONFLICT (user_id) DO NOTHING
    `, userID); err != nil {
        return 0, fmt.Errorf("GetLibraryLevel ensure row: %w", err)
    }

    var level int
    if err := DB.QueryRow(`SELECT library_level FROM player_base_buildings WHERE user_id = $1`, userID).Scan(&level); err != nil {
        return 0, fmt.Errorf("GetLibraryLevel select: %w", err)
    }
    return level, nil
}

func BuildForge(userID int) error {
    tx, err := DB.Begin()
    if err != nil {
        return fmt.Errorf("BuildForge begin tx: %w", err)
    }
    defer tx.Rollback()

    if _, err := tx.Exec(`
        INSERT INTO player_base_buildings (user_id, forge_level, library_level)
        VALUES ($1, 0, 0)
        ON CONFLICT (user_id) DO NOTHING
    `, userID); err != nil {
        return fmt.Errorf("BuildForge ensure row: %w", err)
    }

    var forgeLevel int
    if err := tx.QueryRow(`SELECT forge_level FROM player_base_buildings WHERE user_id = $1 FOR UPDATE`, userID).Scan(&forgeLevel); err != nil {
        return fmt.Errorf("BuildForge lock forge row: %w", err)
    }
    if forgeLevel > 0 {
        return fmt.Errorf("forge already built")
    }

    var invRaw string
    if err := tx.QueryRow(`SELECT inventory FROM players WHERE user_id = $1 FOR UPDATE`, userID).Scan(&invRaw); err != nil {
        return fmt.Errorf("BuildForge lock player inventory: %w", err)
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

    if woodHave < ForgeCostWood || stoneHave < ForgeCostStone || ironHave < ForgeCostIron {
        return fmt.Errorf("not enough resources")
    }

    setResourceCount(inv, woodKey, woodHave-ForgeCostWood, "wood")
    setResourceCount(inv, stoneKey, stoneHave-ForgeCostStone, "stone")
    setResourceCount(inv, ironKey, ironHave-ForgeCostIron, "iron")

    invBytes, err := json.Marshal(inv)
    if err != nil {
        return fmt.Errorf("BuildForge marshal inventory: %w", err)
    }

    if _, err := tx.Exec(`UPDATE players SET inventory = $1 WHERE user_id = $2`, string(invBytes), userID); err != nil {
        return fmt.Errorf("BuildForge update player inventory: %w", err)
    }

    if _, err := tx.Exec(`
        UPDATE player_base_buildings
        SET forge_level = 1,
            updated_at = CURRENT_TIMESTAMP
        WHERE user_id = $1
    `, userID); err != nil {
        return fmt.Errorf("BuildForge set forge level: %w", err)
    }

    if err := tx.Commit(); err != nil {
        return fmt.Errorf("BuildForge commit: %w", err)
    }
    return nil
}

func BuildLibrary(userID int) error {
    tx, err := DB.Begin()
    if err != nil {
        return fmt.Errorf("BuildLibrary begin tx: %w", err)
    }
    defer tx.Rollback()

    if _, err := tx.Exec(`
        INSERT INTO player_base_buildings (user_id, forge_level, library_level)
        VALUES ($1, 0, 0)
        ON CONFLICT (user_id) DO NOTHING
    `, userID); err != nil {
        return fmt.Errorf("BuildLibrary ensure row: %w", err)
    }

    var libraryLevel int
    if err := tx.QueryRow(`SELECT library_level FROM player_base_buildings WHERE user_id = $1 FOR UPDATE`, userID).Scan(&libraryLevel); err != nil {
        return fmt.Errorf("BuildLibrary lock library row: %w", err)
    }
    if libraryLevel > 0 {
        return fmt.Errorf("library already built")
    }

    var invRaw string
    if err := tx.QueryRow(`SELECT inventory FROM players WHERE user_id = $1 FOR UPDATE`, userID).Scan(&invRaw); err != nil {
        return fmt.Errorf("BuildLibrary lock player inventory: %w", err)
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

    if woodHave < LibraryCostWood || stoneHave < LibraryCostStone || ironHave < LibraryCostIron {
        return fmt.Errorf("not enough resources")
    }

    setResourceCount(inv, woodKey, woodHave-LibraryCostWood, "wood")
    setResourceCount(inv, stoneKey, stoneHave-LibraryCostStone, "stone")
    setResourceCount(inv, ironKey, ironHave-LibraryCostIron, "iron")

    invBytes, err := json.Marshal(inv)
    if err != nil {
        return fmt.Errorf("BuildLibrary marshal inventory: %w", err)
    }

    if _, err := tx.Exec(`UPDATE players SET inventory = $1 WHERE user_id = $2`, string(invBytes), userID); err != nil {
        return fmt.Errorf("BuildLibrary update player inventory: %w", err)
    }

    if _, err := tx.Exec(`
        UPDATE player_base_buildings
        SET library_level = 1,
            updated_at = CURRENT_TIMESTAMP
        WHERE user_id = $1
    `, userID); err != nil {
        return fmt.Errorf("BuildLibrary set library level: %w", err)
    }

    if err := tx.Commit(); err != nil {
        return fmt.Errorf("BuildLibrary commit: %w", err)
    }
    return nil
}
