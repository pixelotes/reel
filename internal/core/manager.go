package core

import (
	"database/sql"
	"fmt"
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

type Manager struct {
	config          *config.Config
	mediaRepo       *models.MediaRepository
	indexerClient   indexers.Client // Use the generic interface
	torrentClient   torrent.TorrentClient
	metadataClients []metadata.Client
	logger          *utils.Logger
	scheduler       *cron.Cron
}

func NewManager(cfg *config.Config, db *sql.DB, logger *utils.Logger) *Manager {
	m := &Manager{
		config:    cfg,
		mediaRepo: models.NewMediaRepository(db),
		logger:    logger,
		scheduler: cron.New(),
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

	return m
}

func (m *Manager) AddMedia(mediaType models.MediaType, imdbID, title string, year int, language, minQuality, maxQuality string) (*models.Media, error) {
	var overview, posterURL *string
	var rating *float64
	var tmdbID *int

	for _, client := range m.metadataClients {
		if mediaType == models.MediaTypeMovie {
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
				if id, err := strconv.Atoi(movieData.ID); err == nil {
					tmdbID = &id
				}
				break
			}
		}
	}

	media := &models.Media{
		Type:       mediaType,
		IMDBId:     imdbID,
		TMDBId:     tmdbID,
		Title:      title,
		Year:       year,
		Language:   language,
		MinQuality: minQuality,
		MaxQuality: maxQuality,
		Status:     models.StatusPending,
		Overview:   overview,
		PosterURL:  posterURL,
		Rating:     rating,
	}

	if err := m.mediaRepo.Create(media); err != nil {
		return nil, fmt.Errorf("failed to create media: %w", err)
	}

	m.logger.Info("Added new media:", media.Title, "("+string(media.Type)+")")
	return media, nil
}

func (m *Manager) GetAllMedia() ([]models.Media, error) {
	return m.mediaRepo.GetAll()
}

func (m *Manager) StartScheduler() {
	m.scheduler.AddFunc(m.config.Automation.SearchInterval, m.processPendingMedia)
	m.scheduler.AddFunc("@every 10m", m.updateDownloadStatus)
	m.scheduler.Start()
	m.logger.Info("Scheduler started")
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
		return
	}

	for _, media := range pendingMedia {
		if err := m.searchAndDownload(&media); err != nil {
			m.logger.Error("Failed to process media:", media.Title, err)
		}
	}
}

func (m *Manager) searchAndDownload(media *models.Media) error {
	media.Status = models.StatusSearching
	m.mediaRepo.Update(media)

	query := fmt.Sprintf("%s %d", media.Title, media.Year)
	results, err := m.indexerClient.SearchMovies(query, media.IMDBId)
	if err != nil {
		media.Status = models.StatusFailed
		m.mediaRepo.Update(media)
		return fmt.Errorf("search failed: %w", err)
	}

	bestTorrent := m.selectBestTorrent(results, media)
	if bestTorrent == nil {
		media.Status = models.StatusFailed
		m.mediaRepo.Update(media)
		return fmt.Errorf("no suitable torrent found")
	}

	hash, err := m.torrentClient.AddTorrent(bestTorrent.DownloadURL, m.config.TorrentClient.DownloadPath)
	if err != nil {
		media.Status = models.StatusFailed
		m.mediaRepo.Update(media)
		return fmt.Errorf("failed to add torrent: %w", err)
	}

	media.Status = models.StatusDownloading
	media.TorrentHash = &hash
	media.TorrentName = &bestTorrent.Title
	m.mediaRepo.Update(media)

	m.logger.Info("Started downloading:", media.Title)
	return nil
}

// Updated to use the generic IndexerResult
func (m *Manager) selectBestTorrent(results []indexers.IndexerResult, media *models.Media) *indexers.IndexerResult {
	var bestTorrent *indexers.IndexerResult
	bestScore := -1

	for i := range results {
		result := results[i]
		if result.Seeders < m.config.Automation.MinSeeders {
			continue
		}

		score := result.Seeders
		title := strings.ToLower(result.Title)

		for _, quality := range m.config.Automation.QualityPreferences {
			if strings.Contains(title, strings.ToLower(quality)) {
				score += 1000
				break
			}
		}

		if score > bestScore {
			bestScore = score
			bestTorrent = &result
		}
	}

	return bestTorrent
}

func (m *Manager) updateDownloadStatus() {
	downloadingMedia, err := m.mediaRepo.GetByStatus(models.StatusDownloading)
	if err != nil {
		m.logger.Error("Failed to get downloading media:", err)
		return
	}

	for i := range downloadingMedia {
		media := downloadingMedia[i]
		if media.TorrentHash == nil {
			continue
		}

		status, err := m.torrentClient.GetTorrentStatus(*media.TorrentHash)
		if err != nil {
			m.logger.Error("Failed to get torrent status:", err)
			continue
		}

		media.Progress = status.Progress

		if status.IsCompleted {
			media.Status = models.StatusDownloaded
			now := time.Now()
			media.CompletedAt = &now
			m.logger.Info("Download completed:", media.Title)
		}

		m.mediaRepo.Update(&media)
	}
}

func (m *Manager) DeleteMedia(id int) error {
	return m.mediaRepo.Delete(id)
}

func (m *Manager) RetryMedia(id int) error {
	allMedia, err := m.mediaRepo.GetAll()
	if err != nil {
		return err
	}

	var mediaToRetry *models.Media
	for i := range allMedia {
		if allMedia[i].ID == id {
			mediaToRetry = &allMedia[i]
			break
		}
	}

	if mediaToRetry == nil {
		return fmt.Errorf("media with id %d not found", id)
	}

	if mediaToRetry.Status == models.StatusFailed {
		mediaToRetry.Status = models.StatusPending
		return m.mediaRepo.Update(mediaToRetry)
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
		for _, client := range m.metadataClients {
			result, err := client.SearchMovie(query, 0)
			if err == nil && result != nil {
				return []*metadata.MovieResult{result}, nil
			}
			m.logger.Error("Metadata search failed with one provider:", err)
		}
	}
	return nil, fmt.Errorf("no metadata provider could find results for '%s'", query)
}

func (m *Manager) GetSystemStatus() map[string]bool {
	return map[string]bool{
		"indexer":        m.TestIndexerConnection(),
		"torrent_client": m.TestTorrentConnection(),
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
	if m.torrentClient == nil {
		return false
	}
	_, err := m.torrentClient.AddTorrent("magnet:?xt=urn:btih:0000000000000000000000000000000000000000", m.config.TorrentClient.DownloadPath)
	return err == nil || strings.Contains(err.Error(), "login failed")
}
