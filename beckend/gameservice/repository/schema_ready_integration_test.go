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
	// RunMigrations already applied the embedded stages. Simulate missing tables
	// in this isolated schema instead of replaying non-repeatable Stage 1 SQL.
	for _, stage := range []struct{ table, diagnostic string }{
		{"match_hex_edges", "Stage 1"},
		{"match_commands", "Stage 2"},
	} {
		if _, err := db.Exec(`ALTER TABLE ` + stage.table + ` RENAME TO readiness_missing_table`); err != nil {
			t.Fatal(err)
		}
		checkMissing(stage.diagnostic)
		if _, err := db.Exec(`ALTER TABLE readiness_missing_table RENAME TO ` + stage.table); err != nil {
			t.Fatal(err)
		}
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
