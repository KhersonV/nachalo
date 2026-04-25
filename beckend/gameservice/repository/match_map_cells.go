package repository

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"log"

	"gameservice/game"
)

var errDBNotInitialized = errors.New("repository DB is not initialized")

func CreateMatchMapCellsTable() {
	query := `
	CREATE TABLE IF NOT EXISTS match_map_cells (
		instance_id TEXT NOT NULL REFERENCES matches(instance_id) ON DELETE CASCADE,
		cell_id INTEGER NOT NULL,
		x INTEGER NOT NULL,
		y INTEGER NOT NULL,
		tile_code INTEGER NOT NULL,
		resource JSONB,
		barbel JSONB,
		monster JSONB,
		is_portal BOOLEAN NOT NULL DEFAULT FALSE,
		is_player BOOLEAN NOT NULL DEFAULT FALSE,
		structure_type TEXT NOT NULL DEFAULT '',
		structure_owner_user_id INTEGER NOT NULL DEFAULT 0,
		structure_health INTEGER NOT NULL DEFAULT 0,
		structure_defense INTEGER NOT NULL DEFAULT 0,
		structure_attack INTEGER NOT NULL DEFAULT 0,
		structure_energy INTEGER NOT NULL DEFAULT 0,
		is_under_construction BOOLEAN NOT NULL DEFAULT FALSE,
		construction_turns_left INTEGER NOT NULL DEFAULT 0,
		PRIMARY KEY (instance_id, cell_id)
	);
	CREATE UNIQUE INDEX IF NOT EXISTS idx_match_map_cells_instance_xy
		ON match_map_cells(instance_id, x, y);
	`
	if _, err := DB.Exec(query); err != nil {
		log.Fatalf("Ошибка создания таблицы match_map_cells: %v", err)
	}
}

func EnsureMatchMapCellsBackfilled(instanceID string) error {
	if DB == nil {
		return errDBNotInitialized
	}

	var rowsCount int
	if err := DB.QueryRow(
		`SELECT COUNT(1) FROM match_map_cells WHERE instance_id = $1`,
		instanceID,
	).Scan(&rowsCount); err != nil {
		return err
	}
	if rowsCount > 0 {
		return nil
	}

	var raw []byte
	err := DB.QueryRow(
		`SELECT map FROM matches WHERE instance_id = $1`,
		instanceID,
	).Scan(&raw)
	if err != nil {
		return err
	}
	if len(raw) == 0 || string(raw) == "null" {
		return nil
	}

	var cells []game.FullCell
	if err := json.Unmarshal(raw, &cells); err != nil {
		return fmt.Errorf("EnsureMatchMapCellsBackfilled: unmarshal legacy map: %w", err)
	}
	if len(cells) == 0 {
		return nil
	}

	return replaceMatchMapCells(instanceID, cells)
}

func LoadMapCell(instanceID string, x int, y int) (*game.FullCell, error) {
	if err := EnsureMatchMapCellsBackfilled(instanceID); err != nil {
		return nil, err
	}

	row := DB.QueryRow(`
		SELECT
			cell_id,
			x,
			y,
			tile_code,
			resource,
			barbel,
			monster,
			is_portal,
			is_player,
			structure_type,
			structure_owner_user_id,
			structure_health,
			structure_defense,
			structure_attack,
			structure_energy,
			is_under_construction,
			construction_turns_left
		FROM match_map_cells
		WHERE instance_id = $1 AND x = $2 AND y = $3
	`, instanceID, x, y)

	cell, err := scanMatchMapCell(row)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}

	return cell, nil
}

func LoadMapCellForUpdateTx(tx *sql.Tx, instanceID string, x int, y int) (*game.FullCell, error) {
	if err := EnsureMatchMapCellsBackfilled(instanceID); err != nil {
		return nil, err
	}

	row := tx.QueryRow(`
		SELECT
			cell_id,
			x,
			y,
			tile_code,
			resource,
			barbel,
			monster,
			is_portal,
			is_player,
			structure_type,
			structure_owner_user_id,
			structure_health,
			structure_defense,
			structure_attack,
			structure_energy,
			is_under_construction,
			construction_turns_left
		FROM match_map_cells
		WHERE instance_id = $1 AND x = $2 AND y = $3
		FOR UPDATE
	`, instanceID, x, y)

	cell, err := scanMatchMapCell(row)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}

	return cell, nil
}

func SaveMapCell(instanceID string, cell game.FullCell) error {
	if DB == nil {
		return errDBNotInitialized
	}
	if err := EnsureMatchMapCellsBackfilled(instanceID); err != nil && err != sql.ErrNoRows {
		return err
	}
	return upsertMatchMapCell(DB, instanceID, cell)
}

func SaveMapCellTx(tx *sql.Tx, instanceID string, cell game.FullCell) error {
	return upsertMatchMapCell(tx, instanceID, cell)
}

func InsertMatchMapCells(instanceID string, cells []game.FullCell) error {
	return replaceMatchMapCells(instanceID, cells)
}

func CountBarrelCells(instanceID string) (int, error) {
	if err := EnsureMatchMapCellsBackfilled(instanceID); err != nil {
		return 0, err
	}

	var count int
	err := DB.QueryRow(`
		SELECT COUNT(1)
		FROM match_map_cells
		WHERE instance_id = $1
		  AND barbel IS NOT NULL
		  AND barbel <> 'null'::jsonb
	`, instanceID).Scan(&count)
	return count, err
}

type matchMapCellScanner interface {
	Scan(dest ...interface{}) error
}

type matchMapCellExecer interface {
	Exec(query string, args ...interface{}) (sql.Result, error)
}

func scanMatchMapCell(scanner matchMapCellScanner) (*game.FullCell, error) {
	var (
		cell                               game.FullCell
		resourceRaw, barbelRaw, monsterRaw []byte
	)

	err := scanner.Scan(
		&cell.CellID,
		&cell.X,
		&cell.Y,
		&cell.TileCode,
		&resourceRaw,
		&barbelRaw,
		&monsterRaw,
		&cell.IsPortal,
		&cell.IsPlayer,
		&cell.StructureType,
		&cell.StructureOwnerUserID,
		&cell.StructureHealth,
		&cell.StructureDefense,
		&cell.StructureAttack,
		&cell.StructureEnergy,
		&cell.IsUnderConstruction,
		&cell.ConstructionTurnsLeft,
	)
	if err != nil {
		return nil, err
	}

	if len(resourceRaw) > 0 && string(resourceRaw) != "null" {
		var resource game.ResourceData
		if err := json.Unmarshal(resourceRaw, &resource); err != nil {
			return nil, err
		}
		cell.Resource = &resource
	}

	if len(barbelRaw) > 0 && string(barbelRaw) != "null" {
		var barbel game.ResourceData
		if err := json.Unmarshal(barbelRaw, &barbel); err != nil {
			return nil, err
		}
		cell.Barbel = &barbel
	}

	if len(monsterRaw) > 0 && string(monsterRaw) != "null" {
		var monster game.MonsterData
		if err := json.Unmarshal(monsterRaw, &monster); err != nil {
			return nil, err
		}
		cell.Monster = &monster
	}

	return &cell, nil
}

func replaceMatchMapCells(instanceID string, cells []game.FullCell) error {
	if DB == nil {
		return errDBNotInitialized
	}

	tx, err := DB.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	for _, cell := range cells {
		if err := upsertMatchMapCell(tx, instanceID, cell); err != nil {
			return err
		}
	}

	return tx.Commit()
}

func upsertMatchMapCell(execer matchMapCellExecer, instanceID string, cell game.FullCell) error {
	resourceRaw, err := marshalNullableJSON(cell.Resource)
	if err != nil {
		return err
	}
	barbelRaw, err := marshalNullableJSON(cell.Barbel)
	if err != nil {
		return err
	}
	monsterRaw, err := marshalNullableJSON(cell.Monster)
	if err != nil {
		return err
	}

	_, err = execer.Exec(`
		INSERT INTO match_map_cells (
			instance_id,
			cell_id,
			x,
			y,
			tile_code,
			resource,
			barbel,
			monster,
			is_portal,
			is_player,
			structure_type,
			structure_owner_user_id,
			structure_health,
			structure_defense,
			structure_attack,
			structure_energy,
			is_under_construction,
			construction_turns_left
		) VALUES (
			$1, $2, $3, $4, $5,
			$6::jsonb, $7::jsonb, $8::jsonb,
			$9, $10, $11, $12, $13, $14, $15, $16, $17, $18
		)
		ON CONFLICT (instance_id, cell_id) DO UPDATE SET
			x = EXCLUDED.x,
			y = EXCLUDED.y,
			tile_code = EXCLUDED.tile_code,
			resource = EXCLUDED.resource,
			barbel = EXCLUDED.barbel,
			monster = EXCLUDED.monster,
			is_portal = EXCLUDED.is_portal,
			is_player = EXCLUDED.is_player,
			structure_type = EXCLUDED.structure_type,
			structure_owner_user_id = EXCLUDED.structure_owner_user_id,
			structure_health = EXCLUDED.structure_health,
			structure_defense = EXCLUDED.structure_defense,
			structure_attack = EXCLUDED.structure_attack,
			structure_energy = EXCLUDED.structure_energy,
			is_under_construction = EXCLUDED.is_under_construction,
			construction_turns_left = EXCLUDED.construction_turns_left
	`, instanceID,
		cell.CellID,
		cell.X,
		cell.Y,
		cell.TileCode,
		resourceRaw,
		barbelRaw,
		monsterRaw,
		cell.IsPortal,
		cell.IsPlayer,
		cell.StructureType,
		cell.StructureOwnerUserID,
		cell.StructureHealth,
		cell.StructureDefense,
		cell.StructureAttack,
		cell.StructureEnergy,
		cell.IsUnderConstruction,
		cell.ConstructionTurnsLeft,
	)
	return err
}

func marshalNullableJSON(value interface{}) ([]byte, error) {
	if value == nil {
		return []byte("null"), nil
	}
	return json.Marshal(value)
}
