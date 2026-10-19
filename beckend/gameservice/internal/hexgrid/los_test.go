package hexgrid

import "testing"

func TestVisibleHexWallsRockAndNegativeCoordinates(t *testing.T) {
	from, to := Coord{-2, 0}, Coord{0, 0}
	floor := map[Coord]bool{from: true, {-1, 0}: true, to: true}
	if !Visible(from, to, floor, nil) || !Visible(to, from, floor, nil) {
		t.Fatal("open line blocked")
	}
	edge, _ := CanonicalEdge(from, Coord{-1, 0})
	if Visible(from, to, floor, map[EdgeKey]bool{edge: true}) || Visible(to, from, floor, map[EdgeKey]bool{edge: true}) {
		t.Fatal("wall did not block both directions")
	}
	floor[Coord{-1, 0}] = false
	if Visible(from, to, floor, nil) || Visible(to, from, floor, nil) {
		t.Fatal("rock did not block")
	}
	if Visible(from, from, floor, nil) {
		t.Fatal("overlap is not a line of sight")
	}
}
