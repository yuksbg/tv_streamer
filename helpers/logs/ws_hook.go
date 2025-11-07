package logs

import (
	"fmt"

	"github.com/sirupsen/logrus"
)

// WebSocketBroadcaster is an interface to avoid circular dependencies
type WebSocketBroadcaster interface {
	BroadcastLog(message string)
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

	// Format the log message
	message := fmt.Sprintf("[%s] %s", entry.Level.String(), entry.Message)

	// Add fields if present
	if len(entry.Data) > 0 {
		for k, v := range entry.Data {
			message = fmt.Sprintf("%s | %s=%v", message, k, v)
		}
	}

	// Broadcast to WebSocket clients
	hook.broadcaster.BroadcastLog(message)

	return nil
}

// SetBroadcaster sets the WebSocket broadcaster
func (hook *WebSocketHook) SetBroadcaster(broadcaster WebSocketBroadcaster) {
	hook.broadcaster = broadcaster
}
