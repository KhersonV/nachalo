
//Match_Making
package main

import (
	"bytes"
	"encoding/json"
	"log"
	"net/http"
	"sync"
	"time"

	"github.com/gorilla/mux"
	"github.com/google/uuid"
)

type JoinRequest struct {
	PlayerID int    `json:"player_id"`
	Mode     string `json:"mode"`
	Rating   int    `json:"rating"`
}

type CancelRequest struct {
	PlayerID int    `json:"player_id"`
	Mode     string `json:"mode"`
}

type QueueEntry struct {
	PlayerID int       `json:"player_id"`
	Rating   int       `json:"rating"`
	JoinTime time.Time `json:"join_time"`
}

type MatchInfo struct {
	InstanceID   string       `json:"instance_id"`
	Mode         string       `json:"mode"`
	Players      []QueueEntry `json:"players"`
	TeamsCount   int          `json:"teams_count"`
	TotalPlayers int          `json:"total_players"`
}

var (
	queues = map[string][]QueueEntry{
		"PVE":  {},
		"1x1":  {},
		"3x3":  {},
		"5x5":  {},
	}
	mu sync.Mutex

	currentMatches = make(map[string]MatchInfo)
	matchMu        sync.Mutex
)

func joinHandler(w http.ResponseWriter, r *http.Request) {
	var req JoinRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Bad request", http.StatusBadRequest)
		return
	}

	mu.Lock()
	defer mu.Unlock()

	q, ok := queues[req.Mode]
	if !ok {
		http.Error(w, "Unknown mode", http.StatusBadRequest)
		return
	}

	entry := QueueEntry{
		PlayerID: req.PlayerID,
		Rating:   req.Rating,
		JoinTime: time.Now(),
	}
	queues[req.Mode] = append(q, entry)
	log.Printf("Player %d joined mode %s", req.PlayerID, req.Mode)

	checkAndMakeMatch(req.Mode)
	w.WriteHeader(http.StatusOK)
	w.Write([]byte("You have joined the queue"))
}

func cancelHandler(w http.ResponseWriter, r *http.Request) {
	var req CancelRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Bad request", http.StatusBadRequest)
		return
	}
	mu.Lock()
	defer mu.Unlock()

	q, ok := queues[req.Mode]
	if !ok {
		http.Error(w, "Unknown mode", http.StatusBadRequest)
		return
	}

	newQueue := []QueueEntry{}
	found := false
	for _, entry := range q {
		if entry.PlayerID == req.PlayerID {
			found = true
			continue
		}
		newQueue = append(newQueue, entry)
	}
	queues[req.Mode] = newQueue
	if found {
		log.Printf("Player %d cancelled from mode %s", req.PlayerID, req.Mode)
		w.Write([]byte("Cancelled successfully"))
	} else {
		http.Error(w, "Player not found in queue", http.StatusNotFound)
	}
}

func statusHandler(w http.ResponseWriter, r *http.Request) {
	mode := r.URL.Query().Get("mode")
	mu.Lock()
	defer mu.Unlock()
	q, ok := queues[mode]
	if !ok {
		http.Error(w, "Unknown mode", http.StatusBadRequest)
		return
	}
	response, err := json.Marshal(q)
	if err != nil {
		http.Error(w, "Error marshalling response", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.Write(response)
}

func matchHandler(w http.ResponseWriter, r *http.Request) {
	mode := r.URL.Query().Get("mode")
	playerID := r.URL.Query().Get("player_id")
	if mode == "" || playerID == "" {
		http.Error(w, "mode и player_id обязательны", http.StatusBadRequest)
		return
	}

	matchMu.Lock()
	defer matchMu.Unlock()
	if match, ok := currentMatches[mode]; ok {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(match)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": "waiting"})
}

func checkAndMakeMatch(mode string) {
	var needed int
	if mode == "PVE" {
		needed = 1
	} else {
		switch mode {
		case "1x1":
			needed = 2
		case "3x3":
			needed = 6
		case "5x5":
			needed = 10
		default:
			return
		}
	}

	q := queues[mode]
	if len(q) >= needed {
		group := q[:needed]
		queues[mode] = q[needed:]
		go createMatch(mode, group)
	}
}

func createMatch(mode string, group []QueueEntry) {
	instanceID := uuid.New().String()
	log.Printf("Match formed: instanceID=%s, mode=%s, players=%v", instanceID, mode, group)

	var totalPlayers int
	var teamsCount int
	if mode == "PVE" {
		totalPlayers = 1
		teamsCount = 1
	} else {
		switch mode {
		case "1x1":
			totalPlayers = 2
		case "3x3":
			totalPlayers = 6
		case "5x5":
			totalPlayers = 10
		default:
			totalPlayers = len(group)
		}
		teamsCount = 2
	}

	playerIDs := make([]int, len(group))
for i, entry := range group {
	playerIDs[i] = entry.PlayerID
}

	matchReq := map[string]interface{}{
		"instance_id":   instanceID,
		"mode":          mode,
		"player_ids":    playerIDs,
		"teams_count":   teamsCount,
		"total_players": totalPlayers,
	}

	reqJSON, err := json.Marshal(matchReq)
	if err != nil {
		log.Printf("Ошибка маршалинга запроса матча: %v", err)
		return
	}

	log.Printf("Отправляем запрос в Game-сервис: %s", string(reqJSON))
	resp, err := http.Post("http://localhost:8001/game/createMatch", "application/json", bytes.NewBuffer(reqJSON))
	if err != nil {
		log.Printf("Ошибка вызова Game-сервиса: %v", err)
		return
	}
	defer resp.Body.Close()
	log.Printf("Game-сервис вернул статус: %s", resp.Status)

	matchInfo := MatchInfo{
		InstanceID:   instanceID,
		Mode:         mode,
		Players:      group,
		TeamsCount:   teamsCount,
		TotalPlayers: totalPlayers,
	}
	matchMu.Lock()
	currentMatches[mode] = matchInfo
	matchMu.Unlock()
}

func enableCors(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusOK)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func main() {
	r := mux.NewRouter()
	r.HandleFunc("/matchmaking/join", joinHandler).Methods("POST")
	r.HandleFunc("/matchmaking/cancel", cancelHandler).Methods("POST")
	r.HandleFunc("/matchmaking/status", statusHandler).Methods("GET")
	r.HandleFunc("/matchmaking/match", matchHandler).Methods("GET")

	handler := enableCors(r)

	srv := &http.Server{
		Handler: handler,
		Addr:    ":8002",
	}
	log.Println("Matchmaking-сервис запущен на порту 8002")
	log.Fatal(srv.ListenAndServe())
}
