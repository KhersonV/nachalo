// ==============================================
// gameservice/handlers/combat_handler_test.go
// ==============================================
package handlers

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"gameservice/game"
	"gameservice/models"
	"gameservice/repository"
)

func TestUniversalAttackHandler(t *testing.T) {
	// 1) Подменяем все зависимости через Combat
	Combat = CombatDeps{
		UpdatePlayer: func(_ string, _ *models.PlayerResponse) error { return nil },
		GetPlayer: func(_ string, userID int) (*models.PlayerResponse, error) {
			return &models.PlayerResponse{
				UserID: userID, Attack: 5, Defense: 1, Health: 10,
			}, nil
		},
		GetMonster: func(_ string, monsterID int) (*repository.MatchMonster, error) {
			return &repository.MatchMonster{
				MonsterInstanceID: monsterID, Attack: 3, Defense: 1, Health: 8,
			}, nil
		},
		UpdateMonsterHealth: func(_ string, _, _ int) error { return nil },
		DeleteMonster:       func(_ string, _ int) error { return nil },
		MarkPlayerDead:      func(_ string, _ int) error { return nil },
		ClearPlayerFlag:     func(_ string, _ repository.Position) error { return nil },
		UpdateTurn:          func(_ string, _, _ int) error { return nil },
		Finalize:            func(_ string) error { return nil },
		LoadGameState: func(_ string) (*game.MatchState, bool) {
			return &game.MatchState{
				InstanceID:   "m1",
				TurnOrder:    []int{1},
				ActiveUserID: 1,
				TurnNumber:   1,
			}, true
		},
		LoadMap: func(_ string) ([]game.FullCell, error) {
			return []game.FullCell{{X: 1, Y: 1, TileCode: 48}}, nil
		},
		SaveMap: func(_ string, _ []game.FullCell) error {
			return nil
		},
	}
	defer RestoreDefaults()

	// 2) Формируем запрос
	payload := map[string]interface{}{
		"instance_id":   "m1",
		"attacker_type": "player",
		"attacker_id":   1,
		"target_type":   "monster",
		"target_id":     42,
	}
	body, _ := json.Marshal(payload)
	req := httptest.NewRequest("POST", "/attack", bytes.NewReader(body))
	rec := httptest.NewRecorder()

	// 3) Вызываем хендлер
	UniversalAttackHandler(rec, req)

	// 4) Проверяем HTTP-статус
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 OK, got %d: %s", rec.Code, rec.Body.String())
	}

	// 5) Проверяем тело ответа
	var resp map[string]interface{}
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("invalid JSON response: %v", err)
	}
	for _, key := range []string{
		"damage_to_target", "new_target_hp", "counter_damage", "new_attacker_hp",
	} {
		if _, ok := resp[key]; !ok {
			t.Errorf("response missing field %q", key)
		}
	}
}

func TestBuildCombatExchangePayload_UsesDeterministicSequenceAndSteps(t *testing.T) {
	const instanceID = "exchange-match"

	Combat = CombatDeps{
		LoadGameState: func(_ string) (*game.MatchState, bool) {
			return &game.MatchState{
				InstanceID: instanceID,
				TurnNumber: 3,
			}, true
		},
	}
	defer RestoreDefaults()

	payload := buildCombatExchangePayload(
		instanceID,
		"player",
		11,
		"monster",
		44,
		stats{CharacterType: "mystic"},
		attackModeRanged,
		attackResult{Damage: 7, NewHealth: 0},
		attackResult{Damage: 0, NewHealth: 10},
	)

	if payload.ExchangeID != "exchange-match:3:1" {
		t.Fatalf("expected deterministic exchange id, got %q", payload.ExchangeID)
	}
	if payload.AttackStyle != AttackStyleMagic {
		t.Fatalf("expected magic attack style, got %q", payload.AttackStyle)
	}
	if len(payload.Steps) != 2 {
		t.Fatalf("expected hit + death, got %d steps", len(payload.Steps))
	}
	if payload.Steps[0].Kind != "hit" {
		t.Fatalf("expected first step to be hit, got %q", payload.Steps[0].Kind)
	}
	if payload.Steps[0].Target.Type != CombatActorMonster {
		t.Fatalf("expected monster target, got %q", payload.Steps[0].Target.Type)
	}
	if payload.Steps[1].Kind != "death" {
		t.Fatalf("expected second step to be death, got %q", payload.Steps[1].Kind)
	}
}

func TestBuildCombatExchangePayload_AddsCounterAndAttackerDeath(t *testing.T) {
	Combat = CombatDeps{
		LoadGameState: func(_ string) (*game.MatchState, bool) {
			return &game.MatchState{
				InstanceID: "counter-exchange",
				TurnNumber: 4,
			}, true
		},
	}
	defer RestoreDefaults()

	payload := buildCombatExchangePayload(
		"counter-exchange",
		"player",
		10,
		"player",
		20,
		stats{CharacterType: "guardian"},
		attackModeMelee,
		attackResult{Damage: 5, NewHealth: 6},
		attackResult{Damage: 8, NewHealth: 0},
	)

	if len(payload.Steps) != 3 {
		t.Fatalf("expected hit + counter + death, got %d steps", len(payload.Steps))
	}
	if payload.Steps[1].Kind != "counter" {
		t.Fatalf("expected second step to be counter, got %q", payload.Steps[1].Kind)
	}
	if payload.Steps[1].Target.ID != 10 {
		t.Fatalf("expected counter target to be attacker, got %d", payload.Steps[1].Target.ID)
	}
	if payload.Steps[2].Kind != "death" || payload.Steps[2].Target.ID != 10 {
		t.Fatalf("expected attacker death step, got %+v", payload.Steps[2])
	}
}

func TestDoCounterattackWithEnergy_RecordsDefenderDamageEvent(t *testing.T) {
	const instanceID = "counter-match"
	const attackerID = 1
	const defenderID = 2

	Combat = CombatDeps{
		UpdatePlayer: func(_ string, _ *models.PlayerResponse) error { return nil },
		GetPlayer: func(_ string, userID int) (*models.PlayerResponse, error) {
			return &models.PlayerResponse{
				UserID: userID,
				Energy: 10,
				Health: 10,
			}, nil
		},
		UpdateMonsterHealth: func(_ string, _, _ int) error { return nil },
		MarkPlayerDead:      func(_ string, _ int) error { return nil },
		ClearPlayerFlag:     func(_ string, _ repository.Position) error { return nil },
		UpdateTurn:          func(_ string, _, _ int) error { return nil },
		Finalize:            func(_ string) error { return nil },
		LoadMap:             func(_ string) ([]game.FullCell, error) { return nil, nil },
		SaveMap:             func(_ string, _ []game.FullCell) error { return nil },
	}
	defer RestoreDefaults()

	game.MatchStatesMu.Lock()
	game.MatchStates[instanceID] = &game.MatchState{InstanceID: instanceID, TurnOrder: []int{attackerID, defenderID}, ActiveUserID: attackerID, TurnNumber: 1}
	game.MatchStatesMu.Unlock()
	defer func() {
		game.MatchStatesMu.Lock()
		delete(game.MatchStates, instanceID)
		game.MatchStatesMu.Unlock()
	}()

	result, err := doCounterattackWithEnergy(
		instanceID,
		"player", attackerID,
		"player", defenderID,
		stats{Attack: 5, Defense: 1, Health: 10},
		stats{Attack: 7, Defense: 1, Health: 10},
		true,
		true,
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Damage != 6 {
		t.Fatalf("expected counter damage 6, got %d", result.Damage)
	}

	ms, ok := game.GetMatchState(instanceID)
	if !ok {
		t.Fatal("expected match state to exist")
	}
	if len(ms.DamageEvents) != 1 {
		t.Fatalf("expected 1 damage event, got %d", len(ms.DamageEvents))
	}
	got := ms.DamageEvents[0]
	if got.DealerID != defenderID {
		t.Fatalf("expected defender to be dealer, got %d", got.DealerID)
	}
	if got.TargetType != "player" {
		t.Fatalf("expected target type player, got %q", got.TargetType)
	}
	if got.Amount != 6 {
		t.Fatalf("expected counter damage amount 6, got %d", got.Amount)
	}
	if len(ms.KillEvents) != 0 {
		t.Fatalf("expected no kill events for non-lethal counterattack, got %d", len(ms.KillEvents))
	}
}

func TestDoCounterattackWithEnergy_SkipsOnRangedAttack(t *testing.T) {
	result, err := doCounterattackWithEnergy(
		"range-match",
		"player", 1,
		"player", 2,
		stats{Attack: 5, Defense: 1, Health: 10},
		stats{Attack: 7, Defense: 1, Health: 10},
		true,
		false,
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Damage != 0 {
		t.Fatalf("expected no counter damage for ranged attack, got %d", result.Damage)
	}
	if result.NewHealth != 10 {
		t.Fatalf("expected attacker health unchanged, got %d", result.NewHealth)
	}
}
