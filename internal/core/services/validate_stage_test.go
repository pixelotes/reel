package services

import (
	"testing"

	"reel/internal/database/models"
)

func TestValidateStage_AcceptsValidFiles(t *testing.T) {
	srcDir := t.TempDir()
	dstDir := t.TempDir()
	cfg := testConfig(srcDir, dstDir)

	mkv := createTestFile(t, srcDir, "movie.mkv", 2)
	mp4 := createTestFile(t, srcDir, "movie.mp4", 2)

	ctx := testContext(t, models.MediaTypeMovie)
	ctx.OriginalFiles = []string{mkv, mp4}

	stage := NewValidateStage(cfg, testLogger())
	if err := stage.Execute(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(ctx.OriginalFiles) != 2 {
		t.Errorf("expected 2 valid files, got %d", len(ctx.OriginalFiles))
	}
}

func TestValidateStage_AllExtensions(t *testing.T) {
	srcDir := t.TempDir()
	dstDir := t.TempDir()
	cfg := testConfig(srcDir, dstDir)

	exts := []string{"movie.mkv", "movie.mp4", "movie.avi", "movie.mov"}
	var files []string
	for _, name := range exts {
		files = append(files, createTestFile(t, srcDir, name, 2))
	}

	ctx := testContext(t, models.MediaTypeMovie)
	ctx.OriginalFiles = files

	stage := NewValidateStage(cfg, testLogger())
	if err := stage.Execute(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(ctx.OriginalFiles) != 4 {
		t.Errorf("expected 4 valid files, got %d", len(ctx.OriginalFiles))
	}
}

func TestValidateStage_FiltersNonVideoExtensions(t *testing.T) {
	srcDir := t.TempDir()
	dstDir := t.TempDir()
	cfg := testConfig(srcDir, dstDir)

	mkv := createTestFile(t, srcDir, "movie.mkv", 2)
	createTestFile(t, srcDir, "readme.txt", 2)
	createTestFile(t, srcDir, "subs.srt", 2)

	ctx := testContext(t, models.MediaTypeMovie)
	ctx.OriginalFiles = []string{
		mkv,
		srcDir + "/readme.txt",
		srcDir + "/subs.srt",
	}

	stage := NewValidateStage(cfg, testLogger())
	if err := stage.Execute(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(ctx.OriginalFiles) != 1 {
		t.Errorf("expected 1 valid file, got %d", len(ctx.OriginalFiles))
	}
}

func TestValidateStage_FiltersByMinSize(t *testing.T) {
	srcDir := t.TempDir()
	dstDir := t.TempDir()
	cfg := testConfig(srcDir, dstDir)

	small := createSmallFile(t, srcDir, "small.mkv", 500) // 500KB < 1MB min
	big := createTestFile(t, srcDir, "big.mkv", 2)

	ctx := testContext(t, models.MediaTypeMovie)
	ctx.OriginalFiles = []string{small, big}

	stage := NewValidateStage(cfg, testLogger())
	if err := stage.Execute(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(ctx.OriginalFiles) != 1 {
		t.Errorf("expected 1 valid file, got %d", len(ctx.OriginalFiles))
	}
}

func TestValidateStage_NoValidFiles_ReturnsError(t *testing.T) {
	srcDir := t.TempDir()
	dstDir := t.TempDir()
	cfg := testConfig(srcDir, dstDir)

	txt := createTestFile(t, srcDir, "readme.txt", 2)

	ctx := testContext(t, models.MediaTypeMovie)
	ctx.OriginalFiles = []string{txt}

	stage := NewValidateStage(cfg, testLogger())
	err := stage.Execute(ctx)
	if err == nil {
		t.Fatal("expected error for no valid files")
	}
}

func TestValidateStage_DisabledSkips(t *testing.T) {
	srcDir := t.TempDir()
	dstDir := t.TempDir()
	cfg := testConfig(srcDir, dstDir)
	cfg.PostProcessing.Enabled = false

	ctx := testContext(t, models.MediaTypeMovie)
	ctx.OriginalFiles = []string{"/nonexistent.txt"}

	stage := NewValidateStage(cfg, testLogger())
	if err := stage.Execute(ctx); err != nil {
		t.Fatalf("disabled stage should not error: %v", err)
	}
}

func TestValidateStage_Rollback_IsNoop(t *testing.T) {
	cfg := testConfig(t.TempDir(), t.TempDir())
	stage := NewValidateStage(cfg, testLogger())
	ctx := testContext(t, models.MediaTypeMovie)
	if err := stage.Rollback(ctx); err != nil {
		t.Fatalf("rollback should be noop: %v", err)
	}
}
