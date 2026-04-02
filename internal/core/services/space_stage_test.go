package services

import (
	"testing"

	"reel/internal/database/models"
)

func TestSpaceCheck_NoDestination_Skips(t *testing.T) {
	cfg := testConfig(t.TempDir(), t.TempDir())

	ctx := testContext(t, models.MediaTypeMovie)
	ctx.DestinationDir = ""

	stage := NewSpaceCheckStage(cfg, testLogger())
	if err := stage.Execute(ctx); err != nil {
		t.Fatalf("should skip with no destination: %v", err)
	}
}

func TestSpaceCheck_SufficientSpace_Passes(t *testing.T) {
	srcDir := t.TempDir()
	dstDir := t.TempDir()
	cfg := testConfig(srcDir, dstDir)

	f := createTestFile(t, srcDir, "movie.mkv", 2)

	ctx := testContext(t, models.MediaTypeMovie)
	ctx.OriginalFiles = []string{f}
	ctx.DestinationDir = dstDir

	stage := NewSpaceCheckStage(cfg, testLogger())
	if err := stage.Execute(ctx); err != nil {
		t.Fatalf("should pass with sufficient space: %v", err)
	}
}

func TestSpaceCheck_SetsRequiredSpaceMetadata(t *testing.T) {
	srcDir := t.TempDir()
	dstDir := t.TempDir()
	cfg := testConfig(srcDir, dstDir)

	f := createTestFile(t, srcDir, "movie.mkv", 2)

	ctx := testContext(t, models.MediaTypeMovie)
	ctx.OriginalFiles = []string{f}
	ctx.DestinationDir = dstDir

	stage := NewSpaceCheckStage(cfg, testLogger())
	stage.Execute(ctx)

	if _, ok := ctx.Metadata["required_space_mb"]; !ok {
		t.Error("expected required_space_mb in metadata")
	}
}

func TestSpaceCheck_Rollback_IsNoop(t *testing.T) {
	cfg := testConfig(t.TempDir(), t.TempDir())
	stage := NewSpaceCheckStage(cfg, testLogger())
	ctx := testContext(t, models.MediaTypeMovie)
	if err := stage.Rollback(ctx); err != nil {
		t.Fatalf("rollback should be noop: %v", err)
	}
}
