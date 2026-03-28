package utils

import (
	"regexp"
	"strings"
)

// Quality resolutions in priority order
var resolutions = []string{"2160p", "1080p", "720p", "480p", "360p"}

// Quality indicators and their patterns
var qualityPatterns = []struct {
	name    string
	pattern *regexp.Regexp
}{
	{"2160p", regexp.MustCompile(`(?i)\b(2160p?|4k|uhd)\b`)},
	{"1080p", regexp.MustCompile(`(?i)\b1080p?\b`)},
	{"720p", regexp.MustCompile(`(?i)\b720p?\b`)},
	{"480p", regexp.MustCompile(`(?i)\b480p?\b`)},
	{"360p", regexp.MustCompile(`(?i)\b360p?\b`)},
	{"WEB-DL", regexp.MustCompile(`(?i)\b(web-?dl|webdl)\b`)},
	{"WEBRip", regexp.MustCompile(`(?i)\bwebrip\b`)},
	{"BluRay", regexp.MustCompile(`(?i)\b(bluray|blu-ray|bdrip|brrip)\b`)},
	{"HDTV", regexp.MustCompile(`(?i)\bhdtv\b`)},
	{"DVDRip", regexp.MustCompile(`(?i)\bdvdrip\b`)},
}

// ParseQuality extracts quality information from torrent name
// Memory-efficient: uses pre-compiled regex, no allocations
func ParseQuality(torrentName string) string {
	if torrentName == "" {
		return "Unknown"
	}

	// Check for resolution first (highest priority)
	for _, res := range resolutions {
		if strings.Contains(strings.ToLower(torrentName), res) {
			return res
		}
	}

	// Check for other quality indicators using regex
	for _, qp := range qualityPatterns {
		if qp.pattern.MatchString(torrentName) {
			return qp.name
		}
	}

	return "Unknown"
}

// ParseQualityDetailed extracts all quality information
// Returns resolution and source (e.g., "1080p", "WEB-DL")
func ParseQualityDetailed(torrentName string) (resolution string, source string) {
	resolution = "Unknown"
	source = "Unknown"

	if torrentName == "" {
		return
	}

	lowerName := strings.ToLower(torrentName)

	// Find resolution
	for _, res := range resolutions {
		if strings.Contains(lowerName, res) {
			resolution = res
			break
		}
	}

	// Find source
	if strings.Contains(lowerName, "web-dl") || strings.Contains(lowerName, "webdl") {
		source = "WEB-DL"
	} else if strings.Contains(lowerName, "webrip") {
		source = "WEBRip"
	} else if strings.Contains(lowerName, "bluray") || strings.Contains(lowerName, "blu-ray") {
		source = "BluRay"
	} else if strings.Contains(lowerName, "bdrip") {
		source = "BDRip"
	} else if strings.Contains(lowerName, "brrip") {
		source = "BRRip"
	} else if strings.Contains(lowerName, "hdtv") {
		source = "HDTV"
	} else if strings.Contains(lowerName, "dvdrip") {
		source = "DVDRip"
	}

	return
}

// CompareQuality compares two quality strings
// Returns: 1 if q1 > q2, -1 if q1 < q2, 0 if equal
func CompareQuality(q1, q2 string) int {
	// Resolution priority
	resolutionPriority := map[string]int{
		"2160p":  5,
		"1080p":  4,
		"720p":   3,
		"480p":   2,
		"360p":   1,
		"Unknown": 0,
	}

	p1, ok1 := resolutionPriority[q1]
	p2, ok2 := resolutionPriority[q2]

	if ok1 && ok2 {
		if p1 > p2 {
			return 1
		} else if p1 < p2 {
			return -1
		}
		return 0
	}

	// Source quality priority (if not resolution)
	sourcePriority := map[string]int{
		"BluRay":  5,
		"WEB-DL":  4,
		"WEBRip":  3,
		"BDRip":   3,
		"BRRip":   3,
		"HDTV":    2,
		"DVDRip":  1,
		"Unknown": 0,
	}

	p1, ok1 = sourcePriority[q1]
	p2, ok2 = sourcePriority[q2]

	if ok1 && ok2 {
		if p1 > p2 {
			return 1
		} else if p1 < p2 {
			return -1
		}
	}

	return 0
}
