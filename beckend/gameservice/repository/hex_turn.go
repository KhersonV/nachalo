package repository

import (
	"context"
	"encoding/json"
	"fmt"
	"time"
)

type HexTurnResult struct {
	ActiveUser    int       `json:"activeUser"`
	CompletedUser int       `json:"completedUser"`
	TurnNumber    int       `json:"turnNumber"`
	Energy        int       `json:"energy"`
	Deadline      time.Time `json:"deadline"`
	TimedOut      bool      `json:"timedOut,omitempty"`
	StateRevision int64     `json:"stateRevision"`
	MapRevision   int64     `json:"mapRevision"`
	Replayed      bool      `json:"-"`
}

func (r *HexTurnResult) SetCommandRevisions(state, mapRevision int64) {
	r.StateRevision, r.MapRevision = state, mapRevision
}

// ApplyHexEndTurn uses the same match-row lock and command ledger as movement.
// regenForClass is the existing pure square character-template lookup. This
// Buildings remain unsupported; reject injected construction effects until
// they have transactional HEX implementations.
func ApplyHexEndTurn(ctx context.Context, instanceID string, userID int, commandID string, expectedTurn int, regenForClass func(string) int, publish func(HexTurnResult)) (HexTurnResult, error) {
	return applyHexEndTurn(ctx, instanceID, userID, commandID, expectedTurn, false, regenForClass, publish)
}

func ApplyHexExpiredTurn(ctx context.Context, instanceID string, userID int, commandID string, expectedTurn int, regenForClass func(string) int, publish func(HexTurnResult)) (HexTurnResult, error) {
	return applyHexEndTurn(ctx, instanceID, userID, commandID, expectedTurn, true, regenForClass, publish)
}

func applyHexEndTurn(ctx context.Context, instanceID string, userID int, commandID string, expectedTurn int, expired bool, regenForClass func(string) int, publish func(HexTurnResult)) (HexTurnResult, error) {
	var out HexTurnResult
	if expectedTurn < 1 || regenForClass == nil {
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
	var effects int
	if err := c.Tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM match_map_cells WHERE instance_id=$1 AND (structure_type<>'' OR is_under_construction)`, instanceID).Scan(&effects); err != nil {
		return out, err
	}
	if effects != 0 {
		return out, ErrHexUnavailable
	}
	var characterType string
	var energy, maxEnergy, health int
	err = c.Tx.QueryRowContext(ctx, `SELECT character_type,energy,max_energy,health FROM match_players WHERE instance_id=$1 AND user_id=$2`, instanceID, nextUser).Scan(&characterType, &energy, &maxEnergy, &health)
	if err != nil {
		return out, err
	}
	if health <= 0 {
		return out, ErrCommandRejected
	}
	regen := regenForClass(characterType)
	if regen < 0 {
		regen = 0
	}
	energy += regen
	if energy > maxEnergy {
		energy = maxEnergy
	}
	if _, err := c.Tx.ExecContext(ctx, `UPDATE match_players SET energy=$1 WHERE instance_id=$2 AND user_id=$3`, energy, instanceID, nextUser); err != nil {
		return out, err
	}
	if err := c.Tx.QueryRowContext(ctx, `UPDATE matches SET active_user_id=$2,turn_number=$3,hex_turn_deadline=now()+$4 * interval '1 second' WHERE instance_id=$1 RETURNING hex_turn_deadline`, instanceID, nextUser, nextTurn, int(HexTurnDuration().Seconds())).Scan(&out.Deadline); err != nil {
		return out, err
	}
	if _, err := c.Tx.ExecContext(ctx, `UPDATE match_players SET hex_armor_break_turns=GREATEST(0,hex_armor_break_turns-1),hex_armor_break_stacks=CASE WHEN hex_armor_break_turns<=1 THEN 0 ELSE hex_armor_break_stacks END WHERE instance_id=$1 AND hex_armor_break_turns>0`, instanceID); err != nil {
		return out, err
	}
	if _, err := c.Tx.ExecContext(ctx, `UPDATE match_monsters SET hex_armor_break_turns=GREATEST(0,hex_armor_break_turns-1),hex_armor_break_stacks=CASE WHEN hex_armor_break_turns<=1 THEN 0 ELSE hex_armor_break_stacks END WHERE instance_id=$1 AND hex_armor_break_turns>0`, instanceID); err != nil {
		return out, err
	}
	out.ActiveUser, out.CompletedUser, out.TurnNumber, out.Energy = nextUser, userID, nextTurn, energy
	out.TimedOut = expired
	out.StateRevision, out.MapRevision, err = c.Commit(ctx, &out, false)
	if err != nil {
		return HexTurnResult{}, err
	}
	if publish != nil {
		publish(out)
	}
	return out, nil
}
