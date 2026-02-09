package services

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
	"reel/internal/clients/subtitles"
	"reel/internal/clients/torrent"
	"reel/internal/config"
	"reel/internal/database/models"
	"reel/internal/utils"
)

// PostProcessor handles the tasks after a download is complete.
type PostProcessor struct {
	config         *config.Config
	logger         *utils.Logger
	mediaRepo      *models.MediaRepository
	notifiers      []notifications.Notifier
	subtitleClient *subtitles.Client
	pipeline       *Pipeline // New pipeline-based processor
}

// NewPostProcessor creates a new instance of the PostProcessor.
func NewPostProcessor(cfg *config.Config, logger *utils.Logger, mediaRepo *models.MediaRepository, notifiers []notifications.Notifier, subClient *subtitles.Client) *PostProcessor {
	pp := &PostProcessor{
		config:         cfg,
		logger:         logger,
		mediaRepo:      mediaRepo,
		notifiers:      notifiers,
		subtitleClient: subClient,
	}

	// Initialize pipeline if enabled
	if cfg.PostProcessing.Pipeline.Enabled {
		var err error
		pp.pipeline, err = ConfigurablePipelineFactory(cfg, logger, mediaRepo, notifiers, subClient)
		if err != nil {
			logger.Error("Failed to create configurable pipeline, falling back to default:", err)
			pp.pipeline = DefaultPipelineFactory(cfg, logger, mediaRepo, notifiers, subClient)
		}
		pp.pipeline.rollback = cfg.PostProcessing.Pipeline.Rollback
		pp.logger.Info("PostProcessor: Pipeline mode enabled")
	} else {
		pp.logger.Info("PostProcessor: Legacy mode enabled")
	}

	return pp
}

// ProcessDownload is the main entry point for post-processing a completed download.
func (pp *PostProcessor) ProcessDownload(media models.Media, torrentStatus torrent.TorrentStatus, seasonNumber int, episodeNumber int, downloadPath string) error {
	pp.logger.Info("Starting post-processing for:", media.Title)

	// Use pipeline if enabled
	if pp.pipeline != nil {
		return pp.processWithPipeline(media, torrentStatus, seasonNumber, episodeNumber, downloadPath)
	}

	// Legacy processing path
	return pp.processLegacy(media, torrentStatus, seasonNumber, episodeNumber, downloadPath)
}

// processWithPipeline uses the new pipeline architecture
func (pp *PostProcessor) processWithPipeline(media models.Media, torrentStatus torrent.TorrentStatus, seasonNumber int, episodeNumber int, downloadPath string) error {
	mediaFiles := pp.identifyMediaFiles(downloadPath, torrentStatus.Files)
	if len(mediaFiles) == 0 {
		return fmt.Errorf("no media files identified for: %s", media.Title)
	}

	ctx := &ProcessingContext{
		Media:         &media,
		TorrentStatus: torrentStatus,
		SeasonNumber:  seasonNumber,
		EpisodeNumber: episodeNumber,
		DownloadPath:  downloadPath,
		OriginalFiles: mediaFiles,
		Metadata:      make(map[string]string),
		Errors:        make([]error, 0),
	}

	if err := pp.pipeline.Execute(ctx); err != nil {
		pp.logger.Error("Pipeline execution failed:", err)
		return err
	}

	// Log any non-fatal errors that occurred during processing
	if len(ctx.Errors) > 0 {
		pp.logger.Warn(fmt.Sprintf("Pipeline completed with %d non-fatal errors", len(ctx.Errors)))
		for _, err := range ctx.Errors {
			pp.logger.Warn(fmt.Sprintf("  - %v", err))
		}
	}

	pp.logger.Info("Finished post-processing for:", media.Title)
	return nil
}

// processLegacy uses the original monolithic approach
func (pp *PostProcessor) processLegacy(media models.Media, torrentStatus torrent.TorrentStatus, seasonNumber int, episodeNumber int, downloadPath string) error {
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

	pp.renameFiles(&media, destinationPath, seasonNumber, episodeNumber, torrentStatus.Name, mediaFiles)

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
		if !pp.waitForFile(file) {
			return fmt.Errorf("source file did not appear in time: %s", file)
		}

		var lastErr error
		success := false
		retries := 1
		if pp.config.PostProcessing.Enabled && pp.config.PostProcessing.RetryAttempts > 0 {
			retries = pp.config.PostProcessing.RetryAttempts
		}

		for attempt := 0; attempt < retries; attempt++ {
			if attempt > 0 {
				delay := 5 * time.Second
				if pp.config.PostProcessing.Enabled && pp.config.PostProcessing.RetryDelay > 0 {
					delay = time.Duration(pp.config.PostProcessing.RetryDelay) * time.Second
				}
				time.Sleep(delay)
				pp.logger.Info(fmt.Sprintf("Retry attempt %d/%d for file: %s", attempt+1, retries, file))
			}

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
				break // Success, move to the next method
			}
			lastErr = err
			pp.logger.Warn(fmt.Sprintf("Method '%s' failed for file '%s': %v. Trying next method.", method, file, err))
		}

		if success {
			break // Success, move to the next file
		}
		}

		if !success {
			pp.logger.Error(fmt.Sprintf("All processing methods failed for file '%s' after %d attempts. Last error: %v", file, retries, lastErr))
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
func (pp *PostProcessor) waitForFile(filePath string) bool {
	timeout := 30 * time.Second
	if pp.config.PostProcessing.Enabled && pp.config.PostProcessing.WaitForFileTimeout > 0 {
		timeout = time.Duration(pp.config.PostProcessing.WaitForFileTimeout) * time.Second
	}

	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	ticker := time.NewTicker(500 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return false
		case <-ticker.C:
			if stat, err := os.Stat(filePath); err == nil {
				// If validation is enabled, check file size
				if pp.config.PostProcessing.Enabled && pp.config.PostProcessing.ValidateFiles {
					minSize := int64(pp.config.PostProcessing.MinFileSizeMB) * 1024 * 1024
					if stat.Size() < minSize {
						continue // File too small, keep waiting
					}
				}
				return true
			}
		}
	}
}

func (pp *PostProcessor) parseQualityFromTorrentName(torrentName string) string {
	// Use improved quality parser
	return utils.ParseQuality(torrentName)
}

// renameFiles renames the moved/linked files to a clean, standardized format.
func (pp *PostProcessor) renameFiles(media *models.Media, destination string, season, episode int, torrentName string, filesToRename []string) {
	quality := pp.parseQualityFromTorrentName(torrentName)

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
			} else {
				// Successfully renamed. Now check for subtitles.
				if pp.subtitleClient != nil && pp.config.Subtitles.Enabled {
					pp.downloadSubtitles(media, newPath, season, episode)
				}
			}
		} else {
			pp.logger.Error("Could not find file to rename at path:", movedPath)
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

func (pp *PostProcessor) downloadSubtitles(media *models.Media, videoPath string, season, episode int) {
	pp.logger.Info(fmt.Sprintf("Attempting to download subtitles for: %s", videoPath))

	subs, err := pp.subtitleClient.Search(videoPath, media, season, episode)
	if err != nil {
		pp.logger.Error("Failed to search subtitles:", err)
		return
	}

	if len(subs) == 0 {
		pp.logger.Info("No subtitles found.")
		return
	}

	// Download the best one (first one usually has highest score)
	bestSub := subs[0]
	pp.logger.Info("Downloading best subtitle:", bestSub.FileName, "Lang:", bestSub.Language)

	// Construct subtitle path: same as video but with .lang.srt
	// videoPath: /path/to/Movie (2020).mkv
	// subPath:   /path/to/Movie (2020).en.srt

	ext := filepath.Ext(videoPath)
	base := strings.TrimSuffix(videoPath, ext)
	subPath := fmt.Sprintf("%s.%s.srt", base, bestSub.Language)

	if err := pp.subtitleClient.Download(bestSub, subPath); err != nil {
		pp.logger.Error("Failed to download subtitle:", err)
	} else {
		pp.logger.Info("Subtitle downloaded to:", subPath)
	}
}
