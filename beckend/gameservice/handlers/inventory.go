

//====================================
// gameservice/handlers/inventory.go
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

// AddInventoryHandler принимает запрос на добавление предмета в инвентарь игрока.
// Ожидаемый JSON в теле запроса:
// {
//    "item_type": "food",
//    "item_id": 1,
//    "count": 2,
//    "image": "/path/to/image.webp",         // опционально
//    "description": "Описание предмета"        // опционально
// }
// Выполняется SQL-запрос для получения текущего инвентаря (SELECT inventory, match_instance_id ...),
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

    // Вызов функции репозитория для добавления предмета.
    // Внутри repository.AddInventoryItem:
    // 1. Выполняется SQL-запрос:
    //      SELECT inventory, match_instance_id FROM match_players WHERE user_id = $1 LIMIT 1;
    //    - Здесь берётся текущий инвентарь игрока (в формате JSON) и идентификатор матча.
    // 2. Производится проверка и обновление содержимого инвентаря (с добавлением нового или увеличением кол-ва).
    // 3. После обновления инвентарь сохраняется через SQL-запрос:
    //      UPDATE match_players SET inventory = $1 WHERE match_instance_id = $2 AND user_id = $3;
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

    // Возвращаем явный ответ, что предмет успешно добавлен.
    w.WriteHeader(http.StatusOK)
    w.Write([]byte("Предмет успешно добавлен"))
}

// UseInventoryHandler обрабатывает запрос на использование (удаление/уменьшение количества) предмета.
// Ожидаемый JSON в теле запроса:
// {
//    "item_type": "water",
//    "item_id": 3,
//    "count": 1
// }
// Последовательность действий:
// 1. SQL-запрос в таблице inventory_items для получения текущего количества:
//      SELECT count FROM inventory_items WHERE user_id = $1 AND item_type = $2 AND item_id = $3;
// 2. Если новое количество (текущее - запрошенное) <= 0, выполняется SQL-запрос:
//      DELETE FROM inventory_items WHERE user_id = $1 AND item_type = $2 AND item_id = $3;
//    иначе – обновляется количество с помощью:
//      UPDATE inventory_items SET count = $1 WHERE user_id = $2 AND item_type = $3 AND item_id = $4;
// 3. После успешного изменения предмета, извлекаются данные игрока (через GetMatchPlayerByUserID)
//    для применения эффекта предмета (например, увеличение энергии или здоровья).
// 4. Применённый эффект определяется с помощью запроса (например, из таблицы resources):
//      (логика внутри repository.GetItemEffect)
// 5. Обновление игрока производится через repository.UpdateMatchPlayer, который внутри выполняет SQL UPDATE.
func UseInventoryHandler(w http.ResponseWriter, r *http.Request) {
    // Получаем playerID из URL-параметров
    vars := mux.Vars(r)
    playerID, err := strconv.Atoi(vars["id"])
    if err != nil {
        http.Error(w, "Некорректный ID игрока", http.StatusBadRequest)
        return
    }

    // Декодируем JSON-запрос с параметрами использования предмета
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

    // Уменьшаем количество предмета в таблице inventory_items.
    // Внутри repository.RemoveInventoryItem происходит:
    // 1. SELECT count FROM inventory_items WHERE user_id = $1 AND item_type = $2 AND item_id = $3;
    // 2. Если (current count - req.Count) <= 0, выполняется DELETE:
    //      DELETE FROM inventory_items WHERE user_id = $1 AND item_type = $2 AND item_id = $3;
    //    иначе – UPDATE:
    //      UPDATE inventory_items SET count = $1 WHERE user_id = $2 AND item_type = $3 AND item_id = $4;
    err = repository.RemoveInventoryItem(playerID, req.ItemType, req.ItemID, req.Count)
    if err != nil {
        http.Error(w, fmt.Sprintf("Ошибка использования предмета: %v", err), http.StatusInternalServerError)
        return
    }

    // Извлекаем обновлённые данные игрока для применения эффекта предмета.
    // SQL-запрос внутри repository.GetMatchPlayerByUserID, например:
    //      SELECT * FROM match_players WHERE user_id = $1;
    player, err := repository.GetMatchPlayerByUserID(playerID)
    if err != nil {
        http.Error(w, fmt.Sprintf("Ошибка получения игрока: %v", err), http.StatusInternalServerError)
        return
    }

    // Получаем эффект предмета из таблицы ресурсов (или другой логики) через repository.GetItemEffect.
    // Внутри функции может выполняться SQL-запрос к таблице resources:
    //      SELECT energy, health, ... FROM resources WHERE item_id = $1;
    effect, err := repository.GetItemEffect(req.ItemID)
    if err != nil {
        log.Printf("Эффект предмета не найден: %v", err)
    } else {
        // Применяем эффект, например, увеличиваем энергию и здоровье.
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
        // Здесь можно добавить обработку других эффектов по необходимости.
    }

    // Обновляем данные игрока в базе.
    // Функция repository.UpdateMatchPlayer внутри выполняет SQL UPDATE для сохранения изменённых параметров.
    err = repository.UpdateMatchPlayer(player)
    if err != nil {
        http.Error(w, fmt.Sprintf("Ошибка обновления игрока: %v", err), http.StatusInternalServerError)
        return
    }

    // Возвращаем явный ответ, что предмет использован и эффект применён.
    w.WriteHeader(http.StatusOK)
    w.Write([]byte("Предмет успешно использован и эффект применён"))
}
