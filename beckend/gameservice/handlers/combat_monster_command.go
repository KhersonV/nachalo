package handlers

import (
	"encoding/json"
	"net/http"

	"gameservice/game"
	"gameservice/internal/game/balance"
	"gameservice/repository"
)

// tryAtomicNonlethalPvE handles supported monster deaths and counterattacks
// under one MatchCommand. Ranger third shots and quest artifacts remain legacy.
func tryAtomicNonlethalPvE(w http.ResponseWriter, r *http.Request, req AttackRequest) bool {
	if req.AttackerType != "player" || req.TargetType != "monster" {
		return false
	}
	c, err := repository.BeginMatchCommand(r.Context(), req.InstanceID, req.AttackerID, req.CommandID, combatCommandCanonical(req, "nonlethal-pve-v1"))
	if err != nil {
		writeCombatCommandError(w, err)
		return true
	}
	defer c.Rollback()
	if c.Replay != nil {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write(c.Replay)
		return true
	}
	if c.ActiveUser != req.AttackerID {
		http.Error(w, game.ErrNotYourTurn.Error(), http.StatusBadRequest)
		return true
	}
	attacker, err := c.MatchCombatPlayerTx(r.Context(), req.AttackerID)
	if err != nil {
		writeCombatCommandError(w, err)
		return true
	}
	if attacker.CharacterType != "guardian" && attacker.CharacterType != "mystic" && attacker.CharacterType != "berserker" && attacker.CharacterType != "ranger" {
		return false
	}
	artifact, err := c.QuestArtifactTx(r.Context(), req.AttackerID)
	if err != nil {
		writeCombatCommandError(w, err)
		return true
	}
	if artifact.Name != "" {
		return false
	}
	monster, err := c.MatchCombatMonsterTx(r.Context(), req.TargetID)
	if err != nil {
		writeCombatCommandError(w, err)
		return true
	}
	if req.AutoAttack && (req.MoveX < 0 || req.MoveY < 0 || req.MoveX >= c.Width || req.MoveY >= c.Height || monster.X != req.MoveX || monster.Y != req.MoveY) {
		http.Error(w, "target moved", http.StatusBadRequest)
		return true
	}
	if attacker.Health <= 0 || monster.Health <= 0 {
		http.Error(w, "actor defeated", http.StatusBadRequest)
		return true
	}
	a := stats{Attack: attacker.Attack, Defense: attacker.Defense, Health: attacker.Health, MaxHealth: attacker.MaxHealth, Agility: attacker.Agility, CharacterType: attacker.CharacterType, IsRanged: attacker.IsRanged, AttackRange: attacker.AttackRange, X: attacker.Position.X, Y: attacker.Position.Y}
	d := stats{Attack: monster.Attack, Defense: monster.Defense, Health: monster.Health, MaxHealth: monster.MaxHealth, AttackRange: 1, X: monster.X, Y: monster.Y}
	mode, err := resolveAttackMode(a, d)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return true
	}
	if attacker.Energy < resolveAttackEnergyCost(mode) {
		http.Error(w, "Недостаточно энергии для атаки", http.StatusBadRequest)
		return true
	}
	effectiveMonster := d
	effectiveMonster.Defense = effectiveDefense(req.InstanceID, "monster", req.TargetID, d.Defense)
	if attacker.CharacterType == "mystic" {
		effectiveMonster.Defense = effectiveMonster.Defense * mysticMonsterDefensePercent / 100
	}
	hit := applyDamage(a, effectiveMonster)
	preArmorBreak := game.ArmorBreakState{}
	if ms, ok := game.GetMatchState(req.InstanceID); ok {
		preArmorBreak = ms.GetArmorBreakState("monster", req.TargetID)
	}
	if attacker.CharacterType == "ranger" {
		if mode == attackModeRanged && preArmorBreak.Stacks >= armorBreakMaxStacks {
			return false
		}
	}
	counterDamage := 0
	if mode == attackModeMelee && hit.NewHealth > 0 {
		effectiveAttacker := a
		effectiveAttacker.Defense = effectiveDefense(req.InstanceID, "player", req.AttackerID, a.Defense)
		counterDamage = applyDamage(d, effectiveAttacker).Damage
		if attacker.CharacterType == "berserker" && counterDamage > 0 {
			counterDamage = counterDamage * berserkerMonsterCounterPercent / 100
			if counterDamage < 1 {
				counterDamage = 1
			}
		}
		if a.Health-counterDamage <= 0 {
			// A last-player death needs match finalization. That path cannot
			// yet use FinalizeMatchTx for an incomplete combat history.
			var survivors int
			if err = c.Tx.QueryRowContext(r.Context(), `SELECT jsonb_array_length(turn_order) FROM matches WHERE instance_id=$1`, req.InstanceID).Scan(&survivors); err != nil {
				writeCombatCommandError(w, err)
				return true
			}
			if survivors <= 1 {
				return false
			}
		}
	}
	attackerRef := CombatTargetRef{ID: req.AttackerID, Type: CombatActorPlayer}
	targetRef := CombatTargetRef{ID: req.TargetID, Type: CombatActorMonster}
	effects := []CombatEffect{}
	criticalShot := false
	if attacker.CharacterType == "ranger" {
		hit, criticalShot = tryApplyRangerCriticalShot(a, monster.Health, hit)
		if criticalShot {
			effects = append(effects, CombatEffect{Kind: "crit", Source: &attackerRef, Target: &targetRef, Succeeded: true})
		}
	}
	finalMonsterHP := hit.NewHealth
	steps := []CombatStep{{Kind: "hit", Source: &attackerRef, Target: targetRef, Damage: hit.Damage, TargetHPAfter: hit.NewHealth}}
	if attacker.CharacterType == "berserker" && hit.Damage > 0 && rollReflexProc(balance.CalculateReflexProcChance(attacker.Agility)) {
		heal := actualDamageDealt(monster.Health, hit.NewHealth) / 2
		missing := attacker.MaxHealth - a.Health
		if heal > missing {
			heal = missing
		}
		if heal > 0 {
			a.Health += heal
			effects = append(effects, CombatEffect{Kind: "lifesteal", Source: &attackerRef, Target: &attackerRef, Amount: heal, Succeeded: true})
		}
	}
	counter := attackResult{NewHealth: a.Health}
	if mode == attackModeMelee && finalMonsterHP > 0 {
		counter = attackResult{Damage: counterDamage, NewHealth: a.Health - counterDamage, Triggered: true}
		steps = append(steps, CombatStep{Kind: "counter", Source: &targetRef, Target: attackerRef, Damage: counter.Damage, TargetHPAfter: counter.NewHealth})
	}
	var guardianResponse attackResult
	if counter.Triggered && counter.NewHealth > 0 && attacker.CharacterType == "guardian" {
		responseDamage := a.Defense - effectiveMonster.Defense/2
		if responseDamage < 1 {
			responseDamage = 1
		}
		guardianResponse = applyFlatDamage(finalMonsterHP, responseDamage)
		finalMonsterHP = guardianResponse.NewHealth
		steps = append(steps, CombatStep{Kind: "guardianResponse", Source: &attackerRef, Target: targetRef, Damage: guardianResponse.Damage, TargetHPAfter: finalMonsterHP})
	}
	followupDamage := 0
	if attacker.CharacterType == "berserker" && mode == attackModeMelee && counter.Triggered && hit.Triggered && finalMonsterHP > 0 && counter.NewHealth > 0 && berserkerFollowUpLimitPerTurn == 0 {
		followupDamage = hit.Damage / 2
		if followupDamage > 0 {
			followup := applyFlatDamage(finalMonsterHP, followupDamage)
			finalMonsterHP = followup.NewHealth
			steps = append(steps, CombatStep{Kind: "followup", Source: &attackerRef, Target: targetRef, Damage: followup.Damage, TargetHPAfter: finalMonsterHP})
		}
	}
	applyArmorBreak := attacker.CharacterType == "ranger" && mode == attackModeRanged && hit.Triggered && finalMonsterHP > 0
	if applyArmorBreak {
		if _, ok := game.GetMatchState(req.InstanceID); ok {
			effects = append(effects, CombatEffect{Kind: "armorBreak", Source: &attackerRef, Target: &targetRef, Value: -monsterArmorBreakPenaltyPerStack, Stacks: preArmorBreak.Stacks + 1, DurationTurns: armorBreakDurationTurns, Succeeded: true})
		} else {
			applyArmorBreak = false
		}
	}
	attackerKilled := counter.Triggered && counter.NewHealth <= 0
	attackerHPToPersist := counter.NewHealth
	if attackerKilled {
		attackerHPToPersist = a.Health
	}
	if err = c.SetCombatPlayerVitalsTx(r.Context(), req.AttackerID, attackerHPToPersist, attacker.Energy-resolveAttackEnergyCost(mode)); err != nil {
		writeCombatCommandError(w, err)
		return true
	}
	var mapChanged bool
	var nextActive int
	var deathCell *game.FullCell
	if finalMonsterHP <= 0 {
		deathCell, mapChanged, err = c.KillMonsterAndClearCellTx(r.Context(), req.TargetID)
	} else {
		mapChanged, err = c.SetMonsterHealthAndCellTx(r.Context(), req.TargetID, finalMonsterHP)
	}
	if err != nil {
		writeCombatCommandError(w, err)
		return true
	}
	var drops []EquipmentDropPayload
	if finalMonsterHP <= 0 {
		steps = append(steps, CombatStep{Kind: "death", Target: targetRef})
		if equipmentDropPersistenceAvailable() && RollEquipmentDropChance() {
			item, dropErr := repository.GrantRandomEquipmentDropForMatchTx(c.Tx, req.AttackerID, req.InstanceID)
			if dropErr != nil {
				writeCombatCommandError(w, dropErr)
				return true
			}
			if item != nil {
				drops = []EquipmentDropPayload{equipmentDropPayloadFromItem(req.AttackerID, item)}
			}
		}
		if err = c.QueueEvent(map[string]interface{}{"type": "UPDATE_CELL", "payload": map[string]interface{}{"instanceId": req.InstanceID, "updatedCell": serialiseUpdatedCell(*deathCell)}}); err != nil {
			writeCombatCommandError(w, err)
			return true
		}
		for _, drop := range drops {
			if err = c.QueueEvent(map[string]interface{}{"type": "EQUIPMENT_DROPPED", "payload": map[string]interface{}{"instanceId": req.InstanceID, "itemInstanceId": drop.InstanceID, "userId": drop.UserID, "ownerUserId": drop.OwnerUserID, "templateCode": drop.TemplateCode, "name": drop.Name, "rarity": drop.Rarity, "imageUrl": drop.ImageURL, "slot": drop.Slot, "itemType": drop.ItemType, "item": drop.Item}}); err != nil {
				writeCombatCommandError(w, err)
				return true
			}
		}
	}
	if attackerKilled {
		steps = append(steps, CombatStep{Kind: "death", Target: attackerRef})
		turn, _, changed, deathErr := finishCombatPlayerDeathTx(r.Context(), c, req.AttackerID, game.ResultSourceMonster, req.TargetID, counter.Damage, repository.Position{X: attacker.Position.X, Y: attacker.Position.Y})
		if deathErr != nil {
			writeCombatCommandError(w, deathErr)
			return true
		}
		nextActive, mapChanged = turn.ActiveUserID, mapChanged || changed
	}
	for _, event := range []game.DamageEvent{
		{SourceType: game.ResultSourcePlayer, DealerID: req.AttackerID, TargetType: "monster", TargetID: req.TargetID, Amount: hit.Damage},
		{SourceType: game.ResultSourceMonster, DealerID: req.TargetID, TargetType: "player", TargetID: req.AttackerID, Amount: counter.Damage},
		{SourceType: game.ResultSourcePlayer, DealerID: req.AttackerID, TargetType: "monster", TargetID: req.TargetID, Amount: guardianResponse.Damage},
		{SourceType: game.ResultSourcePlayer, DealerID: req.AttackerID, TargetType: "monster", TargetID: req.TargetID, Amount: followupDamage},
	} {
		if err = c.RecordDamageEventTx(r.Context(), event); err != nil {
			writeCombatCommandError(w, err)
			return true
		}
	}
	if finalMonsterHP <= 0 {
		killDamage := hit.Damage
		if followupDamage > 0 {
			killDamage = followupDamage
		} else if guardianResponse.Damage > 0 {
			killDamage = guardianResponse.Damage
		}
		if _, err = c.RecordKillEventTx(r.Context(), game.KillEvent{SourceType: game.ResultSourcePlayer, KillerID: req.AttackerID, VictimType: "monster", VictimID: req.TargetID, Damage: killDamage}); err != nil {
			writeCombatCommandError(w, err)
			return true
		}
	}
	payload := CombatExchangePayload{InstanceID: req.InstanceID, ExchangeID: req.InstanceID + ":" + req.CommandID, AttackerID: req.AttackerID, AttackerType: attackerRef.Type, TargetID: req.TargetID, TargetType: targetRef.Type, AttackStyle: resolvePresentationAttackStyle(a, mode), Steps: steps, Effects: effects, Drops: drops}
	if err = c.QueueEvent(map[string]interface{}{"type": "COMBAT_EXCHANGE", "payload": payload}); err != nil {
		writeCombatCommandError(w, err)
		return true
	}
	response := &atomicAttackResponse{DamageToTarget: hit.Damage, NewTargetHP: finalMonsterHP, CounterDamage: counter.Damage, NewAttackerHP: counter.NewHealth, AttackMode: mode, Drops: drops}
	state, mapRevision, err := c.Commit(r.Context(), response, mapChanged)
	if err != nil {
		writeCombatCommandError(w, err)
		return true
	}
	if ms, ok := game.GetMatchState(req.InstanceID); ok {
		if hit.Damage > 0 {
			ms.RecordDamageEvent(game.ResultSourcePlayer, req.AttackerID, "monster", req.TargetID, hit.Damage)
		}
		if counter.Triggered && counter.Damage > 0 {
			ms.RecordDamageEvent(game.ResultSourceMonster, req.TargetID, "player", req.AttackerID, counter.Damage)
		}
		if guardianResponse.Damage > 0 {
			ms.RecordDamageEvent(game.ResultSourcePlayer, req.AttackerID, "monster", req.TargetID, guardianResponse.Damage)
		}
		if followupDamage > 0 {
			ms.RecordDamageEvent(game.ResultSourcePlayer, req.AttackerID, "monster", req.TargetID, followupDamage)
			ms.TryUseBerserkerFury(req.AttackerID, berserkerFollowUpLimitPerTurn)
		}
		if applyArmorBreak {
			ms.ApplyArmorBreak("monster", req.TargetID, armorBreakMaxStacks, armorBreakDurationTurns)
		}
		if finalMonsterHP <= 0 {
			killDamage := hit.Damage
			if followupDamage > 0 {
				killDamage = followupDamage
			} else if guardianResponse.Damage > 0 {
				killDamage = guardianResponse.Damage
			}
			ms.RecordKillEvent(game.ResultSourcePlayer, req.AttackerID, "monster", req.TargetID, killDamage)
		}
		if attackerKilled {
			ms.RecordKillEvent(game.ResultSourceMonster, req.TargetID, "player", req.AttackerID, counter.Damage)
			ms.RecordPlayerDefeat(req.AttackerID)
			ms.RemovePlayerFromTurnOrder(req.AttackerID)
			ms.AdvanceTurnCombatState(nextActive, false)
		}
	}
	if attackerKilled {
		unbindPlayer(req.AttackerID)
		startTurnTimer(req.InstanceID, nextActive)
	}
	changes := make([]json.RawMessage, 0, len(c.Events)+1)
	for _, event := range c.Events {
		changes = append(changes, event)
	}
	if player, loadErr := repository.GetMatchPlayerByID(req.InstanceID, req.AttackerID); loadErr == nil {
		update, _ := json.Marshal(map[string]interface{}{"type": "UPDATE_PLAYER", "payload": map[string]interface{}{"instanceId": req.InstanceID, "player": player}})
		changes = append(changes, update)
	}
	patch, _ := json.Marshal(map[string]interface{}{"type": "MATCH_PATCH", "payload": map[string]interface{}{"instanceId": req.InstanceID, "schemaVersion": 1, "stateRevision": state, "mapRevision": mapRevision, "commandId": req.CommandID, "changes": changes}})
	Broadcast(patch)
	for _, event := range changes {
		var msg map[string]json.RawMessage
		if json.Unmarshal(event, &msg) != nil {
			continue
		}
		var p map[string]interface{}
		if json.Unmarshal(msg["payload"], &p) != nil {
			continue
		}
		p["commandId"] = req.CommandID
		var typ string
		_ = json.Unmarshal(msg["type"], &typ)
		legacy, _ := json.Marshal(map[string]interface{}{"type": typ, "payload": p})
		Broadcast(legacy)
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(response)
	return true
}
