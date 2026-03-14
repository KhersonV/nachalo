package handlers

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"gameservice/middleware"
	"gameservice/repository"
)

const (
	foodUnitPrice  = 12
	waterUnitPrice = 8
)

type shopItem struct {
	ID          int            `json:"id"`
	Type        string         `json:"type"`
	Name        string         `json:"name"`
	Description string         `json:"description"`
	Image       string         `json:"image"`
	Effect      map[string]int `json:"effect"`
	Price       int            `json:"price"`
}

type buyShopItemRequest struct {
	PlayerID int    `json:"player_id"`
	ItemType string `json:"item_type"`
	Count    int    `json:"count"`
}

func resourceDisplayName(resourceType string) string {
	switch resourceType {
	case "food":
		return "Food"
	case "water":
		return "Water"
	default:
		return strings.Title(resourceType)
	}
}

func shopPrice(resourceType string) (int, bool) {
	switch resourceType {
	case "food":
		return foodUnitPrice, true
	case "water":
		return waterUnitPrice, true
	default:
		return 0, false
	}
}

func getShopItem(resourceType string) (*shopItem, error) {
	res, err := repository.GetResourceByType(resourceType)
	if err != nil {
		return nil, err
	}
	price, ok := shopPrice(resourceType)
	if !ok {
		return nil, fmt.Errorf("unsupported item type: %s", resourceType)
	}
	return &shopItem{
		ID:          res.ID,
		Type:        res.Type,
		Name:        resourceDisplayName(res.Type),
		Description: res.Description,
		Image:       res.Image,
		Effect:      res.Effect,
		Price:       price,
	}, nil
}

// GetShopItemsHandler возвращает доступные товары магазина подготовки.
func GetShopItemsHandler(w http.ResponseWriter, r *http.Request) {
	items := make([]shopItem, 0, 2)
	for _, resourceType := range []string{"food", "water"} {
		item, err := getShopItem(resourceType)
		if err != nil {
			http.Error(w, fmt.Sprintf("failed to load shop item %s: %v", resourceType, err), http.StatusInternalServerError)
			return
		}
		items = append(items, *item)
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{"items": items})
}

// BuyShopItemHandler покупает еду/воду за баланс игрока до старта матча.
func BuyShopItemHandler(w http.ResponseWriter, r *http.Request) {
	userIDFromToken, ok := middleware.GetUserIDFromContext(r.Context())
	if !ok {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	var req buyShopItemRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}
	if req.PlayerID == 0 {
		req.PlayerID = userIDFromToken
	}
	if req.PlayerID != userIDFromToken {
		http.Error(w, "forbidden", http.StatusForbidden)
		return
	}
	if req.Count <= 0 {
		req.Count = 1
	}
	if req.Count > 99 {
		http.Error(w, "count is too large", http.StatusBadRequest)
		return
	}

	itemType := strings.ToLower(strings.TrimSpace(req.ItemType))
	unitPrice, supported := shopPrice(itemType)
	if !supported {
		http.Error(w, "unsupported shop item", http.StatusBadRequest)
		return
	}

	item, err := getShopItem(itemType)
	if err != nil {
		http.Error(w, fmt.Sprintf("failed to load item: %v", err), http.StatusInternalServerError)
		return
	}

	player, err := repository.GetPlayerByUserID(req.PlayerID)
	if err != nil {
		http.Error(w, fmt.Sprintf("failed to load player: %v", err), http.StatusInternalServerError)
		return
	}

	totalCost := unitPrice * req.Count
	if player.Balance < totalCost {
		http.Error(w, "insufficient balance", http.StatusBadRequest)
		return
	}

	inv := make(map[string]map[string]interface{})
	if player.Inventory != "" && player.Inventory != "{}" {
		if err := json.Unmarshal([]byte(player.Inventory), &inv); err != nil {
			inv = make(map[string]map[string]interface{})
		}
	}

	key := "resource_" + strconv.Itoa(item.ID)
	entry, exists := inv[key]
	if !exists {
		entry = map[string]interface{}{
			"name":        item.Name,
			"item_count":  0,
			"image":       item.Image,
			"description": item.Description,
			"effect":      item.Effect,
		}
	}

	currentCount := 0
	switch c := entry["item_count"].(type) {
	case float64:
		currentCount = int(c)
	case int:
		currentCount = c
	case int64:
		currentCount = int(c)
	case string:
		if parsed, err := strconv.Atoi(c); err == nil {
			currentCount = parsed
		}
	}
	entry["item_count"] = currentCount + req.Count
	entry["name"] = item.Name
	entry["image"] = item.Image
	entry["description"] = item.Description
	entry["effect"] = item.Effect
	inv[key] = entry

	invBytes, err := json.Marshal(inv)
	if err != nil {
		http.Error(w, "failed to encode inventory", http.StatusInternalServerError)
		return
	}

	player.Balance -= totalCost
	player.Inventory = string(invBytes)
	if err := repository.UpdatePlayer(player); err != nil {
		http.Error(w, fmt.Sprintf("failed to update player: %v", err), http.StatusInternalServerError)
		return
	}

	updatedPlayer, err := repository.GetPlayerByUserID(req.PlayerID)
	if err != nil {
		if err == sql.ErrNoRows {
			http.Error(w, "player not found", http.StatusNotFound)
			return
		}
		http.Error(w, fmt.Sprintf("failed to reload player: %v", err), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{
		"player":      updatedPlayer,
		"bought_item": item,
		"count":       req.Count,
		"total_cost":  totalCost,
	})
}
