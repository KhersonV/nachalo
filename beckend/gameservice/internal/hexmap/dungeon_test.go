package hexmap

import (
	"gameservice/game"
	"gameservice/internal/hexgrid"
	"reflect"
	"testing"
)

func testCatalogs() Catalogs {
	return Catalogs{Monsters: []game.MonsterData{{ID: 2, Name: "Goblin", Type: "aggressive", Health: 35, MaxHealth: 35, Attack: 8, Defense: 3, Speed: 2, Maneuverability: 1, Vision: 3, Image: "goblin.webp"}, {ID: 1, Name: "Bat", Type: "flying", Health: 20, MaxHealth: 20, Attack: 5, Defense: 2, Speed: 4, Image: "bat.webp"}}, Resources: []game.ResourceData{{ID: 6, Type: "barrel", Image: "barrel.webp"}, {ID: 3, Type: "wood"}, {ID: 1, Type: "food"}, {ID: 2, Type: "water"}}}
}
func testConfig(teams [][]int) Config {
	cfg := Config{MonsterProb: .05, BarrelProb: .1, ResourceProb: .1}
	for i, ids := range teams {
		cfg.Teams = append(cfg.Teams, Team{ID: i + 1, PlayerIDs: ids})
	}
	return cfg
}
func TestDungeonConfigurations(t *testing.T) {
	cases := []struct {
		name  string
		teams [][]int
	}{
		{"solo", [][]int{{1}}}, {"two-ffa", [][]int{{1}, {2}}}, {"four-ffa", [][]int{{1}, {2}, {3}, {4}}},
		{"six-ffa", [][]int{{1}, {2}, {3}, {4}, {5}, {6}}}, {"eight-ffa", [][]int{{1}, {2}, {3}, {4}, {5}, {6}, {7}, {8}}},
		{"eight-one-team", [][]int{{1, 2, 3, 4, 5, 6, 7, 8}}},
		{"two-teams", [][]int{{1, 2, 3}, {4, 5, 6}}}, {"unequal-three-teams", [][]int{{1}, {2, 3}, {4, 5, 6, 7, 8}}},
		{"ten-two-teams", [][]int{{1, 2, 3, 4, 5}, {6, 7, 8, 9, 10}}},
	}
	radii := map[string]int{}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			cfg := testConfig(tc.teams)
			a, err := GenerateDungeon(cfg, testCatalogs(), 97)
			if err != nil {
				t.Fatal(err)
			}
			b, err := GenerateDungeon(cfg, testCatalogs(), 97)
			if err != nil || !reflect.DeepEqual(a, b) {
				t.Fatal("non-deterministic map", err)
			}
			radii[tc.name] = a.Radius
			floor := map[hexgrid.Coord]floorInfo{}
			occupied := map[hexgrid.Coord]string{}
			starts := map[int]hexgrid.Coord{}
			for _, c := range a.Cells {
				if c.Terrain == "floor" {
					floor[c.Position] = floorInfo{}
				}
				if c.IsPortal {
					if a.Portal == nil || c.Position != *a.Portal {
						t.Fatal("portal mismatch")
					}
				}
				if c.Resource != nil || c.Barrel != nil {
					if c.Terrain != "floor" {
						t.Fatal("object on rock")
					}
					if c.Resource != nil && c.Barrel != nil {
						t.Fatal("object overlap")
					}
					occupied[c.Position] = "object"
				}
			}
			if !connected(floor) || a.Portal == nil || *a.Portal == (hexgrid.Coord{}) {
				t.Fatal("topology or portal")
			}
			for _, actor := range a.Actors {
				if _, ok := floor[actor.Position]; !ok {
					t.Fatal("actor on rock")
				}
				if occupied[actor.Position] != "" {
					t.Fatal("actor overlap")
				}
				occupied[actor.Position] = actor.Kind
				if actor.Kind == "monster" && actor.MonsterRefID <= 0 {
					t.Fatal("monster catalog lost")
				}
			}
			for _, start := range a.Starts {
				if occupied[start.Position] != "player" {
					t.Fatal("bad start")
				}
				starts[start.UserID] = start.Position
				blocked := map[hexgrid.Coord]floorInfo{}
				for c, info := range floor {
					if occupied[c] != "monster" && occupied[c] != "object" {
						blocked[c] = info
					}
				}
				if path := floorPath(start.Position, *a.Portal, blocked); len(path) == 0 {
					t.Fatal("portal unreachable")
				}
			}
			if len(starts) != len(a.Starts) {
				t.Fatal("duplicate start")
			}
			for _, team := range tc.teams {
				for _, id := range team[1:] {
					d, _ := hexgrid.Distance(starts[team[0]], starts[id])
					if d > 4 {
						t.Fatalf("team too spread: %d", d)
					}
				}
			}
			for i, team := range tc.teams {
				for _, other := range tc.teams[i+1:] {
					for _, id := range team {
						for _, otherID := range other {
							d, _ := hexgrid.Distance(starts[id], starts[otherID])
							if d < 3 {
								t.Fatalf("teams start too close: %d and %d, distance %d", id, otherID, d)
							}
						}
					}
				}
			}
			monsters, barrels, resources := 0, 0, 0
			for _, actor := range a.Actors {
				if actor.Kind == "monster" {
					monsters++
				}
			}
			for _, c := range a.Cells {
				if c.Barrel != nil {
					barrels++
					if c.Barrel.ID != 6 {
						t.Fatal("barrel catalog mismatch")
					}
				}
				if c.Resource != nil {
					resources++
					if c.Resource.Type == "barrel" {
						t.Fatal("barrel in resources")
					}
				}
			}
			if monsters == 0 || barrels == 0 || resources == 0 {
				t.Fatalf("missing objects %d/%d/%d", monsters, barrels, resources)
			}
		})
	}
	if radii["eight-ffa"] <= radii["solo"] {
		t.Fatalf("size did not grow: %v", radii)
	}
}
func TestDungeonRejectsInvalidTeams(t *testing.T) {
	_, err := GenerateDungeon(testConfig([][]int{{1, 2}, {2}}), testCatalogs(), 1)
	if err == nil {
		t.Fatal("duplicate player accepted")
	}
}
