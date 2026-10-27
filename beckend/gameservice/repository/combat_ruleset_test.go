package repository

import (
	"testing"
	"gameservice/internal/game/balance"
)

// Prepared only. It exercises no database, equipment mutation or match creation.
func TestRulesetBaseStatsProjection(t *testing.T) {
	for _, row := range []struct{class string; hp,attack,defense int}{
		{"guardian",218,12,16},{"berserker",176,17,7},
		{"ranger",146,21,9},{"mystic",134,24,4},{"unknown",218,12,16},
	} {
		legacy := ResolveHeroClassStats(row.class)
		if got := ResolveHeroClassStatsForRuleset(row.class,balance.Legacy); got != legacy {
			t.Fatalf("legacy %s changed",row.class)
		}
		got := ResolveHeroClassStatsForRuleset(row.class,balance.ExperimentalV34)
		if got.Health!=row.hp || got.MaxHealth!=row.hp || got.Attack!=row.attack || got.Defense!=row.defense {
			t.Fatalf("experimental %s: %+v",row.class,got)
		}
		if got.MaxAP!=legacy.MaxAP || got.MaxMP!=legacy.MaxMP || got.MaxReactions!=legacy.MaxReactions || got.Mobility!=legacy.Mobility || got.Agility!=legacy.Agility || got.SightRange!=legacy.SightRange || got.IsRanged!=legacy.IsRanged || got.AttackRange!=legacy.AttackRange {
			t.Fatalf("experimental %s changed unrelated stats",row.class)
		}
	}
}
