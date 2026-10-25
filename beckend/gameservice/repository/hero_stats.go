package repository

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

// ResolveHeroClassStats returns base stats for the given hero class id.
// Values are copied from handlers/players.go characterTemplates to preserve balance.
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
			Attack:      9,
			Defense:     8,
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
			Attack:      14,
			Defense:     3,
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
			Attack:      11,
			Defense:     4,
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
			Attack:      10,
			Defense:     4,
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
			Attack:      9,
			Defense:     8,
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
