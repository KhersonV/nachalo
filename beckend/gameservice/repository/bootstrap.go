package repository

import (
	"context"
	"embed"
	"encoding/json"
	"fmt"
	"os"
	"time"

	"gameservice/internal/game/balance"
)

// Existing stage DDL, embedded so a deployed binary needs no docs directory.
//
//go:embed sql/hex_bootstrap/*.sql
var bootstrapSQL embed.FS

// Run after base tables and before readiness checks. No player reset is wired
// here. The ledger and each stage commit together under one startup lock.
func EnsureAutomaticHexSchema(ctx context.Context) error {
	tx, err := DB.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err = tx.ExecContext(ctx, `SELECT pg_advisory_xact_lock(711010,1);
 CREATE TABLE IF NOT EXISTS game_schema_migrations(version text PRIMARY KEY,applied_at timestamptz NOT NULL DEFAULT now());`); err != nil {
		return err
	}
	stages := []struct{ name, adopted string }{
		{"stage1_geometry", `SELECT to_regclass('public.hex_stage1_migration_marker') IS NOT NULL`},
		{"stage2_commands", ""}, {"stage3_hex_pve", ""}, {"stage4_storage_loadout", ""},
		{"stage5_quest_artifact", ""}, {"stage5_portal_extraction", ""}, {"stage6_fog_turn", ""},
		{"stage8_1_guardian_aura", ""}, {"stage10_presence", ""},
	}
	for _, stage := range stages {
		var applied bool
		if err = tx.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM game_schema_migrations WHERE version=$1)`, stage.name).Scan(&applied); err != nil {
			return err
		}
		if applied {
			continue
		}
		adopted := false
		if stage.adopted != "" {
			if err = tx.QueryRowContext(ctx, stage.adopted).Scan(&adopted); err != nil {
				return err
			}
		}
		if !adopted {
			ddl, e := bootstrapSQL.ReadFile("sql/hex_bootstrap/" + stage.name + ".sql")
			if e != nil {
				return e
			}
			if _, err = tx.ExecContext(ctx, string(ddl)); err != nil {
				return fmt.Errorf("bootstrap %s: %w", stage.name, err)
			}
		} else {
			var version bool
			if err = tx.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM hex_stage1_migration_marker WHERE version=1)`).Scan(&version); err != nil {
				return err
			}
			if !version {
				return fmt.Errorf("incomplete existing HEX Stage 1 marker")
			}
			var guards bool
			if err = tx.QueryRowContext(ctx, `SELECT count(*)=4 FROM pg_trigger WHERE NOT tgisinternal AND (
             (tgname='geometry_cells_guard' AND tgrelid=to_regclass('public.match_map_cells')) OR
             (tgname='geometry_monsters_guard' AND tgrelid=to_regclass('public.match_monsters')) OR
             (tgname='geometry_players_guard' AND tgrelid=to_regclass('public.match_players')) OR
             (tgname='geometry_edges_guard' AND tgrelid=to_regclass('public.match_hex_edges')))`).Scan(&guards); err != nil {
				return err
			}
			if !guards {
				return fmt.Errorf("existing HEX Stage 1 is incomplete: geometry guards missing")
			}

		}
		if _, err = tx.ExecContext(ctx, `INSERT INTO game_schema_migrations(version) VALUES($1)`, stage.name); err != nil {
			return err
		}
	}
	return tx.Commit()
}

// Records what the running binary actually registered, not an alternative
// source of balance/recipes. SQL verification cannot read the server's ENV.
func RecordGameContentBootstrap(ctx context.Context) error {
	rules, err := balance.ParseCombatRuleset(os.Getenv("NACHALO_COMBAT_RULESET"))
	if err != nil {
		return err
	}
	for _, table := range []string{"monsters", "resources", "artifacts", "item_templates", "profession_catalog", "market_stack_catalog"} {
		var count int
		if err = DB.QueryRowContext(ctx, `SELECT count(*) FROM `+table).Scan(&count); err != nil {
			return err
		}
		if count == 0 {
			return fmt.Errorf("bootstrap content missing: %s", table)
		}
	}
	for _, set := range append(append([]EquipmentCatalogSet{}, equipmentSetsCatalog...), jewelryCatalog.Sets...) {
		for _, item := range set.Items {
			var exists bool
			if err = DB.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM item_templates WHERE code=$1 AND rarity=$2)`, item.Code, set.Rarity).Scan(&exists); err != nil {
				return err
			}
			if !exists {
				return fmt.Errorf("bootstrap equipment missing: %s", item.Code)
			}
		}
	}
	metadata, err := json.Marshal(map[string]any{"ruleset": rules, "recipes": craftingRecipes, "equipmentSets": equipmentSetsCatalog, "jewelrySets": jewelryCatalog.Sets, "equipmentChancePercent": EquipmentDropChancePercent()})
	if err != nil {
		return err
	}
	_, err = DB.ExecContext(ctx, `CREATE TABLE IF NOT EXISTS game_content_bootstrap(key text PRIMARY KEY,metadata jsonb NOT NULL,updated_at timestamptz NOT NULL DEFAULT now());
 `)
	if err != nil {
		return err
	}
	_, err = DB.ExecContext(ctx, `INSERT INTO game_content_bootstrap(key,metadata) VALUES('current',$1::jsonb) ON CONFLICT(key) DO UPDATE SET metadata=EXCLUDED.metadata,updated_at=now()`, string(metadata))
	return err
}

// Reset deliberately preserves identity sequences: deleted IDs are never reused.
func AuthenticationPlayerExists(ctx context.Context, userID int) (bool, error) {
	if userID <= 0 {
		return false, nil
	}
	if DB == nil {
		return false, fmt.Errorf("database is not initialized")
	}
	ctx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()
	var exists bool
	err := DB.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM player_profiles WHERE user_id=$1)`, userID).Scan(&exists)
	return exists, err
}
