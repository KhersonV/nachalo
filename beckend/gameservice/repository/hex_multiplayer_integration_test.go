package repository

import (
	"context"
	"encoding/json"
	"errors"
	"reflect"
	"strings"
	"sync"
	"testing"

	"gameservice/internal/hexmap"
)

func TestHexMultiplayerPostgres(t *testing.T) {
	db, _ := hexTestDatabase(t)
	ctx := context.Background()
	for _, id := range []int{702, 703} {
		if _, err := CreatePlayerProfile(CreatePlayerProfileInput{UserID: id, Name: "HEX player", CharacterType: "guardian"}); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := db.Exec(`UPDATE player_profiles SET inventory='{"private_marker":{"item_name":"private_marker","item_count":1}}'::jsonb WHERE user_id=701`); err != nil {
		t.Fatal(err)
	}
	plan := HexMatchPlan{InstanceID: "hex-multi-two", Mode: "1x1", Teams: []hexmap.Team{{ID: 1, PlayerIDs: []int{701}}, {ID: 2, PlayerIDs: []int{702}}}, TurnOrder: []int{701, 702}, Seed: 97}
	first, err := CreateHexMatch(ctx, plan)
	if err != nil {
		t.Fatal(err)
	}
	second, err := LoadHexTestMatch(ctx, plan.InstanceID, 702)
	if err != nil {
		t.Fatal(err)
	}
	if first.Mode != "1x1" || second.InstanceID != first.InstanceID || first.Player.UserID != 701 || second.Player.UserID != 702 || first.ActiveUser != 701 || len(first.Starts) != 2 || len(first.TurnOrder) != 2 || !reflect.DeepEqual(first.Cells, second.Cells) || !reflect.DeepEqual(first.Edges, second.Edges) || !reflect.DeepEqual(first.Portal, second.Portal) {
		t.Fatal("players did not load one common HEX match")
	}
	var encoded []byte
	encoded, err = json.Marshal(second)
	if err != nil {
		t.Fatal(err)
	}
	if len(second.Actors) < 3 || second.Actors[0].TeamID == second.Actors[1].TeamID || second.Player.Inventory["private_marker"] != nil || strings.Contains(string(encoded), "private_marker") {
		t.Fatal("player teams or private inventory leaked")
	}
	if _, err := LoadHexTestMatch(ctx, plan.InstanceID, 999); !errors.Is(err, ErrHexUnavailable) {
		t.Fatalf("outsider loaded match: %v", err)
	}
	firstDest := walkableNeighbor(first)
	move, err := ApplyHexMove(ctx, plan.InstanceID, 701, "first-move", firstDest)
	if err != nil || move.StateRevision != 2 {
		t.Fatalf("first move: %+v %v", move, err)
	}
	viewTwo, err := LoadHexTestMatch(ctx, plan.InstanceID, 702)
	if err != nil {
		t.Fatal(err)
	}
	foundMoved := false
	for _, actor := range viewTwo.Actors {
		if actor.UserID == 701 && actor.Position == firstDest {
			foundMoved = true
		}
	}
	if !foundMoved {
		t.Fatal("second player did not see first move")
	}
	regen := func(string) int { return 1 }
	turn, err := ApplyHexEndTurn(ctx, plan.InstanceID, 701, "first-turn", 1, regen, nil)
	if err != nil || turn.ActiveUser != 702 || turn.TurnNumber != 1 || turn.StateRevision != 3 {
		t.Fatalf("first turn: %+v %v", turn, err)
	}
	replay, err := ApplyHexEndTurn(ctx, plan.InstanceID, 701, "first-turn", 1, regen, nil)
	if err != nil || !replay.Replayed || replay.StateRevision != turn.StateRevision {
		t.Fatalf("turn replay: %+v %v", replay, err)
	}
	if _, err := ApplyHexEndTurn(ctx, plan.InstanceID, 701, "wrong-turn", 1, regen, nil); !errors.Is(err, ErrCommandRejected) {
		t.Fatalf("inactive player passed: %v", err)
	}
	secondDest := walkableNeighbor(viewTwo)
	if _, err := ApplyHexMove(ctx, plan.InstanceID, 702, "second-move", secondDest); err != nil {
		t.Fatal(err)
	}
	turn, err = ApplyHexEndTurn(ctx, plan.InstanceID, 702, "second-turn", 1, regen, nil)
	if err != nil || turn.ActiveUser != 701 || turn.TurnNumber != 2 || turn.StateRevision != 5 {
		t.Fatalf("second turn: %+v %v", turn, err)
	}
	reloaded, err := LoadHexTestMatch(ctx, plan.InstanceID, 701)
	if err != nil || reloaded.ActiveUser != 701 || reloaded.TurnNumber != 2 || reloaded.StateRevision != 5 || reloaded.MapRevision != 3 {
		t.Fatalf("reload: %+v %v", reloaded, err)
	}
	three, err := CreateHexMatch(ctx, HexMatchPlan{InstanceID: "hex-multi-three", Mode: "1x2", Teams: []hexmap.Team{{ID: 1, PlayerIDs: []int{701}}, {ID: 2, PlayerIDs: []int{702}}, {ID: 3, PlayerIDs: []int{703}}}, TurnOrder: []int{701, 702, 703}, Seed: 98})
	if err != nil || len(three.Starts) != 3 || len(three.TurnOrder) != 3 {
		t.Fatalf("three teams: %+v %v", three.Starts, err)
	}
}

func TestHexMultiplayerTurnRaceAndRollbackPostgres(t *testing.T) {
	db, _ := hexTestDatabase(t)
	ctx := context.Background()
	if _, err := CreatePlayerProfile(CreatePlayerProfileInput{UserID: 702, Name: "HEX second", CharacterType: "guardian"}); err != nil {
		t.Fatal(err)
	}
	id := "hex-multi-race"
	_, err := CreateHexMatch(ctx, HexMatchPlan{InstanceID: id, Mode: "1x1", Teams: []hexmap.Team{{ID: 1, PlayerIDs: []int{701}}, {ID: 2, PlayerIDs: []int{702}}}, TurnOrder: []int{701, 702}, Seed: 103})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`CREATE FUNCTION fail_multi_turn() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN IF NEW.command_id='rollback-turn' THEN RAISE EXCEPTION 'injected rollback'; END IF; RETURN NEW; END $$`); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`CREATE TRIGGER fail_multi_turn BEFORE INSERT ON match_commands FOR EACH ROW EXECUTE FUNCTION fail_multi_turn()`); err != nil {
		t.Fatal(err)
	}
	published := 0
	regen := func(string) int { return 1 }
	if _, err := ApplyHexEndTurn(ctx, id, 701, "rollback-turn", 1, regen, func(HexTurnResult) { published++ }); err == nil {
		t.Fatal("expected rollback")
	}
	var active, turn, revision int
	if err := db.QueryRow(`SELECT active_user_id,turn_number,state_revision FROM matches WHERE instance_id=$1`, id).Scan(&active, &turn, &revision); err != nil || active != 701 || turn != 1 || revision != 1 || published != 0 {
		t.Fatalf("rollback active=%d turn=%d revision=%d published=%d err=%v", active, turn, revision, published, err)
	}
	if _, err := db.Exec(`DROP TRIGGER fail_multi_turn ON match_commands`); err != nil {
		t.Fatal(err)
	}
	results := make(chan error, 2)
	var wg sync.WaitGroup
	for _, command := range []string{"race-a", "race-b"} {
		wg.Add(1)
		go func(command string) {
			defer wg.Done()
			_, err := ApplyHexEndTurn(ctx, id, 701, command, 1, regen, nil)
			results <- err
		}(command)
	}
	wg.Wait()
	close(results)
	succeeded, rejected := 0, 0
	for err := range results {
		if err == nil {
			succeeded++
		} else if errors.Is(err, ErrCommandRejected) {
			rejected++
		} else {
			t.Fatal(err)
		}
	}
	if succeeded != 1 || rejected != 1 {
		t.Fatalf("race results %d/%d", succeeded, rejected)
	}
	if err := db.QueryRow(`SELECT active_user_id,turn_number,state_revision FROM matches WHERE instance_id=$1`, id).Scan(&active, &turn, &revision); err != nil || active != 702 || turn != 1 || revision != 2 {
		t.Fatalf("race state %d/%d/%d %v", active, turn, revision, err)
	}
}

func TestHexMultiplayerCreateRollbackPostgres(t *testing.T) {
	db, _ := hexTestDatabase(t)
	if _, err := CreatePlayerProfile(CreatePlayerProfileInput{UserID: 702, Name: "HEX second", CharacterType: "guardian"}); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`CREATE FUNCTION fail_second_hex_player() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN IF NEW.user_id=702 THEN RAISE EXCEPTION 'injected player insert failure'; END IF; RETURN NEW; END $$`); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`CREATE TRIGGER fail_second_hex_player BEFORE INSERT ON match_players FOR EACH ROW EXECUTE FUNCTION fail_second_hex_player()`); err != nil {
		t.Fatal(err)
	}
	id := "hex-multi-create-rollback"
	_, err := CreateHexMatch(context.Background(), HexMatchPlan{InstanceID: id, Mode: "1x1", Teams: []hexmap.Team{{ID: 1, PlayerIDs: []int{701}}, {ID: 2, PlayerIDs: []int{702}}}, TurnOrder: []int{701, 702}, Seed: 111})
	if err == nil {
		t.Fatal("expected atomic creation rollback")
	}
	var count int
	if err := db.QueryRow(`SELECT count(*) FROM matches WHERE instance_id=$1`, id).Scan(&count); err != nil || count != 0 {
		t.Fatalf("match survived failed creation: %d %v", count, err)
	}
	if err := db.QueryRow(`SELECT count(*) FROM match_map_cells WHERE instance_id=$1`, id).Scan(&count); err != nil || count != 0 {
		t.Fatalf("cells survived failed creation: %d %v", count, err)
	}
}

func TestHexMatchModeSizesPostgres(t *testing.T) {
	db, _ := hexTestDatabase(t)
	for id := 702; id <= 710; id++ {
		if _, err := CreatePlayerProfile(CreatePlayerProfileInput{UserID: id, Name: "HEX team member", CharacterType: "guardian"}); err != nil {
			t.Fatal(err)
		}
	}
	cases := []struct {
		mode  string
		teams []hexmap.Team
		order []int
	}{
		{"PVE", []hexmap.Team{{ID: 1, PlayerIDs: []int{701}}}, []int{701}},
		{"2x2", []hexmap.Team{{ID: 1, PlayerIDs: []int{701, 703}}, {ID: 2, PlayerIDs: []int{702, 704}}}, []int{701, 702, 703, 704}},
		{"3x3", []hexmap.Team{{ID: 1, PlayerIDs: []int{701, 703, 705}}, {ID: 2, PlayerIDs: []int{702, 704, 706}}}, []int{701, 702, 703, 704, 705, 706}},
		{"5x5", []hexmap.Team{{ID: 1, PlayerIDs: []int{701, 703, 705, 707, 709}}, {ID: 2, PlayerIDs: []int{702, 704, 706, 708, 710}}}, []int{701, 702, 703, 704, 705, 706, 707, 708, 709, 710}},
	}
	for _, tc := range cases {
		t.Run(tc.mode, func(t *testing.T) {
			id := "hex-mode-" + tc.mode
			snapshot, err := CreateHexMatch(context.Background(), HexMatchPlan{InstanceID: id, Mode: tc.mode, Teams: tc.teams, TurnOrder: tc.order, Seed: 97})
			if err != nil || len(snapshot.Starts) != len(tc.order) || snapshot.ActiveUser != tc.order[0] {
				t.Fatalf("mode %s: starts=%d active=%d err=%v", tc.mode, len(snapshot.Starts), snapshot.ActiveUser, err)
			}
			var players int
			if err := db.QueryRow(`SELECT count(*) FROM match_players WHERE instance_id=$1`, id).Scan(&players); err != nil || players != len(tc.order) {
				t.Fatalf("mode %s players=%d err=%v", tc.mode, players, err)
			}
		})
	}
}
