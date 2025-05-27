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
	ActiveUserID   int             `json:"active_user_id"`
	TurnNumber     int             `json:"turn_number"`
	StartPositions json.RawMessage `json:"start_positions"`
	PortalPosition json.RawMessage `json:"portal_position"`
	WinnerID       int             `json:"winner_id"`
	WinnerGroupID  int             `json:"winner_group_id"`
}

// PlayerMatchStat соответствует одной записи в match_player_stats
type PlayerMatchStat struct {
	InstanceID       string          `db:"instance_id" json:"instanceId"`
	UserID           int             `db:"user_id" json:"userId"`
	ExpGained        int             `db:"exp_gained" json:"expGained"`
	Rewards          json.RawMessage `db:"rewards" json:"rewards"` // JSONB
	PlayerKills      int             `db:"player_kills" json:"playerKills"`
	MonsterKills     int             `db:"monster_kills" json:"monsterKills"`
	DamageTotal      int             `db:"damage_total" json:"damageTotal"`
	DamageToPlayers  int             `db:"damage_to_players" json:"damageToPlayers"`
	DamageToMonsters int             `db:"damage_to_monsters" json:"damageToMonsters"`
}
