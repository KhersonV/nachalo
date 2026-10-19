package handlers

import (
	"context"
	"encoding/json"
	"fmt"

	"gameservice/game"
	"gameservice/repository"
)

// internalMonsterAttackCommand is intentionally not wired to HTTP. Monster
// actions are server initiated, but share the same row lock and command ledger
// as player actions. Callers must provide a stable command ID for retries.
func internalMonsterAttackCommand(ctx context.Context, req AttackRequest) (*atomicAttackResponse, error) {
	if req.AttackerType != "monster" || req.TargetType != "player" || req.CommandID == "" {
		return nil, repository.ErrCommandRejected
	}
	canonical := fmt.Sprintf("internal-monster-attack-v1:%d:%d", req.AttackerID, req.TargetID)
	c, err := repository.BeginMatchCommand(ctx, req.InstanceID, 0, req.CommandID, canonical)
	if err != nil {
		return nil, err
	}
	defer c.Rollback()
	if c.Replay != nil {
		var response atomicAttackResponse
		return &response, json.Unmarshal(c.Replay, &response)
	}
	monster, err := c.MatchCombatMonsterTx(ctx, req.AttackerID)
	if err != nil {
		return nil, err
	}
	target, err := c.MatchCombatPlayerTx(ctx, req.TargetID)
	if err != nil {
		return nil, err
	}
	if monster.Health <= 0 || target.Health <= 0 {
		return nil, repository.ErrCommandRejected
	}
	artifact, err := c.QuestArtifactTx(ctx, req.TargetID)
	if err != nil {
		return nil, err
	}
	if artifact.Name != "" {
		return nil, repository.ErrLegacyAction
	}
	a := stats{Attack: monster.Attack, Defense: monster.Defense, Health: monster.Health, MaxHealth: monster.MaxHealth, AttackRange: 1, X: monster.X, Y: monster.Y}
	d := stats{Attack: target.Attack, Defense: target.Defense, Health: target.Health, MaxHealth: target.MaxHealth, Agility: target.Agility, CharacterType: target.CharacterType, IsRanged: target.IsRanged, AttackRange: target.AttackRange, X: target.Position.X, Y: target.Position.Y}
	mode, err := resolveAttackMode(a, d)
	if err != nil {
		return nil, err
	}
	effectiveTarget := d
	effectiveTarget.Defense = effectiveDefense(req.InstanceID, "player", req.TargetID, d.Defense)
	hit := applyDamage(a, effectiveTarget)
	blocked := resolveGuardianShieldBlock("player", d)
	if blocked {
		hit = attackResult{NewHealth: d.Health}
	}
	if hit.NewHealth <= 0 {
		var survivors int
		if err := c.Tx.QueryRowContext(ctx, `SELECT jsonb_array_length(turn_order) FROM matches WHERE instance_id=$1`, req.InstanceID).Scan(&survivors); err != nil {
			return nil, err
		}
		// Last-player death still requires a finalizer with complete result history.
		if survivors <= 1 {
			return nil, repository.ErrLegacyAction
		}
	}
	monsterRef := CombatTargetRef{ID: req.AttackerID, Type: CombatActorMonster}
	playerRef := CombatTargetRef{ID: req.TargetID, Type: CombatActorPlayer}
	steps := []CombatStep{{Kind: "hit", Source: &monsterRef, Target: playerRef, Damage: hit.Damage, TargetHPAfter: hit.NewHealth}}
	effects := []CombatEffect{}
	if blocked {
		effects = append(effects, CombatEffect{Kind: "block", Target: &playerRef, Succeeded: true})
	}
	counter := attackResult{NewHealth: monster.Health}
	targetEnergy := target.Energy
	counterCost := standardCounterAttackEnergyCost
	if target.CharacterType == "guardian" {
		counterCost = 0
	}
	if hit.NewHealth > 0 && (mode == attackModeMelee || (blocked && mode == attackModeRanged && manhattanDistance(a, d) <= 1)) && targetEnergy >= counterCost {
		targetEnergy -= counterCost
		effectiveMonster := a
		effectiveMonster.Defense = effectiveDefense(req.InstanceID, "monster", req.AttackerID, a.Defense)
		counter = applyDamage(d, effectiveMonster)
		steps = append(steps, CombatStep{Kind: "counter", Source: &playerRef, Target: monsterRef, Damage: counter.Damage, TargetHPAfter: counter.NewHealth})
	}
	mapChanged := false
	nextActive := 0
	if hit.NewHealth > 0 {
		if err := c.SetCombatPlayerVitalsTx(ctx, req.TargetID, hit.NewHealth, targetEnergy); err != nil {
			return nil, err
		}
	} else {
		steps = append(steps, CombatStep{Kind: "death", Target: playerRef})
		turn, _, changed, deathErr := finishCombatPlayerDeathTx(ctx, c, req.TargetID, game.ResultSourceMonster, req.AttackerID, hit.Damage, repository.Position{X: target.Position.X, Y: target.Position.Y})
		if deathErr != nil {
			return nil, deathErr
		}
		nextActive, mapChanged = turn.ActiveUserID, changed
	}
	if counter.Triggered {
		if counter.NewHealth > 0 {
			changed, err := c.SetMonsterHealthAndCellTx(ctx, req.AttackerID, counter.NewHealth)
			if err != nil {
				return nil, err
			}
			mapChanged = mapChanged || changed
		} else {
			cell, changed, err := c.KillMonsterAndClearCellTx(ctx, req.AttackerID)
			if err != nil {
				return nil, err
			}
			mapChanged = mapChanged || changed
			steps = append(steps, CombatStep{Kind: "death", Target: monsterRef})
			if err := c.QueueEvent(map[string]interface{}{"type": "UPDATE_CELL", "payload": map[string]interface{}{"instanceId": req.InstanceID, "updatedCell": serialiseUpdatedCell(*cell)}}); err != nil {
				return nil, err
			}
		}
	}
	for _, event := range []game.DamageEvent{
		{SourceType: game.ResultSourceMonster, DealerID: req.AttackerID, TargetType: "player", TargetID: req.TargetID, Amount: hit.Damage},
		{SourceType: game.ResultSourcePlayer, DealerID: req.TargetID, TargetType: "monster", TargetID: req.AttackerID, Amount: counter.Damage},
	} {
		if err := c.RecordDamageEventTx(ctx, event); err != nil {
			return nil, err
		}
	}
	if counter.Triggered && counter.NewHealth <= 0 {
		if _, err := c.RecordKillEventTx(ctx, game.KillEvent{SourceType: game.ResultSourcePlayer, KillerID: req.TargetID, VictimType: "monster", VictimID: req.AttackerID, Damage: counter.Damage}); err != nil {
			return nil, err
		}
	}
	payload := CombatExchangePayload{InstanceID: req.InstanceID, ExchangeID: req.InstanceID + ":" + req.CommandID, AttackerID: req.AttackerID, AttackerType: monsterRef.Type, TargetID: req.TargetID, TargetType: playerRef.Type, AttackStyle: resolvePresentationAttackStyle(a, mode), Steps: steps, Effects: effects}
	if err := c.QueueEvent(map[string]interface{}{"type": "COMBAT_EXCHANGE", "payload": payload}); err != nil {
		return nil, err
	}
	response := &atomicAttackResponse{DamageToTarget: hit.Damage, NewTargetHP: hit.NewHealth, CounterDamage: counter.Damage, NewAttackerHP: counter.NewHealth, AttackMode: mode}
	state, mapRevision, err := c.Commit(ctx, response, mapChanged)
	if err != nil {
		return nil, err
	}
	if ms, ok := game.GetMatchState(req.InstanceID); ok {
		if hit.Damage > 0 {
			ms.RecordDamageEvent(game.ResultSourceMonster, req.AttackerID, "player", req.TargetID, hit.Damage)
		}
		if counter.Damage > 0 {
			ms.RecordDamageEvent(game.ResultSourcePlayer, req.TargetID, "monster", req.AttackerID, counter.Damage)
		}
		if hit.NewHealth <= 0 {
			ms.RecordKillEvent(game.ResultSourceMonster, req.AttackerID, "player", req.TargetID, hit.Damage)
			ms.RecordPlayerDefeat(req.TargetID)
			ms.RemovePlayerFromTurnOrder(req.TargetID)
			ms.AdvanceTurnCombatState(nextActive, false)
		}
		if counter.Triggered && counter.NewHealth <= 0 {
			ms.RecordKillEvent(game.ResultSourcePlayer, req.TargetID, "monster", req.AttackerID, counter.Damage)
		}
	}
	if hit.NewHealth <= 0 {
		unbindPlayer(req.TargetID)
		startTurnTimer(req.InstanceID, nextActive)
	}
	changes := append([][]byte(nil), c.Events...)
	if player, err := repository.GetMatchPlayerByID(req.InstanceID, req.TargetID); err == nil {
		update, _ := json.Marshal(map[string]interface{}{"type": "UPDATE_PLAYER", "payload": map[string]interface{}{"instanceId": req.InstanceID, "player": player}})
		changes = append(changes, update)
	}
	patch, _ := json.Marshal(map[string]interface{}{"type": "MATCH_PATCH", "payload": map[string]interface{}{"instanceId": req.InstanceID, "schemaVersion": 1, "stateRevision": state, "mapRevision": mapRevision, "commandId": req.CommandID, "changes": changes}})
	Broadcast(patch)
	for _, event := range changes {
		var msg struct {
			Type    string                 `json:"type"`
			Payload map[string]interface{} `json:"payload"`
		}
		if json.Unmarshal(event, &msg) == nil {
			msg.Payload["commandId"] = req.CommandID
			legacy, _ := json.Marshal(msg)
			Broadcast(legacy)
		}
	}
	return response, nil
}
