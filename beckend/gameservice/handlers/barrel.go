//================================
// gameservice/handlers/barrel.go
//================================
package handlers

import (
    "encoding/json"
    "fmt"
    "log"

    "gameservice/game"
    
    "gameservice/models"
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

func broadcastCellRemoval(instanceID string, cell game.FullCell) error {
    cell.Barbel = nil
    cell.TileCode = 48
    // сохраняем в БД
    cells, err := Barrel.LoadMap(instanceID)
    if err != nil {
        return err
    }
    for i := range cells {
        if cells[i].CellID == cell.CellID {
            cells[i].Barbel = nil
            cells[i].TileCode = 48
        }
    }
    if err := Barrel.SaveMap(instanceID, cells); err != nil {
        return err
    }
    // финальный WS-апдейт
    final := serialiseUpdatedCell(cell)
   upd := map[string]interface{}{
        "type": "UPDATE_CELL",
        "payload": map[string]interface{}{
            "instanceId": instanceID,
            "updatedCell": final,
        },
    }
    b, _ := json.Marshal(upd)
    Broadcast(b)
    return nil
}

func HandleOpenBarrel(
    cell game.FullCell,
    instanceID string,
    userID int,
    resources, artifacts []game.ResourceData,
) (game.FullCell, *models.PlayerResponse, bool, error) {
   
    // 1) Загрузить игрока
    player, err := Barrel.GetPlayer(instanceID, userID)
    if err != nil {
        return cell, nil, false, fmt.Errorf("fetch player: %w", err)
    }

    // 2) OpenBarbel → outcome
    outcome, err := game.OpenBarbel(cell, resources, artifacts)
    if err != nil {
        return cell, nil, false, fmt.Errorf("open barrel: %w", err)
    }

    // 3) Обработка результата
    switch v := outcome.(type) {

    case game.DamageEvent:
        // а) вычитаем здоровье
        player.Health -= v.Amount
        if player.Health < 0 {
            player.Health = 0
        }
        if err := Barrel.UpdatePlayer(player); err != nil {
            return cell, nil, false, fmt.Errorf("update player HP: %w", err)
        }

        // б) сообщаем о уроне всем
        dmgMsg := map[string]interface{}{
            "type": "BARREL_DAMAGE",
            "payload": map[string]interface{}{
                "instanceId": instanceID,
                "userId": userID,
                "amount": v.Amount,
                "hp":     player.Health,
            },
        }
        b, _ := json.Marshal(dmgMsg)
        Broadcast(b)
        log.Printf("[broadcastCellRemoval] → %s", string(b))

        // в) если игрок умер
        if player.Health == 0 {
            log.Printf("[handleBarrel] player %d died from barrel", player.UserID)
            handlePlayerDeath(instanceID, player)
            // передаём ход дальше
            if ms, ok := game.GetMatchState(instanceID); ok && len(ms.TurnOrder) > 0 {
                nextUser := ms.TurnOrder[0]
                ms.ActiveUserID = nextUser

                nextPlayer, err := Barrel.GetPlayer(instanceID, nextUser)
                if err != nil {
                    log.Printf("GetMatchPlayerByID вернула ошибку: %v", err)
                }

                passMsg := map[string]interface{}{
                    "type": "TURN_PASSED",
                    "payload": map[string]interface{}{
                         "instanceId": instanceID,
                        "active_user": nextUser,
                        "turnNumber":  ms.TurnNumber,
                        "energy":      nextPlayer.Energy,
                    },
                }
                pm, _ := json.Marshal(passMsg)
                Broadcast(pm)
            }

            // очистка и рассылка UPDATE_CELL
            if err := broadcastCellRemoval(instanceID, cell); err != nil {
                return cell, player, true, fmt.Errorf("cleanup cell: %w", err)
            }

            return cell, player, true, nil
        }

        // если игрок жив — просто удаляем бочку и уходим
        if err := broadcastCellRemoval(instanceID, cell); err != nil {
            return cell, player, false, fmt.Errorf("cleanup cell: %w", err)
        }
        return cell, player, false, nil

    case game.ResourceData:
         itemType := "resource"
        if isArtifact(v, artifacts) {
            itemType = "artifact"
    }
     fmt.Printf("DEBUG: AddInventoryItem for ID=%d Name=%q → itemType=%q\n", v.ID, v.Type, itemType)
      
		// а) кладём в инвентарь
       if err := Barrel.AddItem(
            instanceID,  // string
            userID,      // int
            itemType,    // "resource" или "artifact"
            v.ID,        // int — item_id
            v.Type,      // string — item_name (для ресурсов и артефактов)
            v.Image,     // string — image_url
            v.Description,// string — item_description
            1,              // count
       ); err != nil {
           return cell, nil, false, fmt.Errorf("add inventory: %w", err)
        
       }
        // б) перезагружаем игрока
        updatedPlayer, err := Barrel.GetPlayer(instanceID, userID)
        if err != nil {
            return cell, nil, false, fmt.Errorf("reload player: %w", err)
       }
        // в) WS: сначала общий апдейт инвентаря
        invUpd := map[string]interface{}{
            "type": "UPDATE_INVENTORY",
            "payload": map[string]interface{}{
                "instanceId": instanceID,
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
                "instanceId": instanceID,
                "updatedCell":   cellJSON,
                "updatedPlayer": updatedPlayer,
            },
        }
        rb, _ := json.Marshal(resMsg)
        Broadcast(rb)

        if err := broadcastCellRemoval(instanceID, cell); err != nil {
            return cell, updatedPlayer, false, fmt.Errorf("cleanup cell: %w", err)
        }
        return cell, updatedPlayer, false, nil

    default:
        return cell, nil, false, fmt.Errorf("unexpected outcome: %T", v)
    }
}
