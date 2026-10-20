package repository

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sync"
	"testing"

	"gameservice/game"
	"gameservice/internal/hexgrid"
	"gameservice/internal/hexmap"
)

func placeTestBarrel(t *testing.T, instanceID string, pos hexgrid.Coord) {
	t.Helper()
	var raw []byte
	if err := DB.QueryRow(`SELECT barbel FROM match_map_cells WHERE instance_id=$1 AND barbel IS NOT NULL LIMIT 1`, instanceID).Scan(&raw); err != nil {
		t.Fatal(err)
	}
	if _, err := DB.Exec(`UPDATE match_map_cells SET barbel=$4,tile_code=66 WHERE instance_id=$1 AND q=$2 AND r=$3`, instanceID, pos.Q, pos.R, raw); err != nil {
		t.Fatal(err)
	}
}

func giveHexItem(t *testing.T, instanceID string, itemType string, itemID int) {
	t.Helper()
	tx, err := DB.Begin()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := AddInventoryItemTx(tx, instanceID, 701, itemType, itemID, fmt.Sprintf("%s %d", itemType, itemID), "", "", 1); err != nil {
		tx.Rollback()
		t.Fatal(err)
	}
	if err := tx.Commit(); err != nil {
		t.Fatal(err)
	}
}

func TestHexBarrelOutcomesPostgres(t *testing.T) {
	db, _ := hexTestDatabase(t)
	ctx := context.Background()
	oldRoll := hexBarrelRoll
	defer func() { hexBarrelRoll = oldRoll }()
	for _, outcome := range []string{"damage", "resource", "artifact"} {
		t.Run(outcome, func(t *testing.T) {
			s, err := CreateHexTestMatch(ctx, 701, int64(100+len(outcome)))
			if err != nil {
				t.Fatal(err)
			}
			id := s.InstanceID
			near := walkableNeighbor(s)
			placeTestBarrel(t, id, near)
			published := 0
			if _, err := ApplyHexBarrel(ctx, id, 701, "too-far", near, func(HexBarrelResult) { published++ }); !errors.Is(err, ErrHexInteractionPosition) {
				t.Fatalf("position: %v", err)
			}
			var state, mapRev, ledger int
			if err := db.QueryRow(`SELECT state_revision,map_revision FROM matches WHERE instance_id=$1`, id).Scan(&state, &mapRev); err != nil {
				t.Fatal(err)
			}
			if err := db.QueryRow(`SELECT count(*) FROM match_commands WHERE instance_id=$1`, id).Scan(&ledger); err != nil {
				t.Fatal(err)
			}
			if state != 1 || mapRev != 1 || ledger != 0 || published != 0 {
				t.Fatalf("remote barrel changed state: %d/%d ledger=%d pub=%d", state, mapRev, ledger, published)
			}
			if _, err := ApplyHexMove(ctx, id, 701, "move-to-barrel", near); err != nil {
				t.Fatalf("barrel must be walkable: %v", err)
			}
			hexBarrelRoll = func(_ game.FullCell, resources, artifacts []game.ResourceData) (any, error) {
				switch outcome {
				case "damage":
					return game.DamageEvent{SourceType: game.ResultSourceEnvironment, Amount: 4}, nil
				case "resource":
					for _, v := range resources {
						if v.Type != "barrel" {
							return v, nil
						}
					}
				case "artifact":
					if len(artifacts) > 0 {
						return artifacts[0], nil
					}
				}
				return nil, fmt.Errorf("empty catalog")
			}
			result, err := ApplyHexBarrel(ctx, id, 701, "open", near, func(HexBarrelResult) { published++ })
			if err != nil {
				t.Fatal(err)
			}
			if result.Outcome != outcome || result.StateRevision != 3 || result.MapRevision != 3 || published != 1 {
				t.Fatalf("result %+v pub=%d", result, published)
			}
			replay, err := ApplyHexBarrel(ctx, id, 701, "open", near, func(HexBarrelResult) { published++ })
			if err != nil || !replay.Replayed || published != 1 {
				t.Fatalf("replay %+v %v pub=%d", replay, err, published)
			}
			if _, err := ApplyHexBarrel(ctx, id, 701, "open-again", near, nil); !errors.Is(err, ErrCommandRejected) {
				t.Fatalf("barrel repeated: %v", err)
			}
			var remaining int
			if err := db.QueryRow(`SELECT count(*) FROM match_map_cells WHERE instance_id=$1 AND q=$2 AND r=$3 AND barbel IS NOT NULL`, id, near.Q, near.R).Scan(&remaining); err != nil || remaining != 0 {
				t.Fatalf("barrel remains %d %v", remaining, err)
			}
			if outcome == "damage" {
				var events game.ResultEvents
				var raw []byte
				if err := db.QueryRow(`SELECT combat_result_events FROM matches WHERE instance_id=$1`, id).Scan(&raw); err != nil {
					t.Fatal(err)
				}
				if err := json.Unmarshal(raw, &events); err != nil || len(events.DamageEvents) != 1 || events.DamageEvents[0].Amount != 4 {
					t.Fatalf("damage events %+v %v", events, err)
				}
			}
			if outcome != "damage" {
				var count int
				if err := db.QueryRow(`SELECT item_count FROM inventory_items WHERE instance_id=$1 AND user_id=701 AND item_type=$2 AND item_id=$3`, id, result.ItemType, result.ItemID).Scan(&count); err != nil || count != 1 {
					t.Fatalf("inventory %d %v", count, err)
				}
			}
		})
	}
}

func TestHexBarrelRollbackPostgres(t *testing.T) {
	db, _ := hexTestDatabase(t)
	ctx := context.Background()
	s, err := CreateHexTestMatch(ctx, 701, 165)
	if err != nil {
		t.Fatal(err)
	}
	id := s.InstanceID
	placeTestBarrel(t, id, s.Player.Position)
	old := hexBarrelRoll
	hexBarrelRoll = func(_ game.FullCell, resources, artifacts []game.ResourceData) (any, error) { return artifacts[0], nil }
	defer func() { hexBarrelRoll = old }()
	if _, err := db.Exec(`CREATE FUNCTION fail_hex_barrel() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN IF NEW.command_id='fail-barrel' THEN RAISE EXCEPTION 'rollback'; END IF; RETURN NEW; END $$`); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`CREATE TRIGGER fail_hex_barrel BEFORE INSERT ON match_commands FOR EACH ROW EXECUTE FUNCTION fail_hex_barrel()`); err != nil {
		t.Fatal(err)
	}
	pub := 0
	if _, err := ApplyHexBarrel(ctx, id, 701, "fail-barrel", s.Player.Position, func(HexBarrelResult) { pub++ }); err == nil {
		t.Fatal("expected rollback")
	}
	var remain, items, state int
	_ = db.QueryRow(`SELECT count(*) FROM match_map_cells WHERE instance_id=$1 AND q=$2 AND r=$3 AND barbel IS NOT NULL`, id, s.Player.Position.Q, s.Player.Position.R).Scan(&remain)
	_ = db.QueryRow(`SELECT count(*) FROM inventory_items WHERE instance_id=$1`, id).Scan(&items)
	_ = db.QueryRow(`SELECT state_revision FROM matches WHERE instance_id=$1`, id).Scan(&state)
	if remain != 1 || items != 0 || state != 1 || pub != 0 {
		t.Fatalf("rollback barrel=%d items=%d rev=%d pub=%d", remain, items, state, pub)
	}
}

func TestHexUseItemPostgres(t *testing.T) {
	db, _ := hexTestDatabase(t)
	ctx := context.Background()
	s, err := CreateHexTestMatch(ctx, 701, 172)
	if err != nil {
		t.Fatal(err)
	}
	id := s.InstanceID
	var food, water int
	if err := db.QueryRow(`SELECT id FROM resources WHERE type='food' LIMIT 1`).Scan(&food); err != nil {
		t.Fatal(err)
	}
	if err := db.QueryRow(`SELECT id FROM resources WHERE type='water' LIMIT 1`).Scan(&water); err != nil {
		t.Fatal(err)
	}
	for _, item := range []struct {
		typ string
		id  int
	}{{"resource", food}, {"resource", water}, {"scroll", 2001}, {"scroll", 2002}, {"scroll", 2005}, {"scroll", 2006}} {
		giveHexItem(t, id, item.typ, item.id)
	}
	if _, err := db.Exec(`UPDATE match_players SET health=max_health-10,energy=max_energy-10 WHERE instance_id=$1 AND user_id=701`, id); err != nil {
		t.Fatal(err)
	}
	pub := 0
	apply := func(command, typ string, item int) HexItemResult {
		t.Helper()
		result, err := ApplyHexItem(ctx, id, 701, command, typ, item, func(HexItemResult) { pub++ })
		if err != nil {
			t.Fatal(err)
		}
		return result
	}
	f := apply("food", "resource", food)
	if f.Health < s.Player.Health-10 {
		t.Fatalf("food did not heal: %+v", f)
	}
	if _, err := ApplyHexItem(ctx, id, 701, "food-again", "resource", food, nil); !errors.Is(err, ErrHexFoodCooldown) {
		t.Fatalf("food cooldown: %v", err)
	}
	w := apply("water", "resource", water)
	if w.Energy < s.Player.Energy-10 {
		t.Fatalf("water did not restore energy: %+v", w)
	}
	for _, item := range []int{2001, 2002, 2005, 2006} {
		result := apply(fmt.Sprintf("scroll-%d", item), "scroll", item)
		if result.Axis == "" {
			t.Fatalf("scroll result: %+v", result)
		}
	}
	replay, err := ApplyHexItem(ctx, id, 701, "food", "resource", food, func(HexItemResult) { pub++ })
	if err != nil || !replay.Replayed || pub != 6 {
		t.Fatalf("replay %+v %v pub=%d", replay, err, pub)
	}
	var state, mapRev, ledger int
	_ = db.QueryRow(`SELECT state_revision,map_revision FROM matches WHERE instance_id=$1`, id).Scan(&state, &mapRev)
	_ = db.QueryRow(`SELECT count(*) FROM match_commands WHERE instance_id=$1`, id).Scan(&ledger)
	if state != 7 || mapRev != 1 || ledger != 6 {
		t.Fatalf("item rev %d/%d ledger=%d", state, mapRev, ledger)
	}
	if _, err := ApplyHexItem(ctx, id, 701, "blueprint", "blueprint", 1, nil); !errors.Is(err, ErrCommandRejected) {
		t.Fatalf("blueprint consumed: %v", err)
	}
}

func TestHexBarrelFatalAndItemRollbackPostgres(t *testing.T) {
	db, _ := hexTestDatabase(t)
	ctx := context.Background()
	s, err := CreateHexTestMatch(ctx, 701, 191)
	if err != nil {
		t.Fatal(err)
	}
	id := s.InstanceID
	placeTestBarrel(t, id, s.Player.Position)
	old := hexBarrelRoll
	hexBarrelRoll = func(_ game.FullCell, _, _ []game.ResourceData) (any, error) {
		return game.DamageEvent{SourceType: game.ResultSourceEnvironment, Amount: 5}, nil
	}
	defer func() { hexBarrelRoll = old }()
	if _, err := db.Exec(`UPDATE match_players SET health=3 WHERE instance_id=$1 AND user_id=701`, id); err != nil {
		t.Fatal(err)
	}
	result, err := ApplyHexBarrel(ctx, id, 701, "fatal", s.Player.Position, nil)
	if err != nil || result.Health != 0 {
		t.Fatalf("fatal %+v %v", result, err)
	}
	var active, state, mapRev int
	var finished bool
	var order []byte
	if err := db.QueryRow(`SELECT active_user_id,turn_order,finished_at IS NOT NULL,state_revision,map_revision FROM matches WHERE instance_id=$1`, id).Scan(&active, &order, &finished, &state, &mapRev); err != nil {
		t.Fatal(err)
	}
	if active != 0 || string(order) != "[]" || !finished || state != 2 || mapRev != 2 {
		t.Fatalf("death active=%d order=%s finished=%v rev=%d/%d", active, order, finished, state, mapRev)
	}
	finalSnapshot, err := LoadHexTestMatch(ctx, id, 701)
	if err != nil || !finalSnapshot.Finished || finalSnapshot.Player.Health != 0 || finalSnapshot.StateRevision != 2 {
		t.Fatalf("fatal reload %+v %v", finalSnapshot.Player, err)
	}
	var events game.ResultEvents
	var raw []byte
	if err := db.QueryRow(`SELECT combat_result_events FROM matches WHERE instance_id=$1`, id).Scan(&raw); err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(raw, &events); err != nil || len(events.DamageEvents) != 1 || len(events.KillEvents) != 1 || len(events.DefeatedUsers) != 1 {
		t.Fatalf("fatal events %+v %v", events, err)
	}
	replay, err := ApplyHexBarrel(ctx, id, 701, "fatal", s.Player.Position, nil)
	if err != nil || !replay.Replayed {
		t.Fatalf("fatal replay %+v %v", replay, err)
	}

	s2, err := CreateHexTestMatch(ctx, 701, 192)
	if err != nil {
		t.Fatal(err)
	}
	id = s2.InstanceID
	var water int
	if err := db.QueryRow(`SELECT id FROM resources WHERE type='water' LIMIT 1`).Scan(&water); err != nil {
		t.Fatal(err)
	}
	giveHexItem(t, id, "resource", water)
	if _, err := db.Exec(`CREATE FUNCTION fail_hex_item() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN IF NEW.command_id='fail-item' THEN RAISE EXCEPTION 'rollback'; END IF; RETURN NEW; END $$`); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`CREATE TRIGGER fail_hex_item BEFORE INSERT ON match_commands FOR EACH ROW EXECUTE FUNCTION fail_hex_item()`); err != nil {
		t.Fatal(err)
	}
	pub := 0
	if _, err := ApplyHexItem(ctx, id, 701, "fail-item", "resource", water, func(HexItemResult) { pub++ }); err == nil {
		t.Fatal("expected item rollback")
	}
	var inventory []byte
	var count int
	_ = db.QueryRow(`SELECT inventory FROM match_players WHERE instance_id=$1 AND user_id=701`, id).Scan(&inventory)
	_ = db.QueryRow(`SELECT item_count FROM inventory_items WHERE instance_id=$1 AND user_id=701 AND item_type='resource' AND item_id=$2`, id, water).Scan(&count)
	_ = db.QueryRow(`SELECT state_revision FROM matches WHERE instance_id=$1`, id).Scan(&state)
	if count != 1 || state != 1 || pub != 0 || !json.Valid(inventory) {
		t.Fatalf("item rollback count=%d rev=%d pub=%d inv=%s", count, state, pub, inventory)
	}
}

func TestHexItemConcurrentAndPlayerScrollsPostgres(t *testing.T) {
	db, _ := hexTestDatabase(t)
	ctx := context.Background()
	if _, err := CreatePlayerProfile(CreatePlayerProfileInput{UserID: 702, Name: "Other", CharacterType: "guardian"}); err != nil {
		t.Fatal(err)
	}
	plan := HexMatchPlan{InstanceID: "hex-item-multi", Mode: "1x1", Teams: []hexmap.Team{{ID: 1, PlayerIDs: []int{701}}, {ID: 2, PlayerIDs: []int{702}}}, TurnOrder: []int{701, 702}, Seed: 210}
	s, err := CreateHexMatch(ctx, plan)
	if err != nil {
		t.Fatal(err)
	}
	for _, id := range []int{2003, 2004} {
		giveHexItem(t, s.InstanceID, "scroll", id)
	}
	for _, id := range []int{2003, 2004} {
		r, err := ApplyHexItem(ctx, s.InstanceID, 701, fmt.Sprintf("player-scroll-%d", id), "scroll", id, nil)
		if err != nil {
			t.Fatal(err)
		}
		expected := s.Starts[1].Position.Q
		if id == 2004 {
			expected = s.Starts[1].Position.R
		}
		if r.Value != expected {
			t.Fatalf("scroll %d got %d want %d", id, r.Value, expected)
		}
	}
	var water int
	if err := db.QueryRow(`SELECT id FROM resources WHERE type='water' LIMIT 1`).Scan(&water); err != nil {
		t.Fatal(err)
	}
	giveHexItem(t, s.InstanceID, "resource", water)
	var wg sync.WaitGroup
	results := make(chan error, 2)
	for _, command := range []string{"concurrent-water-a", "concurrent-water-b"} {
		wg.Add(1)
		go func(command string) {
			defer wg.Done()
			_, err := ApplyHexItem(ctx, s.InstanceID, 701, command, "resource", water, nil)
			results <- err
		}(command)
	}
	wg.Wait()
	close(results)
	succeeded, rejected := 0, 0
	for err := range results {
		if err == nil {
			succeeded++
		} else if errors.Is(err, ErrCommandRejected) {
			rejected++
		} else {
			t.Fatal(err)
		}
	}
	if succeeded != 1 || rejected != 1 {
		t.Fatalf("concurrent consume %d/%d", succeeded, rejected)
	}
	var count int
	if err := db.QueryRow(`SELECT count(*) FROM match_commands WHERE instance_id=$1 AND response->>'itemId'=$2`, s.InstanceID, fmt.Sprint(water)).Scan(&count); err != nil || count != 1 {
		t.Fatalf("double ledger %d %v", count, err)
	}
	for i := 0; i < 5; i++ {
		giveHexItem(t, s.InstanceID, "resource", water)
	}
	for i := 2; i <= 5; i++ {
		if _, err := ApplyHexItem(ctx, s.InstanceID, 701, fmt.Sprintf("water-%d", i), "resource", water, nil); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := ApplyHexItem(ctx, s.InstanceID, 701, "water-6", "resource", water, nil); !errors.Is(err, ErrHexWaterLimit) {
		t.Fatalf("water limit: %v", err)
	}
	placeTestBarrel(t, s.InstanceID, s.Player.Position)
	old := hexBarrelRoll
	hexBarrelRoll = func(_ game.FullCell, _, _ []game.ResourceData) (any, error) {
		return game.DamageEvent{SourceType: game.ResultSourceEnvironment, Amount: 3}, nil
	}
	defer func() { hexBarrelRoll = old }()
	barrel, err := ApplyHexBarrel(ctx, s.InstanceID, 701, "multi-barrel", s.Player.Position, nil)
	if err != nil || barrel.Outcome != "damage" {
		t.Fatalf("ordinary multiplayer barrel %+v %v", barrel, err)
	}
}
