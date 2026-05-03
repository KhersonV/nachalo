package repository_test

import (
	"testing"

	"gameservice/repository"
)

func TestGrantEquipmentDropToUserPersistsSourceEventAndUsesExistingTemplate(t *testing.T) {
	db := openCharacterProfileTestDB(t)
	t.Cleanup(func() {
		_ = db.Close()
	})

	userID := uniqueUserID()
	createTestProfile(t, userID, "mystic")

	var templateCountBefore int
	if err := db.QueryRow(`
		SELECT count(*)
		FROM item_templates
		WHERE code = 'sagecloth_staff'
	`).Scan(&templateCountBefore); err != nil {
		t.Fatalf("count templates before drop: %v", err)
	}

	item, err := repository.GrantEquipmentDropToUser(userID, "sagecloth_staff")
	if err != nil {
		t.Fatalf("GrantEquipmentDropToUser: %v", err)
	}
	if item.InstanceID == "" {
		t.Fatal("drop item returned empty instance id")
	}
	if item.Code != "sagecloth_staff" || item.Name != "Sagecloth Staff" || item.Rarity != "green" {
		t.Fatalf("unexpected drop item DTO: %+v", item)
	}
	if item.ImageURL != "/equipment/mystic/sagecloth/staff.png" || item.Slot != "main_hand" || item.ItemType != "staff" {
		t.Fatalf("unexpected drop item display fields: %+v", item)
	}

	var ownerUserID int
	var source, status string
	if err := db.QueryRow(`
		SELECT owner_user_id, source, status
		FROM item_instances
		WHERE id = $1::uuid
	`, item.InstanceID).Scan(&ownerUserID, &source, &status); err != nil {
		t.Fatalf("select dropped item instance: %v", err)
	}
	if ownerUserID != userID || source != "drop" || status != "inventory" {
		t.Fatalf("unexpected dropped item persistence: owner=%d source=%s status=%s", ownerUserID, source, status)
	}

	var createdEvents int
	if err := db.QueryRow(`
		SELECT count(*)
		FROM item_instance_events
		WHERE item_instance_id = $1::uuid
		  AND event_type = 'created'
		  AND to_user_id = $2
	`, item.InstanceID, userID).Scan(&createdEvents); err != nil {
		t.Fatalf("count drop created event: %v", err)
	}
	if createdEvents != 1 {
		t.Fatalf("expected one created event for drop, got %d", createdEvents)
	}

	var templateCountAfter int
	if err := db.QueryRow(`
		SELECT count(*)
		FROM item_templates
		WHERE code = 'sagecloth_staff'
	`).Scan(&templateCountAfter); err != nil {
		t.Fatalf("count templates after drop: %v", err)
	}
	if templateCountAfter != templateCountBefore {
		t.Fatalf("drop should use existing template, before=%d after=%d", templateCountBefore, templateCountAfter)
	}
}

func TestGrantRandomEquipmentDropUsesGlobalPoolAndAllowsDuplicates(t *testing.T) {
	db := openCharacterProfileTestDB(t)
	t.Cleanup(func() {
		_ = db.Close()
	})

	userID := uniqueUserID()
	createTestProfile(t, userID, "guardian")

	var templateCountBefore int
	if err := db.QueryRow(`SELECT count(*) FROM item_templates`).Scan(&templateCountBefore); err != nil {
		t.Fatalf("count templates before random drop: %v", err)
	}

	var globalPoolSize int
	if err := db.QueryRow(`
		SELECT count(*)
		FROM item_templates it
		JOIN item_sets s ON s.id = it.set_id
		WHERE it.rarity = 'green'
		  AND s.rarity = 'green'
		  AND s.code IN ('aegiswarden_set', 'bloodroot_set', 'greenwisp_set', 'sagecloth_set')
		  AND it.class_restriction = s.class_restriction
	`).Scan(&globalPoolSize); err != nil {
		t.Fatalf("count global drop pool: %v", err)
	}
	if globalPoolSize == 0 {
		t.Fatal("expected non-empty global equipment drop pool")
	}

	seen := map[string]int{}
	for i := 0; i < globalPoolSize+1; i++ {
		item, err := repository.GrantRandomEquipmentDrop(userID)
		if err != nil {
			t.Fatalf("GrantRandomEquipmentDrop #%d: %v", i+1, err)
		}
		if item == nil {
			t.Fatalf("expected global drop #%d, got nil", i+1)
		}
		if item.InstanceID == "" || item.OwnerUserID != userID {
			t.Fatalf("unexpected dropped item identity: %+v", item)
		}
		if item.Rarity != "green" {
			t.Fatalf("drop should be green, got %+v", item)
		}
		switch item.SetCode {
		case "aegiswarden_set", "bloodroot_set", "greenwisp_set", "sagecloth_set":
		default:
			t.Fatalf("drop should come from global starter sets, got %+v", item)
		}
		seen[item.TemplateCode]++
	}

	var droppedInstances int
	if err := db.QueryRow(`
		SELECT count(*)
		FROM item_instances ii
		WHERE ii.owner_user_id = $1
		  AND ii.source = 'drop'
		  AND ii.status <> 'deleted'
	`, userID).Scan(&droppedInstances); err != nil {
		t.Fatalf("count dropped instances: %v", err)
	}
	if droppedInstances != globalPoolSize+1 {
		t.Fatalf("expected %d dropped instances, got %d", globalPoolSize+1, droppedInstances)
	}

	duplicateSeen := false
	for _, count := range seen {
		if count > 1 {
			duplicateSeen = true
			break
		}
	}
	if !duplicateSeen {
		t.Fatalf("expected at least one duplicate template after %d drops from %d templates, got %+v", globalPoolSize+1, globalPoolSize, seen)
	}

	var createdEvents int
	if err := db.QueryRow(`
		SELECT count(*)
		FROM item_instance_events e
		JOIN item_instances ii ON ii.id = e.item_instance_id
		WHERE ii.owner_user_id = $1
		  AND e.event_type = 'created'
	`, userID).Scan(&createdEvents); err != nil {
		t.Fatalf("count created drop events: %v", err)
	}
	if createdEvents != globalPoolSize+1 {
		t.Fatalf("expected one created event per drop, got %d", createdEvents)
	}

	var templateCountAfter int
	if err := db.QueryRow(`SELECT count(*) FROM item_templates`).Scan(&templateCountAfter); err != nil {
		t.Fatalf("count templates after random drop: %v", err)
	}
	if templateCountAfter != templateCountBefore {
		t.Fatalf("random drop should use existing templates, before=%d after=%d", templateCountBefore, templateCountAfter)
	}
}

func TestMatchEquipmentLootStoresMatchAndFiltersByOwner(t *testing.T) {
	db := openCharacterProfileTestDB(t)
	t.Cleanup(func() {
		_ = db.Close()
	})

	matchID := "equipment-loot-match"
	otherMatchID := "equipment-loot-other-match"
	userID := uniqueUserID()
	otherUserID := uniqueUserID()
	createTestProfile(t, userID, "guardian")
	createTestProfile(t, otherUserID, "mystic")

	item, err := repository.GrantRandomEquipmentDropForMatch(userID, matchID)
	if err != nil {
		t.Fatalf("GrantRandomEquipmentDropForMatch user: %v", err)
	}
	if item == nil {
		t.Fatal("expected dropped item for user")
	}
	otherItem, err := repository.GrantRandomEquipmentDropForMatch(otherUserID, matchID)
	if err != nil {
		t.Fatalf("GrantRandomEquipmentDropForMatch other user: %v", err)
	}
	if otherItem == nil {
		t.Fatal("expected dropped item for other user")
	}
	_, err = repository.GrantRandomEquipmentDropForMatch(userID, otherMatchID)
	if err != nil {
		t.Fatalf("GrantRandomEquipmentDropForMatch other match: %v", err)
	}

	var storedMatchID string
	var source string
	if err := db.QueryRow(`
		SELECT match_instance_id, source
		FROM item_instances
		WHERE id = $1::uuid
	`, item.InstanceID).Scan(&storedMatchID, &source); err != nil {
		t.Fatalf("select dropped item match context: %v", err)
	}
	if storedMatchID != matchID || source != "drop" {
		t.Fatalf("unexpected drop match context: match=%s source=%s", storedMatchID, source)
	}

	items, err := repository.ListMatchEquipmentLootForUser(userID, matchID)
	if err != nil {
		t.Fatalf("ListMatchEquipmentLootForUser: %v", err)
	}
	if len(items) != 1 {
		t.Fatalf("expected only current user's one match loot item, got %+v", items)
	}
	if items[0].ItemInstanceID != item.InstanceID || items[0].Name == "" || items[0].Image == "" {
		t.Fatalf("unexpected loot item response: %+v", items[0])
	}
	if items[0].ItemInstanceID == otherItem.InstanceID {
		t.Fatalf("other user's loot leaked into response: %+v", items)
	}

	emptyItems, err := repository.ListMatchEquipmentLootForUser(userID, "empty-match")
	if err != nil {
		t.Fatalf("ListMatchEquipmentLootForUser empty match: %v", err)
	}
	if len(emptyItems) != 0 {
		t.Fatalf("expected empty match loot, got %+v", emptyItems)
	}
}
