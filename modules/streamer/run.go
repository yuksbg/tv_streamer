package streamer

import (
	"tv_streamer/helpers/logs"
)

// StartStream initializes and starts the TV streaming service
func StartStream() {
	logger := logs.GetLogger().WithField("module", "streamer")
	logger.Info("========================================")
	logger.Info("Starting TV Streaming Service...")
	logger.Info("========================================")

	// Get persistent player instance
	player := GetPersistentPlayer()

	// Start the player
	if err := player.Start(); err != nil {
		logger.WithError(err).Fatal("Failed to start streaming player")
		return
	}

	logger.Info("========================================")
	logger.Info("✓ TV Streaming Service Started Successfully")
	logger.Info("========================================")
}
