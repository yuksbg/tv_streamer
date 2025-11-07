package web

import (
	"encoding/json"
	"sync"
	"tv_streamer/helpers/logs"

	"github.com/gorilla/websocket"
	"github.com/sirupsen/logrus"
)

// WebSocket message types
type WSMessage struct {
	Type string `json:"type"`
}

type WSCurrentlyPlayingMessage struct {
	Type        string `json:"type"`
	FileID      string `json:"file_id"`
	StartedTime int64  `json:"started_time"`
}

// WebSocketHub manages WebSocket connections
type WebSocketHub struct {
	mu          sync.RWMutex
	clients     map[*websocket.Conn]bool
	broadcast   chan []byte
	register    chan *websocket.Conn
	unregister  chan *websocket.Conn
	logger      *logrus.Entry
}

var (
	wsHub     *WebSocketHub
	wsHubOnce sync.Once
)

// GetWebSocketHub returns the singleton WebSocketHub instance
func GetWebSocketHub() *WebSocketHub {
	wsHubOnce.Do(func() {
		logger := logs.GetLogger().WithField("module", "websocket")
		wsHub = &WebSocketHub{
			clients:    make(map[*websocket.Conn]bool),
			broadcast:  make(chan []byte, 256),
			register:   make(chan *websocket.Conn),
			unregister: make(chan *websocket.Conn),
			logger:     logger,
		}

		// Start the hub goroutine
		go wsHub.run()

		logger.Info("✓ WebSocket Hub initialized")
	})
	return wsHub
}

// run handles hub operations
func (h *WebSocketHub) run() {
	h.logger.Info("WebSocket Hub started")

	for {
		select {
		case client := <-h.register:
			h.mu.Lock()
			h.clients[client] = true
			h.mu.Unlock()
			h.logger.WithField("total_clients", len(h.clients)).Info("WebSocket client connected")

		case client := <-h.unregister:
			h.mu.Lock()
			if _, ok := h.clients[client]; ok {
				delete(h.clients, client)
				client.Close()
				h.logger.WithField("total_clients", len(h.clients)).Info("WebSocket client disconnected")
			}
			h.mu.Unlock()

		case message := <-h.broadcast:
			h.mu.RLock()
			for client := range h.clients {
				err := client.WriteMessage(websocket.TextMessage, message)
				if err != nil {
					h.logger.WithError(err).Warn("Failed to send message to WebSocket client")
					// Schedule for removal
					go func(c *websocket.Conn) {
						h.unregister <- c
					}(client)
				}
			}
			h.mu.RUnlock()
		}
	}
}

// RegisterClient adds a new WebSocket client
func (h *WebSocketHub) RegisterClient(conn *websocket.Conn) {
	h.register <- conn
}

// UnregisterClient removes a WebSocket client
func (h *WebSocketHub) UnregisterClient(conn *websocket.Conn) {
	h.unregister <- conn
}

// BroadcastStructuredLog sends a structured log message to all connected clients
func (h *WebSocketHub) BroadcastStructuredLog(logData *logs.StructuredLogMessage) {
	data, err := json.Marshal(logData)
	if err != nil {
		h.logger.WithError(err).Error("Failed to marshal structured log message")
		return
	}

	select {
	case h.broadcast <- data:
		// Message queued successfully
	default:
		// Broadcast channel is full, log warning
		h.logger.Warn("Broadcast channel full, dropping log message")
	}
}

// BroadcastCurrentlyPlaying sends currently playing info to all connected clients
func (h *WebSocketHub) BroadcastCurrentlyPlaying(fileID string, startedTime int64) {
	msg := WSCurrentlyPlayingMessage{
		Type:        "currently_playing",
		FileID:      fileID,
		StartedTime: startedTime,
	}

	data, err := json.Marshal(msg)
	if err != nil {
		h.logger.WithError(err).Error("Failed to marshal currently_playing message")
		return
	}

	select {
	case h.broadcast <- data:
		h.logger.WithFields(logrus.Fields{
			"file_id":      fileID,
			"started_time": startedTime,
		}).Debug("Broadcasting currently_playing event")
	default:
		// Broadcast channel is full, log warning
		h.logger.Warn("Broadcast channel full, dropping currently_playing message")
	}
}

// GetClientCount returns the number of connected clients
func (h *WebSocketHub) GetClientCount() int {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return len(h.clients)
}
