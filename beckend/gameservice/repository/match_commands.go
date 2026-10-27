package repository

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"gameservice/internal/game/balance"
	"gameservice/internal/hexgrid"
)

var ErrCommandConflict = errors.New("commandId already used for different command")
var ErrCommandRejected = errors.New("command no longer valid")

// MatchCommand is the shared transaction boundary for match mutations.
// Every participating command locks the same matches row before reading game
// state, so different Go processes serialize on PostgreSQL.
type MatchCommand struct {
	Tx               *sql.Tx
	InstanceID       string
	CommandID        string
	UserID           int
	ActiveUser       int
	Hash             string
	Replay           json.RawMessage
	Events           [][]byte
	CombatTargetKind string
	CombatTargetID   int
	MapChanged       bool
	NextPersonalTurn bool
	AuraBefore       []hexAuraMember
	AuraResolved     bool
	auraMovements    map[int]AuraMovementCause
	ActionEffects    []map[string]any
	Finished         bool
	Ruleset          balance.CombatRuleset
	CombatConfig     *balance.V34ResearchConfig
	RandomSeed       int64
	RandomCounter    int64
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
	var combatConfig []byte
	if err := tx.QueryRowContext(ctx, `SELECT COALESCE(active_user_id,0),finished_at IS NOT NULL,combat_ruleset,combat_config,combat_rng_seed,combat_rng_counter FROM matches WHERE instance_id=$1 FOR UPDATE`, instanceID).Scan(&c.ActiveUser, &c.Finished, &c.Ruleset,&combatConfig,&c.RandomSeed,&c.RandomCounter); err != nil {
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
	if err := balance.RequirePlayableRuleset(c.Ruleset); err != nil {
		tx.Rollback()
		return nil, err
	}
	var configErr error
	c.CombatConfig,configErr = DecodeMatchCombatConfig(c.Ruleset,combatConfig)
	if configErr != nil { tx.Rollback(); return nil,configErr }
	if err := c.SyncHexArtifactsTx(ctx); err != nil {
		tx.Rollback()
		return nil, err
	}
	c.AuraBefore, err = c.AuraMembersTx(ctx)
	if err != nil {
		tx.Rollback()
		return nil, err
	}
	for _, p := range c.AuraBefore {
		if err := recordHexExploredTx(ctx, c.Tx, c.InstanceID, p.User, p.Position); err != nil {
			tx.Rollback()
			return nil, err
		}
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
	if !c.AuraResolved {
		if err := c.ResolveAuraTransitionTx(ctx, c.AuraBefore); err != nil {
			return 0, 0, err
		}
	}
	if err := c.SyncHexArtifactsTx(ctx); err != nil {
		return 0, 0, err
	}
	if err := c.ResolveHexTurnStartTx(ctx); err != nil {
		return 0, 0, err
	}
	if c.Ruleset==balance.ExperimentalV34 {
		if _,err:=c.Tx.ExecContext(ctx,`UPDATE matches SET combat_rng_counter=$2 WHERE instance_id=$1`,c.InstanceID,c.RandomCounter); err!=nil {return 0,0,err}
	}
	alive, err := c.AuraMembersTx(ctx)
	if err != nil {
		return 0, 0, err
	}
	for _, p := range alive {
		if err := recordHexExploredTx(ctx, c.Tx, c.InstanceID, p.User, p.Position); err != nil {
			return 0, 0, err
		}
	}
	if err := c.persistHexCompletedMatchTx(ctx); err != nil {
		return 0, 0, err
	}
	mapDelta := 0
	if mapChanged || c.MapChanged {
		mapDelta = 1
	}
	var stateRevision, mapRevision int64
	err = c.Tx.QueryRowContext(ctx, `UPDATE matches SET state_revision=state_revision+1,map_revision=map_revision+$2 WHERE instance_id=$1 RETURNING state_revision,map_revision`, c.InstanceID, mapDelta).Scan(&stateRevision, &mapRevision)
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
	var payload map[string]any
	if err = json.Unmarshal(encoded, &payload); err != nil {
		return 0, 0, err
	}
	if c.UserID > 0 {
		b, readErr := ReadHexBudgetsTx(ctx, c.Tx, c.InstanceID, c.UserID)
		if readErr != nil {
			return 0, 0, readErr
		}
		payload["budget"] = b
		var hp int
		if err = c.Tx.QueryRowContext(ctx, `SELECT health FROM match_players WHERE instance_id=$1 AND user_id=$2`, c.InstanceID, c.UserID).Scan(&hp); err != nil {
			return 0, 0, err
		}
		switch response.(type) {
		case *HexItemResult, *HexBarrelResult:
			payload["health"] = hp
		}
		if _, ok := payload["newAttackerHP"]; ok {
			payload["newAttackerHP"] = hp
		}
		raw, _ := json.Marshal(b)
		var values map[string]any
		if err = json.Unmarshal(raw, &values); err != nil {
			return 0, 0, err
		}
		for key, value := range values {
			payload[key] = value
		}
	}
	if c.CombatTargetKind == "player" && c.CombatTargetID > 0 {
		var hp int
		if err = c.Tx.QueryRowContext(ctx, `SELECT health FROM match_players WHERE instance_id=$1 AND user_id=$2`, c.InstanceID, c.CombatTargetID).Scan(&hp); err != nil {
			return 0, 0, err
		}
		payload["newTargetHP"], payload["playerDied"] = hp, hp <= 0
	}
	payload["_visuals"] = c.ActionEffects // durable event payload, never a snapshot field
	safeEffects := []map[string]any{}
	if c.UserID > 0 && len(c.ActionEffects) > 0 {
		visible, err := HexVisibleCellsTx(ctx, c.Tx, c.InstanceID, c.UserID)
		if err != nil {
			return 0, 0, err
		}
		for _, effect := range c.ActionEffects {
			pos, ok := effect["position"].(hexgrid.Coord)
			if effect["userId"] == c.UserID || ok && visible[pos] {
				safeEffects = append(safeEffects, effect)
			}
		}
	}
	payload["actionEffects"] = safeEffects
	if _, ok := payload["activeUser"]; ok {
		var active int
		if err = c.Tx.QueryRowContext(ctx, `SELECT COALESCE(active_user_id,0) FROM matches WHERE instance_id=$1`, c.InstanceID).Scan(&active); err != nil {
			return 0, 0, err
		}
		payload["activeUser"] = active
	}
	encoded, err = json.Marshal(payload)
	if err != nil {
		return 0, 0, err
	}
	if err = json.Unmarshal(encoded, response); err != nil {
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
