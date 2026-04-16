// ==============================
// /gameservice/models/match.go
// ==============================

package models

import (
	"encoding/json"
	"time"
)

type MatchParticipantSnapshot struct {
	UserID        int    `json:"userId"`
	Name          string `json:"name"`
	GroupID       int    `json:"groupId"`
	CharacterType string `json:"characterType,omitempty"`
	Placement     int    `json:"placement,omitempty"`
	IsWinner      bool   `json:"isWinner"`
	Survived      bool   `json:"survived"`
}

type MatchInfo struct {
	InstanceID      string                     `json:"instance_id"`
	Mode            string                     `json:"mode"`
	TeamsCount      int                        `json:"teams_count"`
	TotalPlayers    int                        `json:"total_players"`
	MapWidth        int                        `json:"map_width"`
	MapHeight       int                        `json:"map_height"`
	Map             json.RawMessage            `json:"map"`
	TurnOrder       json.RawMessage            `json:"turn_order,omitempty"`
	ActiveUserID    int                        `json:"active_user_id"`
	TurnNumber      int                        `json:"turn_number"`
	StartPositions  json.RawMessage            `json:"start_positions"`
	PortalPosition  json.RawMessage            `json:"portal_position"`
	WinnerID        int                        `json:"winner_id"`
	WinnerGroupID   int                        `json:"winner_group_id"`
	WinnerUserIDs   []int                      `json:"winner_user_ids,omitempty"`
	Participants    []MatchParticipantSnapshot `json:"participants,omitempty"`
	FinishedAt      time.Time                  `json:"finished_at"`
	QuestArtifactID int                        `json:"quest_artifact_id"`
}

// PlayerMatchStat соответствует одной записи в match_player_stats
type PlayerMatchStat struct {
	InstanceID        string          `db:"instance_id" json:"instanceId"`
	UserID            int             `db:"user_id" json:"userId"`
	PlayerName        string          `db:"player_name" json:"playerName"`
	GroupID           int             `db:"group_id" json:"groupId"`
	CharacterType     string          `db:"character_type" json:"characterType"`
	IsWinner          bool            `db:"is_winner" json:"isWinner"`
	Survived          bool            `db:"survived" json:"survived"`
	Deaths            int             `db:"deaths" json:"deaths"`
	ExpGained         int             `db:"exp_gained" json:"expGained"`
	Rewards           json.RawMessage `db:"rewards" json:"rewards"` // JSONB
	PlayerKills       int             `db:"player_kills" json:"playerKills"`
	MonsterKills      int             `db:"monster_kills" json:"monsterKills"`
	DamageTotal       int             `db:"damage_total" json:"damageTotal"`
	DamageToPlayers   int             `db:"damage_to_players" json:"damageToPlayers"`
	DamageToMonsters  int             `db:"damage_to_monsters" json:"damageToMonsters"`
	DamageTaken       int             `db:"damage_taken" json:"damageTaken"`
	Placement         int             `db:"placement" json:"placement"`
	InventorySnapshot json.RawMessage `db:"inventory_snapshot" json:"inventorySnapshot"`
}
