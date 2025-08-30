package core

import (
	"database/sql"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/robfig/cron/v3"

	"reel/internal/clients/indexers"
	"reel/internal/clients/metadata"
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
	"2160p": {"2160p", "4k", "uhd"},
	"1440p": {"1440p", "2k"},
	"1080p": {"1080p", "fhd"},
	"720p":  {"720p", "hd"},
	"480p":  {"480p", "sd"},
	"360p":  {"360p"},
}

var RESOLUTION_RANK = map[string]int{
	"360p":  0,
	"480p":  1,
	"720p":  2,
	"1080p": 3,
	"1440p": 4,
	"2160p": 5,
}

// Ordered from highest to lowest for matching
var SUPPORTED_RESOLUTIONS = []string{"2160p", "1440p", "1080p", "720p", "480p", "360p"}

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

type Manager struct {
	config          *config.Config
	mediaRepo       *models.MediaRepository
	indexerClients  map[models.MediaType][]indexers.Client
	metadataClients map[models.MediaType][]metadata.Client
	torrentClient   torrent.TorrentClient
	logger          *utils.Logger
	scheduler       *cron.Cron
	searchQueue     chan models.Media
}

func NewManager(cfg *config.Config, db *sql.DB, logger *utils.Logger) *Manager {
	m := &Manager{
		config:          cfg,
		mediaRepo:       models.NewMediaRepository(db),
		logger:          logger,
		scheduler:       cron.New(),
		searchQueue:     make(chan models.Media, 100),
		indexerClients:  make(map[models.MediaType][]indexers.Client),
		metadataClients: make(map[models.MediaType][]metadata.Client),
	}

	// --- Initialize Clients based on new Config Structure ---

	// Helper function to initialize metadata providers
	initMetadataProvider := func(provider string) metadata.Client {
		switch provider {
		case "tmdb":
			return metadata.NewTMDBClient(cfg.Metadata.TMDB.APIKey, cfg.Metadata.Language)
		case "imdb":
			return metadata.NewIMDBClient(cfg.Metadata.IMDB.APIKey)
		case "tvmaze":
			return metadata.NewTVmazeClient()
		}
		return nil
	}

	// Helper function to initialize indexer sources
	initIndexerClient := func(source struct {
		Type   string `yaml:"type"`
		URL    string `yaml:"url"`
		APIKey string `yaml:"api_key"`
	}) indexers.Client {
		switch source.Type {
		case "scarf":
			timeout, _ := time.ParseDuration("30s")
			return indexers.NewScarfClient(source.URL, source.APIKey, timeout)
		case "jackett":
			return indexers.NewJackettClient(source.URL, source.APIKey)
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
		if client := initIndexerClient(source); client != nil {
			m.indexerClients[models.MediaTypeMovie] = append(m.indexerClients[models.MediaTypeMovie], client)
		}
	}

	// Initialize TV Show Clients
	for _, providerName := range cfg.TVShows.Providers {
		if client := initMetadataProvider(providerName); client != nil {
			m.metadataClients[models.MediaTypeTVShow] = append(m.metadataClients[models.MediaTypeTVShow], client)
		}
	}
	for _, source := range cfg.TVShows.Sources {
		if client := initIndexerClient(source); client != nil {
			m.indexerClients[models.MediaTypeTVShow] = append(m.indexerClients[models.MediaTypeTVShow], client)
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
		if media.Type == models.MediaTypeMovie {
			m.searchAndDownloadMovie(&media)
		} else if media.Type == models.MediaTypeTVShow {
			m.searchAndDownloadNextEpisode(&media)
		}
		time.Sleep(30 * time.Second)
	}
}

func (m *Manager) AddMedia(mediaType models.MediaType, id string, title string, year int, language, minQuality, maxQuality string, autoDownload bool, startSeason, startEpisode int) (*models.Media, error) {
	m.logger.Info("=== STARTING AddMedia FUNCTION ===")
	m.logger.Info("Parameters - Type:", mediaType, "ID:", id, "Title:", title, "Year:", year)

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

		if mediaType == models.MediaTypeMovie {
			m.logger.Info("Processing movie metadata...")
			movieData, err := client.SearchMovie(title, year)
			if err != nil {
				m.logger.Error("Movie metadata search failed:", err)
			} else if movieData != nil {
				m.logger.Info("Movie metadata found - ID:", movieData.ID, "Title:", movieData.Title)
				// Parse TMDB ID
				if tmdbID, parseErr := strconv.Atoi(movieData.ID); parseErr == nil {
					metadataID = &tmdbID
					m.logger.Info("Parsed TMDB ID:", *metadataID)
				} else {
					m.logger.Error("Failed to parse TMDB ID:", movieData.ID, "Error:", parseErr)
				}

				overview = &movieData.Overview
				posterURL = &movieData.PosterURL
				rating = &movieData.Rating
				if title == "" {
					title = movieData.Title
				}
				if year == 0 {
					year = movieData.Year
				}
				m.logger.Info("Movie data processed successfully")
			} else {
				m.logger.Info("No movie metadata found")
			}
		} else if mediaType == models.MediaTypeTVShow {
			m.logger.Info("Processing TV show metadata...")
			var err error
			tvShowData, err = client.SearchTVShow(title)
			if err != nil {
				m.logger.Error("TV show metadata search failed:", err)
			} else if tvShowData != nil {
				m.logger.Info("TV show metadata found - ID:", tvShowData.ID, "Title:", tvShowData.Title)
				overview = &tvShowData.Overview
				posterURL = &tvShowData.PosterURL
				rating = &tvShowData.Rating
				if title == "" {
					title = tvShowData.Title
				}
				if year == 0 {
					year = tvShowData.Year
				}
				m.logger.Info("TV show data processed successfully")
			} else {
				m.logger.Info("No TV show metadata found")
			}
		}
	}

	var tvShowID *int
	if mediaType == models.MediaTypeTVShow && tvShowData != nil {
		m.logger.Info("Creating TV show database entries...")
		show := &models.TVShow{
			Status:   tvShowData.Status,
			TVmazeID: tvShowData.ID,
		}

		m.logger.Info("Creating TV show record...")
		if err := m.mediaRepo.CreateTVShow(show); err != nil {
			m.logger.Error("CRITICAL: Failed to create TV show:", err)
			return nil, fmt.Errorf("failed to create tv show: %w", err)
		}
		m.logger.Info("TV show created with ID:", show.ID)
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
				airDate, _ := time.Parse("2006-01-02", ep.AirDate)
				if airDate.After(time.Now()) {
					status = models.StatusTBA
				} else if seasonNum < startSeason || (seasonNum == startSeason && ep.EpisodeNumber < startEpisode) {
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
		m.logger.Info("All TV show data created successfully")
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

	m.logger.Info("=== MEDIA CREATED SUCCESSFULLY ===")
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

	m.logger.Info("=== AddMedia FUNCTION COMPLETED ===")
	return media, nil
}

func (m *Manager) GetTVShowDetails(mediaID int) (*models.TVShow, error) {
	return m.mediaRepo.GetTVShowByMediaID(mediaID)
}

func (m *Manager) searchAndDownloadMovie(media *models.Media) {
	m.logger.Info("🔍 Starting automatic search for movie:", media.Title)
	m.mediaRepo.UpdateStatus(media.ID, models.StatusSearching)

	results, err := m.performSearch(media, 0, 0)
	if err != nil {
		m.logger.Error("Search failed for", media.Title, ":", err)
		m.mediaRepo.UpdateStatus(media.ID, models.StatusFailed)
		return
	}

	bestTorrent := m.selectBestTorrent(media, results)
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
				m.logger.Info("🔍 Searching for episode:", media.Title, fmt.Sprintf("S%02dE%02d", season.SeasonNumber, episode.EpisodeNumber))
				results, err := m.performSearch(media, season.SeasonNumber, episode.EpisodeNumber)
				if err != nil {
					m.logger.Error("Episode search failed:", err)
					continue
				}

				bestTorrent := m.selectBestTorrent(media, results)
				if bestTorrent != nil {
					m.StartDownload(media.ID, *bestTorrent)
					downloadsStarted++
				}
			}
		}
	}
	if downloadsStarted == 0 {
		m.logger.Info("No pending episodes to download for", media.Title)
	}
}

func (m *Manager) selectBestTorrent(media *models.Media, results []indexers.IndexerResult) *indexers.IndexerResult {
	minRank := RESOLUTION_RANK[media.MinQuality]
	maxRank := RESOLUTION_RANK[media.MaxQuality]

	var qualityFilteredTorrents []indexers.IndexerResult
	for _, r := range results {
		lowerTitle := strings.ToLower(r.Title)
		for _, res := range SUPPORTED_RESOLUTIONS {
			synonyms := RESOLUTION_SYNONYMS[res]
			for _, s := range synonyms {
				if strings.Contains(lowerTitle, strings.ToLower(s)) {
					rank := RESOLUTION_RANK[res]
					if rank >= minRank && rank <= maxRank {
						qualityFilteredTorrents = append(qualityFilteredTorrents, r)
					}
					goto nextTorrent // Found a resolution, move to the next torrent
				}
			}
		}
	nextTorrent:
	}

	if len(qualityFilteredTorrents) == 0 {
		return nil
	}

	var eligibleTorrents []indexers.IndexerResult
	for _, r := range qualityFilteredTorrents {
		if r.Seeders >= m.config.Automation.MinSeeders {
			eligibleTorrents = append(eligibleTorrents, r)
		}
	}

	if len(eligibleTorrents) == 0 {
		return nil
	}

	for i := range eligibleTorrents {
		eligibleTorrents[i].Score = getQualityScore(eligibleTorrents[i].Title)
		eligibleTorrents[i].Score += eligibleTorrents[i].Seeders
	}

	sort.Slice(eligibleTorrents, func(i, j int) bool {
		return eligibleTorrents[i].Score > eligibleTorrents[j].Score
	})

	bestTorrent := eligibleTorrents[0]
	m.logger.Info(fmt.Sprintf("🏆 Best torrent found: %s (Score: %d, Seeders: %d, Leechers: %d)", bestTorrent.Title, bestTorrent.Score, bestTorrent.Seeders, bestTorrent.Leechers))

	for i := 1; i < len(eligibleTorrents) && i < 3; i++ {
		runnerUp := eligibleTorrents[i]
		m.logger.Info(fmt.Sprintf("  - Runner-up: %s (Score: %d, Seeders: %d, Leechers: %d)", runnerUp.Title, runnerUp.Score, runnerUp.Seeders, runnerUp.Leechers))
	}

	return &bestTorrent
}

func (m *Manager) GetAllMedia() ([]models.Media, error) {
	m.logger.Info("=== Manager.GetAllMedia called ===")

	result, err := m.mediaRepo.GetAll()
	if err != nil {
		m.logger.Error("Manager.GetAllMedia: Repository error:", err)
		return nil, err
	}

	m.logger.Info("Manager.GetAllMedia: Retrieved", len(result), "items from repository")
	return result, nil
}

func (m *Manager) StartScheduler() {
	m.scheduler.AddFunc("@every 30m", m.processPendingMedia)
	m.scheduler.AddFunc("@every 6h", m.checkForNewEpisodes)
	m.scheduler.AddFunc("@every 10s", m.updateDownloadStatus)
	m.scheduler.Start()
	m.logger.Info("Scheduler started. Performing initial search for pending media.")
	go m.processPendingMedia()
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
		if item.Type == models.MediaTypeTVShow {
			provider := m.metadataClients[models.MediaTypeTVShow][0] // Assuming first provider
			m.updateShowMetadata(&item, provider)
		}
	}
}

func (m *Manager) updateShowMetadata(media *models.Media, provider metadata.Client) {
	m.logger.Info("Updating metadata for show:", media.Title)
	remoteShow, err := provider.SearchTVShow(media.Title)
	if err != nil {
		m.logger.Error("Failed to fetch remote show data for", media.Title, ":", err)
		return
	}

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
				airDate, _ := time.Parse("2006-01-02", remoteEpisode.AirDate)
				if airDate.After(time.Now()) {
					status = models.StatusTBA
				}
				newEpisode := &models.Episode{
					SeasonID:      localSeason.ID,
					EpisodeNumber: remoteEpisode.EpisodeNumber,
					Title:         remoteEpisode.Title,
					AirDate:       remoteEpisode.AirDate,
					Status:        status,
				}
				m.mediaRepo.CreateEpisode(newEpisode)
			} else if localEpisode.Status == models.StatusTBA {
				airDate, _ := time.Parse("2006-01-02", remoteEpisode.AirDate)
				if airDate.Before(time.Now()) {
					m.mediaRepo.UpdateEpisodeDownloadInfo(media.ID, seasonNum, localEpisode.EpisodeNumber, models.StatusPending, nil, nil)
				}
			}
		}
	}
	m.updateShowProgress(media.ID)
}

func (m *Manager) updateShowProgress(mediaID int) {
	show, err := m.mediaRepo.GetTVShowByMediaID(mediaID)
	if err != nil {
		return
	}
	var totalProgress float64
	var downloadableEpisodes int
	for _, season := range show.Seasons {
		for _, episode := range season.Episodes {
			if episode.Status != models.StatusSkipped && episode.Status != models.StatusTBA {
				downloadableEpisodes++
				if episode.Status == models.StatusDownloaded {
					totalProgress += 1.0
				}
			}
		}
	}
	if downloadableEpisodes > 0 {
		progress := totalProgress / float64(downloadableEpisodes)
		m.mediaRepo.UpdateProgress(mediaID, models.StatusDownloading, progress, nil)
	}
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
				if err := m.mediaRepo.UpdateStatus(media.ID, models.StatusFailed); err != nil {
					m.logger.Error("Failed to update media status to failed for", media.Title, ":", err)
				}
				continue
			}

			newStatus := models.StatusDownloading
			var completedAt *time.Time
			if status.IsCompleted {
				newStatus = models.StatusDownloaded
				now := time.Now()
				completedAt = &now
			}
			m.mediaRepo.UpdateProgress(media.ID, newStatus, status.Progress, completedAt)
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
		results = append(results, res)
	} else if mediaType == string(models.MediaTypeTVShow) {
		res, err := client.SearchTVShow(query)
		if err != nil {
			return nil, err
		}
		results = append(results, res)
	} else {
		return nil, fmt.Errorf("unsupported media type for metadata search: %s", mediaType)
	}
	return results, nil
}

func (m *Manager) GetSystemStatus() map[string]bool {
	// This should be improved to give real status
	return map[string]bool{
		"indexer":        true,
		"torrent_client": true,
		"metadata":       true,
	}
}

func (m *Manager) TestIndexerConnection() bool {
	// A basic test, a better one would ping the client
	for _, clients := range m.indexerClients {
		for _, client := range clients {
			ok, err := client.HealthCheck()
			if err != nil || !ok {
				m.logger.Error("Indexer health check failed:", err)
				return false
			}
		}
	}
	return true
}

func (m *Manager) TestTorrentConnection() bool {
	// A basic test, a better one would ping the client
	return m.torrentClient != nil
}

func (m *Manager) performSearch(media *models.Media, season, episode int) ([]indexers.IndexerResult, error) {
	clients := m.indexerClients[media.Type]
	if len(clients) == 0 {
		return nil, fmt.Errorf("no indexer sources configured for media type: %s", media.Type)
	}

	query := media.Title
	if media.Type == models.MediaTypeMovie {
		query = fmt.Sprintf("%s %d", media.Title, media.Year)
	} else if media.Type == models.MediaTypeTVShow && season > 0 && episode > 0 {
		// Try SxxExx format first
		query = fmt.Sprintf("%s S%02dE%02d", media.Title, season, episode)
	}

	tmdbIDStr := ""
	if media.TMDBId != nil {
		tmdbIDStr = strconv.Itoa(*media.TMDBId)
	}

	var allResults []indexers.IndexerResult
	for _, client := range clients {
		results, err := client.SearchMovies(query, tmdbIDStr)
		if err != nil {
			m.logger.Error("Search failed for indexer:", err)
			continue
		}
		allResults = append(allResults, results...)
	}

	// If no results with SxxExx, try 1x01 format
	if len(allResults) == 0 && media.Type == models.MediaTypeTVShow && season > 0 && episode > 0 {
		query = fmt.Sprintf("%s %dx%02d", media.Title, season, episode)
		for _, client := range clients {
			results, err := client.SearchMovies(query, tmdbIDStr)
			if err != nil {
				m.logger.Error("Search failed for indexer with alternative format:", err)
				continue
			}
			allResults = append(allResults, results...)
		}
	}

	m.logger.Info(fmt.Sprintf("Found %d total results for %s", len(allResults), media.Title))
	return allResults, nil
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

	for i := range results {
		results[i].Score = getQualityScore(results[i].Title)
		results[i].Score += results[i].Seeders
	}

	sort.Slice(results, func(i, j int) bool {
		return results[i].Score > results[j].Score
	})

	return results, nil
}

func (m *Manager) StartDownload(id int, torrent indexers.IndexerResult) error {
	m.logger.Info("🚀 Sending to download client:", m.config.TorrentClient.Type)
	hash, err := m.torrentClient.AddTorrent(torrent.DownloadURL, m.config.TorrentClient.DownloadPath)
	if err != nil {
		m.logger.Error("Failed to add torrent to client:", err)
		m.mediaRepo.UpdateStatus(id, models.StatusFailed)
		return err
	}

	m.logger.Info("✅ Torrent successfully sent to download client! Hash:", hash)

	if err := m.mediaRepo.UpdateDownloadInfo(id, models.StatusDownloading, &hash, &torrent.Title); err != nil {
		m.logger.Error("Failed to update media status after adding torrent:", err)
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

	if media.Type != models.MediaTypeTVShow {
		return nil, fmt.Errorf("media is not a TV show")
	}

	// Perform search with specific season/episode
	results, err := m.performSearch(media, seasonNumber, episodeNumber)
	if err != nil {
		return nil, err
	}

	// Score and sort results
	for i := range results {
		results[i].Score = getQualityScore(results[i].Title)
		results[i].Score += results[i].Seeders
	}

	sort.Slice(results, func(i, j int) bool {
		return results[i].Score > results[j].Score
	})

	m.logger.Info(fmt.Sprintf("Found %d results for %s S%02dE%02d",
		len(results), media.Title, seasonNumber, episodeNumber))

	return results, nil
}

// StartEpisodeDownload starts downloading a specific episode with a chosen torrent
func (m *Manager) StartEpisodeDownload(mediaID int, seasonNumber int, episodeNumber int, torrent indexers.IndexerResult) error {
	media, err := m.mediaRepo.GetByID(mediaID)
	if err != nil {
		return err
	}
	if media == nil {
		return fmt.Errorf("media not found")
	}

	if media.Type != models.MediaTypeTVShow {
		return fmt.Errorf("media is not a TV show")
	}

	m.logger.Info(fmt.Sprintf("🚀 Starting manual download for %s S%02dE%02d: %s",
		media.Title, seasonNumber, episodeNumber, torrent.Title))

	// Start the torrent download
	hash, err := m.torrentClient.AddTorrent(torrent.DownloadURL, m.config.TorrentClient.DownloadPath)
	if err != nil {
		m.logger.Error("Failed to add episode torrent to client:", err)
		return err
	}

	m.logger.Info("✅ Episode torrent successfully sent to download client! Hash:", hash)

	// Update the specific episode status in database
	if err := m.mediaRepo.UpdateEpisodeDownloadInfo(mediaID, seasonNumber, episodeNumber, models.StatusDownloading, &hash, &torrent.Title); err != nil {
		m.logger.Error("Failed to update episode status after adding torrent:", err)
		return err
	}

	return nil
}
