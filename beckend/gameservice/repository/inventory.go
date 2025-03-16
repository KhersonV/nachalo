
// ====================================
// gameservice/repository/inventory.go
// ====================================

package repository

import (
	"encoding/json"
	"fmt"
)

// AddInventoryItem добавляет указанное количество предмета в инвентарь игрока.
// Инвентарь хранится в поле inventory таблицы match_players в формате JSON.
// Дополнительно принимает image и description для сохранения данных.
func AddInventoryItem(playerID int, itemType string, itemID int, count int, image string, description string) error {
	// Получаем текущее значение инвентаря и match_instance_id для игрока из match_players.
	var inventoryJSON string
	var matchInstanceID string
	// Используем "user_id" вместо "player_id"
	query := `SELECT inventory, match_instance_id FROM match_players WHERE user_id = $1 LIMIT 1;`
	err := DB.QueryRow(query, playerID).Scan(&inventoryJSON, &matchInstanceID)
	if err != nil {
		return err
	}

	// Если инвентарь пустой, используем пустую мапу.
	inv := make(map[string]map[string]interface{})
	if inventoryJSON != "" && inventoryJSON != "{}" {
		if err := json.Unmarshal([]byte(inventoryJSON), &inv); err != nil {
			inv = make(map[string]map[string]interface{})
		}
	}

	// Используем ключ вида "itemType_itemID" для хранения данных.
	key := fmt.Sprintf("%s_%d", itemType, itemID)
	if entry, exists := inv[key]; exists {
		// Если предмет уже есть – увеличиваем количество.
		if currentCount, ok := entry["count"].(float64); ok {
			entry["count"] = currentCount + float64(count)
		} else {
			entry["count"] = count
		}
	} else {
		// Если предмета ещё нет, создаём новую запись.
		inv[key] = map[string]interface{}{
			"name":        itemType,
			"count":       count,
			"image":       image,
			"description": description,
		}
	}

	// Преобразуем обновлённую мапу обратно в JSON.
	newInvBytes, err := json.Marshal(inv)
	if err != nil {
		return err
	}

	// Обновляем запись в match_players.
	return UpdatePlayerInventory(matchInstanceID, playerID, string(newInvBytes))
}

// RemoveInventoryItem уменьшает или удаляет количество предмета в инвентаре игрока.
func RemoveInventoryItem(playerID int, itemType string, itemID int, count int) error {
	var existingCount int
	// Если используется таблица inventory_items, убедитесь, что в ней поле для игрока называется корректно.
	err := DB.QueryRow(
		`SELECT count FROM inventory_items 
         WHERE user_id = $1 AND item_type = $2 AND item_id = $3`,
		playerID, itemType, itemID,
	).Scan(&existingCount)
	if err != nil {
		return err
	}
	newCount := existingCount - count
	if newCount <= 0 {
		// Удаляем запись, если количество <= 0.
		_, err = DB.Exec(
			`DELETE FROM inventory_items 
             WHERE user_id = $1 AND item_type = $2 AND item_id = $3`,
			playerID, itemType, itemID,
		)
		return err
	}
	// Иначе обновляем.
	_, err = DB.Exec(
		`UPDATE inventory_items SET count = $1 
         WHERE user_id = $2 AND item_type = $3 AND item_id = $4`,
		newCount, playerID, itemType, itemID,
	)
	return err
}

// UpdatePlayerInventory обновляет поле inventory в таблице match_players для указанного игрока и матча.
func UpdatePlayerInventory(matchInstanceID string, playerID int, newInventory string) error {
	query := `
		UPDATE match_players 
		SET inventory = $1 
		WHERE match_instance_id = $2 AND user_id = $3;
	`
	_, err := DB.Exec(query, newInventory, matchInstanceID, playerID)
	return err
}
