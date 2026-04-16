package handlers

import (
	"encoding/json"
	"fmt"
	"net/http"
	"sort"
	"strings"
	"time"

	"gameservice/middleware"
	"gameservice/models"
	"gameservice/repository"

	"github.com/gorilla/mux"
)

type matchHistoryReward struct {
	Type   string `json:"type"`
	Amount int    `json:"amount"`
}

type matchHistoryWinner struct {
	Type    string `json:"type"`
	ID      int    `json:"id"`
	UserIDs []int  `json:"userIds,omitempty"`
}

type matchHistoryParticipant struct {
	UserID        int    `json:"userId"`
	Name          string `json:"name"`
	GroupID       int    `json:"groupId"`
	CharacterType string `json:"characterType,omitempty"`
	Placement     int    `json:"placement,omitempty"`
	IsWinner      bool   `json:"isWinner"`
	Survived      bool   `json:"survived"`
}

type matchPlayerRewardSummary struct {
	ExpGained      int                  `json:"expGained"`
	CurrencyGained int                  `json:"currencyGained"`
	Rewards        []matchHistoryReward `json:"rewards"`
}

type matchPlayerResultSummary struct {
	Placement int    `json:"placement"`
	IsWinner  bool   `json:"isWinner"`
	Survived  bool   `json:"survived"`
	Deaths    int    `json:"deaths"`
	Kills     int    `json:"kills"`
	Status    string `json:"status"`
}

type matchHistoryListItem struct {
	MatchID             string                    `json:"matchId"`
	FinishedAt          time.Time                 `json:"finishedAt"`
	Mode                string                    `json:"mode"`
	Participants        []matchHistoryParticipant `json:"participants"`
	Winner              matchHistoryWinner        `json:"winner"`
	PlayerRewardSummary matchPlayerRewardSummary  `json:"playerRewardSummary"`
	PlayerResultSummary matchPlayerResultSummary  `json:"playerResultSummary"`
	Status              string                    `json:"status"`
}

type matchHistoryListResponse struct {
	Status string                 `json:"status"`
	Data   []matchHistoryListItem `json:"data"`
}

type matchHistoryArtifact struct {
	ItemID      int    `json:"itemId"`
	Name        string `json:"name"`
	Count       int    `json:"count"`
	Image       string `json:"image,omitempty"`
	Description string `json:"description,omitempty"`
}

type matchHistoryBonus struct {
	Type     string `json:"type"`
	Label    string `json:"label"`
	Amount   int    `json:"amount,omitempty"`
	ExpBonus int    `json:"expBonus,omitempty"`
}

type currentPlayerMatchStats struct {
	Rewards          []matchHistoryReward   `json:"rewards"`
	ExpGained        int                    `json:"expGained"`
	CurrencyGained   int                    `json:"currencyGained"`
	DamageDealt      int                    `json:"damageDealt"`
	DamageTaken      int                    `json:"damageTaken"`
	DamageToPlayers  int                    `json:"damageToPlayers"`
	DamageToMonsters int                    `json:"damageToMonsters"`
	Kills            int                    `json:"kills"`
	PlayerKills      int                    `json:"playerKills"`
	MonsterKills     int                    `json:"monsterKills"`
	Deaths           int                    `json:"deaths"`
	Survived         bool                   `json:"survived"`
	Placement        int                    `json:"placement"`
	Artifacts        []matchHistoryArtifact `json:"artifacts"`
	OtherBonuses     []matchHistoryBonus    `json:"otherBonuses"`
}

type matchHistoryDetailItem struct {
	MatchID             string                    `json:"matchId"`
	FinishedAt          time.Time                 `json:"finishedAt"`
	Mode                string                    `json:"mode"`
	Participants        []matchHistoryParticipant `json:"participants"`
	Winner              matchHistoryWinner        `json:"winner"`
	PlayerRewardSummary matchPlayerRewardSummary  `json:"playerRewardSummary"`
	PlayerResultSummary matchPlayerResultSummary  `json:"playerResultSummary"`
	CurrentPlayerStats  currentPlayerMatchStats   `json:"currentPlayerStats"`
	Status              string                    `json:"status"`
}

type matchHistoryDetailResponse struct {
	Status string                  `json:"status"`
	Data   *matchHistoryDetailItem `json:"data,omitempty"`
}

func writeMatchHistoryError(w http.ResponseWriter, statusCode int, errorCode string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	_ = json.NewEncoder(w).Encode(map[string]string{
		"error": errorCode,
	})
}

func parseMatchRewards(raw json.RawMessage) []matchHistoryReward {
	if len(raw) == 0 {
		return []matchHistoryReward{}
	}

	rewards := make([]matchHistoryReward, 0)
	var asArray []struct {
		Type   string `json:"type"`
		Amount int    `json:"amount"`
	}
	if err := json.Unmarshal(raw, &asArray); err == nil {
		for _, reward := range asArray {
			if reward.Type == "" {
				continue
			}
			rewards = append(rewards, matchHistoryReward{
				Type:   reward.Type,
				Amount: reward.Amount,
			})
		}
		return rewards
	}

	var asMap map[string]int
	if err := json.Unmarshal(raw, &asMap); err == nil {
		for rewardType, amount := range asMap {
			rewards = append(rewards, matchHistoryReward{
				Type:   rewardType,
				Amount: amount,
			})
		}
		sort.SliceStable(rewards, func(i, j int) bool {
			return rewards[i].Type < rewards[j].Type
		})
	}

	return rewards
}

func totalCurrencyGained(rewards []matchHistoryReward) int {
	total := 0
	for _, reward := range rewards {
		normalized := strings.ToLower(strings.TrimSpace(reward.Type))
		switch normalized {
		case "balance", "coin", "coins", "gold":
			total += reward.Amount
		}
	}
	return total
}

func buildMatchWinner(matchInfo models.MatchInfo) matchHistoryWinner {
	winnerType := "user"
	winnerID := matchInfo.WinnerID
	if matchInfo.WinnerGroupID > 0 {
		winnerType = "group"
		winnerID = matchInfo.WinnerGroupID
	}

	return matchHistoryWinner{
		Type:    winnerType,
		ID:      winnerID,
		UserIDs: append([]int(nil), matchInfo.WinnerUserIDs...),
	}
}

func buildResultStatus(playerStats models.PlayerMatchStat) string {
	switch {
	case playerStats.IsWinner:
		return "winner"
	case playerStats.Survived:
		return "survived"
	default:
		return "eliminated"
	}
}

func toMatchHistoryParticipant(snapshot models.MatchParticipantSnapshot) matchHistoryParticipant {
	return matchHistoryParticipant{
		UserID:        snapshot.UserID,
		Name:          snapshot.Name,
		GroupID:       snapshot.GroupID,
		CharacterType: snapshot.CharacterType,
		Placement:     snapshot.Placement,
		IsWinner:      snapshot.IsWinner,
		Survived:      snapshot.Survived,
	}
}

func participantsFromStoredStats(stats []models.PlayerMatchStat) []matchHistoryParticipant {
	participants := make([]matchHistoryParticipant, 0, len(stats))
	for _, playerStat := range stats {
		name := playerStat.PlayerName
		if strings.TrimSpace(name) == "" {
			name = fmt.Sprintf("Player %d", playerStat.UserID)
		}
		participants = append(participants, matchHistoryParticipant{
			UserID:        playerStat.UserID,
			Name:          name,
			GroupID:       playerStat.GroupID,
			CharacterType: playerStat.CharacterType,
			Placement:     playerStat.Placement,
			IsWinner:      playerStat.IsWinner,
			Survived:      playerStat.Survived,
		})
	}

	sort.SliceStable(participants, func(i, j int) bool {
		if participants[i].Placement != participants[j].Placement {
			return participants[i].Placement < participants[j].Placement
		}
		return participants[i].UserID < participants[j].UserID
	})

	return participants
}

func resolveParticipants(matchInfo models.MatchInfo) []matchHistoryParticipant {
	if len(matchInfo.Participants) > 0 {
		participants := make([]matchHistoryParticipant, 0, len(matchInfo.Participants))
		for _, snapshot := range matchInfo.Participants {
			participants = append(participants, toMatchHistoryParticipant(snapshot))
		}
		sort.SliceStable(participants, func(i, j int) bool {
			if participants[i].Placement != participants[j].Placement {
				return participants[i].Placement < participants[j].Placement
			}
			return participants[i].UserID < participants[j].UserID
		})
		return participants
	}

	storedStats, err := repository.LoadMatchPlayerStats(matchInfo.InstanceID)
	if err != nil || len(storedStats) == 0 {
		return []matchHistoryParticipant{}
	}

	return participantsFromStoredStats(storedStats)
}

func asMap(value interface{}) map[string]interface{} {
	typed, ok := value.(map[string]interface{})
	if !ok {
		return nil
	}
	return typed
}

func asSlice(value interface{}) []interface{} {
	typed, ok := value.([]interface{})
	if !ok {
		return nil
	}
	return typed
}

func extractArtifact(raw map[string]interface{}) *matchHistoryArtifact {
	itemType, _ := raw["item_type"].(string)
	if itemType != "artifact" {
		return nil
	}

	name, _ := raw["name"].(string)
	image, _ := raw["image"].(string)
	description, _ := raw["description"].(string)

	itemID, _ := raw["item_id"].(float64)
	count, _ := raw["item_count"].(float64)
	if count <= 0 {
		count = 1
	}

	return &matchHistoryArtifact{
		ItemID:      int(itemID),
		Name:        name,
		Count:       int(count),
		Image:       image,
		Description: description,
	}
}

func parseArtifacts(raw json.RawMessage) []matchHistoryArtifact {
	if len(raw) == 0 {
		return []matchHistoryArtifact{}
	}

	var decoded interface{}
	if err := json.Unmarshal(raw, &decoded); err != nil {
		return []matchHistoryArtifact{}
	}

	artifacts := make([]matchHistoryArtifact, 0)
	appendArtifact := func(candidate interface{}) {
		if item := extractArtifact(asMap(candidate)); item != nil {
			artifacts = append(artifacts, *item)
		}
	}

	switch typed := decoded.(type) {
	case []interface{}:
		for _, entry := range typed {
			appendArtifact(entry)
		}
	case map[string]interface{}:
		if typedArtifacts := asMap(typed["artifacts"]); typedArtifacts != nil {
			for _, entry := range typedArtifacts {
				appendArtifact(entry)
			}
		} else if typedArtifactsList := asSlice(typed["artifacts"]); typedArtifactsList != nil {
			for _, entry := range typedArtifactsList {
				appendArtifact(entry)
			}
		} else {
			for _, entry := range typed {
				appendArtifact(entry)
			}
		}
	}

	sort.SliceStable(artifacts, func(i, j int) bool {
		if artifacts[i].Name != artifacts[j].Name {
			return artifacts[i].Name < artifacts[j].Name
		}
		return artifacts[i].ItemID < artifacts[j].ItemID
	})

	return artifacts
}

func buildOtherBonuses(playerStats models.PlayerMatchStat) []matchHistoryBonus {
	bonuses := make([]matchHistoryBonus, 0, 1)
	if playerStats.IsWinner {
		bonuses = append(bonuses, matchHistoryBonus{
			Type:     "winner_bonus",
			Label:    "Winner bonus",
			Amount:   100,
			ExpBonus: 120,
		})
	}
	return bonuses
}

func buildMatchHistoryListItem(record repository.CompletedMatchRecord) matchHistoryListItem {
	rewards := parseMatchRewards(record.PlayerStats.Rewards)
	return matchHistoryListItem{
		MatchID:      record.Match.InstanceID,
		FinishedAt:   record.Match.FinishedAt,
		Mode:         record.Match.Mode,
		Participants: resolveParticipants(record.Match),
		Winner:       buildMatchWinner(record.Match),
		PlayerRewardSummary: matchPlayerRewardSummary{
			ExpGained:      record.PlayerStats.ExpGained,
			CurrencyGained: totalCurrencyGained(rewards),
			Rewards:        rewards,
		},
		PlayerResultSummary: matchPlayerResultSummary{
			Placement: record.PlayerStats.Placement,
			IsWinner:  record.PlayerStats.IsWinner,
			Survived:  record.PlayerStats.Survived,
			Deaths:    record.PlayerStats.Deaths,
			Kills:     record.PlayerStats.PlayerKills + record.PlayerStats.MonsterKills,
			Status:    buildResultStatus(record.PlayerStats),
		},
		Status: "completed",
	}
}

func buildMatchHistoryDetailItem(record repository.CompletedMatchRecord) matchHistoryDetailItem {
	rewards := parseMatchRewards(record.PlayerStats.Rewards)
	return matchHistoryDetailItem{
		MatchID:      record.Match.InstanceID,
		FinishedAt:   record.Match.FinishedAt,
		Mode:         record.Match.Mode,
		Participants: resolveParticipants(record.Match),
		Winner:       buildMatchWinner(record.Match),
		PlayerRewardSummary: matchPlayerRewardSummary{
			ExpGained:      record.PlayerStats.ExpGained,
			CurrencyGained: totalCurrencyGained(rewards),
			Rewards:        rewards,
		},
		PlayerResultSummary: matchPlayerResultSummary{
			Placement: record.PlayerStats.Placement,
			IsWinner:  record.PlayerStats.IsWinner,
			Survived:  record.PlayerStats.Survived,
			Deaths:    record.PlayerStats.Deaths,
			Kills:     record.PlayerStats.PlayerKills + record.PlayerStats.MonsterKills,
			Status:    buildResultStatus(record.PlayerStats),
		},
		CurrentPlayerStats: currentPlayerMatchStats{
			Rewards:          rewards,
			ExpGained:        record.PlayerStats.ExpGained,
			CurrencyGained:   totalCurrencyGained(rewards),
			DamageDealt:      record.PlayerStats.DamageTotal,
			DamageTaken:      record.PlayerStats.DamageTaken,
			DamageToPlayers:  record.PlayerStats.DamageToPlayers,
			DamageToMonsters: record.PlayerStats.DamageToMonsters,
			Kills:            record.PlayerStats.PlayerKills + record.PlayerStats.MonsterKills,
			PlayerKills:      record.PlayerStats.PlayerKills,
			MonsterKills:     record.PlayerStats.MonsterKills,
			Deaths:           record.PlayerStats.Deaths,
			Survived:         record.PlayerStats.Survived,
			Placement:        record.PlayerStats.Placement,
			Artifacts:        parseArtifacts(record.PlayerStats.InventorySnapshot),
			OtherBonuses:     buildOtherBonuses(record.PlayerStats),
		},
		Status: "completed",
	}
}

func ListCompletedMatchesHandler(w http.ResponseWriter, r *http.Request) {
	userID, ok := middleware.GetUserIDFromContext(r.Context())
	if !ok || userID == 0 {
		writeMatchHistoryError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	records, err := repository.ListCompletedMatchesForUser(userID)
	if err != nil {
		writeMatchHistoryError(w, http.StatusInternalServerError, "failed_to_load_match_history")
		return
	}

	items := make([]matchHistoryListItem, 0, len(records))
	for _, record := range records {
		items = append(items, buildMatchHistoryListItem(record))
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(matchHistoryListResponse{
		Status: "ok",
		Data:   items,
	})
}

func GetCompletedMatchDetailsHandler(w http.ResponseWriter, r *http.Request) {
	userID, ok := middleware.GetUserIDFromContext(r.Context())
	if !ok || userID == 0 {
		writeMatchHistoryError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	instanceID := mux.Vars(r)["instance_id"]
	if strings.TrimSpace(instanceID) == "" {
		writeMatchHistoryError(w, http.StatusBadRequest, "instance_id_required")
		return
	}

	record, err := repository.GetCompletedMatchForUser(userID, instanceID)
	if err != nil {
		if !repository.IsNotFoundError(err) {
			writeMatchHistoryError(w, http.StatusInternalServerError, "failed_to_load_match_details")
			return
		}

		storedExists, storedErr := repository.StoredMatchExists(instanceID)
		if storedErr != nil {
			writeMatchHistoryError(w, http.StatusInternalServerError, "failed_to_load_match_details")
			return
		}
		if storedExists {
			isParticipant, participantErr := repository.CompletedMatchParticipantExists(instanceID, userID)
			if participantErr != nil {
				writeMatchHistoryError(w, http.StatusInternalServerError, "failed_to_load_match_details")
				return
			}
			if !isParticipant {
				writeMatchHistoryError(w, http.StatusForbidden, "match_access_denied")
				return
			}
		}

		activeExists, activeErr := repository.ActiveMatchExists(instanceID)
		if activeErr != nil {
			writeMatchHistoryError(w, http.StatusInternalServerError, "failed_to_load_match_details")
			return
		}
		if activeExists {
			isParticipant, participantErr := repository.ActiveMatchParticipantExists(instanceID, userID)
			if participantErr != nil {
				writeMatchHistoryError(w, http.StatusInternalServerError, "failed_to_load_match_details")
				return
			}
			if !isParticipant {
				writeMatchHistoryError(w, http.StatusForbidden, "match_access_denied")
				return
			}
			writeMatchHistoryError(w, http.StatusConflict, "match_not_completed")
			return
		}

		writeMatchHistoryError(w, http.StatusNotFound, "match_not_found")
		return
	}

	item := buildMatchHistoryDetailItem(*record)

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(matchHistoryDetailResponse{
		Status: "ok",
		Data:   &item,
	})
}
