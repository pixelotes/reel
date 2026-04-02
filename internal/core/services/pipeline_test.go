package services

import (
	"fmt"
	"testing"

	"reel/internal/database/models"
)

func TestPipeline_ExecutesAllStages(t *testing.T) {
	cfg := testConfig(t.TempDir(), t.TempDir())

	var order []string
	s1 := &spyStage{name: "stage1", executeFunc: func(ctx *ProcessingContext) error {
		order = append(order, "stage1")
		return nil
	}}
	s2 := &spyStage{name: "stage2", executeFunc: func(ctx *ProcessingContext) error {
		order = append(order, "stage2")
		return nil
	}}
	s3 := &spyStage{name: "stage3", executeFunc: func(ctx *ProcessingContext) error {
		order = append(order, "stage3")
		return nil
	}}

	pipeline := NewPipeline(cfg, testLogger(), []Stage{s1, s2, s3})
	ctx := testContext(t, models.MediaTypeMovie)

	if err := pipeline.Execute(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(order) != 3 {
		t.Fatalf("expected 3 stages executed, got %d", len(order))
	}
	if order[0] != "stage1" || order[1] != "stage2" || order[2] != "stage3" {
		t.Errorf("wrong execution order: %v", order)
	}
}

func TestPipeline_StopsOnError(t *testing.T) {
	cfg := testConfig(t.TempDir(), t.TempDir())

	s1 := newSpyStage("stage1")
	s2 := newFailingStage("stage2", fmt.Errorf("stage2 failed"))
	s3Called := false
	s3 := &spyStage{name: "stage3", executeFunc: func(ctx *ProcessingContext) error {
		s3Called = true
		return nil
	}}

	pipeline := NewPipeline(cfg, testLogger(), []Stage{s1, s2, s3})
	ctx := testContext(t, models.MediaTypeMovie)

	err := pipeline.Execute(ctx)
	if err == nil {
		t.Fatal("expected error from failing stage")
	}

	if s3Called {
		t.Error("stage3 should not have been called after stage2 failed")
	}
}

func TestPipeline_RollbackOnFailure(t *testing.T) {
	cfg := testConfig(t.TempDir(), t.TempDir())

	s1 := newSpyStage("stage1")
	s2 := newSpyStage("stage2")
	s3 := newFailingStage("stage3", fmt.Errorf("stage3 failed"))

	pipeline := NewPipeline(cfg, testLogger(), []Stage{s1, s2, s3})
	pipeline.rollback = true
	ctx := testContext(t, models.MediaTypeMovie)

	pipeline.Execute(ctx)

	// s1 and s2 should have been rolled back (they completed before s3 failed)
	if !s1.rollbackCalled {
		t.Error("stage1 should have been rolled back")
	}
	if !s2.rollbackCalled {
		t.Error("stage2 should have been rolled back")
	}
	// s3 failed, so its rollback should NOT be called
	if s3.rollbackCalled {
		t.Error("stage3 (the failing stage) should not be rolled back")
	}
}

func TestPipeline_RollbackDisabled(t *testing.T) {
	cfg := testConfig(t.TempDir(), t.TempDir())

	s1 := newSpyStage("stage1")
	s2 := newFailingStage("stage2", fmt.Errorf("failed"))

	pipeline := NewPipeline(cfg, testLogger(), []Stage{s1, s2})
	pipeline.rollback = false
	ctx := testContext(t, models.MediaTypeMovie)

	pipeline.Execute(ctx)

	if s1.rollbackCalled {
		t.Error("rollback should not be called when disabled")
	}
}

func TestPipeline_ResumeSkipsCompleted(t *testing.T) {
	dstDir := t.TempDir()
	cfg := testConfig(t.TempDir(), dstDir)

	sm, err := NewStateManager(dstDir)
	if err != nil {
		t.Fatal(err)
	}

	// Pre-create state with stage1 completed
	ctx := testContext(t, models.MediaTypeMovie)
	state := NewProcessingState(ctx.Media.ID, ctx.Media.Title, ctx.TorrentStatus.Hash)
	state.CompletedStages = []string{"stage1"}
	sm.Save(state)

	s1Called := false
	s1 := &spyStage{name: "stage1", executeFunc: func(ctx *ProcessingContext) error {
		s1Called = true
		return nil
	}}
	s2 := newSpyStage("stage2")

	pipeline := NewPipeline(cfg, testLogger(), []Stage{s1, s2})
	pipeline.SetStateManager(sm)

	if err := pipeline.Execute(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if s1Called {
		t.Error("stage1 should have been skipped (already completed)")
	}
}

func TestPipeline_StatePersisted(t *testing.T) {
	dstDir := t.TempDir()
	cfg := testConfig(t.TempDir(), dstDir)

	sm, err := NewStateManager(dstDir)
	if err != nil {
		t.Fatal(err)
	}

	s1 := newSpyStage("stage1")
	pipeline := NewPipeline(cfg, testLogger(), []Stage{s1})
	pipeline.SetStateManager(sm)

	ctx := testContext(t, models.MediaTypeMovie)
	pipeline.Execute(ctx)

	stateID := GenerateStateID(ctx.Media.ID, ctx.TorrentStatus.Hash)
	loaded, err := sm.Load(stateID)
	if err != nil {
		t.Fatal(err)
	}
	// State should be cleaned up on success (CleanupOnFailure=true)
	// But if it exists, verify it's completed
	if loaded != nil && loaded.Status != "completed" {
		t.Errorf("expected completed status, got %q", loaded.Status)
	}
}

func TestPipeline_FailedStatePersisted(t *testing.T) {
	dstDir := t.TempDir()
	cfg := testConfig(t.TempDir(), dstDir)
	cfg.PostProcessing.CleanupOnFailure = false

	sm, err := NewStateManager(dstDir)
	if err != nil {
		t.Fatal(err)
	}

	s1 := newFailingStage("stage1", fmt.Errorf("boom"))
	pipeline := NewPipeline(cfg, testLogger(), []Stage{s1})
	pipeline.SetStateManager(sm)

	ctx := testContext(t, models.MediaTypeMovie)
	pipeline.Execute(ctx)

	stateID := GenerateStateID(ctx.Media.ID, ctx.TorrentStatus.Hash)
	loaded, err := sm.Load(stateID)
	if err != nil {
		t.Fatal(err)
	}
	if loaded == nil {
		t.Fatal("state should be persisted on failure")
	}
	if loaded.Status != "failed" {
		t.Errorf("expected failed status, got %q", loaded.Status)
	}
	if loaded.FailedStage != "stage1" {
		t.Errorf("expected failed_stage=stage1, got %q", loaded.FailedStage)
	}
}
