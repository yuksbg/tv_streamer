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
	"syscall"
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
	stdin          io.WriteCloser
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
	p.mu.Unlock()

	p.logger.Info("Starting TV Streamer Player...")

	// Create output directory
	if err := os.MkdirAll(p.outputDir, 0755); err != nil {
		p.logger.WithError(err).Error("Failed to create output directory")
		return fmt.Errorf("failed to create output directory: %w", err)
	}
	p.logger.WithField("path", p.outputDir).Info("Output directory created/verified")

	// Start FFmpeg process
	if err := p.startFFmpeg(); err != nil {
		p.logger.WithError(err).Error("Failed to start FFmpeg")
		return err
	}

	// Start file feeder goroutine
	go p.fileFeeder()

	p.logger.Info("✓ TV Streamer Player started successfully")
	return nil
}

// startFFmpeg spawns the FFmpeg process with HLS output
func (p *Player) startFFmpeg() error {
	p.logger.Info("Starting FFmpeg process...")

	cmd := exec.Command("ffmpeg",
		"-f", "mpegts",
		"-i", "pipe:0",
		"-c:v", "copy",
		"-c:a", "copy",
		"-f", "hls",
		"-hls_time", fmt.Sprintf("%d", p.hlsSegmentTime),
		"-hls_list_size", fmt.Sprintf("%d", p.hlsListSize),
		"-hls_flags", "delete_segments+append_list",
		"-hls_segment_filename", filepath.Join(p.outputDir, "segment_%03d.ts"),
		filepath.Join(p.outputDir, "stream.m3u8"),
	)

	p.logger.WithField("command", cmd.String()).Debug("FFmpeg command prepared")

	// Create a pipe with increased buffer size for better throughput
	// Default pipe size is 64KB, we increase to 1MB for smoother data flow
	stdinReader, stdinWriter, err := os.Pipe()
	if err != nil {
		p.logger.WithError(err).Error("Failed to create stdin pipe for FFmpeg")
		return fmt.Errorf("failed to create stdin pipe: %w", err)
	}

	// Increase pipe buffer size to 1MB for better performance
	// F_SETPIPE_SZ = 1031 on Linux
	const F_SETPIPE_SZ = 1031
	pipeSize := 1024 * 1024 // 1MB
	_, _, errno := syscall.Syscall(syscall.SYS_FCNTL, stdinWriter.Fd(), F_SETPIPE_SZ, uintptr(pipeSize))
	if errno != 0 {
		p.logger.WithError(errno).Warn("Failed to increase pipe buffer size, using default")
	} else {
		p.logger.WithField("pipe_size", pipeSize).Debug("Increased pipe buffer size")
	}

	cmd.Stdin = stdinReader
	stdin := stdinWriter

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

	p.stdin = stdin
	p.cmd = cmd

	// Monitor FFmpeg output
	go p.monitorFFmpegOutput(stdout, stderr)

	if err := cmd.Start(); err != nil {
		p.logger.WithError(err).Error("Failed to start FFmpeg process")
		return fmt.Errorf("failed to start FFmpeg: %w", err)
	}

	p.logger.WithFields(logrus.Fields{
		"pid":         cmd.Process.Pid,
		"output_file": filepath.Join(p.outputDir, "stream.m3u8"),
	}).Info("✓ FFmpeg process started successfully")

	// Monitor FFmpeg process exit
	go func() {
		err := cmd.Wait()
		if err != nil {
			p.logger.WithError(err).Error("FFmpeg process exited with error")
		} else {
			p.logger.Info("FFmpeg process exited normally")
		}
		p.mu.Lock()
		p.running = false
		p.mu.Unlock()
	}()

	return nil
}

// monitorFFmpegOutput monitors FFmpeg stdout and stderr for logging
func (p *Player) monitorFFmpegOutput(stdout, stderr io.Reader) {
	p.logger.Info("Starting FFmpeg output monitor...")

	// Combine stdout and stderr
	reader := io.MultiReader(stdout, stderr)
	scanner := bufio.NewScanner(reader)

	lineCount := 0
	for scanner.Scan() {
		line := scanner.Text()
		lineCount++

		// Log based on content
		if strings.Contains(line, "error") || strings.Contains(line, "Error") {
			p.logger.WithField("ffmpeg_output", line).Error("FFmpeg error detected")
		} else if strings.Contains(line, "warning") || strings.Contains(line, "Warning") {
			p.logger.WithField("ffmpeg_output", line).Warn("FFmpeg warning detected")
		} else if strings.Contains(line, "frame=") {
			// Frame progress - log at debug level to avoid spam
			p.logger.WithField("ffmpeg_output", line).Debug("FFmpeg encoding progress")
		} else if strings.Contains(line, "Opening") || strings.Contains(line, "Input") {
			p.logger.WithField("ffmpeg_output", line).Info("FFmpeg input information")
		} else if strings.Contains(line, "Output") || strings.Contains(line, "Stream") {
			p.logger.WithField("ffmpeg_output", line).Info("FFmpeg output information")
		} else if line != "" {
			// Log other non-empty lines at trace level
			p.logger.WithField("ffmpeg_output", line).Trace("FFmpeg output")
		}
	}

	if err := scanner.Err(); err != nil {
		p.logger.WithError(err).Error("Error reading FFmpeg output")
	}

	p.logger.WithField("lines_processed", lineCount).Info("FFmpeg output monitor stopped")
}

// fileFeeder continuously feeds video files to FFmpeg stdin
func (p *Player) fileFeeder() {
	p.logger.Info("Starting file feeder goroutine...")

	for {
		select {
		case <-p.stopChan:
			p.logger.Info("Stop signal received, exiting file feeder")
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

			// Stream the video
			if err := p.streamVideo(video); err != nil {
				p.logger.WithError(err).WithFields(logrus.Fields{
					"file_id":  video.FileID,
					"filepath": video.FilePath,
					"is_ad":    video.IsAd == 1,
				}).Error("Failed to stream video")

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

				// Add small delay before trying next video to let FFmpeg recover
				time.Sleep(2 * time.Second)
			}
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

// streamVideo streams a single video file to FFmpeg
func (p *Player) streamVideo(video *models.VideoQueue) error {
	startTime := time.Now()

	p.logger.WithFields(logrus.Fields{
		"video_id":  video.ID,
		"file_id":   video.FileID,
		"filepath":  video.FilePath,
		"is_ad":     video.IsAd == 1,
		"timestamp": startTime.Format(time.RFC3339),
	}).Info("▶ Starting to stream video")

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
		p.logger.WithField("history_id", history.ID).Debug("Play history record created")
	}

	p.mu.Lock()
	p.currentFile = video
	p.currentHistory = history
	p.mu.Unlock()

	// Open video file
	file, err := os.Open(video.FilePath)
	if err != nil {
		return fmt.Errorf("failed to open file: %w", err)
	}
	defer file.Close()

	fileInfo, _ := file.Stat()
	p.logger.WithFields(logrus.Fields{
		"file_size": fileInfo.Size(),
		"file_mode": fileInfo.Mode().String(),
	}).Debug("Video file opened successfully")

	// Stream file to FFmpeg stdin with progress tracking
	// Using 8KB chunks for efficient I/O without excessive system calls
	bytesCopied := int64(0)
	buffer := make([]byte, 8*1024) // 8KB buffer
	lastLogTime := time.Now()
	logInterval := 5 * time.Second

streamLoop:
	for {
		// Check for skip signal (non-blocking)
		select {
		case <-p.skipChan:
			p.logger.WithField("filepath", video.FilePath).Warn("⏭ Skip requested, stopping current video")
			history.MarkAsSkipped()
			if _, err := helpers.GetXORM().ID(history.ID).Cols("finished_at", "duration_seconds", "skip_requested").Update(history); err != nil {
				p.logger.WithError(err).Error("Failed to update play history")
			}
			video.MarkAsPlayed()
			if _, err := helpers.GetXORM().ID(video.ID).Cols("played", "played_at").Update(video); err != nil {
				p.logger.WithError(err).Error("Failed to mark video as played")
			}
			return fmt.Errorf("video skipped by user")
		default:
			// Continue with reading
		}

		// Read chunk from file
		n, err := file.Read(buffer)
		if n > 0 {
			// Write with timeout protection to detect FFmpeg failures
			// Without -re flag, FFmpeg consumes data quickly, so 30s timeout is generous
			writeChan := make(chan error, 1)
			go func() {
				_, writeErr := p.stdin.Write(buffer[:n])
				writeChan <- writeErr
			}()

			select {
			case writeErr := <-writeChan:
				if writeErr != nil {
					p.logger.WithError(writeErr).Error("Failed to write to FFmpeg stdin")
					return fmt.Errorf("failed to write to FFmpeg stdin: %w", writeErr)
				}
			case <-time.After(30 * time.Second):
				p.logger.Error("Write to FFmpeg stdin timed out (30s) - FFmpeg may have stopped reading")
				return fmt.Errorf("write to FFmpeg stdin timed out - FFmpeg may have stopped processing")
			}
			bytesCopied += int64(n)

			// Log progress periodically
			if time.Since(lastLogTime) >= logInterval {
				progress := float64(bytesCopied) / float64(fileInfo.Size()) * 100
				p.logger.WithFields(logrus.Fields{
					"bytes_copied":    bytesCopied,
					"total_bytes":     fileInfo.Size(),
					"progress_pct":    fmt.Sprintf("%.2f%%", progress),
					"elapsed_seconds": time.Since(startTime).Seconds(),
				}).Info("Streaming progress")
				lastLogTime = time.Now()
			}
		}

		if err != nil {
			if err == io.EOF {
				// End of file reached - exit the loop
				break streamLoop
			}
			p.logger.WithError(err).Error("Error reading video file")
			return fmt.Errorf("error reading file: %w", err)
		}
	}

	duration := time.Since(startTime)
	p.logger.WithFields(logrus.Fields{
		"filepath":         video.FilePath,
		"bytes_streamed":   bytesCopied,
		"duration":         duration.String(),
		"duration_seconds": duration.Seconds(),
	}).Info("✓ Video streaming completed successfully")

	// Update play history
	history.MarkAsFinished()
	if _, err := helpers.GetXORM().ID(history.ID).Cols("finished_at", "duration_seconds").Update(history); err != nil {
		p.logger.WithError(err).Error("Failed to update play history")
	} else {
		p.logger.WithField("history_id", history.ID).Debug("Play history updated")
	}

	// Mark video as played
	video.MarkAsPlayed()
	if _, err := helpers.GetXORM().ID(video.ID).Cols("played", "played_at").Update(video); err != nil {
		p.logger.WithError(err).Error("Failed to mark video as played")
	} else {
		p.logger.WithField("video_id", video.ID).Debug("Video marked as played in queue")
	}

	p.mu.Lock()
	p.currentFile = nil
	p.currentHistory = nil
	p.mu.Unlock()

	// Add small delay to let FFmpeg flush buffers before next video
	// This helps FFmpeg transition between MPEGTS files smoothly
	p.logger.Debug("Waiting 1 second before loading next video to allow FFmpeg buffer flush")
	time.Sleep(1 * time.Second)

	return nil
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
	p.mu.Unlock()

	close(p.stopChan)

	if p.stdin != nil {
		if err := p.stdin.Close(); err != nil {
			p.logger.WithError(err).Warn("Error closing FFmpeg stdin")
		}
	}

	if p.cmd != nil && p.cmd.Process != nil {
		p.logger.WithField("pid", p.cmd.Process.Pid).Info("Terminating FFmpeg process")
		if err := p.cmd.Process.Kill(); err != nil {
			p.logger.WithError(err).Error("Failed to kill FFmpeg process")
			return err
		}
	}

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
