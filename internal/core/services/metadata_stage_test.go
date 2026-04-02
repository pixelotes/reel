package services

import (
	"testing"
	"time"

	"reel/internal/database/models"
)

func TestMetadata_AddsProcessingTime(t *testing.T) {
	srcDir := t.TempDir()
	cfg := testConfig(srcDir, t.TempDir())

	f := createTestFile(t, srcDir, "movie.mkv", 2)

	ctx := testContext(t, models.MediaTypeMovie)
	ctx.OriginalFiles = []string{f}

	stage := NewMetadataEnrichmentStage(cfg, testLogger())
	if err := stage.Execute(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	ts, ok := ctx.Metadata["processing_time"]
	if !ok {
		t.Fatal("expected processing_time in metadata")
	}
	if _, err := time.Parse(time.RFC3339, ts); err != nil {
		t.Errorf("processing_time is not valid RFC3339: %v", err)
	}
}

func TestMetadata_AddsTorrentInfo(t *testing.T) {
	srcDir := t.TempDir()
	cfg := testConfig(srcDir, t.TempDir())

	f := createTestFile(t, srcDir, "movie.mkv", 2)

	ctx := testContext(t, models.MediaTypeMovie)
	ctx.OriginalFiles = []string{f}

	stage := NewMetadataEnrichmentStage(cfg, testLogger())
	stage.Execute(ctx)

	if ctx.Metadata["torrent_hash"] != "abc123def456" {
		t.Errorf("expected torrent_hash=abc123def456, got %q", ctx.Metadata["torrent_hash"])
	}
	if ctx.Metadata["torrent_name"] == "" {
		t.Error("expected torrent_name in metadata")
	}
}

func TestMetadata_AddsQuality(t *testing.T) {
	srcDir := t.TempDir()
	cfg := testConfig(srcDir, t.TempDir())

	f := createTestFile(t, srcDir, "Show.1080p.WEB-DL.mkv", 2)

	ctx := testContext(t, models.MediaTypeMovie)
	ctx.OriginalFiles = []string{f}

	stage := NewMetadataEnrichmentStage(cfg, testLogger())
	stage.Execute(ctx)

	if ctx.Metadata["quality"] != "1080p" {
		t.Errorf("expected quality=1080p, got %q", ctx.Metadata["quality"])
	}
}

func TestMetadata_AddsFileCount(t *testing.T) {
	srcDir := t.TempDir()
	cfg := testConfig(srcDir, t.TempDir())

	f1 := createTestFile(t, srcDir, "ep01.mkv", 2)
	f2 := createTestFile(t, srcDir, "ep02.mkv", 2)
	f3 := createTestFile(t, srcDir, "ep03.mkv", 2)

	ctx := testContext(t, models.MediaTypeMovie)
	ctx.OriginalFiles = []string{f1, f2, f3}

	stage := NewMetadataEnrichmentStage(cfg, testLogger())
	stage.Execute(ctx)

	if ctx.Metadata["file_count"] != "3" {
		t.Errorf("expected file_count=3, got %q", ctx.Metadata["file_count"])
	}
}

func TestMetadata_AddsTotalSize(t *testing.T) {
	srcDir := t.TempDir()
	cfg := testConfig(srcDir, t.TempDir())

	f1 := createTestFile(t, srcDir, "ep01.mkv", 1)
	f2 := createTestFile(t, srcDir, "ep02.mkv", 1)

	ctx := testContext(t, models.MediaTypeMovie)
	ctx.OriginalFiles = []string{f1, f2}

	stage := NewMetadataEnrichmentStage(cfg, testLogger())
	stage.Execute(ctx)

	if ctx.Metadata["total_size_mb"] != "2" {
		t.Errorf("expected total_size_mb=2, got %q", ctx.Metadata["total_size_mb"])
	}
}

func TestMetadata_InitializesNilMetadataMap(t *testing.T) {
	srcDir := t.TempDir()
	cfg := testConfig(srcDir, t.TempDir())

	f := createTestFile(t, srcDir, "movie.mkv", 2)

	ctx := testContext(t, models.MediaTypeMovie)
	ctx.OriginalFiles = []string{f}
	ctx.Metadata = nil // Force nil

	stage := NewMetadataEnrichmentStage(cfg, testLogger())
	if err := stage.Execute(ctx); err != nil {
		t.Fatalf("should not panic on nil metadata: %v", err)
	}
	if ctx.Metadata == nil {
		t.Error("metadata should be initialized")
	}
}
