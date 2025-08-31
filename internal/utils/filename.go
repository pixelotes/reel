package utils

import (
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
