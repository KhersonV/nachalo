package game

import "testing"

func TestCalculateResults_DamageAndExpFormula(t *testing.T) {
	const instanceID = "test-instance-results"
	const userID = 42

	MatchStatesMu.Lock()
	MatchStates[instanceID] = &MatchState{
		InstanceID: instanceID,
		KillEvents: []KillEvent{
			{KillerID: userID, VictimType: "monster", Damage: 99},
			{KillerID: userID, VictimType: "player", Damage: 77},
		},
		DamageEvents: []DamageEvent{
			{DealerID: userID, TargetType: "monster", Amount: 24},
			{DealerID: userID, TargetType: "player", Amount: 11},
			{DealerID: 99, TargetType: "monster", Amount: 1000},
		},
	}
	MatchStatesMu.Unlock()

	t.Cleanup(func() {
		MatchStatesMu.Lock()
		delete(MatchStates, instanceID)
		MatchStatesMu.Unlock()
	})

	exp, rewards, playerKills, monsterKills, dmgTotal, dmgPlayers, dmgMonsters, err := CalculateResults(instanceID, userID, true)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if playerKills != 1 {
		t.Fatalf("expected playerKills=1, got %d", playerKills)
	}
	if monsterKills != 1 {
		t.Fatalf("expected monsterKills=1, got %d", monsterKills)
	}

	if dmgPlayers != 11 {
		t.Fatalf("expected dmgPlayers=11, got %d", dmgPlayers)
	}
	if dmgMonsters != 24 {
		t.Fatalf("expected dmgMonsters=24, got %d", dmgMonsters)
	}
	if dmgTotal != 35 {
		t.Fatalf("expected dmgTotal=35 (players+monsters), got %d", dmgTotal)
	}

	// XP = floor(24/5)*1 + floor(11/5)*2 = 4 + 4 = 8
	if exp != 8 {
		t.Fatalf("expected exp=8, got %d", exp)
	}

	// Coins (survived=true): 20 + 1*20 + 1*8 + floor(35/50)=48
	if len(rewards) != 1 {
		t.Fatalf("expected exactly 1 reward entry, got %d", len(rewards))
	}
	if rewards[0].Type != "balance" {
		t.Fatalf("expected reward type balance, got %q", rewards[0].Type)
	}
	if rewards[0].Amount != 48 {
		t.Fatalf("expected reward amount=48, got %d", rewards[0].Amount)
	}
}

func TestCalculateResults_NoBaseCoinsForDeadPlayer(t *testing.T) {
	const instanceID = "test-instance-results-dead"
	const userID = 7

	MatchStatesMu.Lock()
	MatchStates[instanceID] = &MatchState{
		InstanceID: instanceID,
		KillEvents: []KillEvent{
			{KillerID: userID, VictimType: "monster", Damage: 10},
		},
		DamageEvents: []DamageEvent{
			{DealerID: userID, TargetType: "monster", Amount: 10},
		},
	}
	MatchStatesMu.Unlock()

	t.Cleanup(func() {
		MatchStatesMu.Lock()
		delete(MatchStates, instanceID)
		MatchStatesMu.Unlock()
	})

	_, rewards, _, _, _, _, _, err := CalculateResults(instanceID, userID, false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Coins (survived=false): 0 + 0*20 + 1*8 + floor(10/50)=8
	if len(rewards) != 1 {
		t.Fatalf("expected exactly 1 reward entry, got %d", len(rewards))
	}
	if rewards[0].Amount != 8 {
		t.Fatalf("expected reward amount=8, got %d", rewards[0].Amount)
	}
}
