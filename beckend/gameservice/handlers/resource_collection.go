// ============================================
// gameservice/handlers/resource_collection.go
// ============================================
package handlers

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"gameservice/game"
	"gameservice/middleware"
	"gameservice/repository"
)

// CollectResourceRequest – структура запроса для сбора ресурса
type CollectResourceRequest struct {
	InstanceID string `json:"instance_id"`
	PlayerID   int    `json:"user_id"`
	CellX      int    `json:"cell_x"`
	CellY      int    `json:"cell_y"`
}

// UpdatedCellResponse – структура ответа для обновлённой клетки.
type UpdatedCellResponse struct {
	CellID                int         `json:"cell_id"`
	X                     int         `json:"x"`
	Y                     int         `json:"y"`
	TileCode              int         `json:"tileCode"`
	Resource              interface{} `json:"resource"`
	Barbel                interface{} `json:"barbel"`
	Monster               interface{} `json:"monster"`
	IsPortal              bool        `json:"isPortal"`
	IsPlayer              bool        `json:"isPlayer"`
	StructureType         string      `json:"structure_type,omitempty"`
	StructureOwnerUserID  int         `json:"structure_owner_user_id,omitempty"`
	StructureHealth       int         `json:"structure_health,omitempty"`
	StructureDefense      int         `json:"structure_defense,omitempty"`
	StructureAttack       int         `json:"structure_attack,omitempty"`
	StructureImage        string      `json:"structure_image,omitempty"`
	IsUnderConstruction   bool        `json:"is_under_construction"`
	ConstructionTurnsLeft int         `json:"construction_turns_left,omitempty"`
}

// CollectResourceHandler обрабатывает и сбор ресурсов, и открытие бочек.
func CollectResourceHandler(w http.ResponseWriter, r *http.Request) {
	// 1) Чтение и парсинг запроса
	body, _ := io.ReadAll(r.Body)
	r.Body = io.NopCloser(bytes.NewReader(body))

	var req CollectResourceRequest
	if err := json.Unmarshal(body, &req); err != nil {
		http.Error(w, fmt.Sprintf("парсинг запроса: %v", err), http.StatusBadRequest)
		return
	}
	if req.InstanceID == "" || req.PlayerID == 0 {
		http.Error(w, "instance_id и user_id обязательны", http.StatusBadRequest)
		return
	}
	tokenUserID, ok := middleware.GetUserIDFromContext(r.Context())
	if !ok || tokenUserID != req.PlayerID {
		http.Error(w, "forbidden", http.StatusForbidden)
		return
	}

	lockPlayer(req.PlayerID)
	defer unlockPlayer(req.PlayerID)

	matchState, ok := game.GetMatchState(req.InstanceID)
	if !ok {
		http.Error(w, "match not found", http.StatusNotFound)
		return
	}
	if matchState.ActiveUserID != req.PlayerID {
		http.Error(w, "it's not your turn", http.StatusBadRequest)
		return
	}

	// 2) Загружаем только нужную клетку и блокируем её на время сбора
	tx, err := repository.DB.Begin()
	if err != nil {
		http.Error(w, fmt.Sprintf("begin tx: %v", err), http.StatusInternalServerError)
		return
	}
	defer tx.Rollback()

	targetCell, err := repository.LoadMapCellForUpdateTx(tx, req.InstanceID, req.CellX, req.CellY)
	if err != nil {
		http.Error(w, fmt.Sprintf("загрузка клетки: %v", err), http.StatusInternalServerError)
		return
	}
	if targetCell == nil {
		http.Error(w, "клетка не найдена", http.StatusBadRequest)
		return
	}

	// 3) Проверяем, есть ли ресурс в этой ячейке
	if targetCell.Resource == nil {
		http.Error(w, "resource already collected", http.StatusConflict)
		return
	}

	// Сохраняем нужные поля ресурса ДО очистки клетки
	itemID := targetCell.Resource.ID
	itemName := targetCell.Resource.Type
	itemDesc := targetCell.Resource.Description
	itemImage := targetCell.Resource.Image
	// item_type: "artifact" если это дроп артефакта, иначе "resource"
	itemType := "resource"
	if targetCell.Resource.ItemType == "artifact" {
		itemType = "artifact"
	}

	// 4) Очищаем клетку
	targetCell.Resource = nil
	targetCell.TileCode = 48
	targetCell.Monster = nil
	targetCell.IsPortal = false

	// 5) Сохраняем обновлённую клетку в транзакции
	if err := repository.SaveMapCellTx(tx, req.InstanceID, *targetCell); err != nil {
		http.Error(w, fmt.Sprintf("сохранение клетки: %v", err), http.StatusInternalServerError)
		return
	}

	if err := tx.Commit(); err != nil {
		http.Error(w, fmt.Sprintf("commit tx: %v", err), http.StatusInternalServerError)
		return
	}

	if err := repository.AddInventoryItem(
		req.InstanceID,
		req.PlayerID,
		itemType, // "resource" или "artifact"
		itemID,
		itemName,
		itemImage,
		itemDesc,
		1,
	); err != nil {
		http.Error(w, fmt.Sprintf("добавление в инвентарь: %v", err), http.StatusInternalServerError)
		return
	}
	// 7) Формируем и отправляем ответ клиенту
	updatedCell := serialiseUpdatedCell(*targetCell)
	playerResp, _ := repository.GetMatchPlayerByID(req.InstanceID, req.PlayerID)

	wsMsg := map[string]interface{}{
		"type": "RESOURCE_COLLECTED",
		"payload": map[string]interface{}{
			"instanceId":    req.InstanceID,
			"updatedCell":   updatedCell,
			"updatedPlayer": playerResp,
		},
	}
	msgBuf, _ := json.Marshal(wsMsg)
	Broadcast(msgBuf)

	// Если собранный предмет — квест-артефакт, уведомляем всех игроков
	if itemType == "artifact" {
		var questArtifactID int
		repository.DB.QueryRow(
			`SELECT COALESCE(quest_artifact_id, 0) FROM matches WHERE instance_id=$1`,
			req.InstanceID,
		).Scan(&questArtifactID)

		if questArtifactID != 0 && itemID == questArtifactID {
			questMsg := map[string]interface{}{
				"type": "QUEST_ARTIFACT_FOUND",
				"payload": map[string]interface{}{
					"instanceId": req.InstanceID,
					"playerName": playerResp.Name,
					"x":          updatedCell.X,
					"y":          updatedCell.Y,
				},
			}
			questBuf, _ := json.Marshal(questMsg)
			Broadcast(questBuf)
		}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"message":       "успешно",
		"updatedCell":   updatedCell,
		"updatedPlayer": playerResp,
	})
}
