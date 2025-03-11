

// ==============================
// /gameservice/models/match.go
// ==============================

package models

import (
    "encoding/json"
    "time"
)

type MatchInfo struct {
    InstanceID   string          `json:"instance_id"`
    Mode         string          `json:"mode"`
    TeamsCount   int             `json:"teams_count"`
    TotalPlayers int             `json:"total_players"`
    MapWidth     int             `json:"map_width"`
    MapHeight    int             `json:"map_height"`
    Map          json.RawMessage `json:"map"`
    CreatedAt    time.Time       `json:"created_at"`
    TurnNumber   int             `json:"turn_number"`
}
