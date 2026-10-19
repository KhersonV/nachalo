// ============================================
// gameservice/handlers/resource_collection.go
// ============================================
package handlers

import (
	"encoding/json"
	"errors"
	"net/http"

	"gameservice/middleware"
	"gameservice/repository"
)

// CollectResourceRequest – структура запроса для сбора ресурса
type CollectResourceRequest struct {
	InstanceID string `json:"instance_id"`
	PlayerID   int    `json:"user_id"`
	CommandID  string `json:"commandId"`
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
	var req CollectResourceRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request", http.StatusBadRequest)
		return
	}
	if req.InstanceID == "" || req.PlayerID == 0 || req.CommandID == "" {
		http.Error(w, "instance_id, user_id and commandId are required", http.StatusBadRequest)
		return
	}
	tokenUserID, ok := middleware.GetUserIDFromContext(r.Context())
	if !ok || tokenUserID != req.PlayerID {
		http.Error(w, "forbidden", http.StatusForbidden)
		return
	}
	if hexMode, err := repository.IsHexTestMatch(r.Context(), req.InstanceID); err != nil {
		http.Error(w, "match mode unavailable", http.StatusServiceUnavailable)
		return
	} else if hexMode {
		http.Error(w, "use the HEX command API", http.StatusBadRequest)
		return
	}
	releaseMatch, lockErr := acquireMatchSerialization(r.Context(), req.InstanceID)
	if lockErr != nil {
		http.Error(w, "match lock unavailable", http.StatusServiceUnavailable)
		return
	}
	defer releaseMatch()

	result, err := repository.ApplyResourceCollection(r.Context(), req.InstanceID, req.PlayerID, req.CommandID, req.CellX, req.CellY)
	if err != nil {
		status := http.StatusInternalServerError
		if errors.Is(err, repository.ErrCommandConflict) {
			status = http.StatusConflict
		}
		if errors.Is(err, repository.ErrCommandRejected) {
			status = http.StatusBadRequest
		}
		http.Error(w, err.Error(), status)
		return
	}
	updatedCell := serialiseUpdatedCell(result.Cell)
	playerResp, err := repository.GetMatchPlayerByID(req.InstanceID, req.PlayerID)
	if err != nil {
		http.Error(w, "committed; reload match snapshot", http.StatusServiceUnavailable)
		return
	}
	playerResp.Inventory = result.Inventory
	if !result.Replayed {
		payload := map[string]interface{}{
			"instanceId":    req.InstanceID,
			"updatedCell":   updatedCell,
			"updatedPlayer": playerResp,
			"commandId":     req.CommandID,
		}
		change := map[string]interface{}{"type": "RESOURCE_COLLECTED", "payload": payload}
		patch := map[string]interface{}{"type": "MATCH_PATCH", "payload": map[string]interface{}{
			"instanceId":    req.InstanceID,
			"schemaVersion": 1,
			"stateRevision": result.StateRevision,
			"mapRevision":   result.MapRevision,
			"commandId":     req.CommandID,
			"changes":       []interface{}{change},
		}}
		if b, e := json.Marshal(patch); e == nil {
			Broadcast(b)
		}
		if b, e := json.Marshal(change); e == nil {
			Broadcast(b)
		}
		if result.ItemType == "artifact" {
			var questArtifactID int
			if e := repository.DB.QueryRow(`SELECT COALESCE(quest_artifact_id,0) FROM matches WHERE instance_id=$1`, req.InstanceID).Scan(&questArtifactID); e == nil && questArtifactID == result.ItemID {
				msg := map[string]interface{}{"type": "QUEST_ARTIFACT_FOUND", "payload": map[string]interface{}{
					"instanceId": req.InstanceID, "playerName": playerResp.Name, "x": updatedCell.X, "y": updatedCell.Y,
				}}
				if b, e := json.Marshal(msg); e == nil {
					Broadcast(b)
				}
			}
		}
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]interface{}{"message": "успешно", "updatedCell": updatedCell, "updatedPlayer": playerResp})
}
