package handlers

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"reflect"
	"testing"

	"gameservice/game"
	"gameservice/internal/hexgrid"
	"gameservice/internal/hexmap"
	"gameservice/repository"
)

type v34TransitionActor struct{ user, team, q, r, hp, attack int }

func TestExperimentalV34LegacyCounterPostgres(t *testing.T) {
	path := os.Getenv("NACHALO_V34_LEGACY")
	if path == "" {
		t.Skip("opt in with NACHALO_V34_LEGACY")
	}
	if os.Getenv("STAGE2_TEST_DSN") != "postgres://v34_validation@127.0.0.1:55434/nachalo_v34_validation?sslmode=disable" {
		t.Fatal("isolated cluster required")
	}
	t.Setenv("NACHALO_COMBAT_RULESET", "legacy")
	db, _ := hexCombatDatabase(t)
	ctx := context.Background()
	for i, class := range []string{"berserker", "ranger"} {
		if _, err := repository.CreatePlayerProfile(repository.CreatePlayerProfileInput{UserID: 782 + i, Name: fmt.Sprintf("legacy %d", 782+i), CharacterType: class}); err != nil {
			t.Fatal(err)
		}
	}
	actors := []v34TransitionActor{{782, 1, 0, 0, 10, 0}, {783, 3, -1, 0, 0, 0}, {781, 2, 1, 0, 0, 200}}
	id := "legacy-counter-replay"
	v34TransitionFixture(t, db, id, "hex-test", actors)
	before := v34TransitionSnapshotOf(t, db, id, actors)
	if before.Players[0].MaxHP != 100 || before.Players[2].MaxHP != 130 {
		t.Fatal("legacy bases changed")
	}
	published := 0
	publish := func(HexAttackResult) { published++ }
	hit, err := ApplyHexPlayerAttack(ctx, id, 782, 781, "legacy-lethal", publish)
	if err != nil {
		t.Fatal(err)
	}
	after := v34TransitionSnapshotOf(t, db, id, actors)
	if after.Players[0].HP != 0 || after.Players[0].AP != 4 || after.Active != 783 || after.Players[1].PersonalTurn != 2 || after.RNG != before.RNG {
		t.Fatal("legacy counter transition changed", after)
	}
	if _, err = ApplyHexPlayerAttack(ctx, id, 782, 781, "legacy-lethal", publish); err != nil {
		t.Fatal(err)
	}
	if published != 1 || !reflect.DeepEqual(after, v34TransitionSnapshotOf(t, db, id, actors)) {
		t.Fatal("legacy replay republished/reapplied")
	}
	rules, cfg, err := repository.ReadMatchCombatRulesTx(ctx, db, id)
	if err != nil || rules != "legacy" || cfg != nil {
		t.Fatal("legacy ruleset changed")
	}
	raw, err := json.Marshal(map[string]any{"scenario": id, "before": before, "after": after, "result": hit, "published": published})
	if err != nil {
		t.Fatal(err)
	}
	if err = os.WriteFile(path, append(raw, '\n'), 0600); err != nil {
		t.Fatal(err)
	}
}

type v34TransitionSnapshot struct {
	Players                        []v34ValidationState
	Active, Turn, Results, History int
	RNG, Revision                  int64
	Finished                       bool
	Events                         json.RawMessage
	Wallet                         map[int][2]int // balance, character XP; only fake test participants.
}

func v34TransitionSnapshotOf(t *testing.T, db *sql.DB, id string, actors []v34TransitionActor) v34TransitionSnapshot {
	t.Helper()
	s := v34TransitionSnapshot{Wallet: map[int][2]int{}}
	for _, a := range actors {
		s.Players = append(s.Players, v34State(t, db, id, [2]int{a.user, a.user})[0])
		var b, x int
		if err := db.QueryRow(`SELECT p.balance,c.exp FROM player_profiles p JOIN match_players m ON m.user_id=p.user_id AND m.instance_id=$1 JOIN player_characters c ON c.id=m.character_id WHERE p.user_id=$2`, id, a.user).Scan(&b, &x); err != nil {
			t.Fatal(err)
		}
		s.Wallet[a.user] = [2]int{b, x}
	}
	if err := db.QueryRow(`SELECT COALESCE(active_user_id,0),turn_number,combat_rng_counter,state_revision,finished_at IS NOT NULL,combat_result_events,(SELECT count(*) FROM hex_player_results WHERE instance_id=$1),(SELECT count(*) FROM match_stats WHERE instance_id=$1) FROM matches WHERE instance_id=$1`, id).Scan(&s.Active, &s.Turn, &s.RNG, &s.Revision, &s.Finished, &s.Events, &s.Results, &s.History); err != nil {
		t.Fatal(err)
	}
	return s
}

func v34TransitionFixture(t *testing.T, db *sql.DB, id, mode string, actors []v34TransitionActor) {
	t.Helper()
	groups := map[int][]int{}
	order := []int{}
	for _, a := range actors {
		groups[a.team] = append(groups[a.team], a.user)
		order = append(order, a.user)
	}
	teams := []hexmap.Team{}
	for group := 1; group <= 8; group++ {
		if len(groups[group]) > 0 {
			teams = append(teams, hexmap.Team{ID: group, PlayerIDs: groups[group]})
		}
	}
	if _, err := repository.CreateHexMatch(context.Background(), repository.HexMatchPlan{InstanceID: id, Mode: mode, Teams: teams, TurnOrder: order, Seed: 97}); err != nil {
		t.Fatal(err)
	}
	v34Arena(t, db, id, "open", [2]int{actors[0].user, actors[1].user})
	if _, err := db.Exec(`UPDATE match_map_cells SET is_player=false WHERE instance_id=$1`, id); err != nil {
		t.Fatal(err)
	}
	for _, a := range actors {
		if _, err := db.Exec(`UPDATE match_players SET position_q=$3,position_r=$4,health=CASE WHEN $5>0 THEN $5 ELSE max_health END,attack=CASE WHEN $6>0 THEN $6 ELSE attack END,ap=max_ap,mp=max_mp,reactions=max_reactions,combat_started=true,personal_turn_counter=1 WHERE instance_id=$1 AND user_id=$2`, id, a.user, a.q, a.r, a.hp, a.attack); err != nil {
			t.Fatal(err)
		}
		if _, err := db.Exec(`UPDATE match_map_cells SET is_player=true WHERE instance_id=$1 AND q=$2 AND r=$3`, id, a.q, a.r); err != nil {
			t.Fatal(err)
		}
	}
}

func TestExperimentalV34TransitionsPostgres(t *testing.T) {
	path := os.Getenv("NACHALO_V34_TRANSITIONS")
	if path == "" {
		t.Skip("opt in with NACHALO_V34_TRANSITIONS")
	}
	if os.Getenv("STAGE2_TEST_DSN") != "postgres://v34_validation@127.0.0.1:55434/nachalo_v34_validation?sslmode=disable" {
		t.Fatal("isolated cluster required")
	}
	t.Setenv("NACHALO_COMBAT_RULESET", "experimental_v34")
	t.Setenv("NACHALO_COMBAT_SEED", "34")
	db, _ := hexCombatDatabase(t)
	ctx := context.Background()
	var mobility json.RawMessage
	if err := db.QueryRow(`SELECT jsonb_build_object('items',COALESCE((SELECT jsonb_agg(jsonb_build_object('code',code,'class',class_restriction,'rarity',rarity,'mobility',mobility_bonus) ORDER BY code) FROM item_templates WHERE mobility_bonus<>0),'[]'::jsonb),'sets',COALESCE((SELECT jsonb_agg(jsonb_build_object('code',s.code,'pieces',b.pieces_required,'mobility',b.mobility_bonus) ORDER BY s.code) FROM item_set_bonuses b JOIN item_sets s ON s.id=b.set_id WHERE b.mobility_bonus<>0),'[]'::jsonb))`).Scan(&mobility); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path+".mobility.json", mobility, 0600); err != nil {
		t.Fatal(err)
	}
	for i, class := range []string{"berserker", "ranger", "mystic", "guardian", "berserker", "guardian"} {
		if _, err := repository.CreatePlayerProfile(repository.CreatePlayerProfileInput{UserID: 782 + i, Name: fmt.Sprintf("transition %d", 782+i), CharacterType: class}); err != nil {
			t.Fatal(err)
		}
	}
	file, err := os.OpenFile(path, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	enc := json.NewEncoder(file)
	write := func(name string, before, after v34TransitionSnapshot, result any) {
		t.Helper()
		if err := enc.Encode(map[string]any{"scenario": name, "before": before, "after": after, "result": result}); err != nil {
			t.Fatal(err)
		}
		t.Log(name, "passed")
	}
	// Four classes die with AP remaining. In a three-player match the successor
	// is the next ordered living participant, not necessarily the counter killer.
	for _, victim := range []int{781, 782, 783, 784} {
		actors := []v34TransitionActor{{victim, 1, 0, 0, 10, 0}, {786, 3, -1, 0, 0, 0}, {785, 2, 1, 0, 0, 200}}
		id := fmt.Sprintf("counter-death-%d", victim)
		v34TransitionFixture(t, db, id, "hex-test", actors)
		if victim == 783 {
			if _, err = db.Exec(`UPDATE match_players SET hex_armor_break_owner=$2,hex_armor_break_stacks=2 WHERE instance_id=$1 AND user_id=785`, id, victim); err != nil {
				t.Fatal(err)
			}
		}
		if victim == 784 {
			if _, err = db.Exec(`UPDATE match_players SET hex_overload_target='player:785',hex_overload_count=2 WHERE instance_id=$1 AND user_id=784`, id); err != nil {
				t.Fatal(err)
			}
		}
		before := v34TransitionSnapshotOf(t, db, id, actors)
		if victim == 781 {
			if _, err = db.Exec(`CREATE FUNCTION fail_transition_commit() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN IF NEW.command_id='rollback-counter' THEN RAISE EXCEPTION 'controlled rollback'; END IF; RETURN NEW; END $$; CREATE TRIGGER fail_transition_commit BEFORE INSERT ON match_commands FOR EACH ROW EXECUTE FUNCTION fail_transition_commit()`); err != nil {
				t.Fatal(err)
			}
			if _, e := ApplyHexPlayerAttack(ctx, id, victim, 785, "rollback-counter", nil); e == nil {
				t.Fatal("expected controlled transaction failure")
			}
			if !reflect.DeepEqual(before, v34TransitionSnapshotOf(t, db, id, actors)) {
				t.Fatal("rollback retained death/turn/RNG/reward")
			}
			write("rollback-counter", before, v34TransitionSnapshotOf(t, db, id, actors), nil)
		}
		hit, e := ApplyHexPlayerAttack(ctx, id, victim, 785, "lethal-counter", nil)
		if e != nil {
			t.Fatal(e)
		}
		after := v34TransitionSnapshotOf(t, db, id, actors)
		var events game.ResultEvents
		if e = json.Unmarshal(after.Events, &events); e != nil {
			t.Fatal(e)
		}
		if len(events.KillEvents) != 1 || events.KillEvents[0].VictimID != victim {
			t.Fatal("kill event missing/duplicated")
		}
		if after.Players[0].HP != 0 || after.Players[0].AP != 4 || after.Active != 786 || after.Players[1].PersonalTurn != 2 || after.Players[2].PersonalTurn != 1 || after.Players[2].Reactions != 1 {
			t.Fatalf("counter death state: %+v", after)
		}
		if after.Players[1].HP != before.Players[1].HP {
			t.Fatal("dead Guardian aura damaged successor")
		}
		seenCounter := false
		for _, step := range hit.Steps {
			if step.Kind == "counter" {
				seenCounter = true
			}
			if seenCounter && step.Source != nil && step.Source.ID == victim && step.Damage > 0 {
				t.Fatal("dead actor dealt later damage")
			}
		}
		if victim == 783 && after.Players[2].BreakStacks != 0 {
			t.Fatal("dead Ranger retained series")
		}
		if victim == 784 && (after.Players[0].Overload != 3 || !after.Players[2].Drain) {
			t.Fatal("Mystic sequence/drain not applied once before counter")
		}
		if _, e = ApplyHexPlayerAttack(ctx, id, victim, 785, "lethal-counter", nil); e != nil {
			t.Fatal("replay", e)
		}
		if !reflect.DeepEqual(after, v34TransitionSnapshotOf(t, db, id, actors)) {
			t.Fatal("replay repeated turn/effects/rewards")
		}
		write(id, before, after, hit)
	}
	// A lethal periodic aura advances the turn chain only once per survivor.
	actors := []v34TransitionActor{{783, 1, -2, 0, 0, 0}, {782, 2, 0, 0, 1, 0}, {781, 3, 1, 0, 0, 0}}
	id := "lethal-aura-turn-chain"
	v34TransitionFixture(t, db, id, "hex-test", actors)
	before := v34TransitionSnapshotOf(t, db, id, actors)
	turn, e := repository.ApplyHexEndTurn(ctx, id, 783, "aura-turn", 1, nil)
	if e != nil {
		t.Fatal(e)
	}
	after := v34TransitionSnapshotOf(t, db, id, actors)
	if after.Active != 781 || after.Players[1].HP != 0 || after.Players[1].PersonalTurn != 2 || after.Players[2].PersonalTurn != 2 {
		t.Fatal("lethal aura turn chain", after)
	}
	hits := 0
	for _, effect := range turn.ActionEffects {
		if effect["kind"] == "guardianAuraStart" {
			hits++
			if effect["amount"] != float64(1) {
				t.Fatal("aura visual should cap to actual HP", effect)
			}
		}
	}
	if hits != 1 {
		t.Fatal("duplicate aura tick", hits)
	}
	if _, e = repository.ApplyHexEndTurn(ctx, id, 783, "aura-turn", 1, nil); e != nil {
		t.Fatal(e)
	}
	if !reflect.DeepEqual(after, v34TransitionSnapshotOf(t, db, id, actors)) {
		t.Fatal("aura replay advanced again")
	}
	write(id, before, after, turn)
	// Two overlapping enemy Guardians cause one union tick, including when the
	// victim is itself a Guardian. Friendly/own aura never hits its owner.
	for _, subject := range []int{782, 785} {
		other := 785
		if subject == 785 {
			other = 787
		}
		actors = []v34TransitionActor{{783, 1, -3, 1, 0, 0}, {subject, 1, 0, 0, 0, 0}, {781, 2, -1, 0, 0, 0}, {other, 2, 1, 0, 0, 0}}
		id = fmt.Sprintf("aura-subject-%d", subject)
		v34TransitionFixture(t, db, id, "hex-test", actors)
		before = v34TransitionSnapshotOf(t, db, id, actors)
		turn, e = repository.ApplyHexEndTurn(ctx, id, 783, "union-tick", 1, nil)
		if e != nil {
			t.Fatal(e)
		}
		after = v34TransitionSnapshotOf(t, db, id, actors)
		if after.Players[1].HP != before.Players[1].HP-2 || after.Players[1].PersonalTurn != 2 {
			t.Fatal("wrong union tick", after)
		}
		count := 0
		for _, effect := range turn.ActionEffects {
			if effect["kind"] == "guardianAuraStart" {
				count++
			}
		}
		if count != 1 {
			t.Fatal("union tick repeated")
		}
		write(id, before, after, turn)
	}
	actors = []v34TransitionActor{{783, 2, -3, 0, 0, 0}, {781, 1, 0, 0, 0, 0}, {782, 1, 1, 0, 0, 0}}
	id = "friendly-and-self-aura"
	v34TransitionFixture(t, db, id, "hex-test", actors)
	before = v34TransitionSnapshotOf(t, db, id, actors)
	turn, e = repository.ApplyHexEndTurn(ctx, id, 783, "own-aura", 1, nil)
	if e != nil {
		t.Fatal(e)
	}
	after = v34TransitionSnapshotOf(t, db, id, actors)
	if after.Players[1].HP != before.Players[1].HP || len(turn.ActionEffects) != 0 {
		t.Fatal("own aura damage")
	}
	write(id, before, after, turn)
	// Real Ranger third shot forces an aura owner away from a third enemy.
	actors = []v34TransitionActor{{783, 2, 2, 0, 0, 0}, {781, 1, 0, 0, 0, 0}, {782, 2, 1, -1, 0, 0}}
	id = "forced-aura-owner"
	v34TransitionFixture(t, db, id, "hex-test", actors)
	if _, err = db.Exec(`UPDATE match_players SET hex_armor_break_owner=783,hex_armor_break_stacks=2 WHERE instance_id=$1 AND user_id=781`, id); err != nil {
		t.Fatal(err)
	}
	before = v34TransitionSnapshotOf(t, db, id, actors)
	hit, e := ApplyHexPlayerAttack(ctx, id, 783, 781, "push-owner", nil)
	if e != nil {
		t.Fatal(e)
	}
	after = v34TransitionSnapshotOf(t, db, id, actors)
	if after.Players[1].Position != (hexgrid.Coord{Q: -1, R: 0}) || after.Players[2].HP != before.Players[2].HP-2 {
		t.Fatal("forced owner exit", after)
	}
	write(id, before, after, hit)
	// Death on a route cancels the attack part of the real atomic engagement.
	// A voluntarily moving Guardian still exits the OTHER Guardian's aura;
	// only its own aura's departure is exempted for the stationary opponent.
	actors = []v34TransitionActor{{781, 1, 0, 0, 0, 0}, {785, 2, 1, 0, 0, 0}, {783, 3, -3, 0, 0, 0}}
	id = "independent-guardian-exit"
	v34TransitionFixture(t, db, id, "hex-test", actors)
	before = v34TransitionSnapshotOf(t, db, id, actors)
	moved, e := repository.ApplyHexMove(ctx, id, 781, "independent-exit", hexgrid.Coord{Q: -1, R: 0})
	if e != nil {
		t.Fatal(e)
	}
	after = v34TransitionSnapshotOf(t, db, id, actors)
	if after.Players[0].HP != before.Players[0].HP-2 || after.Players[1].HP != before.Players[1].HP || after.Players[0].MP != before.Players[0].MP-3 {
		t.Fatal("independent aura was suppressed", after)
	}
	write(id, before, after, moved)
	// Staying in at least one enemy aura must not discharge either aura twice.
	actors = []v34TransitionActor{{782, 1, 0, 0, 0, 0}, {781, 2, -1, 0, 0, 0}, {785, 2, 1, 0, 0, 0}}
	id = "overlap-union-exit"
	v34TransitionFixture(t, db, id, "hex-test", actors)
	before = v34TransitionSnapshotOf(t, db, id, actors)
	firstMove, e := repository.ApplyHexMove(ctx, id, 782, "inside-union", hexgrid.Coord{Q: -1, R: 1})
	if e != nil {
		t.Fatal(e)
	}
	inside := v34TransitionSnapshotOf(t, db, id, actors)
	if inside.Players[0].HP != before.Players[0].HP {
		t.Fatal("exit while still inside another aura")
	}
	secondMove, e := repository.ApplyHexMove(ctx, id, 782, "outside-union", hexgrid.Coord{Q: -2, R: 2})
	if e != nil {
		t.Fatal(e)
	}
	after = v34TransitionSnapshotOf(t, db, id, actors)
	if after.Players[0].HP != before.Players[0].HP-2 || after.Players[0].MP != before.Players[0].MP-4 {
		t.Fatal("union exit/tax duplicated", after)
	}
	write(id, before, after, map[string]any{"first": firstMove, "second": secondMove})
	// A real third shot pushes an enemy out of an allied Guardian's aura.
	actors = []v34TransitionActor{{783, 1, -1, 1, 0, 0}, {782, 2, 1, 0, 0, 0}, {781, 1, 0, 0, 0, 0}}
	id = "forced-enemy-exit"
	v34TransitionFixture(t, db, id, "hex-test", actors)
	if _, err = db.Exec(`UPDATE match_players SET hex_armor_break_owner=783,hex_armor_break_stacks=2 WHERE instance_id=$1 AND user_id=782`, id); err != nil {
		t.Fatal(err)
	}
	before = v34TransitionSnapshotOf(t, db, id, actors)
	hit, e = ApplyHexPlayerAttack(ctx, id, 783, 782, "push-enemy", nil)
	if e != nil {
		t.Fatal(e)
	}
	after = v34TransitionSnapshotOf(t, db, id, actors)
	mainHP := -1
	for _, step := range hit.Steps {
		if step.Kind == "hit" {
			mainHP = step.TargetHPAfter
		}
	}
	distance, _ := hexgrid.Distance(after.Players[1].Position, after.Players[2].Position)
	if mainHP < 0 || after.Players[1].HP != mainHP-2 || distance <= 1 {
		t.Fatal("forced enemy exit damage", after)
	}
	write(id, before, after, hit)
	// Death on a route cancels the attack part of the real atomic engagement.
	actors = []v34TransitionActor{{782, 1, 1, 0, 1, 0}, {784, 2, 3, 0, 0, 0}, {781, 2, 0, 0, 0, 0}}
	id = "aura-exit-cancels-attack"
	v34TransitionFixture(t, db, id, "hex-test", actors)
	before = v34TransitionSnapshotOf(t, db, id, actors)
	preview, e := PreviewHexEngagement(ctx, id, 782, 784, "player")
	if e != nil {
		t.Fatal(e)
	}
	engage, e := ApplyHexEngagement(ctx, id, 782, 784, "player", "fatal-route", preview.PreviewToken, nil)
	if e != nil {
		t.Fatal(e)
	}
	after = v34TransitionSnapshotOf(t, db, id, actors)
	if engage.Attacked || after.Players[0].HP != 0 || after.Players[1].HP != before.Players[1].HP || after.Active != 784 || after.RNG != before.RNG {
		t.Fatal("dead mover attacked", after)
	}
	write(id, before, after, engage)
	// Finish an actual reward-bearing match through the normal paid portal,
	// then replay both commands and ensure there is one history/payout only.
	if _, err = db.Exec(`UPDATE player_profiles SET balance=1000 WHERE user_id=781`); err != nil {
		t.Fatal(err)
	}
	actors = []v34TransitionActor{{782, 1, 0, 0, 10, 0}, {781, 2, 1, 0, 0, 200}}
	id = "counter-death-and-extraction"
	v34TransitionFixture(t, db, id, "1x1", actors)
	before = v34TransitionSnapshotOf(t, db, id, actors)
	hit, e = ApplyHexPlayerAttack(ctx, id, 782, 781, "reward-counter", nil)
	if e != nil {
		t.Fatal(e)
	}
	if _, err = db.Exec(`UPDATE match_map_cells SET is_portal=true WHERE instance_id=$1 AND q=1 AND r=0`, id); err != nil {
		t.Fatal(err)
	}
	extracted, e := repository.ApplyHexExtraction(ctx, id, 781, "reward-extract", "paid", true, nil)
	if e != nil {
		t.Fatal(e)
	}
	after = v34TransitionSnapshotOf(t, db, id, actors)
	if !after.Finished || after.Active != 0 || after.History != 1 || after.Results != 2 {
		t.Fatal("match/rewards did not finalize once", after)
	}
	for _, a := range actors {
		var gold, spent int
		if err = db.QueryRow(`SELECT (result->>'goldEarned')::int,COALESCE((result->>'goldSpent')::int,0) FROM hex_player_results WHERE instance_id=$1 AND user_id=$2`, id, a.user).Scan(&gold, &spent); err != nil {
			t.Fatal(err)
		}
		if after.Wallet[a.user][0]-before.Wallet[a.user][0] != gold-spent {
			t.Fatal("terminal payout does not equal final reward minus fee")
		}
	}
	if _, e = ApplyHexPlayerAttack(ctx, id, 782, 781, "reward-counter", nil); e != nil {
		t.Fatal(e)
	}
	if _, e = repository.ApplyHexExtraction(ctx, id, 781, "reward-extract", "paid", true, nil); e != nil {
		t.Fatal(e)
	}
	if !reflect.DeepEqual(after, v34TransitionSnapshotOf(t, db, id, actors)) {
		t.Fatal("replay duplicated terminal payout")
	}
	write(id, before, after, map[string]any{"attack": hit, "extraction": extracted})
}
