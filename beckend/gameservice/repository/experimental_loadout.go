package repository

import "gameservice/internal/game/balance"

// Historical fractional model for configurations without equipment_model.
// Seven equal logical slots. A two-handed weapon consumes both hand shares;
// a one-handed weapon and shield consume one each. Helmet/chest/pants/gloves/
// boots consume one each. Gray=0, green=1, blue/purple/orange=2, capped at 14/7.
// Sum rational shares BEFORE rounding once per stat: partial gear cannot receive
// phantom set buffs. Jewelry keeps its actual bonuses/2-of-2 outside this budget.
func normalizedV34Loadout(e *CharacterEffectiveStats, cfg *balance.V34ResearchConfig) (EquipmentStats, balance.CombatProfile) {
	base := ResolveHeroClassStatsForRuleset(e.HeroClassID, balance.ExperimentalV34)
	index := 0
	switch e.HeroClassID {
	case "berserker":
		index = 1
	case "ranger":
		index = 2
	case "mystic":
		index = 3
	}
	rawBase, rawEffective := rawEquipmentStats(e)
	stats := rawBase
	stats.MaxHealth, stats.Attack, stats.Defense, stats.Agility = cfg.BaseHP[index], cfg.BaseAttack[index], cfg.BaseDefense[index], base.Agility
	shares := 0
	jewelryCounts := map[string]int{}
	for slot, item := range e.Equipped {
		if item == nil || item.Slot != slot || (item.ClassRestriction != "" && item.ClassRestriction != e.HeroClassID) {
			continue
		}
		if slot == "ring" || slot == "amulet" {
			addEquipmentBonuses(&stats, v34ItemBonuses(item.Bonuses, slot, item.Rarity, item.Handedness, cfg))
			if _, set, ok := jewelryItem(item.Code); ok {
				jewelryCounts[set.Code]++
			}
			continue
		}
		if !isValidEquipmentSlot(slot) {
			continue
		}
		if slot == "off_hand" {
			if main := e.Equipped["main_hand"]; main != nil && main.Handedness == "two_hand" {
				continue
			}
		}
		rank := 0
		switch item.Rarity {
		case "green":
			rank = 1
		case "blue", "purple", "orange":
			rank = 2
		}
		weight := 1
		if slot == "main_hand" && item.Handedness == "two_hand" {
			weight = 2
		}
		shares += rank * weight
		if cfg.EquipmentModel == "slots_v2" || cfg.EquipmentModel == "slots_v3_reflex" {
			b := v34ItemBonuses(item.Bonuses, slot, item.Rarity, item.Handedness, cfg)
			stats.MaxHealth += b.MaxHealth
			stats.Attack += b.Attack
			stats.Defense += b.Defense
			stats.Agility += b.Agility
		}
	}
	if shares > 14 {
		shares = 14
	}
	if cfg.EquipmentModel != "slots_v2" && cfg.EquipmentModel != "slots_v3_reflex" {
		stats.MaxHealth += balance.ResearchRound(float64(cfg.GearHP*shares) / 7)
		stats.Attack += balance.ResearchRound(float64(cfg.GearAttack*shares) / 7)
		stats.Defense += balance.ResearchRound(float64(cfg.GearDefense*shares) / 7)
		stats.Agility += balance.ResearchRound(float64(cfg.GearMain*shares) / 7)
	}
	for _, set := range jewelryCatalog.Sets {
		if jewelryCounts[set.Code] >= 2 {
			addEquipmentBonuses(&stats, set.PairBonus)
		}
	}
	stats.Mobility, stats.SightRange, stats.AttackRange = rawEffective.Mobility, rawEffective.SightRange, rawEffective.AttackRange
	stats.MaxMP = stats.Mobility * 2
	if stats.MaxMP > HexMPCap {
		stats.MaxMP = HexMPCap
	}
	p := balance.DefaultCombatProfile()
	p.Ruleset = balance.ExperimentalV34
	p.GearLevel = shares * 1000 / 7
	p.CriticalMultiplier = cfg.Factors["crit"]
	p.ClassHealPercent = 50 * cfg.Factors["heal"]
	// Experimental abilities follow research PARAM, not legacy slot ability upgrades.
	return stats, p
}

func legacySnapshotStats(e *CharacterEffectiveStats) EquipmentStats {
	rawBase, stats := rawEquipmentStats(e)
	// New registrations may have v34 base rows. A later rollback still creates
	// legacy matches from the legacy base; ordinary owned item stats stay intact.
	experimental := ResolveHeroClassStatsForRuleset(e.HeroClassID, balance.ExperimentalV34)
	if rawBase.MaxHealth == experimental.MaxHealth && rawBase.Attack == experimental.Attack && rawBase.Defense == experimental.Defense {
		base := ResolveHeroClassStats(e.HeroClassID)
		stats.MaxHealth += base.MaxHealth - experimental.MaxHealth
		stats.Attack += base.Attack - experimental.Attack
		stats.Defense += base.Defense - experimental.Defense
	}
	return stats
}
