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
}

func NewSearcherService(clients map[models.MediaType][]IndexerClientWithMode, ts *TorrentSelector, repo *models.MediaRepository, logger *utils.Logger) *SearcherService {
	return &SearcherService{
		indexerClients:  clients,
		torrentSelector: ts,
		mediaRepo:       repo,
		logger:          logger,
	}
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

	for _, searchTerm := range searchTerms {
		for _, clientWithMode := range clients {
			client := clientWithMode.Client
			searchMode := clientWithMode.Source.SearchMode

			var results []indexers.IndexerResult
			var err error

			query := searchTerm
			if media.Type == models.MediaTypeTVShow || media.Type == models.MediaTypeAnime || media.Type == models.MediaTypeManga {
				if searchMode == "search" && season > 0 && episode > 0 {
					query = fmt.Sprintf("%s S%02dE%02d", searchTerm, season, episode)
				}
				results, err = client.SearchTVShows(query, season, episode, searchMode)

				// Fallback for "search" mode if no results are found
				if len(results) == 0 && searchMode == "search" && season > 0 && episode > 0 {
					query = fmt.Sprintf("%s %dx%02d", searchTerm, season, episode)
					var fallbackResults []indexers.IndexerResult
					fallbackResults, err = client.SearchTVShows(query, season, episode, searchMode)
					if err == nil {
						results = append(results, fallbackResults...)
					}
				}
			} else { // Movie or Ebook
				if media.Type == models.MediaTypeMovie && media.Year > 0 {
					query = fmt.Sprintf("%s %d", searchTerm, media.Year)
				}
				results, err = client.SearchMovies(query, tmdbIDStr, searchMode)
			}

			if err != nil {
				s.logger.Error("Search failed for indexer:", err)
				continue
			}
			allResults = append(allResults, results...)
		}
		time.Sleep(5 * time.Second) // 5-second delay between search terms
	}

	s.logger.Info(fmt.Sprintf("Found %d total results for %s", len(allResults), media.Title))
	return allResults, nil
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

	if media.Type != models.MediaTypeTVShow && media.Type != models.MediaTypeAnime && media.Type != models.MediaTypeManga {
		return nil, fmt.Errorf("media is not a TV show, anime, or manga")
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
