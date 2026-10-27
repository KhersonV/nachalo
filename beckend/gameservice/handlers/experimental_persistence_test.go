package handlers

import (
	"context"
	"database/sql"
	"os"
	"testing"

	"gameservice/internal/game/balance"
	"gameservice/internal/hexmap"
	"gameservice/repository"
)

func TestExperimentalV34PersistencePostgres(t *testing.T) {
	if os.Getenv("STAGE2_TEST_DSN") != "postgres://v34_validation@127.0.0.1:55434/nachalo_v34_validation?sslmode=disable" {
		t.Skip("dedicated validation cluster required")
	}
	t.Setenv("NACHALO_COMBAT_RULESET", "legacy")
	db, dsn := hexCombatDatabase(t)
	ctx := context.Background()
	legacy, err := repository.CreateHexTestMatch(ctx, 781, 97)
	if err != nil {
		t.Fatal(err)
	}
	t.Setenv("NACHALO_COMBAT_RULESET", "experimental_v34")
	t.Setenv("NACHALO_COMBAT_SEED", "34")
	character, err := repository.GetSelectedCharacterForUser(781)
	if err != nil {
		t.Fatal(err)
	}
	item, err := repository.GrantItemInstanceToUser(781, "aegiswarden_vest_uncommon", "shop")
	if err != nil {
		t.Fatal(err)
	}
	if err = repository.EquipItemToCharacter(781, character.ID, item.InstanceID); err != nil {
		t.Fatal(err)
	}
	var itemBefore string
	if err = db.QueryRow(`SELECT to_jsonb(ii)::text FROM item_instances ii WHERE id=$1`, item.InstanceID).Scan(&itemBefore); err != nil {
		t.Fatal(err)
	}
	if _, err = repository.CreatePlayerProfile(repository.CreatePlayerProfileInput{UserID: 782, Name: "Buffer persistence", CharacterType: "mystic"}); err != nil {
		t.Fatal(err)
	}
	match, err := repository.CreateHexMatch(ctx, repository.HexMatchPlan{InstanceID: "v34-persistence", Mode: "hex-test", Teams: []hexmap.Team{{ID: 1, PlayerIDs: []int{781}}, {ID: 2, PlayerIDs: []int{782}}}, TurnOrder: []int{781, 782}, Seed: 97})
	if err != nil {
		t.Fatal(err)
	}
	if match.Player.MaxHealth != 224 || legacy.Player.MaxHealth != 130 {
		t.Fatal("ruleset/partial gear snapshot mismatch")
	}
	c, err := repository.BeginMatchCommand(ctx, match.InstanceID, 781, "persistent-draw", "persistent-draw")
	if err != nil {
		t.Fatal(err)
	}
	defer c.Rollback()
	before := c.RandomCounter
	if c.RandomSeed != 34 || c.RandomUnit() != balance.V34RandomUnit(34, before) {
		t.Fatal("seed/counter mismatch")
	}
	result, err := c.ResolveResearchDamageTx(ctx, 782, balance.V34Incoming{Kind: "counter", Damage: 1, Defender: "mystic"})
	if err != nil || result.Damage != 0 || result.Shield != 0 {
		t.Fatalf("buffer: %+v %v", result, err)
	}
	body := map[string]any{"draw": true}
	if _, _, err = c.Commit(ctx, &body, false); err != nil {
		t.Fatal(err)
	}
	t.Setenv("NACHALO_COMBAT_RULESET", "legacy")
	// Fresh connection pool exercises durable state, without running startup
	// workers or changing any existing match's ruleset/config/effects.
	fresh, err := sql.Open("postgres", dsn)
	if err != nil {
		t.Fatal(err)
	}
	defer fresh.Close()
	repository.DB = fresh
	defer func() { repository.DB = db }()
	rules, cfg, err := repository.ReadMatchCombatRulesTx(ctx, fresh, match.InstanceID)
	if err != nil || rules != balance.ExperimentalV34 || cfg == nil || cfg.HatredPercent != 16 {
		t.Fatal("frozen configuration lost")
	}
	oldRules, _, err := repository.ReadMatchCombatRulesTx(ctx, fresh, legacy.InstanceID)
	if err != nil || oldRules != balance.Legacy {
		t.Fatal("legacy match retuned")
	}
	replay, err := repository.BeginMatchCommand(ctx, match.InstanceID, 781, "persistent-draw", "persistent-draw")
	if err != nil {
		t.Fatal(err)
	}
	defer replay.Rollback()
	if replay.Replay == nil {
		t.Fatal("receipt lost on reconnect")
	}
	check := func() {
		var counter int64
		var shield int
		if err = fresh.QueryRow(`SELECT combat_rng_counter FROM matches WHERE instance_id=$1`, match.InstanceID).Scan(&counter); err != nil {
			t.Fatal(err)
		}
		if err = fresh.QueryRow(`SELECT combat_magic_shield FROM match_players WHERE instance_id=$1 AND user_id=782`, match.InstanceID).Scan(&shield); err != nil {
			t.Fatal(err)
		}
		if counter != before+1 || shield != 0 {
			t.Fatalf("replay/reset: counter=%d shield=%d", counter, shield)
		}
	}
	check()
	rollback, err := repository.BeginMatchCommand(ctx, match.InstanceID, 781, "rolled-back", "rolled-back")
	if err != nil {
		t.Fatal(err)
	}
	rollback.RandomUnit()
	if _, err = rollback.Tx.Exec(`UPDATE match_players SET combat_magic_shield=9 WHERE instance_id=$1 AND user_id=782`, match.InstanceID); err != nil {
		rollback.Rollback()
		t.Fatal(err)
	}
	rollback.Rollback()
	check()
	if err = repository.EnsureHexActionSchema(); err != nil {
		t.Fatal("repeat schema application:", err)
	}
	check()
	var itemAfter string
	if err = fresh.QueryRow(`SELECT to_jsonb(ii)::text FROM item_instances ii WHERE id=$1`, item.InstanceID).Scan(&itemAfter); err != nil {
		t.Fatal(err)
	}
	if itemBefore != itemAfter {
		t.Fatal("match mutated owned item")
	}
}
