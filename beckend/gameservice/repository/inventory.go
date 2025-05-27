// ====================================
// gameservice/repository/inventory.go
// ====================================

package repository

import (
	"encoding/json"
	"fmt"
)

const (
	// Запрос для получения JSON-инвентаря и instance_id игрока
	selectInventoryQuery = `
        SELECT inventory, instance_id
        FROM match_players
		WHERE instance_id = $1
        AND user_id = $2
        LIMIT 1;
    `

	// Запрос для обновления JSON-инвентаря в таблице match_players
	updatePlayerInventoryQuery = `
        UPDATE match_players
        SET inventory = $1
        WHERE instance_id = $2
          AND user_id = $3;
    `

	// // Запрос для получения количества предмета в normalized таблице inventory_items
	// // для заданной комбинации (user_id, instance_id, item_type, item_id)
	// selectInventoryItemCountQuery = `
	//     SELECT item_count
	//     FROM inventory_items
	//     WHERE user_id = $1
	//       AND instance_id = $2
	//       AND item_type = $3
	//       AND item_id = $4;
	// `

	// Запрос для обновления количества предмета в inventory_items
	updateInventoryItemCountQuery = `
UPDATE inventory_items
SET item_count = item_count + $5
WHERE user_id = $2
  AND instance_id = $1
  AND item_type = $3
  AND item_id = $4;`

	// Запрос для удаления записи из inventory_items
	deleteInventoryItemQuery = `
        DELETE FROM inventory_items
        WHERE instance_id = $1
          AND user_id = $2
          AND item_type = $3
          AND item_id = $4;
    `
	deleteZeroInventoryItemQuery = `
        DELETE FROM inventory_items
        WHERE   instance_id = $1
          AND   user_id = $2
          AND item_type = $3
          AND item_id = $4
          AND item_count <= 0;
    `
	// Запрос для вставки новой записи в inventory_items
	insertInventoryItemQuery = `
    INSERT INTO inventory_items
    (instance_id, user_id, item_type, item_id, item_name, item_description, item_count)
    VALUES
    ($1,           $2,     $3,        $4,      $5,           $6,               $7);`
)

// AddInventoryItem добавляет указанное количество предмета в инвентарь игрока.
// При этом:
// 1) Считывает текущий JSON-инвентарь и instance_id из match_players.
// 2) Обновляет JSON-инвентарь в таблице match_players.
// 3) Обновляет или вставляет запись в normalized таблице inventory_items.

func AddInventoryItem(
	instanceID string,
	playerID int,
	itemType string,
	itemID int,
	itemName string,
	imageURL string,
	description string,
	count int,
) error {
	tx, err := DB.Begin()
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback()

	// 1) Получаем текущий JSON-инвентарь и matchInstanceID
	var inventoryJSON, matchInstanceID string
	if err := tx.
		QueryRow(selectInventoryQuery, instanceID, playerID).
		Scan(&inventoryJSON, &matchInstanceID); err != nil {
		return fmt.Errorf("fetch inventory: %w", err)
	}

	// 2) Распарсиваем JSON в map или инициализируем пустой map
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
			"name":        itemName,
			"item_count":  count,
			"image":       imageURL,
			"description": description,
		}
	}

	// 4) Сохраняем JSON-инвентарь обратно в match_players
	newInvBytes, err := json.Marshal(inv)
	if err != nil {
		return fmt.Errorf("marshal inventory: %w", err)
	}
	if _, err := tx.Exec(
		updatePlayerInventoryQuery,
		string(newInvBytes),
		matchInstanceID,
		playerID,
	); err != nil {
		return fmt.Errorf("update match_players.inventory: %w", err)
	}

	// 5) Обновляем или вставляем строку в inventory_items
	res, err := tx.Exec(
		updateInventoryItemCountQuery,
		instanceID, // $1
		playerID,   // $2
		itemType,   // $3
		itemID,     // $4
		count,      // $5
	)
	if err != nil {
		return fmt.Errorf("update inventory_items: %w", err)
	}
	rows, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("rows affected: %w", err)
	}
	if rows == 0 {
		if _, err := tx.Exec(
			insertInventoryItemQuery,
			instanceID,
			playerID,
			itemType,
			itemID,
			itemName,
			description,
			count,
		); err != nil {
			return fmt.Errorf("insert inventory_items: %w", err)
		}
	}

	// 6) Закрываем транзакцию
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("tx commit: %w", err)
	}
	return nil
}

// RemoveInventoryItemAndSyncJSON уменьшает количество предмета в inventory_items
// и синхронизирует JSON-инвентарь в match_players:
// 1) Считывает JSON-инвентарь и instance_id.
// 2) Пытается UPDATE или DELETE в inventory_items.
// 3) Пересчитывает JSON и обновляет поле inventory.
func RemoveInventoryItemAndSyncJSON(instanceID string, playerID int, itemType string, itemID int, count int) error {
	tx, err := DB.Begin()
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback()

	// 1) Получаем текущий JSON-инвентарь и matchInstanceID
	var inventoryJSON, matchInstanceID string
	if err := tx.QueryRow(selectInventoryQuery, instanceID, playerID).Scan(&inventoryJSON, &matchInstanceID); err != nil {
		return fmt.Errorf("fetch inventory: %w", err)
	}

	// 2) Пытаемся вычесть count из inventory_items
	res, err := tx.Exec(
		updateInventoryItemCountQuery,
		matchInstanceID,
		playerID,
		itemType,
		itemID,
		-count,
	)
	if err != nil {
		return fmt.Errorf("update inventory_items: %w", err)
	}
	rows, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("rows affected: %w", err)
	}

	// 2a) Если не было строк для обновления — удаляем запись целиком
	if rows == 0 {
		if _, err := tx.Exec(
			deleteInventoryItemQuery,
			matchInstanceID, // $1
			playerID,        // $2
			itemType,        // $3
			itemID,          // $4
		); err != nil {
			return fmt.Errorf("delete inventory_items: %w", err)
		}
	} else {
		// 2b) Если после вычитания стало <= 0 — удаляем строку
		if _, err := tx.Exec(
			deleteZeroInventoryItemQuery,
			matchInstanceID,
			playerID,
			itemType,
			itemID,
		); err != nil {
			return fmt.Errorf("clean zero items: %w", err)
		}
	}

	// 3) Обновляем JSON-инвентарь в match_players
	inv := make(map[string]map[string]interface{})
	if inventoryJSON != "" && inventoryJSON != "{}" {
		if err := json.Unmarshal([]byte(inventoryJSON), &inv); err != nil {
			inv = make(map[string]map[string]interface{})
		}
	}
	key := fmt.Sprintf("%s_%d", itemType, itemID)
	if entry, ok := inv[key]; ok {
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
