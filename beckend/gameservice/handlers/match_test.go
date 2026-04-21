package handlers

import "testing"

func TestNormalizeMatchPlan_UsesExplicitTeamsInsteadOfPlayerIndex(t *testing.T) {
	req := RequestMatch{
		InstanceID:   "match-1",
		Mode:         "2x2",
		PlayerIDs:    []int{1, 2, 3, 4},
		Teams:        []RequestMatchTeam{{TeamID: 2, MemberIDs: []int{2, 3}}, {TeamID: 1, MemberIDs: []int{1, 4}}},
		TurnOrder:    []int{1, 2, 4, 3},
		TeamsCount:   2,
		TotalPlayers: 4,
	}

	plan, err := normalizeMatchPlan(req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	wantPlayerIDs := []int{1, 4, 2, 3}
	if !equalIntSlices(plan.PlayerIDs, wantPlayerIDs) {
		t.Fatalf("unexpected player ids: got %v want %v", plan.PlayerIDs, wantPlayerIDs)
	}

	wantTurnOrder := []int{1, 2, 4, 3}
	if !equalIntSlices(plan.TurnOrder, wantTurnOrder) {
		t.Fatalf("unexpected turn order: got %v want %v", plan.TurnOrder, wantTurnOrder)
	}

	if plan.GroupByPlayer[1] != 1 || plan.GroupByPlayer[4] != 1 {
		t.Fatalf("expected players 1 and 4 to be in team 1, got %+v", plan.GroupByPlayer)
	}
	if plan.GroupByPlayer[2] != 2 || plan.GroupByPlayer[3] != 2 {
		t.Fatalf("expected players 2 and 3 to be in team 2, got %+v", plan.GroupByPlayer)
	}
}

func TestMapStartPositionsToPlayers_AssignsTeamClusters(t *testing.T) {
	tests := []struct {
		name       string
		teams      []RequestMatchTeam
		starts     [][2]int
		wantStarts map[int][2]int
	}{
		{
			name: "2x2 mixed solo premade",
			teams: []RequestMatchTeam{
				{TeamID: 1, MemberIDs: []int{1, 4}},
				{TeamID: 2, MemberIDs: []int{2, 3}},
			},
			starts: [][2]int{{0, 0}, {1, 0}, {9, 9}, {10, 9}},
			wantStarts: map[int][2]int{
				1: {0, 0},
				4: {1, 0},
				2: {9, 9},
				3: {10, 9},
			},
		},
		{
			name: "3x3 mixed solo premade",
			teams: []RequestMatchTeam{
				{TeamID: 1, MemberIDs: []int{1, 2, 3}},
				{TeamID: 2, MemberIDs: []int{4, 5, 6}},
			},
			starts: [][2]int{{0, 0}, {1, 0}, {2, 0}, {9, 9}, {10, 9}, {11, 9}},
			wantStarts: map[int][2]int{
				1: {0, 0},
				2: {1, 0},
				3: {2, 0},
				4: {9, 9},
				5: {10, 9},
				6: {11, 9},
			},
		},
		{
			name: "5x5 mixed solo premade",
			teams: []RequestMatchTeam{
				{TeamID: 1, MemberIDs: []int{1, 2, 3, 4, 5}},
				{TeamID: 2, MemberIDs: []int{6, 7, 8, 9, 10}},
			},
			starts: [][2]int{
				{0, 0}, {1, 0}, {2, 0}, {3, 0}, {4, 0},
				{10, 10}, {11, 10}, {12, 10}, {13, 10}, {14, 10},
			},
			wantStarts: map[int][2]int{
				1:  {0, 0},
				2:  {1, 0},
				3:  {2, 0},
				4:  {3, 0},
				5:  {4, 0},
				6:  {10, 10},
				7:  {11, 10},
				8:  {12, 10},
				9:  {13, 10},
				10: {14, 10},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			startByPlayer, err := mapStartPositionsToPlayers(tt.teams, tt.starts)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if len(startByPlayer) != len(tt.wantStarts) {
				t.Fatalf("unexpected start map size: got %d want %d", len(startByPlayer), len(tt.wantStarts))
			}
			for playerID, want := range tt.wantStarts {
				got, ok := startByPlayer[playerID]
				if !ok {
					t.Fatalf("missing start position for player %d", playerID)
				}
				if got != want {
					t.Fatalf("unexpected start for player %d: got %v want %v", playerID, got, want)
				}
			}
		})
	}
}
