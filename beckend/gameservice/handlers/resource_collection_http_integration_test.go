package handlers

import (
	"bytes"
	"context"
	"database/sql"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"strings"
	"testing"
	"time"

	"auth/common"
	"gameservice/repository"
	_ "github.com/lib/pq"
)

func TestCollectResourceHTTPReplayPostgres(t *testing.T) {
	dsn := os.Getenv("STAGE2_TEST_DSN")
	if dsn == "" {
		t.Skip("set STAGE2_TEST_DSN to an isolated PostgreSQL database")
	}
	admin, err := sql.Open("postgres", dsn)
	if err != nil {
		t.Fatal(err)
	}
	defer admin.Close()
	schema := fmt.Sprintf("collect_http_%d", time.Now().UnixNano())
	if _, err := admin.Exec(`CREATE SCHEMA ` + schema); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _, _ = admin.Exec(`DROP SCHEMA ` + schema + ` CASCADE`) })
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
	old := repository.DB
	repository.DB = db
	defer func() { repository.DB = old }()
	setup := []string{
		`CREATE TABLE matches(instance_id text primary key,active_user_id int,map_width int,map_height int,state_revision bigint default 0,map_revision bigint default 0,quest_artifact_id int default 0)`,
		`CREATE TABLE match_players(instance_id text,user_id int,name text default 'Tester',image text default '',character_id int,character_type text default 'guardian',position jsonb,inventory jsonb default '{}'::jsonb,level int default 1,energy int default 5,max_energy int default 5,health int default 10,max_health int default 10,experience int default 0,max_experience int default 100,attack int default 1,defense int default 1,mobility int default 3,agility int default 1,sight_range int default 3,is_ranged bool default false,attack_range int default 1,group_id int default 0,balance int default 0,primary key(instance_id,user_id))`,
		`CREATE TABLE match_map_cells(instance_id text,cell_id serial,x int,y int,tile_code int,resource jsonb,barbel jsonb,monster jsonb,is_portal bool default false,is_player bool default false,structure_type text default '',structure_owner_user_id int default 0,structure_health int default 0,structure_defense int default 0,structure_attack int default 0,structure_energy int default 0,is_under_construction bool default false,construction_turns_left int default 0,primary key(instance_id,cell_id),unique(instance_id,x,y))`,
		`CREATE TABLE inventory_items(instance_id text,user_id int,item_type text,item_id int,item_name text,item_description text,item_count int,primary key(instance_id,user_id,item_type,item_id))`,
		`CREATE TABLE artifacts(id int primary key,name text)`,
	}
	for _, statement := range setup {
		if _, err := db.Exec(statement); err != nil {
			t.Fatal(err)
		}
	}
	stage2, err := os.ReadFile("../../../docs/hex-migration/stage2_commands.sql")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(string(stage2)); err != nil {
		t.Fatal(err)
	}
	for _, statement := range []string{
		`INSERT INTO matches(instance_id,active_user_id,map_width,map_height) VALUES('pickup-http',1,2,1)`,
		`INSERT INTO match_players(instance_id,user_id,position) VALUES('pickup-http',1,'{"x":0,"y":0}')`,
		`INSERT INTO match_map_cells(instance_id,x,y,tile_code,resource) VALUES('pickup-http',1,0,82,'{"id":9,"type":"wood","description":"Wood","image":"wood.png"}')`,
	} {
		if _, err := db.Exec(statement); err != nil {
			t.Fatal(err)
		}
	}

	call := func(commandID string) *httptest.ResponseRecorder {
		t.Helper()
		body := fmt.Sprintf(`{"instance_id":"pickup-http","user_id":1,"cell_x":1,"cell_y":0,"commandId":%q}`, commandID)
		req := httptest.NewRequest(http.MethodPost, "/game/collectResource", bytes.NewBufferString(body))
		req = req.WithContext(context.WithValue(req.Context(), common.UserIDKey, 1))
		res := httptest.NewRecorder()
		CollectResourceHandler(res, req)
		return res
	}
	for _, id := range []string{"one-attempt", "one-attempt"} {
		res := call(id)
		if res.Code != http.StatusOK {
			t.Fatalf("pickup %s: status=%d body=%s", id, res.Code, res.Body.String())
		}
	}
	if res := call("new-attempt"); res.Code != http.StatusBadRequest {
		t.Fatalf("second pickup: status=%d body=%s", res.Code, res.Body.String())
	}
	var count int
	if err := db.QueryRow(`SELECT item_count FROM inventory_items WHERE instance_id='pickup-http' AND user_id=1 AND item_id=9`).Scan(&count); err != nil || count != 1 {
		t.Fatalf("resource count=%d err=%v", count, err)
	}
	var state, mapRevision int64
	if err := db.QueryRow(`SELECT state_revision,map_revision FROM matches WHERE instance_id='pickup-http'`).Scan(&state, &mapRevision); err != nil || state != 1 || mapRevision != 1 {
		t.Fatalf("revisions=%d/%d err=%v", state, mapRevision, err)
	}
}
