// ================================
// gameservice/handlers/barrel.go
// ================================
package handlers

import (
	"encoding/json"
	"fmt"
	"log"

	"gameservice/game"
	"gameservice/repository"
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



func updateCellInMap(instanceID string, cell game.FullCell) {
    cells, err := repository.LoadMapCells(instanceID)
    if err == nil {
        for i := range cells {
            if cells[i].X == cell.X && cells[i].Y == cell.Y {
                cells[i] = cell
                break
            }
        }
        _ = repository.SaveMapCells(instanceID, cells)
    }
}


// HandleOpenBarrel вызывается по WS/HTTP, когда игрок открывает бочку.
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

	// 2) Load quest artifact state for this match to control drop rules.
	matchInfo, err := repository.GetMatchByID(instanceID)
	if err != nil {
		return cell, nil, false, fmt.Errorf("get match: %w", err)
	}

	questDropped := false
	remainingBarrels := 0
	if matchInfo.QuestArtifactID > 0 {
		questDropped, err = repository.MatchHasQuestArtifact(instanceID, matchInfo.QuestArtifactID)
		if err != nil {
			return cell, nil, false, fmt.Errorf("check quest artifact dropped: %w", err)
		}

		cells, err := repository.LoadMapCells(instanceID)
		if err != nil {
			return cell, nil, false, fmt.Errorf("load map cells: %w", err)
		}
		for _, c := range cells {
			if c.Barbel != nil {
				remainingBarrels++
			}
		}
	}

	filteredArtifacts := artifacts
	if matchInfo.QuestArtifactID > 0 {
		filteredArtifacts = make([]game.ResourceData, 0, len(artifacts))
		for _, a := range artifacts {
			if questDropped && a.ID == matchInfo.QuestArtifactID {
				continue
			}
			filteredArtifacts = append(filteredArtifacts, a)
		}
	}

	var outcome interface{}
	// If this is the last barrel and quest artifact has not dropped yet,
	// force quest artifact drop to keep match completable.
	if matchInfo.QuestArtifactID > 0 && !questDropped && remainingBarrels == 1 {
		forced := false
		for _, a := range artifacts {
			if a.ID == matchInfo.QuestArtifactID {
				outcome = a
				forced = true
				break
			}
		}
		if !forced {
			qa, err := repository.GetArtifactFromCatalogByID(matchInfo.QuestArtifactID)
			if err != nil {
				return cell, nil, false, fmt.Errorf("load forced quest artifact: %w", err)
			}
			outcome = game.ResourceData{
				ID:          qa.ID,
				Type:        qa.Name,
				Description: qa.Description,
				Effect:      map[string]int{},
				Image:       qa.Image,
			}
		}
	} else {
		// 3) OpenBarbel → outcome
		outcome, err = game.OpenBarbel(cell, resources, filteredArtifacts)
	}
	if err != nil {
		return cell, nil, false, fmt.Errorf("open barrel: %w", err)
	}

	// 4) Обработка результата
	switch v := outcome.(type) {

	case game.DamageEvent:
		// а) вычитаем здоровье
		player.Health -= v.Amount
		if player.Health < 0 {
			player.Health = 0
		}
		if err := Barrel.UpdatePlayer(instanceID, player); err != nil {
			return cell, nil, false, fmt.Errorf("update player HP: %w", err)
		}

		// б) всегда удаляем бочку сразу!
		cell.Barbel = nil
		cell.TileCode = 48
		updateCellInMap(instanceID, cell)

		// в) Отправляем BARREL_DAMAGE всем
		dmgMsg := map[string]interface{}{
			"type": "BARREL_DAMAGE",
			"payload": map[string]interface{}{
				"instanceId": instanceID,
				"userId":     userID,
				"amount":     v.Amount,
				"hp":         player.Health,
			},
		}
		b, _ := json.Marshal(dmgMsg)
		Broadcast(b)
		log.Printf("[BARREL_DAMAGE] → %s", string(b))

		// г) Отправляем ОБНОВЛЁННУЮ клетку (уже без бочки!)
		cellJSON := serialiseUpdatedCell(cell)
		upd := map[string]interface{}{
			"type": "UPDATE_CELL",
			"payload": map[string]interface{}{
				"instanceId":  instanceID,
				"updatedCell": cellJSON,
			},
		}
		updBytes, _ := json.Marshal(upd)
		Broadcast(updBytes)

		// д) Если игрок умер — передать ход и т.д. (как у тебя)
		if player.Health == 0 {
			log.Printf("[handleBarrel] player %d died from barrel", player.UserID)
			// Barrel kill: no player killer → killerID=0, killerIsPlayer=false
			handlePlayerDeath(instanceID, player, 0, false)
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
						"userId":     nextUser,
						"turnNumber": ms.TurnNumber,
						"energy":     nextPlayer.Energy,
					},
				}
				pm, _ := json.Marshal(passMsg)
				Broadcast(pm)
			}
		}

		return cell, player, player.Health == 0, nil

	case game.ResourceData:
		itemType := "resource"
		if isArtifact(v, artifacts) {
			itemType = "artifact"
		}
		fmt.Printf("DEBUG: AddInventoryItem for ID=%d Name=%q → itemType=%q\n", v.ID, v.Type, itemType)

		// а) кладём в инвентарь
		if err := Barrel.AddItem(
			instanceID,    // string
			userID,        // int
			itemType,      // "resource" или "artifact"
			v.ID,          // int — item_id
			v.Type,        // string — item_name (для ресурсов и артефактов)
			v.Image,       // string — image_url
			v.Description, // string — item_description
			1,             // count
		); err != nil {
			return cell, nil, false, fmt.Errorf("add inventory: %w", err)
		}

		// б) перезагружаем игрока
		updatedPlayer, err := Barrel.GetPlayer(instanceID, userID)
		if err != nil {
			return cell, nil, false, fmt.Errorf("reload player: %w", err)
		}

		// в) WS: общий апдейт инвентаря
		invUpd := map[string]interface{}{
			"type": "UPDATE_INVENTORY",
			"payload": map[string]interface{}{
				"instanceId": instanceID,
				"userId":     userID,
				"inventory":  updatedPlayer.Inventory,
			},
		}
		ib, _ := json.Marshal(invUpd)
		Broadcast(ib)

		// г) ОЧИЩАЕМ БОЧКУ И ФОРМИРУЕМ СОБЫТИЕ!
		cell.Barbel = nil
		cell.TileCode = 48
		updateCellInMap(instanceID, cell)
		
		cellJSON := serialiseUpdatedCell(cell)
		evtType := "BARREL_RESOURCE"
		if isArtifact(v, artifacts) {
			evtType = "BARREL_ARTIFACT"
		}
		resMsg := map[string]interface{}{
			"type": evtType,
			"payload": map[string]interface{}{
				"instanceId":    instanceID,
				"updatedCell":   cellJSON,
				"updatedPlayer": updatedPlayer,
			},
		}
		rb, _ := json.Marshal(resMsg)
		Broadcast(rb)

		// Если из бочки выпал именно квестовый артефакт — уведомляем всех игроков.
		if matchInfo.QuestArtifactID > 0 && v.ID == matchInfo.QuestArtifactID {
			questMsg := map[string]interface{}{
				"type": "QUEST_ARTIFACT_FOUND",
				"payload": map[string]interface{}{
					"instanceId": instanceID,
					"playerName": updatedPlayer.Name,
					"x":          cell.X,
					"y":          cell.Y,
				},
			}
			qb, _ := json.Marshal(questMsg)
			Broadcast(qb)
		}

		return cell, updatedPlayer, false, nil

	default:
		return cell, nil, false, fmt.Errorf("unexpected outcome: %T", v)
	}
}
