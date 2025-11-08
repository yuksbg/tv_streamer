package web

import (
	"encoding/json"
	"net/http"
	"time"
	"tv_streamer/helpers/logs"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
	"github.com/sirupsen/logrus"
)

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	// Allow all origins for now (adjust in production)
	CheckOrigin: func(r *http.Request) bool {
		return true
	},
}

// handleWebSocket handles WebSocket connections at /api/ws
func handleWebSocket(c *gin.Context) {
	logger := logs.GetLogger().WithFields(logrus.Fields{
		"module":    "web",
		"handler":   "handleWebSocket",
		"client_ip": c.ClientIP(),
	})

	logger.Info("WebSocket connection request received")

	// Upgrade HTTP connection to WebSocket
	conn, err := upgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		logger.WithError(err).Error("Failed to upgrade connection to WebSocket")
		return
	}

	logger.Info("✓ WebSocket connection established")

	// Get the WebSocket hub
	hub := GetWebSocketHub()

	// Create and register the client (this also starts the write pump)
	client := hub.NewClient(conn)

	// Send welcome message through the send channel
	welcomeMsg := map[string]interface{}{
		"type":    "connection",
		"status":  "connected",
		"message": "Connected to TV Streamer WebSocket API",
	}
	if err := client.SendJSON(welcomeMsg); err != nil {
		logger.WithError(err).Warn("Failed to send welcome message")
	}

	// Handle client disconnection
	defer func() {
		hub.UnregisterClient(client)
		logger.Info("WebSocket connection closed")
	}()

	// Configure connection for reading
	conn.SetReadDeadline(time.Now().Add(pongWait))
	conn.SetReadLimit(maxMessageSize)
	conn.SetPongHandler(func(string) error {
		conn.SetReadDeadline(time.Now().Add(pongWait))
		return nil
	})

	// Read loop - keep connection alive and handle messages from client
	for {
		messageType, message, err := conn.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				logger.WithError(err).Warn("WebSocket connection closed unexpectedly")
			} else {
				logger.Debug("WebSocket connection closed normally")
			}
			break
		}

		// Process messages received from client
		if len(message) > 0 && messageType == websocket.TextMessage {
			logger.WithFields(logrus.Fields{
				"message_type": messageType,
				"message":      string(message),
			}).Debug("Received message from WebSocket client")

			// Parse message to determine type
			handleClientMessage(client, message, logger)
		}
	}
}

// handleClientMessage routes incoming WebSocket messages to appropriate handlers
func handleClientMessage(client *Client, message []byte, logger *logrus.Entry) {
	// Parse the message to determine its type
	var baseMsg struct {
		Type string `json:"type"`
	}

	if err := json.Unmarshal(message, &baseMsg); err != nil {
		logger.WithError(err).Warn("Failed to parse message type")
		client.SendJSON(map[string]interface{}{
			"type":    "error",
			"message": "Invalid message format",
		})
		return
	}

	// Route message based on type
	switch baseMsg.Type {
	case "upload_init":
		var msg WSUploadInitMessage
		if err := json.Unmarshal(message, &msg); err != nil {
			logger.WithError(err).Warn("Failed to parse upload_init message")
			return
		}
		handleUploadInit(client, msg)

	case "upload_chunk":
		var msg WSUploadChunkMessage
		if err := json.Unmarshal(message, &msg); err != nil {
			logger.WithError(err).Warn("Failed to parse upload_chunk message")
			return
		}
		handleUploadChunk(client, msg)

	case "upload_complete":
		var msg WSUploadCompleteMessage
		if err := json.Unmarshal(message, &msg); err != nil {
			logger.WithError(err).Warn("Failed to parse upload_complete message")
			return
		}
		handleUploadComplete(client, msg)

	default:
		logger.WithField("message_type", baseMsg.Type).Debug("Unknown message type")
	}
}
