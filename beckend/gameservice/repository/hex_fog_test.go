package repository

import (
	"testing"

	"gameservice/internal/hexgrid"
	"gameservice/internal/hexmap"
)

func TestHexVisibleBlocksWallsAndLimitsRadius(t *testing.T) {
	cells := []hexmap.Cell{}
	for q := 0; q <= 5; q++ {
		cells = append(cells, hexmap.Cell{Position: hexgrid.Coord{Q: q, R: 0}, Terrain: "floor"})
	}
	origin := hexgrid.Coord{Q: 0, R: 0}
	open := hexVisible([]hexgrid.Coord{origin}, cells, nil)
	if !open[origin] || !open[hexgrid.Coord{Q: 2, R: 0}] || open[hexgrid.Coord{Q: 5, R: 0}] {
		t.Fatalf("unexpected open vision: %+v", open)
	}
	blocked := hexVisible([]hexgrid.Coord{origin}, cells, []hexmap.Edge{{Position: origin, Direction: 0, Kind: "wall"}})
	if blocked[hexgrid.Coord{Q: 1, R: 0}] || blocked[hexgrid.Coord{Q: 2, R: 0}] {
		t.Fatalf("wall leaked vision: %+v", blocked)
	}
}
