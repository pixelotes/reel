package core

import (
	"database/sql"
	"fmt"
	"net/http"
	"time"

	"reel/internal/clients/indexers"
	"reel/internal/clients/metadata"
	"reel/internal/clients/notifications"
	"reel/internal/clients/subtitles"
	"reel/internal/clients/torrent"
	"reel/internal/config"
	"reel/internal/core/services"
	"reel/internal/database/models"
	"reel/internal/utils"

	"gopkg.in/yaml.v3"
)

type SubtitleTrack = services.SubtitleTrack

type Manager struct {
	config    *config.Config
	db        *sql.DB
	mediaRepo *models.MediaRepository
	logger    *utils.Logger

	// Services
	downloaderService *services.DownloaderService
	searcherService   *services.SearcherService
	rssService        *services.RSSService
	schedulerService  *services.SchedulerService
	libraryService    *services.LibraryService
	postProcessor     *services.PostProcessor // Kept for reload logic

	// Clients
	subtitleClient *subtitles.Client

	searchQueue chan models.Media
	httpClient  *http.Client
}

type SystemStatus struct {
	TorrentClient   services.ClientStatus            `json:"torrent_client"`
	IndexerClients  map[string]services.ClientStatus `json:"indexer_clients"`
	MetadataClients []string                         `json:"metadata_clients"`
}

func NewManager(cfg *config.Config, db *sql.DB, logger *utils.Logger) *Manager {
	m := &Manager{
		config:      cfg,
		db:          db,
		logger:      logger,
		searchQueue: make(chan models.Media, 100),
		httpClient:  &http.Client{},
	}

	m.reloadConfig(cfg)

	go m.startSearchQueueWorker()

	return m
}

func (m *Manager) reloadConfig(cfg *config.Config) {
	m.config = cfg
	m.logger.Info("Reloading the config...")

	// --- Initialize Timeouts ---
	searchTimeout := time.Duration(cfg.App.SearchTimeout) * time.Second
	if cfg.App.SearchTimeout <= 0 {
		searchTimeout = 30 * time.Second
	}
	m.httpClient.Timeout = searchTimeout

	metadataTimeout := time.Duration(cfg.Metadata.Timeout) * time.Second
	if cfg.Metadata.Timeout <= 0 {
		metadataTimeout = 15 * time.Second
	}

	// --- Initialize Notifiers ---
	var notifiers []notifications.Notifier
	for _, notifierName := range cfg.Automation.Notifications {
		switch notifierName {
		case "pushbullet":
			if cfg.Notifications.Pushbullet.APIKey != "" {
				client := notifications.NewPushbulletClient(cfg.Notifications.Pushbullet.APIKey, m.logger)
				notifiers = append(notifiers, client)
				m.logger.Info("Pushbullet notifier enabled.")
			}
		case "telegram":
			if cfg.Notifications.Telegram.BotToken != "" && cfg.Notifications.Telegram.ChatID != "" {
				client := notifications.NewTelegramClient(cfg.Notifications.Telegram.BotToken, cfg.Notifications.Telegram.ChatID, m.logger)
				notifiers = append(notifiers, client)
				m.logger.Info("Telegram notifier enabled.")
			}
		}
	}

	m.mediaRepo = models.NewMediaRepository(m.db, m.logger)

	// Initialize Subtitle Client
	m.subtitleClient = subtitles.NewClient(cfg, m.logger)

	m.postProcessor = services.NewPostProcessor(cfg, m.logger, m.mediaRepo, notifiers, m.subtitleClient)
	// --- Initialize Metadata Clients ---
	tmdbClient := metadata.NewTMDBClient(cfg.Metadata.TMDB.APIKey, cfg.Metadata.Language, metadataTimeout)
	initMetadataProvider := func(provider string) metadata.Client {
		switch provider {
		case "tmdb":
			return tmdbClient
		case "imdb":
			return metadata.NewIMDBClient(cfg.Metadata.IMDB.APIKey, metadataTimeout, m.logger)
		case "tvmaze":
			return metadata.NewTVmazeClient(metadataTimeout)
		case "anilist":
			return metadata.NewAniListClient(metadataTimeout)
		case "trakt":
			return metadata.NewTraktClient(cfg.Metadata.Trakt.ClientID, tmdbClient, metadataTimeout, m.logger)
		}
		return nil
	}

	metadataClients := make(map[models.MediaType][]metadata.Client)
	// Movies
	for _, providerName := range cfg.Movies.Providers {
		if client := initMetadataProvider(providerName); client != nil {
			metadataClients[models.MediaTypeMovie] = append(metadataClients[models.MediaTypeMovie], client)
		}
	}
	// TV Shows
	for _, providerName := range cfg.TVShows.Providers {
		if client := initMetadataProvider(providerName); client != nil {
			metadataClients[models.MediaTypeTVShow] = append(metadataClients[models.MediaTypeTVShow], client)
		}
	}
	// Anime
	for _, providerName := range cfg.Anime.Providers {
		if client := initMetadataProvider(providerName); client != nil {
			metadataClients[models.MediaTypeAnime] = append(metadataClients[models.MediaTypeAnime], client)
		}
	}

	// --- Initialize Indexer Clients ---
	initIndexerClient := func(source config.SourceConfig) indexers.Client {
		switch source.Type {
		case "scarf":
			return indexers.NewScarfClient(source.URL, source.APIKey, searchTimeout)
		case "jackett":
			return indexers.NewJackettClient(source.URL, source.APIKey, searchTimeout)
		case "prowlarr":
			return indexers.NewProwlarrClient(source.URL, source.APIKey, searchTimeout)
		}
		return nil
	}

	indexerClients := make(map[models.MediaType][]services.IndexerClientWithMode)
	// Movies
	for _, source := range cfg.Movies.Sources {
		if source.Type != "rss" {
			if client := initIndexerClient(source); client != nil {
				indexerClients[models.MediaTypeMovie] = append(indexerClients[models.MediaTypeMovie], services.IndexerClientWithMode{
					Client: client,
					Source: source,
				})
			}
		}
	}
	// TV Shows
	for _, source := range cfg.TVShows.Sources {
		if source.Type != "rss" {
			if client := initIndexerClient(source); client != nil {
				indexerClients[models.MediaTypeTVShow] = append(indexerClients[models.MediaTypeTVShow], services.IndexerClientWithMode{
					Client: client,
					Source: source,
				})
			}
		}
	}
	// Anime
	for _, source := range cfg.Anime.Sources {
		if source.Type != "rss" {
			if client := initIndexerClient(source); client != nil {
				indexerClients[models.MediaTypeAnime] = append(indexerClients[models.MediaTypeAnime], services.IndexerClientWithMode{
					Client: client,
					Source: source,
				})
			}
		}
	}

	// --- Initialize Torrent Client ---
	var torrentClient torrent.TorrentClient
	switch cfg.TorrentClient.Type {
	case "transmission":
		torrentClient = torrent.NewTransmissionClient(cfg.TorrentClient.Host, cfg.TorrentClient.Username, cfg.TorrentClient.Password)
	case "qbittorrent":
		torrentClient = torrent.NewQBittorrentClient(cfg.TorrentClient.Host, cfg.TorrentClient.Username, cfg.TorrentClient.Password, m.logger)
	case "aria2":
		torrentClient = torrent.NewAria2Client(cfg.TorrentClient.Host, cfg.TorrentClient.Secret)
	case "deluge":
		client, err := torrent.NewDelugeClient(cfg.TorrentClient.Host, cfg.TorrentClient.Password)
		if err != nil {
			m.logger.Fatal("Failed to create Deluge client:", err)
		}
		torrentClient = client
	case "mock":
		torrentClient = torrent.NewMockClient(m.logger)
	default:
		m.logger.Fatal("Unsupported torrent client type:", cfg.TorrentClient.Type)
	}

	// --- Instantiate Services ---
	matcherService := services.NewMatcherService(cfg, m.logger)
	torrentSelector := services.NewTorrentSelector(cfg, matcherService, m.logger)

	m.libraryService = services.NewLibraryService(cfg, m.mediaRepo, metadataClients, m.logger)
	m.downloaderService = services.NewDownloaderService(cfg, m.logger, torrentClient, m.mediaRepo, m.postProcessor, notifiers)
	m.searcherService = services.NewSearcherService(indexerClients, torrentSelector, m.mediaRepo, m.logger)
	m.rssService = services.NewRSSService(cfg, m.mediaRepo, m.downloaderService, torrentSelector, matcherService, m.logger, m.httpClient)

	// Scheduler needs the queue channel
	// Note: We need to stop previous scheduler if it exists
	if m.schedulerService != nil {
		m.schedulerService.Stop()
	}
	m.schedulerService = services.NewSchedulerService(cfg, m.searchQueue, m.libraryService, m.rssService, m.downloaderService, m.logger)

	m.logger.Info("Configuration reloaded successfully.")

	// Start Scheduler (Facade method)
	m.StartScheduler()
}

func (m *Manager) StartScheduler() {
	m.schedulerService.Start()
}

func (m *Manager) Stop() {
	m.schedulerService.Stop()
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

// Orchestration Logic: Movie
func (m *Manager) searchAndDownloadMovie(media *models.Media) {
	m.logger.Info("Starting automatic search for movie:", media.Title)
	// Update status to searching
	if err := m.mediaRepo.UpdateStatus(media.ID, models.StatusSearching); err != nil {
		m.logger.Error("Failed to update status to searching:", err)
		return
	}

	// Search
	bestTorrent, err := m.searcherService.FindBestTorrent(media, 0, 0)
	if err != nil {
		m.logger.Error("Search failed for movie:", media.Title, err)
		m.mediaRepo.UpdateStatus(media.ID, models.StatusFailed)
		return
	}

	if bestTorrent != nil {
		m.logger.Info("Found suitable torrent for movie:", media.Title, bestTorrent.Title)
		// Download
		if err := m.downloaderService.StartDownload(media.ID, *bestTorrent); err != nil {
			m.logger.Error("Failed to start download for movie:", media.Title, err)
			m.mediaRepo.UpdateStatus(media.ID, models.StatusFailed)
		}
	} else {
		m.logger.Info("No suitable torrent found for movie:", media.Title)
		m.mediaRepo.UpdateStatus(media.ID, models.StatusFailed)
	}
}

// Orchestration Logic: TV Show / Anime
func (m *Manager) searchAndDownloadNextEpisode(media *models.Media) {
	show, err := m.mediaRepo.GetTVShowByMediaID(media.ID)
	if err != nil || show == nil {
		m.logger.Error("Failed to get show details for:", media.Title)
		return
	}

	// Iterate through seasons to find pending episodes
	for _, season := range show.Seasons {
		for _, episode := range season.Episodes {
			if episode.Status == models.StatusPending {
				m.logger.Info(fmt.Sprintf("Searching for %s S%02dE%02d", media.Title, season.SeasonNumber, episode.EpisodeNumber))

				// Update episode status to searching
				// Note: MediaRepo doesn't have explicit UpdateEpisodeStatus, usually stored in DB.
				// We can just proceed to search.

				bestTorrent, err := m.searcherService.FindBestTorrent(media, season.SeasonNumber, episode.EpisodeNumber)
				if err != nil {
					m.logger.Error("Search failed for episode:", media.Title, err)
					continue
				}

				if bestTorrent != nil {
					m.logger.Info("Found suitable torrent for episode:", media.Title, bestTorrent.Title)
					if err := m.downloaderService.StartEpisodeDownload(media.ID, season.SeasonNumber, episode.EpisodeNumber, *bestTorrent); err != nil {
						m.logger.Error("Failed to start download for episode:", media.Title, err)
						// Update status handled in service? Downloader service handles failure status (logic in StartEpisodeDownload).
					}
				} else {
					m.logger.Info("No suitable torrent found for episode:", media.Title)
					// Mark as failed or leave pending?
					// Ideally mark failed so it can be retried later.
					// We'll leave it pending or mark failed?
					// Logic in original Manager was to mark Failed.
					// We should update status to failed here.
					// But we need a method to update episode status.
					// DownloaderService has UpdateEpisodeDownloadInfo which takes Hash.
					// Can we pass nil hash?
					m.mediaRepo.UpdateEpisodeDownloadInfo(media.ID, season.SeasonNumber, episode.EpisodeNumber, models.StatusFailed, nil, nil)
				}

				// Process one episode at a time per loop?
				// Original manager looped through ALL pending episodes.
				// We should probably sleep between episodes if multiple found?
				if bestTorrent != nil {
					time.Sleep(10 * time.Second)
				}
			}
		}
	}
	// Update overall show progress
	m.libraryService.UpdateShowProgress(media.ID)
}

// Data Access / Facade Methods

func (m *Manager) GetAllMedia() ([]models.Media, error) {
	return m.mediaRepo.GetAll()
}

func (m *Manager) GetMediaByID(id int) (*models.Media, error) {
	return m.mediaRepo.GetByID(id)
}

func (m *Manager) AddMedia(mediaType models.MediaType, id string, title string, year int, language, minQuality, maxQuality string, autoDownload bool, startSeason, startEpisode int) (*models.Media, error) {
	media, err := m.libraryService.AddMedia(mediaType, id, title, year, language, minQuality, maxQuality, autoDownload, startSeason, startEpisode)
	if err != nil {
		return nil, err
	}
	if autoDownload {
		m.searchQueue <- *media
	}
	return media, nil
}

func (m *Manager) DeleteMedia(id int) error {
	return m.libraryService.DeleteMedia(id)
}

func (m *Manager) RetryMedia(id int) error {
	err := m.libraryService.RetryMedia(id)
	if err != nil {
		return err
	}
	// Fetch and enqueue
	media, err := m.mediaRepo.GetByID(id)
	if err == nil && media != nil && media.Status == models.StatusPending {
		m.searchQueue <- *media
	}
	return nil
}

func (m *Manager) ClearFailedMedia() error {
	return m.libraryService.ClearFailedMedia()
}

// Search & Download Facade

func (m *Manager) PerformSearch(id int) ([]indexers.IndexerResult, error) {
	return m.searcherService.PerformSearch(id)
}

func (m *Manager) StartDownload(mediaID int, t indexers.IndexerResult) error {
	return m.downloaderService.StartDownload(mediaID, t)
}

func (m *Manager) PerformEpisodeSearch(mediaID int, seasonNumber int, episodeNumber int) ([]indexers.IndexerResult, error) {
	return m.searcherService.PerformEpisodeSearch(mediaID, seasonNumber, episodeNumber)
}

func (m *Manager) StartEpisodeDownload(mediaID int, seasonNumber int, episodeNumber int, t indexers.IndexerResult) error {
	return m.downloaderService.StartEpisodeDownload(mediaID, seasonNumber, episodeNumber, t)
}

// Metadata & Library Facade

func (m *Manager) GetTVShowDetails(mediaID int) (*models.TVShow, error) {
	return m.libraryService.GetTVShowDetails(mediaID)
}

func (m *Manager) SearchMetadata(query string, mediaType string) ([]interface{}, error) {
	return m.libraryService.SearchMetadata(query, mediaType)
}

func (m *Manager) UpdateMediaSettings(id int, minQuality, maxQuality string, autoDownload bool) error {
	return m.libraryService.UpdateMediaSettings(id, minQuality, maxQuality, autoDownload)
}

func (m *Manager) GetMediaFilePath(mediaID int, seasonNumber int, episodeNumber int) (string, error) {
	return m.libraryService.GetMediaFilePath(mediaID, seasonNumber, episodeNumber)
}

func (m *Manager) GetAllSubtitleFiles(mediaID int, seasonNumber int, episodeNumber int) ([]services.SubtitleTrack, error) {
	return m.libraryService.GetAllSubtitleFiles(mediaID, seasonNumber, episodeNumber)
}

func (m *Manager) GetAnimeSearchTerms(mediaID int) ([]models.AnimeSearchTerm, error) {
	return m.libraryService.GetAnimeSearchTerms(mediaID)
}

func (m *Manager) AddAnimeSearchTerm(mediaID int, term string) (*models.AnimeSearchTerm, error) {
	return m.libraryService.AddAnimeSearchTerm(mediaID, term)
}

func (m *Manager) DeleteAnimeSearchTerm(id int) error {
	return m.libraryService.DeleteAnimeSearchTerm(id)
}

// SystemStatus & Config Facade

func (m *Manager) GetSystemStatus() (*SystemStatus, error) {
	// Collect status from services
	torrentStatus, _ := m.downloaderService.GetTorrentClientStatus()

	// Create simplified ClientStatus for torrent
	// We need type name. Config knows it.
	tType := m.config.TorrentClient.Type
	tStatus := services.ClientStatus{
		Type:   tType,
		Name:   tType,
		Status: torrentStatus,
	}

	indexerStatuses := m.searcherService.GetIndexerStatuses()
	metadataClients := m.libraryService.GetMetadataClients()

	return &SystemStatus{
		TorrentClient:   tStatus,
		IndexerClients:  indexerStatuses,
		MetadataClients: metadataClients,
	}, nil
}

func (m *Manager) TestIndexerConnection(indexerKey string) (bool, error) {
	return m.searcherService.TestIndexer(indexerKey)
}

func (m *Manager) TestTorrentConnection() (bool, error) {
	return m.downloaderService.TestTorrentConnection()
}

func (m *Manager) GetCalendarEvents() ([]services.CalendarEvent, error) {
	return []services.CalendarEvent{}, nil
}

func (m *Manager) GetConfig() (*config.Config, error) {
	return m.config, nil
}

func (m *Manager) SaveAndReloadConfig(configData string) error {
	var newConfig config.Config
	if err := yaml.Unmarshal([]byte(configData), &newConfig); err != nil {
		return err
	}
	m.reloadConfig(&newConfig)
	return nil
}

// Helpers

func (m *Manager) ResetSystem() {
	// Stop existing services
	m.Stop()
	// Re-init?
	// This usually just resets clients. ReloadConfig does that.
	m.reloadConfig(m.config)
}
