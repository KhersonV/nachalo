package hexgrid

import (
	"encoding/json"
	"math"
	"os"
	"testing"
)

type fixture struct {
	Radius     float64 `json:"radius"`
	Directions [][]int `json:"directions"`
	Distances  []struct {
		A    []int `json:"a"`
		B    []int `json:"b"`
		Want int   `json:"want"`
	} `json:"distances"`
	Round []struct {
		Input []float64 `json:"input"`
		Want  []int     `json:"want"`
	} `json:"round"`
	RingSizes []int `json:"ringSizes"`
	DiskSizes []int `json:"diskSizes"`
}

func load(t *testing.T) fixture {
	t.Helper()
	b, e := os.ReadFile("../../../../docs/hex-migration/fixtures/geometry-v1.json")
	if e != nil {
		t.Fatal(e)
	}
	var f fixture
	if e = json.Unmarshal(b, &f); e != nil {
		t.Fatal(e)
	}
	return f
}
func TestSharedFixtures(t *testing.T) {
	f := load(t)
	for i, d := range f.Directions {
		if Directions[i] != (Coord{d[0], d[1]}) {
			t.Fatalf("dir %d", i)
		}
		n, e := Neighbor(Coord{}, i)
		if e != nil || n != Directions[i] {
			t.Fatal(i, e)
		}
		back, e := Neighbor(n, (i+3)%6)
		if e != nil || back != (Coord{}) {
			t.Fatal("opposite", i)
		}
	}
	for _, v := range f.Distances {
		d, e := Distance(Coord{v.A[0], v.A[1]}, Coord{v.B[0], v.B[1]})
		if e != nil || d != v.Want {
			t.Fatalf("distance %+v %d %v", v, d, e)
		}
	}
	for _, v := range f.Round {
		h, e := RoundAxial(v.Input[0], v.Input[1])
		if e != nil || h != (Coord{v.Want[0], v.Want[1]}) {
			t.Fatalf("round %+v %+v %v", v, h, e)
		}
	}
	for k, w := range f.RingSizes {
		a, e := Ring(Coord{}, k)
		if e != nil || len(a) != w {
			t.Fatalf("ring %d %d %v", k, len(a), e)
		}
	}
	for k, w := range f.DiskSizes {
		a, e := Disk(Coord{}, k)
		if e != nil || len(a) != w {
			t.Fatalf("disk %d %d %v", k, len(a), e)
		}
	}
	for q := -20; q <= 20; q++ {
		for r := -20; r <= 20; r++ {
			c := Coord{q, r}
			p, e := ToWorld(c, f.Radius)
			if e != nil {
				t.Fatal(e)
			}
			h, e := FromWorld(p, f.Radius)
			if e != nil || h != c {
				t.Fatalf("world roundtrip %+v -> %+v -> %+v: %v", c, p, h, e)
			}
			cube, _ := ToCube(c)
			ret, e := FromCube(cube)
			if e != nil || ret != c {
				t.Fatal("cube", c)
			}
		}
	}
	for k := 0; k < 6; k++ {
		p, _ := ToWorld(Directions[k], f.Radius)
		if math.Abs(math.Hypot(p.X, p.Z)-1) > 1e-9 {
			t.Fatal("length", k)
		}
	}
	e, _ := CanonicalEdge(Coord{}, Directions[0])
	back, _ := CanonicalEdge(Directions[0], Coord{})
	if e != back {
		t.Fatal("edge symmetry")
	}
}
func TestRejectInvalid(t *testing.T) {
	if _, e := RoundAxial(math.NaN(), 0); e == nil {
		t.Fatal("NaN accepted")
	}
	if _, e := Neighbor(Coord{}, 6); e == nil {
		t.Fatal("bad direction")
	}
	if _, e := CanonicalEdge(Coord{}, Coord{2, 0}); e == nil {
		t.Fatal("non-neighbor edge")
	}
}
