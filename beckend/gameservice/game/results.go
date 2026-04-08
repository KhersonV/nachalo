
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
    survived   bool,
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

    // 2) Считаем убийства
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
    }

    // 3) Считаем урон только по DamageEvents.
    // KillEvents уже отражают факт убийства и могут дублировать последний удар.
    for _, de := range state.DamageEvents {
        if de.DealerID != userID {
            continue
        }
        switch de.TargetType {
        case "player":
            dmgPlayers += de.Amount
        case "monster":
            dmgMonsters += de.Amount
        }
    }

    dmgTotal = dmgPlayers + dmgMonsters

    // 4) Вычисляем опыт строго от урона:
    // - за каждые 5 урона по монстрам: +1 XP
    // - за каждые 5 урона по игрокам: +2 XP
    exp = (dmgMonsters / 5) + ((dmgPlayers / 5) * 2)

    // 5) Деньги за матч с анти-инфляционной корректировкой.
    // Базовая награда даётся только тем, кто дожил до конца матча.
    baseCoins := 0
    if survived {
        baseCoins = 20
    }
    coins := baseCoins + playerKills*20 + monsterKills*8 + (dmgTotal / 50)
    if coins > 0 {
        rewards = append(rewards, Reward{
            Type:   "balance",
            Amount: coins,
        })
    }

    return exp, rewards, playerKills, monsterKills, dmgTotal, dmgPlayers, dmgMonsters, nil
}

