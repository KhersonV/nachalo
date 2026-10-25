package repository

import (
	"context"
	"database/sql"
	"encoding/json"

	"gameservice/internal/hexgrid"
	"gameservice/internal/hexmap"
)

// Fallback for the standalone illustrative preview only. Playable matches use p.sight_range.
func hexVisionRadius() int { return 2 }

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
	return hexVisibleCellsFromTx(ctx, tx, instanceID, userID, nil)
}

func HexVisibleCellsFromTx(ctx context.Context, tx *sql.Tx, instanceID string, userID int, origin hexgrid.Coord) (map[hexgrid.Coord]bool, error) {
	return hexVisibleCellsFromTx(ctx, tx, instanceID, userID, &origin)
}

func hexVisibleCellsFromTx(ctx context.Context, tx *sql.Tx, instanceID string, userID int, origin *hexgrid.Coord) (map[hexgrid.Coord]bool, error) {
	rows, err := tx.QueryContext(ctx, `SELECT p.user_id,p.position_q,p.position_r,p.sight_range FROM match_players p LEFT JOIN match_player_structures s ON s.instance_id=p.instance_id AND s.user_id=p.user_id WHERE p.instance_id=$1 AND p.group_id=(SELECT group_id FROM match_players WHERE instance_id=$1 AND user_id=$2) AND p.health>0`, instanceID, userID)
	if err != nil {
		return nil, err
	}
	positions := []hexgrid.Coord{}
	radii := map[hexgrid.Coord]int{}
	for rows.Next() {
		var p hexgrid.Coord
		var sight, id int
		if err := rows.Scan(&id, &p.Q, &p.R, &sight); err != nil {
			rows.Close()
			return nil, err
		}
		if origin != nil && id == userID {
			p = *origin
		}
		positions = append(positions, p)
		radii[p] = sight
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
	return hexVisibleWithRadii(positions, radii, cells, edges), nil
}

func hexSightRadius(bonus bool) int {
	radius := hexVisionRadius()
	if bonus && radius < 5 {
		radius++
	}
	return radius
}

func hexVisible(positions []hexgrid.Coord, cells []hexmap.Cell, edges []hexmap.Edge) map[hexgrid.Coord]bool {
	return hexVisibleWithRadii(positions, nil, cells, edges)
}

func hexVisibleWithRadii(positions []hexgrid.Coord, radii map[hexgrid.Coord]int, cells []hexmap.Cell, edges []hexmap.Edge) map[hexgrid.Coord]bool {
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
			radius := hexVisionRadius()
			if radii != nil {
				radius = radii[origin]
			}
			if err != nil || d > radius {
				continue
			}
			if d == 0 || hexgrid.Visible(origin, cell.Position, walkable, walls) {
				visible[cell.Position] = true
			}
		}
	}
	return visible
}

// Reveal only rock touching a legitimately visible floor cell. This is visual
// discovery for snapshots, never a substitute for gameplay line of sight.
func hexDiscoveredRock(positions []hexgrid.Coord, radii map[hexgrid.Coord]int, cells []hexmap.Cell, visible map[hexgrid.Coord]bool) map[hexgrid.Coord]bool {
	terrain := make(map[hexgrid.Coord]string, len(cells))
	for _, cell := range cells {
		terrain[cell.Position] = cell.Terrain
	}
	discovered := map[hexgrid.Coord]bool{}
	for floor := range visible {
		if terrain[floor] != "floor" {
			continue
		}
		for direction := 0; direction < 6; direction++ {
			neighbor, err := hexgrid.Neighbor(floor, direction)
			if err != nil || terrain[neighbor] != "rock" {
				continue
			}
			for _, origin := range positions {
				limit := hexVisionRadius()
				if radii != nil {
					limit = radii[origin]
				}
				distance, err := hexgrid.Distance(origin, neighbor)
				if err == nil && distance <= limit {
					discovered[neighbor] = true
					break
				}
			}
		}
	}
	return discovered
}

// Called under the match-row lock before the move commits. The stored set is
// a discovery history; currently visible cells are always recomputed.
func recordHexExploredTx(ctx context.Context, tx *sql.Tx, instanceID string, userID int, positions ...hexgrid.Coord) error {
	var saved []byte
	var sight int
	if err := tx.QueryRowContext(ctx, `SELECT p.hex_explored,p.sight_range FROM match_players p LEFT JOIN match_player_structures s ON s.instance_id=p.instance_id AND s.user_id=p.user_id WHERE p.instance_id=$1 AND p.user_id=$2`, instanceID, userID).Scan(&saved, &sight); err != nil {
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
	radii := map[hexgrid.Coord]int{}
	for _, pos := range positions {
		radii[pos] = sight
	}
	visible := hexVisibleWithRadii(positions, radii, cells, edges)
	for pos := range visible {
		known[pos] = true
	}
	for pos := range hexDiscoveredRock(positions, radii, cells, visible) {
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
