
// ==============================
// /gameservice/models/player.go
// ==============================

package models

import (
    "database/sql"
    "time"
)

type Player struct {
    ID              int            `json:"id"`
    UserID          int            `json:"user_id"`
    Name            string         `json:"name"`
    Image           string         `json:"image"`
    ColorClass      string         `json:"color_class"`
    PosX            int            `json:"pos_x"`
    PosY            int            `json:"pos_y"`
    Energy          int            `json:"energy"`
    MaxEnergy       int            `json:"max_energy"`
    Health          int            `json:"health"`
    MaxHealth       int            `json:"max_health"`
    Level           int            `json:"level"`
    Experience      int            `json:"experience"`
    MaxExperience   int            `json:"max_experience"`
    Attack          int            `json:"attack"`
    Defense         int            `json:"defense"`
    Speed           int            `json:"speed"`
    Maneuverability int            `json:"maneuverability"`
    Vision          int            `json:"vision"`
    VisionRange     int            `json:"vision_range"`
    Balance         int            `json:"balance"`
    Inventory       string         `json:"inventory"`
    InstanceID      sql.NullString `json:"instance_id"`
    UpdatedAt       time.Time      `json:"updated_at"`
}
