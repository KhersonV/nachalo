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

const (
	meleeAttackEnergyCost            = 6
	rangedAttackEnergyCost           = 8
	standardCounterAttackEnergyCost  = 2
	guardianZoneControlRange         = 2
	guardianZoneControlMovePenalty   = 1
	armorBreakDefensePenaltyPerStack = 2
	armorBreakDurationTurns          = 2
	armorBreakMaxStacks              = 2
	berserkerFollowUpLimitPerTurn    = 3
	energyDrainPerHit                = 2
	energyDrainGainPerHit            = 1
	energyDrainPerTargetLimit        = 3
)

type CombatActorType string

const (
	CombatActorPlayer  CombatActorType = "player"
	CombatActorMonster CombatActorType = "monster"
)

type AttackStyle string

const (
	AttackStyleMelee  AttackStyle = "melee"
	AttackStyleRanged AttackStyle = "ranged"
	AttackStyleMagic  AttackStyle = "magic"
)

type CombatTargetRef struct {
	ID   int             `json:"id"`
	Type CombatActorType `json:"type"`
}

type CombatPoint struct {
	X int `json:"x"`
	Y int `json:"y"`
}

type CombatStep struct {
	Kind          string           `json:"kind"`
	Source        *CombatTargetRef `json:"source,omitempty"`
	Target        CombatTargetRef  `json:"target"`
	Damage        int              `json:"damage,omitempty"`
	TargetHPAfter int              `json:"targetHpAfter,omitempty"`
}

type CombatEffect struct {
	Kind              string           `json:"kind"`
	Source            *CombatTargetRef `json:"source,omitempty"`
	Target            *CombatTargetRef `json:"target,omitempty"`
	Value             int              `json:"value,omitempty"`
	Stacks            int              `json:"stacks,omitempty"`
	DurationTurns     int              `json:"durationTurns,omitempty"`
	Succeeded         bool             `json:"succeeded"`
	PositionAfter     *CombatPoint     `json:"positionAfter,omitempty"`
	BonusDamage       int              `json:"bonusDamage,omitempty"`
	EnergyGranted     int              `json:"energyGranted,omitempty"`
	EnergyDrained     int              `json:"energyDrained,omitempty"`
	SourceEnergyAfter int              `json:"sourceEnergyAfter,omitempty"`
	TargetEnergyAfter int              `json:"targetEnergyAfter,omitempty"`
}

// CombatExchangePayload — полезная нагрузка для WS-события боевого обмена
type CombatExchangePayload struct {
	InstanceID   string          `json:"instanceId"`
	ExchangeID   string          `json:"exchangeId"`
	AttackerID   int             `json:"attackerId"`
	AttackerType CombatActorType `json:"attackerType"`
	TargetID     int             `json:"targetId"`
	TargetType   CombatActorType `json:"targetType"`
	AttackStyle  AttackStyle     `json:"attackStyle"`
	Steps        []CombatStep    `json:"steps"`
	Effects      []CombatEffect  `json:"effects,omitempty"`
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

func toCombatActorType(entityType string) CombatActorType {
	if entityType == "monster" {
		return CombatActorMonster
	}
	return CombatActorPlayer
}

func nextCombatExchangeID(instanceID string) string {
	if ms, ok := Combat.LoadGameState(instanceID); ok && ms != nil {
		turn, seq := ms.NextCombatExchangeMeta()
		return fmt.Sprintf("%s:%d:%d", instanceID, turn, seq)
	}
	return fmt.Sprintf("%s:0:0", instanceID)
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

	cellIdx := -1
	var targetCell *game.FullCell
	for i := range cells {
		if cells[i].X == req.NewPosX && cells[i].Y == req.NewPosY {
			cellIdx = i
			targetCell = &cells[i]
			break
		}
	}
	if cellIdx == -1 || targetCell == nil {
		http.Error(w, "Клетка не найдена", http.StatusBadRequest)
		return
	}

	if !cellPassable(targetCell.TileCode) {
		http.Error(w, fmt.Sprintf("Непроходимый тайл %d", targetCell.TileCode), http.StatusBadRequest)
		return
	}

	if targetCell.StructureType != "" || targetCell.IsUnderConstruction {
		player, err := repository.GetMatchPlayerByID(instanceID, userID)
		if err != nil {
			http.Error(w, "Ошибка загрузки игрока", http.StatusInternalServerError)
			return
		}

		if manhattan(player.Position.X, player.Position.Y, req.NewPosX, req.NewPosY) != 1 {
			http.Error(w, "строение можно атаковать только в соседней клетке", http.StatusBadRequest)
			return
		}

		if !canAttackStructure(userID, targetCell) {
			http.Error(w, "это строение нельзя атаковать", http.StatusBadRequest)
			return
		}

		if player.Energy < meleeAttackEnergyCost {
			http.Error(w, "Недостаточно энергии для атаки", http.StatusBadRequest)
			return
		}

		structureType := targetCell.StructureType

		attackerStats := stats{
			Attack:        player.Attack,
			Defense:       player.Defense,
			Health:        player.Health,
			MaxHealth:     player.MaxHealth,
			IsRanged:      player.IsRanged,
			AttackRange:   player.AttackRange,
			CharacterType: player.CharacterType,
			X:             player.Position.X,
			Y:             player.Position.Y,
		}

		player.Energy -= meleeAttackEnergyCost
		if err := repository.UpdateMatchPlayer(instanceID, player); err != nil {
			http.Error(w, "Ошибка обновления энергии", http.StatusInternalServerError)
			return
		}

		targetRes := applyDamageToStructure(attackerStats, targetCell.StructureHealth, targetCell.StructureDefense)

		if targetRes.NewHealth > 0 {
			if err := damageStructureOnCell(instanceID, cells, cellIdx, targetRes.NewHealth); err != nil {
				http.Error(w, "Ошибка обновления строения", http.StatusInternalServerError)
				return
			}
		} else {
			if err := destroyStructureOnCell(instanceID, cells, cellIdx); err != nil {
				http.Error(w, "Ошибка разрушения строения", http.StatusInternalServerError)
				return
			}
		}

		sendUpdatePlayerWS(instanceID, userID)

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"attack_mode":      "melee",
			"damage_to_target": targetRes.Damage,
			"new_target_hp":    targetRes.NewHealth,
			"target_type":      "structure",
			"structure_type":   structureType,
		})
		return
	}

	// 6) Обычное перемещение
	player, err := repository.GetMatchPlayerByID(instanceID, userID)
	if err != nil {
		http.Error(w, "Ошибка загрузки игрока", http.StatusInternalServerError)
		return
	}
	if manhattan(player.Position.X, player.Position.Y, req.NewPosX, req.NewPosY) != 1 {
		http.Error(w, "можно двигаться только на соседнюю клетку", http.StatusBadRequest)
		return
	}

	moveCost, err := resolveMoveEnergyCost(instanceID, player)
	if err != nil {
		http.Error(w, "Ошибка расчёта стоимости движения", http.StatusInternalServerError)
		return
	}
	if player.Energy < moveCost {
		http.Error(w, "Недостаточно энергии", http.StatusBadRequest)
		return
	}

	oldPos := player.Position
	player.Energy -= moveCost
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

type attackMode string

const (
	attackModeMelee  attackMode = "melee"
	attackModeRanged attackMode = "ranged"
)

type stats struct {
	Attack        int
	Defense       int
	Health        int
	MaxHealth     int
	IsRanged      bool
	AttackRange   int
	CharacterType string
	X             int
	Y             int
}

type attackResult struct {
	Damage    int
	NewHealth int
	Triggered bool
}

// --- Загрузка статов игрока или монстра ------------------------------------

func loadStats(instanceID, entityType string, entityID int) (stats, error) {
	if entityType == "player" {
		p, err := Combat.GetPlayer(instanceID, entityID)
		if err != nil {
			return stats{}, err
		}
		return stats{
			Attack:        p.Attack,
			Defense:       p.Defense,
			Health:        p.Health,
			MaxHealth:     p.MaxHealth,
			IsRanged:      p.IsRanged,
			AttackRange:   p.AttackRange,
			CharacterType: p.CharacterType,
			X:             p.Position.X,
			Y:             p.Position.Y,
		}, nil
	}

	// monster

	mm, err := Combat.GetMonster(instanceID, entityID)
	if err != nil {
		return stats{}, err
	}
	return stats{
		Attack:        mm.Attack,
		Defense:       mm.Defense,
		Health:        mm.Health,
		MaxHealth:     mm.MaxHealth,
		IsRanged:      false,
		AttackRange:   1,
		CharacterType: "",
		X:             mm.X,
		Y:             mm.Y,
	}, nil
}

func manhattanDistance(att stats, def stats) int {
	dx := att.X - def.X
	if dx < 0 {
		dx = -dx
	}
	dy := att.Y - def.Y
	if dy < 0 {
		dy = -dy
	}
	return dx + dy
}

func abs(v int) int {
	if v < 0 {
		return -v
	}
	return v
}

func resolveAttackMode(attacker stats, target stats) (attackMode, error) {
	distance := manhattanDistance(attacker, target)
	if distance == 1 {
		return attackModeMelee, nil
	}
	if attacker.IsRanged && distance > 1 && distance <= attacker.AttackRange {
		return attackModeRanged, nil
	}
	return "", fmt.Errorf("цель вне дистанции атаки")
}

func resolvePresentationAttackStyle(attacker stats, mode attackMode) AttackStyle {
	if attacker.CharacterType == "mystic" {
		return AttackStyleMagic
	}
	if mode == attackModeRanged {
		return AttackStyleRanged
	}
	return AttackStyleMelee
}

func buildCombatExchangePayload(
	instanceID string,
	attackerType string,
	attackerID int,
	targetType string,
	targetID int,
	attackerStats stats,
	mode attackMode,
	steps []CombatStep,
	effects []CombatEffect,
) CombatExchangePayload {
	attackerRef := CombatTargetRef{ID: attackerID, Type: toCombatActorType(attackerType)}
	targetRef := CombatTargetRef{ID: targetID, Type: toCombatActorType(targetType)}

	return CombatExchangePayload{
		InstanceID:   instanceID,
		ExchangeID:   nextCombatExchangeID(instanceID),
		AttackerID:   attackerID,
		AttackerType: attackerRef.Type,
		TargetID:     targetID,
		TargetType:   targetRef.Type,
		AttackStyle:  resolvePresentationAttackStyle(attackerStats, mode),
		Steps:        steps,
		Effects:      effects,
	}
}

// --- Расчёт урона и оставшегося HP ----------------------------------------

func baseMoveEnergyCost(mobility int) int {
	switch {
	case mobility <= 2:
		return 4
	case mobility <= 5:
		return 3
	case mobility <= 8:
		return 2
	default:
		return 1
	}
}

func resolveMoveEnergyCost(instanceID string, player *models.PlayerResponse) (int, error) {
	cost := baseMoveEnergyCost(player.Mobility)

	players, err := repository.LoadMatchPlayers(instanceID)
	if err != nil {
		return 0, err
	}
	for _, other := range players {
		if other.UserID == player.UserID || other.Health <= 0 {
			continue
		}
		if other.CharacterType != "guardian" {
			continue
		}
		if player.GroupID != 0 && other.GroupID == player.GroupID {
			continue
		}
		if manhattan(player.Position.X, player.Position.Y, other.Position.X, other.Position.Y) <= guardianZoneControlRange {
			return cost + guardianZoneControlMovePenalty, nil
		}
	}

	return cost, nil
}

func resolveAttackEnergyCost(mode attackMode) int {
	if mode == attackModeRanged {
		return rangedAttackEnergyCost
	}
	return meleeAttackEnergyCost
}

func effectiveDefense(instanceID string, targetType string, targetID int, baseDefense int) int {
	defense := baseDefense
	if ms, ok := game.GetMatchState(instanceID); ok {
		state := ms.GetArmorBreakState(targetType, targetID)
		defense -= state.Stacks * armorBreakDefensePenaltyPerStack
	}
	if defense < 0 {
		return 0
	}
	return defense
}

func resolvePrimaryDamage(att stats, def stats) int {
	dmg := att.Attack - def.Defense
	if dmg < 0 {
		dmg = 0
	}
	if att.CharacterType == "berserker" && def.MaxHealth > 0 && dmg > 0 {
		hpRatio := float64(def.Health) / float64(def.MaxHealth)
		bonusMultiplier := 1.0
		switch {
		case hpRatio < 0.25:
			bonusMultiplier = 1.35
		case hpRatio < 0.50:
			bonusMultiplier = 1.20
		case hpRatio < 0.75:
			bonusMultiplier = 1.10
		}
		dmg = int(float64(dmg) * bonusMultiplier)
	}
	return dmg
}

func applyDamage(att stats, def stats) attackResult {
	dmg := resolvePrimaryDamage(att, def)
	newHP := def.Health - dmg
	if newHP < 0 {
		newHP = 0
	}
	return attackResult{Damage: dmg, NewHealth: newHP, Triggered: true}
}

func applyFlatDamage(targetHealth int, damage int) attackResult {
	if damage < 0 {
		damage = 0
	}
	newHP := targetHealth - damage
	if newHP < 0 {
		newHP = 0
	}
	return attackResult{Damage: damage, NewHealth: newHP, Triggered: true}
}

func canAttackStructure(attackerUserID int, cell *game.FullCell) bool {
	if cell == nil {
		return false
	}
	if cell.StructureType == "" || cell.IsUnderConstruction {
		return false
	}

	// Чужие здания можно атаковать всегда.
	if cell.StructureOwnerUserID != attackerUserID {
		return true
	}

	// Свои можно атаковать только если это стена.
	return cell.StructureType == "wall"
}

func applyDamageToStructure(att stats, structureHealth int, structureDefense int) attackResult {
	dmg := att.Attack - structureDefense
	if dmg < 0 {
		dmg = 0
	}

	newHP := structureHealth - dmg
	if newHP < 0 {
		newHP = 0
	}

	return attackResult{
		Damage:    dmg,
		NewHealth: newHP,
	}
}

func destroyStructureOnCell(instanceID string, cells []game.FullCell, idx int) error {
	cell := &cells[idx]

	ownerUserID := cell.StructureOwnerUserID
	structureType := cell.StructureType

	cell.StructureType = ""
	cell.StructureOwnerUserID = 0
	cell.StructureHealth = 0
	cell.StructureDefense = 0
	cell.StructureAttack = 0
	cell.StructureEnergy = 0
	cell.IsUnderConstruction = false
	cell.ConstructionTurnsLeft = 0

	if err := repository.SaveMapCells(instanceID, cells); err != nil {
		return err
	}

	if ownerUserID > 0 && structureType != "" {
		if err := repository.DecrementStructureCount(instanceID, ownerUserID, structureType); err != nil {
			log.Printf("destroyStructureOnCell: DecrementStructureCount failed: instance=%s owner=%d type=%s err=%v",
				instanceID, ownerUserID, structureType, err)
		}

		if structureType == "scout_tower" {
			if err := repository.RemoveScoutTowerBonusIfNeeded(instanceID, ownerUserID); err != nil {
				log.Printf("destroyStructureOnCell: RemoveScoutTowerBonusIfNeeded failed: instance=%s owner=%d err=%v",
					instanceID, ownerUserID, err)
			}
		}
	}

	// Build updatedCell as a map so we always include structure fields
	// (empty string / zero) — struct tags with `omitempty` would drop
	// empty values and client would not clear the structure.
	uc := serialiseUpdatedCell(*cell)
	updatedCellPayload := map[string]interface{}{
		"cell_id":  uc.CellID,
		"x":        uc.X,
		"y":        uc.Y,
		"tileCode": uc.TileCode,
		"resource": uc.Resource,
		"barbel":   uc.Barbel,
		"monster":  uc.Monster,
		"isPortal": uc.IsPortal,
		"isPlayer": uc.IsPlayer,
		// always include structure fields explicitly
		"structure_type":          uc.StructureType,
		"structure_owner_user_id": uc.StructureOwnerUserID,
		"structure_health":        uc.StructureHealth,
		"structure_defense":       uc.StructureDefense,
		"structure_attack":        uc.StructureAttack,
		"is_under_construction":   uc.IsUnderConstruction,
		"construction_turns_left": uc.ConstructionTurnsLeft,
	}
	update := map[string]interface{}{
		"type": "UPDATE_CELL",
		"payload": map[string]interface{}{
			"instanceId":  instanceID,
			"updatedCell": updatedCellPayload,
		},
	}
	buf, _ := json.Marshal(update)
	Broadcast(buf)

	return nil
}

func damageStructureOnCell(instanceID string, cells []game.FullCell, idx int, newHP int) error {
	cell := &cells[idx]
	cell.StructureHealth = newHP

	if err := repository.SaveMapCells(instanceID, cells); err != nil {
		return err
	}

	update := map[string]interface{}{
		"type": "UPDATE_CELL",
		"payload": map[string]interface{}{
			"instanceId":  instanceID,
			"updatedCell": serialiseUpdatedCell(*cell),
		},
	}
	buf, _ := json.Marshal(update)
	Broadcast(buf)

	return nil
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
				// Monster HP is now synced to clients via COMBAT_EXCHANGE.
				// Broadcasting a non-lethal UPDATE_CELL here makes the frontend
				// see the same combat twice: once as a legacy HP diff on the grid
				// update, and once from the normalized combat timeline.
				// We still persist the map state, and lethal removals continue to
				// broadcast UPDATE_CELL from handleMonsterDeath.
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

			// Если последний удар нанес игрок, фиксируем его (или его группу) победителем.
			if killerIsPlayer && killerID > 0 {
				if killer, kerr := repository.GetMatchPlayerByID(instanceID, killerID); kerr == nil && killer != nil {
					winnerID := killerID
					winnerGroupID := 0
					if killer.GroupID > 0 {
						winnerID = 0
						winnerGroupID = killer.GroupID
					}
					if err := repository.SetMatchWinner(instanceID, winnerID, winnerGroupID); err != nil {
						log.Printf("[combat] SetMatchWinner failed for %s: %v", instanceID, err)
					}
				}
			}

			// Важно: для последнего погибшего тоже отправляем PLAYER_DEFEATED,
			// иначе клиент не покажет модалку "Вы погибли".
			defeatedMsg := map[string]interface{}{
				"type": "PLAYER_DEFEATED",
				"payload": map[string]interface{}{
					"instanceId": instanceID,
					"userId":     userID,
				},
			}
			if b, derr := json.Marshal(defeatedMsg); derr == nil {
				Broadcast(b)
			}

			var endedPayload []byte
			if results, rerr := repository.GetMatchResults(instanceID); rerr == nil {
				if allStats, winnerType, winnerID, serr := buildAllPlayersGameStats(instanceID, results); serr == nil {
					endedMsg := matchEndedBroadcastResponse{
						Type: "MATCH_ENDED",
						Payload: matchEndedBroadcastResult{
							InstanceID: instanceID,
							WinnerType: winnerType,
							WinnerID:   winnerID,
							Stats:      allStats,
						},
					}
					if b, merr := json.Marshal(endedMsg); merr == nil {
						endedPayload = b
					}
				}
			}

			if err := Combat.Finalize(instanceID); err != nil {
				log.Printf("[combat] FinalizeMatch failed: %v", err)
			} else {
				log.Printf("[combat] FinalizeMatch OK for %s", instanceID)
			}

			if len(endedPayload) > 0 {
				Broadcast(endedPayload)
			} else {
				msg := map[string]interface{}{
					"type":    "MATCH_ENDED",
					"payload": map[string]string{"instanceId": instanceID},
				}
				b, _ := json.Marshal(msg)
				Broadcast(b)
			}
			return
		}

		nextID := ms.ActiveUserID
		if nextID != 0 {
			ms.AdvanceTurnCombatState(nextID, false)
			if err := Combat.UpdateTurn(instanceID, nextID, ms.TurnNumber); err != nil {
				log.Printf("UpdateMatchTurn error: %v", err)
			}
			if err := regenEnergyForNextPlayer(instanceID, nextID); err != nil {
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

	// Broadcast the full serialized cell so clients receive a complete,
	// consistent snapshot (prevents partial updates leaving stale data).
	var updated interface{}
	for i := range cells {
		if cells[i].X == x && cells[i].Y == y {
			updated = serialiseUpdatedCell(cells[i])
			break
		}
	}

	// Fallback to minimal payload if for some reason we don't have the full cell
	// (shouldn't happen, but be defensive).
	updatedCellPayload := updated
	if updatedCellPayload == nil {
		updatedCellPayload = map[string]interface{}{
			"x":        x,
			"y":        y,
			"tileCode": 48,
			"monster":  nil,
		}
	}

	update := map[string]interface{}{
		"type": "UPDATE_CELL",
		"payload": map[string]interface{}{
			"instanceId":  instanceID,
			"updatedCell": updatedCellPayload,
		},
	}
	buf, _ := json.Marshal(update)
	Broadcast(buf)
}

func sendUpdatePlayerWS(instanceID string, playerID int) {
	p, err := Combat.GetPlayer(instanceID, playerID)
	if err != nil || p == nil {
		p, err = repository.GetMatchPlayerByID(instanceID, playerID)
	}
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

func findCellIndex(cells []game.FullCell, x int, y int) int {
	for i := range cells {
		if cells[i].X == x && cells[i].Y == y {
			return i
		}
	}
	return -1
}

func isPushDestinationBlocked(cell *game.FullCell) bool {
	if cell == nil {
		return true
	}
	if !cellPassable(cell.TileCode) || cell.IsPortal || cell.IsPlayer || cell.Monster != nil {
		return true
	}
	if cell.Resource != nil || cell.Barbel != nil {
		return true
	}
	if cell.StructureType != "" || cell.IsUnderConstruction {
		return true
	}
	return false
}

func broadcastUpdatedCells(instanceID string, cells []game.FullCell) {
	seen := make(map[string]bool)
	for _, cell := range cells {
		key := fmt.Sprintf("%d:%d", cell.X, cell.Y)
		if seen[key] {
			continue
		}
		seen[key] = true

		update := map[string]interface{}{
			"type": "UPDATE_CELL",
			"payload": map[string]interface{}{
				"instanceId":  instanceID,
				"updatedCell": serialiseUpdatedCell(cell),
			},
		}
		buf, _ := json.Marshal(update)
		Broadcast(buf)
	}
}

func broadcastMovePlayer(instanceID string, userID int, position CombatPoint) {
	moveMsg := map[string]interface{}{
		"type": "MOVE_PLAYER",
		"payload": map[string]interface{}{
			"userId":      userID,
			"newPosition": map[string]int{"x": position.X, "y": position.Y},
			"instanceId":  instanceID,
		},
	}
	buf, _ := json.Marshal(moveMsg)
	Broadcast(buf)
}

func applyKnockbackOccupancy(
	cells []game.FullCell,
	targetType string,
	oldIdx int,
	newIdx int,
	movedMonster *game.MonsterData,
) ([]game.FullCell, error) {
	if oldIdx < 0 || newIdx < 0 {
		return nil, fmt.Errorf("invalid knockback cell indices")
	}

	switch targetType {
	case "player":
		cells[oldIdx].IsPlayer = false
		cells[newIdx].IsPlayer = true
	case "monster":
		if movedMonster == nil {
			return nil, fmt.Errorf("missing monster for knockback occupancy update")
		}
		monsterCopy := *movedMonster
		cells[oldIdx].Monster = nil
		cells[oldIdx].TileCode = int(game.Walkable)
		cells[newIdx].Monster = &monsterCopy
		cells[newIdx].TileCode = int('M')
	default:
		return nil, fmt.Errorf("unsupported knockback target type %s", targetType)
	}

	return []game.FullCell{cells[oldIdx], cells[newIdx]}, nil
}

func tryPushCombatTarget(
	instanceID string,
	targetType string,
	targetID int,
	current stats,
	destination CombatPoint,
) (bool, []game.FullCell, error) {
	cells, err := repository.LoadMapCells(instanceID)
	if err != nil {
		return false, nil, err
	}

	oldIdx := findCellIndex(cells, current.X, current.Y)
	newIdx := findCellIndex(cells, destination.X, destination.Y)
	if oldIdx < 0 || newIdx < 0 {
		return false, nil, nil
	}
	if isPushDestinationBlocked(&cells[newIdx]) {
		return false, nil, nil
	}

	switch targetType {
	case "player":
		targetPlayer, err := Combat.GetPlayer(instanceID, targetID)
		if err != nil {
			return false, nil, err
		}
		targetPlayer.Position.X = destination.X
		targetPlayer.Position.Y = destination.Y
		if err := Combat.UpdatePlayer(instanceID, targetPlayer); err != nil {
			return false, nil, err
		}
	case "monster":
		movedMonster := cells[oldIdx].Monster
		if movedMonster == nil {
			monster, err := Combat.GetMonster(instanceID, targetID)
			if err != nil {
				return false, nil, err
			}
			movedMonster = &game.MonsterData{
				ID:              monster.RefID,
				DBInstanceID:    monster.MonsterInstanceID,
				Name:            "",
				Type:            "monster",
				Health:          monster.Health,
				MaxHealth:       monster.MaxHealth,
				Attack:          monster.Attack,
				Defense:         monster.Defense,
				Speed:           monster.Speed,
				Maneuverability: monster.Maneuverability,
				Vision:          monster.Vision,
				Image:           monster.Image,
			}
		}
		updatedCells, err := applyKnockbackOccupancy(cells, targetType, oldIdx, newIdx, movedMonster)
		if err != nil {
			return false, nil, err
		}
		if err := repository.UpdateMatchMonsterPosition(instanceID, targetID, destination.X, destination.Y); err != nil {
			return false, nil, err
		}
		if err := repository.SaveMapCells(instanceID, cells); err != nil {
			return false, nil, err
		}
		return true, updatedCells, nil
	default:
		return false, nil, nil
	}

	updatedCells, err := applyKnockbackOccupancy(cells, targetType, oldIdx, newIdx, nil)
	if err != nil {
		return false, nil, err
	}
	if err := repository.SaveMapCells(instanceID, cells); err != nil {
		return false, nil, err
	}

	return true, updatedCells, nil
}

func resolveRangerPushFallbackDamage(
	instanceID string,
	targetType string,
	targetID int,
	attacker stats,
	target stats,
	currentHealth int,
) attackResult {
	bonusTargetStats := target
	bonusTargetStats.Health = currentHealth
	bonusTargetStats.Defense = effectiveDefense(instanceID, targetType, targetID, target.Defense)
	return applyDamage(attacker, bonusTargetStats)
}

func tryApplyMysticEnergyDrain(instanceID string, attackerID int, targetType string, targetID int) (*CombatEffect, error) {
	if targetType != "player" {
		return nil, nil
	}

	attacker, err := Combat.GetPlayer(instanceID, attackerID)
	if err != nil {
		return nil, err
	}
	target, err := Combat.GetPlayer(instanceID, targetID)
	if err != nil {
		return nil, err
	}

	drainAmount := energyDrainPerHit
	if target.Energy < drainAmount {
		drainAmount = target.Energy
	}
	gainAmount := energyDrainGainPerHit
	if available := attacker.MaxEnergy - attacker.Energy; gainAmount > available {
		gainAmount = available
	}
	if drainAmount <= 0 && gainAmount <= 0 {
		return nil, nil
	}

	if ms, ok := game.GetMatchState(instanceID); ok {
		if !ms.TryUseMysticDrain(attackerID, targetID, energyDrainPerTargetLimit) {
			return nil, nil
		}
	}

	target.Energy -= drainAmount
	attacker.Energy += gainAmount

	if err := Combat.UpdatePlayer(instanceID, target); err != nil {
		return nil, err
	}
	if err := Combat.UpdatePlayer(instanceID, attacker); err != nil {
		return nil, err
	}

	sourceRef := CombatTargetRef{ID: attackerID, Type: CombatActorPlayer}
	targetRef := CombatTargetRef{ID: targetID, Type: CombatActorPlayer}
	return &CombatEffect{
		Kind:              "energyDrain",
		Source:            &sourceRef,
		Target:            &targetRef,
		Succeeded:         true,
		EnergyGranted:     gainAmount,
		EnergyDrained:     drainAmount,
		SourceEnergyAfter: attacker.Energy,
		TargetEnergyAfter: target.Energy,
	}, nil
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
	allowCounterattack bool,
) (attackResult, error) {
	if !targetAlive || !allowCounterattack {
		return attackResult{Damage: 0, NewHealth: attackerStats.Health, Triggered: false}, nil
	}

	// Проверяем энергию для контратаки (только если defender — игрок)
	if defenderType == "player" {
		p, err := Combat.GetPlayer(instanceID, defenderID)
		if err != nil {
			return attackResult{Damage: 0, NewHealth: attackerStats.Health, Triggered: false}, nil
		}
		counterCost := standardCounterAttackEnergyCost
		if defenderStats.CharacterType == "guardian" {
			counterCost = 0
		}
		if p.Energy < counterCost {
			return attackResult{Damage: 0, NewHealth: attackerStats.Health, Triggered: false}, nil
		}
		if counterCost > 0 {
			p.Energy -= counterCost
			Combat.UpdatePlayer(instanceID, p)
		}
	}

	// Контратака происходит
	effectiveAttackerStats := attackerStats
	effectiveAttackerStats.Defense = effectiveDefense(instanceID, attackerType, attackerID, attackerStats.Defense)
	ar := applyDamage(defenderStats, effectiveAttackerStats) // defender контратакует attacker
	if ms, ok := game.GetMatchState(instanceID); ok && ar.Damage > 0 {
		ms.RecordDamageEvent(defenderID, attackerType, ar.Damage)
		if ar.NewHealth <= 0 {
			ms.RecordKillEvent(defenderID, attackerType, ar.Damage)
		}
	}

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
	mode, err := resolveAttackMode(atkStats, defStats)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	// Prevent friendly fire: if both are players and belong to same non-zero group
	if req.AttackerType == "player" && req.TargetType == "player" {
		atkP, aerr := repository.GetMatchPlayerByID(req.InstanceID, req.AttackerID)
		if aerr != nil {
			log.Printf("[DEBUG] UniversalAttackHandler: load attacker player error: %v", aerr)
			http.Error(w, "failed to load attacker player", http.StatusInternalServerError)
			return
		}
		tgtP, terr := repository.GetMatchPlayerByID(req.InstanceID, req.TargetID)
		if terr != nil {
			log.Printf("[DEBUG] UniversalAttackHandler: load target player error: %v", terr)
			http.Error(w, "failed to load target player", http.StatusInternalServerError)
			return
		}
		if atkP.GroupID != 0 && atkP.GroupID == tgtP.GroupID {
			http.Error(w, "Нельзя атаковать союзника", http.StatusBadRequest)
			return
		}
	}

	// --- ENERGY COST FOR ATTACK ---
	if req.AttackerType == "player" {
		player, err := Combat.GetPlayer(req.InstanceID, req.AttackerID)
		if err != nil {
			http.Error(w, "Ошибка загрузки игрока", http.StatusInternalServerError)
			return
		}
		attackCost := resolveAttackEnergyCost(mode)
		if player.Energy < attackCost {
			http.Error(w, "Недостаточно энергии для атаки", http.StatusBadRequest)
			return
		}
		player.Energy -= attackCost
		if err := Combat.UpdatePlayer(req.InstanceID, player); err != nil {
			http.Error(w, "Ошибка обновления энергии", http.StatusInternalServerError)
			return
		}
	}
	// -------------------------------

	attackerRef := CombatTargetRef{ID: req.AttackerID, Type: toCombatActorType(req.AttackerType)}
	targetRef := CombatTargetRef{ID: req.TargetID, Type: toCombatActorType(req.TargetType)}
	steps := make([]CombatStep, 0, 6)
	effects := make([]CombatEffect, 0, 4)
	var pushedCells []game.FullCell
	var pushedPlayerPosition *CombatPoint

	preArmorBreak := game.ArmorBreakState{}
	if ms, ok := game.GetMatchState(req.InstanceID); ok {
		preArmorBreak = ms.GetArmorBreakState(req.TargetType, req.TargetID)
	}

	effectiveTargetStats := defStats
	effectiveTargetStats.Defense = effectiveDefense(req.InstanceID, req.TargetType, req.TargetID, defStats.Defense)

	targetRes := applyDamage(atkStats, effectiveTargetStats)
	steps = append(steps, CombatStep{
		Kind:          "hit",
		Source:        &attackerRef,
		Target:        targetRef,
		Damage:        targetRes.Damage,
		TargetHPAfter: targetRes.NewHealth,
	})

	if ms, ok := game.GetMatchState(req.InstanceID); ok && targetRes.Damage > 0 {
		ms.RecordDamageEvent(req.AttackerID, req.TargetType, targetRes.Damage)
	}
	saveTargetHealth(req.InstanceID, req.TargetType, req.TargetID, req.AttackerID, req.AttackerType, targetRes)
	finalTargetHP := targetRes.NewHealth
	finalAttackerHP := atkStats.Health

	if atkStats.CharacterType == "mystic" {
		drainEffect, err := tryApplyMysticEnergyDrain(req.InstanceID, req.AttackerID, req.TargetType, req.TargetID)
		if err != nil {
			http.Error(w, "Ошибка применения Energy Drain", http.StatusInternalServerError)
			return
		}
		if drainEffect != nil {
			effects = append(effects, *drainEffect)
		}
	}

	if atkStats.CharacterType == "ranger" && mode == attackModeRanged && finalTargetHP > 0 {
		if preArmorBreak.Stacks >= armorBreakMaxStacks && targetRes.Damage > 0 {
			dx := defStats.X - atkStats.X
			if dx != 0 {
				dx /= abs(dx)
			}
			dy := defStats.Y - atkStats.Y
			if dy != 0 {
				dy /= abs(dy)
			}
			pushTo := CombatPoint{X: defStats.X + dx, Y: defStats.Y + dy}
			pushed, updatedCells, err := tryPushCombatTarget(req.InstanceID, req.TargetType, req.TargetID, defStats, pushTo)
			if err != nil {
				http.Error(w, "Ошибка применения push-эффекта", http.StatusInternalServerError)
				return
			}
			if pushed {
				pushedCells = updatedCells
				if req.TargetType == "player" {
					pushedPlayerPosition = &CombatPoint{X: pushTo.X, Y: pushTo.Y}
				}
				energyGranted := 0
				if req.AttackerType == "player" {
					attackerPlayer, err := Combat.GetPlayer(req.InstanceID, req.AttackerID)
					if err != nil {
						http.Error(w, "Ошибка обновления энергии ranger", http.StatusInternalServerError)
						return
					}
					energyGranted = baseMoveEnergyCost(attackerPlayer.Mobility)
					if available := attackerPlayer.MaxEnergy - attackerPlayer.Energy; energyGranted > available {
						energyGranted = available
					}
					attackerPlayer.Energy += energyGranted
					if err := Combat.UpdatePlayer(req.InstanceID, attackerPlayer); err != nil {
						http.Error(w, "Ошибка обновления энергии ranger", http.StatusInternalServerError)
						return
					}
				}
				effects = append(effects, CombatEffect{
					Kind:          "push",
					Source:        &attackerRef,
					Target:        &targetRef,
					Succeeded:     true,
					PositionAfter: &pushTo,
					EnergyGranted: energyGranted,
				})
			} else {
				bonusRes := resolveRangerPushFallbackDamage(
					req.InstanceID,
					req.TargetType,
					req.TargetID,
					atkStats,
					defStats,
					finalTargetHP,
				)
				if ms, ok := game.GetMatchState(req.InstanceID); ok && bonusRes.Damage > 0 {
					ms.RecordDamageEvent(req.AttackerID, req.TargetType, bonusRes.Damage)
				}
				saveTargetHealth(req.InstanceID, req.TargetType, req.TargetID, req.AttackerID, req.AttackerType, bonusRes)
				finalTargetHP = bonusRes.NewHealth
				steps = append(steps, CombatStep{
					Kind:          "bonus",
					Source:        &attackerRef,
					Target:        targetRef,
					Damage:        bonusRes.Damage,
					TargetHPAfter: bonusRes.NewHealth,
				})
				effects = append(effects, CombatEffect{
					Kind:        "push",
					Source:      &attackerRef,
					Target:      &targetRef,
					Succeeded:   false,
					BonusDamage: bonusRes.Damage,
				})
			}
		}

		if finalTargetHP > 0 {
			if ms, ok := game.GetMatchState(req.InstanceID); ok {
				armorBreak := ms.ApplyArmorBreak(req.TargetType, req.TargetID, armorBreakMaxStacks, armorBreakDurationTurns)
				effects = append(effects, CombatEffect{
					Kind:          "armorBreak",
					Source:        &attackerRef,
					Target:        &targetRef,
					Value:         -armorBreakDefensePenaltyPerStack,
					Stacks:        armorBreak.Stacks,
					DurationTurns: armorBreakDurationTurns,
					Succeeded:     true,
				})
			}
		}
	}

	counterRes, _ := doCounterattackWithEnergy(
		req.InstanceID,
		req.AttackerType, req.AttackerID,
		req.TargetType, req.TargetID,
		atkStats, defStats,
		finalTargetHP > 0,
		mode == attackModeMelee,
	)
	if counterRes.Triggered {
		finalAttackerHP = counterRes.NewHealth
		steps = append(steps, CombatStep{
			Kind:          "counter",
			Source:        &targetRef,
			Target:        attackerRef,
			Damage:        counterRes.Damage,
			TargetHPAfter: counterRes.NewHealth,
		})
	}

	if req.AttackerType == "player" &&
		atkStats.CharacterType == "berserker" &&
		mode == attackModeMelee &&
		counterRes.Triggered &&
		finalTargetHP > 0 &&
		finalAttackerHP > 0 {
		followUpDamage := targetRes.Damage / 2
		allowFollowUp := followUpDamage > 0
		if allowFollowUp {
			if ms, ok := game.GetMatchState(req.InstanceID); ok {
				allowFollowUp = ms.TryUseBerserkerFury(req.AttackerID, berserkerFollowUpLimitPerTurn)
			}
		}
		if allowFollowUp {
			followUpRes := applyFlatDamage(finalTargetHP, followUpDamage)
			if ms, ok := game.GetMatchState(req.InstanceID); ok && followUpRes.Damage > 0 {
				ms.RecordDamageEvent(req.AttackerID, req.TargetType, followUpRes.Damage)
			}
			saveTargetHealth(req.InstanceID, req.TargetType, req.TargetID, req.AttackerID, req.AttackerType, followUpRes)
			finalTargetHP = followUpRes.NewHealth
			steps = append(steps, CombatStep{
				Kind:          "followup",
				Source:        &attackerRef,
				Target:        targetRef,
				Damage:        followUpRes.Damage,
				TargetHPAfter: followUpRes.NewHealth,
			})
		}
	}

	if finalTargetHP <= 0 {
		steps = append(steps, CombatStep{
			Kind:   "death",
			Target: targetRef,
		})
	}
	if counterRes.Triggered && finalAttackerHP <= 0 {
		steps = append(steps, CombatStep{
			Kind:   "death",
			Target: attackerRef,
		})
	}

	// 8) HTTP-ответ
	resp := map[string]interface{}{
		"damage_to_target": targetRes.Damage,
		"new_target_hp":    finalTargetHP,
		"counter_damage":   counterRes.Damage,
		"new_attacker_hp":  finalAttackerHP,
		"attack_mode":      mode,
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)

	// 9) WS: COMBAT_EXCHANGE
	msg := CombatExchangeMessage{
		Type: "COMBAT_EXCHANGE",
		Payload: buildCombatExchangePayload(
			req.InstanceID,
			req.AttackerType,
			req.AttackerID,
			req.TargetType,
			req.TargetID,
			atkStats,
			mode,
			steps,
			effects,
		),
	}

	data, _ := json.Marshal(msg)
	Broadcast(data)

	if pushedPlayerPosition != nil {
		broadcastMovePlayer(req.InstanceID, req.TargetID, *pushedPlayerPosition)
	}
	if len(pushedCells) > 0 {
		broadcastUpdatedCells(req.InstanceID, pushedCells)
	}

	// 10) WS: MATCH_UPDATE — обновим статы игроков (HP, energy и т.п.)
	if req.AttackerType == "player" {
		sendUpdatePlayerWS(req.InstanceID, req.AttackerID)
	}
	if req.TargetType == "player" {
		sendUpdatePlayerWS(req.InstanceID, req.TargetID)
	}

}
