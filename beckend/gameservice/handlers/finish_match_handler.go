
// ============================================
// gameservice/handlers/finish_match_handler.go
// ============================================

package handlers

import (
	"encoding/json"
    "fmt"
	"log"
	"net/http"

    "gameservice/game"
	"gameservice/middleware"
	"gameservice/repository"
	"gameservice/service"
)

type finishMatchStatsResponse struct {
    Status string                 `json:"status"`
    Stats  *playerGameStatsResult `json:"stats,omitempty"`
}

type matchEndedBroadcastResponse struct {
    Type    string                    `json:"type"`
    Payload matchEndedBroadcastResult `json:"payload"`
}

type matchEndedBroadcastResult struct {
    InstanceID string            `json:"instanceId"`
    WinnerType string            `json:"winnerType"`
    WinnerID   int               `json:"winnerId"`
    Stats      []playerGameStats `json:"stats"`
}

type playerGameStatsResult struct {
    InstanceID string          `json:"instanceId"`
    WinnerType string          `json:"winnerType"`
    WinnerID   int             `json:"winnerId"`
    Player     playerGameStats `json:"player"`
}

type playerGameStats struct {
    UserID           int             `json:"userId"`
    Name             string          `json:"name"`
    ExpGained        int             `json:"expGained"`
    PlayerKills      int             `json:"playerKills"`
    MonsterKills     int             `json:"monsterKills"`
    DamageTotal      int             `json:"damageTotal"`
    DamageToPlayers  int             `json:"damageToPlayers"`
    DamageToMonsters int             `json:"damageToMonsters"`
    Rewards          json.RawMessage `json:"rewards"`
}

func buildPlayerGameStats(instanceID string, userID int, playerName string, results *game.MatchResults) (*playerGameStatsResult, error) {
    if results == nil {
        return nil, fmt.Errorf("match results are nil")
    }

    var current *game.PlayerResult
    for i := range results.PlayerResults {
        if results.PlayerResults[i].UserID == userID {
            current = &results.PlayerResults[i]
            break
        }
    }
    if current == nil {
        return nil, fmt.Errorf("player %d stats not found", userID)
    }

    winnerType := "user"
    winnerID := results.WinnerID
    if results.WinnerGroupID > 0 {
        winnerType = "group"
        winnerID = results.WinnerGroupID
    }

    return &playerGameStatsResult{
        InstanceID: instanceID,
        WinnerType: winnerType,
        WinnerID:   winnerID,
        Player: playerGameStats{
            UserID:           current.UserID,
            Name:             playerName,
            ExpGained:        current.ExpGained,
            PlayerKills:      current.PlayerKills,
            MonsterKills:     current.MonsterKills,
            DamageTotal:      current.DamageTotal,
            DamageToPlayers:  current.DamageToPlayers,
            DamageToMonsters: current.DamageToMonsters,
            Rewards:          current.RewardsData,
        },
    }, nil
}

func buildAllPlayersGameStats(instanceID string, results *game.MatchResults) ([]playerGameStats, string, int, error) {
    if results == nil {
        return nil, "", 0, fmt.Errorf("match results are nil")
    }

    winnerType := "user"
    winnerID := results.WinnerID
    if results.WinnerGroupID > 0 {
        winnerType = "group"
        winnerID = results.WinnerGroupID
    }

    stats := make([]playerGameStats, 0, len(results.PlayerResults))
    for _, pr := range results.PlayerResults {
        playerName := fmt.Sprintf("Player %d", pr.UserID)
        if p, err := repository.GetMatchPlayerByID(instanceID, pr.UserID); err == nil && p != nil && p.Name != "" {
            playerName = p.Name
        }
        stats = append(stats, playerGameStats{
            UserID:           pr.UserID,
            Name:             playerName,
            ExpGained:        pr.ExpGained,
            PlayerKills:      pr.PlayerKills,
            MonsterKills:     pr.MonsterKills,
            DamageTotal:      pr.DamageTotal,
            DamageToPlayers:  pr.DamageToPlayers,
            DamageToMonsters: pr.DamageToMonsters,
            Rewards:          pr.RewardsData,
        })
    }

    return stats, winnerType, winnerID, nil
}

func FinishMatchHandler(w http.ResponseWriter, r *http.Request) {
    // 1. Метод
    if r.Method != http.MethodPost {
        http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
        return
    }
    // 2. Auth: get player user_id from JWT context
    userID, ok := middleware.GetUserIDFromContext(r.Context())
    if !ok || userID == 0 {
        http.Error(w, `{"error":"unauthorized"}`, http.StatusUnauthorized)
        return
    }
    // 3. Декодирование
    defer r.Body.Close()
    var req struct {
        InstanceID string `json:"instanceId"`
    }
    if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
        http.Error(w, `{"error":"invalid JSON"}`, http.StatusBadRequest)
        return
    }
    // 4. Получаем quest_artifact_id матча
    match, err := repository.GetMatchByID(req.InstanceID)
    if err != nil {
        log.Printf("FinishMatchHandler: GetMatchByID error for %s: %v", req.InstanceID, err)
        http.Error(w, `{"error":"match not found"}`, http.StatusNotFound)
        return
    }
    player, err := repository.GetMatchPlayerByID(req.InstanceID, userID)
    if err != nil {
        log.Printf("FinishMatchHandler: GetMatchPlayerByID error for %s user %d: %v", req.InstanceID, userID, err)
        http.Error(w, `{"error":"player not found"}`, http.StatusNotFound)
        return
    }

    // 5. Проверяем, есть ли квест-артефакт у игрока
    if match.QuestArtifactID > 0 {
        has, err := repository.PlayerHasQuestArtifact(req.InstanceID, userID, match.QuestArtifactID)
        if err != nil {
            log.Printf("FinishMatchHandler: PlayerHasQuestArtifact error: %v", err)
            http.Error(w, `{"error":"internal error"}`, http.StatusInternalServerError)
            return
        }
        if !has {
            http.Error(w, `{"error":"quest_artifact_missing"}`, http.StatusForbidden)
            return
        }
    }

    // 5.1. Считаем итоговую статистику до финализации (до удаления матча).
    results, err := repository.GetMatchResults(req.InstanceID)
    if err != nil {
        log.Printf("FinishMatchHandler: GetMatchResults error for %s: %v", req.InstanceID, err)
        http.Error(w, `{"error":"failed_to_collect_stats"}`, http.StatusInternalServerError)
        return
    }
    playerStats, err := buildPlayerGameStats(req.InstanceID, userID, player.Name, results)
    if err != nil {
        log.Printf("FinishMatchHandler: buildPlayerGameStats error for %s user %d: %v", req.InstanceID, userID, err)
        http.Error(w, `{"error":"failed_to_collect_player_stats"}`, http.StatusInternalServerError)
        return
    }

    // 5.2 Отправляем всем событие, что игрок вышел через портал
    portalX, portalY := player.Position.X, player.Position.Y
    var portalPos [2]int
    if err := json.Unmarshal(match.PortalPosition, &portalPos); err == nil {
        portalX, portalY = portalPos[0], portalPos[1]
    }

    portalMsg := map[string]interface{}{
        "type": "PLAYER_LEFT_PORTAL",
        "payload": map[string]interface{}{
            "instanceId": req.InstanceID,
            "playerName": player.Name,
            "x":          portalX,
            "y":          portalY,
        },
    }
    if b, err := json.Marshal(portalMsg); err == nil {
        Broadcast(b)
    }

    // 6. Вызов финализации
    if err := service.FinalizeMatch(req.InstanceID); err != nil {
        log.Printf("FinalizeMatch error for %s: %v", req.InstanceID, err)
        http.Error(w, `{"error":"internal error"}`, http.StatusInternalServerError)
        return
    }

    // 6.1 Рассылаем финальную статистику всем игрокам этого матча.
    if allStats, winnerType, winnerID, err := buildAllPlayersGameStats(req.InstanceID, results); err == nil {
        endedMsg := matchEndedBroadcastResponse{
            Type: "MATCH_ENDED",
            Payload: matchEndedBroadcastResult{
                InstanceID: req.InstanceID,
                WinnerType: winnerType,
                WinnerID:   winnerID,
                Stats:      allStats,
            },
        }
        if b, marshalErr := json.Marshal(endedMsg); marshalErr == nil {
            Broadcast(b)
        }
    } else {
        log.Printf("FinishMatchHandler: buildAllPlayersGameStats error for %s: %v", req.InstanceID, err)
    }

    // 7. Ответ
    w.Header().Set("Content-Type", "application/json")
    w.WriteHeader(http.StatusOK)
    _ = json.NewEncoder(w).Encode(finishMatchStatsResponse{
        Status: "ok",
        Stats:  playerStats,
    })
}
