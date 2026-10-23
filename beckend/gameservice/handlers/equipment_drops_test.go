package handlers

import (
	"testing"

	"gameservice/repository"
)

func withEquipmentDropStubs(t *testing.T) {
	t.Helper()

	origRoll := rollEquipmentDrop
	origGrant := grantEquipmentDropItem
	origAvailable := equipmentDropPersistenceAvailable

	t.Cleanup(func() {
		rollEquipmentDrop = origRoll
		grantEquipmentDropItem = origGrant
		equipmentDropPersistenceAvailable = origAvailable
	})

	equipmentDropPersistenceAvailable = func() bool { return true }
}

func TestResolveEquipmentDropChanceOverride(t *testing.T) {
	t.Setenv("EQUIPMENT_DROP_CHANCE_OVERRIDE", "100")
	t.Setenv("EQUIPMENT_DEV_GRANT_ENABLED", "")
	t.Setenv("APP_ENV", "")
	if got := resolveEquipmentDropChancePercent(); got != 100 {
		t.Fatalf("expected override 100, got %d", got)
	}

	t.Setenv("EQUIPMENT_DROP_CHANCE_OVERRIDE", "101")
	if got := resolveEquipmentDropChancePercent(); got != defaultEquipmentDropChancePercent {
		t.Fatalf("out-of-range override should fall back to default, got %d", got)
	}
}

func TestMaybeGrantMonsterEquipmentDropReturnsDropDTO(t *testing.T) {
	withEquipmentDropStubs(t)
	t.Setenv("EQUIPMENT_DROP_CHANCE_OVERRIDE", "")
	t.Setenv("EQUIPMENT_DEV_GRANT_ENABLED", "")
	t.Setenv("APP_ENV", "")

	var gotChance int
	rollEquipmentDrop = func(chancePercent int) bool {
		gotChance = chancePercent
		return true
	}
	grantEquipmentDropItem = func(userID int, matchInstanceID string) (*repository.DroppedEquipment, error) {
		if userID != 77 {
			t.Fatalf("unexpected grant user: %d", userID)
		}
		if matchInstanceID != "test-match" {
			t.Fatalf("unexpected match instance id: %s", matchInstanceID)
		}
		return &repository.DroppedEquipment{
			InstanceID:   "drop-instance-id",
			OwnerUserID:  77,
			TemplateID:   4,
			TemplateCode: "sagecloth_staff",
			Name:         "Sagecloth Staff",
			Rarity:       "green",
			ImageURL:     "/equipment/mystic/Elderbloom/Elderbloom Staff.png",
			Slot:         "main_hand",
			ItemType:     "staff",
			ClassID:      "mystic",
			SetCode:      "sagecloth_set",
			SetName:      "Sagecloth Set",
		}, nil
	}

	drops, err := maybeGrantMonsterEquipmentDrop(77, "test-match")
	if err != nil {
		t.Fatalf("maybeGrantMonsterEquipmentDrop: %v", err)
	}
	if gotChance != defaultEquipmentDropChancePercent {
		t.Fatalf("expected default chance %d, got %d", defaultEquipmentDropChancePercent, gotChance)
	}
	if len(drops) != 1 {
		t.Fatalf("expected one drop, got %+v", drops)
	}
	drop := drops[0]
	if drop.InstanceID != "drop-instance-id" || drop.TemplateCode != "sagecloth_staff" || drop.OwnerUserID != 77 {
		t.Fatalf("unexpected drop payload ids: %+v", drop)
	}
	if drop.Name != "Sagecloth Staff" || drop.Rarity != "green" || drop.ImageURL != "/equipment/mystic/Elderbloom/Elderbloom Staff.png" {
		t.Fatalf("unexpected drop display payload: %+v", drop)
	}
	if drop.Item.ItemInstanceID != "drop-instance-id" || drop.Item.ClassID != "mystic" || drop.Item.SetCode != "sagecloth_set" {
		t.Fatalf("unexpected nested drop item: %+v", drop.Item)
	}
}

func TestMaybeGrantMonsterEquipmentDropDisabledChanceDoesNotGrant(t *testing.T) {
	withEquipmentDropStubs(t)
	t.Setenv("EQUIPMENT_DROP_CHANCE_OVERRIDE", "0")

	rollEquipmentDrop = func(chancePercent int) bool {
		if chancePercent != 0 {
			t.Fatalf("expected disabled drop chance 0, got %d", chancePercent)
		}
		return false
	}
	grantEquipmentDropItem = func(userID int, matchInstanceID string) (*repository.DroppedEquipment, error) {
		t.Fatal("drop grant should not be called when chance is disabled")
		return nil, nil
	}

	drops, err := maybeGrantMonsterEquipmentDrop(77, "test-match")
	if err != nil {
		t.Fatalf("maybeGrantMonsterEquipmentDrop: %v", err)
	}
	if len(drops) != 0 {
		t.Fatalf("expected no drops with disabled chance, got %+v", drops)
	}
}

func TestMaybeGrantMonsterEquipmentDropOverrideAlwaysGrants(t *testing.T) {
	withEquipmentDropStubs(t)
	t.Setenv("EQUIPMENT_DROP_CHANCE_OVERRIDE", "100")

	rollEquipmentDrop = func(chancePercent int) bool {
		if chancePercent != 100 {
			t.Fatalf("expected forced drop chance 100, got %d", chancePercent)
		}
		return true
	}
	grantEquipmentDropItem = func(userID int, matchInstanceID string) (*repository.DroppedEquipment, error) {
		if matchInstanceID != "test-match" {
			t.Fatalf("unexpected match instance id: %s", matchInstanceID)
		}
		return &repository.DroppedEquipment{
			InstanceID:   "forced-drop",
			OwnerUserID:  userID,
			TemplateID:   1,
			TemplateCode: "aegiswarden_sword",
			Name:         "Aegiswarden Sword",
			Rarity:       "green",
			ImageURL:     "/equipment/guardian/aegiswarden/Aegiswarden Sword.png",
			Slot:         "main_hand",
			ItemType:     "sword",
			ClassID:      "guardian",
			SetCode:      "aegiswarden_set",
			SetName:      "Aegiswarden Set",
		}, nil
	}

	drops, err := maybeGrantMonsterEquipmentDrop(78, "test-match")
	if err != nil {
		t.Fatalf("maybeGrantMonsterEquipmentDrop: %v", err)
	}
	if len(drops) != 1 || drops[0].TemplateCode != "aegiswarden_sword" {
		t.Fatalf("expected forced guardian drop, got %+v", drops)
	}
}
