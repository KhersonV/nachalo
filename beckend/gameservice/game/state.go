//==================================
// gameservice/game/state.go
//==================================

package game

import (
	"encoding/json"
	"sync"
)

// PlayerResult — результат одного игрока
type PlayerResult struct {
	UserID           int             // ID игрока
	IsWinner         bool            // победил ли игрок в этом матче
	ExpGained        int             // сколько опыта дать
	RewardsData      json.RawMessage // JSON-представление наград
	PlayerKills      int             // сколько игроков убил
	MonsterKills     int             // сколько монстров убил
	DamageTotal      int             // общий нанесённый урон
	DamageToPlayers  int             // урон, нанесённый другим игрокам
	DamageToMonsters int             // урон, нанесённый монстрам
}

// MatchResults — итоги по всему матчу
type MatchResults struct {
	PlayerResults []PlayerResult
	WinnerID      int
	WinnerGroupID int
	WinnerUserIDs []int
}

type MonsterState struct {
	ID                int
	MonsterInstanceID int
	Health            int
	MaxHealth         int
	Attack            int
	Defense           int
	Speed             int
	Maneuverability   int
	Vision            int
	Image             string
}

// Reward — пример структуры для награды
type Reward struct {
	Type   string `json:"type"`
	Amount int    `json:"amount"`
}

// одно убийство или добивание монстра/игрока
type KillEvent struct {
	KillerID   int
	VictimType string // "player" или "monster"
	Damage     int    // сколько урона привело к смерти
}

// одно нанесение урона (не обязательно смертельное)
type DamageEvent struct {
	DealerID   int
	TargetType string // "player" или "monster"
	Amount     int
}

type ArmorBreakState struct {
	Stacks         int
	RemainingTurns int
}

// MatchState инкапсулирует состояние конкретного матча.
type MatchState struct {
	InstanceID     string
	ActiveUserID   int   // ID игрока, чей сейчас ход
	TurnOrder      []int // Очередность ходов (список ID игроков)
	TurnNumber     int   // Номер текущего круга
	KillEvents     []KillEvent
	DamageEvents   []DamageEvent
	Monsters       map[int]*MonsterState
	ArmorBreak     map[string]ArmorBreakState
	BerserkerFury  map[int]int
	MysticDrains   map[string]int
	CombatSequence uint64
	mu             sync.Mutex
}

// глобальная ма́па: instanceID → MatchState
var (
	MatchStates   = make(map[string]*MatchState)
	MatchStatesMu sync.RWMutex
)
