package core

import (
	"fmt"
	"os"
	"path/filepath"

	"reel/internal/clients/notifications"
	"reel/internal/clients/torrent"
	"reel/internal/config"
	"reel/internal/database/models"
	"reel/internal/utils"
)

// PostProcessor handles the tasks after a download is complete.
type PostProcessor struct {
	config    *config.Config
	logger    *utils.Logger
	mediaRepo *models.MediaRepository
	notifiers []notifications.Notifier
}

// NewPostProcessor creates a new instance of the PostProcessor.
func NewPostProcessor(cfg *config.Config, logger *utils.Logger, mediaRepo *models.MediaRepository, notifiers []notifications.Notifier) *PostProcessor {
	return &PostProcessor{
		config:    cfg,
		logger:    logger,
		mediaRepo: mediaRepo,
		notifiers: notifiers,
	}
}

// ProcessDownload is the main entry point for post-processing a completed download.
func (pp *PostProcessor) ProcessDownload(media models.Media, torrentStatus torrent.TorrentStatus, seasonNumber int) {
	pp.logger.Info("Starting post-processing for:", media.Title)

	destinationPath := pp.createDestinationFolder(&media, seasonNumber)
	if destinationPath == "" {
		pp.logger.Error("Failed to create destination folder for:", media.Title)
		return
	}

	mediaFiles := pp.identifyMediaFiles(&media, torrentStatus)
	if len(mediaFiles) == 0 {
		pp.logger.Error("No media files identified for:", media.Title)
		return
	}

	pp.moveOrLinkFiles(&media, mediaFiles, destinationPath)

	pp.renameFiles(&media, destinationPath)

	// Send notification at the very end of the pipeline.
	pp.notifyDownloadCompleted(&media, torrentStatus.Name)

	pp.logger.Info("Finished post-processing for:", media.Title)
}

// createDestinationFolder handles the creation of the final directory for the media.
func (pp *PostProcessor) createDestinationFolder(media *models.Media, seasonNumber int) string {
	var baseDestPath string

	switch media.Type {
	case models.MediaTypeMovie:
		baseDestPath = pp.config.Movies.DestinationFolder
	case models.MediaTypeTVShow:
		baseDestPath = pp.config.TVShows.DestinationFolder
	case models.MediaTypeAnime:
		baseDestPath = pp.config.Anime.DestinationFolder
	default:
		pp.logger.Error("Unknown media type for destination path:", media.Type)
		return ""
	}

	// Sanitize folder name to remove invalid characters
	safeTitle := utils.SanitizeFilename(media.Title)
	mediaFolderName := fmt.Sprintf("%s (%d)", safeTitle, media.Year)
	fullPath := filepath.Join(baseDestPath, mediaFolderName)

	// If it's a show, add the season subfolder
	if media.Type == models.MediaTypeTVShow || media.Type == models.MediaTypeAnime {
		seasonFolderName := fmt.Sprintf("S%02d", seasonNumber)
		fullPath = filepath.Join(fullPath, seasonFolderName)
	}

	// Create the directory structure. os.MkdirAll will not return an error if the path already exists.
	err := os.MkdirAll(fullPath, os.ModePerm)
	if err != nil {
		pp.logger.Error("Failed to create destination folder:", fullPath, "Error:", err)
		return ""
	}

	pp.logger.Info("Successfully created or verified destination folder:", fullPath)
	return fullPath
}

// identifyMediaFiles finds the relevant video and subtitle files within the downloaded content.
func (pp *PostProcessor) identifyMediaFiles(media *models.Media, torrentStatus torrent.TorrentStatus) []string {
	pp.logger.Info("[DUMMY] Identifying media files in torrent:", torrentStatus.Name)
	// In the future, this will scan the download folder and return a list of file paths.
	return []string{"/dummy/path/to/movie.mkv", "/dummy/path/to/movie.srt"}
}

// moveOrLinkFiles moves or symlinks the identified media files to the destination folder.
func (pp *PostProcessor) moveOrLinkFiles(media *models.Media, files []string, destination string) {
	pp.logger.Info("[DUMMY] Moving/linking files for:", media.Title, "to", destination)
	// In the future, this will perform the actual file operation based on the config.
}

// renameFiles renames the moved/linked files to a clean, standardized format.
func (pp *PostProcessor) renameFiles(media *models.Media, destination string) {
	pp.logger.Info("[DUMMY] Renaming files for:", media.Title, "in", destination)
	// In the future, this will rename files to a format like "Movie Title (2023).mkv".
}

// notifyDownloadCompleted dispatches notifications for a completed download.
func (pp *PostProcessor) notifyDownloadCompleted(media *models.Media, torrentName string) {
	for _, n := range pp.notifiers {
		// Run in a goroutine to avoid blocking the main application flow.
		go n.NotifyDownloadComplete(media, torrentName)
	}
}
