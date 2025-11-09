package web

import (
	"fmt"
	"io"
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

	// Store old path for potential rollback
	oldPath := file.FilePath

	// Rename the physical file
	if err := moveFile(file.FilePath, newPath); err != nil {
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
		moveFile(newPath, oldPath)
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"error":   "Failed to update file path in database",
		})
		return
	}

	logger.WithFields(logrus.Fields{
		"file_id":  fileID,
		"old_path": oldPath,
		"new_path": newPath,
	}).Info("✓ Successfully renamed file")

	c.JSON(http.StatusOK, gin.H{
		"success":  true,
		"message":  "File renamed successfully",
		"file_id":  fileID,
		"old_path": oldPath,
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

// handleFileUpdateDescription updates the description of a file
func handleFileUpdateDescription(c *gin.Context) {
	logger := logs.GetLogger().WithFields(logrus.Fields{
		"module":    "web",
		"handler":   "handleFileUpdateDescription",
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
		Description string `json:"description"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		logger.WithError(err).Warn("Invalid request body")
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"error":   "Invalid request body: description field is required",
		})
		return
	}

	logger.WithFields(logrus.Fields{
		"file_id":     fileID,
		"description": req.Description,
	}).Info("Received request to update file description")

	db := helpers.GetXORM()

	// Check if file exists
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

	// Update description using XORM
	file.Description = req.Description
	_, err = db.Where("file_id = ?", fileID).Cols("description").Update(&file)
	if err != nil {
		logger.WithError(err).Error("Failed to update file description")
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"error":   "Failed to update file description",
		})
		return
	}

	logger.WithFields(logrus.Fields{
		"file_id":     fileID,
		"description": req.Description,
	}).Info("✓ Successfully updated file description")

	c.JSON(http.StatusOK, gin.H{
		"success":     true,
		"message":     "File description updated successfully",
		"file_id":     fileID,
		"description": req.Description,
	})
}

// moveFile moves a file from src to dst, handling cross-filesystem moves
// by copying the file and then removing the source if os.Rename fails
func moveFile(src, dst string) error {
	// Try a simple rename first (works if on same filesystem)
	err := os.Rename(src, dst)
	if err == nil {
		return nil
	}

	// If rename failed, copy the file and then remove the source
	srcFile, err := os.Open(src)
	if err != nil {
		return fmt.Errorf("failed to open source file: %w", err)
	}
	defer srcFile.Close()

	dstFile, err := os.Create(dst)
	if err != nil {
		return fmt.Errorf("failed to create destination file: %w", err)
	}
	defer dstFile.Close()

	// Copy the file contents
	_, err = io.Copy(dstFile, srcFile)
	if err != nil {
		// Remove incomplete destination file
		os.Remove(dst)
		return fmt.Errorf("failed to copy file contents: %w", err)
	}

	// Ensure all data is written to disk
	err = dstFile.Sync()
	if err != nil {
		os.Remove(dst)
		return fmt.Errorf("failed to sync destination file: %w", err)
	}

	// Copy file permissions
	srcInfo, err := os.Stat(src)
	if err == nil {
		os.Chmod(dst, srcInfo.Mode())
	}

	// Remove the source file only after successful copy
	err = os.Remove(src)
	if err != nil {
		// Destination exists but source couldn't be removed
		return fmt.Errorf("file copied but failed to remove source: %w", err)
	}

	return nil
}
