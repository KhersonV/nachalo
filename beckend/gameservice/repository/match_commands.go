package repository

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
)

var ErrCommandConflict = errors.New("commandId already used for different command")
var ErrCommandRejected = errors.New("command no longer valid")

// MatchCommand is the shared transaction boundary for match mutations.
// Every participating command locks the same matches row before reading game
// state, so different Go processes serialize on PostgreSQL.
type MatchCommand struct {
	Tx         *sql.Tx
	InstanceID string
	CommandID  string
	UserID     int
	ActiveUser int
	Hash       string
	Replay     json.RawMessage
	Events     [][]byte
	Finished   bool
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

func BeginMatchCommand(ctx context.Context, instanceID string, userID int, commandID, canonical string) (*MatchCommand, error) {
	if DB == nil || instanceID == "" || commandID == "" || len(commandID) > 128 {
		return nil, ErrCommandRejected
	}
	tx, err := DB.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	c := &MatchCommand{Tx: tx, InstanceID: instanceID, CommandID: commandID, UserID: userID}
	if err := tx.QueryRowContext(ctx, `SELECT COALESCE(active_user_id,0),finished_at IS NOT NULL FROM matches WHERE instance_id=$1 FOR UPDATE`, instanceID).Scan(&c.ActiveUser, &c.Finished); err != nil {
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
	if c.Finished {
		tx.Rollback()
		return nil, ErrCommandRejected
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
	if err := c.persistHexCompletedMatchTx(ctx); err != nil {
		return 0, 0, err
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
