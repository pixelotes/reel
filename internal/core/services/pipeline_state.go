package services

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// ProcessingState tracks the state of pipeline execution
type ProcessingState struct {
	ID              string            `json:"id"`                // Unique processing ID (hash-based)
	MediaID         int               `json:"media_id"`          // Media ID being processed
	MediaTitle      string            `json:"media_title"`       // For logging
	TorrentHash     string            `json:"torrent_hash"`      // Torrent hash
	StartTime       time.Time         `json:"start_time"`        // When processing started
	LastUpdate      time.Time         `json:"last_update"`       // Last state update
	Status          string            `json:"status"`            // "in_progress", "completed", "failed"
	CompletedStages []string          `json:"completed_stages"`  // Stages that completed
	CurrentStage    string            `json:"current_stage"`     // Currently executing stage
	FailedStage     string            `json:"failed_stage"`      // Stage that failed
	Error           string            `json:"error,omitempty"`   // Error message if failed
	ProcessedFiles  []string          `json:"processed_files"`   // Files that were processed
	Metadata        map[string]string `json:"metadata"`          // Additional metadata
}

// StateManager manages processing state persistence
type StateManager struct {
	stateDir string
	mu       sync.RWMutex
}

// NewStateManager creates a new state manager
func NewStateManager(dataPath string) (*StateManager, error) {
	stateDir := filepath.Join(dataPath, "processing_states")
	if err := os.MkdirAll(stateDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create state directory: %w", err)
	}

	return &StateManager{
		stateDir: stateDir,
	}, nil
}

// Save persists the processing state to disk
func (sm *StateManager) Save(state *ProcessingState) error {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	state.LastUpdate = time.Now()

	data, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal state: %w", err)
	}

	statePath := filepath.Join(sm.stateDir, state.ID+".json")
	if err := os.WriteFile(statePath, data, 0644); err != nil {
		return fmt.Errorf("failed to write state file: %w", err)
	}

	return nil
}

// Load loads a processing state from disk
func (sm *StateManager) Load(stateID string) (*ProcessingState, error) {
	sm.mu.RLock()
	defer sm.mu.RUnlock()

	statePath := filepath.Join(sm.stateDir, stateID+".json")
	data, err := os.ReadFile(statePath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil // State doesn't exist
		}
		return nil, fmt.Errorf("failed to read state file: %w", err)
	}

	var state ProcessingState
	if err := json.Unmarshal(data, &state); err != nil {
		return nil, fmt.Errorf("failed to unmarshal state: %w", err)
	}

	return &state, nil
}

// Delete removes a processing state
func (sm *StateManager) Delete(stateID string) error {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	statePath := filepath.Join(sm.stateDir, stateID+".json")
	if err := os.Remove(statePath); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("failed to delete state file: %w", err)
	}

	return nil
}

// ListIncomplete returns all incomplete processing states
func (sm *StateManager) ListIncomplete() ([]*ProcessingState, error) {
	sm.mu.RLock()
	defer sm.mu.RUnlock()

	files, err := os.ReadDir(sm.stateDir)
	if err != nil {
		return nil, fmt.Errorf("failed to read state directory: %w", err)
	}

	var incomplete []*ProcessingState
	for _, file := range files {
		if file.IsDir() || filepath.Ext(file.Name()) != ".json" {
			continue
		}

		stateID := file.Name()[:len(file.Name())-5] // Remove .json
		state, err := sm.Load(stateID)
		if err != nil {
			continue // Skip invalid states
		}

		if state != nil && state.Status == "in_progress" {
			incomplete = append(incomplete, state)
		}
	}

	return incomplete, nil
}

// CleanupOld removes old completed/failed states
func (sm *StateManager) CleanupOld(maxAge time.Duration) error {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	files, err := os.ReadDir(sm.stateDir)
	if err != nil {
		return fmt.Errorf("failed to read state directory: %w", err)
	}

	cutoff := time.Now().Add(-maxAge)

	for _, file := range files {
		if file.IsDir() || filepath.Ext(file.Name()) != ".json" {
			continue
		}

		statePath := filepath.Join(sm.stateDir, file.Name())
		data, err := os.ReadFile(statePath)
		if err != nil {
			continue
		}

		var state ProcessingState
		if err := json.Unmarshal(data, &state); err != nil {
			continue
		}

		// Remove old completed/failed states
		if state.Status != "in_progress" && state.LastUpdate.Before(cutoff) {
			os.Remove(statePath)
		}
	}

	return nil
}

// GenerateStateID generates a unique state ID from media and torrent info
func GenerateStateID(mediaID int, torrentHash string) string {
	return fmt.Sprintf("%d_%s", mediaID, torrentHash)
}

// NewProcessingState creates a new processing state
func NewProcessingState(mediaID int, mediaTitle, torrentHash string) *ProcessingState {
	return &ProcessingState{
		ID:              GenerateStateID(mediaID, torrentHash),
		MediaID:         mediaID,
		MediaTitle:      mediaTitle,
		TorrentHash:     torrentHash,
		StartTime:       time.Now(),
		LastUpdate:      time.Now(),
		Status:          "in_progress",
		CompletedStages: make([]string, 0),
		ProcessedFiles:  make([]string, 0),
		Metadata:        make(map[string]string),
	}
}
