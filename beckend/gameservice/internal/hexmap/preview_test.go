package hexmap

import (
	"encoding/json"
	"reflect"
	"testing"

	"gameservice/internal/hexgrid"
	"gameservice/models"
)

func TestPreviewTopologyAndContract(t *testing.T) {
	for seed := int64(-20); seed <= 20; seed++ {
		preview, err := Generate(seed)
		if err != nil {
			t.Fatal(err)
		}
		if preview.SchemaVersion != 2 || preview.Geometry != models.HexV1 || preview.MapRevision != 1 {
			t.Fatalf("invalid version: %+v", preview)
		}
		floor := map[hexgrid.Coord]floorInfo{}
		cells := map[hexgrid.Coord]Cell{}
		negative := false
		for _, cell := range preview.Cells {
			if _, exists := cells[cell.Position]; exists {
				t.Fatalf("duplicate cell %v", cell.Position)
			}
			cells[cell.Position] = cell
			if cell.Position.Q < 0 && cell.Position.R < 0 {
				negative = true
			}
			if cell.Terrain == "floor" {
				floor[cell.Position] = floorInfo{room: cell.Room}
			}
		}
		if !negative || !connected(floor) {
			t.Fatalf("seed %d: negative=%v connected=%v", seed, negative, connected(floor))
		}
		for _, actor := range preview.Actors {
			if cells[actor.Position].Terrain != "floor" {
				t.Fatalf("actor on rock: %+v", actor)
			}
		}
		walls := map[[3]int]bool{}
		for _, edge := range preview.Edges {
			if edge.Direction < 0 || edge.Direction > 2 {
				t.Fatal(edge)
			}
			next, _ := hexgrid.Neighbor(edge.Position, edge.Direction)
			if cells[edge.Position].Terrain == cells[next].Terrain {
				t.Fatalf("wall not on floor boundary: %+v", edge)
			}
			walls[[3]int{edge.Position.Q, edge.Position.R, edge.Direction}] = true
		}
		for _, cell := range preview.Cells {
			for direction := 0; direction < 3; direction++ {
				next, _ := hexgrid.Neighbor(cell.Position, direction)
				other, exists := cells[next]
				if !exists {
					continue
				}
				want := cell.Terrain != other.Terrain
				if walls[[3]int{cell.Position.Q, cell.Position.R, direction}] != want {
					t.Fatalf("seed %d: edge %v dir %d", seed, cell.Position, direction)
				}
			}
		}
		data, err := json.Marshal(preview)
		if err != nil {
			t.Fatal(err)
		}
		var decoded Snapshot
		if err := json.Unmarshal(data, &decoded); err != nil {
			t.Fatal(err)
		}
		if !reflect.DeepEqual(preview, decoded) {
			t.Fatal("HEX round trip changed map")
		}
		var raw map[string]any
		_ = json.Unmarshal(data, &raw)
		position := raw["cells"].([]any)[0].(map[string]any)["position"].(map[string]any)
		if _, exists := position["x"]; exists {
			t.Fatal("HEX position leaked x")
		}
		if _, exists := position["q"]; !exists {
			t.Fatal("HEX position lacks q")
		}
	}
}

func TestPreviewDeterministic(t *testing.T) {
	a, _ := Generate(937)
	b, _ := Generate(937)
	if !reflect.DeepEqual(a, b) {
		t.Fatal("same seed changed topology")
	}
}
