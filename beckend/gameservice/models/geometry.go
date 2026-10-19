package models

import (
	"encoding/json"
	"errors"
	"fmt"
)

var ErrUnsupportedGeometry = errors.New("unsupported match geometry")

// Grid positions and scene positions are intentionally different types.
type SquarePosition struct {
	X int `json:"x"`
	Y int `json:"y"`
}
type HexPosition struct {
	Q int `json:"q"`
	R int `json:"r"`
}
type WorldPosition struct {
	X float64 `json:"x"`
	Y float64 `json:"y"`
	Z float64 `json:"z"`
}

type Geometry struct {
	Kind        string `json:"kind"`
	Coordinates string `json:"coordinates"`
	Orientation string `json:"orientation,omitempty"`
	Version     int    `json:"version"`
}

var SquareV1 = Geometry{Kind: "square", Coordinates: "cartesian", Version: 1}
var HexV1 = Geometry{Kind: "hex", Coordinates: "axial", Orientation: "pointy", Version: 1}

func ParseGeometry(schemaVersion int, raw json.RawMessage) (Geometry, error) {
	if len(raw) == 0 || string(raw) == "null" {
		if schemaVersion == 0 || schemaVersion == 1 {
			return SquareV1, nil
		}
		return Geometry{}, fmt.Errorf("%w: schema version %d requires geometry metadata", ErrUnsupportedGeometry, schemaVersion)
	}
	var g Geometry
	if err := json.Unmarshal(raw, &g); err != nil {
		return Geometry{}, fmt.Errorf("invalid geometry: %w", err)
	}
	if schemaVersion == 1 && g == SquareV1 {
		return g, nil
	}
	if schemaVersion == 2 && g == HexV1 {
		return g, nil
	}
	return Geometry{}, fmt.Errorf("%w: schemaVersion=%d kind=%q coordinates=%q orientation=%q version=%d", ErrUnsupportedGeometry, schemaVersion, g.Kind, g.Coordinates, g.Orientation, g.Version)
}

func DecodePosition(raw json.RawMessage, g Geometry) (any, error) {
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(raw, &fields); err != nil || fields == nil {
		return nil, fmt.Errorf("invalid position")
	}
	var keys []string
	if g == SquareV1 {
		keys = []string{"x", "y"}
	} else if g == HexV1 {
		keys = []string{"q", "r"}
	} else {
		return nil, fmt.Errorf("unsupported geometry")
	}
	if len(fields) != 2 {
		return nil, fmt.Errorf("mixed or incomplete position")
	}
	for _, key := range keys {
		value, ok := fields[key]
		if !ok || string(value) == "null" {
			return nil, fmt.Errorf("missing %s position coordinate", key)
		}
	}
	if g == SquareV1 {
		var p SquarePosition
		if err := json.Unmarshal(raw, &p); err != nil {
			return nil, err
		}
		return p, nil
	}
	var p HexPosition
	if err := json.Unmarshal(raw, &p); err != nil {
		return nil, err
	}
	return p, nil
}
