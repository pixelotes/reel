package core

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

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
func (pp *PostProcessor) ProcessDownload(media models.Media, torrentStatus torrent.TorrentStatus, seasonNumber int, episodeNumber int) {
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

	pp.renameFiles(&media, destinationPath, seasonNumber, episodeNumber)

	// Send post-processing completion notification
	pp.notifyPostProcessCompleted(&media, torrentStatus.Name)

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
	if (media.Type == models.MediaTypeTVShow || media.Type == models.MediaTypeAnime) && seasonNumber > 0 {
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
	videoExtensions := map[string]bool{".mkv": true, ".mp4": true, ".avi": true, ".mov": true}
	subtitleExtensions := map[string]bool{".srt": true, ".sub": true, ".ass": true}

	var files []string
	downloadPath := *media.DownloadPath

	filepath.Walk(downloadPath, func(path string, info fs.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if !info.IsDir() {
			ext := strings.ToLower(filepath.Ext(path))
			if videoExtensions[ext] || subtitleExtensions[ext] {
				files = append(files, path)
			}
		}
		return nil
	})
	return files
}

// moveOrLinkFiles moves or symlinks the identified media files to the destination folder.
func (pp *PostProcessor) moveOrLinkFiles(media *models.Media, files []string, destination string) {
	var moveMethod string
	switch media.Type {
	case models.MediaTypeMovie:
		moveMethod = pp.config.Movies.MoveMethod
	case models.MediaTypeTVShow:
		moveMethod = pp.config.TVShows.MoveMethod
	case models.MediaTypeAnime:
		moveMethod = pp.config.Anime.MoveMethod
	}

	for _, file := range files {
		newPath := filepath.Join(destination, filepath.Base(file))
		if moveMethod == "move" {
			err := os.Rename(file, newPath)
			if err != nil {
				pp.logger.Error("Failed to move file:", err)
			}
		} else {
			err := os.Link(file, newPath)
			if err != nil {
				pp.logger.Error("Failed to link file:", err)
			}
		}
	}
}

// renameFiles renames the moved/linked files to a clean, standardized format.
func (pp *PostProcessor) renameFiles(media *models.Media, destination string, season, episode int) {
	files, err := os.ReadDir(destination)
	if err != nil {
		pp.logger.Error("Failed to read destination directory:", err)
		return
	}

	for _, file := range files {
		oldPath := filepath.Join(destination, file.Name())
		ext := filepath.Ext(file.Name())
		quality := media.MaxQuality

		var newName string
		if media.Type == models.MediaTypeMovie {
			newName = fmt.Sprintf("%s (%d) [%s]%s", media.Title, media.Year, quality, ext)
		} else {
			newName = fmt.Sprintf("%s - S%02dE%02d [%s]%s", media.Title, season, episode, quality, ext)
		}
		newPath := filepath.Join(destination, newName)

		err := os.Rename(oldPath, newPath)
		if err != nil {
			pp.logger.Error("Failed to rename file:", err)
		}
	}
}

func (pp *PostProcessor) notifyPostProcessCompleted(media *models.Media, torrentName string) {
	pp.logger.Info("Sending post-processing completion notifications to", len(pp.notifiers), "notifiers")
	for i, n := range pp.notifiers {
		pp.logger.Info("Sending post-process notification via notifier", i)
		go func(notifier notifications.Notifier, index int) {
			notifier.NotifyPostProcessComplete(media, torrentName)
			pp.logger.Info("Completed post-process notification for notifier", index)
		}(n, i)
	}
}
