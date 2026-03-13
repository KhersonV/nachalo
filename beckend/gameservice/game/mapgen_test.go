
//=================================
// gameservice/game/mapgen_test.go
//=================================

package game

import (
	"testing"
)

func testCellReachableToPortal(cells []FullCell, width, height int, fromX, fromY, toX, toY int) bool {
	indexByXY := make(map[[2]int]FullCell, len(cells))
	for _, c := range cells {
		indexByXY[[2]int{c.X, c.Y}] = c
	}

	isPassable := func(c FullCell) bool {
		tile := c.TileCode
		return tile == int(Walkable) ||
			tile == int(StartTile) ||
			tile == int(Portal) ||
			tile == int('R') ||
			tile == int('B') ||
			tile == int('M')
	}

	visited := make(map[[2]int]bool)
	q := [][2]int{{fromX, fromY}}
	visited[[2]int{fromX, fromY}] = true
	dirs := [][2]int{{1, 0}, {-1, 0}, {0, 1}, {0, -1}}

	for len(q) > 0 {
		p := q[0]
		q = q[1:]
		if p[0] == toX && p[1] == toY {
			return true
		}
		for _, d := range dirs {
			nx, ny := p[0]+d[0], p[1]+d[1]
			if nx < 0 || ny < 0 || nx >= width || ny >= height {
				continue
			}
			key := [2]int{nx, ny}
			if visited[key] {
				continue
			}
			cell, ok := indexByXY[key]
			if !ok || !isPassable(cell) {
				continue
			}
			visited[key] = true
			q = append(q, key)
		}
	}

	return false
}

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

func TestGenerateFullMap_AllObjectsReachPortal(t *testing.T) {
	cfg := MapConfig{
		TotalPlayers: 2,
		TeamsCount:   1,
		WalkableProb: 0.7,
		MonsterProb:  0.25,
		BarbelProb:   0.35,
		ResourceProb: 0.35,
	}

	resources := []ResourceData{
		{ID: 6, Type: "barrel", Description: "Barrel", Effect: map[string]int{}},
		{ID: 1, Type: "wood", Description: "Wood", Effect: map[string]int{"mat": 1}},
	}
	monsters := []MonsterData{
		{ID: 1, Name: "Goblin", Type: "aggressive", Health: 20, MaxHealth: 20, Attack: 4, Defense: 1, Speed: 3, Maneuverability: 3, Vision: 3},
	}

	for i := 0; i < 20; i++ {
		full, w, h, _, portal, err := GenerateFullMap(cfg, resources, monsters)
		if err != nil {
			t.Fatalf("unexpected map generation error on iteration %d: %v", i, err)
		}

		for _, cell := range full {
			hasObject := cell.Barbel != nil || cell.Resource != nil || cell.Monster != nil
			if !hasObject {
				continue
			}
			if !testCellReachableToPortal(full, w, h, cell.X, cell.Y, portal[0], portal[1]) {
				t.Fatalf("object at (%d,%d) not reachable to portal (%d,%d) on iteration %d", cell.X, cell.Y, portal[0], portal[1], i)
			}
		}
	}
}
