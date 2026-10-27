package balance

// CombatItemBonuses are additive properties of equipped items, never set procs.
type CombatItemBonuses struct {
	AuraRadius         int `json:"auraRadius,omitempty"`
	AuraTick           int `json:"auraTick,omitempty"`
	AuraExit           int `json:"auraExit,omitempty"`
	HealPercent        int `json:"healPercent,omitempty"`
	FollowupDamage     int `json:"followupDamage,omitempty"`
	CriticalPercent    int `json:"criticalPercent,omitempty"`
	OverloadSecond     int `json:"overloadSecond,omitempty"`
	OverloadThird      int `json:"overloadThird,omitempty"`
	MysticReflexDamage int `json:"mysticReflexDamage,omitempty"`
}

// Frozen with the match's ordinary stats; lobby equipment changes cannot alter a duel.
type CombatProfile struct {
	Ruleset CombatRuleset `json:"ruleset,omitempty"`
	CriticalMultiplier float64 `json:"criticalMultiplier,omitempty"`
	ClassHealPercent float64 `json:"classHealPercent,omitempty"`
	FollowupMultiplier float64 `json:"followupMultiplier,omitempty"`
	ReflexChance *int `json:"reflexChance,omitempty"`
	AbilityAttackScale float64 `json:"abilityAttackScale,omitempty"`
	GearLevel          int `json:"gearLevel"` // 0..2000; 1000 is the green catalog's effective bonus budget.
	AuraRadius         int `json:"auraRadius"`
	AuraTick           int `json:"auraTick"`
	AuraExit           int `json:"auraExit"`
	HealPercent        int `json:"healPercent"`
	FollowupDamage     int `json:"followupDamage"`
	CriticalPercent    int `json:"criticalPercent"`
	OverloadSecond     int `json:"overloadSecond"`
	OverloadThird      int `json:"overloadThird"`
	MysticReflexDamage int `json:"mysticReflexDamage"`
}

func DefaultCombatProfile() CombatProfile {
	return CombatProfile{AuraRadius: 1, AuraTick: 2, AuraExit: 3, HealPercent: 50, CriticalPercent: 142, OverloadSecond: 2, OverloadThird: 5, MysticReflexDamage: 3}
}

func DefenseDamage(attack, defense int) int {
	if attack <= 0 {
		return 0
	}
	if defense < 0 {
		defense = 0
	}
	damage := int(int64(attack) * 20 / (20 + int64(defense)))
	if damage < 1 {
		return 1
	}
	return damage
}

var classAdvantage = map[string]string{"guardian": "berserker", "berserker": "ranger", "ranger": "mystic", "mystic": "guardian"}

// Owner can be the defender (Guardian block). Only class Reflex is countered.
func ClassReflexChance(reflex int, owner, opponent string, pvp bool) int {
	chance := CalculateReflexProcChance(reflex)
	if !pvp {
		return chance
	}
	if classAdvantage[opponent] == owner {
		return 0
	}
	if classAdvantage[owner] == opponent {
		chance += 10
	}
	if chance > 35 {
		chance = 35
	}
	return chance
}

// Percent anchors: none / green / blue. Unlisted pairs, including mirrors, are 100%.
// This is tuning data, not a prediction or a guarantee of any duel's winner.
var pvpPrimaryDamage = map[[2]string][3]int{
	{"ranger", "mystic"}:   {131, 119, 120},
	{"mystic", "guardian"}: {123, 126, 122},
}

func PvPPrimaryDamage(damage int, attacker, defender string, gearLevel int) int {
	if damage <= 0 {
		return 0
	}
	anchors, ok := pvpPrimaryDamage[[2]string{attacker, defender}]
	if !ok {
		return damage
	}
	if gearLevel < 0 {
		gearLevel = 0
	}
	if gearLevel > 2000 {
		gearLevel = 2000
	}
	lo, hi, fraction := anchors[0], anchors[1], gearLevel
	if gearLevel > 1000 {
		lo, hi, fraction = anchors[1], anchors[2], gearLevel-1000
	}
	// Interpolate the multiplier before one integer floor of the scaled primary hit.
	scaled := int(int64(damage) * int64(lo*(1000-fraction)+hi*fraction) / 100000)
	if scaled < 1 {
		return 1
	}
	return scaled
}
