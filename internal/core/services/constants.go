package services

import (
	"strings"
)

// --- Quality Scoring Logic ---
var QUALITY_SCORES = map[string]int{
	// Resolution - These are now synonyms, the rank will be used for filtering
	"4k": 8, "2160p": 8, "uhd": 8,
	"1440p": 6, "2k": 6,
	"1080p": 5, "fhd": 5,
	"720p": 4, "hd": 4,
	"480p": 3, "sd": 2,
	"360p": 1,
	"xvid": 1,
	// Source quality
	"remux":  10,
	"bluray": 8, "bdrip": 8, "brrip": 6,
	"webdl": 7, "webrip": 5, "web": 6, // consolidated keys
	"hdtv": 4, "dvdrip": 3,
	"cam": 1, "ts": 1,
	// Codec
	"av1": 6, "x265": 5, "h265": 5, "hevc": 5, // Boosted x265/AV1
	"x264": 2, "h264": 2, "avc": 2,
	// Audio
	"atmos": 3, "truehd": 3, "dtshd": 3, "dtsx": 3, // Normalized keys
	"dts": 2, "eac3": 2, "ac3": 1, "aac": 1,
	// Special
	"repack": 1, "proper": 1, "extended": 1, "uncut": 1, "directors": 1,
	"hdr": 2, "hdr10": 2, "dolbyvision": 3, "dv": 3, "imax": 2,
}

var RESOLUTION_SYNONYMS = map[string][]string{
	"4320p": {"4320p", "8k"},
	"2160p": {"2160p", "4k", "uhd"},
	"1440p": {"1440p", "2k"},
	"1080p": {"1080p", "fhd"},
	"720p":  {"720p", "hd", "hdtv", "xvid"},
	"480p":  {"480p", "576p", "sd", "msd", "dvdrip", "ntsc", "pal"},
	"360p":  {"360p"},
}

var RESOLUTION_RANK = map[string]int{
	"360p":  0,
	"480p":  1,
	"720p":  2,
	"1080p": 3,
	"1440p": 4,
	"2160p": 5,
	"4320p": 6,
}

// Ordered from highest to lowest for matching
var SUPPORTED_RESOLUTIONS = []string{"2160p", "1440p", "1080p", "720p", "480p", "360p"}

func getQualityScore(title string) int {
	score := 0
	lowerTitle := strings.ToLower(title)

	// Normalize common compound terms to single tokens to safely handle separators
	lowerTitle = strings.ReplaceAll(lowerTitle, "web-dl", "webdl")
	lowerTitle = strings.ReplaceAll(lowerTitle, "dts-hd", "dtshd")
	lowerTitle = strings.ReplaceAll(lowerTitle, "dts-x", "dtsx")
	lowerTitle = strings.ReplaceAll(lowerTitle, "true-hd", "truehd")
	lowerTitle = strings.ReplaceAll(lowerTitle, "e-ac3", "eac3")

	// Replace separators with spaces
	lowerTitle = strings.ReplaceAll(lowerTitle, ".", " ")
	lowerTitle = strings.ReplaceAll(lowerTitle, "-", " ")
	lowerTitle = strings.ReplaceAll(lowerTitle, "_", " ")
	lowerTitle = strings.ReplaceAll(lowerTitle, "[", " ")
	lowerTitle = strings.ReplaceAll(lowerTitle, "]", " ")
	lowerTitle = strings.ReplaceAll(lowerTitle, "(", " ")
	lowerTitle = strings.ReplaceAll(lowerTitle, ")", " ")

	tokens := strings.Fields(lowerTitle)

	for _, token := range tokens {
		if val, ok := QUALITY_SCORES[token]; ok {
			score += val
		}
	}

	return score
}
