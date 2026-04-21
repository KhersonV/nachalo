package main

import (
	"reflect"
	"testing"
)

func testQueueEntry(leaderID int, members ...int) QueueEntry {
	return QueueEntry{
		PlayerID:  leaderID,
		LeaderID:  leaderID,
		SeedID:    buildQueueSeedID("", leaderID),
		MemberIDs: append([]int(nil), members...),
		PartySize: len(members),
	}
}

func TestFindMatchCandidates_2x2MixedSoloAndPremade(t *testing.T) {
	queue := []QueueEntry{
		testQueueEntry(1, 1),
		{
			PlayerID:  2,
			LeaderID:  2,
			PartyID:   "party-23",
			SeedID:    "party-23",
			MemberIDs: []int{2, 3},
			PartySize: 2,
		},
		testQueueEntry(4, 4),
	}

	selected, plan, ok := findMatchCandidates("2x2", queue)
	if !ok {
		t.Fatal("expected a valid 2x2 match plan")
	}
	if len(selected) != len(queue) {
		t.Fatalf("expected %d selected entries, got %d", len(queue), len(selected))
	}

	wantTeams := []MatchTeam{
		{TeamID: 1, MemberIDs: []int{1, 4}},
		{TeamID: 2, MemberIDs: []int{2, 3}},
	}
	if !reflect.DeepEqual(plan.Teams, wantTeams) {
		t.Fatalf("unexpected teams: got %+v want %+v", plan.Teams, wantTeams)
	}

	if !reflect.DeepEqual(plan.PlayerIDs, []int{1, 4, 2, 3}) {
		t.Fatalf("unexpected player ids: %v", plan.PlayerIDs)
	}
	if !reflect.DeepEqual(plan.GroupIDs, []int{1, 1, 2, 2}) {
		t.Fatalf("unexpected group ids: %v", plan.GroupIDs)
	}
	if !reflect.DeepEqual(plan.TurnOrder, []int{1, 2, 4, 3}) {
		t.Fatalf("unexpected turn order: %v", plan.TurnOrder)
	}
}

func TestFindMatchCandidates_3x3MixedSoloAndPremade(t *testing.T) {
	queue := []QueueEntry{
		{
			PlayerID:  1,
			LeaderID:  1,
			PartyID:   "party-12",
			SeedID:    "party-12",
			MemberIDs: []int{1, 2},
			PartySize: 2,
		},
		testQueueEntry(3, 3),
		testQueueEntry(4, 4),
		{
			PlayerID:  5,
			LeaderID:  5,
			PartyID:   "party-56",
			SeedID:    "party-56",
			MemberIDs: []int{5, 6},
			PartySize: 2,
		},
	}

	_, plan, ok := findMatchCandidates("3x3", queue)
	if !ok {
		t.Fatal("expected a valid 3x3 match plan")
	}

	wantTeams := []MatchTeam{
		{TeamID: 1, MemberIDs: []int{1, 2, 3}},
		{TeamID: 2, MemberIDs: []int{4, 5, 6}},
	}
	if !reflect.DeepEqual(plan.Teams, wantTeams) {
		t.Fatalf("unexpected teams: got %+v want %+v", plan.Teams, wantTeams)
	}
	if !reflect.DeepEqual(plan.TurnOrder, []int{1, 4, 2, 5, 3, 6}) {
		t.Fatalf("unexpected turn order: %v", plan.TurnOrder)
	}
}

func TestFindMatchCandidates_5x5MixedSoloAndPremade(t *testing.T) {
	queue := []QueueEntry{
		{
			PlayerID:  1,
			LeaderID:  1,
			PartyID:   "party-12",
			SeedID:    "party-12",
			MemberIDs: []int{1, 2},
			PartySize: 2,
		},
		{
			PlayerID:  3,
			LeaderID:  3,
			PartyID:   "party-34",
			SeedID:    "party-34",
			MemberIDs: []int{3, 4},
			PartySize: 2,
		},
		testQueueEntry(5, 5),
		{
			PlayerID:  6,
			LeaderID:  6,
			PartyID:   "party-67",
			SeedID:    "party-67",
			MemberIDs: []int{6, 7},
			PartySize: 2,
		},
		{
			PlayerID:  8,
			LeaderID:  8,
			PartyID:   "party-89",
			SeedID:    "party-89",
			MemberIDs: []int{8, 9},
			PartySize: 2,
		},
		testQueueEntry(10, 10),
	}

	_, plan, ok := findMatchCandidates("5x5", queue)
	if !ok {
		t.Fatal("expected a valid 5x5 match plan")
	}

	wantTeams := []MatchTeam{
		{TeamID: 1, MemberIDs: []int{1, 2, 3, 4, 5}},
		{TeamID: 2, MemberIDs: []int{6, 7, 8, 9, 10}},
	}
	if !reflect.DeepEqual(plan.Teams, wantTeams) {
		t.Fatalf("unexpected teams: got %+v want %+v", plan.Teams, wantTeams)
	}
	if !reflect.DeepEqual(plan.TurnOrder, []int{1, 6, 2, 7, 3, 8, 4, 9, 5, 10}) {
		t.Fatalf("unexpected turn order: %v", plan.TurnOrder)
	}
}
