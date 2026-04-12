// ==============================================
// gameservice/handlers/combat_handler_test.go
// ==============================================
package handlers

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"slices"
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
				UserID: userID,
				Attack: 5, Defense: 1, Health: 10, MaxHealth: 10, Energy: 12,
				Position: struct {
					X int `json:"x"`
					Y int `json:"y"`
				}{X: 1, Y: 1},
			}, nil
		},
		GetMonster: func(_ string, monsterID int) (*repository.MatchMonster, error) {
			return &repository.MatchMonster{
				MonsterInstanceID: monsterID,
				Attack:            3,
				Defense:           1,
				Health:            8,
				MaxHealth:         8,
				X:                 2,
				Y:                 1,
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
		[]CombatStep{
			{
				Kind:          "hit",
				Source:        &CombatTargetRef{ID: 11, Type: CombatActorPlayer},
				Target:        CombatTargetRef{ID: 44, Type: CombatActorMonster},
				Damage:        7,
				TargetHPAfter: 0,
			},
			{
				Kind:   "death",
				Target: CombatTargetRef{ID: 44, Type: CombatActorMonster},
			},
		},
		nil,
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
		[]CombatStep{
			{
				Kind:          "hit",
				Source:        &CombatTargetRef{ID: 10, Type: CombatActorPlayer},
				Target:        CombatTargetRef{ID: 20, Type: CombatActorPlayer},
				Damage:        5,
				TargetHPAfter: 6,
			},
			{
				Kind:          "counter",
				Source:        &CombatTargetRef{ID: 20, Type: CombatActorPlayer},
				Target:        CombatTargetRef{ID: 10, Type: CombatActorPlayer},
				Damage:        8,
				TargetHPAfter: 0,
			},
			{
				Kind:   "death",
				Target: CombatTargetRef{ID: 10, Type: CombatActorPlayer},
			},
		},
		nil,
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

func TestSaveTargetHealth_NonLethalMonsterDamage_DoesNotBroadcastUpdateCell(t *testing.T) {
	const (
		instanceID = "m1"
		monsterID  = 42
	)

	var savedCells []game.FullCell

	Combat = CombatDeps{
		UpdateMonsterHealth: func(_ string, gotMonsterID, gotHP int) error {
			if gotMonsterID != monsterID {
				t.Fatalf("expected monster id %d, got %d", monsterID, gotMonsterID)
			}
			if gotHP != 5 {
				t.Fatalf("expected updated hp 5, got %d", gotHP)
			}
			return nil
		},
		GetMonster: func(_ string, gotMonsterID int) (*repository.MatchMonster, error) {
			if gotMonsterID != monsterID {
				t.Fatalf("expected monster id %d, got %d", monsterID, gotMonsterID)
			}
			return &repository.MatchMonster{
				MonsterInstanceID: monsterID,
				X:                 3,
				Y:                 4,
				Health:            5,
			}, nil
		},
		LoadMap: func(_ string) ([]game.FullCell, error) {
			return []game.FullCell{
				{
					X: 3,
					Y: 4,
					Monster: &game.MonsterData{
						ID:           7,
						DBInstanceID: monsterID,
						Health:       10,
					},
				},
			}, nil
		},
		SaveMap: func(_ string, cells []game.FullCell) error {
			savedCells = cells
			return nil
		},
	}
	defer RestoreDefaults()

	origBroadcast := broadcastFn
	var broadcastTypes []string
	broadcastFn = func(message []byte) {
		var msg struct {
			Type string `json:"type"`
		}
		if err := json.Unmarshal(message, &msg); err != nil {
			t.Fatalf("failed to decode broadcast: %v", err)
		}
		broadcastTypes = append(broadcastTypes, msg.Type)
	}
	defer func() { broadcastFn = origBroadcast }()

	saveTargetHealth(
		instanceID,
		"monster",
		monsterID,
		1,
		"player",
		attackResult{Damage: 5, NewHealth: 5},
	)

	if len(savedCells) != 1 || savedCells[0].Monster == nil {
		t.Fatalf("expected saved map with updated monster cell, got %+v", savedCells)
	}
	if savedCells[0].Monster.Health != 5 {
		t.Fatalf("expected saved monster hp 5, got %d", savedCells[0].Monster.Health)
	}
	if slices.Contains(broadcastTypes, "UPDATE_CELL") {
		t.Fatalf("expected no UPDATE_CELL for non-lethal monster damage, got broadcasts %v", broadcastTypes)
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

func TestDoCounterattackWithEnergy_GuardianCounterIsFree(t *testing.T) {
	Combat = CombatDeps{
		UpdatePlayer: func(_ string, _ *models.PlayerResponse) error { return nil },
		GetPlayer: func(_ string, userID int) (*models.PlayerResponse, error) {
			return &models.PlayerResponse{
				UserID: userID,
				Energy: 0,
				Health: 12,
			}, nil
		},
		UpdateMonsterHealth: func(_ string, _, _ int) error { return nil },
	}
	defer RestoreDefaults()

	result, err := doCounterattackWithEnergy(
		"guardian-counter",
		"player", 1,
		"player", 2,
		stats{Attack: 5, Defense: 1, Health: 12},
		stats{Attack: 9, Defense: 8, Health: 12, CharacterType: "guardian"},
		true,
		true,
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.Triggered {
		t.Fatal("expected guardian counterattack to trigger without energy")
	}
	if result.Damage != 8 {
		t.Fatalf("expected guardian counter damage 8, got %d", result.Damage)
	}
}

func TestBaseMoveEnergyCost(t *testing.T) {
	cases := []struct {
		mobility int
		cost     int
	}{
		{mobility: 0, cost: 4},
		{mobility: 2, cost: 4},
		{mobility: 3, cost: 3},
		{mobility: 5, cost: 3},
		{mobility: 6, cost: 2},
		{mobility: 8, cost: 2},
		{mobility: 9, cost: 1},
		{mobility: 12, cost: 1},
	}

	for _, tc := range cases {
		if got := baseMoveEnergyCost(tc.mobility); got != tc.cost {
			t.Fatalf("mobility %d: expected move cost %d, got %d", tc.mobility, tc.cost, got)
		}
	}
}

func TestApplyDamage_BerserkerBonusThresholds(t *testing.T) {
	attacker := stats{Attack: 20, CharacterType: "berserker"}

	cases := []struct {
		name     string
		health   int
		maxHP    int
		defense  int
		expected int
	}{
		{name: "below_75_percent", health: 74, maxHP: 100, defense: 10, expected: 11},
		{name: "below_50_percent", health: 49, maxHP: 100, defense: 10, expected: 12},
		{name: "below_25_percent", health: 24, maxHP: 100, defense: 10, expected: 13},
	}

	for _, tc := range cases {
		result := applyDamage(attacker, stats{
			Health:    tc.health,
			MaxHealth: tc.maxHP,
			Defense:   tc.defense,
		})
		if result.Damage != tc.expected {
			t.Fatalf("%s: expected damage %d, got %d", tc.name, tc.expected, result.Damage)
		}
	}
}

func TestApplyKnockbackOccupancy_PlayerClearsOldTileAndOccupiesNewTile(t *testing.T) {
	cells := []game.FullCell{
		{X: 1, Y: 1, TileCode: int(game.Walkable), IsPlayer: true},
		{X: 2, Y: 1, TileCode: int(game.Walkable), IsPlayer: false},
	}

	updated, err := applyKnockbackOccupancy(cells, "player", 0, 1, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cells[0].IsPlayer {
		t.Fatal("expected old player tile to be cleared")
	}
	if !cells[1].IsPlayer {
		t.Fatal("expected new player tile to become occupied")
	}
	if len(updated) != 2 || updated[0].IsPlayer || !updated[1].IsPlayer {
		t.Fatalf("unexpected updated cells payload: %+v", updated)
	}
}

func TestApplyKnockbackOccupancy_MonsterRestoresOldTileAndOccupiesNewTile(t *testing.T) {
	cells := []game.FullCell{
		{
			X: 1, Y: 1, TileCode: int('M'),
			Monster: &game.MonsterData{DBInstanceID: 42, Health: 10},
		},
		{X: 2, Y: 1, TileCode: int(game.Walkable)},
	}

	updated, err := applyKnockbackOccupancy(cells, "monster", 0, 1, cells[0].Monster)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cells[0].Monster != nil {
		t.Fatal("expected old monster tile to be cleared")
	}
	if cells[0].TileCode != int(game.Walkable) {
		t.Fatalf("expected old monster tile to become walkable, got %d", cells[0].TileCode)
	}
	if cells[1].Monster == nil {
		t.Fatal("expected new monster tile to become occupied")
	}
	if cells[1].TileCode != int('M') {
		t.Fatalf("expected new monster tile code to be monster tile, got %d", cells[1].TileCode)
	}
	if len(updated) != 2 || updated[0].Monster != nil || updated[1].Monster == nil {
		t.Fatalf("unexpected updated cells payload: %+v", updated)
	}
}

func TestResolveRangerPushFallbackDamage_UsesDefenseAwareFormula(t *testing.T) {
	const instanceID = "ranger-fallback"

	game.MatchStatesMu.Lock()
	game.MatchStates[instanceID] = &game.MatchState{
		InstanceID: instanceID,
		ArmorBreak: map[string]game.ArmorBreakState{
			"player:7": {Stacks: 2, RemainingTurns: 2},
		},
	}
	game.MatchStatesMu.Unlock()
	defer func() {
		game.MatchStatesMu.Lock()
		delete(game.MatchStates, instanceID)
		game.MatchStatesMu.Unlock()
	}()

	result := resolveRangerPushFallbackDamage(
		instanceID,
		"player",
		7,
		stats{Attack: 11, CharacterType: "ranger"},
		stats{Defense: 10, Health: 20, MaxHealth: 20},
		20,
	)

	if result.Damage != 5 {
		t.Fatalf("expected defense-aware fallback damage 5, got %d", result.Damage)
	}
	if result.NewHealth != 15 {
		t.Fatalf("expected new health 15, got %d", result.NewHealth)
	}
}
