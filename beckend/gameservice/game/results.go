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
	userID int,
	survived bool,
) (
	exp int,
	rewards []Reward,
	playerKills int,
	monsterKills int,
	dmgTotal int,
	dmgPlayers int,
	dmgMonsters int,
	dmgTaken int,
	err error,
) {
	// 1) Достаём состояние матча
	MatchStatesMu.RLock()
	state, ok := MatchStates[instanceID]
	MatchStatesMu.RUnlock()
	if !ok {
		return 0, nil, 0, 0, 0, 0, 0, 0, fmt.Errorf("no state for match %s", instanceID)
	}

	return CalculateResultsFromEvents(state.SnapshotResultEvents(), userID, survived)
}

// CalculateResultsFromEvents uses the same rules as CalculateResults but accepts
// a consistent persisted snapshot instead of process-local MatchState.
func CalculateResultsFromEvents(events ResultEvents, userID int, survived bool) (
	exp int, rewards []Reward, playerKills int, monsterKills int,
	dmgTotal int, dmgPlayers int, dmgMonsters int, dmgTaken int, err error,
) {
	// 2) Считаем убийства
	countedKills := make(map[string]struct{}, len(events.KillEvents))
	for _, ke := range events.KillEvents {
		isDuplicateKill := false
		if ke.VictimID > 0 {
			key := fmt.Sprintf("%s:%d", ke.VictimType, ke.VictimID)
			if _, exists := countedKills[key]; exists {
				isDuplicateKill = true
			} else {
				countedKills[key] = struct{}{}
			}
		}
		if isDuplicateKill || ke.KillerID != userID {
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
	for _, de := range events.DamageEvents {
		if de.DealerID == userID {
			switch de.TargetType {
			case "player":
				dmgPlayers += de.Amount
			case "monster":
				dmgMonsters += de.Amount
			}
		}

		if de.TargetType == "player" && de.TargetID == userID {
			dmgTaken += de.Amount
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

	return exp, rewards, playerKills, monsterKills, dmgTotal, dmgPlayers, dmgMonsters, dmgTaken, nil
}
