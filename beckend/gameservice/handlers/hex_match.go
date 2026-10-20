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
	if errors.Is(err, repository.ErrHexPortalPosition) || errors.Is(err, repository.ErrHexExtractionMethod) || errors.Is(err, repository.ErrStorageInsufficientGold) {
		return http.StatusBadRequest
	}
	if errors.Is(err, repository.ErrHexFoodCooldown) || errors.Is(err, repository.ErrHexWaterLimit) || errors.Is(err, repository.ErrHexNoTarget) {
		return http.StatusBadRequest
	}
	return http.StatusInternalServerError
}

func publishHexPatch(instanceID, commandID string, state, mapRevision int64, changeType string, result any) {
	publishHexPatchChanges(instanceID, commandID, state, mapRevision, []any{map[string]any{"type": changeType, "payload": result}})
}

func publishHexPatchChanges(instanceID, commandID string, state, mapRevision int64, changes []any) {
	// Every client fetches its own filtered snapshot after this revision notice.
	// Command payloads may contain hidden positions or inventory, so do not
	// broadcast them to the other members of the match.
	safeChanges := []any{}
	for _, raw := range changes {
		change, ok := raw.(map[string]any)
		if ok && change["type"] == "TURN_TIMEOUT" {
			safeChanges = append(safeChanges, map[string]any{"type": "TURN_TIMEOUT"})
		}
	}
	patch := map[string]any{"type": "MATCH_PATCH", "payload": map[string]any{"instanceId": instanceID, "schemaVersion": 2, "geometry": map[string]any{"kind": "hex", "coordinates": "axial", "orientation": "pointy", "version": 1}, "stateRevision": state, "mapRevision": mapRevision, "commandId": commandID, "changes": safeChanges}}
	if data, err := json.Marshal(patch); err == nil {
		Broadcast(data)
	}
}

// Visual notices contain only committed combat facts. The socket layer checks
// each recipient's current HEX visibility before sending them.
func publishHexVisual(instanceID, commandID string, revision int64, kind string, visual map[string]any) {
	visual["instanceId"] = instanceID
	visual["schemaVersion"] = 2
	visual["commandId"] = commandID
	visual["stateRevision"] = revision
	visual["kind"] = kind
	if data, err := json.Marshal(map[string]any{"type": "MATCH_HEX_VISUAL", "payload": visual}); err == nil {
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

func rejectLegacyHexAction(w http.ResponseWriter, instanceID string) bool {
	hex, err := repository.IsHexMatch(instanceID)
	if err != nil {
		http.Error(w, "match unavailable", http.StatusNotFound)
		return true
	}
	if hex {
		http.Error(w, "This Square action is unavailable in HEX; use the HEX match action", http.StatusBadRequest)
		return true
	}
	return false
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
	if !snapshot.Finished {
		scheduleHexTurn(snapshot.InstanceID, snapshot.ActiveUser, snapshot.TurnNumber)
	}
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
		if committed.AuraEnergyCost > 0 || len(committed.AuraHits) > 0 || committed.AuraEnded {
			publishHexVisual(instanceID, req.CommandID, committed.StateRevision, "move", map[string]any{"userId": userID, "position": committed.Position, "path": committed.Path, "auraEnergyCost": committed.AuraEnergyCost, "auraHits": committed.AuraHits, "auraEnded": committed.AuraEnded})
		}
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
		changes := []any{map[string]any{"type": "RESOURCE_COLLECTED", "payload": map[string]any{"instanceId": id, "userId": userID, "position": committed.Position, "itemType": committed.ItemType}}}
		if committed.ItemType == "artifact" {
			changes = append(changes, map[string]any{"type": "QUEST_ARTIFACT_FOUND", "payload": map[string]any{"instanceId": id, "userId": userID, "playerName": committed.PlayerName, "position": committed.Position}})
		}
		publishHexPatchChanges(id, req.CommandID, committed.StateRevision, committed.MapRevision, changes)
	})
	if err != nil {
		http.Error(w, err.Error(), hexCommandStatus(err))
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(result)
}

func OpenHexBarrelHandler(w http.ResponseWriter, r *http.Request) {
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
	result, err := repository.ApplyHexBarrel(r.Context(), id, userID, req.CommandID, req.Position, func(committed repository.HexBarrelResult) {
		changes := []any{map[string]any{"type": "BARREL_OPENED", "payload": map[string]any{"instanceId": id, "userId": userID, "position": committed.Position, "outcome": committed.Outcome}}}
		if committed.QuestFound {
			changes = append(changes, map[string]any{"type": "QUEST_ARTIFACT_FOUND", "payload": map[string]any{"instanceId": id, "userId": userID, "playerName": committed.PlayerName, "position": committed.Position}})
		}
		publishHexPatchChanges(id, req.CommandID, committed.StateRevision, committed.MapRevision, changes)
	})
	if err != nil {
		http.Error(w, err.Error(), hexCommandStatus(err))
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(result)
}

func UseHexItemHandler(w http.ResponseWriter, r *http.Request) {
	userID, ok := hexUser(w, r)
	if !ok {
		return
	}
	var req struct {
		CommandID string `json:"commandId"`
		ItemType  string `json:"itemType"`
		ItemID    int    `json:"itemId"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.CommandID == "" || len(req.CommandID) > 128 {
		http.Error(w, "commandId and item required", 400)
		return
	}
	id := mux.Vars(r)["instance_id"]
	result, err := repository.ApplyHexItem(r.Context(), id, userID, req.CommandID, req.ItemType, req.ItemID, func(committed repository.HexItemResult) {
		publishHexPatch(id, req.CommandID, committed.StateRevision, committed.MapRevision, "ITEM_USED", map[string]any{"instanceId": id, "userId": userID, "itemType": committed.ItemType, "itemId": committed.ItemID})
	})
	if err != nil {
		http.Error(w, err.Error(), hexCommandStatus(err))
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(result)
}

func PlaceHexBlueprintHandler(w http.ResponseWriter, r *http.Request) {
	userID, ok := hexUser(w, r)
	if !ok {
		return
	}
	var req struct {
		CommandID    string        `json:"commandId"`
		BlueprintKey string        `json:"blueprintKey"`
		Position     hexgrid.Coord `json:"position"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.CommandID == "" || len(req.CommandID) > 128 {
		http.Error(w, "commandId, blueprintKey and position required", http.StatusBadRequest)
		return
	}
	id := mux.Vars(r)["instance_id"]
	result, err := repository.ApplyHexBlueprint(r.Context(), id, userID, req.CommandID, req.BlueprintKey, req.Position, func(committed repository.HexBuildResult) {
		publishHexPatch(id, req.CommandID, committed.StateRevision, committed.MapRevision, "BLUEPRINT_PLACED", nil)
	})
	if err != nil {
		http.Error(w, err.Error(), hexCommandStatus(err))
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(result)
}

func AttackHexStructureHandler(w http.ResponseWriter, r *http.Request) {
	userID, ok := hexUser(w, r)
	if !ok {
		return
	}
	var req struct {
		CommandID string        `json:"commandId"`
		Position  hexgrid.Coord `json:"position"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.CommandID == "" || len(req.CommandID) > 128 {
		http.Error(w, "commandId and position required", http.StatusBadRequest)
		return
	}
	id := mux.Vars(r)["instance_id"]
	result, err := repository.ApplyHexStructureAttack(r.Context(), id, userID, req.CommandID, req.Position, func(committed repository.HexStructureAttackResult) {
		publishHexPatch(id, req.CommandID, committed.StateRevision, committed.MapRevision, "STRUCTURE_ATTACKED", nil)
	})
	if err != nil {
		http.Error(w, err.Error(), hexCommandStatus(err))
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(result)
}

func ExtractHexMatchHandler(w http.ResponseWriter, r *http.Request) {
	userID, ok := hexUser(w, r)
	if !ok {
		return
	}
	var req struct {
		CommandID     string `json:"commandId"`
		Method        string `json:"method"`
		ConfirmedPaid bool   `json:"confirmedPaid"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.CommandID == "" || len(req.CommandID) > 128 {
		http.Error(w, "commandId and extraction method required", http.StatusBadRequest)
		return
	}
	id := mux.Vars(r)["instance_id"]
	result, err := repository.ApplyHexExtraction(r.Context(), id, userID, req.CommandID, req.Method, req.ConfirmedPaid, func(committed repository.HexPlayerResult) {
		publishHexPatch(id, req.CommandID, committed.StateRevision, committed.MapRevision, "PLAYER_LEFT_PORTAL", map[string]any{"instanceId": id, "userId": userID, "method": committed.Method, "activeUser": committed.ActiveUser})
	})
	if err != nil {
		http.Error(w, err.Error(), hexCommandStatus(err))
		return
	}
	if !result.Replayed {
		if result.ActiveUser > 0 {
			scheduleHexTurn(id, result.ActiveUser, result.TurnNumber)
		} else {
			cancelHexTurn(id)
		}
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
