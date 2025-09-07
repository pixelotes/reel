package core

import (
	"database/sql"
	"encoding/xml"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/robfig/cron/v3"
	"golang.org/x/net/html/charset"

	"reel/internal/clients/indexers"
	"reel/internal/clients/metadata"
	"reel/internal/clients/notifications"
	"reel/internal/clients/torrent"
	"reel/internal/config"
	"reel/internal/database/models"
	"reel/internal/utils"
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
	"bluray": 8, "blu-ray": 8, "bdrip": 8, "brrip": 6,
	"webdl": 7, "web-dl": 7, "web": 6, "webrip": 5,
	"hdtv": 4, "dvdrip": 3,
	"cam": 1, "ts": 1,
	// Codec
	"av1": 5, "x265": 3, "h265": 3, "hevc": 3,
	"x264": 2, "h264": 2, "avc": 2,
	// Audio
	"atmos": 3, "truehd": 3, "dts-hd": 3, "dts-x": 3,
	"dts": 2, "ac3": 1, "aac": 1,
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

func getQualityScore(title string) int {
	score := 0
	lowerTitle := strings.ToLower(title)
	for key, value := range QUALITY_SCORES {
		if strings.Contains(lowerTitle, key) {
			score += value
		}
	}
	return score
}

type IndexerClientWithMode struct {
	Client indexers.Client
	Source config.SourceConfig
}

type Manager struct {
	config          *config.Config
	mediaRepo       *models.MediaRepository
	indexerClients  map[models.MediaType][]IndexerClientWithMode
	metadataClients map[models.MediaType][]metadata.Client
	torrentClient   torrent.TorrentClient
	torrentSelector *TorrentSelector
	notifiers       []notifications.Notifier
	postProcessor   *PostProcessor
	logger          *utils.Logger
	scheduler       *cron.Cron
	searchQueue     chan models.Media
	httpClient      *http.Client
}

type SubtitleTrack struct {
	Language string `json:"language"`
	Label    string `json:"label"`
	FilePath string `json:"-"` // Don't expose file path to frontend
}

type SystemStatus struct {
	TorrentClient   ClientStatus            `json:"torrent_client"`
	IndexerClients  map[string]ClientStatus `json:"indexer_clients"`
	MetadataClients []string                `json:"metadata_clients"`
}

type ClientStatus struct {
	Type   string `json:"type"`
	Name   string `json:"name"`
	Status bool   `json:"status"`
}

func NewManager(cfg *config.Config, db *sql.DB, logger *utils.Logger) *Manager {
	m := &Manager{
		config:          cfg,
		mediaRepo:       models.NewMediaRepository(db),
		torrentSelector: NewTorrentSelector(cfg, logger), // Assuming this exists
		notifiers:       make([]notifications.Notifier, 0),
		logger:          logger,
		scheduler:       cron.New(),
		searchQueue:     make(chan models.Media, 100),
		indexerClients:  make(map[models.MediaType][]IndexerClientWithMode),
		metadataClients: make(map[models.MediaType][]metadata.Client),
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}

	// --- Initialize Notifiers ---
	for _, notifierName := range cfg.Automation.Notifications {
		switch notifierName {
		case "pushbullet":
			if cfg.Notifications.Pushbullet.APIKey != "" {
				client := notifications.NewPushbulletClient(cfg.Notifications.Pushbullet.APIKey, logger)
				m.notifiers = append(m.notifiers, client)
				logger.Info("Pushbullet notifier enabled.")
			}
			// Add other notifiers here in the future
		}
	}

	m.postProcessor = NewPostProcessor(cfg, logger, models.NewMediaRepository(db), m.notifiers)

	// --- Initialize Clients based on new Config Structure ---

	// Create a TMDB client instance to be shared
	tmdbClient := metadata.NewTMDBClient(cfg.Metadata.TMDB.APIKey, cfg.Metadata.Language)

	// Helper function to initialize metadata providers
	initMetadataProvider := func(provider string) metadata.Client {
		switch provider {
		case "tmdb":
			return tmdbClient // Return the shared instance
		case "imdb":
			return metadata.NewIMDBClient(cfg.Metadata.IMDB.APIKey)
		case "tvmaze":
			return metadata.NewTVmazeClient()
		case "anilist":
			return metadata.NewAniListClient()
		case "trakt":
			return metadata.NewTraktClient(cfg.Metadata.Trakt.ClientID, tmdbClient) // Pass TMDB client
		}
		return nil
	}

	// Helper function to initialize indexer sources
	initIndexerClient := func(source config.SourceConfig) indexers.Client {
		switch source.Type {
		case "scarf":
			timeout, _ := time.ParseDuration("30s")
			return indexers.NewScarfClient(source.URL, source.APIKey, timeout)
		case "jackett":
			return indexers.NewJackettClient(source.URL, source.APIKey)
		case "prowlarr":
			return indexers.NewProwlarrClient(source.URL, source.APIKey)
		}
		return nil
	}

	// Initialize Movie Clients
	for _, providerName := range cfg.Movies.Providers {
		if client := initMetadataProvider(providerName); client != nil {
			m.metadataClients[models.MediaTypeMovie] = append(m.metadataClients[models.MediaTypeMovie], client)
		}
	}
	for _, source := range cfg.Movies.Sources {
		if source.Type != "rss" {
			if client := initIndexerClient(source); client != nil {
				m.indexerClients[models.MediaTypeMovie] = append(m.indexerClients[models.MediaTypeMovie], IndexerClientWithMode{
					Client: client,
					Source: source,
				})
			}
		}
	}

	// Initialize TV Show Clients
	for _, providerName := range cfg.TVShows.Providers {
		if client := initMetadataProvider(providerName); client != nil {
			m.metadataClients[models.MediaTypeTVShow] = append(m.metadataClients[models.MediaTypeTVShow], client)
		}
	}
	for _, source := range cfg.TVShows.Sources {
		if source.Type != "rss" {
			if client := initIndexerClient(source); client != nil {
				m.indexerClients[models.MediaTypeTVShow] = append(m.indexerClients[models.MediaTypeTVShow], IndexerClientWithMode{
					Client: client,
					Source: source,
				})
			}
		}
	}

	// Initialize Anime Clients
	for _, providerName := range cfg.Anime.Providers {
		if client := initMetadataProvider(providerName); client != nil {
			m.metadataClients[models.MediaTypeAnime] = append(m.metadataClients[models.MediaTypeAnime], client)
		}
	}
	for _, source := range cfg.Anime.Sources {
		if source.Type != "rss" {
			if client := initIndexerClient(source); client != nil {
				m.indexerClients[models.MediaTypeAnime] = append(m.indexerClients[models.MediaTypeAnime], IndexerClientWithMode{
					Client: client,
					Source: source,
				})
			}
		}
	}

	// Setup Torrent Client (this remains global)
	switch cfg.TorrentClient.Type {
	case "transmission":
		m.torrentClient = torrent.NewTransmissionClient(cfg.TorrentClient.Host, cfg.TorrentClient.Username, cfg.TorrentClient.Password)
	case "qbittorrent":
		m.torrentClient = torrent.NewQBittorrentClient(cfg.TorrentClient.Host, cfg.TorrentClient.Username, cfg.TorrentClient.Password)
	default:
		logger.Fatal("Unsupported torrent client type:", cfg.TorrentClient.Type)
	}

	go m.startSearchQueueWorker()

	return m
}

func (m *Manager) startSearchQueueWorker() {
	m.logger.Info("Search queue worker started.")
	for media := range m.searchQueue {
		switch media.Type {
		case models.MediaTypeMovie:
			m.searchAndDownloadMovie(&media)
		case models.MediaTypeTVShow, models.MediaTypeAnime:
			m.searchAndDownloadNextEpisode(&media)
		}
		time.Sleep(30 * time.Second)
	}
}

func (m *Manager) AddMedia(mediaType models.MediaType, id string, title string, year int, language, minQuality, maxQuality string, autoDownload bool, startSeason, startEpisode int) (*models.Media, error) {
	m.logger.Info("Parameters - Type:", mediaType, "ID:", id, "Title:", title, "Year:", year, "StartSeason:", startSeason, "StartEpisode:", startEpisode)

	var overview, posterURL *string
	var rating *float64
	var tvShowData *metadata.TVShowResult
	var metadataID *int

	m.logger.Info("Looking for metadata providers for type:", mediaType)
	providers := m.metadataClients[mediaType]
	m.logger.Info("Found", len(providers), "metadata providers")

	if len(providers) > 0 {
		client := providers[0]
		m.logger.Info("Using first metadata provider")

		switch mediaType {
		case models.MediaTypeMovie:
			m.logger.Info("Processing movie metadata...")
			movieData, err := client.SearchMovie(title, year)
			if err != nil {
				m.logger.Error("Movie metadata search failed:", err)
			} else if len(movieData) > 0 {
				m.logger.Info("Movie metadata found - ID:", movieData[0].ID, "Title:", movieData[0].Title)
				if tmdbID, parseErr := strconv.Atoi(movieData[0].ID); parseErr == nil {
					metadataID = &tmdbID
					m.logger.Info("Parsed TMDB ID:", *metadataID)
				} else {
					m.logger.Error("Failed to parse TMDB ID:", movieData[0].ID, "Error:", parseErr)
				}
				overview = &movieData[0].Overview
				posterURL = &movieData[0].PosterURL
				rating = &movieData[0].Rating
				if title == "" {
					title = movieData[0].Title
				}
				if year == 0 {
					year = movieData[0].Year
				}
				m.logger.Info("Movie data processed successfully")
			} else {
				m.logger.Info("No movie metadata found")
			}
		case models.MediaTypeTVShow, models.MediaTypeAnime:
			m.logger.Info("Processing TV show/anime metadata...")
			tvShowDataSlice, err := client.SearchTVShow(title)
			if err != nil {
				m.logger.Error("TV show/anime metadata search failed:", err)
			} else if len(tvShowDataSlice) > 0 {
				tvShowData = tvShowDataSlice[0]
				m.logger.Info("TV show/anime metadata found - ID:", tvShowData.ID, "Title:", tvShowData.Title)
				overview = &tvShowData.Overview
				posterURL = &tvShowData.PosterURL
				rating = &tvShowData.Rating
				if title == "" {
					title = tvShowData.Title
				}
				if year == 0 {
					year = tvShowData.Year
				}
				m.logger.Info("TV show/anime data processed successfully")
			} else {
				m.logger.Info("No TV show/anime metadata found")
			}
		}
	}

	var tvShowID *int
	if (mediaType == models.MediaTypeTVShow || mediaType == models.MediaTypeAnime) && tvShowData != nil {
		m.logger.Info("Creating TV show/anime database entries...")
		show := &models.TVShow{
			Status:   tvShowData.Status,
			TVmazeID: tvShowData.ID, // Using TVmazeID for both for now
		}

		m.logger.Info("Creating TV show/anime record...")
		if err := m.mediaRepo.CreateTVShow(show); err != nil {
			m.logger.Error("CRITICAL: Failed to create TV show/anime:", err)
			return nil, fmt.Errorf("failed to create tv show/anime: %w", err)
		}
		m.logger.Info("TV show/anime created with ID:", show.ID)
		tvShowID = &show.ID

		m.logger.Info("Creating", len(tvShowData.Seasons), "seasons...")
		for seasonNum, episodes := range tvShowData.Seasons {
			m.logger.Info("Creating season", seasonNum, "with", len(episodes), "episodes")
			season := &models.Season{ShowID: show.ID, SeasonNumber: seasonNum}
			if err := m.mediaRepo.CreateSeason(season); err != nil {
				m.logger.Error("CRITICAL: Failed to create season:", seasonNum, "Error:", err)
				return nil, fmt.Errorf("failed to create season: %w", err)
			}
			m.logger.Info("Season", seasonNum, "created with ID:", season.ID)

			for _, ep := range episodes {
				status := models.StatusPending
				if ep.AirDate != "" {
					airDate, _ := time.Parse("2006-01-02", ep.AirDate)
					if airDate.After(time.Now()) {
						status = models.StatusTBA
					}
				}
				if seasonNum < startSeason || (seasonNum == startSeason && ep.EpisodeNumber < startEpisode) {
					status = models.StatusSkipped
				}
				episode := &models.Episode{
					SeasonID:      season.ID,
					EpisodeNumber: ep.EpisodeNumber,
					Title:         ep.Title,
					AirDate:       ep.AirDate,
					Status:        status,
				}
				if err := m.mediaRepo.CreateEpisode(episode); err != nil {
					m.logger.Error("CRITICAL: Failed to create episode:", ep.EpisodeNumber, "Error:", err)
					return nil, fmt.Errorf("failed to create episode: %w", err)
				}
			}
		}
		m.logger.Info("All TV show/anime data created successfully")
	}

	m.logger.Info("Creating main media record...")
	media := &models.Media{
		Type:         mediaType,
		TMDBId:       metadataID,
		TVShowID:     tvShowID,
		Title:        title,
		Year:         year,
		Language:     language,
		MinQuality:   minQuality,
		MaxQuality:   maxQuality,
		Status:       models.StatusPending,
		Overview:     overview,
		PosterURL:    posterURL,
		Rating:       rating,
		AutoDownload: autoDownload,
	}

	m.logger.Info("About to create media record - TMDB ID:", metadataID, "TV Show ID:", tvShowID)

	if err := m.mediaRepo.Create(media); err != nil {
		m.logger.Error("CRITICAL: Failed to create media entry:", err)
		m.logger.Error("Media details - Title:", media.Title, "Type:", media.Type, "TMDB ID:", media.TMDBId, "TV Show ID:", media.TVShowID)
		return nil, fmt.Errorf("failed to create media: %w", err)
	}

	m.logger.Info("Media ID:", media.ID, "Title:", media.Title, "Type:", media.Type)

	if autoDownload {
		m.logger.Info("Adding to search queue...")
		select {
		case m.searchQueue <- *media:
			m.logger.Info("Added to search queue successfully")
		default:
			m.logger.Error("Search queue is full!")
		}
	}

	return media, nil
}

func (m *Manager) GetTVShowDetails(mediaID int) (*models.TVShow, error) {
	return m.mediaRepo.GetTVShowByMediaID(mediaID)
}

func (m *Manager) searchAndDownloadMovie(media *models.Media) {
	m.logger.Info("Starting automatic search for movie:", media.Title)
	m.mediaRepo.UpdateStatus(media.ID, models.StatusSearching)

	results, err := m.performSearch(media, 0, 0)
	if err != nil {
		m.logger.Error("Search failed for", media.Title, ":", err)
		m.mediaRepo.UpdateStatus(media.ID, models.StatusFailed)
		return
	}

	bestTorrent := m.torrentSelector.SelectBestTorrent(media, results, 0, 0, []string{media.Title})
	if bestTorrent == nil {
		m.logger.Info("No suitable torrent found for:", media.Title)
		m.mediaRepo.UpdateStatus(media.ID, models.StatusFailed)
		return
	}

	m.StartDownload(media.ID, *bestTorrent)
}

func (m *Manager) searchAndDownloadNextEpisode(media *models.Media) {
	show, err := m.mediaRepo.GetTVShowByMediaID(media.ID)
	if err != nil {
		m.logger.Error("Could not get TV show details for", media.Title, ":", err)
		return
	}

	downloadsStarted := 0
	for _, season := range show.Seasons {
		for _, episode := range season.Episodes {
			if downloadsStarted >= m.config.Automation.MaxConcurrentDownloads {
				return
			}
			if episode.Status == models.StatusPending {
				m.logger.Info("Searching for episode:", media.Title, fmt.Sprintf("S%02dE%02d", season.SeasonNumber, episode.EpisodeNumber))
				results, err := m.performSearch(media, season.SeasonNumber, episode.EpisodeNumber)
				if err != nil {
					m.logger.Error("Episode search failed:", err)
					continue
				}

				searchTerms := []string{media.Title}
				if media.Type == models.MediaTypeAnime {
					animeSearchTerms, err := m.mediaRepo.GetAnimeSearchTerms(media.ID)
					if err == nil {
						for _, term := range animeSearchTerms {
							searchTerms = append(searchTerms, term.Term)
						}
					}
				}

				bestTorrent := m.torrentSelector.SelectBestTorrent(media, results, season.SeasonNumber, episode.EpisodeNumber, searchTerms)
				if bestTorrent != nil {
					m.StartEpisodeDownload(media.ID, season.SeasonNumber, episode.EpisodeNumber, *bestTorrent)
					downloadsStarted++
					time.Sleep(5 * time.Second) // Add a 5-second delay between each download
				}
			}
		}
	}
	if downloadsStarted == 0 {
		m.logger.Info("No pending episodes to download for", media.Title)
	}
}

func (m *Manager) GetAllMedia() ([]models.Media, error) {

	result, err := m.mediaRepo.GetAll()
	if err != nil {
		m.logger.Error("Manager.GetAllMedia: Repository error:", err)
		return nil, err
	}

	//m.logger.Info("Manager.GetAllMedia: Retrieved", len(result), "items from repository")
	return result, nil
}

// pixelotes/reel/reel-912718c2894dddc773eede72733de790bc7912b3/internal/core/manager.go
func (m *Manager) cleanupCompletedTorrents() {
	if m.config.Automation.KeepTorrentsForDays <= 0 && m.config.Automation.KeepTorrentsSeedRatio <= 0 { // Modified line
		return // Feature is disabled
	}

	downloadedMedia, err := m.mediaRepo.GetByStatus(models.StatusDownloaded)
	if err != nil {
		m.logger.Error("Failed to get downloaded media for cleanup:", err)
		return
	}

	cleanupThreshold := time.Now().AddDate(0, 0, -m.config.Automation.KeepTorrentsForDays)

	for _, media := range downloadedMedia {
		if media.CompletedAt != nil && media.TorrentHash != nil {
			status, err := m.torrentClient.GetTorrentStatus(*media.TorrentHash)
			if err != nil {
				m.logger.Error("Failed to get torrent status for cleanup:", err)
				continue
			}

			shouldDelete := false
			if m.config.Automation.KeepTorrentsForDays > 0 && media.CompletedAt.Before(cleanupThreshold) {
				shouldDelete = true
			}
			if m.config.Automation.KeepTorrentsSeedRatio > 0 && status.UploadRatio >= m.config.Automation.KeepTorrentsSeedRatio {
				shouldDelete = true
			}

			if shouldDelete {
				m.logger.Info("Cleaning up torrent for:", media.Title)
				if err := m.torrentClient.RemoveTorrent(*media.TorrentHash); err != nil {
					m.logger.Error("Failed to remove torrent from client:", err)
				} else {
					m.mediaRepo.UpdateStatus(media.ID, models.StatusArchived)
				}
			}
		}
	}
}

func (m *Manager) StartScheduler() {
	m.scheduler.AddFunc("@every 30m", m.processPendingMedia)
	m.scheduler.AddFunc("@every 6h", m.checkForNewEpisodes)
	m.scheduler.AddFunc("@every 10s", m.updateDownloadStatus)
	m.scheduler.AddFunc("@every 1h", m.processRSSFeeds)
	m.scheduler.AddFunc("@every 24h", m.cleanupCompletedTorrents)
	m.scheduler.AddFunc("@every 1h", m.retryFailedDownloads)
	m.scheduler.Start()
	m.logger.Info("Scheduler started.")
	go m.processPendingMedia()
	go m.processRSSFeeds()
}

func (m *Manager) Stop() {
	if m.scheduler != nil {
		m.scheduler.Stop()
	}
}

func (m *Manager) processPendingMedia() {
	pendingMedia, err := m.mediaRepo.GetByStatus(models.StatusPending)
	if err != nil {
		m.logger.Error("Failed to get pending media:", err)
	}

	failedMedia, err := m.mediaRepo.GetByStatus(models.StatusFailed)
	if err != nil {
		m.logger.Error("Failed to get failed media:", err)
		return
	}

	mediaToProcess := append(pendingMedia, failedMedia...)

	if len(mediaToProcess) > 0 {
		m.logger.Info(fmt.Sprintf("Processing %d pending and failed media items.", len(mediaToProcess)))
		for i := range mediaToProcess {
			if mediaToProcess[i].AutoDownload {
				mediaCopy := mediaToProcess[i]
				m.searchQueue <- mediaCopy
			}
		}
	}
}

func (m *Manager) checkForNewEpisodes() {
	m.logger.Info("Checking for new episodes...")
	media, err := m.mediaRepo.GetAll()
	if err != nil {
		m.logger.Error("Failed to get all media for new episode check:", err)
		return
	}

	for _, item := range media {
		if item.Type == models.MediaTypeTVShow || item.Type == models.MediaTypeAnime {
			if item.Status == models.StatusMonitoring || item.Status == models.StatusPending {
				provider := m.metadataClients[item.Type][0] // Assuming first provider
				m.updateShowMetadata(&item, provider)
			}
		}
	}
}

// pixelotes/reel/reel-912718c2894dddc773eede72733de790bc7912b3/internal/core/manager.go
func (m *Manager) updateShowMetadata(media *models.Media, provider metadata.Client) {
	m.logger.Info("Updating metadata for show:", media.Title)
	remoteShowSlice, err := provider.SearchTVShow(media.Title)
	if err != nil {
		m.logger.Error("Failed to fetch remote show data for", media.Title, ":", err)
		return
	}

	if len(remoteShowSlice) == 0 {
		m.logger.Error("No remote show data found for", media.Title)
		return
	}
	remoteShow := remoteShowSlice[0]

	localShow, err := m.mediaRepo.GetTVShowByMediaID(media.ID)
	if err != nil {
		m.logger.Error("Failed to get local show data for", media.Title, ":", err)
		return
	}

	// Logic to compare and update seasons and episodes
	// ... (This would be a comprehensive comparison logic)
	// For now, let's just re-add and update statuses
	for seasonNum, episodes := range remoteShow.Seasons {
		var localSeason *models.Season
		for i := range localShow.Seasons {
			if localShow.Seasons[i].SeasonNumber == seasonNum {
				localSeason = &localShow.Seasons[i]
				break
			}
		}

		if localSeason == nil {
			// New season
			newSeason := &models.Season{ShowID: localShow.ID, SeasonNumber: seasonNum}
			m.mediaRepo.CreateSeason(newSeason)
			localShow.Seasons = append(localShow.Seasons, *newSeason)
			localSeason = newSeason
		}

		for _, remoteEpisode := range episodes {
			var localEpisode *models.Episode
			for i := range localSeason.Episodes {
				if localSeason.Episodes[i].EpisodeNumber == remoteEpisode.EpisodeNumber {
					localEpisode = &localSeason.Episodes[i]
					break
				}
			}

			if localEpisode == nil {
				// New episode
				status := models.StatusPending
				if remoteEpisode.AirDate != "" {
					airDate, _ := time.Parse("2006-01-02", remoteEpisode.AirDate)
					if airDate.After(time.Now()) {
						status = models.StatusTBA
					}
				}
				newEpisode := &models.Episode{
					SeasonID:      localSeason.ID,
					EpisodeNumber: remoteEpisode.EpisodeNumber,
					Title:         remoteEpisode.Title,
					AirDate:       remoteEpisode.AirDate,
					Status:        status,
				}
				m.mediaRepo.CreateEpisode(newEpisode)
				// If a new episode is found, set the media status to pending
				if media.Status == models.StatusMonitoring {
					m.mediaRepo.UpdateStatus(media.ID, models.StatusPending)
				}
			} else if localEpisode.Status == models.StatusTBA && remoteEpisode.AirDate != "" {
				airDate, _ := time.Parse("2006-01-02", remoteEpisode.AirDate)
				downloadDelay := time.Duration(m.config.Automation.EpisodeDownloadDelayHours) * time.Hour
				if airDate.Add(downloadDelay).Before(time.Now()) {
					m.mediaRepo.UpdateEpisodeDownloadInfo(media.ID, seasonNum, localEpisode.EpisodeNumber, models.StatusPending, nil, nil)
					// If a TBA episode becomes available, set the media status to pending
					if media.Status == models.StatusMonitoring {
						m.mediaRepo.UpdateStatus(media.ID, models.StatusPending)
					}
				}
			}
		}
	}
	m.updateShowProgress(media.ID)
}

func (m *Manager) updateShowProgress(mediaID int) {
	show, err := m.mediaRepo.GetTVShowByMediaID(mediaID)
	if err != nil {
		m.logger.Error("Failed to get show for progress update:", err)
		return
	}
	if show == nil {
		return // Not a show, nothing to do
	}

	var downloadableEpisodes, downloadedEpisodes, pendingEpisodes, tbaEpisodes int

	for _, season := range show.Seasons {
		for _, episode := range season.Episodes {
			// Count episodes for progress calculation
			if episode.Status != models.StatusSkipped && episode.Status != models.StatusTBA {
				downloadableEpisodes++
				if episode.Status == models.StatusDownloaded {
					downloadedEpisodes++
				}
			}
			// Count episodes for status determination
			if episode.Status == models.StatusPending || episode.Status == models.StatusDownloading {
				pendingEpisodes++
			}
			if episode.Status == models.StatusTBA {
				tbaEpisodes++
			}
		}
	}

	var progress float64
	if downloadableEpisodes > 0 {
		progress = float64(downloadedEpisodes) / float64(downloadableEpisodes)
	}

	// Determine the new overall status for the media item
	newStatus := models.StatusDownloading
	if pendingEpisodes == 0 {
		if tbaEpisodes > 0 || strings.ToLower(show.Status) == "running" {
			newStatus = models.StatusMonitoring
		} else {
			newStatus = models.StatusDownloaded
		}
	}

	// Use the generic UpdateProgress which now handles status correctly
	m.mediaRepo.UpdateProgress(mediaID, newStatus, progress, nil)
	m.logger.Info("Updated show progress for Media ID", mediaID, "New Status:", newStatus, "Progress:", progress)
}

func (m *Manager) updateDownloadStatus() {
	downloadingMedia, err := m.mediaRepo.GetByStatus(models.StatusDownloading)
	if err != nil {
		m.logger.Error("Failed to get downloading media:", err)
		return
	}

	for _, media := range downloadingMedia {
		if media.TorrentHash != nil {
			status, err := m.torrentClient.GetTorrentStatus(*media.TorrentHash)
			if err != nil {
				m.logger.Error("Failed to get torrent status for", media.Title, ":", err)
				m.mediaRepo.UpdateStatus(media.ID, models.StatusFailed)
				continue
			}

			// Only process completion once - check current status is still downloading
			if status.IsCompleted {
				var completedAt *time.Time
				now := time.Now()
				completedAt = &now

				if media.Type == models.MediaTypeTVShow || media.Type == models.MediaTypeAnime {
					show, err := m.mediaRepo.GetTVShowByMediaID(media.ID)
					if err == nil {
						// Find the downloading episode and mark it as downloaded
						for _, season := range show.Seasons {
							for _, episode := range season.Episodes {
								// Only process if episode is still downloading
								if episode.Status == models.StatusDownloading {
									// Start post-processing in a new goroutine to avoid blocking
									go m.postProcessor.ProcessDownload(media, status, season.SeasonNumber, episode.EpisodeNumber, status.DownloadDir)
									m.mediaRepo.UpdateEpisodeDownloadInfo(media.ID, season.SeasonNumber, episode.EpisodeNumber, models.StatusDownloaded, nil, nil)
									goto ShowStatusUpdate
								}
							}
						}
					}
				} else {
					// For movies - only process if still downloading
					if media.Status == models.StatusDownloading {
						// Start post-processing in a new goroutine to avoid blocking
						go m.postProcessor.ProcessDownload(media, status, 0, 0, status.DownloadDir)
						// For movies, just update the main media item
						m.mediaRepo.UpdateProgress(media.ID, models.StatusDownloaded, 1.0, completedAt)
					}
				}

			ShowStatusUpdate:
				// After any episode completes, always recalculate the show's overall status
				if media.Type == models.MediaTypeTVShow || media.Type == models.MediaTypeAnime {
					m.updateShowProgress(media.ID)
				}

			} else {
				// If not completed, just update the progress percentage
				m.mediaRepo.UpdateProgress(media.ID, models.StatusDownloading, status.Progress, nil)
			}
		}
	}
}

func (m *Manager) DeleteMedia(id int) error {
	return m.mediaRepo.Delete(id)
}

func (m *Manager) RetryMedia(id int) error {
	media, err := m.mediaRepo.GetByID(id)
	if err != nil {
		return err
	}
	if media == nil {
		return fmt.Errorf("media with id %d not found", id)
	}

	if media.Status == models.StatusFailed || media.Status == models.StatusPending {
		if err := m.mediaRepo.UpdateStatus(media.ID, models.StatusPending); err != nil {
			return err
		}
		m.searchQueue <- *media
	}
	return nil
}

func (m *Manager) ClearFailedMedia() error {
	failedMedia, err := m.mediaRepo.GetByStatus(models.StatusFailed)
	if err != nil {
		return err
	}
	for _, media := range failedMedia {
		if err := m.mediaRepo.Delete(media.ID); err != nil {
			m.logger.Error("failed to delete media %d: %v", media.ID, err)
		}
	}
	return nil
}

func (m *Manager) SearchMetadata(query string, mediaType string) ([]interface{}, error) {
	providers := m.metadataClients[models.MediaType(mediaType)]
	if len(providers) == 0 {
		return nil, fmt.Errorf("no metadata provider configured for '%s'", mediaType)
	}

	client := providers[0] // Use first provider
	var results []interface{}
	if mediaType == string(models.MediaTypeMovie) {
		res, err := client.SearchMovie(query, 0)
		if err != nil {
			return nil, err
		}
		for _, r := range res {
			results = append(results, r)
		}
	} else if mediaType == string(models.MediaTypeTVShow) || mediaType == string(models.MediaTypeAnime) {
		res, err := client.SearchTVShow(query)
		if err != nil {
			return nil, err
		}
		for _, r := range res {
			results = append(results, r)
		}
	} else {
		return nil, fmt.Errorf("unsupported media type for metadata search: %s", mediaType)
	}
	return results, nil
}

func (m *Manager) GetSystemStatus() (*SystemStatus, error) {
	status := &SystemStatus{
		IndexerClients:  make(map[string]ClientStatus),
		MetadataClients: []string{},
	}

	// Torrent Client Status
	torrentStatus, _ := m.torrentClient.HealthCheck()
	status.TorrentClient = ClientStatus{
		Type:   m.config.TorrentClient.Type,
		Status: torrentStatus,
	}

	// Indexer Clients Status (deduplicated)
	uniqueIndexers := make(map[string]config.SourceConfig)
	allSources := append(m.config.Movies.Sources, m.config.TVShows.Sources...)
	allSources = append(allSources, m.config.Anime.Sources...)
	for _, source := range allSources {
		if source.Type != "rss" {
			uniqueIndexers[source.URL] = source
		}
	}

	for key, source := range uniqueIndexers {
		var client indexers.Client
		switch source.Type {
		case "scarf":
			client = indexers.NewScarfClient(source.URL, source.APIKey, 30*time.Second)
		case "jackett":
			client = indexers.NewJackettClient(source.URL, source.APIKey)
		case "prowlarr":
			client = indexers.NewProwlarrClient(source.URL, source.APIKey)
		}
		if client != nil {
			ok, _ := client.HealthCheck()

			// Parse the indexer name from the URL
			var indexerName string
			parsedURL, err := url.Parse(source.URL)
			if err == nil {
				pathParts := strings.Split(strings.Trim(parsedURL.Path, "/"), "/")
				if len(pathParts) > 0 {
					indexerName = pathParts[len(pathParts)-1]
				}
			}

			status.IndexerClients[key] = ClientStatus{
				Type:   source.Type,
				Name:   indexerName,
				Status: ok,
			}
		}
	}

	// Metadata Clients
	uniqueProviders := make(map[string]bool)
	for _, provider := range m.config.Movies.Providers {
		uniqueProviders[provider] = true
	}
	for _, provider := range m.config.TVShows.Providers {
		uniqueProviders[provider] = true
	}
	for _, provider := range m.config.Anime.Providers {
		uniqueProviders[provider] = true
	}
	for provider := range uniqueProviders {
		status.MetadataClients = append(status.MetadataClients, provider)
	}

	return status, nil
}

func (m *Manager) performSearch(media *models.Media, season, episode int) ([]indexers.IndexerResult, error) {
	clients := m.indexerClients[media.Type]
	if len(clients) == 0 {
		m.logger.Warn("No search-based indexers configured for media type:", media.Type)
		return nil, nil
	}

	var allResults []indexers.IndexerResult

	// Get search terms
	searchTerms := []string{media.Title}
	if media.Type == models.MediaTypeAnime {
		animeSearchTerms, err := m.mediaRepo.GetAnimeSearchTerms(media.ID)
		if err == nil {
			for _, term := range animeSearchTerms {
				searchTerms = append(searchTerms, term.Term)
			}
		}
	}

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
			if media.Type == models.MediaTypeTVShow || media.Type == models.MediaTypeAnime {
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
			} else { // Movie
				if media.Year > 0 {
					query = fmt.Sprintf("%s %d", searchTerm, media.Year)
				}
				results, err = client.SearchMovies(query, tmdbIDStr, searchMode)
			}

			if err != nil {
				m.logger.Error("Search failed for indexer:", err)
				continue
			}
			allResults = append(allResults, results...)
		}
		time.Sleep(5 * time.Second) // 5-second delay between search terms
	}

	m.logger.Info(fmt.Sprintf("Found %d total results for %s", len(allResults), media.Title))
	return allResults, nil
}

func (m *Manager) processRSSFeeds() {
	m.logger.Info("Starting RSS feed processing...")

	allSources := append(m.config.TVShows.Sources, m.config.Anime.Sources...)

	for _, source := range allSources {
		if source.Type == "rss" {
			m.logger.Info("Fetching RSS feed:", source.URL)

			resp, err := m.httpClient.Get(source.URL)
			if err != nil {
				m.logger.Error("Failed to fetch RSS feed", source.URL, ":", err)
				continue
			}
			defer resp.Body.Close()

			if resp.StatusCode != http.StatusOK {
				m.logger.Error("RSS feed request failed for", source.URL, "with status:", resp.StatusCode)
				continue
			}

			var feed rssFeed
			decoder := xml.NewDecoder(resp.Body)
			decoder.CharsetReader = charset.NewReaderLabel
			if err := decoder.Decode(&feed); err != nil {
				m.logger.Error("Failed to parse RSS feed", source.URL, ":", err)
				continue
			}

			m.matchFeedItems(feed.Channel.Items)
		}
	}
	m.logger.Info("Finished RSS feed processing.")
}

func (m *Manager) matchFeedItems(items []rssItem) {
	// 1. Get all TV shows and anime from the library that are being monitored or are pending.
	mediaToMonitor, err := m.mediaRepo.GetByStatus(models.StatusMonitoring)
	if err != nil {
		m.logger.Error("Failed to get monitoring media for RSS check:", err)
		return
	}
	pendingMedia, err := m.mediaRepo.GetByStatus(models.StatusPending)
	if err != nil {
		m.logger.Error("Failed to get pending media for RSS check:", err)
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
				animeSearchTerms, err := m.mediaRepo.GetAnimeSearchTerms(media.ID)
				if err == nil {
					for _, term := range animeSearchTerms {
						searchTerms = append(searchTerms, term.Term)
					}
				}
			}

			for _, term := range searchTerms {
				if !strings.Contains(strings.ToLower(item.Title), strings.ToLower(term)) {
					continue
				}

				show, err := m.mediaRepo.GetTVShowByMediaID(media.ID)
				if err != nil || show == nil {
					continue
				}

				for _, season := range show.Seasons {
					for _, episode := range season.Episodes {
						if episode.Status == models.StatusPending {
							bestTorrent := m.torrentSelector.SelectBestTorrent(&media, []indexers.IndexerResult{indexerResult}, season.SeasonNumber, episode.EpisodeNumber, searchTerms)
							if bestTorrent != nil {
								m.logger.Info("Found match in RSS feed for", media.Title, fmt.Sprintf("S%02dE%02d", season.SeasonNumber, episode.EpisodeNumber))
								m.StartEpisodeDownload(media.ID, season.SeasonNumber, episode.EpisodeNumber, *bestTorrent)
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

func (m *Manager) PerformSearch(id int) ([]indexers.IndexerResult, error) {
	media, err := m.mediaRepo.GetByID(id)
	if err != nil {
		return nil, err
	}
	if media == nil {
		return nil, fmt.Errorf("media not found")
	}

	// For manual search, we don't know the episode yet, so just search for the show title
	results, err := m.performSearch(media, 0, 0)
	if err != nil {
		return nil, err
	}

	searchTerms := []string{media.Title}
	if media.Type == models.MediaTypeAnime {
		animeSearchTerms, err := m.mediaRepo.GetAnimeSearchTerms(media.ID)
		if err == nil {
			for _, term := range animeSearchTerms {
				searchTerms = append(searchTerms, term.Term)
			}
		}
	}

	// Use the TorrentSelector to filter and score the results
	filteredResults := m.torrentSelector.FilterAndScoreTorrents(media, results, 0, 0, searchTerms)

	return filteredResults, nil
}

func (m *Manager) StartDownload(id int, torrent indexers.IndexerResult) error {
	media, err := m.mediaRepo.GetByID(id)
	if err != nil {
		return err
	}
	if media == nil {
		return fmt.Errorf("media not found")
	}

	var downloadPath string
	switch media.Type {
	case models.MediaTypeMovie:
		downloadPath = m.config.Movies.DownloadFolder
	case models.MediaTypeTVShow:
		downloadPath = m.config.TVShows.DownloadFolder
	case models.MediaTypeAnime:
		downloadPath = m.config.Anime.DownloadFolder
	default:
		downloadPath = m.config.TorrentClient.DownloadPath // Fallback
	}

	m.logger.Info("Sending to download client:", m.config.TorrentClient.Type)

	var hash string

	if m.config.App.MagnetToTorrentEnabled && strings.HasPrefix(torrent.DownloadURL, "magnet:") {
		timeout := time.Duration(m.config.App.MagnetToTorrentTimeout) * time.Second
		if timeout <= 0 {
			timeout = 60 * time.Second // Default to 60 seconds
		}
		m.logger.Info("Attempting to convert magnet to .torrent with timeout:", timeout)
		torrentFileBytes, convErr := utils.ConvertMagnetToTorrent(torrent.DownloadURL, timeout, m.config.App.DataPath)
		if convErr == nil {
			m.logger.Info("Magnet conversion successful, adding as .torrent file.")
			hash, err = m.torrentClient.AddTorrentFile(torrentFileBytes, downloadPath)
		} else {
			m.logger.Warn("Magnet conversion failed:", convErr, "- falling back to magnet link.")
			hash, err = m.torrentClient.AddTorrent(torrent.DownloadURL, downloadPath)
		}
	} else {
		hash, err = m.torrentClient.AddTorrent(torrent.DownloadURL, downloadPath)
	}

	if err != nil {
		m.logger.Error("Failed to add torrent to client:", err)
		m.mediaRepo.UpdateStatus(id, models.StatusFailed)
		return err
	}

	m.addExtraTrackers(hash)

	// Notidication
	m.notifyDownloadStarted(media, torrent.Title)
	m.logger.Info("Torrent successfully sent to download client! Hash:", hash)

	m.logger.Info("Torrent successfully sent to download client! Hash:", hash)

	if err := m.mediaRepo.UpdateDownloadInfo(id, models.StatusDownloading, &hash, &torrent.Title); err != nil {
		m.logger.Error("Failed to update media status after adding torrent:", err)
		return err
	}
	return nil
}

func (m *Manager) StartEpisodeDownload(mediaID int, seasonNumber int, episodeNumber int, torrent indexers.IndexerResult) error {
	media, err := m.mediaRepo.GetByID(mediaID)
	if err != nil {
		return err
	}
	if media == nil {
		return fmt.Errorf("media not found")
	}

	if media.Type != models.MediaTypeTVShow && media.Type != models.MediaTypeAnime {
		return fmt.Errorf("media is not a TV show or anime")
	}

	var downloadPath string
	switch media.Type {
	case models.MediaTypeTVShow:
		downloadPath = m.config.TVShows.DownloadFolder
	case models.MediaTypeAnime:
		downloadPath = m.config.Anime.DownloadFolder
	default:
		downloadPath = m.config.TorrentClient.DownloadPath // Fallback
	}

	m.logger.Info(fmt.Sprintf("Starting manual download for %s S%02dE%02d: %s",
		media.Title, seasonNumber, episodeNumber, torrent.Title))

	// Start the torrent download
	var hash string

	if m.config.App.MagnetToTorrentEnabled && strings.HasPrefix(torrent.DownloadURL, "magnet:") {
		timeout := time.Duration(m.config.App.MagnetToTorrentTimeout) * time.Second
		if timeout <= 0 {
			timeout = 60 * time.Second // Default to 60 seconds
		}
		m.logger.Info("Attempting to convert magnet to .torrent with timeout:", timeout)
		torrentFileBytes, convErr := utils.ConvertMagnetToTorrent(torrent.DownloadURL, timeout, m.config.App.DataPath)
		if convErr == nil {
			m.logger.Info("Magnet conversion successful, adding as .torrent file.")
			hash, err = m.torrentClient.AddTorrentFile(torrentFileBytes, downloadPath)
		} else {
			m.logger.Warn("Magnet conversion failed:", convErr, "- falling back to magnet link.")
			hash, err = m.torrentClient.AddTorrent(torrent.DownloadURL, downloadPath)
		}
	} else {
		hash, err = m.torrentClient.AddTorrent(torrent.DownloadURL, downloadPath)
	}

	if err != nil {
		m.logger.Error("Failed to add episode torrent to client:", err)
		return err
	}

	m.addExtraTrackers(hash)

	m.logger.Info("Episode torrent successfully sent to download client! Hash:", hash)

	// Update the specific episode status in database
	if err := m.mediaRepo.UpdateEpisodeDownloadInfo(mediaID, seasonNumber, episodeNumber, models.StatusDownloading, &hash, &torrent.Title); err != nil {
		m.logger.Error("Failed to update episode status after adding torrent:", err)
		return err
	}

	return nil
}

// PerformEpisodeSearch performs a manual search for a specific episode
func (m *Manager) PerformEpisodeSearch(mediaID int, seasonNumber int, episodeNumber int) ([]indexers.IndexerResult, error) {
	media, err := m.mediaRepo.GetByID(mediaID)
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
	results, err := m.performSearch(media, seasonNumber, episodeNumber)
	if err != nil {
		return nil, err
	}

	searchTerms := []string{media.Title}
	if media.Type == models.MediaTypeAnime {
		animeSearchTerms, err := m.mediaRepo.GetAnimeSearchTerms(media.ID)
		if err == nil {
			for _, term := range animeSearchTerms {
				searchTerms = append(searchTerms, term.Term)
			}
		}
	}

	// Use the TorrentSelector to filter and score the results
	filteredResults := m.torrentSelector.FilterAndScoreTorrents(media, results, seasonNumber, episodeNumber, searchTerms)

	m.logger.Info(fmt.Sprintf("Found %d results for %s S%02dE%02d",
		len(filteredResults), media.Title, seasonNumber, episodeNumber))

	return filteredResults, nil
}

func (m *Manager) addExtraTrackers(hash string) {
	if len(m.config.ExtraTrackersList) > 0 {
		go func() {
			time.Sleep(10 * time.Second)
			m.logger.Info("Adding extra trackers to torrent:", hash)
			err := m.torrentClient.AddTrackers(hash, m.config.ExtraTrackersList)
			if err != nil {
				m.logger.Error("Failed to add extra trackers:", err)
			} else {
				m.logger.Info("Successfully added extra trackers.")
			}
		}()
	}
}

func (m *Manager) retryFailedDownloads() {
	failedMedia, err := m.mediaRepo.GetByStatus(models.StatusFailed)
	if err != nil {
		m.logger.Error("Failed to get failed media for retry:", err)
		return
	}

	if len(failedMedia) > 0 {
		m.logger.Info(fmt.Sprintf("Retrying %d failed media items.", len(failedMedia)))
		for i := range failedMedia {
			if failedMedia[i].AutoDownload {
				mediaCopy := failedMedia[i]
				if err := m.mediaRepo.UpdateStatus(mediaCopy.ID, models.StatusPending); err != nil {
					m.logger.Error("Failed to update status for retry:", err)
					continue
				}
				m.searchQueue <- mediaCopy
			}
		}
	}
}

func (m *Manager) notifyDownloadStarted(media *models.Media, torrentName string) {
	for _, n := range m.notifiers {
		// Run in a goroutine to avoid blocking the main application flow.
		go n.NotifyDownloadStart(media, torrentName)
	}
}

func (m *Manager) notifyDownloadCompleted(media *models.Media, torrentName string) {
	for _, n := range m.notifiers {
		// Run in a goroutine to avoid blocking the main application flow.
		go n.NotifyDownloadComplete(media, torrentName)
	}
}

func (m *Manager) GetMediaFilePath(mediaID int, seasonNumber int, episodeNumber int) (string, error) {
	media, err := m.mediaRepo.GetByID(mediaID)
	if err != nil {
		return "", err
	}
	if media == nil {
		return "", fmt.Errorf("media with ID %d not found", mediaID)
	}

	var baseDestPath string
	switch media.Type {
	case models.MediaTypeMovie:
		baseDestPath = m.config.Movies.DestinationFolder
	case models.MediaTypeTVShow:
		baseDestPath = m.config.TVShows.DestinationFolder
	case models.MediaTypeAnime:
		baseDestPath = m.config.Anime.DestinationFolder
	default:
		return "", fmt.Errorf("unknown media type: %s", media.Type)
	}

	safeTitle := utils.SanitizeFilename(media.Title)
	mediaFolderName := fmt.Sprintf("%s (%d)", safeTitle, media.Year)
	fullPath := filepath.Join(baseDestPath, mediaFolderName)

	if media.Type == models.MediaTypeTVShow || media.Type == models.MediaTypeAnime {
		if seasonNumber <= 0 {
			return "", fmt.Errorf("season number must be provided for TV shows")
		}
		seasonFolderName := fmt.Sprintf("S%02d", seasonNumber)
		fullPath = filepath.Join(fullPath, seasonFolderName)
	}

	// Scan the directory for a video file
	files, err := os.ReadDir(fullPath)
	if err != nil {
		return "", fmt.Errorf("could not read destination directory '%s': %w", fullPath, err)
	}

	videoExtensions := map[string]bool{".mkv": true, ".mp4": true, ".avi": true, ".mov": true}

	for _, file := range files {
		if !file.IsDir() {
			ext := strings.ToLower(filepath.Ext(file.Name()))
			if videoExtensions[ext] {
				// If it's a TV show/anime, match the episode number
				if media.Type == models.MediaTypeTVShow || media.Type == models.MediaTypeAnime {
					if episodeNumber <= 0 {
						return "", fmt.Errorf("episode number must be provided for TV shows")
					}
					episodePattern := fmt.Sprintf("S%02dE%02d", seasonNumber, episodeNumber)
					if strings.Contains(strings.ToUpper(file.Name()), episodePattern) {
						return filepath.Join(fullPath, file.Name()), nil
					}
				} else { // It's a movie, return the first video file found
					return filepath.Join(fullPath, file.Name()), nil
				}
			}
		}
	}

	return "", fmt.Errorf("no video file found in %s", fullPath)
}

func (m *Manager) GetSubtitleFilePath(mediaID int, seasonNumber int, episodeNumber int, lang string) (string, error) {
	videoPath, err := m.GetMediaFilePath(mediaID, seasonNumber, episodeNumber)
	if err != nil {
		return "", err
	}

	baseName := strings.TrimSuffix(videoPath, filepath.Ext(videoPath))

	// Try to find a language-specific subtitle file first
	langSubPath := fmt.Sprintf("%s.%s.srt", baseName, lang)
	m.logger.Debug("Checking for subtitle file:", langSubPath)
	if _, err := os.Stat(langSubPath); err == nil {
		m.logger.Debug("Found language-specific subtitle file:", langSubPath)
		return langSubPath, nil
	}

	// If not found, try to find a default subtitle file
	defaultSubPath := fmt.Sprintf("%s.srt", baseName)
	m.logger.Debug("Checking for subtitle file:", defaultSubPath)
	if _, err := os.Stat(defaultSubPath); err == nil {
		m.logger.Debug("Found default subtitle file:", defaultSubPath)
		return defaultSubPath, nil
	}

	return "", fmt.Errorf("no subtitle file found for language '%s'", lang)
}

func (m *Manager) GetAllSubtitleFiles(mediaID int, seasonNumber int, episodeNumber int) ([]SubtitleTrack, error) {
	videoPath, err := m.GetMediaFilePath(mediaID, seasonNumber, episodeNumber)
	if err != nil {
		return nil, err
	}

	baseName := strings.TrimSuffix(videoPath, filepath.Ext(videoPath))
	videoDir := filepath.Dir(videoPath)

	// Read all files in the directory
	files, err := os.ReadDir(videoDir)
	if err != nil {
		return nil, fmt.Errorf("could not read video directory: %w", err)
	}

	var subtitles []SubtitleTrack
	foundEnglish := false

	// First pass: look for language-specific subtitle files
	for _, file := range files {
		if file.IsDir() {
			continue
		}

		fileName := file.Name()
		fileExt := filepath.Ext(fileName)

		// Only process .srt files
		if strings.ToLower(fileExt) != ".srt" {
			continue
		}

		// Check if this subtitle file belongs to our video
		fileBaseName := strings.TrimSuffix(fileName, fileExt)
		videoBaseName := filepath.Base(baseName)

		// Skip if this subtitle doesn't match our video file
		if !strings.HasPrefix(fileBaseName, videoBaseName) {
			continue
		}

		// Extract language code from filename
		// Expected format: videoname.lang.srt or videoname.srt
		parts := strings.Split(fileBaseName, ".")

		var langCode string
		var label string

		if len(parts) >= 2 && parts[len(parts)-1] != videoBaseName {
			// Has language code: videoname.en.srt
			langCode = parts[len(parts)-1]
			label = getLanguageLabel(langCode)
		} else if fileName == videoBaseName+".srt" {
			// Default subtitle file without language code
			langCode = "default"
			label = "Default"
		} else {
			// Skip files that don't match our expected pattern
			continue
		}

		if langCode == "en" || langCode == "eng" {
			foundEnglish = true
		}

		subtitles = append(subtitles, SubtitleTrack{
			Language: langCode,
			Label:    label,
			FilePath: filepath.Join(videoDir, fileName),
		})
	}

	// Second pass: if no English subtitle found, include the default one as English
	if !foundEnglish {
		defaultSubPath := baseName + ".srt"
		if _, err := os.Stat(defaultSubPath); err == nil {
			// Check if we already added this as "default" and update it
			for i, sub := range subtitles {
				if sub.Language == "default" {
					subtitles[i].Language = "en"
					subtitles[i].Label = "English (Default)"
					foundEnglish = true
					break
				}
			}
		}
	}

	// Sort subtitles: English first, then alphabetically by label
	sort.Slice(subtitles, func(i, j int) bool {
		if subtitles[i].Language == "en" {
			return true
		}
		if subtitles[j].Language == "en" {
			return false
		}
		return subtitles[i].Label < subtitles[j].Label
	})

	return subtitles, nil
}

// Helper function to convert language codes to readable labels
func getLanguageLabel(langCode string) string {
	languageMap := map[string]string{
		"en": "English",
		"es": "Spanish",
		"fr": "French",
		"de": "German",
		"it": "Italian",
		"pt": "Portuguese",
		"ru": "Russian",
		"ja": "Japanese",
		"ko": "Korean",
		"zh": "Chinese",
		"ar": "Arabic",
		"hi": "Hindi",
		"th": "Thai",
		"tr": "Turkish",
		"pl": "Polish",
		"nl": "Dutch",
		"sv": "Swedish",
		"da": "Danish",
		"no": "Norwegian",
		"fi": "Finnish",
		"cs": "Czech",
		"hu": "Hungarian",
		"ro": "Romanian",
		"bg": "Bulgarian",
		"hr": "Croatian",
		"sk": "Slovak",
		"sl": "Slovenian",
		"et": "Estonian",
		"lv": "Latvian",
		"lt": "Lithuanian",
		"uk": "Ukrainian",
		"be": "Belarusian",
		"mk": "Macedonian",
		"sr": "Serbian",
		"bs": "Bosnian",
		"me": "Montenegrin",
		"sq": "Albanian",
		"el": "Greek",
		"he": "Hebrew",
		"fa": "Persian",
		"ur": "Urdu",
		"bn": "Bengali",
		"ta": "Tamil",
		"te": "Telugu",
		"ml": "Malayalam",
		"kn": "Kannada",
		"gu": "Gujarati",
		"pa": "Punjabi",
		"mr": "Marathi",
		"ne": "Nepali",
		"si": "Sinhala",
		"my": "Burmese",
		"km": "Khmer",
		"lo": "Lao",
		"vi": "Vietnamese",
		"id": "Indonesian",
		"ms": "Malay",
		"tl": "Filipino",
		"sw": "Swahili",
		"am": "Amharic",
		"yo": "Yoruba",
		"ig": "Igbo",
		"ha": "Hausa",
		"zu": "Zulu",
		"af": "Afrikaans",
		"ca": "Catalan",
		"eu": "Basque",
		"gl": "Galician",
		"cy": "Welsh",
		"ga": "Irish",
		"gd": "Scottish Gaelic",
		"is": "Icelandic",
		"fo": "Faroese",
		"mt": "Maltese",
		"lb": "Luxembourgish",
	}

	if label, exists := languageMap[langCode]; exists {
		return label
	}

	// If not found in map, return the code in uppercase
	return strings.ToUpper(langCode)
}

// UpdateMediaSettings updates the settings for a given media item.
func (m *Manager) UpdateMediaSettings(id int, minQuality, maxQuality string, autoDownload bool) error {
	m.logger.Info(fmt.Sprintf("Updating settings for media ID %d: minQ=%s, maxQ=%s, auto=%t", id, minQuality, maxQuality, autoDownload))
	return m.mediaRepo.UpdateSettings(id, minQuality, maxQuality, autoDownload)
}

// Add this function to read the config file content
func (m *Manager) GetConfig() (string, error) {
	// Assumes the config path is stored in the config object,
	// but the Load function doesn't store it. We'll need to know the path.
	// For now, let's assume a default path or find a way to pass it.
	// Let's pass the config path to the NewManager function.
	// For now, let's just hardcode it for simplicity, but this should be improved.
	configPath := "config/config.yml"
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		configPath = "config.yml"
	}

	data, err := ioutil.ReadFile(configPath)
	if err != nil {
		return "", err
	}
	return string(data), nil
}

func (m *Manager) TestIndexerConnection(indexerKey string) (bool, error) {
	var clientToTest indexers.Client
	var sourceURL string

	// Find the client that matches the key.
	for _, clients := range m.indexerClients {
		for _, clientWithMode := range clients {
			// A simple check to see if the key is part of the URL.
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
		return false, fmt.Errorf("indexer '%s' not found in any configuration", indexerKey)
	}

	// Perform the actual health check on the found client.
	ok, err := clientToTest.HealthCheck()
	m.logger.Info(fmt.Sprintf("Testing indexer with url %s: %t", sourceURL, ok))
	if err != nil {
		return false, fmt.Errorf("health check for %s failed: %w", sourceURL, err)
	}
	if !ok {
		return false, fmt.Errorf("indexer at %s is offline or misconfigured", sourceURL)
	}

	return true, nil
}

func (m *Manager) TestTorrentConnection() (bool, error) {
	if m.torrentClient == nil {
		return false, fmt.Errorf("torrent client not initialized")
	}
	return m.torrentClient.HealthCheck()
}

func (m *Manager) GetAnimeSearchTerms(mediaID int) ([]models.AnimeSearchTerm, error) {
	return m.mediaRepo.GetAnimeSearchTerms(mediaID)
}

func (m *Manager) AddAnimeSearchTerm(mediaID int, term string) (*models.AnimeSearchTerm, error) {
	return m.mediaRepo.AddAnimeSearchTerm(mediaID, term)
}

func (m *Manager) DeleteAnimeSearchTerm(id int) error {
	return m.mediaRepo.DeleteAnimeSearchTerm(id)
}
