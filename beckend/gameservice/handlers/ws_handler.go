
//====================================
// gameservice/handlers/ws_handler.go
//====================================

package handlers

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"sync"
	"time"

	"github.com/golang-jwt/jwt/v4"
	"github.com/gorilla/websocket"
)

var (
	upgrader = websocket.Upgrader{
		CheckOrigin: func(r *http.Request) bool { return true },
	}
	clients   = make(map[int]*Client)
	clientsMu sync.Mutex
)

// Client представляет подключённого клиента WebSocket.
type Client struct {
	Conn       *websocket.Conn
	instanceID string
	userID     int
}

type broadcastEnvelope struct {
	Payload struct {
		InstanceID string `json:"instanceId"`
	} `json:"payload"`
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
	client := &Client{Conn: ws, userID: int(userID)}

	clientsMu.Lock()
	if oldClient, ok := clients[client.userID]; ok {
		log.Printf("[WsHandler] Закрываем старый сокет для userID=%d, old instance=%s", oldClient.userID, oldClient.instanceID)
		// Set a short read deadline so the old goroutine exits promptly, then
		// send the close frame (code 1000). Do NOT force-close TCP immediately —
		// that would cause the close frame to be lost and the client would receive
		// 1006 Abnormal Closure, triggering a reconnect loop.
		oldClient.Conn.SetReadDeadline(time.Now().Add(2 * time.Second))
		_ = oldClient.Conn.WriteControl(
			websocket.CloseMessage,
			websocket.FormatCloseMessage(websocket.CloseNormalClosure, "replaced by new connection"),
			time.Now().Add(time.Second),
		)
		delete(clients, client.userID)
	}
	clients[client.userID] = client
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
            if client.instanceID == "" {
                client.instanceID = envelope.InstanceID
                log.Printf("[WsHandler] client %d joined match %s", client.userID, envelope.InstanceID)
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
                        client.Conn.WriteMessage(websocket.TextMessage, b)
                    }
                } else {
                    log.Printf("[WsHandler] Не удалось собрать матч для %d: %v", client.userID, err)
					endedMsg := map[string]interface{}{
						"type": "MATCH_ENDED",
						"payload": map[string]interface{}{
							"instanceId": client.instanceID,
						},
					}
					if b, marshalErr := json.Marshal(endedMsg); marshalErr == nil {
						_ = client.Conn.WriteMessage(websocket.TextMessage, b)
					}
                }
            }
				continue
			}
		}
	}

	ws.Close()
	clientsMu.Lock()
	if current, ok := clients[client.userID]; ok && current == client {
		delete(clients, client.userID)
	}
	clientsMu.Unlock()
}


var broadcastFn = func(message []byte) {
    var env broadcastEnvelope
    _ = json.Unmarshal(message, &env)

    clientsMu.Lock()
    var targets []*Client
    for _, client := range clients {
        if client.instanceID == "" || (env.Payload.InstanceID != "" && client.instanceID != env.Payload.InstanceID) {
            continue
        }
        targets = append(targets, client)
    }
    clientsMu.Unlock()

    // Теперь рассылка без lock!
    for _, client := range targets {
        err := client.Conn.WriteMessage(websocket.TextMessage, message)
        if err != nil {
            // Если ошибка — безопасно удалить клиента (требуется ещё один lock)
            clientsMu.Lock()
            client.Conn.Close()
			if current, ok := clients[client.userID]; ok && current == client {
				delete(clients, client.userID)
			}
            clientsMu.Unlock()
        }
    }
}


func Broadcast(msg []byte) {
	broadcastFn(msg)
}
