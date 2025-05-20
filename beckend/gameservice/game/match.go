
//==================================
// gameservice/game/match.go
//==================================

package game

import (
	"errors"
	"log"
	"sync"
)

// MatchState инкапсулирует состояние конкретного матча.
type MatchState struct {
	InstanceID   string
	ActiveUserID int   // ID игрока, чей сейчас ход
	TurnOrder    []int // Очередность ходов (список ID игроков)
	TurnNumber   int   // Номер текущего круга
	mu           sync.Mutex
}

var (
	ErrNoPlayers         = errors.New("no players in turn order")
	ErrNotYourTurn       = errors.New("not your turn")
	ErrPlayerNotInOrder  = errors.New("player not found in turn order")
)

// EndTurn завершает ход: берём следующего живого игрока по кругу.
func (m *MatchState) EndTurn(currentPlayerID int) (int, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	n := len(m.TurnOrder)
	if n == 0 {
		return 0, ErrNoPlayers
	}

	// Найти текущий индекс, если есть
	startIdx := 0
	for i, id := range m.TurnOrder {
		if id == currentPlayerID {
			startIdx = i
			break
		}
	}

	// Ищем следующего — всегда берём (startIdx+1)%n
	nextIdx := (startIdx + 1) % n
	// Если мы обернулись назад в начало — это новый круг
	if nextIdx <= startIdx {
		m.TurnNumber++
	}

	m.ActiveUserID = m.TurnOrder[nextIdx]
	log.Printf(
		"EndTurn: %d → %d (turn %d)",
		currentPlayerID, m.ActiveUserID, m.TurnNumber,
	)
	return m.ActiveUserID, nil
}

// RemovePlayerFromTurnOrder чистит очередь от погибшего игрока.
// Если погибший был среди TurnOrder, просто вырезаем его.
// Если он совпадал с ActiveUserID, передаём ход сразу следующему.
func (m *MatchState) RemovePlayerFromTurnOrder(userID int) {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Фильтруем очередь
	newOrder := make([]int, 0, len(m.TurnOrder))
	for _, id := range m.TurnOrder {
		if id != userID {
			newOrder = append(newOrder, id)
		}
	}
	m.TurnOrder = newOrder

	// Если очередь опустела — ничего больше не делаем
	if len(m.TurnOrder) == 0 {
		m.ActiveUserID = 0
		return
	}

	// Если погибший был активным, передаём ход следующему в новом списке
	if m.ActiveUserID == userID {
		// выбираем новый ActiveUserID из начала TurnOrder
		m.ActiveUserID = m.TurnOrder[0]
		m.TurnNumber++
		log.Printf(
			"RemovePlayer: %d died, new active %d (turn %d)",
			userID, m.ActiveUserID, m.TurnNumber,
		)
	}
}

// --- Хранилище состояний матчей ---

var (
	MatchStates   = make(map[string]*MatchState)
	MatchStatesMu sync.Mutex
)

// CreateMatchState создаёт новую игру с указанными игроками.
func CreateMatchState(instanceID string, playerIDs []int) *MatchState {
	ms := &MatchState{
		InstanceID:   instanceID,
		TurnOrder:    append([]int(nil), playerIDs...), // копия слайса
		TurnNumber:   1,
		ActiveUserID: 0,
	}
	if len(playerIDs) > 0 {
		ms.ActiveUserID = playerIDs[0]
	}
	MatchStatesMu.Lock()
	MatchStates[instanceID] = ms
	MatchStatesMu.Unlock()
	return ms
}

// GetMatchState возвращает текущее состояние матча.
func GetMatchState(instanceID string) (*MatchState, bool) {
	MatchStatesMu.Lock()
	defer MatchStatesMu.Unlock()
	ms, ok := MatchStates[instanceID]
	return ms, ok
}
