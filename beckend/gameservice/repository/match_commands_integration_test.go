package repository

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"os"
	"sync"
	"testing"
	"time"

	_ "github.com/lib/pq"
)

func TestSquareMoveTransactionIntegration(t *testing.T) {
	dsn := os.Getenv("STAGE2_TEST_DSN")
	if dsn == "" {
		t.Skip("set STAGE2_TEST_DSN to an isolated PostgreSQL database")
	}
	db, err := sql.Open("postgres", dsn)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	old := DB
	DB = db
	defer func() { DB = old }()
	setup := []string{
		`CREATE TABLE matches(instance_id text primary key, active_user_id int, map_width int, map_height int, state_revision bigint default 0, map_revision bigint default 0)`,
		`CREATE TABLE match_players(instance_id text, user_id int, position jsonb, energy int, health int, inventory jsonb default '{}'::jsonb, primary key(instance_id,user_id))`,
		`CREATE TABLE match_map_cells(instance_id text, cell_id serial, x int, y int, tile_code int, resource jsonb, barbel jsonb, monster jsonb, is_portal bool default false, is_player bool default false, structure_type text default '', structure_owner_user_id int default 0, structure_health int default 0, structure_defense int default 0, structure_attack int default 0, structure_energy int default 0, is_under_construction bool default false, construction_turns_left int default 0, primary key(instance_id,cell_id), unique(instance_id,x,y))`,
		`CREATE TABLE match_monsters(instance_id text, pos_x int, pos_y int, health int)`,
		`CREATE TABLE inventory_items(instance_id text, user_id int, item_type text, item_id int, item_name text, item_description text, item_count int, primary key(instance_id,user_id,item_type,item_id))`,
	}
	for _, q := range setup {
		if _, err := db.Exec(q); err != nil {
			t.Fatal(err)
		}
	}
	migration, err := os.ReadFile("../../../docs/hex-migration/stage2_commands.sql")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(string(migration)); err != nil {
		t.Fatal(err)
	}
	for _, id := range []string{"race", "rollback"} {
		for _, q := range []string{
			fmt.Sprintf(`INSERT INTO matches VALUES('%s',1,3,1,0,0)`, id),
			fmt.Sprintf(`INSERT INTO match_players(instance_id,user_id,position,energy,health) VALUES('%s',1,'{"x":0,"y":0}',5,10)`, id),
			fmt.Sprintf(`INSERT INTO match_map_cells(instance_id,x,y,tile_code,is_player) VALUES('%s',0,0,48,true),('%s',1,0,48,false),('%s',2,0,48,false)`, id, id, id),
		} {
			if _, err := db.Exec(q); err != nil {
				t.Fatal(err)
			}
		}
	}

	var wg sync.WaitGroup
	results := make(chan error, 2)
	for _, id := range []string{"first", "second"} {
		wg.Add(1)
		go func(commandID string) {
			defer wg.Done()
			_, err := ApplySquareMove(context.Background(), "race", 1, commandID, 1, 0, 3)
			results <- err
		}(id)
	}
	wg.Wait()
	close(results)
	success, rejected := 0, 0
	for err := range results {
		if err == nil {
			success++
		} else if errors.Is(err, ErrCommandRejected) {
			rejected++
		} else {
			t.Fatal(err)
		}
	}
	if success != 1 || rejected != 1 {
		t.Fatalf("concurrent moves: success=%d rejected=%d", success, rejected)
	}
	var energy int
	var state, mapRev int64
	if err := db.QueryRow(`SELECT energy FROM match_players WHERE instance_id='race'`).Scan(&energy); err != nil {
		t.Fatal(err)
	}
	if err := db.QueryRow(`SELECT state_revision,map_revision FROM matches WHERE instance_id='race'`).Scan(&state, &mapRev); err != nil {
		t.Fatal(err)
	}
	if energy != 2 || state != 1 || mapRev != 1 {
		t.Fatalf("energy=%d state=%d map=%d", energy, state, mapRev)
	}
	var winningID string
	if err := db.QueryRow(`SELECT command_id FROM match_commands WHERE instance_id='race'`).Scan(&winningID); err != nil {
		t.Fatal(err)
	}
	replay, err := ApplySquareMove(context.Background(), "race", 1, winningID, 1, 0, 3)
	if err != nil || !replay.Replayed || replay.StateRevision != 1 {
		t.Fatalf("replay=%+v err=%v", replay, err)
	}
	_, err = ApplySquareMove(context.Background(), "race", 1, winningID, 2, 0, 3)
	if !errors.Is(err, ErrCommandConflict) {
		t.Fatalf("payload conflict: %v", err)
	}

	if _, err := db.Exec(`CREATE FUNCTION fail_rollback_ledger() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN IF NEW.instance_id='rollback' THEN RAISE EXCEPTION 'injected failure'; END IF; RETURN NEW; END $$`); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`CREATE TRIGGER fail_rollback BEFORE INSERT ON match_commands FOR EACH ROW EXECUTE FUNCTION fail_rollback_ledger()`); err != nil {
		t.Fatal(err)
	}
	if _, err := ApplySquareMove(context.Background(), "rollback", 1, "failed", 1, 0, 3); err == nil {
		t.Fatal("expected injected save error")
	}
	var pos string
	if err := db.QueryRow(`SELECT position::text,energy FROM match_players WHERE instance_id='rollback'`).Scan(&pos, &energy); err != nil {
		t.Fatal(err)
	}
	if err := db.QueryRow(`SELECT state_revision,map_revision FROM matches WHERE instance_id='rollback'`).Scan(&state, &mapRev); err != nil {
		t.Fatal(err)
	}
	if pos != `{"x": 0, "y": 0}` || energy != 5 || state != 0 || mapRev != 0 {
		t.Fatalf("rollback left pos=%s energy=%d state=%d map=%d", pos, energy, state, mapRev)
	}

	if _, err := db.Exec(`INSERT INTO matches VALUES('loot',1,2,1,0,0)`); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`INSERT INTO match_players(instance_id,user_id,position,energy,health) VALUES('loot',1,'{"x":0,"y":0}',5,10)`); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`INSERT INTO match_map_cells(instance_id,x,y,tile_code,resource) VALUES('loot',1,0,82,'{"id":9,"type":"wood","description":"Wood","image":"wood.png"}')`); err != nil {
		t.Fatal(err)
	}
	collected, err := ApplyResourceCollection(context.Background(), "loot", 1, "pickup", 1, 0)
	if err != nil || collected.StateRevision != 1 || collected.MapRevision != 1 {
		t.Fatalf("collect=%+v err=%v", collected, err)
	}
	replayCollection, err := ApplyResourceCollection(context.Background(), "loot", 1, "pickup", 1, 0)
	if err != nil || !replayCollection.Replayed || replayCollection.StateRevision != 1 {
		t.Fatalf("replay collection=%+v err=%v", replayCollection, err)
	}
	_, err = ApplyResourceCollection(context.Background(), "loot", 1, "pickup", 0, 0)
	if !errors.Is(err, ErrCommandConflict) {
		t.Fatalf("collection payload conflict: %v", err)
	}
	_, err = ApplyResourceCollection(context.Background(), "loot", 1, "again", 1, 0)
	if !errors.Is(err, ErrCommandRejected) {
		t.Fatalf("second pickup: %v", err)
	}
	var itemCount int
	if err := db.QueryRow(`SELECT item_count FROM inventory_items WHERE instance_id='loot' AND user_id=1 AND item_id=9`).Scan(&itemCount); err != nil {
		t.Fatal(err)
	}
	if itemCount != 1 {
		t.Fatalf("item count=%d", itemCount)
	}
	if err := db.QueryRow(`SELECT state_revision,map_revision FROM matches WHERE instance_id='loot'`).Scan(&state, &mapRev); err != nil {
		t.Fatal(err)
	}
	if state != 1 || mapRev != 1 {
		t.Fatalf("loot revisions=%d/%d", state, mapRev)
	}
	if _, err := db.Exec(`INSERT INTO matches VALUES('lootfail',1,2,1,0,0)`); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`INSERT INTO match_players(instance_id,user_id,position,energy,health) VALUES('lootfail',1,'{"x":0,"y":0}',5,10)`); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`INSERT INTO match_map_cells(instance_id,x,y,tile_code,resource) VALUES('lootfail',1,0,82,'{"id":10,"type":"stone","description":"Stone"}')`); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`CREATE FUNCTION fail_inventory_write() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN IF NEW.instance_id='lootfail' THEN RAISE EXCEPTION 'injected inventory failure'; END IF; RETURN NEW; END $$`); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`CREATE TRIGGER fail_inventory BEFORE INSERT ON inventory_items FOR EACH ROW EXECUTE FUNCTION fail_inventory_write()`); err != nil {
		t.Fatal(err)
	}
	if _, err := ApplyResourceCollection(context.Background(), "lootfail", 1, "fail", 1, 0); err == nil {
		t.Fatal("expected inventory failure")
	}
	var resourcePresent bool
	if err := db.QueryRow(`SELECT resource IS NOT NULL FROM match_map_cells WHERE instance_id='lootfail' AND x=1`).Scan(&resourcePresent); err != nil {
		t.Fatal(err)
	}
	if err := db.QueryRow(`SELECT state_revision,map_revision FROM matches WHERE instance_id='lootfail'`).Scan(&state, &mapRev); err != nil {
		t.Fatal(err)
	}
	if !resourcePresent || state != 0 || mapRev != 0 {
		t.Fatalf("partial pickup: resource=%t revisions=%d/%d", resourcePresent, state, mapRev)
	}
	if _, err := db.Exec(`INSERT INTO matches VALUES('stateonly',1,1,1,0,0)`); err != nil {
		t.Fatal(err)
	}
	command, err := BeginMatchCommand(context.Background(), "stateonly", 1, "energy", "energy-v1:1")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := command.Tx.Exec(`UPDATE matches SET active_user_id=2 WHERE instance_id='stateonly'`); err != nil {
		command.Rollback()
		t.Fatal(err)
	}
	if _, _, err := command.Commit(context.Background(), map[string]string{"status": "ok"}, false); err != nil {
		command.Rollback()
		t.Fatal(err)
	}
	reopened, err := sql.Open("postgres", dsn)
	if err != nil {
		t.Fatal(err)
	}
	defer reopened.Close()
	if err := reopened.QueryRow(`SELECT state_revision,map_revision FROM matches WHERE instance_id='stateonly'`).Scan(&state, &mapRev); err != nil {
		t.Fatal(err)
	}
	if state != 1 || mapRev != 0 {
		t.Fatalf("state-only revisions after new connection=%d/%d", state, mapRev)
	}
	if _, err := db.Exec(`INSERT INTO matches VALUES('interleave',1,2,1,0,0)`); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`INSERT INTO match_players(instance_id,user_id,position,energy,health) VALUES('interleave',1,'{"x":0,"y":0}',5,10)`); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`INSERT INTO match_map_cells(instance_id,x,y,tile_code,is_player,resource) VALUES('interleave',0,0,48,true,NULL),('interleave',1,0,82,false,'{"id":11,"type":"ore","description":"Ore"}')`); err != nil {
		t.Fatal(err)
	}
	moveErr, collectErr := make(chan error, 1), make(chan error, 1)
	go func() {
		_, err := ApplySquareMove(context.Background(), "interleave", 1, "walk", 1, 0, 3)
		moveErr <- err
	}()
	go func() {
		_, err := ApplyResourceCollection(context.Background(), "interleave", 1, "take", 1, 0)
		collectErr <- err
	}()
	if err := <-moveErr; err != nil {
		t.Fatalf("interleaved move: %v", err)
	}
	if err := <-collectErr; err != nil {
		t.Fatalf("interleaved collect: %v", err)
	}
	if err := db.QueryRow(`SELECT state_revision,map_revision FROM matches WHERE instance_id='interleave'`).Scan(&state, &mapRev); err != nil {
		t.Fatal(err)
	}
	if state != 2 || mapRev != 2 {
		t.Fatalf("interleaved revisions=%d/%d", state, mapRev)
	}
	releaseFirst, err := AcquireMatchSerialization(context.Background(), "interleave")
	if err != nil {
		t.Fatal(err)
	}
	acquired := make(chan error, 1)
	go func() {
		release, err := AcquireMatchSerialization(context.Background(), "interleave")
		if err == nil {
			release()
		}
		acquired <- err
	}()
	select {
	case err := <-acquired:
		releaseFirst()
		t.Fatalf("second match lock entered early: %v", err)
	case <-time.After(75 * time.Millisecond):
	}
	releaseFirst()
	select {
	case err := <-acquired:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("second match lock never acquired")
	}
}
