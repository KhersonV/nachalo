package hexgrid

import "math"

// Visible uses the center-to-center segment and conservatively blocks any
// touched wall edge or intervening rock hexagon, including vertex ties. The
// caller supplies the authoritative walkable cells and edge walls.
func Visible(from, to Coord, walkable map[Coord]bool, walls map[EdgeKey]bool) bool {
	if !Valid(from) || !Valid(to) || from == to || !walkable[from] || !walkable[to] {
		return false
	}
	a, _ := ToWorld(from, 1)
	b, _ := ToWorld(to, 1)
	lineA, lineB := point{a.X, a.Z}, point{b.X, b.Z}
	for edge := range walls {
		wa, _ := ToWorld(edge.A, 1)
		wb, _ := ToWorld(edge.B, 1)
		p, q := point{wa.X, wa.Z}, point{wb.X, wb.Z}
		middle := point{(p.x + q.x) / 2, (p.y + q.y) / 2}
		dx, dy := q.x-p.x, q.y-p.y
		length := math.Hypot(dx, dy)
		if length == 0 {
			continue
		}
		wallA := point{middle.x - dy/length/2, middle.y + dx/length/2}
		wallB := point{middle.x + dy/length/2, middle.y - dx/length/2}
		if segmentsTouch(lineA, lineB, wallA, wallB) {
			return false
		}
	}
	for cell, floor := range walkable {
		if floor || cell == from || cell == to {
			continue
		}
		center, _ := ToWorld(cell, 1)
		corners := [6]point{}
		for i := 0; i < 6; i++ {
			angle := (-90 + float64(i)*60) * math.Pi / 180
			corners[i] = point{center.X + math.Cos(angle), center.Z + math.Sin(angle)}
		}
		for i := 0; i < 6; i++ {
			if segmentsTouch(lineA, lineB, corners[i], corners[(i+1)%6]) {
				return false
			}
		}
	}
	return true
}

type point struct{ x, y float64 }

func cross(a, b, c point) float64 { return (b.x-a.x)*(c.y-a.y) - (b.y-a.y)*(c.x-a.x) }
func onSegment(a, b, p point) bool {
	const epsilon = 1e-8
	return math.Abs(cross(a, b, p)) <= epsilon && p.x >= math.Min(a.x, b.x)-epsilon && p.x <= math.Max(a.x, b.x)+epsilon && p.y >= math.Min(a.y, b.y)-epsilon && p.y <= math.Max(a.y, b.y)+epsilon
}
func segmentsTouch(a, b, c, d point) bool {
	const epsilon = 1e-8
	abc, abd := cross(a, b, c), cross(a, b, d)
	cda, cdb := cross(c, d, a), cross(c, d, b)
	if math.Abs(abc) <= epsilon && onSegment(a, b, c) {
		return true
	}
	if math.Abs(abd) <= epsilon && onSegment(a, b, d) {
		return true
	}
	if math.Abs(cda) <= epsilon && onSegment(c, d, a) {
		return true
	}
	if math.Abs(cdb) <= epsilon && onSegment(c, d, b) {
		return true
	}
	return (abc > epsilon && abd < -epsilon || abc < -epsilon && abd > epsilon) && (cda > epsilon && cdb < -epsilon || cda < -epsilon && cdb > epsilon)
}
