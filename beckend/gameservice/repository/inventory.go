
// ====================================
// gameservice/repository/inventory.go
// ====================================

package repository

import (
	"encoding/json"
	"fmt"
)

// Определяем константы для SQL-запросов:

const (
	// Запрос для получения инвентаря и match_instance_id игрока
	selectInventoryQuery = `
		SELECT inventory, match_instance_id 
		FROM match_players 
		WHERE user_id = $1 
		LIMIT 1;
	`

	// Запрос для обновления инвентаря в таблице match_players
	updatePlayerInventoryQuery = `
		UPDATE match_players 
		SET inventory = $1 
		WHERE match_instance_id = $2 AND user_id = $3;
	`

	// Запрос для получения количества предмета в таблице inventory_items
	selectInventoryItemCountQuery = `
		SELECT count 
		FROM inventory_items 
		WHERE user_id = $1 AND item_type = $2 AND item_id = $3;
	`

	// Запрос для удаления записи из inventory_items
	deleteInventoryItemQuery = `
		DELETE FROM inventory_items 
		WHERE user_id = $1 AND item_type = $2 AND item_id = $3;
	`

	// Запрос для обновления количества предмета в inventory_items
	updateInventoryItemCountQuery = `
		UPDATE inventory_items 
		SET count = $1 
		WHERE user_id = $2 AND item_type = $3 AND item_id = $4;
	`
)

// AddInventoryItem добавляет указанное количество предмета в инвентарь игрока.
// Инвентарь хранится в поле inventory таблицы match_players в формате JSON.

func AddInventoryItem(playerID int, itemType string, itemID int, count int, image string, description string) error {

	var inventoryJSON string
	var matchInstanceID string

	// Выполняем запрос для получения текущего инвентаря
	err := DB.QueryRow(selectInventoryQuery, playerID).Scan(&inventoryJSON, &matchInstanceID)
	if err != nil {
		return fmt.Errorf("error fetching inventory and match instance: %w", err)
	}

	// Если инвентарь пустой, инициализируем пустую мапу
	inv := make(map[string]map[string]interface{})
	if inventoryJSON != "" && inventoryJSON != "{}" {
		if err := json.Unmarshal([]byte(inventoryJSON), &inv); err != nil {
			// Если не удалось распарсить, инициализируем новый инвентарь
			inv = make(map[string]map[string]interface{})
		}
	}

	// Формируем ключ в формате "itemType_itemID"
	key := fmt.Sprintf("%s_%d", itemType, itemID)
	if entry, exists := inv[key]; exists {
		// Если предмет уже существует, увеличиваем его количество
		if currentCount, ok := entry["count"].(float64); ok {
			entry["count"] = currentCount + float64(count)
		} else {
			entry["count"] = count
		}
	} else {
		// Если предмета ещё нет, создаём новую запись
		inv[key] = map[string]interface{}{
			"name":        itemType,
			"count":       count,
			"image":       image,
			"description": description,
		}
	}

	// Преобразуем обновлённую мапу обратно в JSON
	newInvBytes, err := json.Marshal(inv)
	if err != nil {
		return fmt.Errorf("error marshalling updated inventory: %w", err)
	}

	// Обновляем инвентарь игрока в базе данных
	err = UpdatePlayerInventory(matchInstanceID, playerID, string(newInvBytes))
	if err != nil {
		return fmt.Errorf("error updating player inventory: %w", err)
	}

	return nil
}

// RemoveInventoryItem уменьшает или удаляет количество предмета в инвентаре игрока.
func RemoveInventoryItem(playerID int, itemType string, itemID int, count int) error {
	var existingCount int

	// Выполняем запрос для получения текущего количества предмета
	err := DB.QueryRow(selectInventoryItemCountQuery, playerID, itemType, itemID).Scan(&existingCount)
	if err != nil {
		return fmt.Errorf("error fetching item count: %w", err)
	}

	newCount := existingCount - count
	if newCount <= 0 {
		// Если новое количество <= 0, удаляем запись
		_, err = DB.Exec(deleteInventoryItemQuery, playerID, itemType, itemID)
		if err != nil {
			return fmt.Errorf("error deleting inventory item: %w", err)
		}
	} else {
		// Иначе обновляем количество предмета
		_, err = DB.Exec(updateInventoryItemCountQuery, newCount, playerID, itemType, itemID)
		if err != nil {
			return fmt.Errorf("error updating inventory item count: %w", err)
		}
	}

	return nil
}

// UpdatePlayerInventory обновляет поле inventory в таблице match_players для указанного игрока и матча.
func UpdatePlayerInventory(matchInstanceID string, playerID int, newInventory string) error {
	result, err := DB.Exec(updatePlayerInventoryQuery, newInventory, matchInstanceID, playerID)
	if err != nil {
		return fmt.Errorf("error executing update query for inventory: %w", err)
	}

	// Можно добавить дополнительную проверку затронутых строк
	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("error fetching rows affected: %w", err)
	}
	if rowsAffected == 0 {
		return fmt.Errorf("no rows updated for playerID %d", playerID)
	}

	return nil
}
