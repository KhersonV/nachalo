// ==============================
// gameservice/handlers/turn.go
// ==============================
package handlers

import (
	"encoding/json"
	"gameservice/game"
	"gameservice/middleware"
	"gameservice/repository"
	"log"
	"net/http"
)

// EndTurnRequest представляет запрос на завершение хода.
// Теперь содержит instance_id для идентификации матча.
type EndTurnRequest struct {
	UserID     int    `json:"user_id"`
	InstanceID string `json:"instance_id"`
}

// EndTurnResponse возвращает статус матча, включая активного игрока, энергию и номер хода.
type EndTurnResponse struct {
	ActiveUser int `json:"active_user"`
	Energy     int `json:"energy"`
	TurnNumber int `json:"turn_number"`
}

// Константа пополнения энергии при получении хода
const energyRegen = 10

// EndTurnHandler теперь полностью выполняет логику смены хода на сервере
func EndTurnHandler(w http.ResponseWriter, r *http.Request) {
	var req EndTurnRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		log.Printf("Ошибка декодирования запроса завершения хода: %v", err)
		http.Error(w, "Bad request", http.StatusBadRequest)
		return
	}

	// Проверяем, что игрок завершает свой ход
	tokenUserID, ok := middleware.GetUserIDFromContext(r.Context())
	if !ok || tokenUserID != req.UserID {
		http.Error(w, "Запрещено завершать ход другому игроку", http.StatusForbidden)
		return
	}

	// Получаем состояние матча
	matchState, ok := game.GetMatchState(req.InstanceID)
	if !ok {
		log.Printf("Матч с instance_id %s не найден", req.InstanceID)
		http.Error(w, "Match not found", http.StatusNotFound)
		return
	}

	// Завершаем ход (логика выбора следующего игрока находится в EndTurn)
	nextUserID, err := matchState.EndTurn(req.UserID)
	if err != nil {
		log.Printf("Ошибка завершения хода: %v", err)
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	// Получаем данные нового активного игрока
	nextUser, err := repository.GetMatchPlayerByID(req.InstanceID, nextUserID)
	if err != nil {
		log.Printf("Ошибка получения данных нового активного игрока: %v", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	// Обновляем данные матча в БД
	if err := repository.UpdateMatchTurn(req.InstanceID, nextUserID, matchState.TurnNumber); err != nil {
		log.Printf("Ошибка обновления матча в БД: %v", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	// Пополняем энергию нового активного игрока
	newEnergy := nextUser.Energy + energyRegen
	if newEnergy > nextUser.MaxEnergy {
		newEnergy = nextUser.MaxEnergy
	}
	nextUser.Energy = newEnergy

	// Обновляем данные игрока в БД (энергия и прочее обновление состояния)
	if err = repository.UpdateMatchPlayer(matchState.InstanceID, nextUser); err != nil {
		log.Printf("Ошибка обновления данных нового активного игрока: %v", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	// Формируем сообщение для WebSocket с полным новым состоянием матча
	updateMsg := map[string]interface{}{
		"type": "SET_ACTIVE_USER",
		"payload": map[string]interface{}{
			"instanceId":  req.InstanceID,
			"active_user": nextUserID,
			"energy":      newEnergy,
			"turnNumber":  matchState.TurnNumber,
		},
	}
	updateJSON, _ := json.Marshal(updateMsg)
	Broadcast(updateJSON)

	// Отправляем ответ клиенту
	response := EndTurnResponse{
		ActiveUser: nextUserID,
		Energy:     newEnergy,
		TurnNumber: matchState.TurnNumber,
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}
