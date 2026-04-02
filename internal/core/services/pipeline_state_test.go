package services

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestStateManager_SaveAndLoad(t *testing.T) {
	dir := t.TempDir()
	sm, err := NewStateManager(dir)
	if err != nil {
		t.Fatal(err)
	}

	state := NewProcessingState(1, "Test Movie", "hash123")
	state.CompletedStages = []string{"validate", "create_folders"}
	state.ProcessedFiles = []string{"/path/to/file.mkv"}

	if err := sm.Save(state); err != nil {
		t.Fatalf("save failed: %v", err)
	}

	loaded, err := sm.Load(state.ID)
	if err != nil {
		t.Fatalf("load failed: %v", err)
	}
	if loaded == nil {
		t.Fatal("loaded state should not be nil")
	}
	if loaded.MediaTitle != "Test Movie" {
		t.Errorf("expected title 'Test Movie', got %q", loaded.MediaTitle)
	}
	if len(loaded.CompletedStages) != 2 {
		t.Errorf("expected 2 completed stages, got %d", len(loaded.CompletedStages))
	}
	if len(loaded.ProcessedFiles) != 1 {
		t.Errorf("expected 1 processed file, got %d", len(loaded.ProcessedFiles))
	}
}

func TestStateManager_LoadNonexistent(t *testing.T) {
	dir := t.TempDir()
	sm, err := NewStateManager(dir)
	if err != nil {
		t.Fatal(err)
	}

	state, err := sm.Load("nonexistent_id")
	if err != nil {
		t.Fatalf("should not error for nonexistent: %v", err)
	}
	if state != nil {
		t.Error("should return nil for nonexistent state")
	}
}

func TestStateManager_Delete(t *testing.T) {
	dir := t.TempDir()
	sm, err := NewStateManager(dir)
	if err != nil {
		t.Fatal(err)
	}

	state := NewProcessingState(1, "Test", "hash")
	sm.Save(state)
	sm.Delete(state.ID)

	loaded, _ := sm.Load(state.ID)
	if loaded != nil {
		t.Error("state should be nil after delete")
	}
}

func TestStateManager_ListIncomplete(t *testing.T) {
	dir := t.TempDir()
	sm, err := NewStateManager(dir)
	if err != nil {
		t.Fatal(err)
	}

	// Create 2 in_progress and 1 completed
	s1 := NewProcessingState(1, "Movie A", "hash1")
	s2 := NewProcessingState(2, "Movie B", "hash2")
	s3 := NewProcessingState(3, "Movie C", "hash3")
	s3.Status = "completed"

	sm.Save(s1)
	sm.Save(s2)
	sm.Save(s3)

	incomplete, err := sm.ListIncomplete()
	if err != nil {
		t.Fatalf("list failed: %v", err)
	}
	if len(incomplete) != 2 {
		t.Errorf("expected 2 incomplete, got %d", len(incomplete))
	}
}

func TestStateManager_CleanupOld(t *testing.T) {
	dir := t.TempDir()
	sm, err := NewStateManager(dir)
	if err != nil {
		t.Fatal(err)
	}

	// Save both states first (Save overwrites LastUpdate with time.Now())
	old := NewProcessingState(1, "Old Movie", "hash_old")
	old.Status = "completed"
	sm.Save(old)

	recent := NewProcessingState(2, "Recent Movie", "hash_recent")
	recent.Status = "completed"
	sm.Save(recent)

	// Now manually rewrite the old state's JSON with a past LastUpdate
	old.LastUpdate = time.Now().Add(-48 * time.Hour)
	data, _ := json.MarshalIndent(old, "", "  ")
	os.WriteFile(filepath.Join(dir, "processing_states", old.ID+".json"), data, 0644)

	sm.CleanupOld(24 * time.Hour)

	// Old should be removed
	loaded, _ := sm.Load(old.ID)
	if loaded != nil {
		t.Error("old completed state should be cleaned up")
	}

	// Recent should still exist
	loaded, _ = sm.Load(recent.ID)
	if loaded == nil {
		t.Error("recent completed state should not be cleaned up")
	}
}

func TestGenerateStateID(t *testing.T) {
	id := GenerateStateID(42, "abc123")
	if id != "42_abc123" {
		t.Errorf("expected '42_abc123', got %q", id)
	}
}

func TestNewProcessingState(t *testing.T) {
	state := NewProcessingState(1, "Test", "hash")

	if state.Status != "in_progress" {
		t.Errorf("expected status 'in_progress', got %q", state.Status)
	}
	if state.MediaID != 1 {
		t.Errorf("expected media_id 1, got %d", state.MediaID)
	}
	if state.ID != "1_hash" {
		t.Errorf("expected id '1_hash', got %q", state.ID)
	}
	if state.CompletedStages == nil {
		t.Error("CompletedStages should be initialized")
	}
}
