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
	"reel/internal/config"
	"reel/internal/database/models"
	"reel/internal/utils"
)

// ValidateStage checks if files meet minimum requirements
type ValidateStage struct {
	config *config.Config
	logger *utils.Logger
}

func NewValidateStage(cfg *config.Config, logger *utils.Logger) *ValidateStage {
	return &ValidateStage{config: cfg, logger: logger}
}

func (s *ValidateStage) Name() string {
	return "validate"
}

func (s *ValidateStage) Execute(ctx *ProcessingContext) error {
	if !s.config.PostProcessing.Enabled || !s.config.PostProcessing.ValidateFiles {
		s.logger.Info("Validation disabled, skipping")
		return nil
	}

	videoExtensions := map[string]bool{".mkv": true, ".mp4": true, ".avi": true, ".mov": true}
	minSize := int64(s.config.PostProcessing.MinFileSizeMB) * 1024 * 1024

	validFiles := make([]string, 0, len(ctx.OriginalFiles))
	for _, file := range ctx.OriginalFiles {
		ext := strings.ToLower(filepath.Ext(file))
		if !videoExtensions[ext] {
			continue // Skip non-video files
		}

		stat, err := os.Stat(file)
		if err != nil {
			s.logger.Warn(fmt.Sprintf("Cannot stat file %s: %v", file, err))
			continue
		}

		if stat.Size() < minSize {
			s.logger.Warn(fmt.Sprintf("File %s too small (%d MB < %d MB), skipping",
				file, stat.Size()/1024/1024, s.config.PostProcessing.MinFileSizeMB))
			continue
		}

		validFiles = append(validFiles, file)
	}

	if len(validFiles) == 0 {
		return fmt.Errorf("no valid video files found")
	}

	ctx.OriginalFiles = validFiles
	s.logger.Info(fmt.Sprintf("Validated %d files", len(validFiles)))
	return nil
}

func (s *ValidateStage) Rollback(ctx *ProcessingContext) error {
	// Nothing to rollback for validation
	return nil
}

// CreateFoldersStage creates destination directories
type CreateFoldersStage struct {
	config *config.Config
	logger *utils.Logger
}

func NewCreateFoldersStage(cfg *config.Config, logger *utils.Logger) *CreateFoldersStage {
	return &CreateFoldersStage{config: cfg, logger: logger}
}

func (s *CreateFoldersStage) Name() string {
	return "create_folders"
}

func (s *CreateFoldersStage) Execute(ctx *ProcessingContext) error {
	var baseDestPath string

	switch ctx.Media.Type {
	case models.MediaTypeMovie:
		baseDestPath = s.config.Movies.DestinationFolder
	case models.MediaTypeTVShow:
		baseDestPath = s.config.TVShows.DestinationFolder
	case models.MediaTypeAnime:
		baseDestPath = s.config.Anime.DestinationFolder
	default:
		return fmt.Errorf("unknown media type: %s", ctx.Media.Type)
	}

	safeTitle := utils.SanitizeFilename(ctx.Media.Title)
	mediaFolderName := fmt.Sprintf("%s (%d)", safeTitle, ctx.Media.Year)
	fullPath := filepath.Join(baseDestPath, mediaFolderName)

	if (ctx.Media.Type == models.MediaTypeTVShow || ctx.Media.Type == models.MediaTypeAnime) && ctx.SeasonNumber > 0 {
		seasonFolderName := fmt.Sprintf("S%02d", ctx.SeasonNumber)
		fullPath = filepath.Join(fullPath, seasonFolderName)
	}

	if err := os.MkdirAll(fullPath, os.ModePerm); err != nil {
		return fmt.Errorf("failed to create destination folder: %w", err)
	}

	ctx.DestinationDir = fullPath
	s.logger.Info(fmt.Sprintf("Created destination folder: %s", fullPath))
	return nil
}

func (s *CreateFoldersStage) Rollback(ctx *ProcessingContext) error {
	// We don't remove folders on rollback as they might be used by other media
	return nil
}

// MoveFilesStage handles moving/linking files with retry logic
type MoveFilesStage struct {
	config *config.Config
	logger *utils.Logger
}

func NewMoveFilesStage(cfg *config.Config, logger *utils.Logger) *MoveFilesStage {
	return &MoveFilesStage{config: cfg, logger: logger}
}

func (s *MoveFilesStage) Name() string {
	return "move_files"
}

func (s *MoveFilesStage) Execute(ctx *ProcessingContext) error {
	var moveMethods []string
	switch ctx.Media.Type {
	case models.MediaTypeMovie:
		moveMethods = s.config.Movies.MoveMethod
	case models.MediaTypeTVShow:
		moveMethods = s.config.TVShows.MoveMethod
	case models.MediaTypeAnime:
		moveMethods = s.config.Anime.MoveMethod
	}

	if len(moveMethods) == 0 {
		return fmt.Errorf("no move methods configured")
	}

	retries := 1
	if s.config.PostProcessing.Enabled && s.config.PostProcessing.RetryAttempts > 0 {
		retries = s.config.PostProcessing.RetryAttempts
	}

	ctx.ProcessedFiles = make([]string, 0, len(ctx.OriginalFiles))

	for _, file := range ctx.OriginalFiles {
		if !s.waitForFile(file) {
			return fmt.Errorf("source file did not appear in time: %s", file)
		}

		var lastErr error
		success := false

		for attempt := 0; attempt < retries; attempt++ {
			if attempt > 0 {
				delay := 5 * time.Second
				if s.config.PostProcessing.Enabled && s.config.PostProcessing.RetryDelay > 0 {
					delay = time.Duration(s.config.PostProcessing.RetryDelay) * time.Second
				}
				time.Sleep(delay)
				s.logger.Info(fmt.Sprintf("Retry attempt %d/%d for file: %s", attempt+1, retries, file))
			}

			for _, method := range moveMethods {
				newPath := filepath.Join(ctx.DestinationDir, filepath.Base(file))
				s.logger.Info(fmt.Sprintf("Attempting to '%s' file: %s", method, file))

				var err error
				switch method {
				case "hardlink":
					err = os.Link(file, newPath)
				case "symlink":
					err = os.Symlink(file, newPath)
				case "move":
					err = os.Rename(file, newPath)
				case "copy":
					err = s.copyFileAndRemoveOriginal(file, newPath)
				default:
					err = fmt.Errorf("unknown move_method: %s", method)
				}

				if err == nil {
					s.logger.Info(fmt.Sprintf("Successfully processed file with method: '%s'", method))
					ctx.ProcessedFiles = append(ctx.ProcessedFiles, newPath)
					success = true
					break
				}
				lastErr = err
				s.logger.Warn(fmt.Sprintf("Method '%s' failed: %v", method, err))
			}

			if success {
				break
			}
		}

		if !success {
			return fmt.Errorf("failed to process file '%s' after %d attempts: %w", file, retries, lastErr)
		}
	}

	return nil
}

func (s *MoveFilesStage) Rollback(ctx *ProcessingContext) error {
	// Remove processed files on rollback
	for _, file := range ctx.ProcessedFiles {
		if err := os.Remove(file); err != nil {
			s.logger.Warn(fmt.Sprintf("Failed to remove file during rollback: %s: %v", file, err))
		} else {
			s.logger.Info(fmt.Sprintf("Rolled back file: %s", file))
		}
	}
	return nil
}

func (s *MoveFilesStage) waitForFile(filePath string) bool {
	timeout := 30 * time.Second
	if s.config.PostProcessing.Enabled && s.config.PostProcessing.WaitForFileTimeout > 0 {
		timeout = time.Duration(s.config.PostProcessing.WaitForFileTimeout) * time.Second
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
				if s.config.PostProcessing.Enabled && s.config.PostProcessing.ValidateFiles {
					minSize := int64(s.config.PostProcessing.MinFileSizeMB) * 1024 * 1024
					if stat.Size() < minSize {
						continue
					}
				}
				return true
			}
		}
	}
}

func (s *MoveFilesStage) copyFileAndRemoveOriginal(src, dst string) error {
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

	if _, err = io.Copy(destinationFile, sourceFile); err != nil {
		return err
	}

	return os.Remove(src)
}

// RenameStage renames files according to configured templates
type RenameStage struct {
	config *config.Config
	logger *utils.Logger
}

func NewRenameStage(cfg *config.Config, logger *utils.Logger) *RenameStage {
	return &RenameStage{config: cfg, logger: logger}
}

func (s *RenameStage) Name() string {
	return "rename"
}

func (s *RenameStage) Execute(ctx *ProcessingContext) error {
	quality := s.parseQualityFromTorrentName(ctx.TorrentStatus.Name)

	// Store original names for rollback
	if ctx.Metadata == nil {
		ctx.Metadata = make(map[string]string)
	}

	renamedFiles := make([]string, 0, len(ctx.ProcessedFiles))

	for _, oldPath := range ctx.ProcessedFiles {
		ext := filepath.Ext(oldPath)

		// Skip subtitle files, they're handled by subtitle stage
		if ext == ".srt" || ext == ".sub" || ext == ".ass" {
			continue
		}

		var newName string
		var template string
		switch ctx.Media.Type {
		case models.MediaTypeMovie:
			template = s.config.FileRenaming.MovieTemplate
		case models.MediaTypeTVShow:
			template = s.config.FileRenaming.SeriesTemplate
		case models.MediaTypeAnime:
			template = s.config.FileRenaming.AnimeTemplate
		}

		if template == "" {
			if ctx.Media.Type == models.MediaTypeMovie {
				newName = fmt.Sprintf("%s (%d) [%s]%s", ctx.Media.Title, ctx.Media.Year, quality, ext)
			} else {
				newName = fmt.Sprintf("%s - S%02dE%02d [%s]%s",
					ctx.Media.Title, ctx.SeasonNumber, ctx.EpisodeNumber, quality, ext)
			}
		} else {
			r := strings.NewReplacer(
				"{title}", ctx.Media.Title,
				"{year}", strconv.Itoa(ctx.Media.Year),
				"{season}", fmt.Sprintf("%02d", ctx.SeasonNumber),
				"{episode}", fmt.Sprintf("%02d", ctx.EpisodeNumber),
				"{quality}", quality,
			)
			newName = r.Replace(template) + ext
		}

		newPath := filepath.Join(filepath.Dir(oldPath), newName)

		if err := os.Rename(oldPath, newPath); err != nil {
			return fmt.Errorf("failed to rename %s to %s: %w", oldPath, newPath, err)
		}

		s.logger.Info(fmt.Sprintf("Renamed: %s -> %s", filepath.Base(oldPath), newName))
		renamedFiles = append(renamedFiles, newPath)

		// Store for rollback
		ctx.Metadata[newPath] = oldPath
	}

	// Update ProcessedFiles with new names
	ctx.ProcessedFiles = renamedFiles
	return nil
}

func (s *RenameStage) Rollback(ctx *ProcessingContext) error {
	// Restore original names
	for newPath, oldPath := range ctx.Metadata {
		if err := os.Rename(newPath, oldPath); err != nil {
			s.logger.Warn(fmt.Sprintf("Failed to restore filename during rollback: %v", err))
		}
	}
	return nil
}

func (s *RenameStage) parseQualityFromTorrentName(torrentName string) string {
	// Use improved quality parser
	return utils.ParseQuality(torrentName)
}

// SubtitlesStage downloads subtitles for processed files
type SubtitlesStage struct {
	config         *config.Config
	logger         *utils.Logger
	subtitleClient *subtitles.Client
}

func NewSubtitlesStage(cfg *config.Config, logger *utils.Logger, subClient *subtitles.Client) *SubtitlesStage {
	return &SubtitlesStage{
		config:         cfg,
		logger:         logger,
		subtitleClient: subClient,
	}
}

func (s *SubtitlesStage) Name() string {
	return "subtitles"
}

func (s *SubtitlesStage) Execute(ctx *ProcessingContext) error {
	if !s.config.Subtitles.Enabled || s.subtitleClient == nil {
		s.logger.Info("Subtitles disabled, skipping")
		return nil
	}

	for _, videoPath := range ctx.ProcessedFiles {
		ext := filepath.Ext(videoPath)
		if ext != ".mkv" && ext != ".mp4" && ext != ".avi" && ext != ".mov" {
			continue // Only process video files
		}

		s.logger.Info(fmt.Sprintf("Searching subtitles for: %s", videoPath))

		subs, err := s.subtitleClient.Search(videoPath, ctx.Media, ctx.SeasonNumber, ctx.EpisodeNumber)
		if err != nil {
			s.logger.Error(fmt.Sprintf("Subtitle search failed: %v", err))
			ctx.Errors = append(ctx.Errors, err)
			continue
		}

		if len(subs) == 0 {
			s.logger.Info("No subtitles found")
			continue
		}

		// Download subtitles for all configured languages
		downloaded := 0
		for _, lang := range s.config.Subtitles.Languages {
			var foundSub *subtitles.Subtitle
			for i := range subs {
				if subs[i].Language == lang {
					foundSub = &subs[i]
					break
				}
			}

			if foundSub == nil {
				s.logger.Info(fmt.Sprintf("No subtitle found for language: %s", lang))
				continue
			}

			base := strings.TrimSuffix(videoPath, ext)
			subPath := fmt.Sprintf("%s.%s.srt", base, lang)

			if err := s.subtitleClient.Download(*foundSub, subPath); err != nil {
				s.logger.Error(fmt.Sprintf("Failed to download %s subtitle: %v", lang, err))
				ctx.Errors = append(ctx.Errors, err)
			} else {
				s.logger.Info(fmt.Sprintf("Downloaded %s subtitle to: %s", lang, subPath))
				downloaded++
			}
		}

		if downloaded > 0 {
			s.logger.Info(fmt.Sprintf("Downloaded %d subtitle(s) for: %s", downloaded, filepath.Base(videoPath)))
		}
	}

	return nil
}

func (s *SubtitlesStage) Rollback(ctx *ProcessingContext) error {
	// We don't remove subtitles on rollback as they're not critical
	return nil
}

// NotifyStage sends notifications about completed processing
type NotifyStage struct {
	logger    *utils.Logger
	notifiers []notifications.Notifier
}

func NewNotifyStage(logger *utils.Logger, notifiers []notifications.Notifier) *NotifyStage {
	return &NotifyStage{
		logger:    logger,
		notifiers: notifiers,
	}
}

func (s *NotifyStage) Name() string {
	return "notify"
}

func (s *NotifyStage) Execute(ctx *ProcessingContext) error {
	if len(s.notifiers) == 0 {
		s.logger.Info("No notifiers configured, skipping")
		return nil
	}

	s.logger.Info(fmt.Sprintf("Sending notifications to %d notifier(s)", len(s.notifiers)))
	for i, n := range s.notifiers {
		go func(notifier notifications.Notifier, index int) {
			notifier.NotifyPostProcessComplete(ctx.Media, ctx.TorrentStatus.Name)
			s.logger.Info(fmt.Sprintf("Notification sent via notifier %d", index))
		}(n, i)
	}

	return nil
}

func (s *NotifyStage) Rollback(ctx *ProcessingContext) error {
	// Cannot rollback notifications
	return nil
}
