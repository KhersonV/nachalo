package repository

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"gameservice/internal/hexgrid"
)

type HexBuildResult struct {
	Position      hexgrid.Coord   `json:"position"`
	StructureType string          `json:"structureType"`
	Inventory     json.RawMessage `json:"inventory"`
	Energy        int             `json:"energy"`
	StateRevision int64           `json:"stateRevision"`
	MapRevision   int64           `json:"mapRevision"`
	Replayed      bool            `json:"-"`
}

type HexStructureAttackResult struct {
	Position      hexgrid.Coord `json:"position"`
	StructureType string        `json:"structureType"`
	Damage        int           `json:"damage"`
	Health        int           `json:"health"`
	Energy        int           `json:"energy"`
	Destroyed     bool          `json:"destroyed"`
	StateRevision int64         `json:"stateRevision"`
	MapRevision   int64         `json:"mapRevision"`
	Replayed      bool          `json:"-"`
}

func (r *HexStructureAttackResult) SetCommandRevisions(state, mapRevision int64) {
	r.StateRevision, r.MapRevision = state, mapRevision
}

// Square structure attacks are adjacent melee strikes with flat attack minus defense.
func ApplyHexStructureAttack(ctx context.Context, instanceID string, userID int, commandID string, pos hexgrid.Coord, publish func(HexStructureAttackResult)) (HexStructureAttackResult, error) {
	const meleeAttackEnergyCost = 6 // Square combat.go
	var out HexStructureAttackResult
	if !hexgrid.Valid(pos) {
		return out, ErrCommandRejected
	}
	ctx, cancel := context.WithTimeout(ctx, 8*time.Second)
	defer cancel()
	c, err := BeginMatchCommand(ctx, instanceID, userID, commandID, fmt.Sprintf("hex-structure-attack-v1:%d:%d:%d", userID, pos.Q, pos.R))
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
	var version, hp, energy, attack int
	var playerPos hexgrid.Coord
	err = c.Tx.QueryRowContext(ctx, `SELECT m.geometry->>'kind',m.schema_version,p.health,p.energy,p.attack,p.position_q,p.position_r FROM matches m JOIN match_players p ON p.instance_id=m.instance_id AND p.user_id=$2 WHERE m.instance_id=$1`, instanceID, userID).Scan(&mode, &version, &hp, &energy, &attack, &playerPos.Q, &playerPos.R)
	if err != nil {
		return out, err
	}
	distance, err := hexgrid.Distance(playerPos, pos)
	if err != nil || mode != "hex" || version != 2 || hp <= 0 || distance != 1 || energy < meleeAttackEnergyCost {
		return out, ErrCommandRejected
	}
	var kind string
	var owner, structureHP, defense int
	var building bool
	err = c.Tx.QueryRowContext(ctx, `SELECT structure_type,structure_owner_user_id,structure_health,structure_defense,is_under_construction FROM match_map_cells WHERE instance_id=$1 AND q=$2 AND r=$3`, instanceID, pos.Q, pos.R).Scan(&kind, &owner, &structureHP, &defense, &building)
	if err != nil || kind == "" || building || structureHP <= 0 || (owner == userID && kind != "wall") {
		return out, ErrCommandRejected
	}
	damage := attack - defense
	if damage < 0 {
		damage = 0
	}
	newHP := structureHP - damage
	if newHP < 0 {
		newHP = 0
	}
	if _, err := c.Tx.ExecContext(ctx, `UPDATE match_players SET energy=energy-$3 WHERE instance_id=$1 AND user_id=$2 AND energy>=$3`, instanceID, userID, meleeAttackEnergyCost); err != nil {
		return out, err
	}
	if newHP > 0 {
		if _, err := c.Tx.ExecContext(ctx, `UPDATE match_map_cells SET structure_health=$4 WHERE instance_id=$1 AND q=$2 AND r=$3`, instanceID, pos.Q, pos.R, newHP); err != nil {
			return out, err
		}
	} else {
		if _, err := c.Tx.ExecContext(ctx, `UPDATE match_map_cells SET structure_type='',structure_owner_user_id=0,structure_health=0,structure_defense=0,structure_attack=0,structure_energy=0,is_under_construction=false,construction_turns_left=0 WHERE instance_id=$1 AND q=$2 AND r=$3`, instanceID, pos.Q, pos.R); err != nil {
			return out, err
		}
		column, err := structureTypeToColumn(kind)
		if err != nil {
			return out, err
		}
		query := fmt.Sprintf(`UPDATE match_player_structures SET %s=GREATEST(0,%s-1),total_buildings=GREATEST(0,total_buildings-1) WHERE instance_id=$1 AND user_id=$2`, column, column)
		if _, err := c.Tx.ExecContext(ctx, query, instanceID, owner); err != nil {
			return out, err
		}
		if kind == "scout_tower" {
			if _, err := c.Tx.ExecContext(ctx, `UPDATE match_players SET sight_range=GREATEST(1,sight_range-1) WHERE instance_id=$1 AND user_id=$2 AND EXISTS(SELECT 1 FROM match_player_structures WHERE instance_id=$1 AND user_id=$2 AND scout_towers_count=0 AND has_scout_tower_bonus)`, instanceID, owner); err != nil {
				return out, err
			}
			if _, err := c.Tx.ExecContext(ctx, `UPDATE match_player_structures SET has_scout_tower_bonus=false WHERE instance_id=$1 AND user_id=$2 AND scout_towers_count=0`, instanceID, owner); err != nil {
				return out, err
			}
		}
	}
	out.Position, out.StructureType, out.Damage, out.Health, out.Energy, out.Destroyed = pos, kind, damage, newHP, energy-meleeAttackEnergyCost, newHP == 0
	out.StateRevision, out.MapRevision, err = c.Commit(ctx, &out, true)
	if err != nil {
		return HexStructureAttackResult{}, err
	}
	if publish != nil {
		publish(out)
	}
	return out, nil
}

func (r *HexBuildResult) SetCommandRevisions(state, mapRevision int64) {
	r.StateRevision, r.MapRevision = state, mapRevision
}

// Square blueprint statistics and its two-turn construction are kept here.
func hexBlueprintStats(key string) (kind string, hp, defense, attack int, ok bool) {
	switch key {
	case "blueprint_scout_tower":
		return "scout_tower", 30, 5, 0, true
	case "blueprint_turret":
		return "turret", 30, 0, 10, true
	case "blueprint_wall":
		return "wall", 30, 8, 0, true
	default:
		return "", 0, 0, 0, false
	}
}

func ApplyHexBlueprint(ctx context.Context, instanceID string, userID int, commandID, key string, pos hexgrid.Coord, publish func(HexBuildResult)) (HexBuildResult, error) {
	var out HexBuildResult
	kind, hp, defense, attack, ok := hexBlueprintStats(key)
	if !ok || !hexgrid.Valid(pos) {
		return out, ErrCommandRejected
	}
	ctx, cancel := context.WithTimeout(ctx, 8*time.Second)
	defer cancel()
	c, err := BeginMatchCommand(ctx, instanceID, userID, commandID, fmt.Sprintf("hex-build-v1:%d:%s:%d:%d", userID, key, pos.Q, pos.R))
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
	var version, health, energy int
	var playerPos hexgrid.Coord
	var raw []byte
	err = c.Tx.QueryRowContext(ctx, `SELECT m.geometry->>'kind',m.schema_version,p.health,p.energy,p.position_q,p.position_r,p.inventory FROM matches m JOIN match_players p ON p.instance_id=m.instance_id AND p.user_id=$2 WHERE m.instance_id=$1`, instanceID, userID).Scan(&mode, &version, &health, &energy, &playerPos.Q, &playerPos.R, &raw)
	if err != nil {
		return out, err
	}
	distance, err := hexgrid.Distance(playerPos, pos)
	if err != nil || mode != "hex" || version != 2 || health <= 0 || distance != 1 {
		return out, ErrCommandRejected
	}
	var tile int
	var occupied, portal, building bool
	var structure string
	var resource, barrel, monster []byte
	err = c.Tx.QueryRowContext(ctx, `SELECT tile_code,is_player,is_portal,is_under_construction,structure_type,resource,barbel,monster FROM match_map_cells WHERE instance_id=$1 AND q=$2 AND r=$3`, instanceID, pos.Q, pos.R).Scan(&tile, &occupied, &portal, &building, &structure, &resource, &barrel, &monster)
	if err != nil {
		return out, ErrCommandRejected
	}
	if tile != 48 || occupied || portal || building || structure != "" || len(resource) > 0 || len(barrel) > 0 || len(monster) > 0 {
		return out, ErrCommandRejected
	}
	var enemy int
	if err := c.Tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM match_monsters WHERE instance_id=$1 AND pos_q=$2 AND pos_r=$3 AND health>0`, instanceID, pos.Q, pos.R).Scan(&enemy); err != nil {
		return out, err
	}
	if enemy != 0 {
		return out, ErrCommandRejected
	}
	var inventory map[string]map[string]any
	if err := json.Unmarshal(raw, &inventory); err != nil {
		return out, err
	}
	entry, found := inventory[key]
	if !found {
		return out, ErrCommandRejected
	}
	count, valid := entry["item_count"].(float64)
	if !valid || count < 1 || count != float64(int(count)) {
		return out, ErrCommandRejected
	}
	if count == 1 {
		delete(inventory, key)
	} else {
		entry["item_count"] = int(count) - 1
	}
	out.Inventory, err = json.Marshal(inventory)
	if err != nil {
		return out, err
	}
	if _, err := c.Tx.ExecContext(ctx, `UPDATE match_map_cells SET structure_type=$4,structure_owner_user_id=$5,structure_health=$6,structure_defense=$7,structure_attack=$8,is_under_construction=true,construction_turns_left=2 WHERE instance_id=$1 AND q=$2 AND r=$3`, instanceID, pos.Q, pos.R, kind, userID, hp, defense, attack); err != nil {
		return out, err
	}
	if _, err := c.Tx.ExecContext(ctx, `UPDATE match_players SET inventory=$3 WHERE instance_id=$1 AND user_id=$2`, instanceID, userID, out.Inventory); err != nil {
		return out, err
	}
	out.Position, out.StructureType, out.Energy = pos, kind, energy
	out.StateRevision, out.MapRevision, err = c.Commit(ctx, &out, true)
	if err != nil {
		return HexBuildResult{}, err
	}
	if publish != nil {
		publish(out)
	}
	return out, nil
}
