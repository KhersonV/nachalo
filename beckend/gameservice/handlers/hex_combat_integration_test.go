package handlers

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"gameservice/game"
	"gameservice/internal/hexgrid"
	"gameservice/internal/hexmap"
	"gameservice/repository"
	_ "github.com/lib/pq"
)

func hexCombatDatabase(t *testing.T) (*sql.DB, string) {
	t.Helper()
	dsn := os.Getenv("STAGE2_TEST_DSN")
	if dsn == "" {
		t.Skip("set STAGE2_TEST_DSN to isolated PostgreSQL")
	}
	admin, err := sql.Open("postgres", dsn)
	if err != nil {
		t.Fatal(err)
	}
	schema := fmt.Sprintf("hex_combat_%d", time.Now().UnixNano())
	if _, err := admin.Exec(`CREATE SCHEMA ` + schema); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _, _ = admin.Exec(`DROP SCHEMA ` + schema + ` CASCADE`); admin.Close() })
	if strings.HasPrefix(dsn, "postgres://") || strings.HasPrefix(dsn, "postgresql://") {
		u, err := url.Parse(dsn)
		if err != nil {
			t.Fatal(err)
		}
		q := u.Query()
		q.Set("options", "-csearch_path="+schema)
		u.RawQuery = q.Encode()
		dsn = u.String()
	} else {
		dsn += " options='-csearch_path=" + schema + "'"
	}
	db, err := sql.Open("postgres", dsn)
	if err != nil {
		t.Fatal(err)
	}
	old := repository.DB
	repository.DB = db
	t.Cleanup(func() { repository.DB = old; db.Close() })
	repository.RunMigrations()
	for _, file := range []string{"stage1_geometry.sql", "stage2_commands.sql", "stage3_hex_pve.sql"} {
		raw, err := os.ReadFile("../../../docs/hex-migration/" + file)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := db.Exec(string(raw)); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := repository.CreatePlayerProfile(repository.CreatePlayerProfileInput{UserID: 781, Name: "HEX combat", CharacterType: "guardian"}); err != nil {
		t.Fatal(err)
	}
	return db, dsn
}

func hexCombatFixture(t *testing.T, db *sql.DB, class string, distance int, monsterHP int) (hexmap.Snapshot, int) {
	t.Helper()
	snapshot, err := repository.CreateHexTestMatch(context.Background(), 781, 97)
	if err != nil {
		t.Fatal(err)
	}
	id := snapshot.InstanceID
	floors := map[hexgrid.Coord]bool{}
	for _, cell := range snapshot.Cells {
		floors[cell.Position] = cell.Terrain == "floor" && cell.Barrel == nil
	}
	occupied := map[hexgrid.Coord]bool{}
	for _, actor := range snapshot.Actors {
		occupied[actor.Position] = true
	}
	walls := map[hexgrid.EdgeKey]bool{}
	for _, edge := range snapshot.Edges {
		next, _ := hexgrid.Neighbor(edge.Position, edge.Direction)
		key, _ := hexgrid.CanonicalEdge(edge.Position, next)
		walls[key] = true
	}
	var chosen hexgrid.Coord
	var monster hexmap.Actor
	found := false
	for _, candidate := range snapshot.Actors[1:] {
		for _, cell := range snapshot.Cells {
			d, _ := hexgrid.Distance(cell.Position, candidate.Position)
			if floors[cell.Position] && !occupied[cell.Position] && d == distance && hexgrid.Visible(cell.Position, candidate.Position, floors, walls) {
				chosen, monster, found = cell.Position, candidate, true
				break
			}
		}
		if found {
			break
		}
	}
	if !found {
		t.Fatalf("no visible cell at HEX distance %d", distance)
	}
	var monsterID int
	if _, err := fmt.Sscanf(monster.ID, "monster-%d", &monsterID); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`UPDATE match_map_cells SET is_player=false WHERE instance_id=$1`, id); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`UPDATE match_map_cells SET is_player=true WHERE instance_id=$1 AND q=$2 AND r=$3`, id, chosen.Q, chosen.R); err != nil {
		t.Fatal(err)
	}
	isRanged := class == "mystic" || class == "ranger"
	rangeValue := 1
	if class == "mystic" {
		rangeValue = 3
	} else if class == "ranger" {
		rangeValue = 2
	}
	if _, err := db.Exec(`UPDATE match_players SET position_q=$2,position_r=$3,character_type=$4,energy=100,max_energy=100,health=80,max_health=100,attack=14,defense=8,agility=10,is_ranged=$5,attack_range=$6 WHERE instance_id=$1 AND user_id=781`, id, chosen.Q, chosen.R, class, isRanged, rangeValue); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`UPDATE match_monsters SET health=$3,max_health=$3,attack=12,defense=4 WHERE instance_id=$1 AND monster_instance_id=$2`, id, monsterID, monsterHP); err != nil {
		t.Fatal(err)
	}
	return snapshot, monsterID
}

func TestHexPvEClassesAndDeathPostgres(t *testing.T) {
	db, dsn := hexCombatDatabase(t)
	oldProc, oldDrop := rollReflexProc, rollEquipmentDrop
	rollReflexProc = func(int) bool { return true }
	rollEquipmentDrop = func(int) bool { return true }
	t.Cleanup(func() { rollReflexProc = oldProc; rollEquipmentDrop = oldDrop })
	ctx := context.Background()
	for _, tc := range []struct {
		name, class  string
		distance, hp int
		effect       string
		mode         attackMode
	}{
		{"guardian-response", "guardian", 1, 30, "guardianResponse", attackModeMelee},
		{"mystic-melee", "mystic", 1, 30, "", attackModeMelee},
		{"mystic-ranged", "mystic", 2, 30, "", attackModeRanged},
		{"berserker-lifesteal", "berserker", 1, 30, "lifesteal", attackModeMelee},
		{"ranger-melee-crit", "ranger", 1, 30, "crit", attackModeMelee},
	} {
		t.Run(tc.name, func(t *testing.T) {
			snapshot, targetID := hexCombatFixture(t, db, tc.class, tc.distance, tc.hp)
			preview, err := PreviewHexAttack(ctx, snapshot.InstanceID, 781, targetID)
			if err != nil || preview.Distance != tc.distance || preview.Mode != tc.mode {
				t.Fatalf("preview=%+v err=%v", preview, err)
			}
			published := 0
			result, err := ApplyHexAttack(ctx, snapshot.InstanceID, 781, targetID, "first-hit", func(HexAttackResult) { published++ })
			if err != nil || result.Damage <= 0 || result.StateRevision != 2 || result.MapRevision != 1 || published != 1 {
				t.Fatalf("attack=%+v err=%v publish=%d", result, err, published)
			}
			if tc.mode == attackModeMelee && result.NewTargetHP > 0 && result.CounterDamage <= 0 {
				t.Fatal("monster counter missing")
			}
			if tc.class == "mystic" && result.Damage != 12 {
				t.Fatalf("mystic defense percentage changed: damage=%d", result.Damage)
			}
			if tc.class == "ranger" && result.Damage != 14 {
				t.Fatalf("ranger crit percentage changed: damage=%d", result.Damage)
			}
			if tc.class == "berserker" && result.CounterDamage != 2 {
				t.Fatalf("berserker counter reduction changed: damage=%d", result.CounterDamage)
			}
			if tc.effect != "" {
				found := false
				for _, effect := range result.Effects {
					if effect.Kind == tc.effect {
						found = true
					}
				}
				for _, step := range result.Steps {
					if step.Kind == tc.effect {
						found = true
					}
				}
				if !found {
					t.Fatalf("missing %s: %+v %+v", tc.effect, result.Effects, result.Steps)
				}
			}
			if tc.class == "berserker" {
				found := false
				for _, step := range result.Steps {
					if step.Kind == "followup" {
						found = true
					}
				}
				if !found {
					t.Fatal("berserker followup missing")
				}
			}
			replay, err := ApplyHexAttack(ctx, snapshot.InstanceID, 781, targetID, "first-hit", func(HexAttackResult) { published++ })
			if err != nil || !replay.Replayed || published != 1 {
				t.Fatalf("replay=%+v err=%v", replay, err)
			}
			if _, err := ApplyHexAttack(ctx, snapshot.InstanceID, 781, targetID+1, "first-hit", nil); !errors.Is(err, repository.ErrCommandConflict) {
				t.Fatalf("conflicting payload: %v", err)
			}
			fresh, err := sql.Open("postgres", dsn)
			if err != nil {
				t.Fatal(err)
			}
			defer fresh.Close()
			var raw []byte
			if err := fresh.QueryRow(`SELECT combat_result_events FROM matches WHERE instance_id=$1`, snapshot.InstanceID).Scan(&raw); err != nil {
				t.Fatal(err)
			}
			var events game.ResultEvents
			if err := json.Unmarshal(raw, &events); err != nil || len(events.DamageEvents) == 0 {
				t.Fatalf("restored events=%+v err=%v", events, err)
			}
			repository.DB = fresh
			restored, loadErr := repository.LoadHexTestMatch(ctx, snapshot.InstanceID, 781)
			repository.DB = db
			if loadErr != nil || restored.Player.Health != result.NewAttackerHP || restored.StateRevision != result.StateRevision {
				t.Fatalf("fresh-session HP/revision=%+v err=%v", restored.Player, loadErr)
			}
		})
	}
	// A lethal attack uses the same transaction for HP, monster removal,
	// equipment drop, result event and revision.
	snapshot, targetID := hexCombatFixture(t, db, "guardian", 1, 4)
	result, err := ApplyHexAttack(ctx, snapshot.InstanceID, 781, targetID, "kill", nil)
	if err != nil || !result.MonsterDied || len(result.Drops) != 1 || result.MapRevision != 2 {
		t.Fatalf("kill=%+v err=%v", result, err)
	}
	var count int
	if err := db.QueryRow(`SELECT count(*) FROM match_monsters WHERE instance_id=$1 AND monster_instance_id=$2`, snapshot.InstanceID, targetID).Scan(&count); err != nil || count != 0 {
		t.Fatal("monster not removed", err)
	}
	if err := db.QueryRow(`SELECT count(*) FROM item_instances WHERE match_instance_id=$1`, snapshot.InstanceID).Scan(&count); err != nil || count != 1 {
		t.Fatal("drop not persisted once", err, count)
	}
	if _, err := ApplyHexAttack(ctx, snapshot.InstanceID, 781, targetID, "kill", nil); err != nil {
		t.Fatal(err)
	}
	if err := db.QueryRow(`SELECT count(*) FROM item_instances WHERE match_instance_id=$1`, snapshot.InstanceID).Scan(&count); err != nil || count != 1 {
		t.Fatal("replay duplicated drop", err, count)
	}
	var raw []byte
	if err := db.QueryRow(`SELECT combat_result_events FROM matches WHERE instance_id=$1`, snapshot.InstanceID).Scan(&raw); err != nil {
		t.Fatal(err)
	}
	var events game.ResultEvents
	if err := json.Unmarshal(raw, &events); err != nil || len(events.KillEvents) != 1 {
		t.Fatalf("kill events=%+v err=%v", events, err)
	}
}

func TestHexPvEBlockingRaceAndRollbackPostgres(t *testing.T) {
	db, _ := hexCombatDatabase(t)
	oldDrop := rollEquipmentDrop
	rollEquipmentDrop = func(int) bool { return true }
	t.Cleanup(func() { rollEquipmentDrop = oldDrop })
	ctx := context.Background()
	snapshot, targetID := hexCombatFixture(t, db, "guardian", 1, 4)
	id := snapshot.InstanceID
	var wg sync.WaitGroup
	results := make(chan error, 2)
	for _, commandID := range []string{"race-a", "race-b"} {
		wg.Add(1)
		go func(commandID string) {
			defer wg.Done()
			_, err := ApplyHexAttack(ctx, id, 781, targetID, commandID, nil)
			results <- err
		}(commandID)
	}
	wg.Wait()
	close(results)
	success, rejected := 0, 0
	for err := range results {
		if err == nil {
			success++
		} else if errors.Is(err, repository.ErrCommandRejected) {
			rejected++
		} else {
			t.Fatal(err)
		}
	}
	if success != 1 || rejected != 1 {
		t.Fatalf("race success=%d rejected=%d", success, rejected)
	}
	snapshot, targetID = hexCombatFixture(t, db, "guardian", 1, 4)
	id = snapshot.InstanceID
	if _, err := db.Exec(`CREATE FUNCTION fail_hex_drop() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN RAISE EXCEPTION 'drop write failed'; END $$`); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`CREATE TRIGGER fail_hex_drop BEFORE INSERT ON item_instances FOR EACH ROW EXECUTE FUNCTION fail_hex_drop()`); err != nil {
		t.Fatal(err)
	}
	published := 0
	if _, err := ApplyHexAttack(ctx, id, 781, targetID, "rollback-kill", func(HexAttackResult) { published++ }); err == nil {
		t.Fatal("expected drop failure")
	}
	if published != 0 {
		t.Fatal("published before commit")
	}
	var hp, energy, revision int
	if err := db.QueryRow(`SELECT health,energy FROM match_players WHERE instance_id=$1 AND user_id=781`, id).Scan(&hp, &energy); err != nil || hp != 80 || energy != 100 {
		t.Fatalf("rollback player=%d/%d err=%v", hp, energy, err)
	}
	if err := db.QueryRow(`SELECT health FROM match_monsters WHERE instance_id=$1 AND monster_instance_id=$2`, id, targetID).Scan(&hp); err != nil || hp != 4 {
		t.Fatalf("rollback monster=%d err=%v", hp, err)
	}
	if err := db.QueryRow(`SELECT state_revision FROM matches WHERE instance_id=$1`, id).Scan(&revision); err != nil || revision != 1 {
		t.Fatalf("rollback revision=%d err=%v", revision, err)
	}
	if _, err := db.Exec(`DROP TRIGGER fail_hex_drop ON item_instances`); err != nil {
		t.Fatal(err)
	}
	if _, err := ApplyHexAttack(ctx, id, 781, targetID, "rollback-kill", nil); err != nil {
		t.Fatalf("retry after rollback: %v", err)
	}
	// A wall across a melee edge blocks the attack even with adjacent q/r.
	snapshot, targetID = hexCombatFixture(t, db, "guardian", 1, 20)
	id = snapshot.InstanceID
	var player, monster hexgrid.Coord
	if err := db.QueryRow(`SELECT position_q,position_r FROM match_players WHERE instance_id=$1`, id).Scan(&player.Q, &player.R); err != nil {
		t.Fatal(err)
	}
	if err := db.QueryRow(`SELECT pos_q,pos_r FROM match_monsters WHERE instance_id=$1 AND monster_instance_id=$2`, id, targetID).Scan(&monster.Q, &monster.R); err != nil {
		t.Fatal(err)
	}
	inserted := false
	for d := 0; d < 6; d++ {
		next, _ := hexgrid.Neighbor(player, d)
		if next != monster {
			continue
		}
		from, direction := player, d
		if d >= 3 {
			from = monster
			direction = d - 3
		}
		_, err := db.Exec(`INSERT INTO match_hex_edges(instance_id,q,r,direction,wall_kind) VALUES($1,$2,$3,$4,'wall')`, id, from.Q, from.R, direction)
		if err != nil {
			t.Fatal(err)
		}
		inserted = true
		break
	}
	if !inserted {
		t.Fatal("adjacent edge missing")
	}
	if _, err := PreviewHexAttack(ctx, id, 781, targetID); !errors.Is(err, repository.ErrHexBlocked) {
		t.Fatalf("wall LOS: %v", err)
	}
	if _, err := ApplyHexAttack(ctx, id, 781, targetID, "wall-hit", nil); !errors.Is(err, repository.ErrHexBlocked) {
		t.Fatalf("wall attack: %v", err)
	}
	// Rock between a ranged attacker and target also blocks line of sight.
	snapshot, targetID = hexCombatFixture(t, db, "mystic", 2, 20)
	id = snapshot.InstanceID
	if err := db.QueryRow(`SELECT position_q,position_r FROM match_players WHERE instance_id=$1`, id).Scan(&player.Q, &player.R); err != nil {
		t.Fatal(err)
	}
	if err := db.QueryRow(`SELECT pos_q,pos_r FROM match_monsters WHERE instance_id=$1 AND monster_instance_id=$2`, id, targetID).Scan(&monster.Q, &monster.R); err != nil {
		t.Fatal(err)
	}
	blocked := false
	for d := 0; d < 6; d++ {
		middle, _ := hexgrid.Neighbor(player, d)
		distance, _ := hexgrid.Distance(middle, monster)
		if distance != 1 || middle == monster {
			continue
		}
		res, err := db.Exec(`UPDATE match_map_cells SET tile_code=32 WHERE instance_id=$1 AND q=$2 AND r=$3 AND is_player=false`, id, middle.Q, middle.R)
		if err != nil {
			t.Fatal(err)
		}
		if n, _ := res.RowsAffected(); n == 1 {
			blocked = true
			break
		}
	}
	if !blocked {
		t.Fatal("no intermediate cell for rock test")
	}
	if _, err := PreviewHexAttack(ctx, id, 781, targetID); !errors.Is(err, repository.ErrHexBlocked) {
		t.Fatalf("rock LOS: %v", err)
	}
	// Ranger ranged attack now records its first armor-break stack.
	snapshot, targetID = hexCombatFixture(t, db, "ranger", 2, 20)
	if _, err := PreviewHexAttack(ctx, snapshot.InstanceID, 781, targetID); err != nil {
		t.Fatalf("ranger ranged preview: %v", err)
	}
	if result, err := ApplyHexAttack(ctx, snapshot.InstanceID, 781, targetID, "ranger-shot", nil); err != nil || result.Mode != attackModeRanged {
		t.Fatalf("ranger ranged attack: %+v %v", result, err)
	}
	// A lethal monster counter closes the sole-player Lab match atomically.
	snapshot, targetID = hexCombatFixture(t, db, "guardian", 1, 100)
	if _, err := db.Exec(`UPDATE match_players SET health=1 WHERE instance_id=$1`, snapshot.InstanceID); err != nil {
		t.Fatal(err)
	}
	if result, err := ApplyHexAttack(ctx, snapshot.InstanceID, 781, targetID, "lethal-counter", nil); err != nil || result.NewAttackerHP > 0 {
		t.Fatalf("last-player death path: %+v %v", result, err)
	}
}
