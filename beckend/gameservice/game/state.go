//==================================
// gameservice/game/state.go
//==================================

package game

import "encoding/json"

// PlayerResult — результат одного игрока
type PlayerResult struct {
	UserID            int             // ID игрока
	PlayerName        string          // имя игрока на момент завершения матча
	GroupID           int             // команда / группа игрока в матче
	CharacterType     string          // архетип персонажа на момент матча
	CharacterID       int             // id в player_characters (если известен)
	IsWinner          bool            // победил ли игрок в этом матче
	Survived          bool            // дожил ли игрок до финала матча
	Deaths            int             // количество смертей в матче
	ExpGained         int             // сколько опыта дать
	RewardsData       json.RawMessage // JSON-представление наград
	PlayerKills       int             // сколько игроков убил
	MonsterKills      int             // сколько монстров убил
	DamageTotal       int             // общий нанесённый урон
	DamageToPlayers   int             // урон, нанесённый другим игрокам
	DamageToMonsters  int             // урон, нанесённый монстрам
	DamageTaken       int             // урон, полученный игроком
	Placement         int             // итоговое место в матче
	InventorySnapshot json.RawMessage // финальный снимок инвентаря/лута
}

// MatchResults — итоги по всему матчу
type MatchResults struct {
	PlayerResults []PlayerResult
	WinnerID      int
	WinnerGroupID int
	WinnerUserIDs []int
}

// Reward — пример структуры для награды
type Reward struct {
	Type   string `json:"type"`
	Amount int    `json:"amount"`
}

// одно убийство или добивание монстра/игрока
type KillEvent struct {
	SourceType string `json:"sourceType,omitempty"` // player, monster, environment; absent in legacy data means unknown
	KillerID   int
	VictimType string // "player" или "monster"
	VictimID   int
	Damage     int // сколько урона привело к смерти
}

// одно нанесение урона (не обязательно смертельное)
type DamageEvent struct {
	SourceType string `json:"sourceType,omitempty"` // player, monster, environment; absent in legacy data means unknown
	DealerID   int
	TargetType string // "player" или "monster"
	TargetID   int
	Amount     int
}

const (
	ResultSourcePlayer      = "player"
	ResultSourceMonster     = "monster"
	ResultSourceEnvironment = "environment"
)

func ValidResultSourceType(sourceType string) bool {
	return sourceType == ResultSourcePlayer || sourceType == ResultSourceMonster || sourceType == ResultSourceEnvironment
}

// ResultEvents are persisted combat facts used for HEX rewards and placement.
type ResultEvents struct {
	DamageEvents  []DamageEvent `json:"damageEvents"`
	KillEvents    []KillEvent   `json:"killEvents"`
	DefeatedUsers []int         `json:"defeatedUsers"`
}
