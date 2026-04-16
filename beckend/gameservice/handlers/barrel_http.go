// ============================================
// gameservice/handlers/barrel_http.go
// ============================================
package handlers

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"gameservice/game"
	"gameservice/middleware"
	"gameservice/repository"
)

// OpenBarrelRequest – структура запроса на открытие бочки.
type OpenBarrelRequest struct {
	InstanceID string `json:"instance_id"`
	PlayerID   int    `json:"user_id"`
	CellX      int    `json:"cell_x"`
	CellY      int    `json:"cell_y"`
}

// OpenBarrelResponse – структура ответа фронту.
type OpenBarrelResponse struct {
	UpdatedCell   UpdatedCellResponse `json:"updatedCell"`
	UpdatedPlayer interface{}         `json:"updatedPlayer"`
	MatchEnded    bool                `json:"matchEnded,omitempty"`
}

// OpenBarrelHandler — HTTP-хендлер для открытия бочки.
func OpenBarrelHandler(w http.ResponseWriter, r *http.Request) {
	// 1) читаем JSON
	if r.Method != http.MethodPost {
		http.Error(w, "only POST is allowed", http.StatusMethodNotAllowed)
		return
	}
	body, _ := io.ReadAll(r.Body)
	r.Body = io.NopCloser(bytes.NewReader(body))

	var req OpenBarrelRequest
	if err := json.Unmarshal(body, &req); err != nil {
		http.Error(w, fmt.Sprintf("парсинг запроса: %v", err), http.StatusBadRequest)
		return
	}
	tokenUserID, ok := middleware.GetUserIDFromContext(r.Context())
	if !ok || tokenUserID != req.PlayerID {
		http.Error(w, "forbidden", http.StatusForbidden)
		return
	}

	lockPlayer(req.PlayerID)
	defer unlockPlayer(req.PlayerID)

	matchState, ok := game.GetMatchState(req.InstanceID)
	if !ok {
		http.Error(w, "match not found", http.StatusNotFound)
		return
	}
	if matchState.ActiveUserID != req.PlayerID {
		http.Error(w, "it's not your turn", http.StatusBadRequest)
		return
	}

	// 2) находим cell точечно
	cell, err := repository.LoadMapCell(req.InstanceID, req.CellX, req.CellY)
	if err != nil {
		http.Error(w, fmt.Sprintf("загрузка клетки: %v", err), http.StatusInternalServerError)
		return
	}
	if cell == nil || cell.Barbel == nil {
		http.Error(w, "бочка не найдена в этой клетке", http.StatusBadRequest)
		return
	}

	// 4) для открытия берём справочники
	resList, err := repository.GetResourcesData()
	if err != nil {
		http.Error(w, fmt.Sprintf("загрузка ресурсов: %v", err), http.StatusInternalServerError)
		return
	}
	artList, err := repository.GetArtifactsData()
	if err != nil {
		http.Error(w, fmt.Sprintf("загрузка артефактов: %v", err), http.StatusInternalServerError)
		return
	}
	// 5) ВЫНОСИМ HandleOpenBarrel в отдельную строку
	updatedCell, updatedPlayer, matchEnded, err := HandleOpenBarrel(
		*cell, req.InstanceID, req.PlayerID, resList, artList,
	)
	if err != nil {
		http.Error(w, fmt.Sprintf("open barrel: %v", err), http.StatusInternalServerError)
		return
	}

	// 6) Формируем ответ — без повторного GetMatchPlayerByID
	resp := OpenBarrelResponse{
		UpdatedCell:   serialiseUpdatedCell(updatedCell),
		UpdatedPlayer: *updatedPlayer,
		MatchEnded:    matchEnded,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}
