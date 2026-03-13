// ====================================
// gameservice/handlers/combat.go
// ====================================

package handlers

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strconv"

	"gameservice/game"
	"gameservice/middleware"
	"gameservice/models"
	"gameservice/repository"
	"github.com/gorilla/mux"
)

// Стоимость перемещения: 1 единица энергии.
const moveEnergyCost = 1
const attackEnergyCost = 4
const counterAttackEnergyCost = 2

// CombatExchangePayload — полезная нагрузка для WS-события боевого обмена
type CombatExchangePayload struct {
	Attacker struct {
		ID     int `json:"id"`
		NewHP  int `json:"new_hp"`
		Damage int `json:"damage"`
	} `json:"attacker"`
	Target struct {
		ID     int `json:"id"`
		NewHP  int `json:"new_hp"`
		Damage int `json:"damage"`
	} `json:"target"`
	InstanceID string `json:"instanceId"`
}

// CombatExchangeMessage — сообщение WS-события боевого обмена
type CombatExchangeMessage struct {
	Type    string                `json:"type"`
	Payload CombatExchangePayload `json:"payload"`
}

type AttackRequest struct {
	AttackerType string `json:"attacker_type"`
	AttackerID   int    `json:"attacker_id"`
	TargetType   string `json:"target_type"`
	TargetID     int    `json:"target_id"`
	InstanceID   string `json:"instance_id"`
}

// cellPassable возвращает, можно ли ходить по тайлу с данным кодом
func cellPassable(code int) bool {
	for _, c := range []int{48, 80, 82, 112, 66} {
		if code == c {
			return true
		}
	}
	return false
}

// nopCloser нужно, чтобы bytes.Buffer имплементировал io.ReadCloser
type nopCloser struct{ *bytes.Buffer }

func (nopCloser) Close() error { return nil }

// rWithBody создаёт http.Request с JSON-телом для внутреннего повторного вызова
func rWithBody(body interface{}) *http.Request {
	buf := new(bytes.Buffer)
	json.NewEncoder(buf).Encode(body)
	return &http.Request{Body: nopCloser{buf}}
}

// MoveOrAttackHandler объединяет ход и атаку в один эндпоинт.
func MoveOrAttackHandler(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	instanceID := vars["instance_id"]

	// 1) Аутентификация
	userID, err := strconv.Atoi(vars["id"])
	if err != nil {
		http.Error(w, "Некорректный ID игрока", http.StatusBadRequest)
		return
	}
	tokenUserID, ok := middleware.GetUserIDFromContext(r.Context())
	if !ok || tokenUserID != userID {
		http.Error(w, "Запрещено действовать от лица другого игрока", http.StatusForbidden)
		return
	}

	// 2) Парсим тело — только новые координаты
	var req struct {
		NewPosX int `json:"new_pos_x"`
		NewPosY int `json:"new_pos_y"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Неверный формат запроса", http.StatusBadRequest)
		return
	}

	// 3) Проверяем границы карты
	var mapW, mapH int
	if err := repository.DB.QueryRow(
		`SELECT map_width, map_height FROM matches WHERE instance_id = $1`,
		instanceID,
	).Scan(&mapW, &mapH); err != nil {
		http.Error(w, "Ошибка загрузки размеров карты", http.StatusInternalServerError)
		return
	}
	if req.NewPosX < 0 || req.NewPosX >= mapW || req.NewPosY < 0 || req.NewPosY >= mapH {
		http.Error(w, "Координаты вне карты", http.StatusBadRequest)
		return
	}

	// 4) Пытаемся найти монстра
	mm, err := repository.GetMatchMonsterAt(instanceID, req.NewPosX, req.NewPosY)
	if err != nil {
		http.Error(w, "Ошибка чтения монстров", http.StatusInternalServerError)
		return
	}
	if mm != nil {
		attackReq := AttackRequest{
			AttackerType: "player",
			AttackerID:   userID,
			TargetType:   "monster",
			TargetID:     mm.MonsterInstanceID,
			InstanceID:   instanceID,
		}
		UniversalAttackHandler(w, rWithBody(attackReq))
		return
	}

	// 5) Проверяем игрока на той же клетке
	if cnt, err := repository.CollisionCount(instanceID, req.NewPosX, req.NewPosY, userID); err == nil && cnt > 0 {
		other, err := repository.GetOtherPlayerID(instanceID, req.NewPosX, req.NewPosY, userID)
		if err != nil {
			http.Error(w, "Не удалось найти другого игрока", http.StatusInternalServerError)
			return
		}
		attackReq := AttackRequest{
			AttackerType: "player",
			AttackerID:   userID,
			TargetType:   "player",
			TargetID:     other,
			InstanceID:   instanceID,
		}
		UniversalAttackHandler(w, rWithBody(attackReq))
		return
	}

	// 5.1) Проверяем, что тайл проходимый
	var mapJSON []byte
	if err := repository.DB.QueryRow(
		`SELECT map FROM matches WHERE instance_id = $1`,
		instanceID,
	).Scan(&mapJSON); err != nil {
		http.Error(w, "Ошибка загрузки карты", http.StatusInternalServerError)
		return
	}
	var cells []game.FullCell
	if err := json.Unmarshal(mapJSON, &cells); err != nil {
		http.Error(w, "Ошибка разбора карты", http.StatusInternalServerError)
		return
	}
	var targetCell *game.FullCell
	for i := range cells {
		if cells[i].X == req.NewPosX && cells[i].Y == req.NewPosY {
			targetCell = &cells[i]
			break
		}
	}
	if targetCell == nil {
		http.Error(w, "Клетка не найдена", http.StatusBadRequest)
		return
	}
	if !cellPassable(targetCell.TileCode) {
		http.Error(w, fmt.Sprintf("Непроходимый тайл %d", targetCell.TileCode), http.StatusBadRequest)
		return
	}

	// 6) Обычное перемещение
	player, err := repository.GetMatchPlayerByID(instanceID, userID)
	if err != nil {
		http.Error(w, "Ошибка загрузки игрока", http.StatusInternalServerError)
		return
	}
	if player.Energy < moveEnergyCost {
		http.Error(w, "Недостаточно энергии", http.StatusBadRequest)
		return
	}

	oldPos := player.Position
	player.Energy--
	player.Position.X = req.NewPosX
	player.Position.Y = req.NewPosY

	if err := repository.UpdateMatchPlayer(instanceID, player); err != nil {
		http.Error(w, "Ошибка обновления игрока", http.StatusInternalServerError)
		return
	}
	if err := repository.UpdateCellPlayerFlags(instanceID, oldPos, player.Position); err != nil {
		http.Error(w, "Ошибка обновления карты", http.StatusInternalServerError)
		return
	}

	// WS: MOVE_PLAYER
	moveMsg := map[string]interface{}{
		"type": "MOVE_PLAYER",
		"payload": map[string]interface{}{
			"userId":      userID,
			"newPosition": map[string]int{"x": req.NewPosX, "y": req.NewPosY},
			"instanceId":  instanceID,
		},
	}
	b2, _ := json.Marshal(moveMsg)
	Broadcast(b2)

	// WS: UPDATE_PLAYER (для изменения энергии)
	playerForWS, _ := repository.GetMatchPlayerByID(instanceID, userID)
	updatePlayerMsg := map[string]interface{}{
		"type": "UPDATE_PLAYER",
		"payload": map[string]interface{}{
			"instanceId": instanceID,
			"player":     playerForWS, // тут hp, energy и все статы игрока!
		},
	}
	buf, _ := json.Marshal(updatePlayerMsg)
	Broadcast(buf)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(player)
}

// --- Вспомогательные типы --------------------------------------------------

type stats struct{ Attack, Defense, Health int }

type attackResult struct {
	Damage    int
	NewHealth int
}

// --- Загрузка статов игрока или монстра ------------------------------------

func loadStats(instanceID, entityType string, entityID int) (stats, error) {
	if entityType == "player" {
		p, err := repository.GetMatchPlayerByID(instanceID, entityID)
		if err != nil {
			return stats{}, err
		}
		return stats{p.Attack, p.Defense, p.Health}, nil
	}

	// monster

	mm, err := repository.GetMatchMonsterByID(instanceID, entityID)
	if err != nil {
		return stats{}, err
	}
	return stats{mm.Attack, mm.Defense, mm.Health}, nil
}

// --- Расчёт урона и оставшегося HP ----------------------------------------

func applyDamage(att stats, def stats) attackResult {
	dmg := att.Attack - def.Defense
	if dmg < 0 {
		dmg = 0
	}
	newHP := def.Health - dmg
	if newHP < 0 {
		newHP = 0
	}
	return attackResult{Damage: dmg, NewHealth: newHP}
}

// --- Обновление HP и обработка смерти цели --------------------------------
// saveTargetHealth сохраняет урон и обрабатывает смерть цели.

func saveTargetHealth(
	instanceID string,
	targetType string,
	targetID int,
	attackerID int,
	attackerType string,
	ar attackResult,
) {
	log.Printf("[saveTargetHealth] Called for %s #%d (new hp: %d), attacker: %d", targetType, targetID, ar.NewHealth, attackerID)

	if targetType == "player" {
		p, err := Combat.GetPlayer(instanceID, targetID)
		if err != nil {
			log.Printf("[saveTargetHealth] load player error: %v", err)
			return
		}
		log.Printf("[saveTargetHealth] player %d HP before: %d, after: %d", targetID, p.Health, ar.NewHealth)
		p.Health = ar.NewHealth
		if ar.NewHealth > 0 {
			log.Printf("[saveTargetHealth] player %d survives, updating HP.", targetID)
			Combat.UpdatePlayer(instanceID, p)
		} else {
			log.Printf("[saveTargetHealth] player %d died, calling handlePlayerDeath.", targetID)
			if ms, ok := game.GetMatchState(instanceID); ok {
				ms.RecordKillEvent(attackerID, "player", ar.Damage)
			}
			handlePlayerDeath(instanceID, p, attackerID, attackerType == "player")
			log.Printf("[saveTargetHealth] handlePlayerDeath called for %d", targetID)
		}
	} else {
		// 1. Сохраняем HP в БД
		log.Printf("[saveTargetHealth] monster %d HP before update: new: %d", targetID, ar.NewHealth)
		err := Combat.UpdateMonsterHealth(instanceID, targetID, ar.NewHealth)
		if err != nil {
			log.Printf("[saveTargetHealth] UpdateMonsterHealth error: %v", err)
			return
		}

		// 2. Грузим монстра из БД (HP теперь актуален)
		m, err := Combat.GetMonster(instanceID, targetID)
		if err != nil || m == nil {
			log.Printf("[saveTargetHealth] GetMonster error: %v", err)
			return
		}

		// 3. Грузим карту
		cells, err := Combat.LoadMap(instanceID)
		if err != nil {
			log.Printf("[saveTargetHealth] LoadMap error: %v", err)
			return
		}
		for i := range cells {
			if cells[i].X == m.X && cells[i].Y == m.Y {
				if cells[i].Monster != nil {
					log.Printf("[saveTargetHealth] Update monster HP on cell %d,%d", m.X, m.Y)
					cells[i].Monster.Health = m.Health
				}
				_ = Combat.SaveMap(instanceID, cells)
				update := map[string]interface{}{
					"type": "UPDATE_CELL",
					"payload": map[string]interface{}{
						"instanceId":  instanceID,
						"updatedCell": serialiseUpdatedCell(cells[i]),
					},
				}
				buf, _ := json.Marshal(update)
				Broadcast(buf)
				break
			}
		}

		if ar.NewHealth <= 0 {
			log.Printf("[saveTargetHealth] monster %d died, calling handleMonsterDeath.", targetID)
			if ms, ok := game.GetMatchState(instanceID); ok {
				ms.RecordKillEvent(attackerID, "monster", ar.Damage)
			}
			handleMonsterDeath(instanceID, targetID)
		}
	}
}

// transferQuestArtifact handles quest-artifact fate when a player dies.
// If killed by another player, the artifact is transferred to the killer's inventory.
// If killed by a monster or a barrel trap (killerIsPlayer=false), the artifact is
// dropped on the death cell as a collectible resource.
func transferQuestArtifact(instanceID string, deadPlayerID, deadX, deadY, killerID int, killerIsPlayer bool) {
	matchInfo, err := repository.GetMatchByID(instanceID)
	if err != nil || matchInfo.QuestArtifactID == 0 {
		return
	}
	has, err := repository.PlayerHasQuestArtifact(instanceID, deadPlayerID, matchInfo.QuestArtifactID)
	if err != nil || !has {
		return
	}
	qa, err := repository.GetArtifactFromCatalogByID(matchInfo.QuestArtifactID)
	if err != nil {
		log.Printf("[transferQuestArtifact] GetArtifactFromCatalogByID: %v", err)
		return
	}

	if killerIsPlayer && killerID > 0 {
		// Transfer to killer's inventory
		if err := repository.AddInventoryItem(instanceID, killerID, "artifact", qa.ID, qa.Name, qa.Image, qa.Description, 1); err != nil {
			log.Printf("[transferQuestArtifact] AddInventoryItem to killer %d: %v", killerID, err)
			return
		}
		killerPlayer, err := repository.GetMatchPlayerByID(instanceID, killerID)
		if err == nil {
			invMsg := map[string]interface{}{
				"type": "UPDATE_INVENTORY",
				"payload": map[string]interface{}{
					"instanceId": instanceID,
					"userId":     killerID,
					"inventory":  killerPlayer.Inventory,
				},
			}
			b, _ := json.Marshal(invMsg)
			Broadcast(b)
		}
		log.Printf("[transferQuestArtifact] artifact %d transferred from dead player %d to killer %d", qa.ID, deadPlayerID, killerID)
	} else {
		// Drop artifact on the death cell as a collectible resource
		cells, err := Combat.LoadMap(instanceID)
		if err != nil {
			log.Printf("[transferQuestArtifact] LoadMap: %v", err)
			return
		}
		for i := range cells {
			if cells[i].X == deadX && cells[i].Y == deadY {
				if cells[i].Resource == nil {
					cells[i].Resource = &game.ResourceData{
						ID:          qa.ID,
						Type:        qa.Name,
						Description: qa.Description,
						Image:       qa.Image,
						Effect:      map[string]int{},
						ItemType:    "artifact",
					}
					_ = Combat.SaveMap(instanceID, cells)
					upd := map[string]interface{}{
						"type": "UPDATE_CELL",
						"payload": map[string]interface{}{
							"instanceId":  instanceID,
							"updatedCell": serialiseUpdatedCell(cells[i]),
						},
					}
					b, _ := json.Marshal(upd)
					Broadcast(b)
					log.Printf("[transferQuestArtifact] artifact %d dropped at (%d,%d)", qa.ID, deadX, deadY)
				}
				break
			}
		}
	}
}

// --- Смерть игрока (удаление, флаги, WS: PLAYER_DEFEATED) -------------------
func handlePlayerDeath(instanceID string, p *models.PlayerResponse, killerID int, killerIsPlayer bool) {
	userID := p.UserID
	oldPos := p.Position

	log.Printf("[handlePlayerDeath] start, userID=%d, pos=%+v", userID, oldPos)

	// Transfer quest artifact before removing player from match
	transferQuestArtifact(instanceID, userID, oldPos.X, oldPos.Y, killerID, killerIsPlayer)

	if err := Combat.MarkPlayerDead(instanceID, p.UserID); err != nil {
		log.Printf("[handlePlayerDeath] MarkPlayerDead error: %v", err)
	}

	unbindPlayer(p.UserID)

	// 2 Если в памяти есть матч
	if ms, ok := game.GetMatchState(instanceID); ok {
		ms.RemovePlayerFromTurnOrder(userID)

		// 2а Если больше нет игроков — завершаем матч
		if len(ms.TurnOrder) == 0 {
			log.Printf("[combat] all players dead → auto-finalize match %s", instanceID)
			if err := Combat.Finalize(instanceID); err != nil {
				log.Printf("[combat] FinalizeMatch failed: %v", err)
			} else {
				log.Printf("[combat] FinalizeMatch OK for %s", instanceID)
			}
			msg := map[string]interface{}{
				"type":    "MATCH_ENDED",
				"payload": map[string]string{"instanceId": instanceID},
			}
			b, _ := json.Marshal(msg)
			Broadcast(b)
			return
		}

		nextID := ms.ActiveUserID
		if nextID != 0 {
			if err := Combat.UpdateTurn(instanceID, nextID, ms.TurnNumber); err != nil {
				log.Printf("UpdateMatchTurn error: %v", err)
			}
			if err := regenEnergyForNextPlayer(instanceID, nextID, energyRegen); err != nil {
				log.Printf("Ошибка регенерации энергии новому игроку после смерти: %v", err)
			}
			turnMsg := map[string]interface{}{
				"type": "TURN_PASSED",
				"payload": map[string]interface{}{
					"instanceId": instanceID,
					"userId":     nextID,
					"turnNumber": ms.TurnNumber,
				},
			}
			buf, _ := json.Marshal(turnMsg)
			Broadcast(buf)
		}
	}

	// 3 Обновляем флаги клетки И шлём только один WS: PLAYER_DEFEATED
	if err := Combat.ClearPlayerFlag(instanceID, oldPos); err != nil {
		log.Printf("[handlePlayerDeath] ClearCellPlayerFlag error: %v", err)
	} else {
		cells, err := Combat.LoadMap(instanceID)
		if err == nil {
			for i := range cells {
				if cells[i].X == oldPos.X && cells[i].Y == oldPos.Y {
					updatedCell := serialiseUpdatedCell(cells[i])

					// --- 1. Можно удалить UPDATE_CELL, если фронт ждёт только PLAYER_DEFEATED ---

					// --- 2. PLAYER_DEFEATED с updatedCell ---
					msg := map[string]interface{}{
						"type": "PLAYER_DEFEATED",
						"payload": map[string]interface{}{
							"instanceId":  instanceID,
							"userId":      userID,
							"updatedCell": updatedCell,
						},
					}
					b, _ := json.Marshal(msg)
					log.Printf("!!! Broadcast PLAYER_DEFEATED for user %d, cell %+v", userID, updatedCell)
					log.Printf("!!! Broadcast PLAYER_DEFEATED JSON: %s", string(b))

					Broadcast(b)
					break
				}
			}
		}
	}
}

// --- Смерть монстра (удаление, очистка клетки, WS: UPDATE_CELL) -------------

func handleMonsterDeath(instanceID string, monsterID int) {
	m, err := Combat.GetMonster(instanceID, monsterID)
	if err != nil {
		return
	}
	x, y := m.X, m.Y
	_ = Combat.DeleteMonster(instanceID, monsterID)

	cells, err := Combat.LoadMap(instanceID)
	if err == nil {
		for i := range cells {
			if cells[i].X == x && cells[i].Y == y {
				cells[i].Monster = nil
				cells[i].TileCode = 48
				break
			}
		}
		_ = Combat.SaveMap(instanceID, cells)
	}

	update := map[string]interface{}{
		"type": "UPDATE_CELL",
		"payload": map[string]interface{}{
			"instanceId": instanceID,
			"updatedCell": map[string]interface{}{
				"x":        x,
				"y":        y,
				"tileCode": 48,
				"monster":  nil,
			},
		},
	}
	buf, _ := json.Marshal(update)
	Broadcast(buf)
}

func sendUpdatePlayerWS(instanceID string, playerID int) {
	p, err := repository.GetMatchPlayerByID(instanceID, playerID)
	if err != nil {
		return
	}
	updatePlayerMsg := map[string]interface{}{
		"type": "UPDATE_PLAYER",
		"payload": map[string]interface{}{
			"instanceId": instanceID,
			"player":     p,
		},
	}
	buf, _ := json.Marshal(updatePlayerMsg)
	Broadcast(buf)
}

// --- Контратака + логика TURN_PASSED для игрока ----------------------------
// attackerType/attackerID - тот, кто атаковал
// defenderType/defenderID - тот, кто защищается (и может делать контратаку)
func doCounterattackWithEnergy(
	instanceID string,
	attackerType string, attackerID int, // <-- кто получает урон
	defenderType string, defenderID int, // <-- кто контратакует
	attackerStats stats, defenderStats stats,
	targetAlive bool,
) (attackResult, error) {
	if !targetAlive {
		return attackResult{Damage: 0, NewHealth: attackerStats.Health}, nil
	}

	// Проверяем энергию для контратаки (только если defender — игрок)
	if defenderType == "player" {
		p, err := Combat.GetPlayer(instanceID, defenderID)
		if err != nil {
			return attackResult{Damage: 0, NewHealth: attackerStats.Health}, nil
		}
		if p.Energy < counterAttackEnergyCost {
			// Недостаточно энергии — нет контратаки
			return attackResult{Damage: 0, NewHealth: attackerStats.Health}, nil
		}
		// Списываем энергию
		p.Energy -= counterAttackEnergyCost
		Combat.UpdatePlayer(instanceID, p)
	}

	// Контратака происходит
	ar := applyDamage(defenderStats, attackerStats) // defender контратакует attacker

	// Обновляем здоровье атакующего
	if attackerType == "player" {
		p, err := Combat.GetPlayer(instanceID, attackerID)
		if err == nil {
			p.Health = ar.NewHealth
			if p.Health > 0 {
				Combat.UpdatePlayer(instanceID, p)
			} else {
				// Counterattack: defender killed the attacker
				handlePlayerDeath(instanceID, p, defenderID, defenderType == "player")
			}
		}
	} else {
		Combat.UpdateMonsterHealth(instanceID, attackerID, ar.NewHealth)
		if ar.NewHealth <= 0 {
			handleMonsterDeath(instanceID, attackerID)
		}
	}
	return ar, nil
}

// --- Главная функция обработки атаки --------------------------------------
func UniversalAttackHandler(w http.ResponseWriter, r *http.Request) {
	// 1) Декодируем
	var req struct {
		InstanceID   string `json:"instance_id"`
		AttackerType string `json:"attacker_type"`
		AttackerID   int    `json:"attacker_id"`
		TargetType   string `json:"target_type"`
		TargetID     int    `json:"target_id"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		log.Printf("[DEBUG] UniversalAttackHandler: decode error: %v", err)
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}

	// --- ENERGY COST FOR ATTACK ---
	if req.AttackerType == "player" {
		player, err := repository.GetMatchPlayerByID(req.InstanceID, req.AttackerID)
		if err != nil {
			http.Error(w, "Ошибка загрузки игрока", http.StatusInternalServerError)
			return
		}
		if player.Energy < attackEnergyCost {
			http.Error(w, "Недостаточно энергии для атаки", http.StatusBadRequest)
			return
		}
		player.Energy -= attackEnergyCost
		if err := repository.UpdateMatchPlayer(req.InstanceID, player); err != nil {
			http.Error(w, "Ошибка обновления энергии", http.StatusInternalServerError)
			return
		}
	}
	// -------------------------------

	atkStats, err := loadStats(req.InstanceID, req.AttackerType, req.AttackerID)
	if err != nil {
		log.Printf("[DEBUG] UniversalAttackHandler: loadStats attacker error: %v", err)
		http.Error(w, "failed to load attacker stats", http.StatusInternalServerError)
		return
	}
	defStats, err := loadStats(req.InstanceID, req.TargetType, req.TargetID)
	if err != nil {
		log.Printf("[DEBUG] UniversalAttackHandler: loadStats target error: %v", err)
		http.Error(w, "failed to load target stats", http.StatusInternalServerError)
		return
	}

	// 4) Урон по цели
	targetRes := applyDamage(atkStats, defStats)
	//  Добавляем в состояние матча запись о нанесённом уроне
	if ms, ok := game.GetMatchState(req.InstanceID); ok {
		ms.RecordDamageEvent(req.AttackerID, req.TargetType, targetRes.Damage)
	}
	// 5) Сохранение цели + возможная смерть

	saveTargetHealth(req.InstanceID, req.TargetType, req.TargetID, req.AttackerID, req.AttackerType, targetRes)

	// 6+7) Контратака (если цель жива) + возможный TURN_PASSED
	counterRes, _ := doCounterattackWithEnergy(
		req.InstanceID,
		req.AttackerType, req.AttackerID,
		req.TargetType, req.TargetID,
		atkStats, defStats,
		targetRes.NewHealth > 0,
	)

	// 8) HTTP-ответ
	resp := map[string]interface{}{
		"damage_to_target": targetRes.Damage,
		"new_target_hp":    targetRes.NewHealth,
		"counter_damage":   counterRes.Damage,
		"new_attacker_hp":  counterRes.NewHealth,
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)

	// 9) WS: COMBAT_EXCHANGE
	msg := CombatExchangeMessage{Type: "COMBAT_EXCHANGE"}
	msg.Payload.Attacker.ID = req.AttackerID
	msg.Payload.Attacker.Damage = counterRes.Damage
	msg.Payload.Attacker.NewHP = counterRes.NewHealth
	msg.Payload.Target.ID = req.TargetID
	msg.Payload.Target.Damage = targetRes.Damage
	msg.Payload.Target.NewHP = targetRes.NewHealth
	msg.Payload.InstanceID = req.InstanceID

	data, _ := json.Marshal(msg)
	Broadcast(data)

	// 10) WS: MATCH_UPDATE — обновим статы игроков (HP, energy и т.п.)
	if req.AttackerType == "player" {
		sendUpdatePlayerWS(req.InstanceID, req.AttackerID)
	}
	if req.TargetType == "player" {
		sendUpdatePlayerWS(req.InstanceID, req.TargetID)
	}

}
