package services

import (
	"io"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"reel/internal/clients/torrent"
	"reel/internal/config"
	"reel/internal/database/models"
	"reel/internal/utils"
)

// testConfig returns a minimal config suitable for testing.
// srcDir and dstDir should be t.TempDir() paths.
func testConfig(srcDir, dstDir string) *config.Config {
	cfg := &config.Config{}
	cfg.PostProcessing.Enabled = true
	cfg.PostProcessing.ValidateFiles = true
	cfg.PostProcessing.MinFileSizeMB = 1
	cfg.PostProcessing.WaitForFileTimeout = 1
	cfg.PostProcessing.RetryAttempts = 1
	cfg.PostProcessing.RetryDelay = 0
	cfg.PostProcessing.CleanupOnFailure = true
	cfg.PostProcessing.Pipeline.Enabled = true
	cfg.PostProcessing.Pipeline.Rollback = true
	cfg.App.DataPath = dstDir

	cfg.Movies.DestinationFolder = filepath.Join(dstDir, "movies")
	cfg.Movies.MoveMethod = []string{"copy"}
	cfg.TVShows.DestinationFolder = filepath.Join(dstDir, "tvshows")
	cfg.TVShows.MoveMethod = []string{"copy"}
	cfg.Anime.DestinationFolder = filepath.Join(dstDir, "anime")
	cfg.Anime.MoveMethod = []string{"copy"}

	return cfg
}

// testLogger returns a silent logger for testing.
func testLogger() *utils.Logger {
	return utils.NewLogger(false, io.Discard)
}

// testMedia returns a Media instance for testing.
func testMedia(mediaType models.MediaType) *models.Media {
	return &models.Media{
		ID:         1,
		Type:       mediaType,
		Title:      "Test Movie",
		Year:       2024,
		MinQuality: "360p",
		MaxQuality: "2160p",
	}
}

// testContext returns a fully populated ProcessingContext for testing.
func testContext(t *testing.T, mediaType models.MediaType) *ProcessingContext {
	t.Helper()
	srcDir := t.TempDir()
	return &ProcessingContext{
		Media: testMedia(mediaType),
		TorrentStatus: torrent.TorrentStatus{
			Hash:        "abc123def456",
			Name:        "Test.Movie.2024.1080p.WEB-DL.mkv",
			Progress:    1.0,
			IsCompleted: true,
			DownloadDir: srcDir,
		},
		SeasonNumber:  1,
		EpisodeNumber: 5,
		DownloadPath:  srcDir,
		OriginalFiles: []string{},
		ProcessedFiles: []string{},
		Errors:        []error{},
		Metadata:      make(map[string]string),
	}
}

// createTestFile creates a file of sizeMB megabytes in dir. Returns full path.
func createTestFile(t *testing.T, dir, name string, sizeMB int) string {
	t.Helper()
	path := filepath.Join(dir, name)
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		t.Fatal(err)
	}
	data := make([]byte, sizeMB*1024*1024)
	if err := os.WriteFile(path, data, 0644); err != nil {
		t.Fatal(err)
	}
	// Set mtime to past to avoid HealthCheckStage 5s sleep
	past := time.Now().Add(-10 * time.Second)
	os.Chtimes(path, past, past)
	return path
}

// createSmallFile creates a file smaller than 1MB (for validation rejection tests).
func createSmallFile(t *testing.T, dir, name string, sizeKB int) string {
	t.Helper()
	path := filepath.Join(dir, name)
	data := make([]byte, sizeKB*1024)
	if err := os.WriteFile(path, data, 0644); err != nil {
		t.Fatal(err)
	}
	// Set mtime to past to avoid HealthCheckStage 5s sleep
	past := time.Now().Add(-10 * time.Second)
	os.Chtimes(path, past, past)
	return path
}

// mockNotifier implements notifications.Notifier and records calls.
type mockNotifier struct {
	mu    sync.Mutex
	calls []string
}

func (m *mockNotifier) NotifyDownloadStart(media *models.Media, name string) {
	m.record("download_start:" + media.Title)
}
func (m *mockNotifier) NotifyNotEnoughSpace(media *models.Media, name string) {
	m.record("not_enough_space:" + media.Title)
}
func (m *mockNotifier) NotifyDownloadError(media *models.Media, name string) {
	m.record("download_error:" + media.Title)
}
func (m *mockNotifier) NotifyDownloadComplete(media *models.Media, name string) {
	m.record("download_complete:" + media.Title)
}
func (m *mockNotifier) NotifyPostProcessComplete(media *models.Media, name string) {
	m.record("post_process:" + media.Title)
}
func (m *mockNotifier) Test() error {
	m.record("test")
	return nil
}
func (m *mockNotifier) record(call string) {
	m.mu.Lock()
	m.calls = append(m.calls, call)
	m.mu.Unlock()
}
func (m *mockNotifier) getCalls() []string {
	m.mu.Lock()
	defer m.mu.Unlock()
	cp := make([]string, len(m.calls))
	copy(cp, m.calls)
	return cp
}

// spyStage is a configurable fake Stage for testing pipeline orchestration.
type spyStage struct {
	name           string
	executeFunc    func(*ProcessingContext) error
	rollbackCalled bool
}

func (s *spyStage) Name() string { return s.name }
func (s *spyStage) Execute(ctx *ProcessingContext) error {
	if s.executeFunc != nil {
		return s.executeFunc(ctx)
	}
	return nil
}
func (s *spyStage) Rollback(ctx *ProcessingContext) error {
	s.rollbackCalled = true
	return nil
}

// newSpyStage creates a spy stage that succeeds.
func newSpyStage(name string) *spyStage {
	return &spyStage{name: name}
}

// newFailingStage creates a spy stage that returns an error.
func newFailingStage(name string, err error) *spyStage {
	return &spyStage{
		name: name,
		executeFunc: func(ctx *ProcessingContext) error {
			return err
		},
	}
}
