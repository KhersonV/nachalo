// =====================================
// /gameservice/service/match_service.go
// =====================================

package service

import (
	"encoding/json"
	"log"

	"gameservice/models"

	"gameservice/repository"
)

// FinalizeMatch завершает матч: сохраняет результаты, начисляет опыт/награды и удаляет матч.
func FinalizeMatch(instanceID string) error {
	

	// 1) Получить финальные данные по матчу
	results, err := repository.GetMatchResults(instanceID)
	if err != nil {
		return err
	}

	// 2) Начислить опыт и награды каждому игроку
	for _, r := range results.PlayerResults {
		if err := repository.SyncPersistentInventoryFromMatchResources(instanceID, r.UserID); err != nil {
			log.Printf("SyncPersistentInventoryFromMatchResources failed for user %d: %v", r.UserID, err)
		}

		if err := repository.AddPlayerExperience(r.UserID, r.ExpGained); err != nil {
			log.Printf("AddPlayerExperience failed for user %d: %v", r.UserID, err)
		}
		if err := repository.AddPlayerRewards(r.UserID, r.RewardsData); err != nil {
			log.Printf("AddPlayerRewards failed for user %d: %v", r.UserID, err)
		}
	}

	// 3) Сохранить общую статистику матча
	stats := models.MatchInfo{
		InstanceID:    instanceID,
		WinnerID:      results.WinnerID,
		WinnerGroupID: results.WinnerGroupID,
		WinnerUserIDs: results.WinnerUserIDs,
	}
	if err := repository.SaveMatchStats(&stats); err != nil {
		log.Printf("SaveMatchStats failed: %v", err)
	}

	// 4) **Сохранить детальную статистику по каждому игроку**
	
	if err := repository.SaveMatchPlayerStats(instanceID, results.PlayerResults); err != nil {
		log.Printf("[FinalizeMatch] SaveMatchPlayerStats failed for match %s: %v", instanceID, err)
	} else {
		log.Printf("[FinalizeMatch] SaveMatchPlayerStats succeeded for match %s", instanceID)
	}

	// 5) Удалить (или пометить завершённым) сам матч и все связанные данные
	if err := repository.DeleteMatch(instanceID); err != nil {
		return err
	}
	
	return nil
}

// 1) Расширить структуру payload, чтобы она включала всю статистику:
type MatchEndedPayload struct {
	Type    string           `json:"type"`
	Payload EndMatchResponse `json:"payload"`
}

type EndMatchResponse struct {
	InstanceID string                   `json:"instanceId"`
	Stats      []models.PlayerMatchStat `json:"stats"`  // детальная статистика по каждому игроку
	Winner     models.WinnerInfo        `json:"winner"` // { Type:"user"|"group", ID:int }
}

// 2) В BuildMatchEndedPayload собрать эти данные:
func BuildMatchEndedPayload(instanceID string) ([]byte, error) {
	// a) Из БД забираем сохранённую статистику
	playerStats, err := repository.LoadMatchPlayerStats(instanceID)
	if err != nil {
		return nil, err
	}
	// b) Из БД (или из результатов) берём WinnerID и WinnerGroupID
	matchInfo, err := repository.LoadMatchStats(instanceID)
	if err != nil {
		return nil, err
	}
	// c) Формируем структуру
	payload := MatchEndedPayload{
		Type: "MATCH_ENDED",
		Payload: EndMatchResponse{
			InstanceID: instanceID,
			Stats:      playerStats,
			Winner: models.WinnerInfo{
				Type: func() string {
					if matchInfo.WinnerGroupID != 0 {
						return "group"
					}
					return "user"
				}(),
				ID: func() int {
					if matchInfo.WinnerGroupID != 0 {
						return matchInfo.WinnerGroupID
					}
					return matchInfo.WinnerID
				}(),
			},
		},
	}
	return json.Marshal(payload)
}
