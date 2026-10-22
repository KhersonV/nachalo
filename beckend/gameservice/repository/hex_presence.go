package repository

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	"gameservice/internal/hexgrid"
)

const HexReconnectGrace = 3 * time.Minute

// After a process restart, old sockets no longer exist. Give their players
// Square's full reconnect window unless they already have an older deadline.
func MarkHexPresenceUnknownOnStart(ctx context.Context) error {
	if DB == nil {
		return ErrCommandRejected
	}
	_, err := DB.ExecContext(ctx, `UPDATE match_players p SET hex_disconnected_at=now() FROM matches m WHERE m.instance_id=p.instance_id AND m.geometry->>'kind'='hex' AND m.finished_at IS NULL AND p.health>0 AND p.hex_disconnected_at IS NULL AND m.turn_order @> to_jsonb(p.user_id)`)
	return err
}

// SetHexPresence only changes living, active HEX participants. A reconnect
// clears the durable deadline; a disconnect starts it only once.
func SetHexPresence(ctx context.Context, instanceID string, userID int, connected bool) (bool, error) {
	if DB == nil || instanceID == "" || userID <= 0 {
		return false, ErrCommandRejected
	}
	tx, err := DB.BeginTx(ctx, nil)
	if err != nil {
		return false, err
	}
	defer tx.Rollback()
	var active bool
	err = tx.QueryRowContext(ctx, `SELECT geometry->>'kind'='hex' AND finished_at IS NULL AND turn_order @> to_jsonb($2::integer) FROM matches WHERE instance_id=$1 FOR UPDATE`, instanceID, userID).Scan(&active)
	if err == sql.ErrNoRows || !active {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	var result sql.Result
	if connected {
		result, err = tx.ExecContext(ctx, `UPDATE match_players SET hex_disconnected_at=NULL WHERE instance_id=$1 AND user_id=$2 AND health>0 AND hex_disconnected_at IS NOT NULL`, instanceID, userID)
	} else {
		result, err = tx.ExecContext(ctx, `UPDATE match_players SET hex_disconnected_at=now() WHERE instance_id=$1 AND user_id=$2 AND health>0 AND hex_disconnected_at IS NULL`, instanceID, userID)
	}
	if err != nil {
		return false, err
	}
	n, err := result.RowsAffected()
	if err != nil {
		return false, err
	}
	if err := tx.Commit(); err != nil {
		return false, err
	}
	return n == 1, nil
}

type HexLightningResult struct {
	UserID        int   `json:"userId"`
	StateRevision int64 `json:"stateRevision"`
	MapRevision   int64 `json:"mapRevision"`
	Replayed      bool  `json:"-"`
}

func (r *HexLightningResult) SetCommandRevisions(state, mapRevision int64) {
	r.StateRevision, r.MapRevision = state, mapRevision
}

// ApplyHexLightning serializes with attacks, exits and turn changes on the
// match row. The disconnect timestamp is rechecked after that lock is held.
func ApplyHexLightning(ctx context.Context, instanceID string, userID int, publish func(HexLightningResult)) (HexLightningResult, error) {
	var out HexLightningResult
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	c, err := BeginMatchCommand(ctx, instanceID, userID, fmt.Sprintf("hex-lightning:%d", userID), fmt.Sprintf("hex-lightning-v1:%d", userID))
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
	var pos hexgrid.Coord
	err = c.Tx.QueryRowContext(ctx, `SELECT p.position_q,p.position_r FROM match_players p JOIN matches m ON m.instance_id=p.instance_id WHERE p.instance_id=$1 AND p.user_id=$2 AND m.geometry->>'kind'='hex' AND p.health>0 AND p.hex_disconnected_at<=now()-$3 * interval '1 second' AND m.turn_order @> to_jsonb($2::integer)`, instanceID, userID, int(HexReconnectGrace.Seconds())).Scan(&pos.Q, &pos.R)
	if err == sql.ErrNoRows {
		return out, ErrCommandRejected
	}
	if err != nil {
		return out, err
	}
	if err := c.MarkCombatPlayerDeadTx(ctx, userID); err != nil {
		return out, err
	}
	if err := c.LoseHexBackpackTx(ctx, userID, pos); err != nil {
		return out, err
	}
	if _, err := c.Tx.ExecContext(ctx, `UPDATE hex_player_results SET result=jsonb_set(result,'{reason}','"lightning"'::jsonb) WHERE instance_id=$1 AND user_id=$2`, instanceID, userID); err != nil {
		return out, err
	}
	turn, err := c.RemoveCombatPlayerFromTurnOrderTx(ctx, userID)
	if err != nil {
		return out, err
	}
	if c.ActiveUser == userID {
		if err := c.ResetHexTurnDeadlineTx(ctx, turn.ActiveUserID); err != nil {
			return out, err
		}
	}
	result, err := c.Tx.ExecContext(ctx, `UPDATE match_map_cells SET is_player=false WHERE instance_id=$1 AND q=$2 AND r=$3 AND is_player=true`, instanceID, pos.Q, pos.R)
	if err != nil {
		return out, err
	}
	if n, _ := result.RowsAffected(); n != 1 {
		return out, ErrCommandRejected
	}
	if turn.ActiveUserID == 0 {
		if _, err := c.Tx.ExecContext(ctx, `UPDATE matches SET finished_at=now(),hex_turn_deadline=NULL WHERE instance_id=$1`, instanceID); err != nil {
			return out, err
		}
	}
	out.UserID = userID
	out.StateRevision, out.MapRevision, err = c.Commit(ctx, &out, true)
	if err != nil {
		return HexLightningResult{}, err
	}
	if publish != nil {
		publish(out)
	}
	return out, nil
}
