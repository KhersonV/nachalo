package hexmap

import (
	"fmt"
	"gameservice/game"
	"gameservice/internal/hexgrid"
	"gameservice/models"
	"math"
	"math/rand"
	"sort"
	"strings"
)

// Teams are formed by the existing match plan. Generation only places them.
type Team struct {
	ID        int
	PlayerIDs []int
}
type Config struct {
	Teams                                 []Team
	MonsterProb, BarrelProb, ResourceProb float64
}
type Catalogs struct {
	Monsters  []game.MonsterData
	Resources []game.ResourceData
}
type Start struct {
	UserID   int           `json:"userId"`
	TeamID   int           `json:"teamId"`
	Position hexgrid.Coord `json:"position"`
}
type floorInfo struct{ room int }

func validateConfig(cfg Config) (int, error) {
	if len(cfg.Teams) < 1 || len(cfg.Teams) > 64 {
		return 0, fmt.Errorf("HEX requires 1..64 teams")
	}
	teams, players := map[int]bool{}, map[int]bool{}
	total := 0
	for _, t := range cfg.Teams {
		if t.ID <= 0 || teams[t.ID] || len(t.PlayerIDs) == 0 {
			return 0, fmt.Errorf("invalid HEX team %d", t.ID)
		}
		teams[t.ID] = true
		for _, id := range t.PlayerIDs {
			if id <= 0 || players[id] {
				return 0, fmt.Errorf("invalid or duplicate HEX player %d", id)
			}
			players[id] = true
			total++
		}
	}
	if total > 64 {
		return 0, fmt.Errorf("HEX supports at most 64 players")
	}
	for _, p := range []float64{cfg.MonsterProb, cfg.BarrelProb, cfg.ResourceProb} {
		if math.IsNaN(p) || math.IsInf(p, 0) || p < 0 || p > 1 {
			return 0, fmt.Errorf("invalid HEX placement probability")
		}
	}
	if cfg.BarrelProb+cfg.ResourceProb > 1 {
		return 0, fmt.Errorf("barrel and resource chances exceed one")
	}
	return total, nil
}
func sortCoords(a []hexgrid.Coord) {
	sort.Slice(a, func(i, j int) bool {
		if a[i].R == a[j].R {
			return a[i].Q < a[j].Q
		}
		return a[i].R < a[j].R
	})
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
	for i := 0; i < len(queue); i++ {
		for d := 0; d < 6; d++ {
			n, _ := hexgrid.Neighbor(queue[i], d)
			if _, ok := floor[n]; ok && !seen[n] {
				seen[n] = true
				queue = append(queue, n)
			}
		}
	}
	return len(seen) == len(floor)
}

// GenerateDungeon is pure: same config, sorted catalogs and seed give the same plan.
func GenerateDungeon(cfg Config, catalogs Catalogs, seed int64) (Snapshot, error) {
	total, err := validateConfig(cfg)
	if err != nil {
		return Snapshot{}, err
	}
	monsters := append([]game.MonsterData(nil), catalogs.Monsters...)
	resources := append([]game.ResourceData(nil), catalogs.Resources...)
	sort.Slice(monsters, func(i, j int) bool { return monsters[i].ID < monsters[j].ID })
	sort.Slice(resources, func(i, j int) bool { return resources[i].ID < resources[j].ID })
	var barrel *game.ResourceData
	normal := make([]game.ResourceData, 0, len(resources))
	for _, r := range resources {
		if strings.EqualFold(r.Type, "barrel") || strings.EqualFold(r.Type, "barbel") {
			if barrel == nil {
				v := r
				barrel = &v
			}
		} else {
			normal = append(normal, r)
		}
	}
	if barrel == nil {
		for i, r := range normal {
			if r.ID == 6 {
				v := r
				barrel = &v
				normal = append(normal[:i], normal[i+1:]...)
				break
			}
		}
	}
	minFloor := 24 + 12*total + 6*len(cfg.Teams)
	radius := 8
	for (1+3*radius*(radius+1))*65/100 < 48+24*total+12*len(cfg.Teams) {
		radius++
	}
	for ; radius <= 32; radius++ {
		for attempt := 0; attempt < 8; attempt++ {
			rng := rand.New(rand.NewSource(seed + int64(radius)*1000003 + int64(attempt)*7919))
			snap, centers, floor, err := geometry(radius, len(cfg.Teams), total, seed, rng)
			if err != nil || len(floor) < minFloor {
				continue
			}
			if placeStarts(&snap, cfg, centers, floor) != nil {
				continue
			}
			placeObjects(&snap, cfg, monsters, normal, barrel, floor, rng)
			nm, nb, nr := 0, 0, 0
			for _, a := range snap.Actors {
				if a.Kind == "monster" {
					nm++
				}
			}
			for _, c := range snap.Cells {
				if c.Barrel != nil {
					nb++
				}
				if c.Resource != nil {
					nr++
				}
			}
			if cfg.MonsterProb > 0 && len(monsters) > 0 && nm == 0 || cfg.BarrelProb > 0 && barrel != nil && nb == 0 || cfg.ResourceProb > 0 && len(normal) > 0 && nr == 0 {
				continue
			}
			return snap, nil
		}
	}
	return Snapshot{}, fmt.Errorf("HEX cannot fit %d players in %d teams with portal and objects", total, len(cfg.Teams))
}

func geometry(radius, teams, total int, seed int64, rng *rand.Rand) (Snapshot, []hexgrid.Coord, map[hexgrid.Coord]floorInfo, error) {
	domain, err := hexgrid.Disk(hexgrid.Coord{}, radius)
	if err != nil {
		return Snapshot{}, nil, nil, err
	}
	rooms := teams + 2
	if rooms < 6 {
		rooms = 6
	}
	if rooms < total/2+3 {
		rooms = total/2 + 3
	}
	ring, err := hexgrid.Ring(hexgrid.Coord{}, radius-3)
	if err != nil {
		return Snapshot{}, nil, nil, err
	}
	centers := []hexgrid.Coord{{}}
	for i := 0; i < rooms; i++ {
		idx := (i*len(ring)/rooms + rng.Intn(3) - 1 + len(ring)) % len(ring)
		centers = append(centers, ring[idx])
	}
	floor := map[hexgrid.Coord]floorInfo{}
	protected := map[hexgrid.Coord]bool{}
	for i, c := range centers {
		size := 2 + rng.Intn(2)
		if i == 0 {
			size = 3
		}
		room, _ := hexgrid.Disk(c, size)
		for _, p := range room {
			if hexgrid.InDisk(p, radius-1) {
				floor[p] = floorInfo{room: i + 1}
			}
		}
		protected[c] = true
	}
	for i := 1; i < len(centers); i++ {
		from, to := centers[i], centers[0]
		for from != to {
			old, _ := hexgrid.Distance(from, to)
			choices := []hexgrid.Coord{}
			for d := 0; d < 6; d++ {
				n, _ := hexgrid.Neighbor(from, d)
				dist, _ := hexgrid.Distance(n, to)
				if dist < old {
					choices = append(choices, n)
				}
			}
			if len(choices) == 0 {
				return Snapshot{}, nil, nil, fmt.Errorf("HEX corridor disconnected")
			}
			from = choices[rng.Intn(len(choices))]
			protected[from] = true
			if _, ok := floor[from]; !ok {
				floor[from] = floorInfo{}
			}
		}
	}
	candidates := []hexgrid.Coord{}
	for c := range floor {
		if !protected[c] {
			candidates = append(candidates, c)
		}
	}
	sortCoords(candidates)
	rng.Shuffle(len(candidates), func(i, j int) { candidates[i], candidates[j] = candidates[j], candidates[i] })
	removed := 0
	for _, c := range candidates {
		if removed >= len(floor)/18 {
			break
		}
		old := floor[c]
		delete(floor, c)
		if connected(floor) {
			removed++
		} else {
			floor[c] = old
		}
	}
	sortCoords(domain)
	snap := Snapshot{SchemaVersion: 2, Geometry: models.HexV1, InstanceID: fmt.Sprintf("hex-preview:%d", seed), MapRevision: 1, StateRevision: 1, Seed: seed, Radius: radius, Cells: make([]Cell, 0, len(domain)), Edges: []Edge{}, Actors: []Actor{}}
	for i, c := range domain {
		terrain := "rock"
		info, ok := floor[c]
		if ok {
			terrain = "floor"
		}
		snap.Cells = append(snap.Cells, Cell{CellID: i + 1, Position: c, Terrain: terrain, Room: info.room})
		for d := 0; d < 3; d++ {
			n, _ := hexgrid.Neighbor(c, d)
			if !hexgrid.InDisk(n, radius) {
				continue
			}
			_, a := floor[c]
			_, b := floor[n]
			if a != b {
				snap.Edges = append(snap.Edges, Edge{Position: c, Direction: d, Kind: "wall"})
			}
		}
	}
	return snap, centers, floor, nil
}
func placeStarts(s *Snapshot, cfg Config, centers []hexgrid.Coord, floor map[hexgrid.Coord]floorInfo) error {
	used := map[hexgrid.Coord]bool{}
	s.Starts = []Start{}
	for i, t := range cfg.Teams {
		anchor := centers[i+1]
		candidates := []hexgrid.Coord{}
		for c := range floor {
			d, _ := hexgrid.Distance(anchor, c)
			if d <= 4 {
				candidates = append(candidates, c)
			}
		}
		sort.Slice(candidates, func(a, b int) bool {
			da, _ := hexgrid.Distance(anchor, candidates[a])
			db, _ := hexgrid.Distance(anchor, candidates[b])
			if da == db {
				if candidates[a].R == candidates[b].R {
					return candidates[a].Q < candidates[b].Q
				}
				return candidates[a].R < candidates[b].R
			}
			return da < db
		})
		count := 0
		for _, c := range candidates {
			if used[c] {
				continue
			}
			used[c] = true
			id := t.PlayerIDs[count]
			s.Starts = append(s.Starts, Start{UserID: id, TeamID: t.ID, Position: c})
			s.Actors = append(s.Actors, Actor{ID: fmt.Sprintf("player-%d", id), Kind: "player", Position: c})
			count++
			if count == len(t.PlayerIDs) {
				break
			}
		}
		if count != len(t.PlayerIDs) {
			return fmt.Errorf("team %d has no adjacent floor area", t.ID)
		}
	}
	candidates := []hexgrid.Coord{}
	for c := range floor {
		d, _ := hexgrid.Distance(hexgrid.Coord{}, c)
		if d <= 3 && !used[c] && c != (hexgrid.Coord{}) {
			candidates = append(candidates, c)
		}
	}
	if len(candidates) == 0 {
		return fmt.Errorf("portal has no floor cell")
	}
	score := func(c hexgrid.Coord) int {
		min := 100000
		for _, st := range s.Starts {
			d, _ := hexgrid.Distance(c, st.Position)
			if d < min {
				min = d
			}
		}
		return min
	}
	sort.Slice(candidates, func(i, j int) bool {
		a, b := score(candidates[i]), score(candidates[j])
		if a == b {
			if candidates[i].R == candidates[j].R {
				return candidates[i].Q < candidates[j].Q
			}
			return candidates[i].R < candidates[j].R
		}
		return a > b
	})
	portal := candidates[0]
	s.Portal = &portal
	for i := range s.Cells {
		if s.Cells[i].Position == portal {
			s.Cells[i].IsPortal = true
			break
		}
	}
	return nil
}
func floorPath(from, to hexgrid.Coord, floor map[hexgrid.Coord]floorInfo) []hexgrid.Coord {
	seen := map[hexgrid.Coord]bool{from: true}
	prev := map[hexgrid.Coord]hexgrid.Coord{}
	q := []hexgrid.Coord{from}
	for i := 0; i < len(q); i++ {
		c := q[i]
		if c == to {
			break
		}
		for d := 0; d < 6; d++ {
			n, _ := hexgrid.Neighbor(c, d)
			if _, ok := floor[n]; ok && !seen[n] {
				seen[n] = true
				prev[n] = c
				q = append(q, n)
			}
		}
	}
	if !seen[to] {
		return nil
	}
	p := []hexgrid.Coord{to}
	for p[len(p)-1] != from {
		p = append(p, prev[p[len(p)-1]])
	}
	return p
}
func placeObjects(s *Snapshot, cfg Config, monsters []game.MonsterData, resources []game.ResourceData, barrel *game.ResourceData, floor map[hexgrid.Coord]floorInfo, rng *rand.Rand) {
	reserved := map[hexgrid.Coord]bool{}
	for _, st := range s.Starts {
		for _, c := range floorPath(st.Position, *s.Portal, floor) {
			reserved[c] = true
		}
	}
	reserved[*s.Portal] = true
	for i := range s.Cells {
		c := &s.Cells[i]
		if c.Terrain != "floor" || reserved[c.Position] {
			continue
		}
		if rng.Float64() < cfg.MonsterProb && len(monsters) > 0 {
			m := monsters[rng.Intn(len(monsters))]
			id := len(s.Actors) + 1
			s.Actors = append(s.Actors, Actor{ID: fmt.Sprintf("monster-%d", id), Kind: "monster", Position: c.Position, Health: m.Health, MaxHealth: m.MaxHealth, MonsterRefID: m.ID, Name: m.Name, Type: m.Type, Image: m.Image, Attack: m.Attack, Defense: m.Defense, Speed: m.Speed, Maneuverability: m.Maneuverability, Vision: m.Vision})
			continue
		}
		v := rng.Float64()
		if v < cfg.BarrelProb && barrel != nil {
			b := *barrel
			c.Barrel = &b
		} else if v < cfg.BarrelProb+cfg.ResourceProb && len(resources) > 0 {
			r := resources[rng.Intn(len(resources))]
			c.Resource = &r
		}
	}
}
