package core

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"reel/internal/clients/notifications"
	"reel/internal/clients/torrent"
	"reel/internal/config"
	"reel/internal/database/models"
	"reel/internal/utils"

	"github.com/martinlindhe/subtitles"
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
func (pp *PostProcessor) ProcessDownload(media models.Media, torrentStatus torrent.TorrentStatus, seasonNumber int, episodeNumber int, downloadPath string) error {
	pp.logger.Info("Starting post-processing for:", media.Title)

	destinationPath := pp.createDestinationFolder(&media, seasonNumber)
	if destinationPath == "" {
		err := fmt.Errorf("failed to create destination folder for: %s", media.Title)
		pp.logger.Error(err.Error())
		return err
	}

	mediaFiles := pp.identifyMediaFiles(downloadPath, torrentStatus.Files)
	if len(mediaFiles) == 0 {
		err := fmt.Errorf("no media files identified for: %s", media.Title)
		pp.logger.Error(err.Error())
		return err
	}

	if err := pp.processFilesWithFallback(&media, mediaFiles, destinationPath); err != nil {
		return err
	}

	newVideoFileName := pp.renameFiles(&media, destinationPath, seasonNumber, episodeNumber, torrentStatus.Name, mediaFiles)

	// After renaming, if we have a video file, try to get subtitles for it.
	if newVideoFileName != "" {
		pp.downloadSubtitles(&media, destinationPath, newVideoFileName)
	}

	pp.notifyPostProcessCompleted(&media, torrentStatus.Name)

	pp.logger.Info("Finished post-processing for:", media.Title)
	return nil
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

	safeTitle := utils.SanitizeFilename(media.Title)
	mediaFolderName := fmt.Sprintf("%s (%d)", safeTitle, media.Year)
	fullPath := filepath.Join(baseDestPath, mediaFolderName)

	if (media.Type == models.MediaTypeTVShow || media.Type == models.MediaTypeAnime) && seasonNumber > 0 {
		seasonFolderName := fmt.Sprintf("S%02d", seasonNumber)
		fullPath = filepath.Join(fullPath, seasonFolderName)
	}

	err := os.MkdirAll(fullPath, os.ModePerm)
	if err != nil {
		pp.logger.Error("Failed to create destination folder:", fullPath, "Error:", err)
		return ""
	}

	pp.logger.Info("Successfully created or verified destination folder:", fullPath)
	return fullPath
}

// identifyMediaFiles finds the relevant video and subtitle files within the downloaded content.
func (pp *PostProcessor) identifyMediaFiles(downloadPath string, torrentFiles []string) []string {
	videoExtensions := map[string]bool{".mkv": true, ".mp4": true, ".avi": true, ".mov": true}
	subtitleExtensions := map[string]bool{".srt": true, ".sub": true, ".ass": true}

	var files []string
	for _, file := range torrentFiles {
		ext := strings.ToLower(filepath.Ext(file))
		if videoExtensions[ext] || subtitleExtensions[ext] {
			fullPath := filepath.Join(downloadPath, file)
			files = append(files, fullPath)
		}
	}
	return files
}

// processFilesWithFallback attempts to process files using a sequential list of methods.
func (pp *PostProcessor) processFilesWithFallback(media *models.Media, files []string, destination string) error {
	var moveMethods []string
	switch media.Type {
	case models.MediaTypeMovie:
		moveMethods = pp.config.Movies.MoveMethod
	case models.MediaTypeTVShow:
		moveMethods = pp.config.TVShows.MoveMethod
	case models.MediaTypeAnime:
		moveMethods = pp.config.Anime.MoveMethod
	}

	if len(moveMethods) == 0 {
		return fmt.Errorf("no move_method defined for media type: %s", media.Type)
	}

	for _, file := range files {
		if !waitForFile(file, 30*time.Second) {
			return fmt.Errorf("source file did not appear in time: %s", file)
		}

		var lastErr error
		success := false
		for _, method := range moveMethods {
			newPath := filepath.Join(destination, filepath.Base(file))
			pp.logger.Info(fmt.Sprintf("Attempting to '%s' file: %s", method, file))

			var err error
			switch method {
			case "hardlink":
				err = os.Link(file, newPath)
			case "symlink":
				err = os.Symlink(file, newPath)
			case "move":
				err = os.Rename(file, newPath)
			case "copy":
				err = pp.copyFileAndRemoveOriginal(file, newPath)
			default:
				err = fmt.Errorf("unknown move_method: %s", method)
			}

			if err == nil {
				pp.logger.Info(fmt.Sprintf("Successfully processed file with method: '%s'", method))
				success = true
				break // Success, move to the next file
			}
			lastErr = err
			pp.logger.Warn(fmt.Sprintf("Method '%s' failed for file '%s': %v. Trying next method.", method, file, err))
		}

		if !success {
			pp.logger.Error(fmt.Sprintf("All processing methods failed for file '%s'. Last error: %v", file, lastErr))
			return fmt.Errorf("failed to process file '%s' after all fallbacks", file)
		}
	}
	return nil
}

// copyFileAndRemoveOriginal performs a manual copy and then deletes the source.
func (pp *PostProcessor) copyFileAndRemoveOriginal(src, dst string) error {
	sourceFile, err := os.Open(src)
	if err != nil {
		return err
	}
	defer sourceFile.Close()

	destinationFile, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer destinationFile.Close()

	_, err = io.Copy(destinationFile, sourceFile)
	if err != nil {
		return err
	}

	// The copy was successful, now remove the original file.
	return os.Remove(src)
}

// waitForFile waits for a file to exist for a certain duration.
func waitForFile(filePath string, timeout time.Duration) bool {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	ticker := time.NewTicker(500 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return false // Timeout reached
		case <-ticker.C:
			if _, err := os.Stat(filePath); err == nil {
				return true // File exists
			}
		}
	}
}

func (pp *PostProcessor) parseQualityFromTorrentName(torrentName string) string {
	lowerName := strings.ToLower(torrentName)
	// Check for resolutions first
	for _, res := range SUPPORTED_RESOLUTIONS {
		if strings.Contains(lowerName, res) {
			return res
		}
	}
	// Fallback to other quality indicators
	if strings.Contains(lowerName, "web-dl") || strings.Contains(lowerName, "webdl") {
		return "WEB-DL"
	}
	if strings.Contains(lowerName, "bluray") {
		return "BluRay"
	}
	if strings.Contains(lowerName, "webrip") {
		return "WEBRip"
	}
	if strings.Contains(lowerName, "bdrip") {
		return "BDRip"
	}
	if strings.Contains(lowerName, "brrip") {
		return "BRRip"
	}
	if strings.Contains(lowerName, "hdtv") {
		return "HDTV"
	}
	if strings.Contains(lowerName, "dvdrip") {
		return "DVDRip"
	}
	if strings.Contains(lowerName, "xvid") {
		return "Xvid" // Not really a quality, but it's quite common
	}
	return "Unknown"
}

// renameFiles renames the moved/linked files to a clean, standardized format.
func (pp *PostProcessor) renameFiles(media *models.Media, destination string, season, episode int, torrentName string, filesToRename []string) string {
	quality := pp.parseQualityFromTorrentName(torrentName)
	var videoFileName string

	for _, oldPath := range filesToRename {
		// We need to construct the path of the file *after* it has been moved/symlinked
		movedPath := filepath.Join(destination, filepath.Base(oldPath))
		ext := filepath.Ext(movedPath)

		var newName string
		var template string
		switch media.Type {
		case models.MediaTypeMovie:
			template = pp.config.FileRenaming.MovieTemplate
		case models.MediaTypeTVShow:
			template = pp.config.FileRenaming.SeriesTemplate
		case models.MediaTypeAnime:
			template = pp.config.FileRenaming.AnimeTemplate
		}

		if template == "" {
			// Fallback to old naming scheme if no template is provided
			if media.Type == models.MediaTypeMovie {
				newName = fmt.Sprintf("%s (%d) [%s]%s", media.Title, media.Year, quality, ext)
			} else {
				newName = fmt.Sprintf("%s - S%02dE%02d [%s]%s", media.Title, season, episode, quality, ext)
			}
		} else {
			r := strings.NewReplacer(
				"{title}", media.Title,
				"{year}", strconv.Itoa(media.Year),
				"{season}", fmt.Sprintf("%02d", season),
				"{episode}", fmt.Sprintf("%02d", episode),
				"{quality}", quality,
			)
			newName = r.Replace(template) + ext
		}

		newPath := filepath.Join(destination, newName)

		// Check if the moved file actually exists before trying to rename it
		if _, err := os.Stat(movedPath); err == nil {
			err := os.Rename(movedPath, newPath)
			if err != nil {
				pp.logger.Error("Failed to rename file:", err)
			} else if videoFileName == "" && (strings.HasSuffix(newPath, ".mkv") || strings.HasSuffix(newPath, ".mp4") || strings.HasSuffix(newPath, ".avi")) {
				videoFileName = newPath
			}
		} else {
			pp.logger.Error("Could not find file to rename at path:", movedPath)
		}
	}
	return videoFileName
}

func (pp *PostProcessor) downloadSubtitles(media *models.Media, destination, videoFileName string) {
	// Check if subtitle files already exist
	baseName := strings.TrimSuffix(filepath.Base(videoFileName), filepath.Ext(videoFileName))
	files, err := os.ReadDir(destination)
	if err == nil {
		for _, file := range files {
			if !file.IsDir() && strings.HasPrefix(file.Name(), baseName) && (strings.HasSuffix(file.Name(), ".srt") || strings.HasSuffix(file.Name(), ".sub") || strings.HasSuffix(file.Name(), ".ass")) {
				pp.logger.Info("Subtitle file already exists, skipping download:", file.Name())
				return
			}
		}
	}

	pp.logger.Info("Searching for subtitles for:", videoFileName)

	f, err := os.Open(videoFileName)
	if err != nil {
		pp.logger.Error("Could not open video file to find subtitles:", err)
		return
	}
	defer f.Close()

	lang := media.Language
	if lang == "" {
		lang = "en" // Default to English if no language is specified for the media
	}

	finder := subtitles.NewSubFinder(f, videoFileName, lang)

	// The library provides multiple sources, we can try them in order.
	// For this example, we'll just use TheSubDb.
	content, err := finder.TheSubDb()
	if err != nil {
		pp.logger.Error("Error searching for subtitles via TheSubDb:", err)
		return
	}

	if len(content) == 0 {
		pp.logger.Info("No subtitles found for:", media.Title)
		return
	}

	pp.logger.Info("Successfully downloaded subtitles for:", media.Title)

	// Construct the new subtitle file name.
	subtitleName := fmt.Sprintf("%s.%s.srt", baseName, lang)
	subtitlePath := filepath.Join(destination, subtitleName)

	err = os.WriteFile(subtitlePath, content, 0644)
	if err != nil {
		pp.logger.Error("Error saving subtitle file:", err)
	} else {
		pp.logger.Info("Subtitle saved to:", subtitlePath)
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
