// ==============================
// gameservice/handlers/turn.go
// ==============================
package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"gameservice/game"
	"gameservice/middleware"
	"gameservice/repository"
	"log"
	"net/http"
	"sync"
	"time"
)

// EndTurnRequest представляет запрос на завершение хода.
// Теперь содержит instance_id для идентификации матча.
type EndTurnRequest struct {
	UserID     int    `json:"user_id"`
	InstanceID string `json:"instance_id"`
}

// EndTurnResponse возвращает статус матча, включая активного игрока, энергию и номер хода.
type EndTurnResponse struct {
	ActiveUser int `json:"active_user"`
	Energy     int `json:"energy"`
	TurnNumber int `json:"turn_number"`
}

// Константа пополнения энергии при получении хода
const energyRegen = 10

func progressConstructionByTurn(instanceID string) error {
	cells, err := repository.LoadMapCells(instanceID)
	if err != nil {
		return err
	}

	changed := make([]UpdatedCellResponse, 0)
	for i := range cells {
		if !cells[i].IsUnderConstruction {
			continue
		}
		if cells[i].ConstructionTurnsLeft > 0 {
			cells[i].ConstructionTurnsLeft--
		}
		if cells[i].ConstructionTurnsLeft <= 0 {
	cells[i].ConstructionTurnsLeft = 0
	cells[i].IsUnderConstruction = false

	ownerUserID := cells[i].StructureOwnerUserID
	structureType := cells[i].StructureType

	if ownerUserID > 0 && structureType != "" {
		if err := repository.IncrementStructureCount(instanceID, ownerUserID, structureType); err != nil {
			log.Printf("progressConstructionByTurn: IncrementStructureCount failed: instance=%s user=%d type=%s err=%v",
				instanceID, ownerUserID, structureType, err)
		}

		if structureType == "scout_tower" {
			if err := repository.ApplyScoutTowerBonusIfNeeded(instanceID, ownerUserID); err != nil {
				log.Printf("progressConstructionByTurn: ApplyScoutTowerBonusIfNeeded failed: instance=%s user=%d err=%v",
					instanceID, ownerUserID, err)
			}
		}
	}
}
		changed = append(changed, serialiseUpdatedCell(cells[i]))
	}

	if len(changed) == 0 {
		return nil
	}

	if err := repository.SaveMapCells(instanceID, cells); err != nil {
		return err
	}

	for _, updatedCell := range changed {
		msg := map[string]interface{}{
			"type": "UPDATE_CELL",
			"payload": map[string]interface{}{
				"instanceId":  instanceID,
				"updatedCell": updatedCell,
			},
		}
		buf, _ := json.Marshal(msg)
		Broadcast(buf)
	}

	return nil
}


func applyTurretDamage(instanceID string, ownerUserID int, targetType string, targetID int, turretAttack int, targetDefense int, targetHealth int) {
	attackerStats := stats{
		Attack: turretAttack,
	}

	targetStats := stats{
		Defense: targetDefense,
		Health:  targetHealth,
	}

	targetRes := applyDamage(attackerStats, targetStats)

	// Засчитываем урон владельцу турели
	if ms, ok := game.GetMatchState(instanceID); ok {
		ms.RecordDamageEvent(ownerUserID, targetType, targetRes.Damage)
	}

	saveTargetHealth(
		instanceID,
		targetType,
		targetID,
		ownerUserID,
		"player",
		targetRes,
	)

	if targetType == "player" && targetRes.NewHealth > 0 {
		sendUpdatePlayerWS(instanceID, targetID)
	}
}

// progressStructuresEffectsByTurn — обработка эффектов построек (турель)
func progressStructuresEffectsByTurn(instanceID string) error {
	cells, err := repository.LoadMapCells(instanceID)
	if err != nil {
		return err
	}
	monsters, err := repository.LoadMatchMonsters(instanceID)
	if err != nil {
		return err
	}
	players, err := repository.LoadMatchPlayers(instanceID)
	if err != nil {
		return err
	}

	var turretRadius = 2
	var turretDamage = 15
	var turretMaxEnergy = 8
	var turretAttackCost = 6

	// Track which cells we modified (structure fields) so we can merge
	// changes into the latest map snapshot before saving — this avoids
	// overwriting monster deletions performed by saveTargetHealth.
	modified := map[string]bool{}
	for i := range cells {
		cell := &cells[i]
		if cell.StructureType != "turret" || cell.IsUnderConstruction {
			continue
		}
		if cell.StructureHealth <= 0 {
			continue
		}
		if cell.StructureOwnerUserID == 0 {
			continue
		}

		if cell.StructureAttack == 0 {
			cell.StructureAttack = turretDamage
			modified[fmt.Sprintf("%d:%d", cell.X, cell.Y)] = true
		}
		if cell.StructureDefense == 0 {
			cell.StructureDefense = 5
			modified[fmt.Sprintf("%d:%d", cell.X, cell.Y)] = true
		}

		// текущая логика: каждый ход турель полностью заряжается
		if cell.StructureEnergy < turretMaxEnergy {
			cell.StructureEnergy = turretMaxEnergy
			modified[fmt.Sprintf("%d:%d", cell.X, cell.Y)] = true
		}

		ownerGroup := 0
		for _, p := range players {
			if p.UserID == cell.StructureOwnerUserID {
				ownerGroup = p.GroupID
				break
			}
		}

		attacked := false

		// 1) Приоритет — вражеский игрок
		for _, p := range players {
			if p.Health <= 0 {
				continue
			}
			if p.UserID == cell.StructureOwnerUserID {
				continue
			}
			if p.GroupID == ownerGroup {
				continue
			}

			dx := p.Position.X - cell.X
			dy := p.Position.Y - cell.Y
			dist := dx*dx + dy*dy
			if dist > turretRadius*turretRadius {
				continue
			}
			if cell.StructureEnergy < turretAttackCost {
				break
			}

				cell.StructureEnergy -= turretAttackCost
				modified[fmt.Sprintf("%d:%d", cell.X, cell.Y)] = true
				applyTurretDamage(
				instanceID,
				cell.StructureOwnerUserID,
				"player",
				p.UserID,
				cell.StructureAttack,
				p.Defense,
				p.Health,
			)
			attacked = true
			break
		}

		// 2) Если не атаковали игрока — атакуем монстра
		if !attacked {
			for _, m := range monsters {
				if m.Health <= 0 {
					continue
				}

				dx := m.X - cell.X
				dy := m.Y - cell.Y
				dist := dx*dx + dy*dy
				if dist > turretRadius*turretRadius {
					continue
				}
				if cell.StructureEnergy < turretAttackCost {
					break
				}

				cell.StructureEnergy -= turretAttackCost
				modified[fmt.Sprintf("%d:%d", cell.X, cell.Y)] = true
				applyTurretDamage(
					instanceID,
					cell.StructureOwnerUserID,
					"monster",
					m.MonsterInstanceID,
					cell.StructureAttack,
					m.Defense,
					m.Health,
					)
				break
			}
		}
	}

	// Merge only structure-related fields into the latest map snapshot
	// so that deletions (e.g., dead monsters) saved by other routines are
	// not overwritten by this function's original snapshot.
	if len(modified) > 0 {
		latest, err := repository.LoadMapCells(instanceID)
		if err != nil {
			return err
		}

		// Build lookup for our modified cells
		src := map[string]game.FullCell{}
		for i := range cells {
			key := fmt.Sprintf("%d:%d", cells[i].X, cells[i].Y)
			if modified[key] {
				src[key] = cells[i]
			}
		}

		// Apply structure fields from src into latest
		changed := make([]game.FullCell, 0)
		for i := range latest {
			key := fmt.Sprintf("%d:%d", latest[i].X, latest[i].Y)
			if s, ok := src[key]; ok {
				latest[i].StructureType = s.StructureType
				latest[i].StructureOwnerUserID = s.StructureOwnerUserID
				latest[i].StructureHealth = s.StructureHealth
				latest[i].StructureDefense = s.StructureDefense
				latest[i].StructureAttack = s.StructureAttack
				latest[i].StructureEnergy = s.StructureEnergy
				latest[i].IsUnderConstruction = s.IsUnderConstruction
				latest[i].ConstructionTurnsLeft = s.ConstructionTurnsLeft
				changed = append(changed, latest[i])
			}
		}

		if err := repository.SaveMapCells(instanceID, latest); err != nil {
			return err
		}

		// Broadcast updated cells so clients refresh structure stats
		for _, c := range changed {
			upd := map[string]interface{}{
				"type": "UPDATE_CELL",
				"payload": map[string]interface{}{
					"instanceId":  instanceID,
					"updatedCell": serialiseUpdatedCell(c),
				},
			}
			b, _ := json.Marshal(upd)
			Broadcast(b)
		}
	}

	return nil
}

// Можно оставить energyRegen = 10 как дефолтное значение, а в функцию передавать явно нужное значение.
func regenEnergyForNextPlayer(instanceID string, userID int, regenEnergy int) error {
    if regenEnergy <= 0 {
        regenEnergy = 10 // дефолт, если не передан или <=0
    }
    nextPlayer, err := repository.GetMatchPlayerByID(instanceID, userID)
    if err != nil {
        return err
    }
    newEnergy := nextPlayer.Energy + regenEnergy
    if newEnergy > nextPlayer.MaxEnergy {
        newEnergy = nextPlayer.MaxEnergy
    }
    nextPlayer.Energy = newEnergy

    if err = repository.UpdateMatchPlayer(instanceID, nextPlayer); err != nil {
        return err
    }

    // WebSocket сообщение для клиента об обновлении энергии/игрока
    updatePlayerMsg := map[string]interface{}{
        "type": "UPDATE_PLAYER",
        "payload": map[string]interface{}{
            "instanceId": instanceID,
            "player":     nextPlayer,
        },
    }
    buf, _ := json.Marshal(updatePlayerMsg)
    Broadcast(buf)

    return nil
}


// turnTimerLimit is the per-turn time limit.
const turnTimerLimit = 60 * time.Second

// turnTimers holds per-instance cancel funcs for active turn timers.
var turnTimers sync.Map // key: instanceID → context.CancelFunc

// CancelTurnTimer stops the running turn timer for an instance.
func CancelTurnTimer(instanceID string) {
	if v, loaded := turnTimers.LoadAndDelete(instanceID); loaded {
		v.(context.CancelFunc)()
	}
}

// startTurnTimer schedules an auto-endTurn for instanceID/userID after turnTimerLimit.
func startTurnTimer(instanceID string, userID int) {
	CancelTurnTimer(instanceID)
	ctx, cancel := context.WithCancel(context.Background())
	turnTimers.Store(instanceID, cancel)
	go func() {
		select {
		case <-ctx.Done():
			return
		case <-time.After(turnTimerLimit):
		}
		ms, ok := game.GetMatchState(instanceID)
		if !ok || ms.ActiveUserID != userID {
			return
		}
		log.Printf("[turnTimer] auto-ending turn for user %d in match %s", userID, instanceID)
		doEndTurn(instanceID, userID, ms)
	}()
}

// doEndTurn executes the turn-change logic without an HTTP context (used by both
// the timer goroutine and EndTurnHandler to avoid duplication).
func doEndTurn(instanceID string, userID int, ms *game.MatchState) {
	if err := progressConstructionByTurn(instanceID); err != nil {
		log.Printf("[doEndTurn] progressConstructionByTurn error: %v", err)
	}
	// Эффекты построек
	if err := progressStructuresEffectsByTurn(instanceID); err != nil {
		log.Printf("[doEndTurn] progressStructuresEffectsByTurn error: %v", err)
	}

	nextUserID, err := ms.EndTurn(userID)
	if err != nil {
		log.Printf("[doEndTurn] EndTurn error for user %d: %v", userID, err)
		return
	}
	if err := repository.UpdateMatchTurn(instanceID, nextUserID, ms.TurnNumber); err != nil {
		log.Printf("[doEndTurn] UpdateMatchTurn error: %v", err)
		return
	}
	if err := regenEnergyForNextPlayer(instanceID, nextUserID, energyRegen); err != nil {
		log.Printf("[doEndTurn] regenEnergy error: %v", err)
		return
	}
	nextUser, err := repository.GetMatchPlayerByID(instanceID, nextUserID)
	if err != nil {
		log.Printf("[doEndTurn] GetMatchPlayerByID error: %v", err)
		return
	}
	updateMsg := map[string]interface{}{
		"type": "SET_ACTIVE_USER",
		"payload": map[string]interface{}{
			"instanceId":  instanceID,
			"active_user": nextUserID,
			"energy":      nextUser.Energy,
			"turnNumber":  ms.TurnNumber,
		},
	}
	updateJSON, _ := json.Marshal(updateMsg)
	Broadcast(updateJSON)
	startTurnTimer(instanceID, nextUserID)
}

// EndTurnHandler теперь полностью выполняет логику смены хода на сервере
func EndTurnHandler(w http.ResponseWriter, r *http.Request) {
	var req EndTurnRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		log.Printf("Ошибка декодирования запроса завершения хода: %v", err)
		http.Error(w, "Bad request", http.StatusBadRequest)
		return
	}

	// Проверяем, что игрок завершает свой ход
	tokenUserID, ok := middleware.GetUserIDFromContext(r.Context())
	if !ok || tokenUserID != req.UserID {
		http.Error(w, "Запрещено завершать ход другому игроку", http.StatusForbidden)
		return
	}

	// Получаем состояние матча
	matchState, ok := game.GetMatchState(req.InstanceID)
	if !ok {
		log.Printf("Матч с instance_id %s не найден", req.InstanceID)
		http.Error(w, "Match not found", http.StatusNotFound)
		return
	}

	// Завершаем ход (логика выбора следующего игрока находится в EndTurn)
	if err := progressConstructionByTurn(req.InstanceID); err != nil {
		log.Printf("Ошибка прогресса строительства: %v", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}
	if err := progressStructuresEffectsByTurn(req.InstanceID); err != nil {
	log.Printf("Ошибка эффектов построек: %v", err)
	http.Error(w, "Internal server error", http.StatusInternalServerError)
	return
}

	nextUserID, err := matchState.EndTurn(req.UserID)
	if err != nil {
		log.Printf("Ошибка завершения хода: %v", err)
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	// Обновляем данные матча в БД
	if err := repository.UpdateMatchTurn(req.InstanceID, nextUserID, matchState.TurnNumber); err != nil {
		log.Printf("Ошибка обновления матча в БД: %v", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	// Регенерируем энергию для следующего игрока через общую функцию
	if err := regenEnergyForNextPlayer(req.InstanceID, nextUserID, energyRegen); err != nil {
		log.Printf("Ошибка начисления энергии следующему игроку: %v", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	// Получаем свежие данные нового активного игрока для ответа
	nextUser, err := repository.GetMatchPlayerByID(req.InstanceID, nextUserID)
	if err != nil {
		log.Printf("Ошибка получения данных нового активного игрока: %v", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	// Формируем сообщение для WebSocket с полным новым состоянием матча
	updateMsg := map[string]interface{}{
		"type": "SET_ACTIVE_USER",
		"payload": map[string]interface{}{
			"instanceId":  req.InstanceID,
			"active_user": nextUserID,
			"energy":      nextUser.Energy,
			"turnNumber":  matchState.TurnNumber,
		},
	}
	updateJSON, _ := json.Marshal(updateMsg)
	Broadcast(updateJSON)

	// Запускаем/сбрасываем таймер хода для нового активного игрока
	startTurnTimer(req.InstanceID, nextUserID)

	// Отправляем ответ клиенту
	response := EndTurnResponse{
		ActiveUser: nextUserID,
		Energy:     nextUser.Energy,
		TurnNumber: matchState.TurnNumber,
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}
