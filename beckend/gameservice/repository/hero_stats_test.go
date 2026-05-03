package repository

import "testing"

func TestResolveHeroClassStatsUsesBaseReflexFive(t *testing.T) {
	for _, heroClassID := range []string{"guardian", "berserker", "ranger", "mystic"} {
		t.Run(heroClassID, func(t *testing.T) {
			stats := ResolveHeroClassStats(heroClassID)
			if stats.Agility != 5 {
				t.Fatalf("%s agility = %d, want 5", heroClassID, stats.Agility)
			}
		})
	}
}
