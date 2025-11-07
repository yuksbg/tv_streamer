package web

import (
	"encoding/json"
	"sync"
	"time"
	"tv_streamer/helpers/logs"

	"github.com/gorilla/websocket"
	"github.com/sirupsen/logrus"
)

const (
	// Time allowed to write a message to the peer
	writeWait = 10 * time.Second

	// Time allowed to read the next pong message from the peer
	pongWait = 60 * time.Second

	// Send pings to peer with this period (must be less than pongWait)
	pingPeriod = (pongWait * 9) / 10

	// Maximum message size allowed from peer
	maxMessageSize = 512

	// Size of the send channel buffer
	sendBufferSize = 256
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

// Client represents a WebSocket client with its own send channel
type Client struct {
	hub  *WebSocketHub
	conn *websocket.Conn
	send chan []byte
}

// WebSocketHub manages WebSocket connections
type WebSocketHub struct {
	mu          sync.RWMutex
	clients     map[*Client]bool
	broadcast   chan []byte
	register    chan *Client
	unregister  chan *Client
	logger      *logrus.Entry
}

var (
	wsHub     *WebSocketHub
	wsHubOnce sync.Once
)

// SendJSON marshals v to JSON and sends it to the client
func (c *Client) SendJSON(v interface{}) error {
	data, err := json.Marshal(v)
	if err != nil {
		return err
	}

	select {
	case c.send <- data:
		return nil
	default:
		return websocket.ErrCloseSent
	}
}

// writePump pumps messages from the send channel to the websocket connection
// This ensures only one goroutine writes to the connection at a time
func (c *Client) writePump() {
	ticker := time.NewTicker(pingPeriod)
	defer func() {
		ticker.Stop()
		c.conn.Close()
	}()

	for {
		select {
		case message, ok := <-c.send:
			c.conn.SetWriteDeadline(time.Now().Add(writeWait))
			if !ok {
				// Hub closed the channel
				c.conn.WriteMessage(websocket.CloseMessage, []byte{})
				return
			}

			if err := c.conn.WriteMessage(websocket.TextMessage, message); err != nil {
				c.hub.logger.WithError(err).Warn("Failed to write message to WebSocket client")
				return
			}

		case <-ticker.C:
			c.conn.SetWriteDeadline(time.Now().Add(writeWait))
			if err := c.conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				c.hub.logger.WithError(err).Debug("Failed to send ping to WebSocket client")
				return
			}
		}
	}
}

// GetWebSocketHub returns the singleton WebSocketHub instance
func GetWebSocketHub() *WebSocketHub {
	wsHubOnce.Do(func() {
		logger := logs.GetLogger().WithField("module", "websocket")
		wsHub = &WebSocketHub{
			clients:    make(map[*Client]bool),
			broadcast:  make(chan []byte, 256),
			register:   make(chan *Client),
			unregister: make(chan *Client),
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
				close(client.send)
				h.logger.WithField("total_clients", len(h.clients)).Info("WebSocket client disconnected")
			}
			h.mu.Unlock()

		case message := <-h.broadcast:
			h.mu.RLock()
			for client := range h.clients {
				select {
				case client.send <- message:
					// Message queued successfully
				default:
					// Client's send buffer is full, unregister it
					h.logger.Warn("Client send buffer full, closing connection")
					go func(c *Client) {
						h.unregister <- c
					}(client)
				}
			}
			h.mu.RUnlock()
		}
	}
}

// NewClient creates a new Client and starts its write pump
func (h *WebSocketHub) NewClient(conn *websocket.Conn) *Client {
	client := &Client{
		hub:  h,
		conn: conn,
		send: make(chan []byte, sendBufferSize),
	}

	// Register the client with the hub
	h.register <- client

	// Start the write pump goroutine
	go client.writePump()

	return client
}

// UnregisterClient removes a WebSocket client
func (h *WebSocketHub) UnregisterClient(client *Client) {
	h.unregister <- client
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
