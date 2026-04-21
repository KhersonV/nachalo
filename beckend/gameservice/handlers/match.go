//===============================
// gameservice/handlers/match.go
//===============================

package handlers

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"

	"gameservice/game"
	"gameservice/models"
	"gameservice/repository"
)

// DTO для ответа на создание и получение матча
type MatchResponse struct {
	InstanceID               string                  `json:"instance_id"`
	Mode                     string                  `json:"mode"`
	TeamsCount               int                     `json:"teams_count"`
	TotalPlayers             int                     `json:"total_players"`
	MapWidth                 int                     `json:"map_width"`
	MapHeight                int                     `json:"map_height"`
	Map                      []game.FullCell         `json:"map"`
	Players                  []models.PlayerResponse `json:"players"`
	ActiveUser               int                     `json:"active_user"`
	TurnNumber               int                     `json:"turn_number"`
	StartPositions           [][2]int                `json:"start_positions"`
	PortalPosition           [2]int                  `json:"portal_position"`
	Winner                   *models.WinnerInfo      `json:"winner,omitempty"`
	QuestArtifactID          int                     `json:"quest_artifact_id"`
	QuestArtifactName        string                  `json:"quest_artifact_name"`
	QuestArtifactImage       string                  `json:"quest_artifact_image"`
	QuestArtifactDescription string                  `json:"quest_artifact_description"`
}

type RequestMatchTeam struct {
	TeamID    int   `json:"team_id"`
	MemberIDs []int `json:"member_ids"`
}

type RequestMatch struct {
	InstanceID   string             `json:"instance_id"`
	Mode         string             `json:"mode"`
	PlayerIDs    []int              `json:"player_ids"`
	GroupIDs     []int              `json:"group_ids,omitempty"`
	Teams        []RequestMatchTeam `json:"teams,omitempty"`
	TurnOrder    []int              `json:"turn_order,omitempty"`
	TeamsCount   int                `json:"teams_count"`
	TotalPlayers int                `json:"total_players"`
}

type normalizedMatchPlan struct {
	Teams         []RequestMatchTeam
	PlayerIDs     []int
	TurnOrder     []int
	GroupByPlayer map[int]int
}

func handleError(w http.ResponseWriter, context string, err error) {
	log.Printf("%s: %v", context, err)
	http.Error(w, "Internal server error", http.StatusInternalServerError)
}

func toJSON(v interface{}) []byte {
	b, _ := json.Marshal(v)
	return b
}

func copyInts(ids []int) []int {
	if len(ids) == 0 {
		return []int{}
	}
	result := make([]int, len(ids))
	copy(result, ids)
	return result
}

func equalIntSlices(left []int, right []int) bool {
	if len(left) != len(right) {
		return false
	}
	for idx := range left {
		if left[idx] != right[idx] {
			return false
		}
	}
	return true
}

func samePlayerSet(left []int, right []int) bool {
	if len(left) != len(right) {
		return false
	}

	counts := make(map[int]int, len(left))
	for _, id := range left {
		counts[id]++
	}
	for _, id := range right {
		counts[id]--
		if counts[id] < 0 {
			return false
		}
	}
	for _, count := range counts {
		if count != 0 {
			return false
		}
	}
	return true
}

func flattenRequestTeams(teams []RequestMatchTeam) []int {
	totalPlayers := 0
	for _, team := range teams {
		totalPlayers += len(team.MemberIDs)
	}

	playerIDs := make([]int, 0, totalPlayers)
	for _, team := range teams {
		playerIDs = append(playerIDs, team.MemberIDs...)
	}
	return playerIDs
}

func buildTurnOrderFromTeams(teams []RequestMatchTeam) []int {
	maxTeamSize := 0
	totalPlayers := 0
	for _, team := range teams {
		totalPlayers += len(team.MemberIDs)
		if len(team.MemberIDs) > maxTeamSize {
			maxTeamSize = len(team.MemberIDs)
		}
	}

	turnOrder := make([]int, 0, totalPlayers)
	for memberIdx := 0; memberIdx < maxTeamSize; memberIdx++ {
		for _, team := range teams {
			if memberIdx < len(team.MemberIDs) {
				turnOrder = append(turnOrder, team.MemberIDs[memberIdx])
			}
		}
	}
	return turnOrder
}

func buildGroupByPlayer(teams []RequestMatchTeam) map[int]int {
	groupByPlayer := make(map[int]int)
	for _, team := range teams {
		for _, memberID := range team.MemberIDs {
			groupByPlayer[memberID] = team.TeamID
		}
	}
	return groupByPlayer
}

func normalizeExplicitTeams(teams []RequestMatchTeam) ([]RequestMatchTeam, error) {
	if len(teams) == 0 {
		return nil, nil
	}

	normalized := make([]RequestMatchTeam, 0, len(teams))
	seenTeamIDs := make(map[int]bool, len(teams))
	seenPlayers := make(map[int]bool)

	for _, team := range teams {
		if team.TeamID <= 0 {
			return nil, fmt.Errorf("team_id must be positive")
		}
		if seenTeamIDs[team.TeamID] {
			return nil, fmt.Errorf("duplicate team_id %d", team.TeamID)
		}
		if len(team.MemberIDs) == 0 {
			return nil, fmt.Errorf("team %d has no members", team.TeamID)
		}

		memberIDs := make([]int, 0, len(team.MemberIDs))
		for _, memberID := range team.MemberIDs {
			if memberID <= 0 {
				return nil, fmt.Errorf("team %d contains invalid player id", team.TeamID)
			}
			if seenPlayers[memberID] {
				return nil, fmt.Errorf("player %d is present in multiple teams", memberID)
			}
			seenPlayers[memberID] = true
			memberIDs = append(memberIDs, memberID)
		}

		seenTeamIDs[team.TeamID] = true
		normalized = append(normalized, RequestMatchTeam{
			TeamID:    team.TeamID,
			MemberIDs: memberIDs,
		})
	}

	for teamIdx := 0; teamIdx < len(normalized)-1; teamIdx++ {
		for nextIdx := teamIdx + 1; nextIdx < len(normalized); nextIdx++ {
			if normalized[nextIdx].TeamID < normalized[teamIdx].TeamID {
				normalized[teamIdx], normalized[nextIdx] = normalized[nextIdx], normalized[teamIdx]
			}
		}
	}

	return normalized, nil
}

func buildTeamsFromGroupIDs(playerIDs []int, groupIDs []int) ([]RequestMatchTeam, error) {
	if len(groupIDs) != len(playerIDs) {
		return nil, fmt.Errorf("group_ids must align with player_ids")
	}

	teamsByID := make(map[int][]int)
	teamOrder := make([]int, 0)
	seenPlayers := make(map[int]bool, len(playerIDs))

	for idx, playerID := range playerIDs {
		if playerID <= 0 {
			return nil, fmt.Errorf("invalid player id %d", playerID)
		}
		if seenPlayers[playerID] {
			return nil, fmt.Errorf("duplicate player id %d", playerID)
		}
		seenPlayers[playerID] = true

		groupID := groupIDs[idx]
		if groupID <= 0 {
			return nil, fmt.Errorf("group_ids must be positive")
		}
		if _, exists := teamsByID[groupID]; !exists {
			teamOrder = append(teamOrder, groupID)
		}
		teamsByID[groupID] = append(teamsByID[groupID], playerID)
	}

	for idx := 0; idx < len(teamOrder)-1; idx++ {
		for nextIdx := idx + 1; nextIdx < len(teamOrder); nextIdx++ {
			if teamOrder[nextIdx] < teamOrder[idx] {
				teamOrder[idx], teamOrder[nextIdx] = teamOrder[nextIdx], teamOrder[idx]
			}
		}
	}

	teams := make([]RequestMatchTeam, 0, len(teamOrder))
	for _, groupID := range teamOrder {
		teams = append(teams, RequestMatchTeam{
			TeamID:    groupID,
			MemberIDs: copyInts(teamsByID[groupID]),
		})
	}
	return teams, nil
}

func fallbackTeamsForMode(playerIDs []int, teamsCount int) ([]RequestMatchTeam, error) {
	if teamsCount <= 0 {
		return nil, fmt.Errorf("teams_count must be positive")
	}
	if len(playerIDs) == 0 {
		return nil, fmt.Errorf("player_ids must not be empty")
	}

	seenPlayers := make(map[int]bool, len(playerIDs))
	for _, playerID := range playerIDs {
		if playerID <= 0 {
			return nil, fmt.Errorf("invalid player id %d", playerID)
		}
		if seenPlayers[playerID] {
			return nil, fmt.Errorf("duplicate player id %d", playerID)
		}
		seenPlayers[playerID] = true
	}

	switch {
	case teamsCount == 1:
		return []RequestMatchTeam{{
			TeamID:    1,
			MemberIDs: copyInts(playerIDs),
		}}, nil
	case teamsCount == len(playerIDs):
		teams := make([]RequestMatchTeam, 0, len(playerIDs))
		for idx, playerID := range playerIDs {
			teams = append(teams, RequestMatchTeam{
				TeamID:    idx + 1,
				MemberIDs: []int{playerID},
			})
		}
		return teams, nil
	default:
		return nil, fmt.Errorf("explicit teams are required for team modes")
	}
}

func normalizeMatchPlan(req RequestMatch) (*normalizedMatchPlan, error) {
	var teams []RequestMatchTeam
	var err error

	switch {
	case len(req.Teams) > 0:
		teams, err = normalizeExplicitTeams(req.Teams)
	case len(req.GroupIDs) > 0:
		teams, err = buildTeamsFromGroupIDs(req.PlayerIDs, req.GroupIDs)
	default:
		teams, err = fallbackTeamsForMode(req.PlayerIDs, req.TeamsCount)
	}
	if err != nil {
		return nil, err
	}

	playerIDs := flattenRequestTeams(teams)
	if !samePlayerSet(req.PlayerIDs, playerIDs) {
		return nil, fmt.Errorf("teams do not match player_ids")
	}

	if req.TeamsCount > 0 && len(teams) != req.TeamsCount {
		return nil, fmt.Errorf("teams_count does not match provided teams")
	}

	turnOrder := buildTurnOrderFromTeams(teams)
	if len(req.TurnOrder) > 0 && !equalIntSlices(req.TurnOrder, turnOrder) {
		return nil, fmt.Errorf("turn_order does not match team rotation")
	}

	return &normalizedMatchPlan{
		Teams:         teams,
		PlayerIDs:     playerIDs,
		TurnOrder:     turnOrder,
		GroupByPlayer: buildGroupByPlayer(teams),
	}, nil
}

func mapStartPositionsToPlayers(teams []RequestMatchTeam, starts [][2]int) (map[int][2]int, error) {
	playerCount := len(flattenRequestTeams(teams))
	if len(starts) != playerCount {
		return nil, fmt.Errorf("expected %d start positions, got %d", playerCount, len(starts))
	}

	startByPlayer := make(map[int][2]int, playerCount)
	startIdx := 0
	for _, team := range teams {
		if startIdx+len(team.MemberIDs) > len(starts) {
			return nil, fmt.Errorf("not enough start positions for team %d", team.TeamID)
		}
		for _, memberID := range team.MemberIDs {
			startByPlayer[memberID] = starts[startIdx]
			startIdx++
		}
	}

	return startByPlayer, nil
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

	// Safety-net: старые/конфликтные матчи могли сохраниться без quest_artifact_id.
	if match.QuestArtifactID <= 0 {
		if qa, qerr := repository.GetRandomArtifactFromCatalog(); qerr == nil {
			if _, uerr := repository.DB.Exec(
				`UPDATE matches SET quest_artifact_id = $1 WHERE instance_id = $2`,
				qa.ID,
				instanceID,
			); uerr != nil {
				log.Printf("[BuildMatchResponse] failed to backfill quest_artifact_id for %s: %v", instanceID, uerr)
			} else {
				match.QuestArtifactID = qa.ID
			}
		} else {
			log.Printf("[BuildMatchResponse] failed to get random quest artifact for %s: %v", instanceID, qerr)
		}
	}

	players, err := repository.GetPlayersInMatch(instanceID)
	if err != nil {
		return nil, err
	}

	// Поправим порядок игроков по серверному turn_order (если он задан),
	// чтобы фронтенд видел игроков в той же последовательности, что и сервер.
	if len(match.TurnOrder) > 0 {
		var turnOrderIDs []int
		if err := json.Unmarshal(match.TurnOrder, &turnOrderIDs); err == nil {
			byID := make(map[int]models.PlayerResponse, len(players))
			for _, p := range players {
				byID[p.UserID] = p
			}
			ordered := make([]models.PlayerResponse, 0, len(players))
			used := make(map[int]bool)
			for _, uid := range turnOrderIDs {
				if pl, ok := byID[uid]; ok {
					ordered = append(ordered, pl)
					used[uid] = true
				}
			}
			for _, p := range players {
				if !used[p.UserID] {
					ordered = append(ordered, p)
				}
			}
			players = ordered
		} else {
			log.Printf("[BuildMatchResponse] failed to parse turn_order for %s: %v", instanceID, err)
		}
	}

	fullMap, err := repository.LoadMapCells(instanceID)
	if err != nil {
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
	if match.QuestArtifactID > 0 {
		if qa, err := repository.GetArtifactFromCatalogByID(match.QuestArtifactID); err == nil {
			resp.QuestArtifactID = qa.ID
			resp.QuestArtifactName = qa.Name
			resp.QuestArtifactImage = qa.Image
			resp.QuestArtifactDescription = qa.Description
		}
	}
	return resp, nil
}

// assignMatchPlayers создаёт копии игроков в матче, используя явные команды
// и уже рассчитанные стартовые позиции для каждого игрока.
func assignMatchPlayers(
	instanceID string,
	teams []RequestMatchTeam,
	startByPlayer map[int][2]int,
) error {
	for _, team := range teams {
		for _, uid := range team.MemberIDs {
			p, err := repository.GetPlayerByUserID(uid)
			if err != nil {
				return fmt.Errorf("player %d not found in players: %w", uid, err)
			}

			start, ok := startByPlayer[uid]
			if !ok {
				return fmt.Errorf("missing start position for player %d", uid)
			}

			if err := repository.CreateMatchPlayerCopy(instanceID, p, start[0], start[1], team.TeamID); err != nil {
				return fmt.Errorf("failed to insert player %d: %w", uid, err)
			}
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
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]string{"error": "instance_id обязателен"})
		return
	}
	resp, err := BuildMatchResponse(instanceID)
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		if errors.Is(err, sql.ErrNoRows) {
			w.WriteHeader(http.StatusNotFound)
			_ = json.NewEncoder(w).Encode(map[string]string{"error": "match_not_found"})
			return
		}
		w.WriteHeader(http.StatusInternalServerError)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
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
	if len(req.GroupIDs) > 0 && len(req.GroupIDs) != len(req.PlayerIDs) {
		http.Error(w, "Неверное количество group_ids", http.StatusBadRequest)
		return
	}
	plan, err := normalizeMatchPlan(req)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	teamsCount := len(plan.Teams)
	totalPlayers := len(plan.PlayerIDs)
	activeUserID := 0
	if len(plan.TurnOrder) > 0 {
		activeUserID = plan.TurnOrder[0]
	}

	cfg := game.MapConfig{
		TotalPlayers: totalPlayers,
		TeamsCount:   teamsCount,
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

	// 5. В matches больше не храним runtime-карту целиком:
	// source of truth переехал в match_map_cells.
	mapJSON := []byte("[]")
	turnOrderJSON := toJSON(plan.TurnOrder)
	startPosJSON := toJSON(startPositions)
	portalPosJSON := toJSON(portalPos)

	// 5.5. Выбираем случайный артефакт-квест
	questArtifact, err := repository.GetRandomArtifactFromCatalog()
	if err != nil {
		handleError(w, "[CreateMatch] GetRandomArtifactFromCatalog failed", err)
		return
	}

	// 6. Сначала вставляем матч — он нужен для внешнего ключа монстров!
	_, err = repository.DB.Exec(`
		INSERT INTO matches (
			instance_id, mode, teams_count, total_players,
			map_width, map_height, map, active_user_id,
			turn_order, turn_number, start_positions, portal_position,
			quest_artifact_id
		) VALUES (
			$1, $2, $3, $4,
			$5, $6, $7, $8,
			$9, 1, $10, $11, $12
		)
		ON CONFLICT (instance_id) DO NOTHING
	`,
		req.InstanceID,
		req.Mode,
		teamsCount,
		totalPlayers,
		mapWidth,
		mapHeight,
		mapJSON, // Пока без db_instance_id
		activeUserID,
		turnOrderJSON,
		startPosJSON,
		portalPosJSON,
		questArtifact.ID,
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

	// 8. Сохраняем runtime-карту в специализированную таблицу клеток
	if err := repository.InsertMatchMapCells(req.InstanceID, fullMap); err != nil {
		handleError(w, "[CreateMatch] failed to persist match map cells", err)
		return
	}

	startByPlayer, err := mapStartPositionsToPlayers(plan.Teams, startPositions)
	if err != nil {
		_, _ = repository.DB.Exec(`DELETE FROM matches WHERE instance_id = $1`, req.InstanceID)
		handleError(w, "[CreateMatch] failed to map start positions", err)
		return
	}

	// 9. Копируем игроков в матч
	if err := assignMatchPlayers(req.InstanceID, plan.Teams, startByPlayer); err != nil {
		_, _ = repository.DB.Exec(`DELETE FROM matches WHERE instance_id = $1`, req.InstanceID)
		handleError(w, "[CreateMatch] failed to assign players", err)
		return
	}

	game.CreateMatchState(req.InstanceID, plan.TurnOrder)

	// 10. Обновляем состояние матча: активный игрок – первый, turn_number = 1.
	if err := repository.UpdateMatchTurn(req.InstanceID, activeUserID, 1); err != nil {
		log.Printf("[CreateMatch] UpdateMatchTurn failed: %v", err)
	}
	// Запускаем таймер хода для первого игрока
	startTurnTimer(req.InstanceID, activeUserID)

	// 11. Теперь получаем список игроков для ответа (они уже точно есть!)
	playersInMatch, err := repository.GetPlayersInMatch(req.InstanceID)
	if err != nil {
		handleError(w, "[CreateMatch] GetPlayersInMatch failed", err)
		return
	}

	// 12. Собираем и отдаём JSON-ответ
	resp := MatchResponse{
		InstanceID:               req.InstanceID,
		Mode:                     req.Mode,
		TeamsCount:               teamsCount,
		TotalPlayers:             totalPlayers,
		MapWidth:                 mapWidth,
		MapHeight:                mapHeight,
		Map:                      fullMap,
		Players:                  playersInMatch,
		ActiveUser:               activeUserID,
		TurnNumber:               1,
		StartPositions:           startPositions,
		PortalPosition:           portalPos,
		QuestArtifactID:          questArtifact.ID,
		QuestArtifactName:        questArtifact.Name,
		QuestArtifactImage:       questArtifact.Image,
		QuestArtifactDescription: questArtifact.Description,
	}
	w.WriteHeader(http.StatusCreated)
	if err := json.NewEncoder(w).Encode(resp); err != nil {
		log.Printf("[CreateMatch] Encode response failed: %v", err)
	}
}
