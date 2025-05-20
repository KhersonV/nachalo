// ====================================
// gameservice/handlers/combat.go
// ====================================

package handlers

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"log"

	"gameservice/game"
	"gameservice/middleware"
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

// nopCloser нужен, чтобы bytes.Buffer имплементировал io.ReadCloser
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
		// Ветка: атака монстра
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
	if cnt, _ := repository.CollisionCount(instanceID, req.NewPosX, req.NewPosY, userID); cnt > 0 {
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
	// Загрузка JSON-карты из БД
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
	// Находим целевую клетку
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
	// Проверяем проходимость
	if !cellPassable(targetCell.TileCode) {
		http.Error(w, fmt.Sprintf("Непроходимый тайл %d", targetCell.TileCode), http.StatusBadRequest)
		return
	}

	// 6) Иначе — обычное перемещение
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

	if err := repository.UpdateMatchPlayer(player); err != nil {
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

	// HTTP-ответ
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(player)
}

// UniversalAttackHandler — выполняет атаку + кантратака + COMBAT_EXCHANGE

// UniversalAttackHandler — выполняет атаку + контратаку + COMBAT_EXCHANGE
func UniversalAttackHandler(w http.ResponseWriter, r *http.Request) {
    // 1) Декодируем запрос
    var req struct {
        InstanceID   string `json:"instance_id"`
        AttackerType string `json:"attacker_type"` // "player" или "monster"
        AttackerID   int    `json:"attacker_id"`
        TargetType   string `json:"target_type"` // "player" или "monster"
        TargetID     int    `json:"target_id"`
    }
    if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
        http.Error(w, "invalid request body", http.StatusBadRequest)
        return
    }

    // 2) Загружаем статы атакующего
    var atkAttack, atkDefense, atkHealth int
    if req.AttackerType == "player" {
        player, err := repository.GetMatchPlayerByID(req.InstanceID, req.AttackerID)
        if err != nil {
            http.Error(w, "failed to load attacker player", http.StatusInternalServerError)
            return
        }
        atkAttack, atkDefense, atkHealth = player.Attack, player.Defense, player.Health
    } else {
        monster, err := repository.GetMatchMonsterByID(req.InstanceID, req.AttackerID)
        if err != nil {
            http.Error(w, "failed to load attacker monster", http.StatusInternalServerError)
            return
        }
        atkAttack, atkDefense, atkHealth = monster.Attack, monster.Defense, monster.Health
    }

    // 3) Загружаем статы цели
    var defAttack, defDefense, defHealth int
    if req.TargetType == "player" {
        player, err := repository.GetMatchPlayerByID(req.InstanceID, req.TargetID)
        if err != nil {
            http.Error(w, "failed to load target player", http.StatusInternalServerError)
            return
        }
        defAttack, defDefense, defHealth = player.Attack, player.Defense, player.Health
    } else {
        monster, err := repository.GetMatchMonsterByID(req.InstanceID, req.TargetID)
        if err != nil {
            http.Error(w, "failed to load target monster", http.StatusInternalServerError)
            return
        }
        defAttack, defDefense, defHealth = monster.Attack, monster.Defense, monster.Health
    }

    // 4) Расчёт урона по цели
    damageToTarget := atkAttack - defDefense
    if damageToTarget < 0 {
        damageToTarget = 0
    }
    newTargetHealth := defHealth - damageToTarget
    if newTargetHealth < 0 {
        newTargetHealth = 0
    }

    // 5) Сохраняем здоровье цели (игрок или монстр)
    if req.TargetType == "player" {
        player, _ := repository.GetMatchPlayerByID(req.InstanceID, req.TargetID)
        player.Health = newTargetHealth
        if newTargetHealth > 0 {
            // обычный случай
            repository.UpdateMatchPlayer(player)
        } else {
            // игрок погиб — удаляем его из матча
            if err := repository.DeleteMatchPlayer(req.InstanceID, req.TargetID); err != nil {
                log.Printf("Ошибка удаления погибшего игрока: %v", err)
            }
            if ms, ok := game.GetMatchState(req.InstanceID); ok {
                ms.RemovePlayerFromTurnOrder(req.TargetID)
            }
            // сброс флага isPlayer в карте
            oldPos := player.Position
            _ = repository.UpdateCellPlayerFlags(req.InstanceID, oldPos, oldPos)

            // WS: PLAYER_DEFEATED
            defeatMsg := map[string]interface{}{
                "type": "PLAYER_DEFEATED",
                "payload": map[string]interface{}{
                    "instanceId": req.InstanceID,
                    "userId":     req.TargetID,
                },
            }
            data, _ := json.Marshal(defeatMsg)
            Broadcast(data)
        }
    } else {
        // Для монстра
        repository.UpdateMatchMonsterHealth(req.InstanceID, req.TargetID, newTargetHealth)
        if newTargetHealth == 0 {
            // получить координаты и очистить клетку
            monsterRec, err := repository.GetMatchMonsterByID(req.InstanceID, req.TargetID)
            if err == nil {
                x, y := monsterRec.X, monsterRec.Y
                repository.DeleteMatchMonster(req.InstanceID, req.TargetID)
                cells, err := repository.LoadMapCells(req.InstanceID)
                if err == nil {
                    for i := range cells {
                        if cells[i].X == x && cells[i].Y == y {
                            cells[i].Monster = nil
                            cells[i].TileCode = 48
                            break
                        }
                    }
                    repository.SaveMapCells(req.InstanceID, cells)
                }
                updateCell := map[string]interface{}{
                    "type": "UPDATE_CELL",
                    "payload": map[string]interface{}{
                        "updatedCell": map[string]interface{}{
                            "x":        x,
                            "y":        y,
                            "tileCode": 48,
                            "monster":  nil,
                        },
                    },
                }
                buf, _ := json.Marshal(updateCell)
                Broadcast(buf)
            }
        }
    }

    // 6) Расчёт контратаки
    counterDamage := defAttack - atkDefense
    if counterDamage < 0 {
        counterDamage = 0
    }
    newAttackerHealth := atkHealth - counterDamage
    if newAttackerHealth < 0 {
        newAttackerHealth = 0
    }

    // 7) Сохраняем здоровье атакующего и обрабатываем смерть
    if req.AttackerType == "player" {
        attacker, _ := repository.GetMatchPlayerByID(req.InstanceID, req.AttackerID)
        attacker.Health = newAttackerHealth
        repository.UpdateMatchPlayer(attacker)

        if newAttackerHealth == 0 {
            // удалить из матча
            if err := repository.DeleteMatchPlayer(req.InstanceID, req.AttackerID); err != nil {
                log.Printf("Ошибка удаления погибшего игрока (атакующий): %v", err)
            }
            if ms, ok := game.GetMatchState(req.InstanceID); ok {
                ms.RemovePlayerFromTurnOrder(req.AttackerID)
                nextID := ms.ActiveUserID
                // сохранить в БД новый ход
                if nextID != 0 {
    			// сохраняем в БД новый active_user_id и turn_number
					if err := repository.UpdateMatchTurn(
						req.InstanceID,
						nextID,
						ms.TurnNumber,        // возьмём актуальный счётчик ходов из in-memory
					); err != nil {
						log.Printf("Ошибка UpdateMatchTurn: %v", err)
					}
				}
                // WS: PLAYER_DEFEATED
                defeatMsg := map[string]interface{}{
                    "type": "PLAYER_DEFEATED",
                    "payload": map[string]interface{}{
                        "instanceId": req.InstanceID,
                        "userId":     req.AttackerID,
                    },
                }
                buf1, _ := json.Marshal(defeatMsg)
                Broadcast(buf1)
                // WS: TURN_PASSED
                if nextID != 0 {
                    turnMsg := map[string]interface{}{
                        "type": "TURN_PASSED",
                        "payload": map[string]interface{}{
                            "instanceId": req.InstanceID,
                            "userId":     nextID,
							"turnNumber": ms.TurnNumber,
                        },
                    }
                    buf2, _ := json.Marshal(turnMsg)
                    Broadcast(buf2)
                }
            }
        }
    } else {
        repository.UpdateMatchMonsterHealth(req.InstanceID, req.AttackerID, newAttackerHealth)
    }

    // 8) HTTP-ответ инициатору
    w.Header().Set("Content-Type", "application/json")
    json.NewEncoder(w).Encode(map[string]interface{}{
        "damage_to_target": damageToTarget,
        "new_target_hp":    newTargetHealth,
        "counter_damage":   counterDamage,
        "new_attacker_hp":  newAttackerHealth,
    })

    // 9) WS: COMBAT_EXCHANGE
    msg := CombatExchangeMessage{Type: "COMBAT_EXCHANGE"}
    msg.Payload.InstanceID = req.InstanceID
    msg.Payload.Attacker.ID = req.AttackerID
    msg.Payload.Attacker.Damage = counterDamage
    msg.Payload.Attacker.NewHP = newAttackerHealth
    msg.Payload.Target.ID = req.TargetID
    msg.Payload.Target.Damage = damageToTarget
    msg.Payload.Target.NewHP = newTargetHealth

    data, _ := json.Marshal(msg)
    Broadcast(data)
}
