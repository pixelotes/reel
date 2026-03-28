package services

import (
	"encoding/xml"
	"fmt"
	"net/http"
	"time"

	"golang.org/x/net/html/charset"

	"reel/internal/clients/indexers"
	"reel/internal/config"
	"reel/internal/database/models"
	"reel/internal/utils"
)

// --- RSS Parsing Structs ---
type rssItem struct {
	Title string `xml:"title"`
	Link  string `xml:"link"`
}
type rssChannel struct {
	Items []rssItem `xml:"item"`
}
type rssFeed struct {
	XMLName xml.Name   `xml:"rss"`
	Channel rssChannel `xml:"channel"`
}

type RSSService struct {
	config            *config.Config
	mediaRepo         *models.MediaRepository
	downloaderService *DownloaderService
	torrentSelector   *TorrentSelector
	matcher           *MatcherService
	logger            *utils.Logger
	httpClient        *http.Client
}

func NewRSSService(cfg *config.Config, repo *models.MediaRepository, downloader *DownloaderService, ts *TorrentSelector, matcher *MatcherService, logger *utils.Logger, httpClient *http.Client) *RSSService {
	return &RSSService{
		config:            cfg,
		mediaRepo:         repo,
		downloaderService: downloader,
		torrentSelector:   ts,
		matcher:           matcher,
		logger:            logger,
		httpClient:        httpClient,
	}
}

func (s *RSSService) ProcessRSSFeeds() {
	s.logger.Info("Starting RSS feed processing...")

	allSources := append(s.config.TVShows.Sources, s.config.Anime.Sources...)

	for _, source := range allSources {
		if source.Type == "rss" {
			s.logger.Info("Fetching RSS feed:", source.URL)

			resp, err := s.httpClient.Get(source.URL)
			if err != nil {
				s.logger.Error("Failed to fetch RSS feed", source.URL, ":", err)
				continue
			}
			defer resp.Body.Close()

			if resp.StatusCode != http.StatusOK {
				s.logger.Error("RSS feed request failed for", source.URL, "with status:", resp.StatusCode)
				continue
			}

			var feed rssFeed
			decoder := xml.NewDecoder(resp.Body)
			decoder.CharsetReader = charset.NewReaderLabel
			if err := decoder.Decode(&feed); err != nil {
				s.logger.Error("Failed to parse RSS feed", source.URL, ":", err)
				continue
			}

			s.matchFeedItems(feed.Channel.Items)
		}
	}
	s.logger.Info("Finished RSS feed processing.")
}

func (s *RSSService) matchFeedItems(items []rssItem) {
	// 1. Get all TV shows and anime from the library that are being monitored or are pending.
	mediaToMonitor, err := s.mediaRepo.GetByStatus(models.StatusMonitoring)
	if err != nil {
		s.logger.Error("Failed to get monitoring media for RSS check:", err)
		return
	}
	pendingMedia, err := s.mediaRepo.GetByStatus(models.StatusPending)
	if err != nil {
		s.logger.Error("Failed to get pending media for RSS check:", err)
		return
	}
	allMedia := append(mediaToMonitor, pendingMedia...)

	if len(allMedia) == 0 {
		return
	}

	// 2. Match feed items against the local media library.
	for _, item := range items {
		indexerResult := indexers.IndexerResult{
			Title:       item.Title,
			DownloadURL: item.Link,
			Indexer:     "RSS",
		}

		for _, media := range allMedia {
			searchTerms := []string{media.Title}
			if media.Type == models.MediaTypeAnime {
				animeSearchTerms, err := s.mediaRepo.GetAnimeSearchTerms(media.ID)
				if err == nil {
					for _, term := range animeSearchTerms {
						searchTerms = append(searchTerms, term.Term)
					}
				}
			}

			for _, term := range searchTerms {
				if !s.matcher.Matches(term, item.Title) {
					continue
				}

				show, err := s.mediaRepo.GetTVShowByMediaID(media.ID)
				if err != nil || show == nil {
					continue
				}

				for _, season := range show.Seasons {
					for _, episode := range season.Episodes {
						if episode.Status == models.StatusPending {
							bestTorrent := s.torrentSelector.SelectBestTorrent(&media, []indexers.IndexerResult{indexerResult}, season.SeasonNumber, episode.EpisodeNumber, searchTerms)
							if bestTorrent != nil {
								s.logger.Info("Found match in RSS feed for", media.Title, fmt.Sprintf("S%02dE%02d", season.SeasonNumber, episode.EpisodeNumber))
								s.downloaderService.StartEpisodeDownload(media.ID, season.SeasonNumber, episode.EpisodeNumber, *bestTorrent)
								time.Sleep(10 * time.Second) // Avoid overwhelming the download client
								goto nextItem                // Move to the next RSS item once a match is found and downloaded
							}
						}
					}
				}
			}
		}
	nextItem:
	}
}
