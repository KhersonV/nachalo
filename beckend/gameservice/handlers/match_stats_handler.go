package handlers

import (
    "encoding/json"
    "net/http"

    "gameservice/middleware"

    "github.com/gorilla/mux"
)

type myMatchStatsResponse struct {
    Status string                 `json:"status"`
    Stats  *playerGameStatsResult `json:"stats,omitempty"`
}

// GetMyMatchStatsHandler возвращает сохраненную статистику текущего пользователя
// по instance_id из таблиц match_stats/match_player_stats.
func GetMyMatchStatsHandler(w http.ResponseWriter, r *http.Request) {
    userID, ok := middleware.GetUserIDFromContext(r.Context())
    if !ok || userID == 0 {
        http.Error(w, `{"error":"unauthorized"}` , http.StatusUnauthorized)
        return
    }

    instanceID := mux.Vars(r)["instance_id"]
    if instanceID == "" {
        http.Error(w, `{"error":"instance_id_required"}`, http.StatusBadRequest)
        return
    }

    allStats, winnerType, winnerID, err := buildAllPlayersGameStatsFromStored(instanceID)
    if err != nil {
        http.Error(w, `{"error":"stats_not_ready"}`, http.StatusNotFound)
        return
    }

    var me *playerGameStats
    for i := range allStats {
        if allStats[i].UserID == userID {
            me = &allStats[i]
            break
        }
    }
    if me == nil {
        http.Error(w, `{"error":"player_stats_not_found"}`, http.StatusNotFound)
        return
    }

    w.Header().Set("Content-Type", "application/json")
    _ = json.NewEncoder(w).Encode(myMatchStatsResponse{
        Status: "ok",
        Stats: &playerGameStatsResult{
            InstanceID: instanceID,
            WinnerType: winnerType,
            WinnerID:   winnerID,
            Player:     *me,
        },
    })
}
