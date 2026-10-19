package handlers

import (
	"encoding/json"
	"net/http"
	"strconv"

	"gameservice/internal/hexmap"
)

// HexPreviewHandler serves a generated fixture; it creates no match or player.
func HexPreviewHandler(w http.ResponseWriter, r *http.Request) {
	seed := int64(1)
	if raw := r.URL.Query().Get("seed"); raw != "" {
		parsed, err := strconv.ParseInt(raw, 10, 64)
		if err != nil {
			http.Error(w, "seed must be a signed 64-bit integer", http.StatusBadRequest)
			return
		}
		seed = parsed
	}
	snapshot, err := hexmap.Generate(seed)
	if err != nil {
		http.Error(w, "HEX preview generation failed", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	_ = json.NewEncoder(w).Encode(snapshot)
}
