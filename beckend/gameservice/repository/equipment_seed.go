package repository

import (
	"database/sql"
	"fmt"
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
		Rarity:           "green",
		Description:      "A beginner guardian set focused on defense, control and frontline staying power.",
		Items: []equipmentItemSeed{
			{Code: "aegiswarden_sword", Name: "Aegiswarden Sword", Slot: "main_hand", ItemType: "sword", Handedness: "one_hand", Rarity: "green", ClassRestriction: "guardian", ImageURL: "/equipment/guardian/aegiswarden/Aegiswarden Sword.png", Bonuses: EquipmentBonuses{Attack: 1}},
			{Code: "aegiswarden_shield", Name: "Aegiswarden Shield", Slot: "off_hand", ItemType: "shield", Handedness: "one_hand", Rarity: "green", ClassRestriction: "guardian", ImageURL: "/equipment/guardian/aegiswarden/Aegiswarden Shield.png", Bonuses: EquipmentBonuses{Defense: 2}},
			{Code: "aegiswarden_helm", Name: "Aegiswarden Helm", Slot: "helmet", ItemType: "helmet", Handedness: "none", Rarity: "green", ClassRestriction: "guardian", ImageURL: "/equipment/guardian/aegiswarden/Aegiswarden Helm.png", Bonuses: EquipmentBonuses{Defense: 1}},
			{Code: "aegiswarden_vest", Name: "Aegiswarden Vest", Slot: "chest", ItemType: "chest", Handedness: "none", Rarity: "green", ClassRestriction: "guardian", ImageURL: "/equipment/guardian/aegiswarden/Aegiswarden Vest.png", Bonuses: EquipmentBonuses{Defense: 1, MaxHealth: 2}},
			{Code: "aegiswarden_pants", Name: "Aegiswarden Pants", Slot: "pants", ItemType: "pants", Handedness: "none", Rarity: "green", ClassRestriction: "guardian", ImageURL: "/equipment/guardian/aegiswarden/Aegiswarden Pants.png", Bonuses: EquipmentBonuses{Mobility: 1}},
			{Code: "aegiswarden_gloves", Name: "Aegiswarden Gloves", Slot: "gloves", ItemType: "gloves", Handedness: "none", Rarity: "green", ClassRestriction: "guardian", ImageURL: "/equipment/guardian/aegiswarden/Aegiswarden Gloves.png", Bonuses: EquipmentBonuses{Agility: 1}},
			{Code: "aegiswarden_boots", Name: "Aegiswarden Boots", Slot: "boots", ItemType: "boots", Handedness: "none", Rarity: "green", ClassRestriction: "guardian", ImageURL: "/equipment/guardian/aegiswarden/boots.png", Bonuses: EquipmentBonuses{Mobility: 1}},
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
		Rarity:           "green",
		Description:      "A beginner berserker set focused on melee pressure and early survivability.",
		Items: []equipmentItemSeed{
			{Code: "bloodroot_axe", Name: "Bloodroot Axe", Slot: "main_hand", ItemType: "axe", Handedness: "two_hand", Rarity: "green", ClassRestriction: "berserker", ImageURL: "/equipment/berserker/bloodroot/Bloodroot Axe.png", Bonuses: EquipmentBonuses{Attack: 2}},
			{Code: "bloodroot_horned_helm", Name: "Bloodroot Horned Helm", Slot: "helmet", ItemType: "helmet", Handedness: "none", Rarity: "green", ClassRestriction: "berserker", ImageURL: "/equipment/berserker/bloodroot/Bloodroot Horned Helm.png", Bonuses: EquipmentBonuses{MaxHealth: 1}},
			{Code: "bloodroot_war_vest", Name: "Bloodroot War Vest", Slot: "chest", ItemType: "chest", Handedness: "none", Rarity: "green", ClassRestriction: "berserker", ImageURL: "/equipment/berserker/bloodroot/Bloodroot War Vest.png", Bonuses: EquipmentBonuses{Defense: 1, MaxHealth: 2}},
			{Code: "bloodroot_raider_pants", Name: "Bloodroot Raider Pants", Slot: "pants", ItemType: "pants", Handedness: "none", Rarity: "green", ClassRestriction: "berserker", ImageURL: "/equipment/berserker/bloodroot/Bloodroot Raider Pants.png", Bonuses: EquipmentBonuses{Mobility: 1}},
			{Code: "bloodroot_grips", Name: "Bloodroot Grips", Slot: "gloves", ItemType: "gloves", Handedness: "none", Rarity: "green", ClassRestriction: "berserker", ImageURL: "/equipment/berserker/bloodroot/Bloodroot Grips.png", Bonuses: EquipmentBonuses{Attack: 1}},
			{Code: "bloodroot_stompers", Name: "Bloodroot Stompers", Slot: "boots", ItemType: "boots", Handedness: "none", Rarity: "green", ClassRestriction: "berserker", ImageURL: "/equipment/berserker/bloodroot/Bloodroot Stompers.png", Bonuses: EquipmentBonuses{Mobility: 1}},
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
		Rarity:           "green",
		Description:      "A beginner ranger set focused on Reflex, mobility and ranged pressure.",
		Items: []equipmentItemSeed{
			{Code: "greenwisp_bow", Name: "Greenwisp Bow", Slot: "main_hand", ItemType: "bow", Handedness: "two_hand", Rarity: "green", ClassRestriction: "ranger", ImageURL: "/equipment/ranger/greenwisp/Greenwisp Bow.png", Bonuses: EquipmentBonuses{Attack: 1, Agility: 1}},
			{Code: "greenwisp_hood", Name: "Greenwisp Hood", Slot: "helmet", ItemType: "helmet", Handedness: "none", Rarity: "green", ClassRestriction: "ranger", ImageURL: "/equipment/ranger/greenwisp/Greenwisp Hood.png", Bonuses: EquipmentBonuses{Agility: 1}},
			{Code: "greenwisp_vest", Name: "Greenwisp Vest", Slot: "chest", ItemType: "chest", Handedness: "none", Rarity: "green", ClassRestriction: "ranger", ImageURL: "/equipment/ranger/greenwisp/Greenwisp Vest.png", Bonuses: EquipmentBonuses{Defense: 1}},
			{Code: "greenwisp_ranger_pants", Name: "Greenwisp Ranger Pants", Slot: "pants", ItemType: "pants", Handedness: "none", Rarity: "green", ClassRestriction: "ranger", ImageURL: "/equipment/ranger/greenwisp/Greenwisp Ranger Pants.png", Bonuses: EquipmentBonuses{Mobility: 1}},
			{Code: "greenwisp_gloves", Name: "Greenwisp Gloves", Slot: "gloves", ItemType: "gloves", Handedness: "none", Rarity: "green", ClassRestriction: "ranger", ImageURL: "/equipment/ranger/greenwisp/Greenwisp Gloves.png", Bonuses: EquipmentBonuses{Agility: 1}},
			{Code: "greenwisp_boots", Name: "Greenwisp Boots", Slot: "boots", ItemType: "boots", Handedness: "none", Rarity: "green", ClassRestriction: "ranger", ImageURL: "/equipment/ranger/greenwisp/Greenwisp Boots.png", Bonuses: EquipmentBonuses{Mobility: 1}},
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
		Rarity:           "green",
		Description:      "A beginner mystic set focused on energy, Reflex and early spell tempo.",
		Items: []equipmentItemSeed{
			{Code: "sagecloth_staff", Name: "Sagecloth Staff", Slot: "main_hand", ItemType: "staff", Handedness: "two_hand", Rarity: "green", ClassRestriction: "mystic", ImageURL: "/equipment/mystic/sagecloth/staff.png", Bonuses: EquipmentBonuses{Attack: 1, Agility: 1, MaxEnergy: 4}},
			{Code: "sagecloth_jacket", Name: "Sagecloth Jacket", Slot: "chest", ItemType: "chest", Handedness: "none", Rarity: "green", ClassRestriction: "mystic", ImageURL: "/equipment/mystic/sagecloth/jacket.png", Bonuses: EquipmentBonuses{Defense: 1, MaxHealth: 6, MaxEnergy: 3}},
			{Code: "sagecloth_pants", Name: "Sagecloth Pants", Slot: "pants", ItemType: "pants", Handedness: "none", Rarity: "green", ClassRestriction: "mystic", ImageURL: "/equipment/mystic/sagecloth/pants.png", Bonuses: EquipmentBonuses{Agility: 1, MaxHealth: 3, MaxEnergy: 3}},
			{Code: "sagecloth_boots", Name: "Sagecloth Boots", Slot: "boots", ItemType: "boots", Handedness: "none", Rarity: "green", ClassRestriction: "mystic", ImageURL: "/equipment/mystic/sagecloth/boots.png", Bonuses: EquipmentBonuses{Mobility: 1, MaxEnergy: 2}},
			{Code: "sagecloth_gloves", Name: "Sagecloth Gloves", Slot: "gloves", ItemType: "gloves", Handedness: "none", Rarity: "green", ClassRestriction: "mystic", ImageURL: "/equipment/mystic/sagecloth/gloves.png", Bonuses: EquipmentBonuses{Attack: 1, MaxEnergy: 3}},
			{Code: "sagecloth_hood", Name: "Sagecloth Hood", Slot: "helmet", ItemType: "helmet", Handedness: "none", Rarity: "green", ClassRestriction: "mystic", ImageURL: "/equipment/mystic/sagecloth/hood.png", Bonuses: EquipmentBonuses{Defense: 1, Agility: 1, MaxHealth: 2, MaxEnergy: 3}},
		},
		Bonuses: []equipmentSetBonusSeed{
			{PiecesRequired: 2, Description: "+5 Max Energy while wearing 2 pieces of Sagecloth Set.", Bonuses: EquipmentBonuses{MaxEnergy: 5}},
			{PiecesRequired: 4, Description: "+1 Reflex while wearing 4 pieces of Sagecloth Set.", Bonuses: EquipmentBonuses{Agility: 1}},
			{PiecesRequired: 6, Description: "+1 Attack while wearing 6 pieces of Sagecloth Set.", Bonuses: EquipmentBonuses{Attack: 1}},
		},
	},
}

func defaultEquipmentSetCodeForClass(heroClassID string) (string, bool) {
	heroClassID = NormalizeHeroClassID(heroClassID)
	for _, set := range equipmentSeedCatalog {
		if set.ClassRestriction == heroClassID {
			return set.Code, true
		}
	}
	return "", false
}

func seedEquipmentCatalog() error {
	tx, err := DB.Begin()
	if err != nil {
		return fmt.Errorf("seed equipment catalog begin tx: %w", err)
	}
	defer tx.Rollback()

	for _, set := range equipmentSeedCatalog {
		var setID int64
		if err := tx.QueryRow(`
			INSERT INTO item_sets (code, name, class_restriction, rarity, description)
			VALUES ($1, $2, $3, $4, $5)
			ON CONFLICT (code) DO UPDATE SET
				name = EXCLUDED.name,
				class_restriction = EXCLUDED.class_restriction,
				rarity = EXCLUDED.rarity,
				description = EXCLUDED.description,
				updated_at = now()
			RETURNING id
		`, set.Code, set.Name, set.ClassRestriction, set.Rarity, set.Description).Scan(&setID); err != nil {
			return fmt.Errorf("seed equipment set %s: %w", set.Code, err)
		}

		for _, item := range set.Items {
			if _, err := tx.Exec(`
				INSERT INTO item_templates (
					code,
					name,
					set_id,
					slot,
					item_type,
					handedness,
					rarity,
					class_restriction,
					image_url,
					attack_bonus,
					defense_bonus,
					mobility_bonus,
					agility_bonus,
					max_health_bonus,
					max_energy_bonus,
					sight_range_bonus,
					attack_range_bonus
				)
				VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16, $17)
				ON CONFLICT (code) DO UPDATE SET
					name = EXCLUDED.name,
					set_id = EXCLUDED.set_id,
					slot = EXCLUDED.slot,
					item_type = EXCLUDED.item_type,
					handedness = EXCLUDED.handedness,
					rarity = EXCLUDED.rarity,
					class_restriction = EXCLUDED.class_restriction,
					image_url = EXCLUDED.image_url,
					attack_bonus = EXCLUDED.attack_bonus,
					defense_bonus = EXCLUDED.defense_bonus,
					mobility_bonus = EXCLUDED.mobility_bonus,
					agility_bonus = EXCLUDED.agility_bonus,
					max_health_bonus = EXCLUDED.max_health_bonus,
					max_energy_bonus = EXCLUDED.max_energy_bonus,
					sight_range_bonus = EXCLUDED.sight_range_bonus,
					attack_range_bonus = EXCLUDED.attack_range_bonus,
					updated_at = now()
			`,
				item.Code,
				item.Name,
				setID,
				item.Slot,
				item.ItemType,
				item.Handedness,
				item.Rarity,
				item.ClassRestriction,
				item.ImageURL,
				item.Bonuses.Attack,
				item.Bonuses.Defense,
				item.Bonuses.Mobility,
				item.Bonuses.Agility,
				item.Bonuses.MaxHealth,
				item.Bonuses.MaxEnergy,
				item.Bonuses.SightRange,
				item.Bonuses.AttackRange,
			); err != nil {
				return fmt.Errorf("seed equipment item %s: %w", item.Code, err)
			}
		}

		for _, bonus := range set.Bonuses {
			effectCode := sql.NullString{}
			if bonus.EffectCode != nil {
				effectCode = sql.NullString{String: *bonus.EffectCode, Valid: true}
			}
			if _, err := tx.Exec(`
				INSERT INTO item_set_bonuses (
					set_id,
					pieces_required,
					description,
					attack_bonus,
					defense_bonus,
					mobility_bonus,
					agility_bonus,
					max_health_bonus,
					max_energy_bonus,
					sight_range_bonus,
					attack_range_bonus,
					effect_code,
					effect_value
				)
				VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13)
				ON CONFLICT (set_id, pieces_required) DO UPDATE SET
					description = EXCLUDED.description,
					attack_bonus = EXCLUDED.attack_bonus,
					defense_bonus = EXCLUDED.defense_bonus,
					mobility_bonus = EXCLUDED.mobility_bonus,
					agility_bonus = EXCLUDED.agility_bonus,
					max_health_bonus = EXCLUDED.max_health_bonus,
					max_energy_bonus = EXCLUDED.max_energy_bonus,
					sight_range_bonus = EXCLUDED.sight_range_bonus,
					attack_range_bonus = EXCLUDED.attack_range_bonus,
					effect_code = EXCLUDED.effect_code,
					effect_value = EXCLUDED.effect_value
			`,
				setID,
				bonus.PiecesRequired,
				bonus.Description,
				bonus.Bonuses.Attack,
				bonus.Bonuses.Defense,
				bonus.Bonuses.Mobility,
				bonus.Bonuses.Agility,
				bonus.Bonuses.MaxHealth,
				bonus.Bonuses.MaxEnergy,
				bonus.Bonuses.SightRange,
				bonus.Bonuses.AttackRange,
				effectCode,
				bonus.EffectValue,
			); err != nil {
				return fmt.Errorf("seed equipment set bonus %s/%d: %w", set.Code, bonus.PiecesRequired, err)
			}
		}
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("seed equipment catalog commit: %w", err)
	}
	return nil
}
