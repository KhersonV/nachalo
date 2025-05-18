
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
		SELECT  item_count
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
		SET item_count = $1 
		WHERE user_id = $2 AND item_type = $3 AND item_id = $4;
	`
)

// AddInventoryItem добавляет указанное количество предмета в инвентарь игрока.
// Инвентарь хранится в поле inventory таблицы match_players в формате JSON.
// При этом запись в inventory_items обновляется или создаётся вручную.
func AddInventoryItem(playerID int, itemType string, itemID int, count int, image string, description string) error {
    // Начинаем транзакцию
    tx, err := DB.Begin()
    if err != nil {
        return fmt.Errorf("begin tx: %w", err)
    }
    defer tx.Rollback()

    // 1) Получаем текущий JSON-инвентарь и matchInstanceID через tx
    var inventoryJSON string
    var matchInstanceID string
    err = tx.QueryRow(selectInventoryQuery, playerID).Scan(&inventoryJSON, &matchInstanceID)
    if err != nil {
        return fmt.Errorf("fetch inventory: %w", err)
    }

    // 2) Распарсиваем или инициализируем карту
    inv := make(map[string]map[string]interface{})
    if inventoryJSON != "" && inventoryJSON != "{}" {
        if err := json.Unmarshal([]byte(inventoryJSON), &inv); err != nil {
            inv = make(map[string]map[string]interface{})
        }
    }

    // 3) Обновляем map-представление инвентаря
    key := fmt.Sprintf("%s_%d", itemType, itemID)
    if entry, exists := inv[key]; exists {
        if cur, ok := entry["item_count"].(float64); ok {
            entry["item_count"] = cur + float64(count)
        } else {
            entry["item_count"] = count
        }
    } else {
        inv[key] = map[string]interface{}{
            "name":        itemType,
            "item_count":       count,
            "image":       image,
            "description": description,
        }
    }

    // 4) Сериализуем обратно в JSON и обновляем поле в match_players
    newInvBytes, err := json.Marshal(inv)
    if err != nil {
        return fmt.Errorf("marshal inventory: %w", err)
    }
    if _, err := tx.Exec(updatePlayerInventoryQuery, string(newInvBytes), matchInstanceID, playerID); err != nil {
        return fmt.Errorf("update match_players.inventory: %w", err)
    }

    // 5) Пытаемся UPDATE в normalized таблице
    res, err := tx.Exec(`
        UPDATE inventory_items
           SET item_count = item_count + $4
         WHERE user_id = $1
           AND item_type = $2
           AND item_id = $3
    `, playerID, itemType, itemID, count)
    if err != nil {
        return fmt.Errorf("update inventory_items: %w", err)
    }

    rows, err := res.RowsAffected()
    if err != nil {
        return fmt.Errorf("rows affected: %w", err)
    }

    // 6) Если не обновили ни одной строки — INSERT новую
    if rows == 0 {
        if _, err := tx.Exec(`
            INSERT INTO inventory_items (user_id, item_type, item_id, item_count)
            VALUES ($1, $2, $3, $4)
        `, playerID, itemType, itemID, count); err != nil {
            return fmt.Errorf("insert inventory_items: %w", err)
        }
    }

    // 7) Фиксируем транзакцию
    if err := tx.Commit(); err != nil {
        return fmt.Errorf("tx commit: %w", err)
    }

    return nil
}


// RemoveInventoryItemAndSyncJSON уменьшает количество предмета в inventory_items и синхронизирует JSON-инвентарь в match_players.
func RemoveInventoryItemAndSyncJSON(playerID int, itemType string, itemID int, count int) error {
    tx, err := DB.Begin()
    if err != nil {
        return fmt.Errorf("begin tx: %w", err)
    }
    defer tx.Rollback()

    // 1) Получаем текущий JSON-инвентарь и matchInstanceID
    var inventoryJSON, matchInstanceID string
    if err := tx.QueryRow(selectInventoryQuery, playerID).Scan(&inventoryJSON, &matchInstanceID); err != nil {
        return fmt.Errorf("fetch inventory: %w", err)
    }

    // 2) Уменьшаем или удаляем запись в normalized таблице
    //   - сначала пытаемся обновить
    res, err := tx.Exec(`
        UPDATE inventory_items
           SET item_count = item_count - $4
         WHERE user_id = $1
           AND item_type = $2
           AND item_id = $3
    `, playerID, itemType, itemID, count)
    if err != nil {
        return fmt.Errorf("update inventory_items: %w", err)
    }
    rows, err := res.RowsAffected()
    if err != nil {
        return fmt.Errorf("rows affected: %w", err)
    }
    if rows == 0 {
        // ничего не обновилось — удалим на всякий случай
        if _, err := tx.Exec(deleteInventoryItemQuery, playerID, itemType, itemID); err != nil {
            return fmt.Errorf("delete inventory_items: %w", err)
        }
    } else {
        // если после вычитания стало <=0, удалим строку
        if _, err := tx.Exec(`
            DELETE FROM inventory_items
             WHERE user_id = $1
               AND item_type = $2
               AND item_id = $3
               AND item_count <= 0
        `, playerID, itemType, itemID); err != nil {
            return fmt.Errorf("clean zero items: %w", err)
        }
    }

    // 3) Обновляем JSON-поле inventory
    inv := make(map[string]map[string]interface{})
    if inventoryJSON != "" && inventoryJSON != "{}" {
        if err := json.Unmarshal([]byte(inventoryJSON), &inv); err != nil {
            inv = make(map[string]map[string]interface{})
        }
    }
    key := fmt.Sprintf("%s_%d", itemType, itemID)
    if entry, ok := inv[key]; ok {
        // уменьшаем
        if cur, ok2 := entry["item_count"].(float64); ok2 {
            newCount := int(cur) - count
            if newCount > 0 {
                entry["item_count"] = newCount
            } else {
                delete(inv, key)
            }
        } else {
            delete(inv, key)
        }
    }
    // сохраняем обратно
    newInvBytes, err := json.Marshal(inv)
    if err != nil {
        return fmt.Errorf("marshal inventory: %w", err)
    }
    if _, err := tx.Exec(updatePlayerInventoryQuery, string(newInvBytes), matchInstanceID, playerID); err != nil {
        return fmt.Errorf("update match_players.inventory: %w", err)
    }

    if err := tx.Commit(); err != nil {
        return fmt.Errorf("tx commit: %w", err)
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
