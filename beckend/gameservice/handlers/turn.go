//==============================
// gameservice/handlers/turn.go
//==============================
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
	PlayerID   int    `json:"player_id"`
	InstanceID string `json:"instance_id"`
}

// EndTurnResponse возвращает статус матча, включая активного игрока, энергию и номер хода.
type EndTurnResponse struct {
	ActivePlayer int `json:"active_player"`
	Energy       int `json:"energy"`
	TurnNumber   int `json:"turn_number"`
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
	if !ok || tokenUserID != req.PlayerID {
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
	nextPlayerID, err := matchState.EndTurn(req.PlayerID)
	if err != nil {
		log.Printf("Ошибка завершения хода: %v", err)
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	// Получаем данные нового активного игрока
	nextPlayer, err := repository.GetMatchPlayerByID(req.InstanceID, nextPlayerID)
	if err != nil {
		log.Printf("Ошибка получения данных нового активного игрока: %v", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	// Обновляем данные матча в БД
if err := repository.UpdateMatchTurn(req.InstanceID, nextPlayerID, matchState.TurnNumber); err != nil {
    log.Printf("Ошибка обновления матча в БД: %v", err)
    http.Error(w, "Internal server error", http.StatusInternalServerError)
    return
}

	// Пополняем энергию нового активного игрока
	newEnergy := nextPlayer.Energy + energyRegen
	if newEnergy > nextPlayer.MaxEnergy {
		newEnergy = nextPlayer.MaxEnergy
	}
	nextPlayer.Energy = newEnergy

	// Обновляем данные игрока в БД (энергия и прочее обновление состояния)
	if err = repository.UpdateMatchPlayer(req.InstanceID, nextPlayer); err != nil {
		log.Printf("Ошибка обновления данных нового активного игрока: %v", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	// Формируем сообщение для WebSocket с полным новым состоянием матча
	updateMsg := map[string]interface{}{
		"type": "SET_ACTIVE_PLAYER",
		"payload": map[string]interface{}{
			"instanceId":   req.InstanceID,
			"activePlayer": nextPlayerID,
			"energy":       newEnergy,
			"turnNumber":   matchState.TurnNumber,
		},
	}
	updateJSON, _ := json.Marshal(updateMsg)
	Broadcast(updateJSON)

	// Отправляем ответ клиенту
	response := EndTurnResponse{
		ActivePlayer: nextPlayerID,
		Energy:       newEnergy,
		TurnNumber:   matchState.TurnNumber,
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}
