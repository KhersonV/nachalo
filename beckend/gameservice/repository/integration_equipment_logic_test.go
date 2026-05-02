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
