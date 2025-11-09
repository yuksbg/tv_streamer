package streamer

import (
	"crypto/md5"
	"fmt"
	"os"
	"time"
	"tv_streamer/helpers"
	"tv_streamer/helpers/logs"
	"tv_streamer/modules/streamer/models"

	"github.com/sirupsen/logrus"
)

// AddToSchedule adds a video file to the schedule
func AddToSchedule(filepath string) error {
	logger := logs.GetLogger().WithFields(logrus.Fields{
		"module":   "streamer",
		"function": "AddToSchedule",
	})

	logger.WithField("filepath", filepath).Info("Adding video to schedule...")

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

	logger.WithField("file_size", fileInfo.Size()).Debug("File validated successfully")

	// Generate file ID (MD5 of filepath)
	fileID := fmt.Sprintf("%x", md5.Sum([]byte(filepath)))
	logger.WithField("file_id", fileID).Debug("Generated file ID")

	// Check if file is already in availible_files
	var availFile models.AvailableFiles
	has, err := helpers.GetXORM().Where("file_id = ?", fileID).Get(&availFile)
	if err != nil {
		logger.WithError(err).Error("Failed to query available files")
		return fmt.Errorf("database error: %w", err)
	}

	if !has {
		// Get ffprobe data
		ffprobeData, err := GetFFProbeData(filepath)
		if err != nil {
			logger.WithError(err).Warn("Failed to get ffprobe data, using empty JSON")
			ffprobeData = "{}"
		}

		// Parse video duration
		videoLength := ParseDuration(ffprobeData)

		// Add to availible_files table
		availFile = models.AvailableFiles{
			FileID:      fileID,
			FilePath:    filepath,
			FileSize:    fileInfo.Size(),
			VideoLength: videoLength,
			AddedTime:   time.Now().Unix(),
			FFProbeData: ffprobeData,
		}

		if _, err := helpers.GetXORM().Insert(&availFile); err != nil {
			logger.WithError(err).Error("Failed to insert into available files")
			return fmt.Errorf("failed to add to available files: %w", err)
		}

		logger.WithFields(logrus.Fields{
			"file_id":      fileID,
			"video_length": videoLength,
		}).Info("✓ Added to available files with ffprobe data")
	} else {
		logger.WithField("file_id", fileID).Debug("File already exists in available files")
	}

	// Check if already in schedule
	var existingSchedule models.Schedule
	has, err = helpers.GetXORM().Where("file_id = ?", fileID).Get(&existingSchedule)
	if err != nil {
		logger.WithError(err).Error("Failed to query schedule")
		return fmt.Errorf("database error: %w", err)
	}

	if has {
		logger.WithField("file_id", fileID).Info("File already exists in schedule")
		return nil
	}

	// Get next schedule position
	var maxPosition int
	_, err = helpers.GetXORM().SQL("SELECT COALESCE(MAX(schedule_position), -1) FROM schedule").Get(&maxPosition)
	if err != nil {
		logger.WithError(err).Error("Failed to get max schedule position")
		return fmt.Errorf("failed to get schedule position: %w", err)
	}

	nextPosition := maxPosition + 1
	logger.WithField("position", nextPosition).Debug("Calculated next schedule position")

	// Add to schedule
	scheduleItem := &models.Schedule{
		FileID:           fileID,
		FilePath:         filepath,
		SchedulePosition: nextPosition,
		IsCurrent:        0,
		AddedAt:          time.Now().Unix(),
	}

	if _, err := helpers.GetXORM().Insert(scheduleItem); err != nil {
		logger.WithError(err).Error("Failed to insert into schedule")
		return fmt.Errorf("failed to add to schedule: %w", err)
	}

	logger.WithFields(logrus.Fields{
		"schedule_id":       scheduleItem.ID,
		"file_id":           fileID,
		"filepath":          filepath,
		"schedule_position": nextPosition,
	}).Info("✓ Video added to schedule successfully")

	return nil
}

// GetNextFromSchedule gets the next video from the schedule (with endless loop)
func GetNextFromSchedule() (*models.Schedule, error) {
	logger := logs.GetLogger().WithFields(logrus.Fields{
		"module":   "streamer",
		"function": "GetNextFromSchedule",
	})

	logger.Debug("Getting next video from schedule...")

	// First, check if there's a current item
	var current models.Schedule
	has, err := helpers.GetXORM().Where("is_current = ?", 1).Get(&current)
	if err != nil {
		return nil, fmt.Errorf("failed to get current schedule item: %w", err)
	}

	var nextItem models.Schedule
	if has {
		// Get the next item after current
		hasNext, err := helpers.GetXORM().
			Where("schedule_position > ?", current.SchedulePosition).
			OrderBy("schedule_position ASC").
			Get(&nextItem)

		if err != nil {
			return nil, fmt.Errorf("failed to query next schedule item: %w", err)
		}

		if hasNext {
			// Found next item, unmark current and mark next as current
			_, err = helpers.GetXORM().Where("is_current = ?", 1).
				Cols("is_current").
				Update(&models.Schedule{IsCurrent: 0})
			if err != nil {
				logger.WithError(err).Warn("Failed to unmark current item")
			}

			nextItem.MarkAsCurrent()
			_, err = helpers.GetXORM().ID(nextItem.ID).Cols("is_current").Update(&nextItem)
			if err != nil {
				logger.WithError(err).Warn("Failed to mark next item as current")
			}

			logger.WithFields(logrus.Fields{
				"schedule_id":       nextItem.ID,
				"file_id":           nextItem.FileID,
				"filepath":          nextItem.FilePath,
				"schedule_position": nextItem.SchedulePosition,
			}).Info("✓ Next video retrieved from schedule")

			return &nextItem, nil
		}

		// No next item found, loop back to the beginning
		logger.Info("Reached end of schedule, looping back to beginning")
		hasFirst, err := helpers.GetXORM().
			OrderBy("schedule_position ASC").
			Get(&nextItem)

		if err != nil {
			return nil, fmt.Errorf("failed to query first schedule item: %w", err)
		}

		if !hasFirst {
			logger.Warn("No items in schedule")
			return nil, nil
		}

		// Unmark current and mark first as current
		_, err = helpers.GetXORM().Where("is_current = ?", 1).
			Cols("is_current").
			Update(&models.Schedule{IsCurrent: 0})
		if err != nil {
			logger.WithError(err).Warn("Failed to unmark current item")
		}

		nextItem.MarkAsCurrent()
		_, err = helpers.GetXORM().ID(nextItem.ID).Cols("is_current").Update(&nextItem)
		if err != nil {
			logger.WithError(err).Warn("Failed to mark first item as current")
		}

		logger.WithFields(logrus.Fields{
			"schedule_id":       nextItem.ID,
			"file_id":           nextItem.FileID,
			"filepath":          nextItem.FilePath,
			"schedule_position": nextItem.SchedulePosition,
		}).Info("✓ Looped back to first video in schedule")

		return &nextItem, nil
	}

	// No current item, start from the beginning
	logger.Info("No current item, starting from beginning of schedule")
	hasFirst, err := helpers.GetXORM().
		OrderBy("schedule_position ASC").
		Get(&nextItem)

	if err != nil {
		return nil, fmt.Errorf("failed to query first schedule item: %w", err)
	}

	if !hasFirst {
		logger.Warn("No items in schedule")
		return nil, nil
	}

	// Mark as current
	nextItem.MarkAsCurrent()
	_, err = helpers.GetXORM().ID(nextItem.ID).Cols("is_current").Update(&nextItem)
	if err != nil {
		logger.WithError(err).Warn("Failed to mark item as current")
	}

	logger.WithFields(logrus.Fields{
		"schedule_id":       nextItem.ID,
		"file_id":           nextItem.FileID,
		"filepath":          nextItem.FilePath,
		"schedule_position": nextItem.SchedulePosition,
	}).Info("✓ First video retrieved from schedule")

	return &nextItem, nil
}

// GetSchedule returns all items in the schedule
func GetSchedule() ([]models.Schedule, error) {
	logger := logs.GetLogger().WithFields(logrus.Fields{
		"module":   "streamer",
		"function": "GetSchedule",
	})

	logger.Debug("Fetching schedule...")

	var schedule []models.Schedule
	err := helpers.GetXORM().
		OrderBy("schedule_position ASC").
		Find(&schedule)

	if err != nil {
		logger.WithError(err).Error("Failed to fetch schedule")
		return nil, fmt.Errorf("failed to fetch schedule: %w", err)
	}

	logger.WithField("total_items", len(schedule)).Info("✓ Schedule fetched successfully")

	return schedule, nil
}

// ResetSchedulePosition resets the schedule position to the beginning
func ResetSchedulePosition() error {
	logger := logs.GetLogger().WithFields(logrus.Fields{
		"module":   "streamer",
		"function": "ResetSchedulePosition",
	})

	logger.Info("Resetting schedule position...")

	// Unmark all items
	_, err := helpers.GetXORM().
		Cols("is_current").
		Update(&models.Schedule{IsCurrent: 0})

	if err != nil {
		logger.WithError(err).Error("Failed to reset schedule position")
		return fmt.Errorf("failed to reset schedule position: %w", err)
	}

	logger.Info("✓ Schedule position reset successfully")
	return nil
}

// RemoveFromSchedule removes a video from the schedule by file_id
func RemoveFromSchedule(fileID string) error {
	logger := logs.GetLogger().WithFields(logrus.Fields{
		"module":   "streamer",
		"function": "RemoveFromSchedule",
		"file_id":  fileID,
	})

	logger.Info("Removing video from schedule...")

	result, err := helpers.GetXORM().Where("file_id = ?", fileID).Delete(&models.Schedule{})
	if err != nil {
		logger.WithError(err).Error("Failed to remove from schedule")
		return fmt.Errorf("failed to remove from schedule: %w", err)
	}

	if result == 0 {
		logger.Warn("File not found in schedule")
		return fmt.Errorf("file not found in schedule")
	}

	logger.WithField("deleted_count", result).Info("✓ Video removed from schedule")
	return nil
}

// ClearSchedule removes all items from the schedule
func ClearSchedule() (int64, error) {
	logger := logs.GetLogger().WithFields(logrus.Fields{
		"module":   "streamer",
		"function": "ClearSchedule",
	})

	logger.Info("Clearing schedule...")

	result, err := helpers.GetXORM().Where("1=1").Delete(&models.Schedule{})
	if err != nil {
		logger.WithError(err).Error("Failed to clear schedule")
		return 0, fmt.Errorf("failed to clear schedule: %w", err)
	}

	logger.WithField("deleted_count", result).Info("✓ Schedule cleared")
	return result, nil
}
