package handlers

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"gameservice/game"
	"gameservice/middleware"
	"gameservice/models"
	"gameservice/repository"
)

type blueprintPlacementRequest struct {
	InstanceID   string `json:"instance_id"`
	UserID       int    `json:"user_id"`
	CellX        int    `json:"cell_x"`
	CellY        int    `json:"cell_y"`
	BlueprintKey string `json:"blueprint_key"`
}

type structureStats struct {
	StructureType string
	Health        int
	Defense       int
	Attack        int
}

func resolveBlueprintStats(key string) (structureStats, int, error) {
	switch strings.TrimSpace(strings.ToLower(key)) {
	case "blueprint_scout_tower":
		return structureStats{StructureType: "scout_tower", Health: 30, Defense: 5, Attack: 0}, 1001, nil
	case "blueprint_turret":
		return structureStats{StructureType: "turret", Health: 30, Defense: 0, Attack: 10}, 1002, nil
	case "blueprint_wall":
		return structureStats{StructureType: "wall", Health: 30, Defense: 8, Attack: 0}, 1003, nil
	default:
		return structureStats{}, 0, fmt.Errorf("неизвестный чертеж")
	}
}

func manhattan(x1, y1, x2, y2 int) int {
	dx := x1 - x2
	if dx < 0 {
		dx = -dx
	}
	dy := y1 - y2
	if dy < 0 {
		dy = -dy
	}
	return dx + dy
}

func isBuildableOrdinaryCell(cell game.FullCell) bool {
	if cell.TileCode != 48 {
		return false
	}
	if cell.Resource != nil || cell.Barbel != nil || cell.Monster != nil {
		return false
	}
	if cell.IsPortal || cell.IsPlayer {
		return false
	}
	if cell.StructureType != "" || cell.IsUnderConstruction {
		return false
	}
	return true
}

func decrementInventoryBlueprint(player *models.PlayerResponse, blueprintKey string) error {
	inv := make(map[string]map[string]interface{})
	if player.Inventory != "" && player.Inventory != "{}" {
		if err := json.Unmarshal([]byte(player.Inventory), &inv); err != nil {
			inv = make(map[string]map[string]interface{})
		}
	}

	entry, ok := inv[blueprintKey]
	if !ok {
		return fmt.Errorf("чертеж не найден в инвентаре")
	}

	countAny, ok := entry["item_count"]
	if !ok {
		return fmt.Errorf("некорректная запись инвентаря")
	}

	count := 0
	switch v := countAny.(type) {
	case float64:
		count = int(v)
	case int:
		count = v
	case int64:
		count = int(v)
	default:
		return fmt.Errorf("некорректное количество в инвентаре")
	}

	if count <= 0 {
		return fmt.Errorf("чертеж закончился")
	}

	if count == 1 {
		delete(inv, blueprintKey)
	} else {
		entry["item_count"] = count - 1
		inv[blueprintKey] = entry
	}

	raw, err := json.Marshal(inv)
	if err != nil {
		return err
	}
	player.Inventory = string(raw)
	return nil
}

// PlaceBlueprintHandler запускает строительство на соседней свободной клетке.
func PlaceBlueprintHandler(w http.ResponseWriter, r *http.Request) {
	tokenUserID, ok := middleware.GetUserIDFromContext(r.Context())
	if !ok {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	var req blueprintPlacementRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request", http.StatusBadRequest)
		return
	}
	if req.InstanceID == "" || req.UserID == 0 || req.BlueprintKey == "" {
		http.Error(w, "instance_id, user_id и blueprint_key обязательны", http.StatusBadRequest)
		return
	}
	if req.UserID != tokenUserID {
		http.Error(w, "forbidden", http.StatusForbidden)
		return
	}

	matchState, ok := game.GetMatchState(req.InstanceID)
	if !ok {
		http.Error(w, "match not found", http.StatusNotFound)
		return
	}
	if matchState.ActiveUserID != req.UserID {
		http.Error(w, "сейчас не ваш ход", http.StatusBadRequest)
		return
	}

	stats, _, err := resolveBlueprintStats(req.BlueprintKey)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	player, err := repository.GetMatchPlayerByID(req.InstanceID, req.UserID)
	if err != nil {
		http.Error(w, "player not found", http.StatusNotFound)
		return
	}

	if manhattan(player.Position.X, player.Position.Y, req.CellX, req.CellY) != 1 {
		http.Error(w, "строить можно только в соседней клетке", http.StatusBadRequest)
		return
	}

	cells, err := repository.LoadMapCells(req.InstanceID)
	if err != nil {
		http.Error(w, "failed to load map", http.StatusInternalServerError)
		return
	}

	cellIdx := -1
	for i := range cells {
		if cells[i].X == req.CellX && cells[i].Y == req.CellY {
			cellIdx = i
			break
		}
	}
	if cellIdx == -1 {
		http.Error(w, "клетка не найдена", http.StatusBadRequest)
		return
	}
	if !isBuildableOrdinaryCell(cells[cellIdx]) {
		http.Error(w, "строить можно только на обычной свободной клетке", http.StatusBadRequest)
		return
	}

	if err := decrementInventoryBlueprint(player, req.BlueprintKey); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	cells[cellIdx].StructureType = stats.StructureType
	cells[cellIdx].StructureOwnerUserID = req.UserID
	cells[cellIdx].StructureHealth = stats.Health
	cells[cellIdx].StructureDefense = stats.Defense
	cells[cellIdx].StructureAttack = stats.Attack
	cells[cellIdx].IsUnderConstruction = true
	cells[cellIdx].ConstructionTurnsLeft = 2

	if err := repository.SaveMapCells(req.InstanceID, cells); err != nil {
		http.Error(w, "failed to save map", http.StatusInternalServerError)
		return
	}
	if err := repository.UpdateMatchPlayerInventory(req.InstanceID, player.UserID, player.Inventory); err != nil {
	http.Error(w, "failed to update player inventory", http.StatusInternalServerError)
	return
}

	updatedCell := serialiseUpdatedCell(cells[cellIdx])
	cellMsg := map[string]interface{}{
		"type": "UPDATE_CELL",
		"payload": map[string]interface{}{
			"instanceId":  req.InstanceID,
			"updatedCell": updatedCell,
		},
	}
	cellBuf, _ := json.Marshal(cellMsg)
	Broadcast(cellBuf)

	playerMsg := map[string]interface{}{
		"type": "UPDATE_PLAYER",
		"payload": map[string]interface{}{
			"instanceId": req.InstanceID,
			"player":     player,
		},
	}
	playerBuf, _ := json.Marshal(playerMsg)
	Broadcast(playerBuf)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"updatedCell":   updatedCell,
		"updatedPlayer": player,
	})
}
