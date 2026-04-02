package services

import (
	"testing"

	"reel/internal/database/models"
)

func TestDuplicate_NoDestination_Skips(t *testing.T) {
	cfg := testConfig(t.TempDir(), t.TempDir())

	ctx := testContext(t, models.MediaTypeMovie)
	ctx.DestinationDir = ""

	stage := NewDuplicateCheckStage(cfg, testLogger())
	if err := stage.Execute(ctx); err != nil {
		t.Fatalf("should skip with no destination: %v", err)
	}
}

func TestDuplicate_NoDuplicates(t *testing.T) {
	srcDir := t.TempDir()
	dstDir := t.TempDir()
	cfg := testConfig(srcDir, dstDir)

	f := createTestFile(t, srcDir, "movie.mkv", 2)

	ctx := testContext(t, models.MediaTypeMovie)
	ctx.OriginalFiles = []string{f}
	ctx.DestinationDir = dstDir

	stage := NewDuplicateCheckStage(cfg, testLogger())
	if err := stage.Execute(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if ctx.Metadata["duplicate_warning"] == "true" {
		t.Error("should not detect duplicate in empty destination")
	}
}

func TestDuplicate_DetectsSimilarSize(t *testing.T) {
	srcDir := t.TempDir()
	dstDir := t.TempDir()
	cfg := testConfig(srcDir, dstDir)

	// Create source file
	f := createTestFile(t, srcDir, "movie.mkv", 2)
	// Create existing file in destination with same size
	createTestFile(t, dstDir, "existing.mkv", 2)

	ctx := testContext(t, models.MediaTypeMovie)
	ctx.OriginalFiles = []string{f}
	ctx.DestinationDir = dstDir

	stage := NewDuplicateCheckStage(cfg, testLogger())
	if err := stage.Execute(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if ctx.Metadata["duplicate_warning"] != "true" {
		t.Error("should detect duplicate with similar file size")
	}
}

func TestDuplicate_DifferentSize_NoDuplicate(t *testing.T) {
	srcDir := t.TempDir()
	dstDir := t.TempDir()
	cfg := testConfig(srcDir, dstDir)

	f := createTestFile(t, srcDir, "movie.mkv", 2)
	createTestFile(t, dstDir, "existing.mkv", 5)

	ctx := testContext(t, models.MediaTypeMovie)
	ctx.OriginalFiles = []string{f}
	ctx.DestinationDir = dstDir

	stage := NewDuplicateCheckStage(cfg, testLogger())
	if err := stage.Execute(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if ctx.Metadata["duplicate_warning"] == "true" {
		t.Error("should not detect duplicate with different file sizes")
	}
}
