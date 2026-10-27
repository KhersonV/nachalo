//====================================
// gameservice/handlers/ws_handler.go
//====================================

package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"sync"
	"time"

	"gameservice/internal/hexmap"
	"gameservice/repository"

	"github.com/golang-jwt/jwt/v4"
	"github.com/gorilla/websocket"
)

var (
	upgrader = websocket.Upgrader{
		CheckOrigin: func(r *http.Request) bool { return true },
	}
	clients       = make(map[int]map[*Client]struct{})
	clientsMu     sync.Mutex
	presenceOpsMu sync.Mutex

	httpPresenceMu sync.Mutex
	httpPresence   = make(map[int]time.Time)
)

const reconnectGracePeriod = 3 * time.Minute
const httpPresenceTTL = 30 * time.Second
const socketHeartbeatInterval = 15 * time.Second
const socketReadTimeout = 45 * time.Second

func StartSocketHeartbeat(ctx context.Context) {
	go func() {
		ticker := time.NewTicker(socketHeartbeatInterval)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				clientsMu.Lock()
				var active []*Client
				for _, connections := range clients {
					for client := range connections {
						active = append(active, client)
					}
				}
				clientsMu.Unlock()
				for _, client := range active {
					if err := client.Conn.WriteControl(websocket.PingMessage, nil, time.Now().Add(2*time.Second)); err != nil {
						_ = client.Conn.Close()
					}
				}
			}
		}
	}()
}

// Client представляет подключённого клиента WebSocket.
type Client struct {
	Conn       *websocket.Conn
	instanceID string
	userID     int
	writeMu    sync.Mutex
}

func (c *Client) write(message []byte) error {
	c.writeMu.Lock()
	defer c.writeMu.Unlock()
	return c.Conn.WriteMessage(websocket.TextMessage, message)
}

func hasMatchSocketLocked(userID int, instanceID string) bool {
	for client := range clients[userID] {
		if client.instanceID == instanceID {
			return true
		}
	}
	return false
}

func broadcastPresence(kind, instanceID string, userID int) {
	message, err := json.Marshal(map[string]any{"type": kind, "payload": map[string]any{"instanceId": instanceID, "schemaVersion": 2, "userId": userID, "graceMs": int(reconnectGracePeriod / time.Millisecond)}})
	if err == nil {
		Broadcast(message)
	}
}

type broadcastEnvelope struct {
	Payload struct {
		InstanceID    string `json:"instanceId"`
		SchemaVersion int    `json:"schemaVersion"`
	} `json:"payload"`
}

func isAliveMatchPlayer(instanceID string, userID int) bool {
	if instanceID == "" || userID == 0 {
		return false
	}
	var alive bool
	err := repository.DB.QueryRow(`SELECT EXISTS(SELECT 1 FROM match_players p JOIN matches m ON m.instance_id=p.instance_id WHERE p.instance_id=$1 AND p.user_id=$2 AND p.health>0 AND m.finished_at IS NULL AND m.geometry->>'kind'='hex')`, instanceID, userID).Scan(&alive)
	return err == nil && alive
}

func WsHandler(w http.ResponseWriter, r *http.Request) {
	// 1. Получаем токен из query-параметра
	tokenString := r.URL.Query().Get("token")
	if tokenString == "" {
		http.Error(w, "Missing token", http.StatusUnauthorized)
		return
	}

	// 2. Проверяем JWT токен и достаем user_id
	token, err := jwt.Parse(tokenString, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}
		return []byte(os.Getenv("JWT_SECRET_KEY")), nil
	})
	if err != nil || !token.Valid {
		http.Error(w, "Invalid token", http.StatusUnauthorized)
		return
	}
	claims, ok := token.Claims.(jwt.MapClaims)
	if !ok {
		http.Error(w, "Invalid token claims", http.StatusUnauthorized)
		return
	}
	userID, ok := claims["user_id"].(float64)
	if !ok || userID <= 0 || userID != float64(int(userID)) {
		http.Error(w, "user_id not found in token", http.StatusUnauthorized)
		return
	}

	exists, lookupErr := repository.AuthenticationPlayerExists(r.Context(), int(userID))
	if lookupErr != nil {
		http.Error(w, "Authentication storage unavailable", http.StatusServiceUnavailable)
		return
	}
	if !exists {
		http.Error(w, "Account no longer exists", http.StatusUnauthorized)
		return
	}

	// 3. Апгрейдим соединение, если всё ок
	ws, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		return
	}
	_ = ws.SetReadDeadline(time.Now().Add(socketReadTimeout))
	ws.SetPongHandler(func(string) error { return ws.SetReadDeadline(time.Now().Add(socketReadTimeout)) })
	client := &Client{Conn: ws, userID: int(userID)}
	// Pre-fill instanceID from query param to avoid race where server
	// broadcasts match events before client sends JOIN_MATCH message.
	if qInstance := r.URL.Query().Get("instanceId"); isAliveMatchPlayer(qInstance, client.userID) {
		client.instanceID = qInstance
	}

	clientsMu.Lock()
	firstSession := len(clients[client.userID]) == 0
	if clients[client.userID] == nil {
		clients[client.userID] = make(map[*Client]struct{})
	}
	clients[client.userID][client] = struct{}{}
	clientsMu.Unlock()
	if firstSession {
		publishOnlineJoin(client.userID)
	}
	log.Printf("[WsHandler] Новый клиент userID=%d подключён.", client.userID)

	// 4. Читаем сообщения клиента
	for {
		messageType, message, err := ws.ReadMessage()
		if err != nil {
			break
		}
		log.Printf("[WsHandler] Получено сообщение (тип %d): %s", messageType, string(message))
		var envelope struct {
			Type       string `json:"type"`
			InstanceID string `json:"instanceId"`
		}
		if err := json.Unmarshal(message, &envelope); err == nil {
			if envelope.Type == "JOIN_MATCH" {
				instanceID := envelope.InstanceID
				if instanceID == "" {
					continue
				}
				hex, err := repository.IsHexMatch(instanceID)
				if err != nil || !hex {
					b, _ := json.Marshal(map[string]any{"type": "MATCH_ERROR", "payload": map[string]string{"instanceId": instanceID, "error": "Match unavailable or uses a retired map"}})
					_ = client.write(b)
					continue
				}
				loadCtx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
				snapshot, loadErr := repository.LoadHexTestMatch(loadCtx, instanceID, client.userID)
				cancel()
				if loadErr != nil {
					b, _ := json.Marshal(map[string]any{"type": "MATCH_ERROR", "payload": map[string]string{"instanceId": instanceID, "error": "HEX match unavailable"}})
					_ = client.write(b)
					continue
				}
				clientsMu.Lock()
				client.instanceID = instanceID
				clientsMu.Unlock()
				presenceOpsMu.Lock()
				changed, presenceErr := repository.SetHexPresence(r.Context(), instanceID, client.userID, true)
				presenceOpsMu.Unlock()
				if presenceErr != nil {
					log.Printf("HEX reconnect %s user %d: %v", instanceID, client.userID, presenceErr)
				}
				if changed {
					broadcastPresence("PLAYER_RECONNECTED", instanceID, client.userID)
				}
				if b, marshalErr := json.Marshal(map[string]any{"type": "MATCH_UPDATE", "payload": snapshot}); marshalErr == nil {
					_ = client.write(b)
				}
				continue
			}
		}
	}

	ws.Close()
	shouldScheduleGrace := false
	instanceID := client.instanceID
	userIDInt := client.userID
	clientsMu.Lock()
	wasOnline := len(clients[client.userID]) > 0
	delete(clients[client.userID], client)
	shouldScheduleGrace = !hasMatchSocketLocked(client.userID, instanceID)
	lastSession := wasOnline && len(clients[client.userID]) == 0
	if lastSession {
		delete(clients, client.userID)
	}
	clientsMu.Unlock()
	if lastSession {
		publishOnlineLeave(userIDInt)
	}

	if shouldScheduleGrace && instanceID != "" && isAliveMatchPlayer(instanceID, userIDInt) {
		presenceOpsMu.Lock()
		clientsMu.Lock()
		stillDisconnected := !hasMatchSocketLocked(userIDInt, instanceID)
		clientsMu.Unlock()
		changed := false
		var err error
		if stillDisconnected {
			changed, err = repository.SetHexPresence(context.Background(), instanceID, userIDInt, false)
		}
		presenceOpsMu.Unlock()
		if err != nil {
			log.Printf("HEX disconnect %s user %d: %v", instanceID, userIDInt, err)
		}
		if changed {
			broadcastPresence("PLAYER_DISCONNECTED", instanceID, userIDInt)
		}
	}
}

var broadcastFn = func(message []byte) {
	var env broadcastEnvelope
	_ = json.Unmarshal(message, &env)

	// Try to extract message type for logging
	var raw map[string]interface{}
	_ = json.Unmarshal(message, &raw)
	msgType := "<unknown>"
	if t, ok := raw["type"].(string); ok {
		msgType = t
	}

	clientsMu.Lock()
	var targets []*Client
	for _, connections := range clients {
		for client := range connections {
			if env.Payload.SchemaVersion == 2 {
				var member bool
				if repository.DB == nil || repository.DB.QueryRow(`SELECT EXISTS(SELECT 1 FROM match_players WHERE instance_id=$1 AND user_id=$2)`, env.Payload.InstanceID, client.userID).Scan(&member) != nil || !member {
					continue
				}
			}
			// If client already has instanceID set — match normally.
			if client.instanceID != "" {
				if env.Payload.InstanceID != "" && client.instanceID != env.Payload.InstanceID {
					continue
				}
				targets = append(targets, client)
				continue
			}

			// client.instanceID is empty — as a fallback, if message targets a
			// specific instance, include the client when the user belongs to that match.
			if env.Payload.InstanceID != "" {
				var member bool
				if repository.DB != nil && repository.DB.QueryRow(`SELECT EXISTS(SELECT 1 FROM match_players WHERE instance_id=$1 AND user_id=$2)`, env.Payload.InstanceID, client.userID).Scan(&member) == nil && member {
					targets = append(targets, client)
					continue
				}
			}
			// otherwise skip
		}
	}
	clientsMu.Unlock()

	log.Printf("[Broadcast] type=%s instance=%s targets=%d", msgType, env.Payload.InstanceID, len(targets))
	// Теперь рассылка без lock!
	for _, client := range targets {
		if msgType == "MATCH_HEX_VISUAL" {
			payload, _ := raw["payload"].(map[string]interface{})
			ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
			snapshot, err := repository.LoadHexTestMatch(ctx, env.Payload.InstanceID, client.userID)
			cancel()
			if err != nil || !hexVisualVisibleToUser(snapshot.Cells, payload) {
				continue
			}
		}
		err := client.write(message)
		if err != nil {
			log.Printf("[Broadcast] write error user=%d err=%v", client.userID, err)
			// Если ошибка — безопасно удалить клиента (требуется ещё один lock)
			clientsMu.Lock()
			wasOnline := len(clients[client.userID]) > 0
			client.Conn.Close()
			delete(clients[client.userID], client)
			lastSession := wasOnline && len(clients[client.userID]) == 0
			if lastSession {
				delete(clients, client.userID)
			}
			clientsMu.Unlock()
			if lastSession {
				publishOnlineLeave(client.userID)
			}
		} else {
			// Log successful send for critical turn events to aid debugging
			if msgType == "SET_ACTIVE_USER" || msgType == "TURN_PASSED" {
				log.Printf("[Broadcast] sent user=%d instance=%s type=%s", client.userID, env.Payload.InstanceID, msgType)
			}
		}
	}
}

func hexVisualVisibleToUser(cells []hexmap.Cell, payload map[string]interface{}) bool {
	if payload == nil {
		return false
	}
	positions := []interface{}{}
	if payload["kind"] == "attack" {
		positions = append(positions, payload["attackerPosition"], payload["targetPosition"])
		if effects, ok := payload["effects"].([]interface{}); ok {
			for _, rawEffect := range effects {
				effect, ok := rawEffect.(map[string]interface{})
				if !ok || effect["positionAfter"] == nil {
					continue
				}
				point, ok := effect["positionAfter"].(map[string]interface{})
				if !ok {
					return false
				}
				positions = append(positions, map[string]interface{}{"q": point["x"], "r": point["y"]})
			}
		}
	} else if payload["kind"] == "move" {
		positions = append(positions, payload["position"])
		path, ok := payload["path"].([]interface{})
		if !ok {
			return false
		}
		positions = append(positions, path...)
	} else {
		return false
	}
	for _, raw := range positions {
		position, ok := raw.(map[string]interface{})
		if !ok {
			return false
		}
		q, qOK := position["q"].(float64)
		r, rOK := position["r"].(float64)
		if !qOK || !rOK {
			return false
		}
		visible := false
		for _, cell := range cells {
			if cell.Position.Q == int(q) && cell.Position.R == int(r) && cell.Visibility == "visible" {
				visible = true
				break
			}
		}
		if !visible {
			return false
		}
	}
	return true
}

func Broadcast(msg []byte) {
	broadcastFn(msg)
}

// IsUserOnline returns true when user currently has an active WebSocket connection.
func IsUserOnline(userID int) bool {
	if userID == 0 {
		return false
	}
	return GetUserActivityStatus(userID) != "offline"
}

// MarkUserHTTPActive updates lobby presence for users interacting via authenticated HTTP endpoints.
func MarkUserHTTPActive(userID int) {
	if userID == 0 {
		return
	}
	httpPresenceMu.Lock()
	httpPresence[userID] = time.Now()
	httpPresenceMu.Unlock()
}

// GetUserActivityStatus returns one of: in_match, in_lobby, offline.
func GetUserActivityStatus(userID int) string {
	if userID == 0 {
		return "offline"
	}

	clientsMu.Lock()
	connections := clients[userID]
	var client *Client
	for current := range connections {
		client = current
		if current.instanceID != "" {
			break
		}
	}
	ok := client != nil
	clientsMu.Unlock()

	if !ok {
		httpPresenceMu.Lock()
		lastSeen, hasHTTP := httpPresence[userID]
		httpPresenceMu.Unlock()
		if hasHTTP && time.Since(lastSeen) <= httpPresenceTTL {
			return "in_lobby"
		}
		return "offline"
	}

	if client.instanceID != "" {
		return "in_match"
	}

	return "in_lobby"
}
