package services

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// StageMetrics tracks metrics for a single stage execution
type StageMetrics struct {
	StageName     string        `json:"stage_name"`
	StartTime     time.Time     `json:"start_time"`
	EndTime       time.Time     `json:"end_time"`
	Duration      time.Duration `json:"duration_ms"` // in milliseconds
	Success       bool          `json:"success"`
	Error         string        `json:"error,omitempty"`
	RolledBack    bool          `json:"rolled_back"`
	RollbackError string        `json:"rollback_error,omitempty"`
}

// PipelineMetrics tracks metrics for a complete pipeline execution
type PipelineMetrics struct {
	ID            string          `json:"id"`              // Processing state ID
	MediaID       int             `json:"media_id"`
	MediaTitle    string          `json:"media_title"`
	StartTime     time.Time       `json:"start_time"`
	EndTime       time.Time       `json:"end_time"`
	TotalDuration time.Duration   `json:"total_duration_ms"`
	Success       bool            `json:"success"`
	StagesRun     int             `json:"stages_run"`
	StagesFailed  int             `json:"stages_failed"`
	Stages        []StageMetrics  `json:"stages"`
	Resumed       bool            `json:"resumed"`          // Was this resumed from checkpoint?
	StagesSkipped int             `json:"stages_skipped"`   // Number of stages skipped (resume)
}

// MetricsAggregation provides aggregate statistics
type MetricsAggregation struct {
	TotalRuns          int                      `json:"total_runs"`
	SuccessfulRuns     int                      `json:"successful_runs"`
	FailedRuns         int                      `json:"failed_runs"`
	ResumedRuns        int                      `json:"resumed_runs"`
	TotalDuration      time.Duration            `json:"total_duration_ms"`
	AverageDuration    time.Duration            `json:"avg_duration_ms"`
	StageStats         map[string]*StageStats   `json:"stage_stats"`
	LastUpdated        time.Time                `json:"last_updated"`
}

// StageStats provides per-stage statistics
type StageStats struct {
	Name           string        `json:"name"`
	Executions     int           `json:"executions"`
	Successes      int           `json:"successes"`
	Failures       int           `json:"failures"`
	Rollbacks      int           `json:"rollbacks"`
	TotalDuration  time.Duration `json:"total_duration_ms"`
	AvgDuration    time.Duration `json:"avg_duration_ms"`
	MinDuration    time.Duration `json:"min_duration_ms"`
	MaxDuration    time.Duration `json:"max_duration_ms"`
	SuccessRate    float64       `json:"success_rate"`    // 0.0 - 1.0
}

// MetricsManager manages pipeline metrics collection and persistence
type MetricsManager struct {
	metricsDir   string
	currentRun   *PipelineMetrics
	aggregation  *MetricsAggregation
	mu           sync.RWMutex
	autoSave     bool
	maxHistoryMB int // Maximum size of history files in MB
}

// NewMetricsManager creates a new metrics manager
func NewMetricsManager(dataPath string, autoSave bool, maxHistoryMB int) (*MetricsManager, error) {
	metricsDir := filepath.Join(dataPath, "metrics")
	if err := os.MkdirAll(metricsDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create metrics directory: %w", err)
	}

	mm := &MetricsManager{
		metricsDir:   metricsDir,
		autoSave:     autoSave,
		maxHistoryMB: maxHistoryMB,
		aggregation: &MetricsAggregation{
			StageStats:  make(map[string]*StageStats),
			LastUpdated: time.Now(),
		},
	}

	// Load existing aggregation
	if err := mm.loadAggregation(); err != nil {
		// If no existing aggregation, start fresh (not an error)
		mm.aggregation = &MetricsAggregation{
			StageStats:  make(map[string]*StageStats),
			LastUpdated: time.Now(),
		}
	}

	return mm, nil
}

// StartPipeline begins tracking a new pipeline run
func (mm *MetricsManager) StartPipeline(stateID string, mediaID int, mediaTitle string, resumed bool, stagesSkipped int) {
	mm.mu.Lock()
	defer mm.mu.Unlock()

	mm.currentRun = &PipelineMetrics{
		ID:            stateID,
		MediaID:       mediaID,
		MediaTitle:    mediaTitle,
		StartTime:     time.Now(),
		Success:       false,
		Stages:        make([]StageMetrics, 0),
		Resumed:       resumed,
		StagesSkipped: stagesSkipped,
	}
}

// StartStage begins tracking a stage execution
func (mm *MetricsManager) StartStage(stageName string) {
	mm.mu.Lock()
	defer mm.mu.Unlock()

	if mm.currentRun == nil {
		return
	}

	stage := StageMetrics{
		StageName: stageName,
		StartTime: time.Now(),
	}

	mm.currentRun.Stages = append(mm.currentRun.Stages, stage)
}

// EndStage marks a stage as completed
func (mm *MetricsManager) EndStage(stageName string, success bool, err error) {
	mm.mu.Lock()
	defer mm.mu.Unlock()

	if mm.currentRun == nil {
		return
	}

	// Find the stage (should be the last one)
	for i := len(mm.currentRun.Stages) - 1; i >= 0; i-- {
		if mm.currentRun.Stages[i].StageName == stageName {
			mm.currentRun.Stages[i].EndTime = time.Now()
			mm.currentRun.Stages[i].Duration = mm.currentRun.Stages[i].EndTime.Sub(mm.currentRun.Stages[i].StartTime)
			mm.currentRun.Stages[i].Success = success
			if err != nil {
				mm.currentRun.Stages[i].Error = err.Error()
			}
			break
		}
	}

	mm.currentRun.StagesRun++
	if !success {
		mm.currentRun.StagesFailed++
	}
}

// RecordRollback marks a stage as rolled back
func (mm *MetricsManager) RecordRollback(stageName string, rollbackErr error) {
	mm.mu.Lock()
	defer mm.mu.Unlock()

	if mm.currentRun == nil {
		return
	}

	// Find the stage
	for i := range mm.currentRun.Stages {
		if mm.currentRun.Stages[i].StageName == stageName {
			mm.currentRun.Stages[i].RolledBack = true
			if rollbackErr != nil {
				mm.currentRun.Stages[i].RollbackError = rollbackErr.Error()
			}
			break
		}
	}
}

// EndPipeline completes the current pipeline run and updates aggregations
func (mm *MetricsManager) EndPipeline(success bool) {
	mm.mu.Lock()
	defer mm.mu.Unlock()

	if mm.currentRun == nil {
		return
	}

	mm.currentRun.EndTime = time.Now()
	mm.currentRun.TotalDuration = mm.currentRun.EndTime.Sub(mm.currentRun.StartTime)
	mm.currentRun.Success = success

	// Update aggregation
	mm.updateAggregation(mm.currentRun)

	// Save if enabled
	if mm.autoSave {
		mm.saveRunHistory(mm.currentRun)
		mm.saveAggregation()
	}

	// Clear current run
	mm.currentRun = nil
}

// updateAggregation updates aggregate statistics with the completed run
func (mm *MetricsManager) updateAggregation(run *PipelineMetrics) {
	mm.aggregation.TotalRuns++
	if run.Success {
		mm.aggregation.SuccessfulRuns++
	} else {
		mm.aggregation.FailedRuns++
	}
	if run.Resumed {
		mm.aggregation.ResumedRuns++
	}

	mm.aggregation.TotalDuration += run.TotalDuration
	mm.aggregation.AverageDuration = mm.aggregation.TotalDuration / time.Duration(mm.aggregation.TotalRuns)

	// Update per-stage stats
	for _, stage := range run.Stages {
		stats, exists := mm.aggregation.StageStats[stage.StageName]
		if !exists {
			stats = &StageStats{
				Name:        stage.StageName,
				MinDuration: stage.Duration,
				MaxDuration: stage.Duration,
			}
			mm.aggregation.StageStats[stage.StageName] = stats
		}

		stats.Executions++
		if stage.Success {
			stats.Successes++
		} else {
			stats.Failures++
		}
		if stage.RolledBack {
			stats.Rollbacks++
		}

		stats.TotalDuration += stage.Duration
		stats.AvgDuration = stats.TotalDuration / time.Duration(stats.Executions)

		if stage.Duration < stats.MinDuration {
			stats.MinDuration = stage.Duration
		}
		if stage.Duration > stats.MaxDuration {
			stats.MaxDuration = stage.Duration
		}

		if stats.Executions > 0 {
			stats.SuccessRate = float64(stats.Successes) / float64(stats.Executions)
		}
	}

	mm.aggregation.LastUpdated = time.Now()
}

// GetAggregation returns current aggregate statistics
func (mm *MetricsManager) GetAggregation() *MetricsAggregation {
	mm.mu.RLock()
	defer mm.mu.RUnlock()

	// Return a copy to avoid race conditions
	agg := *mm.aggregation
	agg.StageStats = make(map[string]*StageStats)
	for k, v := range mm.aggregation.StageStats {
		statsCopy := *v
		agg.StageStats[k] = &statsCopy
	}

	return &agg
}

// GetCurrentRun returns the currently executing pipeline metrics
func (mm *MetricsManager) GetCurrentRun() *PipelineMetrics {
	mm.mu.RLock()
	defer mm.mu.RUnlock()

	if mm.currentRun == nil {
		return nil
	}

	// Return a copy
	run := *mm.currentRun
	run.Stages = make([]StageMetrics, len(mm.currentRun.Stages))
	copy(run.Stages, mm.currentRun.Stages)

	return &run
}

// saveRunHistory saves a completed run to disk
func (mm *MetricsManager) saveRunHistory(run *PipelineMetrics) error {
	historyPath := filepath.Join(mm.metricsDir, "history", run.StartTime.Format("2006-01-02"))
	if err := os.MkdirAll(historyPath, 0755); err != nil {
		return fmt.Errorf("failed to create history directory: %w", err)
	}

	filename := fmt.Sprintf("%s_%s.json", run.StartTime.Format("15-04-05"), run.ID)
	filePath := filepath.Join(historyPath, filename)

	data, err := json.MarshalIndent(run, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal run metrics: %w", err)
	}

	if err := os.WriteFile(filePath, data, 0644); err != nil {
		return fmt.Errorf("failed to write run metrics: %w", err)
	}

	// Check and cleanup old history if needed
	go mm.cleanupOldHistory()

	return nil
}

// saveAggregation saves aggregate statistics to disk
func (mm *MetricsManager) saveAggregation() error {
	filePath := filepath.Join(mm.metricsDir, "aggregation.json")

	data, err := json.MarshalIndent(mm.aggregation, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal aggregation: %w", err)
	}

	if err := os.WriteFile(filePath, data, 0644); err != nil {
		return fmt.Errorf("failed to write aggregation: %w", err)
	}

	return nil
}

// loadAggregation loads aggregate statistics from disk
func (mm *MetricsManager) loadAggregation() error {
	filePath := filepath.Join(mm.metricsDir, "aggregation.json")

	data, err := os.ReadFile(filePath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil // Not an error if file doesn't exist
		}
		return fmt.Errorf("failed to read aggregation: %w", err)
	}

	var agg MetricsAggregation
	if err := json.Unmarshal(data, &agg); err != nil {
		return fmt.Errorf("failed to unmarshal aggregation: %w", err)
	}

	mm.aggregation = &agg
	return nil
}

// cleanupOldHistory removes old history files to keep size under limit
func (mm *MetricsManager) cleanupOldHistory() {
	historyDir := filepath.Join(mm.metricsDir, "history")

	// Calculate current size
	var totalSize int64
	filepath.Walk(historyDir, func(path string, info os.FileInfo, err error) error {
		if err == nil && !info.IsDir() {
			totalSize += info.Size()
		}
		return nil
	})

	maxBytes := int64(mm.maxHistoryMB) * 1024 * 1024
	if totalSize <= maxBytes {
		return // Under limit
	}

	// Remove oldest date directories until under limit
	entries, err := os.ReadDir(historyDir)
	if err != nil {
		return
	}

	// Sort by name (date format sorts chronologically)
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		dirPath := filepath.Join(historyDir, entry.Name())
		dirInfo, err := entry.Info()
		if err != nil {
			continue
		}

		os.RemoveAll(dirPath)
		totalSize -= getDirSize(dirPath)

		if totalSize <= maxBytes {
			break
		}
	}
}

// getDirSize calculates directory size
func getDirSize(path string) int64 {
	var size int64
	filepath.Walk(path, func(_ string, info os.FileInfo, err error) error {
		if err == nil && !info.IsDir() {
			size += info.Size()
		}
		return nil
	})
	return size
}

// ResetAggregation resets all aggregate statistics
func (mm *MetricsManager) ResetAggregation() {
	mm.mu.Lock()
	defer mm.mu.Unlock()

	mm.aggregation = &MetricsAggregation{
		StageStats:  make(map[string]*StageStats),
		LastUpdated: time.Now(),
	}

	if mm.autoSave {
		mm.saveAggregation()
	}
}

// ExportMetrics exports metrics in a human-readable format
func (mm *MetricsManager) ExportMetrics(filePath string) error {
	mm.mu.RLock()
	agg := mm.GetAggregation()
	mm.mu.RUnlock()

	f, err := os.Create(filePath)
	if err != nil {
		return err
	}
	defer f.Close()

	// Write summary
	fmt.Fprintf(f, "# Pipeline Metrics Summary\n")
	fmt.Fprintf(f, "Generated: %s\n\n", time.Now().Format(time.RFC3339))
	fmt.Fprintf(f, "## Overall Statistics\n")
	fmt.Fprintf(f, "- Total Runs: %d\n", agg.TotalRuns)
	fmt.Fprintf(f, "- Successful: %d (%.1f%%)\n", agg.SuccessfulRuns, float64(agg.SuccessfulRuns)/float64(agg.TotalRuns)*100)
	fmt.Fprintf(f, "- Failed: %d (%.1f%%)\n", agg.FailedRuns, float64(agg.FailedRuns)/float64(agg.TotalRuns)*100)
	fmt.Fprintf(f, "- Resumed: %d\n", agg.ResumedRuns)
	fmt.Fprintf(f, "- Average Duration: %s\n\n", agg.AverageDuration)

	fmt.Fprintf(f, "## Stage Statistics\n\n")
	for _, stats := range agg.StageStats {
		fmt.Fprintf(f, "### %s\n", stats.Name)
		fmt.Fprintf(f, "- Executions: %d\n", stats.Executions)
		fmt.Fprintf(f, "- Success Rate: %.1f%%\n", stats.SuccessRate*100)
		fmt.Fprintf(f, "- Avg Duration: %s\n", stats.AvgDuration)
		fmt.Fprintf(f, "- Min Duration: %s\n", stats.MinDuration)
		fmt.Fprintf(f, "- Max Duration: %s\n", stats.MaxDuration)
		fmt.Fprintf(f, "- Rollbacks: %d\n\n", stats.Rollbacks)
	}

	return nil
}
