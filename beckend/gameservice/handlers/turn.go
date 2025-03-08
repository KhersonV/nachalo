
//==============================
// gameservice/handlers/turn.go
//==============================

//   обработчик запроса на завершение хода:
// ---------------------------------------------------------------------

package handlers

import (
	"encoding/json"
	"net/http"
	"log"
	"gameservice/middleware"
	"gameservice/game" // Импорт пакета, где определено состояние матчей и логика ходов
)

// EndTurnRequest представляет запрос на завершение хода.
// Теперь содержит instance_id для идентификации матча.
type EndTurnRequest struct {
	PlayerID   int    `json:"player_id"`
	InstanceID string `json:"instance_id"`
}

// EndTurnResponse возвращает ID нового активного игрока.
type EndTurnResponse struct {
	ActivePlayer int `json:"active_player"`
}

// EndTurnHandler обрабатывает запрос на завершение хода.
func EndTurnHandler(w http.ResponseWriter, r *http.Request) {
    var req EndTurnRequest
    if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
        log.Printf("Ошибка декодирования запроса завершения хода: %v", err)
        http.Error(w, "Bad request", http.StatusBadRequest)
        return
    }

    tokenUserID, ok := middleware.GetUserIDFromContext(r.Context())
    if !ok || tokenUserID != req.PlayerID {
        http.Error(w, "Запрещено завершать ход другому игроку", http.StatusForbidden)
        return
    }

    matchState, ok := game.GetMatchState(req.InstanceID)
    if !ok {
        log.Printf("Матч с instance_id %s не найден", req.InstanceID)
        http.Error(w, "Match not found", http.StatusNotFound)
        return
    }

    nextPlayer, err := matchState.EndTurn(req.PlayerID)
    if err != nil {
        log.Printf("Ошибка завершения хода: %v", err)
        http.Error(w, err.Error(), http.StatusBadRequest)
        return
    }

    response := EndTurnResponse{ActivePlayer: nextPlayer}
    w.Header().Set("Content-Type", "application/json")
    json.NewEncoder(w).Encode(response)
}

