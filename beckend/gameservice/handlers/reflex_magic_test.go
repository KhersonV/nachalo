package handlers

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"reflect"
	"testing"

	"gameservice/internal/game/balance"
	"gameservice/repository"
)

func TestMysticAttackOutcomeGate(t *testing.T) {
	cfg := balance.ExperimentalV34Config()
	dodge := []CombatEffect{{Kind: "dodge", Succeeded: true}}
	if !mysticAttackConnects(&cfg, false, true, nil) || mysticAttackConnects(&cfg, true, false, dodge) || !mysticAttackConnects(&cfg, true, false, nil) {
		t.Fatal("block/dodge/hit gate")
	}
	old := cfg
	old.MysticMagicThroughBlock = false
	if mysticAttackConnects(&old, false, true, nil) || !mysticAttackConnects(&old, true, false, dodge) {
		t.Fatal("historical v34 gate changed")
	}
	if mysticAttackConnects(nil, false, true, nil) {
		t.Fatal("legacy block gate changed")
	}
}

func TestV34MysticPenetrationPostgres(t *testing.T) {
	output := os.Getenv("NACHALO_V34_MAGIC_SCENARIOS")
	if output == "" {
		t.Skip("opt in with NACHALO_V34_MAGIC_SCENARIOS")
	}
	if os.Getenv("STAGE2_TEST_DSN") != "postgres://v34_validation@127.0.0.1:55434/nachalo_v34_validation?sslmode=disable" {
		t.Fatal("isolated validation cluster required")
	}
	t.Setenv("NACHALO_COMBAT_RULESET", "experimental_v34")
	db, fixtureDSN := hexCombatDatabase(t)
	for _, row := range []struct {
		id    int
		class string
	}{{782, "mystic"}, {783, "ranger"}, {784, "guardian"}} {
		if _, err := repository.CreatePlayerProfile(repository.CreatePlayerProfileInput{UserID: row.id, Name: fmt.Sprintf("Magic %d", row.id), CharacterType: row.class}); err != nil {
			t.Fatal(err)
		}
	}
	file, err := os.OpenFile(output, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	encoder := json.NewEncoder(file)
	ctx := context.Background()
	// Search only deterministic RNG seeds for the requested outcomes, never combat parameters.
	noProc := int64(0)
	for seed := int64(1); seed < 1000000; seed++ {
		ok := true
		for i := int64(0); i < 3; i++ {
			if int(balance.V34RandomUnit(seed, 2*i)*100) >= 7 || int(balance.V34RandomUnit(seed, 2*i+1)*100) < 10 {
				ok = false
			}
		}
		if ok {
			noProc = seed
			break
		}
	}
	proc := int64(0)
	for seed := int64(1); seed < 1000000; seed++ {
		if int(balance.V34RandomUnit(seed, 0)*100) < 7 && int(balance.V34RandomUnit(seed, 1)*100) < 10 && int(balance.V34RandomUnit(seed, 2)*100) < 7 {
			proc = seed
			break
		}
	}
	if noProc == 0 || proc == 0 {
		t.Fatal("seed search failed")
	}
	fixture := func(name string, distance, count, hp int, seed int64, attack int) (string, []v34TransitionActor) {
		t.Helper()
		actors := []v34TransitionActor{{782, 1, 0, 0, 0, attack}, {781, 2, distance, 0, hp, 0}, {784, 3, -2, 0, 0, 0}}
		id := "magic-" + name
		v34TransitionFixture(t, db, id, "hex-test", actors)
		if _, err := db.Exec(`UPDATE matches SET combat_rng_seed=$2,combat_rng_counter=0 WHERE instance_id=$1`, id, seed); err != nil {
			t.Fatal(err)
		}
		if _, err := db.Exec(`UPDATE match_players SET hex_overload_target='player:781',hex_overload_count=$2,combat_magic_shield=0 WHERE instance_id=$1 AND user_id=782`, id, count); err != nil {
			t.Fatal(err)
		}
		return id, actors
	}
	attack := func(id string, target int, command string) HexAttackResult {
		t.Helper()
		r, err := ApplyHexPlayerAttack(ctx, id, 782, target, command, nil)
		if err != nil {
			t.Fatal(err)
		}
		return r
	}
	has := func(r HexAttackResult, kind string) bool {
		for _, e := range r.Effects {
			if e.Kind == kind && e.Succeeded {
				return true
			}
		}
		return false
	}
	damage := func(r HexAttackResult, kind string) int {
		n := 0
		for _, s := range r.Steps {
			if s.Kind == kind {
				n += s.Damage
			}
		}
		return n
	}
	record := func(name string, before, after v34TransitionSnapshot, hits []HexAttackResult) {
		t.Helper()
		if err := encoder.Encode(map[string]any{"scenario": name, "before": before, "after": after, "hits": hits, "passed": true}); err != nil {
			t.Fatal(err)
		}
	}
	t.Run("three_blocks_and_personal_turn_reset", func(t *testing.T) {
		id, actors := fixture("three-blocks", 2, 0, 0, noProc, 0)
		before := v34TransitionSnapshotOf(t, db, id, actors)
		hits := []HexAttackResult{}
		for i, want := range []int{0, 3, 7} {
			r := attack(id, 781, fmt.Sprintf("blocked-%d", i))
			hits = append(hits, r)
			if !has(r, "block") || damage(r, "hit") != 0 || damage(r, "mysticOverload") != want || has(r, "dodge") {
				t.Fatalf("block #%d result %+v", i+1, r)
			}
			state := v34TransitionSnapshotOf(t, db, id, actors)
			if state.Players[0].Overload != i+1 || state.RNG != int64(2*(i+1)) {
				t.Fatalf("series/RNG %+v", state)
			}
		}
		after := v34TransitionSnapshotOf(t, db, id, actors)
		if after.Players[1].HP != 218-10 || !after.Players[1].Drain || after.Active != 782 {
			t.Fatalf("magic/drain/turn %+v", after)
		}
		_, err := repository.ApplyHexEndTurn(ctx, id, 782, "end-mystic", after.Turn, nil)
		if err != nil {
			t.Fatal(err)
		}
		drained := v34TransitionSnapshotOf(t, db, id, actors)
		if drained.Active != 781 || drained.Players[1].Drain || drained.Players[1].MP != drained.Players[1].MaxMP-2 {
			t.Fatal("MP Drain not applied on turn start")
		}
		_, err = repository.ApplyHexEndTurn(ctx, id, 781, "end-guardian", drained.Turn, nil)
		if err != nil {
			t.Fatal(err)
		}
		reset := v34TransitionSnapshotOf(t, db, id, actors)
		// Third actor has the intervening turn before Mystic's next personal turn.
		if reset.Active == 784 {
			if _, err = repository.ApplyHexEndTurn(ctx, id, 784, "end-third", reset.Turn, nil); err != nil {
				t.Fatal(err)
			}
			reset = v34TransitionSnapshotOf(t, db, id, actors)
		}
		if reset.Active != 782 || reset.Players[0].Overload != 0 || reset.Players[0].ReflexUsed || reset.Players[1].Drain {
			t.Fatalf("personal reset/drain %+v", reset)
		}
		record("three_blocks_and_turn_reset", before, reset, hits)
	})
	for _, tc := range []struct {
		name                                   string
		distance, count, hp, attack, wantMagic int
		seed                                   int64
	}{
		{"reflex_through_block", 2, 0, 0, 0, 5, proc},
		{"weapon_scaled_magic", 2, 1, 0, 30, 4, noProc},
		{"guardian_dies_through_block", 1, 1, 3, 0, 3, noProc},
		{"melee_counter_after_block", 1, 1, 0, 0, 3, noProc},
		{"fourth_hit_no_overload", 2, 3, 0, 0, 0, noProc},
		{"fifth_hit_no_overload", 2, 4, 0, 0, 0, noProc},
	} {
		t.Run(tc.name, func(t *testing.T) {
			id, actors := fixture(tc.name, tc.distance, tc.count, tc.hp, tc.seed, tc.attack)
			before := v34TransitionSnapshotOf(t, db, id, actors)
			r := attack(id, 781, "single-magic")
			after := v34TransitionSnapshotOf(t, db, id, actors)
			if !has(r, "block") || damage(r, "hit") != 0 || damage(r, "mysticOverload") != tc.wantMagic {
				t.Fatalf("magic through block %+v", r)
			}
			if tc.name == "guardian_dies_through_block" && (after.Players[1].HP != 0 || damage(r, "counter") != 0 || after.Players[0].HP != before.Players[0].HP) {
				t.Fatal("dead Guardian countered")
			}
			if tc.name == "melee_counter_after_block" && (damage(r, "counter") <= 0 || after.Players[1].Reactions != before.Players[1].Reactions-1) {
				t.Fatal("counter missing")
			}
			if tc.count >= 3 && after.Players[0].Overload != 4 {
				t.Fatal("unbounded overload")
			}
			record(tc.name, before, after, []HexAttackResult{r})
		})
	}
	t.Run("one_reflex_per_turn", func(t *testing.T) {
		id, actors := fixture("one-reflex", 2, 0, 0, proc, 0)
		before := v34TransitionSnapshotOf(t, db, id, actors)
		first := attack(id, 781, "reflex-first")
		second := attack(id, 781, "reflex-second")
		after := v34TransitionSnapshotOf(t, db, id, actors)
		if damage(first, "mysticOverload") != 5 || damage(second, "mysticOverload") != 3 || !after.Players[0].ReflexUsed || after.RNG != 3 {
			t.Fatalf("repeated Reflex %+v", after)
		}
		record("one_reflex_per_turn", before, after, []HexAttackResult{first, second})
	})
	t.Run("mystic_dies_to_counter", func(t *testing.T) {
		id, actors := fixture("dead-mystic", 1, 1, 0, noProc, 0)
		if _, err = db.Exec(`UPDATE match_players SET health=1,combat_magic_shield=0 WHERE instance_id=$1 AND user_id=782`, id); err != nil {
			t.Fatal(err)
		}
		before := v34TransitionSnapshotOf(t, db, id, actors)
		r := attack(id, 781, "lethal-counter")
		after := v34TransitionSnapshotOf(t, db, id, actors)
		if damage(r, "mysticOverload") != 3 || after.Players[0].HP != 0 || after.Active == 782 || damage(r, "counter") <= 0 {
			t.Fatalf("death order %+v %+v", r, after)
		}
		record("mystic_dies_to_counter", before, after, []HexAttackResult{r})
	})
	t.Run("dodge_is_not_a_magic_hit", func(t *testing.T) {
		id := "magic-dodge"
		actors := []v34TransitionActor{{782, 1, 0, 0, 0, 0}, {783, 2, 2, 0, 0, 0}, {784, 3, -2, 0, 0, 0}}
		v34TransitionFixture(t, db, id, "hex-test", actors)
		if _, err = db.Exec(`UPDATE match_players SET hex_overload_target='player:783',hex_overload_count=1,combat_magic_shield=0 WHERE instance_id=$1 AND user_id=782`, id); err != nil {
			t.Fatal(err)
		}
		seed := int64(1)
		for int(balance.V34RandomUnit(seed, 0)*100) >= 9 {
			seed++
		}
		if _, err = db.Exec(`UPDATE matches SET combat_rng_seed=$2,combat_rng_counter=0 WHERE instance_id=$1`, id, seed); err != nil {
			t.Fatal(err)
		}
		before := v34TransitionSnapshotOf(t, db, id, actors)
		r := attack(id, 783, "dodged-magic")
		after := v34TransitionSnapshotOf(t, db, id, actors)
		if !has(r, "dodge") || has(r, "block") || damage(r, "hit") != 0 || damage(r, "mysticOverload") != 0 || after.Players[0].Overload != 1 || after.Players[0].ReflexUsed || after.Players[1].Drain || after.RNG != 1 {
			t.Fatalf("dodge advanced magic %+v %+v", r, after)
		}
		record("dodge_prevents_magic", before, after, []HexAttackResult{r})
	})
	t.Run("target_change_resets_series", func(t *testing.T) {
		id, actors := fixture("target-change", 2, 2, 0, noProc, 0)
		before := v34TransitionSnapshotOf(t, db, id, actors)
		r := attack(id, 784, "new-magic-target")
		after := v34TransitionSnapshotOf(t, db, id, actors)
		if after.Players[0].Overload != 1 || damage(r, "mysticOverload") != 0 {
			t.Fatal("series leaked across targets")
		}
		record("target_change_resets_series", before, after, []HexAttackResult{r})
	})
	t.Run("frozen_pre_change_v34", func(t *testing.T) {
		id, actors := fixture("frozen-old", 2, 2, 0, noProc, 0)
		cfg := balance.ExperimentalV34Config()
		cfg.EquipmentModel = "slots_v2"
		cfg.GearMain = 2
		cfg.MysticMagicThroughBlock = false
		cfg.RareSlotReflex = nil
		cfg.JewelryReflex = nil
		for slot, b := range cfg.SlotBudgets {
			b.Reflex = 0
			if slot == "helmet" || slot == "boots" {
				b.Reflex = 1
			}
			cfg.SlotBudgets[slot] = b
		}
		raw, err := json.Marshal(cfg)
		if err != nil {
			t.Fatal(err)
		}
		if _, err = db.Exec(`UPDATE matches SET combat_config=$2::jsonb WHERE instance_id=$1`, id, string(raw)); err != nil {
			t.Fatal(err)
		}
		before := v34TransitionSnapshotOf(t, db, id, actors)
		r := attack(id, 781, "old-block")
		after := v34TransitionSnapshotOf(t, db, id, actors)
		if !has(r, "block") || damage(r, "mysticOverload") != 0 || after.Players[0].Overload != 0 || after.RNG != 1 || after.Players[1].HP != before.Players[1].HP {
			t.Fatal("saved v34 match adopted penetration")
		}
		record("frozen_pre_change_v34", before, after, []HexAttackResult{r})
	})
	t.Run("persistence_replay_rollback_seed", func(t *testing.T) {
		id, actors := fixture("persist", 2, 1, 0, noProc, 0)
		before := v34TransitionSnapshotOf(t, db, id, actors)
		c, err := repository.BeginMatchCommand(ctx, id, 782, "rolled-magic", "rolled-magic")
		if err != nil {
			t.Fatal(err)
		}
		rolled, err := applyHexPlayerAttackTx(ctx, c, 782, 781)
		if err != nil {
			c.Rollback()
			t.Fatal(err)
		}
		c.Rollback()
		if !reflect.DeepEqual(before, v34TransitionSnapshotOf(t, db, id, actors)) {
			t.Fatal("rollback changed series/RNG/HP")
		}
		r := attack(id, 781, "persisted-magic")
		after := v34TransitionSnapshotOf(t, db, id, actors)
		replay := attack(id, 781, "persisted-magic")
		if !replay.Replayed || !reflect.DeepEqual(after, v34TransitionSnapshotOf(t, db, id, actors)) || damage(rolled, "mysticOverload") != damage(r, "mysticOverload") {
			t.Fatal("replay/rollback changed state")
		}
		twin, twinActors := fixture("same-seed", 2, 1, 0, noProc, 0)
		again := attack(twin, 781, "same-seed-magic")
		twinState := v34TransitionSnapshotOf(t, db, twin, twinActors)
		if !reflect.DeepEqual(after.Players, twinState.Players) || after.RNG != twinState.RNG || !reflect.DeepEqual(r.Steps, again.Steps) || !reflect.DeepEqual(r.Effects, again.Effects) {
			t.Fatal("same seed diverged")
		}
		// Reload the persisted series through a fresh pool after a process-flag rollback.
		t.Setenv("NACHALO_COMBAT_RULESET", "legacy")
		fresh, err := sql.Open("postgres", fixtureDSN)
		if err != nil {
			t.Fatal(err)
		}
		defer fresh.Close()
		var count int
		var used bool
		var counter int64
		if err = fresh.QueryRow(`SELECT p.hex_overload_count,p.hex_mystic_reflex_used,m.combat_rng_counter FROM match_players p JOIN matches m USING(instance_id) WHERE instance_id=$1 AND user_id=782`, id).Scan(&count, &used, &counter); err != nil {
			t.Fatal(err)
		}
		if count != 2 || used || counter != after.RNG {
			t.Fatal("persisted series/RNG lost")
		}
		record("persistence_replay_rollback_seed", before, after, []HexAttackResult{r, replay})
	})
}
