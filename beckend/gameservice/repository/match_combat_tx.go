package repository

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"

	"gameservice/game"
	"gameservice/models"
)

// MatchCombatPlayerTx reads the combat fields under the match command's row lock.
// The match lock is acquired before this method is called.
func (c *MatchCommand) MatchCombatPlayerTx(ctx context.Context, userID int) (*models.PlayerResponse, error) {
	if c == nil || c.Tx == nil {
		return nil, ErrCommandRejected
	}
	var p models.PlayerResponse
	var position []byte
	err := c.Tx.QueryRowContext(ctx, `SELECT user_id,COALESCE(NULLIF(character_type,''),'guardian'),position,energy,max_energy,health,max_health,attack,defense,agility,is_ranged,attack_range,group_id FROM match_players WHERE instance_id=$1 AND user_id=$2`, c.InstanceID, userID).Scan(
		&p.UserID, &p.CharacterType, &position, &p.Energy, &p.MaxEnergy, &p.Health, &p.MaxHealth, &p.Attack, &p.Defense, &p.Agility, &p.IsRanged, &p.AttackRange, &p.GroupID)
	if err != nil {
		return nil, err
	}
	if _, err = models.DecodePosition(position, models.SquareV1); err != nil {
		return nil, err
	}
	if err = json.Unmarshal(position, &p.Position); err != nil {
		return nil, err
	}
	artifact, err := c.QuestArtifactTx(ctx, userID)
	if err != nil {
		return nil, err
	}
	applyQuestArtifactStats(&p, artifact)
	return &p, nil
}

func (c *MatchCommand) QuestArtifactTx(ctx context.Context, userID int) (QuestArtifactEffect, error) {
	if c == nil || c.Tx == nil {
		return QuestArtifactEffect{}, ErrCommandRejected
	}
	var effect QuestArtifactEffect
	err := c.Tx.QueryRowContext(ctx, `SELECT a.id,a.name FROM matches m JOIN artifacts a ON a.id=m.quest_artifact_id JOIN inventory_items ii ON ii.instance_id=m.instance_id AND ii.user_id=$2 AND ii.item_type='artifact' AND ii.item_id=m.quest_artifact_id AND ii.item_count>0 WHERE m.instance_id=$1 LIMIT 1`, c.InstanceID, userID).Scan(&effect.ID, &effect.Name)
	if errors.Is(err, sql.ErrNoRows) {
		return QuestArtifactEffect{}, nil
	}
	return effect, err
}

func (c *MatchCommand) SetCombatPlayerVitalsTx(ctx context.Context, userID, health, energy int) error {
	if c == nil || c.Tx == nil {
		return ErrCommandRejected
	}
	result, err := c.Tx.ExecContext(ctx, `UPDATE match_players SET health=$3,energy=$4 WHERE instance_id=$1 AND user_id=$2`, c.InstanceID, userID, health, energy)
	if err != nil {
		return err
	}
	n, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if n != 1 {
		return ErrCommandRejected
	}
	return nil
}

func (c *MatchCommand) SetMonsterHealthTx(ctx context.Context, monsterID, health int) error {
	if c == nil || c.Tx == nil {
		return ErrCommandRejected
	}
	result, err := c.Tx.ExecContext(ctx, `UPDATE match_monsters SET health=$3 WHERE instance_id=$1 AND monster_instance_id=$2`, c.InstanceID, monsterID, health)
	if err != nil {
		return err
	}
	n, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if n != 1 {
		return ErrCommandRejected
	}
	return nil
}

func (c *MatchCommand) MatchCombatMonsterTx(ctx context.Context, monsterID int) (*MatchMonster, error) {
	if c == nil || c.Tx == nil {
		return nil, ErrCommandRejected
	}
	var m MatchMonster
	err := c.Tx.QueryRowContext(ctx, `SELECT instance_id,monster_instance_id,monster_ref_id,pos_x,pos_y,health,max_health,attack,defense,speed,maneuverability,vision,image FROM match_monsters WHERE instance_id=$1 AND monster_instance_id=$2`, c.InstanceID, monsterID).Scan(&m.InstanceID, &m.MonsterInstanceID, &m.RefID, &m.X, &m.Y, &m.Health, &m.MaxHealth, &m.Attack, &m.Defense, &m.Speed, &m.Maneuverability, &m.Vision, &m.Image)
	return &m, err
}

// SetMonsterHealthAndCellTx updates the canonical monster row and the map cell
// in the same command. The returned flag reports an actual JSON map change.
func (c *MatchCommand) SetMonsterHealthAndCellTx(ctx context.Context, monsterID, health int) (bool, error) {
	if err := c.SetMonsterHealthTx(ctx, monsterID, health); err != nil {
		return false, err
	}
	result, err := c.Tx.ExecContext(ctx, `UPDATE match_map_cells SET monster=jsonb_set(monster,'{health}',to_jsonb($3::int)) WHERE instance_id=$1 AND monster->>'db_instance_id'=$2::text AND (monster->>'health')::int IS DISTINCT FROM $3`, c.InstanceID, monsterID, health)
	if err != nil {
		return false, err
	}
	n, err := result.RowsAffected()
	return n > 0, err
}

// KillMonsterAndClearCellTx removes both representations of a defeated monster.
// The match row is already locked by BeginMatchCommand; this method makes no
// independent database calls or commits.
func (c *MatchCommand) KillMonsterAndClearCellTx(ctx context.Context, monsterID int) (*game.FullCell, bool, error) {
	if c == nil || c.Tx == nil {
		return nil, false, ErrCommandRejected
	}
	result, err := c.Tx.ExecContext(ctx, `DELETE FROM match_monsters WHERE instance_id=$1 AND monster_instance_id=$2`, c.InstanceID, monsterID)
	if err != nil {
		return nil, false, err
	}
	n, err := result.RowsAffected()
	if err != nil || n != 1 {
		return nil, false, ErrCommandRejected
	}
	row := c.Tx.QueryRowContext(ctx, `SELECT cell_id,x,y,tile_code,resource,barbel,monster,is_portal,is_player,structure_type,structure_owner_user_id,structure_health,structure_defense,structure_attack,structure_energy,is_under_construction,construction_turns_left FROM match_map_cells WHERE instance_id=$1 AND monster->>'db_instance_id'=$2::text FOR UPDATE`, c.InstanceID, monsterID)
	cell, err := scanMatchMapCell(row)
	if err != nil {
		return nil, false, err
	}
	cell.Monster = nil
	cell.TileCode = int(game.Walkable)
	if err := SaveMapCellTx(c.Tx, c.InstanceID, *cell); err != nil {
		return nil, false, err
	}
	return cell, true, nil
}
