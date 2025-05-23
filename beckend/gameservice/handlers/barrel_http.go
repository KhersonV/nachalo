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
	MatchEnded    bool                 `json:"matchEnded,omitempty"`
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

	// 2) находим cell в карте
	mapJSON := ""
	if err := repository.DB.
		QueryRow(`SELECT map FROM matches WHERE instance_id=$1`, req.InstanceID).
		Scan(&mapJSON); err != nil {
		http.Error(w, fmt.Sprintf("загрузка карты: %v", err), http.StatusInternalServerError)
		return
	}

	var cells []map[string]interface{}
	json.Unmarshal([]byte(mapJSON), &cells)
	 if err := json.Unmarshal([]byte(mapJSON), &cells); err != nil {
        http.Error(w, fmt.Sprintf("json.Unmarshal карты: %v", err), http.StatusInternalServerError)
        return
    }

	var target map[string]interface{}
	for _, c := range cells {
		if int(c["x"].(float64)) == req.CellX && int(c["y"].(float64)) == req.CellY {
			target = c
			break
		}
	}
	if target == nil || target["barbel"] == nil {
		http.Error(w, "бочка не найдена в этой клетке", http.StatusBadRequest)
		return
	}

	// 3) собираем game.FullCell
	cell := game.FullCell{
		CellID:   int(target["cell_id"].(float64)),
		X:        req.CellX,
		Y:        req.CellY,
		TileCode: int(target["tileCode"].(float64)),
		Resource: nil,
		Monster:  nil,
		IsPortal: target["isPortal"].(bool),
		IsPlayer: target["isPlayer"].(bool),
	}
	bar := target["barbel"].(map[string]interface{})
	eff := make(map[string]int, len(bar["effect"].(map[string]interface{})))
    for k, v := range bar["effect"].(map[string]interface{}) {
        eff[k] = int(v.(float64))
    }

	cell.Barbel = &game.ResourceData{
		ID:          int(bar["id"].(float64)),
		Type:        bar["type"].(string),
		Description: bar["description"].(string),
		Effect:      eff,
		Image:       bar["image"].(string),
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
        cell, req.InstanceID, req.PlayerID, resList, artList,
    )
    if err != nil {
        http.Error(w, fmt.Sprintf("open barrel: %v", err), http.StatusInternalServerError)
        return
    }

		cell.Barbel = nil
		cell.TileCode = 48 

	    // 6) Формируем ответ — без повторного GetMatchPlayerByID
    resp := OpenBarrelResponse{
        UpdatedCell:   serialiseUpdatedCell(updatedCell),
        UpdatedPlayer: *updatedPlayer,
        MatchEnded:    matchEnded,
    }

    w.Header().Set("Content-Type", "application/json")
    json.NewEncoder(w).Encode(resp)
}


