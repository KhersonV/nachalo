
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

    // 3) Вычисляем опыт: базовый за участие + вклад в бою.
    exp = 80 + playerKills*80 + monsterKills*20 + (dmgTotal / 12)

    // 4) Деньги за матч (пока без артефактов/коллекций).
    coins := 40 + playerKills*30 + monsterKills*10 + (dmgTotal / 30)
    if coins > 0 {
        rewards = append(rewards, Reward{
            Type:   "balance",
            Amount: coins,
        })
    }

    return exp, rewards, playerKills, monsterKills, dmgTotal, dmgPlayers, dmgMonsters, nil
}

