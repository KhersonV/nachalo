//====================================
// gameservice/handlers/ws_handler.go
//====================================

package handlers

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"
	"strconv"
	"sync"
	"time"

	"gameservice/internal/hexmap"
	"gameservice/models"
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

	disconnectGraceMu sync.Mutex
	disconnectGrace   = make(map[string]*time.Timer)
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

func graceKey(instanceID string, userID int) string {
	return instanceID + ":" + strconv.Itoa(userID)
}

func cancelReconnectGrace(instanceID string, userID int) bool {
	key := graceKey(instanceID, userID)
	disconnectGraceMu.Lock()
	timer, ok := disconnectGrace[key]
	if ok {
		delete(disconnectGrace, key)
	}
	disconnectGraceMu.Unlock()
	if !ok {
		return false
	}
	return timer.Stop()
}

func scheduleReconnectGrace(instanceID string, userID int) {
	if instanceID == "" || userID == 0 {
		return
	}
	// Square retains its legacy process-local grace timer. HEX uses the
	// persisted disconnect timestamp and the shared deadline worker.
	if hex, err := repository.IsHexMatch(instanceID); err == nil && hex {
		return
	}

	key := graceKey(instanceID, userID)

	disconnectGraceMu.Lock()
	if oldTimer, exists := disconnectGrace[key]; exists {
		oldTimer.Stop()
	}
	var timer *time.Timer
	timer = time.AfterFunc(reconnectGracePeriod, func() {
		disconnectGraceMu.Lock()
		current, ok := disconnectGrace[key]
		if !ok || current != timer {
			disconnectGraceMu.Unlock()
			return
		}
		delete(disconnectGrace, key)
		disconnectGraceMu.Unlock()

		// Если за 3 минуты не переподключился — "молния" = смертельный урон.
		p, err := repository.GetMatchPlayerByID(instanceID, userID)
		if err != nil || p == nil {
			return
		}
		if p.Health <= 0 {
			return
		}

		log.Printf("[WsHandler] reconnect grace expired: userID=%d instance=%s -> lightning death", userID, instanceID)
		handlePlayerDeath(instanceID, p, 0, false)
	})
	disconnectGrace[key] = timer
	disconnectGraceMu.Unlock()
}

func isAliveMatchPlayer(instanceID string, userID int) bool {
	if instanceID == "" || userID == 0 {
		return false
	}
	if hex, err := repository.IsHexMatch(instanceID); err == nil && hex {
		var alive bool
		err := repository.DB.QueryRow(`SELECT EXISTS(SELECT 1 FROM match_players p JOIN matches m ON m.instance_id=p.instance_id WHERE p.instance_id=$1 AND p.user_id=$2 AND p.health>0 AND m.finished_at IS NULL AND m.turn_order @> to_jsonb($2::integer))`, instanceID, userID).Scan(&alive)
		return err == nil && alive
	}
	p, err := repository.GetMatchPlayerByID(instanceID, userID)
	if err != nil || p == nil {
		return false
	}
	return p.Health > 0
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
	if !ok {
		http.Error(w, "user_id not found in token", http.StatusUnauthorized)
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
	if qInstance := r.URL.Query().Get("instanceId"); qInstance != "" {
		client.instanceID = qInstance
	}

	clientsMu.Lock()
	if clients[client.userID] == nil {
		clients[client.userID] = make(map[*Client]struct{})
	}
	clients[client.userID][client] = struct{}{}
	clientsMu.Unlock()
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
				// Always update instanceID from client message and reply with MATCH_UPDATE.
				clientsMu.Lock()
				prevInstance := client.instanceID
				if envelope.InstanceID != "" {
					client.instanceID = envelope.InstanceID
				}
				clientsMu.Unlock()
				if client.instanceID != "" {
					if hex, err := repository.IsHexMatch(client.instanceID); err == nil && hex {
						presenceOpsMu.Lock()
						changed, err := repository.SetHexPresence(r.Context(), client.instanceID, client.userID, true)
						presenceOpsMu.Unlock()
						if err != nil {
							log.Printf("HEX reconnect %s user %d: %v", client.instanceID, client.userID, err)
						}
						if changed {
							broadcastPresence("PLAYER_RECONNECTED", client.instanceID, client.userID)
						}
					} else if cancelReconnectGrace(client.instanceID, client.userID) {
						broadcastPresence("PLAYER_RECONNECTED", client.instanceID, client.userID)
					}
				}
				log.Printf("[WsHandler] client %d joined match %s (prev=%s)", client.userID, envelope.InstanceID, prevInstance)

				// >>> Отправить MATCH_UPDATE <<<
				matchResp, err := BuildMatchResponse(client.instanceID)
				if err == nil {
					wsMsg := struct {
						Type    string         `json:"type"`
						Payload *MatchResponse `json:"payload"`
					}{
						Type:    "MATCH_UPDATE",
						Payload: matchResp,
					}
					if b, err := json.Marshal(wsMsg); err == nil {
						client.write(b)
					}
				} else {
					log.Printf("[WsHandler] Не удалось собрать матч для %d: %v", client.userID, err)
					if errors.Is(err, models.ErrUnsupportedGeometry) {
						if os.Getenv("HEX_TEST_MATCH_ENABLED") == "true" || os.Getenv("HEX_MATCH_ENABLED") == "true" {
							loadCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
							snapshot, hexErr := repository.LoadHexTestMatch(loadCtx, client.instanceID, client.userID)
							cancel()
							if hexErr == nil {
								if b, marshalErr := json.Marshal(map[string]interface{}{"type": "MATCH_UPDATE", "payload": snapshot}); marshalErr == nil {
									_ = client.write(b)
								}
								continue
							}
						}
						b, _ := json.Marshal(map[string]interface{}{
							"type":    "MATCH_ERROR",
							"payload": map[string]string{"instanceId": client.instanceID, "error": err.Error()},
						})
						_ = client.write(b)
						continue
					}
					if allStats, winnerType, winnerID, serr := buildAllPlayersGameStatsFromStored(client.instanceID); serr == nil {
						endedMsg := matchEndedBroadcastResponse{
							Type: "MATCH_ENDED",
							Payload: matchEndedBroadcastResult{
								InstanceID: client.instanceID,
								WinnerType: winnerType,
								WinnerID:   winnerID,
								Stats:      allStats,
							},
						}
						if b, marshalErr := json.Marshal(endedMsg); marshalErr == nil {
							_ = client.write(b)
						}
					} else {
						endedMsg := map[string]interface{}{
							"type": "MATCH_ENDED",
							"payload": map[string]interface{}{
								"instanceId": client.instanceID,
							},
						}
						if b, marshalErr := json.Marshal(endedMsg); marshalErr == nil {
							_ = client.write(b)
						}
					}
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
	delete(clients[client.userID], client)
	shouldScheduleGrace = !hasMatchSocketLocked(client.userID, instanceID)
	if len(clients[client.userID]) == 0 {
		delete(clients, client.userID)
	}
	clientsMu.Unlock()

	if shouldScheduleGrace && instanceID != "" && isAliveMatchPlayer(instanceID, userIDInt) {
		if hex, err := repository.IsHexMatch(instanceID); err == nil && hex {
			presenceOpsMu.Lock()
			clientsMu.Lock()
			stillDisconnected := !hasMatchSocketLocked(userIDInt, instanceID)
			clientsMu.Unlock()
			changed := false
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
		} else {
			broadcastPresence("PLAYER_DISCONNECTED", instanceID, userIDInt)
			scheduleReconnectGrace(instanceID, userIDInt)
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
				if p, err := repository.GetMatchPlayerByID(env.Payload.InstanceID, client.userID); err == nil && p != nil {
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
			client.Conn.Close()
			delete(clients[client.userID], client)
			if len(clients[client.userID]) == 0 {
				delete(clients, client.userID)
			}
			clientsMu.Unlock()
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
