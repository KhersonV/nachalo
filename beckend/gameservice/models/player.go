// ==============================
// /gameservice/models/player.go
// ==============================

package models

type WinnerInfo struct {
	Type string `json:"type"`
	ID   int    `json:"id"`
}

// PlayerResponse – структура для ответа, приведённая к единому виду (как у клиента)
type PlayerResponse struct {
	UserID   int    `json:"user_id"`
	Name     string `json:"name"`
	Image    string `json:"image"`
	CharacterType string `json:"characterType"`
	Position struct {
		X int `json:"x"`
		Y int `json:"y"`
	} `json:"position"`
	Energy          int    `json:"energy"`
	MaxEnergy       int    `json:"maxEnergy"`
	Health          int    `json:"health"`
	MaxHealth       int    `json:"maxHealth"`
	Level           int    `json:"level"`
	Experience      int    `json:"experience"`
	MaxExperience   int    `json:"maxExperience"`
	Attack          int    `json:"attack"`
	Defense         int    `json:"defense"`
	Mobility        int    `json:"mobility"`
	Agility         int    `json:"agility"`
	SightRange      int    `json:"sightRange"`
	IsRanged        bool   `json:"isRanged"`
	AttackRange     int    `json:"attackRange"`
	Balance         int    `json:"balance"`
	GroupID         int    `json:"group_id"`
	Inventory       string `json:"inventory"`
}
