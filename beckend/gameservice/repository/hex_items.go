package repository

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"gameservice/game"
	"gameservice/internal/hexgrid"
)

type HexBarrelResult struct {
	Position      hexgrid.Coord   `json:"position"`
	Outcome       string          `json:"outcome"`
	Damage        int             `json:"damage,omitempty"`
	Health        int             `json:"health"`
	ItemID        int             `json:"itemId,omitempty"`
	ItemName      string          `json:"itemName,omitempty"`
	ItemType      string          `json:"itemType,omitempty"`
	QuestFound    bool            `json:"questFound,omitempty"`
	PlayerName    string          `json:"playerName,omitempty"`
	Inventory     json.RawMessage `json:"inventory,omitempty"`
	StateRevision int64           `json:"stateRevision"`
	MapRevision   int64           `json:"mapRevision"`
	Replayed      bool            `json:"-"`
}

func (r *HexBarrelResult) SetCommandRevisions(state, mapRevision int64) {
	r.StateRevision, r.MapRevision = state, mapRevision
}

// The Square outcome function owns the probabilities and random damage range.
var hexBarrelRoll = game.OpenBarbel

func ApplyHexBarrel(ctx context.Context, instanceID string, userID int, commandID string, pos hexgrid.Coord, publish func(HexBarrelResult)) (HexBarrelResult, error) {
	var out HexBarrelResult
	if !hexgrid.Valid(pos) {
		return out, ErrCommandRejected
	}
	ctx, cancel := context.WithTimeout(ctx, 8*time.Second)
	defer cancel()
	c, err := BeginMatchCommand(ctx, instanceID, userID, commandID, fmt.Sprintf("hex-barrel-v1:%d:%d:%d", userID, pos.Q, pos.R))
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
	var version, health int
	var playerPos hexgrid.Coord
	if err := c.Tx.QueryRowContext(ctx, `SELECT m.geometry->>'kind',m.schema_version,p.health,p.position_q,p.position_r,COALESCE(p.name,'') FROM matches m JOIN match_players p ON p.instance_id=m.instance_id AND p.user_id=$2 WHERE m.instance_id=$1`, instanceID, userID).Scan(&mode, &version, &health, &playerPos.Q, &playerPos.R, &out.PlayerName); err != nil {
		return out, err
	}
	if mode != "hex" || version != 2 || health <= 0 {
		return out, ErrCommandRejected
	}
	if err := requireHexInteractionPosition(playerPos, pos); err != nil {
		return out, err
	}
	var raw []byte
	if err := c.Tx.QueryRowContext(ctx, `SELECT barbel FROM match_map_cells WHERE instance_id=$1 AND q=$2 AND r=$3 FOR UPDATE`, instanceID, pos.Q, pos.R).Scan(&raw); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return out, ErrCommandRejected
		}
		return out, err
	}
	if len(raw) == 0 {
		return out, ErrCommandRejected
	}
	var barrel game.ResourceData
	if err := json.Unmarshal(raw, &barrel); err != nil {
		return out, err
	}
	resources, err := getHexResourcesTx(ctx, c.Tx)
	if err != nil {
		return out, err
	}
	artifacts, err := getHexArtifactsTx(ctx, c.Tx)
	if err != nil {
		return out, err
	}
	var questID, remainingBarrels int
	var questSpawned bool
	if err := c.Tx.QueryRowContext(ctx, `SELECT COALESCE(quest_artifact_id,0),hex_quest_spawned FROM matches WHERE instance_id=$1`, instanceID).Scan(&questID, &questSpawned); err != nil {
		return out, err
	}
	if err := c.Tx.QueryRowContext(ctx, `SELECT count(*) FROM match_map_cells WHERE instance_id=$1 AND barbel IS NOT NULL`, instanceID).Scan(&remainingBarrels); err != nil {
		return out, err
	}
	outcome, err := rollHexQuestBarrel(barrel, resources, artifacts, questID, questSpawned, remainingBarrels)
	if err != nil {
		return out, err
	}
	out.Position = pos
	switch value := outcome.(type) {
	case game.DamageEvent:
		out.Outcome = "damage"
		out.Damage = value.Amount
		out.Health = health - value.Amount
		if out.Health < 0 {
			out.Health = 0
		}
		if err := c.RecordDamageEventTx(ctx, game.DamageEvent{SourceType: game.ResultSourceEnvironment, TargetType: "player", TargetID: userID, Amount: value.Amount}); err != nil {
			return out, err
		}
		if out.Health == 0 {
			if _, err := c.RecordKillEventTx(ctx, game.KillEvent{SourceType: game.ResultSourceEnvironment, VictimType: "player", VictimID: userID, Damage: value.Amount}); err != nil {
				return out, err
			}
			if err := c.MarkCombatPlayerDeadTx(ctx, userID); err != nil {
				return out, err
			}
			if err := c.LoseHexBackpackTx(ctx, userID, pos); err != nil {
				return out, err
			}
			turn, err := c.RemoveCombatPlayerFromTurnOrderTx(ctx, userID)
			if err != nil {
				return out, err
			}
			if err := c.ResetHexTurnDeadlineTx(ctx, turn.ActiveUserID); err != nil {
				return out, err
			}
			if _, err := c.Tx.ExecContext(ctx, `UPDATE match_map_cells SET is_player=false WHERE instance_id=$1 AND q=$2 AND r=$3`, instanceID, pos.Q, pos.R); err != nil {
				return out, err
			}
			if turn.ActiveUserID == 0 {
				if _, err := c.Tx.ExecContext(ctx, `UPDATE matches SET finished_at=now(),hex_turn_deadline=NULL WHERE instance_id=$1`, instanceID); err != nil {
					return out, err
				}
			}
		} else if _, err := c.Tx.ExecContext(ctx, `UPDATE match_players SET health=$3 WHERE instance_id=$1 AND user_id=$2`, instanceID, userID, out.Health); err != nil {
			return out, err
		}
	case game.ResourceData:
		out.Outcome = "resource"
		out.ItemType = "resource"
		for _, a := range artifacts {
			if a.ID == value.ID && a.Type == value.Type {
				out.Outcome = "artifact"
				out.ItemType = "artifact"
				break
			}
		}
		out.ItemID, out.ItemName, out.Health = value.ID, value.Type, health
		if out.ItemType == "artifact" && value.ID == questID && !questSpawned {
			claimed, err := c.Tx.ExecContext(ctx, `UPDATE matches SET hex_quest_spawned=true WHERE instance_id=$1 AND hex_quest_spawned=false`, instanceID)
			if err != nil {
				return out, err
			}
			if n, _ := claimed.RowsAffected(); n != 1 {
				return out, ErrCommandRejected
			}
			out.QuestFound = true
		}
		inventory, err := AddInventoryItemTx(c.Tx, instanceID, userID, out.ItemType, value.ID, value.Type, value.Image, value.Description, 1)
		if err != nil {
			return out, err
		}
		out.Inventory = json.RawMessage(inventory)
	default:
		return out, ErrCommandRejected
	}
	result, err := c.Tx.ExecContext(ctx, `UPDATE match_map_cells SET barbel=NULL,tile_code=48 WHERE instance_id=$1 AND q=$2 AND r=$3 AND barbel IS NOT NULL`, instanceID, pos.Q, pos.R)
	if err != nil {
		return out, err
	}
	if n, _ := result.RowsAffected(); n != 1 {
		return out, ErrCommandRejected
	}
	if out.Outcome == "damage" && out.Health == 0 {
		if err := c.LoseHexBackpackTx(ctx, userID, pos); err != nil {
			return out, err
		}
	}
	out.StateRevision, out.MapRevision, err = c.Commit(ctx, &out, true)
	if err != nil {
		return HexBarrelResult{}, err
	}
	if publish != nil {
		publish(out)
	}
	return out, nil
}

func rollHexQuestBarrel(barrel game.ResourceData, resources, artifacts []game.ResourceData, questID int, spawned bool, remainingBarrels int) (any, error) {
	filtered := make([]game.ResourceData, 0, len(artifacts))
	var quest game.ResourceData
	for _, artifact := range artifacts {
		if artifact.ID == questID {
			quest = artifact
			// The chosen kind is reserved for the single guaranteed quest drop.
			continue
		}
		filtered = append(filtered, artifact)
	}
	if questID > 0 && !spawned && remainingBarrels == 1 {
		if quest.ID == 0 {
			return nil, ErrCommandRejected
		}
		return quest, nil
	}
	return hexBarrelRoll(game.FullCell{Barbel: &barrel}, resources, filtered)
}

func getHexResourcesTx(ctx context.Context, tx *sql.Tx) ([]game.ResourceData, error) {
	rows, err := tx.QueryContext(ctx, `SELECT id,type,description,effect,image FROM resources WHERE resource_kind='resource'`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []game.ResourceData
	for rows.Next() {
		var v game.ResourceData
		var effect any
		if err := rows.Scan(&v.ID, &v.Type, &v.Description, &effect, &v.Image); err != nil {
			return nil, err
		}
		v.Image = strings.Trim(v.Image, `"`)
		_ = json.Unmarshal(effectToBytes(effect), &v.Effect)
		out = append(out, v)
	}
	return out, rows.Err()
}
func getHexArtifactsTx(ctx context.Context, tx *sql.Tx) ([]game.ResourceData, error) {
	rows, err := tx.QueryContext(ctx, `SELECT id,name,description,bonus,image FROM artifacts`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []game.ResourceData
	for rows.Next() {
		var v game.ResourceData
		var effect any
		if err := rows.Scan(&v.ID, &v.Type, &v.Description, &effect, &v.Image); err != nil {
			return nil, err
		}
		v.Image = strings.Trim(v.Image, `"`)
		_ = json.Unmarshal(effectToBytes(effect), &v.Effect)
		out = append(out, v)
	}
	return out, rows.Err()
}
