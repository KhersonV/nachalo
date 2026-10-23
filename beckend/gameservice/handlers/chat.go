package handlers

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"gameservice/middleware"
	"gameservice/repository"
)

type chatStorage interface {
	PlayerExists(context.Context, int) (bool, error)
	Create(context.Context, repository.ChatCreate) (repository.ChatMessage, error)
	History(context.Context, repository.ChatHistoryQuery) ([]repository.ChatMessage, error)
}

type chatPartyResolver func(context.Context, int) (string, error)

var errNotInParty = errors.New("player is not in a party")
var errChatInvalidBody = errors.New("message must contain 1–1000 characters")
var errChatControlCharacter = errors.New("message contains a control character")

type chatService struct {
	store chatStorage
	party chatPartyResolver
}

type chatSendRequest struct {
	Channel     string `json:"channel"`
	Body        string `json:"body"`
	RecipientID int    `json:"recipientId,omitempty"`
	ConversationID int64 `json:"conversationId,omitempty"`
}

type chatHistoryResponse struct {
	Messages     []repository.ChatMessage `json:"messages"`
	NextBeforeID *int64                   `json:"nextBeforeId"`
}

func newChatService() chatService {
	return chatService{store: repository.ChatStore{DB: repository.DB}, party: currentChatParty}
}

func ChatSendHandler(w http.ResponseWriter, r *http.Request) {
	chatSendHandler(newChatService())(w, r)
}

func ChatHistoryHandler(w http.ResponseWriter, r *http.Request) {
	chatHistoryHandler(newChatService())(w, r)
}

func validClientChatChannel(channel string) bool {
	return channel == "MAIN" || channel == "GROUP" || channel == "TRADE" || channel == "PRIVATE"
}

func validateChatBody(body string) (string, error) {
	body = strings.TrimSpace(body)
	if !utf8.ValidString(body) || strings.ContainsRune(body, 0) || utf8.RuneCountInString(body) == 0 || utf8.RuneCountInString(body) > 1000 {
		return "", errChatInvalidBody
	}
	for _, r := range body {
		if r < 32 && r != '\n' && r != '\t' {
			return "", errChatControlCharacter
		}
	}
	return body, nil
}

func (s chatService) send(ctx context.Context, userID int, req chatSendRequest) (repository.ChatMessage, error) {
	if !validClientChatChannel(req.Channel) {
		return repository.ChatMessage{}, errChatBadChannel
	}
	body, err := validateChatBody(req.Body)
	if err != nil {
		return repository.ChatMessage{}, err
	}
	input := repository.ChatCreate{Channel: req.Channel, SenderUserID: userID, Body: body}
	switch req.Channel {
	case "GROUP":
		if req.RecipientID != 0 || req.ConversationID != 0 {
			return repository.ChatMessage{}, errChatBadRecipient
		}
		input.PartyID, err = s.party(ctx, userID)
		if err != nil {
			return repository.ChatMessage{}, err
		}
	case "PRIVATE":
		if req.RecipientID != 0 || req.ConversationID <= 0 {
			return repository.ChatMessage{}, errChatBadRecipient
		}
		input.ConversationID = req.ConversationID
	default:
		if req.RecipientID != 0 || req.ConversationID != 0 {
			return repository.ChatMessage{}, errChatBadRecipient
		}
	}
	return s.store.Create(ctx, input)
}

func (s chatService) history(ctx context.Context, userID int, channel string, otherUserID int, beforeID int64, limit int) ([]repository.ChatMessage, error) {
	if !validClientChatChannel(channel) {
		return nil, errChatBadChannel
	}
	exists, err := s.store.PlayerExists(ctx, userID)
	if err != nil {
		return nil, err
	}
	if !exists {
		return nil, repository.ErrChatSenderNotFound
	}
	q := repository.ChatHistoryQuery{Channel: channel, ViewerUserID: userID, BeforeID: beforeID, Limit: limit}
	switch channel {
	case "GROUP":
		if otherUserID != 0 {
			return nil, errChatBadRecipient
		}
		q.PartyID, err = s.party(ctx, userID)
		if err != nil {
			return nil, err
		}
	case "PRIVATE":
		return nil, errChatBadChannel
	default:
		if otherUserID != 0 {
			return nil, errChatBadRecipient
		}
	}
	return s.store.History(ctx, q)
}

var (
	errChatBadChannel   = errors.New("unsupported chat channel")
	errChatBadRecipient = errors.New("invalid recipient")
)

func chatError(w http.ResponseWriter, err error) {
	status := http.StatusInternalServerError
	switch {
	case errors.Is(err, errChatBadChannel), errors.Is(err, errChatBadRecipient), errors.Is(err, errChatInvalidBody), errors.Is(err, errChatControlCharacter):
		status = http.StatusBadRequest
	case errors.Is(err, errNotInParty):
		status = http.StatusForbidden
	case errors.Is(err, repository.ErrChatForbidden):
		status = http.StatusForbidden
	case errors.Is(err, repository.ErrChatConversationNotFound):
		status = http.StatusNotFound
	case errors.Is(err, repository.ErrChatSenderNotFound), errors.Is(err, repository.ErrChatRecipientNotFound):
		status = http.StatusNotFound
	case errors.Is(err, repository.ErrChatRateLimited):
		status = http.StatusTooManyRequests
	}
	if status == http.StatusInternalServerError {
		http.Error(w, "chat unavailable", status)
		return
	}
	http.Error(w, err.Error(), status)
}

func writeChatJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}

func chatSendHandler(service chatService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		userID, ok := middleware.GetUserIDFromContext(r.Context())
		if !ok || userID <= 0 {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		r.Body = http.MaxBytesReader(w, r.Body, 4096)
		decoder := json.NewDecoder(r.Body)
		decoder.DisallowUnknownFields()
		var req chatSendRequest
		if err := decoder.Decode(&req); err != nil {
			http.Error(w, "invalid chat request", http.StatusBadRequest)
			return
		}
		var extra any
		if err := decoder.Decode(&extra); err != io.EOF {
			http.Error(w, "invalid chat request", http.StatusBadRequest)
			return
		}
		message, err := service.send(r.Context(), userID, req)
		if err != nil {
			chatError(w, err)
			return
		}
		publishChatMessage(message)
		writeChatJSON(w, http.StatusCreated, message)
	}
}

func chatHistoryHandler(service chatService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		userID, ok := middleware.GetUserIDFromContext(r.Context())
		if !ok || userID <= 0 {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		values := r.URL.Query()
		for key, entries := range values {
			if key != "channel" && key != "recipientId" && key != "beforeId" && key != "limit" {
				http.Error(w, "invalid chat query", http.StatusBadRequest)
				return
			}
			if len(entries) != 1 {
				http.Error(w, "invalid chat query", http.StatusBadRequest)
				return
			}
		}
		recipientID := 0
		if raw := values.Get("recipientId"); raw != "" {
			var err error
			recipientID, err = strconv.Atoi(raw)
			if err != nil || recipientID <= 0 {
				http.Error(w, "invalid recipient", http.StatusBadRequest)
				return
			}
		}
		beforeID := int64(0)
		if raw := values.Get("beforeId"); raw != "" {
			var err error
			beforeID, err = strconv.ParseInt(raw, 10, 64)
			if err != nil || beforeID <= 0 {
				http.Error(w, "invalid cursor", http.StatusBadRequest)
				return
			}
		}
		limit := 50
		if raw := values.Get("limit"); raw != "" {
			var err error
			limit, err = strconv.Atoi(raw)
			if err != nil || limit < 1 || limit > 100 {
				http.Error(w, "invalid limit", http.StatusBadRequest)
				return
			}
		}
		messages, err := service.history(r.Context(), userID, values.Get("channel"), recipientID, beforeID, limit)
		if err != nil {
			chatError(w, err)
			return
		}
		response := chatHistoryResponse{Messages: messages}
		if len(messages) == limit {
			id := messages[len(messages)-1].ID
			response.NextBeforeID = &id
		}
		writeChatJSON(w, http.StatusOK, response)
	}
}

type matchmakingPartyState struct {
	InParty bool   `json:"inParty"`
	PartyID string `json:"partyId"`
	Members []struct {
		UserID int `json:"user_id"`
	} `json:"members"`
}

var chatPartyHTTPClient = &http.Client{Timeout: 3 * time.Second}

func currentChatParty(ctx context.Context, userID int) (string, error) {
	partyID, _, err := currentChatPartyMembers(ctx, userID)
	return partyID, err
}

func currentChatPartyMembers(ctx context.Context, userID int) (string, []int, error) {
	base := strings.TrimRight(os.Getenv("MATCHMAKING_URL"), "/")
	if base == "" {
		base = "http://matchmaking:8002"
	}
	endpoint := fmt.Sprintf("%s/matchmaking/party?player_id=%s", base, url.QueryEscape(strconv.Itoa(userID)))
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return "", nil, err
	}
	response, err := chatPartyHTTPClient.Do(request)
	if err != nil {
		return "", nil, err
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return "", nil, fmt.Errorf("party service returned %d", response.StatusCode)
	}
	var state matchmakingPartyState
	if err = json.NewDecoder(io.LimitReader(response.Body, 8192)).Decode(&state); err != nil {
		return "", nil, err
	}
	if !state.InParty || state.PartyID == "" {
		return "", nil, errNotInParty
	}
	members := make([]int, 0, len(state.Members))
	found := false
	for _, member := range state.Members {
		if member.UserID <= 0 {
			continue
		}
		members = append(members, member.UserID)
		if member.UserID == userID {
			found = true
		}
	}
	if !found {
		return "", nil, errNotInParty
	}
	return state.PartyID, members, nil
}
