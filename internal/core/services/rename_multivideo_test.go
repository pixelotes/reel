package services

import (
	"os"
	"path/filepath"
	"testing"

	"reel/internal/database/models"
)

func TestRename_MultipleVideosOnlyLargestRenamed(t *testing.T) {
	dstDir := t.TempDir()
	cfg := testConfig(t.TempDir(), dstDir)

	mainVideo := createTestFile(t, dstDir, "Big.Movie.mp4", 3)
	extra := createTestFile(t, dstDir, "Big.Movie.Sample.mp4", 1)

	ctx := testContext(t, models.MediaTypeMovie)
	ctx.ProcessedFiles = []string{extra, mainVideo}
	ctx.TorrentStatus.Name = "Test.Movie.2024.1080p.WEB-DL.x264"

	if err := NewRenameStage(cfg, testLogger()).Execute(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if _, err := os.Stat(mainVideo); !os.IsNotExist(err) {
		t.Errorf("main video should have been renamed away from %s", mainVideo)
	}
	if _, err := os.Stat(extra); err != nil {
		t.Errorf("extra video should be untouched: %v", err)
	}
	want := filepath.Join(dstDir, "Test Movie (2024) [1080p].mp4")
	if _, err := os.Stat(want); err != nil {
		t.Errorf("expected renamed main video at %s: %v", want, err)
	}
	if len(ctx.ProcessedFiles) != 2 {
		t.Errorf("expected both files kept in ProcessedFiles, got %v", ctx.ProcessedFiles)
	}
}
