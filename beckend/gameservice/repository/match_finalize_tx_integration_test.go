package repository

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"os"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"

	"gameservice/game"
	_ "github.com/lib/pq"
)

func TestMatchFinalizationPostgres(t *testing.T) {
	dsn := os.Getenv("STAGE2_TEST_DSN")
	if dsn == "" {
		t.Skip("set STAGE2_TEST_DSN to isolated PostgreSQL")
	}
	admin, err := sql.Open("postgres", dsn)
	if err != nil {
		t.Fatal(err)
	}
	defer admin.Close()
	schema := fmt.Sprintf("finalize_tx_%d", time.Now().UnixNano())
	if _, err = admin.Exec(`CREATE SCHEMA ` + schema); err != nil {
		t.Fatal(err)
	}
	defer admin.Exec(`DROP SCHEMA ` + schema + ` CASCADE`)
	if strings.HasPrefix(dsn, "postgres://") || strings.HasPrefix(dsn, "postgresql://") {
		u, e := url.Parse(dsn)
		if e != nil {
			t.Fatal(e)
		}
		q := u.Query()
		q.Set("options", "-csearch_path="+schema)
		u.RawQuery = q.Encode()
		dsn = u.String()
	} else {
		dsn += " options='-csearch_path=" + schema + "'"
	}
	db, err := sql.Open("postgres", dsn)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	oldDB := DB
	DB = db
	defer func() { DB = oldDB }()
	for _, q := range []string{
		`CREATE TABLE matches(instance_id text primary key,mode text,winner_id int,winner_group_id int,active_user_id int,map_width int,map_height int,turn_order jsonb,turn_number int,state_revision bigint default 0,map_revision bigint default 0)`,
		`CREATE TABLE match_players(instance_id text,user_id int,name text,character_id int,character_type text,group_id int,health int,energy int,inventory jsonb,primary key(instance_id,user_id))`,
		`CREATE TABLE match_map_cells(instance_id text,x int,y int,is_player bool,primary key(instance_id,x,y))`,
		`CREATE TABLE player_profiles(user_id int primary key,balance int,inventory jsonb,updated_at timestamp default now())`,
		`CREATE TABLE player_characters(id int primary key,level int,exp int,max_exp int,updated_at timestamp default now())`,
		`CREATE TABLE match_stats(instance_id text primary key,mode text,winner_id int,winner_group_id int,winner_user_ids jsonb,participants jsonb,created_at timestamp)`,
		`CREATE TABLE match_player_stats(instance_id text,user_id int,player_name text,group_id int,character_type text,is_winner bool,survived bool,deaths int,exp_gained int,rewards jsonb,player_kills int,monster_kills int,damage_total int,damage_to_players int,damage_to_monsters int,damage_taken int,placement int,inventory_snapshot jsonb,primary key(instance_id,user_id))`,
	} {
		if _, err = db.Exec(q); err != nil {
			t.Fatal(err)
		}
	}
	migration, err := os.ReadFile("../../../docs/hex-migration/stage2_commands.sql")
	if err != nil {
		t.Fatal(err)
	}
	if _, err = db.Exec(string(migration)); err != nil {
		t.Fatal(err)
	}
	for _, fixture := range []struct {
		id            string
		winner, other int
		complete      bool
	}{
		{"success", 1, 2, true}, {"rollback", 3, 4, true}, {"incomplete", 5, 6, false}, {"lastDeath", 7, 0, true}, {"race", 9, 10, true},
	} {
		id := fixture.id
		order := "[" + fmt.Sprint(fixture.winner) + "]"
		if fixture.other > 0 {
			order = fmt.Sprintf("[%d,%d]", fixture.winner, fixture.other)
		}
		if _, err = db.Exec(`INSERT INTO matches(instance_id,mode,winner_id,winner_group_id,active_user_id,map_width,map_height,turn_order,turn_number,combat_results_complete) VALUES($1,'solo',$2,0,$2,2,1,$3,1,$4)`, id, fixture.winner, order, fixture.complete); err != nil {
			t.Fatal(err)
		}
		users := []int{fixture.winner}
		if fixture.other > 0 {
			users = append(users, fixture.other)
		}
		for index, user := range users {
			health := 10
			if index == 1 {
				health = 0
			}
			if _, err = db.Exec(`INSERT INTO match_players(instance_id,user_id,name,character_id,character_type,group_id,health,energy,inventory) VALUES($1,$2,$3,$4,'guardian',0,$5,5,'{"wood_9":{"name":"Wood","item_count":2},"artifact_1":{"name":"Artifact","item_count":1}}')`, id, user, fmt.Sprintf("Player %d", user), 100+user, health); err != nil {
				t.Fatal(err)
			}
			if _, err = db.Exec(`INSERT INTO player_profiles(user_id,balance,inventory) VALUES($1,5,'{"old":{"name":"old","item_count":1}}')`, user); err != nil {
				t.Fatal(err)
			}
			if _, err = db.Exec(`INSERT INTO player_characters(id,level,exp,max_exp) VALUES($1,1,0,500)`, 100+user); err != nil {
				t.Fatal(err)
			}
		}
		if _, err = db.Exec(`INSERT INTO match_map_cells VALUES($1,0,0,true)`, id); err != nil {
			t.Fatal(err)
		}
	}
	// A complete match must have its result inputs in the same persisted row.
	seed := func(id string, killer, victim int) {
		t.Helper()
		events := game.ResultEvents{DamageEvents: []game.DamageEvent{{DealerID: killer, TargetType: "monster", TargetID: 9, Amount: 15}}, KillEvents: []game.KillEvent{{KillerID: killer, VictimType: "monster", VictimID: 9, Damage: 5}}}
		if victim > 0 {
			events.DamageEvents = append(events.DamageEvents, game.DamageEvent{DealerID: killer, TargetType: "player", TargetID: victim, Amount: 10})
			events.KillEvents = append(events.KillEvents, game.KillEvent{KillerID: killer, VictimType: "player", VictimID: victim, Damage: 10})
			events.DefeatedUsers = []int{victim}
		}
		encoded, _ := json.Marshal(events)
		if _, e := db.Exec(`UPDATE matches SET combat_result_events=$2::jsonb WHERE instance_id=$1`, id, encoded); e != nil {
			t.Fatal(e)
		}
	}
	seed("success", 1, 2)
	seed("rollback", 3, 4)
	seed("incomplete", 5, 6)
	seed("lastDeath", 7, 0)
	seed("race", 9, 10)
	callbackCount := 0
	game.CreateMatchState("success", []int{1, 2})
	defer game.DeleteMatchStateInMemory("success")
	callback := func(result MatchFinalizationResult) {
		callbackCount++
		var finished bool
		var state int64
		if e := db.QueryRow(`SELECT finished_at IS NOT NULL,state_revision FROM matches WHERE instance_id='success'`).Scan(&finished, &state); e != nil || !finished || state != result.StateRevision {
			t.Errorf("event before commit finished=%t state=%d result=%d err=%v", finished, state, result.StateRevision, e)
		}
	}
	ctx := context.Background()
	finalized, err := ApplyMatchFinalization(ctx, "success", 1, "finish", callback)
	if err != nil {
		t.Fatal(err)
	}
	if finalized.StateRevision != 1 || finalized.MapRevision != 0 || callbackCount != 1 || len(finalized.Results.PlayerResults) != 2 {
		t.Fatalf("finalized=%+v callback=%d", finalized, callbackCount)
	}
	if _, ok := game.GetMatchState("success"); ok {
		t.Fatal("finished match remained in memory")
	}
	if finalized.Results.PlayerResults[0].ExpGained != 127 || finalized.Results.PlayerResults[0].MonsterKills != 1 || finalized.Results.PlayerResults[0].PlayerKills != 1 {
		t.Fatalf("winner stats=%+v", finalized.Results.PlayerResults[0])
	}
	second, err := ApplyMatchFinalization(ctx, "success", 1, "finish", callback)
	if err != nil || second.StateRevision != finalized.StateRevision || callbackCount != 1 {
		t.Fatalf("replay=%+v err=%v callback=%d", second, err, callbackCount)
	}
	if _, err = ApplyMatchFinalization(ctx, "success", 1, "other", callback); !errors.Is(err, ErrCommandRejected) {
		t.Fatalf("second command err=%v", err)
	}
	fresh, err := sql.Open("postgres", dsn)
	if err != nil {
		t.Fatal(err)
	}
	defer fresh.Close()
	var balance, exp, statCount, ledgerCount int
	var inventory []byte
	if err = fresh.QueryRow(`SELECT balance,inventory FROM player_profiles WHERE user_id=1`).Scan(&balance, &inventory); err != nil {
		t.Fatal(err)
	}
	if balance != 153 || strings.Contains(string(inventory), "artifact_1") || !strings.Contains(string(inventory), "wood_9") {
		t.Fatalf("winner balance=%d inventory=%s", balance, inventory)
	}
	if err = fresh.QueryRow(`SELECT exp FROM player_characters WHERE id=101`).Scan(&exp); err != nil || exp != 127 {
		t.Fatalf("winner exp=%d err=%v", exp, err)
	}
	if err = fresh.QueryRow(`SELECT count(*) FROM match_player_stats WHERE instance_id='success'`).Scan(&statCount); err != nil || statCount != 2 {
		t.Fatalf("stored player stats=%d err=%v", statCount, err)
	}
	if err = fresh.QueryRow(`SELECT count(*) FROM match_commands WHERE instance_id='success'`).Scan(&ledgerCount); err != nil || ledgerCount != 1 {
		t.Fatalf("retained ledger=%d err=%v", ledgerCount, err)
	}
	if _, err = ApplyMatchFinalization(ctx, "incomplete", 5, "finish", nil); !errors.Is(err, ErrCommandRejected) {
		t.Fatalf("partial history accepted: %v", err)
	}
	if _, err = db.Exec(`CREATE FUNCTION fail_final_reward() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN IF NEW.user_id=3 THEN RAISE EXCEPTION 'injected reward failure'; END IF; RETURN NEW; END $$`); err != nil {
		t.Fatal(err)
	}
	if _, err = db.Exec(`CREATE TRIGGER fail_final_reward BEFORE UPDATE ON player_profiles FOR EACH ROW EXECUTE FUNCTION fail_final_reward()`); err != nil {
		t.Fatal(err)
	}
	rollbackPublished := 0
	if _, err = ApplyMatchFinalization(ctx, "rollback", 3, "finish", func(MatchFinalizationResult) { rollbackPublished++ }); err == nil {
		t.Fatal("expected reward failure")
	}
	var finished bool
	var state, mapRevision int64
	if err = fresh.QueryRow(`SELECT finished_at IS NOT NULL,state_revision,map_revision FROM matches WHERE instance_id='rollback'`).Scan(&finished, &state, &mapRevision); err != nil || finished || state != 0 || mapRevision != 0 || rollbackPublished != 0 {
		t.Fatalf("rollback finished=%t revisions=%d/%d publish=%d err=%v", finished, state, mapRevision, rollbackPublished, err)
	}
	if err = fresh.QueryRow(`SELECT count(*) FROM match_stats WHERE instance_id='rollback'`).Scan(&statCount); err != nil || statCount != 0 {
		t.Fatalf("rollback stats=%d err=%v", statCount, err)
	}
	if err = fresh.QueryRow(`SELECT balance FROM player_profiles WHERE user_id=3`).Scan(&balance); err != nil || balance != 5 {
		t.Fatalf("rollback balance=%d err=%v", balance, err)
	}
	var wg sync.WaitGroup
	var raceResults [2]error
	wg.Add(2)
	for i := 0; i < 2; i++ {
		i := i
		go func() {
			defer wg.Done()
			_, raceResults[i] = ApplyMatchFinalization(ctx, "race", 9, fmt.Sprintf("finish-%d", i), nil)
		}()
	}
	wg.Wait()
	succeeded, rejected := 0, 0
	for _, raceErr := range raceResults {
		if raceErr == nil {
			succeeded++
		} else if errors.Is(raceErr, ErrCommandRejected) {
			rejected++
		} else {
			t.Fatal(raceErr)
		}
	}
	if succeeded != 1 || rejected != 1 {
		t.Fatalf("concurrent finalization succeeded=%d rejected=%d", succeeded, rejected)
	}
	if err = fresh.QueryRow(`SELECT balance FROM player_profiles WHERE user_id=9`).Scan(&balance); err != nil || balance != 153 {
		t.Fatalf("concurrent reward balance=%d err=%v", balance, err)
	}
	// The same command can mark the last player dead and finalize without a
	// nested transaction or a second match lock.
	command, err := BeginMatchCommand(ctx, "lastDeath", 7, "last-death", "last-death-v1:7")
	if err != nil {
		t.Fatal(err)
	}
	defer command.Rollback()
	if err = command.MarkCombatPlayerDeadTx(ctx, 7); err != nil {
		t.Fatal(err)
	}
	turn, err := command.RemoveCombatPlayerFromTurnOrderTx(ctx, 7)
	if err != nil || turn.ActiveUserID != 0 {
		t.Fatalf("last turn=%+v err=%v", turn, err)
	}
	changed, err := command.ClearCombatPlayerCellTx(ctx, Position{X: 0, Y: 0})
	if err != nil || !changed {
		t.Fatalf("last cell=%t err=%v", changed, err)
	}
	if _, err = command.RecordDefeatedUserTx(ctx, 7); err != nil {
		t.Fatal(err)
	}
	lastResults, err := command.FinalizeMatchTx(ctx)
	if err != nil {
		t.Fatal(err)
	}
	lastResponse := &MatchFinalizationResult{Results: *lastResults}
	if _, _, err = command.Commit(ctx, lastResponse, changed); err != nil {
		t.Fatal(err)
	}
	if lastResponse.StateRevision != 1 || lastResponse.MapRevision != 1 {
		t.Fatalf("last death revisions=%+v", lastResponse)
	}
	if err = fresh.QueryRow(`SELECT finished_at IS NOT NULL,state_revision,map_revision FROM matches WHERE instance_id='lastDeath'`).Scan(&finished, &state, &mapRevision); err != nil || !finished || state != 1 || mapRevision != 1 {
		t.Fatalf("last death persisted=%t %d/%d err=%v", finished, state, mapRevision, err)
	}
	// Compare the legacy in-memory result with the result after a fresh
	// MatchState is rebuilt exclusively from PostgreSQL event history.
	beforeState := game.CreateMatchState("incomplete", []int{5, 6})
	beforeState.RecordDamageEvent(5, "monster", 9, 15)
	beforeState.RecordDamageEvent(5, "player", 6, 10)
	beforeState.RecordKillEvent(5, "monster", 9, 5)
	beforeState.RecordKillEvent(5, "player", 6, 10)
	beforeState.RecordPlayerDefeat(6)
	beforeRestart, err := GetMatchResults("incomplete")
	if err != nil {
		t.Fatal(err)
	}
	game.DeleteMatchStateInMemory("incomplete")
	DB = fresh // simulate a backend restart with a new PostgreSQL pool
	if err = RestoreMatchStates(); err != nil {
		t.Fatal(err)
	}
	defer game.DeleteMatchStateInMemory("incomplete")
	defer game.DeleteMatchStateInMemory("rollback")
	if state, ok := game.GetMatchState("incomplete"); !ok || len(state.SnapshotResultEvents().DamageEvents) != 2 {
		t.Fatalf("restored combat events missing: state=%v ok=%t", state, ok)
	}
	afterRestart, err := GetMatchResults("incomplete")
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(beforeRestart, afterRestart) {
		t.Fatalf("restart changed results: before=%+v after=%+v", beforeRestart, afterRestart)
	}
	if _, ok := game.GetMatchState("success"); ok {
		t.Fatal("finished match restored as active")
	}
}
