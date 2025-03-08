
//==========================
// gameservice/game/match.go
//==========================

package game

import (
	"errors"
	"log"
	"sync"
)

// MatchState инкапсулирует состояние конкретного матча.
type MatchState struct {
	InstanceID     string
	ActivePlayerID int   // ID игрока, чей ход
	TurnOrder      []int // Очередность ходов (список ID игроков)
	mu             sync.Mutex
}

var (
	ErrNotYourTurn    = errors.New("not your turn")
	ErrPlayerNotFound = errors.New("player not found in turn order")
)

// EndTurn завершает ход текущего игрока и передаёт ход следующему игроку.
func (m *MatchState) EndTurn(currentPlayerID int) (int, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Проверяем, что запрос отправлен от игрока, у которого и должен быть ход.
	if currentPlayerID != m.ActivePlayerID {
		log.Printf("Ошибка: currentPlayerID (%d) != ActivePlayerID (%d). Невозможно завершить ход.", currentPlayerID, m.ActivePlayerID)
		return 0, ErrNotYourTurn
	}

	// Находим индекс текущего игрока в TurnOrder.
	index := -1
	for i, pid := range m.TurnOrder {
		if pid == currentPlayerID {
			index = i
			break
		}
	}
	if index == -1 {
		log.Printf("Ошибка: Игрок %d не найден в TurnOrder: %v", currentPlayerID, m.TurnOrder)
		return 0, ErrPlayerNotFound
	}

	// Определяем следующего игрока циклически.
	nextIndex := (index + 1) % len(m.TurnOrder)
	m.ActivePlayerID = m.TurnOrder[nextIndex]
	log.Printf("Ход завершён игроком %d, теперь ход у игрока %d", currentPlayerID, m.ActivePlayerID)
	return m.ActivePlayerID, nil
}

// --- Хранилище состояний матчей ---

var (
	MatchStates   = make(map[string]*MatchState)
	MatchStatesMu sync.Mutex
)

// CreateMatchState создаёт состояние матча и сохраняет его в хранилище.
func CreateMatchState(instanceID string, playerIDs []int) *MatchState {
	matchState := &MatchState{
		InstanceID:     instanceID,
		TurnOrder:      playerIDs,
		ActivePlayerID: 0,
	}
	if len(playerIDs) > 0 {
		matchState.ActivePlayerID = playerIDs[0]
	}

	MatchStatesMu.Lock()
	MatchStates[instanceID] = matchState
	MatchStatesMu.Unlock()

	return matchState
}

func GetMatchState(instanceID string) (*MatchState, bool) {
	MatchStatesMu.Lock()
	defer MatchStatesMu.Unlock()
	ms, ok := MatchStates[instanceID]
	return ms, ok
}