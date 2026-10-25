package repository

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"

	"gameservice/game"
	"gameservice/internal/hexgrid"
)

// Construction advances once per committed match turn, as in Square.
func progressHexConstructionTx(ctx context.Context, tx *sql.Tx, instanceID string) (bool, error) {
	rows, err := tx.QueryContext(ctx, `UPDATE match_map_cells SET construction_turns_left=GREATEST(0,construction_turns_left-1),is_under_construction=(construction_turns_left>1) WHERE instance_id=$1 AND is_under_construction RETURNING structure_type,structure_owner_user_id,construction_turns_left`, instanceID)
	if err != nil {
		return false, err
	}
	type completed struct {
		kind  string
		owner int
	}
	finished := []completed{}
	changed := false
	for rows.Next() {
		changed = true
		var kind string
		var owner, left int
		if err := rows.Scan(&kind, &owner, &left); err != nil {
			rows.Close()
			return false, err
		}
		if left == 0 && owner > 0 {
			finished = append(finished, completed{kind, owner})
		}
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return false, err
	}
	rows.Close()
	for _, item := range finished {
		column, err := structureTypeToColumn(item.kind)
		if err != nil {
			return false, err
		}
		query := fmt.Sprintf(`INSERT INTO match_player_structures(instance_id,user_id,%s,total_buildings) VALUES($1,$2,1,1) ON CONFLICT(instance_id,user_id) DO UPDATE SET %s=match_player_structures.%s+1,total_buildings=match_player_structures.total_buildings+1`, column, column, column)
		if _, err := tx.ExecContext(ctx, query, instanceID, item.owner); err != nil {
			return false, err
		}
		if item.kind == "scout_tower" {
			if _, err := tx.ExecContext(ctx, `UPDATE match_players SET sight_range=sight_range+1 WHERE instance_id=$1 AND user_id=$2 AND NOT EXISTS(SELECT 1 FROM match_player_structures WHERE instance_id=$1 AND user_id=$2 AND has_scout_tower_bonus)`, instanceID, item.owner); err != nil {
				return false, err
			}
			if _, err := tx.ExecContext(ctx, `UPDATE match_player_structures SET has_scout_tower_bonus=true WHERE instance_id=$1 AND user_id=$2`, instanceID, item.owner); err != nil {
				return false, err
			}
		}
	}
	return changed, nil
}

func progressHexTurretsTx(ctx context.Context, c *MatchCommand) (bool, error) {
	rows, err := c.Tx.QueryContext(ctx, `SELECT q,r,structure_owner_user_id,structure_attack,structure_defense,structure_energy FROM match_map_cells WHERE instance_id=$1 AND structure_type='turret' AND NOT is_under_construction AND structure_health>0 AND structure_owner_user_id>0 ORDER BY cell_id`, c.InstanceID)
	if err != nil {
		return false, err
	}
	type turret struct {
		pos                            hexgrid.Coord
		owner, attack, defense, energy int
	}
	all := []turret{}
	for rows.Next() {
		var t turret
		if err := rows.Scan(&t.pos.Q, &t.pos.R, &t.owner, &t.attack, &t.defense, &t.energy); err != nil {
			rows.Close()
			return false, err
		}
		all = append(all, t)
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return false, err
	}
	rows.Close()
	changed := false
	for _, t := range all {
		var matchFinished bool
		if err := c.Tx.QueryRowContext(ctx, `SELECT finished_at IS NOT NULL FROM matches WHERE instance_id=$1`, c.InstanceID).Scan(&matchFinished); err != nil {
			return false, err
		}
		if matchFinished {
			break
		}
		if t.attack == 0 {
			t.attack = 15
		}
		if t.defense == 0 {
			t.defense = 5
		}
		if t.energy < 8 {
			t.energy = 8
		}
		var ownerGroup int
		if err := c.Tx.QueryRowContext(ctx, `SELECT group_id FROM match_players WHERE instance_id=$1 AND user_id=$2`, c.InstanceID, t.owner).Scan(&ownerGroup); err != nil {
			return false, err
		}
		// Turrets may keep firing after their owner dies or extracts. Such shots
		// cannot add rewards to a result that has already been finalized.
		var ownerEligible bool
		if err := c.Tx.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM match_players p WHERE p.instance_id=$1 AND p.user_id=$2 AND p.health>0 AND NOT EXISTS(SELECT 1 FROM hex_player_results r WHERE r.instance_id=p.instance_id AND r.user_id=p.user_id))`, c.InstanceID, t.owner).Scan(&ownerEligible); err != nil {
			return false, err
		}
		targetFound := false
		players, err := c.Tx.QueryContext(ctx, `SELECT user_id,position_q,position_r,health,defense FROM match_players WHERE instance_id=$1 AND user_id<>$2 AND group_id<>$3 AND health>0 ORDER BY user_id`, c.InstanceID, t.owner, ownerGroup)
		if err != nil {
			return false, err
		}
		type target struct {
			id              int
			pos             hexgrid.Coord
			health, defense int
		}
		var selected target
		for players.Next() {
			var p target
			if err := players.Scan(&p.id, &p.pos.Q, &p.pos.R, &p.health, &p.defense); err != nil {
				players.Close()
				return false, err
			}
			d, err := hexgrid.Distance(t.pos, p.pos)
			if err != nil {
				players.Close()
				return false, err
			}
			if d <= 2 {
				selected, targetFound = p, true
				break
			}
		}
		if err := players.Err(); err != nil {
			players.Close()
			return false, err
		}
		players.Close()
		monsterTarget := false
		if !targetFound {
			monsters, err := c.Tx.QueryContext(ctx, `SELECT monster_instance_id,pos_q,pos_r,health,defense FROM match_monsters WHERE instance_id=$1 AND health>0 ORDER BY monster_instance_id`, c.InstanceID)
			if err != nil {
				return false, err
			}
			for monsters.Next() {
				var m target
				if err := monsters.Scan(&m.id, &m.pos.Q, &m.pos.R, &m.health, &m.defense); err != nil {
					monsters.Close()
					return false, err
				}
				d, err := hexgrid.Distance(t.pos, m.pos)
				if err != nil {
					monsters.Close()
					return false, err
				}
				if d <= 2 {
					selected, targetFound, monsterTarget = m, true, true
					break
				}
			}
			if err := monsters.Err(); err != nil {
				monsters.Close()
				return false, err
			}
			monsters.Close()
		}
		if targetFound && t.energy >= 6 {
			t.energy -= 6
			damage := t.attack - selected.defense
			if damage < 0 {
				damage = 0
			}
			newHealth := selected.health - damage
			if newHealth < 0 {
				newHealth = 0
			}
			targetType := "player"
			if monsterTarget {
				targetType = "monster"
			}
			damageSource, damageDealer := game.ResultSourcePlayer, t.owner
			if monsterTarget && !ownerEligible {
				damageSource, damageDealer = game.ResultSourceEnvironment, 0
			}
			if err := c.RecordDamageEventTx(ctx, game.DamageEvent{SourceType: damageSource, DealerID: damageDealer, TargetType: targetType, TargetID: selected.id, Amount: selected.health - newHealth}); err != nil {
				return false, err
			}
			if monsterTarget {
				if newHealth == 0 {
					recorded, err := c.RecordKillEventTx(ctx, game.KillEvent{SourceType: damageSource, KillerID: damageDealer, VictimType: "monster", VictimID: selected.id, Damage: damage})
					if err != nil {
						return false, err
					}
					if recorded && ownerEligible {
						if err := c.GrantProfessionMonsterLootTx(ctx, t.owner, selected.id); err != nil {
							return false, err
						}
					}
					removed, err := c.Tx.ExecContext(ctx, `DELETE FROM match_monsters WHERE instance_id=$1 AND monster_instance_id=$2 AND health>0`, c.InstanceID, selected.id)
					if err != nil {
						return false, err
					}
					count, err := removed.RowsAffected()
					if err != nil {
						return false, err
					}
					if count != 1 {
						return false, ErrCommandRejected
					}
					if _, err := c.Tx.ExecContext(ctx, `UPDATE match_map_cells SET monster=NULL WHERE instance_id=$1 AND q=$2 AND r=$3`, c.InstanceID, selected.pos.Q, selected.pos.R); err != nil {
						return false, err
					}
					if recorded && ownerEligible && RollEquipmentDropChance() {
						if _, err := GrantRandomEquipmentDropForMatchTx(c.Tx, t.owner, c.InstanceID); err != nil {
							return false, err
						}
					}
				} else {
					if _, err := c.Tx.ExecContext(ctx, `UPDATE match_monsters SET health=$3 WHERE instance_id=$1 AND monster_instance_id=$2`, c.InstanceID, selected.id, newHealth); err != nil {
						return false, err
					}
					if _, err := c.Tx.ExecContext(ctx, `UPDATE match_map_cells SET monster=jsonb_set(monster,'{health}',to_jsonb($4::int)) WHERE instance_id=$1 AND q=$2 AND r=$3 AND monster IS NOT NULL`, c.InstanceID, selected.pos.Q, selected.pos.R, newHealth); err != nil {
						return false, err
					}
				}
			} else if newHealth == 0 {
				if _, err := c.RecordKillEventTx(ctx, game.KillEvent{SourceType: game.ResultSourcePlayer, KillerID: t.owner, VictimType: "player", VictimID: selected.id, Damage: damage}); err != nil {
					return false, err
				}
				if err := c.MarkCombatPlayerDeadTx(ctx, selected.id); err != nil {
					return false, err
				}
				if err := transferHexQuestToTurretOwnerTx(ctx, c, selected.id, t.owner); err != nil {
					return false, err
				}
				if err := c.LoseHexBackpackTx(ctx, selected.id, selected.pos); err != nil {
					return false, err
				}
				turn, err := c.RemoveCombatPlayerFromTurnOrderTx(ctx, selected.id)
				if err != nil {
					return false, err
				}
				if err := c.ResetHexTurnDeadlineTx(ctx, turn.ActiveUserID); err != nil {
					return false, err
				}
				if _, err := c.Tx.ExecContext(ctx, `UPDATE match_map_cells SET is_player=false WHERE instance_id=$1 AND q=$2 AND r=$3`, c.InstanceID, selected.pos.Q, selected.pos.R); err != nil {
					return false, err
				}
				if turn.ActiveUserID == 0 {
					winnerID, winnerGroup := t.owner, 0
					if ownerGroup > 0 {
						winnerID, winnerGroup = 0, ownerGroup
					}
					if _, err := c.Tx.ExecContext(ctx, `UPDATE matches SET finished_at=now(),hex_turn_deadline=NULL,winner_id=$2,winner_group_id=$3 WHERE instance_id=$1`, c.InstanceID, winnerID, winnerGroup); err != nil {
						return false, err
					}
				}
			} else {
				if _, err := c.Tx.ExecContext(ctx, `UPDATE match_players SET health=$3 WHERE instance_id=$1 AND user_id=$2`, c.InstanceID, selected.id, newHealth); err != nil {
					return false, err
				}
			}
		}
		if _, err := c.Tx.ExecContext(ctx, `UPDATE match_map_cells SET structure_attack=$4,structure_defense=$5,structure_energy=$6 WHERE instance_id=$1 AND q=$2 AND r=$3`, c.InstanceID, t.pos.Q, t.pos.R, t.attack, t.defense, t.energy); err != nil {
			return false, err
		}
		changed = true
	}
	return changed, nil
}

// Square transfers a quest artifact to the player whose turret dealt the kill.
func transferHexQuestToTurretOwnerTx(ctx context.Context, c *MatchCommand, victim, owner int) error {
	var questID int
	if err := c.Tx.QueryRowContext(ctx, `SELECT COALESCE(quest_artifact_id,0) FROM matches WHERE instance_id=$1`, c.InstanceID).Scan(&questID); err != nil {
		return err
	}
	if questID == 0 {
		return nil
	}
	var count int
	if err := c.Tx.QueryRowContext(ctx, `SELECT COALESCE((SELECT item_count FROM inventory_items WHERE instance_id=$1 AND user_id=$2 AND item_type='artifact' AND item_id=$3),0)`, c.InstanceID, victim, questID).Scan(&count); err != nil {
		return err
	}
	if count <= 0 {
		return nil
	}
	key := fmt.Sprintf("artifact_%d", questID)
	var victimRaw, ownerRaw []byte
	if err := c.Tx.QueryRowContext(ctx, `SELECT inventory FROM match_players WHERE instance_id=$1 AND user_id=$2`, c.InstanceID, victim).Scan(&victimRaw); err != nil {
		return err
	}
	if err := c.Tx.QueryRowContext(ctx, `SELECT inventory FROM match_players WHERE instance_id=$1 AND user_id=$2`, c.InstanceID, owner).Scan(&ownerRaw); err != nil {
		return err
	}
	var victimInv, ownerInv map[string]map[string]any
	if err := json.Unmarshal(victimRaw, &victimInv); err != nil {
		return err
	}
	if err := json.Unmarshal(ownerRaw, &ownerInv); err != nil {
		return err
	}
	entry, ok := victimInv[key]
	if !ok {
		return ErrCommandRejected
	}
	delete(victimInv, key)
	if current, ok := ownerInv[key]; ok {
		currentCount, _ := current["item_count"].(float64)
		current["item_count"] = int(currentCount) + count
	} else {
		ownerInv[key] = entry
	}
	victimJSON, err := json.Marshal(victimInv)
	if err != nil {
		return err
	}
	ownerJSON, err := json.Marshal(ownerInv)
	if err != nil {
		return err
	}
	if _, err := c.Tx.ExecContext(ctx, `UPDATE match_players SET inventory=$3 WHERE instance_id=$1 AND user_id=$2`, c.InstanceID, victim, victimJSON); err != nil {
		return err
	}
	if _, err := c.Tx.ExecContext(ctx, `UPDATE match_players SET inventory=$3 WHERE instance_id=$1 AND user_id=$2`, c.InstanceID, owner, ownerJSON); err != nil {
		return err
	}
	if _, err := c.Tx.ExecContext(ctx, `INSERT INTO inventory_items(instance_id,user_id,item_type,item_id,item_name,item_description,item_count,image_url) SELECT instance_id,$3,item_type,item_id,item_name,item_description,item_count,image_url FROM inventory_items WHERE instance_id=$1 AND user_id=$2 AND item_type='artifact' AND item_id=$4 ON CONFLICT(instance_id,user_id,item_type,item_id) DO UPDATE SET item_count=inventory_items.item_count+EXCLUDED.item_count`, c.InstanceID, victim, owner, questID); err != nil {
		return err
	}
	if _, err := c.Tx.ExecContext(ctx, `DELETE FROM inventory_items WHERE instance_id=$1 AND user_id=$2 AND item_type='artifact' AND item_id=$3`, c.InstanceID, victim, questID); err != nil {
		return err
	}
	return nil
}
