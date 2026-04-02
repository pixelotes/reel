package services

import (
	"os"
	"path/filepath"
	"testing"

	"reel/internal/database/models"
)

func TestCreateFolders_Movie(t *testing.T) {
	dstDir := t.TempDir()
	cfg := testConfig(t.TempDir(), dstDir)

	ctx := testContext(t, models.MediaTypeMovie)

	stage := NewCreateFoldersStage(cfg, testLogger())
	if err := stage.Execute(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	expected := filepath.Join(cfg.Movies.DestinationFolder, "Test Movie (2024)")
	if ctx.DestinationDir != expected {
		t.Errorf("expected %q, got %q", expected, ctx.DestinationDir)
	}
	if _, err := os.Stat(ctx.DestinationDir); os.IsNotExist(err) {
		t.Error("destination directory was not created")
	}
}

func TestCreateFolders_TVShow_WithSeason(t *testing.T) {
	dstDir := t.TempDir()
	cfg := testConfig(t.TempDir(), dstDir)

	ctx := testContext(t, models.MediaTypeTVShow)
	ctx.SeasonNumber = 2

	stage := NewCreateFoldersStage(cfg, testLogger())
	if err := stage.Execute(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	expected := filepath.Join(cfg.TVShows.DestinationFolder, "Test Movie (2024)", "S02")
	if ctx.DestinationDir != expected {
		t.Errorf("expected %q, got %q", expected, ctx.DestinationDir)
	}
	if _, err := os.Stat(ctx.DestinationDir); os.IsNotExist(err) {
		t.Error("destination directory was not created")
	}
}

func TestCreateFolders_Anime_WithSeason(t *testing.T) {
	dstDir := t.TempDir()
	cfg := testConfig(t.TempDir(), dstDir)

	ctx := testContext(t, models.MediaTypeAnime)
	ctx.SeasonNumber = 3

	stage := NewCreateFoldersStage(cfg, testLogger())
	if err := stage.Execute(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	expected := filepath.Join(cfg.Anime.DestinationFolder, "Test Movie (2024)", "S03")
	if ctx.DestinationDir != expected {
		t.Errorf("expected %q, got %q", expected, ctx.DestinationDir)
	}
}

func TestCreateFolders_TVShow_NoSeason(t *testing.T) {
	dstDir := t.TempDir()
	cfg := testConfig(t.TempDir(), dstDir)

	ctx := testContext(t, models.MediaTypeTVShow)
	ctx.SeasonNumber = 0

	stage := NewCreateFoldersStage(cfg, testLogger())
	if err := stage.Execute(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Should NOT have season subfolder
	expected := filepath.Join(cfg.TVShows.DestinationFolder, "Test Movie (2024)")
	if ctx.DestinationDir != expected {
		t.Errorf("expected %q (no season folder), got %q", expected, ctx.DestinationDir)
	}
}

func TestCreateFolders_SanitizesTitle(t *testing.T) {
	dstDir := t.TempDir()
	cfg := testConfig(t.TempDir(), dstDir)

	ctx := testContext(t, models.MediaTypeMovie)
	ctx.Media.Title = `Movie: The "Best"?`

	stage := NewCreateFoldersStage(cfg, testLogger())
	if err := stage.Execute(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Should not contain special characters
	if _, err := os.Stat(ctx.DestinationDir); os.IsNotExist(err) {
		t.Errorf("sanitized folder should exist: %s", ctx.DestinationDir)
	}
}

func TestCreateFolders_Rollback_IsNoop(t *testing.T) {
	cfg := testConfig(t.TempDir(), t.TempDir())
	stage := NewCreateFoldersStage(cfg, testLogger())
	ctx := testContext(t, models.MediaTypeMovie)
	if err := stage.Rollback(ctx); err != nil {
		t.Fatalf("rollback should be noop: %v", err)
	}
}
