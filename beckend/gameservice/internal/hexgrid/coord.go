// Package hexgrid implements isolated pointy-top axial hex geometry.
// No game rules, persistence, or existing square-map code depend on it yet.
package hexgrid

import (
	"errors"
	"math"
)

type Coord struct {
	Q int `json:"q"`
	R int `json:"r"`
}
type World struct {
	X float64 `json:"x"`
	Z float64 `json:"z"`
}
type Cube struct{ X, Y, Z int }

var Directions = [6]Coord{{1, 0}, {1, -1}, {0, -1}, {-1, 0}, {-1, 1}, {0, 1}}

const MaxCoordinate = 1000000

var ErrInvalidCoordinate = errors.New("hex coordinate outside supported domain")

func Valid(c Coord) bool {
	return c.Q >= -MaxCoordinate && c.Q <= MaxCoordinate && c.R >= -MaxCoordinate && c.R <= MaxCoordinate && c.Q+c.R >= -2*MaxCoordinate && c.Q+c.R <= 2*MaxCoordinate
}
func Neighbor(c Coord, direction int) (Coord, error) {
	if !Valid(c) || direction < 0 || direction >= 6 {
		return Coord{}, ErrInvalidCoordinate
	}
	d := Directions[direction]
	n := Coord{c.Q + d.Q, c.R + d.R}
	if !Valid(n) {
		return Coord{}, ErrInvalidCoordinate
	}
	return n, nil
}
func Opposite(direction int) (int, error) {
	if direction < 0 || direction >= 6 {
		return 0, ErrInvalidCoordinate
	}
	return (direction + 3) % 6, nil
}
func ToCube(c Coord) (Cube, error) {
	if !Valid(c) {
		return Cube{}, ErrInvalidCoordinate
	}
	return Cube{c.Q, -c.Q - c.R, c.R}, nil
}
func FromCube(c Cube) (Coord, error) {
	if c.X+c.Y+c.Z != 0 {
		return Coord{}, ErrInvalidCoordinate
	}
	h := Coord{c.X, c.Z}
	if !Valid(h) {
		return Coord{}, ErrInvalidCoordinate
	}
	return h, nil
}
func abs(n int) int {
	if n < 0 {
		return -n
	}
	return n
}
func Distance(a, b Coord) (int, error) {
	if !Valid(a) || !Valid(b) {
		return 0, ErrInvalidCoordinate
	}
	dq, dr := a.Q-b.Q, a.R-b.R
	return (abs(dq) + abs(dr) + abs(dq+dr)) / 2, nil
}
func ToWorld(c Coord, radius float64) (World, error) {
	if !Valid(c) || !validRadius(radius) {
		return World{}, ErrInvalidCoordinate
	}
	return World{math.Sqrt(3) * radius * (float64(c.Q) + float64(c.R)/2), 1.5 * radius * float64(c.R)}, nil
}
func validRadius(a float64) bool { return a > 0 && !math.IsNaN(a) && !math.IsInf(a, 0) }
func rounded(v float64) float64  { return math.Floor(v + 0.5) }
func RoundAxial(q, r float64) (Coord, error) {
	if math.IsNaN(q) || math.IsNaN(r) || math.IsInf(q, 0) || math.IsInf(r, 0) || math.Abs(q) > MaxCoordinate+2 || math.Abs(r) > MaxCoordinate+2 {
		return Coord{}, ErrInvalidCoordinate
	}
	x, y, z := q, -q-r, r
	rx, ry, rz := rounded(x), rounded(y), rounded(z)
	dx, dy, dz := math.Abs(rx-x), math.Abs(ry-y), math.Abs(rz-z)
	if dx >= dy && dx >= dz {
		rx = -ry - rz
	} else if dy >= dz {
		ry = -rx - rz
	} else {
		rz = -rx - ry
	}
	h := Coord{int(rx), int(rz)}
	if !Valid(h) {
		return Coord{}, ErrInvalidCoordinate
	}
	return h, nil
}
func FromWorld(p World, radius float64) (Coord, error) {
	if !validRadius(radius) || math.IsNaN(p.X) || math.IsNaN(p.Z) || math.IsInf(p.X, 0) || math.IsInf(p.Z, 0) {
		return Coord{}, ErrInvalidCoordinate
	}
	q := (math.Sqrt(3)*p.X/3 - p.Z/3) / radius
	r := 2 * p.Z / (3 * radius)
	return RoundAxial(q, r)
}
func InDisk(c Coord, radius int) bool {
	if !Valid(c) || radius < 0 {
		return false
	}
	d, _ := Distance(Coord{}, c)
	return d <= radius
}
func Ring(center Coord, radius int) ([]Coord, error) {
	if !Valid(center) || radius < 0 || radius > MaxCoordinate {
		return nil, ErrInvalidCoordinate
	}
	if radius == 0 {
		return []Coord{center}, nil
	}
	start := Directions[4]
	c := Coord{center.Q + start.Q*radius, center.R + start.R*radius}
	out := make([]Coord, 0, 6*radius)
	for d := 0; d < 6; d++ {
		for i := 0; i < radius; i++ {
			if !Valid(c) {
				return nil, ErrInvalidCoordinate
			}
			out = append(out, c)
			c.Q += Directions[d].Q
			c.R += Directions[d].R
		}
	}
	return out, nil
}
func Disk(center Coord, radius int) ([]Coord, error) {
	if radius < 0 || radius > 2048 {
		return nil, ErrInvalidCoordinate
	}
	out := make([]Coord, 0, 1+3*radius*(radius+1))
	for k := 0; k <= radius; k++ {
		ring, err := Ring(center, k)
		if err != nil {
			return nil, err
		}
		out = append(out, ring...)
	}
	return out, nil
}

// EdgeKey identifies the shared edge irrespective of endpoint order.
type EdgeKey struct{ A, B Coord }

func CanonicalEdge(a, b Coord) (EdgeKey, error) {
	d, err := Distance(a, b)
	if err != nil || d != 1 {
		return EdgeKey{}, ErrInvalidCoordinate
	}
	if b.Q < a.Q || (b.Q == a.Q && b.R < a.R) {
		a, b = b, a
	}
	return EdgeKey{a, b}, nil
}
