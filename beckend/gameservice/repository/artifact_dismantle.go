package repository

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
)

var ErrDismantleInvalid = errors.New("artifact_dismantle_invalid")
var ErrDismantleMatch = errors.New("artifact_dismantle_outside_match")
var ErrDismantleReplay = errors.New("artifact_dismantle_request_conflict")

type ArtifactDismantlePreview struct {
	ArtifactID int               `json:"artifactId"`
	Name       string            `json:"name"`
	Quantity   int               `json:"quantity"`
	Owned      int               `json:"owned"`
	Yields     []CraftIngredient `json:"yields"`
}
type ArtifactDismantleResult struct {
	ArtifactDismantlePreview
	Inventory map[string]map[string]any `json:"inventory"`
}

func requireOutsideMatchTx(ctx context.Context, tx *sql.Tx, user int) error {
	var active bool
	if err := tx.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM match_players p JOIN matches m ON m.instance_id=p.instance_id WHERE p.user_id=$1 AND p.health>0 AND m.finished_at IS NULL)`, user).Scan(&active); err != nil {
		return err
	}
	if active {
		return ErrDismantleMatch
	}
	return nil
}
func dismantleRestricted(entry map[string]any) bool {
	if marketStackEntryRestricted(entry) || entry["item_type"] == "quest" {
		return true
	}
	for _, field := range []string{"quest", "is_quest", "isQuest", "quest_item", "quest_artifact", "questArtifact", "quest_artifact_id"} {
		v := entry[field]
		if v != nil && v != false && v != "" && v != "0" && v != "false" && v != float64(0) {
			return true
		}
	}
	return false
}
func artifactDismantlePreviewTx(ctx context.Context, tx *sql.Tx, user, artifact, quantity int, raw []byte) (ArtifactDismantlePreview, error) {
	out := ArtifactDismantlePreview{ArtifactID: artifact, Quantity: quantity}
	if artifact <= 0 || quantity < 1 || quantity > 100 {
		return out, ErrDismantleInvalid
	}
	if err := requireOutsideMatchTx(ctx, tx, user); err != nil {
		return out, err
	}
	var name string
	if err := tx.QueryRowContext(ctx, `SELECT name FROM artifacts WHERE id=$1`, artifact).Scan(&name); err != nil {
		if err == sql.ErrNoRows {
			return out, ErrDismantleInvalid
		}
		return out, err
	}
	table, ok := jewelryCatalog.Dismantle[name]
	// An allowlist of existing ordinary catalogue names excludes new quest-only types.
	// The match's quest copy never enters Storage (mergeHexExtractionStorage).
	if !ok {
		return out, ErrDismantleInvalid
	}
	inv, err := parseStorage(raw)
	if err != nil {
		return out, err
	}
	entry := inv[fmt.Sprintf("artifact_%d", artifact)]
	if dismantleRestricted(entry) {
		return out, ErrDismantleInvalid
	}
	out.Name = name
	out.Owned = storageCount(entry)
	if out.Owned < quantity {
		return out, ErrDismantleInvalid
	}
	out.Yields = append([]CraftIngredient(nil), table...)
	for i := range out.Yields {
		v := &out.Yields[i]
		id, image, description, _, err := craftResourceTx(ctx, tx, v.Code)
		if err != nil {
			return out, err
		}
		v.ResourceID = id
		v.Image = image
		v.Description = description
		v.Name = resourceDisplayName(v.Code)
		v.Quantity *= quantity
		v.Owned = storageCount(inv[fmt.Sprintf("resource_%d", id)])
	}
	return out, nil
}
func PreviewArtifactDismantle(ctx context.Context, user, artifact, quantity int) (ArtifactDismantlePreview, error) {
	tx, err := DB.BeginTx(ctx, &sql.TxOptions{ReadOnly: true, Isolation: sql.LevelRepeatableRead})
	if err != nil {
		return ArtifactDismantlePreview{}, err
	}
	defer tx.Rollback()
	var raw []byte
	if err = tx.QueryRowContext(ctx, `SELECT inventory FROM player_profiles WHERE user_id=$1`, user).Scan(&raw); err != nil {
		return ArtifactDismantlePreview{}, err
	}
	out, err := artifactDismantlePreviewTx(ctx, tx, user, artifact, quantity, raw)
	if err != nil {
		return out, err
	}
	return out, tx.Commit()
}
func DismantleArtifact(ctx context.Context, user, artifact, quantity int, command string) (ArtifactDismantleResult, error) {
	var out ArtifactDismantleResult
	if user <= 0 || artifact <= 0 || quantity < 1 || quantity > 100 || len(strings.TrimSpace(command)) < 8 || len(command) > 128 {
		return out, ErrDismantleInvalid
	}
	tx, err := DB.BeginTx(ctx, nil)
	if err != nil {
		return out, err
	}
	defer tx.Rollback()
	// Same account row lock as crafting, purchases, marketplace and loadout transfer.
	var raw, previous []byte
	if err = tx.QueryRowContext(ctx, `SELECT inventory FROM player_profiles WHERE user_id=$1 FOR UPDATE`, user).Scan(&raw); err != nil {
		return out, err
	}
	err = tx.QueryRowContext(ctx, `SELECT response FROM artifact_dismantle_operations WHERE user_id=$1 AND command_id=$2`, user, command).Scan(&previous)
	if err == nil {
		if err = json.Unmarshal(previous, &out); err != nil {
			return out, err
		}
		if out.ArtifactID != artifact || out.Quantity != quantity {
			return ArtifactDismantleResult{}, ErrDismantleReplay
		}
		return out, nil
	}
	if err != sql.ErrNoRows {
		return out, err
	}
	preview, err := artifactDismantlePreviewTx(ctx, tx, user, artifact, quantity, raw)
	if err != nil {
		return out, err
	}
	inv, err := parseStorage(raw)
	if err != nil {
		return out, err
	}
	key := fmt.Sprintf("artifact_%d", artifact)
	left := preview.Owned - quantity
	if left == 0 {
		delete(inv, key)
	} else {
		inv[key]["item_count"] = left
	}
	for _, v := range preview.Yields {
		key := fmt.Sprintf("resource_%d", v.ResourceID)
		if marketStackEntryRestricted(inv[key]) {
			return out, ErrDismantleInvalid
		}
		count := storageCount(inv[key]) + v.Quantity
		if count > 1_000_000_000 {
			return out, ErrDismantleInvalid
		}
		inv[key] = map[string]any{"item_type": "resource", "item_id": v.ResourceID, "resource_code": v.Code, "name": v.Name, "item_name": v.Name, "item_count": count, "image": v.Image, "description": v.Description, "effect": map[string]int{}}
	}
	encoded, err := json.Marshal(inv)
	if err != nil {
		return out, err
	}
	if _, err = tx.ExecContext(ctx, `UPDATE player_profiles SET inventory=$2,updated_at=now() WHERE user_id=$1`, user, string(encoded)); err != nil {
		return out, err
	}
	out = ArtifactDismantleResult{ArtifactDismantlePreview: preview, Inventory: inv}
	encoded, err = json.Marshal(out)
	if err != nil {
		return out, err
	}
	if _, err = tx.ExecContext(ctx, `INSERT INTO artifact_dismantle_operations(user_id,command_id,artifact_id,quantity,response) VALUES($1,$2,$3,$4,$5)`, user, command, artifact, quantity, string(encoded)); err != nil {
		return out, err
	}
	return out, tx.Commit()
}
