package handlers

import "testing"

func TestHexDamageEventsCountActualHPOnce(t *testing.T) {
	attacker := CombatTargetRef{ID: 3, Type: CombatActorPlayer}
	target := CombatTargetRef{ID: 7, Type: CombatActorMonster}
	steps := []CombatStep{{Kind: "hit", Source: &attacker, Target: target, Damage: 12, TargetHPAfter: 0}, {Kind: "death", Target: target}}
	events := hexDamageEventsFromSteps(steps, nil, map[CombatTargetRef]int{attacker: 20, target: 5})
	if len(events) != 1 || events[0].Amount != 5 || events[0].DealerID != 3 {
		t.Fatalf("overkill counted as damage or duplicated: %+v", events)
	}
}

func TestHexDamageEventsTrackLifestealBeforeCounter(t *testing.T) {
	attacker := CombatTargetRef{ID: 3, Type: CombatActorPlayer}
	target := CombatTargetRef{ID: 7, Type: CombatActorPlayer}
	steps := []CombatStep{{Kind: "hit", Source: &attacker, Target: target, Damage: 4, TargetHPAfter: 6}, {Kind: "counter", Source: &target, Target: attacker, Damage: 8, TargetHPAfter: 0}}
	effects := []CombatEffect{{Kind: "lifesteal", Source: &attacker, Amount: 3, Succeeded: true}}
	events := hexDamageEventsFromSteps(steps, effects, map[CombatTargetRef]int{attacker: 5, target: 10})
	if len(events) != 2 || events[0].Amount != 4 || events[1].Amount != 8 {
		t.Fatalf("counter ignored restored HP: %+v", events)
	}
}
