package handlers

import (
	"gameservice/internal/game/balance"
	"math/rand"
)

var rollReflexProc = defaultRollReflexProc

const (
	meleeAttackEnergyCost            = 2
	rangedAttackEnergyCost           = 2
	standardCounterAttackEnergyCost  = 2
	guardianZoneControlRange         = 2
	guardianZoneControlMovePenalty   = 1
	guardianAuraExitDamageCap        = 5
	armorBreakDefensePenaltyPerStack = 2
	monsterArmorBreakPenaltyPerStack = 5
	armorBreakDurationTurns          = 2
	armorBreakMaxStacks              = 2
	rangerThirdShotEnergyRefund      = 4
	rangerCriticalDamagePercent      = 142
	berserkerFollowUpLimitPerTurn    = 0 // 0 = безлимитные дополнительные удары
	berserkerMonsterCounterPercent   = 55
	energyDrainPerHit                = 3
	energyDrainGainPerHit            = 1
	arcaneOverburnEnergyBurn         = 4
	arcaneOverburnEnergyGain         = 3
	mysticMonsterDefensePercent      = 60
)

type CombatActorType string

type AttackStyle string

type CombatTargetRef struct {
	ID   int             `json:"id"`
	Type CombatActorType `json:"type"`
}

type CombatPoint struct {
	X int `json:"x"`
	Y int `json:"y"`
}

type CombatStep struct {
	Kind          string           `json:"kind"`
	Source        *CombatTargetRef `json:"source,omitempty"`
	Target        CombatTargetRef  `json:"target"`
	Damage        int              `json:"damage,omitempty"`
	TargetHPAfter int              `json:"targetHpAfter"`
}

type CombatEffect struct {
	Kind              string           `json:"kind"`
	Source            *CombatTargetRef `json:"source,omitempty"`
	Target            *CombatTargetRef `json:"target,omitempty"`
	Value             int              `json:"value,omitempty"`
	Stacks            int              `json:"stacks,omitempty"`
	DurationTurns     int              `json:"durationTurns,omitempty"`
	Succeeded         bool             `json:"succeeded"`
	PositionAfter     *CombatPoint     `json:"positionAfter,omitempty"`
	BonusDamage       int              `json:"bonusDamage,omitempty"`
	MPGranted         int              `json:"mpGranted,omitempty"`
	EnergyGranted     int              `json:"energyGranted,omitempty"`
	EnergyDrained     int              `json:"energyDrained,omitempty"`
	Amount            int              `json:"amount,omitempty"`
	SourceEnergyAfter int              `json:"sourceEnergyAfter,omitempty"`
	TargetEnergyAfter int              `json:"targetEnergyAfter,omitempty"`
}

type attackMode string

type stats struct {
	Rules *balance.V34ResearchConfig
	Attack        int
	Defense       int
	Agility       int
	Health        int
	MaxHealth     int
	IsRanged      bool
	AttackRange   int
	CharacterType string
	X             int
	Y             int
}

type attackResult struct {
	Damage    int
	NewHealth int
	Triggered bool
}

const (
	CombatActorPlayer  CombatActorType = "player"
	CombatActorMonster CombatActorType = "monster"
)

const (
	AttackStyleMelee  AttackStyle = "melee"
	AttackStyleRanged AttackStyle = "ranged"
	AttackStyleMagic  AttackStyle = "magic"
)

const (
	attackModeMelee  attackMode = "melee"
	attackModeRanged attackMode = "ranged"
)

func resolvePresentationAttackStyle(attacker stats, mode attackMode) AttackStyle {
	if attacker.CharacterType == "mystic" {
		return AttackStyleMagic
	}
	if mode == attackModeRanged {
		return AttackStyleRanged
	}
	return AttackStyleMelee
}

func resolveAttackEnergyCost(mode attackMode) int {
	if mode == attackModeRanged {
		return rangedAttackEnergyCost
	}
	return meleeAttackEnergyCost
}

func resolvePrimaryDamage(att stats, def stats) int {
	dmg := balance.DefenseDamage(att.Attack, def.Defense)
	if att.Rules!=nil {dmg=balance.V34DefenseDamage(att.Attack,def.Defense,att.Rules)}
	return resolveWoundedDamage(att,def,dmg)
}

func resolveWoundedDamage(att stats, def stats, dmg int) int {
	if att.CharacterType == "berserker" && def.MaxHealth > 0 && dmg > 0 {
		hpRatio := float64(def.Health) / float64(def.MaxHealth)
		bonusMultiplier := 1.0
		switch {
		case hpRatio < 0.25:
			bonusMultiplier = 1.35
		case hpRatio < 0.50:
			bonusMultiplier = 1.20
		case hpRatio < 0.75:
			bonusMultiplier = 1.10
		}
		dmg = int(float64(dmg) * bonusMultiplier)
	}
	return dmg
}

func resolveCounterDamage(att stats, def stats) attackResult {
	if att.Rules==nil {return applyDamage(att,def)}
	damage:=balance.ResearchRound(float64(balance.V34DefenseDamage(att.Attack,def.Defense,att.Rules))*att.Rules.Factors["counter"])
	return applyFlatDamage(def.Health,resolveWoundedDamage(att,def,damage))
}

func classReflexChance(owner,opponent stats) int {
	if owner.Rules!=nil {return balance.CalculateReflexProcChance(owner.Agility)}
	return balance.ClassReflexChance(owner.Agility,owner.CharacterType,opponent.CharacterType,true)
}

func researchEffects(effects []CombatEffect, additions []balance.V34Effect, source,target CombatTargetRef) []CombatEffect {
	for _, effect:=range additions {
		ref:=target
		if effect.Kind=="hatred" || effect.Kind=="armorFocus" {ref=source}
		effects=append(effects,CombatEffect{Kind:effect.Kind,Source:&source,Target:&ref,Amount:effect.Amount,Succeeded:true})
	}
	return effects
}

func applyDamage(att stats, def stats) attackResult {
	dmg := resolvePrimaryDamage(att, def)
	newHP := def.Health - dmg
	if newHP < 0 {
		newHP = 0
	}
	return attackResult{Damage: dmg, NewHealth: newHP, Triggered: true}
}

func defaultRollReflexProc(chance int) bool {
	return chance > 0 && rand.Intn(100) < chance
}

func resolveGuardianShieldBlock(targetType string, defender stats, rolls ...func(int) bool) bool {
	if targetType != "player" || defender.CharacterType != "guardian" {
		return false
	}
	chance:=balance.CalculateReflexProcChance(defender.Agility)
	if defender.Rules!=nil {chance=balance.ResearchRound(float64(chance)*defender.Rules.Factors["block"]);if chance>55 {chance=55}}
	roll:=rollReflexProc
	if len(rolls)>0 {roll=rolls[0]}
	return roll(chance)
}

func actualDamageDealt(healthBefore int, healthAfter int) int {
	damage := healthBefore - healthAfter
	if damage < 0 {
		return 0
	}
	return damage
}

func tryApplyRangerCriticalShot(attacker stats, targetHealth int, result attackResult) (attackResult, bool) {
	if attacker.CharacterType != "ranger" || !result.Triggered || result.Damage <= 0 {
		return result, false
	}
	if !rollReflexProc(balance.CalculateReflexProcChance(attacker.Agility)) {
		return result, false
	}

	result.Damage = (result.Damage * rangerCriticalDamagePercent) / 100
	result.NewHealth = targetHealth - result.Damage
	if result.NewHealth < 0 {
		result.NewHealth = 0
	}
	return result, true
}

func applyFlatDamage(targetHealth int, damage int) attackResult {
	if damage < 0 {
		damage = 0
	}
	newHP := targetHealth - damage
	if newHP < 0 {
		newHP = 0
	}
	return attackResult{Damage: damage, NewHealth: newHP, Triggered: true}
}
