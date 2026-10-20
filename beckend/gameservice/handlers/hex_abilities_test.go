package handlers

import "testing"

func TestHexRangerBlockedPlayerPushUsesSquareDefenseDamage(t *testing.T) {
	attacker := stats{Attack: 11, CharacterType: "ranger"}
	target := stats{Defense: 4, Health: 30, MaxHealth: 30}
	result := resolveHexRangerPlayerPushFallback(attacker, target, 20)
	if result.Damage != 7 || result.NewHealth != 13 {
		t.Fatalf("blocked push must apply a fresh defense-aware hit: %+v", result)
	}
}
