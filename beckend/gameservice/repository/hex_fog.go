package repository

import (
	"context"
	"database/sql"
	"encoding/json"
	"os"
	"strconv"

	"gameservice/internal/hexgrid"
	"gameservice/internal/hexmap"
)

func hexVisionRadius() int {
	if n, err := strconv.Atoi(os.Getenv("HEX_VISION_RADIUS")); err == nil && n >= 1 && n <= 12 {
		return n
	}
	return 3
}

func RecordHexExploredForPush(ctx context.Context, tx *sql.Tx, instanceID string, userID int, positions ...hexgrid.Coord) error {
	return recordHexExploredTx(ctx, tx, instanceID, userID, positions...)
}

// HexPositionVisibleTx makes command validation use the same team vision as
// personalized snapshots. It deliberately reads under the caller's Tx.
func HexPositionVisibleTx(ctx context.Context, tx *sql.Tx, instanceID string, userID int, destination hexgrid.Coord) (bool, error) {
	visible, err := HexVisibleCellsTx(ctx, tx, instanceID, userID)
	return visible[destination], err
}

func HexVisibleCellsTx(ctx context.Context, tx *sql.Tx, instanceID string, userID int) (map[hexgrid.Coord]bool, error) {
	rows, err := tx.QueryContext(ctx, `SELECT p.position_q,p.position_r FROM match_players p WHERE p.instance_id=$1 AND p.group_id=(SELECT group_id FROM match_players WHERE instance_id=$1 AND user_id=$2) AND p.health>0`, instanceID, userID)
	if err != nil {
		return nil, err
	}
	positions := []hexgrid.Coord{}
	for rows.Next() {
		var p hexgrid.Coord
		if err := rows.Scan(&p.Q, &p.R); err != nil {
			rows.Close()
			return nil, err
		}
		positions = append(positions, p)
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return nil, err
	}
	rows.Close()
	rows, err = tx.QueryContext(ctx, `SELECT q,r,tile_code FROM match_map_cells WHERE instance_id=$1`, instanceID)
	if err != nil {
		return nil, err
	}
	cells := []hexmap.Cell{}
	for rows.Next() {
		var cell hexmap.Cell
		var tile int
		if err := rows.Scan(&cell.Position.Q, &cell.Position.R, &tile); err != nil {
			rows.Close()
			return nil, err
		}
		if tile != 32 {
			cell.Terrain = "floor"
		} else {
			cell.Terrain = "rock"
		}
		cells = append(cells, cell)
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return nil, err
	}
	rows.Close()
	edges, err := loadHexEdges(ctx, tx, instanceID)
	if err != nil {
		return nil, err
	}
	return hexVisible(positions, cells, edges), nil
}

func hexVisible(positions []hexgrid.Coord, cells []hexmap.Cell, edges []hexmap.Edge) map[hexgrid.Coord]bool {
	walkable := make(map[hexgrid.Coord]bool, len(cells))
	for _, cell := range cells {
		walkable[cell.Position] = cell.Terrain == "floor"
	}
	walls := make(map[hexgrid.EdgeKey]bool, len(edges))
	for _, edge := range edges {
		next, err := hexgrid.Neighbor(edge.Position, edge.Direction)
		if err == nil {
			key, err := hexgrid.CanonicalEdge(edge.Position, next)
			if err == nil {
				walls[key] = true
			}
		}
	}
	visible := map[hexgrid.Coord]bool{}
	for _, origin := range positions {
		visible[origin] = true
		for _, cell := range cells {
			d, err := hexgrid.Distance(origin, cell.Position)
			if err != nil || d > hexVisionRadius() {
				continue
			}
			if d == 0 || hexgrid.Visible(origin, cell.Position, walkable, walls) {
				visible[cell.Position] = true
			}
		}
	}
	return visible
}

// Called under the match-row lock before the move commits. The stored set is
// a discovery history; currently visible cells are always recomputed.
func recordHexExploredTx(ctx context.Context, tx *sql.Tx, instanceID string, userID int, positions ...hexgrid.Coord) error {
	var saved []byte
	if err := tx.QueryRowContext(ctx, `SELECT hex_explored FROM match_players WHERE instance_id=$1 AND user_id=$2`, instanceID, userID).Scan(&saved); err != nil {
		return err
	}
	var prior []hexgrid.Coord
	if err := json.Unmarshal(saved, &prior); err != nil {
		return err
	}
	rows, err := tx.QueryContext(ctx, `SELECT q,r,tile_code FROM match_map_cells WHERE instance_id=$1`, instanceID)
	if err != nil {
		return err
	}
	cells := []hexmap.Cell{}
	for rows.Next() {
		var cell hexmap.Cell
		var tile int
		if err := rows.Scan(&cell.Position.Q, &cell.Position.R, &tile); err != nil {
			rows.Close()
			return err
		}
		if tile != 32 {
			cell.Terrain = "floor"
		} else {
			cell.Terrain = "rock"
		}
		cells = append(cells, cell)
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return err
	}
	rows.Close()
	edges, err := loadHexEdges(ctx, tx, instanceID)
	if err != nil {
		return err
	}
	known := map[hexgrid.Coord]bool{}
	for _, pos := range prior {
		known[pos] = true
	}
	for pos := range hexVisible(positions, cells, edges) {
		known[pos] = true
	}
	merged := make([]hexgrid.Coord, 0, len(known))
	for pos := range known {
		merged = append(merged, pos)
	}
	raw, err := json.Marshal(merged)
	if err != nil {
		return err
	}
	_, err = tx.ExecContext(ctx, `UPDATE match_players SET hex_explored=$3::jsonb WHERE instance_id=$1 AND user_id=$2`, instanceID, userID, raw)
	return err
}
