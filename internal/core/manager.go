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
	indexerClient   indexers.Client
	torrentClient   torrent.TorrentClient
	metadataClients []metadata.Client
	logger          *utils.Logger
	scheduler       *cron.Cron
	searchQueue     chan *models.Media
}

func NewManager(cfg *config.Config, db *sql.DB, logger *utils.Logger) *Manager {
	m := &Manager{
		config:      cfg,
		mediaRepo:   models.NewMediaRepository(db),
		logger:      logger,
		scheduler:   cron.New(),
		searchQueue: make(chan *models.Media, 100),
	}

	// Setup Indexer based on config
	switch cfg.Indexer.Type {
	case "scarf":
		timeout, _ := time.ParseDuration("30s")
		m.indexerClient = indexers.NewScarfClient(cfg.Indexer.URL, cfg.Indexer.APIKey, timeout)
	case "jackett":
		m.indexerClient = indexers.NewJackettClient(cfg.Indexer.URL, cfg.Indexer.APIKey)
	default:
		logger.Fatal("Unsupported indexer type:", cfg.Indexer.Type)
	}

	// Setup Torrent Client
	switch cfg.TorrentClient.Type {
	case "transmission":
		m.torrentClient = torrent.NewTransmissionClient(cfg.TorrentClient.Host, cfg.TorrentClient.Username, cfg.TorrentClient.Password)
	case "qbittorrent":
		m.torrentClient = torrent.NewQBittorrentClient(cfg.TorrentClient.Host, cfg.TorrentClient.Username, cfg.TorrentClient.Password)
	default:
		logger.Fatal("Unsupported torrent client type:", cfg.TorrentClient.Type)
	}

	// Setup Metadata Clients based on config order
	for _, provider := range cfg.Metadata.Providers {
		switch provider {
		case "tmdb":
			m.metadataClients = append(m.metadataClients, metadata.NewTMDBClient(cfg.Metadata.TMDB.APIKey, cfg.Metadata.Language))
		case "imdb":
			m.metadataClients = append(m.metadataClients, metadata.NewIMDBClient(cfg.Metadata.IMDB.APIKey))
		}
	}

	go m.startSearchQueueWorker()

	return m
}

func (m *Manager) startSearchQueueWorker() {
	m.logger.Info("Search queue worker started.")
	for media := range m.searchQueue {
		m.searchAndDownload(media)
		m.logger.Info("Waiting 30s before next search...")
		time.Sleep(30 * time.Second)
	}
}

func (m *Manager) AddMedia(mediaType models.MediaType, tmdbID int, title string, year int, language, minQuality, maxQuality string, autoDownload bool) (*models.Media, error) {
	var overview, posterURL *string
	var rating *float64

	// Simplified metadata fetch
	if len(m.metadataClients) > 0 {
		client := m.metadataClients[0]
		movieData, err := client.SearchMovie(title, year)
		if err == nil && movieData != nil {
			overview = &movieData.Overview
			posterURL = &movieData.PosterURL
			rating = &movieData.Rating
			if title == "" {
				title = movieData.Title
			}
			if year == 0 {
				year = movieData.Year
			}
		}
	}

	media := &models.Media{
		Type:         mediaType,
		TMDBId:       &tmdbID,
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

	if err := m.mediaRepo.Create(media); err != nil {
		return nil, fmt.Errorf("failed to create media: %w", err)
	}

	m.logger.Info("Added new media:", media.Title)
	if autoDownload {
		m.logger.Info("It will be searched for shortly.")
		m.searchQueue <- media
	}

	return media, nil
}

func (m *Manager) searchAndDownload(media *models.Media) {
	m.logger.Info("🔍 Starting automatic search for:", media.Title)
	m.mediaRepo.UpdateStatus(media.ID, models.StatusSearching)

	results, err := m.performSearch(media)
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
	return m.mediaRepo.GetAll()
}

func (m *Manager) StartScheduler() {
	m.scheduler.AddFunc("@every 30m", m.processPendingMedia)
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
		// Don't return here, we still want to process failed media
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
				m.searchQueue <- &mediaCopy
			}
		}
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
				// Mark as failed
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
		m.searchQueue <- media
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

func (m *Manager) SearchMetadata(query string, mediaType string) ([]*metadata.MovieResult, error) {
	if mediaType == string(models.MediaTypeMovie) {
		if len(m.metadataClients) > 0 {
			client := m.metadataClients[0]
			result, err := client.SearchMovie(query, 0)
			if err == nil && result != nil {
				return []*metadata.MovieResult{result}, nil
			}
			return nil, err
		}
	}
	return nil, fmt.Errorf("no metadata provider configured for '%s'", mediaType)
}

func (m *Manager) GetSystemStatus() map[string]bool {
	// This should be improved to give real status
	return map[string]bool{
		"indexer":        true,
		"torrent_client": true,
		"metadata":       len(m.metadataClients) > 0,
	}
}

func (m *Manager) TestIndexerConnection() bool {
	if m.indexerClient == nil {
		return false
	}
	ok, err := m.indexerClient.HealthCheck()
	if err != nil {
		m.logger.Error("Indexer health check failed:", err)
	}
	return ok
}

func (m *Manager) TestTorrentConnection() bool {
	// A basic test, a better one would ping the client
	return m.torrentClient != nil
}

func (m *Manager) performSearch(media *models.Media) ([]indexers.IndexerResult, error) {
	query := fmt.Sprintf("%s %d", media.Title, media.Year)
	tmdbIDStr := ""
	if media.TMDBId != nil {
		tmdbIDStr = strconv.Itoa(*media.TMDBId)
	}

	results, err := m.indexerClient.SearchMovies(query, tmdbIDStr)
	if err != nil {
		return nil, err
	}
	m.logger.Info(fmt.Sprintf("Found %d results for %s", len(results), media.Title))
	return results, nil
}

func (m *Manager) PerformSearch(id int) ([]indexers.IndexerResult, error) {
	media, err := m.mediaRepo.GetByID(id)
	if err != nil {
		return nil, err
	}
	if media == nil {
		return nil, fmt.Errorf("media not found")
	}

	results, err := m.performSearch(media)
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
