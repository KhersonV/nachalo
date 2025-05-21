// ============================================
// gameservice/handlers/resource_collection.go
// ============================================

package handlers

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"gameservice/repository"
)

// CollectResourceRequest – структура запроса для сбора ресурса
type CollectResourceRequest struct {
	InstanceID string `json:"instance_id"`
	PlayerID   int    `json:"user_id"`
	CellX      int    `json:"cell_x"`
	CellY      int    `json:"cell_y"`
}

// UpdatedCellResponse – структура ответа для обновлённой клетки.
type UpdatedCellResponse struct {
	CellID   int         `json:"cell_id"`
	X        int         `json:"x"`
	Y        int         `json:"y"`
	TileCode int         `json:"tileCode"`
	Resource interface{} `json:"resource"`
	Barbel   interface{} `json:"barbel"`
	Monster  interface{} `json:"monster"`
	IsPortal bool        `json:"isPortal"`
	IsPlayer bool        `json:"isPlayer"`
}

// CollectResourceHandler обрабатывает и сбор ресурсов, и открытие бочек.
func CollectResourceHandler(w http.ResponseWriter, r *http.Request) {
	// 1) Чтение и парсинг запроса
	body, _ := io.ReadAll(r.Body)
	r.Body = io.NopCloser(bytes.NewReader(body))

	var req CollectResourceRequest
	if err := json.Unmarshal(body, &req); err != nil {
		http.Error(w, fmt.Sprintf("парсинг запроса: %v", err), http.StatusBadRequest)
		return
	}
	if req.InstanceID == "" || req.PlayerID == 0 {
		http.Error(w, "instance_id и user_id обязательны", http.StatusBadRequest)
		return
	}

	// 2) Загрузка карты и поиска нужной ячейки
	mapJSON := ""
	if err := repository.DB.
		QueryRow(`SELECT map FROM matches WHERE instance_id=$1`, req.InstanceID).
		Scan(&mapJSON); err != nil {
		http.Error(w, fmt.Sprintf("загрузка карты: %v", err), http.StatusInternalServerError)
		return
	}
	var cells []map[string]interface{}
	if err := json.Unmarshal([]byte(mapJSON), &cells); err != nil {
		http.Error(w, fmt.Sprintf("парсинг карты: %v", err), http.StatusInternalServerError)
		return
	}

	var target map[string]interface{}
	for _, c := range cells {
		if int(c["x"].(float64)) == req.CellX && int(c["y"].(float64)) == req.CellY {
			target = c
			break
		}
	}
	if target == nil {
		http.Error(w, "клетка не найдена", http.StatusBadRequest)
		return
	}

	// 4) Иначе — обычный ресурс
	if target["resource"] == nil {
		return
	}
	// Копируем данные ресурса
	resMap := target["resource"].(map[string]interface{})
	// Убираем ресурс из карты
	target["resource"] = nil
	target["tileCode"] = float64(48)
	target["monster"] = nil
	target["isPortal"] = false

	// Сохраняем карту
	newMap, _ := json.Marshal(cells)
	repository.DB.Exec(`UPDATE matches SET map=$1 WHERE instance_id=$2`, string(newMap), req.InstanceID)

	   // Добавляем ресурс в инвентарь
   itemType := resMap["type"].(string)
   itemID   := int(resMap["id"].(float64))
   imageURL := resMap["image"].(string)
   desc     := resMap["description"].(string)
   if err := repository.AddInventoryItem(
       req.InstanceID, // match-instance
       req.PlayerID,
       itemType,
       itemID,
       desc,     // itemName
       imageURL, // imageURL
       desc,     // description
       1,        // count
   ); err != nil {
       http.Error(w, fmt.Sprintf("добавление в инвентарь: %v", err), http.StatusInternalServerError)
       return
   }

	// Формируем HTTP-ответ точно как раньше
	updatedCell := UpdatedCellResponse{
		CellID:   int(target["cell_id"].(float64)),
		X:        int(target["x"].(float64)),
		Y:        int(target["y"].(float64)),
		TileCode: int(target["tileCode"].(float64)),
		Resource: nil,
		Barbel:   target["barbel"],
		Monster:  target["monster"],
		IsPortal: target["isPortal"].(bool),
		IsPlayer: target["isPlayer"].(bool),
	}
	playerResp, _ := repository.GetMatchPlayerByID(req.InstanceID, req.PlayerID)

	wsMsg := map[string]interface{}{
		"type": "RESOURCE_COLLECTED",
		"payload": map[string]interface{}{
			"updatedCell":   updatedCell,
			"updatedPlayer": playerResp,
		},
	}
	buf, _ := json.Marshal(wsMsg)
	Broadcast(buf)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"message":       "успешно",
		"updatedCell":   updatedCell,
		"updatedPlayer": playerResp,
	})
}
