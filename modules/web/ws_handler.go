package web

import (
	"net/http"
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

	// Register the client
	hub.RegisterClient(conn)

	// Send welcome message
	welcomeMsg := map[string]interface{}{
		"type":    "connection",
		"status":  "connected",
		"message": "Connected to TV Streamer WebSocket API",
	}
	if err := conn.WriteJSON(welcomeMsg); err != nil {
		logger.WithError(err).Warn("Failed to send welcome message")
	}

	// Handle client disconnection
	// The client will be unregistered when the connection closes
	defer func() {
		hub.UnregisterClient(conn)
		logger.Info("WebSocket connection closed")
	}()

	// Keep connection alive and handle ping/pong
	for {
		// Read messages from client (for ping/pong or future client->server messages)
		messageType, message, err := conn.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				logger.WithError(err).Warn("WebSocket connection closed unexpectedly")
			} else {
				logger.Debug("WebSocket connection closed normally")
			}
			break
		}

		// Handle ping messages
		if messageType == websocket.PingMessage {
			if err := conn.WriteMessage(websocket.PongMessage, nil); err != nil {
				logger.WithError(err).Warn("Failed to send pong message")
				break
			}
		}

		// Log any messages received from client (for debugging)
		if len(message) > 0 {
			logger.WithFields(logrus.Fields{
				"message_type": messageType,
				"message":      string(message),
			}).Debug("Received message from WebSocket client")
		}
	}
}
