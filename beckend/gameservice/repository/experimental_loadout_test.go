package repository

import (
	"encoding/json"
	"gameservice/internal/game/balance"
	"testing"
)

func TestV34NormalizedActualSlots(t *testing.T) {
	cfg := balance.ExperimentalV34Config()
	for _, row := range []struct {
		name              string
		rank              int
		twoHand           bool
		hp, atk, def, ref int
	}{
		{"empty", 0, false, 218, 12, 16, 5},
		{"green_shield", 1, false, 238, 15, 19, 12},
		{"green_two_hand", 1, true, 238, 15, 19, 12},
		{"blue_shield", 2, false, 258, 18, 22, 15},
	} {
		t.Run(row.name, func(t *testing.T) {
			e := &CharacterEffectiveStats{HeroClassID: "guardian", BaseStats: EquipmentStats{}, EffectiveStats: EquipmentStats{Mobility: 2, SightRange: 2, AttackRange: 1}, Equipped: map[string]*EquipmentItem{}}
			if row.rank > 0 {
				rarity := "green"
				if row.rank == 2 {
					rarity = "blue"
				}
				for _, slot := range []string{"helmet", "chest", "pants", "gloves", "boots", "main_hand", "off_hand"} {
					if row.twoHand && slot == "off_hand" {
						continue
					}
					item := &EquipmentItem{Slot: slot, Rarity: rarity, ClassRestriction: "guardian", Bonuses: EquipmentBonuses{MaxHealth: 999, Attack: 999}}
					if row.twoHand && slot == "main_hand" {
						item.Handedness = "two_hand"
					}
					e.Equipped[slot] = item
				}
			}
			got, p := normalizedV34Loadout(e, &cfg)
			if got.MaxHealth != row.hp || got.Attack != row.atk || got.Defense != row.def || got.Agility != row.ref || got.MaxMP != 4 || p.Ruleset != balance.ExperimentalV34 {
				t.Fatalf("snapshot: %+v / %+v", got, p)
			}
		})
	}
	e := &CharacterEffectiveStats{HeroClassID: "guardian", Equipped: map[string]*EquipmentItem{"chest": {Slot: "chest", Rarity: "green"}}, EffectiveStats: EquipmentStats{Mobility: 2}}
	partial, _ := normalizedV34Loadout(e, &cfg)
	if partial.MaxHealth != 224 || partial.Attack != 12 || partial.Defense != 17 || partial.Agility != 6 {
		t.Fatalf("phantom partial bonus: %+v", partial)
	}
	e.Equipped["pants"] = &EquipmentItem{Slot: "pants", Rarity: "blue"}
	e.Equipped["ring"] = &EquipmentItem{Slot: "ring", Rarity: "blue", Bonuses: EquipmentBonuses{Attack: 2}}
	mixed, _ := normalizedV34Loadout(e, &cfg)
	if mixed.MaxHealth != 232 || mixed.Attack != 14 || mixed.Defense != 19 || mixed.Agility != 9 {
		t.Fatalf("mixed/jewelry double count: %+v", mixed)
	}
}

func TestV34EquipmentProjectionCompatibility(t *testing.T) {
	cfg := balance.ExperimentalV34Config()
	raw := EquipmentBonuses{MaxHealth: 99, Attack: 77, Defense: 55, Agility: 33, Mobility: 1, PocketBonus: 1}
	for _, rarity := range []string{"gray", "green", "blue", "purple", "orange"} {
		got := v34ItemBonuses(raw, "chest", rarity, "one_hand", &cfg)
		want := raw
		switch rarity {
		case "green":
			want.MaxHealth = 6
			want.Attack = 0
			want.Defense = 1
			want.Agility = 1
		case "blue", "purple", "orange":
			want.MaxHealth = 12
			want.Attack = 0
			want.Defense = 2
			want.Agility = 2
		}
		if got != want {
			t.Fatalf("rarity %s %+v want %+v", rarity, got, want)
		}
	}
	e := &CharacterEffectiveStats{HeroClassID: "guardian", Equipped: map[string]*EquipmentItem{"main_hand": {Slot: "main_hand", Rarity: "green", Handedness: "two_hand"}, "off_hand": {Slot: "off_hand", Rarity: "blue"}}}
	stats, _ := normalizedV34Loadout(e, &cfg)
	if stats.MaxHealth != 222 || stats.Attack != 15 || stats.Defense != 17 {
		t.Fatalf("phantom off-hand: %+v", stats)
	}
	// Previously persisted experimental configurations remain readable and retain
	// their fractional model; no implicit map/version is inserted during decode.
	cfg.MysticMagicThroughBlock = false
	cfg.RareSlotReflex = nil
	cfg.JewelryReflex = nil
	cfg.GearMain = 2
	cfg.EquipmentModel = ""
	cfg.SlotBudgets = nil
	rawConfig, err := json.Marshal(cfg)
	if err != nil {
		t.Fatal(err)
	}
	frozen, err := DecodeMatchCombatConfig(balance.ExperimentalV34, rawConfig)
	if err != nil {
		t.Fatal(err)
	}
	old := &CharacterEffectiveStats{HeroClassID: "guardian", Equipped: map[string]*EquipmentItem{"chest": {Slot: "chest", Rarity: "green"}}}
	historic, _ := normalizedV34Loadout(old, frozen)
	if historic.MaxHealth != 221 || historic.Defense != 16 {
		t.Fatalf("historical model changed %+v", historic)
	}
	cfg = balance.ExperimentalV34Config()
	delete(cfg.SlotBudgets, "chest")
	invalid, _ := json.Marshal(cfg)
	if _, err = DecodeMatchCombatConfig(balance.ExperimentalV34, invalid); err == nil {
		t.Fatal("incomplete frozen slot config accepted")
	}
	// Serialization uses the same projection while leaving the stored structs intact.
	t.Setenv("NACHALO_COMBAT_RULESET", "")
	for _, set := range equipmentSetsCatalog {
		for _, item := range set.Items {
			data, err := json.Marshal(item)
			if err != nil {
				t.Fatal(err)
			}
			var card EquipmentCatalogItem
			if err = json.Unmarshal(data, &card); err != nil {
				t.Fatal(err)
			}
			cfg := balance.ExperimentalV34Config()
			want := v34ItemBonuses(item.Bonuses, item.Slot, set.Rarity, catalogItemHandedness(item), &cfg)
			if card.Bonuses != want || card.CombatBonuses != (balance.CombatItemBonuses{}) {
				t.Fatalf("catalog card %s %+v", item.Code, card)
			}
		}
	}
	t.Setenv("NACHALO_COMBAT_RULESET", "legacy")
	item := EquipmentItem{Slot: "chest", Rarity: "green", Bonuses: raw}
	data, err := json.Marshal(item)
	if err != nil {
		t.Fatal(err)
	}
	var legacy EquipmentItem
	if err = json.Unmarshal(data, &legacy); err != nil {
		t.Fatal(err)
	}
	if legacy.Bonuses != raw {
		t.Fatal("legacy UI silently normalized")
	}
}

func TestV34ReflexProgressionPreservesOtherItemStats(t *testing.T) {
	current := balance.ExperimentalV34Config()
	old := balance.ExperimentalV34Config()
	old.EquipmentModel = "slots_v2"
	old.GearMain = 2
	old.JewelryReflex = nil
	old.RareSlotReflex = nil
	old.MysticMagicThroughBlock = false
	for slot, b := range old.SlotBudgets {
		b.Reflex = 0
		if slot == "helmet" || slot == "boots" {
			b.Reflex = 1
		}
		old.SlotBudgets[slot] = b
	}
	changed, unchanged := 0, 0
	for _, set := range equipmentSetsCatalog {
		for _, item := range set.Items {
			before := v34ItemBonuses(item.Bonuses, item.Slot, set.Rarity, catalogItemHandedness(item), &old)
			after := v34ItemBonuses(item.Bonuses, item.Slot, set.Rarity, catalogItemHandedness(item), &current)
			if before.Agility == after.Agility {
				unchanged++
			} else {
				changed++
			}
			before.Agility = after.Agility
			if before != after {
				t.Fatalf("non-Reflex item change %s: %+v vs %+v", item.Code, before, after)
			}
		}
	}
	if changed != 34 || unchanged != 16 {
		t.Fatalf("unexpected item scope %d changed %d unchanged", changed, unchanged)
	}
	for _, set := range jewelryCatalog.Sets {
		for _, item := range set.Items {
			after := v34ItemBonuses(item.Bonuses, item.Slot, set.Rarity, "none", &current)
			if after != item.Bonuses {
				t.Fatalf("jewelry catalog drift %s", item.Code)
			}
		}
	}
	for _, set := range equipmentSeedCatalog {
		for _, item := range set.Items {
			if got := v34ItemBonuses(item.Bonuses, item.Slot, item.Rarity, item.Handedness, &current); got != item.Bonuses {
				t.Fatalf("Common changed %s", item.Code)
			}
		}
	}
	if balance.CalculateReflexProcChance(30) != 22 {
		t.Fatal("Reflex progression unexpectedly capped at 20")
	}
}
