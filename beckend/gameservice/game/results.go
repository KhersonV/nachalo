
//==================================
// gameservice/game/results.go
//==================================

package game

import (
	"fmt"
)



// CalculateResults рассчитывает все метрики для пользователя в матче
func CalculateResults(
    instanceID string,
    userID     int,
) (
    exp           int,
    rewards       []Reward,
    playerKills   int,
    monsterKills  int,
    dmgTotal      int,
    dmgPlayers    int,
    dmgMonsters   int,
    err           error,
) {
    // 1) Достаём состояние матча
    MatchStatesMu.RLock()
    state, ok := MatchStates[instanceID]
    MatchStatesMu.RUnlock()
    if !ok {
        return 0, nil, 0, 0, 0, 0, 0, fmt.Errorf("no state for match %s", instanceID)
    }

    // 2) Считаем убийства и урон
    for _, ke := range state.KillEvents {
        if ke.KillerID != userID {
            continue
        }
        switch ke.VictimType {
        case "player":
            playerKills++
        case "monster":
            monsterKills++
        }
        dmgTotal += ke.Damage
    }
    for _, de := range state.DamageEvents {
        if de.DealerID != userID {
            continue
        }
        dmgTotal += de.Amount
        switch de.TargetType {
        case "player":
            dmgPlayers += de.Amount
        case "monster":
            dmgMonsters += de.Amount
        }
    }

    // 3) Вычисляем опыт
    // — например: 100 XP за каждого игрока, 20 за монстра, плюс 1 XP за каждые 10 единиц урона
    exp = playerKills*100 + monsterKills*20 + (dmgTotal/10)

    // 4) Генерируем награды
    // — пример: монеты за фраги, артефакт за много фрагов монстров
    if playerKills > 0 {
        rewards = append(rewards, Reward{
            Type:   "coin",
            Amount: playerKills * 50, // по 50 монет за каждого убитого игрока
        })
    }
    if monsterKills > 5 {
        rewards = append(rewards, Reward{
            Type:   "artifact",
            Amount: 1, // даём 1 случайный артефакт
        })
    }

    return exp, rewards, playerKills, monsterKills, dmgTotal, dmgPlayers, dmgMonsters, nil
}

