package handlers

import (
	"database/sql"
	"encoding/json"
	"errors"
	"net/http"
	"strconv"

	"gameservice/middleware"
	"gameservice/repository"
	"github.com/gorilla/mux"
)

func ChatOnlinePlayersHandler(w http.ResponseWriter, r *http.Request) {
	userID, ok := middleware.GetUserIDFromContext(r.Context())
	if !ok || userID <= 0 {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	clientsMu.Lock()
	ids := make([]int, 0, len(clients))
	for id, sessions := range clients {
		if id > 0 && len(sessions) > 0 {
			ids = append(ids, id)
		}
	}
	clientsMu.Unlock()
	players, err := repository.OnlineChatPlayers(r.Context(), ids)
	if err != nil {
		http.Error(w, "online players unavailable", http.StatusServiceUnavailable)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{"players": players, "count": len(players)})
}

func ChatMutesHandler(w http.ResponseWriter, r *http.Request) {
	userID, ok := middleware.GetUserIDFromContext(r.Context())
	if !ok || userID <= 0 {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	ids, err := repository.ListChatMutes(r.Context(), userID)
	if err != nil {
		http.Error(w, "chat mutes unavailable", http.StatusServiceUnavailable)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{"mutedUserIds": ids})
}

func ChatMuteTargetHandler(w http.ResponseWriter, r *http.Request) {
	userID, ok := middleware.GetUserIDFromContext(r.Context())
	if !ok || userID <= 0 {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	targetID, err := strconv.Atoi(mux.Vars(r)["id"])
	if err != nil || targetID <= 0 || targetID == userID {
		http.Error(w, "invalid mute target", http.StatusBadRequest)
		return
	}
	err = repository.SetChatMute(r.Context(), userID, targetID, r.Method == http.MethodPut)
	if errors.Is(err, sql.ErrNoRows) {
		http.Error(w, "player not found", http.StatusNotFound)
		return
	}
	if err != nil {
		http.Error(w, "chat mute unavailable", http.StatusServiceUnavailable)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
