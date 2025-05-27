//===============================
// gameservice/handlers/match.go
//===============================

package handlers

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"

	"gameservice/game"
	"gameservice/models"
	"gameservice/repository"
)

// DTO для ответа на создание и получение матча
type MatchResponse struct {
	InstanceID     string                  `json:"instance_id"`
	Mode           string                  `json:"mode"`
	TeamsCount     int                     `json:"teams_count"`
	TotalPlayers   int                     `json:"total_players"`
	MapWidth       int                     `json:"map_width"`
	MapHeight      int                     `json:"map_height"`
	Map            []game.FullCell         `json:"map"`
	Players        []models.PlayerResponse `json:"players"`
	ActiveUser     int                     `json:"active_user"`
	TurnNumber     int                     `json:"turn_number"`
	StartPositions [][2]int                `json:"start_positions"`
	PortalPosition [2]int                  `json:"portal_position"`
	Winner         *models.WinnerInfo      `json:"winner,omitempty"`
}

type RequestMatch struct {
	InstanceID   string `json:"instance_id"`
	Mode         string `json:"mode"`
	PlayerIDs    []int  `json:"player_ids"`
	TeamsCount   int    `json:"teams_count"`
	TotalPlayers int    `json:"total_players"`
}

func handleError(w http.ResponseWriter, context string, err error) {
	log.Printf("%s: %v", context, err)
	http.Error(w, "Internal server error", http.StatusInternalServerError)
}

func toJSON(v interface{}) []byte {
	b, _ := json.Marshal(v)
	return b
}

// insertMatchMonsters сохраняет всех монстров на карте в БД.
func insertMatchMonsters(instanceID string, cells []game.FullCell) error {
	for _, cell := range cells {
		if cell.Monster == nil {
			continue
		}
		md := cell.Monster
		mm := repository.MatchMonster{
			InstanceID:      instanceID,
			RefID:           md.ID / 1_000_000,
			X:               cell.X,
			Y:               cell.Y,
			Health:          md.Health,
			MaxHealth:       md.MaxHealth,
			Attack:          md.Attack,
			Defense:         md.Defense,
			Speed:           md.Speed,
			Maneuverability: md.Maneuverability,
			Vision:          md.Vision,
			Image:           md.Image,
		}
		if err := repository.InsertMatchMonster(mm); err != nil {
			return fmt.Errorf("insert monster at (%d,%d): %w", cell.X, cell.Y, err)
		}
	}
	return nil
}

// assignMatchPlayers создаёт копии игроков в матче с их стартовыми позициями.
// Если для какого-то игрока не получится получить данные или вставить — логируем и продолжаем.
func assignMatchPlayers(instanceID string, playerIDs []int, starts [][2]int) {
	// Инициализируем локальное состояние матча
	game.CreateMatchState(instanceID, playerIDs)

	for i, uid := range playerIDs {
		p, err := repository.GetPlayerByUserID(uid)
		if err != nil {
			log.Printf("[assignMatchPlayers] player %d not found: %v", uid, err)
			continue
		}

		// выбираем позицию
		var x, y int
		if i < len(starts) {
			x, y = starts[i][0], starts[i][1]
		} else {
			x, y = starts[0][0], starts[0][1]
		}

		// групповая логика (TODO: если нужно различать команды, здесь можно считать groupID из i)
		const groupID = 1

		if err := repository.CreateMatchPlayerCopy(instanceID, p, x, y, groupID); err != nil {
			log.Printf("[assignMatchPlayers] failed to insert player %d: %v", uid, err)
		}
	}
}

func CreateMatchHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Only POST allowed", http.StatusMethodNotAllowed)
		return
	}

	// 2) Всегда JSON
	w.Header().Set("Content-Type", "application/json")

	// 3) Распарсим тело
	var req RequestMatch
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Bad request", http.StatusBadRequest)
		return
	}

	// 4) Валидация
	if req.InstanceID == "" || req.Mode == "" {
		http.Error(w, "instance_id и mode обязательны", http.StatusBadRequest)
		return
	}
	if len(req.PlayerIDs) != req.TotalPlayers {
		http.Error(w, "Неверное количество player_ids", http.StatusBadRequest)
		return
	}

	cfg := game.MapConfig{
		TotalPlayers: req.TotalPlayers,
		TeamsCount:   req.TeamsCount,
		WalkableProb: 0.8,
		BarbelProb:   0.1,
		ResourceProb: 0.1,
		MonsterProb:  0.05,
	}

	// Получаем ресурсы и монстров из БД
	resourcesFromDB, err := repository.GetResourcesData() // []game.ResourceData
	if err != nil {
		handleError(w, "[CreateMatch] Ошибка загрузки ресурсов", err)
		return
	}
	monstersFromDB, err := repository.GetMonstersData() // []game.MonsterData
	if err != nil {
		handleError(w, "[CreateMatch] GetMonstersData failed", err)
		return
	}

	// Генерируем полную карту
	fullMap, mapWidth, mapHeight, startPositions, portalPos, err := game.GenerateFullMap(cfg, resourcesFromDB, monstersFromDB)
	if err != nil {
		handleError(w, "[CreateMatch] Ошибка генерации карты", err)
		return
	}

	// Преобразуем стартовые позиции и позицию портала в JSON
	mapJSON := toJSON(fullMap)
	turnOrderJSON := toJSON(req.PlayerIDs[:1]) // первый игрок в очереди
	startPosJSON := toJSON(startPositions)
	portalPosJSON := toJSON(portalPos)

	// Вставляем матч в таблицу matches.
	// turn_number устанавливаем равным 1.

	_, err = repository.DB.Exec(`
		INSERT INTO matches (
			instance_id, mode, teams_count, total_players,
			map_width, map_height, map, active_user_id,
			turn_order, turn_number, start_positions, portal_position
		) VALUES (
			$1, $2, $3, $4,
			$5, $6, $7, $8,
			$9, 1, $10, $11
		)
		ON CONFLICT (instance_id) DO NOTHING
	`,
		req.InstanceID,
		req.Mode,
		req.TeamsCount,
		req.TotalPlayers,
		mapWidth,
		mapHeight,
		mapJSON, // []byte JSON
		req.PlayerIDs[0],
		turnOrderJSON, // []byte JSON
		startPosJSON,  // []byte JSON
		portalPosJSON, // []byte JSON
	)
	if err != nil {
		handleError(w, "[CreateMatch] Insert match failed", err)
		return
	}

	//  вставляем всех монстров в match_monsters
	if err := insertMatchMonsters(req.InstanceID, fullMap); err != nil {
		handleError(w, "[CreateMatch] failed to insert match_monsters", err)
		return
	}

	// Создаем локальное состояние матча.

	assignMatchPlayers(req.InstanceID, req.PlayerIDs, startPositions)

	playersInMatch, err := repository.GetPlayersInMatch(req.InstanceID)
	if err != nil {
		handleError(w, "[CreateMatch] GetPlayersInMatch failed", err)
		return
	}

	// Обновляем состояние матча: активный игрок – первый игрок, turn_number = 1.
	// Обновляем состояние матча: активный игрок – первый игрок, turn_number = 1.
	if err := repository.UpdateMatchTurn(req.InstanceID, req.PlayerIDs[0], 1); err != nil {
		log.Printf("[CreateMatch] UpdateMatchTurn failed: %v", err)
	}

	// Собираем ответ
	resp := MatchResponse{
		InstanceID:     req.InstanceID,
		Mode:           req.Mode,
		TeamsCount:     req.TeamsCount,
		TotalPlayers:   req.TotalPlayers,
		MapWidth:       mapWidth,
		MapHeight:      mapHeight,
		Map:            fullMap,
		Players:        playersInMatch,
		ActiveUser:     req.PlayerIDs[0],
		TurnNumber:     1,
		StartPositions: startPositions,
		PortalPosition: portalPos,
		// Winner остаётся nil при создании
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated) // 201 Created
	if err := json.NewEncoder(w).Encode(resp); err != nil {
		log.Printf("[CreateMatch] Encode response failed: %v", err)
	}
}

func GetMatchHandler(w http.ResponseWriter, r *http.Request) {
	instanceID := r.URL.Query().Get("instance_id")
	if instanceID == "" {
		http.Error(w, "instance_id обязателен", http.StatusBadRequest)
		return
	}

	// 1) Забираем из БД
	match, err := repository.GetMatchByID(instanceID)
	if err != nil {
		http.Error(w, fmt.Sprintf("Матч не найден: %v", err), http.StatusNotFound)
		return
	}
	players, err := repository.GetPlayersInMatch(instanceID)
	if err != nil {
		http.Error(w, fmt.Sprintf("Ошибка получения игроков: %v", err), http.StatusInternalServerError)
		return
	}

	// 2) Распарсим JSON-поля
	var fullMap []game.FullCell
	if err := json.Unmarshal(match.Map, &fullMap); err != nil {
		http.Error(w, "Ошибка разбора поля map", http.StatusInternalServerError)
		return
	}
	var startPositions [][2]int
	if err := json.Unmarshal(match.StartPositions, &startPositions); err != nil {
		http.Error(w, "Ошибка разбора поля start_positions", http.StatusInternalServerError)
		return
	}
	var portalPos [2]int
	if err := json.Unmarshal(match.PortalPosition, &portalPos); err != nil {
		http.Error(w, "Ошибка разбора поля portal_position", http.StatusInternalServerError)
		return
	}

	// 3) Вычисляем active_user
	activeUser := match.ActiveUserID
	if activeUser == 0 && len(players) > 0 {
		activeUser = players[0].UserID
	}

	// 4) Собираем ответ
	resp := MatchResponse{
		InstanceID:     match.InstanceID,
		Mode:           match.Mode,
		TeamsCount:     match.TeamsCount,
		TotalPlayers:   match.TotalPlayers,
		MapWidth:       match.MapWidth,
		MapHeight:      match.MapHeight,
		Map:            fullMap,
		Players:        players,
		ActiveUser:     activeUser,
		TurnNumber:     match.TurnNumber,
		StartPositions: startPositions,
		PortalPosition: portalPos,
		// Winner оставим nil — он нужен только после финализации
	}

	// 5) Отвечаем
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(resp); err != nil {
		log.Printf("[GetMatch] Encode response failed: %v", err)
	}
}
