package repository

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"testing"

	"gameservice/internal/hexmap"
)

// Runs only with STAGE2_TEST_DSN pointed at an isolated PostgreSQL database.
func TestHexStorageLoadoutAtomicPostgres(t *testing.T) {
	db, _ := hexTestDatabase(t)
	ctx := context.Background()
	water, err := GetResourceByType("water")
	if err != nil {
		t.Fatal(err)
	}
	key := fmt.Sprintf("resource_%d", water.ID)
	if _, err := db.Exec(`UPDATE player_profiles SET inventory=jsonb_build_object($2,jsonb_build_object('name','Water','item_count',3,'image','')) WHERE user_id=$1`, 701, key); err != nil {
		t.Fatal(err)
	}
	if _, err := SaveHexLoadout(ctx, 701, map[string]int{key: 2}); err != nil {
		t.Fatal(err)
	}
	before, err := GetHexStorage(ctx, 701)
	if err != nil || storageCount(before.Inventory[key]) != 3 {
		t.Fatalf("selection spent Storage before match: %+v %v", before, err)
	}
	plan := HexMatchPlan{InstanceID: "loadout-atomic", Mode: "PVE", Teams: []hexmap.Team{{ID: 1, PlayerIDs: []int{701}}}, TurnOrder: []int{701}, Seed: 413}
	snapshot, err := CreateHexMatch(ctx, plan)
	if err != nil {
		t.Fatal(err)
	}
	after, err := GetHexStorage(ctx, 701)
	if err != nil || storageCount(after.Inventory[key]) != 1 || len(after.Loadout) != 0 {
		t.Fatalf("Storage transfer: %+v %v", after, err)
	}
	backpackItem, _ := snapshot.Player.Inventory[key].(map[string]any)
	if storageCount(backpackItem) != 2 {
		t.Fatalf("match backpack: %+v", snapshot.Player.Inventory)
	}
	var count int
	if err := db.QueryRow(`SELECT item_count FROM inventory_items WHERE instance_id=$1 AND user_id=701 AND item_type='resource' AND item_id=$2`, plan.InstanceID, water.ID).Scan(&count); err != nil || count != 2 {
		t.Fatalf("normalized backpack: %d %v", count, err)
	}
	if _, err := SaveHexLoadout(ctx, 701, map[string]int{key: 2}); !errors.Is(err, ErrLoadoutUnavailable) {
		t.Fatalf("overdraw accepted: %v", err)
	}
	if _, err := SaveHexLoadout(ctx, 701, map[string]int{key: 1}); err != nil {
		t.Fatal(err)
	}
	if _, err := CreateHexMatch(ctx, plan); err == nil {
		t.Fatal("duplicate match creation succeeded")
	}
	rolledBack, err := GetHexStorage(ctx, 701)
	if err != nil || storageCount(rolledBack.Inventory[key]) != 1 || rolledBack.Loadout[key] != 1 {
		t.Fatalf("failed match creation spent Storage: %+v %v", rolledBack, err)
	}
}

func TestHexStorageConcurrentCreatePostgres(t *testing.T) {
	db, _ := hexTestDatabase(t)
	ctx := context.Background()
	water, err := GetResourceByType("water")
	if err != nil {
		t.Fatal(err)
	}
	key := fmt.Sprintf("resource_%d", water.ID)
	if _, err := db.Exec(`UPDATE player_profiles SET inventory=jsonb_build_object($2,jsonb_build_object('name','Water','item_count',1)) WHERE user_id=$1`, 701, key); err != nil {
		t.Fatal(err)
	}
	if _, err := SaveHexLoadout(ctx, 701, map[string]int{key: 1}); err != nil {
		t.Fatal(err)
	}
	ids := []string{"loadout-race-a", "loadout-race-b"}
	var wg sync.WaitGroup
	errs := make(chan error, len(ids))
	for _, id := range ids {
		wg.Add(1)
		go func(instanceID string) {
			defer wg.Done()
			_, createErr := CreateHexMatch(ctx, HexMatchPlan{InstanceID: instanceID, Mode: "PVE", Teams: []hexmap.Team{{ID: 1, PlayerIDs: []int{701}}}, TurnOrder: []int{701}, Seed: 51})
			errs <- createErr
		}(id)
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		if err != nil {
			t.Fatal(err)
		}
	}
	var total int
	if err := db.QueryRow(`SELECT COALESCE(sum(item_count),0) FROM inventory_items WHERE user_id=701 AND item_type='resource' AND item_id=$1 AND instance_id IN ($2,$3)`, water.ID, ids[0], ids[1]).Scan(&total); err != nil || total != 1 {
		t.Fatalf("double loadout transfer: total=%d err=%v", total, err)
	}
}
