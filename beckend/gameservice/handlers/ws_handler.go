

//====================================
//gameservice/handlers/ws_handler.go
//====================================

package handlers

import (
	"log"
	"net/http"
	"sync"

	"github.com/gorilla/websocket"
)

var (
	upgrader = websocket.Upgrader{
		CheckOrigin: func(r *http.Request) bool { return true },
	}
	clients   = make(map[*Client]bool)
	clientsMu sync.Mutex
)

type Client struct {
	Conn *websocket.Conn
}

func WsHandler(w http.ResponseWriter, r *http.Request) {
	ws, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Println("Ошибка апгрейда:", err)
		return
	}
	client := &Client{Conn: ws}
	clientsMu.Lock()
	clients[client] = true
	clientsMu.Unlock()
	log.Println("Новый клиент подключён.")

	for {
		_, _, err := ws.ReadMessage()
		if err != nil {
			log.Println("Клиент отключился:", err)
			ws.Close()
			clientsMu.Lock()
			delete(clients, client)
			clientsMu.Unlock()
			break
		}
	}
}

func Broadcast(message []byte) {
	clientsMu.Lock()
	defer clientsMu.Unlock()
	for client := range clients {
		err := client.Conn.WriteMessage(websocket.TextMessage, message)
		if err != nil {
			log.Println("Ошибка при рассылке:", err)
			client.Conn.Close()
			delete(clients, client)
		}
	}
}
