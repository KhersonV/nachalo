// handlers/monster_death_test.go
package handlers

import (
	"encoding/json"
	"testing"

	"gameservice/game"
	"gameservice/repository"
)

// Вспомогательная структура для разбора Broadcast-сообщения
type wsMessage struct {
	Type    string                 `json:"type"`
	Payload map[string]interface{} `json:"payload"`
}

func Test_handleMonsterDeath_clearsCellAndBroadcasts(t *testing.T) {
	// 1) Подменяем Combat-зависимости
	Combat = CombatDeps{
		GetMonster: func(_ string, mid int) (*repository.MatchMonster, error) {
			return &repository.MatchMonster{MonsterInstanceID: mid, X: 5, Y: 7}, nil
		},
		DeleteMonster: func(_ string, _ int) error { return nil },
		LoadMap: func(_ string) ([]game.FullCell, error) {
			return []game.FullCell{{
				CellID:   1,
				X:        5,
				Y:        7,
				TileCode: int('M'),
				Monster:  &game.MonsterData{ID: 123},
			}}, nil
		},
		SaveMap: func(_ string, cells []game.FullCell) error {
			cell := cells[0]
			if cell.Monster != nil {
				t.Errorf("Monster should be nil after death, got %+v", cell.Monster)
			}
			if cell.TileCode != 48 {
				t.Errorf("TileCode should be 48 after death, got %d", cell.TileCode)
			}
			return nil
		},
		// остальные поля не используются, но они обязательны в структуре
		GetPlayer:           defaultCombatDeps.GetPlayer,
		UpdatePlayer:        defaultCombatDeps.UpdatePlayer,
		UpdateMonsterHealth: defaultCombatDeps.UpdateMonsterHealth,
		MarkPlayerDead:      defaultCombatDeps.MarkPlayerDead,
		ClearPlayerFlag:     defaultCombatDeps.ClearPlayerFlag,
		UpdateTurn:          defaultCombatDeps.UpdateTurn,
		Finalize:            defaultCombatDeps.Finalize,
		LoadGameState:       defaultCombatDeps.LoadGameState,
	}
	defer RestoreDefaults()

	// 2) Подменяем Broadcast, чтобы захватить сообщение
	var last []byte
    orig := broadcastFn
    broadcastFn = func(b []byte) { last = b }
    defer func() { broadcastFn = orig }()

	// 3) Вызываем
	handleMonsterDeath("match-1", 42)

	// 4) Проверяем WS-сообщение
	var msg wsMessage
	if err := json.Unmarshal(last, &msg); err != nil {
		t.Fatalf("invalid JSON broadcast: %v", err)
	}
	if msg.Type != "UPDATE_CELL" {
		t.Errorf("expected Type=UPDATE_CELL, got %q", msg.Type)
	}
	upd, ok := msg.Payload["updatedCell"].(map[string]interface{})
	if !ok {
		t.Fatalf("payload.updatedCell has wrong type: %#v", msg.Payload["updatedCell"])
	}
	if x := int(upd["x"].(float64)); x != 5 {
		t.Errorf("expected x=5, got %d", x)
	}
	if y := int(upd["y"].(float64)); y != 7 {
		t.Errorf("expected y=7, got %d", y)
	}
	if tc := int(upd["tileCode"].(float64)); tc != 48 {
		t.Errorf("expected tileCode=48, got %d", tc)
	}
}

func Test_handleMonsterDeath_noMatchingCell(t *testing.T) {
	called := false
	Combat = CombatDeps{
		GetMonster: func(_ string, _ int) (*repository.MatchMonster, error) {
			return &repository.MatchMonster{X: 1, Y: 1}, nil
		},
		DeleteMonster: func(_ string, _ int) error { return nil },
		LoadMap: func(_ string) ([]game.FullCell, error) {
			// клетка в другом месте
			return []game.FullCell{{CellID: 2, X: 2, Y: 2, TileCode: 50}}, nil
		},
		SaveMap: func(_ string, cells []game.FullCell) error {
			called = true
			if len(cells) != 1 || cells[0].TileCode != 50 {
				t.Errorf("unexpected cells in SaveMap: %+v", cells)
			}
			return nil
		},
		GetPlayer:           defaultCombatDeps.GetPlayer,
		UpdatePlayer:        defaultCombatDeps.UpdatePlayer,
		UpdateMonsterHealth: defaultCombatDeps.UpdateMonsterHealth,
		MarkPlayerDead:      defaultCombatDeps.MarkPlayerDead,
		ClearPlayerFlag:     defaultCombatDeps.ClearPlayerFlag,
		UpdateTurn:          defaultCombatDeps.UpdateTurn,
		Finalize:            defaultCombatDeps.Finalize,
		LoadGameState:       defaultCombatDeps.LoadGameState,
	}
	defer RestoreDefaults()

	handleMonsterDeath("match-1", 99)
	if !called {
		t.Error("SaveMap should be called even if no matching cell")
	}
}
