//=================================
// gameservice/game/match_test.go
//=================================

package game

import (
	"testing"
)

func TestEndTurn(t *testing.T) {
	ms := &MatchState{
		TurnOrder:    []int{10, 20, 30},
		ActiveUserID: 10,
		TurnNumber:   1,
	}
	next, err := ms.EndTurn(10)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if next != 20 {
		t.Errorf("expected next 20, got %d", next)
	}
	// завершаем круг
	ms.ActiveUserID = 30
	ms.TurnNumber = 5
	next2, err2 := ms.EndTurn(30)
	if err2 != nil {
		t.Fatalf("unexpected error: %v", err2)
	}
	if next2 != 10 {
		t.Errorf("expected wrap to 10, got %d", next2)
	}
	if ms.TurnNumber != 6 {
		t.Errorf("expected turn increment to 6, got %d", ms.TurnNumber)
	}
}

func TestRemovePlayerFromTurnOrder(t *testing.T) {
	ms := &MatchState{
		TurnOrder:            []int{1, 2, 3},
		ActiveUserID:         2,
		TurnNumber:           1,
		GuardianAuraPressure: map[int]GuardianAuraPressureState{2: {AccumulatedExtraMoveCost: 3, LastSourceUserID: 9}},
	}
	ms.RemovePlayerFromTurnOrder(2)
	// очередь должна стать [1,3], активный уже переключился на 3
	if len(ms.TurnOrder) != 2 || ms.TurnOrder[0] != 1 || ms.TurnOrder[1] != 3 {
		t.Errorf("unexpected TurnOrder: %v", ms.TurnOrder)
	}
	if ms.ActiveUserID != 3 {
		t.Errorf("expected ActiveUserID=3, got %d", ms.ActiveUserID)
	}
	if _, ok := ms.GuardianAuraPressure[2]; ok {
		t.Fatal("expected guardian aura pressure to be cleared for removed player")
	}
}

func TestEndTurn_NoPlayers(t *testing.T) {
	ms := &MatchState{TurnOrder: []int{}}
	if _, err := ms.EndTurn(1); err != ErrNoPlayers {
		t.Errorf("expected ErrNoPlayers, got %v", err)
	}
}

func TestAdvanceTurnCombatState_ResetsTurnScopedEffectsAndExpiresArmorBreak(t *testing.T) {
	ms := &MatchState{
		ArmorBreak: map[string]ArmorBreakState{
			"player:2":  {Stacks: 2, RemainingTurns: 1},
			"monster:9": {Stacks: 1, RemainingTurns: 1},
		},
		BerserkerFury: map[int]int{1: 2},
		MysticDrains:  map[string]int{"1:2": 3},
	}

	ms.AdvanceTurnCombatState(2, true)

	if len(ms.BerserkerFury) != 0 {
		t.Fatalf("expected berserker fury counters to reset, got %+v", ms.BerserkerFury)
	}
	if len(ms.MysticDrains) != 0 {
		t.Fatalf("expected mystic drains to reset, got %+v", ms.MysticDrains)
	}
	if _, ok := ms.ArmorBreak["player:2"]; ok {
		t.Fatal("expected player armor break to expire on target turn")
	}
	if _, ok := ms.ArmorBreak["monster:9"]; ok {
		t.Fatal("expected monster armor break to expire on round advance")
	}
}

func TestTryUseTurnScopedClassLimits(t *testing.T) {
	ms := &MatchState{}

	for i := 0; i < 5; i++ {
		if !ms.TryUseBerserkerFury(10, 0) {
			t.Fatalf("expected unlimited berserker fury proc %d to succeed", i+1)
		}
	}
	if ms.BerserkerFury[10] != 5 {
		t.Fatalf("expected berserker fury counter to track unlimited procs, got %d", ms.BerserkerFury[10])
	}

	for i := 0; i < 3; i++ {
		if !ms.TryUseMysticDrain(10, 20, 3) {
			t.Fatalf("expected mystic drain proc %d to succeed", i+1)
		}
	}
	if ms.TryUseMysticDrain(10, 20, 3) {
		t.Fatal("expected fourth mystic drain proc on same target to fail")
	}
}

func TestGuardianAuraPressure_AccumulatesUpdatesSourceAndResets(t *testing.T) {
	ms := &MatchState{}

	state := ms.AccumulateGuardianAuraPressure(7, 11, 2)
	if state.AccumulatedExtraMoveCost != 2 {
		t.Fatalf("expected accumulated extra cost 2, got %d", state.AccumulatedExtraMoveCost)
	}
	if state.LastSourceUserID != 11 {
		t.Fatalf("expected source guardian 11, got %d", state.LastSourceUserID)
	}

	state = ms.AccumulateGuardianAuraPressure(7, 15, 3)
	if state.AccumulatedExtraMoveCost != 5 {
		t.Fatalf("expected accumulated extra cost 5, got %d", state.AccumulatedExtraMoveCost)
	}
	if state.LastSourceUserID != 15 {
		t.Fatalf("expected latest source guardian 15, got %d", state.LastSourceUserID)
	}

	if got := ms.GetGuardianAuraPressure(7); got.AccumulatedExtraMoveCost != 5 || got.LastSourceUserID != 15 {
		t.Fatalf("unexpected stored guardian aura pressure: %+v", got)
	}

	ms.ResetGuardianAuraPressure(7)
	if got := ms.GetGuardianAuraPressure(7); got.AccumulatedExtraMoveCost != 0 || got.LastSourceUserID != 0 {
		t.Fatalf("expected guardian aura pressure to reset, got %+v", got)
	}
}
