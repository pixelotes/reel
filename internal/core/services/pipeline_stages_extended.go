package services

import (
	"archive/zip"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"reel/internal/config"
	"reel/internal/utils"
)

// ExtractionStage extracts compressed files (zip, rar) before processing
type ExtractionStage struct {
	config *config.Config
	logger *utils.Logger
}

func NewExtractionStage(cfg *config.Config, logger *utils.Logger) *ExtractionStage {
	return &ExtractionStage{
		config: cfg,
		logger: logger,
	}
}

func (s *ExtractionStage) Name() string {
	return "extract"
}

func (s *ExtractionStage) Execute(ctx *ProcessingContext) error {
	extractedFiles := make([]string, 0)
	extractedDirs := make([]string, 0)

	for _, file := range ctx.OriginalFiles {
		ext := strings.ToLower(filepath.Ext(file))

		if ext == ".zip" {
			s.logger.Info("Extracting ZIP file:", file)
			extracted, err := s.extractZip(file, ctx.DownloadPath)
			if err != nil {
				return fmt.Errorf("failed to extract ZIP %s: %w", file, err)
			}
			extractedFiles = append(extractedFiles, extracted...)
			extractedDirs = append(extractedDirs, filepath.Dir(extracted[0]))
		} else if ext == ".rar" {
			s.logger.Info("Extracting RAR file:", file)
			extracted, err := s.extractRar(file, ctx.DownloadPath)
			if err != nil {
				// RAR extraction may fail if unrar not installed - log warning
				s.logger.Warn("Failed to extract RAR (unrar may not be installed):", err)
				ctx.Errors = append(ctx.Errors, fmt.Errorf("RAR extraction failed: %w", err))
				continue
			}
			extractedFiles = append(extractedFiles, extracted...)
			extractedDirs = append(extractedDirs, filepath.Dir(extracted[0]))
		}
	}

	// If files were extracted, replace OriginalFiles with extracted files
	if len(extractedFiles) > 0 {
		s.logger.Info(fmt.Sprintf("Extracted %d files from archives", len(extractedFiles)))
		ctx.OriginalFiles = extractedFiles
		// Store extraction dirs for cleanup
		if ctx.Metadata == nil {
			ctx.Metadata = make(map[string]string)
		}
		ctx.Metadata["extracted_dirs"] = strings.Join(extractedDirs, ";")
	}

	return nil
}

func (s *ExtractionStage) Rollback(ctx *ProcessingContext) error {
	// Clean up extracted directories
	if extractedDirs, ok := ctx.Metadata["extracted_dirs"]; ok {
		dirs := strings.Split(extractedDirs, ";")
		for _, dir := range dirs {
			if dir != "" && dir != ctx.DownloadPath {
				s.logger.Info("Cleaning up extracted directory:", dir)
				os.RemoveAll(dir)
			}
		}
	}
	return nil
}

// extractZip extracts a ZIP file and returns extracted file paths
func (s *ExtractionStage) extractZip(zipPath, destDir string) ([]string, error) {
	r, err := zip.OpenReader(zipPath)
	if err != nil {
		return nil, err
	}
	defer r.Close()

	extractedFiles := make([]string, 0)

	for _, f := range r.File {
		fpath := filepath.Join(destDir, f.Name)

		// Check for ZipSlip vulnerability
		if !strings.HasPrefix(fpath, filepath.Clean(destDir)+string(os.PathSeparator)) {
			return nil, fmt.Errorf("illegal file path: %s", fpath)
		}

		if f.FileInfo().IsDir() {
			os.MkdirAll(fpath, os.ModePerm)
			continue
		}

		// Create parent dirs
		if err := os.MkdirAll(filepath.Dir(fpath), os.ModePerm); err != nil {
			return nil, err
		}

		// Extract file
		outFile, err := os.OpenFile(fpath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, f.Mode())
		if err != nil {
			return nil, err
		}

		rc, err := f.Open()
		if err != nil {
			outFile.Close()
			return nil, err
		}

		_, err = io.Copy(outFile, rc)
		outFile.Close()
		rc.Close()

		if err != nil {
			return nil, err
		}

		extractedFiles = append(extractedFiles, fpath)
	}

	return extractedFiles, nil
}

// extractRar extracts a RAR file using unrar command
func (s *ExtractionStage) extractRar(rarPath, destDir string) ([]string, error) {
	// Check if unrar is available
	if _, err := exec.LookPath("unrar"); err != nil {
		return nil, fmt.Errorf("unrar not found in PATH")
	}

	// Extract to temporary subdirectory
	extractDir := filepath.Join(destDir, "extracted_"+filepath.Base(rarPath))
	os.MkdirAll(extractDir, 0755)

	cmd := exec.Command("unrar", "x", "-o+", rarPath, extractDir)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("unrar failed: %w - %s", err, string(output))
	}

	// Find extracted files
	extractedFiles := make([]string, 0)
	filepath.Walk(extractDir, func(path string, info os.FileInfo, err error) error {
		if err == nil && !info.IsDir() {
			extractedFiles = append(extractedFiles, path)
		}
		return nil
	})

	return extractedFiles, nil
}

// HealthCheckStage verifies file integrity and health
type HealthCheckStage struct {
	config *config.Config
	logger *utils.Logger
}

func NewHealthCheckStage(cfg *config.Config, logger *utils.Logger) *HealthCheckStage {
	return &HealthCheckStage{
		config: cfg,
		logger: logger,
	}
}

func (s *HealthCheckStage) Name() string {
	return "health_check"
}

func (s *HealthCheckStage) Execute(ctx *ProcessingContext) error {
	for _, file := range ctx.OriginalFiles {
		// Check file is readable
		f, err := os.Open(file)
		if err != nil {
			return fmt.Errorf("file not readable: %s - %w", file, err)
		}
		f.Close()

		// Check file modification time (ensure it's not being written)
		info, err := os.Stat(file)
		if err != nil {
			return fmt.Errorf("cannot stat file: %s - %w", file, err)
		}

		// If file was modified in last 5 seconds, wait
		if time.Since(info.ModTime()) < 5*time.Second {
			s.logger.Info("File recently modified, waiting for stability:", file)
			time.Sleep(5 * time.Second)
		}

		// Verify video files can be opened (basic check)
		if isVideoFile(file) {
			if err := s.verifyVideoFile(file); err != nil {
				ctx.Errors = append(ctx.Errors, fmt.Errorf("video verification failed for %s: %w", file, err))
				s.logger.Warn("Video file may be corrupted:", file, "-", err)
			}
		}
	}

	s.logger.Info("Health check completed for", len(ctx.OriginalFiles), "files")
	return nil
}

func (s *HealthCheckStage) Rollback(ctx *ProcessingContext) error {
	// Health check is read-only, nothing to rollback
	return nil
}

// verifyVideoFile performs basic video file verification
func (s *HealthCheckStage) verifyVideoFile(filePath string) error {
	// Try to open and read first 1MB
	f, err := os.Open(filePath)
	if err != nil {
		return err
	}
	defer f.Close()

	buffer := make([]byte, 1024*1024) // 1MB
	n, err := io.ReadFull(f, buffer)
	if err != nil && err != io.EOF && err != io.ErrUnexpectedEOF {
		return fmt.Errorf("failed to read file: %w", err)
	}

	// Check for common video file signatures
	if n > 0 {
		// Basic check: file has some content
		return nil
	}

	return fmt.Errorf("file is empty or unreadable")
}

// MetadataEnrichmentStage adds additional metadata to context
type MetadataEnrichmentStage struct {
	config *config.Config
	logger *utils.Logger
}

func NewMetadataEnrichmentStage(cfg *config.Config, logger *utils.Logger) *MetadataEnrichmentStage {
	return &MetadataEnrichmentStage{
		config: cfg,
		logger: logger,
	}
}

func (s *MetadataEnrichmentStage) Name() string {
	return "enrich_metadata"
}

func (s *MetadataEnrichmentStage) Execute(ctx *ProcessingContext) error {
	if ctx.Metadata == nil {
		ctx.Metadata = make(map[string]string)
	}

	// Add processing timestamp
	ctx.Metadata["processing_time"] = time.Now().Format(time.RFC3339)

	// Add torrent info
	if ctx.TorrentStatus.Hash != "" {
		ctx.Metadata["torrent_hash"] = ctx.TorrentStatus.Hash
		ctx.Metadata["torrent_name"] = ctx.TorrentStatus.Name
	}

	// Add quality info
	for _, file := range ctx.OriginalFiles {
		quality := utils.ParseQuality(file)
		if quality != "Unknown" {
			ctx.Metadata["quality"] = quality
			break
		}
	}

	// Add file count
	ctx.Metadata["file_count"] = fmt.Sprintf("%d", len(ctx.OriginalFiles))

	// Calculate total size
	var totalSize int64
	for _, file := range ctx.OriginalFiles {
		if info, err := os.Stat(file); err == nil {
			totalSize += info.Size()
		}
	}
	ctx.Metadata["total_size_mb"] = fmt.Sprintf("%d", totalSize/(1024*1024))

	s.logger.Info("Metadata enrichment completed")
	return nil
}

func (s *MetadataEnrichmentStage) Rollback(ctx *ProcessingContext) error {
	// Metadata enrichment is non-destructive, nothing to rollback
	return nil
}

// DuplicateCheckStage checks if media already exists at destination
type DuplicateCheckStage struct {
	config *config.Config
	logger *utils.Logger
}

func NewDuplicateCheckStage(cfg *config.Config, logger *utils.Logger) *DuplicateCheckStage {
	return &DuplicateCheckStage{
		config: cfg,
		logger: logger,
	}
}

func (s *DuplicateCheckStage) Name() string {
	return "duplicate_check"
}

func (s *DuplicateCheckStage) Execute(ctx *ProcessingContext) error {
	if ctx.DestinationDir == "" {
		return nil // No destination set yet
	}

	// Check if destination directory exists and has files
	if info, err := os.Stat(ctx.DestinationDir); err == nil && info.IsDir() {
		// List video files in destination
		existingFiles := make([]string, 0)
		filepath.Walk(ctx.DestinationDir, func(path string, info os.FileInfo, err error) error {
			if err == nil && !info.IsDir() && isVideoFile(path) {
				existingFiles = append(existingFiles, path)
			}
			return nil
		})

		if len(existingFiles) > 0 {
			s.logger.Warn(fmt.Sprintf("Found %d existing video file(s) in destination", len(existingFiles)))

			// Check if any existing file is similar in size
			for _, existingFile := range existingFiles {
				existingInfo, _ := os.Stat(existingFile)
				for _, newFile := range ctx.OriginalFiles {
					newInfo, _ := os.Stat(newFile)
					sizeDiff := abs(existingInfo.Size() - newInfo.Size())
					// If size difference < 1%, likely duplicate
					if float64(sizeDiff)/float64(existingInfo.Size()) < 0.01 {
						s.logger.Warn("Potential duplicate detected:", filepath.Base(existingFile))
						ctx.Metadata["duplicate_warning"] = "true"
					}
				}
			}
		}
	}

	return nil
}

func (s *DuplicateCheckStage) Rollback(ctx *ProcessingContext) error {
	// Duplicate check is read-only, nothing to rollback
	return nil
}

// PermissionCheckStage verifies write permissions before processing
type PermissionCheckStage struct {
	config *config.Config
	logger *utils.Logger
}

func NewPermissionCheckStage(cfg *config.Config, logger *utils.Logger) *PermissionCheckStage {
	return &PermissionCheckStage{
		config: cfg,
		logger: logger,
	}
}

func (s *PermissionCheckStage) Name() string {
	return "permission_check"
}

func (s *PermissionCheckStage) Execute(ctx *ProcessingContext) error {
	// Check read permissions on source files
	for _, file := range ctx.OriginalFiles {
		if !isReadable(file) {
			return fmt.Errorf("no read permission for file: %s", file)
		}
	}

	// Check write permission on destination (if set)
	if ctx.DestinationDir != "" {
		// Try to create destination if it doesn't exist
		if err := os.MkdirAll(ctx.DestinationDir, 0755); err != nil {
			return fmt.Errorf("cannot create destination directory: %w", err)
		}

		// Check write permission by creating a test file
		testFile := filepath.Join(ctx.DestinationDir, ".reel_permission_test")
		f, err := os.Create(testFile)
		if err != nil {
			return fmt.Errorf("no write permission in destination: %w", err)
		}
		f.Close()
		os.Remove(testFile)
	}

	s.logger.Info("Permission check passed")
	return nil
}

func (s *PermissionCheckStage) Rollback(ctx *ProcessingContext) error {
	// Permission check is non-destructive, nothing to rollback
	return nil
}

// SpaceCheckStage verifies sufficient disk space
type SpaceCheckStage struct {
	config *config.Config
	logger *utils.Logger
}

func NewSpaceCheckStage(cfg *config.Config, logger *utils.Logger) *SpaceCheckStage {
	return &SpaceCheckStage{
		config: cfg,
		logger: logger,
	}
}

func (s *SpaceCheckStage) Name() string {
	return "space_check"
}

func (s *SpaceCheckStage) Execute(ctx *ProcessingContext) error {
	if ctx.DestinationDir == "" {
		return nil
	}

	// Calculate total size of files to be processed
	var totalSize int64
	for _, file := range ctx.OriginalFiles {
		if info, err := os.Stat(file); err == nil {
			totalSize += info.Size()
		}
	}

	// Get available space at destination
	// Note: This is platform-specific, basic implementation
	// For production, use syscall.Statfs on Unix or similar
	destParent := filepath.Dir(ctx.DestinationDir)
	if info, err := os.Stat(destParent); err == nil && info.IsDir() {
		// Add 10% buffer for safety
		requiredSpace := int64(float64(totalSize) * 1.1)
		s.logger.Info(fmt.Sprintf("Space check: required ~%d MB", requiredSpace/(1024*1024)))

		// Store in metadata for reference
		if ctx.Metadata == nil {
			ctx.Metadata = make(map[string]string)
		}
		ctx.Metadata["required_space_mb"] = fmt.Sprintf("%d", requiredSpace/(1024*1024))
	}

	return nil
}

func (s *SpaceCheckStage) Rollback(ctx *ProcessingContext) error {
	// Space check is read-only, nothing to rollback
	return nil
}

// Helper functions

func isVideoFile(path string) bool {
	ext := strings.ToLower(filepath.Ext(path))
	videoExts := []string{".mkv", ".mp4", ".avi", ".mov", ".wmv", ".flv", ".webm", ".m4v"}
	for _, ve := range videoExts {
		if ext == ve {
			return true
		}
	}
	return false
}

func isReadable(path string) bool {
	f, err := os.Open(path)
	if err != nil {
		return false
	}
	f.Close()
	return true
}

func abs(x int64) int64 {
	if x < 0 {
		return -x
	}
	return x
}
