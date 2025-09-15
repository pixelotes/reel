package utils

import (
	"path/filepath"
	"regexp"
	"strings"
)

// SanitizeFilename removes characters that are invalid in file paths.
func SanitizeFilename(name string) string {
	// Replace characters that are invalid in most filesystems
	re := regexp.MustCompile(`[<>:"/\\|?*]`)
	sanitized := re.ReplaceAllString(name, "")
	// Also remove trailing spaces or periods, which can be problematic
	sanitized = strings.TrimRight(sanitized, " .")
	return sanitized
}

// IsVideoFile checks if a filename has a common video extension.
func IsVideoFile(filename string) bool {
	ext := strings.ToLower(filepath.Ext(filename))
	switch ext {
	case ".mkv", ".mp4", ".avi", ".mov", ".wmv", ".flv", ".webm":
		return true
	default:
		return false
	}
}
