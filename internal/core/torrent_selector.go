package core

import (
	"fmt"
	"log"
	"regexp"
	"sort"
	"strings"

	"reel/internal/clients/indexers"
	"reel/internal/config"
	"reel/internal/database/models"
	"reel/internal/utils"
)

// FilterStats holds statistics about the torrent filtering process.
type FilterStats struct {
	InitialCount   int
	RejectPatterns int
	EpisodeNumber  int
	SeriesName     int
	Quality        int
	MinSeeders     int
	FinalCount     int
}

type TorrentSelector struct {
	config       *config.Config
	logger       *utils.Logger
	filterLogger *log.Logger // New detailed logger
}

func NewTorrentSelector(cfg *config.Config, logger *utils.Logger) *TorrentSelector {
	ts := &TorrentSelector{
		config: cfg,
		logger: logger,
	}

	// This is the effective "single line" to control detailed logging.
	// If the config value is not "detail", the filterLogger will be nil.
	if cfg.App.FilterLogLevel == "detail" {
		filterLogger, err := utils.NewFilterLogger(cfg.App.DataPath)
		if err != nil {
			logger.Error("Could not create filter.log:", err)
		} else {
			ts.filterLogger = filterLogger
			ts.filterLogger.Println("--- New Filter Session Started ---")
		}
	}

	return ts
}

// logReject logs a rejected torrent to filter.log if the logger is enabled.
func (ts *TorrentSelector) logReject(reason string, result indexers.IndexerResult) {
	if ts.filterLogger != nil {
		ts.filterLogger.Printf("REJECT: [%s] | %s", reason, result.Title)
	}
}

// logPass logs a passed torrent to filter.log if the logger is enabled.
func (ts *TorrentSelector) logPass(result indexers.IndexerResult) {
	if ts.filterLogger != nil {
		ts.filterLogger.Printf("PASS: [Score: %d] %s", result.Score, result.Title)
	}
}

// getResolutionRank finds the resolution in a title and returns its numerical rank.
func getResolutionRank(title string) int {
	lowerTitle := strings.ToLower(title)
	// Iterate from highest to lowest to catch the best quality first
	for _, res := range SUPPORTED_RESOLUTIONS {
		synonyms := RESOLUTION_SYNONYMS[res]
		for _, synonym := range synonyms {
			if strings.Contains(lowerTitle, strings.ToLower(synonym)) {
				return RESOLUTION_RANK[res]
			}
		}
	}
	// Return a low rank if no specific resolution is found, which will be filtered out.
	return -1
}

// FilterAndScoreTorrents applies all filtering and scoring logic and returns a sorted list of results.
func (ts *TorrentSelector) FilterAndScoreTorrents(media *models.Media, results []indexers.IndexerResult, season, episode int, searchTerms []string) []indexers.IndexerResult {
	stats := &FilterStats{InitialCount: len(results)}

	// Create a query string for logging purposes
	query := media.Title
	if media.Type == models.MediaTypeTVShow || media.Type == models.MediaTypeAnime {
		if season > 0 && episode > 0 {
			query = fmt.Sprintf("%s S%02dE%02d", media.Title, season, episode)
		}
	} else if media.Type == models.MediaTypeMovie {
		query = fmt.Sprintf("%s (%d)", media.Title, media.Year)
	}

	if ts.filterLogger != nil {
		ts.filterLogger.Printf("--- Filtering for: %s ---", query)
	}

	// Step 1: Filter out torrents matching reject patterns
	results = ts.filterByRejectPatterns(results, stats)

	// Step 2: For TV shows, filter by episode number and series name
	if (media.Type == models.MediaTypeTVShow || media.Type == models.MediaTypeAnime) && season > 0 && episode > 0 {
		results = ts.filterByEpisodeNumber(results, season, episode, stats)
		results = ts.filterBySeriesName(results, searchTerms, stats)
	}

	// Step 3: Filter by quality (resolution)
	results = ts.filterByQuality(results, media.MinQuality, media.MaxQuality, stats)

	// Step 4: Filter by minimum seeders
	results = ts.filterByMinSeeders(results, stats)

	// Step 5: Calculate scores and sort the results
	for i := range results {
		results[i].Score = getQualityScore(results[i].Title) + results[i].Seeders
	}

	sort.Slice(results, func(i, j int) bool {
		return results[i].Score > results[j].Score
	})

	// Log passed torrents
	for _, r := range results {
		ts.logPass(r)
	}

	stats.FinalCount = len(results)
	ts.logFilterStats(query, stats)

	return results
}

// logFilterStats formats and logs the final filtering statistics.
func (ts *TorrentSelector) logFilterStats(query string, stats *FilterStats) {
	var droppedReasons []string
	totalDropped := stats.InitialCount - stats.FinalCount

	if stats.RejectPatterns > 0 {
		droppedReasons = append(droppedReasons, fmt.Sprintf("%d rejectFilter", stats.RejectPatterns))
	}
	if stats.EpisodeNumber > 0 {
		droppedReasons = append(droppedReasons, fmt.Sprintf("%d numberFilter", stats.EpisodeNumber))
	}
	if stats.SeriesName > 0 {
		droppedReasons = append(droppedReasons, fmt.Sprintf("%d nameFilter", stats.SeriesName))
	}
	if stats.Quality > 0 {
		droppedReasons = append(droppedReasons, fmt.Sprintf("%d qualityFilter", stats.Quality))
	}
	if stats.MinSeeders > 0 {
		droppedReasons = append(droppedReasons, fmt.Sprintf("%d seederFilter", stats.MinSeeders))
	}

	if stats.InitialCount > 0 {
		logMessage := fmt.Sprintf("Filtering %d result(s) for '%s': %d drop (%s), %d pass",
			stats.InitialCount,
			query,
			totalDropped,
			strings.Join(droppedReasons, ", "),
			stats.FinalCount,
		)
		ts.logger.Debug(logMessage)
	}
}

// SelectBestTorrent filters and selects the best torrent based on various criteria
func (ts *TorrentSelector) SelectBestTorrent(media *models.Media, results []indexers.IndexerResult, season, episode int, searchTerms []string) *indexers.IndexerResult {
	filteredAndScored := ts.FilterAndScoreTorrents(media, results, season, episode, searchTerms)

	if len(filteredAndScored) == 0 {
		return nil
	}

	bestTorrent := filteredAndScored[0]
	ts.logger.Info("Best torrent selected:", bestTorrent.Title, "Score:", bestTorrent.Score)

	return &bestTorrent
}

// filterByRejectPatterns removes torrents that match any of the reject regex patterns
func (ts *TorrentSelector) filterByRejectPatterns(results []indexers.IndexerResult, stats *FilterStats) []indexers.IndexerResult {
	var filtered []indexers.IndexerResult
	for _, r := range results {
		rejected := false
		var matchedPattern string
		for _, rejectPattern := range ts.config.Automation.RejectCommon {
			regex, err := regexp.Compile("(?i)" + rejectPattern)
			if err != nil {
				ts.logger.Error("Invalid regex pattern:", rejectPattern, "Error:", err)
				continue
			}
			if regex.MatchString(r.Title) {
				rejected = true
				matchedPattern = rejectPattern
				break
			}
		}
		if !rejected {
			filtered = append(filtered, r)
		} else {
			stats.RejectPatterns++
			ts.logReject(fmt.Sprintf("Matches reject pattern '%s'", matchedPattern), r)
		}
	}
	return filtered
}

// filterByEpisodeNumber filters torrents to only include those with the correct episode number
func (ts *TorrentSelector) filterByEpisodeNumber(results []indexers.IndexerResult, season, episode int, stats *FilterStats) []indexers.IndexerResult {
	var filtered []indexers.IndexerResult
	patterns := []*regexp.Regexp{
		regexp.MustCompile(fmt.Sprintf(`(?i)s0*%de0*%d(?:\D|$)`, season, episode)),
		regexp.MustCompile(fmt.Sprintf(`(?i)(?:\D|^)%dx0*%d(?:\D|$)`, season, episode)),
		regexp.MustCompile(fmt.Sprintf(`(?i)s%02de%02d`, season, episode)),
		regexp.MustCompile(fmt.Sprintf(`(?i)s%de%d`, season, episode)),
		regexp.MustCompile(fmt.Sprintf(`(?i)%dx%02d`, season, episode)),
		regexp.MustCompile(fmt.Sprintf(`(?i)%dx%d`, season, episode)),
	}

	for _, r := range results {
		matched := false
		for _, pattern := range patterns {
			if pattern.MatchString(r.Title) {
				matched = true
				break
			}
		}
		if matched {
			filtered = append(filtered, r)
		} else {
			stats.EpisodeNumber++
			ts.logReject("Episode mismatch", r)
		}
	}
	return filtered
}

// filterByQuality filters torrents by resolution quality
func (ts *TorrentSelector) filterByQuality(results []indexers.IndexerResult, minQuality, maxQuality string, stats *FilterStats) []indexers.IndexerResult {
	minRank := RESOLUTION_RANK[minQuality]
	maxRank := RESOLUTION_RANK[maxQuality]
	var filtered []indexers.IndexerResult

	for _, r := range results {
		rank := getResolutionRank(r.Title)
		if rank >= minRank && rank <= maxRank {
			filtered = append(filtered, r)
		} else {
			stats.Quality++
			ts.logReject(fmt.Sprintf("Quality rank %d is outside range [%d, %d]", rank, minRank, maxRank), r)
		}
	}
	return filtered
}

// filterByMinSeeders filters torrents by minimum number of seeders
func (ts *TorrentSelector) filterByMinSeeders(results []indexers.IndexerResult, stats *FilterStats) []indexers.IndexerResult {
	var filtered []indexers.IndexerResult
	for _, r := range results {
		if r.Seeders >= ts.config.Automation.MinSeeders {
			filtered = append(filtered, r)
		} else {
			stats.MinSeeders++
			ts.logReject(fmt.Sprintf("Not enough seeders (%d < %d)", r.Seeders, ts.config.Automation.MinSeeders), r)
		}
	}
	return filtered
}

// This function splits camelCase words
func (ts *TorrentSelector) splitCamelCase(word string) []string {
	// Regular expression to find camelCase boundaries
	camelCaseRegex := regexp.MustCompile(`([a-z])([A-Z])`)

	// Insert spaces before uppercase letters that follow lowercase letters
	spaced := camelCaseRegex.ReplaceAllString(word, "$1 $2")

	// Split by spaces and return non-empty parts
	parts := strings.Fields(spaced)

	// If we got multiple parts, return them. Otherwise return the original word
	if len(parts) > 1 {
		return parts
	}
	return []string{word}
}

// Enhanced extractMeaningfulWords with punctuation removal and camelCase support
func (ts *TorrentSelector) extractMeaningfulWords(title string) []string {
	stopWords := map[string]bool{
		"the": true, "a": true, "an": true, "and": true, "or": true, "but": true,
		"in": true, "on": true, "at": true, "to": true, "for": true, "of": true,
		"with": true, "by": true, "from": true, "up": true, "about": true, "into": true,
	}

	// Step 1: Remove dots, commas, semicolons and other punctuation (but keep spaces and alphanumeric)
	// This converts "Dr. Stone" -> "Dr Stone" and "Steins;Gate" -> "SteinsGate"
	cleanTitle := regexp.MustCompile(`[^\w\s]`).ReplaceAllString(title, "")

	// Step 2: Split by spaces to get individual words
	words := regexp.MustCompile(`\s+`).Split(strings.TrimSpace(cleanTitle), -1)
	var meaningfulWords []string

	for _, word := range words {
		word = strings.TrimSpace(word)
		if len(word) > 1 {
			// Step 3: Check if this word contains camelCase and split if needed
			camelParts := ts.splitCamelCase(word)

			// Add the original word if it's not a stop word
			if !stopWords[strings.ToLower(word)] {
				meaningfulWords = append(meaningfulWords, word)
			}

			// If camelCase was split, also add the individual parts
			if len(camelParts) > 1 {
				for _, part := range camelParts {
					part = strings.TrimSpace(part)
					if len(part) > 1 && !stopWords[strings.ToLower(part)] {
						meaningfulWords = append(meaningfulWords, part)
					}
				}
			}
		}
	}

	// Remove duplicates
	seen := make(map[string]bool)
	var unique []string
	for _, word := range meaningfulWords {
		lowerWord := strings.ToLower(word)
		if !seen[lowerWord] {
			seen[lowerWord] = true
			unique = append(unique, word)
		}
	}

	return unique
}

// Enhanced filterBySeriesName with flexible matching
func (ts *TorrentSelector) filterBySeriesName(results []indexers.IndexerResult, searchTerms []string, stats *FilterStats) []indexers.IndexerResult {
	var filtered []indexers.IndexerResult
	var allMeaningfulWords []string
	for _, term := range searchTerms {
		allMeaningfulWords = append(allMeaningfulWords, ts.extractMeaningfulWords(term)...)
	}

	if len(allMeaningfulWords) == 0 {
		return results
	}

	for _, r := range results {
		titleLower := strings.ToLower(r.Title)
		matchFound := false

		for _, term := range searchTerms {
			// Strategy 1: All words must be found individually
			meaningfulWords := ts.extractMeaningfulWords(term)
			allWordsFound := true
			for _, word := range meaningfulWords {
				if !strings.Contains(titleLower, strings.ToLower(word)) {
					allWordsFound = false
					break
				}
			}
			if allWordsFound {
				matchFound = true
				break
			}

			// Strategy 2: Try original series title as-is (for exact matches)
			if strings.Contains(titleLower, strings.ToLower(term)) {
				matchFound = true
				break
			}

			// Strategy 3: Try camelCase variations
			camelParts := ts.splitCamelCase(term)
			if len(camelParts) > 1 {
				spacedVersion := strings.ToLower(strings.Join(camelParts, " "))
				if strings.Contains(titleLower, spacedVersion) {
					matchFound = true
					break
				}
			}
		}

		if matchFound {
			filtered = append(filtered, r)
		} else {
			stats.SeriesName++
			ts.logReject(fmt.Sprintf("Series name not found in title using terms: %s", strings.Join(searchTerms, ", ")), r)
		}
	}
	return filtered
}
