package repository

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"time"
)

type HexTurnResult struct {
	Budget        *HexBudgets      `json:"budget,omitempty"`
	ActionEffects []map[string]any `json:"actionEffects,omitempty"`
	ActiveUser    int              `json:"activeUser"`
	CompletedUser int              `json:"completedUser"`
	TurnNumber    int              `json:"turnNumber"`
	Deadline      time.Time        `json:"deadline"`
	TimedOut      bool             `json:"timedOut,omitempty"`
	StateRevision int64            `json:"stateRevision"`
	MapRevision   int64            `json:"mapRevision"`
	Replayed      bool             `json:"-"`
}

func (r *HexTurnResult) SetCommandRevisions(state, mapRevision int64) {
	r.StateRevision, r.MapRevision = state, mapRevision
}

// ApplyHexEndTurn uses the same match-row lock and command ledger as movement.
// Personal budgets are restored once at the actual next turn start in Commit.
func ApplyHexEndTurn(ctx context.Context, instanceID string, userID int, commandID string, expectedTurn int, publish func(HexTurnResult)) (HexTurnResult, error) {
	return applyHexEndTurn(ctx, instanceID, userID, commandID, expectedTurn, false, publish)
}

func ApplyHexExpiredTurn(ctx context.Context, instanceID string, userID int, commandID string, expectedTurn int, publish func(HexTurnResult)) (HexTurnResult, error) {
	return applyHexEndTurn(ctx, instanceID, userID, commandID, expectedTurn, true, publish)
}

func applyHexEndTurn(ctx context.Context, instanceID string, userID int, commandID string, expectedTurn int, expired bool, publish func(HexTurnResult)) (HexTurnResult, error) {
	var out HexTurnResult
	if expectedTurn < 1 {
		return out, ErrCommandRejected
	}
	ctx, cancel := context.WithTimeout(ctx, 8*time.Second)
	defer cancel()
	c, err := BeginMatchCommand(ctx, instanceID, userID, commandID, fmt.Sprintf("hex-end-turn-v1:%d:%d", userID, expectedTurn))
	if err != nil {
		return out, err
	}
	defer c.Rollback()
	if c.Replay != nil {
		if err := json.Unmarshal(c.Replay, &out); err != nil {
			return out, err
		}
		out.Replayed = true
		return out, nil
	}
	if c.ActiveUser != userID {
		return out, ErrCommandRejected
	}
	var mode string
	var version, turn, total int
	var orderRaw []byte
	var currentDeadline time.Time
	err = c.Tx.QueryRowContext(ctx, `SELECT geometry->>'kind',schema_version,turn_number,total_players,turn_order,hex_turn_deadline FROM matches WHERE instance_id=$1`, instanceID).Scan(&mode, &version, &turn, &total, &orderRaw, &currentDeadline)
	if err != nil {
		return out, err
	}
	var order []int
	if err := json.Unmarshal(orderRaw, &order); err != nil {
		return out, err
	}
	if mode != "hex" || version != 2 || turn != expectedTurn || total < 1 || len(order) == 0 {
		return out, ErrCommandRejected
	}
	if expired && time.Now().Before(currentDeadline) {
		return out, ErrCommandRejected
	}
	index := -1
	for i, id := range order {
		if id == userID {
			index = i
			break
		}
	}
	if index < 0 {
		return out, ErrCommandRejected
	}
	nextIndex := (index + 1) % len(order)
	nextUser := order[nextIndex]
	nextTurn := turn
	if nextIndex == 0 {
		nextTurn++
	}
	constructionChanged, err := progressHexConstructionTx(ctx, c.Tx, instanceID)
	if err != nil {
		return out, err
	}
	if err := c.Tx.QueryRowContext(ctx, `UPDATE matches SET active_user_id=$2,turn_number=$3,hex_turn_deadline=now()+$4 * interval '1 second' WHERE instance_id=$1 RETURNING hex_turn_deadline`, instanceID, nextUser, nextTurn, int(HexTurnDuration().Seconds())).Scan(&out.Deadline); err != nil {
		return out, err
	}
	turretsChanged, err := progressHexTurretsTx(ctx, c)
	if err != nil {
		return out, err
	}
	var active int
	var deadline sql.NullTime
	if err := c.Tx.QueryRowContext(ctx, `SELECT COALESCE(active_user_id,0),turn_number,hex_turn_deadline FROM matches WHERE instance_id=$1`, instanceID).Scan(&active, &nextTurn, &deadline); err != nil {
		return out, err
	}
	if deadline.Valid {
		out.Deadline = deadline.Time
	}
	out.ActiveUser, out.CompletedUser, out.TurnNumber = active, userID, nextTurn
	c.NextPersonalTurn = true
	out.TimedOut = expired
	out.StateRevision, out.MapRevision, err = c.Commit(ctx, &out, constructionChanged || turretsChanged)
	if err != nil {
		return HexTurnResult{}, err
	}
	if publish != nil {
		publish(out)
	}
	return out, nil
}
