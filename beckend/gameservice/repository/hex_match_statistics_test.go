package repository

import (
	"encoding/json"
	"testing"

	"gameservice/game"
)

func TestHexFinalRewardsPayOnlyDifference(t *testing.T) {
	if hexPayoutDelta(120, 120) != 0 || hexPayoutDelta(100, 125) != 0 {
		t.Fatal("already paid reward would be duplicated")
	}
	if hexPayoutDelta(220, 100) != 120 || hexPayoutDelta(30, 0) != 30 {
		t.Fatal("new reward was not paid")
	}
}

func TestHexWinnerBonusAndClassIdentity(t *testing.T) {
	results := &game.MatchResults{WinnerGroupID: 2, PlayerResults: []game.PlayerResult{
		{UserID: 1, CharacterType: "guardian", RewardsData: json.RawMessage(`[]`)},
		{UserID: 2, CharacterType: "ranger", RewardsData: json.RawMessage(`[]`)},
		{UserID: 3, CharacterType: "mystic", RewardsData: json.RawMessage(`[]`)},
	}}
	applyWinnerBonusesAndPlacements(results, map[int]int{1: 1, 2: 2, 3: 2}, nil)
	if results.PlayerResults[0].IsWinner || results.PlayerResults[0].ExpGained != 0 {
		t.Fatal("loser received winner bonus")
	}
	for _, result := range results.PlayerResults[1:] {
		if !result.IsWinner || result.ExpGained != 120 {
			t.Fatalf("winner bonus missing: %+v", result)
		}
		rewards, err := parseFinalRewards(result.RewardsData)
		if err != nil || rewards["balance"] != 100 {
			t.Fatalf("winner currency missing: %v %v", rewards, err)
		}
	}
	if results.PlayerResults[0].CharacterType != "guardian" || results.PlayerResults[1].CharacterType != "ranger" || results.PlayerResults[2].CharacterType != "mystic" {
		t.Fatal("class identity changed")
	}
}
