
//=====================================
// /gameservice/handlers/deps.go
//=====================================

package handlers

import (
	"gameservice/game"
	"gameservice/models"
	"gameservice/repository"
	"gameservice/service"
)

// BarrelDeps — все зависимости, используемые в barrel.go
type BarrelDeps struct {
	GetPlayer       func(instanceID string, userID int) (*models.PlayerResponse, error)
	UpdatePlayer    func(*models.PlayerResponse) error
	AddItem         func(instanceID string, userID int, itemType string, id int, name, image, desc string, count int) error
	LoadMap         func(instanceID string) ([]game.FullCell, error)
	SaveMap         func(instanceID string, cells []game.FullCell) error
}

// CombatDeps — все зависимости, используемые в combat.go
type CombatDeps struct {
	LoadMap              func(instanceID string) ([]game.FullCell, error)
    SaveMap              func(instanceID string, cells []game.FullCell) error
	UpdatePlayer         func(*models.PlayerResponse) error 
	GetPlayer            func(instanceID string, userID int) (*models.PlayerResponse, error)
	GetMonster           func(instanceID string, monsterID int) (*repository.MatchMonster, error)
	UpdateMonsterHealth  func(instanceID string, monsterID, hp int) error
	DeleteMonster        func(instanceID string, monsterID int) error
	MarkPlayerDead       func(instanceID string, userID int) error
	ClearPlayerFlag      func(instanceID string, pos repository.Position) error
	UpdateTurn           func(instanceID string, nextUserID, turnNum int) error
	Finalize             func(instanceID string) error
	LoadGameState        func(instanceID string) (*game.MatchState, bool)
}

// defaultDeps содержит реальные реализации
var (
	defaultBarrelDeps = BarrelDeps{
		GetPlayer:    repository.GetMatchPlayerByID,
		UpdatePlayer: repository.UpdateMatchPlayer,
		AddItem:      repository.AddInventoryItem,
		LoadMap:      repository.LoadMapCells,
		SaveMap:      repository.SaveMapCells,
	}

	defaultCombatDeps = CombatDeps{
		LoadMap:			 repository.LoadMapCells,
		SaveMap:			 repository.SaveMapCells,
		UpdatePlayer: 		 repository.UpdateMatchPlayer,
		GetPlayer:           repository.GetMatchPlayerByID,
		GetMonster:          repository.GetMatchMonsterByID,
		UpdateMonsterHealth: repository.UpdateMatchMonsterHealth,
		DeleteMonster:       repository.DeleteMatchMonster,
		MarkPlayerDead:      repository.MarkPlayerDead,
		ClearPlayerFlag:     repository.ClearCellPlayerFlag,
		UpdateTurn:          repository.UpdateMatchTurn,
		Finalize:            service.FinalizeMatch,
		LoadGameState:       game.GetMatchState,
	}
)

// Barrel и Combat — пакеты могут брать из них актуальные зависимости.
// Для тестов удобно делать так:
var (
	Barrel BarrelDeps = defaultBarrelDeps
	Combat CombatDeps = defaultCombatDeps
)

// RestoreDefaults возвращает оригинальные реализации (для defer в тестах)
func RestoreDefaults() {
	Barrel = defaultBarrelDeps
	Combat = defaultCombatDeps
}

// Стабы для barrel.go

func stubGetMatchPlayerByID(instanceID string, userID int) (*models.PlayerResponse, error) {
	return &models.PlayerResponse{
		UserID: userID,
		Health: 100,
	}, nil
}
func stubUpdateMatchPlayer(_ *models.PlayerResponse) error { return nil }
func stubAddInventoryItem(_ string, _ int, _ string, _ int, _ string, _ string, _ string, _ int) error {
	return nil
}
func stubLoadMapCells(_ string) ([]game.FullCell, error) {
	// Именно этот CellID ищет broadcastCellRemoval
	return []game.FullCell{
		{CellID: 42, Barbel: &game.ResourceData{ID: 6}, TileCode: 'B'},
	}, nil
}
func stubSaveMapCells(_ string, _ []game.FullCell) error { return nil }

// Функция для восстановления оригинальных зависимостей (barrel)
