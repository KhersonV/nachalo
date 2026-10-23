package handlers

import (
	"database/sql"

	"gameservice/repository"
)

const defaultEquipmentDropChancePercent = repository.DefaultEquipmentDropChancePercent

var (
	rollEquipmentDrop = repository.RollEquipmentDropChancePercent
	grantEquipmentDropItem            = repository.GrantRandomEquipmentDropForMatch
	equipmentDropPersistenceAvailable = func() bool {
		return repository.DB != nil
	}
)

type EquipmentDroppedItemPayload struct {
	ItemInstanceID string `json:"itemInstanceId"`
	TemplateID     int64  `json:"templateId"`
	TemplateCode   string `json:"templateCode"`
	Name           string `json:"name"`
	Slot           string `json:"slot"`
	Rarity         string `json:"rarity"`
	Image          string `json:"image"`
	ImageURL       string `json:"imageUrl"`
	ItemType       string `json:"itemType"`
	ClassID        string `json:"classId"`
	SetCode        string `json:"setCode,omitempty"`
	SetName        string `json:"setName,omitempty"`
}

type EquipmentDropPayload struct {
	InstanceID   string                      `json:"instanceId"`
	UserID       int                         `json:"userId"`
	OwnerUserID  int                         `json:"ownerUserId"`
	TemplateCode string                      `json:"templateCode"`
	Name         string                      `json:"name"`
	Rarity       string                      `json:"rarity"`
	ImageURL     string                      `json:"imageUrl"`
	Slot         string                      `json:"slot"`
	ItemType     string                      `json:"itemType"`
	Item         EquipmentDroppedItemPayload `json:"item"`
}

func resolveEquipmentDropChancePercent() int {
	return repository.EquipmentDropChancePercent()
}

func RollEquipmentDropChance() bool {
	return rollEquipmentDrop(resolveEquipmentDropChancePercent())
}

// grantHexMonsterEquipmentDropTx keeps the roll and item creation in the
// command that removes the monster, so replay cannot create another item.
func grantHexMonsterEquipmentDropTx(tx *sql.Tx, userID int, instanceID string) ([]EquipmentDropPayload, error) {
	if userID <= 0 || !equipmentDropPersistenceAvailable() || !RollEquipmentDropChance() {
		return nil, nil
	}
	item, err := repository.GrantRandomEquipmentDropForMatchTx(tx, userID, instanceID)
	if err != nil || item == nil {
		return nil, err
	}
	return []EquipmentDropPayload{equipmentDropPayloadFromItem(userID, item)}, nil
}

func equipmentDropPayloadFromItem(ownerUserID int, item *repository.DroppedEquipment) EquipmentDropPayload {
	if item == nil {
		return EquipmentDropPayload{UserID: ownerUserID, OwnerUserID: ownerUserID}
	}
	droppedItem := EquipmentDroppedItemPayload{
		ItemInstanceID: item.InstanceID,
		TemplateID:     item.TemplateID,
		TemplateCode:   item.TemplateCode,
		Name:           item.Name,
		Slot:           item.Slot,
		Rarity:         item.Rarity,
		Image:          item.ImageURL,
		ImageURL:       item.ImageURL,
		ItemType:       item.ItemType,
		ClassID:        item.ClassID,
		SetCode:        item.SetCode,
		SetName:        item.SetName,
	}
	return EquipmentDropPayload{
		InstanceID:   item.InstanceID,
		UserID:       ownerUserID,
		OwnerUserID:  ownerUserID,
		TemplateCode: item.TemplateCode,
		Name:         item.Name,
		Rarity:       item.Rarity,
		ImageURL:     item.ImageURL,
		Slot:         item.Slot,
		ItemType:     item.ItemType,
		Item:         droppedItem,
	}
}

func maybeGrantMonsterEquipmentDrop(killerUserID int, matchInstanceID string) ([]EquipmentDropPayload, error) {
	if killerUserID <= 0 {
		return nil, nil
	}
	if !equipmentDropPersistenceAvailable() {
		return nil, nil
	}

	if !RollEquipmentDropChance() {
		return nil, nil
	}

	item, err := grantEquipmentDropItem(killerUserID, matchInstanceID)
	if err != nil {
		return nil, err
	}
	if item == nil {
		return nil, nil
	}

	return []EquipmentDropPayload{equipmentDropPayloadFromItem(killerUserID, item)}, nil
}
