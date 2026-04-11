//====================================
// gameservice/handlers/inventory.go
//====================================

package handlers

import (
	"encoding/json"
	"fmt"
	"math"
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

	// Special-case: scrolls — обработаем отдельно
	if req.ItemType == "scroll" {
		// find scroll metadata by ID
		scItem, ok := getScrollItemByID(req.ItemID)
		if !ok {
			http.Error(w, "unsupported scroll id", http.StatusBadRequest)
			return
		}

		// Ensure player participates in match
		matchPlayer, err := repository.GetMatchPlayerByID(req.InstanceID, playerID)
		if err != nil {
			http.Error(w, fmt.Sprintf("Не удалось получить игрока в матче: %v", err), http.StatusInternalServerError)
			return
		}

		// Try to remove from match inventory first; fallback to persistent inventory
		if err := repository.RemoveInventoryItemAndSyncJSON(req.InstanceID, playerID, "scroll", req.ItemID, req.Count); err != nil {
			if err2 := repository.ConsumePlayerInventoryItem(playerID, "scroll", req.ItemID, req.Count); err2 != nil {
				http.Error(w, fmt.Sprintf("Ошибка списания свитка: %v / %v", err, err2), http.StatusInternalServerError)
				return
			}
		}

		// Compute target coordinate depending on scroll type (re-using scrolls logic)
		st := scItem.Type
		var axis string
		var value int

		switch st {
		case "scroll_portal_x", "scroll_portal_y":
			matchInfo, err := repository.GetMatchByID(req.InstanceID)
			if err != nil {
				http.Error(w, fmt.Sprintf("failed to load match: %v", err), http.StatusInternalServerError)
				return
			}
			var portal [2]int
			if err := json.Unmarshal(matchInfo.PortalPosition, &portal); err != nil {
				http.Error(w, "portal not found", http.StatusInternalServerError)
				return
			}
			if st == "scroll_portal_x" {
				axis = "x"
				value = portal[0]
			} else {
				axis = "y"
				value = portal[1]
			}

		case "scroll_nearest_player_x", "scroll_nearest_player_y":
			players, err := repository.GetPlayersInMatch(req.InstanceID)
			if err != nil {
				http.Error(w, fmt.Sprintf("failed to load match players: %v", err), http.StatusInternalServerError)
				return
			}
			if len(players) <= 1 {
				http.Error(w, "no_target", http.StatusBadRequest)
				return
			}
			minD := math.MaxFloat64
			var targetX, targetY int
			for _, p := range players {
				if p.UserID == matchPlayer.UserID {
					continue
				}
				dx := float64(p.Position.X - matchPlayer.Position.X)
				dy := float64(p.Position.Y - matchPlayer.Position.Y)
				d := math.Sqrt(dx*dx + dy*dy)
				if d < minD {
					minD = d
					targetX = p.Position.X
					targetY = p.Position.Y
				}
			}
			if st == "scroll_nearest_player_x" {
				axis = "x"
				value = targetX
			} else {
				axis = "y"
				value = targetY
			}

		case "scroll_nearest_barrel_x", "scroll_nearest_barrel_y":
			cells, err := repository.LoadMapCells(req.InstanceID)
			if err != nil {
				http.Error(w, fmt.Sprintf("failed to load map cells: %v", err), http.StatusInternalServerError)
				return
			}
			minD := math.MaxFloat64
			var bx, by int
			for _, c := range cells {
				if c.Barbel == nil {
					continue
				}
				dx := float64(c.X - matchPlayer.Position.X)
				dy := float64(c.Y - matchPlayer.Position.Y)
				d := math.Sqrt(dx*dx + dy*dy)
				if d < minD {
					minD = d
					bx = c.X
					by = c.Y
				}
			}
			if minD == math.MaxFloat64 {
				http.Error(w, "no_target", http.StatusBadRequest)
				return
			}
			if st == "scroll_nearest_barrel_x" {
				axis = "x"
				value = bx
			} else {
				axis = "y"
				value = by
			}

		default:
			http.Error(w, "unsupported scroll type", http.StatusBadRequest)
			return
		}

		// Return updated match player and scroll result so frontend can update UI
		updatedPlayer, err := repository.GetMatchPlayerByID(req.InstanceID, playerID)
		if err != nil {
			http.Error(w, fmt.Sprintf("Ошибка получения игрока: %v", err), http.StatusInternalServerError)
			return
		}

		resp := map[string]any{
			"player": updatedPlayer,
			"scroll_result": map[string]any{"axis": axis, "value": value},
		}
		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(resp); err != nil {
			http.Error(w, fmt.Sprintf("Ошибка кодирования ответа: %v", err), http.StatusInternalServerError)
		}
		return
	}

	// Для остальных типов — определим ресурсный тип (food/water)
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
