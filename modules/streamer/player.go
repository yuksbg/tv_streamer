package streamer

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"
	"tv_streamer/helpers"
	"tv_streamer/helpers/logs"
	"tv_streamer/modules/streamer/models"

	"github.com/sirupsen/logrus"
)

// Player manages the FFmpeg streaming pipeline
type Player struct {
	mu             sync.RWMutex
	cmd            *exec.Cmd
	currentFile    *models.VideoQueue
	currentHistory *models.PlayHistory
	stopChan       chan struct{}
	skipChan       chan struct{}
	running        bool
	logger         *logrus.Entry
	outputDir      string
	videoFilesPath string
	hlsSegmentTime int
	hlsListSize    int
	ffmpegPreset   string
	videoBitrate   string
	audioBitrate   string
	videoComplete  chan error // Signal when current video completes
}

var (
	globalPlayer     *Player
	globalPlayerOnce sync.Once
)

// GetPlayer returns the singleton Player instance
func GetPlayer() *Player {
	globalPlayerOnce.Do(func() {
		config := helpers.GetConfig()

		logger := logs.GetLogger().WithField("module", "streamer")
		logger.Info("Initializing TV Streamer Player...")

		globalPlayer = &Player{
			stopChan:       make(chan struct{}),
			skipChan:       make(chan struct{}),
			logger:         logger,
			outputDir:      "./out",
			videoFilesPath: config.App.VideoFilesPath,
			hlsSegmentTime: 6,
			hlsListSize:    10,
			ffmpegPreset:   "veryfast",
			videoBitrate:   "2000k",
			audioBitrate:   "128k",
		}

		logger.WithFields(logrus.Fields{
			"output_dir":       globalPlayer.outputDir,
			"video_files_path": globalPlayer.videoFilesPath,
			"hls_segment_time": globalPlayer.hlsSegmentTime,
			"hls_list_size":    globalPlayer.hlsListSize,
			"stream_copy":      true,
		}).Info("Player configuration loaded")
	})
	return globalPlayer
}

// Start initializes and starts the streaming pipeline
func (p *Player) Start() error {
	p.mu.Lock()
	if p.running {
		p.mu.Unlock()
		p.logger.Warn("Player is already running")
		return fmt.Errorf("player is already running")
	}
	p.running = true
	p.videoComplete = make(chan error, 1)
	p.mu.Unlock()

	p.logger.Info("Starting TV Streamer Player...")

	// Create output directory
	if err := os.MkdirAll(p.outputDir, 0755); err != nil {
		p.logger.WithError(err).Error("Failed to create output directory")
		return fmt.Errorf("failed to create output directory: %w", err)
	}
	p.logger.WithField("path", p.outputDir).Info("✓ Output directory created/verified")

	// Start video player goroutine (no persistent FFmpeg needed)
	go p.videoPlayer()

	p.logger.Info("✓ TV Streamer Player started successfully")
	return nil
}

// startFFmpegForVideo starts FFmpeg to stream a specific video file
func (p *Player) startFFmpegForVideo(videoPath string) error {
	p.logger.WithField("video_path", videoPath).Info("Starting FFmpeg process for video...")

	// Verify file exists
	fileInfo, err := os.Stat(videoPath)
	if err != nil {
		p.logger.WithError(err).WithField("video_path", videoPath).Error("Video file does not exist")
		return fmt.Errorf("video file does not exist: %w", err)
	}

	p.logger.WithFields(logrus.Fields{
		"file_size":  fileInfo.Size(),
		"file_mode":  fileInfo.Mode().String(),
		"video_path": videoPath,
	}).Debug("✓ Video file verified")

	// Build FFmpeg command to read directly from file
	cmd := exec.Command("ffmpeg",
		"-re",           // Read at native frame rate
		"-i", videoPath, // Direct file input (no stdin pipe!)
		"-c:v", "copy", // Copy video codec (no re-encoding)
		"-c:a", "copy", // Copy audio codec (no re-encoding)
		"-f", "hls", // HLS output format
		"-hls_time", fmt.Sprintf("%d", p.hlsSegmentTime), // Segment duration
		"-hls_list_size", fmt.Sprintf("%d", p.hlsListSize), // Playlist size
		"-hls_flags", "delete_segments+append_list", // Auto-cleanup old segments
		"-hls_segment_filename", filepath.Join(p.outputDir, "segment_%03d.ts"),
		filepath.Join(p.outputDir, "stream.m3u8"),
	)

	p.logger.WithFields(logrus.Fields{
		"command": cmd.String(),
		"args":    cmd.Args,
	}).Debug("FFmpeg command prepared")

	// Capture stdout and stderr for debugging
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		p.logger.WithError(err).Error("Failed to create stdout pipe for FFmpeg")
		return fmt.Errorf("failed to create stdout pipe: %w", err)
	}

	stderr, err := cmd.StderrPipe()
	if err != nil {
		p.logger.WithError(err).Error("Failed to create stderr pipe for FFmpeg")
		return fmt.Errorf("failed to create stderr pipe: %w", err)
	}

	p.mu.Lock()
	p.cmd = cmd
	p.mu.Unlock()

	// Monitor FFmpeg output in background
	go p.monitorFFmpegOutput(stdout, stderr)

	// Start FFmpeg process
	startTime := time.Now()
	if err := cmd.Start(); err != nil {
		p.logger.WithError(err).Error("Failed to start FFmpeg process")
		return fmt.Errorf("failed to start FFmpeg: %w", err)
	}

	p.logger.WithFields(logrus.Fields{
		"pid":             cmd.Process.Pid,
		"video_path":      videoPath,
		"output_file":     filepath.Join(p.outputDir, "stream.m3u8"),
		"startup_time_ms": time.Since(startTime).Milliseconds(),
	}).Info("✓ FFmpeg process started successfully")

	// Monitor FFmpeg process completion in background
	go func() {
		err := cmd.Wait()
		elapsed := time.Since(startTime)

		if err != nil {
			p.logger.WithError(err).WithFields(logrus.Fields{
				"video_path":   videoPath,
				"elapsed_time": elapsed.String(),
			}).Error("FFmpeg process exited with error")
			p.videoComplete <- fmt.Errorf("FFmpeg error: %w", err)
		} else {
			p.logger.WithFields(logrus.Fields{
				"video_path":   videoPath,
				"elapsed_time": elapsed.String(),
			}).Info("✓ FFmpeg process completed successfully")
			p.videoComplete <- nil
		}
	}()

	return nil
}

// monitorFFmpegOutput monitors FFmpeg stdout and stderr for logging
func (p *Player) monitorFFmpegOutput(stdout, stderr io.Reader) {
	p.logger.Debug("Starting FFmpeg output monitor...")

	// Monitor stderr (FFmpeg writes progress/errors to stderr)
	go func() {
		scanner := bufio.NewScanner(stderr)
		lineCount := 0
		for scanner.Scan() {
			line := scanner.Text()
			lineCount++

			// Enhanced logging with better categorization
			if strings.Contains(line, "error") || strings.Contains(line, "Error") || strings.Contains(line, "failed") {
				p.logger.WithField("ffmpeg_stderr", line).Error("⚠ FFmpeg error detected")
			} else if strings.Contains(line, "warning") || strings.Contains(line, "Warning") {
				p.logger.WithField("ffmpeg_stderr", line).Warn("FFmpeg warning")
			} else if strings.Contains(line, "frame=") || strings.Contains(line, "time=") {
				// Progress line - log at debug level to avoid spam
				p.logger.WithField("ffmpeg_stderr", line).Debug("FFmpeg progress")
			} else if strings.Contains(line, "Input #") || strings.Contains(line, "Duration:") {
				p.logger.WithField("ffmpeg_stderr", line).Info("📹 FFmpeg input info")
			} else if strings.Contains(line, "Output #") || strings.Contains(line, "Stream #") {
				p.logger.WithField("ffmpeg_stderr", line).Info("📤 FFmpeg output info")
			} else if strings.Contains(line, "Opening") {
				p.logger.WithField("ffmpeg_stderr", line).Info("📂 FFmpeg opening file")
			} else if line != "" {
				// Log all other non-empty lines for complete debugging
				p.logger.WithField("ffmpeg_stderr", line).Debug("FFmpeg stderr")
			}
		}

		if err := scanner.Err(); err != nil {
			p.logger.WithError(err).Error("Error reading FFmpeg stderr")
		}

		p.logger.WithField("stderr_lines", lineCount).Debug("FFmpeg stderr monitor stopped")
	}()

	// Monitor stdout (usually empty for FFmpeg, but log just in case)
	go func() {
		scanner := bufio.NewScanner(stdout)
		lineCount := 0
		for scanner.Scan() {
			line := scanner.Text()
			lineCount++
			if line != "" {
				p.logger.WithField("ffmpeg_stdout", line).Debug("FFmpeg stdout")
			}
		}

		if err := scanner.Err(); err != nil {
			p.logger.WithError(err).Error("Error reading FFmpeg stdout")
		}

		if lineCount > 0 {
			p.logger.WithField("stdout_lines", lineCount).Debug("FFmpeg stdout monitor stopped")
		}
	}()
}

// videoPlayer continuously plays videos from the queue
func (p *Player) videoPlayer() {
	p.logger.Info("Starting video player loop...")

	for {
		select {
		case <-p.stopChan:
			p.logger.Info("Stop signal received, exiting video player")
			// Kill current FFmpeg if running
			p.killCurrentFFmpeg()
			return
		default:
			// Get next video from queue
			video, err := p.getNextVideo()
			if err != nil {
				p.logger.WithError(err).Warn("Failed to get next video, waiting 5 seconds...")
				time.Sleep(5 * time.Second)
				continue
			}

			if video == nil {
				p.logger.Info("No videos in queue, attempting to auto-fill from library...")
				if err := p.autoFillQueueFromLibrary(); err != nil {
					p.logger.WithError(err).Warn("Failed to auto-fill queue from library, waiting 5 seconds...")
					time.Sleep(5 * time.Second)
					continue
				}
				// Try to get next video again after filling
				continue
			}

			// Play the video
			if err := p.playVideo(video); err != nil {
				p.logger.WithError(err).WithFields(logrus.Fields{
					"file_id":  video.FileID,
					"filepath": video.FilePath,
					"is_ad":    video.IsAd == 1,
				}).Error("Failed to play video")

				// Mark as failed in history
				if p.currentHistory != nil {
					p.currentHistory.MarkAsSkipped()
					if _, err := helpers.GetXORM().ID(p.currentHistory.ID).Cols("finished_at", "duration_seconds", "skip_requested").Update(p.currentHistory); err != nil {
						p.logger.WithError(err).Error("Failed to update play history")
					}
				}

				// CRITICAL: Mark video as played even on failure to prevent infinite retry loop
				video.MarkAsPlayed()
				if _, err := helpers.GetXORM().ID(video.ID).Cols("played", "played_at").Update(video); err != nil {
					p.logger.WithError(err).Error("Failed to mark failed video as played")
				} else {
					p.logger.WithField("video_id", video.ID).Info("Marked failed video as played to move to next")
				}

				// Kill any stuck FFmpeg process
				p.killCurrentFFmpeg()

				// Add small delay before trying next video
				time.Sleep(2 * time.Second)
			}
		}
	}
}

// killCurrentFFmpeg kills the current FFmpeg process if running
func (p *Player) killCurrentFFmpeg() {
	p.mu.Lock()
	cmd := p.cmd
	p.mu.Unlock()

	if cmd != nil && cmd.Process != nil {
		p.logger.WithField("pid", cmd.Process.Pid).Warn("Killing current FFmpeg process")
		if err := cmd.Process.Kill(); err != nil {
			p.logger.WithError(err).Error("Failed to kill FFmpeg process")
		} else {
			p.logger.Info("✓ FFmpeg process killed")
		}
	}
}

// getNextVideo retrieves the next video from the queue
func (p *Player) getNextVideo() (*models.VideoQueue, error) {
	p.logger.Debug("Fetching next video from queue...")

	var video models.VideoQueue
	has, err := helpers.GetXORM().
		Where("played = ?", 0).
		OrderBy("queue_position ASC, id ASC").
		Get(&video)

	if err != nil {
		return nil, fmt.Errorf("database query failed: %w", err)
	}

	if !has {
		p.logger.Debug("No unplayed videos in queue")
		return nil, nil
	}

	p.logger.WithFields(logrus.Fields{
		"video_id":       video.ID,
		"file_id":        video.FileID,
		"filepath":       video.FilePath,
		"queue_position": video.QueuePosition,
		"is_ad":          video.IsAd == 1,
	}).Info("✓ Next video retrieved from queue")

	return &video, nil
}

// autoFillQueueFromLibrary automatically fills the queue from schedule (endless loop)
func (p *Player) autoFillQueueFromLibrary() error {
	p.logger.Info("Auto-filling queue from schedule...")

	// Get next video from schedule (handles endless loop automatically)
	scheduleItem, err := GetNextFromSchedule()
	if err != nil {
		return fmt.Errorf("failed to get next from schedule: %w", err)
	}

	if scheduleItem == nil {
		// Schedule is empty, attempt to auto-populate from available_files
		p.logger.Info("Schedule is empty, attempting to populate from available files...")

		var availableFiles []models.AvailableFiles
		err := helpers.GetXORM().Find(&availableFiles)
		if err != nil {
			p.logger.WithError(err).Error("Failed to query available files")
			return fmt.Errorf("failed to query available files: %w", err)
		}

		if len(availableFiles) == 0 {
			p.logger.Warn("No videos in available files - please add videos to the library first")
			return fmt.Errorf("no videos available in library")
		}

		p.logger.WithField("file_count", len(availableFiles)).Info("Found available files, populating schedule...")

		// Add all available files to schedule
		successCount := 0
		for i, file := range availableFiles {
			// Verify file still exists on disk
			if _, err := os.Stat(file.FilePath); err != nil {
				p.logger.WithFields(logrus.Fields{
					"file_id":  file.FileID,
					"filepath": file.FilePath,
				}).Warn("Available file no longer exists on disk, skipping")
				continue
			}

			scheduleItem := &models.Schedule{
				FileID:           file.FileID,
				FilePath:         file.FilePath,
				SchedulePosition: i,
				IsCurrent:        0,
				AddedAt:          time.Now().Unix(),
			}

			if _, err := helpers.GetXORM().Insert(scheduleItem); err != nil {
				p.logger.WithError(err).WithField("filepath", file.FilePath).Warn("Failed to add file to schedule")
				continue
			}
			successCount++
		}

		if successCount == 0 {
			p.logger.Warn("Failed to add any files to schedule")
			return fmt.Errorf("failed to populate schedule from available files")
		}

		p.logger.WithField("added_count", successCount).Info("✓ Schedule auto-populated from available files")

		// Retry getting from schedule
		scheduleItem, err = GetNextFromSchedule()
		if err != nil {
			return fmt.Errorf("failed to get next from schedule after population: %w", err)
		}

		if scheduleItem == nil {
			p.logger.Error("Schedule is still empty after population - this should not happen")
			return fmt.Errorf("schedule population failed unexpectedly")
		}
	}

	p.logger.WithFields(logrus.Fields{
		"file_id":           scheduleItem.FileID,
		"filepath":          scheduleItem.FilePath,
		"schedule_position": scheduleItem.SchedulePosition,
	}).Info("Retrieved next video from schedule")

	// Check if file still exists on disk
	if _, err := os.Stat(scheduleItem.FilePath); err != nil {
		p.logger.WithFields(logrus.Fields{
			"file_id":  scheduleItem.FileID,
			"filepath": scheduleItem.FilePath,
		}).Error("Scheduled file no longer exists on disk")
		return fmt.Errorf("scheduled file does not exist: %w", err)
	}

	// Get current max queue position
	var maxPosition int
	_, err = helpers.GetXORM().SQL("SELECT COALESCE(MAX(queue_position), 0) FROM video_queue").Get(&maxPosition)
	if err != nil {
		return fmt.Errorf("failed to get max queue position: %w", err)
	}

	// Add scheduled video to queue
	nextPosition := maxPosition + 1
	queueItem := &models.VideoQueue{
		FileID:        scheduleItem.FileID,
		FilePath:      scheduleItem.FilePath,
		AddedAt:       time.Now().Unix(),
		Played:        0,
		QueuePosition: nextPosition,
		IsAd:          0,
	}

	if _, err := helpers.GetXORM().Insert(queueItem); err != nil {
		p.logger.WithError(err).WithField("filepath", scheduleItem.FilePath).Error("Failed to add scheduled video to queue")
		return fmt.Errorf("failed to add to queue: %w", err)
	}

	p.logger.WithFields(logrus.Fields{
		"queue_id":          queueItem.ID,
		"file_id":           scheduleItem.FileID,
		"filepath":          scheduleItem.FilePath,
		"queue_position":    nextPosition,
		"schedule_position": scheduleItem.SchedulePosition,
	}).Info("✓ Queue auto-filled with next scheduled video")

	return nil
}

// playVideo plays a single video file using FFmpeg
func (p *Player) playVideo(video *models.VideoQueue) error {
	startTime := time.Now()

	p.logger.WithFields(logrus.Fields{
		"video_id":  video.ID,
		"file_id":   video.FileID,
		"filepath":  video.FilePath,
		"is_ad":     video.IsAd == 1,
		"timestamp": startTime.Format(time.RFC3339),
	}).Info("▶ Starting to play video")

	// Create play history record
	history := &models.PlayHistory{
		FileID:    video.FileID,
		Filename:  filepath.Base(video.FilePath),
		FilePath:  video.FilePath,
		StartedAt: startTime.Unix(),
		IsAd:      video.IsAd,
	}

	if _, err := helpers.GetXORM().Insert(history); err != nil {
		p.logger.WithError(err).Error("Failed to create play history record")
	} else {
		p.logger.WithField("history_id", history.ID).Debug("✓ Play history record created")
	}

	p.mu.Lock()
	p.currentFile = video
	p.currentHistory = history
	p.mu.Unlock()

	// Start FFmpeg to stream this video file (reads directly from disk)
	if err := p.startFFmpegForVideo(video.FilePath); err != nil {
		p.logger.WithError(err).Error("Failed to start FFmpeg for video")
		return fmt.Errorf("failed to start FFmpeg: %w", err)
	}

	// Wait for video to complete or skip signal
	select {
	case <-p.skipChan:
		p.logger.WithField("filepath", video.FilePath).Warn("⏭ Skip requested, stopping current video")

		// Kill FFmpeg process
		p.killCurrentFFmpeg()

		// Mark as skipped in history
		history.MarkAsSkipped()
		if _, err := helpers.GetXORM().ID(history.ID).Cols("finished_at", "duration_seconds", "skip_requested").Update(history); err != nil {
			p.logger.WithError(err).Error("Failed to update play history")
		}

		// Mark video as played
		video.MarkAsPlayed()
		if _, err := helpers.GetXORM().ID(video.ID).Cols("played", "played_at").Update(video); err != nil {
			p.logger.WithError(err).Error("Failed to mark video as played")
		}

		p.mu.Lock()
		p.currentFile = nil
		p.currentHistory = nil
		p.mu.Unlock()

		return fmt.Errorf("video skipped by user")

	case err := <-p.videoComplete:
		duration := time.Since(startTime)

		if err != nil {
			// FFmpeg failed
			p.logger.WithError(err).WithFields(logrus.Fields{
				"filepath": video.FilePath,
				"duration": duration.String(),
			}).Error("FFmpeg failed while playing video")
			return fmt.Errorf("FFmpeg error: %w", err)
		}

		// Video completed successfully
		p.logger.WithFields(logrus.Fields{
			"filepath":         video.FilePath,
			"duration":         duration.String(),
			"duration_seconds": duration.Seconds(),
		}).Info("✓ Video playback completed successfully")

		// Update play history
		history.MarkAsFinished()
		if _, err := helpers.GetXORM().ID(history.ID).Cols("finished_at", "duration_seconds").Update(history); err != nil {
			p.logger.WithError(err).Error("Failed to update play history")
		} else {
			p.logger.WithField("history_id", history.ID).Debug("✓ Play history updated")
		}

		// Mark video as played
		video.MarkAsPlayed()
		if _, err := helpers.GetXORM().ID(video.ID).Cols("played", "played_at").Update(video); err != nil {
			p.logger.WithError(err).Error("Failed to mark video as played")
		} else {
			p.logger.WithField("video_id", video.ID).Debug("✓ Video marked as played in queue")
		}

		p.mu.Lock()
		p.currentFile = nil
		p.currentHistory = nil
		p.mu.Unlock()

		// Small delay before next video for smooth transition
		p.logger.Debug("Waiting 1 second before loading next video")
		time.Sleep(1 * time.Second)

		return nil
	}
}

// Skip skips the currently playing video
func (p *Player) Skip() error {
	p.mu.RLock()
	currentFile := p.currentFile
	p.mu.RUnlock()

	if currentFile == nil {
		p.logger.Warn("Skip requested but no video is currently playing")
		return fmt.Errorf("no video currently playing")
	}

	p.logger.WithFields(logrus.Fields{
		"file_id":  currentFile.FileID,
		"filepath": currentFile.FilePath,
	}).Info("⏭ Skipping current video")

	select {
	case p.skipChan <- struct{}{}:
		p.logger.Info("Skip signal sent successfully")
		return nil
	case <-time.After(1 * time.Second):
		p.logger.Warn("Skip signal timeout")
		return fmt.Errorf("skip signal timeout")
	}
}

// Stop stops the player and FFmpeg process
func (p *Player) Stop() error {
	p.logger.Info("Stopping TV Streamer Player...")

	p.mu.Lock()
	if !p.running {
		p.mu.Unlock()
		p.logger.Warn("Player is not running")
		return fmt.Errorf("player is not running")
	}
	p.running = false
	p.mu.Unlock()

	// Send stop signal to video player goroutine
	close(p.stopChan)

	// Kill current FFmpeg process if running
	p.killCurrentFFmpeg()

	p.logger.Info("✓ TV Streamer Player stopped successfully")
	return nil
}

// GetStatus returns the current player status
func (p *Player) GetStatus() map[string]interface{} {
	p.mu.RLock()
	defer p.mu.RUnlock()

	status := map[string]interface{}{
		"running": p.running,
	}

	if p.currentFile != nil {
		status["current_video"] = map[string]interface{}{
			"file_id":  p.currentFile.FileID,
			"filepath": p.currentFile.FilePath,
			"is_ad":    p.currentFile.IsAd == 1,
		}
	}

	if p.currentHistory != nil {
		status["playback_started_at"] = time.Unix(p.currentHistory.StartedAt, 0).Format(time.RFC3339)
		status["playback_duration_seconds"] = time.Now().Unix() - p.currentHistory.StartedAt
	}

	return status
}
