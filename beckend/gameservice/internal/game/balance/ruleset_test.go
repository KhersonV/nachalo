package balance

import "testing"

// Prepared only; no tests have been executed for this port.
func TestCombatRulesetSelection(t *testing.T) {
	for _, value := range []string{"legacy", " legacy "} {
		got, err := ParseCombatRuleset(value)
		if err != nil || got != Legacy {
			t.Fatalf("%q: %q %v", value, got, err)
		}
	}
	if _, err := ParseCombatRuleset("v34"); err == nil {
		t.Fatal("unknown ruleset accepted")
	}
	got, err := ParseCombatRuleset("")
	if err != nil || got != ExperimentalV34 {
		t.Fatalf("experimental parse: %q %v", got, err)
	}
	if RequirePlayableRuleset(Legacy) != nil {
		t.Fatal("legacy must remain playable")
	}
	if RequirePlayableRuleset(ExperimentalV34) != nil {
		t.Fatal("experimental ruleset rejected")
	}
}

func TestV34DefinedDefenseFormula(t *testing.T) {
	for _, row := range []struct{ attack, defense, want int }{
		{0, 9, 0}, {-1, 9, 0}, {12, -3, 12}, {12, 16, 7}, {17, 7, 15},
		{21, 9, 18}, {24, 4, 23}, {1, 999, 1}, {400, 1250, 20}, {399, 1250, 18},
	} {
		if got := V34DefenseDamage(row.attack, row.defense); got != row.want {
			t.Fatalf("%+v: got %d", row, got)
		}
	}
}

func TestV34HatredUsesOneUniversalEdgeBonus(t *testing.T) {
	classes := []string{"guardian", "berserker", "ranger", "mystic"}
	for i, attacker := range classes {
		for j, defender := range classes {
			want, applied := 100, false
			if j == (i+1)%len(classes) {
				want, applied = 116, true
			}
			got, effect := V34HatredDamage(100, attacker, defender)
			if got != want || effect != applied {
				t.Fatalf("%s -> %s: %d %t", attacker, defender, got, effect)
			}
		}
	}
	if got, effect := V34HatredDamage(0, "guardian", "berserker"); got != 0 || effect {
		t.Fatal("zero hit emitted Hatred")
	}
	if got, _ := V34HatredDamage(7, "guardian", "berserker"); got != 8 {
		t.Fatal("Hatred rounding changed")
	}
}

func TestLegacyDefenseAndPairMatrixRemainAvailable(t *testing.T) {
	if DefenseDamage(13, 9) != 8 {
		t.Fatal("legacy defense changed")
	}
	if PvPPrimaryDamage(100, "ranger", "mystic", 0) != 131 {
		t.Fatal("legacy pair matrix changed")
	}
	if ClassReflexChance(5, "ranger", "berserker", true) != 0 {
		t.Fatal("legacy proc suppression changed")
	}
}
