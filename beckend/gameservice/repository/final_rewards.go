package repository

import (
	"encoding/json"
	"gameservice/game"
)

func parseFinalRewards(raw []byte) (map[string]int, error) {
	rewards := make(map[string]int)
	var mapped map[string]int
	if err := json.Unmarshal(raw, &mapped); err == nil {
		for k, v := range mapped {
			rewards[k] += v
		}
		return rewards, nil
	}
	var list []game.Reward
	if err := json.Unmarshal(raw, &list); err != nil {
		return nil, err
	}
	for _, r := range list {
		if r.Type != "" && r.Amount != 0 {
			rewards[r.Type] += r.Amount
		}
	}
	return rewards, nil
}
