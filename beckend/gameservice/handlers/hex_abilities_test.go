package handlers

import "testing"

func TestHexRangerBlockedPlayerPushUsesSquareDefenseDamage(t *testing.T) {
	attacker := stats{Attack: 11, CharacterType: "ranger"}
	target := stats{Defense: 4, Health: 30, MaxHealth: 30}
	result := resolveHexRangerPlayerPushFallback(attacker, target, 20)
	// Legacy DEF is floor(11*20/(20+4)) = 9; collision is a fresh hit.
	if result.Damage != 9 || result.NewHealth != 11 {
		t.Fatalf("blocked push must apply a fresh defense-aware hit: %+v", result)
	}
}
