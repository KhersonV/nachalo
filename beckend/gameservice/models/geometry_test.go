package models

import (
	"encoding/json"
	"testing"
)

func TestGeometryAndPositions(t *testing.T) {
	legacy, err := ParseGeometry(1, nil)
	if err != nil || legacy != SquareV1 {
		t.Fatalf("legacy square: %v %v", legacy, err)
	}
	for _, raw := range []string{`{"x":0,"y":0}`, `{"x":-3,"y":4}`} {
		p, err := DecodePosition(json.RawMessage(raw), legacy)
		if err != nil {
			t.Fatal(err)
		}
		encoded, _ := json.Marshal(p)
		if string(encoded) != raw {
			t.Fatalf("square changed: %s", encoded)
		}
	}
	hex, err := ParseGeometry(2, json.RawMessage(`{"kind":"hex","coordinates":"axial","orientation":"pointy","version":1}`))
	if err != nil || hex != HexV1 {
		t.Fatalf("hex: %v %v", hex, err)
	}
	p, err := DecodePosition(json.RawMessage(`{"q":-7,"r":-2}`), hex)
	if err != nil {
		t.Fatal(err)
	}
	encoded, _ := json.Marshal(p)
	if string(encoded) != `{"q":-7,"r":-2}` {
		t.Fatalf("hex changed: %s", encoded)
	}
	for _, raw := range []string{`{"q":0}`, `{"q":null,"r":0}`, `{"q":0,"r":0,"x":0}`, `{"x":0,"y":0}`} {
		if _, err := DecodePosition(json.RawMessage(raw), hex); err == nil {
			t.Fatalf("accepted mixed/incomplete %s", raw)
		}
	}
	for _, tc := range []struct {
		version int
		raw     string
	}{
		{2, "null"},
		{3, `{"kind":"hex","coordinates":"axial","orientation":"pointy","version":1}`},
		{2, `{"kind":"hex","coordinates":"axial","orientation":"pointy","version":9}`},
	} {
		if _, err := ParseGeometry(tc.version, json.RawMessage(tc.raw)); err == nil {
			t.Fatalf("accepted geometry %+v", tc)
		}
	}
}
