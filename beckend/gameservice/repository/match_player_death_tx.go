package repository

import (
	"context"
	"encoding/json"
)

// PlayerDeathTurnResult is the persisted turn state after removing a defeated
// player. The transaction keeps turn order and placement consistent.
type PlayerDeathTurnResult struct {
	TurnOrder    []int
	ActiveUserID int
	TurnNumber   int
}

// MarkCombatPlayerDeadTx is a transaction-only building block. Callers must
// record a player kill before calling it, if one occurred, then save all other
// death effects and commit through the same MatchCommand. Recording the defeat
// here keeps placement history in the same transaction as the death itself.
func (c *MatchCommand) MarkCombatPlayerDeadTx(ctx context.Context, userID int) error {
	if c == nil || c.Tx == nil {
		return ErrCommandRejected
	}
	result, err := c.Tx.ExecContext(ctx, `UPDATE match_players SET health=0 WHERE instance_id=$1 AND user_id=$2 AND health>0`, c.InstanceID, userID)
	if err != nil {
		return err
	}
	n, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if n != 1 {
		return ErrCommandRejected
	}
	_, err = c.RecordDefeatedUserTx(ctx, userID)
	return err
}

// RemoveCombatPlayerFromTurnOrderTx uses the locked matches row as the source
// of truth. The turn number does not advance when the active player dies,
// matching the HEX turn order contract.
func (c *MatchCommand) RemoveCombatPlayerFromTurnOrderTx(ctx context.Context, userID int) (PlayerDeathTurnResult, error) {
	var out PlayerDeathTurnResult
	if c == nil || c.Tx == nil {
		return out, ErrCommandRejected
	}
	var raw []byte
	if err := c.Tx.QueryRowContext(ctx, `SELECT turn_order,COALESCE(active_user_id,0),COALESCE(turn_number,1) FROM matches WHERE instance_id=$1`, c.InstanceID).Scan(&raw, &out.ActiveUserID, &out.TurnNumber); err != nil {
		return out, err
	}
	var oldOrder []int
	if err := json.Unmarshal(raw, &oldOrder); err != nil {
		return out, err
	}
	out.TurnOrder = make([]int, 0, len(oldOrder))
	removedIndex := -1
	for _, id := range oldOrder {
		if id == userID {
			if removedIndex < 0 {
				removedIndex = len(out.TurnOrder)
			}
			continue
		}
		out.TurnOrder = append(out.TurnOrder, id)
	}
	if removedIndex < 0 {
		return out, ErrCommandRejected
	}
	if len(out.TurnOrder) == 0 {
		out.ActiveUserID = 0
	} else if out.ActiveUserID == userID {
		out.ActiveUserID = out.TurnOrder[removedIndex%len(out.TurnOrder)]
	}
	encoded, err := json.Marshal(out.TurnOrder)
	if err != nil {
		return out, err
	}
	_, err = c.Tx.ExecContext(ctx, `UPDATE matches SET turn_order=$2::jsonb,active_user_id=$3 WHERE instance_id=$1`, c.InstanceID, encoded, out.ActiveUserID)
	return out, err
}
