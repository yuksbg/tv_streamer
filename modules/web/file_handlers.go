package web

import (
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"tv_streamer/helpers"
	"tv_streamer/helpers/logs"
	"tv_streamer/modules/streamer/models"

	"github.com/gin-gonic/gin"
	"github.com/sirupsen/logrus"
)

// handleFilesList returns all available files
func handleFilesList(c *gin.Context) {
	logger := logs.GetLogger().WithFields(logrus.Fields{
		"module":    "web",
		"handler":   "handleFilesList",
		"client_ip": c.ClientIP(),
	})

	logger.Debug("Received request to list available files")

	var files []models.AvailableFiles
	db := helpers.GetXORM()
	err := db.OrderBy("added_time DESC").Find(&files)
	if err != nil {
		logger.WithError(err).Error("Failed to retrieve available files")
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"error":   "Failed to retrieve files",
		})
		return
	}

	logger.WithField("files_count", len(files)).Info("✓ Successfully retrieved files list")
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"files":   files,
		"count":   len(files),
	})
}

// handleFileInfo returns detailed information about a specific file
func handleFileInfo(c *gin.Context) {
	logger := logs.GetLogger().WithFields(logrus.Fields{
		"module":    "web",
		"handler":   "handleFileInfo",
		"client_ip": c.ClientIP(),
	})

	fileID := c.Param("file_id")
	if fileID == "" {
		logger.Warn("Missing 'file_id' parameter in request")
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"error":   "Missing 'file_id' parameter",
		})
		return
	}

	logger.WithField("file_id", fileID).Debug("Received request to get file info")

	var file models.AvailableFiles
	db := helpers.GetXORM()
	found, err := db.Where("file_id = ?", fileID).Get(&file)
	if err != nil {
		logger.WithError(err).Error("Failed to retrieve file info")
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"error":   "Failed to retrieve file info",
		})
		return
	}

	if !found {
		logger.WithField("file_id", fileID).Warn("File not found")
		c.JSON(http.StatusNotFound, gin.H{
			"success": false,
			"error":   "File not found",
		})
		return
	}

	logger.WithField("file_id", fileID).Info("✓ Successfully retrieved file info")
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"file":    file,
	})
}

// handleFileRename renames a file (both database record and physical file)
func handleFileRename(c *gin.Context) {
	logger := logs.GetLogger().WithFields(logrus.Fields{
		"module":    "web",
		"handler":   "handleFileRename",
		"client_ip": c.ClientIP(),
	})

	fileID := c.Param("file_id")
	if fileID == "" {
		logger.Warn("Missing 'file_id' parameter in request")
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"error":   "Missing 'file_id' parameter",
		})
		return
	}

	var req struct {
		NewName string `json:"new_name" binding:"required"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		logger.WithError(err).Warn("Invalid request body")
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"error":   "Invalid request body: new_name is required",
		})
		return
	}

	logger.WithFields(logrus.Fields{
		"file_id":  fileID,
		"new_name": req.NewName,
	}).Info("Received request to rename file")

	db := helpers.GetXORM()

	// Get the current file info
	var file models.AvailableFiles
	found, err := db.Where("file_id = ?", fileID).Get(&file)
	if err != nil {
		logger.WithError(err).Error("Failed to retrieve file info")
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"error":   "Failed to retrieve file info",
		})
		return
	}

	if !found {
		logger.WithField("file_id", fileID).Warn("File not found")
		c.JSON(http.StatusNotFound, gin.H{
			"success": false,
			"error":   "File not found",
		})
		return
	}

	// Check if the physical file exists
	if _, err := os.Stat(file.FilePath); os.IsNotExist(err) {
		logger.WithField("filepath", file.FilePath).Warn("Physical file does not exist")
		c.JSON(http.StatusNotFound, gin.H{
			"success": false,
			"error":   "Physical file does not exist",
		})
		return
	}

	// Construct the new file path
	dir := filepath.Dir(file.FilePath)
	ext := filepath.Ext(file.FilePath)
	newPath := filepath.Join(dir, req.NewName+ext)

	// Check if the new file path already exists
	if _, err := os.Stat(newPath); err == nil {
		logger.WithField("new_path", newPath).Warn("File with new name already exists")
		c.JSON(http.StatusConflict, gin.H{
			"success": false,
			"error":   "File with new name already exists",
		})
		return
	}

	// Rename the physical file
	if err := os.Rename(file.FilePath, newPath); err != nil {
		logger.WithError(err).Error("Failed to rename physical file")
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"error":   "Failed to rename physical file",
		})
		return
	}

	// Update the database record
	file.FilePath = newPath
	_, err = db.Where("file_id = ?", fileID).Update(&file)
	if err != nil {
		logger.WithError(err).Error("Failed to update file path in database")
		// Try to revert the file rename
		os.Rename(newPath, file.FilePath)
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"error":   "Failed to update file path in database",
		})
		return
	}

	logger.WithFields(logrus.Fields{
		"file_id":  fileID,
		"old_path": file.FilePath,
		"new_path": newPath,
	}).Info("✓ Successfully renamed file")

	c.JSON(http.StatusOK, gin.H{
		"success":  true,
		"message":  "File renamed successfully",
		"file_id":  fileID,
		"old_path": file.FilePath,
		"new_path": newPath,
	})
}

// handleFileDelete deletes a file (both database record and physical file)
func handleFileDelete(c *gin.Context) {
	logger := logs.GetLogger().WithFields(logrus.Fields{
		"module":    "web",
		"handler":   "handleFileDelete",
		"client_ip": c.ClientIP(),
	})

	fileID := c.Param("file_id")
	if fileID == "" {
		logger.Warn("Missing 'file_id' parameter in request")
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"error":   "Missing 'file_id' parameter",
		})
		return
	}

	logger.WithField("file_id", fileID).Info("Received request to delete file")

	db := helpers.GetXORM()

	// Get the current file info
	var file models.AvailableFiles
	found, err := db.Where("file_id = ?", fileID).Get(&file)
	if err != nil {
		logger.WithError(err).Error("Failed to retrieve file info")
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"error":   "Failed to retrieve file info",
		})
		return
	}

	if !found {
		logger.WithField("file_id", fileID).Warn("File not found")
		c.JSON(http.StatusNotFound, gin.H{
			"success": false,
			"error":   "File not found",
		})
		return
	}

	// Delete the physical file if it exists
	if _, err := os.Stat(file.FilePath); err == nil {
		if err := os.Remove(file.FilePath); err != nil {
			logger.WithError(err).Error("Failed to delete physical file")
			c.JSON(http.StatusInternalServerError, gin.H{
				"success": false,
				"error":   "Failed to delete physical file",
			})
			return
		}
	} else {
		logger.WithField("filepath", file.FilePath).Warn("Physical file does not exist, skipping deletion")
	}

	// Delete from database
	_, err = db.Where("file_id = ?", fileID).Delete(&models.AvailableFiles{})
	if err != nil {
		logger.WithError(err).Error("Failed to delete file from database")
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"error":   "Failed to delete file from database",
		})
		return
	}

	// Also remove from queue and schedule if present
	_, err = db.Exec(fmt.Sprintf("DELETE FROM video_queue WHERE file_id = '%s'", fileID))
	if err != nil {
		logger.WithError(err).Warn("Failed to remove file from queue")
	}

	_, err = db.Exec(fmt.Sprintf("DELETE FROM schedule WHERE file_id = '%s'", fileID))
	if err != nil {
		logger.WithError(err).Warn("Failed to remove file from schedule")
	}

	logger.WithFields(logrus.Fields{
		"file_id":  fileID,
		"filepath": file.FilePath,
	}).Info("✓ Successfully deleted file")

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "File deleted successfully",
		"file_id": fileID,
	})
}
