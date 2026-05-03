package repository_test

import (
	"testing"

	"gameservice/repository"
)

func TestEquipmentSchemaAndSageclothSeed(t *testing.T) {
	db := openCharacterProfileTestDB(t)
	t.Cleanup(func() {
		_ = db.Close()
	})

	for _, tableName := range []string{
		"item_sets",
		"item_templates",
		"item_set_bonuses",
		"item_instances",
		"character_equipment",
		"item_instance_events",
	} {
		if !tableExistsForTest(t, db, tableName) {
			t.Fatalf("expected %s to exist", tableName)
		}
	}

	for _, col := range []string{
		"code",
		"name",
		"set_id",
		"slot",
		"item_type",
		"handedness",
		"rarity",
		"class_restriction",
		"attack_bonus",
		"defense_bonus",
		"mobility_bonus",
		"agility_bonus",
		"max_health_bonus",
		"max_energy_bonus",
		"sight_range_bonus",
		"attack_range_bonus",
	} {
		if !columnExistsForTest(t, db, "item_templates", col) {
			t.Fatalf("item_templates missing %s", col)
		}
	}

	var generatedUUID string
	if err := db.QueryRow(`SELECT gen_random_uuid()::text`).Scan(&generatedUUID); err != nil {
		t.Fatalf("pgcrypto gen_random_uuid unavailable: %v", err)
	}
	if generatedUUID == "" {
		t.Fatalf("expected gen_random_uuid to return a value")
	}

	var setID int64
	var setClass, setRarity, setDescription string
	if err := db.QueryRow(`
		SELECT id, class_restriction, rarity, description
		FROM item_sets
		WHERE code = 'sagecloth_set'
	`).Scan(&setID, &setClass, &setRarity, &setDescription); err != nil {
		t.Fatalf("select sagecloth set: %v", err)
	}
	if setClass != "mystic" || setRarity != "green" || setDescription == "" {
		t.Fatalf("unexpected sagecloth set metadata: class=%s rarity=%s description=%q", setClass, setRarity, setDescription)
	}

	var itemCount int
	if err := db.QueryRow(`
		SELECT count(*)
		FROM item_templates
		WHERE set_id = $1
	`, setID).Scan(&itemCount); err != nil {
		t.Fatalf("count sagecloth templates: %v", err)
	}
	if itemCount != 6 {
		t.Fatalf("expected 6 sagecloth templates, got %d", itemCount)
	}

	var slot, itemType, handedness, classRestriction, imageURL string
	var attackBonus, agilityBonus, maxEnergyBonus int
	if err := db.QueryRow(`
		SELECT slot, item_type, handedness, class_restriction, image_url, attack_bonus, agility_bonus, max_energy_bonus
		FROM item_templates
		WHERE code = 'sagecloth_staff'
	`).Scan(&slot, &itemType, &handedness, &classRestriction, &imageURL, &attackBonus, &agilityBonus, &maxEnergyBonus); err != nil {
		t.Fatalf("select sagecloth staff: %v", err)
	}
	if slot != "main_hand" || itemType != "staff" || handedness != "two_hand" || classRestriction != "mystic" {
		t.Fatalf("unexpected sagecloth staff equipment shape: slot=%s type=%s handedness=%s class=%s", slot, itemType, handedness, classRestriction)
	}
	if imageURL != "/equipment/mystic/sagecloth/staff.png" || attackBonus != 1 || agilityBonus != 1 || maxEnergyBonus != 4 {
		t.Fatalf("unexpected sagecloth staff bonuses/image: image=%s attack=%d agility=%d energy=%d", imageURL, attackBonus, agilityBonus, maxEnergyBonus)
	}

	expectedBonuses := map[int]struct {
		attackBonus    int
		agilityBonus   int
		maxEnergyBonus int
	}{
		2: {maxEnergyBonus: 5},
		4: {agilityBonus: 1},
		6: {attackBonus: 1},
	}
	rows, err := db.Query(`
		SELECT pieces_required, attack_bonus, agility_bonus, max_energy_bonus
		FROM item_set_bonuses
		WHERE set_id = $1
	`, setID)
	if err != nil {
		t.Fatalf("select sagecloth bonuses: %v", err)
	}
	defer rows.Close()
	seenBonuses := map[int]bool{}
	for rows.Next() {
		var pieces, attack, agility, maxEnergy int
		if err := rows.Scan(&pieces, &attack, &agility, &maxEnergy); err != nil {
			t.Fatalf("scan sagecloth bonus: %v", err)
		}
		want, ok := expectedBonuses[pieces]
		if !ok {
			t.Fatalf("unexpected sagecloth bonus pieces_required=%d", pieces)
		}
		if attack != want.attackBonus || agility != want.agilityBonus || maxEnergy != want.maxEnergyBonus {
			t.Fatalf("unexpected bonus for %d pieces: attack=%d agility=%d energy=%d", pieces, attack, agility, maxEnergy)
		}
		seenBonuses[pieces] = true
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("iterate sagecloth bonuses: %v", err)
	}
	if len(seenBonuses) != len(expectedBonuses) {
		t.Fatalf("expected %d sagecloth bonuses, got %d", len(expectedBonuses), len(seenBonuses))
	}

	tx, err := db.Begin()
	if err != nil {
		t.Fatalf("begin invalid template tx: %v", err)
	}
	_, err = tx.Exec(`
		INSERT INTO item_templates (code, name, slot, item_type, handedness, rarity, image_url)
		VALUES ($1, 'Invalid Shield', 'main_hand', 'shield', 'one_hand', 'green', '/equipment/invalid.webp')
	`, uniqueCharacterProfileTestName("bad_equipment_template"))
	_ = tx.Rollback()
	if err == nil {
		t.Fatalf("expected shield/main_hand consistency check to reject invalid template")
	}
}

func TestEquipmentSeedCreatesAllClassSets(t *testing.T) {
	db := openCharacterProfileTestDB(t)
	t.Cleanup(func() {
		_ = db.Close()
	})

	expectedSets := map[string]struct {
		classID     string
		itemCount   int
		sampleItem  string
		sampleImage string
	}{
		"aegiswarden_set": {classID: "guardian", itemCount: 7, sampleItem: "aegiswarden_sword", sampleImage: "/equipment/guardian/aegiswarden/Aegiswarden Sword.png"},
		"bloodroot_set":   {classID: "berserker", itemCount: 6, sampleItem: "bloodroot_axe", sampleImage: "/equipment/berserker/bloodroot/Bloodroot Axe.png"},
		"greenwisp_set":   {classID: "ranger", itemCount: 6, sampleItem: "greenwisp_bow", sampleImage: "/equipment/ranger/greenwisp/Greenwisp Bow.png"},
		"sagecloth_set":   {classID: "mystic", itemCount: 6, sampleItem: "sagecloth_staff", sampleImage: "/equipment/mystic/sagecloth/staff.png"},
	}

	for setCode, expected := range expectedSets {
		var setID int64
		var classID string
		if err := db.QueryRow(`
			SELECT id, class_restriction
			FROM item_sets
			WHERE code = $1
		`, setCode).Scan(&setID, &classID); err != nil {
			t.Fatalf("select set %s: %v", setCode, err)
		}
		if classID != expected.classID {
			t.Fatalf("set %s expected class %s, got %s", setCode, expected.classID, classID)
		}

		var itemCount int
		if err := db.QueryRow(`
			SELECT count(*)
			FROM item_templates
			WHERE set_id = $1
		`, setID).Scan(&itemCount); err != nil {
			t.Fatalf("count templates for %s: %v", setCode, err)
		}
		if itemCount != expected.itemCount {
			t.Fatalf("set %s expected %d templates, got %d", setCode, expected.itemCount, itemCount)
		}

		var imageURL string
		if err := db.QueryRow(`
			SELECT image_url
			FROM item_templates
			WHERE code = $1
		`, expected.sampleItem).Scan(&imageURL); err != nil {
			t.Fatalf("select sample item %s: %v", expected.sampleItem, err)
		}
		if imageURL != expected.sampleImage {
			t.Fatalf("sample item %s expected image %s, got %s", expected.sampleItem, expected.sampleImage, imageURL)
		}
	}
}

func TestEquipmentInstanceUniquenessGuards(t *testing.T) {
	db := openCharacterProfileTestDB(t)
	t.Cleanup(func() {
		_ = db.Close()
	})

	userID := uniqueUserID()
	createTestProfile(t, userID, "mystic")

	var characterID int
	if err := db.QueryRow(`
		SELECT selected_character_id
		FROM player_profiles
		WHERE user_id = $1
	`, userID).Scan(&characterID); err != nil {
		t.Fatalf("select selected character: %v", err)
	}

	var staffTemplateID int64
	if err := db.QueryRow(`SELECT id FROM item_templates WHERE code = 'sagecloth_staff'`).Scan(&staffTemplateID); err != nil {
		t.Fatalf("select staff template: %v", err)
	}

	insertInstance := func() string {
		t.Helper()
		var itemInstanceID string
		if err := db.QueryRow(`
			INSERT INTO item_instances (
				template_id,
				owner_user_id,
				current_character_id,
				status,
				source
			)
			VALUES ($1, $2, $3, 'equipped', 'admin')
			RETURNING id::text
		`, staffTemplateID, userID, characterID).Scan(&itemInstanceID); err != nil {
			t.Fatalf("insert item instance: %v", err)
		}
		return itemInstanceID
	}

	firstItemID := insertInstance()
	if _, err := db.Exec(`
		INSERT INTO character_equipment (character_id, item_instance_id, slot)
		VALUES ($1, $2::uuid, 'main_hand')
	`, characterID, firstItemID); err != nil {
		t.Fatalf("equip first item: %v", err)
	}

	if _, err := db.Exec(`
		INSERT INTO item_instance_events (item_instance_id, event_type, to_user_id, to_character_id)
		VALUES ($1::uuid, 'created', $2, $3)
	`, firstItemID, userID, characterID); err != nil {
		t.Fatalf("insert item event: %v", err)
	}

	if _, err := db.Exec(`
		INSERT INTO character_equipment (character_id, item_instance_id, slot)
		VALUES ($1, $2::uuid, 'off_hand')
	`, characterID, firstItemID); err == nil {
		t.Fatalf("expected duplicate item_instance_id equip to be rejected")
	}

	secondItemID := insertInstance()
	if _, err := db.Exec(`
		INSERT INTO character_equipment (character_id, item_instance_id, slot)
		VALUES ($1, $2::uuid, 'main_hand')
	`, characterID, secondItemID); err == nil {
		t.Fatalf("expected duplicate character slot equip to be rejected")
	}

	if _, err := db.Exec(`
		INSERT INTO item_instance_events (item_instance_id, event_type)
		VALUES ($1::uuid, 'duplicated')
	`, firstItemID); err == nil {
		t.Fatalf("expected invalid item event type to be rejected")
	}

	var version int
	if err := db.QueryRow(`
		SELECT version
		FROM item_instances
		WHERE id = $1::uuid
	`, firstItemID).Scan(&version); err != nil {
		t.Fatalf("select item instance version: %v", err)
	}
	if version != 1 {
		t.Fatalf("expected initial optimistic lock version 1, got %d", version)
	}

	if _, err := repository.DB.Exec(`SELECT 1`); err != nil {
		t.Fatalf("repository DB should remain usable after equipment checks: %v", err)
	}
}
