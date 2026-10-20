package repository

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"gameservice/game"
	"gameservice/internal/hexgrid"
)

// Shared position rule for HEX cell interactions, including a future barrel command.
var ErrHexInteractionPosition = errors.New("stand on the HEX cell to interact with it")

func requireHexInteractionPosition(player, target hexgrid.Coord) error {
	if player != target {
		return ErrHexInteractionPosition
	}
	return nil
}

type HexCollectResult struct {
	Position      hexgrid.Coord   `json:"position"`
	ItemID        int             `json:"itemId"`
	ItemType      string          `json:"itemType"`
	PlayerName    string          `json:"playerName,omitempty"`
	Inventory     json.RawMessage `json:"inventory"`
	StateRevision int64           `json:"stateRevision"`
	MapRevision   int64           `json:"mapRevision"`
	Replayed      bool            `json:"-"`
}

func (r *HexCollectResult) SetCommandRevisions(state, mapRevision int64) {
	r.StateRevision, r.MapRevision = state, mapRevision
}

func ApplyHexResourceCollection(ctx context.Context, instanceID string, userID int, commandID string, pos hexgrid.Coord, publish func(HexCollectResult)) (HexCollectResult, error) {
	var out HexCollectResult
	if !hexgrid.Valid(pos) {
		return out, ErrCommandRejected
	}
	ctx, cancel := context.WithTimeout(ctx, 8*time.Second)
	defer cancel()
	c, err := BeginMatchCommand(ctx, instanceID, userID, commandID, fmt.Sprintf("hex-collect-v1:%d:%d:%d", userID, pos.Q, pos.R))
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
	var playerPosition hexgrid.Coord
	if err := c.Tx.QueryRowContext(ctx, `SELECT m.geometry->>'kind',m.schema_version,p.health,p.position_q,p.position_r,COALESCE(p.name,'') FROM matches m JOIN match_players p ON p.instance_id=m.instance_id AND p.user_id=$2 WHERE m.instance_id=$1`, instanceID, userID).Scan(&mode, &version, &health, &playerPosition.Q, &playerPosition.R, &out.PlayerName); err != nil {
		return out, err
	}
	if mode != "hex" || version != 2 || health <= 0 {
		return out, ErrCommandRejected
	}
	if err := requireHexInteractionPosition(playerPosition, pos); err != nil {
		return out, err
	}
	var raw []byte
	var tile int
	err = c.Tx.QueryRowContext(ctx, `SELECT resource,tile_code FROM match_map_cells WHERE instance_id=$1 AND q=$2 AND r=$3 FOR UPDATE`, instanceID, pos.Q, pos.R).Scan(&raw, &tile)
	if err == sql.ErrNoRows {
		return out, ErrCommandRejected
	}
	if err != nil {
		return out, err
	}
	if len(raw) == 0 || tile != 82 {
		return out, ErrCommandRejected
	}
	var resource game.ResourceData
	if err := json.Unmarshal(raw, &resource); err != nil {
		return out, err
	}
	if resource.ID <= 0 {
		return out, ErrCommandRejected
	}
	itemType := "resource"
	if resource.ItemType == "artifact" {
		var questID int
		if err := c.Tx.QueryRowContext(ctx, `SELECT COALESCE(quest_artifact_id,0) FROM matches WHERE instance_id=$1`, instanceID).Scan(&questID); err != nil {
			return out, err
		}
		if questID == 0 || resource.ID != questID {
			return out, ErrCommandRejected
		}
		itemType = "artifact"
	}
	result, err := c.Tx.ExecContext(ctx, `UPDATE match_map_cells SET resource=NULL,tile_code=48 WHERE instance_id=$1 AND q=$2 AND r=$3 AND resource IS NOT NULL`, instanceID, pos.Q, pos.R)
	if err != nil {
		return out, err
	}
	if n, _ := result.RowsAffected(); n != 1 {
		return out, ErrCommandRejected
	}
	updated, err := AddInventoryItemTx(c.Tx, instanceID, userID, itemType, resource.ID, resource.Type, resource.Image, resource.Description, 1)
	if err != nil {
		return out, err
	}
	out.Position, out.ItemID, out.ItemType, out.Inventory = pos, resource.ID, itemType, json.RawMessage(updated)
	out.StateRevision, out.MapRevision, err = c.Commit(ctx, &out, true)
	if err != nil {
		return HexCollectResult{}, err
	}
	if publish != nil {
		publish(out)
	}
	return out, nil
}
