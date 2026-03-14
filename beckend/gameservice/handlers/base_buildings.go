package handlers

import (
    "encoding/json"
    "fmt"
    "net/http"

    "gameservice/middleware"
    "gameservice/repository"
)

type forgeCostResponse struct {
    Wood  int `json:"wood"`
    Stone int `json:"stone"`
    Iron  int `json:"iron"`
}

type forgeResourceResponse struct {
    Wood  int `json:"wood"`
    Stone int `json:"stone"`
    Iron  int `json:"iron"`
}

type forgeRecipeResponse struct {
    ID          string `json:"id"`
    Name        string `json:"name"`
    Description string `json:"description"`
}

type forgeStateResponse struct {
    ForgeLevel int                   `json:"forgeLevel"`
    Built      bool                  `json:"built"`
    Costs      forgeCostResponse     `json:"costs"`
    Resources  forgeResourceResponse `json:"resources"`
    CanBuild   bool                  `json:"canBuild"`
    Recipes    []forgeRecipeResponse `json:"recipes"`
}

func getResourceCountByType(playerInventory string, resourceType string) (int, error) {
    resource, err := repository.GetResourceByType(resourceType)
    if err != nil {
        return 0, err
    }

    parsed := map[string]map[string]interface{}{}
    if playerInventory != "" && playerInventory != "{}" {
        if err := json.Unmarshal([]byte(playerInventory), &parsed); err != nil {
            parsed = map[string]map[string]interface{}{}
        }
    }

    key := fmt.Sprintf("resource_%d", resource.ID)
    entry, ok := parsed[key]
    if !ok {
        return 0, nil
    }

    raw := entry["item_count"]
    switch v := raw.(type) {
    case float64:
        return int(v), nil
    case int:
        return v, nil
    case int64:
        return int(v), nil
    default:
        return 0, nil
    }
}

func buildForgeState(userID int) (*forgeStateResponse, error) {
    level, err := repository.GetForgeLevel(userID)
    if err != nil {
        return nil, err
    }

    player, err := repository.GetPlayerByUserID(userID)
    if err != nil {
        return nil, err
    }

    woodCount, err := getResourceCountByType(player.Inventory, "wood")
    if err != nil {
        return nil, err
    }
    stoneCount, err := getResourceCountByType(player.Inventory, "stone")
    if err != nil {
        return nil, err
    }
    ironCount, err := getResourceCountByType(player.Inventory, "iron")
    if err != nil {
        return nil, err
    }

    built := level > 0
    canBuild := !built &&
        woodCount >= repository.ForgeCostWood &&
        stoneCount >= repository.ForgeCostStone &&
        ironCount >= repository.ForgeCostIron

    recipes := []forgeRecipeResponse{}
    if built {
        recipes = []forgeRecipeResponse{
            {
                ID:          "fortified_blade",
                Name:        "Укрепленный клинок",
                Description: "Открыт рецепт: +урон в матче на первые 3 хода.",
            },
            {
                ID:          "field_armor",
                Name:        "Полевой доспех",
                Description: "Открыт рецепт: +защита в матче на первые 3 хода.",
            },
            {
                ID:          "repair_kit",
                Name:        "Ремкомплект",
                Description: "Открыт рецепт: одноразовый ремонт снаряжения в матче.",
            },
        }
    }

    return &forgeStateResponse{
        ForgeLevel: level,
        Built:      built,
        Costs: forgeCostResponse{
            Wood:  repository.ForgeCostWood,
            Stone: repository.ForgeCostStone,
            Iron:  repository.ForgeCostIron,
        },
        Resources: forgeResourceResponse{
            Wood:  woodCount,
            Stone: stoneCount,
            Iron:  ironCount,
        },
        CanBuild: canBuild,
        Recipes:  recipes,
    }, nil
}

func GetBaseStateHandler(w http.ResponseWriter, r *http.Request) {
    userID, ok := middleware.GetUserIDFromContext(r.Context())
    if !ok || userID == 0 {
        http.Error(w, `{"error":"unauthorized"}`, http.StatusUnauthorized)
        return
    }

    state, err := buildForgeState(userID)
    if err != nil {
        http.Error(w, `{"error":"failed_to_load_base_state"}`, http.StatusInternalServerError)
        return
    }

    w.Header().Set("Content-Type", "application/json")
    _ = json.NewEncoder(w).Encode(state)
}

func BuildForgeHandler(w http.ResponseWriter, r *http.Request) {
    if r.Method != http.MethodPost {
        http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
        return
    }

    userID, ok := middleware.GetUserIDFromContext(r.Context())
    if !ok || userID == 0 {
        http.Error(w, `{"error":"unauthorized"}`, http.StatusUnauthorized)
        return
    }

    if err := repository.BuildForge(userID); err != nil {
        if err.Error() == "forge already built" {
            http.Error(w, `{"error":"forge_already_built"}`, http.StatusConflict)
            return
        }
        if err.Error() == "not enough resources" {
            http.Error(w, `{"error":"not_enough_resources"}`, http.StatusBadRequest)
            return
        }
        http.Error(w, `{"error":"failed_to_build_forge"}`, http.StatusInternalServerError)
        return
    }

    state, err := buildForgeState(userID)
    if err != nil {
        http.Error(w, `{"error":"failed_to_load_base_state"}`, http.StatusInternalServerError)
        return
    }

    w.Header().Set("Content-Type", "application/json")
    _ = json.NewEncoder(w).Encode(state)
}
