package core

import (
	"fmt"
	"regexp"
	"sort"
	"strings"

	"reel/internal/clients/indexers"
	"reel/internal/config"
	"reel/internal/database/models"
	"reel/internal/utils"
)

type TorrentSelector struct {
	config *config.Config
	logger *utils.Logger
}

func NewTorrentSelector(cfg *config.Config, logger *utils.Logger) *TorrentSelector {
	return &TorrentSelector{
		config: cfg,
		logger: logger,
	}
}

// FilterAndScoreTorrents applies all filtering and scoring logic and returns a sorted list of results.
func (ts *TorrentSelector) FilterAndScoreTorrents(media *models.Media, results []indexers.IndexerResult, season, episode int) []indexers.IndexerResult {
	ts.logger.Info("Starting torrent filtering and scoring process with", len(results), "results")

	// Step 1: Filter out torrents matching reject patterns
	results = ts.filterByRejectPatterns(results)
	ts.logger.Info("After reject patterns filter:", len(results), "results remaining")

	if len(results) == 0 {
		ts.logger.Info("No torrents remaining after reject patterns filter")
		return []indexers.IndexerResult{}
	}

	// Step 2: For TV shows, filter by episode number and series name
	if media.Type == models.MediaTypeTVShow && season > 0 && episode > 0 {
		results = ts.filterByEpisodeNumber(results, season, episode)
		ts.logger.Info("After episode number filter:", len(results), "results remaining")

		if len(results) == 0 {
			ts.logger.Info("No torrents remaining after episode number filter")
			return []indexers.IndexerResult{}
		}

		results = ts.filterBySeriesName(results, media.Title)
		ts.logger.Info("After series name filter:", len(results), "results remaining")

		if len(results) == 0 {
			ts.logger.Info("No torrents remaining after series name filter")
			return []indexers.IndexerResult{}
		}
	}

	// Step 3: Filter by quality (resolution)
	results = ts.filterByQuality(results, media.MinQuality, media.MaxQuality)
	ts.logger.Info("After quality filter:", len(results), "results remaining")

	if len(results) == 0 {
		ts.logger.Info("No torrents remaining after quality filter")
		return []indexers.IndexerResult{}
	}

	// Step 4: Filter by minimum seeders
	results = ts.filterByMinSeeders(results)
	ts.logger.Info("After minimum seeders filter:", len(results), "results remaining")

	if len(results) == 0 {
		ts.logger.Info("No torrents remaining after minimum seeders filter")
		return []indexers.IndexerResult{}
	}

	// Step 5: Calculate scores and sort the results
	for i := range results {
		results[i].Score = getQualityScore(results[i].Title) + results[i].Seeders
	}

	// Sort by score (highest first)
	sort.Slice(results, func(i, j int) bool {
		return results[i].Score > results[j].Score
	})

	return results
}

// SelectBestTorrent filters and selects the best torrent based on various criteria
func (ts *TorrentSelector) SelectBestTorrent(media *models.Media, results []indexers.IndexerResult, season, episode int) *indexers.IndexerResult {
	filteredAndScored := ts.FilterAndScoreTorrents(media, results, season, episode)

	if len(filteredAndScored) == 0 {
		return nil
	}

	// Select the best torrent and log it
	bestTorrent := filteredAndScored[0]
	ts.logger.Info("Best torrent selected:", bestTorrent.Title, "Score:", bestTorrent.Score, "Seeders:", bestTorrent.Seeders, "Leechers:", bestTorrent.Leechers)

	// Log runners-up for debugging
	for i := 1; i < len(filteredAndScored) && i < 3; i++ {
		runnerUp := filteredAndScored[i]
		ts.logger.Info("Runner-up:", runnerUp.Title, "Score:", runnerUp.Score, "Seeders:", runnerUp.Seeders, "Leechers:", runnerUp.Leechers)
	}

	return &bestTorrent
}

// filterByRejectPatterns removes torrents that match any of the reject regex patterns
func (ts *TorrentSelector) filterByRejectPatterns(results []indexers.IndexerResult) []indexers.IndexerResult {
	var filtered []indexers.IndexerResult

	for _, r := range results {
		rejected := false
		for _, rejectPattern := range ts.config.Automation.RejectCommon {
			// Compile regex pattern
			regex, err := regexp.Compile("(?i)" + rejectPattern) // Case insensitive
			if err != nil {
				ts.logger.Error("Invalid regex pattern:", rejectPattern, "Error:", err)
				continue
			}

			if regex.MatchString(r.Title) {
				ts.logger.Debug("Rejected torrent by pattern", rejectPattern+":", r.Title)
				rejected = true
				break
			}
		}

		if !rejected {
			filtered = append(filtered, r)
		}
	}

	return filtered
}

// filterByEpisodeNumber filters torrents to only include those with the correct episode number
func (ts *TorrentSelector) filterByEpisodeNumber(results []indexers.IndexerResult, season, episode int) []indexers.IndexerResult {
	var filtered []indexers.IndexerResult

	// Create patterns for both formats: S01E01 and 1x01
	// Use fmt.Sprintf for proper number formatting

	// Patterns that match various formats with different zero-padding
	patterns := []*regexp.Regexp{
		// S01E01 format (with optional leading zeros)
		regexp.MustCompile(fmt.Sprintf(`(?i)s0*%de0*%d(?:\D|$)`, season, episode)),
		// 1x01 format (with optional leading zeros)
		regexp.MustCompile(fmt.Sprintf(`(?i)(?:\D|^)%dx0*%d(?:\D|$)`, season, episode)),
		// More explicit S01E01 patterns
		regexp.MustCompile(fmt.Sprintf(`(?i)s%02de%02d`, season, episode)),
		regexp.MustCompile(fmt.Sprintf(`(?i)s%de%d`, season, episode)),
		// More explicit 1x01 patterns
		regexp.MustCompile(fmt.Sprintf(`(?i)%dx%02d`, season, episode)),
		regexp.MustCompile(fmt.Sprintf(`(?i)%dx%d`, season, episode)),
	}

	for _, r := range results {
		title := r.Title
		matched := false

		// Check all patterns
		for _, pattern := range patterns {
			if pattern.MatchString(title) {
				matched = true
				break
			}
		}

		if matched {
			filtered = append(filtered, r)
			ts.logger.Info("Torrent matches episode pattern:", title)
		} else {
			ts.logger.Info("Torrent doesn't match episode pattern:", title)
		}
	}

	return filtered
}

// filterBySeriesName filters torrents to only include those containing the series name
func (ts *TorrentSelector) filterBySeriesName(results []indexers.IndexerResult, seriesTitle string) []indexers.IndexerResult {
	var filtered []indexers.IndexerResult

	// Extract meaningful words from series title
	meaningfulWords := ts.extractMeaningfulWords(seriesTitle)
	ts.logger.Info("Extracted meaningful words from series title:", meaningfulWords)

	if len(meaningfulWords) == 0 {
		ts.logger.Info("No meaningful words extracted from series title:", seriesTitle)
		return results // If we can't extract words, don't filter
	}

	for _, r := range results {
		titleLower := strings.ToLower(r.Title)

		// Check if all meaningful words are present in the torrent title
		allWordsFound := true
		for _, word := range meaningfulWords {
			if !strings.Contains(titleLower, strings.ToLower(word)) {
				allWordsFound = false
				break
			}
		}

		if allWordsFound {
			filtered = append(filtered, r)
			ts.logger.Info("Torrent matches series name:", r.Title)
		} else {
			ts.logger.Info("Torrent doesn't match series name:", r.Title)
		}
	}

	return filtered
}

// extractMeaningfulWords extracts meaningful words from a title, removing common stop words and symbols
func (ts *TorrentSelector) extractMeaningfulWords(title string) []string {
	// Common stop words to ignore
	stopWords := map[string]bool{
		"the": true, "a": true, "an": true, "and": true, "or": true, "but": true,
		"in": true, "on": true, "at": true, "to": true, "for": true, "of": true,
		"with": true, "by": true, "from": true, "up": true, "about": true, "into": true,
		"through": true, "during": true, "before": true, "after": true, "above": true,
		"below": true, "between": true, "among": true, "is": true, "are": true, "was": true,
		"were": true, "be": true, "been": true, "being": true, "have": true, "has": true,
		"had": true, "do": true, "does": true, "did": true, "will": true, "would": true,
		"should": true, "could": true, "can": true, "may": true, "might": true, "must": true,
	}

	// Remove symbols and split by spaces/punctuation
	cleanTitle := regexp.MustCompile(`[^\w\s]`).ReplaceAllString(title, " ")
	words := regexp.MustCompile(`\s+`).Split(cleanTitle, -1)

	var meaningfulWords []string
	for _, word := range words {
		word = strings.TrimSpace(strings.ToLower(word))
		if len(word) > 0 && !stopWords[word] {
			meaningfulWords = append(meaningfulWords, word)
		}
	}

	return meaningfulWords
}

// filterByQuality filters torrents by resolution quality
func (ts *TorrentSelector) filterByQuality(results []indexers.IndexerResult, minQuality, maxQuality string) []indexers.IndexerResult {
	minRank := RESOLUTION_RANK[minQuality]
	maxRank := RESOLUTION_RANK[maxQuality]

	var filtered []indexers.IndexerResult

	for _, r := range results {
		lowerTitle := strings.ToLower(r.Title)

		// Find the resolution in the title
		for _, res := range SUPPORTED_RESOLUTIONS {
			synonyms := RESOLUTION_SYNONYMS[res]
			for _, synonym := range synonyms {
				if strings.Contains(lowerTitle, strings.ToLower(synonym)) {
					rank := RESOLUTION_RANK[res]
					if rank >= minRank && rank <= maxRank {
						filtered = append(filtered, r)
						ts.logger.Info("Torrent matches quality filter:", r.Title, "Resolution:", res)
					} else {
						ts.logger.Info("Torrent quality out of range:", r.Title, "Resolution:", res, "Rank:", rank)
					}
					goto nextTorrent // Found a resolution, move to next torrent
				}
			}
		}

	nextTorrent:
	}

	return filtered
}

// filterByMinSeeders filters torrents by minimum number of seeders
func (ts *TorrentSelector) filterByMinSeeders(results []indexers.IndexerResult) []indexers.IndexerResult {
	var filtered []indexers.IndexerResult

	for _, r := range results {
		if r.Seeders >= ts.config.Automation.MinSeeders {
			filtered = append(filtered, r)
		} else {
			ts.logger.Info("Torrent filtered out by min seeders:", r.Title, "Seeders:", r.Seeders, "Required:", ts.config.Automation.MinSeeders)
		}
	}

	return filtered
}
