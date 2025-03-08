
// ====================================
// /gameservice/repository/inventory.go
// ====================================

package repository

import (
    "database/sql"
)

func AddInventoryItem(playerID int, itemType string, itemID int, count int) error {
    var existingCount int
    err := DB.QueryRow(
        `SELECT count FROM inventory_items 
         WHERE player_id = $1 AND item_type = $2 AND item_id = $3`,
        playerID, itemType, itemID,
    ).Scan(&existingCount)

    if err != nil {
        if err == sql.ErrNoRows {
            // Если нет строки – вставляем новую
            _, err = DB.Exec(
                `INSERT INTO inventory_items (player_id, item_type, item_id, count)
                 VALUES ($1, $2, $3, $4)`,
                playerID, itemType, itemID, count,
            )
            return err
        }
        return err
    }

    // Если строка есть – обновляем
    newCount := existingCount + count
    _, err = DB.Exec(
        `UPDATE inventory_items SET count = $1 
         WHERE player_id = $2 AND item_type = $3 AND item_id = $4`,
        newCount, playerID, itemType, itemID,
    )
    return err
}

func RemoveInventoryItem(playerID int, itemType string, itemID int, count int) error {
    var existingCount int
    err := DB.QueryRow(
        `SELECT count FROM inventory_items 
         WHERE player_id = $1 AND item_type = $2 AND item_id = $3`,
        playerID, itemType, itemID,
    ).Scan(&existingCount)
    if err != nil {
        return err
    }
    newCount := existingCount - count
    if newCount <= 0 {
        // Удаляем запись, если количество <= 0
        _, err = DB.Exec(
            `DELETE FROM inventory_items 
             WHERE player_id = $1 AND item_type = $2 AND item_id = $3`,
            playerID, itemType, itemID,
        )
        return err
    }
    // Иначе обновляем
    _, err = DB.Exec(
        `UPDATE inventory_items SET count = $1 
         WHERE player_id = $2 AND item_type = $3 AND item_id = $4`,
        newCount, playerID, itemType, itemID,
    )
    return err
}
