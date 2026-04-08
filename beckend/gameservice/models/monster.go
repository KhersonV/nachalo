
// ==============================
// /gameservice/models/monster.go
// ==============================

package models

import "time"

type Monster struct {
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
