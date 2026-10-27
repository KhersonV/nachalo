package repository

import (
	"context"
	_ "embed"
	"encoding/json"
	"fmt"
	"gameservice/internal/game/balance"
)

//go:embed equipment_sets_catalog.json
var equipmentSetsJSON []byte

type EquipmentCatalogItem struct {
	Handedness    string                    `json:"handedness,omitempty"`
	RU            string                    `json:"ru,omitempty"`
	DE            string                    `json:"de,omitempty"`
	Description   string                    `json:"description,omitempty"`
	DescriptionRU string                    `json:"descriptionRu,omitempty"`
	DescriptionDE string                    `json:"descriptionDe,omitempty"`
	CombatBonuses balance.CombatItemBonuses `json:"combatBonuses,omitempty"`
	Code          string                    `json:"code"`
	Name          string                    `json:"name"`
	Slot          string                    `json:"slot"`
	Image         string                    `json:"image"`
	BaseTemplate  string                    `json:"baseTemplate"`
	TemplateID    int64                     `json:"templateId"`
	Bonuses       EquipmentBonuses          `json:"bonuses"`
}
type EquipmentCatalogSet struct {
	Code      string                 `json:"code"`
	Name      string                 `json:"name"`
	HeroClass string                 `json:"heroClass"`
	Rarity    string                 `json:"rarity"`
	Source    string                 `json:"source"`
	PairBonus EquipmentBonuses       `json:"pairBonus,omitempty"`
	Items     []EquipmentCatalogItem `json:"items"`
}

var equipmentSetsCatalog = func() []EquipmentCatalogSet {
	var sets []EquipmentCatalogSet
	if err := json.Unmarshal(equipmentSetsJSON, &sets); err != nil {
		panic(err)
	}
	return sets
}()

func equipmentCatalogItem(code string) (EquipmentCatalogItem, EquipmentCatalogSet, bool) {
	for _, set := range equipmentSetsCatalog {
		for _, item := range set.Items {
			if item.Code == code {
				return item, set, true
			}
		}
	}
	return jewelryItem(code)
}

// An exact server whitelist, shared by every random-drop entry point.
var equipmentDropCatalogJSON = func() string {
	type allowed struct {
		Code      string `json:"code"`
		SetCode   string `json:"set_code"`
		HeroClass string `json:"hero_class"`
		Slot      string `json:"slot"`
	}
	rows := []allowed{}
	for _, set := range equipmentSetsCatalog {
		if set.Source == "monster" && set.Rarity == "green" {
			for _, item := range set.Items {
				rows = append(rows, allowed{item.Code, set.Code, set.HeroClass, item.Slot})
			}
		}
	}
	raw, err := json.Marshal(rows)
	if err != nil {
		panic(err)
	}
	return string(raw)
}()

// Metadata from the authored catalogue, actual IDs/stats from the database.
// Recipes stay in the existing public recipe API and are joined by item code.
func EquipmentSetsCatalog(ctx context.Context) ([]EquipmentCatalogSet, error) {
	result := make([]EquipmentCatalogSet, 0, len(equipmentSetsCatalog))
	for _, source := range equipmentSetsCatalog {
		set := source
		set.Items = append([]EquipmentCatalogItem(nil), source.Items...)
		for i := range set.Items {
			item := &set.Items[i]
			err := DB.QueryRowContext(ctx, `SELECT it.id,it.name,it.image_url,it.handedness,it.attack_bonus,it.defense_bonus,it.mobility_bonus,it.agility_bonus,it.max_health_bonus,it.max_energy_bonus,it.sight_range_bonus,it.attack_range_bonus,`+equipmentPocketBonusSQL+`
    FROM item_templates it JOIN item_sets s ON s.id=it.set_id WHERE it.code=$1 AND s.code=$2 AND it.rarity=$3 AND s.rarity=$3 AND it.class_restriction=$4 AND s.class_restriction=$4 AND it.slot=$5`, item.Code, set.Code, set.Rarity, set.HeroClass, item.Slot).Scan(&item.TemplateID, &item.Name, &item.Image, &item.Handedness, &item.Bonuses.Attack, &item.Bonuses.Defense, &item.Bonuses.Mobility, &item.Bonuses.Agility, &item.Bonuses.MaxHealth, &item.Bonuses.MaxEnergy, &item.Bonuses.SightRange, &item.Bonuses.AttackRange, &item.Bonuses.PocketBonus)
			if err != nil {
				return nil, fmt.Errorf("equipment set catalog %s: %w", item.Code, err)
			}
		}
		result = append(result, set)
	}
	return result, nil
}

func craftEquipmentSetCode(code string) string {
	_, set, _ := equipmentCatalogItem(code)
	return set.Code
}
