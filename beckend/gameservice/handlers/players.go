

//====================================
//gameservice/handlers/players.go
//====================================

package handlers

import (
    "encoding/json"
    "fmt"
    "log"
    "net/http"
    "strconv"
    "time"

    "github.com/gorilla/mux"

    "gameservice/models"
    "gameservice/repository"
)

// Структура запроса для создания игрока
type CreatePlayerRequest struct {
    UserID int    `json:"user_id"`
    Name   string `json:"name"`
    Image  string `json:"image"`
}

var levelThresholds = map[int]int{
    1:  500,
    2:  2000,
    3:  8000,
    4:  32000,
    5:  128000,
    6:  512000,
    7:  2048000,
    8:  8192000,
    9:  32768000,
    10: 131072000,
}

func CreatePlayerHandler(w http.ResponseWriter, r *http.Request) {
    var req CreatePlayerRequest
    if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
        http.Error(w, "Неверный формат запроса", http.StatusBadRequest)
        return
    }
    log.Printf("Получен запрос на создание игрока: %+v", req)

    if req.UserID == 0 {
        http.Error(w, "user_id обязателен", http.StatusBadRequest)
        return
    }
    if req.Name == "" {
        req.Name = "Player"
    }
    if req.Image == "" {
        req.Image = "/player-1.webp"
    }

    // Создаём запись в таблице players
    query := `
        INSERT INTO players (
            user_id, name, image,  pos_x, pos_y, energy, max_energy, 
            health, max_health, level, experience, max_experience, attack, defense, 
            speed, maneuverability, vision, vision_range, balance, inventory
        )
        VALUES (
            $1, $2, $3, 0, 0, 100, 100,
            100, 100, 1, 0, 500, 10, 5,
            3, 2, 2, 2, 0, '{}'
        )
        RETURNING id, user_id, name, image, pos_x, pos_y, energy, max_energy,
                  health, max_health, level, experience, max_experience, attack, defense,
                  speed, maneuverability, vision, vision_range, balance, inventory, updated_at
    `
    player := &models.Player{}
    err := repository.DB.QueryRow(query, req.UserID, req.Name, req.Image).Scan(
        &player.ID,
        &player.UserID,
        &player.Name,
        &player.Image,
        &player.PosX,
        &player.PosY,
        &player.Energy,
        &player.MaxEnergy,
        &player.Health,
        &player.MaxHealth,
        &player.Level,
        &player.Experience,
        &player.MaxExperience,
        &player.Attack,
        &player.Defense,
        &player.Speed,
        &player.Maneuverability,
        &player.Vision,
        &player.VisionRange,
        &player.Balance,
        &player.Inventory,
        &player.UpdatedAt,
    )
    if err != nil {
        http.Error(w, fmt.Sprintf("Ошибка создания игрока: %v", err), http.StatusInternalServerError)
        return
    }
    w.Header().Set("Content-Type", "application/json")
    w.WriteHeader(http.StatusCreated)
    json.NewEncoder(w).Encode(player)
}

func GetPlayerHandler(w http.ResponseWriter, r *http.Request) {
    vars := mux.Vars(r)
    idStr := vars["id"]
    id, err := strconv.Atoi(idStr)
    if err != nil {
        http.Error(w, "Некорректный ID", http.StatusBadRequest)
        return
    }
    player, err := repository.GetPlayerByUserID(id)
    if err != nil {
        http.Error(w, fmt.Sprintf("Ошибка получения игрока: %v", err), http.StatusInternalServerError)
        return
    }
    w.Header().Set("Content-Type", "application/json")
    json.NewEncoder(w).Encode(player)
}

func GainExperienceHandler(w http.ResponseWriter, r *http.Request) {
    vars := mux.Vars(r)
    idStr := vars["id"]
    id, err := strconv.Atoi(idStr)
    if err != nil {
        http.Error(w, "Некорректный ID", http.StatusBadRequest)
        return
    }

    player, err := repository.GetPlayerByUserID(id)
    if err != nil {
        http.Error(w, fmt.Sprintf("Ошибка получения игрока: %v", err), http.StatusInternalServerError)
        return
    }

    var req struct {
        Experience int `json:"experience"`
    }
    if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
        http.Error(w, "Неверный формат запроса", http.StatusBadRequest)
        return
    }

    // Увеличиваем опыт
    player.Experience += req.Experience

    // Проверка порога для левел-апа
    threshold, ok := levelThresholds[player.Level]
    if ok && player.Experience >= threshold {
        player.Level++
        player.Experience = 0
        log.Printf("Игрок %d повышен до уровня %d", player.ID, player.Level)
    }

    // Сохраняем изменения
    player.UpdatedAt = time.Now()
    err = repository.UpdatePlayer(player)
    if err != nil {
        http.Error(w, fmt.Sprintf("Ошибка обновления игрока: %v", err), http.StatusInternalServerError)
        return
    }

    w.Header().Set("Content-Type", "application/json")
    json.NewEncoder(w).Encode(player)
}
