package main

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"gameservice/repository"
)

type lifecycleConnector struct{ alive *bool }

func (c lifecycleConnector) Connect(context.Context) (driver.Conn, error) {
	return lifecycleConn{c.alive}, nil
}
func (c lifecycleConnector) Driver() driver.Driver { return lifecycleDriver{c.alive} }

type lifecycleDriver struct{ alive *bool }

func (d lifecycleDriver) Open(string) (driver.Conn, error) { return lifecycleConn{d.alive}, nil }

type lifecycleConn struct{ alive *bool }

func (c lifecycleConn) Prepare(string) (driver.Stmt, error) { return nil, errors.New("unused") }
func (c lifecycleConn) Close() error                        { return nil }
func (c lifecycleConn) Begin() (driver.Tx, error)           { return nil, errors.New("unused") }
func (c lifecycleConn) QueryContext(_ context.Context, query string, _ []driver.NamedValue) (driver.Rows, error) {
	if !strings.Contains(query, "m.finished_at IS NULL") || !strings.Contains(query, "p.health>0") {
		return nil, errors.New("active membership must check unfinished match and living HEX player")
	}
	return &lifecycleRows{alive: *c.alive}, nil
}

type lifecycleRows struct {
	alive bool
	done  bool
}

func (r *lifecycleRows) Columns() []string { return []string{"exists"} }
func (r *lifecycleRows) Close() error      { return nil }
func (r *lifecycleRows) Next(dest []driver.Value) error {
	if r.done {
		return io.EOF
	}
	dest[0] = r.alive
	r.done = true
	return nil
}

func TestDefeatedPlayerCanQueueWhileSurvivorKeepsMatch(t *testing.T) {
	alive := false
	db := sql.OpenDB(lifecycleConnector{alive: &alive})
	defer db.Close()
	previousDB := repository.DB
	repository.DB = db
	defer func() { repository.DB = previousDB }()

	matchMu.Lock()
	previousMatches, previousPlayers := currentMatches, playerMatches
	currentMatches = map[string]MatchInfo{"old": {InstanceID: "old", Mode: "1x1"}}
	playerMatches = map[int]string{101: "old", 102: "old"}
	matchMu.Unlock()
	defer func() {
		matchMu.Lock()
		currentMatches, playerMatches = previousMatches, previousPlayers
		matchMu.Unlock()
	}()

	request := httptest.NewRequest(http.MethodGet, "/matchmaking/currentMatch?player_id=101", nil)
	response := httptest.NewRecorder()
	currentMatchHandler(response, request)
	var waiting map[string]string
	if err := json.Unmarshal(response.Body.Bytes(), &waiting); err != nil || waiting["status"] != "waiting" {
		t.Fatalf("defeated player still has active match: %s (%v)", response.Body.String(), err)
	}
	if _, linked := playerMatches[101]; linked {
		t.Fatal("defeated player mapping was not cleared")
	}
	if _, linked := playerMatches[102]; !linked {
		t.Fatal("survivor mapping was removed")
	}

	mu.Lock()
	previousQueue := queues["1x1"]
	queues["1x1"] = nil
	mu.Unlock()
	defer func() { mu.Lock(); queues["1x1"] = previousQueue; mu.Unlock() }()
	response = httptest.NewRecorder()
	joinHandler(response, httptest.NewRequest(http.MethodPost, "/matchmaking/join", strings.NewReader(`{"player_id":101,"mode":"1x1"}`)))
	if response.Code != http.StatusOK || len(queues["1x1"]) != 1 {
		t.Fatalf("defeated player cannot queue: %d %s", response.Code, response.Body.String())
	}

	alive = true
	response = httptest.NewRecorder()
	currentMatchHandler(response, httptest.NewRequest(http.MethodGet, "/matchmaking/currentMatch?player_id=102", nil))
	var survivor MatchInfo
	if err := json.Unmarshal(response.Body.Bytes(), &survivor); err != nil || survivor.InstanceID != "old" {
		t.Fatalf("survivor lost original match: %s (%v)", response.Body.String(), err)
	}
	matchMu.Lock()
	delete(playerMatches, 102) // Simulate matchmaking restart with durable DB membership.
	matchMu.Unlock()
	response = httptest.NewRecorder()
	joinHandler(response, httptest.NewRequest(http.MethodPost, "/matchmaking/join", strings.NewReader(`{"player_id":102,"mode":"1x1"}`)))
	if response.Code != http.StatusConflict {
		t.Fatalf("living participant was not blocked by DB state: %d %s", response.Code, response.Body.String())
	}
}
