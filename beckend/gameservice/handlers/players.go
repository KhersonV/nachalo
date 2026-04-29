//====================================
// gameservice/handlers/players.go
//====================================

package handlers

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"

	"gameservice/repository"

	"github.com/gorilla/mux"
)

// CreatePlayerRequest – структура запроса для создания игрока
type CreatePlayerRequest struct {
	UserID        int    `json:"user_id"`
	Name          string `json:"name"`
	Image         string `json:"image"`
	CharacterType string `json:"character_type"`
}

type characterTemplate struct {
	CharacterType string
	Energy        int
	EnergyRegen   int
	MaxEnergy     int
	Health        int
	MaxHealth     int
	Attack        int
	Defense       int
	Mobility      int
	Agility       int
	SightRange    int
	IsRanged      bool
	AttackRange   int
}

const defaultCharacterType = "guardian"

var characterTemplates = map[string]characterTemplate{
	defaultCharacterType: {
		CharacterType: defaultCharacterType,
		Energy:        90, EnergyRegen: 10, MaxEnergy: 90,
		Health: 130, MaxHealth: 130,
		Attack: 9, Defense: 8,
		Mobility: 2, Agility: 2,
		SightRange: 2,
		IsRanged:   false, AttackRange: 1,
	},
	"berserker": {
		CharacterType: "berserker",
		Energy:        100, EnergyRegen: 10, MaxEnergy: 100,
		Health: 100, MaxHealth: 100,
		Attack: 14, Defense: 3,
		Mobility: 3, Agility: 2,
		SightRange: 2,
		IsRanged:   false, AttackRange: 1,
	},
	"ranger": {
		CharacterType: "ranger",
		Energy:        105, EnergyRegen: 11, MaxEnergy: 105,
		Health: 92, MaxHealth: 92,
		Attack: 11, Defense: 4,
		Mobility: 4, Agility: 4,
		SightRange: 2,
		IsRanged:   true, AttackRange: 2,
	},
	"mystic": {
		CharacterType: "mystic",
		Energy:        125, EnergyRegen: 13, MaxEnergy: 125,
		Health: 95, MaxHealth: 95,
		Attack: 10, Defense: 4,
		Mobility: 3, Agility: 3,
		SightRange: 2,
		IsRanged:   true, AttackRange: 3,
	},
}

func resolveCharacterTemplate(characterType string) characterTemplate {
	normalized := strings.TrimSpace(strings.ToLower(characterType))
	if template, ok := characterTemplates[normalized]; ok {
		return template
	}
	return characterTemplates[defaultCharacterType]
}

func resolveCharacterRegen(characterType string) int {
	template := resolveCharacterTemplate(characterType)
	if template.EnergyRegen > 0 {
		return template.EnergyRegen
	}
	return characterTemplates[defaultCharacterType].EnergyRegen
}

// CreatePlayerHandler – создает нового игрока.
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
		req.Image = "/guardian/guardian.webp"
	}

	player, err := repository.CreatePlayerProfile(repository.CreatePlayerProfileInput{
		UserID:        req.UserID,
		Name:          req.Name,
		Image:         req.Image,
		CharacterType: req.CharacterType,
		Balance:       0,
		Inventory:     "{}",
	})
	if err != nil {
		http.Error(w, fmt.Sprintf("Ошибка создания игрока: %v", err), http.StatusInternalServerError)
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

	if err := repository.AddPlayerExperience(id, req.Experience); err != nil {
		http.Error(w, fmt.Sprintf("Ошибка обновления игрока: %v", err), http.StatusInternalServerError)
		return
	}

	player, err = repository.GetPlayerByUserID(id)
	if err != nil {
		http.Error(w, fmt.Sprintf("Ошибка получения игрока: %v", err), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(player)
}

// unbindPlayer удаляет игрока из матчинга по HTTP
func unbindPlayer(playerID int) {
	baseURL := strings.TrimRight(os.Getenv("MATCHMAKING_URL"), "/")
	if baseURL == "" {
		baseURL = "http://matchmaking:8002"
	}
	url := fmt.Sprintf("%s/matchmaking/player/%d", baseURL, playerID)
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
