
// ============================================
// gameservice/handlers/finish_match_handler.go
// ============================================

package handlers

import (
	"log"
	"net/http"
	"encoding/json"
	"gameservice/service"
)

func FinishMatchHandler(w http.ResponseWriter, r *http.Request) {
    // 1. Метод
    if r.Method != http.MethodPost {
        http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
        return
    }
    // 2. Декодирование
    defer r.Body.Close()
    var req struct {
        InstanceID string `json:"instanceId"`
    }
    if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
        http.Error(w, `{"error":"invalid JSON"}`, http.StatusBadRequest)
        return
    }
    // 3. Вызов финализации
    if err := service.FinalizeMatch(req.InstanceID); err != nil {
        log.Printf("FinalizeMatch error for %s: %v", req.InstanceID, err)
        http.Error(w, `{"error":"internal error"}`, http.StatusInternalServerError)
        return
    }
    // 4. Ответ
    w.Header().Set("Content-Type", "application/json")
    w.WriteHeader(http.StatusOK)
    w.Write([]byte(`{"status":"ok"}`))
}
