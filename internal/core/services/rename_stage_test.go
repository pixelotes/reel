package services

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"reel/internal/database/models"
)

func TestRename_MovieDefaultTemplate(t *testing.T) {
	dstDir := t.TempDir()
	cfg := testConfig(t.TempDir(), dstDir)
	// No template set = default

	f := createTestFile(t, dstDir, "raw.download.mkv", 2)

	ctx := testContext(t, models.MediaTypeMovie)
	ctx.ProcessedFiles = []string{f}
	ctx.TorrentStatus.Name = "Test.Movie.2024.1080p.WEB-DL.x264"

	stage := NewRenameStage(cfg, testLogger())
	if err := stage.Execute(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(ctx.ProcessedFiles) != 1 {
		t.Fatalf("expected 1 file, got %d", len(ctx.ProcessedFiles))
	}

	name := filepath.Base(ctx.ProcessedFiles[0])
	if !strings.Contains(name, "Test Movie") {
		t.Errorf("expected title in name, got %q", name)
	}
	if !strings.Contains(name, "2024") {
		t.Errorf("expected year in name, got %q", name)
	}
	if !strings.Contains(name, "1080p") {
		t.Errorf("expected quality in name, got %q", name)
	}
}

func TestRename_TVShowDefaultTemplate(t *testing.T) {
	dstDir := t.TempDir()
	cfg := testConfig(t.TempDir(), dstDir)

	f := createTestFile(t, dstDir, "raw.download.mkv", 2)

	ctx := testContext(t, models.MediaTypeTVShow)
	ctx.ProcessedFiles = []string{f}
	ctx.SeasonNumber = 1
	ctx.EpisodeNumber = 5
	ctx.TorrentStatus.Name = "Test.Show.S01E05.720p.HDTV"

	stage := NewRenameStage(cfg, testLogger())
	if err := stage.Execute(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	name := filepath.Base(ctx.ProcessedFiles[0])
	if !strings.Contains(name, "S01E05") {
		t.Errorf("expected S01E05 in name, got %q", name)
	}
}

func TestRename_CustomTemplate(t *testing.T) {
	dstDir := t.TempDir()
	cfg := testConfig(t.TempDir(), dstDir)
	cfg.FileRenaming.MovieTemplate = "{title} ({year}) [{quality}]"

	f := createTestFile(t, dstDir, "raw.mkv", 2)

	ctx := testContext(t, models.MediaTypeMovie)
	ctx.ProcessedFiles = []string{f}
	ctx.TorrentStatus.Name = "Test.Movie.2024.1080p.BluRay"

	stage := NewRenameStage(cfg, testLogger())
	if err := stage.Execute(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	name := filepath.Base(ctx.ProcessedFiles[0])
	expected := "Test Movie (2024) [1080p].mkv"
	if name != expected {
		t.Errorf("expected %q, got %q", expected, name)
	}
}

func TestRename_SkipsSubtitleFiles(t *testing.T) {
	dstDir := t.TempDir()
	cfg := testConfig(t.TempDir(), dstDir)

	srt := createTestFile(t, dstDir, "movie.en.srt", 1)
	mkv := createTestFile(t, dstDir, "movie.mkv", 2)

	ctx := testContext(t, models.MediaTypeMovie)
	ctx.ProcessedFiles = []string{srt, mkv}
	ctx.TorrentStatus.Name = "Movie.1080p"

	stage := NewRenameStage(cfg, testLogger())
	if err := stage.Execute(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Only mkv should be renamed, srt skipped
	if len(ctx.ProcessedFiles) != 1 {
		t.Errorf("expected 1 renamed file (srt skipped), got %d", len(ctx.ProcessedFiles))
	}
}

func TestRename_Rollback_RestoresOriginalNames(t *testing.T) {
	dstDir := t.TempDir()
	cfg := testConfig(t.TempDir(), dstDir)

	original := createTestFile(t, dstDir, "raw.mkv", 2)

	ctx := testContext(t, models.MediaTypeMovie)
	ctx.ProcessedFiles = []string{original}
	ctx.TorrentStatus.Name = "Movie.1080p"

	stage := NewRenameStage(cfg, testLogger())
	stage.Execute(ctx)

	// ctx.ProcessedFiles now has the new name
	newPath := ctx.ProcessedFiles[0]
	if _, err := os.Stat(newPath); os.IsNotExist(err) {
		t.Fatal("renamed file should exist")
	}

	stage.Rollback(ctx)

	// Original name should be restored
	if _, err := os.Stat(original); os.IsNotExist(err) {
		t.Error("original file should be restored after rollback")
	}
}

func TestRename_StoresMetadataWithPrefix(t *testing.T) {
	dstDir := t.TempDir()
	cfg := testConfig(t.TempDir(), dstDir)

	f := createTestFile(t, dstDir, "raw.mkv", 2)

	ctx := testContext(t, models.MediaTypeMovie)
	ctx.ProcessedFiles = []string{f}
	ctx.TorrentStatus.Name = "Movie.1080p"

	stage := NewRenameStage(cfg, testLogger())
	stage.Execute(ctx)

	found := false
	for key := range ctx.Metadata {
		if strings.HasPrefix(key, renameMetadataPrefix) {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected metadata entries with rename: prefix")
	}
}
