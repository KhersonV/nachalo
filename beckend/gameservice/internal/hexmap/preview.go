// Package hexmap builds an isolated, deterministic HEX preview. It does not
// create a playable match; future HEX commands must use MatchCommand.
package hexmap

import (
	"fmt"
	"math/rand"
	"sort"

	"gameservice/game"
	"gameservice/internal/hexgrid"
	"gameservice/models"
)

type Cell struct {
	CellID   int                `json:"cellId"`
	Position hexgrid.Coord      `json:"position"`
	Terrain  string             `json:"terrain"`
	Room     int                `json:"room,omitempty"`
	Resource *game.ResourceData `json:"resource,omitempty"`
}
type Edge struct {
	Position  hexgrid.Coord `json:"position"`
	Direction int           `json:"direction"`
	Kind      string        `json:"kind"`
}
type Actor struct {
	ID        string        `json:"id"`
	Kind      string        `json:"kind"`
	Position  hexgrid.Coord `json:"position"`
	Health    int           `json:"health,omitempty"`
	MaxHealth int           `json:"maxHealth,omitempty"`
}
type Player struct {
	UserID    int            `json:"userId"`
	Position  hexgrid.Coord  `json:"position"`
	Energy    int            `json:"energy"`
	MaxEnergy int            `json:"maxEnergy"`
	Health    int            `json:"health"`
	Mobility  int            `json:"mobility"`
	Inventory map[string]any `json:"inventory,omitempty"`
}
type Snapshot struct {
	SchemaVersion int             `json:"schemaVersion"`
	Geometry      models.Geometry `json:"geometry"`
	InstanceID    string          `json:"instanceId"`
	MapRevision   int64           `json:"mapRevision"`
	StateRevision int64           `json:"stateRevision"`
	Seed          int64           `json:"seed"`
	Cells         []Cell          `json:"cells"`
	Edges         []Edge          `json:"edges"`
	Actors        []Actor         `json:"actors"`
	Player        *Player         `json:"player,omitempty"`
	ActiveUser    int             `json:"activeUser,omitempty"`
	TurnNumber    int             `json:"turnNumber,omitempty"`
}

const radius = 8

type floorInfo struct{ room int }

// Generate returns a versioned, read-only preview. No database state changes.
func Generate(seed int64) (Snapshot, error) {
	rng := rand.New(rand.NewSource(seed))
	domain, err := hexgrid.Disk(hexgrid.Coord{}, radius)
	if err != nil {
		return Snapshot{}, err
	}
	floor := map[hexgrid.Coord]floorInfo{}
	centers := []hexgrid.Coord{{-4, 0}, {0, -4}, {4, -4}, {4, 0}, {0, 4}, {-4, 4}, {0, 0}}
	for i := range centers {
		if i != len(centers)-1 {
			shift := hexgrid.Directions[rng.Intn(6)]
			centers[i].Q += shift.Q
			centers[i].R += shift.R
		}
		room, err := hexgrid.Disk(centers[i], 1+rng.Intn(2))
		if err != nil {
			return Snapshot{}, err
		}
		for _, c := range room {
			if hexgrid.InDisk(c, radius-1) {
				floor[c] = floorInfo{i + 1}
			}
		}
	}
	// A spanning tree joins every room. Each corridor follows a shortest HEX
	// path; ties vary with the seed but stay reproducible.
	for i := 1; i < len(centers); i++ {
		from, to := centers[i], centers[0]
		if i < len(centers)-1 {
			to = centers[i-1]
		}
		for from != to {
			current, _ := hexgrid.Distance(from, to)
			choices := make([]hexgrid.Coord, 0, 2)
			for direction := 0; direction < 6; direction++ {
				next, _ := hexgrid.Neighbor(from, direction)
				distance, _ := hexgrid.Distance(next, to)
				if distance < current {
					choices = append(choices, next)
				}
			}
			if len(choices) == 0 {
				return Snapshot{}, fmt.Errorf("corridor cannot reach room %d", i)
			}
			from = choices[rng.Intn(len(choices))]
			if _, exists := floor[from]; !exists {
				floor[from] = floorInfo{}
			}
		}
	}
	// A few stone pillars add impassable patches, without cutting corridors.
	for _, c := range []hexgrid.Coord{{-1, -1}, {2, 2}, {-2, 3}} {
		if _, ok := floor[c]; !ok {
			continue
		}
		delete(floor, c)
		if !connected(floor) {
			floor[c] = floorInfo{}
		}
	}
	sort.Slice(domain, func(i, j int) bool {
		if domain[i].R == domain[j].R {
			return domain[i].Q < domain[j].Q
		}
		return domain[i].R < domain[j].R
	})
	snapshot := Snapshot{SchemaVersion: 2, Geometry: models.HexV1, InstanceID: fmt.Sprintf("hex-preview:%d", seed), MapRevision: 1, StateRevision: 1, Seed: seed,
		Cells: make([]Cell, 0, len(domain)), Edges: make([]Edge, 0)}
	for i, c := range domain {
		terrain := "rock"
		info := floor[c]
		if _, ok := floor[c]; ok {
			terrain = "floor"
		}
		snapshot.Cells = append(snapshot.Cells, Cell{CellID: i + 1, Position: c, Terrain: terrain, Room: info.room})
		for direction := 0; direction < 3; direction++ {
			next, _ := hexgrid.Neighbor(c, direction)
			if !hexgrid.InDisk(next, radius) {
				continue
			}
			_, a := floor[c]
			_, b := floor[next]
			if a != b {
				snapshot.Edges = append(snapshot.Edges, Edge{Position: c, Direction: direction, Kind: "wall"})
			}
		}
	}
	snapshot.Actors = []Actor{{ID: "hero-preview", Kind: "player", Position: centers[0]}}
	for i, center := range centers[1:4] {
		snapshot.Actors = append(snapshot.Actors, Actor{ID: fmt.Sprintf("monster-%d", i+1), Kind: "monster", Position: center})
	}
	return snapshot, nil
}

func connected(floor map[hexgrid.Coord]floorInfo) bool {
	if len(floor) == 0 {
		return false
	}
	var first hexgrid.Coord
	for c := range floor {
		first = c
		break
	}
	seen := map[hexgrid.Coord]bool{first: true}
	queue := []hexgrid.Coord{first}
	for len(queue) > 0 {
		c := queue[0]
		queue = queue[1:]
		for d := 0; d < 6; d++ {
			n, err := hexgrid.Neighbor(c, d)
			if err != nil || seen[n] {
				continue
			}
			if _, ok := floor[n]; ok {
				seen[n] = true
				queue = append(queue, n)
			}
		}
	}
	return len(seen) == len(floor)
}
