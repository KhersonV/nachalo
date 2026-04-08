//====================================
// gameservice/handlers/inventory.go
//====================================

package handlers

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"sync"

	"gameservice/repository"

	"github.com/gorilla/mux"
)

const (
	foodCooldownTurns  = 2
	maxWaterUses2Turns = 5
)

type consumableUsageState struct {
	LastFoodTurn int
	WaterByTurn  map[int]int
}

var (
	consumableUseMu          sync.Mutex
	consumableUseByMatchUser = make(map[string]*consumableUsageState)
)

// AddInventoryHandler принимает запрос на добавление предмета в инвентарь игрока.
// Ожидаемый JSON в теле запроса:
//
//	{
//	   "instance_id": "match-123",             // ID текущей игры (обязательно)
//	   "item_type":   "food",                  // тип предмета
//	   "item_id":     1,                       // уникальный идентификатор ресурса/артефакта
//	   "count":       2,                       // сколько штук добавить (по умолчанию 1)
//	   "image":       "/path/to/image.webp",   // опционально, URL иконки
//	   "description": "Описание предмета"      // опционально, текстовое описание
//	}
//
// Выполняется SQL-запрос для получения текущего инвентаря (SELECT inventory, instance_id ...),
// затем происходит обновление инвентаря (UPDATE match_players ...),
// и, при необходимости, вставка/обновление в таблице inventory_items.
func AddInventoryHandler(w http.ResponseWriter, r *http.Request) {
	// Получаем playerID из URL-параметров
	vars := mux.Vars(r)
	playerID, err := strconv.Atoi(vars["id"])
	if err != nil {
		http.Error(w, "Некорректный ID игрока", http.StatusBadRequest)
		return
	}

	// Декодируем JSON-запрос
	var req struct {
		InstanceID  string `json:"instance_id"`
		ItemType    string `json:"item_type"`
		ItemID      int    `json:"item_id"`
		Count       int    `json:"count"`
		Image       string `json:"image,omitempty"`
		Description string `json:"description,omitempty"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Неверный формат запроса", http.StatusBadRequest)
		return
	}
	// Если количество не указано или меньше 1, устанавливаем значение 1
	if req.Count <= 0 {
		req.Count = 1
	}
	if req.InstanceID == "" {
		http.Error(w, "instance_id обязателен", http.StatusBadRequest)
		return
	}

	// Вызов функции репозитория для добавления предмета.
	// Внутри repository.AddInventoryItem:
	// 1. Выполняется SQL-запрос:
	//      SELECT inventory, instance_id FROM match_players WHERE user_id = $1 LIMIT 1;
	//    - Здесь берётся текущий инвентарь игрока (в формате JSON) и идентификатор матча.
	// 2. Производится проверка и обновление содержимого инвентаря (с добавлением нового или увеличением кол-ва).
	// 3. После обновления инвентарь сохраняется через SQL-запрос:
	//      UPDATE match_players SET inventory = $1 WHERE instance_id = $2 AND user_id = $3;
	err = repository.AddInventoryItem(
		req.InstanceID,
		playerID,
		req.ItemType,
		req.ItemID,
		req.Description, // itemName
		req.Image,       // imageURL
		req.Description, // description
		req.Count,
	)
	if err != nil {
		http.Error(w, fmt.Sprintf("Ошибка добавления предмета: %v", err), http.StatusInternalServerError)
		return
	}

	// Возвращаем явный ответ, что предмет успешно добавлен.
	w.WriteHeader(http.StatusOK)
	w.Write([]byte("Предмет успешно добавлен"))
}

// UseInventoryHandler обрабатывает запрос на использование (удаление/уменьшение количества) предмета.
// Ожидаемый JSON в теле запроса:
// {
//    InstanceID  string `json:"instance_id"`
//    "item_type": "water",
//    "item_id": 3,
//    "count": 1
// }

// UseInventoryHandler обрабатывает POST /game/player/{id}/inventory/use.
// После синхронизации таблиц он отправляет обновлённого игрока по WebSocket.
func UseInventoryHandler(w http.ResponseWriter, r *http.Request) {
	// 1. Парсим playerID
	vars := mux.Vars(r)
	playerID, err := strconv.Atoi(vars["id"])
	if err != nil {
		http.Error(w, "Некорректный ID игрока", http.StatusBadRequest)
		return
	}

	// 2. Парсим тело запроса
	var req struct {
		InstanceID string `json:"instance_id"`
		ItemType   string `json:"item_type"`
		ItemID     int    `json:"item_id"`
		Count      int    `json:"count"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Неверный формат JSON", http.StatusBadRequest)
		return
	}
	if req.Count <= 0 {
		req.Count = 1
	}
	if req.InstanceID == "" {
		http.Error(w, "instance_id обязателен", http.StatusBadRequest)
		return
	}

	resourceType, err := repository.GetResourceTypeByID(req.ItemID)
	if err != nil {
		http.Error(w, fmt.Sprintf("Не удалось определить тип ресурса: %v", err), http.StatusBadRequest)
		return
	}

	if resourceType != "food" && resourceType != "water" {
		http.Error(w, "Можно использовать только еду и воду", http.StatusBadRequest)
		return
	}

	matchInfo, err := repository.GetMatchByID(req.InstanceID)
	if err != nil {
		http.Error(w, fmt.Sprintf("Не удалось получить матч: %v", err), http.StatusInternalServerError)
		return
	}

	matchUserKey := fmt.Sprintf("%s:%d", req.InstanceID, playerID)
	turnNumber := matchInfo.TurnNumber

	consumableUseMu.Lock()
	state, ok := consumableUseByMatchUser[matchUserKey]
	if !ok {
		state = &consumableUsageState{WaterByTurn: make(map[int]int)}
		consumableUseByMatchUser[matchUserKey] = state
	}

	for turn := range state.WaterByTurn {
		if turn < turnNumber-1 {
			delete(state.WaterByTurn, turn)
		}
	}

	if resourceType == "food" {
		if req.Count > 1 {
			consumableUseMu.Unlock()
			http.Error(w, "Еду можно использовать только по 1 за раз", http.StatusBadRequest)
			return
		}
		if state.LastFoodTurn > 0 && turnNumber-state.LastFoodTurn < foodCooldownTurns {
			consumableUseMu.Unlock()
			http.Error(w, "Еду можно использовать только 1 раз в 2 хода", http.StatusBadRequest)
			return
		}
		state.LastFoodTurn = turnNumber
	} else if resourceType == "water" {
		waterUsedLastTwoTurns := state.WaterByTurn[turnNumber] + state.WaterByTurn[turnNumber-1]
		if waterUsedLastTwoTurns+req.Count > maxWaterUses2Turns {
			consumableUseMu.Unlock()
			http.Error(w, "Воды можно использовать максимум 5 за 2 хода", http.StatusBadRequest)
			return
		}
		state.WaterByTurn[turnNumber] += req.Count
	} else {
		consumableUseMu.Unlock()
		http.Error(w, "Неподдерживаемый тип расходника", http.StatusBadRequest)
		return
	}
	consumableUseMu.Unlock()

	// 3. Снижаем количество в normalized таблице и синхронизируем JSON
	if err := repository.RemoveInventoryItemAndSyncJSON(req.InstanceID, playerID, req.ItemType, req.ItemID, req.Count); err != nil {
		consumableUseMu.Lock()
		if rollbackState, ok := consumableUseByMatchUser[matchUserKey]; ok {
			if resourceType == "food" {
				if rollbackState.LastFoodTurn == turnNumber {
					rollbackState.LastFoodTurn = 0
				}
			}
			if resourceType == "water" {
				rollbackState.WaterByTurn[turnNumber] -= req.Count
				if rollbackState.WaterByTurn[turnNumber] <= 0 {
					delete(rollbackState.WaterByTurn, turnNumber)
				}
			}
		}
		consumableUseMu.Unlock()
		http.Error(w, fmt.Sprintf("Ошибка использования предмета: %v", err), http.StatusInternalServerError)
		return
	}

	// 4. Получаем свежие данные игрока
	player, err := repository.GetMatchPlayerByID(req.InstanceID, playerID)
	if err != nil {
		http.Error(w, fmt.Sprintf("Ошибка получения данных игрока: %v", err), http.StatusInternalServerError)
		return
	}

	// 5. Применяем эффект предмета и сохраняем изменения энергии/здоровья
	if effect, err := repository.GetItemEffect(req.ItemID); err == nil {
		if add, ok := effect["energy"]; ok {
			player.Energy += add
			if player.Energy > player.MaxEnergy {
				player.Energy = player.MaxEnergy
			}
		}
		if add, ok := effect["health"]; ok {
			player.Health += add
			if player.Health > player.MaxHealth {
				player.Health = player.MaxHealth
			}
		}
		_ = repository.UpdateMatchPlayer(req.InstanceID, player)
	}

	// 6. Рассылаем по WebSocket событие UPDATE_PLAYER с новым состоянием игрока
	wsMsg := map[string]interface{}{
		"type": "UPDATE_PLAYER",
		"payload": map[string]interface{}{
			"player": player,
		},
	}
	msgBytes, _ := json.Marshal(wsMsg)
	Broadcast(msgBytes)

	// 7. Возвращаем HTTP-ответ с обновлённым игроком
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(player); err != nil {
		http.Error(w, fmt.Sprintf("Ошибка кодирования ответа: %v", err), http.StatusInternalServerError)
	}
}
