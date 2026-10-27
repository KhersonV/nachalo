package repository

import (
	"context"
	"encoding/json"
	"gameservice/internal/game/balance"
)

func combatBonusScore(b EquipmentBonuses) int {
	score := 0
	for _, v := range []int{b.MaxHealth, b.Attack, b.Defense, b.Agility} {
		if v > 0 {
			score += v
		}
	}
	return score
}

func mainEquipmentSet(code string) bool {
	for _, set := range equipmentSetsCatalog {
		if set.Code == code {
			return true
		}
	}
	return false
}

// Existing instances keep their stat snapshots and legacy stat set bonuses.
// Current catalog items carry those stats individually instead of activating them twice.
func legacyMainEquipment(item *EquipmentItem) bool {
	current, _, ok := equipmentCatalogItem(item.Code)
	if !ok {
		return true
	}
	a, b := item.Bonuses, current.Bonuses
	return a.Attack != b.Attack || a.Defense != b.Defense || a.MaxHealth != b.MaxHealth || a.Agility != b.Agility
}

func characterCombatProfile(class string, equipped map[string]*EquipmentItem) balance.CombatProfile {
	p := balance.DefaultCombatProfile()
	score, green, blue := 0, 0, 0
	for _, set := range equipmentSetsCatalog {
		if set.HeroClass != class {
			continue
		}
		for _, item := range set.Items {
			if set.Rarity == "green" {
				green += combatBonusScore(item.Bonuses)
			}
			if set.Rarity == "blue" {
				blue += combatBonusScore(item.Bonuses)
			}
		}
	}
	for slot, item := range equipped {
		if item == nil || slot == "ring" || slot == "amulet" || !isValidEquipmentSlot(slot) || item.Slot != slot {
			continue
		}
		if item.ClassRestriction != "" && item.ClassRestriction != class {
			continue
		}
		score += combatBonusScore(item.Bonuses)
		current, set, ok := equipmentCatalogItem(item.Code)
		if !ok || set.HeroClass != class || current.Slot != slot || !mainEquipmentSet(set.Code) {
			continue
		}
		b := current.CombatBonuses
		p.AuraRadius += b.AuraRadius
		p.AuraTick += b.AuraTick
		p.AuraExit += b.AuraExit
		p.HealPercent += b.HealPercent
		p.FollowupDamage += b.FollowupDamage
		p.CriticalPercent += b.CriticalPercent
		p.OverloadSecond += b.OverloadSecond
		p.OverloadThird += b.OverloadThird
		p.MysticReflexDamage += b.MysticReflexDamage
	}
	// Linear effective HP+ATK+DEF+Reflex bonus budget: naked=0, green=1000,
	// blue=2000. Actual instance stats (including legacy snapshots) determine partial
	// and mixed gear; jewelry, set names and last-piece activation never determine it.
	if green > 0 && score <= green {
		p.GearLevel = score * 1000 / green
	} else if blue > green {
		p.GearLevel = 1000 + (score-green)*1000/(blue-green)
	}
	if p.GearLevel > 2000 {
		p.GearLevel = 2000
	}
	return p
}

func ReadHexCombatProfileTx(ctx context.Context, tx storageQuerier, instance string, user int) (balance.CombatProfile, error) {
	p := balance.DefaultCombatProfile()
	var raw []byte
	if err := tx.QueryRowContext(ctx, `SELECT hex_combat_balance FROM match_players WHERE instance_id=$1 AND user_id=$2`, instance, user).Scan(&raw); err != nil {
		return p, err
	}
	// Old matches have {} and retain their already stored ordinary stats.
	err := json.Unmarshal(raw, &p)
	return p, err
}
