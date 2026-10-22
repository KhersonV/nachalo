package repository

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"net/url"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"gameservice/internal/hexgrid"
	"gameservice/internal/hexmap"
)

func hexTestDatabase(t *testing.T) (*sql.DB, string) {
	t.Helper()
	dsn := os.Getenv("STAGE2_TEST_DSN")
	if dsn == "" {
		t.Skip("set STAGE2_TEST_DSN to isolated PostgreSQL")
	}
	admin, err := sql.Open("postgres", dsn)
	if err != nil {
		t.Fatal(err)
	}
	schema := fmt.Sprintf("hex_match_%d", time.Now().UnixNano())
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
	old := DB
	DB = db
	t.Cleanup(func() { DB = old; db.Close() })
	RunMigrations()
	for _, name := range []string{"stage1_geometry.sql", "stage2_commands.sql", "stage3_hex_pve.sql", "stage4_storage_loadout.sql", "stage5_quest_artifact.sql", "stage5_portal_extraction.sql", "stage6_fog_turn.sql", "stage10_presence.sql"} {
		data, err := os.ReadFile("../../../docs/hex-migration/" + name)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := db.Exec(string(data)); err != nil {
			t.Fatalf("%s: %v", name, err)
		}
	}
	if _, err := CreatePlayerProfile(CreatePlayerProfileInput{UserID: 701, Name: "Hex Tester", CharacterType: "guardian"}); err != nil {
		t.Fatal(err)
	}
	return db, dsn
}

func walkableNeighbor(snapshot hexmap.Snapshot) hexgrid.Coord {
	floor := map[hexgrid.Coord]bool{}
	occupied := map[hexgrid.Coord]bool{}
	for _, cell := range snapshot.Cells {
		floor[cell.Position] = cell.Terrain == "floor"
	}
	for _, actor := range snapshot.Actors {
		occupied[actor.Position] = true
	}
	start := snapshot.Player.Position
	for dir := 0; dir < 6; dir++ {
		next, _ := hexgrid.Neighbor(start, dir)
		if floor[next] && !occupied[next] {
			return next
		}
	}
	return hexgrid.Coord{}
}

func TestHexMatchPostgres(t *testing.T) {
	db, dsn := hexTestDatabase(t)
	ctx := context.Background()
	snapshot, err := CreateHexTestMatch(ctx, 701, -17)
	if err != nil {
		t.Fatal(err)
	}
	if snapshot.Geometry.Kind != "hex" || snapshot.Player == nil || len(snapshot.Actors) < 2 || snapshot.Portal == nil || snapshot.Radius < 8 || snapshot.Seed != -17 {
		t.Fatalf("bad saved match: player=%+v actors=%d portal=%+v radius=%d", snapshot.Player, len(snapshot.Actors), snapshot.Portal, snapshot.Radius)
	}
	monsters, barrels, resources, portals := 0, 0, 0, 0
	for _, actor := range snapshot.Actors {
		if actor.Kind == "monster" {
			monsters++
			if actor.MonsterRefID <= 0 || actor.Name == "" || actor.Type == "" || actor.MaxHealth <= 0 || actor.Attack <= 0 {
				t.Fatalf("incomplete catalog monster: %+v", actor)
			}
		}
	}
	for _, cell := range snapshot.Cells {
		if cell.Barrel != nil {
			barrels++
		}
		if cell.Resource != nil {
			resources++
		}
		if cell.IsPortal {
			portals++
			if cell.Position != *snapshot.Portal || cell.Terrain != "floor" {
				t.Fatalf("invalid portal cell: %+v", cell)
			}
		}
	}
	if monsters == 0 || barrels == 0 || resources == 0 || portals != 1 {
		t.Fatalf("objects monsters=%d barrels=%d resources=%d portals=%d", monsters, barrels, resources, portals)
	}
	var dbMonsters, dbBarrels, dbResources, dbPortals int
	if err := db.QueryRow(`SELECT count(*) FROM match_monsters WHERE instance_id=$1`, snapshot.InstanceID).Scan(&dbMonsters); err != nil {
		t.Fatal(err)
	}
	if err := db.QueryRow(`SELECT count(*) FILTER (WHERE barbel IS NOT NULL), count(*) FILTER (WHERE resource IS NOT NULL), count(*) FILTER (WHERE is_portal) FROM match_map_cells WHERE instance_id=$1`, snapshot.InstanceID).Scan(&dbBarrels, &dbResources, &dbPortals); err != nil {
		t.Fatal(err)
	}
	if dbMonsters != monsters || dbBarrels != barrels || dbResources != resources || dbPortals != 1 {
		t.Fatalf("DB objects=%d/%d/%d/%d snapshot=%d/%d/%d/%d", dbMonsters, dbBarrels, dbResources, dbPortals, monsters, barrels, resources, portals)
	}
	var x, y sql.NullInt64
	var q, r int
	if err := db.QueryRow(`SELECT position IS NOT NULL,position_q,position_r FROM match_players WHERE instance_id=$1 AND user_id=701`, snapshot.InstanceID).Scan(new(bool), &q, &r); err != nil {
		t.Fatal(err)
	}
	if err := db.QueryRow(`SELECT x,y FROM match_map_cells WHERE instance_id=$1 AND q=$2 AND r=$3`, snapshot.InstanceID, q, r).Scan(&x, &y); err != nil || x.Valid || y.Valid {
		t.Fatalf("mixed coordinates x=%v y=%v err=%v", x, y, err)
	}
	dest := walkableNeighbor(snapshot)
	if dest == (hexgrid.Coord{}) {
		t.Fatal("no adjacent free cell")
	}
	preview, err := PreviewHexRoute(ctx, snapshot.InstanceID, 701, dest)
	if err != nil || len(preview.Path) != 1 || preview.TotalCost != preview.StepCost {
		t.Fatalf("preview=%+v err=%v", preview, err)
	}
	move, err := ApplyHexMove(ctx, snapshot.InstanceID, 701, "hex-step", dest)
	if err != nil || move.StateRevision != 2 || move.MapRevision != 2 || move.Energy != snapshot.Player.Energy-preview.StepCost {
		t.Fatalf("move=%+v err=%v", move, err)
	}
	replay, err := ApplyHexMove(ctx, snapshot.InstanceID, 701, "hex-step", dest)
	if err != nil || !replay.Replayed || replay.Energy != move.Energy {
		t.Fatalf("replay=%+v err=%v", replay, err)
	}
	if _, err := ApplyHexMove(ctx, snapshot.InstanceID, 701, "hex-step", snapshot.Player.Position); !errors.Is(err, ErrCommandConflict) {
		t.Fatalf("conflict: %v", err)
	}
	fresh, err := sql.Open("postgres", dsn)
	if err != nil {
		t.Fatal(err)
	}
	defer fresh.Close()
	var freshQ, freshR, freshEnergy int
	if err := fresh.QueryRow(`SELECT position_q,position_r,energy FROM match_players WHERE instance_id=$1 AND user_id=701`, snapshot.InstanceID).Scan(&freshQ, &freshR, &freshEnergy); err != nil || freshQ != dest.Q || freshR != dest.R || freshEnergy != move.Energy {
		t.Fatalf("fresh session q/r/energy=%d/%d/%d err=%v", freshQ, freshR, freshEnergy, err)
	}
	DB = fresh
	restarted, restartErr := LoadHexTestMatch(ctx, snapshot.InstanceID, 701)
	DB = db
	if restartErr != nil || restarted.Player.Position != dest || restarted.Player.Energy != move.Energy || restarted.StateRevision != 2 || restarted.MapRevision != 2 {
		t.Fatalf("fresh-session snapshot=%+v err=%v", restarted.Player, restartErr)
	}
	if restarted.Seed != snapshot.Seed || restarted.Radius != snapshot.Radius || restarted.Portal == nil || *restarted.Portal != *snapshot.Portal || len(restarted.Actors) != len(snapshot.Actors) {
		t.Fatalf("fresh-session dungeon metadata changed: seed=%d radius=%d portal=%+v actors=%d", restarted.Seed, restarted.Radius, restarted.Portal, len(restarted.Actors))
	}
	loaded, err := LoadHexTestMatch(ctx, snapshot.InstanceID, 701)
	if err != nil || loaded.Player.Position != dest || loaded.Player.Energy != move.Energy || loaded.StateRevision != 2 {
		t.Fatalf("loaded=%+v err=%v", loaded.Player, err)
	}
}

func TestHexConcurrentAndRollbackPostgres(t *testing.T) {
	db, _ := hexTestDatabase(t)
	ctx := context.Background()
	snapshot, err := CreateHexTestMatch(ctx, 701, 23)
	if err != nil {
		t.Fatal(err)
	}
	dest := walkableNeighbor(snapshot)
	var wg sync.WaitGroup
	results := make(chan error, 2)
	for _, id := range []string{"race-a", "race-b"} {
		wg.Add(1)
		go func(id string) {
			defer wg.Done()
			_, err := ApplyHexMove(ctx, snapshot.InstanceID, 701, id, dest)
			results <- err
		}(id)
	}
	wg.Wait()
	close(results)
	success, rejected := 0, 0
	for err := range results {
		if err == nil {
			success++
		} else if errors.Is(err, ErrCommandRejected) {
			rejected++
		} else {
			t.Fatal(err)
		}
	}
	if success != 1 || rejected != 1 {
		t.Fatalf("race success=%d rejected=%d", success, rejected)
	}
	if _, err := db.Exec(`CREATE FUNCTION fail_hex_ledger() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN IF NEW.command_id='hex-fail' THEN RAISE EXCEPTION 'injected failure'; END IF; RETURN NEW; END $$`); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`CREATE TRIGGER fail_hex_ledger BEFORE INSERT ON match_commands FOR EACH ROW EXECUTE FUNCTION fail_hex_ledger()`); err != nil {
		t.Fatal(err)
	}
	start := snapshot.Player.Position
	if _, err := ApplyHexMove(ctx, snapshot.InstanceID, 701, "hex-fail", start); err == nil {
		t.Fatal("expected rollback")
	}
	loaded, err := LoadHexTestMatch(ctx, snapshot.InstanceID, 701)
	if err != nil || loaded.Player.Position != dest || loaded.StateRevision != 2 {
		t.Fatalf("rollback=%+v err=%v", loaded.Player, err)
	}
}

func TestHexRouteRulesPostgres(t *testing.T) {
	db, _ := hexTestDatabase(t)
	ctx := context.Background()
	snapshot, err := CreateHexTestMatch(ctx, 701, 11)
	if err != nil {
		t.Fatal(err)
	}
	id := snapshot.InstanceID
	start := snapshot.Player.Position
	dest := walkableNeighbor(snapshot)
	if dest == (hexgrid.Coord{}) {
		t.Fatal("no neighbor")
	}
	var rock hexgrid.Coord
	for _, cell := range snapshot.Cells {
		if cell.Terrain == "rock" {
			rock = cell.Position
			break
		}
	}
	if _, err := PreviewHexRoute(ctx, id, 701, rock); !errors.Is(err, ErrHexBlocked) {
		t.Fatalf("rock: %v", err)
	}
	if _, err := PreviewHexRoute(ctx, id, 701, snapshot.Actors[1].Position); !errors.Is(err, ErrHexBlocked) {
		t.Fatalf("monster occupancy: %v", err)
	}
	// Block a floor-to-floor edge. The search must detour or reject it.
	key, _ := hexgrid.CanonicalEdge(start, dest)
	direction := -1
	for i := 0; i < 3; i++ {
		n, _ := hexgrid.Neighbor(key.A, i)
		if n == key.B {
			direction = i
			break
		}
	}
	if direction < 0 { // Lexicographic canonical endpoint can face a negative half direction.
		for i := 0; i < 3; i++ {
			n, _ := hexgrid.Neighbor(key.B, i)
			if n == key.A {
				key.A = key.B
				direction = i
				break
			}
		}
	}
	if direction < 0 {
		t.Fatal("no canonical SQL edge direction")
	}
	if _, err := db.Exec(`INSERT INTO match_hex_edges(instance_id,q,r,direction,wall_kind) VALUES($1,$2,$3,$4,'wall')`, id, key.A.Q, key.A.R, direction); err != nil {
		t.Fatal(err)
	}
	blocked, err := PreviewHexRoute(ctx, id, 701, dest)
	if err == nil && len(blocked.Path) == 1 {
		t.Fatal("crossed inserted wall")
	}
	if err != nil && !errors.Is(err, ErrHexBlocked) {
		t.Fatal(err)
	}
	if _, err := db.Exec(`DELETE FROM match_hex_edges WHERE instance_id=$1 AND q=$2 AND r=$3 AND direction=$4`, id, key.A.Q, key.A.R, direction); err != nil {
		t.Fatal(err)
	}
	// The route can cross multiple rooms and corridors, and cost scales by steps.
	longFound := false
	for _, cell := range snapshot.Cells {
		if cell.Terrain != "floor" || cell.Position == start {
			continue
		}
		route, err := PreviewHexRoute(ctx, id, 701, cell.Position)
		if err == nil && len(route.Path) >= 3 {
			if route.TotalCost != len(route.Path)*route.StepCost {
				t.Fatal("wrong energy cost")
			}
			longFound = true
			break
		}
	}
	if !longFound {
		t.Fatal("no corridor route")
	}
	if _, err := db.Exec(`UPDATE matches SET active_user_id=999 WHERE instance_id=$1`, id); err != nil {
		t.Fatal(err)
	}
	if _, err := PreviewHexRoute(ctx, id, 701, dest); !errors.Is(err, ErrHexUnavailable) {
		t.Fatalf("wrong active player preview: %v", err)
	}
	if _, err := ApplyHexMove(ctx, id, 701, "wrong-turn", dest); !errors.Is(err, ErrCommandRejected) {
		t.Fatalf("wrong active player move: %v", err)
	}
	if _, err := db.Exec(`UPDATE matches SET active_user_id=701 WHERE instance_id=$1`, id); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`UPDATE match_players SET energy=0 WHERE instance_id=$1 AND user_id=701`, id); err != nil {
		t.Fatal(err)
	}
	poor, err := PreviewHexRoute(ctx, id, 701, dest)
	if err != nil || poor.Affordable {
		t.Fatalf("zero energy preview=%+v err=%v", poor, err)
	}
	if _, err := ApplyHexMove(ctx, id, 701, "no-energy", dest); !errors.Is(err, ErrHexEnergy) {
		t.Fatalf("zero energy move: %v", err)
	}
	var state int64
	if err := db.QueryRow(`SELECT state_revision FROM matches WHERE instance_id=$1`, id).Scan(&state); err != nil || state != 1 {
		t.Fatalf("rejected revision=%d err=%v", state, err)
	}
}

func TestHexMidWriteRollbackAndPostCommitPostgres(t *testing.T) {
	db, _ := hexTestDatabase(t)
	ctx := context.Background()
	snapshot, err := CreateHexTestMatch(ctx, 701, 31)
	if err != nil {
		t.Fatal(err)
	}
	id := snapshot.InstanceID
	dest := walkableNeighbor(snapshot)
	if _, err := db.Exec(`CREATE FUNCTION fail_hex_destination() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN IF NEW.is_player AND NEW.q=` + fmt.Sprint(dest.Q) + ` AND NEW.r=` + fmt.Sprint(dest.R) + ` THEN RAISE EXCEPTION 'destination failure'; END IF; RETURN NEW; END $$`); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`CREATE TRIGGER fail_hex_destination BEFORE UPDATE ON match_map_cells FOR EACH ROW EXECUTE FUNCTION fail_hex_destination()`); err != nil {
		t.Fatal(err)
	}
	published := 0
	if _, err := ApplyHexMoveWithPublish(ctx, id, 701, "failed-mid-write", dest, func(HexRouteResult) { published++ }); err == nil {
		t.Fatal("expected mid-write error")
	}
	if published != 0 {
		t.Fatal("event published on rollback")
	}
	loaded, err := LoadHexTestMatch(ctx, id, 701)
	if err != nil || loaded.Player.Position != snapshot.Player.Position || loaded.Player.Energy != snapshot.Player.Energy || loaded.StateRevision != 1 || loaded.MapRevision != 1 {
		t.Fatalf("rollback snapshot=%+v err=%v", loaded.Player, err)
	}
	var startOccupied, destOccupied bool
	if err := db.QueryRow(`SELECT is_player FROM match_map_cells WHERE instance_id=$1 AND q=$2 AND r=$3`, id, snapshot.Player.Position.Q, snapshot.Player.Position.R).Scan(&startOccupied); err != nil {
		t.Fatal(err)
	}
	if err := db.QueryRow(`SELECT is_player FROM match_map_cells WHERE instance_id=$1 AND q=$2 AND r=$3`, id, dest.Q, dest.R).Scan(&destOccupied); err != nil {
		t.Fatal(err)
	}
	if !startOccupied || destOccupied {
		t.Fatal("partial occupancy survived rollback")
	}
	if _, err := db.Exec(`DROP TRIGGER fail_hex_destination ON match_map_cells`); err != nil {
		t.Fatal(err)
	}
	result, err := ApplyHexMoveWithPublish(ctx, id, 701, "successful-after-failure", dest, func(r HexRouteResult) {
		var q, energy int
		if e := db.QueryRow(`SELECT position_q,energy FROM match_players WHERE instance_id=$1 AND user_id=701`, id).Scan(&q, &energy); e != nil || q != dest.Q || energy != r.Energy {
			t.Errorf("published before commit q=%d energy=%d err=%v", q, energy, e)
		}
		published++
	})
	if err != nil || published != 1 || result.StateRevision != 2 {
		t.Fatalf("postcommit result=%+v published=%d err=%v", result, published, err)
	}
	_, err = ApplyHexMoveWithPublish(ctx, id, 701, "successful-after-failure", dest, func(HexRouteResult) { published++ })
	if err != nil || published != 1 {
		t.Fatalf("replay published=%d err=%v", published, err)
	}
}

func firstHexResource(t *testing.T, snapshot hexmap.Snapshot) (hexgrid.Coord, int) {
	t.Helper()
	for _, cell := range snapshot.Cells {
		if cell.Resource != nil {
			return cell.Position, cell.Resource.ID
		}
	}
	t.Fatal("HEX match has no resources")
	return hexgrid.Coord{}, 0
}

func placeHexPlayerOnCell(t *testing.T, db *sql.DB, id string, userID int, pos hexgrid.Coord) {
	t.Helper()
	if _, err := db.Exec(`UPDATE match_map_cells SET is_player=false WHERE instance_id=$1 AND is_player=true`, id); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`UPDATE match_map_cells SET is_player=true WHERE instance_id=$1 AND q=$2 AND r=$3`, id, pos.Q, pos.R); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`UPDATE match_players SET position_q=$3,position_r=$4 WHERE instance_id=$1 AND user_id=$2`, id, userID, pos.Q, pos.R); err != nil {
		t.Fatal(err)
	}
}

func TestHexResourceRequiresSameCellPostgres(t *testing.T) {
	for _, tc := range []struct {
		name     string
		distance int
	}{{"adjacent", 1}, {"far", 2}} {
		t.Run(tc.name, func(t *testing.T) {
			db, _ := hexTestDatabase(t)
			snapshot, err := CreateHexTestMatch(context.Background(), 701, 48)
			if err != nil {
				t.Fatal(err)
			}
			resource, _ := firstHexResource(t, snapshot)
			occupied := map[hexgrid.Coord]bool{}
			for _, actor := range snapshot.Actors {
				occupied[actor.Position] = true
			}
			var playerCell hexgrid.Coord
			found := false
			for _, cell := range snapshot.Cells {
				distance, _ := hexgrid.Distance(resource, cell.Position)
				if cell.Terrain == "floor" && !occupied[cell.Position] && cell.Position != resource && (tc.distance == 1 && distance == 1 || tc.distance == 2 && distance >= 2) {
					playerCell, found = cell.Position, true
					break
				}
			}
			if !found {
				t.Fatal("no test player cell at requested distance")
			}
			placeHexPlayerOnCell(t, db, snapshot.InstanceID, 701, playerCell)
			var inventory, resourceBefore string
			var stateBefore, mapBefore, ledgerBefore, itemsBefore int
			if err := db.QueryRow(`SELECT inventory::text FROM match_players WHERE instance_id=$1 AND user_id=701`, snapshot.InstanceID).Scan(&inventory); err != nil {
				t.Fatal(err)
			}
			if err := db.QueryRow(`SELECT resource::text FROM match_map_cells WHERE instance_id=$1 AND q=$2 AND r=$3`, snapshot.InstanceID, resource.Q, resource.R).Scan(&resourceBefore); err != nil {
				t.Fatal(err)
			}
			if err := db.QueryRow(`SELECT state_revision,map_revision FROM matches WHERE instance_id=$1`, snapshot.InstanceID).Scan(&stateBefore, &mapBefore); err != nil {
				t.Fatal(err)
			}
			if err := db.QueryRow(`SELECT COUNT(*) FROM match_commands WHERE instance_id=$1`, snapshot.InstanceID).Scan(&ledgerBefore); err != nil {
				t.Fatal(err)
			}
			if err := db.QueryRow(`SELECT COUNT(*) FROM inventory_items WHERE instance_id=$1 AND user_id=701`, snapshot.InstanceID).Scan(&itemsBefore); err != nil {
				t.Fatal(err)
			}
			published := 0
			_, err = ApplyHexResourceCollection(context.Background(), snapshot.InstanceID, 701, "wrong-cell", resource, func(HexCollectResult) { published++ })
			if !errors.Is(err, ErrHexInteractionPosition) || published != 0 {
				t.Fatalf("wrong-cell result: err=%v published=%d", err, published)
			}
			var inventoryAfter, resourceAfter string
			var stateAfter, mapAfter, ledgerAfter, itemsAfter int
			if err := db.QueryRow(`SELECT inventory::text FROM match_players WHERE instance_id=$1 AND user_id=701`, snapshot.InstanceID).Scan(&inventoryAfter); err != nil {
				t.Fatal(err)
			}
			if err := db.QueryRow(`SELECT resource::text FROM match_map_cells WHERE instance_id=$1 AND q=$2 AND r=$3`, snapshot.InstanceID, resource.Q, resource.R).Scan(&resourceAfter); err != nil {
				t.Fatal(err)
			}
			if err := db.QueryRow(`SELECT state_revision,map_revision FROM matches WHERE instance_id=$1`, snapshot.InstanceID).Scan(&stateAfter, &mapAfter); err != nil {
				t.Fatal(err)
			}
			if err := db.QueryRow(`SELECT COUNT(*) FROM match_commands WHERE instance_id=$1`, snapshot.InstanceID).Scan(&ledgerAfter); err != nil {
				t.Fatal(err)
			}
			if err := db.QueryRow(`SELECT COUNT(*) FROM inventory_items WHERE instance_id=$1 AND user_id=701`, snapshot.InstanceID).Scan(&itemsAfter); err != nil {
				t.Fatal(err)
			}
			if inventoryAfter != inventory || resourceAfter != resourceBefore || stateAfter != stateBefore || mapAfter != mapBefore || ledgerAfter != ledgerBefore || itemsAfter != itemsBefore {
				t.Fatalf("rejected collection mutated state: inventory=%v resource=%v revisions=%d/%d ledger=%d items=%d", inventoryAfter != inventory, resourceAfter != resourceBefore, stateAfter, mapAfter, ledgerAfter, itemsAfter)
			}
		})
	}
}

func TestHexResourceCollectionPostgres(t *testing.T) {
	db, dsn := hexTestDatabase(t)
	ctx := context.Background()
	snapshot, err := CreateHexTestMatch(ctx, 701, 48)
	if err != nil {
		t.Fatal(err)
	}
	pos, itemID := firstHexResource(t, snapshot)
	id := snapshot.InstanceID
	placeHexPlayerOnCell(t, db, id, 701, pos)
	published := 0
	var wg sync.WaitGroup
	results := make(chan error, 2)
	for _, commandID := range []string{"collect-a", "collect-b"} {
		wg.Add(1)
		go func(commandID string) {
			defer wg.Done()
			_, err := ApplyHexResourceCollection(ctx, id, 701, commandID, pos, func(HexCollectResult) { published++ })
			results <- err
		}(commandID)
	}
	wg.Wait()
	close(results)
	success, rejected := 0, 0
	for err := range results {
		if err == nil {
			success++
		} else if errors.Is(err, ErrCommandRejected) {
			rejected++
		} else {
			t.Fatal(err)
		}
	}
	if success != 1 || rejected != 1 || published != 1 {
		t.Fatalf("collection race success=%d rejected=%d published=%d", success, rejected, published)
	}
	var count int
	if err := db.QueryRow(`SELECT item_count FROM inventory_items WHERE instance_id=$1 AND user_id=701 AND item_id=$2`, id, itemID).Scan(&count); err != nil || count != 1 {
		t.Fatalf("item count=%d err=%v", count, err)
	}
	var commandID string
	if err := db.QueryRow(`SELECT command_id FROM match_commands WHERE instance_id=$1`, id).Scan(&commandID); err != nil {
		t.Fatal(err)
	}
	replay, err := ApplyHexResourceCollection(ctx, id, 701, commandID, pos, func(HexCollectResult) { published++ })
	if err != nil || !replay.Replayed || published != 1 {
		t.Fatalf("replay=%+v err=%v published=%d", replay, err, published)
	}
	if _, err := ApplyHexResourceCollection(ctx, id, 701, commandID, hexgrid.Coord{}, nil); !errors.Is(err, ErrCommandConflict) {
		t.Fatalf("payload conflict: %v", err)
	}
	fresh, err := sql.Open("postgres", dsn)
	if err != nil {
		t.Fatal(err)
	}
	defer fresh.Close()
	DB = fresh
	loaded, loadErr := LoadHexTestMatch(ctx, id, 701)
	DB = db
	if loadErr != nil || loaded.StateRevision != 2 || loaded.MapRevision != 2 {
		t.Fatalf("snapshot revisions=%d/%d err=%v", loaded.StateRevision, loaded.MapRevision, loadErr)
	}
	for _, cell := range loaded.Cells {
		if cell.Position == pos && cell.Resource != nil {
			t.Fatal("collected resource reappeared after restart")
		}
	}
	if len(loaded.Player.Inventory) == 0 {
		t.Fatal("inventory missing after restart")
	}
}

func TestHexCollectionAndTurnRollbackPostgres(t *testing.T) {
	db, _ := hexTestDatabase(t)
	ctx := context.Background()
	snapshot, err := CreateHexTestMatch(ctx, 701, 49)
	if err != nil {
		t.Fatal(err)
	}
	id := snapshot.InstanceID
	pos, _ := firstHexResource(t, snapshot)
	placeHexPlayerOnCell(t, db, id, 701, pos)
	if _, err := db.Exec(`CREATE FUNCTION fail_hex_inventory() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN RAISE EXCEPTION 'inventory write failed'; END $$`); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`CREATE TRIGGER fail_hex_inventory BEFORE INSERT ON inventory_items FOR EACH ROW EXECUTE FUNCTION fail_hex_inventory()`); err != nil {
		t.Fatal(err)
	}
	published := 0
	if _, err := ApplyHexResourceCollection(ctx, id, 701, "collect-fail", pos, func(HexCollectResult) { published++ }); err == nil {
		t.Fatal("expected inventory failure")
	}
	if published != 0 {
		t.Fatal("published before commit")
	}
	loaded, err := LoadHexTestMatch(ctx, id, 701)
	if err != nil {
		t.Fatal(err)
	}
	if loaded.StateRevision != 1 || loaded.MapRevision != 1 {
		t.Fatal("failed collection changed revisions")
	}
	for _, cell := range loaded.Cells {
		if cell.Position == pos && cell.Resource == nil {
			t.Fatal("failed collection removed resource")
		}
	}
	if _, err := db.Exec(`DROP TRIGGER fail_hex_inventory ON inventory_items`); err != nil {
		t.Fatal(err)
	}
	regen := func(class string) int {
		if class != "guardian" {
			t.Errorf("unexpected class %s", class)
		}
		return 10
	}
	if _, err := db.Exec(`UPDATE match_players SET energy=3 WHERE instance_id=$1`, id); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`CREATE FUNCTION fail_hex_turn() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN IF NEW.turn_number>OLD.turn_number THEN RAISE EXCEPTION 'turn write failed'; END IF; RETURN NEW; END $$`); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`CREATE TRIGGER fail_hex_turn BEFORE UPDATE ON matches FOR EACH ROW EXECUTE FUNCTION fail_hex_turn()`); err != nil {
		t.Fatal(err)
	}
	if _, err := ApplyHexEndTurn(ctx, id, 701, "turn-fail", 1, regen, func(HexTurnResult) { published++ }); err == nil {
		t.Fatal("expected turn failure")
	}
	if published != 0 {
		t.Fatal("turn event published before commit")
	}
	var energy, turn int
	if err := db.QueryRow(`SELECT energy FROM match_players WHERE instance_id=$1`, id).Scan(&energy); err != nil || energy != 3 {
		t.Fatalf("rollback energy=%d err=%v", energy, err)
	}
	if err := db.QueryRow(`SELECT turn_number FROM matches WHERE instance_id=$1`, id).Scan(&turn); err != nil || turn != 1 {
		t.Fatalf("rollback turn=%d err=%v", turn, err)
	}
	if _, err := db.Exec(`DROP TRIGGER fail_hex_turn ON matches`); err != nil {
		t.Fatal(err)
	}
	result, err := ApplyHexEndTurn(ctx, id, 701, "turn-ok", 1, regen, func(HexTurnResult) { published++ })
	if err != nil || result.TurnNumber != 2 || result.Energy != 13 || result.StateRevision != 2 || result.MapRevision != 1 || published != 1 {
		t.Fatalf("turn=%+v err=%v published=%d", result, err, published)
	}
	replay, err := ApplyHexEndTurn(ctx, id, 701, "turn-ok", 1, regen, func(HexTurnResult) { published++ })
	if err != nil || !replay.Replayed || published != 1 {
		t.Fatalf("turn replay=%+v err=%v", replay, err)
	}
	if _, err := ApplyHexEndTurn(ctx, id, 701, "timer-stale", 1, regen, nil); !errors.Is(err, ErrCommandRejected) {
		t.Fatalf("stale timer: %v", err)
	}
	if _, err := ApplyHexEndTurn(ctx, id, 701, "turn-ok", 2, regen, nil); !errors.Is(err, ErrCommandConflict) {
		t.Fatalf("turn payload conflict: %v", err)
	}
}

func TestHexConcurrentTurnAndMovementPostgres(t *testing.T) {
	db, dsn := hexTestDatabase(t)
	ctx := context.Background()
	snapshot, err := CreateHexTestMatch(ctx, 701, 50)
	if err != nil {
		t.Fatal(err)
	}
	id := snapshot.InstanceID
	dest := walkableNeighbor(snapshot)
	if dest == (hexgrid.Coord{}) {
		t.Fatal("no free neighbor")
	}
	if _, err := db.Exec(`UPDATE match_players SET energy=40 WHERE instance_id=$1`, id); err != nil {
		t.Fatal(err)
	}
	regen := func(string) int { return 10 }
	var wg sync.WaitGroup
	results := make(chan error, 3)
	for _, commandID := range []string{"http-turn", "timer-turn"} {
		wg.Add(1)
		go func(commandID string) {
			defer wg.Done()
			_, err := ApplyHexEndTurn(ctx, id, 701, commandID, 1, regen, nil)
			results <- err
		}(commandID)
	}
	wg.Add(1)
	go func() { defer wg.Done(); _, err := ApplyHexMove(ctx, id, 701, "turn-race-move", dest); results <- err }()
	wg.Wait()
	close(results)
	success, rejected := 0, 0
	for err := range results {
		if err == nil {
			success++
		} else if errors.Is(err, ErrCommandRejected) {
			rejected++
		} else {
			t.Fatal(err)
		}
	}
	if success != 2 || rejected != 1 {
		t.Fatalf("turn/move race success=%d rejected=%d", success, rejected)
	}
	fresh, err := sql.Open("postgres", dsn)
	if err != nil {
		t.Fatal(err)
	}
	defer fresh.Close()
	DB = fresh
	loaded, loadErr := LoadHexTestMatch(ctx, id, 701)
	DB = db
	if loadErr != nil || loaded.TurnNumber != 2 || loaded.Player.Position != dest || loaded.Player.Energy != 50-4 || loaded.StateRevision != 3 || loaded.MapRevision != 2 {
		t.Fatalf("fresh turn/move snapshot=%+v player=%+v err=%v", loaded, loaded.Player, loadErr)
	}
}
