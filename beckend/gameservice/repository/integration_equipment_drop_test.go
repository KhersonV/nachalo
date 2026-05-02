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
