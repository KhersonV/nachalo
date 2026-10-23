package repository

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"net/url"
	"os"
	"strings"
	"testing"
	"time"

	_ "github.com/lib/pq"
)

func chatTestDB(t *testing.T) *sql.DB {
	t.Helper()
	dsn := os.Getenv("STAGE2_TEST_DSN")
	if dsn == "" {
		t.Skip("set STAGE2_TEST_DSN to an isolated PostgreSQL database")
	}
	admin, err := sql.Open("postgres", dsn)
	if err != nil {
		t.Fatal(err)
	}
	schema := fmt.Sprintf("chat_stage81_%d", time.Now().UnixNano())
	if _, err = admin.Exec(`CREATE SCHEMA ` + schema); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _, _ = admin.Exec(`DROP SCHEMA ` + schema + ` CASCADE`); _ = admin.Close() })
	if strings.HasPrefix(dsn, "postgres://") || strings.HasPrefix(dsn, "postgresql://") {
		parsed, parseErr := url.Parse(dsn)
		if parseErr != nil {
			t.Fatal(parseErr)
		}
		query := parsed.Query()
		query.Set("options", "-csearch_path="+schema)
		parsed.RawQuery = query.Encode()
		dsn = parsed.String()
	} else {
		dsn += " options='-csearch_path=" + schema + "'"
	}
	db, err := sql.Open("postgres", dsn)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	if _, err = db.Exec(`CREATE TABLE player_profiles(user_id integer PRIMARY KEY, name text NOT NULL, selected_character_id integer);
        CREATE TABLE player_characters(id serial PRIMARY KEY, user_id integer NOT NULL);`); err != nil {
		t.Fatal(err)
	}
	previous := DB
	DB = db
	t.Cleanup(func() { DB = previous })
	if err = EnsureChatSchema(); err != nil {
		t.Fatal(err)
	}
	if err = EnsureChatSchema(); err != nil {
		t.Fatalf("idempotent migration: %v", err)
	}
	return db
}

func TestChatPersistencePrivacyRateAndUTC(t *testing.T) {
	db := chatTestDB(t)
	ctx := context.Background()
	if _, err := db.Exec(`INSERT INTO player_profiles(user_id,name) VALUES (1,'<b>Alice</b>'),(2,'Bob'),(3,'Carol');
        INSERT INTO player_characters(id,user_id) VALUES (11,1);
        UPDATE player_profiles SET selected_character_id=11 WHERE user_id=1`); err != nil {
		t.Fatal(err)
	}
	store := ChatStore{DB: db}
	before := time.Now().UTC().Add(-time.Second)
	private, err := store.Create(ctx, ChatCreate{Channel: "PRIVATE", SenderUserID: 1, RecipientUserID: 2, Body: "private <text>"})
	if err != nil {
		t.Fatal(err)
	}
	if private.ID <= 0 || private.SenderUserID != 1 || private.SenderNickname != "<b>Alice</b>" || private.SenderCharacterID == nil || *private.SenderCharacterID != 11 || private.Body != "private <text>" {
		t.Fatalf("stored identity: %+v", private)
	}
	if private.CreatedAt.Before(before) || private.CreatedAt.After(time.Now().UTC().Add(time.Second)) || private.CreatedAt.Location() != time.UTC {
		t.Fatalf("server UTC timestamp: %v", private.CreatedAt)
	}
	if _, err = store.Create(ctx, ChatCreate{Channel: "MAIN", SenderUserID: 1, Body: "too fast"}); !errors.Is(err, ErrChatRateLimited) {
		t.Fatalf("rate limit: %v", err)
	}

	allowed, err := store.History(ctx, ChatHistoryQuery{Channel: "PRIVATE", ViewerUserID: 2, OtherUserID: 1, Limit: 50})
	if err != nil || len(allowed) != 1 || allowed[0].ID != private.ID {
		t.Fatalf("participant private history: %v %+v", err, allowed)
	}
	outsider, err := store.History(ctx, ChatHistoryQuery{Channel: "PRIVATE", ViewerUserID: 3, OtherUserID: 2, Limit: 50})
	if err != nil || len(outsider) != 0 {
		t.Fatalf("third-party private history: %v %+v", err, outsider)
	}
	public, err := store.History(ctx, ChatHistoryQuery{Channel: "MAIN", ViewerUserID: 3, Limit: 50})
	if err != nil || len(public) != 0 {
		t.Fatalf("private leaked into public history: %v %+v", err, public)
	}

	if _, err = db.Exec(`INSERT INTO chat_messages(channel,sender_user_id,sender_nickname,body,party_id) VALUES
        ('GROUP',1,'Alice','party-a only','party-a'),('GROUP',2,'Bob','party-b only','party-b')`); err != nil {
		t.Fatal(err)
	}
	group, err := store.History(ctx, ChatHistoryQuery{Channel: "GROUP", PartyID: "party-a", ViewerUserID: 1, Limit: 50})
	if err != nil || len(group) != 1 || group[0].Body != "party-a only" {
		t.Fatalf("group isolation: %v %+v", err, group)
	}
	if _, err = db.Exec(`INSERT INTO chat_messages(channel,sender_user_id,sender_nickname,body,party_id) VALUES ('GROUP',1,'Alice','invalid',NULL)`); err == nil {
		t.Fatal("group constraint accepted missing party")
	}
	if _, err = db.Exec(`UPDATE chat_messages SET created_at=now()-interval '91 days' WHERE id=$1`, private.ID); err != nil {
		t.Fatal(err)
	}
	allowed, err = store.History(ctx, ChatHistoryQuery{Channel: "PRIVATE", ViewerUserID: 2, OtherUserID: 1, Limit: 50})
	if err != nil || len(allowed) != 0 {
		t.Fatalf("retention visibility: %v %+v", err, allowed)
	}
	if err = store.PruneExpired(ctx); err != nil {
		t.Fatal(err)
	}
}
