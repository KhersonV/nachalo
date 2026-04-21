//=================================
// handlers/barrel_handler_test.go
//=================================

package handlers

import (
	"testing"

	"gameservice/game"
	"gameservice/models"
	"gameservice/repository"
)

// helper-клетка
var testCell = game.FullCell{
	CellID:   42,
	Barbel:   &game.ResourceData{ID: 6, Type: "barrel"},
	TileCode: 'B',
}

func makeTestBarrelDeps(
	addItem func(instanceID string, userID int, itemType string, id int, name, image, desc string, count int) error,
) BarrelDeps {
	return BarrelDeps{
		GetPlayer: func(_ string, uid int) (*models.PlayerResponse, error) {
			return &models.PlayerResponse{UserID: uid, Health: 100}, nil
		},
		UpdatePlayer: func(_ string, _ *models.PlayerResponse) error { return nil },
		AddItem:      addItem,
		LoadMap: func(_ string) ([]game.FullCell, error) {
			return []game.FullCell{{CellID: 42, Barbel: &game.ResourceData{ID: 6}, TileCode: 'B'}}, nil
		},
		SaveMap:               func(_ string, _ []game.FullCell) error { return nil },
		SaveMapCell:           func(_ string, _ game.FullCell) error { return nil },
		GetMatch:              func(_ string) (*models.MatchInfo, error) { return &models.MatchInfo{}, nil },
		MatchHasQuestArtifact: func(_ string, _ int) (bool, error) { return false, nil },
		CountBarrels:          func(_ string) (int, error) { return 0, nil },
		GetArtifactByID:       func(_ int) (*repository.CatalogArtifact, error) { return nil, nil },
	}
}

func TestHandleOpenBarrel_DamageEvent(t *testing.T) {

	Barrel = makeTestBarrelDeps(
		func(_ string, _ int, _ string, _ int, _ string, _ string, _ string, _ int) error {
			return nil
		},
	)

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
	// при уроне бочка тоже должна исчезать
	if cell.Barbel != nil {
		t.Error("expected Barbel to be removed after damage, got non-nil")
	}
}

func TestHandleOpenBarrel_ResourceEvent(t *testing.T) {
	// 1) Подменяем все зависимости через Barrel

	Barrel = makeTestBarrelDeps(
		func(_ string, _ int, _ string, _ int, _ string, _ string, _ string, _ int) error {
			return nil
		},
	)
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
	Barrel = makeTestBarrelDeps(
		func(_ string, _ int, _ string, _ int, _ string, _ string, _ string, _ int) error { return nil },
	)
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

func TestHandleOpenBarrel_DuplicateQuestArtifactFallsBackToResource(t *testing.T) {
	added := struct {
		itemType string
		itemID   int
		name     string
		count    int
	}{}
	matchHasQuestArtifactCalls := 0

	Barrel = makeTestBarrelDeps(
		func(_ string, _ int, itemType string, id int, name, _ string, _ string, count int) error {
			added.itemType = itemType
			added.itemID = id
			added.name = name
			added.count = count
			return nil
		},
	)
	Barrel.GetMatch = func(_ string) (*models.MatchInfo, error) {
		return &models.MatchInfo{QuestArtifactID: 10}, nil
	}
	Barrel.MatchHasQuestArtifact = func(_ string, artifactID int) (bool, error) {
		matchHasQuestArtifactCalls++
		if artifactID != 10 {
			t.Fatalf("unexpected artifact id %d", artifactID)
		}
		// 1) До открытия бочки артефакт еще не найден -> он попадает в пул дропа.
		// 2) Перед выдачей выясняется, что другой игрок уже успел его подобрать.
		return matchHasQuestArtifactCalls > 1, nil
	}
	Barrel.CountBarrels = func(_ string) (int, error) { return 2, nil }

	defer func() {
		RestoreDefaults()
		game.SetRnd(nil)
	}()

	game.SetRnd(func() float64 { return 0.8 })

	resources := []game.ResourceData{{ID: 2, Type: "food"}}
	artifacts := []game.ResourceData{{ID: 10, Type: "quest_orb"}}

	updatedCell, _, ended, err := HandleOpenBarrel(
		testCell, "match-1", 123,
		resources, artifacts,
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if ended {
		t.Fatal("expected matchEnded=false, got true")
	}
	if updatedCell.Barbel != nil {
		t.Fatal("expected barrel to be removed after fallback resource")
	}
	if added.itemType != "resource" {
		t.Fatalf("expected resource item type, got %q", added.itemType)
	}
	if added.itemID != 2 || added.name != "food" || added.count != 1 {
		t.Fatalf("unexpected fallback loot: %+v", added)
	}
}
