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
    Library    buildingStateResponse `json:"library"`
    Forge      buildingStateResponse `json:"forge"`
}

type buildingStateResponse struct {
    Level       int                   `json:"level"`
    Built       bool                  `json:"built"`
    Costs       forgeCostResponse     `json:"costs"`
    Resources   forgeResourceResponse `json:"resources"`
    CanBuild    bool                  `json:"canBuild"`
    Unlockables []forgeRecipeResponse `json:"unlockables"`
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
    forgeLevel, err := repository.GetForgeLevel(userID)
    if err != nil {
        return nil, err
    }
    libraryLevel, err := repository.GetLibraryLevel(userID)
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

    forgeBuilt := forgeLevel > 0
    forgeCanBuild := !forgeBuilt &&
        woodCount >= repository.ForgeCostWood &&
        stoneCount >= repository.ForgeCostStone &&
        ironCount >= repository.ForgeCostIron

    libraryBuilt := libraryLevel > 0
    libraryCanBuild := !libraryBuilt &&
        woodCount >= repository.LibraryCostWood &&
        stoneCount >= repository.LibraryCostStone &&
        ironCount >= repository.LibraryCostIron

    forgeUnlockables := []forgeRecipeResponse{}
    if forgeBuilt {
        forgeUnlockables = []forgeRecipeResponse{
            {
                ID:          "scout_tower",
                Name:        "Scout Tower",
                Description: "Blueprint: increases player's sight by 1 tile (max 5). Stats: +1 sight.",
            },
            {
                ID:          "turret",
                Name:        "Turret",
                Description: "Blueprint: automatically attacks enemies within range. Stats: attack 15, defense 5, energy 8, radius 2, attack cost 6, HP 20.",
            },
            {
                ID:          "wall",
                Name:        "Wall",
                Description: "Blueprint: blocks passage for both enemies and you. Stats: defense 5, durability 20. Can be built on a free tile.",
            },
        }
    }

    libraryUnlockables := []forgeRecipeResponse{}
    if libraryBuilt {
        libraryUnlockables = []forgeRecipeResponse{
            {
                ID:          "cartographer_notes",
                Name:        "Cartographer's Notes",
                Description: "Unlocks bonus: +1 sight at the start of the match.",
            },
            {
                ID:          "battle_treatise",
                Name:        "Battle Treatise",
                Description: "Unlocks bonus: +1 attack for the first 2 turns.",
            },
            {
                ID:          "field_manual",
                Name:        "Field Manual",
                Description: "Unlocks bonus: -1 to the cost of the first attack in the match.",
            },
        }
    }

    return &forgeStateResponse{
        ForgeLevel: forgeLevel,
        Built:      forgeBuilt,
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
        CanBuild: forgeCanBuild,
        Recipes:  forgeUnlockables,
        Forge: buildingStateResponse{
            Level:       forgeLevel,
            Built:       forgeBuilt,
            Costs:       forgeCostResponse{Wood: repository.ForgeCostWood, Stone: repository.ForgeCostStone, Iron: repository.ForgeCostIron},
            Resources:   forgeResourceResponse{Wood: woodCount, Stone: stoneCount, Iron: ironCount},
            CanBuild:    forgeCanBuild,
            Unlockables: forgeUnlockables,
        },
        Library: buildingStateResponse{
            Level:       libraryLevel,
            Built:       libraryBuilt,
            Costs:       forgeCostResponse{Wood: repository.LibraryCostWood, Stone: repository.LibraryCostStone, Iron: repository.LibraryCostIron},
            Resources:   forgeResourceResponse{Wood: woodCount, Stone: stoneCount, Iron: ironCount},
            CanBuild:    libraryCanBuild,
            Unlockables: libraryUnlockables,
        },
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

func BuildLibraryHandler(w http.ResponseWriter, r *http.Request) {
    if r.Method != http.MethodPost {
        http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
        return
    }

    userID, ok := middleware.GetUserIDFromContext(r.Context())
    if !ok || userID == 0 {
        http.Error(w, `{"error":"unauthorized"}`, http.StatusUnauthorized)
        return
    }

    if err := repository.BuildLibrary(userID); err != nil {
        if err.Error() == "library already built" {
            http.Error(w, `{"error":"library_already_built"}`, http.StatusConflict)
            return
        }
        if err.Error() == "not enough resources" {
            http.Error(w, `{"error":"not_enough_resources"}`, http.StatusBadRequest)
            return
        }
        http.Error(w, `{"error":"failed_to_build_library"}`, http.StatusInternalServerError)
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
