package repository

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"gameservice/internal/game/balance"
	"gameservice/internal/hexgrid"
)

type HexRouteResult struct {
	Path          []hexgrid.Coord `json:"path"`
	StepCost      int             `json:"stepCost"`
	TotalCost     int             `json:"totalCost"`
	Energy        int             `json:"energy"`
	Affordable    bool            `json:"affordable"`
	Position      hexgrid.Coord   `json:"position"`
	StateRevision int64           `json:"stateRevision,omitempty"`
	MapRevision   int64           `json:"mapRevision,omitempty"`
	Replayed      bool            `json:"-"`
}

func (r *HexRouteResult) SetCommandRevisions(state, mapRevision int64) {
	r.StateRevision, r.MapRevision = state, mapRevision
}

// PreviewHexRoute is a read-only hint. ApplyHexMove rechecks everything under
// the same matches-row lock used by square commands.
func PreviewHexRoute(ctx context.Context, instanceID string, userID int, destination hexgrid.Coord) (HexRouteResult, error) {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	tx, err := DB.BeginTx(ctx, &sql.TxOptions{ReadOnly: true, Isolation: sql.LevelRepeatableRead})
	if err != nil {
		return HexRouteResult{}, err
	}
	defer tx.Rollback()
	route, err := findHexRouteTx(ctx, tx, instanceID, userID, destination)
	if err != nil {
		return HexRouteResult{}, err
	}
	if err := tx.Commit(); err != nil {
		return HexRouteResult{}, err
	}
	return route, nil
}

func ApplyHexMove(ctx context.Context, instanceID string, userID int, commandID string, destination hexgrid.Coord) (HexRouteResult, error) {
	return ApplyHexMoveWithPublish(ctx, instanceID, userID, commandID, destination, nil)
}

// publish is invoked only after Commit succeeds, never for a replay.
func ApplyHexMoveWithPublish(ctx context.Context, instanceID string, userID int, commandID string, destination hexgrid.Coord, publish func(HexRouteResult)) (HexRouteResult, error) {
	var empty HexRouteResult
	if !hexgrid.Valid(destination) {
		return empty, ErrCommandRejected
	}
	ctx, cancel := context.WithTimeout(ctx, 8*time.Second)
	defer cancel()
	canonical := fmt.Sprintf("hex-move-v1:%d:%d:%d", userID, destination.Q, destination.R)
	c, err := BeginMatchCommand(ctx, instanceID, userID, commandID, canonical)
	if err != nil {
		return empty, err
	}
	defer c.Rollback()
	if c.Replay != nil {
		if err := json.Unmarshal(c.Replay, &empty); err != nil {
			return HexRouteResult{}, err
		}
		empty.Replayed = true
		return empty, nil
	}
	if c.ActiveUser != userID {
		return empty, ErrCommandRejected
	}
	route, err := findHexRouteTx(ctx, c.Tx, instanceID, userID, destination)
	if err != nil {
		return empty, err
	}
	if !route.Affordable {
		return empty, ErrHexEnergy
	}
	start := route.Position
	result, err := c.Tx.ExecContext(ctx, `UPDATE match_players SET position_q=$1,position_r=$2,energy=energy-$3 WHERE instance_id=$4 AND user_id=$5 AND position_q=$6 AND position_r=$7 AND energy>=$3 AND health>0`, destination.Q, destination.R, route.TotalCost, instanceID, userID, start.Q, start.R)
	if err != nil {
		return empty, err
	}
	if n, _ := result.RowsAffected(); n != 1 {
		return empty, ErrCommandRejected
	}
	for _, update := range []struct {
		pos      hexgrid.Coord
		occupied bool
	}{{start, false}, {destination, true}} {
		result, err = c.Tx.ExecContext(ctx, `UPDATE match_map_cells SET is_player=$1 WHERE instance_id=$2 AND q=$3 AND r=$4`, update.occupied, instanceID, update.pos.Q, update.pos.R)
		if err != nil {
			return empty, err
		}
		if n, _ := result.RowsAffected(); n != 1 {
			return empty, ErrCommandRejected
		}
	}
	visited := append([]hexgrid.Coord{start}, route.Path...)
	if err := recordHexExploredTx(ctx, c.Tx, instanceID, userID, visited...); err != nil {
		return empty, err
	}
	route.Position = destination
	route.Energy -= route.TotalCost
	route.Affordable = route.Energy >= 0
	route.StateRevision, route.MapRevision, err = c.Commit(ctx, &route, true)
	if err != nil {
		return empty, err
	}
	if publish != nil {
		publish(route)
	}
	return route, nil
}

func findHexRouteTx(ctx context.Context, tx *sql.Tx, instanceID string, userID int, destination hexgrid.Coord) (HexRouteResult, error) {
	var out HexRouteResult
	if !hexgrid.Valid(destination) {
		return out, ErrCommandRejected
	}
	var mode string
	var version int
	var active int
	err := tx.QueryRowContext(ctx, `SELECT geometry->>'kind',schema_version,COALESCE(active_user_id,0) FROM matches WHERE instance_id=$1 AND finished_at IS NULL`, instanceID).Scan(&mode, &version, &active)
	if err != nil {
		return out, err
	}
	if mode != "hex" || version != 2 || active != userID {
		return out, ErrHexUnavailable
	}
	var health, mobility int
	err = tx.QueryRowContext(ctx, `SELECT position_q,position_r,energy,health,mobility FROM match_players WHERE instance_id=$1 AND user_id=$2`, instanceID, userID).
		Scan(&out.Position.Q, &out.Position.R, &out.Energy, &health, &mobility)
	if err != nil {
		return out, err
	}
	if health <= 0 || out.Position == destination {
		return out, ErrCommandRejected
	}
	visible, err := HexVisibleCellsTx(ctx, tx, instanceID, userID)
	if err != nil {
		return out, err
	}
	if !visible[destination] {
		return out, ErrHexBlocked
	}
	rows, err := tx.QueryContext(ctx, `SELECT q,r,tile_code,is_player,structure_type,is_under_construction FROM match_map_cells WHERE instance_id=$1`, instanceID)
	if err != nil {
		return out, err
	}
	walkable := map[hexgrid.Coord]bool{}
	for rows.Next() {
		var pos hexgrid.Coord
		var tile int
		var occupied, building bool
		var structure string
		if err := rows.Scan(&pos.Q, &pos.R, &tile, &occupied, &structure, &building); err != nil {
			rows.Close()
			return out, err
		}
		walkable[pos] = (tile == 48 || tile == 82 || tile == 66) && !occupied && structure == "" && !building
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return out, err
	}
	if !walkable[destination] {
		return out, ErrHexBlocked
	}
	walkable[out.Position] = true
	rows, err = tx.QueryContext(ctx, `SELECT pos_q,pos_r FROM match_monsters WHERE instance_id=$1 AND health>0`, instanceID)
	if err != nil {
		return out, err
	}
	for rows.Next() {
		var pos hexgrid.Coord
		if err := rows.Scan(&pos.Q, &pos.R); err != nil {
			rows.Close()
			return out, err
		}
		walkable[pos] = false
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return out, err
	}
	rows, err = tx.QueryContext(ctx, `SELECT position_q,position_r FROM match_players WHERE instance_id=$1 AND user_id<>$2 AND health>0`, instanceID, userID)
	if err != nil {
		return out, err
	}
	for rows.Next() {
		var pos hexgrid.Coord
		if err := rows.Scan(&pos.Q, &pos.R); err != nil {
			rows.Close()
			return out, err
		}
		walkable[pos] = false
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return out, err
	}
	if !walkable[destination] {
		return out, ErrHexBlocked
	}
	edges, err := loadHexEdges(ctx, tx, instanceID)
	if err != nil {
		return out, err
	}
	walls := map[hexgrid.EdgeKey]bool{}
	for _, edge := range edges {
		next, err := hexgrid.Neighbor(edge.Position, edge.Direction)
		if err != nil {
			return out, err
		}
		key, err := hexgrid.CanonicalEdge(edge.Position, next)
		if err != nil {
			return out, err
		}
		walls[key] = true
	}
	previous := map[hexgrid.Coord]hexgrid.Coord{}
	seen := map[hexgrid.Coord]bool{out.Position: true}
	queue := []hexgrid.Coord{out.Position}
	for len(queue) > 0 && !seen[destination] {
		current := queue[0]
		queue = queue[1:]
		for direction := 0; direction < 6; direction++ {
			next, err := hexgrid.Neighbor(current, direction)
			if err != nil || seen[next] || !walkable[next] || !visible[next] {
				continue
			}
			key, _ := hexgrid.CanonicalEdge(current, next)
			if walls[key] {
				continue
			}
			seen[next] = true
			previous[next] = current
			queue = append(queue, next)
		}
	}
	if !seen[destination] {
		return out, ErrHexBlocked
	}
	for pos := destination; pos != out.Position; pos = previous[pos] {
		out.Path = append(out.Path, pos)
	}
	for i, j := 0, len(out.Path)-1; i < j; i, j = i+1, j-1 {
		out.Path[i], out.Path[j] = out.Path[j], out.Path[i]
	}
	if len(out.Path) > 128 {
		return HexRouteResult{}, ErrHexBlocked
	}
	out.StepCost = balance.MoveEnergyCost(mobility)
	out.TotalCost = len(out.Path) * out.StepCost
	out.Affordable = out.Energy >= out.TotalCost
	return out, nil
}

func HexMoveErrorStatus(err error) int {
	switch {
	case errors.Is(err, ErrCommandConflict):
		return 409
	case errors.Is(err, ErrHexEnergy):
		return 422
	case errors.Is(err, ErrHexBlocked), errors.Is(err, ErrCommandRejected):
		return 400
	case errors.Is(err, sql.ErrNoRows), errors.Is(err, ErrHexUnavailable):
		return 404
	default:
		return 500
	}
}
