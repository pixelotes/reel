package services

import (
	"os"
	"testing"

	"reel/internal/database/models"
)

func TestHealthCheck_ReadableFiles(t *testing.T) {
	srcDir := t.TempDir()
	cfg := testConfig(srcDir, t.TempDir())

	f := createTestFile(t, srcDir, "movie.mkv", 2)

	ctx := testContext(t, models.MediaTypeMovie)
	ctx.OriginalFiles = []string{f}

	stage := NewHealthCheckStage(cfg, testLogger())
	if err := stage.Execute(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestHealthCheck_UnreadableFile(t *testing.T) {
	if os.Getuid() == 0 {
		t.Skip("running as root, permission tests unreliable")
	}

	srcDir := t.TempDir()
	cfg := testConfig(srcDir, t.TempDir())

	f := createTestFile(t, srcDir, "locked.mkv", 2)
	os.Chmod(f, 0000)
	defer os.Chmod(f, 0644)

	ctx := testContext(t, models.MediaTypeMovie)
	ctx.OriginalFiles = []string{f}

	stage := NewHealthCheckStage(cfg, testLogger())
	err := stage.Execute(ctx)
	if err == nil {
		t.Fatal("expected error for unreadable file")
	}
}

func TestHealthCheck_EmptyFile(t *testing.T) {
	srcDir := t.TempDir()
	cfg := testConfig(srcDir, t.TempDir())

	f := createSmallFile(t, srcDir, "empty.mkv", 0)

	ctx := testContext(t, models.MediaTypeMovie)
	ctx.OriginalFiles = []string{f}

	stage := NewHealthCheckStage(cfg, testLogger())
	// Empty file adds non-fatal error to ctx.Errors
	if err := stage.Execute(ctx); err != nil {
		t.Fatalf("health check should not return fatal error for empty video: %v", err)
	}
	if len(ctx.Errors) == 0 {
		t.Error("expected non-fatal error for empty video file")
	}
}

func TestHealthCheck_Rollback_IsNoop(t *testing.T) {
	cfg := testConfig(t.TempDir(), t.TempDir())
	stage := NewHealthCheckStage(cfg, testLogger())
	ctx := testContext(t, models.MediaTypeMovie)
	if err := stage.Rollback(ctx); err != nil {
		t.Fatalf("rollback should be noop: %v", err)
	}
}
