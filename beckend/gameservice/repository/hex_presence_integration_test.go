package repository

import (
	"context"
	"errors"
	"testing"

	"gameservice/internal/hexmap"
)

func TestHexPresenceLightningPostgres(t *testing.T) {
	db, _ := hexTestDatabase(t)
	ctx := context.Background()
	if _, err := CreatePlayerProfile(CreatePlayerProfileInput{UserID: 702, Name: "Absent player", CharacterType: "guardian"}); err != nil {
		t.Fatal(err)
	}
	id := "hex-presence-lightning"
	_, err := CreateHexMatch(ctx, HexMatchPlan{InstanceID: id, Mode: "1x1", Teams: []hexmap.Team{{ID: 1, PlayerIDs: []int{701}}, {ID: 2, PlayerIDs: []int{702}}}, TurnOrder: []int{701, 702}, Seed: 121})
	if err != nil {
		t.Fatal(err)
	}
	if changed, err := SetHexPresence(ctx, id, 701, true); err != nil || !changed {
		t.Fatalf("first player join: %v changed=%v", err, changed)
	}
	if changed, err := SetHexPresence(ctx, id, 702, true); err != nil || !changed {
		t.Fatalf("second player join: %v changed=%v", err, changed)
	}
	if changed, err := SetHexPresence(ctx, id, 702, false); err != nil || !changed {
		t.Fatalf("disconnect: %v changed=%v", err, changed)
	}
	if changed, err := SetHexPresence(ctx, id, 702, false); err != nil || changed {
		t.Fatalf("duplicate disconnect: %v changed=%v", err, changed)
	}
	view, err := LoadHexTestMatch(ctx, id, 701)
	if err != nil || len(view.DisconnectedPlayers) != 1 || view.DisconnectedPlayers[0].UserID != 702 {
		t.Fatalf("disconnect snapshot: %+v %v", view.DisconnectedPlayers, err)
	}
	if changed, err := SetHexPresence(ctx, id, 702, true); err != nil || !changed {
		t.Fatalf("reconnect: %v changed=%v", err, changed)
	}
	if err := MarkHexPresenceUnknownOnStart(ctx); err != nil {
		t.Fatalf("restart presence: %v", err)
	}
	view, err = LoadHexTestMatch(ctx, id, 701)
	if err != nil || len(view.DisconnectedPlayers) != 2 {
		t.Fatalf("restart snapshot: %+v %v", view.DisconnectedPlayers, err)
	}
	if changed, err := SetHexPresence(ctx, id, 702, true); err != nil || !changed {
		t.Fatalf("restart reconnect: %v changed=%v", err, changed)
	}
	if _, err := ApplyHexLightning(ctx, id, 702, nil); !errors.Is(err, ErrCommandRejected) {
		t.Fatalf("early lightning: %v", err)
	}
	if _, err := db.Exec(`UPDATE matches SET hex_turn_deadline=now()-interval '1 second' WHERE instance_id=$1`, id); err != nil {
		t.Fatal(err)
	}
	turn, err := ApplyHexExpiredTurn(ctx, id, 701, "presence-expired-turn", 1, func(string) int { return 1 }, nil)
	if err != nil || turn.ActiveUser != 702 || !turn.TimedOut {
		t.Fatalf("expired turn: %+v %v", turn, err)
	}
	if changed, err := SetHexPresence(ctx, id, 702, false); err != nil || !changed {
		t.Fatalf("second disconnect: %v changed=%v", err, changed)
	}
	if _, err := db.Exec(`UPDATE match_players SET hex_disconnected_at=now()-interval '4 minutes' WHERE instance_id=$1 AND user_id=702`, id); err != nil {
		t.Fatal(err)
	}
	strike, err := ApplyHexLightning(ctx, id, 702, nil)
	if err != nil || strike.UserID != 702 {
		t.Fatalf("lightning: %+v %v", strike, err)
	}
	if _, err := ApplyHexLightning(ctx, id, 702, nil); err != nil {
		t.Fatalf("replay: %v", err)
	}
	view, err = LoadHexTestMatch(ctx, id, 701)
	if err != nil || view.Finished || view.ActiveUser != 701 || len(view.TurnOrder) != 1 || view.TurnOrder[0] != 701 {
		t.Fatalf("survivor view: %+v %v", view.TurnOrder, err)
	}
	defeated, err := LoadHexTestMatch(ctx, id, 702)
	if err != nil || defeated.Player.Health != 0 || len(defeated.Result) == 0 {
		t.Fatalf("defeat result: %+v %v", defeated.Result, err)
	}
	if _, err := CreateHexMatch(ctx, HexMatchPlan{InstanceID: "hex-presence-next", Mode: "1x1", Teams: []hexmap.Team{{ID: 1, PlayerIDs: []int{702}}}, TurnOrder: []int{702}, Seed: 122}); err != nil {
		t.Fatalf("new match after defeat: %v", err)
	}
}
