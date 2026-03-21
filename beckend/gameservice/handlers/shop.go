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

type blueprintCatalogItem struct {
	ID           int
	Type         string
	Name         string
	Description  string
	Image        string
	Effect       map[string]int
	Price        int
	InventoryKey string
	RequiresForge bool
}

var blueprintCatalog = map[string]blueprintCatalogItem{
	"scout_tower_blueprint": {
		ID:          1001,
		Type:        "scout_tower_blueprint",
		Name:        "Scout Tower Blueprint",
		Description: "Gives +1 to player's sight. In-match stats: defense 5, health 30.",
		Image:       "/ui-icons/base.png",
		Effect: map[string]int{
			"sight_bonus":       1,
			"structure_defense": 5,
			"structure_health":  30,
		},
		Price:        55,
		InventoryKey: "blueprint_scout_tower",
		RequiresForge: true,
	},
	"turret_blueprint": {
		ID:          1002,
		Type:        "turret_blueprint",
		Name:        "Turret Blueprint",
		Description: "Turret attacks enemies at the end of each turn. Base damage 10, health 30.",
		Image:       "/ui-icons/base.png",
		Effect: map[string]int{
			"turret_damage":    10,
			"structure_health": 30,
		},
		Price:        75,
		InventoryKey: "blueprint_turret",
		RequiresForge: true,
	},
	"wall_blueprint": {
		ID:          1003,
		Type:        "wall_blueprint",
		Name:        "Wall Blueprint",
		Description: "Wall blocks passage. Durability 30 HP, defense 8.",
		Image:       "/ui-icons/base.png",
		Effect: map[string]int{
			"structure_blocking": 1,
			"structure_health":   30,
			"structure_defense":  8,
		},
		Price:        60,
		InventoryKey: "blueprint_wall",
		RequiresForge: true,
	},
}

type shopItem struct {
	ID          int            `json:"id"`
	Type        string         `json:"type"`
	Category    string         `json:"category"`
	Name        string         `json:"name"`
	Description string         `json:"description"`
	Image       string         `json:"image"`
	Effect      map[string]int `json:"effect"`
	Price       int            `json:"price"`
	InventoryKey string        `json:"inventoryKey"`
	RequiresForge bool         `json:"requiresForge"`
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
		Category:    "resource",
		Name:        resourceDisplayName(res.Type),
		Description: res.Description,
		Image:       res.Image,
		Effect:      res.Effect,
		Price:       price,
		InventoryKey: "",
		RequiresForge: false,
	}, nil
}

func getBlueprintItem(itemType string) (*shopItem, bool) {
	bp, ok := blueprintCatalog[itemType]
	if !ok {
		return nil, false
	}
	return &shopItem{
		ID:           bp.ID,
		Type:         bp.Type,
		Category:     "blueprint",
		Name:         bp.Name,
		Description:  bp.Description,
		Image:        bp.Image,
		Effect:       bp.Effect,
		Price:        bp.Price,
		InventoryKey: bp.InventoryKey,
		RequiresForge: bp.RequiresForge,
	}, true
}

// GetShopItemsHandler returns available preparatory shop items.
func GetShopItemsHandler(w http.ResponseWriter, r *http.Request) {
	items := make([]shopItem, 0, 5)
	for _, resourceType := range []string{"food", "water"} {
		item, err := getShopItem(resourceType)
		if err != nil {
			http.Error(w, fmt.Sprintf("failed to load shop item %s: %v", resourceType, err), http.StatusInternalServerError)
			return
		}
		items = append(items, *item)
	}
	for _, itemType := range []string{"scout_tower_blueprint", "turret_blueprint", "wall_blueprint"} {
		if bp, ok := getBlueprintItem(itemType); ok {
			items = append(items, *bp)
		}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{"items": items})
}

// BuyShopItemHandler purchases food/water using player's balance before match start.
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
	var (
		item      *shopItem
		err       error
		unitPrice int
	)

	if bp, ok := getBlueprintItem(itemType); ok {
		item = bp
		unitPrice = item.Price
		if item.RequiresForge {
			forgeLevel, ferr := repository.GetForgeLevel(req.PlayerID)
			if ferr != nil {
				http.Error(w, "failed to check forge level", http.StatusInternalServerError)
				return
			}
			if forgeLevel <= 0 {
				http.Error(w, "forge_required", http.StatusBadRequest)
				return
			}
		}
	} else {
		var supported bool
		unitPrice, supported = shopPrice(itemType)
		if !supported {
			http.Error(w, "unsupported shop item", http.StatusBadRequest)
			return
		}
		item, err = getShopItem(itemType)
		if err != nil {
			http.Error(w, fmt.Sprintf("failed to load item: %v", err), http.StatusInternalServerError)
			return
		}
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

	key := item.InventoryKey
	if key == "" {
		key = "resource_" + strconv.Itoa(item.ID)
	}
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
