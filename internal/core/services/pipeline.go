package services

import (
	"fmt"
	"reel/internal/clients/notifications"
	"reel/internal/clients/subtitles"
	"reel/internal/clients/torrent"
	"reel/internal/config"
	"reel/internal/database/models"
	"reel/internal/utils"
)

// ProcessingContext holds shared state across pipeline stages
type ProcessingContext struct {
	Media          *models.Media
	TorrentStatus  torrent.TorrentStatus
	SeasonNumber   int
	EpisodeNumber  int
	DownloadPath   string
	DestinationDir string
	OriginalFiles  []string // Files identified for processing
	ProcessedFiles []string // Files after processing (new paths)
	Errors         []error  // Non-fatal errors collected during processing
	Metadata       map[string]string
}

// Stage represents a single step in the postprocessing pipeline
type Stage interface {
	Name() string
	Execute(ctx *ProcessingContext) error
	Rollback(ctx *ProcessingContext) error
}

// Pipeline manages and executes stages in sequence
type Pipeline struct {
	stages       []Stage
	config       *config.Config
	logger       *utils.Logger
	rollback     bool // Whether to rollback on failure
	stateManager *StateManager
}

// NewPipeline creates a new pipeline with the given stages
func NewPipeline(cfg *config.Config, logger *utils.Logger, stages []Stage) *Pipeline {
	return &Pipeline{
		stages:   stages,
		config:   cfg,
		logger:   logger,
		rollback: true,
	}
}

// SetStateManager sets the state manager for recovery support
func (p *Pipeline) SetStateManager(sm *StateManager) {
	p.stateManager = sm
}

// Execute runs all stages in sequence
// If a stage fails and rollback is enabled, previously completed stages are rolled back
func (p *Pipeline) Execute(ctx *ProcessingContext) error {
	completedStages := make([]Stage, 0, len(p.stages))

	// Create/load processing state if state manager is available
	var state *ProcessingState
	if p.stateManager != nil && ctx.Media != nil {
		stateID := GenerateStateID(ctx.Media.ID, ctx.TorrentStatus.Hash)
		var err error
		state, err = p.stateManager.Load(stateID)
		if err != nil {
			p.logger.Warn("Failed to load processing state:", err)
		}

		// Create new state if doesn't exist
		if state == nil {
			state = NewProcessingState(ctx.Media.ID, ctx.Media.Title, ctx.TorrentStatus.Hash)
			p.logger.Info("Created new processing state:", state.ID)
		} else {
			p.logger.Info("Resuming from previous state:", state.ID)
		}
	}

	for _, stage := range p.stages {
		// Skip already completed stages if resuming
		if state != nil {
			skip := false
			for _, completed := range state.CompletedStages {
				if stage.Name() == completed {
					p.logger.Info(fmt.Sprintf("Pipeline: Skipping completed stage '%s'", stage.Name()))
					completedStages = append(completedStages, stage)
					skip = true
					break
				}
			}
			if skip {
				continue
			}
		}

		p.logger.Info(fmt.Sprintf("Pipeline: Executing stage '%s'", stage.Name()))

		// Update state
		if state != nil {
			state.CurrentStage = stage.Name()
			p.stateManager.Save(state)
		}

		if err := stage.Execute(ctx); err != nil {
			p.logger.Error(fmt.Sprintf("Pipeline: Stage '%s' failed: %v", stage.Name(), err))

			// Update state with failure
			if state != nil {
				state.Status = "failed"
				state.FailedStage = stage.Name()
				state.Error = err.Error()
				p.stateManager.Save(state)
			}

			// Rollback completed stages if enabled
			if p.rollback {
				p.logger.Info("Pipeline: Rolling back completed stages")
				p.performRollback(completedStages, ctx, state)
			}

			return fmt.Errorf("pipeline failed at stage '%s': %w", stage.Name(), err)
		}

		completedStages = append(completedStages, stage)

		// Update state with completion
		if state != nil {
			state.CompletedStages = append(state.CompletedStages, stage.Name())
			state.ProcessedFiles = ctx.ProcessedFiles
			p.stateManager.Save(state)
		}

		p.logger.Info(fmt.Sprintf("Pipeline: Stage '%s' completed successfully", stage.Name()))
	}

	// Mark state as completed
	if state != nil {
		state.Status = "completed"
		state.CurrentStage = ""
		p.stateManager.Save(state)

		// Delete state after successful completion (optional)
		if p.config.PostProcessing.CleanupOnFailure {
			p.stateManager.Delete(state.ID)
		}
	}

	p.logger.Info("Pipeline: All stages completed successfully")
	return nil
}

// performRollback executes rollback with detailed logging
func (p *Pipeline) performRollback(stages []Stage, ctx *ProcessingContext, state *ProcessingState) {
	rollbackCount := 0
	rollbackErrors := 0

	for i := len(stages) - 1; i >= 0; i-- {
		stage := stages[i]
		p.logger.Info(fmt.Sprintf("Pipeline: Rolling back stage '%s' (%d/%d)", stage.Name(), len(stages)-i, len(stages)))

		if rbErr := stage.Rollback(ctx); rbErr != nil {
			p.logger.Error(fmt.Sprintf("Pipeline: Rollback failed for '%s': %v", stage.Name(), rbErr))
			rollbackErrors++

			// Save rollback error in state
			if state != nil {
				if state.Metadata == nil {
					state.Metadata = make(map[string]string)
				}
				state.Metadata[fmt.Sprintf("rollback_error_%s", stage.Name())] = rbErr.Error()
			}
		} else {
			p.logger.Info(fmt.Sprintf("Pipeline: Successfully rolled back '%s'", stage.Name()))
			rollbackCount++
		}
	}

	if rollbackCount > 0 {
		p.logger.Warn(fmt.Sprintf("Pipeline: Rollback complete (%d successful, %d errors)", rollbackCount, rollbackErrors))
	}

	// Update state with rollback info
	if state != nil {
		if state.Metadata == nil {
			state.Metadata = make(map[string]string)
		}
		state.Metadata["rollback_count"] = fmt.Sprintf("%d", rollbackCount)
		state.Metadata["rollback_errors"] = fmt.Sprintf("%d", rollbackErrors)
		p.stateManager.Save(state)
	}
}

// StageConfig holds configuration for individual stages
type StageConfig struct {
	Enabled bool
	Options map[string]interface{}
}

// StageFactory creates stages based on name
type StageFactory struct {
	cfg            *config.Config
	logger         *utils.Logger
	mediaRepo      *models.MediaRepository
	notifiers      []notifications.Notifier
	subtitleClient *subtitles.Client
}

// NewStageFactory creates a new stage factory
func NewStageFactory(
	cfg *config.Config,
	logger *utils.Logger,
	mediaRepo *models.MediaRepository,
	notifiers []notifications.Notifier,
	subtitleClient *subtitles.Client,
) *StageFactory {
	return &StageFactory{
		cfg:            cfg,
		logger:         logger,
		mediaRepo:      mediaRepo,
		notifiers:      notifiers,
		subtitleClient: subtitleClient,
	}
}

// CreateStage creates a stage by name
func (sf *StageFactory) CreateStage(name string) (Stage, error) {
	switch name {
	case "validate":
		return NewValidateStage(sf.cfg, sf.logger), nil
	case "create_folders":
		return NewCreateFoldersStage(sf.cfg, sf.logger), nil
	case "move_files":
		return NewMoveFilesStage(sf.cfg, sf.logger), nil
	case "rename":
		return NewRenameStage(sf.cfg, sf.logger), nil
	case "subtitles":
		return NewSubtitlesStage(sf.cfg, sf.logger, sf.subtitleClient), nil
	case "notify":
		return NewNotifyStage(sf.logger, sf.notifiers), nil
	// New stages (Phase 6)
	case "extract":
		return NewExtractionStage(sf.cfg, sf.logger), nil
	case "health_check":
		return NewHealthCheckStage(sf.cfg, sf.logger), nil
	case "enrich_metadata":
		return NewMetadataEnrichmentStage(sf.cfg, sf.logger), nil
	case "duplicate_check":
		return NewDuplicateCheckStage(sf.cfg, sf.logger), nil
	case "permission_check":
		return NewPermissionCheckStage(sf.cfg, sf.logger), nil
	case "space_check":
		return NewSpaceCheckStage(sf.cfg, sf.logger), nil
	default:
		return nil, fmt.Errorf("unknown stage: %s", name)
	}
}

// ConfigurablePipelineFactory creates a pipeline based on config
func ConfigurablePipelineFactory(
	cfg *config.Config,
	logger *utils.Logger,
	mediaRepo *models.MediaRepository,
	notifiers []notifications.Notifier,
	subtitleClient *subtitles.Client,
) (*Pipeline, error) {
	factory := NewStageFactory(cfg, logger, mediaRepo, notifiers, subtitleClient)

	var stages []Stage
	for _, stageCfg := range cfg.PostProcessing.Pipeline.Stages {
		if !stageCfg.Enabled {
			logger.Info(fmt.Sprintf("Stage '%s' disabled, skipping", stageCfg.Name))
			continue
		}

		stage, err := factory.CreateStage(stageCfg.Name)
		if err != nil {
			return nil, fmt.Errorf("failed to create stage '%s': %w", stageCfg.Name, err)
		}

		// Wrap stage with condition if specified
		if stageCfg.Condition != "" {
			stage = NewConditionalStage(stage, stageCfg.Condition, logger)
		}

		stages = append(stages, stage)
		logger.Info(fmt.Sprintf("Registered stage: %s", stageCfg.Name))
	}

	if len(stages) == 0 {
		return nil, fmt.Errorf("no stages configured")
	}

	pipeline := NewPipeline(cfg, logger, stages)

	// Initialize state manager if enabled
	if cfg.PostProcessing.Enabled {
		stateManager, err := NewStateManager(cfg.App.DataPath)
		if err != nil {
			logger.Warn("Failed to initialize state manager:", err)
		} else {
			pipeline.SetStateManager(stateManager)
			logger.Info("State manager initialized for recovery support")
		}
	}

	return pipeline, nil
}

// DefaultPipelineFactory creates the default pipeline with all stages
func DefaultPipelineFactory(
	cfg *config.Config,
	logger *utils.Logger,
	mediaRepo *models.MediaRepository,
	notifiers []notifications.Notifier,
	subtitleClient *subtitles.Client,
) *Pipeline {
	stages := []Stage{
		NewValidateStage(cfg, logger),
		NewCreateFoldersStage(cfg, logger),
		NewMoveFilesStage(cfg, logger),
		NewRenameStage(cfg, logger),
		NewSubtitlesStage(cfg, logger, subtitleClient),
		NewNotifyStage(logger, notifiers),
	}

	return NewPipeline(cfg, logger, stages)
}
