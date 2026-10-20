package repository

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"

	"gameservice/game"
	"gameservice/internal/hexgrid"
)

// LoseHexBackpackTx is called before a defeated player's cell is cleared.
// Permanent Storage, balance, and worn equipment are never touched.
func (c *MatchCommand) LoseHexBackpackTx(ctx context.Context, userID int, deathCell hexgrid.Coord) error {
	if c == nil || c.Tx == nil {
		return ErrCommandRejected
	}
	var backpackRaw []byte
	var characterID int
	if err := c.Tx.QueryRowContext(ctx, `SELECT inventory,character_id FROM match_players WHERE instance_id=$1 AND user_id=$2`, c.InstanceID, userID).Scan(&backpackRaw, &characterID); err != nil {
		return err
	}
	lost, err := parseStorage(backpackRaw)
	if err != nil {
		return err
	}
	if err := enrichHexResultItemsTx(ctx, c.Tx, lost); err != nil {
		return err
	}
	var questID int
	if err := c.Tx.QueryRowContext(ctx, `SELECT COALESCE(quest_artifact_id,0) FROM matches WHERE instance_id=$1`, c.InstanceID).Scan(&questID); err != nil {
		return err
	}
	var carried bool
	if questID > 0 {
		if err := c.Tx.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM inventory_items WHERE instance_id=$1 AND user_id=$2 AND item_type='artifact' AND item_id=$3 AND item_count>0)`, c.InstanceID, userID, questID).Scan(&carried); err != nil {
			return err
		}
	}
	if carried {
		delete(lost, fmt.Sprintf("artifact_%d", questID))
		var artifact game.ResourceData
		if err := c.Tx.QueryRowContext(ctx, `SELECT id,name,description,image FROM artifacts WHERE id=$1`, questID).Scan(&artifact.ID, &artifact.Type, &artifact.Description, &artifact.Image); err != nil {
			return err
		}
		artifact.ItemType = "artifact"
		drop, err := reachableHexDropCellTx(ctx, c.Tx, c.InstanceID, deathCell)
		if err != nil {
			return err
		}
		raw, err := json.Marshal(artifact)
		if err != nil {
			return err
		}
		result, err := c.Tx.ExecContext(ctx, `UPDATE match_map_cells SET resource=$4,tile_code=82 WHERE instance_id=$1 AND q=$2 AND r=$3 AND resource IS NULL AND barbel IS NULL`, c.InstanceID, drop.Q, drop.R, raw)
		if err != nil {
			return err
		}
		if count, err := result.RowsAffected(); err != nil || count != 1 {
			return ErrCommandRejected
		}
	}
	if _, err := c.Tx.ExecContext(ctx, `DELETE FROM inventory_items WHERE instance_id=$1 AND user_id=$2`, c.InstanceID, userID); err != nil {
		return err
	}
	if _, err := c.Tx.ExecContext(ctx, `UPDATE match_players SET inventory='{}'::jsonb WHERE instance_id=$1 AND user_id=$2`, c.InstanceID, userID); err != nil {
		return err
	}
	rows, err := c.Tx.QueryContext(ctx, `SELECT it.name,it.image_url FROM item_instances ii JOIN item_templates it ON it.id=ii.template_id WHERE ii.match_instance_id=$1 AND ii.owner_user_id=$2 AND ii.status<>'equipped'`, c.InstanceID, userID)
	if err != nil {
		return err
	}
	lostEquipment := []string{}
	lostEquipmentItems := []HexResultItem{}
	for rows.Next() {
		var item HexResultItem
		if err := rows.Scan(&item.Name, &item.Image); err != nil {
			rows.Close()
			return err
		}
		lostEquipment = append(lostEquipment, item.Name)
		lostEquipmentItems = append(lostEquipmentItems, item)
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return err
	}
	rows.Close()
	if _, err := c.Tx.ExecContext(ctx, `DELETE FROM item_instances WHERE match_instance_id=$1 AND owner_user_id=$2 AND status<>'equipped'`, c.InstanceID, userID); err != nil {
		return err
	}
	if _, err := c.Tx.ExecContext(ctx, `UPDATE item_instances SET match_instance_id=NULL,updated_at=now() WHERE match_instance_id=$1 AND owner_user_id=$2 AND status='equipped'`, c.InstanceID, userID); err != nil {
		return err
	}
	wornEquipment, err := loadHexWornEquipmentTx(ctx, c.Tx, characterID)
	if err != nil {
		return err
	}
	result := HexPlayerResult{Method: "defeat", Transferred: map[string]map[string]any{}, Lost: lost, Equipment: []string{}, LostEquipment: lostEquipment, LostEquipmentItems: lostEquipmentItems, WornEquipment: wornEquipment}
	var rewardsReady bool
	if err := c.Tx.QueryRowContext(ctx, `SELECT COALESCE(turn_number,1),hex_reward_events_ready FROM matches WHERE instance_id=$1`, c.InstanceID).Scan(&result.TurnNumber, &rewardsReady); err != nil {
		return err
	}
	if rewardsReady {
		events, err := c.LoadCombatResultEventsTx(ctx)
		if err != nil {
			return err
		}
		_, _, playerKills, monsterKills, damageTotal, damagePlayers, damageMonsters, damageTaken, err := game.CalculateResultsFromEvents(events, userID, false)
		if err != nil {
			return err
		}
		result.Stats = &HexResultStats{PlayerKills: playerKills, MonsterKills: monsterKills, DamageTotal: damageTotal, DamageToPlayers: damagePlayers, DamageToMonsters: damageMonsters, DamageTaken: damageTaken}
	}
	if err := c.Tx.QueryRowContext(ctx, `SELECT state_revision+1,map_revision+1 FROM matches WHERE instance_id=$1`, c.InstanceID).Scan(&result.StateRevision, &result.MapRevision); err != nil {
		return err
	}
	raw, err := json.Marshal(result)
	if err != nil {
		return err
	}
	_, err = c.Tx.ExecContext(ctx, `INSERT INTO hex_player_results(instance_id,user_id,result) VALUES($1,$2,$3::jsonb)`, c.InstanceID, userID, raw)
	return err
}

// A drop must be reachable from the death cell without crossing rock or a wall.
// Other actors may stand on the cell temporarily; they do not make loot unreachable.
func reachableHexDropCellTx(ctx context.Context, tx *sql.Tx, instanceID string, start hexgrid.Coord) (hexgrid.Coord, error) {
	rows, err := tx.QueryContext(ctx, `SELECT q,r,tile_code,resource,barbel FROM match_map_cells WHERE instance_id=$1`, instanceID)
	if err != nil {
		return start, err
	}
	type cell struct{ floor, free bool }
	cells := map[hexgrid.Coord]cell{}
	for rows.Next() {
		var pos hexgrid.Coord
		var tile int
		var resource, barrel []byte
		if err := rows.Scan(&pos.Q, &pos.R, &tile, &resource, &barrel); err != nil {
			rows.Close()
			return start, err
		}
		cells[pos] = cell{floor: tile != 32, free: len(resource) == 0 && len(barrel) == 0}
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return start, err
	}
	rows.Close()
	edges, err := loadHexEdges(ctx, tx, instanceID)
	if err != nil {
		return start, err
	}
	walls := map[hexgrid.EdgeKey]bool{}
	for _, edge := range edges {
		next, err := hexgrid.Neighbor(edge.Position, edge.Direction)
		if err != nil {
			return start, err
		}
		key, err := hexgrid.CanonicalEdge(edge.Position, next)
		if err != nil {
			return start, err
		}
		walls[key] = true
	}
	queue := []hexgrid.Coord{start}
	seen := map[hexgrid.Coord]bool{start: true}
	for len(queue) > 0 {
		current := queue[0]
		queue = queue[1:]
		if candidate := cells[current]; candidate.floor && candidate.free {
			return current, nil
		}
		for direction := 0; direction < 6; direction++ {
			next, err := hexgrid.Neighbor(current, direction)
			if err != nil || seen[next] || !cells[next].floor {
				continue
			}
			key, _ := hexgrid.CanonicalEdge(current, next)
			if walls[key] {
				continue
			}
			seen[next] = true
			queue = append(queue, next)
		}
	}
	return start, ErrHexBlocked
}
