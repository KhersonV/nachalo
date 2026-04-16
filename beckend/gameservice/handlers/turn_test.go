package handlers

import (
	"encoding/json"
	"testing"

	"gameservice/game"
	"gameservice/models"
	"gameservice/repository"
)

func TestApplyTurretDamageToPlayer_UsesUpdatedHealthAcrossShots(t *testing.T) {
	const (
		instanceID = "match-1"
		targetID   = 77
	)

	storedHealth := 40

	Combat = CombatDeps{
		GetPlayer: func(_ string, userID int) (*models.PlayerResponse, error) {
			return &models.PlayerResponse{
				UserID: userID,
				Health: storedHealth,
			}, nil
		},
		UpdatePlayer: func(_ string, p *models.PlayerResponse) error {
			storedHealth = p.Health
			return nil
		},
		GetMonster:          defaultCombatDeps.GetMonster,
		UpdateMonsterHealth: defaultCombatDeps.UpdateMonsterHealth,
		DeleteMonster:       defaultCombatDeps.DeleteMonster,
		LoadMap:             defaultCombatDeps.LoadMap,
		SaveMap:             defaultCombatDeps.SaveMap,
		MarkPlayerDead:      defaultCombatDeps.MarkPlayerDead,
		ClearPlayerFlag:     defaultCombatDeps.ClearPlayerFlag,
		UpdateTurn:          defaultCombatDeps.UpdateTurn,
		Finalize:            defaultCombatDeps.Finalize,
		LoadGameState:       defaultCombatDeps.LoadGameState,
	}
	defer RestoreDefaults()

	target := &models.PlayerResponse{
		UserID:  targetID,
		Health:  40,
		Defense: 0,
	}
	turret := &game.FullCell{
		StructureOwnerUserID: 1,
		StructureAttack:      15,
	}

	applyTurretDamageToPlayer(instanceID, turret, target)
	applyTurretDamageToPlayer(instanceID, turret, target)

	if target.Health != 10 {
		t.Fatalf("expected local player snapshot hp=10 after two shots, got %d", target.Health)
	}
	if storedHealth != 10 {
		t.Fatalf("expected persisted player hp=10 after two shots, got %d", storedHealth)
	}
}

func TestApplyTurretDamageToMonster_BroadcastsNonLethalUpdate(t *testing.T) {
	const (
		instanceID = "match-2"
		monsterID  = 42
	)

	storedHealth := 30
	savedCells := []game.FullCell{{
		CellID:   1,
		X:        4,
		Y:        5,
		TileCode: 48,
		Monster: &game.MonsterData{
			DBInstanceID: monsterID,
			Health:       storedHealth,
			MaxHealth:    30,
		},
	}}

	Combat = CombatDeps{
		GetMonster: func(_ string, gotMonsterID int) (*repository.MatchMonster, error) {
			if gotMonsterID != monsterID {
				t.Fatalf("expected monster id %d, got %d", monsterID, gotMonsterID)
			}
			return &repository.MatchMonster{
				MonsterInstanceID: monsterID,
				X:                 4,
				Y:                 5,
				Health:            storedHealth,
				MaxHealth:         30,
				Defense:           0,
			}, nil
		},
		UpdateMonsterHealth: func(_ string, gotMonsterID, hp int) error {
			if gotMonsterID != monsterID {
				t.Fatalf("expected monster id %d, got %d", monsterID, gotMonsterID)
			}
			storedHealth = hp
			return nil
		},
		LoadMap: func(_ string) ([]game.FullCell, error) {
			cellsCopy := make([]game.FullCell, len(savedCells))
			copy(cellsCopy, savedCells)
			if cellsCopy[0].Monster != nil {
				monsterCopy := *savedCells[0].Monster
				cellsCopy[0].Monster = &monsterCopy
			}
			return cellsCopy, nil
		},
		SaveMap: func(_ string, cells []game.FullCell) error {
			savedCells = make([]game.FullCell, len(cells))
			copy(savedCells, cells)
			if cells[0].Monster != nil {
				monsterCopy := *cells[0].Monster
				savedCells[0].Monster = &monsterCopy
			}
			return nil
		},
		GetPlayer:       defaultCombatDeps.GetPlayer,
		UpdatePlayer:    defaultCombatDeps.UpdatePlayer,
		DeleteMonster:   defaultCombatDeps.DeleteMonster,
		MarkPlayerDead:  defaultCombatDeps.MarkPlayerDead,
		ClearPlayerFlag: defaultCombatDeps.ClearPlayerFlag,
		UpdateTurn:      defaultCombatDeps.UpdateTurn,
		Finalize:        defaultCombatDeps.Finalize,
		LoadGameState:   defaultCombatDeps.LoadGameState,
	}
	defer RestoreDefaults()

	var broadcasts [][]byte
	origBroadcast := broadcastFn
	broadcastFn = func(message []byte) {
		broadcasts = append(broadcasts, append([]byte(nil), message...))
	}
	defer func() { broadcastFn = origBroadcast }()

	target := &repository.MatchMonster{
		MonsterInstanceID: monsterID,
		Health:            30,
		Defense:           0,
	}
	turret := &game.FullCell{
		StructureOwnerUserID: 1,
		StructureAttack:      15,
	}

	applyTurretDamageToMonster(instanceID, turret, target)

	if target.Health != 15 {
		t.Fatalf("expected local monster snapshot hp=15, got %d", target.Health)
	}
	if storedHealth != 15 {
		t.Fatalf("expected persisted monster hp=15, got %d", storedHealth)
	}
	if len(broadcasts) != 1 {
		t.Fatalf("expected 1 broadcast for non-lethal turret hit, got %d", len(broadcasts))
	}

	var msg wsMessage
	if err := json.Unmarshal(broadcasts[0], &msg); err != nil {
		t.Fatalf("invalid JSON broadcast: %v", err)
	}
	if msg.Type != "UPDATE_CELL" {
		t.Fatalf("expected UPDATE_CELL broadcast, got %q", msg.Type)
	}

	updatedCell, ok := msg.Payload["updatedCell"].(map[string]interface{})
	if !ok {
		t.Fatalf("payload.updatedCell has wrong type: %#v", msg.Payload["updatedCell"])
	}

	monsterPayload, ok := updatedCell["monster"].(map[string]interface{})
	if !ok {
		t.Fatalf("updatedCell.monster has wrong type: %#v", updatedCell["monster"])
	}

	if hp := int(monsterPayload["health"].(float64)); hp != 15 {
		t.Fatalf("expected monster health 15 in broadcast, got %d", hp)
	}
}

func TestApplyTurretDamageToMonster_UsesUpdatedHealthAcrossShots(t *testing.T) {
	const (
		instanceID = "match-3"
		monsterID  = 99
	)

	storedHealth := 40
	savedCells := []game.FullCell{{
		CellID:   1,
		X:        2,
		Y:        2,
		TileCode: 48,
		Monster: &game.MonsterData{
			DBInstanceID: monsterID,
			Health:       storedHealth,
			MaxHealth:    40,
		},
	}}

	Combat = CombatDeps{
		GetMonster: func(_ string, gotMonsterID int) (*repository.MatchMonster, error) {
			if gotMonsterID != monsterID {
				t.Fatalf("expected monster id %d, got %d", monsterID, gotMonsterID)
			}
			return &repository.MatchMonster{
				MonsterInstanceID: monsterID,
				X:                 2,
				Y:                 2,
				Health:            storedHealth,
				MaxHealth:         40,
				Defense:           0,
			}, nil
		},
		UpdateMonsterHealth: func(_ string, gotMonsterID, hp int) error {
			if gotMonsterID != monsterID {
				t.Fatalf("expected monster id %d, got %d", monsterID, gotMonsterID)
			}
			storedHealth = hp
			return nil
		},
		LoadMap: func(_ string) ([]game.FullCell, error) {
			cellsCopy := make([]game.FullCell, len(savedCells))
			copy(cellsCopy, savedCells)
			if cellsCopy[0].Monster != nil {
				monsterCopy := *savedCells[0].Monster
				cellsCopy[0].Monster = &monsterCopy
			}
			return cellsCopy, nil
		},
		SaveMap: func(_ string, cells []game.FullCell) error {
			savedCells = make([]game.FullCell, len(cells))
			copy(savedCells, cells)
			if cells[0].Monster != nil {
				monsterCopy := *cells[0].Monster
				savedCells[0].Monster = &monsterCopy
			}
			return nil
		},
		GetPlayer:       defaultCombatDeps.GetPlayer,
		UpdatePlayer:    defaultCombatDeps.UpdatePlayer,
		DeleteMonster:   defaultCombatDeps.DeleteMonster,
		MarkPlayerDead:  defaultCombatDeps.MarkPlayerDead,
		ClearPlayerFlag: defaultCombatDeps.ClearPlayerFlag,
		UpdateTurn:      defaultCombatDeps.UpdateTurn,
		Finalize:        defaultCombatDeps.Finalize,
		LoadGameState:   defaultCombatDeps.LoadGameState,
	}
	defer RestoreDefaults()

	target := &repository.MatchMonster{
		MonsterInstanceID: monsterID,
		Health:            40,
		Defense:           0,
	}
	turretA := &game.FullCell{
		StructureOwnerUserID: 1,
		StructureAttack:      15,
	}
	turretB := &game.FullCell{
		StructureOwnerUserID: 1,
		StructureAttack:      15,
	}

	applyTurretDamageToMonster(instanceID, turretA, target)
	applyTurretDamageToMonster(instanceID, turretB, target)

	if target.Health != 10 {
		t.Fatalf("expected local monster snapshot hp=10 after two shots, got %d", target.Health)
	}
	if storedHealth != 10 {
		t.Fatalf("expected persisted monster hp=10 after two shots, got %d", storedHealth)
	}
}
