package handlers

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"auth/common"
	"gameservice/repository"

	"github.com/gorilla/mux"
)

func openEquipmentLootHandlerTestDB(t *testing.T) *sql.DB {
	t.Helper()

	dsn := os.Getenv("GAME_DB_DSN")
	if dsn == "" {
		t.Skip("integration test skipped: GAME_DB_DSN not set")
	}

	db, err := sql.Open("postgres", dsn)
	if err != nil {
		t.Fatalf("failed to open db: %v", err)
	}
	repository.DB = db
	repository.RunMigrations()
	return db
}

func uniqueEquipmentLootHandlerUserID() int {
	return 1200000000 + int(time.Now().UnixNano()%100000000)
}

func createEquipmentLootHandlerProfile(t *testing.T, userID int, heroClassID string) {
	t.Helper()
	t.Cleanup(func() {
		_, _ = repository.DB.Exec(`DELETE FROM player_profiles WHERE user_id = $1`, userID)
	})

	_, err := repository.CreatePlayerProfile(repository.CreatePlayerProfileInput{
		UserID:        userID,
		Name:          fmt.Sprintf("loot_handler_%d", userID),
		Image:         "/profile/avatar.webp",
		CharacterType: heroClassID,
		Balance:       10000,
		Inventory:     `{}`,
	})
	if err != nil {
		t.Fatalf("CreatePlayerProfile: %v", err)
	}
}

func TestGetMyMatchLootHandlerReturnsOnlyCurrentUserLoot(t *testing.T) {
	db := openEquipmentLootHandlerTestDB(t)
	t.Cleanup(func() {
		_ = db.Close()
	})

	matchID := "handler-loot-match"
	userID := uniqueEquipmentLootHandlerUserID()
	otherUserID := uniqueEquipmentLootHandlerUserID() + 1
	createEquipmentLootHandlerProfile(t, userID, "guardian")
	createEquipmentLootHandlerProfile(t, otherUserID, "ranger")

	item, err := repository.GrantRandomEquipmentDropForMatch(userID, matchID)
	if err != nil {
		t.Fatalf("GrantRandomEquipmentDropForMatch user: %v", err)
	}
	otherItem, err := repository.GrantRandomEquipmentDropForMatch(otherUserID, matchID)
	if err != nil {
		t.Fatalf("GrantRandomEquipmentDropForMatch other user: %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/game/match/"+matchID+"/my-loot", nil)
	req = mux.SetURLVars(req, map[string]string{"instance_id": matchID})
	req = req.WithContext(context.WithValue(req.Context(), common.UserIDKey, userID))
	rec := httptest.NewRecorder()

	GetMyMatchLootHandler(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d body=%s", rec.Code, rec.Body.String())
	}

	var body matchEquipmentLootResponse
	if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if body.Status != "ok" || len(body.Items) != 1 {
		t.Fatalf("unexpected loot response: %+v", body)
	}
	if body.Items[0].ItemInstanceID != item.InstanceID {
		t.Fatalf("expected own item %s, got %+v", item.InstanceID, body.Items)
	}
	if body.Items[0].ItemInstanceID == otherItem.InstanceID {
		t.Fatalf("other player's loot leaked into response: %+v", body.Items)
	}
}

func TestGetMyMatchLootHandlerReturnsEmptyList(t *testing.T) {
	db := openEquipmentLootHandlerTestDB(t)
	t.Cleanup(func() {
		_ = db.Close()
	})

	userID := uniqueEquipmentLootHandlerUserID()
	createEquipmentLootHandlerProfile(t, userID, "mystic")

	req := httptest.NewRequest(http.MethodGet, "/game/match/empty-loot-match/my-loot", nil)
	req = mux.SetURLVars(req, map[string]string{"instance_id": "empty-loot-match"})
	req = req.WithContext(context.WithValue(req.Context(), common.UserIDKey, userID))
	rec := httptest.NewRecorder()

	GetMyMatchLootHandler(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d body=%s", rec.Code, rec.Body.String())
	}

	var body matchEquipmentLootResponse
	if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if body.Status != "ok" || len(body.Items) != 0 {
		t.Fatalf("expected empty loot response, got %+v", body)
	}
}
