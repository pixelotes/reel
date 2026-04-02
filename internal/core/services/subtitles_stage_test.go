package services

import (
	"testing"

	"reel/internal/database/models"
)

func TestSubtitles_Disabled_Skips(t *testing.T) {
	cfg := testConfig(t.TempDir(), t.TempDir())
	cfg.Subtitles.Enabled = false

	stage := NewSubtitlesStage(cfg, testLogger(), nil)
	ctx := testContext(t, models.MediaTypeMovie)

	if err := stage.Execute(ctx); err != nil {
		t.Fatalf("disabled subtitles should not error: %v", err)
	}
}

func TestSubtitles_NilClient_Skips(t *testing.T) {
	cfg := testConfig(t.TempDir(), t.TempDir())
	cfg.Subtitles.Enabled = true

	stage := NewSubtitlesStage(cfg, testLogger(), nil)
	ctx := testContext(t, models.MediaTypeMovie)

	if err := stage.Execute(ctx); err != nil {
		t.Fatalf("nil client should not error: %v", err)
	}
}

func TestSubtitles_Rollback_IsNoop(t *testing.T) {
	cfg := testConfig(t.TempDir(), t.TempDir())
	stage := NewSubtitlesStage(cfg, testLogger(), nil)
	ctx := testContext(t, models.MediaTypeMovie)
	if err := stage.Rollback(ctx); err != nil {
		t.Fatalf("rollback should be noop: %v", err)
	}
}
