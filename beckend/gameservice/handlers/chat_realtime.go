package handlers

import (
	"context"
	"encoding/json"
	"log"
	"sync"
	"time"

	"gameservice/repository"
	"github.com/gorilla/websocket"
)

// Chat delivery is best effort. PostgreSQL history remains authoritative when a
// socket is slow, disconnected, or the bounded delivery queue is full.
var chatDeliveryQueue = make(chan repository.ChatMessage, 256)
var chatDeliveryOnce sync.Once

func publishChatMessage(message repository.ChatMessage) {
	chatDeliveryOnce.Do(func() {
		go func() {
			for saved := range chatDeliveryQueue {
				deliverChatMessage(saved)
			}
		}()
	})
	select {
	case chatDeliveryQueue <- message:
	default:
		log.Printf("chat live delivery queue full; message id=%d remains in history", message.ID)
	}
}

func deliverChatMessage(message repository.ChatMessage) {
	allowed := make(map[int]struct{})
	switch message.Channel {
	case "MAIN", "TRADE":
		// All authenticated sockets are selected below.
	case "PRIVATE":
		if message.ConversationID == nil {
			return
		}
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		members, err := repository.ActiveChatConversationParticipants(ctx, *message.ConversationID)
		cancel()
		if err != nil { return }
		for _, userID := range members { allowed[userID] = struct{}{} }
	case "GROUP":
		if message.PartyID == nil {
			return
		}
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		partyID, members, err := currentChatPartyMembers(ctx, message.SenderUserID)
		cancel()
		if err != nil || partyID != *message.PartyID {
			return
		}
		for _, userID := range members {
			ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
			currentID, err := currentChatParty(ctx, userID)
			cancel()
			if err == nil && currentID == partyID {
				allowed[userID] = struct{}{}
			}
		}
	default:
		// SYSTEM is reserved for trusted server events, not client submissions.
		return
	}

	payload, err := json.Marshal(struct {
		Type    string                 `json:"type"`
		Payload repository.ChatMessage `json:"payload"`
	}{Type: "chat.message", Payload: message})
	if err != nil {
		return
	}

	clientsMu.Lock()
	var targets []*Client
	for userID, connections := range clients {
		if userID <= 0 {
			continue
		}
		if message.Channel != "MAIN" && message.Channel != "TRADE" {
			if _, ok := allowed[userID]; !ok {
				continue
			}
		}
		for client := range connections {
			targets = append(targets, client)
		}
	}
	clientsMu.Unlock()
	if len(targets) == 0 {
		return
	}
	viewerIDs := make([]int, 0, len(targets))
	seen := make(map[int]struct{})
	for _, client := range targets {
		if _, ok := seen[client.userID]; !ok {
			seen[client.userID] = struct{}{}
			viewerIDs = append(viewerIDs, client.userID)
		}
	}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	muted, err := repository.MutedChatViewers(ctx, message.SenderUserID, viewerIDs)
	cancel()
	if err != nil {
		return
	}
	visible := targets[:0]
	for _, client := range targets {
		if _, hidden := muted[client.userID]; !hidden {
			visible = append(visible, client)
		}
	}
	fanoutChatPayload(visible, payload)
}

func fanoutChatPayload(targets []*Client, payload []byte) {
	if len(targets) == 0 {
		return
	}
	workers := len(targets)
	if workers > 8 {
		workers = 8
	}
	jobs := make(chan *Client, len(targets))
	var done sync.WaitGroup
	for i := 0; i < workers; i++ {
		done.Add(1)
		go func() {
			defer done.Done()
			for client := range jobs {
				writeChatEvent(client, payload)
			}
		}()
	}
	for _, client := range targets {
		jobs <- client
	}
	close(jobs)
	done.Wait()
}

func writeChatEvent(client *Client, payload []byte) {
	if !client.writeMu.TryLock() {
		return
	}
	_ = client.Conn.SetWriteDeadline(time.Now().Add(500 * time.Millisecond))
	err := client.Conn.WriteMessage(websocket.TextMessage, payload)
	_ = client.Conn.SetWriteDeadline(time.Time{})
	client.writeMu.Unlock()
	if err == nil {
		return
	}
	_ = client.Conn.Close()
	clientsMu.Lock()
	wasOnline := len(clients[client.userID]) > 0
	delete(clients[client.userID], client)
	last := wasOnline && len(clients[client.userID]) == 0
	if last {
		delete(clients, client.userID)
	}
	clientsMu.Unlock()
	if last {
		publishOnlineLeave(client.userID)
	}
}
