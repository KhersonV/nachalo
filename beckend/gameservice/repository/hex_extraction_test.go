package repository

import "testing"

func TestMergeHexExtractionStorage(t *testing.T) {
	storage := map[string]map[string]any{"resource_1": {"name": "water", "item_count": 2}}
	backpack := map[string]map[string]any{
		"resource_1":     {"name": "water", "item_count": 3},
		"resource_2":     {"name": "wood", "item_count": 4},
		"scroll_2001":    {"name": "scroll", "item_count": 1},
		"blueprint_wall": {"name": "wall", "item_count": 1},
		"artifact_71":    {"name": "quest", "item_count": 1},
		"artifact_72":    {"name": "ordinary", "item_count": 1},
	}
	transferred, err := mergeHexExtractionStorage(storage, backpack, 71)
	if err != nil {
		t.Fatal(err)
	}
	if storageCount(storage["resource_1"]) != 5 || storageCount(storage["resource_2"]) != 4 || storageCount(storage["scroll_2001"]) != 1 || storageCount(storage["blueprint_wall"]) != 1 || storageCount(storage["artifact_72"]) != 1 {
		t.Fatalf("lost loot: %+v", storage)
	}
	if _, ok := storage["artifact_71"]; ok {
		t.Fatal("quest artifact entered permanent Storage")
	}
	if _, ok := transferred["artifact_71"]; ok {
		t.Fatal("quest artifact listed as transferred")
	}
	if storageCount(transferred["resource_1"]) != 3 {
		t.Fatal("transferred list includes prior Storage")
	}
}

func TestMergeHexExtractionStorageRejectsUnknownItem(t *testing.T) {
	_, err := mergeHexExtractionStorage(map[string]map[string]any{}, map[string]map[string]any{"unknown": {"item_count": 1}}, 0)
	if err == nil {
		t.Fatal("unknown item should stop transfer")
	}
}
