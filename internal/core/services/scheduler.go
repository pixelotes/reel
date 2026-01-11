package services

import (
	"reel/internal/database/models"
	"reel/internal/utils"

	"github.com/robfig/cron/v3"
)

type SchedulerService struct {
	cron              *cron.Cron
	searchQueue       chan<- models.Media
	libraryService    *LibraryService
	rssService        *RSSService
	downloaderService *DownloaderService
	logger            *utils.Logger
}

func NewSchedulerService(queue chan<- models.Media, lib *LibraryService, rss *RSSService, dl *DownloaderService, logger *utils.Logger) *SchedulerService {
	return &SchedulerService{
		cron:              cron.New(),
		searchQueue:       queue,
		libraryService:    lib,
		rssService:        rss,
		downloaderService: dl,
		logger:            logger,
	}
}

func (s *SchedulerService) Start() {
	s.cron.AddFunc("@every 30m", s.processPendingMedia)
	s.cron.AddFunc("@every 6h", s.libraryService.CheckForNewEpisodes)
	s.cron.AddFunc("@every 10s", s.downloaderService.UpdateDownloadStatus)
	s.cron.AddFunc("@every 1h", s.rssService.ProcessRSSFeeds)
	s.cron.AddFunc("@every 24h", s.downloaderService.CleanupCompletedTorrents)
	// Retry logic is combined into processPendingMedia

	s.cron.Start()
	s.logger.Info("Scheduler started.")

	// Run immediate tasks on startup
	go s.processPendingMedia()
	go s.rssService.ProcessRSSFeeds()
}

func (s *SchedulerService) Stop() {
	if s.cron != nil {
		s.cron.Stop()
	}
}

func (s *SchedulerService) processPendingMedia() {
	mediaItems := s.libraryService.GetPendingMediaForProcessing()
	if len(mediaItems) > 0 {
		for _, media := range mediaItems {
			// We must create a copy of the media object to avoid a race condition
			// when it is processed in the search queue worker goroutine.
			mediaCopy := media
			select {
			case s.searchQueue <- mediaCopy:
				// Successfully enqueued
			default:
				s.logger.Error("Search queue is full! Skipping media item:", media.Title)
			}
		}
	}
}
