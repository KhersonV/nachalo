package handlers

import (
	"encoding/json"
	"net/http"
	"strings"

	"gameservice/middleware"
	"gameservice/repository"

	"github.com/gorilla/mux"
)

type matchEquipmentLootResponse struct {
	Status string                              `json:"status"`
	Items  []repository.MatchEquipmentLootItem `json:"items"`
}

func GetMyMatchLootHandler(w http.ResponseWriter, r *http.Request) {
	userID, ok := middleware.GetUserIDFromContext(r.Context())
	if !ok || userID == 0 {
		http.Error(w, `{"error":"unauthorized"}`, http.StatusUnauthorized)
		return
	}

	instanceID := strings.TrimSpace(mux.Vars(r)["instance_id"])
	if instanceID == "" {
		http.Error(w, `{"error":"instance_id_required"}`, http.StatusBadRequest)
		return
	}

	items, err := repository.ListMatchEquipmentLootForUser(userID, instanceID)
	if err != nil {
		http.Error(w, `{"error":"loot_error"}`, http.StatusInternalServerError)
		return
	}
	if items == nil {
		items = []repository.MatchEquipmentLootItem{}
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(matchEquipmentLootResponse{
		Status: "ok",
		Items:  items,
	})
}
