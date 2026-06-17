package services

import (
	"regexp"
	"strings"

	"reel/internal/config"
	"reel/internal/utils"
)

// Pre-compiled regex patterns for Normalize and IsIgnorable
var (
	nonAlphanumericRegex   = regexp.MustCompile(`[^a-z0-9\s]`)
	seasonEpisodeRegex     = regexp.MustCompile(`(?i)^(s\d+e\d+|\d+x\d+|s\d+|e\d+)$`)
	yearRegex              = regexp.MustCompile(`^(19|20)\d{2}$`)
	resolutionTokenRegex   = regexp.MustCompile(`(?i)^(\d{3,4}p|4k|8k)$`)
)

// Package-level lookup maps for IsIgnorable
var techTerms = map[string]bool{
	"hevc": true, "x264": true, "x265": true, "h264": true, "h265": true,
	"web": true, "webdl": true, "bluray": true, "hdtv": true, "remux": true,
	"aac": true, "ac3": true, "dts": true, "dolby": true,
	"proper": true, "repack": true, "subbed": true, "dubbed": true,
	"hdr": true, "10bit": true, "atmos": true,
}

var stopWords = map[string]bool{
	"the": true, "a": true, "an": true, "and": true, "&": true,
	"part": true, "vol": true, "season": true, "episode": true,
}

type MatcherService struct {
	config         *config.Config
	logger         *utils.Logger
	rejectPatterns []*regexp.Regexp
}

func NewMatcherService(cfg *config.Config, logger *utils.Logger) *MatcherService {
	// Pre-compile reject patterns once at initialization
	rejectPatterns := make([]*regexp.Regexp, 0, len(cfg.Automation.RejectCommon))
	for _, word := range cfg.Automation.RejectCommon {
		re, err := regexp.Compile("(?i)" + word)
		if err != nil {
			logger.Error("Invalid reject pattern:", word, "Error:", err)
			continue
		}
		rejectPatterns = append(rejectPatterns, re)
	}

	return &MatcherService{
		config:         cfg,
		logger:         logger,
		rejectPatterns: rejectPatterns,
	}
}

// Normalize cleans up the text for comparison: lowercase, alphanum only (keeping spaces).
func (m *MatcherService) Normalize(text string) string {
	text = strings.ToLower(text)
	// Replace dots, underscores, dashes with spaces
	text = strings.ReplaceAll(text, ".", " ")
	text = strings.ReplaceAll(text, "_", " ")
	text = strings.ReplaceAll(text, "-", " ")

	// Remove anything that is not a letter, number, or space
	text = nonAlphanumericRegex.ReplaceAllString(text, "")

	// Collapse multiple spaces
	return strings.Join(strings.Fields(text), " ")
}

// Tokenize splits the text into words.
func (m *MatcherService) Tokenize(text string) []string {
	return strings.Fields(m.Normalize(text))
}

// IsIgnorable checks if a token is safe to ignore (metadata, technical term, or stop word).
func (m *MatcherService) IsIgnorable(token string) bool {
	token = strings.ToLower(token)
	if seasonEpisodeRegex.MatchString(token) {
		return true
	}
	if yearRegex.MatchString(token) {
		return true
	}
	if resolutionTokenRegex.MatchString(token) {
		return true
	}
	if techTerms[token] {
		return true
	}
	return stopWords[token]
}

// Matches checks if the candidate title is a good match for the query.
// It enforces that:
// 1. ALL tokens in the query must form a subset of the candidate tokens.
// 2. Any "extra" tokens in the candidate must be "ignorable" (metadata or stop words).
func (m *MatcherService) Matches(query string, candidateTitle string) bool {
	// 1. Check Global Rejection List first (using pre-compiled patterns)
	for _, pattern := range m.rejectPatterns {
		if pattern.MatchString(candidateTitle) {
			return false
		}
	}

	queryTokens := m.Tokenize(query)
	candidateTokens := m.Tokenize(candidateTitle)

	if len(queryTokens) == 0 {
		return false
	}

	// Step 1: Check if Query is a subset of Candidate
	candidateMap := make(map[string]bool)
	for _, token := range candidateTokens {
		candidateMap[token] = true
	}

	for _, token := range queryTokens {
		if !candidateMap[token] {
			return false // Missing a required word from the query
		}
	}

	// Step 2: Strict Check - Ensure no extra "meaningful" words exist in candidate
	queryMap := make(map[string]bool)
	for _, token := range queryTokens {
		queryMap[token] = true
	}

	for _, token := range candidateTokens {
		if queryMap[token] {
			continue
		}
		if m.IsIgnorable(token) {
			continue
		}
		return false
	}

	return true
}

// MatchesLoose checks that the candidate contains the significant words from the query
// and does not match any reject pattern. It does NOT reject extra unknown words.
// Use this when episode filtering already guarantees the correct content.
func (m *MatcherService) MatchesLoose(query string, candidateTitle string) bool {
	for _, pattern := range m.rejectPatterns {
		if pattern.MatchString(candidateTitle) {
			return false
		}
	}

	queryTokens := m.Tokenize(query)
	candidateTokens := m.Tokenize(candidateTitle)

	if len(queryTokens) == 0 {
		return false
	}

	// Only require significant (non-ignorable) query tokens to be present
	candidateMap := make(map[string]bool)
	for _, token := range candidateTokens {
		candidateMap[token] = true
	}

	for _, token := range queryTokens {
		if m.IsIgnorable(token) {
			continue
		}
		if !candidateMap[token] {
			return false
		}
	}

	return true
}
