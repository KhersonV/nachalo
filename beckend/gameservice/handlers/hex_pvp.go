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
	return HexAttackPreview{TargetID: targetID, Distance: state.Distance, Cost: state.Cost, Mode: state.Mode, AP: state.Player.AP, TargetHP: state.Monster.Stats.Health}, nil
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
	if err := repository.ClearHexGuardianAuraAfterForcedMoveTx(ctx, c.Tx, c.InstanceID, targetID, target, dest); err != nil {
		return false, dest, err
	}
	c.RecordAuraMovement(targetID, repository.AuraMovementForced)
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
		var groupID int
		if err := c.Tx.QueryRowContext(ctx, `SELECT COALESCE(group_id,0) FROM match_players WHERE instance_id=$1 AND user_id=$2`, c.InstanceID, killerID).Scan(&groupID); err != nil {
			return err
		}
		winnerID, winnerGroup := killerID, 0
		if groupID > 0 {
			winnerID, winnerGroup = 0, groupID
		}
		_, err = c.Tx.ExecContext(ctx, `UPDATE matches SET finished_at=now(),hex_turn_deadline=NULL,winner_id=$2,winner_group_id=$3 WHERE instance_id=$1`, c.InstanceID, winnerID, winnerGroup)
	}
	return err
}

func resolveHexRangerPlayerPushFallback(attacker, target stats, currentHealth int) attackResult {
	target.Health = currentHealth
	return applyDamage(attacker, target)
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
	c.CombatTargetKind, c.CombatTargetID = "player", targetID
	if c.Replay != nil {
		if err := json.Unmarshal(c.Replay, &out); err != nil {
			return out, err
		}
		out.Replayed = true
		return out, nil
	}
	out, err = applyHexPlayerAttackTx(ctx, c, userID, targetID)
	if err != nil {
		return out, err
	}
	out.StateRevision, out.MapRevision, err = c.Commit(ctx, &out, c.MapChanged)
	if err != nil {
		return HexAttackResult{}, err
	}
	if publish != nil {
		publish(out)
	}
	return out, nil
}

// Uses the caller's locked MatchCommand; no nested transaction or second commit.
func applyHexPlayerAttackTx(ctx context.Context, c *repository.MatchCommand, userID, targetID int) (HexAttackResult, error) {
	var out HexAttackResult
	instanceID := c.InstanceID
	s, err := readHexAttackTx(ctx, c.Tx, instanceID, userID, targetID, true, "player")
	if err != nil {
		return out, err
	}
	if err := c.PrepareHexMainTargetTx(ctx, userID, "player", targetID); err != nil {
		return out, err
	}
	keepKind, keepID := "", 0
	if s.Player.Stats.CharacterType == "ranger" && s.Mode == attackModeRanged {
		keepKind, keepID = "player", targetID
	}
	if err := c.ClearRangerBreakTx(ctx, userID, keepKind, keepID); err != nil {
		return out, err
	}
	if s.Monster.ArmorBreakOwner != userID {
		s.Monster.ArmorBreakStacks = 0
	}
	profile, err := repository.ReadHexCombatProfileTx(ctx, c.Tx, instanceID, userID)
	if err != nil {
		return out, err
	}
	a, d := s.Player.Stats, s.Monster.Stats
	roll:=combatRoll(c)
	if a.CharacterType == "ranger" && s.Mode == attackModeRanged {
		penalty:=armorBreakDefensePenaltyPerStack
		if c.CombatConfig!=nil {penalty=balance.ResearchRound(c.CombatConfig.Factors["armor_break"])}
		d.Defense -= s.Monster.ArmorBreakStacks * penalty
	}
	if d.Defense < 0 {
		d.Defense = 0
	}
	hit := applyDamage(a, d)
	attackerRef, targetRef := CombatTargetRef{ID: userID, Type: CombatActorPlayer}, CombatTargetRef{ID: targetID, Type: CombatActorPlayer}
	effects := []CombatEffect{}
	blockChance:=classReflexChance(d,a)
	if c.CombatConfig!=nil {blockChance=balance.ResearchRound(float64(blockChance)*c.CombatConfig.Factors["block"]);if blockChance>55 {blockChance=55}}
	blocked := false
	if d.CharacterType == "guardian" && roll(blockChance) {
		blocked = true
		hit = attackResult{NewHealth: s.Monster.Stats.Health}
		effects = append(effects, CombatEffect{Kind: "block", Target: &targetRef, Succeeded: true})
		if a.CharacterType == "mystic" && (c.CombatConfig == nil || !c.CombatConfig.MysticMagicThroughBlock) {
			if err := c.ResetHexOverloadTx(ctx, userID); err != nil {
				return out, err
			}
		}
	}
	rangerChance := 0
	if a.CharacterType == "ranger" {
		rangerChance = classReflexChance(a,d)
	}
	if hit.Triggered && hit.Damage > 0 {
		damage, crit, e := c.CraftCriticalTx(ctx, userID, hit.Damage, rangerChance, roll, profile.CriticalPercent)
		if e != nil {
			return out, e
		}
		hit = applyFlatDamage(s.Monster.Stats.Health, damage)
		if crit {
			effects = append(effects, CombatEffect{Kind: "crit", Source: &attackerRef, Target: &targetRef, Succeeded: true})
		}
	}
	// Scale only the resolved primary hit. Counters, follow-ups and push fallback
	// do not receive this matrix; healing still uses actual primary HP removed.
	followupBase := hit.Damage
	if hit.Triggered {
		if c.CombatConfig==nil {hit = applyFlatDamage(d.Health, balance.PvPPrimaryDamage(hit.Damage, a.CharacterType, d.CharacterType, profile.GearLevel))
		} else {
			var extra []CombatEffect
			hit,extra,err=incomingCombatTx(ctx,c,attackerRef,targetRef,a,s.Monster.Stats,d.Health,hit.Damage,"basic",s.Distance)
			if err!=nil {return out,err};effects=append(effects,extra...)
		}
	}
	steps := []CombatStep{{Kind: "hit", Source: &attackerRef, Target: targetRef, Damage: hit.Damage, TargetHPAfter: hit.NewHealth}}
	attackerAP, targetAP := s.Player.AP-s.Cost, s.Monster.AP
	attackerHP, targetHP := a.Health, hit.NewHealth
	bonus, followup, counterDamage := 0, 0, 0
	heal, err := c.CraftLifestealTx(ctx, userID, actualDamageDealt(s.Monster.Stats.Health, targetHP), a.CharacterType == "berserker" && hit.Damage > 0 && roll(classReflexChance(a,d)), roll, profile.HealPercent)
	if err != nil {
		return out, err
	}
	if heal > 0 {
		if heal > a.MaxHealth-attackerHP {
			heal = a.MaxHealth - attackerHP
		}
		if heal > 0 {
			attackerHP += heal
			effects = append(effects, CombatEffect{Kind: "lifesteal", Source: &attackerRef, Target: &attackerRef, Amount: heal, Succeeded: true})
		}
	}
	if a.CharacterType == "mystic" && mysticAttackConnects(c.CombatConfig, hit.Triggered, blocked, effects) {
		proc:=false
		if c.CombatConfig==nil {proc=roll(classReflexChance(a,d))}
		extra, reflex, err := c.MysticHitTx(ctx, userID, "player", targetID, proc)
		if err != nil {
			return out, err
		}
		if targetHP > 0 && extra+reflex > 0 {
			next,additional,e := incomingCombatTx(ctx,c,attackerRef,targetRef,a,d,targetHP,extra+reflex,"overload",s.Distance)
			if e!=nil {return out,e};effects=append(effects,additional...)
			targetHP = next.NewHealth
			bonus += next.Damage
			steps = append(steps, CombatStep{Kind: "mysticOverload", Source: &attackerRef, Target: targetRef, Damage: next.Damage, TargetHPAfter: targetHP})
			effects = append(effects, CombatEffect{Kind: "mysticOverload", Source: &attackerRef, Target: &targetRef, Amount: extra, BonusDamage: reflex, Succeeded: true})
		}
		if targetHP > 0 {
			applied, err := c.MysticDrainTx(ctx, userID, targetID)
			if err != nil {
				return out, err
			}
			if applied {
				effects = append(effects, CombatEffect{Kind: "mpDrain", Source: &attackerRef, Target: &targetRef, Succeeded: true})
			}
		}
	}
	if s.Mode == attackModeMelee && targetHP > 0 && s.Monster.Reactions > 0 {
		if err := c.SpendHexTx(ctx, targetID, 0, 0, 1); err != nil {
			return out, err
		}
		counterAttacker := s.Monster.Stats
		counterAttacker.Health = targetHP
		counterDefender := a
		counterDefender.Health = attackerHP
		// Only a Ranger main attack can use its owned armor break.
		if counterDefender.Defense < 0 {
			counterDefender.Defense = 0
		}
		counterRaw:=resolveCounterDamage(counterAttacker,counterDefender)
		counter,additional,e := incomingCombatTx(ctx,c,targetRef,attackerRef,counterAttacker,counterDefender,attackerHP,counterRaw.Damage,"counter",s.Distance)
		if e!=nil {return out,e};effects=append(effects,additional...)
		counterDamage = counter.Damage
		attackerHP = counter.NewHealth
		steps = append(steps, CombatStep{Kind: "counter", Source: &targetRef, Target: attackerRef, Damage: counterDamage, TargetHPAfter: attackerHP})
		if attackerHP > 0 && targetHP > 0 && a.CharacterType == "berserker" && (hit.Damage > 0 || c.CombatConfig!=nil && followupBase>0) {
			followDamage:=followupBase/2+profile.FollowupDamage
			if c.CombatConfig!=nil {followDamage=balance.ResearchRound(float64(followupBase)*.5*c.CombatConfig.Factors["followup"])}
			next,additional,e := incomingCombatTx(ctx,c,attackerRef,targetRef,a,d,targetHP,followDamage,"followup",s.Distance)
			if e!=nil {return out,e};effects=append(effects,additional...)
			followup = next.Damage
			targetHP = next.NewHealth
			steps = append(steps, CombatStep{Kind: "followup", Source: &attackerRef, Target: targetRef, Damage: followup, TargetHPAfter: targetHP})
		}
	}
	mapChanged := false
	targetImmune, err := c.CraftImmuneTx(ctx, targetID)
	if err != nil {
		return out, err
	}
	if a.CharacterType == "ranger" && s.Mode == attackModeRanged && hit.Triggered && targetHP > 0 && !targetImmune {
		if s.Monster.ArmorBreakStacks >= armorBreakMaxStacks {
			granted := 2
			b, err := repository.ReadHexBudgetsTx(ctx, c.Tx, instanceID, userID)
			if err != nil {
				return out, err
			}
			if granted > repository.HexMPCap-b.MP {
				granted = repository.HexMPCap - b.MP
			}
			if err := c.GrantHexMPTx(ctx, userID, granted); err != nil {
				return out, err
			}
			effects = append(effects, CombatEffect{Kind: "mpGain", Source: &attackerRef, MPGranted: granted, Succeeded: true})
			pushed, pos, err := pushHexPlayerTx(ctx, c, s.Player.Position, s.Monster.Position, targetID)
			if err != nil {
				return out, err
			}
			if pushed {
				mapChanged = true
				effects = append(effects, CombatEffect{Kind: "push", Source: &attackerRef, Target: &targetRef, Succeeded: true, PositionAfter: &CombatPoint{X: pos.Q, Y: pos.R}})

			} else {
				collisionTarget:=d
				if c.CombatConfig!=nil {collisionTarget.Defense=s.Monster.Stats.Defense-4;if collisionTarget.Defense<0 {collisionTarget.Defense=0}}
				collision:=resolveHexRangerPlayerPushFallback(a, collisionTarget, targetHP)
				next,additional,e := incomingCombatTx(ctx,c,attackerRef,targetRef,a,s.Monster.Stats,targetHP,collision.Damage,"collision",s.Distance)
				if e!=nil {return out,e};effects=append(effects,additional...)
				bonus += next.Damage
				targetHP = next.NewHealth
				steps = append(steps, CombatStep{Kind: "bonus", Source: &attackerRef, Target: targetRef, Damage: next.Damage, TargetHPAfter: targetHP})
				effects = append(effects, CombatEffect{Kind: "push", Source: &attackerRef, Target: &targetRef, Succeeded: false, BonusDamage: next.Damage})
			}
			if err := c.SetRangerBreakTx(ctx, "player", targetID, userID, 0); err != nil {
				return out, err
			}
			endEffect := CombatEffect{Kind: "armorBreakEnd", Target: &targetRef, Succeeded: true}
			if pushed {
				endEffect.PositionAfter = &CombatPoint{X: pos.Q, Y: pos.R}
			}
			effects = append(effects, endEffect)
		} else {
			stacks := s.Monster.ArmorBreakStacks + 1
			if err := c.SetRangerBreakTx(ctx, "player", targetID, userID, stacks); err != nil {
				return out, err
			}
			penalty:=armorBreakDefensePenaltyPerStack
			if c.CombatConfig!=nil {penalty=balance.ResearchRound(c.CombatConfig.Factors["armor_break"])}
			effects = append(effects, CombatEffect{Kind: "armorBreak", Source: &attackerRef, Target: &targetRef, Value: -penalty, Stacks: stacks, Succeeded: true})
		}
	}
	if attackerHP > 0 {
		if err := c.SetHexCombatVitalsTx(ctx, userID, attackerHP, attackerAP); err != nil {
			return out, err
		}
	} else {
		if err := c.SetHexCombatVitalsTx(ctx, userID, a.Health, attackerAP); err != nil {
			return out, err
		}
	}
	if targetHP > 0 {
		if err := c.SetHexCombatVitalsTx(ctx, targetID, targetHP, targetAP); err != nil {
			return out, err
		}
	}
	for _, event := range hexDamageEventsFromSteps(steps, effects, map[CombatTargetRef]int{attackerRef: s.Player.Stats.Health, targetRef: s.Monster.Stats.Health}) {
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
	out = HexAttackResult{HexAttackPreview: HexAttackPreview{TargetID: targetID, Distance: s.Distance, Cost: s.Cost, Mode: s.Mode, AP: attackerAP, TargetHP: s.Monster.Stats.Health}, Damage: hit.Damage + bonus + followup, CounterDamage: counterDamage, NewTargetHP: targetHP, NewAttackerHP: attackerHP, PlayerDied: targetHP <= 0, AttackStyle: resolvePresentationAttackStyle(a, s.Mode), Effects: effects, Steps: steps, Drops: []EquipmentDropPayload{}, AttackerPosition: s.Player.Position, TargetPosition: s.Monster.Position, AttackerHPBefore: a.Health}
	c.MapChanged = c.MapChanged || mapChanged
	return out, nil
}
