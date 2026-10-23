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
