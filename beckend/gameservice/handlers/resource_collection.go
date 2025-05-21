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

	// 2) Загрузка карты и поиск нужной ячейки
	var mapJSON string
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

	// 3) Проверяем, есть ли ресурс в этой ячейке
	resData, ok := target["resource"].(map[string]interface{})
	if !ok {
		// нет ресурса — выходим молча
		return
	}

	// Сохраняем нужные поля ресурса ДО очистки клетки
	itemID := int(resData["id"].(float64))
	itemName := resData["type"].(string)
	itemDesc := resData["description"].(string)
	itemImage := resData["image"].(string)        // картинка ресурса


	// 4) Очищаем клетку
	target["resource"] = nil
	target["tileCode"] = float64(48)
	target["monster"] = nil
	target["isPortal"] = false

	// 5) Сохраняем обновлённую карту
	updatedMap, _ := json.Marshal(cells)
	if _, err := repository.DB.Exec(
		`UPDATE matches SET map=$1 WHERE instance_id=$2`,
		string(updatedMap), req.InstanceID,
	); err != nil {
		http.Error(w, fmt.Sprintf("сохранение карты: %v", err), http.StatusInternalServerError)
		return
	}

	



if err := repository.AddInventoryItem(
    req.InstanceID,   // string
    req.PlayerID,     // int
    "resource",       // string
    itemID,           // int
    itemName,         // string
    itemImage,        // string — imageURL
    itemDesc,         // string — itemDescription
    1,                // int — count
); err != nil {
    http.Error(w, fmt.Sprintf("добавление в инвентарь: %v", err), http.StatusInternalServerError)
    return
}
	// 7) Формируем и отправляем ответ клиенту
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
	msgBuf, _ := json.Marshal(wsMsg)
	Broadcast(msgBuf)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"message":       "успешно",
		"updatedCell":   updatedCell,
		"updatedPlayer": playerResp,
	})
}
