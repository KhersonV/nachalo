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
