package websocket

import (
	"context"
	"sync"

	"CampusMonitorAPI/internal/logger"
)

// Message defines the generic structure for WS communication
type Message struct {
	Type    string      `json:"type"`
	Payload interface{} `json:"payload"`
}

type Hub struct {
	clients    map[*Client]bool
	broadcast  chan Message
	register   chan *Client
	unregister chan *Client
	log        *logger.Logger
	mu         sync.RWMutex
}

func NewHub(log *logger.Logger) *Hub {
	return &Hub{
		broadcast:  make(chan Message),
		register:   make(chan *Client),
		unregister: make(chan *Client),
		clients:    make(map[*Client]bool),
		log:        log,
	}
}

// Run starts the hub logic in a goroutine. It listens for context cancellation for clean shutdown.
func (h *Hub) Run(ctx context.Context) {
	h.log.Info("WebSocket Hub started")
	for {
		select {
		case <-ctx.Done():
			h.log.Info("WebSocket Hub shutting down...")
			return
		case client := <-h.register:
			h.mu.Lock()
			h.clients[client] = true
			h.mu.Unlock()
			h.log.Info("New WS Client connected. Total: %d", len(h.clients))
		case client := <-h.unregister:
			h.mu.Lock()
			if _, ok := h.clients[client]; ok {
				delete(h.clients, client)
				close(client.send)
			}
			h.mu.Unlock()
		case message := <-h.broadcast:
			h.mu.RLock()
			for client := range h.clients {
				select {
				case client.send <- message:
				default:
					close(client.send)
					delete(h.clients, client)
				}
			}
			h.mu.RUnlock()
		}
	}
}

// Broadcast sends a message to all connected clients
func (h *Hub) Broadcast(msgType string, payload interface{}) {
	h.broadcast <- Message{
		Type:    msgType,
		Payload: payload,
	}
}
