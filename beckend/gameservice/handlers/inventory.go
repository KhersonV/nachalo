

//====================================
//gameservice/handlers/inventory.go
//====================================


package handlers

import (
    "log"
    "encoding/json"
    "fmt"
    "net/http"
    "strconv"

    "github.com/gorilla/mux"
    "gameservice/repository"
)

func AddInventoryHandler(w http.ResponseWriter, r *http.Request) {
    vars := mux.Vars(r)
    playerID, err := strconv.Atoi(vars["id"])
    if err != nil {
        http.Error(w, "Некорректный ID игрока", http.StatusBadRequest)
        return
    }

    var req struct {
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
    if req.Count <= 0 {
        req.Count = 1
    }

    // Вызываем расширенный AddInventoryItem, чтобы сохранять image/description
    err = repository.AddInventoryItem(
        playerID,
        req.ItemType,
        req.ItemID,
        req.Count,
        req.Image,
        req.Description,
    )
    if err != nil {
        http.Error(w, fmt.Sprintf("Ошибка добавления предмета: %v", err), http.StatusInternalServerError)
        return
    }

    w.WriteHeader(http.StatusOK)
    w.Write([]byte("Предмет успешно добавлен"))
}

func UseInventoryHandler(w http.ResponseWriter, r *http.Request) {
    vars := mux.Vars(r)
    playerID, err := strconv.Atoi(vars["id"])
    if err != nil {
        http.Error(w, "Некорректный ID игрока", http.StatusBadRequest)
        return
    }

    var req struct {
        ItemType string `json:"item_type"`
        ItemID   int    `json:"item_id"`
        Count    int    `json:"count"`
    }
    if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
        http.Error(w, "Неверный формат запроса", http.StatusBadRequest)
        return
    }
    if req.Count <= 0 {
        req.Count = 1
    }

    // Сначала уменьшаем количество предмета в инвентаре (если используете inventory_items).
    err = repository.RemoveInventoryItem(playerID, req.ItemType, req.ItemID, req.Count)
    if err != nil {
        http.Error(w, fmt.Sprintf("Ошибка использования предмета: %v", err), http.StatusInternalServerError)
        return
    }

    // Получаем данные игрока.
    player, err := repository.GetMatchPlayerByUserID(playerID)
    if err != nil {
        http.Error(w, fmt.Sprintf("Ошибка получения игрока: %v", err), http.StatusInternalServerError)
        return
    }

    // Получаем эффект предмета из таблицы resources (или любой другой логики).
    effect, err := repository.GetItemEffect(req.ItemID)
    if err != nil {
        // Если эффект не найден, просто логируем.
        log.Printf("Эффект предмета не найден: %v", err)
    } else {
        // Применяем эффект: например, увеличиваем энергию или здоровье.
        if energyBonus, ok := effect["energy"]; ok {
            player.Energy += energyBonus
            if player.Energy > player.MaxEnergy {
                player.Energy = player.MaxEnergy
            }
        }
        if healthBonus, ok := effect["health"]; ok {
            player.Health += healthBonus
            if player.Health > player.MaxHealth {
                player.Health = player.MaxHealth
            }
        }
        // Добавьте обработку других эффектов по необходимости.
    }

    // Обновляем данные игрока в БД.
    err = repository.UpdateMatchPlayer(player)
    if err != nil {
        http.Error(w, fmt.Sprintf("Ошибка обновления игрока: %v", err), http.StatusInternalServerError)
        return
    }

    w.WriteHeader(http.StatusOK)
    w.Write([]byte("Предмет успешно использован и эффект применён"))
}
