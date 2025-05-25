
//=================================
// gameservice/game/mapgen_test.go
//=================================

package game

import (
	"testing"
)

func TestGenerateFullMap_InvalidConfig(t *testing.T) {
	_, _, _, _, _, err := GenerateFullMap(MapConfig{TotalPlayers: 0, TeamsCount: 1}, nil, nil)
	if err == nil {
		t.Errorf("expected error for TotalPlayers=0")
	}
	_, _, _, _, _, err = GenerateFullMap(MapConfig{TotalPlayers: 1, TeamsCount: 0}, nil, nil)
	if err == nil {
		t.Errorf("expected error for TeamsCount=0")
	}
}

func TestGenerateFullMap_BasicProperties(t *testing.T) {
	cfg := MapConfig{TotalPlayers: 1, TeamsCount: 1, WalkableProb: 1.0}
	full, w, h, starts, portal, err := GenerateFullMap(cfg, nil, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if w != 15 || h != 15 {
		t.Errorf("expected size 15×15, got %dx%d", w, h)
	}
	if len(starts) != 1 {
		t.Errorf("expected 1 start, got %d", len(starts))
	}
	// портал не совпадает со стартом
	if starts[0][0] == portal[0] && starts[0][1] == portal[1] {
		t.Errorf("portal should differ from start")
	}
	if len(full) != w*h {
		t.Errorf("expected %d cells, got %d", w*h, len(full))
	}
}
