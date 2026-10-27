package hexmap

import (
	"fmt"
	"math"
	"math/rand"

	"gameservice/internal/hexgrid"
)

// placementGraph uses the same six neighbors and canonical wall edges as movement.
// Objects are excluded conservatively, even though resources can be walked over.
type placementGraph struct {
	floor map[hexgrid.Coord]bool
	walls map[hexgrid.EdgeKey]bool
}

func newPlacementGraph(s Snapshot) placementGraph {
	g := placementGraph{map[hexgrid.Coord]bool{}, map[hexgrid.EdgeKey]bool{}}
	for _, c := range s.Cells {
		if c.Terrain == "floor" && c.Resource == nil && c.Barrel == nil && c.StructureType == "" && c.EnvironmentType == "" {
			g.floor[c.Position] = true
		}
	}
	for _, a := range s.Actors {
		if a.Kind != "player" {
			delete(g.floor, a.Position)
		}
	}
	for _, e := range s.Edges {
		n, err := hexgrid.Neighbor(e.Position, e.Direction)
		if err == nil {
			if key, err := hexgrid.CanonicalEdge(e.Position, n); err == nil {
				g.walls[key] = true
			}
		}
	}
	return g
}

func (g placementGraph) neighbors(c hexgrid.Coord) []hexgrid.Coord {
	out := []hexgrid.Coord{}
	for d := 0; d < 6; d++ {
		n, _ := hexgrid.Neighbor(c, d)
		key, _ := hexgrid.CanonicalEdge(c, n)
		if g.floor[n] && !g.walls[key] {
			out = append(out, n)
		}
	}
	return out
}

func (g placementGraph) distances(from hexgrid.Coord) (map[hexgrid.Coord]int, map[hexgrid.Coord]hexgrid.Coord) {
	dist, prev := map[hexgrid.Coord]int{}, map[hexgrid.Coord]hexgrid.Coord{}
	if !g.floor[from] {
		return dist, prev
	}
	dist[from] = 0
	queue := []hexgrid.Coord{from}
	for i := 0; i < len(queue); i++ {
		for _, n := range g.neighbors(queue[i]) {
			if _, seen := dist[n]; !seen {
				dist[n] = dist[queue[i]] + 1
				prev[n] = queue[i]
				queue = append(queue, n)
			}
		}
	}
	return dist, prev
}

func startSeparation(s Snapshot, cfg Config) int {
	distance := int(float64(s.Radius) / math.Sqrt(float64(len(cfg.Teams))))
	if distance < 3 {
		distance = 3
	}
	if distance <= cfg.MaxAttackRange {
		distance = cfg.MaxAttackRange + 1
	}
	return distance
}

func placeStarts(s *Snapshot, cfg Config, rng *rand.Rand) error {
	g := newPlacementGraph(*s)
	anchors := []hexgrid.Coord{}
	for c := range g.floor {
		if len(g.neighbors(c)) >= 2 {
			anchors = append(anchors, c)
		}
	}
	sortCoords(anchors)
	separation := startSeparation(*s, cfg)
	// Finite restarts allow earlier teams to release a zone needed by a larger team.
	for attempt := 0; attempt < 24; attempt++ {
		starts := []Start{}
		used := map[hexgrid.Coord]bool{}
		for _, ti := range rng.Perm(len(cfg.Teams)) {
			team := cfg.Teams[ti]
			rng.Shuffle(len(anchors), func(i, j int) { anchors[i], anchors[j] = anchors[j], anchors[i] })
			placed := false
			for ai, anchor := range anchors {
				if ai >= 192 {
					break
				}
				if used[anchor] {
					continue
				}
				zoneRadius := 1
				for 1+3*zoneRadius*(zoneRadius+1) < 2*len(team.PlayerIDs) {
					zoneRadius++
				}
				queue := []hexgrid.Coord{anchor}
				depth := map[hexgrid.Coord]int{anchor: 0}
				zone := []hexgrid.Coord{}
				for i := 0; i < len(queue) && len(zone) < len(team.PlayerIDs); i++ {
					c := queue[i]
					valid := !used[c] && len(g.neighbors(c)) >= 2
					for _, st := range starts {
						d, _ := hexgrid.Distance(c, st.Position)
						if d < separation {
							valid = false
							break
						}
					}
					if !valid {
						continue
					}
					zone = append(zone, c)
					if depth[c] >= zoneRadius {
						continue
					}
					neighbors := g.neighbors(c)
					rng.Shuffle(len(neighbors), func(i, j int) { neighbors[i], neighbors[j] = neighbors[j], neighbors[i] })
					for _, n := range neighbors {
						if _, seen := depth[n]; !seen {
							depth[n] = depth[c] + 1
							queue = append(queue, n)
						}
					}
				}
				if len(zone) != len(team.PlayerIDs) {
					continue
				}
				for i, id := range team.PlayerIDs {
					starts = append(starts, Start{UserID: id, TeamID: team.ID, Position: zone[i]})
					used[zone[i]] = true
				}
				placed = true
				break
			}
			if !placed {
				break
			}
		}
		total := 0
		for _, t := range cfg.Teams {
			total += len(t.PlayerIDs)
		}
		if len(starts) != total {
			continue
		}
		// Every participant must have an immediately free step, including in large parties.
		exits := true
		for _, st := range starts {
			free := false
			for _, n := range g.neighbors(st.Position) {
				if !used[n] {
					free = true
					break
				}
			}
			if !free {
				exits = false
				break
			}
		}
		if !exits {
			continue
		}
		// Score portals by their shortest wall-aware route from the nearest team member.
		nearest := map[hexgrid.Coord]int{}
		for c := range g.floor {
			nearest[c] = int(^uint(0) >> 1)
		}
		for _, st := range starts {
			dist, _ := g.distances(st.Position)
			for c := range nearest {
				d, ok := dist[c]
				if !ok {
					nearest[c] = -1
				} else if nearest[c] >= 0 && d < nearest[c] {
					nearest[c] = d
				}
			}
		}
		best := 0
		candidates := []hexgrid.Coord{}
		for _, c := range s.Cells {
			if used[c.Position] || !g.floor[c.Position] {
				continue
			}
			minHex := s.Radius / 2
			if minHex < 3 {
				minHex = 3
			}
			valid := nearest[c.Position] >= minHex
			for _, st := range starts {
				d, _ := hexgrid.Distance(c.Position, st.Position)
				if d < minHex {
					valid = false
				}
			}
			if valid {
				candidates = append(candidates, c.Position)
				if nearest[c.Position] > best {
					best = nearest[c.Position]
				}
			}
		}
		far := []hexgrid.Coord{}
		for _, c := range candidates {
			if nearest[c] >= best*3/4 {
				far = append(far, c)
			}
		}
		if len(far) == 0 {
			continue
		}
		portal := far[rng.Intn(len(far))]
		s.Starts, s.Portal = starts, &portal
		for _, st := range starts {
			s.Actors = append(s.Actors, Actor{ID: fmt.Sprintf("player-%d", st.UserID), Kind: "player", UserID: st.UserID, TeamID: st.TeamID, Position: st.Position})
		}
		for i := range s.Cells {
			s.Cells[i].IsPortal = s.Cells[i].Position == portal
		}
		return nil
	}
	return fmt.Errorf("HEX has no reachable, separated start zones and exit")
}

// PlacementReservations protects routes and a small start area before any object
// pass, including profession nodes added later by the repository.
func PlacementReservations(s Snapshot, cfg Config) map[hexgrid.Coord]bool {
	reserved := map[hexgrid.Coord]bool{}
	if s.Portal == nil {
		return reserved
	}
	g := newPlacementGraph(s)
	reserved[*s.Portal] = true
	// Reserve a route that does not require another hero to move first.
	for _, st := range s.Starts {
		delete(g.floor, st.Position)
	}
	for _, st := range s.Starts {
		reserved[st.Position] = true
		radius := 1
		if len(cfg.Teams) == 1 {
			radius = 2
		}
		for _, cell := range s.Cells {
			d, _ := hexgrid.Distance(st.Position, cell.Position)
			if d <= radius {
				reserved[cell.Position] = true
			}
		}
		g.floor[st.Position] = true
		dist, prev := g.distances(st.Position)
		delete(g.floor, st.Position)
		if _, ok := dist[*s.Portal]; ok {
			for c := *s.Portal; c != st.Position; c = prev[c] {
				reserved[c] = true
			}
		}
	}
	return reserved
}

// ValidatePlacement runs only for new maps, after the final object pass.
func ValidatePlacement(s Snapshot, cfg Config) error {
	if s.Portal == nil {
		return fmt.Errorf("HEX exit portal is missing")
	}
	portalCount := 0
	for _, cell := range s.Cells {
		if cell.IsPortal {
			portalCount++
			if cell.Position != *s.Portal {
				return fmt.Errorf("HEX portal coordinates mismatch")
			}
		}
	}
	if portalCount != 1 {
		return fmt.Errorf("HEX requires exactly one exit portal")
	}
	g := newPlacementGraph(s)
	if !g.floor[*s.Portal] {
		return fmt.Errorf("HEX exit portal overlaps an object")
	}
	dist, _ := g.distances(*s.Portal)
	occupied := map[hexgrid.Coord]bool{*s.Portal: true}
	expected := map[int]int{}
	for _, team := range cfg.Teams {
		for _, id := range team.PlayerIDs {
			expected[id] = team.ID
		}
	}
	if len(s.Starts) != len(expected) {
		return fmt.Errorf("HEX participant count mismatch")
	}
	for _, st := range s.Starts {
		if expected[st.UserID] != st.TeamID || st.TeamID == 0 {
			return fmt.Errorf("HEX participant mismatch")
		}
		delete(expected, st.UserID)
		if !g.floor[st.Position] || occupied[st.Position] {
			return fmt.Errorf("HEX start overlaps an object or participant")
		}
		if _, ok := dist[st.Position]; !ok {
			return fmt.Errorf("HEX start cannot reach exit")
		}
		d, _ := hexgrid.Distance(st.Position, *s.Portal)
		if d < 3 {
			return fmt.Errorf("HEX exit too close to start")
		}
		occupied[st.Position] = true
	}
	for _, st := range s.Starts {
		delete(g.floor, st.Position)
	}
	for i, st := range s.Starts {
		g.floor[st.Position] = true
		reachable, _ := g.distances(st.Position)
		delete(g.floor, st.Position)
		if _, ok := reachable[*s.Portal]; !ok {
			return fmt.Errorf("HEX start route is blocked by participants")
		}
		exit := false
		for _, n := range g.neighbors(st.Position) {
			if !occupied[n] {
				exit = true
			}
		}
		if !exit {
			return fmt.Errorf("HEX start has no free step")
		}
		for _, other := range s.Starts[i+1:] {
			if st.TeamID != other.TeamID {
				d, _ := hexgrid.Distance(st.Position, other.Position)
				if d < startSeparation(s, cfg) {
					return fmt.Errorf("HEX enemy starts too close")
				}
			}
		}
		if len(cfg.Teams) == 1 {
			for _, actor := range s.Actors {
				if actor.Kind == "monster" {
					d, _ := hexgrid.Distance(st.Position, actor.Position)
					if d <= 2 {
						return fmt.Errorf("HEX monster in start safety area")
					}
				}
			}
		}
	}
	return nil
}
