//================================
// gameservice/handlers/barrel.go
//================================
package handlers

import (
    "encoding/json"
    "fmt"

    "gameservice/game"
    "gameservice/repository"
)

// вверху файла

func isArtifact(res game.ResourceData, artifacts []game.ResourceData) bool {
    for _, a := range artifacts {
        if a.Type == res.Type {
            return true
        }
    }
    return false
}

func serialiseUpdatedCell(cell game.FullCell) UpdatedCellResponse {
    return UpdatedCellResponse{
        CellID:   cell.CellID,
        X:        cell.X,
        Y:        cell.Y,
        TileCode: cell.TileCode,
        Resource: cell.Resource,
        Barbel:   cell.Barbel,
        Monster:  cell.Monster,
        IsPortal: cell.IsPortal,
        IsPlayer: cell.IsPlayer,
    }
}


// HandleOpenBarrel вызывается по WS/HTTP, когда игрок открывает бочку.
func HandleOpenBarrel(
    cell game.FullCell,
    instanceID string,
    userID int,
    resources, artifacts []game.ResourceData,
) error {
    // 1) Загрузить игрока
    player, err := repository.GetMatchPlayerByID(instanceID, userID)
    if err != nil {
        return fmt.Errorf("fetch player: %w", err)
    }

    // 2) Получить outcome
    outcome, err := game.OpenBarbel(cell, resources, artifacts)
    if err != nil {
        return fmt.Errorf("open barrel: %w", err)
    }

    // 3) Обработать outcome
    switch v := outcome.(type) {
    case game.DamageEvent:
        // а) урон
        player.Health -= v.Amount
        if player.Health < 0 {
            player.Health = 0
        }
        if err := repository.UpdateMatchPlayer(player); err != nil {
            return fmt.Errorf("update player HP: %w", err)
        }
        // б) WS-сообщение
        msg := map[string]interface{}{
            "type": "BARREL_DAMAGE",
            "payload": map[string]interface{}{
                "userId": userID,
                "amount": v.Amount,
                "hp":     player.Health,
            },
        }
        b, _ := json.Marshal(msg)
        Broadcast(b)

    case game.ResourceData:
         itemType := "resource"
        if isArtifact(v, artifacts) {
            itemType = "artifact"
    }
     fmt.Printf("DEBUG: AddInventoryItem for ID=%d Name=%q → itemType=%q\n", v.ID, v.Type, itemType)
      
		// а) кладём в инвентарь
       if err := repository.AddInventoryItem(
            instanceID,  // string
            userID,      // int
            itemType,    // "resource" или "artifact"
            v.ID,        // int — item_id
            v.Type,      // string — item_name (для ресурсов и артефактов)
            v.Image,     // string — image_url
            v.Description,// string — item_description
            1,              // count
       ); err != nil {
           return fmt.Errorf("add inventory: %w", err)
       }
        // б) перезагружаем игрока
        updatedPlayer, err := repository.GetMatchPlayerByID(instanceID, userID)
        if err != nil {
            return fmt.Errorf("reload player: %w", err)
        }
        // в) WS: сначала общий апдейт инвентаря
        invUpd := map[string]interface{}{
            "type": "UPDATE_INVENTORY",
            "payload": map[string]interface{}{
                "userId":    userID,
                "inventory": updatedPlayer.Inventory,
            },
        }
        ib, _ := json.Marshal(invUpd)
        Broadcast(ib)
        // затем BARREL_RESOURCE или BARREL_ARTIFACT
        evtType := "BARREL_RESOURCE"
        if isArtifact(v, artifacts) {
            evtType = "BARREL_ARTIFACT"
        }
        cell.Barbel = nil // убираем бочку из отправляемой клетки
        cell.TileCode = 48
        cellJSON := serialiseUpdatedCell(cell)
        resMsg := map[string]interface{}{
            "type": evtType,
            "payload": map[string]interface{}{
                "updatedCell":   cellJSON,
                "updatedPlayer": updatedPlayer,
            },
        }
        rb, _ := json.Marshal(resMsg)
        Broadcast(rb)

    default:
        return fmt.Errorf("unexpected outcome: %T", v)
    }

    // 4) Удаляем бочку из БД/карты
    cells, err := repository.LoadMapCells(instanceID)
    if err != nil {
        return fmt.Errorf("load map: %w", err)
    }
    for i := range cells {
        if cells[i].CellID == cell.CellID {
            cells[i].Barbel = nil
            cells[i].TileCode = 48
            break
        }
    }
    if err := repository.SaveMapCells(instanceID, cells); err != nil {
        return fmt.Errorf("save map: %w", err)
    }

    // 5) Финальный WS-апдейт клетки
    final := serialiseUpdatedCell(cell)
    upd := map[string]interface{}{
        "type":    "UPDATE_CELL",
        "payload": final,
    }
    fb, _ := json.Marshal(upd)
    Broadcast(fb)

    return nil
}
