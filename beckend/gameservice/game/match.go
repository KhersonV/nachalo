//==================================
// gameservice/game/match.go
//==================================

package game

import (
	"errors"
	"fmt"
	"log"
	"strings"
)

var (
	ErrNoPlayers        = errors.New("no players in turn order")
	ErrNotYourTurn      = errors.New("not your turn")
	ErrPlayerNotInOrder = errors.New("player not found in turn order")
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

	// Сохраним старый порядок, чтобы знать, на каком месте был умерший
	oldOrder := append([]int(nil), m.TurnOrder...)

	// Фильтруем очередь
	newOrder := make([]int, 0, len(oldOrder))
	for _, id := range oldOrder {
		if id != userID {
			newOrder = append(newOrder, id)
		}
	}
	m.TurnOrder = newOrder

	if len(newOrder) == 0 {
		m.ActiveUserID = 0
		return
	}

	// Если умерший был активным, выбираем следующего по индексу без инкремента turnNumber
	if m.ActiveUserID == userID {
		// Найдём позицию погибшего в oldOrder
		removedIdx := 0
		for i, id := range oldOrder {
			if id == userID {
				removedIdx = i
				break
			}
		}
		// Следующий игрок — тот, кто оказался на той же позиции (modulo новый размер)
		nextIdx := removedIdx % len(newOrder)
		m.ActiveUserID = newOrder[nextIdx]
		log.Printf(
			"RemovePlayer: %d died, new active %d (turn %d)",
			userID, m.ActiveUserID, m.TurnNumber,
		)
	}
}

// CreateMatchState создаёт новую игру с указанными игроками.
func CreateMatchState(instanceID string, playerIDs []int) *MatchState {
	ms := &MatchState{
		InstanceID:    instanceID,
		TurnOrder:     append([]int(nil), playerIDs...), // копия слайса
		TurnNumber:    1,
		ActiveUserID:  0,
		ArmorBreak:    make(map[string]ArmorBreakState),
		BerserkerFury: make(map[int]int),
		MysticDrains:  make(map[string]int),
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

func DeleteMatchStateInMemory(instanceID string) {
	MatchStatesMu.Lock()
	defer MatchStatesMu.Unlock()
	delete(MatchStates, instanceID)
}

func (m *MatchState) NextCombatExchangeMeta() (turn int, seq uint64) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.CombatSequence++
	return m.TurnNumber, m.CombatSequence
}

// RecordDamageEvent добавляет в память факт нанесённого урона.
func (m *MatchState) RecordDamageEvent(dealerID int, targetType string, amount int) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.DamageEvents = append(m.DamageEvents, DamageEvent{
		DealerID:   dealerID,
		TargetType: targetType,
		Amount:     amount,
	})
}

// RecordKillEvent добавляет в память факт убийства (смерти).
func (m *MatchState) RecordKillEvent(killerID int, victimType string, damage int) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.KillEvents = append(m.KillEvents, KillEvent{
		KillerID:   killerID,
		VictimType: victimType,
		Damage:     damage,
	})
}

func combatTargetKey(targetType string, targetID int) string {
	return fmt.Sprintf("%s:%d", targetType, targetID)
}

func (m *MatchState) ensureCombatStateLocked() {
	if m.ArmorBreak == nil {
		m.ArmorBreak = make(map[string]ArmorBreakState)
	}
	if m.BerserkerFury == nil {
		m.BerserkerFury = make(map[int]int)
	}
	if m.MysticDrains == nil {
		m.MysticDrains = make(map[string]int)
	}
}

func (m *MatchState) decrementArmorBreakLocked(targetKey string) {
	state, ok := m.ArmorBreak[targetKey]
	if !ok {
		return
	}
	state.RemainingTurns--
	if state.RemainingTurns <= 0 || state.Stacks <= 0 {
		delete(m.ArmorBreak, targetKey)
		return
	}
	m.ArmorBreak[targetKey] = state
}

func (m *MatchState) AdvanceTurnCombatState(nextUserID int, roundAdvanced bool) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.ensureCombatStateLocked()
	clear(m.BerserkerFury)
	clear(m.MysticDrains)

	if nextUserID > 0 {
		m.decrementArmorBreakLocked(combatTargetKey("player", nextUserID))
	}
	if !roundAdvanced {
		return
	}
	for key := range m.ArmorBreak {
		if strings.HasPrefix(key, "monster:") {
			m.decrementArmorBreakLocked(key)
		}
	}
}

func (m *MatchState) GetArmorBreakState(targetType string, targetID int) ArmorBreakState {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.ensureCombatStateLocked()
	return m.ArmorBreak[combatTargetKey(targetType, targetID)]
}

func (m *MatchState) ApplyArmorBreak(targetType string, targetID int, maxStacks int, durationTurns int) ArmorBreakState {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.ensureCombatStateLocked()
	key := combatTargetKey(targetType, targetID)
	state := m.ArmorBreak[key]
	if state.Stacks < maxStacks {
		state.Stacks++
	}
	if durationTurns > 0 {
		state.RemainingTurns = durationTurns
	}
	m.ArmorBreak[key] = state
	return state
}

func (m *MatchState) TryUseBerserkerFury(userID int, limit int) bool {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.ensureCombatStateLocked()
	if limit <= 0 {
		return false
	}
	if m.BerserkerFury[userID] >= limit {
		return false
	}
	m.BerserkerFury[userID]++
	return true
}

func (m *MatchState) TryUseMysticDrain(attackerID int, targetID int, perTargetLimit int) bool {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.ensureCombatStateLocked()
	if perTargetLimit <= 0 {
		return false
	}
	key := fmt.Sprintf("%d:%d", attackerID, targetID)
	if m.MysticDrains[key] >= perTargetLimit {
		return false
	}
	m.MysticDrains[key]++
	return true
}
