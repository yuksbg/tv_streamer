package streamer

import (
	"crypto/md5"
	"fmt"
	"os"
	"path/filepath"
	"time"
	"tv_streamer/helpers"
	"tv_streamer/helpers/logs"
	"tv_streamer/modules/streamer/models"

	"github.com/sirupsen/logrus"
)

// AddToQueue adds a video file to the streaming queue
func AddToQueue(filepath string, isAd bool) error {
	logger := logs.GetLogger().WithFields(logrus.Fields{
		"module":   "streamer",
		"function": "AddToQueue",
	})

	logger.WithFields(logrus.Fields{
		"filepath": filepath,
		"is_ad":    isAd,
	}).Info("Adding video to queue...")

	// Normalize filepath to prevent duplicate entries
	normalizedPath, err := NormalizeFilePath(filepath)
	if err != nil {
		logger.WithError(err).Error("Failed to normalize filepath")
		return fmt.Errorf("failed to normalize filepath: %w", err)
	}

	// Use normalized path from here on
	filepath = normalizedPath
	logger.WithField("normalized_filepath", filepath).Debug("Filepath normalized")

	// Check if file exists
	fileInfo, err := os.Stat(filepath)
	if err != nil {
		logger.WithError(err).Error("File does not exist")
		return fmt.Errorf("file does not exist: %w", err)
	}

	logger.WithFields(logrus.Fields{
		"file_size": fileInfo.Size(),
		"file_mode": fileInfo.Mode().String(),
	}).Debug("File validated successfully")

	// Generate file ID (MD5 of filepath)
	fileID := fmt.Sprintf("%x", md5.Sum([]byte(filepath)))
	logger.WithField("file_id", fileID).Debug("Generated file ID")

	// Check if file exists in availible_files (must be scanned first)
	var availFile models.AvailableFiles
	has, err := helpers.GetXORM().Where("file_id = ?", fileID).Get(&availFile)
	if err != nil {
		logger.WithError(err).Error("Failed to query available files")
		return fmt.Errorf("database error: %w", err)
	}

	if !has {
		logger.WithField("file_id", fileID).Error("File not found in available files")
		return fmt.Errorf("file must be scanned and added to available files before adding to queue (file_id: %s)", fileID)
	}

	logger.WithField("file_id", fileID).Debug("File found in available files")

	// Get next queue position
	var maxPosition int
	_, err = helpers.GetXORM().SQL("SELECT COALESCE(MAX(queue_position), 0) FROM video_queue").Get(&maxPosition)
	if err != nil {
		logger.WithError(err).Error("Failed to get max queue position")
		return fmt.Errorf("failed to get queue position: %w", err)
	}

	nextPosition := maxPosition + 1
	logger.WithField("position", nextPosition).Debug("Calculated next queue position")

	// Add to video_queue
	queueItem := &models.VideoQueue{
		FileID:        fileID,
		FilePath:      filepath,
		AddedAt:       time.Now().Unix(),
		Played:        0,
		QueuePosition: nextPosition,
		IsAd:          0,
	}

	if isAd {
		queueItem.IsAd = 1
	}

	if _, err := helpers.GetXORM().Insert(queueItem); err != nil {
		logger.WithError(err).Error("Failed to insert into video queue")
		return fmt.Errorf("failed to add to queue: %w", err)
	}

	logger.WithFields(logrus.Fields{
		"queue_id":       queueItem.ID,
		"file_id":        fileID,
		"filepath":       filepath,
		"queue_position": nextPosition,
		"is_ad":          isAd,
	}).Info("✓ Video added to queue successfully")

	return nil
}

// GetQueue returns all items in the queue
func GetQueue() ([]models.VideoQueue, error) {
	logger := logs.GetLogger().WithFields(logrus.Fields{
		"module":   "streamer",
		"function": "GetQueue",
	})

	logger.Debug("Fetching queue...")

	var queue []models.VideoQueue
	err := helpers.GetXORM().
		OrderBy("queue_position ASC, id ASC").
		Find(&queue)

	if err != nil {
		logger.WithError(err).Error("Failed to fetch queue")
		return nil, fmt.Errorf("failed to fetch queue: %w", err)
	}

	logger.WithFields(logrus.Fields{
		"total_items":   len(queue),
		"unplayed":      countUnplayed(queue),
		"played":        len(queue) - countUnplayed(queue),
	}).Info("✓ Queue fetched successfully")

	return queue, nil
}

// GetPlayHistory returns recent play history
func GetPlayHistory(limit int) ([]models.PlayHistory, error) {
	logger := logs.GetLogger().WithFields(logrus.Fields{
		"module":   "streamer",
		"function": "GetPlayHistory",
		"limit":    limit,
	})

	logger.Debug("Fetching play history...")

	var history []models.PlayHistory
	err := helpers.GetXORM().
		OrderBy("started_at DESC").
		Limit(limit).
		Find(&history)

	if err != nil {
		logger.WithError(err).Error("Failed to fetch play history")
		return nil, fmt.Errorf("failed to fetch play history: %w", err)
	}

	logger.WithField("records_found", len(history)).Info("✓ Play history fetched successfully")

	return history, nil
}

// ClearPlayedFromQueue removes all played items from the queue
func ClearPlayedFromQueue() (int64, error) {
	logger := logs.GetLogger().WithFields(logrus.Fields{
		"module":   "streamer",
		"function": "ClearPlayedFromQueue",
	})

	logger.Info("Clearing played items from queue...")

	result, err := helpers.GetXORM().Where("played = ?", 1).Delete(&models.VideoQueue{})
	if err != nil {
		logger.WithError(err).Error("Failed to clear played items")
		return 0, fmt.Errorf("failed to clear played items: %w", err)
	}

	logger.WithField("deleted_count", result).Info("✓ Played items cleared from queue")

	return result, nil
}

// ScanAndAddVideos scans a directory for video files and adds them to the queue
func ScanAndAddVideos(directory string, extensions []string) (int, error) {
	logger := logs.GetLogger().WithFields(logrus.Fields{
		"module":     "streamer",
		"function":   "ScanAndAddVideos",
		"directory":  directory,
		"extensions": extensions,
	})

	logger.Info("Scanning directory for videos...")

	if extensions == nil || len(extensions) == 0 {
		extensions = []string{".ts", ".mp4", ".mkv", ".avi", ".mov"}
	}

	addedCount := 0
	err := filepath.Walk(directory, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			logger.WithError(err).WithField("path", path).Warn("Error accessing path")
			return nil // Continue walking
		}

		if info.IsDir() {
			return nil
		}

		// Check if file has valid extension
		ext := filepath.Ext(path)
		validExt := false
		for _, validExtension := range extensions {
			if ext == validExtension {
				validExt = true
				break
			}
		}

		if !validExt {
			return nil
		}

		// First, add to available_files table
		_, err := AddToAvailableFiles(path)
		if err != nil {
			logger.WithError(err).WithField("path", path).Warn("Failed to add video to available files")
			return nil // Continue walking
		}

		// Then add to queue
		if err := AddToQueue(path, false); err != nil {
			logger.WithError(err).WithField("path", path).Warn("Failed to add video to queue")
			return nil // Continue walking
		}

		addedCount++
		return nil
	})

	if err != nil {
		logger.WithError(err).Error("Error walking directory")
		return addedCount, fmt.Errorf("error walking directory: %w", err)
	}

	logger.WithFields(logrus.Fields{
		"added_count": addedCount,
		"directory":   directory,
	}).Info("✓ Directory scan completed")

	return addedCount, nil
}

// InjectAd adds an ad to the front of the queue
func InjectAd(filepath string) error {
	logger := logs.GetLogger().WithFields(logrus.Fields{
		"module":   "streamer",
		"function": "InjectAd",
		"filepath": filepath,
	})

	logger.Info("Injecting ad into queue...")

	// Normalize filepath to prevent duplicate entries
	normalizedPath, err := NormalizeFilePath(filepath)
	if err != nil {
		logger.WithError(err).Error("Failed to normalize filepath")
		return fmt.Errorf("failed to normalize filepath: %w", err)
	}

	// Use normalized path from here on
	filepath = normalizedPath
	logger.WithField("normalized_filepath", filepath).Debug("Filepath normalized")

	// Check if file exists
	fileInfo, err := os.Stat(filepath)
	if err != nil {
		logger.WithError(err).Error("Ad file does not exist")
		return fmt.Errorf("file does not exist: %w", err)
	}

	logger.WithField("file_size", fileInfo.Size()).Debug("Ad file validated")

	// Generate file ID
	fileID := fmt.Sprintf("%x", md5.Sum([]byte(filepath)))
	logger.WithField("file_id", fileID).Debug("Generated file ID")

	// Check if file exists in availible_files (must be scanned first)
	var availFile models.AvailableFiles
	has, err := helpers.GetXORM().Where("file_id = ?", fileID).Get(&availFile)
	if err != nil {
		logger.WithError(err).Error("Failed to query available files")
		return fmt.Errorf("database error: %w", err)
	}

	if !has {
		logger.WithField("file_id", fileID).Error("Ad file not found in available files")
		return fmt.Errorf("ad file must be scanned and added to available files before injecting (file_id: %s)", fileID)
	}

	logger.WithField("file_id", fileID).Debug("Ad file found in available files")

	// Shift all queue positions up by 1
	_, err = helpers.GetXORM().Exec("UPDATE video_queue SET queue_position = queue_position + 1 WHERE played = 0")
	if err != nil {
		logger.WithError(err).Error("Failed to shift queue positions")
		return fmt.Errorf("failed to shift queue positions: %w", err)
	}

	logger.Debug("Queue positions shifted for ad injection")

	// Insert ad at position 0 (front of queue)
	adItem := &models.VideoQueue{
		FileID:        fileID,
		FilePath:      filepath,
		AddedAt:       time.Now().Unix(),
		Played:        0,
		QueuePosition: 0,
		IsAd:          1,
	}

	if _, err := helpers.GetXORM().Insert(adItem); err != nil {
		logger.WithError(err).Error("Failed to inject ad into queue")
		return fmt.Errorf("failed to inject ad: %w", err)
	}

	logger.WithFields(logrus.Fields{
		"queue_id": adItem.ID,
		"file_id":  fileID,
		"filepath": filepath,
	}).Info("✓ Ad injected into queue successfully")

	return nil
}

// Helper function to count unplayed items
func countUnplayed(queue []models.VideoQueue) int {
	count := 0
	for _, item := range queue {
		if item.Played == 0 {
			count++
		}
	}
	return count
}
