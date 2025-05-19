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

	"gameservice/game"
	"gameservice/middleware"
	"gameservice/repository"
	"github.com/gorilla/mux"
)

// Стоимость перемещения: 1 единица энергии.
const moveEnergyCost = 1

// nopCloser нужен, чтобы bytes.Buffer имплементировал io.ReadCloser
type nopCloser struct{ *bytes.Buffer }
func (nopCloser) Close() error { return nil }

// cellPassable возвращает, можно ли ходить по тайлу с данным кодом
func cellPassable(code int) bool {
	for _, c := range []int{48, 80, 82, 112, 66} {
		if code == c {
			return true
		}
	}
	return false
}

// rWithBody создаёт http.Request с JSON-телом для внутреннего повторного вызова
func rWithBody(body interface{}) *http.Request {
	buf := new(bytes.Buffer)
	json.NewEncoder(buf).Encode(body)
	return &http.Request{Body: nopCloser{buf}}
}

// MoveOrAttackHandler объединяет ход и атаку в один эндпоинт.


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

	// 3) Загружаем размеры карты для проверки границ
	var mapW, mapH int
	if err := repository.DB.QueryRow(
		`SELECT map_width, map_height FROM matches WHERE instance_id = $1`,
		instanceID,
	).Scan(&mapW, &mapH); err != nil {
		http.Error(w, fmt.Sprintf("Ошибка загрузки размеров карты: %v", err), http.StatusInternalServerError)
		return
	}
	if req.NewPosX < 0 || req.NewPosX >= mapW || req.NewPosY < 0 || req.NewPosY >= mapH {
		http.Error(w, "Координаты вне карты", http.StatusBadRequest)
		return
	}

	// 4) Пытаемся найти монстра в БД
	mm, err := repository.GetMatchMonsterAt(instanceID, req.NewPosX, req.NewPosY)
	if err != nil {
		http.Error(w, "Ошибка чтения монстров", http.StatusInternalServerError)
		return
	}
	if mm != nil {
		// 5a) Это монстр — считаем урон
		player, err := repository.GetMatchPlayerByID(instanceID, userID)
		if err != nil {
			http.Error(w, "Ошибка загрузки игрока", http.StatusInternalServerError)
			return
		}
		dmg := player.Attack - mm.Defense
		if dmg < 0 {
			dmg = 0
		}
		newHP := mm.Health - dmg
		if newHP < 0 {
			newHP = 0
		}

		// 5b) Обновляем здоровье монстра
		if err := repository.UpdateMatchMonsterHealth(instanceID, mm.MonsterInstanceID, newHP); err != nil {
			http.Error(w, "Ошибка обновления монстра", http.StatusInternalServerError)
			return
		}

		// 5c) Отправляем HTTP-ответ
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"damage":        dmg,
			"new_target_hp": newHP,
		})

		// 5d) Шлём WS-уведомление только об обновлённом монстре
		updateMsg := map[string]interface{}{
			"type": "MONSTER_HIT",
			"payload": map[string]interface{}{
				"x":                 req.NewPosX,
				"y":                 req.NewPosY,
				"monsterInstanceId": mm.MonsterInstanceID,
				"newHP":             newHP,
			},
		}
		b, _ := json.Marshal(updateMsg)
		Broadcast(b)
		return
	}

	// 6) Если там игрок — внутренняя атака
	if cnt, _ := repository.CollisionCount(instanceID, req.NewPosX, req.NewPosY, userID); cnt > 0 {
		other, err := repository.GetOtherPlayerID(instanceID, req.NewPosX, req.NewPosY, userID)
		if err != nil {
			http.Error(w, fmt.Sprintf("Не удалось найти другого игрока: %v", err), http.StatusInternalServerError)
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

	// 7) Обычное перемещение: нужно проверить TileCode в JSON-карте
	var mapJSON []byte
	if err := repository.DB.QueryRow(
		`SELECT map FROM matches WHERE instance_id = $1`,
		instanceID,
	).Scan(&mapJSON); err != nil {
		http.Error(w, fmt.Sprintf("Ошибка загрузки карты: %v", err), http.StatusInternalServerError)
		return
	}
	var cells []game.FullCell
	if err := json.Unmarshal(mapJSON, &cells); err != nil {
		http.Error(w, fmt.Sprintf("Ошибка разбора карты: %v", err), http.StatusInternalServerError)
		return
	}
	var target *game.FullCell
	for i := range cells {
		if cells[i].X == req.NewPosX && cells[i].Y == req.NewPosY {
			target = &cells[i]
			break
		}
	}
	if target == nil {
		http.Error(w, "Клетка не найдена", http.StatusBadRequest)
		return
	}
	if !cellPassable(target.TileCode) {
		http.Error(w, fmt.Sprintf("Непроходимый тайл %d", target.TileCode), http.StatusBadRequest)
		return
	}

	// 8) Обновляем позицию игрока
	player, err := repository.GetMatchPlayerByID(instanceID, userID)
	if err != nil {
		http.Error(w, fmt.Sprintf("Ошибка загрузки игрока: %v", err), http.StatusInternalServerError)
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
		http.Error(w, fmt.Sprintf("Ошибка обновления игрока: %v", err), http.StatusInternalServerError)
		return
	}
	if err := repository.UpdateCellPlayerFlags(instanceID, oldPos, player.Position); err != nil {
		http.Error(w, fmt.Sprintf("Ошибка обновления карты: %v", err), http.StatusInternalServerError)
		return
	}

	// 9) Уведомление по WebSocket о перемещении
	moveMsg := map[string]interface{}{
    "type": "MOVE_PLAYER",
    "payload": map[string]interface{}{
        "userId": userID,
        "newPosition": map[string]int{
            "x": req.NewPosX,
            "y": req.NewPosY,
        },
    },
}
b2, _ := json.Marshal(moveMsg)
Broadcast(b2)

	// 10) Отправляем клиенту обновлённого себя
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(player)
}

// ----------------- АТАКА -----------------

type AttackRequest struct {
	AttackerType string `json:"attacker_type"`
	AttackerID   int    `json:"attacker_id"`
	TargetType   string `json:"target_type"`
	TargetID     int    `json:"target_id"`
	InstanceID   string `json:"instance_id"`
}

// UniversalAttackHandler — выполняет атаку и обновляет либо игрока, либо монстра прямо в JSON-карте.
func UniversalAttackHandler(w http.ResponseWriter, r *http.Request) {
	var req AttackRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Неверный формат запроса", http.StatusBadRequest)
		return
	}

	// --- загрузим карту, чтобы работать с monster внутри неё ---
	var mapJSON []byte
	if err := repository.DB.QueryRow(
		`SELECT map FROM matches WHERE instance_id=$1`, req.InstanceID,
	).Scan(&mapJSON); err != nil {
		http.Error(w, fmt.Sprintf("Ошибка загрузки карты: %v", err), http.StatusInternalServerError)
		return
	}
	var cells []game.FullCell
	if err := json.Unmarshal(mapJSON, &cells); err != nil {
		http.Error(w, fmt.Sprintf("Ошибка парсинга карты: %v", err), http.StatusInternalServerError)
		return
	}

	// найдём cell с нужным TargetID
	var attackerStats, defenderStats struct{ Attack, Defense, Health int }
	for i := range cells {
		c := &cells[i]
		if req.AttackerType == "monster" && c.Monster != nil && c.Monster.ID == req.AttackerID {
			attackerStats.Attack = c.Monster.Attack
		}
		if req.AttackerType == "player" && c.X == cells[i].X && c.Y == cells[i].Y {
			// игроку stats берём из БД
			pl, _ := repository.GetMatchPlayerByID(req.InstanceID, req.AttackerID)
			attackerStats.Attack = pl.Attack
		}
		if req.TargetType == "monster" && c.Monster != nil && c.Monster.ID == req.TargetID {
			defenderStats.Defense = c.Monster.Defense
			defenderStats.Health  = c.Monster.Health
		}
		if req.TargetType == "player" && c.X == cells[i].X && c.Y == cells[i].Y {
			pl, _ := repository.GetMatchPlayerByID(req.InstanceID, req.TargetID)
			defenderStats.Defense = pl.Defense
			defenderStats.Health  = pl.Health
		}
	}

	// --- рассчитываем урон ---
	dmg := attackerStats.Attack - defenderStats.Defense
	if dmg < 0 {
		dmg = 0
	}
	newHP := defenderStats.Health - dmg
	if newHP < 0 {
		newHP = 0
	}

	// --- применяем в карте (если монстр) или в БД (если игрок) ---
	if req.TargetType == "monster" {
		for i := range cells {
			if cells[i].Monster != nil && cells[i].Monster.ID == req.TargetID {
				cells[i].Monster.Health = newHP
				break
			}
		}
		// сохраняем обратно map JSON
		updatedMap, _ := json.Marshal(cells)
		repository.DB.Exec(
			`UPDATE matches SET map = $1 WHERE instance_id = $2`,
			string(updatedMap), req.InstanceID,
		)
	} else {
		pl, _ := repository.GetMatchPlayerByID(req.InstanceID, req.TargetID)
		pl.Health = newHP
		repository.UpdateMatchPlayer(pl)
	}

	// --- отвечаем клиенту ---
	resp := map[string]interface{}{
		"damage":        dmg,
		"new_target_hp": newHP,
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}
