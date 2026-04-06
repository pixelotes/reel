package services

import (
	"reel/internal/config"
	"reel/internal/database/models"
	"reel/internal/utils"

	"github.com/robfig/cron/v3"
)

type SchedulerService struct {
	config            *config.Config
	cron              *cron.Cron
	searchQueue       chan<- models.Media
	libraryService    *LibraryService
	rssService        *RSSService
	downloaderService *DownloaderService
	logger            *utils.Logger
}

func NewSchedulerService(cfg *config.Config, queue chan<- models.Media, lib *LibraryService, rss *RSSService, dl *DownloaderService, logger *utils.Logger) *SchedulerService {
	return &SchedulerService{
		config:            cfg,
		cron:              cron.New(),
		searchQueue:       queue,
		libraryService:    lib,
		rssService:        rss,
		downloaderService: dl,
		logger:            logger,
	}
}

func (s *SchedulerService) getInterval(cfgValue, defaultValue string) string {
	if cfgValue != "" {
		return cfgValue
	}
	return defaultValue
}

func (s *SchedulerService) Start() {
	s.cron.AddFunc(s.getInterval(s.config.Automation.SearchInterval, "@every 30m"), s.processPendingMedia)
	s.cron.AddFunc(s.getInterval(s.config.Automation.NewEpisodesCheckInterval, "@every 6h"), s.libraryService.CheckForNewEpisodes)
	s.cron.AddFunc(s.getInterval(s.config.Automation.DownloadStatusInterval, "@every 10s"), s.downloaderService.UpdateDownloadStatus)
	s.cron.AddFunc(s.getInterval(s.config.Automation.RSSProcessingInterval, "@every 1h"), s.rssService.ProcessRSSFeeds)
	s.cron.AddFunc(s.getInterval(s.config.Automation.CleanupInterval, "@every 24h"), s.downloaderService.CleanupCompletedTorrents)
	s.cron.AddFunc(s.getInterval(s.config.Automation.RetryFailedInterval, "@every 1h"), s.processPendingMedia) // Retry logic reused
	s.cron.AddFunc(s.getInterval(s.config.Automation.SubtitleScanInterval, "@every 12h"), s.libraryService.ScanMissingSubtitles)

	s.cron.Start()
	s.logger.Info("Scheduler started with dynamic intervals.")

	// Run immediate tasks on startup
	go s.processPendingMedia()
	go s.rssService.ProcessRSSFeeds()
	go s.libraryService.ScanMissingSubtitles()
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
