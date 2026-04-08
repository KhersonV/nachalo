
// =================================================
// gameservice/repository/match_player_structures.go
// =================================================

package repository

import (
	"database/sql"
	"fmt"
)

type MatchPlayerStructures struct {
	InstanceID         string
	UserID             int
	ScoutTowersCount   int
	TurretsCount       int
	WallsCount         int
	TotalBuildings     int
	HasScoutTowerBonus bool
}

func CreateMatchPlayerStructuresTable() {
	query := `
	CREATE TABLE IF NOT EXISTS match_player_structures (
		instance_id TEXT NOT NULL REFERENCES matches(instance_id) ON DELETE CASCADE,
		user_id INTEGER NOT NULL,
		scout_towers_count INTEGER NOT NULL DEFAULT 0,
		turrets_count INTEGER NOT NULL DEFAULT 0,
		walls_count INTEGER NOT NULL DEFAULT 0,
		total_buildings INTEGER NOT NULL DEFAULT 0,
		has_scout_tower_bonus BOOLEAN NOT NULL DEFAULT FALSE,
		PRIMARY KEY (instance_id, user_id)
	);
	`
	if _, err := DB.Exec(query); err != nil {
		panic("CreateMatchPlayerStructuresTable: " + err.Error())
	}
}

func UpdateMatchPlayerSightRange(instanceID string, userID int, newSightRange int) error {
	_, err := DB.Exec(`
		UPDATE match_players
		SET sight_range = $1
		WHERE instance_id = $2 AND user_id = $3
	`, newSightRange, instanceID, userID)
	return err
}

func GetMatchPlayerStructures(instanceID string, userID int) (*MatchPlayerStructures, error) {
	var s MatchPlayerStructures

	err := DB.QueryRow(`
		SELECT
			instance_id,
			user_id,
			scout_towers_count,
			turrets_count,
			walls_count,
			total_buildings,
			has_scout_tower_bonus
		FROM match_player_structures
		WHERE instance_id = $1 AND user_id = $2
	`, instanceID, userID).Scan(
		&s.InstanceID,
		&s.UserID,
		&s.ScoutTowersCount,
		&s.TurretsCount,
		&s.WallsCount,
		&s.TotalBuildings,
		&s.HasScoutTowerBonus,
	)
	if err == sql.ErrNoRows {
		if err := ensureMatchPlayerStructuresRow(instanceID, userID); err != nil {
			return nil, err
		}
		return &MatchPlayerStructures{
			InstanceID:         instanceID,
			UserID:             userID,
			ScoutTowersCount:   0,
			TurretsCount:       0,
			WallsCount:         0,
			TotalBuildings:     0,
			HasScoutTowerBonus: false,
		}, nil
	}
	if err != nil {
		return nil, err
	}

	return &s, nil
}

func ensureMatchPlayerStructuresRow(instanceID string, userID int) error {
	_, err := DB.Exec(`
		INSERT INTO match_player_structures (
			instance_id,
			user_id,
			scout_towers_count,
			turrets_count,
			walls_count,
			total_buildings,
			has_scout_tower_bonus
		)
		VALUES ($1, $2, 0, 0, 0, 0, FALSE)
		ON CONFLICT (instance_id, user_id) DO NOTHING
	`, instanceID, userID)
	return err
}

func structureTypeToColumn(structureType string) (string, error) {
	switch structureType {
	case "scout_tower":
		return "scout_towers_count", nil
	case "turret":
		return "turrets_count", nil
	case "wall":
		return "walls_count", nil
	default:
		return "", fmt.Errorf("unsupported structure type: %s", structureType)
	}
}

func IncrementStructureCount(instanceID string, userID int, structureType string) error {
	column, err := structureTypeToColumn(structureType)
	if err != nil {
		return err
	}

	if err := ensureMatchPlayerStructuresRow(instanceID, userID); err != nil {
		return err
	}

	query := fmt.Sprintf(`
		UPDATE match_player_structures
		SET
			%s = %s + 1,
			total_buildings = total_buildings + 1
		WHERE instance_id = $1 AND user_id = $2
	`, column, column)

	_, err = DB.Exec(query, instanceID, userID)
	return err
}

func DecrementStructureCount(instanceID string, userID int, structureType string) error {
	column, err := structureTypeToColumn(structureType)
	if err != nil {
		return err
	}

	if err := ensureMatchPlayerStructuresRow(instanceID, userID); err != nil {
		return err
	}

	query := fmt.Sprintf(`
		UPDATE match_player_structures
		SET
			%s = GREATEST(%s - 1, 0),
			total_buildings = GREATEST(total_buildings - 1, 0)
		WHERE instance_id = $1 AND user_id = $2
	`, column, column)

	_, err = DB.Exec(query, instanceID, userID)
	return err
}

func ApplyScoutTowerBonusIfNeeded(instanceID string, userID int) error {
	state, err := GetMatchPlayerStructures(instanceID, userID)
	if err != nil {
		return err
	}

	if state.HasScoutTowerBonus || state.ScoutTowersCount <= 0 {
		return nil
	}

	player, err := GetMatchPlayerByID(instanceID, userID)
	if err != nil {
		return err
	}

	newSightRange := player.SightRange + 1
	if newSightRange > 5 {
		newSightRange = 5
	}

	if err := UpdateMatchPlayerSightRange(instanceID, userID, newSightRange); err != nil {
		return err
	}

	_, err = DB.Exec(`
		UPDATE match_player_structures
		SET has_scout_tower_bonus = TRUE
		WHERE instance_id = $1 AND user_id = $2
	`, instanceID, userID)

	return err
}

func RemoveScoutTowerBonusIfNeeded(instanceID string, userID int) error {
	state, err := GetMatchPlayerStructures(instanceID, userID)
	if err != nil {
		return err
	}

	if !state.HasScoutTowerBonus || state.ScoutTowersCount > 0 {
		return nil
	}

	player, err := GetMatchPlayerByID(instanceID, userID)
	if err != nil {
		return err
	}

	newSightRange := player.SightRange - 1
	if newSightRange < 1 {
		newSightRange = 1
	}

	if err := UpdateMatchPlayerSightRange(instanceID, userID, newSightRange); err != nil {
		return err
	}

	_, err = DB.Exec(`
		UPDATE match_player_structures
		SET has_scout_tower_bonus = FALSE
		WHERE instance_id = $1 AND user_id = $2
	`, instanceID, userID)

	return err
}
