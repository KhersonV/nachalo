//====================================
// gameservice/handlers/players.go
//====================================

package handlers

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strconv"

	"github.com/gorilla/mux"
	"gameservice/models"
	"gameservice/repository"
)

// CreatePlayerRequest – структура запроса для создания игрока
type CreatePlayerRequest struct {
	UserID int    `json:"user_id"`
	Name   string `json:"name"`
	Image  string `json:"image"`
}

// Уровневые пороги для повышения уровня
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

// CreatePlayerHandler – создает нового игрока.
// Вставка выполняется с полем position, которое хранится как JSONB (например, '{"x": 0, "y": 0}').
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

	
	query := `
		INSERT INTO players (
			user_id, name, image, energy, max_energy, 
			health, max_health, level, experience, max_experience, 
			attack, defense, speed, maneuverability, vision, vision_range, balance, inventory
		)
		VALUES (
			$1, $2, $3, 100, 100,
			100, 100, 1, 0, 500, 10, 5,
			3, 2, 2, 2, 0, '{}'
		)
		RETURNING user_id, name, image, energy, max_energy,
				  health, max_health, level, experience, max_experience, 
				  attack, defense, speed, maneuverability, vision, vision_range, balance, inventory
	`
	player := &models.PlayerResponse{}
	var positionJSON []byte
	err := repository.DB.QueryRow(query, req.UserID, req.Name, req.Image).Scan(
		&player.UserID,
		&player.Name,
		&player.Image,
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
	)
	if err != nil {
		http.Error(w, fmt.Sprintf("Ошибка создания игрока: %v", err), http.StatusInternalServerError)
		return
	}
	// Разбираем JSON из поля position
	if err := json.Unmarshal(positionJSON, &player.Position); err != nil {
		http.Error(w, fmt.Sprintf("Ошибка разбора позиции: %v", err), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(player)
}

// GetPlayerHandler – возвращает данные игрока по user_id
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

// GainExperienceHandler – увеличивает опыт игрока, проверяет порог для повышения уровня, сохраняет изменения.
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
		log.Printf("Игрок %d повышен до уровня %d", player.UserID, player.Level)
	}

	// Сохраняем изменения (без обновления updated_at)
	err = repository.UpdatePlayer(player)
	if err != nil {
		http.Error(w, fmt.Sprintf("Ошибка обновления игрока: %v", err), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(player)
}


// unbindPlayer удаляет игрока из матчинга по HTTP
func unbindPlayer(playerID int) {
    url := fmt.Sprintf("http://localhost:8002/matchmaking/player/%d", playerID)
    req, err := http.NewRequest(http.MethodDelete, url, nil)
    if err != nil {
        log.Printf("unbindPlayer: не удалось создать запрос: %v", err)
        return
    }
    resp, err := http.DefaultClient.Do(req)
    if err != nil {
        log.Printf("unbindPlayer: запрос завершился ошибкой: %v", err)
        return
    }
    defer resp.Body.Close()
    if resp.StatusCode != http.StatusOK {
        log.Printf("unbindPlayer: unexpected status %s", resp.Status)
    }
}