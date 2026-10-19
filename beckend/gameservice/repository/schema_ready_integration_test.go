package repository

import (
	"database/sql"
	"fmt"
	"net/url"
	"os"
	"strings"
	"testing"
	"time"

	_ "github.com/lib/pq"
)

func TestStage2SchemaReadinessPostgres(t *testing.T) {
	dsn := os.Getenv("STAGE2_TEST_DSN")
	if dsn == "" {
		t.Skip("set STAGE2_TEST_DSN to an isolated PostgreSQL database")
	}
	admin, err := sql.Open("postgres", dsn)
	if err != nil {
		t.Fatal(err)
	}
	defer admin.Close()
	schema := fmt.Sprintf("schema_ready_%d", time.Now().UnixNano())
	if _, err = admin.Exec(`CREATE SCHEMA ` + schema); err != nil {
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
	previous := DB
	DB = db
	defer func() { DB = previous }()

	RunMigrations()
	if _, err := db.Exec(`INSERT INTO matches(instance_id,mode,teams_count,total_players,map_height,map_width,map) VALUES('schema-sentinel','solo',1,1,1,1,'{}')`); err != nil {
		t.Fatal(err)
	}
	checkMissing := func(want string) {
		t.Helper()
		ready, err := SchemaReady()
		if ready || err == nil || !strings.Contains(err.Error(), want) {
			t.Fatalf("ready=%v err=%v, want diagnostic %q", ready, err, want)
		}
	}
	checkMissing("Stage 1")
	stage1, err := os.ReadFile("../../../docs/hex-migration/stage1_geometry.sql")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(string(stage1)); err != nil {
		t.Fatal(err)
	}
	checkMissing("Stage 2")
	stage2, err := os.ReadFile("../../../docs/hex-migration/stage2_commands.sql")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(string(stage2)); err != nil {
		t.Fatal(err)
	}
	ready, err := SchemaReady()
	if err != nil || !ready {
		t.Fatalf("complete schema: ready=%v err=%v", ready, err)
	}
	if _, err := db.Exec(`ALTER TABLE match_commands RENAME COLUMN payload_hash TO missing_payload_hash`); err != nil {
		t.Fatal(err)
	}
	checkMissing("match_commands.payload_hash")
	if _, err := db.Exec(`ALTER TABLE match_commands RENAME COLUMN missing_payload_hash TO payload_hash`); err != nil {
		t.Fatal(err)
	}
	ready, err = SchemaReady()
	if err != nil || !ready {
		t.Fatalf("restored schema: ready=%v err=%v", ready, err)
	}
	var count int
	if err := db.QueryRow(`SELECT count(*) FROM matches WHERE instance_id='schema-sentinel'`).Scan(&count); err != nil || count != 1 {
		t.Fatalf("existing match removed: count=%d err=%v", count, err)
	}
}
