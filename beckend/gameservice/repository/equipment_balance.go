package repository

import (
	"database/sql"
	"fmt"
	"strings"
)

// Instance snapshots live in the existing event metadata; no schema migration.
// All instance reads must use this expression, while template/recipe previews
// intentionally show the current catalog. Never replace a stored snapshot.
// Pocket bonuses follow the current template even for legacy stat snapshots.
// Only existing green/blue chest/pants may grant them; bad legacy values cannot
// turn another slot into a pocket source or exceed +1/+2 per item.
const equipmentPocketBonusSQL = `CASE WHEN it.slot IN ('chest','pants') AND it.rarity IN ('green','blue') THEN LEAST(GREATEST(it.pocket_bonus,0),CASE WHEN it.rarity='blue' THEN 2 ELSE 1 END) ELSE 0 END`

func equipmentInstanceStatSQL(column string) string {
	return "COALESCE((SELECT (e.metadata->'equipmentBalance1044'->>'" + column + "')::integer FROM item_instance_events e WHERE e.item_instance_id=ii.id AND e.metadata ? 'equipmentBalance1044' ORDER BY e.id LIMIT 1),it." + column + ")"
}

func equipmentInstanceColumnsSQL(columns string) string {
	for _, column := range []string{"attack_bonus", "defense_bonus", "mobility_bonus", "agility_bonus", "max_health_bonus", "max_energy_bonus", "sight_range_bonus", "attack_range_bonus"} {
		columns = strings.ReplaceAll(columns, "it."+column, equipmentInstanceStatSQL(column))
	}
	return columns
}

// Called inside the existing catalog seed transaction, after all 50 templates
// exist. Exclusive locks serialize other startup seeds and instance acquisition.
// Store the old numbers before changing any template; rollback covers both.
func balanceEquipmentTemplatesTx(tx *sql.Tx) error {
	if _, err := tx.Exec(`LOCK TABLE item_templates,item_instances,item_instance_events IN SHARE ROW EXCLUSIVE MODE`); err != nil {
		return err
	}
	for _, set := range equipmentSetsCatalog {
		for _, item := range set.Items {
			b := item.Bonuses
			// A restart with unchanged template values creates no new snapshots.
			args := []any{item.Code, b.Attack, b.Defense, b.Mobility, b.Agility, b.MaxHealth, b.SightRange, b.AttackRange}
			different := `(it.attack_bonus,it.defense_bonus,it.mobility_bonus,it.agility_bonus,it.max_health_bonus,it.sight_range_bonus,it.attack_range_bonus) IS DISTINCT FROM ($2::int,$3::int,$4::int,$5::int,$6::int,$7::int,$8::int)`
			// Attach to the original creation event, retaining all unrelated metadata.
			// Older imports without a creation event receive a marked historical record.
			_, err := tx.Exec(`WITH candidates AS (
    SELECT ii.id,to_jsonb(it) AS snapshot,(SELECT min(e.id) FROM item_instance_events e WHERE e.item_instance_id=ii.id AND e.event_type='created') AS event_id
    FROM item_instances ii JOIN item_templates it ON it.id=ii.template_id
    WHERE it.code=$1 AND `+different+` AND NOT EXISTS(SELECT 1 FROM item_instance_events e WHERE e.item_instance_id=ii.id AND e.metadata ? 'equipmentBalance1044')
   ), updated AS (
    UPDATE item_instance_events e SET metadata=e.metadata || jsonb_build_object('equipmentBalance1044',c.snapshot)
    FROM candidates c WHERE e.id=c.event_id RETURNING e.id
   ) INSERT INTO item_instance_events(item_instance_id,event_type,metadata)
   SELECT id,'created',jsonb_build_object('equipmentBalance1044',snapshot,'historicalImport',true) FROM candidates WHERE event_id IS NULL`, args...)
			if err != nil {
				return fmt.Errorf("snapshot equipment %s: %w", item.Code, err)
			}
			_, err = tx.Exec(`UPDATE item_templates SET attack_bonus=$2,defense_bonus=$3,mobility_bonus=$4,agility_bonus=$5,max_health_bonus=$6,sight_range_bonus=$7,attack_range_bonus=$8,pocket_bonus=$9 WHERE code=$1`, append(args, b.PocketBonus)...)
			if err != nil {
				return fmt.Errorf("balance equipment %s: %w", item.Code, err)
			}
		}
	}
	return nil
}
