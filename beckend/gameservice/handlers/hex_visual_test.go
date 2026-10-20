package handlers

import (
	"testing"

	"gameservice/internal/hexgrid"
	"gameservice/internal/hexmap"
)

func TestHexVisualRespectsFog(t *testing.T) {
	cells := []hexmap.Cell{
		{Position: hexgrid.Coord{Q: 0, R: 0}, Visibility: "visible"},
		{Position: hexgrid.Coord{Q: 1, R: 0}, Visibility: "explored"},
	}
	attack := map[string]interface{}{"kind": "attack", "attackerPosition": map[string]interface{}{"q": float64(0), "r": float64(0)}, "targetPosition": map[string]interface{}{"q": float64(1), "r": float64(0)}}
	if hexVisualVisibleToUser(cells, attack) {
		t.Fatal("hidden target disclosed")
	}
	cells[1].Visibility = "visible"
	if !hexVisualVisibleToUser(cells, attack) {
		t.Fatal("visible combat suppressed")
	}
	attack["effects"] = []interface{}{map[string]interface{}{"kind": "push", "positionAfter": map[string]interface{}{"x": float64(2), "y": float64(0)}}}
	if hexVisualVisibleToUser(cells, attack) {
		t.Fatal("hidden push destination disclosed")
	}
	delete(attack, "effects")
	move := map[string]interface{}{"kind": "move", "position": map[string]interface{}{"q": float64(1), "r": float64(0)}, "path": []interface{}{map[string]interface{}{"q": float64(0), "r": float64(0)}, map[string]interface{}{"q": float64(1), "r": float64(0)}}}
	if !hexVisualVisibleToUser(cells, move) {
		t.Fatal("visible route suppressed")
	}
	cells[0].Visibility = "explored"
	if hexVisualVisibleToUser(cells, move) {
		t.Fatal("hidden route disclosed")
	}
}
