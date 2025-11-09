package web

import (
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"
	"tv_streamer/helpers"
	"tv_streamer/helpers/logs"
	"tv_streamer/modules/streamer/models"

	"github.com/sirupsen/logrus"
)

// UploadSession represents an active file upload session
type UploadSession struct {
	SessionID       string
	Filename        string
	TotalSize       int64
	ReceivedSize    int64
	File            *os.File
	TempFilePath    string
	StartTime       time.Time
	LastChunkTime   time.Time
}

// WebSocket message types for file upload
type WSUploadInitMessage struct {
	Type     string `json:"type"`
	Filename string `json:"filename"`
	FileSize int64  `json:"file_size"`
}

type WSUploadChunkMessage struct {
	Type      string `json:"type"`
	SessionID string `json:"session_id"`
	ChunkData string `json:"chunk_data"` // base64 encoded
	ChunkNum  int    `json:"chunk_num"`
}

type WSUploadCompleteMessage struct {
	Type      string `json:"type"`
	SessionID string `json:"session_id"`
}

type WSUploadResponseMessage struct {
	Type      string `json:"type"`
	Success   bool   `json:"success"`
	SessionID string `json:"session_id,omitempty"`
	Message   string `json:"message,omitempty"`
	Error     string `json:"error,omitempty"`
	FileID    string `json:"file_id,omitempty"`
}

// Active upload sessions (in production, use a more robust storage)
var uploadSessions = make(map[string]*UploadSession)

// handleUploadInit initializes a new file upload session
func handleUploadInit(client *Client, msg WSUploadInitMessage) {
	logger := logs.GetLogger().WithFields(logrus.Fields{
		"module":   "web",
		"handler":  "handleUploadInit",
		"filename": msg.Filename,
		"size":     msg.FileSize,
	})

	config := helpers.GetConfig()

	// Validate file size
	maxSize := int64(config.Upload.MaxFileSizeMB) * 1024 * 1024
	if msg.FileSize > maxSize {
		logger.Warn("File size exceeds maximum allowed")
		client.SendJSON(WSUploadResponseMessage{
			Type:    "upload_error",
			Success: false,
			Error:   fmt.Sprintf("File size exceeds maximum allowed size of %d MB", config.Upload.MaxFileSizeMB),
		})
		return
	}

	// Validate file extension
	ext := strings.ToLower(strings.TrimPrefix(filepath.Ext(msg.Filename), "."))
	allowed := false
	for _, allowedExt := range config.Upload.AllowedFormats {
		if ext == allowedExt {
			allowed = true
			break
		}
	}
	if !allowed {
		logger.Warn("File format not allowed")
		client.SendJSON(WSUploadResponseMessage{
			Type:    "upload_error",
			Success: false,
			Error:   fmt.Sprintf("File format '%s' not allowed. Allowed formats: %v", ext, config.Upload.AllowedFormats),
		})
		return
	}

	// Create upload directory if it doesn't exist
	if err := os.MkdirAll(config.Upload.UploadDir, 0755); err != nil {
		logger.WithError(err).Error("Failed to create upload directory")
		client.SendJSON(WSUploadResponseMessage{
			Type:    "upload_error",
			Success: false,
			Error:   "Failed to create upload directory",
		})
		return
	}

	// Generate session ID
	sessionID := generateSessionID(msg.Filename)

	// Create temporary file
	tempFilePath := filepath.Join(config.Upload.UploadDir, fmt.Sprintf("%s.tmp", sessionID))
	file, err := os.Create(tempFilePath)
	if err != nil {
		logger.WithError(err).Error("Failed to create temporary file")
		client.SendJSON(WSUploadResponseMessage{
			Type:    "upload_error",
			Success: false,
			Error:   "Failed to create temporary file",
		})
		return
	}

	// Create upload session
	session := &UploadSession{
		SessionID:     sessionID,
		Filename:      msg.Filename,
		TotalSize:     msg.FileSize,
		ReceivedSize:  0,
		File:          file,
		TempFilePath:  tempFilePath,
		StartTime:     time.Now(),
		LastChunkTime: time.Now(),
	}
	uploadSessions[sessionID] = session

	logger.WithField("session_id", sessionID).Info("Upload session initialized")

	// Send success response
	client.SendJSON(WSUploadResponseMessage{
		Type:      "upload_init_success",
		Success:   true,
		SessionID: sessionID,
		Message:   "Upload session initialized. Ready to receive chunks.",
	})
}

// handleUploadChunk processes a file chunk
func handleUploadChunk(client *Client, msg WSUploadChunkMessage) {
	logger := logs.GetLogger().WithFields(logrus.Fields{
		"module":     "web",
		"handler":    "handleUploadChunk",
		"session_id": msg.SessionID,
		"chunk_num":  msg.ChunkNum,
	})

	// Get upload session
	session, exists := uploadSessions[msg.SessionID]
	if !exists {
		logger.Warn("Upload session not found")
		client.SendJSON(WSUploadResponseMessage{
			Type:    "upload_error",
			Success: false,
			Error:   "Upload session not found",
		})
		return
	}

	// Decode base64 chunk data
	chunkData, err := base64Decode(msg.ChunkData)
	if err != nil {
		logger.WithError(err).Error("Failed to decode chunk data")
		client.SendJSON(WSUploadResponseMessage{
			Type:    "upload_error",
			Success: false,
			Error:   "Failed to decode chunk data",
		})
		return
	}

	// Write chunk to file
	n, err := session.File.Write(chunkData)
	if err != nil {
		logger.WithError(err).Error("Failed to write chunk to file")
		cleanupUploadSession(session)
		client.SendJSON(WSUploadResponseMessage{
			Type:    "upload_error",
			Success: false,
			Error:   "Failed to write chunk to file",
		})
		return
	}

	session.ReceivedSize += int64(n)
	session.LastChunkTime = time.Now()

	logger.WithFields(logrus.Fields{
		"received_bytes": session.ReceivedSize,
		"total_bytes":    session.TotalSize,
		"progress_pct":   fmt.Sprintf("%.2f", float64(session.ReceivedSize)/float64(session.TotalSize)*100),
	}).Debug("Chunk received and written")

	// Send chunk acknowledgment
	client.SendJSON(WSUploadResponseMessage{
		Type:      "upload_chunk_ack",
		Success:   true,
		SessionID: msg.SessionID,
		Message:   fmt.Sprintf("Chunk %d received", msg.ChunkNum),
	})
}

// handleUploadComplete finalizes the upload and validates the file
func handleUploadComplete(client *Client, msg WSUploadCompleteMessage) {
	logger := logs.GetLogger().WithFields(logrus.Fields{
		"module":     "web",
		"handler":    "handleUploadComplete",
		"session_id": msg.SessionID,
	})

	// Get upload session
	session, exists := uploadSessions[msg.SessionID]
	if !exists {
		logger.Warn("Upload session not found")
		client.SendJSON(WSUploadResponseMessage{
			Type:    "upload_error",
			Success: false,
			Error:   "Upload session not found",
		})
		return
	}

	// Close the file
	if err := session.File.Close(); err != nil {
		logger.WithError(err).Error("Failed to close file")
		cleanupUploadSession(session)
		client.SendJSON(WSUploadResponseMessage{
			Type:    "upload_error",
			Success: false,
			Error:   "Failed to close file",
		})
		return
	}

	// Verify file size
	if session.ReceivedSize != session.TotalSize {
		logger.WithFields(logrus.Fields{
			"expected": session.TotalSize,
			"received": session.ReceivedSize,
		}).Warn("File size mismatch")
		cleanupUploadSession(session)
		client.SendJSON(WSUploadResponseMessage{
			Type:    "upload_error",
			Success: false,
			Error:   fmt.Sprintf("File size mismatch: expected %d bytes, received %d bytes", session.TotalSize, session.ReceivedSize),
		})
		return
	}

	logger.Info("File upload completed, starting validation...")

	// Validate the file
	fileID, err := validateAndStoreFile(session)
	if err != nil {
		logger.WithError(err).Error("File validation failed")
		cleanupUploadSession(session)
		client.SendJSON(WSUploadResponseMessage{
			Type:    "upload_error",
			Success: false,
			Error:   fmt.Sprintf("File validation failed: %s", err.Error()),
		})
		return
	}

	// Clean up session (but keep the file)
	delete(uploadSessions, msg.SessionID)

	logger.WithField("file_id", fileID).Info("Upload completed successfully")

	// Send success response
	client.SendJSON(WSUploadResponseMessage{
		Type:    "upload_complete",
		Success: true,
		FileID:  fileID,
		Message: "File uploaded and validated successfully. File marked as inactive.",
	})
}

// Helper functions

func generateSessionID(filename string) string {
	data := fmt.Sprintf("%s-%d", filename, time.Now().UnixNano())
	hash := sha256.Sum256([]byte(data))
	return hex.EncodeToString(hash[:])[:32]
}

func base64Decode(data string) ([]byte, error) {
	return base64.StdEncoding.DecodeString(data)
}

func cleanupUploadSession(session *UploadSession) {
	if session.File != nil {
		session.File.Close()
	}
	if session.TempFilePath != "" {
		os.Remove(session.TempFilePath)
	}
	delete(uploadSessions, session.SessionID)
}

// VideoMetadata represents ffprobe output
type VideoMetadata struct {
	Width       int
	Height      int
	Duration    float64
	Format      string
	CodecName   string
	FFProbeData string
}

// validateAndStoreFile validates the uploaded file and stores it in the database
func validateAndStoreFile(session *UploadSession) (string, error) {
	logger := logs.GetLogger().WithFields(logrus.Fields{
		"module":   "web",
		"function": "validateAndStoreFile",
		"filename": session.Filename,
	})

	config := helpers.GetConfig()

	// Run ffprobe to get video metadata
	metadata, err := getVideoMetadata(session.TempFilePath)
	if err != nil {
		return "", fmt.Errorf("failed to get video metadata: %w", err)
	}

	logger.WithFields(logrus.Fields{
		"width":    metadata.Width,
		"height":   metadata.Height,
		"duration": metadata.Duration,
		"format":   metadata.Format,
		"codec":    metadata.CodecName,
	}).Info("Video metadata retrieved")

	// Validate video dimensions
	if metadata.Width != config.Upload.RequiredWidth || metadata.Height != config.Upload.RequiredHeight {
		return "", fmt.Errorf("invalid video dimensions: expected %dx%d, got %dx%d",
			config.Upload.RequiredWidth, config.Upload.RequiredHeight,
			metadata.Width, metadata.Height)
	}

	// Generate file ID
	fileID := generateFileID(session.Filename)

	// Determine final file path
	finalFilename := fmt.Sprintf("%s%s", fileID, filepath.Ext(session.Filename))
	finalFilePath := filepath.Join(config.App.VideoFilesPath, finalFilename)

	// Create video files directory if it doesn't exist
	if err := os.MkdirAll(config.App.VideoFilesPath, 0755); err != nil {
		return "", fmt.Errorf("failed to create video files directory: %w", err)
	}

	// Move file from temp location to final location
	if err := moveFile(session.TempFilePath, finalFilePath); err != nil {
		return "", fmt.Errorf("failed to move file to final location: %w", err)
	}

	logger.WithField("final_path", finalFilePath).Info("File moved to final location")

	// Store file metadata in database
	db := helpers.GetXORM()

	file := &models.AvailableFiles{
		FileID:      fileID,
		FilePath:    finalFilePath,
		FileSize:    session.TotalSize,
		VideoLength: int64(metadata.Duration),
		AddedTime:   time.Now().Unix(),
		FFProbeData: metadata.FFProbeData,
		IsActive:    0, // Mark as inactive
	}

	_, err = db.Insert(file)
	if err != nil {
		// If database insert fails, try to remove the file
		os.Remove(finalFilePath)
		return "", fmt.Errorf("failed to insert file metadata into database: %w", err)
	}

	logger.WithField("file_id", fileID).Info("File metadata stored in database as inactive")

	return fileID, nil
}

// getVideoMetadata uses ffprobe to extract video metadata
func getVideoMetadata(filePath string) (*VideoMetadata, error) {
	// Run ffprobe to get video info in JSON format
	cmd := exec.Command("ffprobe",
		"-v", "quiet",
		"-print_format", "json",
		"-show_format",
		"-show_streams",
		filePath,
	)

	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("ffprobe execution failed: %w", err)
	}

	// Parse JSON output
	var result struct {
		Streams []struct {
			CodecType string `json:"codec_type"`
			CodecName string `json:"codec_name"`
			Width     int    `json:"width"`
			Height    int    `json:"height"`
		} `json:"streams"`
		Format struct {
			Duration   string `json:"duration"`
			FormatName string `json:"format_name"`
		} `json:"format"`
	}

	if err := json.Unmarshal(output, &result); err != nil {
		return nil, fmt.Errorf("failed to parse ffprobe output: %w", err)
	}

	metadata := &VideoMetadata{
		FFProbeData: string(output),
	}

	// Find video stream
	for _, stream := range result.Streams {
		if stream.CodecType == "video" {
			metadata.Width = stream.Width
			metadata.Height = stream.Height
			metadata.CodecName = stream.CodecName
			break
		}
	}

	// Parse duration
	if result.Format.Duration != "" {
		duration, err := strconv.ParseFloat(result.Format.Duration, 64)
		if err == nil {
			metadata.Duration = duration
		}
	}

	metadata.Format = result.Format.FormatName

	// Validate that we got video dimensions
	if metadata.Width == 0 || metadata.Height == 0 {
		return nil, fmt.Errorf("no video stream found or invalid dimensions")
	}

	return metadata, nil
}

// generateFileID generates a unique file ID based on filename and timestamp
func generateFileID(filename string) string {
	data := fmt.Sprintf("%s-%d", filename, time.Now().UnixNano())
	hash := sha256.Sum256([]byte(data))
	return hex.EncodeToString(hash[:])[:16]
}
