

// ==============================
// /gameservice/models/match.go
// ==============================

package models

import (
	"encoding/json"
)

type MatchInfo struct {
	InstanceID     string          `json:"instance_id"`
	Mode           string          `json:"mode"`
	TeamsCount     int             `json:"teams_count"`
	TotalPlayers   int             `json:"total_players"`
	MapWidth       int             `json:"map_width"`
	MapHeight      int             `json:"map_height"`
	Map            json.RawMessage `json:"map"`
	ActiveUserID int             `json:"active_user_id"`
	TurnNumber     int             `json:"turn_number"`
	StartPositions  json.RawMessage `json:"start_positions"`
	PortalPosition  json.RawMessage `json:"portal_position"`
}
