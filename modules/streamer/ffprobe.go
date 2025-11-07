package streamer

import (
	"encoding/json"
	"fmt"
	"os/exec"
	"tv_streamer/helpers/logs"

	"github.com/sirupsen/logrus"
)

// FFProbeData represents the structure of ffprobe output
type FFProbeData struct {
	Format  FFProbeFormat   `json:"format"`
	Streams []FFProbeStream `json:"streams"`
}

// FFProbeFormat contains format information
type FFProbeFormat struct {
	Filename       string            `json:"filename"`
	FormatName     string            `json:"format_name"`
	FormatLongName string            `json:"format_long_name"`
	Duration       string            `json:"duration"`
	Size           string            `json:"size"`
	BitRate        string            `json:"bit_rate"`
	Tags           map[string]string `json:"tags,omitempty"`
}

// FFProbeStream contains stream information
type FFProbeStream struct {
	Index              int               `json:"index"`
	CodecName          string            `json:"codec_name"`
	CodecLongName      string            `json:"codec_long_name"`
	CodecType          string            `json:"codec_type"`
	Width              int               `json:"width,omitempty"`
	Height             int               `json:"height,omitempty"`
	DisplayAspectRatio string            `json:"display_aspect_ratio,omitempty"`
	FrameRate          string            `json:"r_frame_rate,omitempty"`
	AvgFrameRate       string            `json:"avg_frame_rate,omitempty"`
	BitRate            string            `json:"bit_rate,omitempty"`
	Duration           string            `json:"duration,omitempty"`
	Tags               map[string]string `json:"tags,omitempty"`
}

// GetFFProbeData runs ffprobe on a file and returns the JSON data
func GetFFProbeData(filepath string) (string, error) {
	logger := logs.GetLogger().WithFields(logrus.Fields{
		"module":   "streamer",
		"function": "GetFFProbeData",
		"filepath": filepath,
	})

	logger.Debug("Running ffprobe on file...")

	// Run ffprobe command
	cmd := exec.Command("ffprobe",
		"-v", "quiet",
		"-print_format", "json",
		"-show_format",
		"-show_streams",
		filepath,
	)

	output, err := cmd.CombinedOutput()
	if err != nil {
		logger.WithError(err).Warn("Failed to run ffprobe")
		return "{}", fmt.Errorf("ffprobe failed: %w", err)
	}

	// Validate JSON
	var probeData FFProbeData
	if err := json.Unmarshal(output, &probeData); err != nil {
		logger.WithError(err).Warn("Failed to parse ffprobe output")
		return "{}", fmt.Errorf("invalid ffprobe output: %w", err)
	}

	logger.WithFields(logrus.Fields{
		"format":        probeData.Format.FormatName,
		"duration":      probeData.Format.Duration,
		"streams_count": len(probeData.Streams),
	}).Info("✓ FFProbe data retrieved successfully")

	return string(output), nil
}

// ParseDuration parses duration from ffprobe format data
func ParseDuration(probeDataJSON string) int64 {
	var probeData FFProbeData
	if err := json.Unmarshal([]byte(probeDataJSON), &probeData); err != nil {
		return 0
	}

	var duration float64
	fmt.Sscanf(probeData.Format.Duration, "%f", &duration)
	return int64(duration)
}
