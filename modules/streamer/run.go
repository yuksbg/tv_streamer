package streamer

import (
	"tv_streamer/helpers"
	"tv_streamer/helpers/logs"

	"github.com/sirupsen/logrus"
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

	logger.Info("Checking for existing videos in video files path...")
	config := helpers.GetConfig()

	// Scan video directory and add files to queue if available
	if config.App.VideoFilesPath != "" {
		count, err := ScanAndAddVideos(config.App.VideoFilesPath, []string{".ts", ".mp4", ".mkv", ".avi", ".mov"})
		if err != nil {
			logger.WithError(err).Warn("Failed to scan video directory")
		} else {
			logger.WithFields(logrus.Fields{
				"videos_added": count,
				"directory":    config.App.VideoFilesPath,
			}).Info("✓ Initial video scan completed")
		}
	} else {
		logger.Warn("No video_files_path configured, skipping initial scan")
	}

	logger.Info("========================================")
	logger.Info("✓ TV Streaming Service Started Successfully")
	logger.Info("========================================")
}
