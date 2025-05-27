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
		p, err := Combat.GetPlayer(instanceID, entityID)
		if err != nil {
			return stats{}, err
		}
		return stats{p.Attack, p.Defense, p.Health}, nil
	}
	m, err := Combat.GetMonster(instanceID, entityID)
	if err != nil {
		return stats{}, err
	}
	return stats{m.Attack, m.Defense, m.Health}, nil
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
	ar attackResult,
) {
	if targetType == "player" {
		p, err := Combat.GetPlayer(instanceID, targetID)
		if err != nil {
			log.Printf("saveTargetHealth load player error: %v", err)
			return
		}
		p.Health = ar.NewHealth
		if ar.NewHealth > 0 {
			Combat.UpdatePlayer(instanceID, p)
		} else {
			// Записываем факт убийства игрока
			if ms, ok := game.GetMatchState(instanceID); ok {
				ms.RecordKillEvent(attackerID, "player", ar.Damage)
			}
			handlePlayerDeath(instanceID, p)
		}
	} else {
		// монстр получает урон
		Combat.UpdateMonsterHealth(instanceID, targetID, ar.NewHealth)
		if ar.NewHealth <= 0 {
			// Записываем факт убийства монстра
			if ms, ok := game.GetMatchState(instanceID); ok {
				ms.RecordKillEvent(attackerID, "monster", ar.Damage)
			}
			handleMonsterDeath(instanceID, targetID)
		}
	}
}

// --- Смерть игрока (удаление, флаги, WS: PLAYER_DEFEATED) -------------------

func handlePlayerDeath(instanceID string, p *models.PlayerResponse) {
	log.Printf("[handlePlayerDeath] called for user %d in match %s (health now %d)", p.UserID, instanceID, p.Health)
	userID := p.UserID
	oldPos := p.Position

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
			// Разошлём WS-уведомление, что матч окончен
			msg := map[string]interface{}{
				"type":    "MATCH_ENDED",
				"payload": map[string]string{"instanceId": instanceID},
			}
			b, _ := json.Marshal(msg)
			Broadcast(b)

			// Важно: после MATCH_ENDED мы уходим из функции и больше ничего не шлём
			return
		}

		// 2б теперь передаём ход дальше

		nextID := ms.ActiveUserID // после .RemovePlayerFromTurnOrder это следующий игрок
		if nextID != 0 {
			// обновляем в БД
			if err := Combat.UpdateTurn(instanceID, nextID, ms.TurnNumber); err != nil {
				log.Printf("UpdateMatchTurn error: %v", err)
			}
			// шлём событие TURN_PASSED
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

	// 3 Обновляем флаги клетки
	if err := Combat.ClearPlayerFlag(instanceID, oldPos); err != nil {
		log.Printf("[handlePlayerDeath] ClearCellPlayerFlag error: %v", err)
	}

	// 4 WS: PLAYER_DEFEATED
	msg := map[string]interface{}{
		"type": "PLAYER_DEFEATED",
		"payload": map[string]interface{}{
			"instanceId": instanceID,
			"userId":     userID,
		},
	}
	b, _ := json.Marshal(msg)
	Broadcast(b)
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

// --- Контратака + логика TURN_PASSED для игрока ----------------------------

func doCounterattack(
	instanceID, attackerType string,
	attackerID int,
	attStats, defStats stats,
	targetAlive bool,
) (attackResult, error) {
	// если цель мертва — нет контратаки
	if !targetAlive {
		return attackResult{Damage: 0, NewHealth: attStats.Health}, nil
	}
	ar := applyDamage(defStats, attStats)

	if attackerType == "player" {
		p, err := Combat.GetPlayer(instanceID, attackerID)
		if err != nil {
			log.Printf("doCounterattack load player error: %v", err)
			return ar, err
		}
		p.Health = ar.NewHealth
		if ar.NewHealth > 0 {
			Combat.UpdatePlayer(instanceID, p)
		} else {
			// игрок погиб в контратаке
			handlePlayerDeath(instanceID, p)

			// переход хода другому
			if ms, ok := game.GetMatchState(instanceID); ok {
				nextID := ms.ActiveUserID
				if nextID != 0 {
					// сохранить новый ход в БД
					if err := Combat.UpdateTurn(instanceID, nextID, ms.TurnNumber); err != nil {
						log.Printf("UpdateMatchTurn error: %v", err)
					}
					// WS: TURN_PASSED
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
		}
	} else {
		// монстр получает урон
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
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}

	// 2+3) Загружаем статы
	atkStats, err := loadStats(req.InstanceID, req.AttackerType, req.AttackerID)
	if err != nil {
		http.Error(w, "failed to load attacker stats", http.StatusInternalServerError)
		return
	}
	defStats, err := loadStats(req.InstanceID, req.TargetType, req.TargetID)
	if err != nil {
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
	saveTargetHealth(req.InstanceID, req.TargetType, req.TargetID, req.AttackerID, targetRes)

	// 6+7) Контратака (если цель жива) + возможный TURN_PASSED
	counterRes, _ := doCounterattack(
		req.InstanceID,
		req.AttackerType,
		req.AttackerID,
		atkStats,
		defStats,
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
}
