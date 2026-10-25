package repository

import (
	"database/sql"
	"fmt"
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
			if set.Rarity == "green" {
				// Same existing clone IDs/codes and the same original stat derivation.
				if _, err := tx.Exec(`INSERT INTO item_templates(code,name,set_id,slot,item_type,handedness,rarity,class_restriction,image_url,attack_bonus,defense_bonus,mobility_bonus,agility_bonus,max_health_bonus,sight_range_bonus,attack_range_bonus,pocket_bonus,tradable)
      SELECT $2,$3,$4,slot,item_type,handedness,'green',class_restriction,$5,attack_bonus+CASE WHEN slot='main_hand' THEN 1 ELSE 0 END,defense_bonus+CASE WHEN slot<>'main_hand' THEN 1 ELSE 0 END,mobility_bonus,agility_bonus,max_health_bonus+2,sight_range_bonus,attack_range_bonus,CASE WHEN slot IN ('chest','pants') THEN 1 ELSE 0 END,true FROM item_templates WHERE code=$1 ON CONFLICT(code) DO NOTHING`, item.BaseTemplate, item.Code, item.Name, setID, item.Image); err != nil {
					return err
				}
			}
			result, err := tx.Exec(`UPDATE item_templates it SET name=$2,set_id=$3,rarity=$4,class_restriction=$5,image_url=$6,slot=b.slot,item_type=b.item_type,handedness=b.handedness FROM item_templates b WHERE it.code=$1 AND b.code=$7 AND b.class_restriction=$5 AND b.slot=$8`, item.Code, item.Name, setID, set.Rarity, set.HeroClass, item.Image, item.BaseTemplate, item.Slot)
			if err != nil {
				return err
			}
			n, err := result.RowsAffected()
			if err != nil {
				return err
			}
			if n != 1 {
				return fmt.Errorf("equipment catalog template missing: %s", item.Code)
			}
		}
		if set.Rarity == "green" {
			// Retain the previously registered green bonuses; don't introduce bonuses
			// for Rare sets, whose crafted templates previously had no set membership.
			var legacySet, legacyName string
			for _, base := range equipmentSeedCatalog {
				if base.ClassRestriction == set.HeroClass {
					legacySet, legacyName = base.Code, base.Name
				}
			}
			if _, err := tx.Exec(`INSERT INTO item_set_bonuses(set_id,pieces_required,description,attack_bonus,defense_bonus,mobility_bonus,agility_bonus,max_health_bonus,max_energy_bonus,sight_range_bonus,attack_range_bonus,effect_code,effect_value)
    SELECT $1,b.pieces_required,replace(b.description,s.name,$3),b.attack_bonus,b.defense_bonus,b.mobility_bonus,b.agility_bonus,b.max_health_bonus,b.max_energy_bonus,b.sight_range_bonus,b.attack_range_bonus,b.effect_code,b.effect_value FROM item_set_bonuses b JOIN item_sets s ON s.id=b.set_id WHERE s.code=$2 ON CONFLICT(set_id,pieces_required) DO NOTHING`, setID, legacySet, set.Name); err != nil {
				return err
			}
			if _, err := tx.Exec(`UPDATE item_set_bonuses SET description=replace(description,$2,$3) WHERE set_id=$1`, setID, legacyName, set.Name); err != nil {
				return err
			}

		}
	}
	if err := balanceEquipmentTemplatesTx(tx); err != nil { return err }; return seedJewelryTx(tx)
}
