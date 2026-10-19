package repository

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"gameservice/game"
	"time"
)

var ErrCommandConflict = errors.New("commandId already used for different command")
var ErrCommandRejected = errors.New("command no longer valid")
var ErrLegacyAction = errors.New("target requires legacy combat path")

type SquareMoveResult struct {
	Energy        int   `json:"energy"`
	X             int   `json:"x"`
	Y             int   `json:"y"`
	StateRevision int64 `json:"stateRevision"`
	MapRevision   int64 `json:"mapRevision"`
	Replayed      bool  `json:"-"`
}

func (r *SquareMoveResult) SetCommandRevisions(state, mapRevision int64) {
	r.StateRevision, r.MapRevision = state, mapRevision
}

// MatchCommand is the shared transaction boundary for match mutations.
// Every participating command locks the same matches row before reading game
// state, so different Go processes serialize on PostgreSQL.
type MatchCommand struct {
	Tx         *sql.Tx
	InstanceID string
	CommandID  string
	UserID     int
	ActiveUser int
	Width      int
	Height     int
	Hash       string
	Replay     json.RawMessage
	Events     [][]byte
}

// QueueEvent keeps protocol messages private until the command commits.
func (c *MatchCommand) QueueEvent(event interface{}) error {
	if c == nil || c.Tx == nil {
		return ErrCommandRejected
	}
	data, err := json.Marshal(event)
	if err != nil {
		return err
	}
	c.Events = append(c.Events, data)
	return nil
}

// CommitAndPublish never publishes events if any SQL write or the commit fails.
func (c *MatchCommand) CommitAndPublish(ctx context.Context, response interface{}, mapChanged bool, publish func([]byte)) (int64, int64, error) {
	state, mapRevision, err := c.Commit(ctx, response, mapChanged)
	if err != nil {
		return 0, 0, err
	}
	if publish != nil {
		for _, event := range c.Events {
			publish(event)
		}
	}
	c.Events = nil
	return state, mapRevision, nil
}

// ReplayMatchCommand returns a committed result only when the command payload
// matches. Callers still use BeginMatchCommand for new mutations.
func ReplayMatchCommand(ctx context.Context, instanceID, commandID, canonical string) (json.RawMessage, bool, error) {
	if DB == nil || instanceID == "" || commandID == "" {
		return nil, false, ErrCommandRejected
	}
	var hash string
	var response json.RawMessage
	err := DB.QueryRowContext(ctx, `SELECT payload_hash,response FROM match_commands WHERE instance_id=$1 AND command_id=$2`, instanceID, commandID).Scan(&hash, &response)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, false, nil
	}
	if err != nil {
		return nil, false, err
	}
	sum := sha256.Sum256([]byte(canonical))
	if hash != hex.EncodeToString(sum[:]) {
		return nil, false, nil
	}
	return response, true, nil
}

func BeginMatchCommand(ctx context.Context, instanceID string, userID int, commandID, canonical string) (*MatchCommand, error) {
	if DB == nil || instanceID == "" || commandID == "" || len(commandID) > 128 {
		return nil, ErrCommandRejected
	}
	tx, err := DB.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	c := &MatchCommand{Tx: tx, InstanceID: instanceID, CommandID: commandID, UserID: userID}
	if err := tx.QueryRowContext(ctx, `SELECT active_user_id,map_width,map_height FROM matches WHERE instance_id=$1 FOR UPDATE`, instanceID).Scan(&c.ActiveUser, &c.Width, &c.Height); err != nil {
		tx.Rollback()
		return nil, err
	}
	sum := sha256.Sum256([]byte(canonical))
	c.Hash = hex.EncodeToString(sum[:])
	var storedHash string
	err = tx.QueryRowContext(ctx, `SELECT payload_hash,response FROM match_commands WHERE instance_id=$1 AND command_id=$2`, instanceID, commandID).Scan(&storedHash, &c.Replay)
	if err == nil {
		tx.Rollback()
		c.Tx = nil
		if storedHash != c.Hash {
			return nil, ErrCommandConflict
		}
		return c, nil
	}
	if err != sql.ErrNoRows {
		tx.Rollback()
		return nil, err
	}
	return c, nil
}

func (c *MatchCommand) Rollback() {
	if c != nil && c.Tx != nil {
		_ = c.Tx.Rollback()
	}
}

func (c *MatchCommand) Commit(ctx context.Context, response interface{}, mapChanged bool) (int64, int64, error) {
	if c == nil || c.Tx == nil {
		return 0, 0, ErrCommandRejected
	}
	mapDelta := 0
	if mapChanged {
		mapDelta = 1
	}
	var stateRevision, mapRevision int64
	err := c.Tx.QueryRowContext(ctx, `UPDATE matches SET state_revision=state_revision+1,map_revision=map_revision+$2 WHERE instance_id=$1 RETURNING state_revision,map_revision`, c.InstanceID, mapDelta).Scan(&stateRevision, &mapRevision)
	if err != nil {
		return 0, 0, err
	}
	if target, ok := response.(interface{ SetCommandRevisions(int64, int64) }); ok {
		target.SetCommandRevisions(stateRevision, mapRevision)
	}
	encoded, err := json.Marshal(response)
	if err != nil {
		return 0, 0, err
	}
	_, err = c.Tx.ExecContext(ctx, `INSERT INTO match_commands(instance_id,command_id,user_id,payload_hash,response,state_revision,map_revision) VALUES($1,$2,$3,$4,$5,$6,$7)`, c.InstanceID, c.CommandID, c.UserID, c.Hash, encoded, stateRevision, mapRevision)
	if err != nil {
		return 0, 0, err
	}
	if err = c.Tx.Commit(); err != nil {
		return 0, 0, err
	}
	c.Tx = nil
	return stateRevision, mapRevision, nil
}

// ApplySquareMove serializes new square moves across Go processes by locking
// the match row. The lock and ledger entry live in the same transaction as all
// three writes that make up a simple move.
func ApplySquareMove(ctx context.Context, instanceID string, userID int, commandID string, x, y, cost int) (SquareMoveResult, error) {
	var out SquareMoveResult
	if DB == nil || commandID == "" || len(commandID) > 128 || cost < 1 {
		return out, ErrCommandRejected
	}
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	canonical := fmt.Sprintf("square-move-v1:%d:%d:%d", userID, x, y)
	c, err := BeginMatchCommand(ctx, instanceID, userID, commandID, canonical)
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
	tx := c.Tx
	if c.ActiveUser != userID || x < 0 || y < 0 || x >= c.Width || y >= c.Height {
		return out, ErrCommandRejected
	}

	var position []byte
	var energy int
	err = tx.QueryRowContext(ctx, `SELECT position, energy FROM match_players WHERE instance_id=$1 AND user_id=$2 AND health>0 FOR UPDATE`, instanceID, userID).Scan(&position, &energy)
	if err != nil {
		return out, err
	}
	var pos struct {
		X int `json:"x"`
		Y int `json:"y"`
	}
	if err = json.Unmarshal(position, &pos); err != nil {
		return out, err
	}
	distance := absMove(pos.X-x) + absMove(pos.Y-y)
	if distance != 1 || energy < cost {
		return out, ErrCommandRejected
	}

	var tile int
	var occupied, blocked bool
	err = tx.QueryRowContext(ctx, `SELECT tile_code, is_player, (structure_type <> '' OR is_under_construction OR (monster IS NOT NULL AND monster <> 'null'::jsonb)) FROM match_map_cells WHERE instance_id=$1 AND x=$2 AND y=$3`, instanceID, x, y).Scan(&tile, &occupied, &blocked)
	if err != nil {
		return out, err
	}
	if occupied || blocked {
		return out, ErrLegacyAction
	}
	var monsterCount, playerCount int
	if err = tx.QueryRowContext(ctx, `SELECT count(*) FROM match_monsters WHERE instance_id=$1 AND pos_x=$2 AND pos_y=$3 AND health>0`, instanceID, x, y).Scan(&monsterCount); err != nil {
		return out, err
	}
	if err = tx.QueryRowContext(ctx, `SELECT count(*) FROM match_players WHERE instance_id=$1 AND user_id<>$2 AND position->>'x'=$3 AND position->>'y'=$4 AND health>0`, instanceID, userID, fmt.Sprint(x), fmt.Sprint(y)).Scan(&playerCount); err != nil {
		return out, err
	}
	if monsterCount > 0 || playerCount > 0 {
		return out, ErrLegacyAction
	}
	if tile != 48 && tile != 80 && tile != 82 && tile != 112 && tile != 66 {
		return out, ErrCommandRejected
	}

	newPosition, _ := json.Marshal(struct {
		X int `json:"x"`
		Y int `json:"y"`
	}{x, y})
	if _, err = tx.ExecContext(ctx, `UPDATE match_players SET position=$1, energy=energy-$2 WHERE instance_id=$3 AND user_id=$4`, newPosition, cost, instanceID, userID); err != nil {
		return out, err
	}
	if _, err = tx.ExecContext(ctx, `UPDATE match_map_cells SET is_player=false WHERE instance_id=$1 AND x=$2 AND y=$3`, instanceID, pos.X, pos.Y); err != nil {
		return out, err
	}
	if _, err = tx.ExecContext(ctx, `UPDATE match_map_cells SET is_player=true WHERE instance_id=$1 AND x=$2 AND y=$3`, instanceID, x, y); err != nil {
		return out, err
	}
	out.Energy, out.X, out.Y = energy-cost, x, y
	// The persisted response is the operation result. Its revisions are stored
	// in dedicated ledger columns and copied into the returned value below.
	stateRevision, mapRevision, err := c.Commit(ctx, &out, true)
	if err != nil {
		return SquareMoveResult{}, err
	}
	out.StateRevision, out.MapRevision = stateRevision, mapRevision
	return out, nil
}

func absMove(x int) int {
	if x < 0 {
		return -x
	}
	return x
}

type ResourceCollectionResult struct {
	Cell          game.FullCell `json:"cell"`
	Inventory     string        `json:"inventory"`
	ItemType      string        `json:"itemType"`
	ItemID        int           `json:"itemId"`
	StateRevision int64         `json:"stateRevision"`
	MapRevision   int64         `json:"mapRevision"`
	Replayed      bool          `json:"-"`
}

func (r *ResourceCollectionResult) SetCommandRevisions(state, mapRevision int64) {
	r.StateRevision, r.MapRevision = state, mapRevision
}

func ApplyResourceCollection(ctx context.Context, instanceID string, userID int, commandID string, x, y int) (ResourceCollectionResult, error) {
	var out ResourceCollectionResult
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	canonical := fmt.Sprintf("collect-resource-v1:%d:%d:%d", userID, x, y)
	c, err := BeginMatchCommand(ctx, instanceID, userID, commandID, canonical)
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
	if c.ActiveUser != userID || x < 0 || y < 0 || x >= c.Width || y >= c.Height {
		return out, ErrCommandRejected
	}
	var health int
	if err := c.Tx.QueryRowContext(ctx, `SELECT health FROM match_players WHERE instance_id=$1 AND user_id=$2 FOR UPDATE`, instanceID, userID).Scan(&health); err != nil {
		return out, err
	}
	if health <= 0 {
		return out, ErrCommandRejected
	}
	cell, err := LoadMapCellForUpdateTx(c.Tx, instanceID, x, y)
	if err != nil {
		return out, err
	}
	if cell == nil || cell.Resource == nil {
		return out, ErrCommandRejected
	}
	item := cell.Resource
	out.ItemID, out.ItemType = item.ID, "resource"
	if item.ItemType == "artifact" {
		out.ItemType = "artifact"
	}
	cell.Resource = nil
	cell.TileCode = 48
	cell.Monster = nil
	cell.IsPortal = false
	if err := SaveMapCellTx(c.Tx, instanceID, *cell); err != nil {
		return out, err
	}
	out.Inventory, err = AddInventoryItemTx(c.Tx, instanceID, userID, out.ItemType, item.ID, item.Type, item.Image, item.Description, 1)
	if err != nil {
		return out, err
	}
	out.Cell = *cell
	out.StateRevision, out.MapRevision, err = c.Commit(ctx, &out, true)
	return out, err
}
