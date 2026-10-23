package handlers

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"auth/common"
	"gameservice/middleware"
	"gameservice/repository"
	"github.com/golang-jwt/jwt/v4"
)

type fakeChatStore struct {
	created []repository.ChatCreate
	queried []repository.ChatHistoryQuery
}

func (s *fakeChatStore) PlayerExists(_ context.Context, id int) (bool, error) {
	return id >= 1 && id <= 3, nil
}
func (s *fakeChatStore) Create(_ context.Context, input repository.ChatCreate) (repository.ChatMessage, error) {
	s.created = append(s.created, input)
	return repository.ChatMessage{ID: 1, Channel: input.Channel, SenderUserID: input.SenderUserID}, nil
}
func (s *fakeChatStore) History(_ context.Context, query repository.ChatHistoryQuery) ([]repository.ChatMessage, error) {
	s.queried = append(s.queried, query)
	return []repository.ChatMessage{}, nil
}

func TestChatHTTPAuthenticationAndImpersonation(t *testing.T) {
	store := &fakeChatStore{}
	service := chatService{store: store, party: func(context.Context, int) (string, error) { return "party-a", nil }}
	handler := middleware.GameAuthMiddleware("test-secret", chatSendHandler(service))
	request := httptest.NewRequest(http.MethodPost, "/game/chat/messages", strings.NewReader(`{"channel":"MAIN","body":"hello"}`))
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusUnauthorized || len(store.created) != 0 {
		t.Fatalf("unauthenticated send: status=%d writes=%d", response.Code, len(store.created))
	}

	// Even with an authenticated context, identity fields are not accepted from JSON.
	spoof := httptest.NewRequest(http.MethodPost, "/game/chat/messages", strings.NewReader(`{"channel":"MAIN","body":"hello","senderUserId":999,"senderNickname":"Admin"}`))
	spoof = spoof.WithContext(context.WithValue(spoof.Context(), common.UserIDKey, 1))
	response = httptest.NewRecorder()
	chatSendHandler(service)(response, spoof)
	if response.Code != http.StatusBadRequest || len(store.created) != 0 {
		t.Fatalf("impersonation body: status=%d writes=%d", response.Code, len(store.created))
	}

	token, err := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{"user_id": 1}).SignedString([]byte("test-secret"))
	if err != nil {
		t.Fatal(err)
	}
	valid := httptest.NewRequest(http.MethodPost, "/game/chat/messages", strings.NewReader(`{"channel":"MAIN","body":"hello"}`))
	valid.Header.Set("Authorization", "Bearer "+token)
	response = httptest.NewRecorder()
	handler.ServeHTTP(response, valid)
	if response.Code != http.StatusCreated || len(store.created) != 1 || store.created[0].SenderUserID != 1 {
		t.Fatalf("authenticated identity: status=%d writes=%+v", response.Code, store.created)
	}
}

func TestChatGroupPermissionsAndPrivateRecipient(t *testing.T) {
	store := &fakeChatStore{}
	service := chatService{store: store, party: func(context.Context, int) (string, error) { return "", errNotInParty }}
	if _, err := service.send(context.Background(), 1, chatSendRequest{Channel: "GROUP", Body: "hello"}); !errors.Is(err, errNotInParty) {
		t.Fatalf("group send: %v", err)
	}
	if _, err := service.history(context.Background(), 1, "GROUP", 0, 0, 50); !errors.Is(err, errNotInParty) {
		t.Fatalf("group history: %v", err)
	}
	if len(store.created) != 0 || len(store.queried) != 0 {
		t.Fatal("unauthorized group operation reached storage")
	}

	if _, err := service.send(context.Background(), 1, chatSendRequest{Channel: "PRIVATE", RecipientID: 2, Body: "hi"}); err != nil {
		t.Fatal(err)
	}
	if store.created[0].SenderUserID != 1 || store.created[0].RecipientUserID != 2 {
		t.Fatalf("private identity: %+v", store.created[0])
	}
	if _, err := service.history(context.Background(), 3, "PRIVATE", 2, 0, 50); err != nil {
		t.Fatal(err)
	}
	if store.queried[0].ViewerUserID != 3 || store.queried[0].OtherUserID != 2 {
		t.Fatalf("private query identity: %+v", store.queried[0])
	}
}

func TestChatGroupUsesResolvedParty(t *testing.T) {
	store := &fakeChatStore{}
	service := chatService{store: store, party: func(_ context.Context, userID int) (string, error) {
		if userID != 1 {
			t.Fatalf("party lookup for user %d", userID)
		}
		return "actual-party", nil
	}}
	if _, err := service.send(context.Background(), 1, chatSendRequest{Channel: "GROUP", Body: "hello"}); err != nil {
		t.Fatal(err)
	}
	if store.created[0].PartyID != "actual-party" {
		t.Fatalf("stored party: %+v", store.created[0])
	}
	if _, err := service.history(context.Background(), 1, "GROUP", 0, 0, 50); err != nil {
		t.Fatal(err)
	}
	if store.queried[0].PartyID != "actual-party" {
		t.Fatalf("history party: %+v", store.queried[0])
	}
}

func TestChatPartyResolverRequiresCurrentMembership(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("player_id") != "7" {
			t.Errorf("party lookup used %q", r.URL.Query().Get("player_id"))
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"inParty": true, "partyId": "party-a", "members": []map[string]int{{"user_id": 1}}})
	}))
	defer server.Close()
	t.Setenv("MATCHMAKING_URL", server.URL)
	if _, err := currentChatParty(context.Background(), 7); !errors.Is(err, errNotInParty) {
		t.Fatalf("outsider party lookup: %v", err)
	}
}

func TestChatBodyValidation(t *testing.T) {
	for _, body := range []string{"  ", "\x00", strings.Repeat("a", 1001)} {
		if _, err := validateChatBody(body); err == nil {
			t.Fatalf("accepted invalid message %q", body)
		}
	}
	body, err := validateChatBody("  hello <b>world</b>  ")
	if err != nil || body != "hello <b>world</b>" {
		t.Fatalf("plain text: body=%q err=%v", body, err)
	}
	if validClientChatChannel("SYSTEM") {
		t.Fatal("client can send SYSTEM")
	}
}
