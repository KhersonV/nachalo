// ==============================
// /matchmaking/Match_Making.go
// ==============================

package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"strconv"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/gorilla/mux"

	"gameservice/game"
	"gameservice/repository"
)

// Типы запросов и структур остаются без изменений.
type JoinRequest struct {
	PlayerID int    `json:"player_id"`
	Mode     string `json:"mode"`
	Rating   int    `json:"rating"`
}

type CancelRequest struct {
	PlayerID int    `json:"player_id"`
	Mode     string `json:"mode"`
}

type QueueEntry struct {
	PlayerID   int       `json:"player_id"`
	LeaderID   int       `json:"leader_id"`
	PartyID    string    `json:"party_id,omitempty"`
	MemberIDs  []int     `json:"member_ids,omitempty"`
	PartySize  int       `json:"party_size"`
	Rating     int       `json:"rating"`
	JoinTime   time.Time `json:"join_time"`
}

type MatchInfo struct {
	InstanceID   string       `json:"instance_id"`
	Mode         string       `json:"mode"`
	Players      []QueueEntry `json:"players"`
	TeamsCount   int          `json:"teams_count"`
	TotalPlayers int          `json:"total_players"`
}

type PartyActionRequest struct {
	LeaderID int `json:"leader_id"`
	MemberID int `json:"member_id"`
	PlayerID int `json:"player_id"`
}

type PartyInvite struct {
	LeaderID   int
	PartyID    string
	TargetID   int
	CreatedAt  time.Time
}

type PartyInviteState struct {
	Leader   PartyMemberState `json:"leader"`
	PartyID  string           `json:"partyId,omitempty"`
	CreatedAt string          `json:"createdAt"`
}

type PartyInfo struct {
	PartyID  string
	LeaderID int
	Members  []int
}

type PartyMemberState struct {
	UserID         int    `json:"user_id"`
	Name           string `json:"name"`
	Image          string `json:"image"`
	CharacterType  string `json:"characterType"`
	Level          int    `json:"level"`
}

type PartyStateResponse struct {
	InParty   bool               `json:"inParty"`
	PartyID   string             `json:"partyId,omitempty"`
	LeaderID  int                `json:"leaderId"`
	IsLeader  bool               `json:"isLeader"`
	Members   []PartyMemberState `json:"members"`
	PartySize int                `json:"partySize"`
	QueueMode string             `json:"queueMode,omitempty"`
}

var (
	queues = map[string][]QueueEntry{
		"PVE":  {},
		"1x1":  {},
		"1x2":  {},
		"2x2":  {},
		"3x3":  {},
		"5x5":  {},
	}
	mu sync.Mutex

	// Храним матчи по instance_id
	currentMatches = make(map[string]MatchInfo)
	// Глобальная мапа сопоставлений: для каждого игрока его instance_id матча
	playerMatches = make(map[int]string)
	matchMu       sync.Mutex

	parties      = make(map[string]*PartyInfo)
	playerParties = make(map[int]string)
	partyMu      sync.Mutex

	partyInvites = make(map[int]PartyInvite)
)

var gameServiceURL = os.Getenv("GAME_SERVICE_URL")

// RemovePlayerMatch убирает игрока из текущего матча (playerMatches)
func RemovePlayerMatch(playerID int) {
    matchMu.Lock()
    defer matchMu.Unlock()
    delete(playerMatches, playerID)
}

// RemoveMatch помечает матч как завершённый — удаляет из currentMatches
func RemoveMatch(instanceID string) {
    matchMu.Lock()
    defer matchMu.Unlock()
    delete(currentMatches, instanceID)
}

func requiredPlayersForMode(mode string) int {
	requiredPlayers := map[string]int{"PVE": 1, "1x1": 2, "1x2": 3, "2x2": 4, "3x3": 6, "5x5": 10}
	return requiredPlayers[mode]
}

func teamsCountForMode(mode string) int {
	switch mode {
	case "PVE":
		return 1
	case "1x1":
		return 2
	case "1x2":
		return 3
	case "2x2", "3x3", "5x5":
		return 2
	default:
		return 2
	}
}

func teamSizeForMode(mode string) int {
	needed := requiredPlayersForMode(mode)
	teams := teamsCountForMode(mode)
	if needed == 0 || teams == 0 {
		return 1
	}
	return needed / teams
}

func containsPlayer(ids []int, playerID int) bool {
	for _, id := range ids {
		if id == playerID {
			return true
		}
	}
	return false
}

func copyIDs(ids []int) []int {
	if len(ids) == 0 {
		return []int{}
	}
	result := make([]int, len(ids))
	copy(result, ids)
	return result
}

func queueContainsPlayerLocked(playerID int) bool {
	for _, q := range queues {
		for _, entry := range q {
			if containsPlayer(entry.MemberIDs, playerID) || entry.PlayerID == playerID {
				return true
			}
		}
	}
	return false
}

func getQueueModeForLeaderLocked(leaderID int) string {
	for mode, q := range queues {
		for _, entry := range q {
			if entry.LeaderID == leaderID {
				return mode
			}
		}
	}
	return ""
}

func buildPartyMemberState(playerID int) PartyMemberState {
	state := PartyMemberState{UserID: playerID, Name: fmt.Sprintf("Player %d", playerID), Image: "/ranger/ranger.webp", CharacterType: "adventurer"}
	if p, err := repository.GetPlayerByUserID(playerID); err == nil && p != nil {
		state.Name = p.Name
		if state.Name == "" {
			state.Name = fmt.Sprintf("Player %d", playerID)
		}
		if p.Image != "" {
			state.Image = p.Image
		}
		if p.CharacterType != "" {
			state.CharacterType = p.CharacterType
		}
		state.Level = p.Level
	}
	return state
}

func buildPartyStateForPlayer(playerID int) PartyStateResponse {
	partyMu.Lock()
	partyID, ok := playerParties[playerID]
	var partyCopy *PartyInfo
	if ok {
		if party, exists := parties[partyID]; exists {
			partyCopy = &PartyInfo{PartyID: party.PartyID, LeaderID: party.LeaderID, Members: copyIDs(party.Members)}
		}
	}
	partyMu.Unlock()

	if partyCopy == nil {
		return PartyStateResponse{
			InParty:   false,
			LeaderID:  playerID,
			IsLeader:  true,
			Members:   []PartyMemberState{buildPartyMemberState(playerID)},
			PartySize: 1,
		}
	}

	mu.Lock()
	queueMode := getQueueModeForLeaderLocked(partyCopy.LeaderID)
	mu.Unlock()

	members := make([]PartyMemberState, 0, len(partyCopy.Members))
	for _, memberID := range partyCopy.Members {
		members = append(members, buildPartyMemberState(memberID))
	}

	return PartyStateResponse{
		InParty:   true,
		PartyID:   partyCopy.PartyID,
		LeaderID:  partyCopy.LeaderID,
		IsLeader:  partyCopy.LeaderID == playerID,
		Members:   members,
		PartySize: len(members),
		QueueMode: queueMode,
	}
}

func disbandPartyLocked(party *PartyInfo) {
	for _, memberID := range party.Members {
		delete(playerParties, memberID)
		delete(partyInvites, memberID)
	}
	for targetID, invite := range partyInvites {
		if invite.PartyID == party.PartyID || invite.LeaderID == party.LeaderID {
			delete(partyInvites, targetID)
		}
	}
	delete(parties, party.PartyID)
}

func buildInviteStateForPlayer(playerID int) []PartyInviteState {
	partyMu.Lock()
	invite, exists := partyInvites[playerID]
	partyMu.Unlock()
	if !exists {
		return []PartyInviteState{}
	}

	return []PartyInviteState{
		{
			Leader:   buildPartyMemberState(invite.LeaderID),
			PartyID:  invite.PartyID,
			CreatedAt: invite.CreatedAt.Format(time.RFC3339),
		},
	}
}

func partyInvitesHandler(w http.ResponseWriter, r *http.Request) {
	playerIDStr := r.URL.Query().Get("player_id")
	if playerIDStr == "" {
		http.Error(w, "player_id обязателен", http.StatusBadRequest)
		return
	}
	playerID, err := strconv.Atoi(playerIDStr)
	if err != nil || playerID <= 0 {
		http.Error(w, "Некорректный player_id", http.StatusBadRequest)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"status":  "ok",
		"invites": buildInviteStateForPlayer(playerID),
	})
}

func sendPartyInviteHandler(w http.ResponseWriter, r *http.Request) {
	var req PartyActionRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Bad request", http.StatusBadRequest)
		return
	}
	if req.LeaderID <= 0 || req.MemberID <= 0 || req.MemberID == req.LeaderID {
		http.Error(w, "Некорректные участники пати", http.StatusBadRequest)
		return
	}

	areFriends, err := repository.AreFriends(req.LeaderID, req.MemberID)
	if err != nil {
		http.Error(w, "Не удалось проверить дружбу", http.StatusInternalServerError)
		return
	}
	if !areFriends {
		http.Error(w, "В пати можно приглашать только друга", http.StatusForbidden)
		return
	}

	matchMu.Lock()
	_, leaderInMatch := playerMatches[req.LeaderID]
	_, memberInMatch := playerMatches[req.MemberID]
	matchMu.Unlock()
	if leaderInMatch || memberInMatch {
		http.Error(w, "Игрок уже в матче", http.StatusBadRequest)
		return
	}

	mu.Lock()
	leaderQueued := queueContainsPlayerLocked(req.LeaderID)
	memberQueued := queueContainsPlayerLocked(req.MemberID)
	mu.Unlock()
	if leaderQueued || memberQueued {
		http.Error(w, "Нельзя отправлять инвайт во время очереди", http.StatusBadRequest)
		return
	}

	partyMu.Lock()
	defer partyMu.Unlock()

	if _, exists := playerParties[req.MemberID]; exists {
		http.Error(w, "Игрок уже состоит в другой пати", http.StatusBadRequest)
		return
	}

	partyID, exists := playerParties[req.LeaderID]
	if !exists {
		partyID = uuid.New().String()
		parties[partyID] = &PartyInfo{PartyID: partyID, LeaderID: req.LeaderID, Members: []int{req.LeaderID}}
		playerParties[req.LeaderID] = partyID
	}

	party := parties[partyID]
	if party == nil || party.LeaderID != req.LeaderID {
		http.Error(w, "Только лидер может приглашать в пати", http.StatusForbidden)
		return
	}
	if len(party.Members) >= 5 {
		http.Error(w, "Максимальный размер пати 5 игроков", http.StatusBadRequest)
		return
	}
	if containsPlayer(party.Members, req.MemberID) {
		http.Error(w, "Игрок уже в пати", http.StatusBadRequest)
		return
	}

	partyInvites[req.MemberID] = PartyInvite{
		LeaderID:  req.LeaderID,
		PartyID:   partyID,
		TargetID:  req.MemberID,
		CreatedAt: time.Now(),
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{
		"status":  "ok",
		"message": "invite_sent",
	})
}

func acceptPartyInviteHandler(w http.ResponseWriter, r *http.Request) {
	var req PartyActionRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Bad request", http.StatusBadRequest)
		return
	}
	if req.PlayerID <= 0 {
		http.Error(w, "Некорректный player_id", http.StatusBadRequest)
		return
	}

	mu.Lock()
	queued := queueContainsPlayerLocked(req.PlayerID)
	mu.Unlock()
	if queued {
		http.Error(w, "Нельзя принимать инвайт во время очереди", http.StatusBadRequest)
		return
	}

	matchMu.Lock()
	_, inMatch := playerMatches[req.PlayerID]
	matchMu.Unlock()
	if inMatch {
		http.Error(w, "Игрок уже в матче", http.StatusBadRequest)
		return
	}

	partyMu.Lock()
	invite, exists := partyInvites[req.PlayerID]
	if !exists {
		partyMu.Unlock()
		http.Error(w, "Инвайт не найден", http.StatusNotFound)
		return
	}

	if _, inParty := playerParties[req.PlayerID]; inParty {
		delete(partyInvites, req.PlayerID)
		partyMu.Unlock()
		http.Error(w, "Игрок уже состоит в пати", http.StatusBadRequest)
		return
	}

	party := parties[invite.PartyID]
	if party == nil || party.LeaderID != invite.LeaderID {
		delete(partyInvites, req.PlayerID)
		partyMu.Unlock()
		http.Error(w, "Пати больше не существует", http.StatusBadRequest)
		return
	}
	if len(party.Members) >= 5 {
		delete(partyInvites, req.PlayerID)
		partyMu.Unlock()
		http.Error(w, "Пати уже заполнена", http.StatusBadRequest)
		return
	}

	party.Members = append(party.Members, req.PlayerID)
	playerParties[req.PlayerID] = invite.PartyID
	delete(partyInvites, req.PlayerID)
	partyMu.Unlock()

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(buildPartyStateForPlayer(req.PlayerID))
}

func rejectPartyInviteHandler(w http.ResponseWriter, r *http.Request) {
	var req PartyActionRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Bad request", http.StatusBadRequest)
		return
	}
	if req.PlayerID <= 0 {
		http.Error(w, "Некорректный player_id", http.StatusBadRequest)
		return
	}

	partyMu.Lock()
	delete(partyInvites, req.PlayerID)
	partyMu.Unlock()

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{
		"status": "ok",
	})
}

func partyHandler(w http.ResponseWriter, r *http.Request) {
	playerIDStr := r.URL.Query().Get("player_id")
	if playerIDStr == "" {
		http.Error(w, "player_id обязателен", http.StatusBadRequest)
		return
	}
	playerID, err := strconv.Atoi(playerIDStr)
	if err != nil || playerID <= 0 {
		http.Error(w, "Некорректный player_id", http.StatusBadRequest)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(buildPartyStateForPlayer(playerID))
}

func addPartyMemberHandler(w http.ResponseWriter, r *http.Request) {
	http.Error(w, "Используйте инвайт: /matchmaking/party/invite", http.StatusGone)
}

func removePartyMemberHandler(w http.ResponseWriter, r *http.Request) {
	var req PartyActionRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Bad request", http.StatusBadRequest)
		return
	}
	if req.LeaderID <= 0 || req.MemberID <= 0 || req.MemberID == req.LeaderID {
		http.Error(w, "Некорректные участники пати", http.StatusBadRequest)
		return
	}

	mu.Lock()
	leaderQueued := queueContainsPlayerLocked(req.LeaderID)
	mu.Unlock()
	if leaderQueued {
		http.Error(w, "Нельзя менять пати во время очереди", http.StatusBadRequest)
		return
	}

	partyMu.Lock()
	partyID, exists := playerParties[req.LeaderID]
	if !exists {
		partyMu.Unlock()
		http.Error(w, "Пати не найдена", http.StatusNotFound)
		return
	}
	party := parties[partyID]
	if party == nil || party.LeaderID != req.LeaderID {
		partyMu.Unlock()
		http.Error(w, "Только лидер может исключать из пати", http.StatusForbidden)
		return
	}

	nextMembers := make([]int, 0, len(party.Members))
	removed := false
	for _, memberID := range party.Members {
		if memberID == req.MemberID {
			removed = true
			delete(playerParties, memberID)
			delete(partyInvites, memberID)
			continue
		}
		nextMembers = append(nextMembers, memberID)
	}
	if !removed {
		partyMu.Unlock()
		http.Error(w, "Игрок не состоит в пати", http.StatusNotFound)
		return
	}
	party.Members = nextMembers
	if len(party.Members) <= 1 {
		disbandPartyLocked(party)
	}
	partyMu.Unlock()

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(buildPartyStateForPlayer(req.LeaderID))
}

func leavePartyHandler(w http.ResponseWriter, r *http.Request) {
	var req PartyActionRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Bad request", http.StatusBadRequest)
		return
	}
	if req.PlayerID <= 0 {
		http.Error(w, "Некорректный игрок", http.StatusBadRequest)
		return
	}

	mu.Lock()
	queued := queueContainsPlayerLocked(req.PlayerID)
	mu.Unlock()
	if queued {
		http.Error(w, "Нельзя выйти из пати во время очереди", http.StatusBadRequest)
		return
	}

	partyMu.Lock()
	partyID, exists := playerParties[req.PlayerID]
	if !exists {
		partyMu.Unlock()
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(buildPartyStateForPlayer(req.PlayerID))
		return
	}
	party := parties[partyID]
	if party == nil {
		delete(playerParties, req.PlayerID)
		partyMu.Unlock()
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(buildPartyStateForPlayer(req.PlayerID))
		return
	}

	nextMembers := make([]int, 0, len(party.Members))
	for _, memberID := range party.Members {
		if memberID == req.PlayerID {
			delete(playerParties, memberID)
			delete(partyInvites, memberID)
			continue
		}
		nextMembers = append(nextMembers, memberID)
	}
	party.Members = nextMembers
	if len(nextMembers) <= 1 {
		disbandPartyLocked(party)
	} else if party.LeaderID == req.PlayerID {
		party.LeaderID = nextMembers[0]
	}
	partyMu.Unlock()

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(buildPartyStateForPlayer(req.PlayerID))
}

func disbandPartyHandler(w http.ResponseWriter, r *http.Request) {
	var req PartyActionRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Bad request", http.StatusBadRequest)
		return
	}
	if req.LeaderID <= 0 {
		http.Error(w, "Некорректный лидер", http.StatusBadRequest)
		return
	}

	mu.Lock()
	queued := queueContainsPlayerLocked(req.LeaderID)
	mu.Unlock()
	if queued {
		http.Error(w, "Нельзя распустить пати во время очереди", http.StatusBadRequest)
		return
	}

	partyMu.Lock()
	partyID, exists := playerParties[req.LeaderID]
	if !exists {
		partyMu.Unlock()
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(buildPartyStateForPlayer(req.LeaderID))
		return
	}
	party := parties[partyID]
	if party == nil || party.LeaderID != req.LeaderID {
		partyMu.Unlock()
		http.Error(w, "Только лидер может распустить пати", http.StatusForbidden)
		return
	}
	disbandPartyLocked(party)
	partyMu.Unlock()

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(buildPartyStateForPlayer(req.LeaderID))
}

func assignEntriesToTeams(entries []QueueEntry, teamsCount int, teamSize int) (map[int]int, bool) {
	loads := make([]int, teamsCount)
	assignment := make(map[int]int)

	var backtrack func(index int) bool
	backtrack = func(index int) bool {
		if index == len(entries) {
			for _, load := range loads {
				if load != teamSize {
					return false
				}
			}
			return true
		}

		entry := entries[index]
		for team := 0; team < teamsCount; team++ {
			if loads[team]+entry.PartySize > teamSize {
				continue
			}
			loads[team] += entry.PartySize
			for _, memberID := range entry.MemberIDs {
				assignment[memberID] = team + 1
			}
			if backtrack(index + 1) {
				return true
			}
			loads[team] -= entry.PartySize
			for _, memberID := range entry.MemberIDs {
				delete(assignment, memberID)
			}
		}
		return false
	}

	if !backtrack(0) {
		return nil, false
	}
	return assignment, true
}

func findMatchCandidates(mode string, q []QueueEntry) ([]QueueEntry, map[int]int, bool) {
	needed := requiredPlayersForMode(mode)
	teamsCount := teamsCountForMode(mode)
	teamSize := teamSizeForMode(mode)

	selected := make([]QueueEntry, 0)
	var chosen []QueueEntry
	var chosenAssignments map[int]int

	var search func(index int, total int) bool
	search = func(index int, total int) bool {
		if total == needed {
			assignments, ok := assignEntriesToTeams(selected, teamsCount, teamSize)
			if ok {
				chosen = append([]QueueEntry{}, selected...)
				chosenAssignments = assignments
				return true
			}
			return false
		}
		if total > needed || index >= len(q) {
			return false
		}

		selected = append(selected, q[index])
		if search(index+1, total+q[index].PartySize) {
			return true
		}
		selected = selected[:len(selected)-1]

		if search(index+1, total) {
			return true
		}
		return false
	}

	if !search(0, 0) {
		return nil, nil, false
	}
	return chosen, chosenAssignments, true
}

// DELETE /matchmaking/player/{playerID}
func removePlayerHandler(w http.ResponseWriter, r *http.Request) {
    vars := mux.Vars(r)
    playerID, _ := strconv.Atoi(vars["playerID"])
    RemovePlayerMatch(playerID)
    w.WriteHeader(http.StatusOK)
}

// joinHandler – добавляет игрока в очередь и вызывает checkAndMakeMatch
func joinHandler(w http.ResponseWriter, r *http.Request) {
	var req JoinRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Bad request", http.StatusBadRequest)
		return
	}

	matchMu.Lock()
	instID, hasMatch := playerMatches[req.PlayerID]
	if hasMatch {
		if matchInfo, ok := currentMatches[instID]; ok {
			players, err := repository.GetPlayersInMatch(instID)
			if err == nil {
				for _, p := range players {
					if p.UserID == req.PlayerID {
						matchMu.Unlock()
						w.Header().Set("Content-Type", "application/json")
						json.NewEncoder(w).Encode(matchInfo)
						return
					}
				}
			}
		}
		delete(playerMatches, req.PlayerID)
	}
	matchMu.Unlock()

	partyMu.Lock()
	partyID, inParty := playerParties[req.PlayerID]
	memberIDs := []int{req.PlayerID}
	leaderID := req.PlayerID
	if inParty {
		party := parties[partyID]
		if party == nil {
			delete(playerParties, req.PlayerID)
			inParty = false
		} else {
			leaderID = party.LeaderID
			memberIDs = copyIDs(party.Members)
		}
	}
	partyMu.Unlock()

	if leaderID != req.PlayerID {
		http.Error(w, "Только лидер пати может начать поиск матча", http.StatusForbidden)
		return
	}
	if len(memberIDs) > teamSizeForMode(req.Mode) {
		http.Error(w, "Размер пати превышает размер команды для этого режима", http.StatusBadRequest)
		return
	}

	mu.Lock()
	defer mu.Unlock()

	q, ok := queues[req.Mode]
	if !ok {
		http.Error(w, "Unknown mode", http.StatusBadRequest)
		return
	}
	for _, memberID := range memberIDs {
		if queueContainsPlayerLocked(memberID) {
			http.Error(w, "Один из игроков уже в очереди", http.StatusBadRequest)
			return
		}
	}

	entry := QueueEntry{
		PlayerID:  leaderID,
		LeaderID:  leaderID,
		PartyID:   partyID,
		MemberIDs: copyIDs(memberIDs),
		PartySize: len(memberIDs),
		Rating:    req.Rating,
		JoinTime:  time.Now(),
	}
	queues[req.Mode] = append(q, entry)
	log.Printf("Leader %d joined mode %s with party size %d", leaderID, req.Mode, len(memberIDs))

	checkAndMakeMatch(req.Mode)

	w.WriteHeader(http.StatusOK)
	w.Write([]byte("You have joined the queue"))
}

// cancelHandler – удаляет игрока из очереди
func cancelHandler(w http.ResponseWriter, r *http.Request) {
	var req CancelRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Bad request", http.StatusBadRequest)
		return
	}

	mu.Lock()
	defer mu.Unlock()

	q, ok := queues[req.Mode]
	if !ok {
		http.Error(w, "Unknown mode", http.StatusBadRequest)
		return
	}

	newQueue := []QueueEntry{}
	found := false
	for _, entry := range q {
		if entry.LeaderID == req.PlayerID || containsPlayer(entry.MemberIDs, req.PlayerID) {
			found = true
			continue
		}
		newQueue = append(newQueue, entry)
	}

	queues[req.Mode] = newQueue
	if found {
		log.Printf("Player %d cancelled from mode %s", req.PlayerID, req.Mode)
		w.Write([]byte("Cancelled successfully"))
	} else {
		http.Error(w, "Player not found in queue", http.StatusNotFound)
	}
}

// statusHandler – возвращает очередь для указанного режима
func statusHandler(w http.ResponseWriter, r *http.Request) {
	mode := r.URL.Query().Get("mode")
	mu.Lock()
	defer mu.Unlock()

	q, ok := queues[mode]
	if !ok {
		http.Error(w, "Unknown mode", http.StatusBadRequest)
		return
	}

	totalPlayers := 0
	for _, entry := range q {
		totalPlayers += entry.PartySize
	}
	response, err := json.Marshal(map[string]interface{}{
		"queue":        q,
		"totalPlayers": totalPlayers,
	})
	if err != nil {
		http.Error(w, "Error marshalling response", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.Write(response)
}

// Новый эндпоинт: currentMatch – возвращает текущий матч для игрока по player_id
func currentMatchHandler(w http.ResponseWriter, r *http.Request) {
	playerIDStr := r.URL.Query().Get("player_id")
	if playerIDStr == "" {
		http.Error(w, "player_id обязателен", http.StatusBadRequest)
		return
	}

	var playerID int
	if _, err := fmt.Sscanf(playerIDStr, "%d", &playerID); err != nil {
		http.Error(w, "Некорректный player_id", http.StatusBadRequest)
		return
	}

	matchMu.Lock()
	defer matchMu.Unlock()

	instanceID, exists := playerMatches[playerID]
	if !exists {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"status": "waiting"})
		return
	}

	match, ok := currentMatches[instanceID]
	if !ok {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"status": "waiting"})
		return
	}

	// Validate that the player is still active in this match according to the DB.
	// This prevents stale in-memory mappings from redirecting the user to a
	// finished match (e.g., when FinalizeMatch deleted match_players via CASCADE
	// but unbindPlayer hadn't run yet or failed silently).
	players, err := repository.GetPlayersInMatch(instanceID)
	if err != nil || func() bool {
		for _, p := range players {
			if p.UserID == playerID {
				return false
			}
		}
		return true
	}() {
		// Player no longer in DB — clean up stale mapping
		delete(playerMatches, playerID)
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"status": "waiting"})
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(match)
}

// matchHandler – оставляем для совместимости (делегирует currentMatchHandler)
func matchHandler(w http.ResponseWriter, r *http.Request) {
	currentMatchHandler(w, r)
}

// GET /matchmaking/inQueue?player_id=NN
func inQueueHandler(w http.ResponseWriter, r *http.Request) {
	playerIDStr := r.URL.Query().Get("player_id")
	if playerIDStr == "" {
		http.Error(w, "player_id обязателен", http.StatusBadRequest)
		return
	}
	var playerID int
	if _, err := fmt.Sscanf(playerIDStr, "%d", &playerID); err != nil {
		http.Error(w, "Некорректный player_id", http.StatusBadRequest)
		return
	}

	mu.Lock()
	defer mu.Unlock()
	for mode, q := range queues {
		for _, entry := range q {
			if entry.LeaderID == playerID || containsPlayer(entry.MemberIDs, playerID) {
				w.Header().Set("Content-Type", "application/json")
				json.NewEncoder(w).Encode(map[string]interface{}{
					"inQueue":   true,
					"mode":      mode,
					"leaderId":  entry.LeaderID,
					"partySize": entry.PartySize,
				})
				return
			}
		}
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"inQueue": false,
	})
}


// checkAndMakeMatch – если в очереди набрано нужное количество игроков, формирует матч
func checkAndMakeMatch(mode string) {
	if requiredPlayersForMode(mode) == 0 {
		log.Printf("Unknown mode: %s", mode)
		return
	}

	q := queues[mode]
	group, groupAssignments, ok := findMatchCandidates(mode, q)
	if ok {
		remaining := make([]QueueEntry, 0, len(q)-len(group))
		usedLeaders := make(map[int]bool)
		for _, entry := range group {
			usedLeaders[entry.LeaderID] = true
		}
		for _, entry := range q {
			if usedLeaders[entry.LeaderID] {
				continue
			}
			remaining = append(remaining, entry)
		}
		queues[mode] = remaining
		go createMatch(mode, group, groupAssignments)
	}
}

// createMatch – создает новый матч, обновляет currentMatches и playerMatches
func createMatch(mode string, group []QueueEntry, groupAssignments map[int]int) {
	instanceID := uuid.New().String()

	totalPlayers := requiredPlayersForMode(mode)
	teamsCount := teamsCountForMode(mode)
	if totalPlayers == 0 {
		totalPlayers = len(group)
	}

	playerIDs := make([]int, 0, totalPlayers)
	groupIDs := make([]int, 0, totalPlayers)
	for _, entry := range group {
		for _, memberID := range entry.MemberIDs {
			playerIDs = append(playerIDs, memberID)
			groupIDs = append(groupIDs, groupAssignments[memberID])
		}
	}

	// Создаём внутреннее состояние матча
	matchState := game.CreateMatchState(instanceID, playerIDs)
	log.Printf("Создано состояние матча: %+v", matchState)

	// Отправляем запрос в Game-сервис
	matchReq := map[string]interface{}{
		"instance_id":   instanceID,
		"mode":          mode,
		"player_ids":    playerIDs,
		"group_ids":     groupIDs,
		"teams_count":   teamsCount,
		"total_players": totalPlayers,
	}
	reqJSON, err := json.Marshal(matchReq)
	if err != nil {
		log.Printf("Ошибка маршалинга запроса матча: %v", err)
		return
	}
	resp, err := http.Post(gameServiceURL+"/game/createMatch", "application/json", bytes.NewBuffer(reqJSON))
	if err != nil {
		log.Printf("Ошибка вызова Game-сервиса: %v", err)
		mu.Lock()
		queues[mode] = append(group, queues[mode]...)
		mu.Unlock()
		return
	}
	defer resp.Body.Close()
	log.Printf("Game-сервис вернул статус: %s", resp.Status)

	if resp.StatusCode != http.StatusCreated {
		body, _ := io.ReadAll(resp.Body)
		log.Printf("Game-сервис не создал матч: status=%d body=%s", resp.StatusCode, string(body))
		mu.Lock()
		queues[mode] = append(group, queues[mode]...)
		mu.Unlock()
		return
	}

	// Сохраняем состояние матча и сопоставление для каждого игрока
	matchMu.Lock()
	currentMatches[instanceID] = MatchInfo{
		InstanceID:   instanceID,
		Mode:         mode,
		Players:      group,
		TeamsCount:   teamsCount,
		TotalPlayers: totalPlayers,
	}
	for _, entry := range group {
		for _, memberID := range entry.MemberIDs {
			playerMatches[memberID] = instanceID
		}
	}
	matchMu.Unlock()

	// Broadcast match notification to involved players (SSE)
	go func() {
		defer func() {
			if r := recover(); r != nil {
				log.Printf("Recovered in BroadcastMatch: %v", r)
			}
		}()
		// build a copy of match info to broadcast
		matchMu.Lock()
		m, ok := currentMatches[instanceID]
		matchMu.Unlock()
		if ok {
			playerIDs := make([]int, 0, len(m.Players))
			for _, p := range m.Players {
				playerIDs = append(playerIDs, p.PlayerID)
			}
			BroadcastMatchToPlayers(playerIDs, m)
		}
	}()
}

// Простая CORS-обёртка
func enableCors(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusOK)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func main() {
	if gameServiceURL == "" {
		gameServiceURL = "http://gameservice:8001"
	}

	repository.InitDB()

	r := mux.NewRouter()
	r.HandleFunc("/matchmaking/join", joinHandler).Methods("POST")
	r.HandleFunc("/matchmaking/cancel", cancelHandler).Methods("POST")
	r.HandleFunc("/matchmaking/status", statusHandler).Methods("GET")
	r.HandleFunc("/matchmaking/currentMatch", currentMatchHandler).Methods("GET")
	r.HandleFunc("/matchmaking/stream", sseHandler).Methods("GET")
	r.HandleFunc("/matchmaking/match", matchHandler).Methods("GET")
	r.HandleFunc("/matchmaking/inQueue", inQueueHandler).Methods("GET")
	r.HandleFunc("/matchmaking/party", partyHandler).Methods("GET")
	r.HandleFunc("/matchmaking/party/invites", partyInvitesHandler).Methods("GET")
	r.HandleFunc("/matchmaking/party/invite", sendPartyInviteHandler).Methods("POST")
	r.HandleFunc("/matchmaking/party/invite/accept", acceptPartyInviteHandler).Methods("POST")
	r.HandleFunc("/matchmaking/party/invite/reject", rejectPartyInviteHandler).Methods("POST")
	r.HandleFunc("/matchmaking/party/add", addPartyMemberHandler).Methods("POST")
	r.HandleFunc("/matchmaking/party/remove", removePartyMemberHandler).Methods("POST")
	r.HandleFunc("/matchmaking/party/leave", leavePartyHandler).Methods("POST")
	r.HandleFunc("/matchmaking/party/disband", disbandPartyHandler).Methods("POST")
	r.HandleFunc("/matchmaking/player/{playerID}", removePlayerHandler).Methods("DELETE")

	handler := enableCors(r)

	port := os.Getenv("PORT")
	if port == "" {
		port = "8002"
	}

	srv := &http.Server{
		Handler: handler,
		Addr:    ":" + port,
	}

	log.Printf("Matchmaking-сервис запущен на порту %s", port)
	log.Fatal(srv.ListenAndServe())
}
