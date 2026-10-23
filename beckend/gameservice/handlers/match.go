//===============================
// gameservice/handlers/match.go
//===============================

package handlers

import (
	"encoding/json"
	"fmt"
	"hash/fnv"
	"log"
	"net/http"

	"gameservice/internal/hexmap"
	"gameservice/models"
	"gameservice/repository"
)

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

	activeUserID := 0
	if len(plan.TurnOrder) > 0 {
		activeUserID = plan.TurnOrder[0]
	}
	teams := make([]hexmap.Team, 0, len(plan.Teams))
	for _, team := range plan.Teams {
		teams = append(teams, hexmap.Team{ID: team.TeamID, PlayerIDs: copyInts(team.MemberIDs)})
	}
	hash := fnv.New64a()
	_, _ = hash.Write([]byte(req.InstanceID))
	_, err = repository.CreateHexMatch(r.Context(), repository.HexMatchPlan{InstanceID: req.InstanceID, Mode: req.Mode, Teams: teams, TurnOrder: plan.TurnOrder, Seed: int64(hash.Sum64())})
	if err != nil {
		handleError(w, "[CreateMatch] HEX materialization failed", err)
		return
	}
	scheduleHexTurn(req.InstanceID, activeUserID, 1)
	w.WriteHeader(http.StatusCreated)
	_ = json.NewEncoder(w).Encode(map[string]any{"instance_id": req.InstanceID, "schemaVersion": 2, "geometry": models.HexV1})
}
