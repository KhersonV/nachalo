// ==============================
// /matchmaking/Match_Making.go
// ==============================

package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/gorilla/mux"

	"gameservice/game"
	"gameservice/repository"
)

// Типы запросов и структур остаются без изменений.
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
		"1x2":  {},
		"2x2":  {},
		"3x3":  {},
		"5x5":  {},
	}
	mu sync.Mutex

	// Храним матчи по instance_id
	currentMatches = make(map[string]MatchInfo)
	// Глобальная мапа сопоставлений: для каждого игрока его instance_id матча
	playerMatches = make(map[int]string)
	matchMu       sync.Mutex
)

// joinHandler – добавляет игрока в очередь и вызывает checkAndMakeMatch
func joinHandler(w http.ResponseWriter, r *http.Request) {
	var req JoinRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Bad request", http.StatusBadRequest)
		return
	}

	// проверяем в БД через GetPlayersInMatch, не участвует ли уже в каком-то currentMatches ====
	matchMu.Lock()
	for instID, match := range currentMatches {
		players, err := repository.GetPlayersInMatch(instID)
		if err != nil {
			log.Printf("joinHandler: не удалось получить игроков для матча %s: %v", instID, err)
			continue
		}
		for _, p := range players {
			if p.UserID == req.PlayerID {
				// Уже есть активный матч с этим игроком — сразу возвращаем его
				matchMu.Unlock()
				w.Header().Set("Content-Type", "application/json")
				if err := json.NewEncoder(w).Encode(match); err != nil {
					log.Printf("joinHandler: ошибка кодирования существующего матча: %v", err)
				}
				return
			}
		}
	}
	matchMu.Unlock()
	// ==== /конец проверки активного матча ====

	mu.Lock()
	defer mu.Unlock()

	q, ok := queues[req.Mode]
	if !ok {
		http.Error(w, "Unknown mode", http.StatusBadRequest)
		return
	}

	// Если игрок уже в очереди, сообщаем об ошибке
	for _, entry := range q {
		if entry.PlayerID == req.PlayerID {
			http.Error(w, "Player already in queue", http.StatusBadRequest)
			return
		}
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

// cancelHandler – удаляет игрока из очереди
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

// statusHandler – возвращает очередь для указанного режима
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

// Новый эндпоинт: currentMatch – возвращает текущий матч для игрока по player_id
func currentMatchHandler(w http.ResponseWriter, r *http.Request) {
	playerIDStr := r.URL.Query().Get("player_id")
	if playerIDStr == "" {
		http.Error(w, "player_id обязателен", http.StatusBadRequest)
		return
	}

	var playerID int
	if _, err := fmt.Sscanf(playerIDStr, "%d", &playerID); err != nil {
		http.Error(w, "Некорректный player_id", http.StatusBadRequest)
		return
	}

	matchMu.Lock()
	defer matchMu.Unlock()

	instanceID, exists := playerMatches[playerID]
	if !exists {
		// Если для данного игрока матч ещё не сформирован
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"status": "waiting"})
		return
	}

	match, ok := currentMatches[instanceID]
	if !ok {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"status": "waiting"})
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(match)
}

// matchHandler – оставляем для совместимости (делегирует currentMatchHandler)
func matchHandler(w http.ResponseWriter, r *http.Request) {
	currentMatchHandler(w, r)
}

// checkAndMakeMatch – если в очереди набрано нужное количество игроков, формирует матч
func checkAndMakeMatch(mode string) {
	requiredPlayers := map[string]int{"PVE": 1, "1x1": 2, "1x2": 3, "2x2": 4, "3x3": 6, "5x5": 10}
	needed, ok := requiredPlayers[mode]
	if !ok {
		log.Printf("Unknown mode: %s", mode)
		return
	}

	q := queues[mode]
	if len(q) >= needed {
		group := q[:needed]
		queues[mode] = q[needed:]
		go createMatch(mode, group)
	}
}

// createMatch – создает новый матч, обновляет currentMatches и playerMatches
func createMatch(mode string, group []QueueEntry) {
	instanceID := uuid.New().String()

	var totalPlayers, teamsCount int
	if mode == "PVE" {
		totalPlayers, teamsCount = 1, 1
	} else {
		 switch mode {
        case "1x1":
            totalPlayers = 2
            teamsCount = 2
        case "1x2":
            totalPlayers = 3
            teamsCount = 3
        case "2x2":
            totalPlayers = 4
            teamsCount = 2
        case "3x3":
            totalPlayers = 6
            teamsCount = 2
        case "5x5":
            totalPlayers = 10
            teamsCount = 2
        default:
            totalPlayers = len(group)
            teamsCount = 2
        }
    }

	playerIDs := make([]int, len(group))
	for i, entry := range group {
		playerIDs[i] = entry.PlayerID
	}

	// Создаём внутреннее состояние матча
	matchState := game.CreateMatchState(instanceID, playerIDs)
	log.Printf("Создано состояние матча: %+v", matchState)

	// Отправляем запрос в Game-сервис
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
	resp, err := http.Post("http://localhost:8001/game/createMatch", "application/json", bytes.NewBuffer(reqJSON))
	if err != nil {
		log.Printf("Ошибка вызова Game-сервиса: %v", err)
		return
	}
	defer resp.Body.Close()
	log.Printf("Game-сервис вернул статус: %s", resp.Status)

	// Сохраняем состояние матча и сопоставление для каждого игрока
	matchMu.Lock()
	currentMatches[instanceID] = MatchInfo{
		InstanceID:   instanceID,
		Mode:         mode,
		Players:      group,
		TeamsCount:   teamsCount,
		TotalPlayers: totalPlayers,
	}
	for _, entry := range group {
		playerMatches[entry.PlayerID] = instanceID
	}
	matchMu.Unlock()
}

// Простая CORS-обёртка
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
	repository.InitDB()

	r := mux.NewRouter()
	r.HandleFunc("/matchmaking/join", joinHandler).Methods("POST")
	r.HandleFunc("/matchmaking/cancel", cancelHandler).Methods("POST")
	r.HandleFunc("/matchmaking/status", statusHandler).Methods("GET")
	r.HandleFunc("/matchmaking/currentMatch", currentMatchHandler).Methods("GET")
	r.HandleFunc("/matchmaking/match", matchHandler).Methods("GET")

	handler := enableCors(r)
	srv := &http.Server{Handler: handler, Addr: ":8002"}

	log.Println("Matchmaking-сервис запущен на порту 8002")
	log.Fatal(srv.ListenAndServe())
}
