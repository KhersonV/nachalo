
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
	// r ≥ 0.7 → артефакт
	rnd = func() float64 { return 0.8 }
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
		{"artifact", 0.8, reflect.TypeOf(ResourceData{})},
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
