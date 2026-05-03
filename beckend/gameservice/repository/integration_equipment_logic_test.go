package repository_test

import (
	"database/sql"
	"errors"
	"fmt"
	"testing"

	"gameservice/repository"
)

func selectedCharacterIDForEquipmentTest(t *testing.T, userID int) int {
	t.Helper()

	var characterID int
	if err := repository.DB.QueryRow(`
		SELECT selected_character_id
		FROM player_profiles
		WHERE user_id = $1
	`, userID).Scan(&characterID); err != nil {
		t.Fatalf("select selected character: %v", err)
	}
	return characterID
}

func grantEquipmentForTest(t *testing.T, userID int, templateCode string) *repository.EquipmentItem {
	t.Helper()

	item, err := repository.GrantItemInstanceToUser(userID, templateCode, "admin")
	if err != nil {
		t.Fatalf("GrantItemInstanceToUser(%s): %v", templateCode, err)
	}
	if item.InstanceID == "" {
		t.Fatalf("granted item has empty instance id")
	}
	return item
}

func insertEquipmentTemplateForTest(t *testing.T, itemType string, slot string, handedness string, classRestriction *string) string {
	t.Helper()

	code := uniqueCharacterProfileTestName(fmt.Sprintf("tpl_%s", itemType))
	name := fmt.Sprintf("Test %s", itemType)
	imageURL := fmt.Sprintf("/equipment/test/%s.webp", code)
	if _, err := repository.DB.Exec(`
		INSERT INTO item_templates (
			code,
			name,
			slot,
			item_type,
			handedness,
			rarity,
			class_restriction,
			image_url
		)
		VALUES ($1, $2, $3, $4, $5, 'green', $6, $7)
	`, code, name, slot, itemType, handedness, classRestriction, imageURL); err != nil {
		t.Fatalf("insert test equipment template: %v", err)
	}
	return code
}

func TestGrantEquipmentItemInstanceToUser(t *testing.T) {
	db := openCharacterProfileTestDB(t)
	t.Cleanup(func() {
		_ = db.Close()
	})

	userID := uniqueUserID()
	createTestProfile(t, userID, "mystic")

	item := grantEquipmentForTest(t, userID, "sagecloth_staff")
	if item.Code != "sagecloth_staff" || item.Status != "inventory" || item.Version != 1 {
		t.Fatalf("unexpected granted item: %+v", item)
	}

	var events int
	if err := db.QueryRow(`
		SELECT count(*)
		FROM item_instance_events
		WHERE item_instance_id = $1::uuid
		  AND event_type = 'created'
		  AND to_user_id = $2
	`, item.InstanceID, userID).Scan(&events); err != nil {
		t.Fatalf("count created events: %v", err)
	}
	if events != 1 {
		t.Fatalf("expected one created event, got %d", events)
	}
}

func TestEquipSageclothHoodWithNullCurrentCharacterID(t *testing.T) {
	db := openCharacterProfileTestDB(t)
	t.Cleanup(func() {
		_ = db.Close()
	})

	userID := uniqueUserID()
	createTestProfile(t, userID, "mystic")
	characterID := selectedCharacterIDForEquipmentTest(t, userID)

	hood := grantEquipmentForTest(t, userID, "sagecloth_hood")
	var currentCharacterIDBefore sql.NullInt64
	if err := db.QueryRow(`
		SELECT current_character_id
		FROM item_instances
		WHERE id = $1::uuid
	`, hood.InstanceID).Scan(&currentCharacterIDBefore); err != nil {
		t.Fatalf("select current_character_id before equip: %v", err)
	}
	if currentCharacterIDBefore.Valid {
		t.Fatalf("expected current_character_id to start NULL, got %d", currentCharacterIDBefore.Int64)
	}

	if err := repository.EquipItemToCharacter(userID, characterID, hood.InstanceID); err != nil {
		t.Fatalf("EquipItemToCharacter hood: %v", err)
	}

	var equippedRows int
	if err := db.QueryRow(`
		SELECT count(*)
		FROM character_equipment
		WHERE character_id = $1
		  AND item_instance_id = $2::uuid
		  AND slot = 'helmet'
	`, characterID, hood.InstanceID).Scan(&equippedRows); err != nil {
		t.Fatalf("count helmet equipment row: %v", err)
	}
	if equippedRows != 1 {
		t.Fatalf("expected one helmet equipment row, got %d", equippedRows)
	}

	var status string
	var currentCharacterIDAfter sql.NullInt64
	if err := db.QueryRow(`
		SELECT status, current_character_id
		FROM item_instances
		WHERE id = $1::uuid
	`, hood.InstanceID).Scan(&status, &currentCharacterIDAfter); err != nil {
		t.Fatalf("select hood instance after equip: %v", err)
	}
	if status != "equipped" || !currentCharacterIDAfter.Valid || int(currentCharacterIDAfter.Int64) != characterID {
		t.Fatalf("expected equipped hood on character %d, status=%s current_character_id=%v", characterID, status, currentCharacterIDAfter)
	}

	var equippedEvents int
	if err := db.QueryRow(`
		SELECT count(*)
		FROM item_instance_events
		WHERE item_instance_id = $1::uuid
		  AND event_type = 'equipped'
		  AND to_user_id = $2
		  AND to_character_id = $3
	`, hood.InstanceID, userID, characterID).Scan(&equippedEvents); err != nil {
		t.Fatalf("count equipped events: %v", err)
	}
	if equippedEvents != 1 {
		t.Fatalf("expected one equipped event, got %d", equippedEvents)
	}
}

func TestGrantEquipmentSetToUserIsIdempotent(t *testing.T) {
	db := openCharacterProfileTestDB(t)
	t.Cleanup(func() {
		_ = db.Close()
	})

	userID := uniqueUserID()
	createTestProfile(t, userID, "guardian")

	if err := repository.GrantEquipmentSetToUser(userID, "guardian", "aegiswarden_set", "admin"); err != nil {
		t.Fatalf("GrantEquipmentSetToUser first: %v", err)
	}
	if err := repository.GrantEquipmentSetToUser(userID, "guardian", "aegiswarden_set", "admin"); err != nil {
		t.Fatalf("GrantEquipmentSetToUser second: %v", err)
	}

	var itemCount int
	if err := db.QueryRow(`
		SELECT count(*)
		FROM item_instances ii
		JOIN item_templates it ON it.id = ii.template_id
		JOIN item_sets s ON s.id = it.set_id
		WHERE ii.owner_user_id = $1
		  AND s.code = 'aegiswarden_set'
		  AND ii.status <> 'deleted'
	`, userID).Scan(&itemCount); err != nil {
		t.Fatalf("count granted aegiswarden items: %v", err)
	}
	if itemCount != 7 {
		t.Fatalf("expected idempotent grant to leave 7 items, got %d", itemCount)
	}

	var createdEvents int
	if err := db.QueryRow(`
		SELECT count(*)
		FROM item_instance_events e
		JOIN item_instances ii ON ii.id = e.item_instance_id
		JOIN item_templates it ON it.id = ii.template_id
		JOIN item_sets s ON s.id = it.set_id
		WHERE ii.owner_user_id = $1
		  AND s.code = 'aegiswarden_set'
		  AND e.event_type = 'created'
	`, userID).Scan(&createdEvents); err != nil {
		t.Fatalf("count created events for aegiswarden grant: %v", err)
	}
	if createdEvents != 7 {
		t.Fatalf("expected one created event per granted item, got %d", createdEvents)
	}
}

func TestAnotherUsersEquipmentCannotBeEquipped(t *testing.T) {
	db := openCharacterProfileTestDB(t)
	t.Cleanup(func() {
		_ = db.Close()
	})

	ownerUserID := uniqueUserID()
	otherUserID := uniqueUserID()
	createTestProfile(t, ownerUserID, "guardian")
	createTestProfile(t, otherUserID, "guardian")
	otherCharacterID := selectedCharacterIDForEquipmentTest(t, otherUserID)

	if err := repository.GrantEquipmentSetToUser(ownerUserID, "guardian", "aegiswarden_set", "admin"); err != nil {
		t.Fatalf("grant owner equipment: %v", err)
	}
	var swordInstanceID string
	if err := db.QueryRow(`
		SELECT ii.id::text
		FROM item_instances ii
		JOIN item_templates it ON it.id = ii.template_id
		WHERE ii.owner_user_id = $1
		  AND it.code = 'aegiswarden_sword'
	`, ownerUserID).Scan(&swordInstanceID); err != nil {
		t.Fatalf("select owner sword instance: %v", err)
	}

	if err := repository.EquipItemToCharacter(otherUserID, otherCharacterID, swordInstanceID); !errors.Is(err, repository.ErrEquipmentItemNotOwned) {
		t.Fatalf("expected item ownership validation error, got %v", err)
	}
}

func TestGuardianEquipmentBonusesAreSummed(t *testing.T) {
	db := openCharacterProfileTestDB(t)
	t.Cleanup(func() {
		_ = db.Close()
	})

	userID := uniqueUserID()
	createTestProfile(t, userID, "guardian")
	characterID := selectedCharacterIDForEquipmentTest(t, userID)

	if err := repository.GrantEquipmentSetToUser(userID, "guardian", "aegiswarden_set", "admin"); err != nil {
		t.Fatalf("grant guardian set: %v", err)
	}
	items, err := repository.GetAllEquipmentForUser(userID)
	if err != nil {
		t.Fatalf("GetAllEquipmentForUser: %v", err)
	}
	for _, item := range items {
		if item.ClassRestriction != "guardian" {
			continue
		}
		if err := repository.EquipItemToCharacter(userID, characterID, item.InstanceID); err != nil {
			t.Fatalf("equip %s: %v", item.Code, err)
		}
	}

	stats, err := repository.GetCharacterEffectiveStats(characterID)
	if err != nil {
		t.Fatalf("GetCharacterEffectiveStats: %v", err)
	}
	base := repository.ResolveHeroClassStats("guardian")
	if stats.EffectiveStats.Attack != base.Attack+2 ||
		stats.EffectiveStats.Defense != base.Defense+5 ||
		stats.EffectiveStats.Mobility != base.Mobility+2 ||
		stats.EffectiveStats.Agility != base.Agility+1 ||
		stats.EffectiveStats.MaxHealth != base.MaxHealth+4 {
		t.Fatalf("unexpected guardian effective stats: base=%+v effective=%+v", base, stats.EffectiveStats)
	}
}

func TestEquipTwoHandedStaffBlocksOffHandAndPreventsDuplicateEquip(t *testing.T) {
	db := openCharacterProfileTestDB(t)
	t.Cleanup(func() {
		_ = db.Close()
	})

	userID := uniqueUserID()
	createTestProfile(t, userID, "mystic")
	characterID := selectedCharacterIDForEquipmentTest(t, userID)

	staff := grantEquipmentForTest(t, userID, "sagecloth_staff")
	if err := repository.EquipItemToCharacter(userID, characterID, staff.InstanceID); err != nil {
		t.Fatalf("EquipItemToCharacter staff: %v", err)
	}

	equipped, err := repository.GetCharacterEquipment(characterID)
	if err != nil {
		t.Fatalf("GetCharacterEquipment: %v", err)
	}
	if equipped["main_hand"] == nil || equipped["main_hand"].InstanceID != staff.InstanceID {
		t.Fatalf("expected staff in main_hand, got %+v", equipped)
	}
	if equipped["off_hand"] != nil {
		t.Fatalf("two-handed staff should not create off_hand row: %+v", equipped["off_hand"])
	}

	if err := repository.EquipItemToCharacter(userID, characterID, staff.InstanceID); !errors.Is(err, repository.ErrEquipmentItemNotInInventory) {
		t.Fatalf("expected duplicate equip to be blocked by item status, got %v", err)
	}

	shieldCode := insertEquipmentTemplateForTest(t, "shield", "off_hand", "one_hand", nil)
	shield := grantEquipmentForTest(t, userID, shieldCode)
	if err := repository.EquipItemToCharacter(userID, characterID, shield.InstanceID); !errors.Is(err, repository.ErrEquipmentOffHandBlocked) {
		t.Fatalf("expected off_hand blocked by two-handed weapon, got %v", err)
	}
}

func TestEquippingTwoHandedStaffUnequipsExistingOffHandItem(t *testing.T) {
	db := openCharacterProfileTestDB(t)
	t.Cleanup(func() {
		_ = db.Close()
	})

	userID := uniqueUserID()
	createTestProfile(t, userID, "mystic")
	characterID := selectedCharacterIDForEquipmentTest(t, userID)

	shieldCode := insertEquipmentTemplateForTest(t, "shield", "off_hand", "one_hand", nil)
	shield := grantEquipmentForTest(t, userID, shieldCode)
	if err := repository.EquipItemToCharacter(userID, characterID, shield.InstanceID); err != nil {
		t.Fatalf("equip setup shield: %v", err)
	}

	staff := grantEquipmentForTest(t, userID, "sagecloth_staff")
	if err := repository.EquipItemToCharacter(userID, characterID, staff.InstanceID); err != nil {
		t.Fatalf("equip staff over offhand: %v", err)
	}

	equipped, err := repository.GetCharacterEquipment(characterID)
	if err != nil {
		t.Fatalf("GetCharacterEquipment: %v", err)
	}
	if equipped["main_hand"] == nil || equipped["main_hand"].InstanceID != staff.InstanceID {
		t.Fatalf("expected staff in main_hand, got %+v", equipped)
	}
	if equipped["off_hand"] != nil {
		t.Fatalf("expected off_hand to be unequipped by two-handed staff, got %+v", equipped["off_hand"])
	}

	var shieldStatus string
	var currentCharacterID sql.NullInt64
	if err := db.QueryRow(`
		SELECT status, current_character_id
		FROM item_instances
		WHERE id = $1::uuid
	`, shield.InstanceID).Scan(&shieldStatus, &currentCharacterID); err != nil {
		t.Fatalf("select shield instance after replacement: %v", err)
	}
	if shieldStatus != "inventory" || currentCharacterID.Valid {
		t.Fatalf("expected shield back in inventory with no character, status=%s character=%v", shieldStatus, currentCharacterID)
	}
}

func TestEquipmentEffectiveStatsIncludeItemAndSetBonuses(t *testing.T) {
	db := openCharacterProfileTestDB(t)
	t.Cleanup(func() {
		_ = db.Close()
	})

	userID := uniqueUserID()
	createTestProfile(t, userID, "mystic")
	characterID := selectedCharacterIDForEquipmentTest(t, userID)

	for _, code := range []string{
		"sagecloth_staff",
		"sagecloth_jacket",
		"sagecloth_pants",
		"sagecloth_boots",
		"sagecloth_gloves",
		"sagecloth_hood",
	} {
		item := grantEquipmentForTest(t, userID, code)
		if err := repository.EquipItemToCharacter(userID, characterID, item.InstanceID); err != nil {
			t.Fatalf("equip %s: %v", code, err)
		}
	}

	stats, err := repository.GetCharacterEffectiveStats(characterID)
	if err != nil {
		t.Fatalf("GetCharacterEffectiveStats: %v", err)
	}
	base := repository.ResolveHeroClassStats("mystic")
	if stats.BaseStats.Attack != base.Attack || stats.BaseStats.MaxEnergy != base.MaxEnergy {
		t.Fatalf("base stats should remain character stats, got %+v", stats.BaseStats)
	}

	if stats.EffectiveStats.Attack != base.Attack+3 {
		t.Fatalf("expected attack with items+6pc set = %d, got %d", base.Attack+3, stats.EffectiveStats.Attack)
	}
	if stats.EffectiveStats.Defense != base.Defense+2 {
		t.Fatalf("expected defense with items = %d, got %d", base.Defense+2, stats.EffectiveStats.Defense)
	}
	if stats.EffectiveStats.Mobility != base.Mobility+1 {
		t.Fatalf("expected mobility with boots = %d, got %d", base.Mobility+1, stats.EffectiveStats.Mobility)
	}
	if stats.EffectiveStats.Agility != base.Agility+4 {
		t.Fatalf("expected agility with items+4pc set = %d, got %d", base.Agility+4, stats.EffectiveStats.Agility)
	}
	if stats.EffectiveStats.MaxHealth != base.MaxHealth+11 {
		t.Fatalf("expected max health with items = %d, got %d", base.MaxHealth+11, stats.EffectiveStats.MaxHealth)
	}
	if stats.EffectiveStats.MaxEnergy != base.MaxEnergy+23 {
		t.Fatalf("expected max energy with items+2pc set = %d, got %d", base.MaxEnergy+23, stats.EffectiveStats.MaxEnergy)
	}
	if len(stats.ActiveSetBonuses) != 3 {
		t.Fatalf("expected 3 active Sagecloth set bonuses, got %+v", stats.ActiveSetBonuses)
	}
}

func TestWrongClassCannotEquipRestrictedEquipment(t *testing.T) {
	db := openCharacterProfileTestDB(t)
	t.Cleanup(func() {
		_ = db.Close()
	})

	userID := uniqueUserID()
	createTestProfile(t, userID, "guardian")
	characterID := selectedCharacterIDForEquipmentTest(t, userID)
	staff := grantEquipmentForTest(t, userID, "sagecloth_staff")

	if err := repository.EquipItemToCharacter(userID, characterID, staff.InstanceID); !errors.Is(err, repository.ErrEquipmentClassRestricted) {
		t.Fatalf("expected class restriction error, got %v", err)
	}
}

func TestSellDiscardRejectEquippedEquipmentAndAllowAfterUnequip(t *testing.T) {
	db := openCharacterProfileTestDB(t)
	t.Cleanup(func() {
		_ = db.Close()
	})

	userID := uniqueUserID()
	createTestProfile(t, userID, "guardian")
	characterID := selectedCharacterIDForEquipmentTest(t, userID)

	sword := grantEquipmentForTest(t, userID, "aegiswarden_sword")
	if err := repository.EquipItemToCharacter(userID, characterID, sword.InstanceID); err != nil {
		t.Fatalf("equip sword: %v", err)
	}

	if _, err := repository.SellEquipmentItem(userID, sword.InstanceID); !errors.Is(err, repository.ErrEquipmentItemEquipped) {
		t.Fatalf("expected sell equipped item to be rejected, got %v", err)
	}
	if err := repository.DiscardEquipmentItem(userID, sword.InstanceID); !errors.Is(err, repository.ErrEquipmentItemEquipped) {
		t.Fatalf("expected discard equipped item to be rejected, got %v", err)
	}

	inventory, err := repository.GetEquipmentInventoryForUser(userID)
	if err != nil {
		t.Fatalf("GetEquipmentInventoryForUser while equipped: %v", err)
	}
	for _, item := range inventory {
		if item.InstanceID == sword.InstanceID {
			t.Fatalf("equipped item should not appear in inventory list: %+v", item)
		}
	}

	if err := repository.UnequipItemFromCharacter(userID, characterID, "main_hand"); err != nil {
		t.Fatalf("unequip sword: %v", err)
	}
	inventory, err = repository.GetEquipmentInventoryForUser(userID)
	if err != nil {
		t.Fatalf("GetEquipmentInventoryForUser after unequip: %v", err)
	}
	foundSword := false
	for _, item := range inventory {
		if item.InstanceID == sword.InstanceID {
			foundSword = true
			break
		}
	}
	if !foundSword {
		t.Fatal("unequipped sword should appear in inventory list")
	}
	if _, err := repository.SellEquipmentItem(userID, sword.InstanceID); err != nil {
		t.Fatalf("sell unequipped sword: %v", err)
	}

	var swordStatus string
	if err := db.QueryRow(`SELECT status FROM item_instances WHERE id = $1::uuid`, sword.InstanceID).Scan(&swordStatus); err != nil {
		t.Fatalf("select sold sword status: %v", err)
	}
	if swordStatus != "deleted" {
		t.Fatalf("sold sword should be deleted, got status=%s", swordStatus)
	}
	var soldEvents int
	if err := db.QueryRow(`
		SELECT count(*)
		FROM item_instance_events
		WHERE item_instance_id = $1::uuid
		  AND event_type = 'sold'
	`, sword.InstanceID).Scan(&soldEvents); err != nil {
		t.Fatalf("count sold event: %v", err)
	}
	if soldEvents != 1 {
		t.Fatalf("expected one sold event, got %d", soldEvents)
	}

	shield := grantEquipmentForTest(t, userID, "aegiswarden_shield")
	if err := repository.EquipItemToCharacter(userID, characterID, shield.InstanceID); err != nil {
		t.Fatalf("equip shield: %v", err)
	}
	if err := repository.DiscardEquipmentItem(userID, shield.InstanceID); !errors.Is(err, repository.ErrEquipmentItemEquipped) {
		t.Fatalf("expected discard equipped shield to be rejected, got %v", err)
	}
	if err := repository.UnequipItemFromCharacter(userID, characterID, "off_hand"); err != nil {
		t.Fatalf("unequip shield: %v", err)
	}
	if err := repository.DiscardEquipmentItem(userID, shield.InstanceID); err != nil {
		t.Fatalf("discard unequipped shield: %v", err)
	}

	var shieldStatus string
	if err := db.QueryRow(`SELECT status FROM item_instances WHERE id = $1::uuid`, shield.InstanceID).Scan(&shieldStatus); err != nil {
		t.Fatalf("select discarded shield status: %v", err)
	}
	if shieldStatus != "deleted" {
		t.Fatalf("discarded shield should be deleted, got status=%s", shieldStatus)
	}
}

func TestSellingDuplicateInventoryItemDoesNotAffectEquippedCopy(t *testing.T) {
	db := openCharacterProfileTestDB(t)
	t.Cleanup(func() {
		_ = db.Close()
	})

	userID := uniqueUserID()
	createTestProfile(t, userID, "guardian")
	characterID := selectedCharacterIDForEquipmentTest(t, userID)

	equippedCopy := grantEquipmentForTest(t, userID, "aegiswarden_sword")
	inventoryCopy := grantEquipmentForTest(t, userID, "aegiswarden_sword")
	if err := repository.EquipItemToCharacter(userID, characterID, equippedCopy.InstanceID); err != nil {
		t.Fatalf("equip first sword copy: %v", err)
	}

	inventory, err := repository.GetEquipmentInventoryForUser(userID)
	if err != nil {
		t.Fatalf("GetEquipmentInventoryForUser duplicate setup: %v", err)
	}
	seenEquippedCopy := false
	seenInventoryCopy := false
	for _, item := range inventory {
		if item.InstanceID == equippedCopy.InstanceID {
			seenEquippedCopy = true
		}
		if item.InstanceID == inventoryCopy.InstanceID {
			seenInventoryCopy = true
		}
	}
	if seenEquippedCopy {
		t.Fatal("equipped duplicate copy should not appear in inventory")
	}
	if !seenInventoryCopy {
		t.Fatal("unequipped duplicate copy should appear in inventory")
	}

	if _, err := repository.SellEquipmentItem(userID, inventoryCopy.InstanceID); err != nil {
		t.Fatalf("sell unequipped duplicate copy: %v", err)
	}

	var equippedStatus string
	var currentCharacterID sql.NullInt64
	if err := db.QueryRow(`
		SELECT status, current_character_id
		FROM item_instances
		WHERE id = $1::uuid
	`, equippedCopy.InstanceID).Scan(&equippedStatus, &currentCharacterID); err != nil {
		t.Fatalf("select equipped duplicate status: %v", err)
	}
	if equippedStatus != "equipped" || !currentCharacterID.Valid || int(currentCharacterID.Int64) != characterID {
		t.Fatalf("selling inventory duplicate affected equipped copy: status=%s character=%v", equippedStatus, currentCharacterID)
	}

	var equippedRows int
	if err := db.QueryRow(`
		SELECT count(*)
		FROM character_equipment
		WHERE character_id = $1
		  AND item_instance_id = $2::uuid
		  AND slot = 'main_hand'
	`, characterID, equippedCopy.InstanceID).Scan(&equippedRows); err != nil {
		t.Fatalf("count equipped duplicate row: %v", err)
	}
	if equippedRows != 1 {
		t.Fatalf("equipped duplicate row should remain, got %d", equippedRows)
	}

	var soldStatus string
	if err := db.QueryRow(`SELECT status FROM item_instances WHERE id = $1::uuid`, inventoryCopy.InstanceID).Scan(&soldStatus); err != nil {
		t.Fatalf("select sold duplicate status: %v", err)
	}
	if soldStatus != "deleted" {
		t.Fatalf("sold duplicate should be deleted, got %s", soldStatus)
	}
}

func TestCharacterWithoutEquipmentUsesBaseStats(t *testing.T) {
	db := openCharacterProfileTestDB(t)
	t.Cleanup(func() {
		_ = db.Close()
	})

	userID := uniqueUserID()
	createTestProfile(t, userID, "guardian")
	characterID := selectedCharacterIDForEquipmentTest(t, userID)

	stats, err := repository.GetCharacterEffectiveStats(characterID)
	if err != nil {
		t.Fatalf("GetCharacterEffectiveStats: %v", err)
	}
	base := repository.ResolveHeroClassStats("guardian")
	if stats.EffectiveStats.Attack != base.Attack ||
		stats.EffectiveStats.Defense != base.Defense ||
		stats.EffectiveStats.Mobility != base.Mobility ||
		stats.EffectiveStats.Agility != base.Agility ||
		stats.EffectiveStats.MaxHealth != base.MaxHealth ||
		stats.EffectiveStats.MaxEnergy != base.MaxEnergy ||
		stats.EffectiveStats.SightRange != base.SightRange ||
		stats.EffectiveStats.AttackRange != base.AttackRange {
		t.Fatalf("effective stats should equal base stats without equipment: base=%+v effective=%+v", base, stats.EffectiveStats)
	}
	if len(stats.ActiveSetBonuses) != 0 {
		t.Fatalf("expected no active set bonuses, got %+v", stats.ActiveSetBonuses)
	}
}

func TestMatchSnapshotUsesEquipmentEffectiveStats(t *testing.T) {
	db := openCharacterProfileTestDB(t)
	t.Cleanup(func() {
		_ = db.Close()
	})

	userID := uniqueUserID()
	createTestProfile(t, userID, "mystic")
	characterID := selectedCharacterIDForEquipmentTest(t, userID)

	staff := grantEquipmentForTest(t, userID, "sagecloth_staff")
	if err := repository.EquipItemToCharacter(userID, characterID, staff.InstanceID); err != nil {
		t.Fatalf("equip staff: %v", err)
	}

	matchID := uniqueCharacterProfileTestName("equipment_match_snapshot")
	insertMinimalMatch(t, matchID, userID)

	player, err := repository.GetPlayerByUserID(userID)
	if err != nil {
		t.Fatalf("GetPlayerByUserID: %v", err)
	}
	if err := repository.CreateMatchPlayerCopy(matchID, player, 1, 1, 1); err != nil {
		t.Fatalf("CreateMatchPlayerCopy: %v", err)
	}

	var attack, agility, maxEnergy, maxHealth int
	if err := db.QueryRow(`
		SELECT attack, agility, max_energy, max_health
		FROM match_players
		WHERE instance_id = $1
		  AND user_id = $2
	`, matchID, userID).Scan(&attack, &agility, &maxEnergy, &maxHealth); err != nil {
		t.Fatalf("select match player snapshot: %v", err)
	}

	base := repository.ResolveHeroClassStats("mystic")
	if attack != base.Attack+1 ||
		agility != base.Agility+1 ||
		maxEnergy != base.MaxEnergy+4 ||
		maxHealth != base.MaxHealth {
		t.Fatalf("match snapshot did not use equipment effective stats: attack=%d agility=%d maxEnergy=%d maxHealth=%d", attack, agility, maxEnergy, maxHealth)
	}
}
