package repository

import (
	"gameservice/internal/game/balance"
	"os"
)

func ConfiguredHeroClassStats(heroClassID string) (HeroClassStats,error) {
	ruleset,err:=balance.ParseCombatRuleset(os.Getenv("NACHALO_COMBAT_RULESET"))
	if err!=nil {return HeroClassStats{},err}
	return ResolveHeroClassStatsForRuleset(heroClassID,ruleset),nil
}

// HeroClassStats holds base stats for a hero class.
type HeroClassStats struct {
	MaxAP        int `json:"maxAP"`
	MaxMP        int `json:"maxMP"`
	MaxReactions int `json:"maxReactions"`
	Energy       int `json:"-"`
	EnergyRegen  int `json:"-"`
	MaxEnergy    int `json:"-"`
	Health       int
	MaxHealth    int
	Attack       int
	Defense      int
	Mobility     int
	Agility      int
	SightRange   int
	IsRanged     bool
	AttackRange  int
}

// HeroClassMeta contains presentation metadata for a hero class.
type HeroClassMeta struct {
	DisplayName string
	Description string
	Image       string
}

// Explicit ruleset projection for new snapshots; never rewrite a saved character
// or consult the environment when reading an existing match. Legacy stays intact.
func ResolveHeroClassStatsForRuleset(heroClassID string, ruleset balance.CombatRuleset) HeroClassStats {
	stats := ResolveHeroClassStats(heroClassID)
	if ruleset != balance.ExperimentalV34 { return stats }
	index := 0
	switch NormalizeHeroClassID(heroClassID) {
	case "berserker": index = 1
	case "ranger": index = 2
	case "mystic": index = 3
	}
	config := balance.ExperimentalV34Config()
	stats.Health,stats.MaxHealth = config.BaseHP[index],config.BaseHP[index]
	stats.Attack,stats.Defense = config.BaseAttack[index],config.BaseDefense[index]
	return stats
}

// ResolveHeroClassStats returns base stats for the given hero class id.
// Combat Balance 2.0 base model; handlers/players.go uses the same registration values.
func ResolveHeroClassStats(heroClassID string) HeroClassStats {
	switch NormalizeHeroClassID(heroClassID) {
	case "guardian":
		return HeroClassStats{
			MaxAP: 6, MaxMP: 4, MaxReactions: 2,
			Energy:      0,
			EnergyRegen: 0,
			MaxEnergy:   0,
			Health:      130,
			MaxHealth:   130,
			Attack:      13,
			Defense:     9,
			Mobility:    2,
			Agility:     5,
			SightRange:  2,
			IsRanged:    false,
			AttackRange: 1,
		}
	case "berserker":
		return HeroClassStats{
			MaxAP: 6, MaxMP: 6, MaxReactions: 1,
			Energy:      0,
			EnergyRegen: 0,
			MaxEnergy:   0,
			Health:      100,
			MaxHealth:   100,
			Attack:      17,
			Defense:     4,
			Mobility:    3,
			Agility:     5,
			SightRange:  2,
			IsRanged:    false,
			AttackRange: 1,
		}
	case "ranger":
		return HeroClassStats{
			MaxAP: 6, MaxMP: 8, MaxReactions: 1,
			Energy:      0,
			EnergyRegen: 0,
			MaxEnergy:   0,
			Health:      92,
			MaxHealth:   92,
			Attack:      16,
			Defense:     7,
			Mobility:    4,
			Agility:     5,
			SightRange:  2,
			IsRanged:    true,
			AttackRange: 2,
		}
	case "mystic":
		return HeroClassStats{
			MaxAP: 6, MaxMP: 6, MaxReactions: 1,
			Energy:      0,
			EnergyRegen: 0,
			MaxEnergy:   0,
			Health:      95,
			MaxHealth:   95,
			Attack:      18,
			Defense:     5,
			Mobility:    3,
			Agility:     5,
			SightRange:  3,
			IsRanged:    true,
			AttackRange: 3,
		}
	default: // guardian fallback
		return HeroClassStats{
			MaxAP: 6, MaxMP: 4, MaxReactions: 2,
			Energy:      0,
			EnergyRegen: 0,
			MaxEnergy:   0,
			Health:      130,
			MaxHealth:   130,
			Attack:      13,
			Defense:     9,
			Mobility:    2,
			Agility:     5,
			SightRange:  2,
			IsRanged:    false,
			AttackRange: 1,
		}
	}
}

// ResolveHeroClassMeta returns presentation metadata for a hero class.
func ResolveHeroClassMeta(heroClassID string) HeroClassMeta {
	switch NormalizeHeroClassID(heroClassID) {
	case "guardian":
		return HeroClassMeta{DisplayName: "Guardian", Description: "Durable frontline defender.", Image: "/guardian/guardian.webp"}
	case "berserker":
		return HeroClassMeta{DisplayName: "Berserker", Description: "High-risk melee finisher.", Image: "/berserk/berserk.webp"}
	case "ranger":
		return HeroClassMeta{DisplayName: "Ranger", Description: "Mobile ranged scout.", Image: "/ranger/ranger.webp"}
	case "mystic":
		return HeroClassMeta{DisplayName: "Mystic", Description: "Long-range caster with Overload and movement control.", Image: "/mag/mag.webp"}
	default:
		return HeroClassMeta{DisplayName: "Guardian", Description: "Durable frontline defender.", Image: "/guardian/guardian.webp"}
	}
}
