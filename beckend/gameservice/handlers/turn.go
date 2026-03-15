// ==============================
// gameservice/handlers/turn.go
// ==============================
package handlers

import (
	"context"
	"encoding/json"
	"gameservice/game"
	"gameservice/middleware"
	"gameservice/repository"
	"log"
	"net/http"
	"sync"
	"time"
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


// Можно оставить energyRegen = 10 как дефолтное значение, а в функцию передавать явно нужное значение.
func regenEnergyForNextPlayer(instanceID string, userID int, regenEnergy int) error {
    if regenEnergy <= 0 {
        regenEnergy = 10 // дефолт, если не передан или <=0
    }
    nextPlayer, err := repository.GetMatchPlayerByID(instanceID, userID)
    if err != nil {
        return err
    }
    newEnergy := nextPlayer.Energy + regenEnergy
    if newEnergy > nextPlayer.MaxEnergy {
        newEnergy = nextPlayer.MaxEnergy
    }
    nextPlayer.Energy = newEnergy

    if err = repository.UpdateMatchPlayer(instanceID, nextPlayer); err != nil {
        return err
    }

    // WebSocket сообщение для клиента об обновлении энергии/игрока
    updatePlayerMsg := map[string]interface{}{
        "type": "UPDATE_PLAYER",
        "payload": map[string]interface{}{
            "instanceId": instanceID,
            "player":     nextPlayer,
        },
    }
    buf, _ := json.Marshal(updatePlayerMsg)
    Broadcast(buf)

    return nil
}


// turnTimerLimit is the per-turn time limit.
const turnTimerLimit = 60 * time.Second

// turnTimers holds per-instance cancel funcs for active turn timers.
var turnTimers sync.Map // key: instanceID → context.CancelFunc

// CancelTurnTimer stops the running turn timer for an instance.
func CancelTurnTimer(instanceID string) {
	if v, loaded := turnTimers.LoadAndDelete(instanceID); loaded {
		v.(context.CancelFunc)()
	}
}

// startTurnTimer schedules an auto-endTurn for instanceID/userID after turnTimerLimit.
func startTurnTimer(instanceID string, userID int) {
	CancelTurnTimer(instanceID)
	ctx, cancel := context.WithCancel(context.Background())
	turnTimers.Store(instanceID, cancel)
	go func() {
		select {
		case <-ctx.Done():
			return
		case <-time.After(turnTimerLimit):
		}
		ms, ok := game.GetMatchState(instanceID)
		if !ok || ms.ActiveUserID != userID {
			return
		}
		log.Printf("[turnTimer] auto-ending turn for user %d in match %s", userID, instanceID)
		doEndTurn(instanceID, userID, ms)
	}()
}

// doEndTurn executes the turn-change logic without an HTTP context (used by both
// the timer goroutine and EndTurnHandler to avoid duplication).
func doEndTurn(instanceID string, userID int, ms *game.MatchState) {
	nextUserID, err := ms.EndTurn(userID)
	if err != nil {
		log.Printf("[doEndTurn] EndTurn error for user %d: %v", userID, err)
		return
	}
	if err := repository.UpdateMatchTurn(instanceID, nextUserID, ms.TurnNumber); err != nil {
		log.Printf("[doEndTurn] UpdateMatchTurn error: %v", err)
		return
	}
	if err := regenEnergyForNextPlayer(instanceID, nextUserID, energyRegen); err != nil {
		log.Printf("[doEndTurn] regenEnergy error: %v", err)
		return
	}
	nextUser, err := repository.GetMatchPlayerByID(instanceID, nextUserID)
	if err != nil {
		log.Printf("[doEndTurn] GetMatchPlayerByID error: %v", err)
		return
	}
	updateMsg := map[string]interface{}{
		"type": "SET_ACTIVE_USER",
		"payload": map[string]interface{}{
			"instanceId":  instanceID,
			"active_user": nextUserID,
			"energy":      nextUser.Energy,
			"turnNumber":  ms.TurnNumber,
		},
	}
	updateJSON, _ := json.Marshal(updateMsg)
	Broadcast(updateJSON)
	startTurnTimer(instanceID, nextUserID)
}

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

	// Обновляем данные матча в БД
	if err := repository.UpdateMatchTurn(req.InstanceID, nextUserID, matchState.TurnNumber); err != nil {
		log.Printf("Ошибка обновления матча в БД: %v", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	// Регенерируем энергию для следующего игрока через общую функцию
	if err := regenEnergyForNextPlayer(req.InstanceID, nextUserID, energyRegen); err != nil {
		log.Printf("Ошибка начисления энергии следующему игроку: %v", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	// Получаем свежие данные нового активного игрока для ответа
	nextUser, err := repository.GetMatchPlayerByID(req.InstanceID, nextUserID)
	if err != nil {
		log.Printf("Ошибка получения данных нового активного игрока: %v", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	// Формируем сообщение для WebSocket с полным новым состоянием матча
	updateMsg := map[string]interface{}{
		"type": "SET_ACTIVE_USER",
		"payload": map[string]interface{}{
			"instanceId":  req.InstanceID,
			"active_user": nextUserID,
			"energy":      nextUser.Energy,
			"turnNumber":  matchState.TurnNumber,
		},
	}
	updateJSON, _ := json.Marshal(updateMsg)
	Broadcast(updateJSON)

	// Запускаем/сбрасываем таймер хода для нового активного игрока
	startTurnTimer(req.InstanceID, nextUserID)

	// Отправляем ответ клиенту
	response := EndTurnResponse{
		ActiveUser: nextUserID,
		Energy:     nextUser.Energy,
		TurnNumber: matchState.TurnNumber,
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}
