package handlers

import (
	"encoding/json"
	"testing"
)

func TestDecodeSquarePair(t *testing.T) {
	for _, raw := range []string{`[0,0]`, `[-2,3]`} {
		if _, err := decodeSquarePair(json.RawMessage(raw)); err != nil {
			t.Fatalf("%s: %v", raw, err)
		}
	}
	for _, raw := range []string{`[0]`, `[0,0,0]`, `[0,null]`, `{"q":0,"r":0}`} {
		if _, err := decodeSquarePair(json.RawMessage(raw)); err == nil {
			t.Fatalf("accepted %s", raw)
		}
	}
}
