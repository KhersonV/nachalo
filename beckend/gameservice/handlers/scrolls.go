package handlers

import (
	"encoding/json"
	"fmt"
	"math"
	"net/http"
	"strconv"

	"gameservice/middleware"
	"gameservice/repository"

	"github.com/gorilla/mux"
)

// UseScrollHandler позволяет игроку использовать купленный свиток во время матча.
// POST /game/match/{instance_id}/use-scroll
// JSON body: { "scroll_type": "scroll_portal_x" }
func UseScrollHandler(w http.ResponseWriter, r *http.Request) {
	userID, ok := middleware.GetUserIDFromContext(r.Context())
	if !ok {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	vars := mux.Vars(r)
	instanceID := vars["instance_id"]
	if instanceID == "" {
		http.Error(w, "instance_id required", http.StatusBadRequest)
		return
	}

	var req struct {
		ScrollType string `json:"scroll_type"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}
	st := req.ScrollType
	if st == "" {
		http.Error(w, "scroll_type required", http.StatusBadRequest)
		return
	}

	scrollItem, ok := getScrollItem(st)
	if !ok {
		http.Error(w, "unsupported scroll type", http.StatusBadRequest)
		return
	}

	// Ensure player participates in match
	matchPlayer, err := repository.GetMatchPlayerByID(instanceID, userID)
	if err != nil {
		http.Error(w, fmt.Sprintf("failed to load match player: %v", err), http.StatusInternalServerError)
		return
	}

	// Prefer consuming scroll from match-specific inventory (match_players.inventory)
	inv := make(map[string]map[string]interface{})
	if matchPlayer.Inventory != "" && matchPlayer.Inventory != "{}" {
		if err := json.Unmarshal([]byte(matchPlayer.Inventory), &inv); err != nil {
			inv = make(map[string]map[string]interface{})
		}
	}
	entry, exists := inv[scrollItem.InventoryKey]
	if !exists {
		// fallback: check persistent player inventory (players.inventory)
		player, err := repository.GetPlayerByUserID(userID)
		if err != nil {
			http.Error(w, fmt.Sprintf("failed to load player: %v", err), http.StatusInternalServerError)
			return
		}
		if player.Inventory != "" && player.Inventory != "{}" {
			if err := json.Unmarshal([]byte(player.Inventory), &inv); err != nil {
				inv = make(map[string]map[string]interface{})
			}
		}
		entry, exists = inv[scrollItem.InventoryKey]
		if !exists {
			http.Error(w, "no_scroll", http.StatusBadRequest)
			return
		}
	}
	count := 0
	switch v := entry["item_count"].(type) {
	case float64:
		count = int(v)
	case int:
		count = v
	case int64:
		count = int(v)
	case string:
		// ignore parse errors -> 0
		if parsed, err := strconv.Atoi(v); err == nil {
			count = parsed
		}
	}
	if count <= 0 {
		http.Error(w, "no_scroll", http.StatusBadRequest)
		return
	}

	// Compute target coordinate depending on scroll type
	var axis string
	var value int

	switch st {
	case "scroll_portal_x", "scroll_portal_y":
		matchInfo, err := repository.GetMatchByID(instanceID)
		if err != nil {
			http.Error(w, fmt.Sprintf("failed to load match: %v", err), http.StatusInternalServerError)
			return
		}
		var portal [2]int
		if err := json.Unmarshal(matchInfo.PortalPosition, &portal); err != nil {
			http.Error(w, "portal not found", http.StatusInternalServerError)
			return
		}
		if st == "scroll_portal_x" {
			axis = "x"
			value = portal[0]
		} else {
			axis = "y"
			value = portal[1]
		}

	case "scroll_nearest_player_x", "scroll_nearest_player_y":
		// find nearest other player
		players, err := repository.GetPlayersInMatch(instanceID)
		if err != nil {
			http.Error(w, fmt.Sprintf("failed to load match players: %v", err), http.StatusInternalServerError)
			return
		}
		if len(players) <= 1 {
			http.Error(w, "no_target", http.StatusBadRequest)
			return
		}
		minD := math.MaxFloat64
		var targetX, targetY int
		for _, p := range players {
			if p.UserID == matchPlayer.UserID {
				continue
			}
			dx := float64(p.Position.X - matchPlayer.Position.X)
			dy := float64(p.Position.Y - matchPlayer.Position.Y)
			d := math.Sqrt(dx*dx + dy*dy)
			if d < minD {
				minD = d
				targetX = p.Position.X
				targetY = p.Position.Y
			}
		}
		if st == "scroll_nearest_player_x" {
			axis = "x"
			value = targetX
		} else {
			axis = "y"
			value = targetY
		}

	case "scroll_nearest_barrel_x", "scroll_nearest_barrel_y":
		cells, err := repository.LoadMapCells(instanceID)
		if err != nil {
			http.Error(w, fmt.Sprintf("failed to load map cells: %v", err), http.StatusInternalServerError)
			return
		}
		minD := math.MaxFloat64
		var bx, by int
		for _, c := range cells {
			if c.Barbel == nil {
				continue
			}
			dx := float64(c.X - matchPlayer.Position.X)
			dy := float64(c.Y - matchPlayer.Position.Y)
			d := math.Sqrt(dx*dx + dy*dy)
			if d < minD {
				minD = d
				bx = c.X
				by = c.Y
			}
		}
		if minD == math.MaxFloat64 {
			http.Error(w, "no_target", http.StatusBadRequest)
			return
		}
		if st == "scroll_nearest_barrel_x" {
			axis = "x"
			value = bx
		} else {
			axis = "y"
			value = by
		}

	default:
		http.Error(w, "unsupported scroll type", http.StatusBadRequest)
		return
	}

	// Try to consume one scroll from match inventory; fallback to persistent inventory
	if err := repository.RemoveInventoryItemAndSyncJSON(instanceID, userID, "scroll", scrollItem.ID, 1); err != nil {
		// fallback to persistent players.inventory
		if err2 := repository.ConsumePlayerInventoryItem(userID, "scroll", scrollItem.ID, 1); err2 != nil {
			http.Error(w, fmt.Sprintf("failed to consume scroll: %v / %v", err, err2), http.StatusInternalServerError)
			return
		}
	}

	// Return only the single coordinate to the requester
	resp := map[string]any{"axis": axis, "value": value}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}
