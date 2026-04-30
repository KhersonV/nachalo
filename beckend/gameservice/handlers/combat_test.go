// ==============================================
// gameservice/handlers/combat_handler_test.go
// ==============================================
package handlers

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"slices"
	"testing"

	"auth/common"
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
	req = req.WithContext(context.WithValue(req.Context(), common.UserIDKey, 1))
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

type rangerArmorBreakCombatHarness struct {
	t             *testing.T
	instanceID    string
	attacker      models.PlayerResponse
	monster       repository.MatchMonster
	cells         map[[2]int]game.FullCell
	matchState    *game.MatchState
	exchanges     []CombatExchangePayload
	pushCellLoads int
}

func newRangerArmorBreakCombatHarness(t *testing.T, instanceID string, targetHP int, blockedPush bool) *rangerArmorBreakCombatHarness {
	t.Helper()

	h := &rangerArmorBreakCombatHarness{
		t:          t,
		instanceID: instanceID,
		attacker: models.PlayerResponse{
			UserID:        1,
			CharacterType: "ranger",
			Attack:        11,
			Defense:       3,
			Health:        50,
			MaxHealth:     50,
			Energy:        100,
			MaxEnergy:     100,
			Mobility:      4,
			IsRanged:      true,
			AttackRange:   4,
		},
		monster: repository.MatchMonster{
			MonsterInstanceID: 99,
			RefID:             7,
			Health:            targetHP,
			MaxHealth:         targetHP,
			Attack:            4,
			Defense:           10,
			X:                 2,
			Y:                 1,
		},
		cells: make(map[[2]int]game.FullCell),
		matchState: &game.MatchState{
			InstanceID:   instanceID,
			ActiveUserID: 1,
			TurnOrder:    []int{1},
			TurnNumber:   1,
		},
	}
	h.attacker.Position.X = 0
	h.attacker.Position.Y = 1
	h.cells[[2]int{2, 1}] = game.FullCell{
		X:        2,
		Y:        1,
		TileCode: int('M'),
		Monster: &game.MonsterData{
			ID:           h.monster.RefID,
			DBInstanceID: h.monster.MonsterInstanceID,
			Health:       targetHP,
			MaxHealth:    targetHP,
			Defense:      h.monster.Defense,
			Attack:       h.monster.Attack,
		},
	}
	h.cells[[2]int{3, 1}] = game.FullCell{
		X:        3,
		Y:        1,
		TileCode: int(game.Walkable),
		IsPlayer: blockedPush,
	}

	registerCombatMatchState(t, instanceID, h.matchState)
	h.install()

	return h
}

func (h *rangerArmorBreakCombatHarness) install() {
	h.t.Helper()

	Combat = CombatDeps{
		UpdatePlayer: func(_ string, p *models.PlayerResponse) error {
			h.attacker = *p
			return nil
		},
		GetPlayer: func(_ string, userID int) (*models.PlayerResponse, error) {
			if userID != h.attacker.UserID {
				h.t.Fatalf("unexpected player id %d", userID)
			}
			player := h.attacker
			return &player, nil
		},
		GetMonster: func(_ string, monsterID int) (*repository.MatchMonster, error) {
			if monsterID != h.monster.MonsterInstanceID {
				h.t.Fatalf("unexpected monster id %d", monsterID)
			}
			monster := h.monster
			return &monster, nil
		},
		UpdateMonsterHealth: func(_ string, monsterID, hp int) error {
			if monsterID != h.monster.MonsterInstanceID {
				h.t.Fatalf("unexpected monster id %d", monsterID)
			}
			h.monster.Health = hp
			return nil
		},
		DeleteMonster: func(_ string, monsterID int) error {
			if monsterID != h.monster.MonsterInstanceID {
				h.t.Fatalf("unexpected monster id %d", monsterID)
			}
			h.monster.Health = 0
			return nil
		},
		LoadMap: func(_ string) ([]game.FullCell, error) {
			cells := make([]game.FullCell, 0, len(h.cells))
			for _, cell := range h.cells {
				cells = append(cells, cell)
			}
			return cells, nil
		},
		SaveMap: func(_ string, cells []game.FullCell) error {
			h.cells = make(map[[2]int]game.FullCell, len(cells))
			for _, cell := range cells {
				h.cells[[2]int{cell.X, cell.Y}] = cell
			}
			return nil
		},
		MarkPlayerDead: func(_ string, _ int) error { return nil },
		ClearPlayerFlag: func(_ string, _ repository.Position) error {
			return nil
		},
		UpdateTurn: func(_ string, _, _ int) error { return nil },
		Finalize:   func(_ string) error { return nil },
		LoadGameState: func(_ string) (*game.MatchState, bool) {
			return h.matchState, true
		},
	}

	origLoadCombatMapCell := loadCombatMapCell
	origPersistCombatPushCells := persistCombatPushCells
	origUpdateCombatMonsterPosition := updateCombatMonsterPosition
	origBroadcast := broadcastFn

	loadCombatMapCell = func(_ string, x int, y int) (*game.FullCell, error) {
		h.pushCellLoads++
		cell, ok := h.cells[[2]int{x, y}]
		if !ok {
			return nil, nil
		}
		copyCell := cell
		return &copyCell, nil
	}
	persistCombatPushCells = func(_ string, oldCell game.FullCell, newCell game.FullCell) error {
		h.cells[[2]int{oldCell.X, oldCell.Y}] = oldCell
		h.cells[[2]int{newCell.X, newCell.Y}] = newCell
		return nil
	}
	updateCombatMonsterPosition = func(_ string, monsterID int, x int, y int) error {
		if monsterID != h.monster.MonsterInstanceID {
			h.t.Fatalf("unexpected monster id %d", monsterID)
		}
		h.monster.X = x
		h.monster.Y = y
		return nil
	}
	broadcastFn = func(message []byte) {
		var msg CombatExchangeMessage
		if err := json.Unmarshal(message, &msg); err == nil && msg.Type == "COMBAT_EXCHANGE" {
			h.exchanges = append(h.exchanges, msg.Payload)
		}
	}

	h.t.Cleanup(func() {
		RestoreDefaults()
		loadCombatMapCell = origLoadCombatMapCell
		persistCombatPushCells = origPersistCombatPushCells
		updateCombatMonsterPosition = origUpdateCombatMonsterPosition
		broadcastFn = origBroadcast
	})
}

func (h *rangerArmorBreakCombatHarness) attackMonster() CombatExchangePayload {
	h.t.Helper()

	rec := httptest.NewRecorder()
	universalAttackLocked(rec, AttackRequest{
		InstanceID:   h.instanceID,
		AttackerType: "player",
		AttackerID:   h.attacker.UserID,
		TargetType:   "monster",
		TargetID:     h.monster.MonsterInstanceID,
	})
	if rec.Code != http.StatusOK {
		h.t.Fatalf("expected attack status 200, got %d: %s", rec.Code, rec.Body.String())
	}
	if len(h.exchanges) == 0 {
		h.t.Fatal("expected combat exchange broadcast")
	}
	return h.exchanges[len(h.exchanges)-1]
}

func findCombatEffect(payload CombatExchangePayload, kind string) (CombatEffect, bool) {
	for _, effect := range payload.Effects {
		if effect.Kind == kind {
			return effect, true
		}
	}
	return CombatEffect{}, false
}

func hasCombatStep(payload CombatExchangePayload, kind string) bool {
	for _, step := range payload.Steps {
		if step.Kind == kind {
			return true
		}
	}
	return false
}

func findCombatStep(payload CombatExchangePayload, kind string) (CombatStep, bool) {
	for _, step := range payload.Steps {
		if step.Kind == kind {
			return step, true
		}
	}
	return CombatStep{}, false
}

func countCombatEffects(payload CombatExchangePayload, kind string) int {
	count := 0
	for _, effect := range payload.Effects {
		if effect.Kind == kind {
			count++
		}
	}
	return count
}

func stubReflexRolls(t *testing.T, results ...bool) *int {
	t.Helper()

	origRollReflexProc := rollReflexProc
	calls := 0
	rollReflexProc = func(chance int) bool {
		calls++
		if chance != 10 {
			t.Fatalf("expected reflex chance 10, got %d", chance)
		}
		if len(results) == 0 {
			return false
		}
		if calls > len(results) {
			return results[len(results)-1]
		}
		return results[calls-1]
	}

	t.Cleanup(func() {
		rollReflexProc = origRollReflexProc
	})

	return &calls
}

func TestRangerCriticalShotUsesPostDefenseDamageAndRecordsFinalDamage(t *testing.T) {
	h := newRangerArmorBreakCombatHarness(t, "ranger-crit-post-defense", 100, false)
	h.attacker.Attack = 30
	h.attacker.Agility = 5
	h.monster.Defense = 10
	stubReflexRolls(t, true)

	payload := h.attackMonster()
	hit, ok := findCombatStep(payload, "hit")
	if !ok || hit.Damage != 28 || hit.TargetHPAfter != 72 {
		t.Fatalf("expected crit to turn post-defense damage 20 into 28, got step=%+v ok=%v", hit, ok)
	}
	if h.monster.Health != 72 {
		t.Fatalf("expected crit damage applied to monster HP, got %d", h.monster.Health)
	}
	crit, ok := findCombatEffect(payload, "crit")
	if !ok || !crit.Succeeded || crit.Source == nil || crit.Source.ID != h.attacker.UserID {
		t.Fatalf("expected explicit crit effect from ranger, got effect=%+v ok=%v", crit, ok)
	}
	if len(h.matchState.DamageEvents) != 1 || h.matchState.DamageEvents[0].Amount != 28 {
		t.Fatalf("expected damage stats to record final crit damage 28, got %+v", h.matchState.DamageEvents)
	}
}

func TestRangerCriticalShotDoesNotTriggerOnFallbackFullAttack(t *testing.T) {
	h := newRangerArmorBreakCombatHarness(t, "ranger-crit-no-fallback", 200, true)
	h.attacker.Agility = 5
	rollCalls := stubReflexRolls(t, true)

	h.attackMonster()
	h.attackMonster()
	third := h.attackMonster()

	if got := countCombatEffects(third, "crit"); got != 1 {
		t.Fatalf("expected only primary hit to crit on fallback exchange, got %d effects in %+v", got, third.Effects)
	}
	push, ok := findCombatEffect(third, "push")
	if !ok || push.Succeeded || push.BonusDamage != 5 {
		t.Fatalf("expected fallback full attack to keep uncritted damage 5, got effect=%+v ok=%v", push, ok)
	}
	if *rollCalls != 3 {
		t.Fatalf("expected only the three paid primary attacks to roll crit, got %d rolls", *rollCalls)
	}
}

func TestRangerCriticalShotDoesNotTriggerWhenBlocked(t *testing.T) {
	h := newGuardianShieldBlockCombatHarness(t, "ranger-crit-blocked", newGuardianBlockRangedAttacker("ranger"))
	attacker := h.players[1]
	attacker.Agility = 5
	h.players[1] = attacker

	payload := h.attackGuardian()
	if _, ok := findCombatEffect(payload, "crit"); ok {
		t.Fatalf("expected guardian block to prevent ranger crit, got payload %+v", payload)
	}
}

func TestRangerCriticalShotDoesNotTriggerOnZeroDamage(t *testing.T) {
	h := newRangerArmorBreakCombatHarness(t, "ranger-crit-zero-damage", 40, false)
	h.attacker.Agility = 5
	h.monster.Defense = 40
	rollCalls := stubReflexRolls(t, true)

	payload := h.attackMonster()
	if _, ok := findCombatEffect(payload, "crit"); ok {
		t.Fatalf("expected zero-damage hit not to emit crit, got payload %+v", payload)
	}
	hit, ok := findCombatStep(payload, "hit")
	if !ok || hit.Damage != 0 {
		t.Fatalf("expected primary hit damage 0, got step=%+v ok=%v", hit, ok)
	}
	if *rollCalls != 0 {
		t.Fatalf("expected no crit roll for zero calculated damage, got %d rolls", *rollCalls)
	}
}

func TestRangerCriticalShotDoesNotBreakArmorBreakStackProgression(t *testing.T) {
	h := newRangerArmorBreakCombatHarness(t, "ranger-crit-armor-break-progression", 200, false)
	h.attacker.Agility = 5
	stubReflexRolls(t, true)

	first := h.attackMonster()
	firstArmorBreak, ok := findCombatEffect(first, "armorBreak")
	if !ok || firstArmorBreak.Stacks != 1 {
		t.Fatalf("expected critted first hit to apply armor break stack 1, got effect=%+v ok=%v", firstArmorBreak, ok)
	}
	if _, ok := findCombatEffect(first, "crit"); !ok {
		t.Fatalf("expected first paid hit to emit crit effect, got %+v", first.Effects)
	}

	second := h.attackMonster()
	secondArmorBreak, ok := findCombatEffect(second, "armorBreak")
	if !ok || secondArmorBreak.Stacks != 2 {
		t.Fatalf("expected critted second hit to advance armor break stack 2, got effect=%+v ok=%v", secondArmorBreak, ok)
	}
	if state := h.matchState.GetArmorBreakState("monster", h.monster.MonsterInstanceID); state.Stacks != 2 {
		t.Fatalf("expected armor break state stack 2 after critted hits, got %+v", state)
	}
}

func TestRangerCriticalShotArmorBreakThirdHitPushResetsAndRestarts(t *testing.T) {
	h := newRangerArmorBreakCombatHarness(t, "ranger-crit-push-reset", 200, false)
	h.attacker.Agility = 5
	stubReflexRolls(t, true)

	h.attackMonster()
	h.attackMonster()
	third := h.attackMonster()

	if _, ok := findCombatEffect(third, "crit"); !ok {
		t.Fatalf("expected third paid hit to emit crit effect, got %+v", third.Effects)
	}
	push, ok := findCombatEffect(third, "push")
	if !ok || !push.Succeeded {
		t.Fatalf("expected armor break third effect push after crit, got effect=%+v ok=%v", push, ok)
	}
	if state := h.matchState.GetArmorBreakState("monster", h.monster.MonsterInstanceID); state.Stacks != 0 {
		t.Fatalf("expected armor break reset after critted third-hit push, got %+v", state)
	}

	next := h.attackMonster()
	nextArmorBreak, ok := findCombatEffect(next, "armorBreak")
	if !ok || nextArmorBreak.Stacks != 1 {
		t.Fatalf("expected next hit after reset to restart at stack 1, got effect=%+v ok=%v", nextArmorBreak, ok)
	}
}

func TestRangerCriticalShotDoesNotTriggerOnCounterattack(t *testing.T) {
	attacker := newCombatTestPlayer(1, "guardian", 1, 40, 1, 1)
	attacker.Attack = 8
	attacker.Defense = 0
	attacker.Energy = 100
	attacker.MaxEnergy = 100

	target := newCombatTestPlayer(2, "ranger", 2, 50, 2, 1)
	target.Attack = 20
	target.Defense = 0
	target.Energy = 100
	target.MaxEnergy = 100
	target.Agility = 5

	h := newBloodFeastPlayerCombatHarness(t, "ranger-crit-no-counter", attacker, target)
	payload := h.attackPlayer()

	if !hasCombatStep(payload, "counter") {
		t.Fatalf("expected ranger counterattack in control scenario, got steps %+v", payload.Steps)
	}
	if _, ok := findCombatEffect(payload, "crit"); ok {
		t.Fatalf("expected counterattack not to emit crit, got payload %+v", payload)
	}
}

func TestRangerCriticalShotDoesNotTriggerOnBerserkerFuryFollowUp(t *testing.T) {
	attacker := newCombatTestPlayer(1, "berserker", 1, 30, 1, 1)
	attacker.MaxHealth = 50
	attacker.Attack = 20
	attacker.Defense = 0
	attacker.Energy = 100
	attacker.MaxEnergy = 100
	attacker.Agility = 5

	target := newCombatTestPlayer(2, "ranger", 2, 80, 2, 1)
	target.Attack = 1
	target.Defense = 0
	target.Energy = 100
	target.MaxEnergy = 100

	h := newBloodFeastPlayerCombatHarness(t, "ranger-crit-no-fury-followup", attacker, target)
	payload := h.attackPlayer()

	if !hasCombatStep(payload, "followup") {
		t.Fatalf("expected berserker fury follow-up in control scenario, got steps %+v", payload.Steps)
	}
	if _, ok := findCombatEffect(payload, "crit"); ok {
		t.Fatalf("expected berserker fury follow-up not to emit crit, got payload %+v", payload)
	}
}

func newMysticArcaneOverburnHarness(t *testing.T, instanceID string, targetEnergy int, targetHP int) *bloodFeastPlayerCombatHarness {
	t.Helper()

	attacker := newCombatTestPlayer(1, "mystic", 1, 40, 1, 1)
	attacker.Attack = 10
	attacker.Defense = 0
	attacker.Energy = 50
	attacker.MaxEnergy = 100
	attacker.Agility = 5
	attacker.IsRanged = true
	attacker.AttackRange = 4
	attacker.Position.X = 0
	attacker.Position.Y = 1

	target := newCombatTestPlayer(2, "ranger", 2, targetHP, 2, 1)
	target.Attack = 1
	target.Defense = 0
	target.Energy = targetEnergy
	target.MaxEnergy = 100

	return newBloodFeastPlayerCombatHarness(t, instanceID, attacker, target)
}

func TestMysticEnergyDrainNormalBurnsThreeAndRestoresOneWithoutReflexProc(t *testing.T) {
	h := newMysticArcaneOverburnHarness(t, "mystic-normal-drain", 10, 50)
	h.rollResult = false

	payload := h.attackPlayer()
	drain, ok := findCombatEffect(payload, "energyDrain")
	if !ok || drain.EnergyDrained != 3 || drain.Amount != 3 || drain.EnergyGranted != 1 {
		t.Fatalf("expected normal drain burn 3 restore 1, got effect=%+v ok=%v", drain, ok)
	}
	if h.players[1].Energy != 43 {
		t.Fatalf("expected mystic energy 43 after ranged attack cost and normal restore, got %d", h.players[1].Energy)
	}
	if h.players[2].Energy != 7 {
		t.Fatalf("expected target energy 7 after normal drain, got %d", h.players[2].Energy)
	}
	if _, ok := findCombatEffect(payload, "arcaneOverburn"); ok {
		t.Fatalf("expected no overburn effect without proc, got payload %+v", payload)
	}
	if _, ok := findCombatEffect(payload, "pureDamage"); ok {
		t.Fatalf("expected no pure damage while target has enough energy, got payload %+v", payload)
	}
}

func TestMysticArcaneOverburnBurnsFourAndRestoresThree(t *testing.T) {
	h := newMysticArcaneOverburnHarness(t, "mystic-overburn-full-energy", 10, 50)

	payload := h.attackPlayer()
	drain, ok := findCombatEffect(payload, "energyDrain")
	if !ok || drain.EnergyDrained != 4 || drain.Amount != 4 || drain.EnergyGranted != 3 {
		t.Fatalf("expected overburn drain burn 4 restore 3, got effect=%+v ok=%v", drain, ok)
	}
	if h.players[1].Energy != 45 {
		t.Fatalf("expected mystic energy 45 after ranged attack cost and overburn restore, got %d", h.players[1].Energy)
	}
	if h.players[2].Energy != 6 {
		t.Fatalf("expected target energy 6 after overburn drain, got %d", h.players[2].Energy)
	}
	if _, ok := findCombatEffect(payload, "arcaneOverburn"); !ok {
		t.Fatalf("expected explicit arcane overburn effect, got payload %+v", payload)
	}
	if _, ok := findCombatEffect(payload, "pureDamage"); ok {
		t.Fatalf("expected no pure damage while target has enough energy, got payload %+v", payload)
	}
}

func TestMysticArcaneOverburnMissingEnergyBecomesPureDamage(t *testing.T) {
	h := newMysticArcaneOverburnHarness(t, "mystic-overburn-missing-energy", 2, 50)

	payload := h.attackPlayer()
	drain, ok := findCombatEffect(payload, "energyDrain")
	if !ok || drain.EnergyDrained != 2 || drain.Amount != 2 {
		t.Fatalf("expected actual burn 2 from target energy, got effect=%+v ok=%v", drain, ok)
	}
	pure, ok := findCombatEffect(payload, "pureDamage")
	if !ok || pure.Amount != 2 {
		t.Fatalf("expected missing burn to deal pure damage 2, got effect=%+v ok=%v", pure, ok)
	}
	bonus, ok := findCombatStep(payload, "bonus")
	if !ok || bonus.Damage != 2 || bonus.TargetHPAfter != 38 {
		t.Fatalf("expected pure damage bonus step for 2 damage, got step=%+v ok=%v", bonus, ok)
	}
	if h.players[2].Health != 38 {
		t.Fatalf("expected target hp 38 after hit and pure damage, got %d", h.players[2].Health)
	}
	if len(h.matchState.DamageEvents) != 2 || h.matchState.DamageEvents[1].Amount != 2 {
		t.Fatalf("expected pure damage stats amount 2 after primary damage, got %+v", h.matchState.DamageEvents)
	}
}

func TestMysticArcaneOverburnPureDamageIgnoresDefense(t *testing.T) {
	h := newMysticArcaneOverburnHarness(t, "mystic-overburn-ignores-defense", 0, 20)
	target := h.players[2]
	target.Defense = 100
	h.players[2] = target

	payload := h.attackPlayer()
	pure, ok := findCombatEffect(payload, "pureDamage")
	if !ok || pure.Amount != 4 {
		t.Fatalf("expected pure damage 4 despite high defense, got effect=%+v ok=%v", pure, ok)
	}
	hit, ok := findCombatStep(payload, "hit")
	if !ok || hit.Damage != 0 || hit.TargetHPAfter != 20 {
		t.Fatalf("expected primary hit to be fully absorbed by defense, got step=%+v ok=%v", hit, ok)
	}
	if h.players[2].Health != 16 {
		t.Fatalf("expected pure damage to ignore defense and leave hp 16, got %d", h.players[2].Health)
	}
	if len(h.matchState.DamageEvents) != 1 || h.matchState.DamageEvents[0].Amount != 4 {
		t.Fatalf("expected only pure damage stats amount 4, got %+v", h.matchState.DamageEvents)
	}
}

func TestMysticArcaneOverburnZeroEnergyTargetTakesFourPureDamage(t *testing.T) {
	h := newMysticArcaneOverburnHarness(t, "mystic-overburn-zero-energy", 0, 50)

	payload := h.attackPlayer()
	drain, ok := findCombatEffect(payload, "energyDrain")
	if !ok || drain.EnergyDrained != 0 || drain.Amount != 0 {
		t.Fatalf("expected actual burn 0 against empty energy, got effect=%+v ok=%v", drain, ok)
	}
	pure, ok := findCombatEffect(payload, "pureDamage")
	if !ok || pure.Amount != 4 {
		t.Fatalf("expected zero-energy target to take pure damage 4, got effect=%+v ok=%v", pure, ok)
	}
	if h.players[2].Health != 36 {
		t.Fatalf("expected target hp 36 after primary hit and pure damage, got %d", h.players[2].Health)
	}
}

func TestMysticArcaneOverburnPureDamageCanKillAndStatsClampOverkill(t *testing.T) {
	h := newMysticArcaneOverburnHarness(t, "mystic-overburn-kill-clamp", 0, 11)
	h.matchState.ActiveUserID = 0

	rec := httptest.NewRecorder()
	universalAttackLocked(rec, AttackRequest{
		InstanceID:   h.instanceID,
		AttackerType: "player",
		AttackerID:   1,
		TargetType:   "player",
		TargetID:     2,
	})
	if rec.Code != http.StatusOK {
		t.Fatalf("expected attack status 200, got %d: %s", rec.Code, rec.Body.String())
	}
	if len(h.exchanges) == 0 {
		t.Fatal("expected combat exchange broadcast")
	}
	payload := h.exchanges[len(h.exchanges)-1]
	pure, ok := findCombatEffect(payload, "pureDamage")
	if !ok || pure.Amount != 1 {
		t.Fatalf("expected pure damage effect amount clamped to actual hp damage 1, got effect=%+v ok=%v", pure, ok)
	}
	if !hasCombatStep(payload, "death") {
		t.Fatalf("expected pure damage to kill target and emit death step, got steps %+v", payload.Steps)
	}
	if h.players[2].Health != 0 {
		t.Fatalf("expected target to be dead, got hp %d", h.players[2].Health)
	}
	if len(h.matchState.DamageEvents) != 2 || h.matchState.DamageEvents[1].Amount != 1 {
		t.Fatalf("expected pure overkill stats to count actual damage 1, got %+v", h.matchState.DamageEvents)
	}
}

func TestMysticArcaneOverburnRestoreCannotExceedMaxEnergy(t *testing.T) {
	h := newMysticArcaneOverburnHarness(t, "mystic-overburn-restore-cap", 10, 50)
	attacker := h.players[1]
	attacker.Energy = 100
	attacker.MaxEnergy = 93
	h.players[1] = attacker

	payload := h.attackPlayer()
	drain, ok := findCombatEffect(payload, "energyDrain")
	if !ok || drain.EnergyGranted != 1 || drain.SourceEnergyAfter != 93 {
		t.Fatalf("expected restore to be capped at max energy with grant 1, got effect=%+v ok=%v", drain, ok)
	}
	if h.players[1].Energy != 93 {
		t.Fatalf("expected mystic energy capped at 93, got %d", h.players[1].Energy)
	}
}

func TestMysticArcaneOverburnGuardianBlockPreventsDrainOverburnAndPureDamage(t *testing.T) {
	attacker := newGuardianBlockRangedAttacker("mystic")
	attacker.Agility = 5
	h := newGuardianShieldBlockCombatHarness(t, "mystic-overburn-blocked", attacker)

	payload := h.attackGuardian()
	if _, ok := findCombatEffect(payload, "energyDrain"); ok {
		t.Fatalf("expected guardian block to skip energy drain, got payload %+v", payload)
	}
	if _, ok := findCombatEffect(payload, "arcaneOverburn"); ok {
		t.Fatalf("expected guardian block to skip overburn, got payload %+v", payload)
	}
	if _, ok := findCombatEffect(payload, "pureDamage"); ok {
		t.Fatalf("expected guardian block to skip pure damage, got payload %+v", payload)
	}
}

func TestMysticArcaneOverburnGuardianBlockDoesNotBlockPureDamageAfterSuccessfulHit(t *testing.T) {
	attacker := newCombatTestPlayer(1, "mystic", 1, 40, 1, 1)
	attacker.Attack = 10
	attacker.Energy = 50
	attacker.MaxEnergy = 100
	attacker.Agility = 5
	attacker.IsRanged = true
	attacker.AttackRange = 4
	attacker.Position.X = 0
	attacker.Position.Y = 1

	guardian := newCombatTestPlayer(2, "guardian", 2, 50, 2, 1)
	guardian.Defense = 0
	guardian.Energy = 0
	guardian.MaxEnergy = 100
	guardian.Agility = 5

	h := newBloodFeastPlayerCombatHarness(t, "mystic-overburn-guardian-success", attacker, guardian)
	rollCalls := stubReflexRolls(t, false, true)

	payload := h.attackPlayer()
	if _, ok := findCombatEffect(payload, "block"); ok {
		t.Fatalf("expected guardian block roll to fail, got payload %+v", payload)
	}
	if countCombatEffects(payload, "arcaneOverburn") != 1 {
		t.Fatalf("expected overburn to trigger once after successful hit, got effects %+v", payload.Effects)
	}
	pure, ok := findCombatEffect(payload, "pureDamage")
	if !ok || pure.Amount != 4 {
		t.Fatalf("expected pure damage 4 after successful hit, got effect=%+v ok=%v", pure, ok)
	}
	if h.players[2].Health != 36 {
		t.Fatalf("expected guardian hp 36 after successful hit and pure damage, got %d", h.players[2].Health)
	}
	if *rollCalls != 2 {
		t.Fatalf("expected one guardian block roll and one overburn roll, got %d", *rollCalls)
	}
}

func TestRangerArmorBreakPushResetsStacksAndNextHitRestarts(t *testing.T) {
	h := newRangerArmorBreakCombatHarness(t, "ranger-push-reset", 30, false)

	first := h.attackMonster()
	firstArmorBreak, ok := findCombatEffect(first, "armorBreak")
	if !ok || firstArmorBreak.Stacks != 1 {
		t.Fatalf("expected first hit to apply stack 1, got effect=%+v ok=%v", firstArmorBreak, ok)
	}
	if state := h.matchState.GetArmorBreakState("monster", h.monster.MonsterInstanceID); state.Stacks != 1 {
		t.Fatalf("expected state stack 1 after first hit, got %+v", state)
	}

	second := h.attackMonster()
	secondArmorBreak, ok := findCombatEffect(second, "armorBreak")
	if !ok || secondArmorBreak.Stacks != 2 {
		t.Fatalf("expected second hit to apply stack 2, got effect=%+v ok=%v", secondArmorBreak, ok)
	}
	if state := h.matchState.GetArmorBreakState("monster", h.monster.MonsterInstanceID); state.Stacks != 2 {
		t.Fatalf("expected state stack 2 after second hit, got %+v", state)
	}

	third := h.attackMonster()
	push, ok := findCombatEffect(third, "push")
	if !ok || !push.Succeeded {
		t.Fatalf("expected third hit to push successfully, got effect=%+v ok=%v", push, ok)
	}
	if _, ok := findCombatEffect(third, "armorBreak"); ok {
		t.Fatalf("expected third effect not to reapply armor break, got payload %+v", third)
	}
	if state := h.matchState.GetArmorBreakState("monster", h.monster.MonsterInstanceID); state.Stacks != 0 {
		t.Fatalf("expected armor break reset after push, got %+v", state)
	}
	if h.monster.X != 3 || h.monster.Y != 1 {
		t.Fatalf("expected monster pushed to 3,1, got %d,%d", h.monster.X, h.monster.Y)
	}
	if h.attacker.Energy != 79 {
		t.Fatalf("expected ranger energy refund before reset to leave energy 79, got %d", h.attacker.Energy)
	}

	next := h.attackMonster()
	nextArmorBreak, ok := findCombatEffect(next, "armorBreak")
	if !ok || nextArmorBreak.Stacks != 1 {
		t.Fatalf("expected next hit after reset to restart at stack 1, got effect=%+v ok=%v", nextArmorBreak, ok)
	}
}

func TestRangerArmorBreakBlockedPushFallbackResetsStacksAndNextHitRestarts(t *testing.T) {
	h := newRangerArmorBreakCombatHarness(t, "ranger-fallback-reset", 30, true)

	h.attackMonster()
	h.attackMonster()
	third := h.attackMonster()

	push, ok := findCombatEffect(third, "push")
	if !ok || push.Succeeded || push.BonusDamage != 5 {
		t.Fatalf("expected blocked push fallback damage 5, got effect=%+v ok=%v", push, ok)
	}
	if !hasCombatStep(third, "bonus") {
		t.Fatalf("expected fallback full attack bonus step, got steps %+v", third.Steps)
	}
	if state := h.matchState.GetArmorBreakState("monster", h.monster.MonsterInstanceID); state.Stacks != 0 {
		t.Fatalf("expected armor break reset after fallback, got %+v", state)
	}
	if h.monster.X != 2 || h.monster.Y != 1 {
		t.Fatalf("expected blocked push to keep monster at 2,1, got %d,%d", h.monster.X, h.monster.Y)
	}

	next := h.attackMonster()
	nextArmorBreak, ok := findCombatEffect(next, "armorBreak")
	if !ok || nextArmorBreak.Stacks != 1 {
		t.Fatalf("expected next hit after fallback reset to restart at stack 1, got effect=%+v ok=%v", nextArmorBreak, ok)
	}
}

func TestRangerArmorBreakZeroDamageHitAppliesAndAdvancesStacks(t *testing.T) {
	h := newRangerArmorBreakCombatHarness(t, "ranger-zero-damage-stack", 30, false)
	h.monster.Defense = 20

	first := h.attackMonster()
	if first.Steps[0].Damage != 0 {
		t.Fatalf("expected first hit to deal 0 damage, got %+v", first.Steps[0])
	}
	firstArmorBreak, ok := findCombatEffect(first, "armorBreak")
	if !ok || firstArmorBreak.Stacks != 1 {
		t.Fatalf("expected zero-damage hit to apply stack 1, got effect=%+v ok=%v", firstArmorBreak, ok)
	}

	second := h.attackMonster()
	if second.Steps[0].Damage != 0 {
		t.Fatalf("expected second hit to deal 0 damage, got %+v", second.Steps[0])
	}
	secondArmorBreak, ok := findCombatEffect(second, "armorBreak")
	if !ok || secondArmorBreak.Stacks != 2 {
		t.Fatalf("expected zero-damage hit to advance to stack 2, got effect=%+v ok=%v", secondArmorBreak, ok)
	}
	if state := h.matchState.GetArmorBreakState("monster", h.monster.MonsterInstanceID); state.Stacks != 2 {
		t.Fatalf("expected state stack 2 after two zero-damage hits, got %+v", state)
	}
}

func TestRangerArmorBreakZeroDamageThirdHitPushResetsAndRestarts(t *testing.T) {
	h := newRangerArmorBreakCombatHarness(t, "ranger-zero-damage-push-reset", 30, false)
	h.monster.Defense = 20

	h.attackMonster()
	h.attackMonster()
	third := h.attackMonster()
	if third.Steps[0].Damage != 0 {
		t.Fatalf("expected third hit to deal 0 damage, got %+v", third.Steps[0])
	}
	push, ok := findCombatEffect(third, "push")
	if !ok || !push.Succeeded {
		t.Fatalf("expected zero-damage third hit to push, got effect=%+v ok=%v", push, ok)
	}
	if state := h.matchState.GetArmorBreakState("monster", h.monster.MonsterInstanceID); state.Stacks != 0 {
		t.Fatalf("expected armor break reset after zero-damage push, got %+v", state)
	}

	next := h.attackMonster()
	nextArmorBreak, ok := findCombatEffect(next, "armorBreak")
	if !ok || nextArmorBreak.Stacks != 1 {
		t.Fatalf("expected next hit after zero-damage push reset to restart at stack 1, got effect=%+v ok=%v", nextArmorBreak, ok)
	}
}

func TestRangerArmorBreakZeroDamageBlockedPushFallbackResetsAndRestarts(t *testing.T) {
	h := newRangerArmorBreakCombatHarness(t, "ranger-zero-damage-fallback-reset", 30, true)
	h.monster.Defense = 20

	h.attackMonster()
	h.attackMonster()
	third := h.attackMonster()
	if third.Steps[0].Damage != 0 {
		t.Fatalf("expected third hit to deal 0 damage, got %+v", third.Steps[0])
	}
	push, ok := findCombatEffect(third, "push")
	if !ok || push.Succeeded || push.BonusDamage != 0 {
		t.Fatalf("expected blocked zero-damage third hit to run fallback with 0 damage, got effect=%+v ok=%v", push, ok)
	}
	if !hasCombatStep(third, "bonus") {
		t.Fatalf("expected blocked zero-damage third hit to include fallback bonus step, got steps %+v", third.Steps)
	}
	if state := h.matchState.GetArmorBreakState("monster", h.monster.MonsterInstanceID); state.Stacks != 0 {
		t.Fatalf("expected armor break reset after zero-damage fallback, got %+v", state)
	}

	next := h.attackMonster()
	nextArmorBreak, ok := findCombatEffect(next, "armorBreak")
	if !ok || nextArmorBreak.Stacks != 1 {
		t.Fatalf("expected next hit after zero-damage fallback reset to restart at stack 1, got effect=%+v ok=%v", nextArmorBreak, ok)
	}
}

func TestRangerArmorBreakFallbackDeathStillResetsStacks(t *testing.T) {
	h := newRangerArmorBreakCombatHarness(t, "ranger-fallback-death-reset", 8, true)
	h.matchState.ApplyArmorBreak("monster", h.monster.MonsterInstanceID, armorBreakMaxStacks, armorBreakDurationTurns)
	h.matchState.ApplyArmorBreak("monster", h.monster.MonsterInstanceID, armorBreakMaxStacks, armorBreakDurationTurns)

	payload := h.attackMonster()
	if !hasCombatStep(payload, "bonus") || !hasCombatStep(payload, "death") {
		t.Fatalf("expected fallback bonus and death steps, got %+v", payload.Steps)
	}
	if state := h.matchState.GetArmorBreakState("monster", h.monster.MonsterInstanceID); state.Stacks != 0 {
		t.Fatalf("expected armor break reset after lethal fallback, got %+v", state)
	}
	if h.monster.Health != 0 {
		t.Fatalf("expected monster death flow to leave hp 0, got %d", h.monster.Health)
	}
}

type berserkerBloodFeastMonsterHarness struct {
	t          *testing.T
	instanceID string
	attacker   models.PlayerResponse
	monster    repository.MatchMonster
	cells      map[[2]int]game.FullCell
	matchState *game.MatchState
	exchanges  []CombatExchangePayload
	rollResult bool
	rollCalls  int
}

func newBerserkerBloodFeastMonsterHarness(t *testing.T, instanceID string) *berserkerBloodFeastMonsterHarness {
	t.Helper()

	attacker := newCombatTestPlayer(1, "berserker", 0, 30, 1, 1)
	attacker.MaxHealth = 50
	attacker.Attack = 20
	attacker.Defense = 3
	attacker.Energy = 100
	attacker.MaxEnergy = 100
	attacker.Agility = 5

	h := &berserkerBloodFeastMonsterHarness{
		t:          t,
		instanceID: instanceID,
		attacker:   attacker,
		monster: repository.MatchMonster{
			MonsterInstanceID: 99,
			RefID:             7,
			Health:            40,
			MaxHealth:         40,
			Attack:            0,
			Defense:           0,
			X:                 2,
			Y:                 1,
		},
		cells:      make(map[[2]int]game.FullCell),
		rollResult: true,
		matchState: &game.MatchState{
			InstanceID:   instanceID,
			ActiveUserID: attacker.UserID,
			TurnOrder:    []int{attacker.UserID},
			TurnNumber:   1,
		},
	}
	h.syncMonsterCell()
	registerCombatMatchState(t, instanceID, h.matchState)
	h.install()

	return h
}

func (h *berserkerBloodFeastMonsterHarness) syncMonsterCell() {
	h.cells[[2]int{h.monster.X, h.monster.Y}] = game.FullCell{
		X:        h.monster.X,
		Y:        h.monster.Y,
		TileCode: int('M'),
		Monster: &game.MonsterData{
			ID:           h.monster.RefID,
			DBInstanceID: h.monster.MonsterInstanceID,
			Health:       h.monster.Health,
			MaxHealth:    h.monster.MaxHealth,
			Defense:      h.monster.Defense,
			Attack:       h.monster.Attack,
		},
	}
}

func (h *berserkerBloodFeastMonsterHarness) install() {
	h.t.Helper()

	Combat = CombatDeps{
		UpdatePlayer: func(_ string, p *models.PlayerResponse) error {
			if p.UserID != h.attacker.UserID {
				h.t.Fatalf("unexpected player id %d", p.UserID)
			}
			h.attacker = *p
			return nil
		},
		GetPlayer: func(_ string, userID int) (*models.PlayerResponse, error) {
			if userID != h.attacker.UserID {
				h.t.Fatalf("unexpected player id %d", userID)
			}
			player := h.attacker
			return &player, nil
		},
		GetMonster: func(_ string, monsterID int) (*repository.MatchMonster, error) {
			if monsterID != h.monster.MonsterInstanceID {
				h.t.Fatalf("unexpected monster id %d", monsterID)
			}
			monster := h.monster
			return &monster, nil
		},
		UpdateMonsterHealth: func(_ string, monsterID, hp int) error {
			if monsterID != h.monster.MonsterInstanceID {
				h.t.Fatalf("unexpected monster id %d", monsterID)
			}
			h.monster.Health = hp
			return nil
		},
		DeleteMonster: func(_ string, monsterID int) error {
			if monsterID != h.monster.MonsterInstanceID {
				h.t.Fatalf("unexpected monster id %d", monsterID)
			}
			h.monster.Health = 0
			return nil
		},
		LoadMap: func(_ string) ([]game.FullCell, error) {
			cells := make([]game.FullCell, 0, len(h.cells))
			for _, cell := range h.cells {
				cells = append(cells, cell)
			}
			return cells, nil
		},
		SaveMap: func(_ string, cells []game.FullCell) error {
			h.cells = make(map[[2]int]game.FullCell, len(cells))
			for _, cell := range cells {
				h.cells[[2]int{cell.X, cell.Y}] = cell
			}
			return nil
		},
		MarkPlayerDead: func(_ string, _ int) error { return nil },
		ClearPlayerFlag: func(_ string, _ repository.Position) error {
			return nil
		},
		UpdateTurn: func(_ string, _, _ int) error { return nil },
		Finalize:   func(_ string) error { return nil },
		LoadGameState: func(_ string) (*game.MatchState, bool) {
			return h.matchState, true
		},
	}

	origRollReflexProc := rollReflexProc
	origBroadcast := broadcastFn

	rollReflexProc = func(chance int) bool {
		h.rollCalls++
		if chance != 10 {
			h.t.Fatalf("expected berserker reflex chance 10, got %d", chance)
		}
		return h.rollResult
	}
	broadcastFn = func(message []byte) {
		var msg CombatExchangeMessage
		if err := json.Unmarshal(message, &msg); err == nil && msg.Type == "COMBAT_EXCHANGE" {
			h.exchanges = append(h.exchanges, msg.Payload)
		}
	}

	h.t.Cleanup(func() {
		RestoreDefaults()
		rollReflexProc = origRollReflexProc
		broadcastFn = origBroadcast
	})
}

func (h *berserkerBloodFeastMonsterHarness) attackMonster() CombatExchangePayload {
	h.t.Helper()

	rec := httptest.NewRecorder()
	universalAttackLocked(rec, AttackRequest{
		InstanceID:   h.instanceID,
		AttackerType: "player",
		AttackerID:   h.attacker.UserID,
		TargetType:   "monster",
		TargetID:     h.monster.MonsterInstanceID,
	})
	if rec.Code != http.StatusOK {
		h.t.Fatalf("expected attack status 200, got %d: %s", rec.Code, rec.Body.String())
	}
	if len(h.exchanges) == 0 {
		h.t.Fatal("expected combat exchange broadcast")
	}
	return h.exchanges[len(h.exchanges)-1]
}

type bloodFeastPlayerCombatHarness struct {
	t          *testing.T
	instanceID string
	players    map[int]models.PlayerResponse
	matchState *game.MatchState
	exchanges  []CombatExchangePayload
	rollResult bool
}

func newBloodFeastPlayerCombatHarness(t *testing.T, instanceID string, attacker models.PlayerResponse, target models.PlayerResponse) *bloodFeastPlayerCombatHarness {
	t.Helper()

	h := &bloodFeastPlayerCombatHarness{
		t:          t,
		instanceID: instanceID,
		players: map[int]models.PlayerResponse{
			attacker.UserID: attacker,
			target.UserID:   target,
		},
		rollResult: true,
		matchState: &game.MatchState{
			InstanceID:   instanceID,
			ActiveUserID: attacker.UserID,
			TurnOrder:    []int{attacker.UserID, target.UserID},
			TurnNumber:   1,
		},
	}

	registerCombatMatchState(t, instanceID, h.matchState)
	h.install()

	return h
}

func (h *bloodFeastPlayerCombatHarness) install() {
	h.t.Helper()

	Combat = CombatDeps{
		UpdatePlayer: func(_ string, p *models.PlayerResponse) error {
			h.players[p.UserID] = *p
			return nil
		},
		GetPlayer: func(_ string, userID int) (*models.PlayerResponse, error) {
			player, ok := h.players[userID]
			if !ok {
				h.t.Fatalf("unexpected player id %d", userID)
			}
			return &player, nil
		},
		UpdateMonsterHealth: func(_ string, _, _ int) error { return nil },
		DeleteMonster:       func(_ string, _ int) error { return nil },
		MarkPlayerDead: func(_ string, userID int) error {
			player, ok := h.players[userID]
			if ok {
				player.Health = 0
				h.players[userID] = player
			}
			return nil
		},
		ClearPlayerFlag: func(_ string, _ repository.Position) error {
			return nil
		},
		UpdateTurn: func(_ string, _, _ int) error { return nil },
		Finalize:   func(_ string) error { return nil },
		LoadGameState: func(_ string) (*game.MatchState, bool) {
			return h.matchState, true
		},
		LoadMap: func(_ string) ([]game.FullCell, error) { return nil, nil },
		SaveMap: func(_ string, _ []game.FullCell) error {
			return nil
		},
	}

	origRollReflexProc := rollReflexProc
	origBroadcast := broadcastFn
	origTransferCombatQuestArtifact := transferCombatQuestArtifact

	rollReflexProc = func(chance int) bool {
		if chance != 10 {
			h.t.Fatalf("expected reflex chance 10, got %d", chance)
		}
		return h.rollResult
	}
	transferCombatQuestArtifact = func(_ string, _, _, _, _ int, _ bool) {}
	broadcastFn = func(message []byte) {
		var msg CombatExchangeMessage
		if err := json.Unmarshal(message, &msg); err == nil && msg.Type == "COMBAT_EXCHANGE" {
			h.exchanges = append(h.exchanges, msg.Payload)
		}
	}

	h.t.Cleanup(func() {
		RestoreDefaults()
		rollReflexProc = origRollReflexProc
		broadcastFn = origBroadcast
		transferCombatQuestArtifact = origTransferCombatQuestArtifact
	})
}

func (h *bloodFeastPlayerCombatHarness) attackPlayer() CombatExchangePayload {
	h.t.Helper()

	rec := httptest.NewRecorder()
	universalAttackLocked(rec, AttackRequest{
		InstanceID:   h.instanceID,
		AttackerType: "player",
		AttackerID:   h.matchState.ActiveUserID,
		TargetType:   "player",
		TargetID:     h.matchState.TurnOrder[1],
	})
	if rec.Code != http.StatusOK {
		h.t.Fatalf("expected attack status 200, got %d: %s", rec.Code, rec.Body.String())
	}
	if len(h.exchanges) == 0 {
		h.t.Fatal("expected combat exchange broadcast")
	}
	return h.exchanges[len(h.exchanges)-1]
}

func TestBerserkerBloodFeastPrimaryHitHealsHalfActualDamage(t *testing.T) {
	h := newBerserkerBloodFeastMonsterHarness(t, "blood-feast-primary-heal")

	payload := h.attackMonster()
	effect, ok := findCombatEffect(payload, "lifesteal")
	if !ok || effect.Amount != 10 || effect.Source == nil || effect.Source.ID != h.attacker.UserID {
		t.Fatalf("expected lifesteal amount 10 from berserker, got effect=%+v ok=%v", effect, ok)
	}
	if h.attacker.Health != 40 {
		t.Fatalf("expected berserker hp 40 after 10 heal, got %d", h.attacker.Health)
	}
}

func TestBerserkerBloodFeastUsesActualDamageNotOverkill(t *testing.T) {
	h := newBerserkerBloodFeastMonsterHarness(t, "blood-feast-overkill")
	h.monster.Health = 5
	h.monster.MaxHealth = 5
	h.syncMonsterCell()

	payload := h.attackMonster()
	effect, ok := findCombatEffect(payload, "lifesteal")
	if !ok || effect.Amount != 2 {
		t.Fatalf("expected overkill lifesteal amount 2 from actual damage 5, got effect=%+v ok=%v", effect, ok)
	}
	if h.attacker.Health != 32 {
		t.Fatalf("expected berserker hp 32 after overkill heal, got %d", h.attacker.Health)
	}
	if h.monster.Health != 0 {
		t.Fatalf("expected monster to die, got hp %d", h.monster.Health)
	}
}

func TestBerserkerBloodFeastHealCannotExceedMaxHPAndEffectUsesAppliedAmount(t *testing.T) {
	h := newBerserkerBloodFeastMonsterHarness(t, "blood-feast-clamp")
	h.attacker.Health = 48

	payload := h.attackMonster()
	effect, ok := findCombatEffect(payload, "lifesteal")
	if !ok || effect.Amount != 2 {
		t.Fatalf("expected clamped lifesteal amount 2, got effect=%+v ok=%v", effect, ok)
	}
	if h.attacker.Health != h.attacker.MaxHealth {
		t.Fatalf("expected berserker hp clamped at max %d, got %d", h.attacker.MaxHealth, h.attacker.Health)
	}
}

func TestBerserkerBloodFeastBlockedAttackDoesNotTrigger(t *testing.T) {
	h := newGuardianShieldBlockCombatHarness(t, "blood-feast-blocked", newGuardianBlockMeleeAttacker("berserker"))

	payload := h.attackGuardian()
	if _, ok := findCombatEffect(payload, "lifesteal"); ok {
		t.Fatalf("expected guardian block to prevent lifesteal, got payload %+v", payload)
	}
}

func TestBerserkerBloodFeastZeroDamageAttackDoesNotTrigger(t *testing.T) {
	h := newBerserkerBloodFeastMonsterHarness(t, "blood-feast-zero-damage")
	h.monster.Defense = 40
	h.syncMonsterCell()

	payload := h.attackMonster()
	if _, ok := findCombatEffect(payload, "lifesteal"); ok {
		t.Fatalf("expected zero-damage hit not to emit lifesteal, got payload %+v", payload)
	}
	if h.rollCalls != 0 {
		t.Fatalf("expected no reflex roll for zero actual damage, got %d calls", h.rollCalls)
	}
	if h.attacker.Health != 30 {
		t.Fatalf("expected berserker hp unchanged, got %d", h.attacker.Health)
	}
}

func TestBerserkerBloodFeastFuryFollowUpDoesNotTriggerLifesteal(t *testing.T) {
	attacker := newCombatTestPlayer(1, "berserker", 1, 30, 1, 1)
	attacker.MaxHealth = 50
	attacker.Attack = 20
	attacker.Defense = 0
	attacker.Energy = 100
	attacker.MaxEnergy = 100
	attacker.Agility = 5

	target := newCombatTestPlayer(2, "ranger", 2, 80, 2, 1)
	target.Attack = 1
	target.Defense = 0
	target.Energy = 100
	target.MaxEnergy = 100

	h := newBloodFeastPlayerCombatHarness(t, "blood-feast-no-followup-lifesteal", attacker, target)
	payload := h.attackPlayer()

	if !hasCombatStep(payload, "followup") {
		t.Fatalf("expected berserker fury follow-up in control scenario, got steps %+v", payload.Steps)
	}
	if got := countCombatEffects(payload, "lifesteal"); got != 1 {
		t.Fatalf("expected only primary hit to lifesteal once, got %d effects in %+v", got, payload.Effects)
	}
	effect, _ := findCombatEffect(payload, "lifesteal")
	if effect.Amount != 10 {
		t.Fatalf("expected primary lifesteal amount 10, got %+v", effect)
	}
}

func TestBerserkerBloodFeastCounterattackDoesNotTriggerLifesteal(t *testing.T) {
	attacker := newCombatTestPlayer(1, "guardian", 1, 40, 1, 1)
	attacker.Attack = 8
	attacker.Defense = 0
	attacker.Energy = 100
	attacker.MaxEnergy = 100

	target := newCombatTestPlayer(2, "berserker", 2, 50, 2, 1)
	target.Attack = 20
	target.Defense = 0
	target.Energy = 100
	target.MaxEnergy = 100
	target.Agility = 5

	h := newBloodFeastPlayerCombatHarness(t, "blood-feast-no-counter-lifesteal", attacker, target)
	payload := h.attackPlayer()

	if !hasCombatStep(payload, "counter") {
		t.Fatalf("expected berserker counterattack in control scenario, got steps %+v", payload.Steps)
	}
	if _, ok := findCombatEffect(payload, "lifesteal"); ok {
		t.Fatalf("expected counterattack not to emit lifesteal, got payload %+v", payload)
	}
}

func TestBerserkerBloodFeastNoZeroAmountEffect(t *testing.T) {
	h := newBerserkerBloodFeastMonsterHarness(t, "blood-feast-no-zero-effect")
	h.attacker.Attack = 1

	payload := h.attackMonster()
	if _, ok := findCombatEffect(payload, "lifesteal"); ok {
		t.Fatalf("expected floor(1 * 0.5) lifesteal not to emit +0 effect, got payload %+v", payload)
	}
	if h.attacker.Health != 30 {
		t.Fatalf("expected berserker hp unchanged for zero heal, got %d", h.attacker.Health)
	}
	if h.rollCalls != 1 {
		t.Fatalf("expected reflex roll after positive actual damage, got %d calls", h.rollCalls)
	}
}

type guardianShieldBlockCombatHarness struct {
	t          *testing.T
	instanceID string
	players    map[int]models.PlayerResponse
	matchState *game.MatchState
	exchanges  []CombatExchangePayload
}

func newGuardianShieldBlockCombatHarness(t *testing.T, instanceID string, attacker models.PlayerResponse) *guardianShieldBlockCombatHarness {
	t.Helper()

	guardian := newCombatTestPlayer(2, "guardian", 0, 30, 2, 1)
	guardian.Attack = 9
	guardian.Defense = 8
	guardian.Agility = 5
	guardian.Energy = 0
	guardian.MaxEnergy = 90

	h := &guardianShieldBlockCombatHarness{
		t:          t,
		instanceID: instanceID,
		players: map[int]models.PlayerResponse{
			attacker.UserID: attacker,
			guardian.UserID: guardian,
		},
		matchState: &game.MatchState{
			InstanceID:   instanceID,
			ActiveUserID: attacker.UserID,
			TurnOrder:    []int{attacker.UserID, guardian.UserID},
			TurnNumber:   1,
		},
	}

	registerCombatMatchState(t, instanceID, h.matchState)
	h.install()

	return h
}

func (h *guardianShieldBlockCombatHarness) install() {
	h.t.Helper()

	Combat = CombatDeps{
		UpdatePlayer: func(_ string, p *models.PlayerResponse) error {
			h.players[p.UserID] = *p
			return nil
		},
		GetPlayer: func(_ string, userID int) (*models.PlayerResponse, error) {
			player, ok := h.players[userID]
			if !ok {
				h.t.Fatalf("unexpected player id %d", userID)
			}
			return &player, nil
		},
		UpdateMonsterHealth: func(_ string, _, _ int) error { return nil },
		DeleteMonster:       func(_ string, _ int) error { return nil },
		MarkPlayerDead:      func(_ string, _ int) error { return nil },
		ClearPlayerFlag: func(_ string, _ repository.Position) error {
			return nil
		},
		UpdateTurn: func(_ string, _, _ int) error { return nil },
		Finalize:   func(_ string) error { return nil },
		LoadGameState: func(_ string) (*game.MatchState, bool) {
			return h.matchState, true
		},
		LoadMap: func(_ string) ([]game.FullCell, error) { return nil, nil },
		SaveMap: func(_ string, _ []game.FullCell) error {
			return nil
		},
	}

	origRollReflexProc := rollReflexProc
	origBroadcast := broadcastFn

	rollReflexProc = func(chance int) bool {
		if chance != 10 {
			h.t.Fatalf("expected guardian reflex chance 10, got %d", chance)
		}
		return true
	}
	broadcastFn = func(message []byte) {
		var msg CombatExchangeMessage
		if err := json.Unmarshal(message, &msg); err == nil && msg.Type == "COMBAT_EXCHANGE" {
			h.exchanges = append(h.exchanges, msg.Payload)
		}
	}

	h.t.Cleanup(func() {
		RestoreDefaults()
		rollReflexProc = origRollReflexProc
		broadcastFn = origBroadcast
	})
}

func (h *guardianShieldBlockCombatHarness) attackGuardian() CombatExchangePayload {
	h.t.Helper()

	rec := httptest.NewRecorder()
	universalAttackLocked(rec, AttackRequest{
		InstanceID:   h.instanceID,
		AttackerType: "player",
		AttackerID:   1,
		TargetType:   "player",
		TargetID:     2,
	})
	if rec.Code != http.StatusOK {
		h.t.Fatalf("expected attack status 200, got %d: %s", rec.Code, rec.Body.String())
	}
	if len(h.exchanges) == 0 {
		h.t.Fatal("expected combat exchange broadcast")
	}
	return h.exchanges[len(h.exchanges)-1]
}

func newGuardianBlockMeleeAttacker(characterType string) models.PlayerResponse {
	attacker := newCombatTestPlayer(1, characterType, 0, 40, 1, 1)
	attacker.Attack = 14
	attacker.Defense = 3
	attacker.Energy = 100
	attacker.MaxEnergy = 100
	return attacker
}

func newGuardianBlockRangedAttacker(characterType string) models.PlayerResponse {
	attacker := newCombatTestPlayer(1, characterType, 0, 40, 0, 1)
	attacker.Attack = 11
	attacker.Defense = 3
	attacker.Energy = 100
	attacker.MaxEnergy = 100
	attacker.Mobility = 4
	attacker.IsRanged = true
	attacker.AttackRange = 4
	return attacker
}

func TestGuardianShieldBlockPreventsDamageAndEmitsBlockEffect(t *testing.T) {
	h := newGuardianShieldBlockCombatHarness(t, "guardian-block-no-damage", newGuardianBlockRangedAttacker("ranger"))

	payload := h.attackGuardian()
	block, ok := findCombatEffect(payload, "block")
	if !ok || !block.Succeeded || block.Target == nil || block.Target.ID != 2 {
		t.Fatalf("expected block effect for guardian, got effect=%+v ok=%v", block, ok)
	}
	if h.players[2].Health != 30 {
		t.Fatalf("expected guardian hp unchanged after block, got %d", h.players[2].Health)
	}
	if len(h.matchState.DamageEvents) != 0 {
		t.Fatalf("expected no damage events on block, got %+v", h.matchState.DamageEvents)
	}
	hit, ok := findCombatStep(payload, "hit")
	if !ok || hit.Damage != 0 || hit.TargetHPAfter != 30 {
		t.Fatalf("expected zero-damage hit step with unchanged hp, got step=%+v ok=%v", hit, ok)
	}
}

func TestGuardianShieldBlockSkipsRangerArmorBreak(t *testing.T) {
	h := newGuardianShieldBlockCombatHarness(t, "guardian-block-ranger", newGuardianBlockRangedAttacker("ranger"))

	payload := h.attackGuardian()
	if _, ok := findCombatEffect(payload, "armorBreak"); ok {
		t.Fatalf("expected block to skip ranger armor break, got payload %+v", payload)
	}
	if _, ok := findCombatEffect(payload, "push"); ok {
		t.Fatalf("expected block to skip ranger push, got payload %+v", payload)
	}
	if state := h.matchState.GetArmorBreakState("player", 2); state.Stacks != 0 {
		t.Fatalf("expected no armor break stacks on blocked hit, got %+v", state)
	}
}

func TestGuardianShieldBlockSkipsMysticDrain(t *testing.T) {
	attacker := newGuardianBlockRangedAttacker("mystic")
	h := newGuardianShieldBlockCombatHarness(t, "guardian-block-mystic", attacker)
	guardianBeforeEnergy := h.players[2].Energy

	payload := h.attackGuardian()
	if _, ok := findCombatEffect(payload, "energyDrain"); ok {
		t.Fatalf("expected block to skip mystic energy drain, got payload %+v", payload)
	}
	if _, ok := findCombatEffect(payload, "arcaneOverburn"); ok {
		t.Fatalf("expected block to skip mystic overburn, got payload %+v", payload)
	}
	if _, ok := findCombatEffect(payload, "pureDamage"); ok {
		t.Fatalf("expected block to skip mystic pure damage, got payload %+v", payload)
	}
	if h.players[2].Energy != guardianBeforeEnergy {
		t.Fatalf("expected guardian energy unchanged after blocked mystic hit, got %d", h.players[2].Energy)
	}
}

func TestGuardianShieldBlockStillAllowsMeleeCounterattack(t *testing.T) {
	h := newGuardianShieldBlockCombatHarness(t, "guardian-block-counter", newGuardianBlockMeleeAttacker("berserker"))

	payload := h.attackGuardian()
	counter, ok := findCombatStep(payload, "counter")
	if !ok || counter.Damage != 6 || counter.Target.ID != 1 {
		t.Fatalf("expected guardian counterattack after melee block, got step=%+v ok=%v", counter, ok)
	}
	if hasCombatStep(payload, "followup") {
		t.Fatalf("expected blocked berserker hit not to trigger followup, got steps %+v", payload.Steps)
	}
	if h.players[1].Health != 34 {
		t.Fatalf("expected attacker hp reduced by guardian counterattack, got %d", h.players[1].Health)
	}
	if h.players[2].Health != 30 {
		t.Fatalf("expected guardian hp unchanged after melee block, got %d", h.players[2].Health)
	}
}

func TestGuardianShieldBlockRangedAttackDoesNotCounterWhenNotAdjacent(t *testing.T) {
	h := newGuardianShieldBlockCombatHarness(t, "guardian-block-ranged-no-counter", newGuardianBlockRangedAttacker("ranger"))

	payload := h.attackGuardian()
	if hasCombatStep(payload, "counter") {
		t.Fatalf("expected non-adjacent ranged block not to counterattack, got steps %+v", payload.Steps)
	}
	if h.players[1].Health != 40 {
		t.Fatalf("expected ranged attacker hp unchanged without counterattack, got %d", h.players[1].Health)
	}
}

func TestResolveMoveEnergyCostFromPlayers_GuardianAuraBoundaryAndNoStack(t *testing.T) {
	player := newCombatTestPlayer(10, "ranger", 0, 20, 2, 0)
	player.Mobility = 4

	players := []models.PlayerResponse{
		player,
		newCombatTestPlayer(5, "guardian", 0, 20, 0, 0),
		newCombatTestPlayer(1, "guardian", 0, 20, 1, 1),
	}

	cost, extraCost := resolveMoveEnergyCostFromPlayers(&player, players)
	if cost != 4 {
		t.Fatalf("expected base cost 3 + single guardian penalty = 4, got %d", cost)
	}
	if extraCost != 1 {
		t.Fatalf("expected single extra guardian penalty 1, got %d", extraCost)
	}

	player.Position.X = 3
	player.Position.Y = 0
	cost, extraCost = resolveMoveEnergyCostFromPlayers(&player, players)
	if cost != 3 {
		t.Fatalf("expected boundary exit to drop move cost back to 3, got %d", cost)
	}
	if extraCost != 0 {
		t.Fatalf("expected no extra cost outside aura boundary, got %d", extraCost)
	}
}

func TestResolveGuardianAuraExitDamage_AccumulatesInsideZoneAndTriggersOnVoluntaryExit(t *testing.T) {
	const instanceID = "guardian-aura-voluntary-exit"

	registerCombatMatchState(t, instanceID, &game.MatchState{InstanceID: instanceID})

	guardian := newCombatTestPlayer(1, "guardian", 0, 20, 0, 0)
	target := newCombatTestPlayer(2, "ranger", 0, 10, 1, 0)
	players := []models.PlayerResponse{guardian, target}

	target.Position.X = 2
	target.Position.Y = 0
	result := resolveGuardianAuraExitDamage(
		instanceID,
		&target,
		repository.Position{X: 1, Y: 0},
		players,
		1,
		true,
	)
	if result.Triggered {
		t.Fatal("expected no damage while target remains inside guardian aura")
	}

	ms, ok := game.GetMatchState(instanceID)
	if !ok {
		t.Fatal("expected match state to exist")
	}
	if got := ms.GetGuardianAuraPressure(target.UserID); got.AccumulatedExtraMoveCost != 1 {
		t.Fatalf("expected accumulated extra cost 1 after in-zone move, got %+v", got)
	}

	target.Position.X = 3
	target.Position.Y = 0
	result = resolveGuardianAuraExitDamage(
		instanceID,
		&target,
		repository.Position{X: 2, Y: 0},
		players,
		1,
		true,
	)
	if !result.Triggered {
		t.Fatal("expected voluntary exit to trigger guardian aura damage")
	}
	if result.Damage != 2 {
		t.Fatalf("expected accumulated exit damage 2, got %d", result.Damage)
	}
	if result.NewHealth != 8 {
		t.Fatalf("expected target HP 8 after exit damage, got %d", result.NewHealth)
	}
	if result.SourceGuardianID != guardian.UserID {
		t.Fatalf("expected guardian %d as damage source, got %d", guardian.UserID, result.SourceGuardianID)
	}
	if got := ms.GetGuardianAuraPressure(target.UserID); got.AccumulatedExtraMoveCost != 0 || got.LastSourceUserID != 0 {
		t.Fatalf("expected guardian aura pressure reset after exit, got %+v", got)
	}
}

func TestResolveGuardianAuraExitDamage_ForcedExitDoesNotDamageAndResetsPressure(t *testing.T) {
	const instanceID = "guardian-aura-forced-exit"

	registerCombatMatchState(t, instanceID, &game.MatchState{InstanceID: instanceID})

	guardian := newCombatTestPlayer(1, "guardian", 0, 20, 0, 0)
	target := newCombatTestPlayer(2, "ranger", 0, 10, 1, 0)
	players := []models.PlayerResponse{guardian, target}

	target.Position.X = 2
	target.Position.Y = 0
	resolveGuardianAuraExitDamage(
		instanceID,
		&target,
		repository.Position{X: 1, Y: 0},
		players,
		1,
		true,
	)

	target.Position.X = 4
	target.Position.Y = 0
	result := resolveGuardianAuraExitDamage(
		instanceID,
		&target,
		repository.Position{X: 2, Y: 0},
		players,
		0,
		false,
	)
	if result.Triggered {
		t.Fatal("expected forced exit to skip guardian aura damage")
	}

	ms, ok := game.GetMatchState(instanceID)
	if !ok {
		t.Fatal("expected match state to exist")
	}
	if got := ms.GetGuardianAuraPressure(target.UserID); got.AccumulatedExtraMoveCost != 0 || got.LastSourceUserID != 0 {
		t.Fatalf("expected forced exit to reset guardian aura pressure, got %+v", got)
	}
}

func TestResolveGuardianAuraExitDamage_ReentryStartsFromZero(t *testing.T) {
	const instanceID = "guardian-aura-reentry"

	registerCombatMatchState(t, instanceID, &game.MatchState{InstanceID: instanceID})

	guardian := newCombatTestPlayer(1, "guardian", 0, 20, 0, 0)
	target := newCombatTestPlayer(2, "ranger", 0, 10, 1, 0)
	players := []models.PlayerResponse{guardian, target}

	target.Position.X = 3
	target.Position.Y = 0
	firstExit := resolveGuardianAuraExitDamage(
		instanceID,
		&target,
		repository.Position{X: 1, Y: 0},
		players,
		1,
		true,
	)
	if !firstExit.Triggered || firstExit.Damage != 1 {
		t.Fatalf("expected first exit to deal 1 damage, got %+v", firstExit)
	}

	target.Position.X = 2
	target.Position.Y = 0
	reentry := resolveGuardianAuraExitDamage(
		instanceID,
		&target,
		repository.Position{X: 3, Y: 0},
		players,
		0,
		true,
	)
	if reentry.Triggered {
		t.Fatalf("expected no damage on re-entry move, got %+v", reentry)
	}

	target.Position.X = 4
	target.Position.Y = 0
	secondExit := resolveGuardianAuraExitDamage(
		instanceID,
		&target,
		repository.Position{X: 2, Y: 0},
		players,
		1,
		true,
	)
	if !secondExit.Triggered || secondExit.Damage != 1 {
		t.Fatalf("expected second exit to restart from zero and deal 1 damage, got %+v", secondExit)
	}
}

func TestResolveGuardianAuraExitDamage_CapsDamageAtFive(t *testing.T) {
	const instanceID = "guardian-aura-cap"

	registerCombatMatchState(t, instanceID, &game.MatchState{InstanceID: instanceID})

	guardian := newCombatTestPlayer(1, "guardian", 0, 20, 0, 0)
	target := newCombatTestPlayer(2, "ranger", 0, 20, 1, 0)
	players := []models.PlayerResponse{guardian, target}

	for i := 0; i < 6; i++ {
		oldPos := repository.Position{X: 1, Y: 0}
		target.Position.X = 1
		target.Position.Y = 1
		if i%2 == 1 {
			oldPos = repository.Position{X: 1, Y: 1}
			target.Position.X = 1
			target.Position.Y = 0
		}

		result := resolveGuardianAuraExitDamage(
			instanceID,
			&target,
			oldPos,
			players,
			1,
			true,
		)
		if result.Triggered {
			t.Fatalf("expected no damage while accumulating inside aura, got %+v", result)
		}
	}

	target.Position.X = 4
	target.Position.Y = 0
	result := resolveGuardianAuraExitDamage(
		instanceID,
		&target,
		repository.Position{X: 1, Y: 0},
		players,
		1,
		true,
	)
	if !result.Triggered {
		t.Fatal("expected voluntary exit after large accumulation to trigger damage")
	}
	if result.Damage != guardianAuraExitDamageCap {
		t.Fatalf("expected capped exit damage %d, got %d", guardianAuraExitDamageCap, result.Damage)
	}
	if result.NewHealth != 15 {
		t.Fatalf("expected capped damage to leave target at 15 HP, got %d", result.NewHealth)
	}
}

func TestResolveGuardianAuraExitDamage_MultipleGuardiansUseDeterministicSource(t *testing.T) {
	const instanceID = "guardian-aura-multiple"

	registerCombatMatchState(t, instanceID, &game.MatchState{InstanceID: instanceID})

	guardianHighID := newCombatTestPlayer(5, "guardian", 0, 20, 0, 0)
	guardianLowID := newCombatTestPlayer(1, "guardian", 0, 20, 1, 1)
	target := newCombatTestPlayer(2, "ranger", 0, 10, 2, 0)
	target.Position.X = 4
	target.Position.Y = 0

	result := resolveGuardianAuraExitDamage(
		instanceID,
		&target,
		repository.Position{X: 2, Y: 0},
		[]models.PlayerResponse{guardianHighID, guardianLowID, target},
		1,
		true,
	)
	if !result.Triggered {
		t.Fatal("expected voluntary exit from overlapping guardian auras to trigger damage")
	}
	if result.Damage != 1 {
		t.Fatalf("expected shared non-stacking damage 1, got %d", result.Damage)
	}
	if result.SourceGuardianID != guardianLowID.UserID {
		t.Fatalf("expected lowest guardian id %d as deterministic source, got %d", guardianLowID.UserID, result.SourceGuardianID)
	}
}

func TestBuildGuardianAuraExitExchangePayload_AddsLethalDeathStep(t *testing.T) {
	payload := buildGuardianAuraExitExchangePayload("guardian-exchange", 7, 12, 5, 0)

	if payload.AttackerID != 7 || payload.AttackerType != CombatActorPlayer {
		t.Fatalf("expected guardian attacker 7/player, got %d/%s", payload.AttackerID, payload.AttackerType)
	}
	if len(payload.Steps) != 2 {
		t.Fatalf("expected auraExit + death steps, got %d", len(payload.Steps))
	}
	if payload.Steps[0].Kind != "auraExit" {
		t.Fatalf("expected first step auraExit, got %q", payload.Steps[0].Kind)
	}
	if payload.Steps[0].Damage != 5 || payload.Steps[0].TargetHPAfter != 0 {
		t.Fatalf("unexpected auraExit payload: %+v", payload.Steps[0])
	}
	if payload.Steps[1].Kind != "death" || payload.Steps[1].Target.ID != 12 {
		t.Fatalf("expected lethal aura exit death step for player 12, got %+v", payload.Steps[1])
	}
}

func registerCombatMatchState(t *testing.T, instanceID string, state *game.MatchState) {
	t.Helper()

	game.MatchStatesMu.Lock()
	game.MatchStates[instanceID] = state
	game.MatchStatesMu.Unlock()

	t.Cleanup(func() {
		game.MatchStatesMu.Lock()
		delete(game.MatchStates, instanceID)
		game.MatchStatesMu.Unlock()
	})
}

func newCombatTestPlayer(userID int, characterType string, groupID int, health int, x int, y int) models.PlayerResponse {
	player := models.PlayerResponse{
		UserID:        userID,
		CharacterType: characterType,
		GroupID:       groupID,
		Health:        health,
		MaxHealth:     health,
	}
	player.Position.X = x
	player.Position.Y = y
	return player
}
