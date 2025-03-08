//===============================
// gameservice/handlers/match.go
//===============================

package handlers

import (
    "encoding/json"
    "fmt"
    "log"
    "net/http"
    "time"

    "gameservice/game"
    "gameservice/repository"
)

type RequestMatch struct {
    InstanceID   string `json:"instance_id"`
    Mode         string `json:"mode"`
    PlayerIDs    []int  `json:"player_ids"`
    TeamsCount   int    `json:"teams_count"`
    TotalPlayers int    `json:"total_players"`
}

func CreateMatchHandler(w http.ResponseWriter, r *http.Request) {
    var req RequestMatch
    if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
        http.Error(w, "Bad request", http.StatusBadRequest)
        return
    }

    cfg := game.MapConfig{
        TotalPlayers: req.TotalPlayers,
        TeamsCount:   req.TeamsCount,
        WalkableProb: 0.8,
        ResourceProb: 0.1,
        MonsterProb:  0.05,
    }

    grid, startPositions, _, err := game.GenerateMap(cfg)
    if err != nil {
        log.Printf("Ошибка генерации карты: %v", err)
        http.Error(w, "Ошибка генерации карты", http.StatusInternalServerError)
        return
    }

    mapData, err := json.Marshal(grid)
    if err != nil {
        http.Error(w, "Ошибка маршалинга карты", http.StatusInternalServerError)
        return
    }

    createdAt := time.Now()

    _, err = repository.DB.Exec(`
        INSERT INTO matches (
            instance_id, mode, teams_count, total_players, map_width, map_height, map, active_player_id, turn_order, created_at
        ) VALUES (
            $1, $2, $3, $4, $5, $6, $7, $8, $9, $10
        )`,
        req.InstanceID,
        req.Mode,
        req.TeamsCount,
        req.TotalPlayers,
        len(grid[0]),
        len(grid),
        string(mapData),
        req.PlayerIDs[0],
        fmt.Sprintf("[%d]", req.PlayerIDs[0]),
        createdAt,
    )
    if err != nil {
        log.Printf("Ошибка вставки матча: %v", err)
        http.Error(w, "Ошибка сохранения матча", http.StatusInternalServerError)
        return
    }

    // Важно: СОЗДАЁМ СОСТОЯНИЕ МАТЧА сразу
    game.CreateMatchState(req.InstanceID, req.PlayerIDs)

    // Вставляем игроков в таблицу match_players
    for i, playerID := range req.PlayerIDs {
        player, err := repository.GetPlayerByUserID(playerID)
        if err != nil {
            log.Printf("Игрок с ID %d не найден: %v", playerID, err)
            continue
        }

        pos := startPositions[0] // для PVE одна команда
        groupID := 1
        if req.TeamsCount > 1 && i >= len(req.PlayerIDs)/2 {
            pos = startPositions[1]
            groupID = 2
        }

        err = repository.CreateMatchPlayerCopy(req.InstanceID, player, pos[0], pos[1], groupID)
        if err != nil {
            log.Printf("Ошибка вставки игрока в матч: %v", err)
            continue
        }
    }

    playersInMatch, err := repository.GetPlayersInMatch(req.InstanceID)
    if err != nil {
        log.Printf("Ошибка получения игроков после вставки: %v", err)
        http.Error(w, "Ошибка получения игроков", http.StatusInternalServerError)
        return
    }

    response := map[string]interface{}{
        "instance_id": req.InstanceID,
        "mode":        req.Mode,
        "map":         grid,
        "created_at":  createdAt,
        "map_width":   len(grid[0]),
        "map_height":  len(grid),
        "player_ids":  req.PlayerIDs,
        "players":     playersInMatch,
    }

    w.Header().Set("Content-Type", "application/json")
    json.NewEncoder(w).Encode(response)
}

func GetMatchHandler(w http.ResponseWriter, r *http.Request) {
    instanceID := r.URL.Query().Get("instance_id")
    if instanceID == "" {
        http.Error(w, "instance_id обязателен", http.StatusBadRequest)
        return
    }

    match, err := repository.GetMatchByID(instanceID)
    if err != nil {
        http.Error(w, fmt.Sprintf("Матч не найден: %v", err), http.StatusNotFound)
        return
    }

    players, err := repository.GetPlayersInMatch(instanceID)
    if err != nil {
        http.Error(w, fmt.Sprintf("Ошибка получения игроков: %v", err), http.StatusInternalServerError)
        return
    }

    response := map[string]interface{}{
        "instance_id":   match.InstanceID,
        "mode":          match.Mode,
        "teams_count":   match.TeamsCount,
        "total_players": match.TotalPlayers,
        "map_width":     match.MapWidth,
        "map_height":    match.MapHeight,
        "map":           match.Map, // json.RawMessage
        "players":       players,
        "created_at":    match.CreatedAt,
    }

    w.Header().Set("Content-Type", "application/json")
    json.NewEncoder(w).Encode(response)
}
