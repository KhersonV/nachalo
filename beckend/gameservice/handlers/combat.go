package handlers

import (
    "encoding/json"
    "fmt"
    "log"
    "net/http"
    "strconv"

    "github.com/gorilla/mux"

    "gameservice/middleware"
    "gameservice/models"
    "gameservice/repository"
)


// ----------------- ПЕРЕМЕЩЕНИЕ ------------------
func MoveHandler(w http.ResponseWriter, r *http.Request) {
    vars := mux.Vars(r)
    playerID, err := strconv.Atoi(vars["id"])
    if err != nil {
        http.Error(w, "Некорректный ID игрока", http.StatusBadRequest)
        return
    }

    // Извлекаем user_id из контекста (из JWT)
    tokenUserID, ok := middleware.GetUserIDFromContext(r.Context())
    if !ok {
        http.Error(w, "Не удалось определить пользователя из токена", http.StatusUnauthorized)
        return
    }
    // Проверяем, что игрок перемещает сам себя
    if tokenUserID != playerID {
        http.Error(w, "Запрещено изменять данные другого игрока", http.StatusForbidden)
        return
    }

    var req struct {
        NewPosX int `json:"new_pos_x"`
        NewPosY int `json:"new_pos_y"`
    }
    if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
        http.Error(w, "Неверный формат запроса", http.StatusBadRequest)
        return
    }

    instanceID := r.URL.Query().Get("instance_id")
    if instanceID == "" {
        http.Error(w, "instance_id обязателен", http.StatusBadRequest)
        return
    }

    // Получаем данные матча
    var mapWidth, mapHeight int
    var mapJSON []byte
    query := `SELECT map_width, map_height, map FROM matches WHERE instance_id = $1;`
    err = repository.DB.QueryRow(query, instanceID).Scan(&mapWidth, &mapHeight, &mapJSON)
    if err != nil {
        http.Error(w, fmt.Sprintf("Ошибка получения данных матча: %v", err), http.StatusInternalServerError)
        return
    }

    // Проверяем границы
    if req.NewPosX < 0 || req.NewPosX >= mapWidth || req.NewPosY < 0 || req.NewPosY >= mapHeight {
        http.Error(w, "Новые координаты вне границ карты", http.StatusBadRequest)
        return
    }

    // Десериализуем карту
    var rawMap [][]int
    if err := json.Unmarshal(mapJSON, &rawMap); err != nil {
        http.Error(w, fmt.Sprintf("Ошибка парсинга карты: %v", err), http.StatusInternalServerError)
        return
    }

    // Проверяем проходимость
    targetTileCode := rawMap[req.NewPosY][req.NewPosX]
    allowedTileCodes := []int{48, 80, 77, 82, 112} // '0','P','M','R','p' (в int)
    passable := false
    for _, code := range allowedTileCodes {
        if targetTileCode == code {
            passable = true
            break
        }
    }
    if !passable {
        http.Error(w, fmt.Sprintf("Невозможно переместиться: клетка tileCode=%d непроходимая", targetTileCode), http.StatusBadRequest)
        return
    }

    // Проверяем коллизию с другими игроками
    var collisionCount int
    collisionQuery := `
        SELECT COUNT(*) FROM match_players
        WHERE match_instance_id = $1 AND pos_x = $2 AND pos_y = $3 AND player_id <> $4
    `
    err = repository.DB.QueryRow(collisionQuery, instanceID, req.NewPosX, req.NewPosY, playerID).Scan(&collisionCount)
    if err != nil {
        http.Error(w, fmt.Sprintf("Ошибка проверки коллизий: %v", err), http.StatusInternalServerError)
        return
    }
    if collisionCount > 0 {
        http.Error(w, "Невозможно переместиться: клетка занята другим игроком", http.StatusBadRequest)
        return
    }

    // Получаем игрока из match_players
    player, err := repository.GetMatchPlayerByID(instanceID, playerID)
    if err != nil {
        http.Error(w, fmt.Sprintf("Ошибка получения игрока: %v", err), http.StatusInternalServerError)
        return
    }

    // Обновляем позицию
    player.PosX = req.NewPosX
    player.PosY = req.NewPosY

    err = repository.UpdateMatchPlayer(instanceID, player)
    if err != nil {
        http.Error(w, fmt.Sprintf("Ошибка обновления позиции игрока: %v", err), http.StatusInternalServerError)
        return
    }

    w.Header().Set("Content-Type", "application/json")
    json.NewEncoder(w).Encode(player)
}

// ----------------- АТАКА ------------------

type AttackRequest struct {
    AttackerType string `json:"attacker_type"` // "player" или "monster"
    AttackerID   int    `json:"attacker_id"`
    TargetType   string `json:"target_type"`   // "player" или "monster"
    TargetID     int    `json:"target_id"`
    InstanceID   string `json:"instance_id"`   // Для работы с match‑копиями
}

func UniversalAttackHandler(w http.ResponseWriter, r *http.Request) {
    var req AttackRequest
    if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
        http.Error(w, "Неверный формат запроса", http.StatusBadRequest)
        return
    }

    var attackerAttack, targetDefense int
    var err error

    // Определяем атаку
    switch req.AttackerType {
    case "player":
        player, errP := repository.GetMatchPlayerByID(req.InstanceID, req.AttackerID)
        if errP != nil {
            http.Error(w, fmt.Sprintf("Ошибка получения атакующего игрока: %v", errP), http.StatusInternalServerError)
            return
        }
        attackerAttack = player.Attack
    case "monster":
        monster, errM := getMonsterByID(req.AttackerID)
        if errM != nil {
            http.Error(w, fmt.Sprintf("Ошибка получения атакующего монстра: %v", errM), http.StatusInternalServerError)
            return
        }
        attackerAttack = monster.Attack
    default:
        http.Error(w, "Неверный тип атакующего", http.StatusBadRequest)
        return
    }

    // Определяем защиту и здоровье
    var targetHealth int
    switch req.TargetType {
    case "player":
        player, errP := repository.GetMatchPlayerByID(req.InstanceID, req.TargetID)
        if errP != nil {
            http.Error(w, fmt.Sprintf("Ошибка получения цели-игрока: %v", errP), http.StatusInternalServerError)
            return
        }
        targetDefense = player.Defense
        targetHealth = player.Health
    case "monster":
        monster, errM := getMonsterByID(req.TargetID)
        if errM != nil {
            http.Error(w, fmt.Sprintf("Ошибка получения цели-монстра: %v", errM), http.StatusInternalServerError)
            return
        }
        targetDefense = monster.Defense
        targetHealth = monster.Health
    default:
        http.Error(w, "Неверный тип цели", http.StatusBadRequest)
        return
    }

    damage := attackerAttack - targetDefense
    if damage < 1 {
        damage = 0
    }
    newHealth := targetHealth - damage
    if newHealth < 0 {
        newHealth = 0
    }

    // Применяем результат
    switch req.TargetType {
    case "player":
        target, errT := repository.GetMatchPlayerByID(req.InstanceID, req.TargetID)
        if errT != nil {
            http.Error(w, fmt.Sprintf("Ошибка получения цели игрока: %v", errT), http.StatusInternalServerError)
            return
        }
        target.Health = newHealth
        err = repository.UpdateMatchPlayer(req.InstanceID, target)
        if err != nil {
            http.Error(w, fmt.Sprintf("Ошибка обновления цели игрока: %v", err), http.StatusInternalServerError)
            return
        }
    case "monster":
        target, errT := getMonsterByID(req.TargetID)
        if errT != nil {
            http.Error(w, fmt.Sprintf("Ошибка получения цели монстра: %v", errT), http.StatusInternalServerError)
            return
        }
        target.Health = newHealth
        err = updateMonster(target)
        if err != nil {
            http.Error(w, fmt.Sprintf("Ошибка обновления цели монстра: %v", err), http.StatusInternalServerError)
            return
        }
    }

    log.Printf("Атакующий %s %d атаковал цель %s %d, нанеся %d урона", req.AttackerType, req.AttackerID, req.TargetType, req.TargetID, damage)
    response := map[string]interface{}{
        "attacker_type": req.AttackerType,
        "attacker_id":   req.AttackerID,
        "target_type":   req.TargetType,
        "target_id":     req.TargetID,
        "damage":        damage,
        "new_target_hp": newHealth,
    }
    w.Header().Set("Content-Type", "application/json")
    json.NewEncoder(w).Encode(response)
}

// ----------------- Работа с монстрами (упрощённо) ------------------

// В вашем исходном коде эти функции были "getMonsterByID" и "updateMonster".
// Их можно вынести в repository, но оставим здесь для наглядности.

func getMonsterByID(id int) (*models.Monster, error) {
    monster := &models.Monster{}
    query := `
        SELECT id, name, type, health, max_health, attack, defense, speed, 
               maneuverability, vision, created_at
        FROM monsters
        WHERE id = $1
    `
    row := repository.DB.QueryRow(query, id)
    err := row.Scan(
        &monster.ID,
        &monster.Name,
        &monster.Type,
        &monster.Health,
        &monster.MaxHealth,
        &monster.Attack,
        &monster.Defense,
        &monster.Speed,
        &monster.Maneuverability,
        &monster.Vision,
        &monster.CreatedAt,
    )
    if err != nil {
        return nil, err
    }
    return monster, nil
}

func updateMonster(monster *models.Monster) error {
    query := `
        UPDATE monsters
        SET health = $1
        WHERE id = $2
    `
    _, err := repository.DB.Exec(query, monster.Health, monster.ID)
    return err
}
