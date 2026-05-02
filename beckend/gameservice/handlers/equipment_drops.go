package handlers

import (
	"math/rand"
	"os"
	"strconv"
	"strings"

	"gameservice/repository"
)

const defaultEquipmentDropChancePercent = 15

// TODO(equipment-drops): replace this MVP pool with monster-specific drop
// tables keyed by monster_ref_id/type/rarity when more equipment exists.
var equipmentDropTemplatePool = []string{
	"sagecloth_staff",
	"sagecloth_jacket",
	"sagecloth_pants",
	"sagecloth_boots",
	"sagecloth_gloves",
	"sagecloth_hood",
}

var (
	rollEquipmentDrop = func(chancePercent int) bool {
		return chancePercent > 0 && rand.Intn(100) < chancePercent
	}
	pickEquipmentDropTemplate = func(pool []string) string {
		if len(pool) == 0 {
			return ""
		}
		return pool[rand.Intn(len(pool))]
	}
	grantEquipmentDropItem            = repository.GrantEquipmentDropToUser
	equipmentDropPersistenceAvailable = func() bool {
		return repository.DB != nil
	}
)

type EquipmentDropPayload struct {
	InstanceID   string `json:"instanceId"`
	OwnerUserID  int    `json:"ownerUserId"`
	TemplateCode string `json:"templateCode"`
	Name         string `json:"name"`
	Rarity       string `json:"rarity"`
	ImageURL     string `json:"imageUrl"`
	Slot         string `json:"slot"`
	ItemType     string `json:"itemType"`
}

func resolveEquipmentDropChancePercent() int {
	raw := strings.TrimSpace(os.Getenv("EQUIPMENT_DROP_CHANCE_OVERRIDE"))
	if raw == "" || !equipmentDevGrantEnabled() {
		return defaultEquipmentDropChancePercent
	}

	value, err := strconv.Atoi(raw)
	if err != nil || value < 0 || value > 100 {
		return defaultEquipmentDropChancePercent
	}
	return value
}

func equipmentDropPayloadFromItem(ownerUserID int, item *repository.EquipmentItem) EquipmentDropPayload {
	if item == nil {
		return EquipmentDropPayload{OwnerUserID: ownerUserID}
	}
	return EquipmentDropPayload{
		InstanceID:   item.InstanceID,
		OwnerUserID:  ownerUserID,
		TemplateCode: item.Code,
		Name:         item.Name,
		Rarity:       item.Rarity,
		ImageURL:     item.ImageURL,
		Slot:         item.Slot,
		ItemType:     item.ItemType,
	}
}

func maybeGrantMonsterEquipmentDrop(killerUserID int) ([]EquipmentDropPayload, error) {
	if killerUserID <= 0 {
		return nil, nil
	}
	if !equipmentDropPersistenceAvailable() {
		return nil, nil
	}

	if !rollEquipmentDrop(resolveEquipmentDropChancePercent()) {
		return nil, nil
	}

	templateCode := pickEquipmentDropTemplate(equipmentDropTemplatePool)
	if strings.TrimSpace(templateCode) == "" {
		return nil, nil
	}

	item, err := grantEquipmentDropItem(killerUserID, templateCode)
	if err != nil {
		return nil, err
	}

	return []EquipmentDropPayload{equipmentDropPayloadFromItem(killerUserID, item)}, nil
}
