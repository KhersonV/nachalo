
//====================================
//gameservice/handlers/resources.go
//====================================


package handlers

import (
    "encoding/json"
    "fmt"
    "net/http"
    "time"
    "gameservice/repository"
)

// Структура для ресурса
type Resource struct {
    ID          int             `json:"id"`
    Type        string          `json:"type"`
    Description string          `json:"description"`
    Effect      json.RawMessage `json:"effect"`
    Image       string          `json:"image"`
}

func GetResourcesHandler(w http.ResponseWriter, r *http.Request) {
    query := `SELECT id, type, description, effect, image FROM resources;`
    rows, err := repository.DB.Query(query)
    if err != nil {
        http.Error(w, fmt.Sprintf("Ошибка запроса ресурсов: %v", err), http.StatusInternalServerError)
        return
    }
    defer rows.Close()

    var resources []Resource
    for rows.Next() {
        var rsc Resource
        if err := rows.Scan(&rsc.ID, &rsc.Type, &rsc.Description, &rsc.Effect, &rsc.Image); err != nil {
            http.Error(w, fmt.Sprintf("Ошибка чтения ресурсов: %v", err), http.StatusInternalServerError)
            return
        }
        resources = append(resources, rsc)
    }
    w.Header().Set("Content-Type", "application/json")
    json.NewEncoder(w).Encode(resources)
}

// Аналогично для монстров

type MonsterData struct {
	ID              int       `json:"id"`
	Name            string    `json:"name"`
	Type            string    `json:"type"`
	Health          int       `json:"health"`
	MaxHealth       int       `json:"max_health"`
	Attack          int       `json:"attack"`
	Defense         int       `json:"defense"`
	Speed           int       `json:"speed"`
	Maneuverability int       `json:"maneuverability"`
	Vision          int       `json:"vision"`
    Image           string    `json:"image"`
	CreatedAt       time.Time `json:"created_at"`
}

func GetMonstersHandler(w http.ResponseWriter, r *http.Request) {
	query := `SELECT id, name, type, health, max_health, attack, defense, speed, maneuverability, vision, image, created_at FROM monsters;`
	rows, err := repository.DB.Query(query)
	if err != nil {
		http.Error(w, fmt.Sprintf("Ошибка запроса монстров: %v", err), http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	var monsters []MonsterData
	for rows.Next() {
		var m MonsterData
		if err := rows.Scan(&m.ID, &m.Name, &m.Type, &m.Health, &m.MaxHealth, &m.Attack, &m.Defense, &m.Speed, &m.Maneuverability, &m.Vision, &m.Image, &m.CreatedAt); err != nil {
			http.Error(w, fmt.Sprintf("Ошибка чтения монстров: %v", err), http.StatusInternalServerError)
			return
		}
		monsters = append(monsters, m)
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(monsters)
}
