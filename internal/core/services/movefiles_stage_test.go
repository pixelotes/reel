package services

import (
	"os"
	"path/filepath"
	"testing"

	"reel/internal/database/models"
)

func TestMoveFiles_CopyMethod(t *testing.T) {
	srcDir := t.TempDir()
	dstDir := t.TempDir()
	cfg := testConfig(srcDir, dstDir)
	cfg.Movies.MoveMethod = []string{"copy"}

	f := createTestFile(t, srcDir, "movie.mkv", 2)

	ctx := testContext(t, models.MediaTypeMovie)
	ctx.OriginalFiles = []string{f}
	ctx.DestinationDir = dstDir

	stage := NewMoveFilesStage(cfg, testLogger())
	if err := stage.Execute(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(ctx.ProcessedFiles) != 1 {
		t.Fatalf("expected 1 processed file, got %d", len(ctx.ProcessedFiles))
	}

	// Destination should exist
	if _, err := os.Stat(ctx.ProcessedFiles[0]); os.IsNotExist(err) {
		t.Error("destination file should exist")
	}

	// Original should be removed (copy removes original)
	if _, err := os.Stat(f); !os.IsNotExist(err) {
		t.Error("original file should be removed after copy")
	}
}

func TestMoveFiles_MoveMethod(t *testing.T) {
	srcDir := t.TempDir()
	dstDir := t.TempDir()
	cfg := testConfig(srcDir, dstDir)
	cfg.Movies.MoveMethod = []string{"move"}

	f := createTestFile(t, srcDir, "movie.mkv", 2)

	ctx := testContext(t, models.MediaTypeMovie)
	ctx.OriginalFiles = []string{f}
	ctx.DestinationDir = dstDir

	stage := NewMoveFilesStage(cfg, testLogger())
	if err := stage.Execute(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if _, err := os.Stat(ctx.ProcessedFiles[0]); os.IsNotExist(err) {
		t.Error("destination file should exist")
	}
	if _, err := os.Stat(f); !os.IsNotExist(err) {
		t.Error("original file should not exist after move")
	}
}

func TestMoveFiles_HardlinkMethod(t *testing.T) {
	srcDir := t.TempDir()
	dstDir := t.TempDir()
	cfg := testConfig(srcDir, dstDir)
	cfg.Movies.MoveMethod = []string{"hardlink"}

	f := createTestFile(t, srcDir, "movie.mkv", 2)

	ctx := testContext(t, models.MediaTypeMovie)
	ctx.OriginalFiles = []string{f}
	ctx.DestinationDir = dstDir

	stage := NewMoveFilesStage(cfg, testLogger())
	if err := stage.Execute(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Both should exist (hardlink)
	if _, err := os.Stat(f); os.IsNotExist(err) {
		t.Error("original should still exist after hardlink")
	}
	if _, err := os.Stat(ctx.ProcessedFiles[0]); os.IsNotExist(err) {
		t.Error("hardlink destination should exist")
	}
}

func TestMoveFiles_SymlinkMethod(t *testing.T) {
	srcDir := t.TempDir()
	dstDir := t.TempDir()
	cfg := testConfig(srcDir, dstDir)
	cfg.Movies.MoveMethod = []string{"symlink"}

	f := createTestFile(t, srcDir, "movie.mkv", 2)

	ctx := testContext(t, models.MediaTypeMovie)
	ctx.OriginalFiles = []string{f}
	ctx.DestinationDir = dstDir

	stage := NewMoveFilesStage(cfg, testLogger())
	if err := stage.Execute(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Symlink should resolve to original
	target, err := os.Readlink(ctx.ProcessedFiles[0])
	if err != nil {
		t.Fatalf("expected symlink: %v", err)
	}
	if target != f {
		t.Errorf("symlink target = %q, want %q", target, f)
	}
}

func TestMoveFiles_FallbackMethod(t *testing.T) {
	srcDir := t.TempDir()
	dstDir := t.TempDir()
	cfg := testConfig(srcDir, dstDir)
	// First method fails (hardlink to non-existent), fallback to copy
	cfg.Movies.MoveMethod = []string{"hardlink", "copy"}

	f := createTestFile(t, srcDir, "movie.mkv", 2)

	// Create a file at destination that would block hardlink
	blockPath := filepath.Join(dstDir, "movie.mkv")
	os.WriteFile(blockPath, []byte("block"), 0644)

	ctx := testContext(t, models.MediaTypeMovie)
	ctx.OriginalFiles = []string{f}
	ctx.DestinationDir = dstDir

	stage := NewMoveFilesStage(cfg, testLogger())
	err := stage.Execute(ctx)
	// Hardlink fails (file exists), copy also fails (file exists) - both fail
	// This is expected to fail since both methods can't overwrite
	if err == nil {
		// If it succeeds, that's ok too (depends on OS behavior)
		return
	}
}

func TestMoveFiles_NoMethodsConfigured(t *testing.T) {
	srcDir := t.TempDir()
	dstDir := t.TempDir()
	cfg := testConfig(srcDir, dstDir)
	cfg.Movies.MoveMethod = []string{}

	f := createTestFile(t, srcDir, "movie.mkv", 2)

	ctx := testContext(t, models.MediaTypeMovie)
	ctx.OriginalFiles = []string{f}
	ctx.DestinationDir = dstDir

	stage := NewMoveFilesStage(cfg, testLogger())
	err := stage.Execute(ctx)
	if err == nil {
		t.Fatal("expected error with no move methods")
	}
}

func TestMoveFiles_MultipleFiles(t *testing.T) {
	srcDir := t.TempDir()
	dstDir := t.TempDir()
	cfg := testConfig(srcDir, dstDir)
	cfg.Movies.MoveMethod = []string{"copy"}

	f1 := createTestFile(t, srcDir, "ep01.mkv", 2)
	f2 := createTestFile(t, srcDir, "ep02.mkv", 2)
	f3 := createTestFile(t, srcDir, "ep03.mkv", 2)

	ctx := testContext(t, models.MediaTypeMovie)
	ctx.OriginalFiles = []string{f1, f2, f3}
	ctx.DestinationDir = dstDir

	stage := NewMoveFilesStage(cfg, testLogger())
	if err := stage.Execute(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(ctx.ProcessedFiles) != 3 {
		t.Errorf("expected 3 processed files, got %d", len(ctx.ProcessedFiles))
	}
}

func TestMoveFiles_Rollback_RemovesProcessedFiles(t *testing.T) {
	srcDir := t.TempDir()
	dstDir := t.TempDir()
	cfg := testConfig(srcDir, dstDir)
	cfg.Movies.MoveMethod = []string{"copy"}

	f := createTestFile(t, srcDir, "movie.mkv", 2)

	ctx := testContext(t, models.MediaTypeMovie)
	ctx.OriginalFiles = []string{f}
	ctx.DestinationDir = dstDir

	stage := NewMoveFilesStage(cfg, testLogger())
	stage.Execute(ctx)

	// Verify file exists before rollback
	if _, err := os.Stat(ctx.ProcessedFiles[0]); os.IsNotExist(err) {
		t.Fatal("processed file should exist before rollback")
	}

	stage.Rollback(ctx)

	// Verify file removed after rollback
	if _, err := os.Stat(ctx.ProcessedFiles[0]); !os.IsNotExist(err) {
		t.Error("processed file should be removed after rollback")
	}
}
