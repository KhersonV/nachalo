
// ============================================
// gameservice/handlers/finish_match_handler.go
// ============================================

package handlers

import (
	"encoding/json"
	"log"
	"net/http"

	"gameservice/middleware"
	"gameservice/repository"
	"gameservice/service"
)

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
    // 6. Вызов финализации
    if err := service.FinalizeMatch(req.InstanceID); err != nil {
        log.Printf("FinalizeMatch error for %s: %v", req.InstanceID, err)
        http.Error(w, `{"error":"internal error"}`, http.StatusInternalServerError)
        return
    }
    // 7. Ответ
    w.Header().Set("Content-Type", "application/json")
    w.WriteHeader(http.StatusOK)
    w.Write([]byte(`{"status":"ok"}`))
}
