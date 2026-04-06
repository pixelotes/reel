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
	config         *config.Config
	matcher        *MatcherService
	logger         *utils.Logger
	filterLogger   *log.Logger
	rejectPatterns []*regexp.Regexp
}

func NewTorrentSelector(cfg *config.Config, matcher *MatcherService, logger *utils.Logger) *TorrentSelector {
	// Pre-compile reject patterns once
	rejectPatterns := make([]*regexp.Regexp, 0, len(cfg.Automation.RejectCommon))
	for _, pattern := range cfg.Automation.RejectCommon {
		re, err := regexp.Compile("(?i)" + pattern)
		if err != nil {
			logger.Error("Invalid reject regex pattern:", pattern, "Error:", err)
			continue
		}
		rejectPatterns = append(rejectPatterns, re)
	}

	ts := &TorrentSelector{
		config:         cfg,
		matcher:        matcher,
		logger:         logger,
		rejectPatterns: rejectPatterns,
	}

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

// logReject logs a rejected torrent to filter.log and debug output.
func (ts *TorrentSelector) logReject(reason string, result indexers.IndexerResult) {
	ts.logger.Debug(fmt.Sprintf("REJECT: [%s] %s (seeders: %d)", reason, result.Title, result.Seeders))
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

	// Step 3: Filter by quality (resolution) — skip for non-video media
	if media.Type != models.MediaTypeEbook && media.Type != models.MediaTypeManga {
		results = ts.filterByQuality(results, media.MinQuality, media.MaxQuality, stats)
	}

	// Step 4: Filter by minimum seeders
	results = ts.filterByMinSeeders(results, stats)

	// Step 5: Calculate scores and sort the results
	const QualityMultiplier = 1000
	const SeederCap = 100

	for i := range results {
		qualityScore := getQualityScore(results[i].Title)

		normalizedSeeders := results[i].Seeders
		if normalizedSeeders > SeederCap {
			normalizedSeeders = SeederCap
		}

		results[i].Score = (qualityScore * QualityMultiplier) + normalizedSeeders
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
	filtered := make([]indexers.IndexerResult, 0, len(results))
	for _, r := range results {
		if pattern := ts.matchesRejectPattern(r.Title); pattern != nil {
			stats.RejectPatterns++
			ts.logReject(fmt.Sprintf("Matches reject pattern '%s'", pattern.String()), r)
		} else {
			filtered = append(filtered, r)
		}
	}
	return filtered
}

func (ts *TorrentSelector) matchesRejectPattern(title string) *regexp.Regexp {
	for _, pattern := range ts.rejectPatterns {
		if pattern.MatchString(title) {
			return pattern
		}
	}
	return nil
}

// seasonPackPattern detects full season packs that should not match as individual episodes.
var seasonPackPattern = regexp.MustCompile(`(?i)(s\d+[\s.]+complete|s\d+[\s.]+full|s\d+[\s.]+pack|s\d+[\s.]+batch|complete[\s.]+season|complete[\s.]+series|season[\s.]+\d+[\s.]+complete|season[\s.]+\d+[\s.]+full)`)

// buildEpisodePatterns pre-compiles all regex patterns for a given season/episode pair.
func buildEpisodePatterns(season, episode int) (standard []*regexp.Regexp, absolute []*regexp.Regexp, resolutionGuard *regexp.Regexp) {
	standard = []*regexp.Regexp{
		regexp.MustCompile(fmt.Sprintf(`(?i)s0*%de0*%d(?:\D|$)`, season, episode)),
		regexp.MustCompile(fmt.Sprintf(`(?i)(?:\D|^)%dx0*%d(?:\D|$)`, season, episode)),
		regexp.MustCompile(fmt.Sprintf(`(?i)s%02de%02d`, season, episode)),
		regexp.MustCompile(fmt.Sprintf(`(?i)s%de%d`, season, episode)),
		regexp.MustCompile(fmt.Sprintf(`(?i)%dx%02d`, season, episode)),
		regexp.MustCompile(fmt.Sprintf(`(?i)%dx%d`, season, episode)),
	}
	absolute = []*regexp.Regexp{
		regexp.MustCompile(fmt.Sprintf(`(?i)[^a-z0-9]%02d[^a-z0-9]`, episode)),
		regexp.MustCompile(fmt.Sprintf(`(?i)[^a-z0-9]%d[^a-z0-9]`, episode)),
		regexp.MustCompile(fmt.Sprintf(`(?i)\s%02d$`, episode)),
		regexp.MustCompile(fmt.Sprintf(`(?i)\s%d$`, episode)),
	}
	resolutionGuard = regexp.MustCompile(fmt.Sprintf(`(?i)%dp`, episode))
	return
}

func matchesAny(patterns []*regexp.Regexp, title string) bool {
	for _, p := range patterns {
		if p.MatchString(title) {
			return true
		}
	}
	return false
}

// filterByEpisodeNumber filters torrents to only include those with the correct episode number
func (ts *TorrentSelector) filterByEpisodeNumber(results []indexers.IndexerResult, season, episode int, stats *FilterStats) []indexers.IndexerResult {
	standardPatterns, absolutePatterns, resolutionGuard := buildEpisodePatterns(season, episode)
	filtered := make([]indexers.IndexerResult, 0, len(results))

	for _, r := range results {
		// Reject season packs early — they contain season numbers but no specific episode
		if seasonPackPattern.MatchString(r.Title) {
			stats.EpisodeNumber++
			ts.logReject("Season pack (not a single episode)", r)
			continue
		}

		matched := matchesAny(standardPatterns, r.Title)

		// If no standard match, and it's season 1, try absolute number patterns
		if !matched && season == 1 {
			for _, pattern := range absolutePatterns {
				if pattern.MatchString(r.Title) && !resolutionGuard.MatchString(r.Title) {
					matched = true
					break
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
	filtered := make([]indexers.IndexerResult, 0, len(results))

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
	filtered := make([]indexers.IndexerResult, 0, len(results))
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
	filtered := make([]indexers.IndexerResult, 0, len(results))

	for _, r := range results {
		matchFound := false

		for _, term := range searchTerms {
			if ts.matcher.MatchesLoose(term, r.Title) {
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
