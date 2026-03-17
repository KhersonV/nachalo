package main

import (
    "encoding/json"
    "fmt"
    "log"
    "net/http"
    "strconv"
    "sync"
    "time"
)

// Простая SSE-хаб реализация: для каждого playerID держим канал отправки строк
var (
    sseMu sync.Mutex
    sseClients = make(map[int]chan string)
)

// registerSSEClient добавляет клиента для playerID и возвращает канал, который
// будет закрыт при отключении.
func registerSSEClient(playerID int) chan string {
    sseMu.Lock()
    defer sseMu.Unlock()
    ch := make(chan string, 4)
    sseClients[playerID] = ch
    return ch
}

func unregisterSSEClient(playerID int) {
    sseMu.Lock()
    defer sseMu.Unlock()
    if ch, ok := sseClients[playerID]; ok {
        close(ch)
        delete(sseClients, playerID)
    }
}

// BroadcastMatchToPlayers шлёт уведомление матча соответствующим игрокам
func BroadcastMatchToPlayers(playerIDs []int, match MatchInfo) {
    b, err := json.Marshal(match)
    if err != nil {
        log.Printf("Broadcast marshal error: %v", err)
        return
    }
    msg := string(b)
    sseMu.Lock()
    defer sseMu.Unlock()
    for _, pid := range playerIDs {
        if ch, ok := sseClients[pid]; ok {
            // не блокируем основной поток — шлём в буферный канал
            select {
            case ch <- msg:
            default:
                // если канал переполнен, пропускаем
                log.Printf("SSE channel full for player %d, skipping", pid)
            }
        }
    }
}

// sseHandler поддерживает EventSource для конкретного player_id
func sseHandler(w http.ResponseWriter, r *http.Request) {
    playerIDStr := r.URL.Query().Get("player_id")
    if playerIDStr == "" {
        http.Error(w, "player_id обязателен", http.StatusBadRequest)
        return
    }
    pid, err := strconv.Atoi(playerIDStr)
    if err != nil || pid <= 0 {
        http.Error(w, "Некорректный player_id", http.StatusBadRequest)
        return
    }

    // Объявляем SSE заголовки
    w.Header().Set("Content-Type", "text/event-stream")
    w.Header().Set("Cache-Control", "no-cache")
    w.Header().Set("Connection", "keep-alive")
    w.Header().Set("Access-Control-Allow-Origin", "*")

    flusher, ok := w.(http.Flusher)
    if !ok {
        http.Error(w, "Streaming unsupported", http.StatusInternalServerError)
        return
    }

    ch := registerSSEClient(pid)
    defer unregisterSSEClient(pid)

    // immediately send current match if exists
    matchMu.Lock()
    if inst, ok := playerMatches[pid]; ok {
        if m, ok2 := currentMatches[inst]; ok2 {
            b, _ := json.Marshal(m)
            fmt.Fprintf(w, "data: %s\n\n", b)
            flusher.Flush()
        }
    }
    matchMu.Unlock()

    // keepalive ticker
    keep := time.NewTicker(25 * time.Second)
    defer keep.Stop()

    notify := w.(http.CloseNotifier).CloseNotify()

    for {
        select {
        case <-notify:
            return
        case <-keep.C:
            // comment ping to keep connection alive
            fmt.Fprint(w, ": ping\n\n")
            flusher.Flush()
        case msg, ok := <-ch:
            if !ok {
                return
            }
            fmt.Fprintf(w, "data: %s\n\n", msg)
            flusher.Flush()
        }
    }
}
