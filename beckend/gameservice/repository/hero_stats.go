package repository

// HeroClassStats holds base stats for a hero class.
type HeroClassStats struct {
	Energy      int
	EnergyRegen int
	MaxEnergy   int
	Health      int
	MaxHealth   int
	Attack      int
	Defense     int
	Mobility    int
	Agility     int
	SightRange  int
	IsRanged    bool
	AttackRange int
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
			Energy:      90,
			EnergyRegen: 10,
			MaxEnergy:   90,
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
			Energy:      100,
			EnergyRegen: 10,
			MaxEnergy:   100,
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
			Energy:      105,
			EnergyRegen: 11,
			MaxEnergy:   105,
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
			Energy:      125,
			EnergyRegen: 13,
			MaxEnergy:   125,
			Health:      95,
			MaxHealth:   95,
			Attack:      10,
			Defense:     4,
			Mobility:    3,
			Agility:     5,
			SightRange:  2,
			IsRanged:    true,
			AttackRange: 3,
		}
	default: // guardian fallback
		return HeroClassStats{
			Energy:      90,
			EnergyRegen: 10,
			MaxEnergy:   90,
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
		return HeroClassMeta{DisplayName: "Mystic", Description: "Long-range caster with strong energy control.", Image: "/mag/mag.webp"}
	default:
		return HeroClassMeta{DisplayName: "Guardian", Description: "Durable frontline defender.", Image: "/guardian/guardian.webp"}
	}
}
