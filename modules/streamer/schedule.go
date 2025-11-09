package streamer

import (
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

	// Check if file exists in availible_files (must be scanned first)
	// Query by filepath to get the actual file_id from the database
	var availFile models.AvailableFiles
	has, err := helpers.GetXORM().Where("filepath = ?", filepath).Get(&availFile)
	if err != nil {
		logger.WithError(err).Error("Failed to query available files")
		return fmt.Errorf("database error: %w", err)
	}

	if !has {
		logger.WithField("filepath", filepath).Error("File not found in available files")
		return fmt.Errorf("file must be scanned and added to available files before adding to schedule (filepath: %s)", filepath)
	}

	// Use the file_id from the database (don't recalculate it)
	fileID := availFile.FileID
	logger.WithField("file_id", fileID).Debug("File found in available files")

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

// RemoveFromScheduleByID removes a specific schedule item by its ID
func RemoveFromScheduleByID(scheduleID int64) error {
	logger := logs.GetLogger().WithFields(logrus.Fields{
		"module":      "streamer",
		"function":    "RemoveFromScheduleByID",
		"schedule_id": scheduleID,
	})

	logger.Info("Removing schedule item by ID...")

	// Get the item first to find its position
	var item models.Schedule
	has, err := helpers.GetXORM().ID(scheduleID).Get(&item)
	if err != nil {
		logger.WithError(err).Error("Failed to query schedule item")
		return fmt.Errorf("database error: %w", err)
	}

	if !has {
		logger.Warn("Schedule item not found")
		return fmt.Errorf("schedule item not found")
	}

	deletedPosition := item.SchedulePosition

	// Delete the item
	result, err := helpers.GetXORM().ID(scheduleID).Delete(&models.Schedule{})
	if err != nil {
		logger.WithError(err).Error("Failed to remove schedule item")
		return fmt.Errorf("failed to remove schedule item: %w", err)
	}

	if result == 0 {
		logger.Warn("Schedule item not found")
		return fmt.Errorf("schedule item not found")
	}

	// Reorder remaining items to fill the gap
	_, err = helpers.GetXORM().
		Where("schedule_position > ?", deletedPosition).
		Decr("schedule_position", 1).
		Update(&models.Schedule{})

	if err != nil {
		logger.WithError(err).Warn("Failed to reorder schedule items after deletion")
		// Don't return error, item was already deleted
	}

	logger.Info("✓ Schedule item removed successfully")
	return nil
}

// UpdateSchedulePosition updates the position of a schedule item
func UpdateSchedulePosition(scheduleID int64, newPosition int) error {
	logger := logs.GetLogger().WithFields(logrus.Fields{
		"module":       "streamer",
		"function":     "UpdateSchedulePosition",
		"schedule_id":  scheduleID,
		"new_position": newPosition,
	})

	logger.Info("Updating schedule position...")

	// Validate new position is non-negative
	if newPosition < 0 {
		return fmt.Errorf("position must be non-negative")
	}

	// Get the item to be moved
	var item models.Schedule
	has, err := helpers.GetXORM().ID(scheduleID).Get(&item)
	if err != nil {
		logger.WithError(err).Error("Failed to query schedule item")
		return fmt.Errorf("database error: %w", err)
	}

	if !has {
		logger.Warn("Schedule item not found")
		return fmt.Errorf("schedule item not found")
	}

	oldPosition := item.SchedulePosition

	// If position hasn't changed, nothing to do
	if oldPosition == newPosition {
		logger.Info("Position unchanged, skipping update")
		return nil
	}

	// Get total count to validate new position
	totalCount, err := helpers.GetXORM().Count(&models.Schedule{})
	if err != nil {
		logger.WithError(err).Error("Failed to count schedule items")
		return fmt.Errorf("database error: %w", err)
	}

	// Adjust newPosition if it's beyond the end
	if newPosition >= int(totalCount) {
		newPosition = int(totalCount) - 1
		logger.WithField("adjusted_position", newPosition).Info("Adjusted position to max")
	}

	// Move items to make space for the new position
	if newPosition < oldPosition {
		// Moving up (to lower position number)
		// Shift items down between newPosition and oldPosition
		_, err = helpers.GetXORM().
			Where("schedule_position >= ? AND schedule_position < ?", newPosition, oldPosition).
			Incr("schedule_position", 1).
			Update(&models.Schedule{})
	} else {
		// Moving down (to higher position number)
		// Shift items up between oldPosition and newPosition
		_, err = helpers.GetXORM().
			Where("schedule_position > ? AND schedule_position <= ?", oldPosition, newPosition).
			Decr("schedule_position", 1).
			Update(&models.Schedule{})
	}

	if err != nil {
		logger.WithError(err).Error("Failed to reorder schedule items")
		return fmt.Errorf("failed to reorder items: %w", err)
	}

	// Update the item's position
	item.SchedulePosition = newPosition
	_, err = helpers.GetXORM().ID(scheduleID).Cols("schedule_position").Update(&item)
	if err != nil {
		logger.WithError(err).Error("Failed to update item position")
		return fmt.Errorf("failed to update position: %w", err)
	}

	logger.WithFields(logrus.Fields{
		"old_position": oldPosition,
		"new_position": newPosition,
	}).Info("✓ Schedule position updated successfully")

	return nil
}

// BulkReorderSchedule updates positions for multiple schedule items in one operation
// orderMap is a map of schedule_id -> new_position
func BulkReorderSchedule(orderMap map[int64]int) error {
	logger := logs.GetLogger().WithFields(logrus.Fields{
		"module":     "streamer",
		"function":   "BulkReorderSchedule",
		"item_count": len(orderMap),
	})

	logger.Info("Bulk reordering schedule items...")

	if len(orderMap) == 0 {
		return fmt.Errorf("no items provided for reordering")
	}

	// Start a transaction for atomic updates
	session := helpers.GetXORM().NewSession()
	defer session.Close()

	if err := session.Begin(); err != nil {
		logger.WithError(err).Error("Failed to start transaction")
		return fmt.Errorf("failed to start transaction: %w", err)
	}

	// Validate all schedule IDs exist
	for scheduleID := range orderMap {
		var item models.Schedule
		has, err := session.ID(scheduleID).Get(&item)
		if err != nil {
			session.Rollback()
			logger.WithError(err).WithField("schedule_id", scheduleID).Error("Failed to query schedule item")
			return fmt.Errorf("database error: %w", err)
		}
		if !has {
			session.Rollback()
			logger.WithField("schedule_id", scheduleID).Warn("Schedule item not found")
			return fmt.Errorf("schedule item %d not found", scheduleID)
		}
	}

	// Update all positions
	for scheduleID, newPosition := range orderMap {
		if newPosition < 0 {
			session.Rollback()
			return fmt.Errorf("position must be non-negative for schedule_id %d", scheduleID)
		}

		_, err := session.ID(scheduleID).Cols("schedule_position").Update(&models.Schedule{
			SchedulePosition: newPosition,
		})
		if err != nil {
			session.Rollback()
			logger.WithError(err).WithField("schedule_id", scheduleID).Error("Failed to update position")
			return fmt.Errorf("failed to update position for schedule_id %d: %w", scheduleID, err)
		}
	}

	// Commit transaction
	if err := session.Commit(); err != nil {
		logger.WithError(err).Error("Failed to commit transaction")
		return fmt.Errorf("failed to commit transaction: %w", err)
	}

	logger.WithField("updated_count", len(orderMap)).Info("✓ Schedule bulk reordered successfully")
	return nil
}
