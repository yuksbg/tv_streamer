package logs

import (
	"github.com/sirupsen/logrus"
)

// StructuredLogMessage represents a structured log entry for WebSocket broadcast
type StructuredLogMessage struct {
	Type      string                 `json:"type"`
	Level     string                 `json:"level"`
	Message   string                 `json:"message"`
	Timestamp string                 `json:"timestamp"`
	Fields    map[string]interface{} `json:"fields,omitempty"`
}

// WebSocketBroadcaster is an interface to avoid circular dependencies
type WebSocketBroadcaster interface {
	BroadcastStructuredLog(logData *StructuredLogMessage)
}

// WebSocketHook is a logrus hook that broadcasts log messages to WebSocket clients
type WebSocketHook struct {
	broadcaster WebSocketBroadcaster
}

// NewWebSocketHook creates a new WebSocket hook
func NewWebSocketHook(broadcaster WebSocketBroadcaster) *WebSocketHook {
	return &WebSocketHook{
		broadcaster: broadcaster,
	}
}

// Levels returns the log levels this hook should be triggered for
func (hook *WebSocketHook) Levels() []logrus.Level {
	return logrus.AllLevels
}

// Fire is called when a log event is triggered
func (hook *WebSocketHook) Fire(entry *logrus.Entry) error {
	if hook.broadcaster == nil {
		return nil
	}

	// Create structured log message
	logMsg := &StructuredLogMessage{
		Type:      "logs",
		Level:     entry.Level.String(),
		Message:   entry.Message,
		Timestamp: entry.Time.Format("2006-01-02T15:04:05.000Z07:00"),
	}

	// Add fields if present
	if len(entry.Data) > 0 {
		logMsg.Fields = make(map[string]interface{})
		for k, v := range entry.Data {
			// Convert to JSON-serializable format
			logMsg.Fields[k] = v
		}
	}

	// Broadcast to WebSocket clients
	hook.broadcaster.BroadcastStructuredLog(logMsg)

	return nil
}

// SetBroadcaster sets the WebSocket broadcaster
func (hook *WebSocketHook) SetBroadcaster(broadcaster WebSocketBroadcaster) {
	hook.broadcaster = broadcaster
}
