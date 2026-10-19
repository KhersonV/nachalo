package handlers

import (
	"database/sql"
	"os"
	"testing"

	"gameservice/game"
	"gameservice/repository"

	_ "github.com/lib/pq"
)

func TestStaleTurnTimerPostgres(t *testing.T) {
	dsn := os.Getenv("STAGE2_TEST_DSN")
	if dsn == "" {
		t.Skip("set STAGE2_TEST_DSN to an isolated PostgreSQL database")
	}
	root, err := sql.Open("postgres", dsn)
	if err != nil {
		t.Fatal(err)
	}
	defer root.Close()
	if _, err := root.Exec(`CREATE SCHEMA IF NOT EXISTS stage2_timer_test`); err != nil {
		t.Fatal(err)
	}
	db, err := sql.Open("postgres", dsn+" search_path=stage2_timer_test")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	old := repository.DB
	repository.DB = db
	defer func() { repository.DB = old }()
	if _, err := db.Exec(`CREATE TABLE matches(instance_id text primary key,active_user_id integer,turn_number integer)`); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`INSERT INTO matches VALUES('timer-stale',1,2)`); err != nil {
		t.Fatal(err)
	}
	ms := &game.MatchState{InstanceID: "timer-stale", ActiveUserID: 1, TurnNumber: 2, TurnOrder: []int{1}}
	// A timer scheduled for turn 1 must not finish turn 2 even when the same
	// player is active again. The check happens after acquiring PostgreSQL lock.
	doEndTurn("timer-stale", 1, ms, 1)
	var active, turn int
	if err := db.QueryRow(`SELECT active_user_id,turn_number FROM matches WHERE instance_id='timer-stale'`).Scan(&active, &turn); err != nil {
		t.Fatal(err)
	}
	if active != 1 || turn != 2 || ms.TurnNumber != 2 {
		t.Fatalf("stale timer changed turn: db=%d/%d memory=%d", active, turn, ms.TurnNumber)
	}
}
