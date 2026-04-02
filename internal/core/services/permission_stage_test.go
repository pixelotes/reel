package services

import (
	"os"
	"path/filepath"
	"testing"

	"reel/internal/database/models"
)

func TestPermission_ReadableFiles_Pass(t *testing.T) {
	srcDir := t.TempDir()
	cfg := testConfig(srcDir, t.TempDir())

	f := createTestFile(t, srcDir, "movie.mkv", 2)

	ctx := testContext(t, models.MediaTypeMovie)
	ctx.OriginalFiles = []string{f}
	ctx.DestinationDir = ""

	stage := NewPermissionCheckStage(cfg, testLogger())
	if err := stage.Execute(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestPermission_UnreadableFile_Fails(t *testing.T) {
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
	ctx.DestinationDir = ""

	stage := NewPermissionCheckStage(cfg, testLogger())
	err := stage.Execute(ctx)
	if err == nil {
		t.Fatal("expected error for unreadable file")
	}
}

func TestPermission_WritableDestination_Pass(t *testing.T) {
	srcDir := t.TempDir()
	dstDir := t.TempDir()
	cfg := testConfig(srcDir, dstDir)

	f := createTestFile(t, srcDir, "movie.mkv", 2)

	ctx := testContext(t, models.MediaTypeMovie)
	ctx.OriginalFiles = []string{f}
	ctx.DestinationDir = filepath.Join(dstDir, "output")

	stage := NewPermissionCheckStage(cfg, testLogger())
	if err := stage.Execute(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Verify test file was cleaned up
	testFile := filepath.Join(ctx.DestinationDir, ".reel_permission_test")
	if _, err := os.Stat(testFile); !os.IsNotExist(err) {
		t.Error("permission test file should be cleaned up")
	}
}

func TestPermission_Rollback_IsNoop(t *testing.T) {
	cfg := testConfig(t.TempDir(), t.TempDir())
	stage := NewPermissionCheckStage(cfg, testLogger())
	ctx := testContext(t, models.MediaTypeMovie)
	if err := stage.Rollback(ctx); err != nil {
		t.Fatalf("rollback should be noop: %v", err)
	}
}
