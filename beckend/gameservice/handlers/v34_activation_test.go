package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"testing"

	"gameservice/internal/game/balance"
	"gameservice/repository"
)

// No duels: exercise the real DB-backed equipment API and match snapshots.
func TestV34ActivationEquipmentPostgres(t *testing.T) {
	if os.Getenv("STAGE2_TEST_DSN") != "postgres://v34_validation@127.0.0.1:55434/nachalo_v34_validation?sslmode=disable" {
		t.Skip("dedicated validation cluster required")
	}
	t.Setenv("NACHALO_COMBAT_RULESET", "legacy")
	db, _ := hexCombatDatabase(t)
	t.Setenv("NACHALO_COMBAT_RULESET", "") // default activation, not an explicit opt-in
	ctx := context.Background()
	sets, err := repository.EquipmentSetsCatalog(ctx)
	if err != nil {
		t.Fatal(err)
	}
	expected := map[string][4]int{"helmet": {2, 0, 0, 1}, "chest": {6, 0, 1, 1}, "pants": {4, 0, 1, 1}, "boots": {2, 0, 0, 1}, "gloves": {2, 0, 0, 1}, "main_hand": {2, 3, 0, 1}, "off_hand": {2, 0, 1, 1}}
	classes := []string{"guardian", "berserker", "ranger", "mystic"}
	for index, class := range classes {
		t.Run(class, func(t *testing.T) {
			user := 781 + index
			if index > 0 {
				if _, err := repository.CreatePlayerProfile(repository.CreatePlayerProfileInput{UserID: user, Name: fmt.Sprintf("Activation %s", class), CharacterType: class}); err != nil {
					t.Fatal(err)
				}
			}
			char, err := repository.GetSelectedCharacterForUser(user)
			if err != nil {
				t.Fatal(err)
			}
			base := repository.ResolveHeroClassStatsForRuleset(class, balance.ExperimentalV34)
			check := func(want [4]int) {
				t.Helper()
				e, err := repository.GetCharacterEffectiveStats(char.ID)
				if err != nil {
					t.Fatal(err)
				}
				got := [4]int{e.EffectiveStats.MaxHealth - base.MaxHealth, e.EffectiveStats.Attack - base.Attack, e.EffectiveStats.Defense - base.Defense, e.EffectiveStats.Agility - base.Agility}
				if got != want {
					t.Fatalf("equipment bonuses got %v want %v", got, want)
				}
				if e.BaseStats.MaxHealth != base.MaxHealth || e.BaseStats.Attack != base.Attack || e.BaseStats.Defense != base.Defense || e.CombatProfile.Ruleset != balance.ExperimentalV34 {
					t.Fatalf("base/profile %+v", e)
				}
			}
			check([4]int{})
			var greenChest, blueChest string
			for _, set := range sets {
				if set.HeroClass != class {
					continue
				}
				rank := 1
				if set.Rarity == "blue" {
					rank = 2
				}
				total := [4]int{}
				for _, preview := range set.Items {
					item, err := repository.GrantItemInstanceToUser(user, preview.Code, "shop")
					if err != nil {
						t.Fatal(err)
					}
					if item.Slot == "chest" {
						if rank == 1 {
							greenChest = item.InstanceID
						} else {
							blueChest = item.InstanceID
						}
					}
					part := expected[item.Slot]
					if item.Slot == "main_hand" && class != "guardian" {
						part[0] += 2
						part[2]++
						part[3]++
					}
					for i := range part {
						if i < 3 {
							part[i] *= rank
						}
					}
					if rank == 2 && (item.Slot == "helmet" || item.Slot == "chest" || item.Slot == "boots") {
						part[3] = 2
					}
					for i := range part {
						total[i] += part[i]
					}
					rawBonuses := item.Bonuses
					data, err := json.Marshal(item)
					if err != nil {
						t.Fatal(err)
					}
					var card struct {
						Bonuses repository.EquipmentBonuses `json:"bonuses"`
					}
					if err = json.Unmarshal(data, &card); err != nil {
						t.Fatal(err)
					}
					got := [4]int{card.Bonuses.MaxHealth, card.Bonuses.Attack, card.Bonuses.Defense, card.Bonuses.Agility}
					if got != part || item.Bonuses != rawBonuses {
						t.Fatalf("card projection mutated snapshot / wrong values: %v %v", got, part)
					}
					// Every slot alone must contribute exactly its own advertised values.
					if err = repository.EquipItemToCharacter(user, char.ID, item.InstanceID); err != nil {
						t.Fatal(err)
					}
					check(part)
					if err = repository.UnequipItemFromCharacter(user, char.ID, item.Slot); err != nil {
						t.Fatal(err)
					}
					check([4]int{})
				}
				reflex := 7
				if rank == 2 {
					reflex = 10
				}
				if total != [4]int{20 * rank, 3 * rank, 3 * rank, reflex} {
					t.Fatalf("catalog totals %v", total)
				}
				// Assemble with the same already-owned items, without granting duplicates.
				owned, err := repository.GetEquipmentInventoryForUser(user)
				if err != nil {
					t.Fatal(err)
				}
				for _, item := range owned {
					if item.SetCode == set.Code {
						if err = repository.EquipItemToCharacter(user, char.ID, item.InstanceID); err != nil {
							t.Fatal(err)
						}
					}
				}
				check(total)
				snapshot, err := repository.CreateHexTestMatch(ctx, user, 97)
				if err != nil {
					t.Fatal(err)
				}
				var attack, defense int
				if err = db.QueryRow(`SELECT attack,defense FROM match_players WHERE instance_id=$1 AND user_id=$2`, snapshot.InstanceID, user).Scan(&attack, &defense); err != nil {
					t.Fatal(err)
				}
				if snapshot.Player.MaxHealth != base.MaxHealth+total[0] || attack != base.Attack+total[1] || defense != base.Defense+total[2] {
					t.Fatalf("double application in match %+v", snapshot.Player)
				}
				var rules, model string
				if err = db.QueryRow(`SELECT combat_ruleset,combat_config->>'equipment_model' FROM matches WHERE instance_id=$1`, snapshot.InstanceID).Scan(&rules, &model); err != nil {
					t.Fatal(err)
				}
				if rules != "experimental_v34" || model != "slots_v3_reflex" {
					t.Fatal("default/frozen slot config missing")
				}
				for slot := range expected {
					if err = repository.UnequipItemFromCharacter(user, char.ID, slot); err != nil {
						t.Fatal(err)
					}
				}
				check([4]int{})
			}

			owned, err := repository.GetEquipmentInventoryForUser(user)
			if err != nil {
				t.Fatal(err)
			}
			var bluePants string
			for _, item := range owned {
				if item.Slot == "pants" && item.Rarity == "blue" {
					bluePants = item.InstanceID
				}
			}
			if err = repository.EquipItemToCharacter(user, char.ID, greenChest); err != nil {
				t.Fatal(err)
			}
			if err = repository.EquipItemToCharacter(user, char.ID, bluePants); err != nil {
				t.Fatal(err)
			}
			check([4]int{14, 0, 3, 2})
			for _, slot := range []string{"main_hand", "off_hand"} {
				if slot == "off_hand" && class != "guardian" {
					continue
				}
				for _, rarity := range []string{"green", "blue"} {
					for _, item := range owned {
						if item.Slot != slot || item.Rarity != rarity {
							continue
						}
						if err = repository.EquipItemToCharacter(user, char.ID, item.InstanceID); err != nil {
							t.Fatal(err)
						}
						part := expected[slot]
						if slot == "main_hand" && class != "guardian" {
							part[0] += 2
							part[2]++
							part[3]++
						}
						if rarity == "blue" {
							for i := range part {
								if i < 3 {
									part[i] *= 2
								}
							}
						}
						check([4]int{14 + part[0], part[1], 3 + part[2], 2 + part[3]})
					}
				}
				if err = repository.UnequipItemFromCharacter(user, char.ID, slot); err != nil {
					t.Fatal(err)
				}
				check([4]int{14, 0, 3, 2})
			}
			if err = repository.UnequipItemFromCharacter(user, char.ID, "pants"); err != nil {
				t.Fatal(err)
			}
			if err = repository.UnequipItemFromCharacter(user, char.ID, "chest"); err != nil {
				t.Fatal(err)
			}
			t.Setenv("NACHALO_COMBAT_RULESET", "legacy")
			legacyBase := repository.ResolveHeroClassStats(class)
			e, err := repository.GetCharacterEffectiveStats(char.ID)
			if err != nil {
				t.Fatal(err)
			}
			if e.EffectiveStats.MaxHealth != legacyBase.MaxHealth || e.EffectiveStats.Attack != legacyBase.Attack || e.EffectiveStats.Defense != legacyBase.Defense {
				t.Fatalf("legacy rollback projection %+v", e.EffectiveStats)
			}
			t.Setenv("NACHALO_COMBAT_RULESET", "")
			check([4]int{})
			// Same-slot swap replaces the bonus, then removing it restores the base.
			if err = repository.EquipItemToCharacter(user, char.ID, greenChest); err != nil {
				t.Fatal(err)
			}
			check([4]int{6, 0, 1, 1})
			if err = repository.EquipItemToCharacter(user, char.ID, blueChest); err != nil {
				t.Fatal(err)
			}
			check([4]int{12, 0, 2, 2})
			if err = repository.UnequipItemFromCharacter(user, char.ID, "chest"); err != nil {
				t.Fatal(err)
			}
			check([4]int{})
		})
	}
	// Old event snapshots stay stored as-is, but use current per-slot v34 rules.
	char, err := repository.GetSelectedCharacterForUser(781)
	if err != nil {
		t.Fatal(err)
	}
	old, err := repository.GrantItemInstanceToUser(781, "aegiswarden_vest_uncommon", "shop")
	if err != nil {
		t.Fatal(err)
	}
	if _, err = db.Exec(`UPDATE item_instance_events SET metadata=metadata || '{"equipmentBalance1044":{"max_health_bonus":999,"attack_bonus":777}}'::jsonb WHERE item_instance_id=$1 AND event_type='created'`, old.InstanceID); err != nil {
		t.Fatal(err)
	}
	var before, after string
	read := func() string {
		var s string
		if err := db.QueryRow(`SELECT to_jsonb(ii)::text || (SELECT jsonb_agg(to_jsonb(e) ORDER BY e.id)::text FROM item_instance_events e WHERE e.item_instance_id=ii.id) FROM item_instances ii WHERE id=$1`, old.InstanceID).Scan(&s); err != nil {
			t.Fatal(err)
		}
		return s
	}
	if err = repository.EquipItemToCharacter(781, char.ID, old.InstanceID); err != nil {
		t.Fatal(err)
	}
	before = read()
	e, err := repository.GetCharacterEffectiveStats(char.ID)
	if err != nil {
		t.Fatal(err)
	}
	if e.EffectiveStats.MaxHealth != 224 || e.EffectiveStats.Attack != 12 || e.Equipped["chest"].Bonuses.MaxHealth != 999 {
		t.Fatal("old snapshot projection mismatch")
	}
	t.Setenv("NACHALO_COMBAT_RULESET", "legacy")
	legacy, err := repository.GetCharacterEffectiveStats(char.ID)
	if err != nil {
		t.Fatal(err)
	}
	if legacy.EffectiveStats.MaxHealth != 1129 || legacy.EffectiveStats.Attack != 790 {
		t.Fatalf("legacy snapshot changed: %+v", legacy.EffectiveStats)
	}
	after = read()
	if before != after {
		t.Fatal("reading/projection rewrote owned item or history")
	}
	t.Setenv("NACHALO_COMBAT_RULESET", "")
	// Both market template and owned-instance SQL paths carry correct slot metadata.
	market, err := repository.MarketInventory(ctx, 781)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = json.Marshal(market); err != nil {
		t.Fatal(err)
	}
	previews, err := repository.MarketCatalog(ctx, repository.MarketFilter{Page: 1, Limit: 50, Category: "armor"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err = json.Marshal(previews); err != nil {
		t.Fatal(err)
	}
	recipes, err := repository.PublicCraftRecipes(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = json.Marshal(recipes); err != nil {
		t.Fatal(err)
	}
	targets := map[string][4]int{}
	for _, set := range sets {
		for _, item := range set.Items {
			part := expected[item.Slot]
			if item.Handedness == "two_hand" {
				part[0] += 2
				part[2]++
				part[3]++
			}
			if set.Rarity == "blue" {
				for i := range part {
					if i < 3 {
						part[i] *= 2
					}
				}
			}
			if set.Rarity == "blue" && (item.Slot == "helmet" || item.Slot == "chest" || item.Slot == "boots") {
				part[3] = 2
			}
			targets[item.Code] = part
		}
	}
	marketChecked := 0
	for _, item := range append(market, previews...) {
		want, ok := targets[item.Code]
		if !ok {
			continue
		}
		data, err := json.Marshal(item)
		if err != nil {
			t.Fatal(err)
		}
		var card struct {
			Attributes map[string]int `json:"attributes"`
		}
		if err = json.Unmarshal(data, &card); err != nil {
			t.Fatal(err)
		}
		got := [4]int{card.Attributes["health"], card.Attributes["attack"], card.Attributes["defense"], card.Attributes["agility"]}
		if got != want {
			t.Fatalf("market %s got %v want %v", item.Code, got, want)
		}
		if _, ok = card.Attributes["_slot"]; ok {
			t.Fatal("internal slot metadata exposed")
		}
		marketChecked++
	}
	if marketChecked == 0 {
		t.Fatal("no market gear checked")
	}
	recipesChecked := 0
	for _, recipe := range recipes {
		want, ok := targets[recipe.Code]
		if !ok {
			continue
		}
		data, err := json.Marshal(recipe)
		if err != nil {
			t.Fatal(err)
		}
		var card struct {
			Bonuses repository.EquipmentBonuses `json:"bonuses"`
		}
		if err = json.Unmarshal(data, &card); err != nil {
			t.Fatal(err)
		}
		got := [4]int{card.Bonuses.MaxHealth, card.Bonuses.Attack, card.Bonuses.Defense, card.Bonuses.Agility}
		if got != want {
			t.Fatalf("recipe %s got %v want %v", recipe.Code, got, want)
		}
		recipesChecked++
	}
	if recipesChecked != 25 {
		t.Fatalf("checked %d/25 equipment recipes", recipesChecked)
	}
}
