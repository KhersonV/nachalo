package handlers

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"testing"

	"gameservice/game"
	"gameservice/internal/hexgrid"
	"gameservice/internal/hexmap"
	"gameservice/repository"
)

func TestHexMultiplayerLethalCounterPostgres(t *testing.T) {
	db, _ := hexCombatDatabase(t)
	if _, err := repository.CreatePlayerProfile(repository.CreatePlayerProfileInput{UserID: 782, Name: "HEX survivor", CharacterType: "guardian"}); err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	id := "hex-two-death"
	snapshot, err := repository.CreateHexMatch(ctx, repository.HexMatchPlan{InstanceID: id, Mode: "1x1", Teams: []hexmap.Team{{ID: 1, PlayerIDs: []int{781}}, {ID: 2, PlayerIDs: []int{782}}}, TurnOrder: []int{781, 782}, Seed: 97})
	if err != nil {
		t.Fatal(err)
	}
	walls := map[hexgrid.EdgeKey]bool{}
	for _, edge := range snapshot.Edges {
		next, _ := hexgrid.Neighbor(edge.Position, edge.Direction)
		key, _ := hexgrid.CanonicalEdge(edge.Position, next)
		walls[key] = true
	}
	occupied := map[hexgrid.Coord]bool{}
	for _, actor := range snapshot.Actors {
		occupied[actor.Position] = true
	}
	var playerPos hexgrid.Coord
	var monsterID int
	found := false
	for _, actor := range snapshot.Actors {
		if actor.Kind != "monster" {
			continue
		}
		for _, cell := range snapshot.Cells {
			if cell.Terrain != "floor" || cell.Barrel != nil || occupied[cell.Position] {
				continue
			}
			distance, _ := hexgrid.Distance(cell.Position, actor.Position)
			key, _ := hexgrid.CanonicalEdge(cell.Position, actor.Position)
			if distance == 1 && !walls[key] {
				if _, err := fmt.Sscanf(actor.ID, "monster-%d", &monsterID); err != nil {
					t.Fatal(err)
				}
				playerPos = cell.Position
				found = true
				break
			}
		}
		if found {
			break
		}
	}
	if !found {
		t.Fatal("no safe adjacent monster fixture")
	}
	if _, err := db.Exec(`UPDATE match_map_cells SET is_player=false WHERE instance_id=$1 AND is_player=true AND q=$2 AND r=$3`, id, snapshot.Player.Position.Q, snapshot.Player.Position.R); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`UPDATE match_map_cells SET is_player=true WHERE instance_id=$1 AND q=$2 AND r=$3`, id, playerPos.Q, playerPos.R); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`UPDATE match_players SET position_q=$2,position_r=$3,health=1,energy=100 WHERE instance_id=$1 AND user_id=781`, id, playerPos.Q, playerPos.R); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`UPDATE match_monsters SET health=100,max_health=100,attack=100 WHERE instance_id=$1 AND monster_instance_id=$2`, id, monsterID); err != nil {
		t.Fatal(err)
	}
	result, err := ApplyHexAttack(ctx, id, 781, monsterID, "multi-fatal", nil)
	if err != nil || result.NewAttackerHP > 0 {
		t.Fatalf("fatal counter: %+v %v", result, err)
	}
	var active, revision int
	var order []byte
	var finished, cell bool
	if err := db.QueryRow(`SELECT m.active_user_id,m.turn_order,m.finished_at IS NOT NULL,m.state_revision,c.is_player FROM matches m JOIN match_map_cells c ON c.instance_id=m.instance_id AND c.q=$2 AND c.r=$3 WHERE m.instance_id=$1`, id, playerPos.Q, playerPos.R).Scan(&active, &order, &finished, &revision, &cell); err != nil {
		t.Fatal(err)
	}
	if active != 782 || string(order) != "[782]" || finished || revision != 2 || cell {
		t.Fatalf("death state active=%d order=%s finished=%v rev=%d cell=%v", active, order, finished, revision, cell)
	}
	view, err := repository.LoadHexTestMatch(ctx, id, 782)
	if err != nil || view.ActiveUser != 782 || view.Player.UserID != 782 {
		t.Fatalf("survivor view: %+v %v", view.Player, err)
	}
}

func rangerLineFixture(t *testing.T, db *sql.DB) (string, int, hexgrid.Coord) {
	t.Helper()
	snapshot, monsterID := hexCombatFixture(t, db, "ranger", 2, 100)
	id := snapshot.InstanceID
	var monster hexgrid.Coord
	for _, actor := range snapshot.Actors[1:] {
		var id int
		if _, err := fmt.Sscanf(actor.ID, "monster-%d", &id); err == nil && id == monsterID {
			monster = actor.Position
			break
		}
	}
	cells := map[hexgrid.Coord]bool{}
	for _, cell := range snapshot.Cells {
		cells[cell.Position] = true
	}
	var player, destination hexgrid.Coord
	found := false
	for _, direction := range hexgrid.Directions {
		player = hexgrid.Coord{Q: monster.Q - 2*direction.Q, R: monster.R - 2*direction.R}
		destination = hexgrid.Coord{Q: monster.Q + direction.Q, R: monster.R + direction.R}
		middle := hexgrid.Coord{Q: monster.Q - direction.Q, R: monster.R - direction.R}
		if cells[player] && cells[destination] && cells[middle] {
			found = true
			break
		}
	}
	if !found {
		t.Fatal("no straight HEX firing lane")
	}
	for _, query := range []string{
		`DELETE FROM match_hex_edges WHERE instance_id=$1`,
		`DELETE FROM match_monsters WHERE instance_id=$1 AND monster_instance_id<>$2`,
		`UPDATE match_map_cells SET is_player=false WHERE instance_id=$1`,
	} {
		var err error
		if query == `DELETE FROM match_monsters WHERE instance_id=$1 AND monster_instance_id<>$2` {
			_, err = db.Exec(query, id, monsterID)
		} else {
			_, err = db.Exec(query, id)
		}
		if err != nil {
			t.Fatal(err)
		}
	}
	if _, err := db.Exec(`UPDATE match_players SET position_q=$2,position_r=$3,attack=14,energy=100,max_energy=100,agility=0 WHERE instance_id=$1`, id, player.Q, player.R); err != nil {
		t.Fatal(err)
	}
	for _, pos := range []hexgrid.Coord{player, {Q: monster.Q - (monster.Q-player.Q)/2, R: monster.R - (monster.R-player.R)/2}, destination} {
		if _, err := db.Exec(`UPDATE match_map_cells SET tile_code=48,resource=NULL,barbel=NULL,monster=NULL,is_player=($4::bool) WHERE instance_id=$1 AND q=$2 AND r=$3`, id, pos.Q, pos.R, pos == player); err != nil {
			t.Fatal(err)
		}
	}
	return id, monsterID, destination
}

func TestHexRangerThirdShotPostgres(t *testing.T) {
	db, _ := hexCombatDatabase(t)
	old := rollReflexProc
	rollReflexProc = func(int) bool { return false }
	t.Cleanup(func() { rollReflexProc = old })
	for _, name := range []string{"push", "rock-fallback", "occupied-fallback", "wall-fallback"} {
		blocked := name != "push"
		t.Run(name, func(t *testing.T) {
			id, monsterID, destination := rangerLineFixture(t, db)
			ctx := context.Background()
			for shot := 1; shot <= 2; shot++ {
				result, err := ApplyHexAttack(ctx, id, 781, monsterID, fmt.Sprintf("shot-%d", shot), nil)
				if err != nil {
					t.Fatal(err)
				}
				var stacks int
				if err := db.QueryRow(`SELECT hex_armor_break_stacks FROM match_monsters WHERE instance_id=$1 AND monster_instance_id=$2`, id, monsterID).Scan(&stacks); err != nil || stacks != shot {
					t.Fatalf("stack %d: %d %v", shot, stacks, err)
				}
				if len(result.Effects) != 1 || result.Effects[0].Kind != "armorBreak" || result.Effects[0].Stacks != shot {
					t.Fatalf("shot %d effects: %+v", shot, result.Effects)
				}
				if expected := 10 + (shot-1)*4; result.Damage != expected {
					t.Fatalf("shot %d damage=%d want=%d", shot, result.Damage, expected)
				}
			}
			switch name {
			case "rock-fallback":
				if _, err := db.Exec(`UPDATE match_map_cells SET tile_code=32 WHERE instance_id=$1 AND q=$2 AND r=$3`, id, destination.Q, destination.R); err != nil {
					t.Fatal(err)
				}
			case "occupied-fallback":
				if _, err := db.Exec(`UPDATE match_map_cells SET is_player=true WHERE instance_id=$1 AND q=$2 AND r=$3`, id, destination.Q, destination.R); err != nil {
					t.Fatal(err)
				}
			case "wall-fallback":
				var target hexgrid.Coord
				if err := db.QueryRow(`SELECT pos_q,pos_r FROM match_monsters WHERE instance_id=$1 AND monster_instance_id=$2`, id, monsterID).Scan(&target.Q, &target.R); err != nil {
					t.Fatal(err)
				}
				for direction := 0; direction < 6; direction++ {
					next, _ := hexgrid.Neighbor(target, direction)
					if next != destination {
						continue
					}
					origin, index := target, direction
					if direction >= 3 {
						origin, index = destination, direction-3
					}
					if _, err := db.Exec(`INSERT INTO match_hex_edges(instance_id,q,r,direction,wall_kind) VALUES($1,$2,$3,$4,'wall')`, id, origin.Q, origin.R, index); err != nil {
						t.Fatal(err)
					}
					break
				}
			}
			published := 0
			third, err := ApplyHexAttack(ctx, id, 781, monsterID, "third", func(HexAttackResult) { published++ })
			if err != nil || published != 1 || third.StateRevision != 4 {
				t.Fatalf("third: %+v %v published=%d", third, err, published)
			}
			push := third.Effects[0]
			if push.Kind != "push" || push.Succeeded == blocked || push.EnergyGranted != rangerThirdShotEnergyRefund {
				t.Fatalf("push: %+v", push)
			}
			var q, r, stacks, energy int
			if err := db.QueryRow(`SELECT pos_q,pos_r,hex_armor_break_stacks FROM match_monsters WHERE instance_id=$1 AND monster_instance_id=$2`, id, monsterID).Scan(&q, &r, &stacks); err != nil {
				t.Fatal(err)
			}
			if err := db.QueryRow(`SELECT ap FROM match_players WHERE instance_id=$1`, id).Scan(&energy); err != nil {
				t.Fatal(err)
			}
			if stacks != 0 || energy != third.AP {
				t.Fatalf("stacks=%d energy=%d result=%+v", stacks, energy, third)
			}
			if blocked {
				if push.BonusDamage <= 0 || third.MapRevision != 1 || q == destination.Q && r == destination.R {
					t.Fatalf("fallback: %+v pos=%d,%d", third, q, r)
				}
			} else if q != destination.Q || r != destination.R || third.MapRevision != 2 {
				t.Fatalf("push pos=%d,%d destination=%+v revision=%d", q, r, destination, third.MapRevision)
			}
			if !blocked {
				var cellMonster []byte
				if err := db.QueryRow(`SELECT monster FROM match_map_cells WHERE instance_id=$1 AND q=$2 AND r=$3`, id, destination.Q, destination.R).Scan(&cellMonster); err != nil {
					t.Fatal(err)
				}
				var moved struct {
					ID     int `json:"db_instance_id"`
					Health int `json:"health"`
				}
				if err := json.Unmarshal(cellMonster, &moved); err != nil || moved.ID != monsterID || moved.Health != third.NewTargetHP {
					t.Fatalf("push cell=%s %+v %v", cellMonster, moved, err)
				}
			}
			replay, err := ApplyHexAttack(ctx, id, 781, monsterID, "third", func(HexAttackResult) { published++ })
			if err != nil || !replay.Replayed || published != 1 || replay.StateRevision != third.StateRevision {
				t.Fatalf("replay: %+v %v published=%d", replay, err, published)
			}
		})
	}
}

func TestHexPlayerDeathAndInternalMonsterPostgres(t *testing.T) {
	db, _ := hexCombatDatabase(t)
	old := rollReflexProc
	rollReflexProc = func(int) bool { return false }
	t.Cleanup(func() { rollReflexProc = old })
	ctx := context.Background()
	for _, internal := range []bool{false, true} {
		name := "counter"
		if internal {
			name = "internal"
		}
		t.Run(name, func(t *testing.T) {
			snapshot, monsterID := hexCombatFixture(t, db, "guardian", 1, 100)
			id := snapshot.InstanceID
			if _, err := db.Exec(`UPDATE match_players SET health=1 WHERE instance_id=$1`, id); err != nil {
				t.Fatal(err)
			}
			published := 0
			var result HexAttackResult
			var err error
			if internal {
				result, err = ApplyHexMonsterAttack(ctx, id, monsterID, 781, "fatal", func(HexAttackResult) { published++ })
			} else {
				result, err = ApplyHexAttack(ctx, id, 781, monsterID, "fatal", func(HexAttackResult) { published++ })
			}
			if err != nil || result.StateRevision != 2 || result.MapRevision != 2 || published != 1 {
				t.Fatalf("death: %+v %v published=%d", result, err, published)
			}
			var hp, energy, active int
			var cell, finished, complete bool
			var order, raw []byte
			if err := db.QueryRow(`SELECT p.health,p.energy,m.active_user_id,m.turn_order,c.is_player,m.finished_at IS NOT NULL,m.combat_results_complete,m.combat_result_events FROM matches m JOIN match_players p ON p.instance_id=m.instance_id JOIN match_map_cells c ON c.instance_id=m.instance_id AND c.q=p.position_q AND c.r=p.position_r WHERE m.instance_id=$1`, id).Scan(&hp, &energy, &active, &order, &cell, &finished, &complete, &raw); err != nil {
				t.Fatal(err)
			}
			var events game.ResultEvents
			if err := json.Unmarshal(raw, &events); err != nil {
				t.Fatal(err)
			}
			if hp != 0 || active != 0 || string(order) != "[]" || cell || !finished || complete || len(events.KillEvents) != 1 || len(events.DefeatedUsers) != 1 {
				t.Fatalf("death state hp=%d energy=%d active=%d order=%s cell=%v finished=%v complete=%v events=%+v", hp, energy, active, order, cell, finished, complete, events)
			}
			var balance int
			if err := db.QueryRow(`SELECT balance FROM player_profiles WHERE user_id=781`).Scan(&balance); err != nil || balance != 0 {
				t.Fatalf("premature rewards balance=%d %v", balance, err)
			}
			if internal {
				result, err = ApplyHexMonsterAttack(ctx, id, monsterID, 781, "fatal", func(HexAttackResult) { published++ })
			} else {
				result, err = ApplyHexAttack(ctx, id, 781, monsterID, "fatal", func(HexAttackResult) { published++ })
			}
			if err != nil || !result.Replayed || published != 1 {
				t.Fatalf("death replay: %+v %v", result, err)
			}
		})
	}
	snapshot, monsterID := hexCombatFixture(t, db, "mystic", 1, 100)
	id := snapshot.InstanceID
	if _, err := db.Exec(`CREATE FUNCTION fail_hex_monster_ledger() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN IF NEW.instance_id='` + id + `' THEN RAISE EXCEPTION 'forced rollback'; END IF; RETURN NEW; END $$`); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`CREATE TRIGGER fail_hex_monster_ledger BEFORE INSERT ON match_commands FOR EACH ROW EXECUTE FUNCTION fail_hex_monster_ledger()`); err != nil {
		t.Fatal(err)
	}
	published := 0
	if _, err := ApplyHexMonsterAttack(ctx, id, monsterID, 781, "rollback", func(HexAttackResult) { published++ }); err == nil {
		t.Fatal("expected rollback")
	}
	var hp, revision int
	if err := db.QueryRow(`SELECT health FROM match_players WHERE instance_id=$1`, id).Scan(&hp); err != nil || hp != 80 {
		t.Fatalf("rollback hp=%d %v", hp, err)
	}
	if err := db.QueryRow(`SELECT state_revision FROM matches WHERE instance_id=$1`, id).Scan(&revision); err != nil || revision != 1 || published != 0 {
		t.Fatalf("rollback revision=%d published=%d %v", revision, published, err)
	}
	if _, err := db.Exec(`DROP TRIGGER fail_hex_monster_ledger ON match_commands`); err != nil {
		t.Fatal(err)
	}
	if _, err := ApplyHexMonsterAttack(ctx, id, monsterID, 781, "rollback", nil); err != nil {
		t.Fatal(err)
	}
	// The retry commits one hit and one counter after the failed transaction.
	var raw []byte
	if err := db.QueryRow(`SELECT combat_result_events FROM matches WHERE instance_id=$1`, id).Scan(&raw); err != nil {
		t.Fatal(err)
	}
	var events game.ResultEvents
	if err := json.Unmarshal(raw, &events); err != nil || len(events.DamageEvents) != 2 {
		t.Fatalf("internal exchange events=%+v err=%v", events, err)
	}
	if _, err := ApplyHexMonsterAttack(ctx, id, monsterID, 781, "rollback", func(HexAttackResult) { published++ }); err != nil || published != 0 {
		t.Fatalf("internal replay published=%d err=%v", published, err)
	}
	// Failure while persisting a lethal counter rolls back HP, occupancy,
	// result history, revisions, finished marker, and its WS callback.
	deathSnapshot, deathMonster := hexCombatFixture(t, db, "guardian", 1, 100)
	deathID := deathSnapshot.InstanceID
	if _, err := db.Exec(`UPDATE match_players SET health=1 WHERE instance_id=$1`, deathID); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`CREATE FUNCTION fail_hex_death_ledger() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN IF NEW.instance_id='` + deathID + `' THEN RAISE EXCEPTION 'forced death rollback'; END IF; RETURN NEW; END $$`); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`CREATE TRIGGER fail_hex_death_ledger BEFORE INSERT ON match_commands FOR EACH ROW EXECUTE FUNCTION fail_hex_death_ledger()`); err != nil {
		t.Fatal(err)
	}
	if _, err := ApplyHexAttack(ctx, deathID, 781, deathMonster, "fatal-rollback", func(HexAttackResult) { published++ }); err == nil {
		t.Fatal("expected death rollback")
	}
	var active, health int
	var occupied, finished bool
	if err := db.QueryRow(`SELECT p.health,m.active_user_id,c.is_player,m.finished_at IS NOT NULL,m.state_revision FROM matches m JOIN match_players p ON p.instance_id=m.instance_id JOIN match_map_cells c ON c.instance_id=m.instance_id AND c.q=p.position_q AND c.r=p.position_r WHERE m.instance_id=$1`, deathID).Scan(&health, &active, &occupied, &finished, &revision); err != nil {
		t.Fatal(err)
	}
	if health != 1 || active != 781 || !occupied || finished || revision != 1 || published != 0 {
		t.Fatalf("death rollback hp=%d active=%d occupied=%v finished=%v revision=%d published=%d", health, active, occupied, finished, revision, published)
	}
}

func TestHexRangerCriticalOrderAndExpiryPostgres(t *testing.T) {
	db, _ := hexCombatDatabase(t)
	old := rollReflexProc
	rollReflexProc = func(int) bool { return true }
	t.Cleanup(func() { rollReflexProc = old })
	id, monsterID, _ := rangerLineFixture(t, db)
	ctx := context.Background()
	first, err := ApplyHexAttack(ctx, id, 781, monsterID, "crit-first", nil)
	if err != nil || first.Damage != 14 || len(first.Effects) != 2 || first.Effects[0].Kind != "crit" || first.Effects[1].Kind != "armorBreak" {
		t.Fatalf("crit before break: %+v %v", first, err)
	}
	second, err := ApplyHexAttack(ctx, id, 781, monsterID, "crit-second", nil)
	if err != nil || second.Effects[1].Stacks != 2 {
		t.Fatalf("second break: %+v %v", second, err)
	}
	for turn := 1; turn <= 2; turn++ {
		if _, err := repository.ApplyHexEndTurn(ctx, id, 781, fmt.Sprintf("turn-%d", turn), turn, nil); err != nil {
			t.Fatal(err)
		}
	}
	var stacks, turns int
	if err := db.QueryRow(`SELECT hex_armor_break_stacks,hex_armor_break_turns FROM match_monsters WHERE instance_id=$1 AND monster_instance_id=$2`, id, monsterID).Scan(&stacks, &turns); err != nil || stacks != 0 || turns != 0 {
		t.Fatalf("expired stacks=%d turns=%d err=%v", stacks, turns, err)
	}
}
