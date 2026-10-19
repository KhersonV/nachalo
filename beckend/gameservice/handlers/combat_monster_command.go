package handlers

import (
	"encoding/json"
	"net/http"

	"gameservice/game"
	"gameservice/internal/game/balance"
	"gameservice/repository"
)

// tryAtomicNonlethalPvE handles exchanges that leave both actors alive and do
// not trigger a Ranger third shot or quest-artifact effect.
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
	if req.AutoAttack && (req.MoveX<0||req.MoveY<0||req.MoveX>=c.Width||req.MoveY>=c.Height||monster.X!=req.MoveX||monster.Y!=req.MoveY) {http.Error(w,"target moved",http.StatusBadRequest);return true}
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
	if hit.NewHealth <= 0 {
		return false
	}
	preArmorBreak := game.ArmorBreakState{}
	if ms, ok := game.GetMatchState(req.InstanceID); ok {
		preArmorBreak = ms.GetArmorBreakState("monster", req.TargetID)
	}
	if attacker.CharacterType == "ranger" {
		if mode == attackModeRanged && preArmorBreak.Stacks >= armorBreakMaxStacks {
			return false
		}
		if hit.Damage > 0 && monster.Health-(hit.Damage*rangerCriticalDamagePercent)/100 <= 0 {
			return false
		}
	}
	if attacker.CharacterType == "berserker" && mode == attackModeMelee && berserkerFollowUpLimitPerTurn == 0 && hit.NewHealth <= hit.Damage/2 {
		return false
	}
	counterDamage := 0
	if mode == attackModeMelee {
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
			return false
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
	if mode == attackModeMelee {
		counter = attackResult{Damage: counterDamage, NewHealth: a.Health - counterDamage, Triggered: true}
		steps = append(steps, CombatStep{Kind: "counter", Source: &targetRef, Target: attackerRef, Damage: counter.Damage, TargetHPAfter: counter.NewHealth})
	}
	var guardianResponse attackResult
	if counter.Triggered && attacker.CharacterType == "guardian" {
		responseDamage := a.Defense - effectiveMonster.Defense/2
		if responseDamage < 1 {
			responseDamage = 1
		}
		guardianResponse = applyFlatDamage(finalMonsterHP, responseDamage)
		if guardianResponse.NewHealth <= 0 {
			return false
		}
		finalMonsterHP = guardianResponse.NewHealth
		steps = append(steps, CombatStep{Kind: "guardianResponse", Source: &attackerRef, Target: targetRef, Damage: guardianResponse.Damage, TargetHPAfter: finalMonsterHP})
	}
	followupDamage := 0
	if attacker.CharacterType == "berserker" && mode == attackModeMelee && counter.Triggered && hit.Triggered && finalMonsterHP > 0 && counter.NewHealth > 0 && berserkerFollowUpLimitPerTurn == 0 {
		followupDamage = hit.Damage / 2
		if followupDamage > 0 {
			followup := applyFlatDamage(finalMonsterHP, followupDamage)
			if followup.NewHealth <= 0 {
				return false
			}
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
	if err = c.SetCombatPlayerVitalsTx(r.Context(), req.AttackerID, counter.NewHealth, attacker.Energy-resolveAttackEnergyCost(mode)); err != nil {
		writeCombatCommandError(w, err)
		return true
	}
	mapChanged, err := c.SetMonsterHealthAndCellTx(r.Context(), req.TargetID, finalMonsterHP)
	if err != nil {
		writeCombatCommandError(w, err)
		return true
	}
	payload := CombatExchangePayload{InstanceID: req.InstanceID, ExchangeID: req.InstanceID + ":" + req.CommandID, AttackerID: req.AttackerID, AttackerType: attackerRef.Type, TargetID: req.TargetID, TargetType: targetRef.Type, AttackStyle: resolvePresentationAttackStyle(a, mode), Steps: steps, Effects: effects}
	if err = c.QueueEvent(map[string]interface{}{"type": "COMBAT_EXCHANGE", "payload": payload}); err != nil {
		writeCombatCommandError(w, err)
		return true
	}
	response := &atomicAttackResponse{DamageToTarget: hit.Damage, NewTargetHP: finalMonsterHP, CounterDamage: counter.Damage, NewAttackerHP: counter.NewHealth, AttackMode: mode}
	state, mapRevision, err := c.Commit(r.Context(), response, mapChanged)
	if err != nil {
		writeCombatCommandError(w, err)
		return true
	}
	if ms, ok := game.GetMatchState(req.InstanceID); ok {
		if hit.Damage > 0 {
			ms.RecordDamageEvent(req.AttackerID, "monster", req.TargetID, hit.Damage)
		}
		if counter.Triggered && counter.Damage > 0 {
			ms.RecordDamageEvent(req.TargetID, "player", req.AttackerID, counter.Damage)
		}
		if guardianResponse.Damage > 0 {
			ms.RecordDamageEvent(req.AttackerID, "monster", req.TargetID, guardianResponse.Damage)
		}
		if followupDamage > 0 {
			ms.RecordDamageEvent(req.AttackerID, "monster", req.TargetID, followupDamage)
			ms.TryUseBerserkerFury(req.AttackerID, berserkerFollowUpLimitPerTurn)
		}
		if applyArmorBreak {
			ms.ApplyArmorBreak("monster", req.TargetID, armorBreakMaxStacks, armorBreakDurationTurns)
		}
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
