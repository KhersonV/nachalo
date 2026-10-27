package repository

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"gameservice/internal/hexgrid"
)

type HexRouteResult struct {
	Destination       hexgrid.Coord `json:"destination"`
	ReachablePosition hexgrid.Coord `json:"reachablePosition"`
	AffordableSteps   int           `json:"affordableSteps"`
	ReachableCost     int           `json:"reachableCost"`
	StepCosts         []int         `json:"stepCosts"`
	PreviewToken      string        `json:"previewToken"`
	Partial           bool          `json:"partial"`

	Budget        *HexBudgets        `json:"budget,omitempty"`
	ActionEffects []map[string]any   `json:"actionEffects,omitempty"`
	Path          []hexgrid.Coord    `json:"path"`
	StepCost      int                `json:"stepCost"`
	TotalCost     int                `json:"totalCost"`
	MP            int                `json:"mp"`
	Affordable    bool               `json:"affordable"`
	Position      hexgrid.Coord      `json:"position"`
	StateRevision int64              `json:"stateRevision,omitempty"`
	MapRevision   int64              `json:"mapRevision,omitempty"`
	Replayed      bool               `json:"-"`
	AuraMPCost    int                `json:"auraMPCost,omitempty"`
	AuraHits      []HexAuraVisualHit `json:"auraHits,omitempty"`
	AuraEnded     bool               `json:"auraEnded,omitempty"`
}

type HexAuraVisualHit struct {
	Position hexgrid.Coord `json:"position"`
	Damage   int           `json:"damage"`
	SourceID int           `json:"sourceId"`
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
	return ApplyHexMoveWithPreview(ctx, instanceID, userID, commandID, destination, "", publish)
}

func ApplyHexMoveWithPreview(ctx context.Context, instanceID string, userID int, commandID string, destination hexgrid.Coord, token string, publish func(HexRouteResult)) (HexRouteResult, error) {
	var out HexRouteResult
	c, err := BeginMatchCommand(ctx, instanceID, userID, commandID, fmt.Sprintf("hex-move-stage1047:%d:%d:%d:%s", userID, destination.Q, destination.R, token))
	if err != nil {
		return out, err
	}
	defer c.Rollback()
	if c.Replay != nil {
		err = json.Unmarshal(c.Replay, &out)
		out.Replayed = true
		return out, err
	}
	if token != "" {
		planned, e := findHexRouteTx(ctx, c.Tx, instanceID, userID, destination)
		if e != nil {
			return out, e
		}
		if planned.PreviewToken != token {
			return out, ErrCommandRejected
		}
	}
	out, err = ApplyHexMoveTx(ctx, c, userID, destination)
	if err != nil {
		return out, err
	}
	out.StateRevision, out.MapRevision, err = c.Commit(ctx, &out, true)
	if err != nil {
		return HexRouteResult{}, err
	}
	if publish != nil {
		publish(out)
	}
	return out, nil
}

// Shared step execution for movement and an atomic move+attack command.
func ApplyHexMoveTx(ctx context.Context, c *MatchCommand, userID int, destination hexgrid.Coord) (HexRouteResult, error) {
	var out HexRouteResult
	if c.ActiveUser != userID {
		return out, ErrCommandRejected
	}
	instanceID := c.InstanceID
	out, err := findHexRouteTx(ctx, c.Tx, instanceID, userID, destination)
	if err != nil {
		return out, err
	}
	if out.AffordableSteps == 0 {
		return out, ErrHexEnergy
	}
	path := append([]hexgrid.Coord(nil), out.Path[:out.AffordableSteps]...)
	out.StepCosts = nil
	out.AuraMPCost = 0
	out.Path = nil
	out.TotalCost = 0
	for _, to := range path {
		before, err := c.AuraMembersTx(ctx)
		if err != nil {
			return out, err
		}
		costs, extra, err := hexRouteCostsTx(ctx, c.Tx, instanceID, userID, out.Position, []hexgrid.Coord{to})
		if err != nil {
			return out, err
		}
		if err = c.SpendHexTx(ctx, userID, 0, costs[0], 0); err != nil {
			return out, err
		}
		inside := auraMembership(before)[userID] != 0
		if _, err = c.Tx.ExecContext(ctx, `UPDATE match_players SET position_q=$3,position_r=$4,hex_first_step_used=true,hex_aura_slow_used=hex_aura_slow_used OR $5 WHERE instance_id=$1 AND user_id=$2`, instanceID, userID, to.Q, to.R, inside); err != nil {
			return out, err
		}
		if _, err = c.Tx.ExecContext(ctx, `UPDATE match_map_cells SET is_player=false WHERE instance_id=$1 AND q=$2 AND r=$3`, instanceID, out.Position.Q, out.Position.R); err != nil {
			return out, err
		}
		if _, err = c.Tx.ExecContext(ctx, `UPDATE match_map_cells SET is_player=true WHERE instance_id=$1 AND q=$2 AND r=$3`, instanceID, to.Q, to.R); err != nil {
			return out, err
		}
		out.Position = to
		out.Path = append(out.Path, to)
		out.TotalCost += costs[0]
		out.StepCosts = append(out.StepCosts, costs[0])
		out.AuraMPCost += extra
		c.RecordAuraMovement(userID, AuraMovementVoluntary)
		if err = c.ResolveAuraTransitionTx(ctx, before); err != nil {
			return out, err
		}
		if err = recordHexExploredTx(ctx, c.Tx, instanceID, userID, to); err != nil {
			return out, err
		}
		var hp int
		if err = c.Tx.QueryRowContext(ctx, `SELECT health FROM match_players WHERE instance_id=$1 AND user_id=$2`, instanceID, userID).Scan(&hp); err != nil {
			return out, err
		}
		if hp <= 0 {
			break
		}
	}
	c.AuraResolved = true
	b, err := ReadHexBudgetsTx(ctx, c.Tx, instanceID, userID)
	if err != nil {
		return out, err
	}
	out.MP = b.MP
	out.AffordableSteps = len(out.Path)
	out.ReachablePosition = out.Position
	out.ReachableCost = out.TotalCost
	out.Partial = out.Position != destination
	return out, nil
}

// Simulates exactly the per-step costs, including first-step artifact and one aura surcharge.
func hexRouteCostsTx(ctx context.Context, tx *sql.Tx, instance string, user int, start hexgrid.Coord, path []hexgrid.Coord) ([]int, int, error) {
	var artifact string
	var first, slow bool
	if err := tx.QueryRowContext(ctx, `SELECT hex_applied_artifact,hex_first_step_used,hex_aura_slow_used FROM match_players WHERE instance_id=$1 AND user_id=$2`, instance, user).Scan(&artifact, &first, &slow); err != nil {
		return nil, 0, err
	}
	c := &MatchCommand{Tx: tx, InstanceID: instance}
	all, err := c.AuraMembersTx(ctx)
	if err != nil {
		return nil, 0, err
	}
	out := []int{}
	extra := 0
	for _, to := range path {
		cost := 1
		if artifact == ArtifactTitanBreastplate {
			cost++
		}
		if artifact == ArtifactBootsOfStealth && !first {
			cost = 0
		}
		for i := range all {
			if all[i].User == user {
				all[i].Position = start
			}
		}
		if !slow && auraMembership(all)[user] != 0 {
			cost += 2
			extra += 2
			slow = true
		}
		first = true
		start = to
		out = append(out, cost)
	}
	return out, extra, nil
}

func findHexRouteTx(ctx context.Context, tx *sql.Tx, instanceID string, userID int, destination hexgrid.Coord) (HexRouteResult, error) {
	var out HexRouteResult
	if !hexgrid.Valid(destination) {
		return out, ErrCommandRejected
	}
	var mode string
	var version int
	var active int
	err := tx.QueryRowContext(ctx, `SELECT geometry->>'kind',schema_version,COALESCE(active_user_id,0),state_revision,map_revision FROM matches WHERE instance_id=$1 AND finished_at IS NULL`, instanceID).Scan(&mode, &version, &active, &out.StateRevision, &out.MapRevision)
	if err != nil {
		return out, err
	}
	if mode != "hex" || version != 2 || active != userID {
		return out, ErrHexUnavailable
	}
	var health, mobility, teamID int
	err = tx.QueryRowContext(ctx, `SELECT position_q,position_r,mp,health,mobility,COALESCE(group_id,0) FROM match_players WHERE instance_id=$1 AND user_id=$2`, instanceID, userID).
		Scan(&out.Position.Q, &out.Position.R, &out.MP, &health, &mobility, &teamID)
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
	out.StepCost = 1
	costs, extra, err := hexRouteCostsTx(ctx, tx, instanceID, userID, out.Position, out.Path)
	if err != nil {
		return out, err
	}
	out.Destination = destination
	out.ReachablePosition = out.Position
	out.StepCosts = costs
	out.TotalCost = 0
	for i, cost := range costs {
		out.TotalCost += cost
		if out.AffordableSteps == i && out.TotalCost <= out.MP {
			out.AffordableSteps++
			out.ReachableCost = out.TotalCost
			out.ReachablePosition = out.Path[i]
		}
	}
	out.AuraMPCost = extra
	out.Affordable = out.MP >= out.TotalCost
	out.Partial = !out.Affordable
	raw, err := json.Marshal(out)
	if err != nil {
		return out, err
	}
	digest := sha256.Sum256(raw)
	out.PreviewToken = hex.EncodeToString(digest[:])
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

// Preview helper for compound commands; the caller owns the same transaction.
func FindHexRouteTx(ctx context.Context, tx *sql.Tx, instance string, user int, destination hexgrid.Coord) (HexRouteResult, error) {
	return findHexRouteTx(ctx, tx, instance, user, destination)
}
