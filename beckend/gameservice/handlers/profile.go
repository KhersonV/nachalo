package handlers

import (
    "encoding/json"
    "net/http"
    "strings"

    "gameservice/middleware"
    "gameservice/repository"
)

type playerProfileResponse struct {
    UserID         int    `json:"userId"`
    Name           string `json:"name"`
    Image          string `json:"image"`
    Online         bool   `json:"online"`
    ActivityStatus string `json:"activityStatus"`
    CharacterType  string `json:"characterType"`
    Level          int    `json:"level"`
    Experience     int    `json:"experience"`
    MaxExperience  int    `json:"maxExperience"`
    Balance        int    `json:"balance"`
    Attack         int    `json:"attack"`
    Defense        int    `json:"defense"`
    Mobility       int    `json:"mobility"`
    Agility        int    `json:"agility"`
    SightRange     int    `json:"sightRange"`
    IsRanged       bool   `json:"isRanged"`
    AttackRange    int    `json:"attackRange"`
}

type profileResourcesResponse struct {
    Food  int `json:"food"`
    Water int `json:"water"`
    Wood  int `json:"wood"`
    Stone int `json:"stone"`
    Iron  int `json:"iron"`
}

type profileDataResponse struct {
    Player    playerProfileResponse          `json:"player"`
    Progress  *repository.PlayerProgressSummary `json:"progress"`
    Resources profileResourcesResponse       `json:"resources"`
    Base      *forgeStateResponse           `json:"base"`
}

type profileResponse struct {
    Status string              `json:"status"`
    Data   profileDataResponse `json:"data"`
}

func loadProfileData(userID int) (*profileDataResponse, error) {
    player, err := repository.GetPlayerByUserID(userID)
    if err != nil {
        return nil, err
    }

    progress, err := repository.GetPlayerProgressSummary(userID)
    if err != nil {
        return nil, err
    }

    baseState, err := buildForgeState(userID)
    if err != nil {
        return nil, err
    }

    food, err := getResourceCountByType(player.Inventory, "food")
    if err != nil {
        return nil, err
    }
    water, err := getResourceCountByType(player.Inventory, "water")
    if err != nil {
        return nil, err
    }
    wood, err := getResourceCountByType(player.Inventory, "wood")
    if err != nil {
        return nil, err
    }
    stone, err := getResourceCountByType(player.Inventory, "stone")
    if err != nil {
        return nil, err
    }
    iron, err := getResourceCountByType(player.Inventory, "iron")
    if err != nil {
        return nil, err
    }

    activityStatus := GetUserActivityStatus(player.UserID)
    if activityStatus == "offline" {
        // Profile endpoint requires valid JWT, so default to lobby when no WS is attached.
        activityStatus = "in_lobby"
    }

    return &profileDataResponse{
        Player: playerProfileResponse{
            UserID:        player.UserID,
            Name:          player.Name,
            Image:         player.Image,
            Online:        activityStatus != "offline",
            ActivityStatus: activityStatus,
            CharacterType: player.CharacterType,
            Level:         player.Level,
            Experience:    player.Experience,
            MaxExperience: player.MaxExperience,
            Balance:       player.Balance,
            Attack:        player.Attack,
            Defense:       player.Defense,
            Mobility:      player.Mobility,
            Agility:       player.Agility,
            SightRange:    player.SightRange,
            IsRanged:      player.IsRanged,
            AttackRange:   player.AttackRange,
        },
        Progress: progress,
        Resources: profileResourcesResponse{
            Food:  food,
            Water: water,
            Wood:  wood,
            Stone: stone,
            Iron:  iron,
        },
        Base: baseState,
    }, nil
}

func GetProfileHandler(w http.ResponseWriter, r *http.Request) {
    userID, ok := middleware.GetUserIDFromContext(r.Context())
    if !ok || userID == 0 {
        http.Error(w, `{"error":"unauthorized"}`, http.StatusUnauthorized)
        return
    }

    data, err := loadProfileData(userID)
    if err != nil {
        http.Error(w, `{"error":"failed_to_load_profile"}`, http.StatusInternalServerError)
        return
    }

    w.Header().Set("Content-Type", "application/json")
    _ = json.NewEncoder(w).Encode(profileResponse{
        Status: "ok",
        Data:   *data,
    })
}

func UpdateProfileHandler(w http.ResponseWriter, r *http.Request) {
    if r.Method != http.MethodPatch {
        http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
        return
    }

    userID, ok := middleware.GetUserIDFromContext(r.Context())
    if !ok || userID == 0 {
        http.Error(w, `{"error":"unauthorized"}`, http.StatusUnauthorized)
        return
    }

    var req struct {
        Name  string `json:"name"`
        Image string `json:"image"`
    }
    if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
        http.Error(w, `{"error":"invalid_json"}`, http.StatusBadRequest)
        return
    }

    currentName, currentImage, err := repository.GetPlayerIdentityDefaults(userID)
    if err != nil {
        http.Error(w, `{"error":"player_not_found"}`, http.StatusNotFound)
        return
    }

    name := strings.TrimSpace(req.Name)
    image := strings.TrimSpace(req.Image)

    if name == "" {
        name = currentName
    }
    if image == "" {
        image = currentImage
    }

    if len(name) > 32 {
        http.Error(w, `{"error":"name_too_long"}`, http.StatusBadRequest)
        return
    }
    if len(image) > 512 {
        http.Error(w, `{"error":"image_too_long"}`, http.StatusBadRequest)
        return
    }

    if err := repository.UpdatePlayerIdentity(userID, name, image); err != nil {
        http.Error(w, `{"error":"failed_to_update_profile"}`, http.StatusInternalServerError)
        return
    }

    data, err := loadProfileData(userID)
    if err != nil {
        http.Error(w, `{"error":"failed_to_load_profile"}`, http.StatusInternalServerError)
        return
    }

    w.Header().Set("Content-Type", "application/json")
    _ = json.NewEncoder(w).Encode(profileResponse{
        Status: "ok",
        Data:   *data,
    })
}
