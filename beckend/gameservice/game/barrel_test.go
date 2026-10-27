//=================================
// gameservice/game/barrel_test.go
//=================================

package game

import (
	"reflect"
	"testing"
)

// helper для создания клетки с баррелем
func makeCell() FullCell {
	return FullCell{
		CellID:   42,
		X:        0,
		Y:        0,
		TileCode: int('B'),
		Barbel:   &ResourceData{ID: 6, Type: "barrbel"},
	}
}

func TestOpenBarbel_DamageEvent(t *testing.T) {
	// r < 0.3 → урон
	rnd = func() float64 { return 0.1 }
	cell := makeCell()
	out, err := OpenBarbel(cell, nil, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if _, ok := out.(DamageEvent); !ok {
		t.Errorf("expected DamageEvent, got %T", out)
	}
}

func TestOpenBarbel_ResourceEvent(t *testing.T) {
	// 0.3 ≤ r < 0.7 → ресурс
	rnd = func() float64 { return 0.5 }
	cell := makeCell()
	resources := []ResourceData{
		{ID: 2, Type: "food"},
	}
	out, err := OpenBarbel(cell, resources, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	r, ok := out.(ResourceData)
	if !ok {
		t.Fatalf("expected ResourceData, got %T", out)
	}
	if r.Type != "food" {
		t.Errorf("expected food, got %q", r.Type)
	}
}

func TestOpenBarbel_ArtifactEvent(t *testing.T) {
	// 0.7 ≤ r < 0.74 → ordinary artifact
	t.Setenv("BARREL_ARTIFACT_CHANCE_PERCENT", "4")
	rnd = func() float64 { return 0.72 }
	cell := makeCell()
	artifacts := []ResourceData{
		{ID: 10, Type: "magic_ring"},
	}
	out, err := OpenBarbel(cell, nil, artifacts)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	a, ok := out.(ResourceData)
	if !ok {
		t.Fatalf("expected ResourceData, got %T", out)
	}
	if a.Type != "magic_ring" {
		t.Errorf("expected magic_ring, got %q", a.Type)
	}
}

func TestOpenBarbel_Empty_NoBarrel(t *testing.T) {
	// если Barbel == nil, всегда nil
	rnd = func() float64 { return 0.1 } // неважно
	out, err := OpenBarbel(FullCell{}, nil, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if out != nil {
		t.Errorf("expected nil, got %v", out)
	}
}

func TestOpenBarrel_AllCases(t *testing.T) {
	t.Setenv("BARREL_ARTIFACT_CHANCE_PERCENT", "4")
	cell := makeCell()
	resources := []ResourceData{{ID: 2, Type: "food"}}
	artifacts := []ResourceData{{ID: 10, Type: "magic_ring"}}

	cases := []struct {
		name  string
		rnd   float64
		wantT reflect.Type
	}{
		{"damage", 0.2, reflect.TypeOf(DamageEvent{})},
		{"resource", 0.5, reflect.TypeOf(ResourceData{})},
		{"artifact", 0.72, reflect.TypeOf(ResourceData{})},
		{"empty", 0.8, reflect.TypeOf(EmptyBarrelEvent{})},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			rnd = func() float64 { return c.rnd }
			out, _ := OpenBarbel(cell, resources, artifacts)
			if reflect.TypeOf(out) != c.wantT {
				t.Errorf("%q: expected %v, got %T", c.name, c.wantT, out)
			}
		})
	}
}

// Exercise category boundaries and absent pools without statistical sampling.
func TestBarrelCategoryBoundariesAndMissingPools(t *testing.T) {
	t.Setenv("BARREL_ARTIFACT_CHANCE_PERCENT", "4")
	original := rnd
	t.Cleanup(func() { rnd = original })
	resources := []ResourceData{{ID: 2, Type: "food"}}
	artifacts := []ResourceData{{ID: 10, Type: "magic_ring"}}
	for _, tc := range []struct {
		name                 string
		roll                 float64
		resources, artifacts []ResourceData
		want                 any
	}{
		{"damage upper", .299999, resources, artifacts, DamageEvent{}},
		{"resource lower", .3, resources, artifacts, ResourceData{}},
		{"resource upper", .699999, resources, artifacts, ResourceData{}},
		{"artifact lower", .7, resources, artifacts, ResourceData{}},
		{"artifact upper", .739999, resources, artifacts, ResourceData{}},
		{"empty lower", .74, resources, artifacts, EmptyBarrelEvent{}},
		{"empty upper", .999999, resources, artifacts, EmptyBarrelEvent{}},
		{"no resource fallback", .5, nil, artifacts, EmptyBarrelEvent{}},
		{"no artifact fallback", .72, resources, nil, EmptyBarrelEvent{}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			rolls := 0
			rnd = func() float64 { rolls++; return tc.roll }
			got, err := OpenBarbel(makeCell(), tc.resources, tc.artifacts)
			if err != nil || reflect.TypeOf(got) != reflect.TypeOf(tc.want) {
				t.Fatalf("got %T, %v; want %T", got, err, tc.want)
			}
			if rolls != 1 {
				t.Fatalf("category rolls = %d, want 1", rolls)
			}
		})
	}
}

func TestBarrelArtifactChanceConfiguration(t *testing.T) {
	for _, tc := range []struct {
		raw  string
		want int
	}{{"", 4}, {"4", 4}, {"0", 0}, {"12", 12}, {"30", 30}, {"31", 4}, {"-1", 4}, {"invalid", 4}} {
		t.Run(tc.raw, func(t *testing.T) {
			t.Setenv("BARREL_ARTIFACT_CHANCE_PERCENT", tc.raw)
			if got := BarrelArtifactChancePercent(); got != tc.want {
				t.Fatalf("got %d, want %d", got, tc.want)
			}
		})
	}
}
