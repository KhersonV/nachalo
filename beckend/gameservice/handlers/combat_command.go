package handlers

import (
	"encoding/json"
	"fmt"
	"net/http"

	"gameservice/game"
	"gameservice/internal/game/balance"
	"gameservice/repository"
)

// This transactional branch covers nonlethal PvP without
// quest artifacts. Remaining class effects still use legacy combat.
func tryAtomicNonlethalPvP(w http.ResponseWriter, r *http.Request, req AttackRequest) bool {
	if req.AttackerType != "player" || req.TargetType != "player" || req.AttackerID == req.TargetID {
		return false
	}
	canonical := combatCommandCanonical(req, "nonlethal-pvp-v1")
	c, err := repository.BeginMatchCommand(r.Context(), req.InstanceID, req.AttackerID, req.CommandID, canonical)
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
	target, err := c.MatchCombatPlayerTx(r.Context(), req.TargetID)
	if err != nil {
		writeCombatCommandError(w, err)
		return true
	}
	if req.AutoAttack && (req.MoveX < 0 || req.MoveY < 0 || req.MoveX >= c.Width || req.MoveY >= c.Height || target.Position.X != req.MoveX || target.Position.Y != req.MoveY) {
		http.Error(w, "target moved", http.StatusBadRequest)
		return true
	}
	if attacker.CharacterType != "guardian" && attacker.CharacterType != "mystic" && attacker.CharacterType != "berserker" && attacker.CharacterType != "ranger" {
		return false
	}
	aArtifact, err := c.QuestArtifactTx(r.Context(), req.AttackerID)
	if err != nil {
		writeCombatCommandError(w, err)
		return true
	}
	tArtifact, err := c.QuestArtifactTx(r.Context(), req.TargetID)
	if err != nil {
		writeCombatCommandError(w, err)
		return true
	}
	if aArtifact.Name != "" || tArtifact.Name != "" {
		return false
	}
	if attacker.Health <= 0 || target.Health <= 0 {
		http.Error(w, "actor defeated", http.StatusBadRequest)
		return true
	}
	if attacker.GroupID != 0 && attacker.GroupID == target.GroupID {
		http.Error(w, "Нельзя атаковать союзника", http.StatusBadRequest)
		return true
	}
	a := stats{Attack: attacker.Attack, Defense: attacker.Defense, Health: attacker.Health, MaxHealth: attacker.MaxHealth, Agility: attacker.Agility, CharacterType: attacker.CharacterType, IsRanged: attacker.IsRanged, AttackRange: attacker.AttackRange, X: attacker.Position.X, Y: attacker.Position.Y}
	d := stats{Attack: target.Attack, Defense: target.Defense, Health: target.Health, MaxHealth: target.MaxHealth, Agility: target.Agility, CharacterType: target.CharacterType, IsRanged: target.IsRanged, AttackRange: target.AttackRange, X: target.Position.X, Y: target.Position.Y}
	mode, err := resolveAttackMode(a, d)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return true
	}
	if attacker.Energy < resolveAttackEnergyCost(mode) {
		http.Error(w, "Недостаточно энергии для атаки", http.StatusBadRequest)
		return true
	}
	d.Defense = effectiveDefense(req.InstanceID, "player", req.TargetID, d.Defense)
	hit := applyDamage(a, d)
	minSurvivalHP := 0
	if attacker.CharacterType == "mystic" {
		minSurvivalHP = arcaneOverburnEnergyBurn
	}
	if hit.NewHealth <= minSurvivalHP {
		return false
	}
	preArmorBreak := game.ArmorBreakState{}
	if ms, ok := game.GetMatchState(req.InstanceID); ok {
		preArmorBreak = ms.GetArmorBreakState("player", req.TargetID)
	}
	if attacker.CharacterType == "ranger" {
		if mode == attackModeRanged && preArmorBreak.Stacks >= armorBreakMaxStacks {
			return false
		}
		if hit.Damage > 0 && target.Health-(hit.Damage*rangerCriticalDamagePercent)/100 <= 0 {
			return false
		}
	}
	// The legacy counter spends energy only when the defender can afford it.
	counter := attackResult{NewHealth: a.Health}
	targetEnergy := target.Energy
	attackerEnergy := attacker.Energy - resolveAttackEnergyCost(mode)
	counterCost := standardCounterAttackEnergyCost
	if target.CharacterType == "guardian" {
		counterCost = 0
	}
	if attacker.CharacterType == "berserker" && mode == attackModeMelee && targetEnergy >= counterCost && berserkerFollowUpLimitPerTurn == 0 && hit.NewHealth <= hit.Damage/2 {
		return false
	}
	// Check the largest possible counter before rolling shield or overburn.
	if mode == attackModeMelee && targetEnergy >= counterCost {
		a.Defense = effectiveDefense(req.InstanceID, "player", req.AttackerID, a.Defense)
		if applyDamage(d, a).NewHealth <= 0 {
			return false
		}
	}
	shieldBlocked := resolveGuardianShieldBlock("player", d)
	if shieldBlocked {
		hit = attackResult{Damage: 0, NewHealth: target.Health, Triggered: false}
	}
	criticalShot := false
	if attacker.CharacterType == "ranger" {
		hit, criticalShot = tryApplyRangerCriticalShot(a, target.Health, hit)
	}
	attackerRef := CombatTargetRef{ID: req.AttackerID, Type: toCombatActorType("player")}
	targetRef := CombatTargetRef{ID: req.TargetID, Type: toCombatActorType("player")}
	effects := []CombatEffect{}
	if shieldBlocked {
		effects = append(effects, CombatEffect{Kind: "block", Target: &targetRef, Succeeded: true})
	}
	if criticalShot {
		effects = append(effects, CombatEffect{Kind: "crit", Source: &attackerRef, Target: &targetRef, Succeeded: true})
	}
	steps := []CombatStep{{Kind: "hit", Source: &attackerRef, Target: targetRef, Damage: hit.Damage, TargetHPAfter: hit.NewHealth}}
	finalTargetHP := hit.NewHealth
	bonusDamage := 0
	if attacker.CharacterType == "berserker" && hit.Triggered && hit.Damage > 0 && rollReflexProc(balance.CalculateReflexProcChance(attacker.Agility)) {
		heal := actualDamageDealt(target.Health, hit.NewHealth) / 2
		missing := attacker.MaxHealth - a.Health
		if heal > missing {
			heal = missing
		}
		if heal > 0 {
			a.Health += heal
			effects = append(effects, CombatEffect{Kind: "lifesteal", Source: &attackerRef, Target: &attackerRef, Amount: heal, Succeeded: true})
		}
	}
	if attacker.CharacterType == "mystic" && hit.Triggered && finalTargetHP > 0 {
		overburn := rollReflexProc(balance.CalculateReflexProcChance(attacker.Agility))
		burn := energyDrainPerHit
		restore := energyDrainGainPerHit
		if overburn {
			burn = arcaneOverburnEnergyBurn
			restore = arcaneOverburnEnergyGain
		}
		actualBurn := burn
		if targetEnergy < actualBurn {
			actualBurn = targetEnergy
		}
		if actualBurn < 0 {
			actualBurn = 0
		}
		missing := burn - actualBurn
		gain := restore
		available := attacker.MaxEnergy - attackerEnergy
		if available < 0 {
			available = 0
		}
		if gain > available {
			gain = available
		}
		targetEnergy -= actualBurn
		attackerEnergy += gain
		effects = append(effects, CombatEffect{Kind: "energyDrain", Source: &attackerRef, Target: &targetRef, Succeeded: true, Amount: actualBurn, EnergyGranted: gain, EnergyDrained: actualBurn, SourceEnergyAfter: attackerEnergy, TargetEnergyAfter: targetEnergy})
		if overburn {
			effects = append(effects, CombatEffect{Kind: "arcaneOverburn", Source: &attackerRef, Target: &targetRef, Succeeded: true})
		}
		if missing > 0 {
			pure := applyFlatDamage(finalTargetHP, missing)
			bonusDamage = actualDamageDealt(finalTargetHP, pure.NewHealth)
			finalTargetHP = pure.NewHealth
			if bonusDamage > 0 {
				steps = append(steps, CombatStep{Kind: "bonus", Source: &attackerRef, Target: targetRef, Damage: bonusDamage, TargetHPAfter: finalTargetHP})
				effects = append(effects, CombatEffect{Kind: "pureDamage", Source: &attackerRef, Target: &targetRef, Amount: bonusDamage, Succeeded: true})
			}
		}
	}
	counter.NewHealth = a.Health
	if mode == attackModeMelee && targetEnergy >= counterCost {
		targetEnergy -= counterCost
		counter = applyDamage(d, a)
		steps = append(steps, CombatStep{Kind: "counter", Source: &targetRef, Target: attackerRef, Damage: counter.Damage, TargetHPAfter: counter.NewHealth})
	}
	applyArmorBreak := attacker.CharacterType == "ranger" && mode == attackModeRanged && hit.Triggered && finalTargetHP > 0
	if applyArmorBreak {
		if _, ok := game.GetMatchState(req.InstanceID); ok {
			effects = append(effects, CombatEffect{Kind: "armorBreak", Source: &attackerRef, Target: &targetRef, Value: -armorBreakDefensePenaltyPerStack, Stacks: preArmorBreak.Stacks + 1, DurationTurns: armorBreakDurationTurns, Succeeded: true})
		} else {
			applyArmorBreak = false
		}
	}
	followupDamage := 0
	if attacker.CharacterType == "berserker" && mode == attackModeMelee && counter.Triggered && hit.Triggered && finalTargetHP > 0 && counter.NewHealth > 0 && berserkerFollowUpLimitPerTurn == 0 {
		followupDamage = hit.Damage / 2
		if followupDamage > 0 {
			followup := applyFlatDamage(finalTargetHP, followupDamage)
			finalTargetHP = followup.NewHealth
			steps = append(steps, CombatStep{Kind: "followup", Source: &attackerRef, Target: targetRef, Damage: followup.Damage, TargetHPAfter: finalTargetHP})
		}
	}
	if err = c.SetCombatPlayerVitalsTx(r.Context(), req.AttackerID, counter.NewHealth, attackerEnergy); err != nil {
		writeCombatCommandError(w, err)
		return true
	}
	if err = c.SetCombatPlayerVitalsTx(r.Context(), req.TargetID, finalTargetHP, targetEnergy); err != nil {
		writeCombatCommandError(w, err)
		return true
	}
	for _, event := range []game.DamageEvent{
		{DealerID: req.AttackerID, TargetType: "player", TargetID: req.TargetID, Amount: hit.Damage},
		{DealerID: req.AttackerID, TargetType: "player", TargetID: req.TargetID, Amount: bonusDamage},
		{DealerID: req.AttackerID, TargetType: "player", TargetID: req.TargetID, Amount: followupDamage},
		{DealerID: req.TargetID, TargetType: "player", TargetID: req.AttackerID, Amount: counter.Damage},
	} {
		if err = c.RecordDamageEventTx(r.Context(), event); err != nil {
			writeCombatCommandError(w, err)
			return true
		}
	}
	payload := CombatExchangePayload{InstanceID: req.InstanceID, ExchangeID: req.InstanceID + ":" + req.CommandID, AttackerID: req.AttackerID, AttackerType: attackerRef.Type, TargetID: req.TargetID, TargetType: targetRef.Type, AttackStyle: resolvePresentationAttackStyle(a, mode), Steps: steps, Effects: effects}
	if err = c.QueueEvent(map[string]interface{}{"type": "COMBAT_EXCHANGE", "payload": payload}); err != nil {
		writeCombatCommandError(w, err)
		return true
	}
	response := &atomicAttackResponse{DamageToTarget: hit.Damage, NewTargetHP: finalTargetHP, CounterDamage: counter.Damage, NewAttackerHP: counter.NewHealth, AttackMode: mode}
	state, mapRevision, err := c.Commit(r.Context(), response, false)
	if err != nil {
		writeCombatCommandError(w, err)
		return true
	}
	// Memory statistics and WebSocket messages are applied only after commit.
	if ms, ok := game.GetMatchState(req.InstanceID); ok {
		if hit.Damage > 0 {
			ms.RecordDamageEvent(req.AttackerID, "player", req.TargetID, hit.Damage)
		}
		if bonusDamage > 0 {
			ms.RecordDamageEvent(req.AttackerID, "player", req.TargetID, bonusDamage)
		}
		if followupDamage > 0 {
			ms.RecordDamageEvent(req.AttackerID, "player", req.TargetID, followupDamage)
			ms.TryUseBerserkerFury(req.AttackerID, berserkerFollowUpLimitPerTurn)
		}
		if applyArmorBreak {
			ms.ApplyArmorBreak("player", req.TargetID, armorBreakMaxStacks, armorBreakDurationTurns)
		}
		if counter.Triggered && counter.Damage > 0 {
			ms.RecordDamageEvent(req.TargetID, "player", req.AttackerID, counter.Damage)
		}
	}
	changes := make([]json.RawMessage, 0, len(c.Events))
	for _, event := range c.Events {
		changes = append(changes, event)
	}
	for _, id := range []int{req.AttackerID, req.TargetID} {
		player, loadErr := repository.GetMatchPlayerByID(req.InstanceID, id)
		if loadErr == nil {
			update, _ := json.Marshal(map[string]interface{}{"type": "UPDATE_PLAYER", "payload": map[string]interface{}{"instanceId": req.InstanceID, "player": player}})
			changes = append(changes, update)
		}
	}
	patch, _ := json.Marshal(map[string]interface{}{"type": "MATCH_PATCH", "payload": map[string]interface{}{"instanceId": req.InstanceID, "schemaVersion": 1, "stateRevision": state, "mapRevision": mapRevision, "commandId": req.CommandID, "changes": changes}})
	Broadcast(patch)
	// Legacy copies carry commandId so revision-aware clients ignore them.
	for _, event := range changes {
		var msg map[string]json.RawMessage
		if json.Unmarshal(event, &msg) == nil {
			var p map[string]interface{}
			if json.Unmarshal(msg["payload"], &p) == nil {
				p["commandId"] = req.CommandID
				var messageType string
				_ = json.Unmarshal(msg["type"], &messageType)
				legacy, _ := json.Marshal(map[string]interface{}{"type": messageType, "payload": p})
				Broadcast(legacy)
			}
		}
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(response)
	return true
}

func combatCommandCanonical(req AttackRequest, kind string) string {
	if req.AutoAttack {
		return fmt.Sprintf("autoattack-v1:%d:%d:%d", req.AttackerID, req.MoveX, req.MoveY)
	}
	return fmt.Sprintf("%s:%d:%d", kind, req.AttackerID, req.TargetID)
}

type atomicAttackResponse struct {
	DamageToTarget int                    `json:"damage_to_target"`
	NewTargetHP    int                    `json:"new_target_hp"`
	CounterDamage  int                    `json:"counter_damage"`
	NewAttackerHP  int                    `json:"new_attacker_hp"`
	AttackMode     attackMode             `json:"attack_mode"`
	StateRevision  int64                  `json:"stateRevision"`
	MapRevision    int64                  `json:"mapRevision"`
	Drops          []EquipmentDropPayload `json:"drops,omitempty"`
}

func (r *atomicAttackResponse) SetCommandRevisions(state, mapRevision int64) {
	r.StateRevision, r.MapRevision = state, mapRevision
}

func writeCombatCommandError(w http.ResponseWriter, err error) {
	if err == repository.ErrCommandConflict {
		http.Error(w, err.Error(), http.StatusConflict)
		return
	}
	if err == repository.ErrCommandRejected {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	http.Error(w, "combat command failed", http.StatusInternalServerError)
}
