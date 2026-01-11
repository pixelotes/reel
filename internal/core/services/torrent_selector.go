package services

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
	matcher      *MatcherService
	logger       *utils.Logger
	filterLogger *log.Logger // New detailed logger
}

func NewTorrentSelector(cfg *config.Config, matcher *MatcherService, logger *utils.Logger) *TorrentSelector {
	ts := &TorrentSelector{
		config:  cfg,
		matcher: matcher,
		logger:  logger,
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

	// --- Standard SxxExx patterns ---
	standardPatterns := []*regexp.Regexp{
		regexp.MustCompile(fmt.Sprintf(`(?i)s0*%de0*%d(?:\D|$)`, season, episode)),
		regexp.MustCompile(fmt.Sprintf(`(?i)(?:\D|^)%dx0*%d(?:\D|$)`, season, episode)),
		regexp.MustCompile(fmt.Sprintf(`(?i)s%02de%02d`, season, episode)),
		regexp.MustCompile(fmt.Sprintf(`(?i)s%de%d`, season, episode)),
		regexp.MustCompile(fmt.Sprintf(`(?i)%dx%02d`, season, episode)),
		regexp.MustCompile(fmt.Sprintf(`(?i)%dx%d`, season, episode)),
	}

	// --- Lenient, absolute number patterns (for single-season shows) ---
	absolutePatterns := []*regexp.Regexp{
		// Matches " 01 ", " - 01.", "[01]", etc. It looks for the number surrounded by non-alphanumeric characters.
		regexp.MustCompile(fmt.Sprintf(`(?i)[^a-z0-9]%02d[^a-z0-9]`, episode)),
		regexp.MustCompile(fmt.Sprintf(`(?i)[^a-z0-9]%d[^a-z0-9]`, episode)),
		// Matches at the very end of the string, e.g., "Series Name 01"
		regexp.MustCompile(fmt.Sprintf(`(?i)\s%02d$`, episode)),
		regexp.MustCompile(fmt.Sprintf(`(?i)\s%d$`, episode)),
	}

	for _, r := range results {
		matched := false

		// 1. Try standard patterns first
		for _, pattern := range standardPatterns {
			if pattern.MatchString(r.Title) {
				matched = true
				break
			}
		}

		// 2. If no standard match, and it's season 1, try absolute number patterns
		if !matched && season == 1 {
			for _, pattern := range absolutePatterns {
				if pattern.MatchString(r.Title) {
					// Extra check to avoid matching resolutions like 1080p
					if !regexp.MustCompile(fmt.Sprintf(`(?i)%dp`, episode)).MatchString(r.Title) {
						matched = true
						break
					}
				}
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

// Enhanced filterBySeriesName with MatcherService
func (ts *TorrentSelector) filterBySeriesName(results []indexers.IndexerResult, searchTerms []string, stats *FilterStats) []indexers.IndexerResult {
	var filtered []indexers.IndexerResult

	for _, r := range results {
		matchFound := false

		for _, term := range searchTerms {
			if ts.matcher.Matches(term, r.Title) {
				matchFound = true
				break
			}
		}

		if matchFound {
			filtered = append(filtered, r)
		} else {
			stats.SeriesName++
			ts.logReject(fmt.Sprintf("Series name mismatch (terms: %s)", strings.Join(searchTerms, ", ")), r)
		}
	}
	return filtered
}
