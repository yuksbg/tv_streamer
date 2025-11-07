package streamer

import "sync"

// Broadcaster is an interface for broadcasting events
type Broadcaster interface {
	BroadcastCurrentlyPlaying(fileID string, startedTime int64)
}

var (
	broadcaster   Broadcaster
	broadcasterMu sync.RWMutex
)

// SetBroadcaster sets the broadcaster for the streamer module
func SetBroadcaster(b Broadcaster) {
	broadcasterMu.Lock()
	defer broadcasterMu.Unlock()
	broadcaster = b
}

// GetBroadcaster gets the current broadcaster
func GetBroadcaster() Broadcaster {
	broadcasterMu.RLock()
	defer broadcasterMu.RUnlock()
	return broadcaster
}

// BroadcastCurrentlyPlaying broadcasts currently playing info (helper function)
func BroadcastCurrentlyPlaying(fileID string, startedTime int64) {
	b := GetBroadcaster()
	if b != nil {
		b.BroadcastCurrentlyPlaying(fileID, startedTime)
	}
}
