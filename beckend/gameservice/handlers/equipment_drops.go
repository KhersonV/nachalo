package handlers

import (
	"math/rand"
	"os"
	"strconv"
	"strings"

	"gameservice/repository"
)

const defaultEquipmentDropChancePercent = 10

var (
	rollEquipmentDrop = func(chancePercent int) bool {
		return chancePercent > 0 && rand.Intn(100) < chancePercent
	}
	grantEquipmentDropItem            = repository.GrantRandomEquipmentDrop
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
	raw := strings.TrimSpace(os.Getenv("EQUIPMENT_DROP_CHANCE_OVERRIDE"))
	if raw == "" {
		return defaultEquipmentDropChancePercent
	}

	value, err := strconv.Atoi(raw)
	if err != nil || value < 0 || value > 100 {
		return defaultEquipmentDropChancePercent
	}
	return value
}

func RollEquipmentDropChance() bool {
	return rollEquipmentDrop(resolveEquipmentDropChancePercent())
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

func maybeGrantMonsterEquipmentDrop(killerUserID int) ([]EquipmentDropPayload, error) {
	if killerUserID <= 0 {
		return nil, nil
	}
	if !equipmentDropPersistenceAvailable() {
		return nil, nil
	}

	if !RollEquipmentDropChance() {
		return nil, nil
	}

	item, err := grantEquipmentDropItem(killerUserID)
	if err != nil {
		return nil, err
	}
	if item == nil {
		return nil, nil
	}

	return []EquipmentDropPayload{equipmentDropPayloadFromItem(killerUserID, item)}, nil
}
