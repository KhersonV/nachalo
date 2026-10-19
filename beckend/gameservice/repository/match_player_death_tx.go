package repository

import (
	"context"
	"database/sql"
	"encoding/json"

	"gameservice/game"
)

// PlayerDeathTurnResult is the persisted turn state after removing a defeated
// player. It mirrors MatchState.RemovePlayerFromTurnOrder without mutating the
// process-local MatchState before the surrounding command commits.
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
// matching MatchState.RemovePlayerFromTurnOrder.
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

// ClearCombatPlayerCellTx clears the death cell in the same transaction and
// reports whether the map actually changed, for mapRevision.
func (c *MatchCommand) ClearCombatPlayerCellTx(ctx context.Context, pos Position) (bool, error) {
	if c == nil || c.Tx == nil {
		return false, ErrCommandRejected
	}
	var occupied bool
	err := c.Tx.QueryRowContext(ctx, `SELECT is_player FROM match_map_cells WHERE instance_id=$1 AND x=$2 AND y=$3 FOR UPDATE`, c.InstanceID, pos.X, pos.Y).Scan(&occupied)
	if err == sql.ErrNoRows {
		return false, ErrCommandRejected
	}
	if err != nil || !occupied {
		return false, err
	}
	_, err = c.Tx.ExecContext(ctx, `UPDATE match_map_cells SET is_player=false WHERE instance_id=$1 AND x=$2 AND y=$3`, c.InstanceID, pos.X, pos.Y)
	return err == nil, err
}

// CombatMapCellTx reads the death cell through the command transaction. The
// general map loader may backfill through DB, which would escape this lock.
func (c *MatchCommand) CombatMapCellTx(ctx context.Context, pos Position) (*game.FullCell, error) {
	if c == nil || c.Tx == nil {
		return nil, ErrCommandRejected
	}
	row := c.Tx.QueryRowContext(ctx, `SELECT cell_id,x,y,tile_code,resource,barbel,monster,is_portal,is_player,structure_type,structure_owner_user_id,structure_health,structure_defense,structure_attack,structure_energy,is_under_construction,construction_turns_left FROM match_map_cells WHERE instance_id=$1 AND x=$2 AND y=$3`, c.InstanceID, pos.X, pos.Y)
	return scanMatchMapCell(row)
}
