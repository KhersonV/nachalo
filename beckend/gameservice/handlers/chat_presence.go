package handlers

import (
	"context"
	"encoding/json"
	"sync"
	"time"

	"gameservice/repository"
)

type onlinePresenceEvent struct {
	kind   string
	player repository.ChatOnlinePlayer
}

var onlinePresenceQueue = make(chan onlinePresenceEvent, 256)
var onlinePresenceOnce sync.Once

func queueOnlinePresence(event onlinePresenceEvent) {
	onlinePresenceOnce.Do(func() {
		go func() {
			for next := range onlinePresenceQueue {
				broadcastOnlineEvent(next.kind, next.player)
			}
		}()
	})
	select {
	case onlinePresenceQueue <- event:
	default:
		// Clients resynchronize from the authenticated snapshot on reconnect.
	}
}

func publishOnlineJoin(userID int) {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	player, err := repository.OnlineChatPlayer(ctx, userID)
	cancel()
	if err != nil {
		return
	}
	clientsMu.Lock()
	stillOnline := len(clients[userID]) > 0
	clientsMu.Unlock()
	if stillOnline {
		queueOnlinePresence(onlinePresenceEvent{kind: "presence.join", player: player})
	}
}

func publishOnlineLeave(userID int) {
	queueOnlinePresence(onlinePresenceEvent{kind: "presence.leave", player: repository.ChatOnlinePlayer{UserID: userID}})
}

func broadcastOnlineEvent(kind string, player repository.ChatOnlinePlayer) {
	payload, err := json.Marshal(struct {
		Type    string                      `json:"type"`
		Payload repository.ChatOnlinePlayer `json:"payload"`
	}{Type: kind, Payload: player})
	if err != nil {
		return
	}
	clientsMu.Lock()
	var targets []*Client
	for _, sessions := range clients {
		for client := range sessions {
			targets = append(targets, client)
		}
	}
	clientsMu.Unlock()
	fanoutChatPayload(targets, payload)
}
