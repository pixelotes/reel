package services

import (
	"regexp"
	"strings"

	"reel/internal/config"
	"reel/internal/utils"
)

type MatcherService struct {
	config *config.Config
	logger *utils.Logger
}

func NewMatcherService(cfg *config.Config, logger *utils.Logger) *MatcherService {
	return &MatcherService{
		config: cfg,
		logger: logger,
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
	re := regexp.MustCompile(`[^a-z0-9\s]`)
	text = re.ReplaceAllString(text, "")

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
	// 1. Season/Episode patterns (s01e01, 1x01, s01, e01)
	if matched, _ := regexp.MatchString(`(?i)^(s\d+e\d+|\d+x\d+|s\d+|e\d+)$`, token); matched {
		return true
	}
	// 2. Year (1900-2099)
	if matched, _ := regexp.MatchString(`^(19|20)\d{2}$`, token); matched {
		return true
	}
	// 3. Resolution (720p, 1080p, 2160p, 4k, 8k)
	if matched, _ := regexp.MatchString(`(?i)^(\d{3,4}p|4k|8k)$`, token); matched {
		return true
	}
	// 4. Common technical terms
	techTerms := map[string]bool{
		"hevc": true, "x264": true, "x265": true, "h264": true, "h265": true,
		"web": true, "webdl": true, "bluray": true, "hdtv": true, "remux": true,
		"aac": true, "ac3": true, "dts": true, "dolby": true,
		"proper": true, "repack": true, "subbed": true, "dubbed": true,
		"hdr": true, "10bit": true, "atmos": true,
	}
	if techTerms[token] {
		return true
	}

	// 5. Stop words that are safe to have as "extra" in candidate
	stopWords := map[string]bool{
		"the": true, "a": true, "an": true, "and": true, "&": true,
		"part": true, "vol": true, "season": true, "episode": true,
	}
	return stopWords[token]
}

// Matches checks if the candidate title is a good match for the query.
// It enforces that:
// 1. ALL tokens in the query must form a subset of the candidate tokens.
// 2. Any "extra" tokens in the candidate must be "ignorable" (metadata or stop words).
func (m *MatcherService) Matches(query string, candidateTitle string) bool {
	// 1. Check Global Rejection List first
	for _, word := range m.config.Automation.RejectCommon {
		// Use regex for reject words as they might be patterns like "\bword\b"
		if matched, _ := regexp.MatchString("(?i)"+word, candidateTitle); matched {
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
		// If token is in the query, it's explained.
		if queryMap[token] {
			continue
		}

		// If token is ignorable, it's allowed noise.
		if m.IsIgnorable(token) {
			continue
		}

		// Found an extra word that is NOT in query and NOT ignorable -> REJECT
		return false
	}

	return true
}
