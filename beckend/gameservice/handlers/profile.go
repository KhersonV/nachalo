package handlers

import (
    "encoding/json"
    "fmt"
    "net/http"
    "strconv"
    "strings"

    "gameservice/middleware"
    "gameservice/repository"

    "github.com/gorilla/mux"
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

type publicProfileDataResponse struct {
    Player         playerProfileResponse            `json:"player"`
    Progress       *repository.PlayerProgressSummary `json:"progress"`
    IsFriend       bool                             `json:"isFriend"`
    FriendRelation string                           `json:"friendRelation"`
}

type publicProfileResponse struct {
    Status string                    `json:"status"`
    Data   publicProfileDataResponse `json:"data"`
}

type friendListItem struct {
    UserID         int    `json:"userId"`
    Name           string `json:"name"`
    Image          string `json:"image"`
    CharacterType  string `json:"characterType"`
    Level          int    `json:"level"`
    ActivityStatus string `json:"activityStatus"`
}

type friendsListResponse struct {
    Status string           `json:"status"`
    Data   []friendListItem `json:"data"`
}

type playerSearchItem struct {
    UserID         int    `json:"userId"`
    Name           string `json:"name"`
    Image          string `json:"image"`
    CharacterType  string `json:"characterType"`
    Level          int    `json:"level"`
    IsFriend       bool   `json:"isFriend"`
    FriendRelation string `json:"friendRelation"`
    ActivityStatus string `json:"activityStatus"`
}

type playersSearchResponse struct {
    Status string             `json:"status"`
    Data   []playerSearchItem `json:"data"`
}

type friendRequestItem struct {
    UserID         int    `json:"userId"`
    Name           string `json:"name"`
    Image          string `json:"image"`
    CharacterType  string `json:"characterType"`
    Level          int    `json:"level"`
    ActivityStatus string `json:"activityStatus"`
    CreatedAt      string `json:"createdAt"`
}

type friendRequestsResponse struct {
    Status string              `json:"status"`
    Data   []friendRequestItem `json:"data"`
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
    MarkUserHTTPActive(userID)

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

func GetPublicProfileHandler(w http.ResponseWriter, r *http.Request) {
    requesterID, ok := middleware.GetUserIDFromContext(r.Context())
    if !ok || requesterID == 0 {
        http.Error(w, `{"error":"unauthorized"}`, http.StatusUnauthorized)
        return
    }
    MarkUserHTTPActive(requesterID)

    vars := mux.Vars(r)
    idStr := strings.TrimSpace(vars["id"])
    targetUserID, err := strconv.Atoi(idStr)
    if err != nil || targetUserID <= 0 {
        http.Error(w, `{"error":"invalid_user_id"}`, http.StatusBadRequest)
        return
    }

    player, err := repository.GetPlayerByUserID(targetUserID)
    if err != nil {
        http.Error(w, `{"error":"player_not_found"}`, http.StatusNotFound)
        return
    }

    progress, err := repository.GetPlayerProgressSummary(targetUserID)
    if err != nil {
        http.Error(w, `{"error":"failed_to_load_profile"}`, http.StatusInternalServerError)
        return
    }

    activityStatus := GetUserActivityStatus(targetUserID)
    friendRelation, err := repository.GetFriendRelation(requesterID, targetUserID)
    if err != nil {
        http.Error(w, `{"error":"failed_to_load_friendship"}`, http.StatusInternalServerError)
        return
    }
    isFriend := friendRelation == "friend" || friendRelation == "self"

    w.Header().Set("Content-Type", "application/json")
    _ = json.NewEncoder(w).Encode(publicProfileResponse{
        Status: "ok",
        Data: publicProfileDataResponse{
            Player: playerProfileResponse{
                UserID:         player.UserID,
                Name:           player.Name,
                Image:          player.Image,
                Online:         activityStatus != "offline",
                ActivityStatus: activityStatus,
                CharacterType:  player.CharacterType,
                Level:          player.Level,
                Experience:     player.Experience,
                MaxExperience:  player.MaxExperience,
                Balance:        player.Balance,
                Attack:         player.Attack,
                Defense:        player.Defense,
                Mobility:       player.Mobility,
                Agility:        player.Agility,
                SightRange:     player.SightRange,
                IsRanged:       player.IsRanged,
                AttackRange:    player.AttackRange,
            },
            Progress: progress,
            IsFriend: isFriend,
            FriendRelation: friendRelation,
        },
    })
}

func AddFriendHandler(w http.ResponseWriter, r *http.Request) {
    userID, ok := middleware.GetUserIDFromContext(r.Context())
    if !ok || userID == 0 {
        http.Error(w, `{"error":"unauthorized"}`, http.StatusUnauthorized)
        return
    }
    MarkUserHTTPActive(userID)

    var req struct {
        FriendUserID int `json:"friendUserId"`
    }
    if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
        http.Error(w, `{"error":"invalid_json"}`, http.StatusBadRequest)
        return
    }

    if req.FriendUserID <= 0 || req.FriendUserID == userID {
        http.Error(w, `{"error":"invalid_friend_user_id"}`, http.StatusBadRequest)
        return
    }

    exists, err := repository.PlayerExists(req.FriendUserID)
    if err != nil {
        http.Error(w, `{"error":"failed_to_validate_player"}`, http.StatusInternalServerError)
        return
    }
    if !exists {
        http.Error(w, `{"error":"player_not_found"}`, http.StatusNotFound)
        return
    }

    if err := repository.CreateFriendRequest(userID, req.FriendUserID); err != nil {
        http.Error(w, `{"error":"failed_to_send_friend_request"}`, http.StatusInternalServerError)
        return
    }

    w.Header().Set("Content-Type", "application/json")
    _ = json.NewEncoder(w).Encode(map[string]string{
        "status": "ok",
        "message": fmt.Sprintf("friend_request_sent:%d", req.FriendUserID),
    })
}

func GetFriendsHandler(w http.ResponseWriter, r *http.Request) {
    userID, ok := middleware.GetUserIDFromContext(r.Context())
    if !ok || userID == 0 {
        http.Error(w, `{"error":"unauthorized"}`, http.StatusUnauthorized)
        return
    }
    MarkUserHTTPActive(userID)

    friends, err := repository.ListFriends(userID)
    if err != nil {
        http.Error(w, `{"error":"failed_to_load_friends"}`, http.StatusInternalServerError)
        return
    }

    result := make([]friendListItem, 0, len(friends))
    for _, f := range friends {
        result = append(result, friendListItem{
            UserID:         f.UserID,
            Name:           f.Name,
            Image:          f.Image,
            CharacterType:  f.CharacterType,
            Level:          f.Level,
            ActivityStatus: GetUserActivityStatus(f.UserID),
        })
    }

    w.Header().Set("Content-Type", "application/json")
    _ = json.NewEncoder(w).Encode(friendsListResponse{
        Status: "ok",
        Data:   result,
    })
}

func RemoveFriendHandler(w http.ResponseWriter, r *http.Request) {
    userID, ok := middleware.GetUserIDFromContext(r.Context())
    if !ok || userID == 0 {
        http.Error(w, `{"error":"unauthorized"}`, http.StatusUnauthorized)
        return
    }
    MarkUserHTTPActive(userID)

    vars := mux.Vars(r)
    idStr := strings.TrimSpace(vars["id"])
    friendUserID, err := strconv.Atoi(idStr)
    if err != nil || friendUserID <= 0 || friendUserID == userID {
        http.Error(w, `{"error":"invalid_friend_user_id"}`, http.StatusBadRequest)
        return
    }

    if err := repository.RemoveFriendBidirectional(userID, friendUserID); err != nil {
        http.Error(w, `{"error":"failed_to_remove_friend"}`, http.StatusInternalServerError)
        return
    }

    w.Header().Set("Content-Type", "application/json")
    _ = json.NewEncoder(w).Encode(map[string]string{
        "status":  "ok",
        "message": fmt.Sprintf("friend_removed:%d", friendUserID),
    })
}

func SearchPlayersHandler(w http.ResponseWriter, r *http.Request) {
    userID, ok := middleware.GetUserIDFromContext(r.Context())
    if !ok || userID == 0 {
        http.Error(w, `{"error":"unauthorized"}`, http.StatusUnauthorized)
        return
    }
    MarkUserHTTPActive(userID)

    query := strings.TrimSpace(r.URL.Query().Get("q"))
    if len(query) < 2 {
        http.Error(w, `{"error":"query_too_short"}`, http.StatusBadRequest)
        return
    }

    found, err := repository.SearchPlayersByName(query, 20)
    if err != nil {
        http.Error(w, `{"error":"failed_to_search_players"}`, http.StatusInternalServerError)
        return
    }

    result := make([]playerSearchItem, 0, len(found))
    for _, p := range found {
        friendRelation, err := repository.GetFriendRelation(userID, p.UserID)
        if err != nil {
            http.Error(w, `{"error":"failed_to_load_friendship"}`, http.StatusInternalServerError)
            return
        }
        isFriend := friendRelation == "friend" || friendRelation == "self"

        result = append(result, playerSearchItem{
            UserID:         p.UserID,
            Name:           p.Name,
            Image:          p.Image,
            CharacterType:  p.CharacterType,
            Level:          p.Level,
            IsFriend:       isFriend,
            FriendRelation: friendRelation,
            ActivityStatus: GetUserActivityStatus(p.UserID),
        })
    }

    w.Header().Set("Content-Type", "application/json")
    _ = json.NewEncoder(w).Encode(playersSearchResponse{
        Status: "ok",
        Data:   result,
    })
}

func GetIncomingFriendRequestsHandler(w http.ResponseWriter, r *http.Request) {
    userID, ok := middleware.GetUserIDFromContext(r.Context())
    if !ok || userID == 0 {
        http.Error(w, `{"error":"unauthorized"}`, http.StatusUnauthorized)
        return
    }
    MarkUserHTTPActive(userID)

    requests, err := repository.ListIncomingFriendRequests(userID)
    if err != nil {
        http.Error(w, `{"error":"failed_to_load_friend_requests"}`, http.StatusInternalServerError)
        return
    }

    result := make([]friendRequestItem, 0, len(requests))
    for _, req := range requests {
        result = append(result, friendRequestItem{
            UserID:         req.UserID,
            Name:           req.Name,
            Image:          req.Image,
            CharacterType:  req.CharacterType,
            Level:          req.Level,
            ActivityStatus: GetUserActivityStatus(req.UserID),
            CreatedAt:      req.CreatedAt,
        })
    }

    w.Header().Set("Content-Type", "application/json")
    _ = json.NewEncoder(w).Encode(friendRequestsResponse{
        Status: "ok",
        Data:   result,
    })
}

func GetOutgoingFriendRequestsHandler(w http.ResponseWriter, r *http.Request) {
    userID, ok := middleware.GetUserIDFromContext(r.Context())
    if !ok || userID == 0 {
        http.Error(w, `{"error":"unauthorized"}`, http.StatusUnauthorized)
        return
    }
    MarkUserHTTPActive(userID)

    requests, err := repository.ListOutgoingFriendRequests(userID)
    if err != nil {
        http.Error(w, `{"error":"failed_to_load_friend_requests"}`, http.StatusInternalServerError)
        return
    }

    result := make([]friendRequestItem, 0, len(requests))
    for _, req := range requests {
        result = append(result, friendRequestItem{
            UserID:         req.UserID,
            Name:           req.Name,
            Image:          req.Image,
            CharacterType:  req.CharacterType,
            Level:          req.Level,
            ActivityStatus: GetUserActivityStatus(req.UserID),
            CreatedAt:      req.CreatedAt,
        })
    }

    w.Header().Set("Content-Type", "application/json")
    _ = json.NewEncoder(w).Encode(friendRequestsResponse{
        Status: "ok",
        Data:   result,
    })
}

func CancelOutgoingFriendRequestHandler(w http.ResponseWriter, r *http.Request) {
    userID, ok := middleware.GetUserIDFromContext(r.Context())
    if !ok || userID == 0 {
        http.Error(w, `{"error":"unauthorized"}`, http.StatusUnauthorized)
        return
    }
    MarkUserHTTPActive(userID)

    vars := mux.Vars(r)
    targetStr := strings.TrimSpace(vars["id"])
    targetUserID, err := strconv.Atoi(targetStr)
    if err != nil || targetUserID <= 0 || targetUserID == userID {
        http.Error(w, `{"error":"invalid_target_user_id"}`, http.StatusBadRequest)
        return
    }

    if err := repository.CancelOutgoingFriendRequest(userID, targetUserID); err != nil {
        http.Error(w, `{"error":"failed_to_cancel_friend_request"}`, http.StatusInternalServerError)
        return
    }

    w.Header().Set("Content-Type", "application/json")
    _ = json.NewEncoder(w).Encode(map[string]string{
        "status": "ok",
    })
}

func AcceptFriendRequestHandler(w http.ResponseWriter, r *http.Request) {
    userID, ok := middleware.GetUserIDFromContext(r.Context())
    if !ok || userID == 0 {
        http.Error(w, `{"error":"unauthorized"}`, http.StatusUnauthorized)
        return
    }
    MarkUserHTTPActive(userID)

    vars := mux.Vars(r)
    requesterStr := strings.TrimSpace(vars["id"])
    requesterUserID, err := strconv.Atoi(requesterStr)
    if err != nil || requesterUserID <= 0 || requesterUserID == userID {
        http.Error(w, `{"error":"invalid_requester_user_id"}`, http.StatusBadRequest)
        return
    }

    if err := repository.AcceptFriendRequest(userID, requesterUserID); err != nil {
        http.Error(w, `{"error":"failed_to_accept_friend_request"}`, http.StatusInternalServerError)
        return
    }

    w.Header().Set("Content-Type", "application/json")
    _ = json.NewEncoder(w).Encode(map[string]string{
        "status": "ok",
    })
}

func RejectFriendRequestHandler(w http.ResponseWriter, r *http.Request) {
    userID, ok := middleware.GetUserIDFromContext(r.Context())
    if !ok || userID == 0 {
        http.Error(w, `{"error":"unauthorized"}`, http.StatusUnauthorized)
        return
    }
    MarkUserHTTPActive(userID)

    vars := mux.Vars(r)
    requesterStr := strings.TrimSpace(vars["id"])
    requesterUserID, err := strconv.Atoi(requesterStr)
    if err != nil || requesterUserID <= 0 || requesterUserID == userID {
        http.Error(w, `{"error":"invalid_requester_user_id"}`, http.StatusBadRequest)
        return
    }

    if err := repository.RejectFriendRequest(userID, requesterUserID); err != nil {
        http.Error(w, `{"error":"failed_to_reject_friend_request"}`, http.StatusInternalServerError)
        return
    }

    w.Header().Set("Content-Type", "application/json")
    _ = json.NewEncoder(w).Encode(map[string]string{
        "status": "ok",
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
    MarkUserHTTPActive(userID)

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
