package repository

import (
	"database/sql"
	"fmt"
	"strings"
)

// Reconcile the existing 25 Uncommon and 25 crafted template codes in place.
// Metadata is reconciled first; Stage 10.4.4 then snapshots legacy instance
// bonuses and updates template stats atomically. Prices/set bonuses stay intact.
func seedEquipmentSetsTx(tx *sql.Tx) error {
	for _, set := range equipmentSetsCatalog {
		var setID int64
		if err := tx.QueryRow(`INSERT INTO item_sets(code,name,class_restriction,rarity,description) VALUES($1,$2,$3,$4,$5)
   ON CONFLICT(code) DO UPDATE SET name=EXCLUDED.name,class_restriction=EXCLUDED.class_restriction,rarity=EXCLUDED.rarity,description=EXCLUDED.description RETURNING id`, set.Code, set.Name, set.HeroClass, set.Rarity, set.Source).Scan(&setID); err != nil {
			return err
		}
		for _, item := range set.Items {
			base, ok := equipmentBaseForCode(item.BaseTemplate)
			if !ok || base.ClassRestriction != set.HeroClass || base.Slot != item.Slot {
				return fmt.Errorf("equipment construction metadata missing: %s", item.Code)
			}
			bonus := base.Bonuses
			rank := 1
			if set.Rarity == "blue" {
				rank = 2
			}
			if base.Slot == "main_hand" {
				bonus.Attack += rank
			} else {
				bonus.Defense += rank
			}
			bonus.MaxHealth += rank * 2
			pockets := 0
			if base.Slot == "chest" || base.Slot == "pants" {
				pockets = rank
			}
			// Preserve existing stats/prices and instance snapshots on conflict.
			_, err := tx.Exec(`INSERT INTO item_templates(code,name,set_id,slot,item_type,handedness,rarity,class_restriction,image_url,attack_bonus,defense_bonus,mobility_bonus,agility_bonus,max_health_bonus,sight_range_bonus,attack_range_bonus,pocket_bonus,tradable)
                VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16,$17,true)
                ON CONFLICT(code) DO UPDATE SET name=EXCLUDED.name,set_id=EXCLUDED.set_id,slot=EXCLUDED.slot,item_type=EXCLUDED.item_type,handedness=EXCLUDED.handedness,rarity=EXCLUDED.rarity,class_restriction=EXCLUDED.class_restriction,image_url=CASE WHEN EXCLUDED.image_url='/ui-icons/backpack.png' AND NULLIF(btrim(item_templates.image_url),'') IS NOT NULL THEN item_templates.image_url ELSE EXCLUDED.image_url END`, item.Code, item.Name, setID, base.Slot, base.ItemType, base.Handedness, set.Rarity, set.HeroClass, equipmentImageOrPlaceholder(item.Image), bonus.Attack, bonus.Defense, bonus.Mobility, bonus.Agility, bonus.MaxHealth, bonus.SightRange, bonus.AttackRange, pockets)
			if err != nil {
				return err
			}

		}
		if set.Rarity == "green" {
			// Retain the previously registered green bonuses; don't introduce bonuses
			// for Rare sets, whose crafted templates previously had no set membership.
			for _, base := range equipmentSeedCatalog {
				if base.ClassRestriction != set.HeroClass {
					continue
				}
				for _, entry := range base.Bonuses {
					v := entry.Bonuses
					_, err := tx.Exec(`INSERT INTO item_set_bonuses(set_id,pieces_required,description,attack_bonus,defense_bonus,mobility_bonus,agility_bonus,max_health_bonus,max_energy_bonus,sight_range_bonus,attack_range_bonus,effect_code,effect_value)
                    VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13) ON CONFLICT(set_id,pieces_required) DO NOTHING`, setID, entry.PiecesRequired, strings.ReplaceAll(entry.Description, base.Name, set.Name), v.Attack, v.Defense, v.Mobility, v.Agility, v.MaxHealth, v.MaxEnergy, v.SightRange, v.AttackRange, entry.EffectCode, entry.EffectValue)
					if err != nil {
						return err
					}
				}
			}

		}
	}
	if err := balanceEquipmentTemplatesTx(tx); err != nil {
		return err
	}
	return seedJewelryTx(tx)
}
