// hex-preview is a local read-only server for the HEX Lab. It uses no database.
package main

import (
	"log"
	"net/http"

	"gameservice/handlers"
)

func main() {
	mux := http.NewServeMux()
	mux.HandleFunc("/game/hex-preview", func(w http.ResponseWriter, r *http.Request) {
		origin := r.Header.Get("Origin")
		if origin == "http://localhost:3000" || origin == "http://127.0.0.1:3000" {
			w.Header().Set("Access-Control-Allow-Origin", origin)
		}
		if r.Method != http.MethodGet {
			http.Error(w, "GET only", http.StatusMethodNotAllowed)
			return
		}
		handlers.HexPreviewHandler(w, r)
	})
	log.Println("HEX preview on http://127.0.0.1:18081")
	log.Fatal(http.ListenAndServe("127.0.0.1:18081", mux))
}
