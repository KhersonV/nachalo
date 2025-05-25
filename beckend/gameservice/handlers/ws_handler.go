//====================================
// gameservice/handlers/ws_handler.go
//====================================

package handlers

import (
	"log"
	"net/http"
	"sync"
	"encoding/json"

	"github.com/gorilla/websocket"
)

var (
	upgrader = websocket.Upgrader{
		// Разрешаем все запросы (для разработки)
		CheckOrigin: func(r *http.Request) bool { return true },
	}
	clients   = make(map[*Client]bool)
	clientsMu sync.Mutex
)

// Client представляет подключённого клиента WebSocket.
type Client struct {
    Conn       *websocket.Conn
    instanceID string
}

type broadcastEnvelope struct {
    Payload struct {
        InstanceID string `json:"instanceId"`
    } `json:"payload"`
}

func WsHandler(w http.ResponseWriter, r *http.Request) {
	ws, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		//log.Println("[WsHandler] Ошибка апгрейда:", err)
		return
	}
	client := &Client{Conn: ws}
	clientsMu.Lock()
	clients[client] = true
	clientsMu.Unlock()
	//log.Println("[WsHandler] Новый клиент подключён.")

	// Чтение сообщений от клиента
	for {
		messageType, message, err := ws.ReadMessage()
		if err != nil {
			//log.Printf("[WsHandler] Клиент отключился: %v", err)
			break
		}
		log.Printf("[WsHandler] Получено сообщение (тип %d): %s", messageType, string(message))
		var envelope struct {
        Type       string `json:"type"`
        InstanceID string `json:"instanceId"`
    }
        if err := json.Unmarshal(message, &envelope); err == nil {
        if envelope.Type == "JOIN_MATCH" {
            // логируем и сохраняем только один раз, при первом JOIN
            if client.instanceID == "" {
                client.instanceID = envelope.InstanceID
                log.Printf("[WsHandler] client joined match %s", envelope.InstanceID)
            }
            continue
        }
    }
	}

	// Завершаем соединение и удаляем клиента
	ws.Close()
	clientsMu.Lock()
	delete(clients, client)
	clientsMu.Unlock()
	//log.Println("[WsHandler] Клиент удалён из списка подключённых")
}

var broadcastFn = func(message []byte) {
	var env broadcastEnvelope
    _ = json.Unmarshal(message, &env)  // если невалидный JSON или нет поля – игнорируем

	clientsMu.Lock()
	defer clientsMu.Unlock()
	//log.Printf("[Broadcast] Рассылка сообщения: %s", string(message))
	for client := range clients {

		 if client.instanceID == "" || (env.Payload.InstanceID != "" && client.instanceID != env.Payload.InstanceID) {
            continue
        }
		err := client.Conn.WriteMessage(websocket.TextMessage, message)
		if err != nil {
			//log.Printf("[Broadcast] Ошибка при рассылке клиенту: %v", err)
			client.Conn.Close()
			delete(clients, client)
		}
	}
}

func Broadcast(msg []byte) {
    broadcastFn(msg)
}
