package web

import (
	"net/http"
	"strconv"
	"tv_streamer/helpers/logs"
	"tv_streamer/modules/streamer"

	"github.com/gin-gonic/gin"
	"github.com/sirupsen/logrus"
)

// handleStreamNext skips to the next video in the queue
func handleStreamNext(c *gin.Context) {
	logger := logs.GetLogger().WithFields(logrus.Fields{
		"module":   "web",
		"handler":  "handleStreamNext",
		"client_ip": c.ClientIP(),
	})

	logger.Info("Received request to skip to next video")

	player := streamer.GetPlayer()
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

	logger.WithField("queue_size", len(queue)).Info("✓ Successfully retrieved queue")
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"queue":   queue,
		"count":   len(queue),
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

	player := streamer.GetPlayer()
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

	logger.WithField("records_count", len(history)).Info("✓ Successfully retrieved play history")
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"history": history,
		"count":   len(history),
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
