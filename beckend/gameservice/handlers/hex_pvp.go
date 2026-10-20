package handlers

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"math"
	"time"

	"gameservice/game"
	"gameservice/internal/game/balance"
	"gameservice/internal/hexgrid"
	"gameservice/repository"
)

func PreviewHexPlayerAttack(ctx context.Context, instanceID string, userID, targetID int) (HexAttackPreview, error) {
	var out HexAttackPreview
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	tx, err := repository.DB.BeginTx(ctx, &sql.TxOptions{ReadOnly: true, Isolation: sql.LevelRepeatableRead})
	if err != nil {
		return out, err
	}
	defer tx.Rollback()
	state, err := readHexAttackTx(ctx, tx, instanceID, userID, targetID, true, "player")
	if err != nil {
		return out, err
	}
	if err := tx.Commit(); err != nil {
		return out, err
	}
	return HexAttackPreview{TargetID: targetID, Distance: state.Distance, Cost: state.Cost, Mode: state.Mode, Energy: state.Player.Energy, TargetHP: state.Monster.Stats.Health}, nil
}

func pushHexPlayerTx(ctx context.Context, c *repository.MatchCommand, from, target hexgrid.Coord, targetID int) (bool, hexgrid.Coord, error) {
	a, _ := hexgrid.ToWorld(from, 1)
	b, _ := hexgrid.ToWorld(target, 1)
	best, score := 0, math.Inf(-1)
	for i, direction := range hexgrid.Directions {
		v, _ := hexgrid.ToWorld(direction, 1)
		if dot := (b.X-a.X)*v.X + (b.Z-a.Z)*v.Z; dot > score {
			best, score = i, dot
		}
	}
	dest, err := hexgrid.Neighbor(target, best)
	if err != nil {
		return false, dest, nil
	}
	var tile int
	var player, portal, construction bool
	var structure string
	var resource, barrel, monster []byte
	err = c.Tx.QueryRowContext(ctx, `SELECT tile_code,is_player,is_portal,is_under_construction,structure_type,resource,barbel,monster FROM match_map_cells WHERE instance_id=$1 AND q=$2 AND r=$3`, c.InstanceID, dest.Q, dest.R).Scan(&tile, &player, &portal, &construction, &structure, &resource, &barrel, &monster)
	if err == sql.ErrNoRows {
		return false, dest, nil
	}
	if err != nil {
		return false, dest, err
	}
	if (tile != 48 && tile != 82) || player || portal || construction || structure != "" || len(resource) > 0 || len(barrel) > 0 || len(monster) > 0 {
		return false, dest, nil
	}
	var occupied bool
	if err := c.Tx.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM match_monsters WHERE instance_id=$1 AND pos_q=$2 AND pos_r=$3 AND health>0)`, c.InstanceID, dest.Q, dest.R).Scan(&occupied); err != nil {
		return false, dest, err
	}
	if occupied {
		return false, dest, nil
	}
	edges, err := c.Tx.QueryContext(ctx, `SELECT q,r,direction FROM match_hex_edges WHERE instance_id=$1`, c.InstanceID)
	if err != nil {
		return false, dest, err
	}
	blocked := false
	wanted, _ := hexgrid.CanonicalEdge(target, dest)
	for edges.Next() {
		var p hexgrid.Coord
		var d int
		if err := edges.Scan(&p.Q, &p.R, &d); err != nil {
			edges.Close()
			return false, dest, err
		}
		n, err := hexgrid.Neighbor(p, d)
		if err != nil {
			edges.Close()
			return false, dest, err
		}
		key, _ := hexgrid.CanonicalEdge(p, n)
		if key == wanted {
			blocked = true
		}
	}
	if err := edges.Err(); err != nil {
		edges.Close()
		return false, dest, err
	}
	edges.Close()
	if blocked {
		return false, dest, nil
	}
	result, err := c.Tx.ExecContext(ctx, `UPDATE match_players SET position_q=$3,position_r=$4 WHERE instance_id=$1 AND user_id=$2 AND position_q=$5 AND position_r=$6 AND health>0`, c.InstanceID, targetID, dest.Q, dest.R, target.Q, target.R)
	if err != nil {
		return false, dest, err
	}
	if n, _ := result.RowsAffected(); n != 1 {
		return false, dest, repository.ErrCommandRejected
	}
	if _, err := c.Tx.ExecContext(ctx, `UPDATE match_map_cells SET is_player=false WHERE instance_id=$1 AND q=$2 AND r=$3`, c.InstanceID, target.Q, target.R); err != nil {
		return false, dest, err
	}
	if _, err := c.Tx.ExecContext(ctx, `UPDATE match_map_cells SET is_player=true WHERE instance_id=$1 AND q=$2 AND r=$3`, c.InstanceID, dest.Q, dest.R); err != nil {
		return false, dest, err
	}
	if err := repository.RecordHexExploredForPush(ctx, c.Tx, c.InstanceID, targetID, target, dest); err != nil {
		return false, dest, err
	}
	return true, dest, nil
}

func finishHexPvPDeathTx(ctx context.Context, c *repository.MatchCommand, victimID, killerID, damage int, pos hexgrid.Coord) error {
	if _, err := c.RecordKillEventTx(ctx, game.KillEvent{SourceType: game.ResultSourcePlayer, KillerID: killerID, VictimType: "player", VictimID: victimID, Damage: damage}); err != nil {
		return err
	}
	if err := c.MarkCombatPlayerDeadTx(ctx, victimID); err != nil {
		return err
	}
	if err := c.LoseHexBackpackTx(ctx, victimID, pos); err != nil {
		return err
	}
	turn, err := c.RemoveCombatPlayerFromTurnOrderTx(ctx, victimID)
	if err != nil {
		return err
	}
	if c.ActiveUser == victimID {
		if err := c.ResetHexTurnDeadlineTx(ctx, turn.ActiveUserID); err != nil {
			return err
		}
	}
	result, err := c.Tx.ExecContext(ctx, `UPDATE match_map_cells SET is_player=false WHERE instance_id=$1 AND q=$2 AND r=$3 AND is_player=true`, c.InstanceID, pos.Q, pos.R)
	if err != nil {
		return err
	}
	if n, _ := result.RowsAffected(); n != 1 {
		return repository.ErrCommandRejected
	}
	if turn.ActiveUserID == 0 {
		_, err = c.Tx.ExecContext(ctx, `UPDATE matches SET finished_at=now(),hex_turn_deadline=NULL WHERE instance_id=$1`, c.InstanceID)
	}
	return err
}

func ApplyHexPlayerAttack(ctx context.Context, instanceID string, userID, targetID int, commandID string, publish func(HexAttackResult)) (HexAttackResult, error) {
	var out HexAttackResult
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	c, err := repository.BeginMatchCommand(ctx, instanceID, userID, commandID, fmt.Sprintf("hex-pvp-v1:%d:%d", userID, targetID))
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
	s, err := readHexAttackTx(ctx, c.Tx, instanceID, userID, targetID, true, "player")
	if err != nil {
		return out, err
	}
	a, d := s.Player.Stats, s.Monster.Stats
	d.Defense -= s.Monster.ArmorBreakStacks * armorBreakDefensePenaltyPerStack
	if d.Defense < 0 {
		d.Defense = 0
	}
	hit := applyDamage(a, d)
	attackerRef, targetRef := CombatTargetRef{ID: userID, Type: CombatActorPlayer}, CombatTargetRef{ID: targetID, Type: CombatActorPlayer}
	effects := []CombatEffect{}
	if resolveGuardianShieldBlock("player", s.Monster.Stats) {
		hit = attackResult{NewHealth: s.Monster.Stats.Health}
		effects = append(effects, CombatEffect{Kind: "block", Target: &targetRef, Succeeded: true})
	}
	if a.CharacterType == "ranger" {
		var crit bool
		hit, crit = tryApplyRangerCriticalShot(a, s.Monster.Stats.Health, hit)
		if crit {
			effects = append(effects, CombatEffect{Kind: "crit", Source: &attackerRef, Target: &targetRef, Succeeded: true})
		}
	}
	steps := []CombatStep{{Kind: "hit", Source: &attackerRef, Target: targetRef, Damage: hit.Damage, TargetHPAfter: hit.NewHealth}}
	attackerEnergy, targetEnergy := s.Player.Energy-s.Cost, s.Monster.Energy
	attackerHP, targetHP := a.Health, hit.NewHealth
	bonus, followup, counterDamage := 0, 0, 0
	if a.CharacterType == "berserker" && hit.Damage > 0 && rollReflexProc(balance.CalculateReflexProcChance(a.Agility)) {
		heal := actualDamageDealt(s.Monster.Stats.Health, targetHP) / 2
		if heal > a.MaxHealth-attackerHP {
			heal = a.MaxHealth - attackerHP
		}
		if heal > 0 {
			attackerHP += heal
			effects = append(effects, CombatEffect{Kind: "lifesteal", Source: &attackerRef, Target: &attackerRef, Amount: heal, Succeeded: true})
		}
	}
	if a.CharacterType == "mystic" && hit.Triggered && targetHP > 0 {
		burn, gain := energyDrainPerHit, energyDrainGainPerHit
		if rollReflexProc(balance.CalculateReflexProcChance(a.Agility)) {
			burn, gain = arcaneOverburnEnergyBurn, arcaneOverburnEnergyGain
			effects = append(effects, CombatEffect{Kind: "arcaneOverburn", Source: &attackerRef, Target: &targetRef, Succeeded: true})
		}
		actual := burn
		if actual > targetEnergy {
			actual = targetEnergy
		}
		targetEnergy -= actual
		if gain > s.Player.MaxEnergy-attackerEnergy {
			gain = s.Player.MaxEnergy - attackerEnergy
		}
		if gain < 0 {
			gain = 0
		}
		attackerEnergy += gain
		bonus = burn - actual
		if bonus > 0 {
			next := applyFlatDamage(targetHP, bonus)
			bonus = actualDamageDealt(targetHP, next.NewHealth)
			targetHP = next.NewHealth
			steps = append(steps, CombatStep{Kind: "bonus", Source: &attackerRef, Target: targetRef, Damage: bonus, TargetHPAfter: targetHP})
		}
		effects = append(effects, CombatEffect{Kind: "energyDrain", Source: &attackerRef, Target: &targetRef, Amount: actual, EnergyGranted: gain, EnergyDrained: actual, Succeeded: true})
	}
	counterCost := standardCounterAttackEnergyCost
	if d.CharacterType == "guardian" {
		counterCost = 0
	}
	if s.Mode == attackModeMelee && targetHP > 0 && targetEnergy >= counterCost {
		targetEnergy -= counterCost
		counterAttacker := s.Monster.Stats
		counterAttacker.Health = targetHP
		counterDefender := a
		counterDefender.Health = attackerHP
		counterDefender.Defense -= s.Player.ArmorBreakStacks * armorBreakDefensePenaltyPerStack
		if counterDefender.Defense < 0 {
			counterDefender.Defense = 0
		}
		counter := applyDamage(counterAttacker, counterDefender)
		counterDamage = counter.Damage
		attackerHP = counter.NewHealth
		steps = append(steps, CombatStep{Kind: "counter", Source: &targetRef, Target: attackerRef, Damage: counterDamage, TargetHPAfter: attackerHP})
		if attackerHP > 0 && targetHP > 0 && a.CharacterType == "berserker" && hit.Damage > 0 {
			next := applyFlatDamage(targetHP, hit.Damage/2)
			followup = next.Damage
			targetHP = next.NewHealth
			steps = append(steps, CombatStep{Kind: "followup", Source: &attackerRef, Target: targetRef, Damage: followup, TargetHPAfter: targetHP})
		}
	}
	mapChanged := false
	if a.CharacterType == "ranger" && s.Mode == attackModeRanged && hit.Triggered && targetHP > 0 {
		if s.Monster.ArmorBreakStacks >= armorBreakMaxStacks {
			attackerEnergy += rangerThirdShotEnergyRefund
			if attackerEnergy > s.Player.MaxEnergy {
				attackerEnergy = s.Player.MaxEnergy
			}
			pushed, pos, err := pushHexPlayerTx(ctx, c, s.Player.Position, s.Monster.Position, targetID)
			if err != nil {
				return out, err
			}
			if pushed {
				mapChanged = true
				effects = append(effects, CombatEffect{Kind: "push", Source: &attackerRef, Target: &targetRef, Succeeded: true, PositionAfter: &CombatPoint{X: pos.Q, Y: pos.R}})
			} else {
				next := applyFlatDamage(targetHP, hit.Damage/2)
				bonus += next.Damage
				targetHP = next.NewHealth
				effects = append(effects, CombatEffect{Kind: "push", Source: &attackerRef, Target: &targetRef, Succeeded: false, BonusDamage: next.Damage})
			}
			if _, err := c.Tx.ExecContext(ctx, `UPDATE match_players SET hex_armor_break_stacks=0,hex_armor_break_turns=0 WHERE instance_id=$1 AND user_id=$2`, instanceID, targetID); err != nil {
				return out, err
			}
		} else {
			stacks := s.Monster.ArmorBreakStacks + 1
			if _, err := c.Tx.ExecContext(ctx, `UPDATE match_players SET hex_armor_break_stacks=$3,hex_armor_break_turns=$4 WHERE instance_id=$1 AND user_id=$2`, instanceID, targetID, stacks, armorBreakDurationTurns); err != nil {
				return out, err
			}
			effects = append(effects, CombatEffect{Kind: "armorBreak", Source: &attackerRef, Target: &targetRef, Value: -armorBreakDefensePenaltyPerStack, Stacks: stacks, DurationTurns: armorBreakDurationTurns, Succeeded: true})
		}
	}
	if attackerHP > 0 {
		if err := c.SetCombatPlayerVitalsTx(ctx, userID, attackerHP, attackerEnergy); err != nil {
			return out, err
		}
	} else {
		if err := c.SetCombatPlayerVitalsTx(ctx, userID, a.Health, attackerEnergy); err != nil {
			return out, err
		}
	}
	if targetHP > 0 {
		if err := c.SetCombatPlayerVitalsTx(ctx, targetID, targetHP, targetEnergy); err != nil {
			return out, err
		}
	}
	for _, event := range []game.DamageEvent{{SourceType: game.ResultSourcePlayer, DealerID: userID, TargetType: "player", TargetID: targetID, Amount: hit.Damage}, {SourceType: game.ResultSourcePlayer, DealerID: userID, TargetType: "player", TargetID: targetID, Amount: bonus}, {SourceType: game.ResultSourcePlayer, DealerID: userID, TargetType: "player", TargetID: targetID, Amount: followup}, {SourceType: game.ResultSourcePlayer, DealerID: targetID, TargetType: "player", TargetID: userID, Amount: counterDamage}} {
		if err := c.RecordDamageEventTx(ctx, event); err != nil {
			return out, err
		}
	}
	if targetHP <= 0 {
		steps = append(steps, CombatStep{Kind: "death", Target: targetRef})
		if err := finishHexPvPDeathTx(ctx, c, targetID, userID, hit.Damage+bonus+followup, s.Monster.Position); err != nil {
			return out, err
		}
		mapChanged = true
	}
	if attackerHP <= 0 {
		steps = append(steps, CombatStep{Kind: "death", Target: attackerRef})
		if err := finishHexPvPDeathTx(ctx, c, userID, targetID, counterDamage, s.Player.Position); err != nil {
			return out, err
		}
		mapChanged = true
	}
	out = HexAttackResult{HexAttackPreview: HexAttackPreview{TargetID: targetID, Distance: s.Distance, Cost: s.Cost, Mode: s.Mode, Energy: attackerEnergy, TargetHP: s.Monster.Stats.Health}, Damage: hit.Damage + bonus + followup, CounterDamage: counterDamage, NewTargetHP: targetHP, NewAttackerHP: attackerHP, PlayerDied: targetHP <= 0, AttackStyle: resolvePresentationAttackStyle(a, s.Mode), Effects: effects, Steps: steps, Drops: []EquipmentDropPayload{}}
	out.StateRevision, out.MapRevision, err = c.Commit(ctx, &out, mapChanged)
	if err != nil {
		return HexAttackResult{}, err
	}
	if publish != nil {
		publish(out)
	}
	return out, nil
}
