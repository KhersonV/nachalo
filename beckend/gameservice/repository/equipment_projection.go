package repository

import (
	"encoding/json"
	"gameservice/internal/game/balance"
	"os"
)

func configuredEquipmentRuleset() (balance.CombatRuleset, error) {
	return balance.ParseCombatRuleset(os.Getenv("NACHALO_COMBAT_RULESET"))
}

func rawEquipmentStats(e *CharacterEffectiveStats) (EquipmentStats, EquipmentStats) {
	base, effective := e.BaseStats, e.EffectiveStats
	if e.RawBaseStats != nil {
		base = *e.RawBaseStats
	}
	if e.RawEffectiveStats != nil {
		effective = *e.RawEffectiveStats
	}
	return base, effective
}

// Only core combat stats are projected. Movement, range, pockets and jewelry
// retain their existing semantics. This never changes an instance/template row.
func v34ItemBonuses(raw EquipmentBonuses, slot, rarity, handedness string, cfg *balance.V34ResearchConfig) EquipmentBonuses {
	if bySlot := cfg.JewelryReflex[rarity]; bySlot != nil {
		if reflex, ok := bySlot[slot]; ok {
			raw.Agility = reflex
			return raw
		}
	}
	b, ok := cfg.SlotBudgets[slot]
	if !ok {
		return raw
	}
	rank := 0
	switch rarity {
	case "green":
		rank = 1
	case "blue", "purple", "orange":
		rank = 2
	default:
		return raw
	}
	// Rare Reflex has its own integer allocation; HP/ATK/DEF still double.
	if cfg.EquipmentModel == "slots_v3_reflex" && rank == 2 {
		b.Reflex = cfg.RareSlotReflex[slot]
	}
	if slot == "main_hand" && handedness == "two_hand" {
		other := cfg.SlotBudgets["off_hand"]
		if cfg.EquipmentModel == "slots_v3_reflex" && rank == 2 {
			other.Reflex = cfg.RareSlotReflex["off_hand"]
		}
		b.HP += other.HP
		b.Attack += other.Attack
		b.Defense += other.Defense
		b.Reflex += other.Reflex
	}
	raw.MaxHealth = b.HP * rank
	raw.Attack = b.Attack * rank
	raw.Defense = b.Defense * rank
	raw.Agility = b.Reflex * rank
	if cfg.EquipmentModel == "slots_v3_reflex" && rank == 2 {
		raw.Agility = b.Reflex
	}
	return raw
}

func catalogItemHandedness(item EquipmentCatalogItem) string {
	if item.Handedness != "" {
		return item.Handedness
	}
	// Authored previews use the very same base-template metadata as the seeder.
	for _, set := range equipmentSeedCatalog {
		for _, base := range set.Items {
			if base.Code == item.BaseTemplate {
				return base.Handedness
			}
		}
	}
	return ""
}

// JSON projection is deliberately separate from DB scanning: trading,
// legacy snapshots and catalog validation always see the original stored data.
func (item EquipmentItem) MarshalJSON() ([]byte, error) {
	type plain EquipmentItem
	rules, err := configuredEquipmentRuleset()
	if err != nil {
		return nil, err
	}
	if rules != balance.ExperimentalV34 {
		return json.Marshal(plain(item))
	}
	cfg := balance.ExperimentalV34Config()
	item.Bonuses = v34ItemBonuses(item.Bonuses, item.Slot, item.Rarity, item.Handedness, &cfg)
	item.CombatBonuses = balance.CombatItemBonuses{}
	return json.Marshal(struct {
		plain
		Ruleset balance.CombatRuleset `json:"combatRuleset"`
		Model   string                `json:"equipmentModel"`
	}{plain(item), rules, cfg.EquipmentModel})
}

func (item EquipmentCatalogItem) MarshalJSON() ([]byte, error) {
	type plain EquipmentCatalogItem
	rules, err := configuredEquipmentRuleset()
	if err != nil {
		return nil, err
	}
	if rules == balance.ExperimentalV34 {
		_, set, ok := equipmentCatalogItem(item.Code)
		if ok {
			cfg := balance.ExperimentalV34Config()
			item.Bonuses = v34ItemBonuses(item.Bonuses, item.Slot, set.Rarity, catalogItemHandedness(item), &cfg)
		}
		item.CombatBonuses = balance.CombatItemBonuses{}
	}
	return json.Marshal(plain(item))
}

func (recipe CraftRecipe) MarshalJSON() ([]byte, error) {
	type plain CraftRecipe
	rules, err := configuredEquipmentRuleset()
	if err != nil {
		return nil, err
	}
	if rules == balance.ExperimentalV34 {
		cfg := balance.ExperimentalV34Config()
		handedness := recipe.Handedness
		if handedness == "" {
			if item, _, ok := equipmentCatalogItem(recipe.Code); ok {
				handedness = catalogItemHandedness(item)
			}
		}
		recipe.Bonuses = v34ItemBonuses(recipe.Bonuses, recipe.Slot, recipe.Rarity, handedness, &cfg)
		recipe.CombatBonuses = balance.CombatItemBonuses{}
	}
	return json.Marshal(plain(recipe))
}

func projectMarketItem(item MarketItem) (MarketItem, error) {
	rules, err := configuredEquipmentRuleset()
	if err != nil {
		return item, err
	}
	if rules == balance.ExperimentalV34 && item.Kind == "equipment" {
		slot, handedness := "", ""
		// These metadata are included in every equipment attributes query, including
		// old snapshots and templates not in the authored set catalogue.
		for i, name := range []string{"helmet", "chest", "pants", "boots", "gloves", "main_hand", "off_hand", "ring", "amulet"} {
			if item.Attributes["_slot"] == i+1 {
				slot = name
			}
		}
		if item.Attributes["_twoHand"] == 1 {
			handedness = "two_hand"
		}
		raw := EquipmentBonuses{MaxHealth: item.Attributes["health"], Attack: item.Attributes["attack"], Defense: item.Attributes["defense"], Agility: item.Attributes["agility"]}
		cfg := balance.ExperimentalV34Config()
		b := v34ItemBonuses(raw, slot, item.Rarity, handedness, &cfg)
		attrs := make(map[string]int, len(item.Attributes))
		for k, v := range item.Attributes {
			attrs[k] = v
		}
		attrs["health"] = b.MaxHealth
		attrs["attack"] = b.Attack
		attrs["defense"] = b.Defense
		attrs["agility"] = b.Agility
		item.Attributes = attrs
	}
	// Internal slot metadata are never exposed as numeric item attributes.
	if item.Attributes != nil {
		attrs := make(map[string]int, len(item.Attributes))
		for k, v := range item.Attributes {
			if k != "_slot" && k != "_twoHand" {
				attrs[k] = v
			}
		}
		item.Attributes = attrs
	}
	return item, nil
}
