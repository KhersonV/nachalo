package repository

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"os"
	"strings"
	"testing"
	"time"

	"gameservice/game"
	_ "github.com/lib/pq"
)

// This exercises repository primitives only. No combat handler uses them yet:
// finalization, quest artifacts and MatchState still require a single boundary.
func TestPlayerDeathPrimitivesPostgres(t *testing.T) {
	dsn := os.Getenv("STAGE2_TEST_DSN")
	if dsn == "" {
		t.Skip("set STAGE2_TEST_DSN to isolated PostgreSQL")
	}
	admin, err := sql.Open("postgres", dsn)
	if err != nil {
		t.Fatal(err)
	}
	defer admin.Close()
	schema := fmt.Sprintf("death_primitives_%d", time.Now().UnixNano())
	if _, err = admin.Exec(`CREATE SCHEMA ` + schema); err != nil {
		t.Fatal(err)
	}
	defer admin.Exec(`DROP SCHEMA ` + schema + ` CASCADE`)
	if strings.HasPrefix(dsn, "postgres://") || strings.HasPrefix(dsn, "postgresql://") {
		u, parseErr := url.Parse(dsn)
		if parseErr != nil {
			t.Fatal(parseErr)
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
		`CREATE TABLE matches(instance_id text primary key,active_user_id int,map_width int,map_height int,turn_order jsonb,turn_number int,state_revision bigint default 0,map_revision bigint default 0)`,
		`CREATE TABLE match_players(instance_id text,user_id int,health int,energy int,primary key(instance_id,user_id))`,
		`CREATE TABLE match_map_cells(instance_id text,x int,y int,is_player bool,primary key(instance_id,x,y))`,
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
	for _, id := range []string{"success", "rollback"} {
		if _, err = db.Exec(`INSERT INTO matches(instance_id,active_user_id,map_width,map_height,turn_order,turn_number) VALUES($1,1,2,1,'[1,2]',4)`, id); err != nil {
			t.Fatal(err)
		}
		if _, err = db.Exec(`INSERT INTO match_players VALUES($1,1,8,5),($1,2,10,5)`, id); err != nil {
			t.Fatal(err)
		}
		if _, err = db.Exec(`INSERT INTO match_map_cells VALUES($1,0,0,true)`, id); err != nil {
			t.Fatal(err)
		}
	}
	apply := func(id, commandID string) error {
		ctx := context.Background()
		command, beginErr := BeginMatchCommand(ctx, id, 2, commandID, "death-primitives-v1:1")
		if beginErr != nil {
			return beginErr
		}
		defer command.Rollback()
		if command.Replay != nil {
			return nil
		}
		if err := command.RecordDamageEventTx(ctx, game.DamageEvent{SourceType: game.ResultSourcePlayer, DealerID: 2, TargetType: "player", TargetID: 1, Amount: 8}); err != nil {
			return err
		}
		if _, err := command.RecordKillEventTx(ctx, game.KillEvent{SourceType: game.ResultSourcePlayer, KillerID: 2, VictimType: "player", VictimID: 1, Damage: 8}); err != nil {
			return err
		}
		if err := command.MarkCombatPlayerDeadTx(ctx, 1); err != nil {
			return err
		}
		turn, err := command.RemoveCombatPlayerFromTurnOrderTx(ctx, 1)
		if err != nil {
			return err
		}
		if turn.ActiveUserID != 2 || turn.TurnNumber != 4 || len(turn.TurnOrder) != 1 || turn.TurnOrder[0] != 2 {
			return fmt.Errorf("turn=%+v", turn)
		}
		changed, err := command.ClearCombatPlayerCellTx(ctx, Position{X: 0, Y: 0})
		if err != nil || !changed {
			return fmt.Errorf("cell changed=%t err=%w", changed, err)
		}
		_, _, err = command.Commit(ctx, map[string]string{"status": "prepared"}, changed)
		return err
	}
	if err = apply("success", "death"); err != nil {
		t.Fatal(err)
	}
	if err = apply("success", "death"); err != nil {
		t.Fatalf("replay: %v", err)
	}
	fresh, err := sql.Open("postgres", dsn)
	if err != nil {
		t.Fatal(err)
	}
	defer fresh.Close()
	var hp, active, turn int
	var order string
	var flag bool
	var state, mapRevision int64
	if err = fresh.QueryRow(`SELECT mp.health,m.active_user_id,m.turn_number,m.turn_order::text,c.is_player,m.state_revision,m.map_revision FROM matches m JOIN match_players mp ON mp.instance_id=m.instance_id AND mp.user_id=1 JOIN match_map_cells c ON c.instance_id=m.instance_id AND c.x=0 AND c.y=0 WHERE m.instance_id='success'`).Scan(&hp, &active, &turn, &order, &flag, &state, &mapRevision); err != nil {
		t.Fatal(err)
	}
	if hp != 0 || active != 2 || turn != 4 || order != `[2]` || flag || state != 1 || mapRevision != 1 {
		t.Fatalf("persisted hp=%d active=%d turn=%d order=%s flag=%t revisions=%d/%d", hp, active, turn, order, flag, state, mapRevision)
	}
	var eventRaw []byte
	var complete bool
	if err = fresh.QueryRow(`SELECT combat_result_events,combat_results_complete FROM matches WHERE instance_id='success'`).Scan(&eventRaw, &complete); err != nil {
		t.Fatal(err)
	}
	var events game.ResultEvents
	if err = json.Unmarshal(eventRaw, &events); err != nil {
		t.Fatal(err)
	}
	if len(events.DamageEvents) != 1 || len(events.KillEvents) != 1 || len(events.DefeatedUsers) != 1 || events.DefeatedUsers[0] != 1 || complete {
		t.Fatalf("death history=%+v complete=%t", events, complete)
	}
	if _, err = db.Exec(`CREATE FUNCTION fail_death_ledger() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN IF NEW.instance_id='rollback' THEN RAISE EXCEPTION 'injected failure'; END IF; RETURN NEW; END $$`); err != nil {
		t.Fatal(err)
	}
	if _, err = db.Exec(`CREATE TRIGGER fail_death_ledger BEFORE INSERT ON match_commands FOR EACH ROW EXECUTE FUNCTION fail_death_ledger()`); err != nil {
		t.Fatal(err)
	}
	if err = apply("rollback", "death"); err == nil {
		t.Fatal("expected ledger failure")
	}
	if err = fresh.QueryRow(`SELECT mp.health,m.active_user_id,m.turn_number,m.turn_order::text,c.is_player,m.state_revision,m.map_revision FROM matches m JOIN match_players mp ON mp.instance_id=m.instance_id AND mp.user_id=1 JOIN match_map_cells c ON c.instance_id=m.instance_id AND c.x=0 AND c.y=0 WHERE m.instance_id='rollback'`).Scan(&hp, &active, &turn, &order, &flag, &state, &mapRevision); err != nil {
		t.Fatal(err)
	}
	if hp != 8 || active != 1 || turn != 4 || order != `[1, 2]` || !flag || state != 0 || mapRevision != 0 {
		t.Fatalf("rollback hp=%d active=%d turn=%d order=%s flag=%t revisions=%d/%d", hp, active, turn, order, flag, state, mapRevision)
	}
	if err = fresh.QueryRow(`SELECT combat_result_events FROM matches WHERE instance_id='rollback'`).Scan(&eventRaw); err != nil {
		t.Fatal(err)
	}
	if err = json.Unmarshal(eventRaw, &events); err != nil {
		t.Fatal(err)
	}
	if len(events.DamageEvents) != 0 || len(events.KillEvents) != 0 || len(events.DefeatedUsers) != 0 {
		t.Fatalf("rollback kept defeat history: %+v", events)
	}
	unknown, err := BeginMatchCommand(context.Background(), "rollback", 2, "unknown-source", "unknown-source-v1")
	if err != nil {
		t.Fatal(err)
	}
	if err := unknown.SetCombatPlayerVitalsTx(context.Background(), 1, 3, 2); err != nil {
		t.Fatal(err)
	}
	if err := unknown.RecordDamageEventTx(context.Background(), game.DamageEvent{DealerID: 2, TargetType: "player", TargetID: 1, Amount: 5}); !errors.Is(err, ErrCommandRejected) {
		t.Fatalf("untyped new event accepted: %v", err)
	}
	unknown.Rollback()
	if err := fresh.QueryRow(`SELECT health FROM match_players WHERE instance_id='rollback' AND user_id=1`).Scan(&hp); err != nil || hp != 8 {
		t.Fatalf("untyped event rollback health=%d err=%v", hp, err)
	}
}
