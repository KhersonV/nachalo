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
	"gameservice/repository"
)

type RequestMatch struct {
	InstanceID   string `json:"instance_id"`
	Mode         string `json:"mode"`
	PlayerIDs    []int  `json:"player_ids"`
	TeamsCount   int    `json:"teams_count"`
	TotalPlayers int    `json:"total_players"`
}

func CreateMatchHandler(w http.ResponseWriter, r *http.Request) {
	var req RequestMatch
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Bad request", http.StatusBadRequest)
		return
	}

	cfg := game.MapConfig{
		TotalPlayers: req.TotalPlayers,
		TeamsCount:   req.TeamsCount,
		WalkableProb: 0.8,
		BarbelProb: 0.1,
		ResourceProb: 0.1,
		MonsterProb:  0.05,
	}

	// Получаем ресурсы и монстров из БД
	resourcesFromDB, err := repository.GetResourcesData() // []game.ResourceData
	if err != nil {
		http.Error(w, "Ошибка загрузки ресурсов", http.StatusInternalServerError)
		return
	}
	monstersFromDB, err := repository.GetMonstersData() // []game.MonsterData
	if err != nil {
		http.Error(w, "Ошибка загрузки монстров", http.StatusInternalServerError)
		return
	}

	// Генерируем полную карту
	fullMap, mapWidth, mapHeight, startPositions, portalPos, err := game.GenerateFullMap(cfg, resourcesFromDB, monstersFromDB)
	if err != nil {
		log.Printf("Ошибка генерации полной карты: %v", err)
		http.Error(w, "Ошибка генерации карты", http.StatusInternalServerError)
		return
	}

	// Преобразуем стартовые позиции и позицию портала в JSON
	startPosJSON, err := json.Marshal(startPositions)
	if err != nil {
		http.Error(w, "Ошибка маршалинга стартовых позиций", http.StatusInternalServerError)
		return
	}
	portalPosJSON, err := json.Marshal(portalPos)
	if err != nil {
		http.Error(w, "Ошибка маршалинга позиции портала", http.StatusInternalServerError)
		return
	}

	mapData, err := json.Marshal(fullMap)
	if err != nil {
		http.Error(w, "Ошибка маршалинга карты", http.StatusInternalServerError)
		return
	}

	// Вставляем матч в таблицу matches.
	// turn_number устанавливаем равным 1.
	_, err = repository.DB.Exec(`
        INSERT INTO matches (
            instance_id, mode, teams_count, total_players, map_width, map_height, map, 
            active_user_id, turn_order, turn_number, start_positions, portal_position
        ) VALUES (
            $1, $2, $3, $4, $5, $6, $7, $8, $9, 1, $10, $11
        )`,
		req.InstanceID,
		req.Mode,
		req.TeamsCount,
		req.TotalPlayers,
		mapWidth,
		mapHeight,
		string(mapData),
		req.PlayerIDs[0],
		fmt.Sprintf("[%d]", req.PlayerIDs[0]),
		string(startPosJSON),
		string(portalPosJSON),
	)
	if err != nil {
		log.Printf("Ошибка вставки матча: %v", err)
		http.Error(w, "Ошибка сохранения матча", http.StatusInternalServerError)
		return
	}



	//  вставляем всех монстров в match_monsters
    for _, cell := range fullMap {
        if cell.Monster != nil {
            md := cell.Monster
            err := repository.InsertMatchMonster(repository.MatchMonster{
                InstanceID:      req.InstanceID,
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
            })
            if err != nil {
                log.Printf("failed to insert match_monster: %v", err)
                http.Error(w, "internal error", http.StatusInternalServerError)
                return
            }
        }
    }


	// Создаем локальное состояние матча.
	game.CreateMatchState(req.InstanceID, req.PlayerIDs)

	// Распределяем игроков по стартовым позициям.
	for i, playerID := range req.PlayerIDs {
		player, err := repository.GetPlayerByUserID(playerID)
		if err != nil {
			log.Printf("Игрок с ID %d не найден: %v", playerID, err)
			continue
		}

		var pos [2]int
		// Распределяем стартовые позиции по очереди.
		if i < len(startPositions) {
			pos = startPositions[i]
		} else {
			pos = startPositions[0]
		}

		groupID := 1
		err = repository.CreateMatchPlayerCopy(req.InstanceID, player, pos[0], pos[1], groupID)
		if err != nil {
			log.Printf("Ошибка вставки игрока в матч: %v", err)
			continue
		}
	}

	playersInMatch, err := repository.GetPlayersInMatch(req.InstanceID)
	if err != nil {
		log.Printf("Ошибка получения игроков после вставки: %v", err)
		http.Error(w, "Ошибка получения игроков", http.StatusInternalServerError)
		return
	}

	// Обновляем состояние матча: активный игрок – первый игрок, turn_number = 1.
	err = repository.UpdateMatchTurn(req.InstanceID, req.PlayerIDs[0], 1)
	if err != nil {
		log.Printf("Ошибка обновления состояния матча: %v", err)
	}

	response := map[string]interface{}{
		"instance_id":     req.InstanceID,
		"mode":            req.Mode,
		"map":             fullMap,
		"map_width":       mapWidth,
		"map_height":      mapHeight,
		"player_ids":      req.PlayerIDs,
		"players":         playersInMatch,
		"start_positions": string(startPosJSON),
		"portal_position": string(portalPosJSON),
	}
	
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}


func GetMatchHandler(w http.ResponseWriter, r *http.Request) {
	instanceID := r.URL.Query().Get("instance_id")
	if instanceID == "" {
		http.Error(w, "instance_id обязателен", http.StatusBadRequest)
		return
	}

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

	activeUser := match.ActiveUserID
	if activeUser == 0 && len(players) > 0 {
		activeUser = players[0].UserID
	}

	response := map[string]interface{}{
		"instance_id":     match.InstanceID,
		"mode":            match.Mode,
		"teams_count":     match.TeamsCount,
		"total_players":   match.TotalPlayers,
		"map_width":       match.MapWidth,
		"map_height":      match.MapHeight,
		"map":             match.Map, // сохранённый массив FullCell (JSON)
		"players":         players,
		"active_user":   activeUser,
		"turn_number":     match.TurnNumber,
		"start_positions": match.StartPositions, // если у вас в модели MatchInfo есть это поле
		"portal_position": match.PortalPosition,   // аналогично
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}
