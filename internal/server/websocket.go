package server

import (
	"encoding/json"
	"net/http"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool {
		return true // Allow local studio connections
	},
}

// Hub maintains active WebSocket client connections and broadcasts real-time events.
type Hub struct {
	mu         sync.Mutex
	clients    map[*websocket.Conn]struct{}
	broadcast  chan []byte
	register   chan *websocket.Conn
	unregister chan *websocket.Conn
	done       chan struct{}
}

// NewHub creates a new WebSocket hub.
func NewHub() *Hub {
	return &Hub{
		clients:    make(map[*websocket.Conn]struct{}),
		broadcast:  make(chan []byte, 256),
		register:   make(chan *websocket.Conn),
		unregister: make(chan *websocket.Conn),
		done:       make(chan struct{}),
	}
}

// Run starts the event pump goroutine.
func (h *Hub) Run() {
	for {
		select {
		case <-h.done:
			return
		case client := <-h.register:
			h.mu.Lock()
			h.clients[client] = struct{}{}
			h.mu.Unlock()
		case client := <-h.unregister:
			h.mu.Lock()
			if _, ok := h.clients[client]; ok {
				delete(h.clients, client)
				_ = client.Close()
			}
			h.mu.Unlock()
		case message := <-h.broadcast:
			h.mu.Lock()
			for client := range h.clients {
				_ = client.SetWriteDeadline(time.Now().Add(2 * time.Second))
				if err := client.WriteMessage(websocket.TextMessage, message); err != nil {
					_ = client.Close()
					delete(h.clients, client)
				}
			}
			h.mu.Unlock()
		}
	}
}

// BroadcastEvent formats and broadcasts an event object.
func (h *Hub) BroadcastEvent(eventType string, data interface{}) {
	payload := map[string]interface{}{
		"type":      eventType,
		"data":      data,
		"timestamp": time.Now().Format(time.RFC3339),
	}
	bytes, err := json.Marshal(payload)
	if err == nil {
		select {
		case h.broadcast <- bytes:
		default:
		}
	}
}

// HandleWebSocket upgrades incoming HTTP connections to WebSocket.
func (h *Hub) HandleWebSocket(w http.ResponseWriter, r *http.Request) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		return
	}

	h.register <- conn

	go func() {
		defer func() {
			h.unregister <- conn
		}()
		for {
			_, _, err := conn.ReadMessage()
			if err != nil {
				break
			}
		}
	}()
}

// Close shuts down the hub.
func (h *Hub) Close() {
	close(h.done)
	h.mu.Lock()
	for client := range h.clients {
		_ = client.Close()
	}
	h.clients = make(map[*websocket.Conn]struct{})
	h.mu.Unlock()
}
