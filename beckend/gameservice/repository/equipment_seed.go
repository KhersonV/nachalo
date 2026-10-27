package repository

import (
	"strings"
)

type equipmentSetSeed struct {
	Code             string
	Name             string
	ClassRestriction string
	Rarity           string
	Description      string
	Items            []equipmentItemSeed
	Bonuses          []equipmentSetBonusSeed
}

type equipmentItemSeed struct {
	Code             string
	Name             string
	Slot             string
	ItemType         string
	Handedness       string
	Rarity           string
	ClassRestriction string
	ImageURL         string
	Bonuses          EquipmentBonuses
}

type equipmentSetBonusSeed struct {
	PiecesRequired int
	Description    string
	Bonuses        EquipmentBonuses
	EffectCode     *string
	EffectValue    int
}

var equipmentSeedCatalog = []equipmentSetSeed{
	{
		Code:             "aegiswarden_set",
		Name:             "Aegiswarden Set",
		ClassRestriction: "guardian",
		Rarity:           "gray",
		Description:      "A beginner guardian set focused on defense, control and frontline staying power.",
		Items: []equipmentItemSeed{
			{Code: "aegiswarden_sword", Name: "Aegiswarden Sword", Slot: "main_hand", ItemType: "sword", Handedness: "one_hand", Rarity: "gray", ClassRestriction: "guardian", ImageURL: "/equipment/guardian/Aegiswarden/Aegiswarden Sword.png", Bonuses: EquipmentBonuses{Attack: 1}},
			{Code: "aegiswarden_shield", Name: "Aegiswarden Shield", Slot: "off_hand", ItemType: "shield", Handedness: "one_hand", Rarity: "gray", ClassRestriction: "guardian", ImageURL: "/equipment/guardian/Aegiswarden/Aegiswarden Shield.png", Bonuses: EquipmentBonuses{Defense: 2}},
			{Code: "aegiswarden_helm", Name: "Aegiswarden Helm", Slot: "helmet", ItemType: "helmet", Handedness: "none", Rarity: "gray", ClassRestriction: "guardian", ImageURL: "/equipment/guardian/Aegiswarden/Aegiswarden Helm.png", Bonuses: EquipmentBonuses{Defense: 1}},
			{Code: "aegiswarden_vest", Name: "Aegiswarden Vest", Slot: "chest", ItemType: "chest", Handedness: "none", Rarity: "gray", ClassRestriction: "guardian", ImageURL: "/equipment/guardian/Aegiswarden/Aegiswarden Vest.png", Bonuses: EquipmentBonuses{Defense: 1, MaxHealth: 2}},
			{Code: "aegiswarden_pants", Name: "Aegiswarden Pants", Slot: "pants", ItemType: "pants", Handedness: "none", Rarity: "gray", ClassRestriction: "guardian", ImageURL: "/equipment/guardian/Aegiswarden/Aegiswarden Pants.png", Bonuses: EquipmentBonuses{Mobility: 1}},
			{Code: "aegiswarden_gloves", Name: "Aegiswarden Gloves", Slot: "gloves", ItemType: "gloves", Handedness: "none", Rarity: "gray", ClassRestriction: "guardian", ImageURL: "/equipment/guardian/Aegiswarden/Aegiswarden Gloves.png", Bonuses: EquipmentBonuses{Agility: 1}},
			{Code: "aegiswarden_boots", Name: "Aegiswarden Boots", Slot: "boots", ItemType: "boots", Handedness: "none", Rarity: "gray", ClassRestriction: "guardian", ImageURL: "/equipment/guardian/Aegiswarden/boots.png", Bonuses: EquipmentBonuses{Mobility: 1}},
		},
		Bonuses: []equipmentSetBonusSeed{
			{PiecesRequired: 2, Description: "+2 Max Health while wearing 2 pieces of Aegiswarden Set.", Bonuses: EquipmentBonuses{MaxHealth: 2}},
			{PiecesRequired: 4, Description: "+1 Defense while wearing 4 pieces of Aegiswarden Set.", Bonuses: EquipmentBonuses{Defense: 1}},
			{PiecesRequired: 6, Description: "+1 Attack while wearing 6 pieces of Aegiswarden Set.", Bonuses: EquipmentBonuses{Attack: 1}},
		},
	},
	{
		Code:             "bloodroot_set",
		Name:             "Bloodroot Set",
		ClassRestriction: "berserker",
		Rarity:           "gray",
		Description:      "A beginner berserker set focused on melee pressure and early survivability.",
		Items: []equipmentItemSeed{
			{Code: "bloodroot_axe", Name: "Bloodroot Axe", Slot: "main_hand", ItemType: "axe", Handedness: "two_hand", Rarity: "gray", ClassRestriction: "berserker", ImageURL: "/equipment/berserker/Bloodroot/Bloodroot Axe.png", Bonuses: EquipmentBonuses{Attack: 2}},
			{Code: "bloodroot_horned_helm", Name: "Bloodroot Horned Helm", Slot: "helmet", ItemType: "helmet", Handedness: "none", Rarity: "gray", ClassRestriction: "berserker", ImageURL: "/equipment/berserker/Bloodroot/Bloodroot Horned Helm.png", Bonuses: EquipmentBonuses{MaxHealth: 1}},
			{Code: "bloodroot_war_vest", Name: "Bloodroot War Vest", Slot: "chest", ItemType: "chest", Handedness: "none", Rarity: "gray", ClassRestriction: "berserker", ImageURL: "/equipment/berserker/Bloodroot/Bloodroot War Vest.png", Bonuses: EquipmentBonuses{Defense: 1, MaxHealth: 2}},
			{Code: "bloodroot_raider_pants", Name: "Bloodroot Raider Pants", Slot: "pants", ItemType: "pants", Handedness: "none", Rarity: "gray", ClassRestriction: "berserker", ImageURL: "/equipment/berserker/Bloodroot/Bloodroot Raider Pants.png", Bonuses: EquipmentBonuses{Mobility: 1}},
			{Code: "bloodroot_grips", Name: "Bloodroot Grips", Slot: "gloves", ItemType: "gloves", Handedness: "none", Rarity: "gray", ClassRestriction: "berserker", ImageURL: "/equipment/berserker/Bloodroot/Bloodroot Grips.png", Bonuses: EquipmentBonuses{Attack: 1}},
			{Code: "bloodroot_stompers", Name: "Bloodroot Stompers", Slot: "boots", ItemType: "boots", Handedness: "none", Rarity: "gray", ClassRestriction: "berserker", ImageURL: "/equipment/berserker/Bloodroot/Bloodroot Stompers.png", Bonuses: EquipmentBonuses{Mobility: 1}},
		},
		Bonuses: []equipmentSetBonusSeed{
			{PiecesRequired: 2, Description: "+1 Attack while wearing 2 pieces of Bloodroot Set.", Bonuses: EquipmentBonuses{Attack: 1}},
			{PiecesRequired: 4, Description: "+2 Max Health while wearing 4 pieces of Bloodroot Set.", Bonuses: EquipmentBonuses{MaxHealth: 2}},
			{PiecesRequired: 6, Description: "+1 Defense while wearing 6 pieces of Bloodroot Set.", Bonuses: EquipmentBonuses{Defense: 1}},
		},
	},
	{
		Code:             "greenwisp_set",
		Name:             "Greenwisp Set",
		ClassRestriction: "ranger",
		Rarity:           "gray",
		Description:      "A beginner ranger set focused on Reflex, mobility and ranged pressure.",
		Items: []equipmentItemSeed{
			{Code: "greenwisp_bow", Name: "Greenwisp Bow", Slot: "main_hand", ItemType: "bow", Handedness: "two_hand", Rarity: "gray", ClassRestriction: "ranger", ImageURL: "/equipment/ranger/Greenwisp/Greenwisp Bow.png", Bonuses: EquipmentBonuses{Attack: 1, Agility: 1}},
			{Code: "greenwisp_hood", Name: "Greenwisp Hood", Slot: "helmet", ItemType: "helmet", Handedness: "none", Rarity: "gray", ClassRestriction: "ranger", ImageURL: "/equipment/ranger/Greenwisp/Greenwisp Hood.png", Bonuses: EquipmentBonuses{Agility: 1}},
			{Code: "greenwisp_vest", Name: "Greenwisp Vest", Slot: "chest", ItemType: "chest", Handedness: "none", Rarity: "gray", ClassRestriction: "ranger", ImageURL: "/equipment/ranger/Greenwisp/Greenwisp Vest.png", Bonuses: EquipmentBonuses{Defense: 1}},
			{Code: "greenwisp_ranger_pants", Name: "Greenwisp Ranger Pants", Slot: "pants", ItemType: "pants", Handedness: "none", Rarity: "gray", ClassRestriction: "ranger", ImageURL: "/equipment/ranger/Greenwisp/Greenwisp Ranger Pants.png", Bonuses: EquipmentBonuses{Mobility: 1}},
			{Code: "greenwisp_gloves", Name: "Greenwisp Gloves", Slot: "gloves", ItemType: "gloves", Handedness: "none", Rarity: "gray", ClassRestriction: "ranger", ImageURL: "/equipment/ranger/Greenwisp/Greenwisp Gloves.png", Bonuses: EquipmentBonuses{Agility: 1}},
			{Code: "greenwisp_boots", Name: "Greenwisp Boots", Slot: "boots", ItemType: "boots", Handedness: "none", Rarity: "gray", ClassRestriction: "ranger", ImageURL: "/equipment/ranger/Greenwisp/Greenwisp Boots.png", Bonuses: EquipmentBonuses{Mobility: 1}},
		},
		Bonuses: []equipmentSetBonusSeed{
			{PiecesRequired: 2, Description: "+1 Reflex while wearing 2 pieces of Greenwisp Set.", Bonuses: EquipmentBonuses{Agility: 1}},
			{PiecesRequired: 4, Description: "+1 Mobility while wearing 4 pieces of Greenwisp Set.", Bonuses: EquipmentBonuses{Mobility: 1}},
			{PiecesRequired: 6, Description: "+1 Attack while wearing 6 pieces of Greenwisp Set.", Bonuses: EquipmentBonuses{Attack: 1}},
		},
	},
	{
		Code:             "sagecloth_set",
		Name:             "Sagecloth Set",
		ClassRestriction: "mystic",
		Rarity:           "gray",
		Description:      "A beginner mystic set focused on survivability, Reflex and spell pressure.",
		Items: []equipmentItemSeed{
			{Code: "sagecloth_staff", Name: "Sagecloth Staff", Slot: "main_hand", ItemType: "staff", Handedness: "two_hand", Rarity: "gray", ClassRestriction: "mystic", ImageURL: "/equipment/mystic/Elderbloom/Elderbloom Staff.png", Bonuses: EquipmentBonuses{Attack: 2, Agility: 1}},
			{Code: "sagecloth_jacket", Name: "Sagecloth Jacket", Slot: "chest", ItemType: "chest", Handedness: "none", Rarity: "gray", ClassRestriction: "mystic", ImageURL: "/equipment/mystic/Elderbloom/Elderbloom Vest.png", Bonuses: EquipmentBonuses{Defense: 1, MaxHealth: 9}},
			{Code: "sagecloth_pants", Name: "Sagecloth Pants", Slot: "pants", ItemType: "pants", Handedness: "none", Rarity: "gray", ClassRestriction: "mystic", ImageURL: "/equipment/mystic/Elderbloom/Elderbloom Pants.png", Bonuses: EquipmentBonuses{Agility: 1, MaxHealth: 6}},
			{Code: "sagecloth_boots", Name: "Sagecloth Boots", Slot: "boots", ItemType: "boots", Handedness: "none", Rarity: "gray", ClassRestriction: "mystic", ImageURL: "/equipment/mystic/Elderbloom/Elderbloom Boots.png", Bonuses: EquipmentBonuses{Mobility: 1, MaxHealth: 2}},
			{Code: "sagecloth_gloves", Name: "Sagecloth Gloves", Slot: "gloves", ItemType: "gloves", Handedness: "none", Rarity: "gray", ClassRestriction: "mystic", ImageURL: "/equipment/mystic/Elderbloom/Elderbloom Gloves.png", Bonuses: EquipmentBonuses{Attack: 1, Agility: 1}},
			{Code: "sagecloth_hood", Name: "Sagecloth Hood", Slot: "helmet", ItemType: "helmet", Handedness: "none", Rarity: "gray", ClassRestriction: "mystic", ImageURL: "/equipment/mystic/Elderbloom/Elderbloom Hood.png", Bonuses: EquipmentBonuses{Defense: 1, Agility: 1, MaxHealth: 5}},
		},
		Bonuses: []equipmentSetBonusSeed{
			{PiecesRequired: 2, Description: "+5 Max Health while wearing 2 pieces of Sagecloth Set.", Bonuses: EquipmentBonuses{MaxHealth: 5}},
			{PiecesRequired: 4, Description: "+1 Reflex while wearing 4 pieces of Sagecloth Set.", Bonuses: EquipmentBonuses{Agility: 1}},
			{PiecesRequired: 6, Description: "+1 Attack while wearing 6 pieces of Sagecloth Set.", Bonuses: EquipmentBonuses{Attack: 1}},
		},
	},
}

// Historical numbers are private construction metadata, never persisted Common
// templates. Active templates are seeded directly by seedEquipmentSetsTx.
func equipmentBaseForCode(code string) (equipmentItemSeed, bool) {
	for _, set := range equipmentSeedCatalog {
		for _, item := range set.Items {
			if item.Code == code {
				return item, true
			}
		}
	}
	return equipmentItemSeed{}, false
}

func defaultEquipmentSetCodeForClass(heroClassID string) (string, bool) {
	for _, set := range equipmentSetsCatalog {
		if set.HeroClass == NormalizeHeroClassID(heroClassID) && set.Rarity == "green" {
			return set.Code, true
		}
	}
	return "", false
}

func seedEquipmentCatalog() error { return nil }

// PostgreSQL checks the candidate INSERT before ON CONFLICT handles an existing row.
func equipmentImageOrPlaceholder(image string) string {
	image = strings.TrimSpace(image)
	if image == "" {
		return "/ui-icons/backpack.png"
	}
	return image
}
