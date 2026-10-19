package handlers

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"gameservice/internal/hexmap"
)

func TestHexPreviewHTTP(t *testing.T) {
	for _, tc := range []struct {
		path   string
		status int
	}{{"/game/hex-preview?seed=-17", http.StatusOK}, {"/game/hex-preview?seed=bad", http.StatusBadRequest}} {
		r := httptest.NewRecorder()
		HexPreviewHandler(r, httptest.NewRequest(http.MethodGet, tc.path, nil))
		if r.Code != tc.status {
			t.Fatalf("%s: %d", tc.path, r.Code)
		}
		if tc.status == http.StatusOK {
			var snapshot hexmap.Snapshot
			if err := json.Unmarshal(r.Body.Bytes(), &snapshot); err != nil {
				t.Fatal(err)
			}
			if snapshot.Seed != -17 || snapshot.Geometry.Kind != "hex" || len(snapshot.Cells) == 0 {
				t.Fatal("bad HTTP snapshot")
			}
		}
	}
}
