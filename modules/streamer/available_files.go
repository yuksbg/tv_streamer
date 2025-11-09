package streamer

import (
	"fmt"
	"tv_streamer/helpers"
	"tv_streamer/helpers/logs"
	"tv_streamer/modules/streamer/models"

	"github.com/sirupsen/logrus"
)

// GetAvailableFiles returns all files from the availible_files table
func GetAvailableFiles() ([]models.AvailableFiles, error) {
	logger := logs.GetLogger().WithFields(logrus.Fields{
		"module":   "streamer",
		"function": "GetAvailableFiles",
	})

	logger.Debug("Fetching all available files...")

	var files []models.AvailableFiles
	err := helpers.GetXORM().
		OrderBy("added_time DESC").
		Find(&files)

	if err != nil {
		logger.WithError(err).Error("Failed to fetch available files")
		return nil, fmt.Errorf("failed to fetch available files: %w", err)
	}

	logger.WithField("total_files", len(files)).Info("✓ Available files fetched successfully")

	return files, nil
}

// UpdateFFProbeData updates the ffprobe data for a file
func UpdateFFProbeData(fileID string, ffprobeData string, videoLength int64) error {
	logger := logs.GetLogger().WithFields(logrus.Fields{
		"module":   "streamer",
		"function": "UpdateFFProbeData",
		"file_id":  fileID,
	})

	logger.Debug("Updating ffprobe data...")

	_, err := helpers.GetXORM().
		Where("file_id = ?", fileID).
		Cols("ffprobe_data", "video_length").
		Update(&models.AvailableFiles{
			FFProbeData: ffprobeData,
			VideoLength: videoLength,
		})

	if err != nil {
		logger.WithError(err).Error("Failed to update ffprobe data")
		return fmt.Errorf("failed to update ffprobe data: %w", err)
	}

	logger.WithField("file_id", fileID).Info("✓ FFProbe data updated successfully")

	return nil
}

// UpdateFileDescription updates the description for a file
func UpdateFileDescription(fileID string, description string) error {
	logger := logs.GetLogger().WithFields(logrus.Fields{
		"module":   "streamer",
		"function": "UpdateFileDescription",
		"file_id":  fileID,
	})

	logger.Debug("Updating file description...")

	affected, err := helpers.GetXORM().
		Where("file_id = ?", fileID).
		Cols("description").
		Update(&models.AvailableFiles{
			Description: description,
		})

	if err != nil {
		logger.WithError(err).Error("Failed to update file description")
		return fmt.Errorf("failed to update file description: %w", err)
	}

	if affected == 0 {
		logger.WithField("file_id", fileID).Warn("File not found")
		return fmt.Errorf("file not found")
	}

	logger.WithField("file_id", fileID).Info("✓ File description updated successfully")

	return nil
}
