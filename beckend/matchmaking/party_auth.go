package main

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"os"

	"gameservice/middleware"
)

// The existing Party handlers keep their request format, but a caller may only
// mutate Party state using their own authenticated player/leader ID.
func authenticatedPartyMutation(actor string, next http.HandlerFunc) http.Handler {
	secret := os.Getenv("JWT_SECRET_KEY")
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if secret == "" {
			http.Error(w, "party authentication unavailable", http.StatusServiceUnavailable)
			return
		}
		middleware.GameAuthMiddleware(secret, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			userID, ok := middleware.GetUserIDFromContext(r.Context())
			if !ok || userID <= 0 {
				http.Error(w, "unauthorized", http.StatusUnauthorized)
				return
			}
			body, err := io.ReadAll(io.LimitReader(r.Body, 4097))
			if err != nil || len(body) > 4096 {
				http.Error(w, "invalid party request", http.StatusBadRequest)
				return
			}
			var action PartyActionRequest
			if err := json.Unmarshal(body, &action); err != nil {
				http.Error(w, "invalid party request", http.StatusBadRequest)
				return
			}
			claimedID := action.PlayerID
			if actor == "leader" {
				claimedID = action.LeaderID
			}
			if claimedID != userID {
				http.Error(w, "party actor does not match authenticated player", http.StatusForbidden)
				return
			}
			r.Body = io.NopCloser(bytes.NewReader(body))
			next(w, r)
		})).ServeHTTP(w, r)
	})
}
