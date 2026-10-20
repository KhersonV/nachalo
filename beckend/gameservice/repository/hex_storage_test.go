package repository

import (
	"encoding/json"
	"errors"
	"testing"
)

func TestHexLoadoutTransferIsSeparateFromStorage(t *testing.T) {
	storage, err := parseStorage([]byte(`{"resource_7":{"name":"Water","item_count":3},"resource_8":{"name":"Wood","item_count":4}}`))
	if err != nil {
		t.Fatal(err)
	}
	match, err := transferLoadout(storage, map[string]int{"resource_7": 2})
	if err != nil {
		t.Fatal(err)
	}
	if storageCount(match["resource_7"]) != 2 || storageCount(storage["resource_7"]) != 1 || storageCount(storage["resource_8"]) != 4 {
		t.Fatalf("unexpected transfer: match=%v storage=%v", match, storage)
	}
	match["resource_7"]["item_count"] = 0
	if storageCount(storage["resource_7"]) != 1 {
		t.Fatal("match entry aliases permanent Storage")
	}
}

func TestHexLoadoutRejectsOverdrawWithoutMutation(t *testing.T) {
	storage := map[string]map[string]any{"resource_7": {"item_count": float64(1)}}
	_, err := transferLoadout(storage, map[string]int{"resource_7": 2})
	if !errors.Is(err, ErrLoadoutUnavailable) || storageCount(storage["resource_7"]) != 1 {
		t.Fatalf("overdraw changed Storage: %v %v", err, storage)
	}
	_, err = transferLoadout(storage, map[string]int{"resource_7": -1})
	if !errors.Is(err, ErrLoadoutInvalid) {
		t.Fatalf("negative selection accepted: %v", err)
	}
	data, err := json.Marshal(storage)
	if err != nil || string(data) != `{"resource_7":{"item_count":1}}` {
		t.Fatalf("Storage changed: %s %v", data, err)
	}
}
