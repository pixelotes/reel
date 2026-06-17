package services

import (
	"fmt"
	"os"
	"strings"
	"sync"
	"syscall"
	"time"

	"reel/internal/clients/indexers"
	"reel/internal/clients/notifications"
	"reel/internal/clients/torrent"
	"reel/internal/config"
	"reel/internal/database/models"
	"reel/internal/utils"
)

type DownloaderService struct {
	config        *config.Config
	logger        *utils.Logger
	torrentClient torrent.TorrentClient
	directClient  torrent.TorrentClient // For direct HTTP downloads (ebooks)
	mediaRepo     *models.MediaRepository
	postProcessor *PostProcessor
	notifiers     []notifications.Notifier
	statusMu      sync.Mutex
}

func NewDownloaderService(cfg *config.Config, logger *utils.Logger, torrentClient torrent.TorrentClient, mediaRepo *models.MediaRepository, pp *PostProcessor, notifiers []notifications.Notifier) *DownloaderService {
	return &DownloaderService{
		config:        cfg,
		logger:        logger,
		torrentClient: torrentClient,
		directClient:  torrent.NewDirectDownloadClient(logger),
		mediaRepo:     mediaRepo,
		postProcessor: pp,
		notifiers:     notifiers,
	}
}

// clientFor returns the direct download client for ebooks, torrent client for everything else.
func (d *DownloaderService) clientFor(mediaType models.MediaType) torrent.TorrentClient {
	if mediaType == models.MediaTypeEbook || mediaType == models.MediaTypeManga {
		return d.directClient
	}
	return d.torrentClient
}

func (d *DownloaderService) checkDiskSpace(downloadPath string, size int64, mediaID int, title string) error {
	const securityBuffer int64 = 500 * 1024 * 1024 // 500MB
	requiredSpace := uint64(size + securityBuffer)

	var stat syscall.Statfs_t
	if err := syscall.Statfs(downloadPath, &stat); err != nil {
		d.logger.Error("Failed to check disk space for path", downloadPath, ":", err)
		return fmt.Errorf("could not verify disk space: %w", err)
	}
	freeSpace := stat.Bavail * uint64(stat.Bsize)

	if freeSpace < requiredSpace {
		d.logger.Warn(fmt.Sprintf("Not enough disk space in %s. Required: %d bytes, Available: %d bytes", downloadPath, requiredSpace, freeSpace))

		// Create a dummy media object for notification (since we only have ID here usually, but caller might have media)
		// Ideally we should pass media object, but let's see usages.
		// Caller passes mediaID. We might need to fetch media or let caller handle notification.
		// Actually StartDownload fetches media.

		return fmt.Errorf("not enough disk space")
	}
	return nil
}

func (d *DownloaderService) StartDownload(mediaID int, t indexers.IndexerResult) error {
	media, err := d.mediaRepo.GetByID(mediaID)
	if err != nil {
		return err
	}
	if media == nil {
		return fmt.Errorf("media not found")
	}

	var downloadPath string
	switch media.Type {
	case models.MediaTypeMovie:
		downloadPath = d.config.Movies.DownloadFolder
	case models.MediaTypeTVShow:
		downloadPath = d.config.TVShows.DownloadFolder
	case models.MediaTypeAnime:
		downloadPath = d.config.Anime.DownloadFolder
	case models.MediaTypeEbook:
		downloadPath = d.config.Ebooks.DownloadFolder
	default:
		downloadPath = d.config.TorrentClient.DownloadPath // Fallback
	}

	if err := os.MkdirAll(downloadPath, os.ModePerm); err != nil {
		return fmt.Errorf("failed to create download directory: %w", err)
	}

	if err := d.checkDiskSpace(downloadPath, t.Size, mediaID, t.Title); err != nil {
		if err.Error() == "not enough disk space" {
			d.notifyNotEnoughSpace(media, t.Title)
			d.mediaRepo.UpdateStatus(mediaID, models.StatusFailed)
			return fmt.Errorf("not enough disk space to download '%s'", t.Title)
		}
		return err
	}

	client := d.clientFor(media.Type)
	d.logger.Info("Sending to download client:", d.config.TorrentClient.Type)

	var hash string

	if media.Type != models.MediaTypeEbook && d.config.App.MagnetToTorrentEnabled && strings.HasPrefix(t.DownloadURL, "magnet:") {
		timeout := time.Duration(d.config.App.MagnetToTorrentTimeout) * time.Second
		if timeout <= 0 {
			timeout = 60 * time.Second // Default to 60 seconds
		}
		d.logger.Info("Attempting to convert magnet to .torrent with timeout:", timeout)
		torrentFileBytes, convErr := utils.ConvertMagnetToTorrent(t.DownloadURL, timeout, d.config.App.DataPath, d.logger)
		if convErr == nil {
			d.logger.Info("Magnet conversion successful, adding as .torrent file.")
			hash, err = client.AddTorrentFile(torrentFileBytes, downloadPath)
		} else {
			d.logger.Warn("Magnet conversion failed:", convErr, "- falling back to magnet link.")
			hash, err = client.AddTorrent(t.DownloadURL, downloadPath)
		}
	} else {
		hash, err = client.AddTorrent(t.DownloadURL, downloadPath)
	}

	if err != nil {
		// If the torrent already exists, log a warning but treat it as success
		if hash != "" && strings.Contains(err.Error(), "already exists") {
			d.logger.Warn("Torrent already in client, skipping:", err)
		} else {
			d.logger.Error("Failed to add torrent to client:", err)
			d.mediaRepo.UpdateStatus(mediaID, models.StatusFailed)
			return err
		}
	}

	if media.Type != models.MediaTypeEbook {
		d.addExtraTrackers(hash)
	}

	// Notification
	d.notifyDownloadStarted(media, t.Title)
	d.logger.Info("Torrent successfully sent to download client! Hash:", hash)

	if err := d.mediaRepo.UpdateDownloadInfo(mediaID, models.StatusDownloading, &hash, &t.Title); err != nil {
		d.logger.Error("Failed to update media status after adding torrent:", err)
		return err
	}
	return nil
}

func (d *DownloaderService) StartEpisodeDownload(mediaID int, seasonNumber int, episodeNumber int, t indexers.IndexerResult) error {
	media, err := d.mediaRepo.GetByID(mediaID)
	if err != nil {
		return err
	}
	if media == nil {
		return fmt.Errorf("media not found")
	}

	if media.Type != models.MediaTypeTVShow && media.Type != models.MediaTypeAnime && media.Type != models.MediaTypeManga {
		return fmt.Errorf("media is not a TV show, anime, or manga")
	}

	var downloadPath string
	switch media.Type {
	case models.MediaTypeTVShow:
		downloadPath = d.config.TVShows.DownloadFolder
	case models.MediaTypeAnime:
		downloadPath = d.config.Anime.DownloadFolder
	case models.MediaTypeManga:
		downloadPath = d.config.Manga.DownloadFolder
	default:
		downloadPath = d.config.TorrentClient.DownloadPath // Fallback
	}

	if err := os.MkdirAll(downloadPath, os.ModePerm); err != nil {
		return fmt.Errorf("failed to create download directory: %w", err)
	}

	if err := d.checkDiskSpace(downloadPath, t.Size, mediaID, t.Title); err != nil {
		if err.Error() == "not enough disk space" {
			d.notifyNotEnoughSpace(media, t.Title)
			d.mediaRepo.UpdateEpisodeDownloadInfo(mediaID, seasonNumber, episodeNumber, models.StatusFailed, nil, nil)
			return fmt.Errorf("not enough disk space to download '%s'", t.Title)
		}
		return err
	}

	d.logger.Info(fmt.Sprintf("Starting manual download for %s S%02dE%02d: %s",
		media.Title, seasonNumber, episodeNumber, t.Title))

	// Start the download
	client := d.clientFor(media.Type)
	var hash string

	isMangaOrEbook := media.Type == models.MediaTypeManga || media.Type == models.MediaTypeEbook
	if !isMangaOrEbook && d.config.App.MagnetToTorrentEnabled && strings.HasPrefix(t.DownloadURL, "magnet:") {
		timeout := time.Duration(d.config.App.MagnetToTorrentTimeout) * time.Second
		if timeout <= 0 {
			timeout = 60 * time.Second // Default to 60 seconds
		}
		d.logger.Info("Attempting to convert magnet to .torrent with timeout:", timeout)
		torrentFileBytes, convErr := utils.ConvertMagnetToTorrent(t.DownloadURL, timeout, d.config.App.DataPath, d.logger)
		if convErr == nil {
			d.logger.Info("Magnet conversion successful, adding as .torrent file.")
			hash, err = client.AddTorrentFile(torrentFileBytes, downloadPath)
		} else {
			d.logger.Warn("Magnet conversion failed:", convErr, "- falling back to magnet link.")
			hash, err = client.AddTorrent(t.DownloadURL, downloadPath)
		}
	} else {
		hash, err = client.AddTorrent(t.DownloadURL, downloadPath)
	}

	if err != nil {
		if hash != "" && strings.Contains(err.Error(), "already exists") {
			d.logger.Warn("Episode torrent already in client, skipping:", err)
		} else {
			d.logger.Error("Failed to add episode torrent to client:", err)
			return err
		}
	}

	if !isMangaOrEbook {
		d.addExtraTrackers(hash)
	}

	d.logger.Info("Episode torrent successfully sent to download client! Hash:", hash)

	// Update the specific episode status in database
	if err := d.mediaRepo.UpdateEpisodeDownloadInfo(mediaID, seasonNumber, episodeNumber, models.StatusDownloading, &hash, &t.Title); err != nil {
		d.logger.Error("Failed to update episode status after adding torrent:", err)
		return err
	}

	return nil
}

func (d *DownloaderService) addExtraTrackers(hash string) {
	if len(d.config.ExtraTrackersList) > 0 {
		go func() {
			time.Sleep(10 * time.Second)
			d.logger.Info("Adding extra trackers to torrent:", hash)
			err := d.torrentClient.AddTrackers(hash, d.config.ExtraTrackersList)
			if err != nil {
				d.logger.Error("Failed to add extra trackers:", err)
			} else {
				d.logger.Info("Successfully added extra trackers.")
			}
		}()
	}
}

func (d *DownloaderService) UpdateDownloadStatus() {
	if !d.statusMu.TryLock() {
		return // Previous status check still running
	}
	defer d.statusMu.Unlock()

	// Get all media items (movies or series) that have at least one active download.
	downloadingMedia, err := d.mediaRepo.GetByStatus(models.StatusDownloading)
	if err != nil {
		d.logger.Error("Failed to get downloading media:", err)
		return
	}

	for _, media := range downloadingMedia {
		// --- Logic for Movies and Ebooks (no episodes) ---
		if media.Type == models.MediaTypeMovie || media.Type == models.MediaTypeEbook {
			if media.TorrentHash == nil {
				continue
			}
			status, err := d.clientFor(media.Type).GetTorrentStatus(*media.TorrentHash)
			if err != nil {
				d.logger.Error("Failed to get torrent status for", media.Title, ":", err)
				d.mediaRepo.UpdateStatus(media.ID, models.StatusFailed)
				continue
			}

			if status.IsCompleted {
				now := time.Now()
				// Mark as downloaded BEFORE launching post-processing to prevent duplicate runs
				d.mediaRepo.UpdateProgress(media.ID, models.StatusDownloaded, 1.0, &now)
				go func() {
					if err := d.postProcessor.ProcessDownload(media, status, 0, 0, status.DownloadDir); err != nil {
						d.logger.Error("Post-processing failed for", media.Title, ":", err)
						d.mediaRepo.UpdateStatus(media.ID, models.StatusFailed)
					}
				}()
			} else {
				d.mediaRepo.UpdateProgress(media.ID, models.StatusDownloading, status.Progress, nil)
			}
			continue // Move to the next media item
		}

		// --- New Per-Episode Logic for TV Shows & Anime ---
		if media.Type == models.MediaTypeTVShow || media.Type == models.MediaTypeAnime || media.Type == models.MediaTypeManga {
			if media.TVShowID == nil {
				continue
			}

			// Get full show details once to map season IDs to season numbers
			show, err := d.mediaRepo.GetTVShowByMediaID(media.ID)
			if err != nil {
				d.logger.Error("Could not get show details for status update:", err)
				continue
			}
			seasonMap := make(map[int]int)
			for _, s := range show.Seasons {
				seasonMap[s.ID] = s.SeasonNumber
			}

			// Get all individual episodes for this series that are in a 'downloading' state.
			downloadingEpisodes, err := d.mediaRepo.GetDownloadingEpisodesForShow(*media.TVShowID)
			if err != nil {
				d.logger.Error("Could not get downloading episodes for show:", media.Title, err)
				continue
			}

			// Loop through each downloading episode and check its unique hash.
			for _, episode := range downloadingEpisodes {
				if episode.TorrentHash == nil {
					continue
				}

				status, err := d.clientFor(media.Type).GetTorrentStatus(*episode.TorrentHash)
				if err != nil {
					d.logger.Error("Failed to get torrent status for episode:", media.Title, episode.Title, err)
					// Mark this specific episode as failed
					seasonNum := seasonMap[episode.SeasonID]
					d.mediaRepo.UpdateEpisodeDownloadInfo(media.ID, seasonNum, episode.EpisodeNumber, models.StatusFailed, nil, nil)
					continue
				}

				if status.IsCompleted {
					d.logger.Info("Episode download completed:", media.Title, fmt.Sprintf("S%02dE%02d", seasonMap[episode.SeasonID], episode.EpisodeNumber))
					// Mark as downloaded BEFORE launching post-processing to prevent duplicate runs
					d.mediaRepo.UpdateEpisodeDownloadInfo(media.ID, seasonMap[episode.SeasonID], episode.EpisodeNumber, models.StatusDownloaded, nil, nil)
					go func(m models.Media, s torrent.TorrentStatus, sn, en int) {
						if err := d.postProcessor.ProcessDownload(m, s, sn, en, s.DownloadDir); err != nil {
							d.logger.Error(fmt.Sprintf("Post-processing failed for %s S%02dE%02d: %v", m.Title, sn, en, err))
						}
					}(media, status, seasonMap[episode.SeasonID], episode.EpisodeNumber)
				}
				// If not complete, we don't need to do anything here.
				// The overall show progress will be updated below by UpdateShowProgress (called by Scheduler/Library)
				// Or we should call it here? Manager called it at the end.
			}
			// Update overall progress
			// Note: We need to access LibraryService for UpdateShowProgress if we want to call it here.
			// But DownloaderService shouldn't depend on LibraryService (Circular?).
			// We can duplicate the logic or extract it to a shared helper or make UpdateProgress generic in MediaRepo?
			// UpdateShowProgress logic is in Manager (lines 776). It calculates progress.
			// Ideally, MediaRepo could handle "RecalculateProgress(mediaID)".
			// Or we just leave it for now and let the Manager/Scheduler trigger a progress update periodically?
			// Manager.updateDownloadStatus called updateShowProgress at the end.
			// If I remove it, progress won't update in UI until next refresh?

			// Let's implement RecalculateShowProgress in MediaRepo? Or duplicated logic here?
			// Better: Let's assume LibraryService will have this method.
			// But we can't call it.
			// How about we return a list of MediaIDs that need progress update?
			// Or we copy the logic of UpdateShowProgress to `DownloaderService` as a private method or shared helper?
			// Since we have MediaRepo, we can just implement the logic here too. It's safe.
			d.updateShowProgress(media.ID)
		}
	}
}

// updateShowProgress re-calculates the show progress.
// Copied from Manager.go to avoid circular dependency with LibraryService.
func (d *DownloaderService) updateShowProgress(mediaID int) {
	show, err := d.mediaRepo.GetTVShowByMediaID(mediaID)
	if err != nil {
		d.logger.Error("Failed to get show for progress update:", err)
		return
	}
	if show == nil {
		return // Not a show, nothing to do
	}

	var downloadableEpisodes, downloadedEpisodes, pendingEpisodes, downloadingEpisodes, tbaEpisodes int

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
			switch episode.Status {
			case models.StatusPending:
				pendingEpisodes++
			case models.StatusDownloading:
				downloadingEpisodes++
			case models.StatusTBA:
				tbaEpisodes++
			}
		}
	}

	var progress float64
	if downloadableEpisodes > 0 {
		progress = float64(downloadedEpisodes) / float64(downloadableEpisodes)
	}

	// Determine the new overall status for the media item
	var newStatus models.MediaStatus
	if downloadingEpisodes > 0 {
		newStatus = models.StatusDownloading
	} else if pendingEpisodes > 0 {
		newStatus = models.StatusPending
	} else {
		if tbaEpisodes > 0 || strings.ToLower(show.Status) == "running" {
			newStatus = models.StatusMonitoring
		} else {
			newStatus = models.StatusDownloaded
		}
	}

	d.mediaRepo.UpdateProgress(mediaID, newStatus, progress, nil)
	d.logger.Info("Updated show progress for Media ID", mediaID, "New Status:", newStatus, "Progress:", progress)
}

func (d *DownloaderService) CleanupCompletedTorrents() {
	if d.config.Automation.KeepTorrentsForDays <= 0 && d.config.Automation.KeepTorrentsSeedRatio <= 0 {
		return // Feature is disabled
	}

	downloadedMedia, err := d.mediaRepo.GetByStatus(models.StatusDownloaded)
	if err != nil {
		d.logger.Error("Failed to get downloaded media for cleanup:", err)
		return
	}

	cleanupThreshold := time.Now().AddDate(0, 0, -d.config.Automation.KeepTorrentsForDays)

	for _, media := range downloadedMedia {
		if media.CompletedAt != nil && media.TorrentHash != nil {
			status, err := d.torrentClient.GetTorrentStatus(*media.TorrentHash)
			if err != nil {
				d.logger.Error("Failed to get torrent status for cleanup:", err)
				continue
			}

			shouldDelete := false
			if d.config.Automation.KeepTorrentsForDays > 0 && media.CompletedAt.Before(cleanupThreshold) {
				shouldDelete = true
			}
			if d.config.Automation.KeepTorrentsSeedRatio > 0 && status.UploadRatio >= d.config.Automation.KeepTorrentsSeedRatio {
				shouldDelete = true
			}

			if shouldDelete {
				d.logger.Info("Cleaning up torrent for:", media.Title)
				if err := d.torrentClient.RemoveTorrent(*media.TorrentHash); err != nil {
					d.logger.Error("Failed to remove torrent from client:", err)
				} else {
					d.mediaRepo.UpdateStatus(media.ID, models.StatusArchived)
				}
			}
		}
	}
}

func (d *DownloaderService) notifyNotEnoughSpace(media *models.Media, torrentName string) {
	for _, n := range d.notifiers {
		go n.NotifyNotEnoughSpace(media, torrentName)
	}
}

func (d *DownloaderService) notifyDownloadStarted(media *models.Media, torrentName string) {
	for _, n := range d.notifiers {
		go n.NotifyDownloadStart(media, torrentName)
	}
}

func (d *DownloaderService) GetTorrentClientStatus() (bool, error) {
	if d.torrentClient == nil {
		return false, fmt.Errorf("torrent client not initialized")
	}
	return d.torrentClient.HealthCheck()
}

func (d *DownloaderService) TestTorrentConnection() (bool, error) {
	return d.GetTorrentClientStatus()
}
