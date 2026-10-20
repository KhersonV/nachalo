package handlers

import (
	"encoding/json"
	"errors"
	"net/http"

	"gameservice/internal/hexgrid"
	"gameservice/middleware"
	"gameservice/repository"

	"github.com/gorilla/mux"
)

func hexCommandStatus(err error) int {
	if errors.Is(err, repository.ErrCommandConflict) {
		return http.StatusConflict
	}
	if errors.Is(err, repository.ErrCommandRejected) || errors.Is(err, repository.ErrHexUnavailable) {
		return http.StatusBadRequest
	}
	if errors.Is(err, repository.ErrHexInteractionPosition) {
		return http.StatusBadRequest
	}
	return http.StatusInternalServerError
}

func publishHexPatch(instanceID, commandID string, state, mapRevision int64, changeType string, result any) {
	patch := map[string]any{"type": "MATCH_PATCH", "payload": map[string]any{"instanceId": instanceID, "schemaVersion": 2, "geometry": map[string]any{"kind": "hex", "coordinates": "axial", "orientation": "pointy", "version": 1}, "stateRevision": state, "mapRevision": mapRevision, "commandId": commandID, "changes": []any{map[string]any{"type": changeType, "payload": result}}}}
	if data, err := json.Marshal(patch); err == nil {
		Broadcast(data)
	}
}

func hexUser(w http.ResponseWriter, r *http.Request) (int, bool) {
	userID, ok := middleware.GetUserIDFromContext(r.Context())
	if !ok || userID <= 0 {
		http.Error(w, "authentication required", http.StatusUnauthorized)
		return 0, false
	}
	return userID, true
}

func CreateHexTestMatchHandler(w http.ResponseWriter, r *http.Request) {
	userID, ok := hexUser(w, r)
	if !ok {
		return
	}
	var req struct {
		Seed int64 `json:"seed"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid JSON", 400)
		return
	}
	snapshot, err := repository.CreateHexTestMatch(r.Context(), userID, req.Seed)
	if err != nil {
		http.Error(w, err.Error(), 500)
		return
	}
	scheduleHexTurn(snapshot.InstanceID, userID, snapshot.TurnNumber)
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	_ = json.NewEncoder(w).Encode(snapshot)
}

func GetHexTestMatchHandler(w http.ResponseWriter, r *http.Request) {
	userID, ok := hexUser(w, r)
	if !ok {
		return
	}
	snapshot, err := repository.LoadHexTestMatch(r.Context(), mux.Vars(r)["instance_id"], userID)
	if err != nil {
		http.Error(w, "HEX match unavailable", 404)
		return
	}
	scheduleHexTurn(snapshot.InstanceID, snapshot.ActiveUser, snapshot.TurnNumber)
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(snapshot)
}

func PreviewHexRouteHandler(w http.ResponseWriter, r *http.Request) {
	userID, ok := hexUser(w, r)
	if !ok {
		return
	}
	var req struct {
		Destination hexgrid.Coord `json:"destination"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid JSON", 400)
		return
	}
	route, err := repository.PreviewHexRoute(r.Context(), mux.Vars(r)["instance_id"], userID, req.Destination)
	if err != nil {
		http.Error(w, err.Error(), repository.HexMoveErrorStatus(err))
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(route)
}

func MoveHexTestMatchHandler(w http.ResponseWriter, r *http.Request) {
	userID, ok := hexUser(w, r)
	if !ok {
		return
	}
	var req struct {
		CommandID   string        `json:"commandId"`
		Destination hexgrid.Coord `json:"destination"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid JSON", 400)
		return
	}
	if req.CommandID == "" || len(req.CommandID) > 128 {
		http.Error(w, "commandId is required", 400)
		return
	}
	instanceID := mux.Vars(r)["instance_id"]
	result, err := repository.ApplyHexMoveWithPublish(r.Context(), instanceID, userID, req.CommandID, req.Destination, func(committed repository.HexRouteResult) {
		publishHexPatch(instanceID, req.CommandID, committed.StateRevision, committed.MapRevision, "MOVE_PLAYER", map[string]any{"instanceId": instanceID, "userId": userID, "newPosition": committed.Position})
	})
	if err != nil {
		http.Error(w, err.Error(), repository.HexMoveErrorStatus(err))
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(result)
}

func CollectHexResourceHandler(w http.ResponseWriter, r *http.Request) {
	userID, ok := hexUser(w, r)
	if !ok {
		return
	}
	var req struct {
		CommandID string        `json:"commandId"`
		Position  hexgrid.Coord `json:"position"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.CommandID == "" || len(req.CommandID) > 128 {
		http.Error(w, "commandId and position required", 400)
		return
	}
	id := mux.Vars(r)["instance_id"]
	result, err := repository.ApplyHexResourceCollection(r.Context(), id, userID, req.CommandID, req.Position, func(committed repository.HexCollectResult) {
		publishHexPatch(id, req.CommandID, committed.StateRevision, committed.MapRevision, "RESOURCE_COLLECTED", map[string]any{"instanceId": id, "userId": userID, "position": committed.Position, "itemType": committed.ItemType})
	})
	if err != nil {
		http.Error(w, err.Error(), hexCommandStatus(err))
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(result)
}

func EndHexTurnHandler(w http.ResponseWriter, r *http.Request) {
	userID, ok := hexUser(w, r)
	if !ok {
		return
	}
	var req struct {
		CommandID    string `json:"commandId"`
		ExpectedTurn int    `json:"expectedTurn"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.CommandID == "" || len(req.CommandID) > 128 || req.ExpectedTurn < 1 {
		http.Error(w, "commandId and expectedTurn required", 400)
		return
	}
	id := mux.Vars(r)["instance_id"]
	result, err := repository.ApplyHexEndTurn(r.Context(), id, userID, req.CommandID, req.ExpectedTurn, resolveCharacterRegen, func(committed repository.HexTurnResult) {
		publishHexPatch(id, req.CommandID, committed.StateRevision, committed.MapRevision, "TURN_PASSED", map[string]any{"instanceId": id, "userId": committed.CompletedUser, "active_user": committed.ActiveUser, "turnNumber": committed.TurnNumber})
	})
	if err != nil {
		http.Error(w, err.Error(), hexCommandStatus(err))
		return
	}
	if !result.Replayed {
		scheduleHexTurn(id, result.ActiveUser, result.TurnNumber)
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(result)
}
