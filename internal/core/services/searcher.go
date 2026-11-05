package services

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"reel/internal/clients/indexers"
	"reel/internal/database/models"
	"reel/internal/utils"
)

// IndexerClientWithMode moved to types.go

type SearcherService struct {
	indexerClients  map[models.MediaType][]IndexerClientWithMode
	torrentSelector *TorrentSelector
	mediaRepo       *models.MediaRepository
	logger          *utils.Logger
	// searchBudget caps a whole search (all terms, all indexers). When it runs
	// out, the results already collected are returned instead of nothing.
	searchBudget time.Duration
}

// termDelay spaces consecutive search terms (anime alternate titles) so
// indexers aren't hammered. It is skipped after the last term.
const termDelay = 5 * time.Second

func NewSearcherService(clients map[models.MediaType][]IndexerClientWithMode, ts *TorrentSelector, repo *models.MediaRepository, logger *utils.Logger, searchBudget time.Duration) *SearcherService {
	if searchBudget <= 0 {
		searchBudget = 30 * time.Second
	}
	return &SearcherService{
		indexerClients:  clients,
		torrentSelector: ts,
		mediaRepo:       repo,
		logger:          logger,
		searchBudget:    searchBudget,
	}
}

// SearchBudget is how long a search may take before partial results are returned.
func (s *SearcherService) SearchBudget() time.Duration {
	return s.searchBudget
}

// getSearchTerms returns the media title plus any additional anime search terms.
func (s *SearcherService) getSearchTerms(media *models.Media) []string {
	terms := []string{media.Title}
	if media.Type == models.MediaTypeAnime {
		if animeTerms, err := s.mediaRepo.GetAnimeSearchTerms(media.ID); err == nil {
			for _, term := range animeTerms {
				terms = append(terms, term.Term)
			}
		}
	}
	return terms
}

func (s *SearcherService) performSearch(media *models.Media, season, episode int) ([]indexers.IndexerResult, error) {
	clients := s.indexerClients[media.Type]
	if len(clients) == 0 {
		s.logger.Warn("No search-based indexers configured for media type:", media.Type)
		return nil, nil
	}

	var allResults []indexers.IndexerResult

	searchTerms := s.getSearchTerms(media)

	tmdbIDStr := ""
	if media.TMDBId != nil {
		tmdbIDStr = strconv.Itoa(*media.TMDBId)
	}

	deadline := time.Now().Add(s.searchBudget)
	timedOut := false

	for i, searchTerm := range searchTerms {
		results, pending := s.searchTerm(media, clients, searchTerm, tmdbIDStr, season, episode, deadline)
		allResults = append(allResults, results...)
		if pending > 0 {
			timedOut = true
			s.logger.Warn(fmt.Sprintf("Search for %q ran out of time (%s): %d indexer(s) still pending, returning %d result(s) collected so far",
				searchTerm, s.searchBudget, pending, len(allResults)))
			break
		}
		if i < len(searchTerms)-1 {
			if time.Until(deadline) <= termDelay {
				timedOut = true
				s.logger.Warn(fmt.Sprintf("Search time budget (%s) exhausted after %d of %d search terms", s.searchBudget, i+1, len(searchTerms)))
				break
			}
			time.Sleep(termDelay)
		}
	}

	if timedOut {
		s.logger.Info(fmt.Sprintf("Found %d total results for %s (partial, search timed out)", len(allResults), media.Title))
	} else {
		s.logger.Info(fmt.Sprintf("Found %d total results for %s", len(allResults), media.Title))
	}
	return allResults, nil
}

// searchTerm queries every indexer for one term concurrently and returns what
// arrives before deadline, plus how many indexers had not answered by then.
// Indexers that answer late are abandoned; their results are discarded.
func (s *SearcherService) searchTerm(media *models.Media, clients []IndexerClientWithMode, searchTerm, tmdbIDStr string, season, episode int, deadline time.Time) ([]indexers.IndexerResult, int) {
	type answer struct {
		results []indexers.IndexerResult
		err     error
	}
	// Buffered so goroutines that finish after the deadline never block.
	answers := make(chan answer, len(clients))

	for _, clientWithMode := range clients {
		go func(cwm IndexerClientWithMode) {
			results, err := s.queryIndexer(media, cwm, searchTerm, tmdbIDStr, season, episode)
			answers <- answer{results, err}
		}(clientWithMode)
	}

	var collected []indexers.IndexerResult
	timer := time.NewTimer(time.Until(deadline))
	defer timer.Stop()

	for received := 0; received < len(clients); received++ {
		select {
		case a := <-answers:
			if a.err != nil {
				s.logger.Error("Search failed for indexer:", a.err)
				continue
			}
			collected = append(collected, a.results...)
		case <-timer.C:
			return collected, len(clients) - received
		}
	}
	return collected, 0
}

// queryIndexer runs one indexer's search for a term, including the TV
// "NxEE" fallback.
func (s *SearcherService) queryIndexer(media *models.Media, clientWithMode IndexerClientWithMode, searchTerm, tmdbIDStr string, season, episode int) ([]indexers.IndexerResult, error) {
	client := clientWithMode.Client
	searchMode := clientWithMode.Source.SearchMode

	query := searchTerm
	if media.Type == models.MediaTypeTVShow || media.Type == models.MediaTypeAnime {
		if searchMode == "search" && season > 0 && episode > 0 {
			query = fmt.Sprintf("%s S%02dE%02d", searchTerm, season, episode)
		}
		results, err := client.SearchTVShows(query, season, episode, searchMode)

		// Fallback for "search" mode if no results are found
		if len(results) == 0 && searchMode == "search" && season > 0 && episode > 0 {
			query = fmt.Sprintf("%s %dx%02d", searchTerm, season, episode)
			fallbackResults, fbErr := client.SearchTVShows(query, season, episode, searchMode)
			if fbErr == nil {
				return append(results, fallbackResults...), nil
			}
		}
		return results, err
	}

	// Movie or Ebook
	if media.Type == models.MediaTypeMovie && media.Year > 0 {
		query = fmt.Sprintf("%s %d", searchTerm, media.Year)
	}
	return client.SearchMovies(query, tmdbIDStr, searchMode)
}

func (s *SearcherService) PerformSearch(id int) ([]indexers.IndexerResult, error) {
	media, err := s.mediaRepo.GetByID(id)
	if err != nil {
		return nil, err
	}
	if media == nil {
		return nil, fmt.Errorf("media not found")
	}

	// For manual search, we don't know the episode yet, so just search for the show title
	results, err := s.performSearch(media, 0, 0)
	if err != nil {
		return nil, err
	}

	searchTerms := s.getSearchTerms(media)

	// Use the TorrentSelector to filter and score the results
	filteredResults := s.torrentSelector.FilterAndScoreTorrents(media, results, 0, 0, searchTerms)

	return filteredResults, nil
}

func (s *SearcherService) PerformEpisodeSearch(mediaID int, seasonNumber int, episodeNumber int) ([]indexers.IndexerResult, error) {
	media, err := s.mediaRepo.GetByID(mediaID)
	if err != nil {
		return nil, err
	}
	if media == nil {
		return nil, fmt.Errorf("media not found")
	}

	if media.Type != models.MediaTypeTVShow && media.Type != models.MediaTypeAnime {
		return nil, fmt.Errorf("media is not a TV show or anime")
	}

	// Perform search with specific season/episode
	results, err := s.performSearch(media, seasonNumber, episodeNumber)
	if err != nil {
		return nil, err
	}

	searchTerms := s.getSearchTerms(media)

	// Use the TorrentSelector to filter and score the results
	filteredResults := s.torrentSelector.FilterAndScoreTorrents(media, results, seasonNumber, episodeNumber, searchTerms)

	s.logger.Info(fmt.Sprintf("Found %d results for %s S%02dE%02d",
		len(filteredResults), media.Title, seasonNumber, episodeNumber))

	return filteredResults, nil
}

// Internal method for automatic search (used by worker, via a helper or direct usage if we expose it?)
// The user requirement said: "Worker de Cola: Mantener startSearchQueueWorker en el Manager"
// The worker calls `searchAndDownloadMovie` and `searchAndDownloadNextEpisode`.
// These methods were in `Manager`.
// Now `Manager` should call `SearcherService` to search, and `DownloaderService` to download.
// So `SearcherService` needs to expose `SearchMovie` and `SearchEpisode` that returns *BestTorrent*.
// `Manager` logic:
// 1. Search (get best torrent).
// 2. Download (start download).
// `SearcherService` has `performSearch` (internal) and generic `PerformSearch`.
// Does `SearcherService` expose "FindBestTorrent"?
// `PerformSearch` returns a list. `TorrentSelector.SelectBestTorrent` returns the best.
// So `Manager` can call `SearcherService.PerformSearch` then `TorrentSelector.SelectBestTorrent`?
// Or `SearcherService` can have `FindBestTorrent(media, season, ep) *IndexerResult`.
// Let's implement `FindBestTorrent` for convenience.

func (s *SearcherService) FindBestTorrent(media *models.Media, season, episode int) (*indexers.IndexerResult, error) {
	results, err := s.performSearch(media, season, episode)
	if err != nil {
		return nil, err
	}

	searchTerms := s.getSearchTerms(media)

	return s.torrentSelector.SelectBestTorrent(media, results, season, episode, searchTerms), nil
}

func (s *SearcherService) GetIndexerStatuses() map[string]ClientStatus {
	statuses := make(map[string]ClientStatus)

	// Deduplicate by URL
	processed := make(map[string]bool)

	for _, clients := range s.indexerClients {
		for _, cm := range clients {
			if processed[cm.Source.URL] {
				continue
			}
			processed[cm.Source.URL] = true

			ok, _ := cm.Client.HealthCheck()

			// Parse name (simplified logic from Manager)
			indexerName := "Unknown"
			// We can use the Type or extract from URL if needed, or just use URL as name if not available
			// Manager logic used: path parts.
			// Let's rely on Type mostly or just pass checking.

			statuses[cm.Source.URL] = ClientStatus{
				Type:   cm.Source.Type,
				Name:   indexerName, // Use indexerName
				Status: ok,
			}
		}
	}
	return statuses
}

func (s *SearcherService) TestIndexer(indexerKey string) (bool, error) {
	var clientToTest indexers.Client
	var sourceURL string

	for _, clients := range s.indexerClients {
		for _, clientWithMode := range clients {
			if strings.Contains(clientWithMode.Source.URL, indexerKey) {
				clientToTest = clientWithMode.Client
				sourceURL = clientWithMode.Source.URL
				break
			}
		}
		if clientToTest != nil {
			break
		}
	}

	if clientToTest == nil {
		return false, fmt.Errorf("indexer '%s' not found", indexerKey)
	}

	// Check health
	ok, err := clientToTest.HealthCheck()
	if err != nil {
		return false, fmt.Errorf("health check for %s failed: %w", sourceURL, err)
	}
	return ok, nil
}
