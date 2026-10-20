package repository

import (
	"context"
	"database/sql"
	"errors"

	"gameservice/internal/hexgrid"
	"gameservice/internal/hexmap"
)

// Only quest metadata and its publicly announced carrier/location are exposed.
func loadHexQuestArtifactTx(ctx context.Context, tx *sql.Tx, instanceID string, questID int, spawned bool) (*hexmap.QuestArtifact, error) {
	quest := &hexmap.QuestArtifact{ID: questID, Status: "not_found"}
	if err := tx.QueryRowContext(ctx, `SELECT name,COALESCE(image,''),COALESCE(description,'') FROM artifacts WHERE id=$1`, questID).Scan(&quest.Name, &quest.Image, &quest.Description); err != nil {
		return nil, err
	}
	if !spawned {
		return quest, nil
	}
	var pos hexgrid.Coord
	err := tx.QueryRowContext(ctx, `SELECT p.user_id,COALESCE(p.name,''),p.position_q,p.position_r FROM inventory_items ii JOIN match_players p ON p.instance_id=ii.instance_id AND p.user_id=ii.user_id WHERE ii.instance_id=$1 AND ii.item_type='artifact' AND ii.item_id=$2 AND ii.item_count>0 LIMIT 1`, instanceID, questID).Scan(&quest.CarrierUserID, &quest.CarrierName, &pos.Q, &pos.R)
	if err == nil {
		quest.Status = "carried"
		quest.Position = &pos
		return quest, nil
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return nil, err
	}
	err = tx.QueryRowContext(ctx, `SELECT q,r FROM match_map_cells WHERE instance_id=$1 AND resource->>'item_type'='artifact' AND resource->>'id'=$2::text LIMIT 1`, instanceID, questID).Scan(&pos.Q, &pos.R)
	if err == nil {
		quest.Status = "ground"
		quest.Position = &pos
		return quest, nil
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return nil, err
	}
	var extracted bool
	if err := tx.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM hex_player_results WHERE instance_id=$1 AND result->>'method'='artifact')`, instanceID).Scan(&extracted); err != nil {
		return nil, err
	}
	if extracted {
		quest.Status = "extracted"
	} else {
		quest.Status = "found"
	}
	return quest, nil
}
