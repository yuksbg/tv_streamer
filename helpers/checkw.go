package helpers

import (
	"os/exec"
)

// IsFFmpegInstalled checks if ffmpeg is installed and available in the system's PATH.
// It returns true if ffmpeg is found, false otherwise.
func IsFFmpegInstalled() bool {
	cmd := exec.Command("ffmpeg", "-version")
	if err := cmd.Run(); err != nil {
		return false
	}
	return true
}
