package web

import (
	"net/http"
	"path/filepath"
	"strconv"
	"tv_streamer/helpers/logs"
	"tv_streamer/modules/streamer"
	"tv_streamer/modules/streamer/models"

	"github.com/gin-gonic/gin"
	"github.com/sirupsen/logrus"
)

// Response DTOs to maintain API compatibility with filepath
type QueueItemResponse struct {
	ID            int64  `json:"id"`
	FileID        string `json:"file_id"`
	FilePath      string `json:"filepath"`
	AddedAt       int64  `json:"added_at"`
	Played        int    `json:"played"`
	PlayedAt      int64  `json:"played_at"`
	QueuePosition int    `json:"queue_position"`
	IsAd          int    `json:"is_ad"`
}

type ScheduleItemResponse struct {
	ID               int64  `json:"id"`
	FileID           string `json:"file_id"`
	FilePath         string `json:"filepath"`
	SchedulePosition int    `json:"schedule_position"`
	IsCurrent        int    `json:"is_current"`
	AddedAt          int64  `json:"added_at"`
}

type PlayHistoryResponse struct {
	ID              int64  `json:"id"`
	FileID          string `json:"file_id"`
	Filename        string `json:"filename"`
	FilePath        string `json:"filepath"`
	StartedAt       int64  `json:"started_at"`
	FinishedAt      int64  `json:"finished_at"`
	DurationSeconds int64  `json:"duration_seconds"`
	IsAd            int    `json:"is_ad"`
	SkipRequested   int    `json:"skip_requested"`
}

// Helper functions to enrich models with filepath
func enrichQueueItem(item *models.VideoQueue) QueueItemResponse {
	filePath, _ := streamer.GetFilePathByID(item.FileID)
	return QueueItemResponse{
		ID:            item.ID,
		FileID:        item.FileID,
		FilePath:      filePath,
		AddedAt:       item.AddedAt,
		Played:        item.Played,
		PlayedAt:      item.PlayedAt,
		QueuePosition: item.QueuePosition,
		IsAd:          item.IsAd,
	}
}

func enrichScheduleItem(item *models.Schedule) ScheduleItemResponse {
	filePath, _ := streamer.GetFilePathByID(item.FileID)
	return ScheduleItemResponse{
		ID:               item.ID,
		FileID:           item.FileID,
		FilePath:         filePath,
		SchedulePosition: item.SchedulePosition,
		IsCurrent:        item.IsCurrent,
		AddedAt:          item.AddedAt,
	}
}

func enrichPlayHistory(item *models.PlayHistory) PlayHistoryResponse {
	filePath, _ := streamer.GetFilePathByID(item.FileID)
	filename := filepath.Base(filePath)
	return PlayHistoryResponse{
		ID:              item.ID,
		FileID:          item.FileID,
		Filename:        filename,
		FilePath:        filePath,
		StartedAt:       item.StartedAt,
		FinishedAt:      item.FinishedAt,
		DurationSeconds: item.DurationSeconds,
		IsAd:            item.IsAd,
		SkipRequested:   item.SkipRequested,
	}
}

// handleStreamNext skips to the next video in the queue
func handleStreamNext(c *gin.Context) {
	logger := logs.GetLogger().WithFields(logrus.Fields{
		"module":   "web",
		"handler":  "handleStreamNext",
		"client_ip": c.ClientIP(),
	})

	logger.Info("Received request to skip to next video")

	player := streamer.GetPersistentPlayer()
	if err := player.Skip(); err != nil {
		logger.WithError(err).Error("Failed to skip to next video")
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"error":   err.Error(),
		})
		return
	}

	logger.Info("✓ Successfully skipped to next video")
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "Skipped to next video",
	})
}

// handleStreamAdd adds a video to the queue
func handleStreamAdd(c *gin.Context) {
	logger := logs.GetLogger().WithFields(logrus.Fields{
		"module":   "web",
		"handler":  "handleStreamAdd",
		"client_ip": c.ClientIP(),
	})

	filepath := c.Query("file")
	if filepath == "" {
		logger.Warn("Missing 'file' parameter in request")
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"error":   "Missing 'file' parameter",
		})
		return
	}

	logger.WithField("filepath", filepath).Info("Received request to add video to queue")

	if err := streamer.AddToQueue(filepath, false); err != nil {
		logger.WithError(err).Error("Failed to add video to queue")
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"error":   err.Error(),
		})
		return
	}

	logger.WithField("filepath", filepath).Info("✓ Successfully added video to queue")
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "Video added to queue",
		"file":    filepath,
	})
}

// handleStreamQueue returns the current queue
func handleStreamQueue(c *gin.Context) {
	logger := logs.GetLogger().WithFields(logrus.Fields{
		"module":   "web",
		"handler":  "handleStreamQueue",
		"client_ip": c.ClientIP(),
	})

	logger.Debug("Received request to get queue")

	queue, err := streamer.GetQueue()
	if err != nil {
		logger.WithError(err).Error("Failed to get queue")
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"error":   err.Error(),
		})
		return
	}

	// Enrich queue items with filepath for API compatibility
	enrichedQueue := make([]QueueItemResponse, len(queue))
	for i, item := range queue {
		enrichedQueue[i] = enrichQueueItem(&item)
	}

	logger.WithField("queue_size", len(queue)).Info("✓ Successfully retrieved queue")
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"queue":   enrichedQueue,
		"count":   len(enrichedQueue),
	})
}

// handleStreamStatus returns the current player status
func handleStreamStatus(c *gin.Context) {
	logger := logs.GetLogger().WithFields(logrus.Fields{
		"module":   "web",
		"handler":  "handleStreamStatus",
		"client_ip": c.ClientIP(),
	})

	logger.Debug("Received request to get player status")

	player := streamer.GetPersistentPlayer()
	status := player.GetStatus()

	logger.WithField("running", status["running"]).Info("✓ Successfully retrieved player status")
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"status":  status,
	})
}

// handleInjectAd injects an ad at the front of the queue
func handleInjectAd(c *gin.Context) {
	logger := logs.GetLogger().WithFields(logrus.Fields{
		"module":   "web",
		"handler":  "handleInjectAd",
		"client_ip": c.ClientIP(),
	})

	filepath := c.Query("file")
	if filepath == "" {
		logger.Warn("Missing 'file' parameter in request")
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"error":   "Missing 'file' parameter",
		})
		return
	}

	logger.WithField("filepath", filepath).Info("Received request to inject ad")

	if err := streamer.InjectAd(filepath); err != nil {
		logger.WithError(err).Error("Failed to inject ad")
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"error":   err.Error(),
		})
		return
	}

	logger.WithField("filepath", filepath).Info("✓ Successfully injected ad")
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "Ad injected successfully",
		"file":    filepath,
	})
}

// handleStreamHistory returns play history
func handleStreamHistory(c *gin.Context) {
	logger := logs.GetLogger().WithFields(logrus.Fields{
		"module":   "web",
		"handler":  "handleStreamHistory",
		"client_ip": c.ClientIP(),
	})

	limitStr := c.DefaultQuery("limit", "50")
	limit, err := strconv.Atoi(limitStr)
	if err != nil || limit <= 0 {
		limit = 50
	}

	logger.WithField("limit", limit).Debug("Received request to get play history")

	history, err := streamer.GetPlayHistory(limit)
	if err != nil {
		logger.WithError(err).Error("Failed to get play history")
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"error":   err.Error(),
		})
		return
	}

	// Enrich history items with filepath and filename for API compatibility
	enrichedHistory := make([]PlayHistoryResponse, len(history))
	for i, item := range history {
		enrichedHistory[i] = enrichPlayHistory(&item)
	}

	logger.WithField("records_count", len(history)).Info("✓ Successfully retrieved play history")
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"history": enrichedHistory,
		"count":   len(enrichedHistory),
	})
}

// handleScanVideos scans a directory for videos and adds them to the queue
func handleScanVideos(c *gin.Context) {
	logger := logs.GetLogger().WithFields(logrus.Fields{
		"module":   "web",
		"handler":  "handleScanVideos",
		"client_ip": c.ClientIP(),
	})

	directory := c.Query("directory")
	if directory == "" {
		logger.Warn("Missing 'directory' parameter in request")
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"error":   "Missing 'directory' parameter",
		})
		return
	}

	logger.WithField("directory", directory).Info("Received request to scan directory")

	count, err := streamer.ScanAndAddVideos(directory, nil)
	if err != nil {
		logger.WithError(err).Error("Failed to scan directory")
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"error":   err.Error(),
		})
		return
	}

	logger.WithFields(logrus.Fields{
		"directory":    directory,
		"videos_added": count,
	}).Info("✓ Successfully scanned directory")

	c.JSON(http.StatusOK, gin.H{
		"success":      true,
		"message":      "Directory scanned successfully",
		"videos_added": count,
		"directory":    directory,
	})
}

// handleClearPlayed removes all played items from the queue
func handleClearPlayed(c *gin.Context) {
	logger := logs.GetLogger().WithFields(logrus.Fields{
		"module":   "web",
		"handler":  "handleClearPlayed",
		"client_ip": c.ClientIP(),
	})

	logger.Info("Received request to clear played items from queue")

	deletedCount, err := streamer.ClearPlayedFromQueue()
	if err != nil {
		logger.WithError(err).Error("Failed to clear played items")
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"error":   err.Error(),
		})
		return
	}

	logger.WithField("deleted_count", deletedCount).Info("✓ Successfully cleared played items")
	c.JSON(http.StatusOK, gin.H{
		"success":       true,
		"message":       "Played items cleared from queue",
		"deleted_count": deletedCount,
	})
}

// handleScheduleAdd adds a video to the schedule
func handleScheduleAdd(c *gin.Context) {
	logger := logs.GetLogger().WithFields(logrus.Fields{
		"module":    "web",
		"handler":   "handleScheduleAdd",
		"client_ip": c.ClientIP(),
	})

	filepath := c.Query("file")
	if filepath == "" {
		logger.Warn("Missing 'file' parameter in request")
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"error":   "Missing 'file' parameter",
		})
		return
	}

	logger.WithField("filepath", filepath).Info("Received request to add video to schedule")

	if err := streamer.AddToSchedule(filepath); err != nil {
		logger.WithError(err).Error("Failed to add video to schedule")
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"error":   err.Error(),
		})
		return
	}

	logger.WithField("filepath", filepath).Info("✓ Successfully added video to schedule")
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "Video added to schedule",
		"file":    filepath,
	})
}

// handleScheduleGet returns the current schedule
func handleScheduleGet(c *gin.Context) {
	logger := logs.GetLogger().WithFields(logrus.Fields{
		"module":    "web",
		"handler":   "handleScheduleGet",
		"client_ip": c.ClientIP(),
	})

	logger.Debug("Received request to get schedule")

	schedule, err := streamer.GetSchedule()
	if err != nil {
		logger.WithError(err).Error("Failed to get schedule")
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"error":   err.Error(),
		})
		return
	}

	// Enrich schedule items with filepath for API compatibility
	enrichedSchedule := make([]ScheduleItemResponse, len(schedule))
	for i, item := range schedule {
		enrichedSchedule[i] = enrichScheduleItem(&item)
	}

	logger.WithField("schedule_size", len(schedule)).Info("✓ Successfully retrieved schedule")
	c.JSON(http.StatusOK, gin.H{
		"success":  true,
		"schedule": enrichedSchedule,
		"count":    len(enrichedSchedule),
	})
}

// handleScheduleRemove removes a video from the schedule
func handleScheduleRemove(c *gin.Context) {
	logger := logs.GetLogger().WithFields(logrus.Fields{
		"module":    "web",
		"handler":   "handleScheduleRemove",
		"client_ip": c.ClientIP(),
	})

	fileID := c.Query("file_id")
	if fileID == "" {
		logger.Warn("Missing 'file_id' parameter in request")
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"error":   "Missing 'file_id' parameter",
		})
		return
	}

	logger.WithField("file_id", fileID).Info("Received request to remove video from schedule")

	if err := streamer.RemoveFromSchedule(fileID); err != nil {
		logger.WithError(err).Error("Failed to remove video from schedule")
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"error":   err.Error(),
		})
		return
	}

	logger.WithField("file_id", fileID).Info("✓ Successfully removed video from schedule")
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "Video removed from schedule",
		"file_id": fileID,
	})
}

// handleScheduleClear clears all items from the schedule
func handleScheduleClear(c *gin.Context) {
	logger := logs.GetLogger().WithFields(logrus.Fields{
		"module":    "web",
		"handler":   "handleScheduleClear",
		"client_ip": c.ClientIP(),
	})

	logger.Info("Received request to clear schedule")

	deletedCount, err := streamer.ClearSchedule()
	if err != nil {
		logger.WithError(err).Error("Failed to clear schedule")
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"error":   err.Error(),
		})
		return
	}

	logger.WithField("deleted_count", deletedCount).Info("✓ Successfully cleared schedule")
	c.JSON(http.StatusOK, gin.H{
		"success":       true,
		"message":       "Schedule cleared",
		"deleted_count": deletedCount,
	})
}

// handleScheduleReset resets the schedule position to the beginning
func handleScheduleReset(c *gin.Context) {
	logger := logs.GetLogger().WithFields(logrus.Fields{
		"module":    "web",
		"handler":   "handleScheduleReset",
		"client_ip": c.ClientIP(),
	})

	logger.Info("Received request to reset schedule position")

	if err := streamer.ResetSchedulePosition(); err != nil {
		logger.WithError(err).Error("Failed to reset schedule position")
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"error":   err.Error(),
		})
		return
	}

	logger.Info("✓ Successfully reset schedule position")
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "Schedule position reset to beginning",
	})
}

// handleScheduleRemoveByID removes a specific schedule item by its ID
func handleScheduleRemoveByID(c *gin.Context) {
	logger := logs.GetLogger().WithFields(logrus.Fields{
		"module":    "web",
		"handler":   "handleScheduleRemoveByID",
		"client_ip": c.ClientIP(),
	})

	scheduleIDStr := c.Query("id")
	if scheduleIDStr == "" {
		logger.Warn("Missing 'id' parameter in request")
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"error":   "Missing 'id' parameter",
		})
		return
	}

	scheduleID, err := strconv.ParseInt(scheduleIDStr, 10, 64)
	if err != nil {
		logger.WithError(err).Warn("Invalid schedule ID format")
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"error":   "Invalid schedule ID format",
		})
		return
	}

	logger.WithField("schedule_id", scheduleID).Info("Received request to remove schedule item by ID")

	if err := streamer.RemoveFromScheduleByID(scheduleID); err != nil {
		logger.WithError(err).Error("Failed to remove schedule item")
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"error":   err.Error(),
		})
		return
	}

	logger.WithField("schedule_id", scheduleID).Info("✓ Successfully removed schedule item")
	c.JSON(http.StatusOK, gin.H{
		"success":     true,
		"message":     "Schedule item removed",
		"schedule_id": scheduleID,
	})
}

// handleScheduleReorder updates the position of a schedule item
func handleScheduleReorder(c *gin.Context) {
	logger := logs.GetLogger().WithFields(logrus.Fields{
		"module":    "web",
		"handler":   "handleScheduleReorder",
		"client_ip": c.ClientIP(),
	})

	scheduleIDStr := c.Query("id")
	newPositionStr := c.Query("position")

	if scheduleIDStr == "" || newPositionStr == "" {
		logger.Warn("Missing required parameters")
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"error":   "Missing 'id' or 'position' parameter",
		})
		return
	}

	scheduleID, err := strconv.ParseInt(scheduleIDStr, 10, 64)
	if err != nil {
		logger.WithError(err).Warn("Invalid schedule ID format")
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"error":   "Invalid schedule ID format",
		})
		return
	}

	newPosition, err := strconv.Atoi(newPositionStr)
	if err != nil {
		logger.WithError(err).Warn("Invalid position format")
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"error":   "Invalid position format",
		})
		return
	}

	logger.WithFields(logrus.Fields{
		"schedule_id":  scheduleID,
		"new_position": newPosition,
	}).Info("Received request to reorder schedule item")

	if err := streamer.UpdateSchedulePosition(scheduleID, newPosition); err != nil {
		logger.WithError(err).Error("Failed to reorder schedule item")
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"error":   err.Error(),
		})
		return
	}

	logger.WithFields(logrus.Fields{
		"schedule_id":  scheduleID,
		"new_position": newPosition,
	}).Info("✓ Successfully reordered schedule item")
	c.JSON(http.StatusOK, gin.H{
		"success":     true,
		"message":     "Schedule item reordered",
		"schedule_id": scheduleID,
		"position":    newPosition,
	})
}

// handleGetAvailableFiles returns all available files with ffprobe data
func handleGetAvailableFiles(c *gin.Context) {
	logger := logs.GetLogger().WithFields(logrus.Fields{
		"module":    "web",
		"handler":   "handleGetAvailableFiles",
		"client_ip": c.ClientIP(),
	})

	logger.Debug("Received request to get all available files")

	files, err := streamer.GetAvailableFiles()
	if err != nil {
		logger.WithError(err).Error("Failed to get available files")
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"error":   err.Error(),
		})
		return
	}

	logger.WithField("total_files", len(files)).Info("✓ Successfully retrieved available files")
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"files":   files,
		"count":   len(files),
	})
}
