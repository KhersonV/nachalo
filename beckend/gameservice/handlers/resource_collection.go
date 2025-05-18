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

// CollectResourceRequest – структура запроса для сбора ресурса.
type CollectResourceRequest struct {
	InstanceID string `json:"instance_id"` // JSON-тег именно instance_id
	PlayerID   int    `json:"user_id"`     // идентификатор игрока
	CellX      int    `json:"cell_x"`
	CellY      int    `json:"cell_y"`
}

// UpdatedCellResponse – структура ответа для обновлённой клетки.
type UpdatedCellResponse struct {
	CellID   int         `json:"cell_id"`
	X        int         `json:"x"`
	Y        int         `json:"y"`
	TileCode int         `json:"tileCode"`
	Resource interface{} `json:"resource"` // можно заменить на конкретный тип, если известно
	Barbel   interface{} `json:"barbel"`
	Monster  interface{} `json:"monster"`
	IsPortal bool        `json:"isPortal"`
	IsPlayer bool        `json:"isPlayer"`
}

// CollectResourceHandler обрабатывает запрос на сбор ресурса.
// Он обновляет карту матча, удаляя ресурс из клетки, добавляет ресурс в инвентарь игрока
// и возвращает обновлённую клетку с явной структурой ответа.
func CollectResourceHandler(w http.ResponseWriter, r *http.Request) {
	// Читаем тело запроса для логирования и дальнейшей обработки.
	bodyBytes, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, "Не удалось прочитать тело запроса", http.StatusBadRequest)
		return
	}
	fmt.Printf("Получено тело запроса: %s\n", string(bodyBytes))

	// Восстанавливаем r.Body.
	r.Body = io.NopCloser(bytes.NewReader(bodyBytes))

	// Парсинг запроса.
	var req CollectResourceRequest
	if err := json.Unmarshal(bodyBytes, &req); err != nil {
		http.Error(w, fmt.Sprintf("Ошибка парсинга запроса: %v", err), http.StatusBadRequest)
		return
	}
	fmt.Printf("Parsed Request: InstanceID: '%s', PlayerID: %d, CellX: %d, CellY: %d\n",
		req.InstanceID, req.PlayerID, req.CellX, req.CellY)

	// Проверка обязательных полей.
	if req.InstanceID == "" {
		http.Error(w, "instance_id обязателен", http.StatusBadRequest)
		return
	}
	if req.PlayerID == 0 {
		http.Error(w, "user_id обязателен", http.StatusBadRequest)
		return
	}

	// Получаем карту матча из БД.
	var mapJSON string
	query := `SELECT map FROM matches WHERE instance_id = $1;`
	err = repository.DB.QueryRow(query, req.InstanceID).Scan(&mapJSON)
	if err != nil {
		http.Error(w, fmt.Sprintf("Ошибка получения карты: %v", err), http.StatusInternalServerError)
		return
	}

	// Десериализуем карту как срез клеток.
	var cells []map[string]interface{}
	if err := json.Unmarshal([]byte(mapJSON), &cells); err != nil {
		http.Error(w, fmt.Sprintf("Ошибка парсинга карты: %v", err), http.StatusInternalServerError)
		return
	}

	// Ищем клетку с указанными координатами.
	var targetCell map[string]interface{}
	cellFound := false
	for i, cell := range cells {
		// При парсинге JSON числа становятся float64, поэтому приводим.
		if int(cell["x"].(float64)) == req.CellX && int(cell["y"].(float64)) == req.CellY {
			targetCell = cells[i]
			cellFound = true
			break
		}
	}
	if !cellFound {
		http.Error(w, "Клетка не найдена", http.StatusBadRequest)
		return
	}

	// Проверяем, что в клетке есть ресурс.
	if targetCell["resource"] == nil {
		http.Error(w, "В данной клетке нет ресурса", http.StatusBadRequest)
		return
	}

	// Сохраняем данные ресурса для добавления в инвентарь.
	resDataCopy := targetCell["resource"]

	// Обновляем клетку: удаляем ресурс, устанавливаем tileCode в 48 (проходимая клетка),
	// сбрасываем монстра и флаг портала.
	targetCell["resource"] = nil
	targetCell["tileCode"] = float64(48)
	targetCell["monster"] = nil
	targetCell["isPortal"] = false

	// Сериализуем обновлённую карту и сохраняем её в БД.
	newMapJSONBytes, err := json.Marshal(cells)
	if err != nil {
		http.Error(w, fmt.Sprintf("Ошибка сериализации обновленной карты: %v", err), http.StatusInternalServerError)
		return
	}
	newMapJSON := string(newMapJSONBytes)

	updateQuery := `UPDATE matches SET map = $1 WHERE instance_id = $2;`
	_, err = repository.DB.Exec(updateQuery, newMapJSON, req.InstanceID)
	if err != nil {
		http.Error(w, fmt.Sprintf("Ошибка обновления карты в БД: %v", err), http.StatusInternalServerError)
		return
	}

	// Добавляем ресурс в инвентарь игрока.
	resDataMap, ok := resDataCopy.(map[string]interface{})
	if !ok {
		http.Error(w, "Данные ресурса недоступны", http.StatusInternalServerError)
		return
	}
	itemType, _ := resDataMap["type"].(string)
	var itemID int
	if idVal, exists := resDataMap["id"]; exists {
		itemID = int(idVal.(float64))
	}
	imageStr, _ := resDataMap["image"].(string)
	descriptionStr, _ := resDataMap["description"].(string)
	err = repository.AddInventoryItem(req.PlayerID, itemType, itemID, 1, imageStr, descriptionStr)
	if err != nil {
		http.Error(w, fmt.Sprintf("Ошибка добавления ресурса в инвентарь: %v", err), http.StatusInternalServerError)
		return
	}

	// Формируем ответ с явной структурой UpdatedCellResponse.
updatedCell := UpdatedCellResponse{
    CellID:   int(targetCell["cell_id"].(float64)),
    X:        int(targetCell["x"].(float64)),
    Y:        int(targetCell["y"].(float64)),
    TileCode: int(targetCell["tileCode"].(float64)),
    Resource: targetCell["resource"], // теперь nil
    Barbel:   targetCell["barbel"],
    Monster:  targetCell["monster"],
    IsPortal: targetCell["isPortal"].(bool),
    IsPlayer: targetCell["isPlayer"].(bool),
}

playerResp, err := repository.GetMatchPlayerByUserID(req.PlayerID)
	if err != nil {
		http.Error(w, fmt.Sprintf("Ошибка получения игрока: %v", err), http.StatusInternalServerError)
		return
	}

// Создаем сообщение для WS, которое уведомляет всех клиентов о сборе ресурса.
wsMsg := map[string]interface{}{
    "type": "RESOURCE_COLLECTED",
    "payload": map[string]interface{}{
        "updatedCell": updatedCell,
		"updatedPlayer": playerResp,
        // Если требуется, можно добавить данные игрока или инвентаря.
    },
}

// Отправляем сообщение всем подключенным клиентам.
wsMsgBytes, _ := json.Marshal(wsMsg)
Broadcast(wsMsgBytes)

// Затем отправляем ответ клиенту, который инициировал запрос.
response := map[string]interface{}{
    "message":     "Ресурс собран",
    "updatedCell": updatedCell,
	"updatedPlayer": playerResp,
}
w.Header().Set("Content-Type", "application/json")
json.NewEncoder(w).Encode(response)
}
