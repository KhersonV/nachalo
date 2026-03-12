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
	dbID, err := repository.InsertMatchMonsterReturningID(mm)
if err != nil {
    return fmt.Errorf("insert monster at (%d,%d): %w", cell.X, cell.Y, err)
}
cell.Monster.DBInstanceID = dbID // <- запишем id в клетку!
	}
	return nil
}



// Сборка полного MatchResponse по instanceID. Возвращает структуру и ошибку.
func BuildMatchResponse(instanceID string) (*MatchResponse, error) {
	match, err := repository.GetMatchByID(instanceID)
	if err != nil {
		return nil, err
	}
	players, err := repository.GetPlayersInMatch(instanceID)
	if err != nil {
		return nil, err
	}

	var fullMap []game.FullCell
	if err := json.Unmarshal(match.Map, &fullMap); err != nil {
		return nil, err
	}
	var startPositions [][2]int
	if err := json.Unmarshal(match.StartPositions, &startPositions); err != nil {
		return nil, err
	}
	var portalPos [2]int
	if err := json.Unmarshal(match.PortalPosition, &portalPos); err != nil {
		return nil, err
	}
	activeUser := match.ActiveUserID
	if activeUser == 0 && len(players) > 0 {
		activeUser = players[0].UserID
	}
	resp := &MatchResponse{
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
	}
	return resp, nil
}



// assignMatchPlayers создаёт копии игроков в матче с их стартовыми позициями.
// Если для какого-то игрока не получится получить данные или вставить — логируем и продолжаем.
// assignMatchPlayers – распределяет игроков по группам в зависимости от режима игры и вызывает создание копий игроков для матча.
func assignMatchPlayers(
	instanceID string,
	playerIDs []int,
	starts [][2]int,
	mode string,
) error {
	// Определяем число команд
	teamsCount := 1
	switch mode {
	case "pve":
		teamsCount = 1
	case "1x1":
		teamsCount = 2
	case "1x2":
		teamsCount = 3
	case "2x2":
		teamsCount = 2
	case "3x3":
		teamsCount = 2
	case "5x5":
		teamsCount = 2
	default:
		teamsCount = 2 // по умолчанию 2 команды
	}
	playersPerTeam := len(playerIDs) / teamsCount
	if playersPerTeam == 0 {
		playersPerTeam = 1
	}

	game.CreateMatchState(instanceID, playerIDs)

	for i, uid := range playerIDs {
		p, err := repository.GetPlayerByUserID(uid)
		if err != nil {
			return fmt.Errorf("player %d not found in players: %w", uid, err)
		}
		var x, y int
		if i < len(starts) {
			x, y = starts[i][0], starts[i][1]
		} else {
			x, y = starts[0][0], starts[0][1]
		}
		groupID := (i / playersPerTeam) + 1
		if err := repository.CreateMatchPlayerCopy(instanceID, p, x, y, groupID); err != nil {
			return fmt.Errorf("failed to insert player %d: %w", uid, err)
		}
	}

	return nil
}


func ConvertToMonsterState(md *game.MonsterData, id int) *game.MonsterState {
	return &game.MonsterState{
		ID:                md.ID,
		MonsterInstanceID: id,
		Health:            md.Health,
		MaxHealth:         md.MaxHealth,
		Attack:            md.Attack,
		Defense:           md.Defense,
		Speed:             md.Speed,
		Maneuverability:   md.Maneuverability,
		Vision:            md.Vision,
		Image:             md.Image,
	}
}



func GetMatchHandler(w http.ResponseWriter, r *http.Request) {
    instanceID := r.URL.Query().Get("instance_id")
    if instanceID == "" {
        http.Error(w, "instance_id обязателен", http.StatusBadRequest)
        return
    }
    resp, err := BuildMatchResponse(instanceID)
    if err != nil {
        http.Error(w, fmt.Sprintf("Ошибка: %v", err), http.StatusInternalServerError)
        return
    }
    w.Header().Set("Content-Type", "application/json")
    _ = json.NewEncoder(w).Encode(resp)
}




func CreateMatchHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Only POST allowed", http.StatusMethodNotAllowed)
		return
	}

	w.Header().Set("Content-Type", "application/json")

	// 1. Распарсим тело
	var req RequestMatch
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Bad request", http.StatusBadRequest)
		return
	}

	// 2. Валидация
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

	// 3. Получаем ресурсы и монстров из БД
	resourcesFromDB, err := repository.GetResourcesData()
	if err != nil {
		handleError(w, "[CreateMatch] Ошибка загрузки ресурсов", err)
		return
	}
	monstersFromDB, err := repository.GetMonstersData()
	if err != nil {
		handleError(w, "[CreateMatch] GetMonstersData failed", err)
		return
	}

	// 4. Генерируем полную карту
	fullMap, mapWidth, mapHeight, startPositions, portalPos, err := game.GenerateFullMap(cfg, resourcesFromDB, monstersFromDB)
	if err != nil {
		handleError(w, "[CreateMatch] Ошибка генерации карты", err)
		return
	}

	// 5. Теперь сразу сериализуем карту — пока без db_instance_id монстров!
	mapJSON := toJSON(fullMap)
	turnOrderJSON := toJSON(req.PlayerIDs[:1])
	startPosJSON := toJSON(startPositions)
	portalPosJSON := toJSON(portalPos)

	// 6. Сначала вставляем матч — он нужен для внешнего ключа монстров!
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
		mapJSON, // Пока без db_instance_id
		req.PlayerIDs[0],
		turnOrderJSON,
		startPosJSON,
		portalPosJSON,
	)
	if err != nil {
		handleError(w, "[CreateMatch] Insert match failed", err)
		return
	}

	// 7. Теперь можем вставить монстров и уже записать db_instance_id для каждого в fullMap!
	if err := insertMatchMonsters(req.InstanceID, fullMap); err != nil {
		handleError(w, "[CreateMatch] failed to insert match_monsters", err)
		return
	}

	// 8. Теперь пересохраняем map уже с заполненными db_instance_id монстров
	mapJSON = toJSON(fullMap)
	_, err = repository.DB.Exec(
		`UPDATE matches SET map=$1 WHERE instance_id=$2`,
		mapJSON, req.InstanceID,
	)
	if err != nil {
		handleError(w, "[CreateMatch] failed to update map with monster db_instance_id", err)
		return
	}
// 9. Копируем игроков в матч
if err := assignMatchPlayers(req.InstanceID, req.PlayerIDs, startPositions, req.Mode); err != nil {
	_, _ = repository.DB.Exec(`DELETE FROM matches WHERE instance_id = $1`, req.InstanceID)
	handleError(w, "[CreateMatch] failed to assign players", err)
	return
}


// 10. Обновляем состояние матча: активный игрок – первый, turn_number = 1.
if err := repository.UpdateMatchTurn(req.InstanceID, req.PlayerIDs[0], 1); err != nil {
	log.Printf("[CreateMatch] UpdateMatchTurn failed: %v", err)
}

// 11. Теперь получаем список игроков для ответа (они уже точно есть!)
playersInMatch, err := repository.GetPlayersInMatch(req.InstanceID)
if err != nil {
	handleError(w, "[CreateMatch] GetPlayersInMatch failed", err)
	return
}

// 12. Собираем и отдаём JSON-ответ
resp := MatchResponse{
	InstanceID:     req.InstanceID,
	Mode:           req.Mode,
	TeamsCount:     req.TeamsCount,
	TotalPlayers:   req.TotalPlayers,
	MapWidth:       mapWidth,
	MapHeight:      mapHeight,
	Map:            fullMap,
	Players:        playersInMatch, // <-- теперь не пусто!
	ActiveUser:     req.PlayerIDs[0],
	TurnNumber:     1,
	StartPositions: startPositions,
	PortalPosition: portalPos,
}
w.WriteHeader(http.StatusCreated)
if err := json.NewEncoder(w).Encode(resp); err != nil {
	log.Printf("[CreateMatch] Encode response failed: %v", err)
}
}


