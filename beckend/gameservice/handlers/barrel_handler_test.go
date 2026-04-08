//=================================
// handlers/barrel_handler_test.go
//=================================

package handlers

import (
	"testing"

	"gameservice/game"
	"gameservice/models"
)

// helper-клетка
var testCell = game.FullCell{
	CellID:   42,
	Barbel:   &game.ResourceData{ID: 6, Type: "barrel"},
	TileCode: 'B',
}

func TestHandleOpenBarrel_DamageEvent(t *testing.T) {

	Barrel = BarrelDeps{
		GetPlayer: func(_ string, uid int) (*models.PlayerResponse, error) {
			return &models.PlayerResponse{UserID: uid, Health: 100}, nil
		},
		UpdatePlayer: func(_ string, _ *models.PlayerResponse) error { return nil },
		AddItem: func(_ string, _ int, _ string, _ int, _ string, _ string, _ string, _ int) error {
			return nil
		},
		LoadMap: func(_ string) ([]game.FullCell, error) {
			return []game.FullCell{{CellID: 42, Barbel: &game.ResourceData{ID: 6}, TileCode: 'B'}}, nil
		},
		SaveMap: func(_ string, _ []game.FullCell) error { return nil },
	}

	defer RestoreDefaults() // востановим после теста
	// 2) Заставляем rnd() вернуть 0.1 → damage
	game.SetRnd(func() float64 { return 0.1 })
	defer game.SetRnd(nil)

	// 3) Вызываем
	cell, player, ended, err := HandleOpenBarrel(
		testCell, "match-1", 123,
		nil, nil,
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// 4) Проверяем
	if ended {
		t.Errorf("expected matchEnded=false, got true")
	}
	if player.Health >= 100 {
		t.Errorf("expected Health<100 after damage, got %d", player.Health)
	}
	// при уроне Barbel в возвращённой клетке НЕ чистят
	if cell.Barbel == nil {
		t.Error("expected Barbel to remain after damage, got nil")
	}
}

func TestHandleOpenBarrel_ResourceEvent(t *testing.T) {
	// 1) Подменяем все зависимости через Barrel

	Barrel = BarrelDeps{
		GetPlayer: func(_ string, uid int) (*models.PlayerResponse, error) {
			return &models.PlayerResponse{UserID: uid, Health: 100}, nil
		},
		UpdatePlayer: func(_ string, _ *models.PlayerResponse) error { return nil },
		AddItem: func(_ string, _ int, _ string, _ int, _ string, _ string, _ string, _ int) error {
			return nil
		},
		LoadMap: func(_ string) ([]game.FullCell, error) { return []game.FullCell{testCell}, nil },
		SaveMap: func(_ string, _ []game.FullCell) error { return nil },
	}
	defer func() {
		// Восстанавливаем и зависимости, и RNG
		RestoreDefaults()
		game.SetRnd(nil)
	}()

	// 2) Стабируем rnd → попадаем в блок ресурса
	game.SetRnd(func() float64 { return 0.5 })

	// 3) Вызываем
	resources := []game.ResourceData{{ID: 2, Type: "food"}}
	updatedCell, player, ended, err := HandleOpenBarrel(
		testCell, "match-1", 123,
		resources, nil,
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// 4) Проверяем
	if ended {
		t.Error("expected matchEnded=false, got true")
	}
	if player.Health != 100 {
		t.Errorf("expected Health unchanged, got %d", player.Health)
	}
	if updatedCell.Barbel != nil {
		t.Error("expected Barbel to be removed on resource, got non-nil")
	}
}

func TestHandleOpenBarrel_ArtifactEvent(t *testing.T) {
	// 1) Подменяем через Barrel
	Barrel = BarrelDeps{
		GetPlayer: func(_ string, uid int) (*models.PlayerResponse, error) {
			return &models.PlayerResponse{UserID: uid, Health: 100}, nil
		},
		UpdatePlayer: func(_ string, _ *models.PlayerResponse) error { return nil },
		AddItem:      func(_ string, _ int, _ string, _ int, _ string, _ string, _ string, _ int) error { return nil },
		LoadMap:      func(_ string) ([]game.FullCell, error) { return []game.FullCell{testCell}, nil },
		SaveMap:      func(_ string, _ []game.FullCell) error { return nil },
	}
	// восстановим зависимости и RNG
	defer func() {
		RestoreDefaults()
		game.SetRnd(nil)
	}()

	// 2) rnd → артефакт
	game.SetRnd(func() float64 { return 0.8 })

	// 3) Вызываем
	artifacts := []game.ResourceData{{ID: 10, Type: "magic_ring"}}
	updatedCell, player, ended, err := HandleOpenBarrel(
		testCell, "match-1", 123,
		nil, artifacts,
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// 4) Проверяем
	if ended {
		t.Error("expected matchEnded=false, got true")
	}
	if player.Health != 100 {
		t.Errorf("expected Health unchanged, got %d", player.Health)
	}
	if updatedCell.Barbel != nil {
		t.Error("expected Barbel to be removed on artifact, got non-nil")
	}
}
