package handlers

import (
	"encoding/json"
	"errors"
	"net/http"

	"gameservice/middleware"
	"gameservice/repository"
)

// Storage is the existing account inventory. Only the authenticated owner may view or select it.
func HexStorageHandler(w http.ResponseWriter, r *http.Request) {
	userID, ok := middleware.GetUserIDFromContext(r.Context())
	if !ok {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	var (
		state repository.StorageState
		err   error
	)
	switch r.Method {
	case http.MethodGet:
		state, err = repository.GetHexStorage(r.Context(), userID)
	case http.MethodPut:
		var request struct {
			Loadout map[string]int `json:"loadout"`
		}
		decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, 64<<10))
		decoder.DisallowUnknownFields()
		if decodeErr := decoder.Decode(&request); decodeErr != nil {
			http.Error(w, "invalid loadout", http.StatusBadRequest)
			return
		}
		state, err = repository.SaveHexLoadout(r.Context(), userID, request.Loadout)
	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if err != nil {
		if errors.Is(err, repository.ErrLoadoutUnavailable) || errors.Is(err, repository.ErrLoadoutInvalid) || errors.Is(err, repository.ErrHexPockets) {
			http.Error(w, err.Error(), http.StatusConflict)
			return
		}
		http.Error(w, "Storage unavailable", http.StatusInternalServerError)
		return
	}
	_ = json.NewEncoder(w).Encode(state)
}
