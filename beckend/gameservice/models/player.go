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
	Speed           int    `json:"speed"`
	Maneuverability int    `json:"maneuverability"`
	Vision          int    `json:"vision"`
	VisionRange     int    `json:"visionRange"`
	Balance         int    `json:"balance"`
	GroupID         int    `json:"group_id"`
	Inventory       string `json:"inventory"`
}
