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

// NormalizeFilePath converts a filepath to an absolute, cleaned path to ensure
// consistent file_id generation regardless of how the path is specified
func NormalizeFilePath(path string) (string, error) {
	// Convert to absolute path
	absPath, err := filepath.Abs(path)
	if err != nil {
		return "", fmt.Errorf("failed to get absolute path: %w", err)
	}

	// Clean the path (removes redundant separators, resolves . and ..)
	cleanPath := filepath.Clean(absPath)

	return cleanPath, nil
}

// AddToAvailableFiles adds a file to the availible_files table
// This is the only place where files should be added to availible_files
// Returns the file_id of the added (or existing) file
func AddToAvailableFiles(filePath string) (string, error) {
	logger := logs.GetLogger().WithFields(logrus.Fields{
		"module":   "streamer",
		"function": "AddToAvailableFiles",
	})

	logger.WithField("filepath", filePath).Debug("Adding file to available files...")

	// Normalize filepath first
	normalizedPath, err := NormalizeFilePath(filePath)
	if err != nil {
		logger.WithError(err).Error("Failed to normalize filepath")
		return "", fmt.Errorf("failed to normalize filepath: %w", err)
	}

	filePath = normalizedPath
	logger.WithField("normalized_filepath", filePath).Debug("Filepath normalized")

	// Generate file ID (MD5 of normalized filepath)
	fileID := fmt.Sprintf("%x", md5.Sum([]byte(filePath)))
	logger.WithField("file_id", fileID).Debug("Generated file ID")

	// Check if file already exists in availible_files
	var existingFile models.AvailableFiles
	has, err := helpers.GetXORM().Where("file_id = ?", fileID).Get(&existingFile)
	if err != nil {
		logger.WithError(err).Error("Failed to query available files")
		return "", fmt.Errorf("database error: %w", err)
	}

	if has {
		logger.WithField("file_id", fileID).Debug("File already exists in available files")
		return fileID, nil
	}

	// Get file info from filesystem
	fileInfo, err := os.Stat(filePath)
	if err != nil {
		logger.WithError(err).Error("File does not exist on filesystem")
		return "", fmt.Errorf("file does not exist: %w", err)
	}

	// Get ffprobe data
	ffprobeData, err := GetFFProbeData(filePath)
	if err != nil {
		logger.WithError(err).Warn("Failed to get ffprobe data, using empty JSON")
		ffprobeData = "{}"
	}

	// Parse video duration
	videoLength := ParseDuration(ffprobeData)

	// Insert into availible_files table
	newFile := models.AvailableFiles{
		FileID:      fileID,
		FilePath:    filePath,
		FileSize:    fileInfo.Size(),
		VideoLength: videoLength,
		AddedTime:   time.Now().Unix(),
		FFProbeData: ffprobeData,
	}

	if _, err := helpers.GetXORM().Insert(&newFile); err != nil {
		logger.WithError(err).Error("Failed to insert into available files")
		return "", fmt.Errorf("failed to add to available files: %w", err)
	}

	logger.WithFields(logrus.Fields{
		"file_id":      fileID,
		"video_length": videoLength,
		"file_size":    fileInfo.Size(),
	}).Info("✓ File added to available files")

	return fileID, nil
}
