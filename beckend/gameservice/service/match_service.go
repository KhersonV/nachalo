
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
	log.Printf("[FinalizeMatch] start for match %s", instanceID)

    // 1) Получить финальные данные по матчу
    results, err := repository.GetMatchResults(instanceID)
    if err != nil {
        return err
    }

    // 2) Начислить опыт и награды каждому игроку
    for _, r := range results.PlayerResults {
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
    }
    if err := repository.SaveMatchStats(&stats); err != nil {
        log.Printf("SaveMatchStats failed: %v", err)
    }

    // 4) **Сохранить детальную статистику по каждому игроку**
     log.Printf("[FinalizeMatch] will save %d player stats for match %s", len(results.PlayerResults), instanceID)
    if err := repository.SaveMatchPlayerStats(instanceID, results.PlayerResults); err != nil {
        log.Printf("[FinalizeMatch] SaveMatchPlayerStats failed for match %s: %v", instanceID, err)
    } else {
        log.Printf("[FinalizeMatch] SaveMatchPlayerStats succeeded for match %s", instanceID)
    }

    // 5) Удалить (или пометить завершённым) сам матч и все связанные данные
    if err := repository.DeleteMatch(instanceID); err != nil {
        return err
    }
	log.Printf("[FinalizeMatch] match %s deleted", instanceID)
    return nil
}

// Payload, который пошлём в WS после успешной финализации
func BuildMatchEndedPayload(instanceID string) []byte {
    msg := struct {
        Type    string `json:"type"`
        Payload struct {
            InstanceID string `json:"instanceId"`
        } `json:"payload"`
    }{
        Type: "MATCH_ENDED",
    }
    msg.Payload.InstanceID = instanceID
    b, _ := json.Marshal(msg)
    return b
}
