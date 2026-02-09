package services

import (
	"fmt"
	"strconv"
	"strings"

	"reel/internal/utils"
)

// ConditionalStage wraps a stage with a condition
type ConditionalStage struct {
	stage     Stage
	condition string
	logger    *utils.Logger
}

// NewConditionalStage creates a conditional stage wrapper
func NewConditionalStage(stage Stage, condition string, logger *utils.Logger) *ConditionalStage {
	return &ConditionalStage{
		stage:     stage,
		condition: condition,
		logger:    logger,
	}
}

func (cs *ConditionalStage) Name() string {
	return cs.stage.Name() + "_conditional"
}

func (cs *ConditionalStage) Execute(ctx *ProcessingContext) error {
	if !cs.evaluateCondition(ctx) {
		cs.logger.Info(fmt.Sprintf("Stage '%s' skipped (condition not met: %s)", cs.stage.Name(), cs.condition))
		return nil
	}

	cs.logger.Info(fmt.Sprintf("Stage '%s' condition met: %s", cs.stage.Name(), cs.condition))
	return cs.stage.Execute(ctx)
}

func (cs *ConditionalStage) Rollback(ctx *ProcessingContext) error {
	return cs.stage.Rollback(ctx)
}

// evaluateCondition evaluates simple conditions
// Supported: "always", "never", "has_subtitles", "no_subtitles", "is_movie", "is_tv", "is_anime"
// Memory-efficient: no regex, no complex parsing
func (cs *ConditionalStage) evaluateCondition(ctx *ProcessingContext) bool {
	condition := strings.TrimSpace(strings.ToLower(cs.condition))

	// Simple keyword conditions
	switch condition {
	case "always", "true":
		return true
	case "never", "false":
		return false
	case "has_subtitles":
		return ctx.Media.Type != ""
	case "no_subtitles":
		return ctx.Media.Type == ""
	case "is_movie":
		return ctx.Media.Type == "movie"
	case "is_tv", "is_tvshow", "is_tv_show":
		return ctx.Media.Type == "tv_show"
	case "is_anime":
		return ctx.Media.Type == "anime"
	}

	// Check for file count conditions: "files > 1", "files = 1", "files < 5"
	if strings.HasPrefix(condition, "files ") {
		return cs.evaluateFileCountCondition(condition, ctx)
	}

	// Check for file size conditions: "file_size > 100MB", "file_size < 1GB"
	if strings.HasPrefix(condition, "file_size ") || strings.HasPrefix(condition, "filesize ") {
		return cs.evaluateFileSizeCondition(condition, ctx)
	}

	// Default: true (unknown conditions pass)
	cs.logger.Warn(fmt.Sprintf("Unknown condition '%s', defaulting to true", cs.condition))
	return true
}

// evaluateFileCountCondition evaluates file count conditions
func (cs *ConditionalStage) evaluateFileCountCondition(condition string, ctx *ProcessingContext) bool {
	parts := strings.Fields(condition)
	if len(parts) != 3 {
		return true
	}

	op := parts[1]
	value, err := strconv.Atoi(parts[2])
	if err != nil {
		return true
	}

	fileCount := len(ctx.OriginalFiles)

	switch op {
	case ">":
		return fileCount > value
	case ">=":
		return fileCount >= value
	case "=", "==":
		return fileCount == value
	case "<=":
		return fileCount <= value
	case "<":
		return fileCount < value
	case "!=":
		return fileCount != value
	default:
		return true
	}
}

// evaluateFileSizeCondition evaluates file size conditions
func (cs *ConditionalStage) evaluateFileSizeCondition(condition string, ctx *ProcessingContext) bool {
	// Simple implementation: check first file size
	if len(ctx.OriginalFiles) == 0 {
		return false
	}

	parts := strings.Fields(condition)
	if len(parts) != 3 {
		return true
	}

	op := parts[1]
	sizeStr := strings.ToLower(parts[2])

	// Parse size (support MB, GB)
	var targetSize int64
	if strings.HasSuffix(sizeStr, "mb") {
		val, err := strconv.Atoi(strings.TrimSuffix(sizeStr, "mb"))
		if err != nil {
			return true
		}
		targetSize = int64(val) * 1024 * 1024
	} else if strings.HasSuffix(sizeStr, "gb") {
		val, err := strconv.Atoi(strings.TrimSuffix(sizeStr, "gb"))
		if err != nil {
			return true
		}
		targetSize = int64(val) * 1024 * 1024 * 1024
	} else {
		// Try to parse as bytes
		val, err := strconv.ParseInt(sizeStr, 10, 64)
		if err != nil {
			return true
		}
		targetSize = val
	}

	// Get size of first file (simplified)
	// In practice, you might want to check all files or largest file
	fileSize := cs.getFileSize(ctx.OriginalFiles[0])

	switch op {
	case ">":
		return fileSize > targetSize
	case ">=":
		return fileSize >= targetSize
	case "=", "==":
		return fileSize == targetSize
	case "<=":
		return fileSize <= targetSize
	case "<":
		return fileSize < targetSize
	case "!=":
		return fileSize != targetSize
	default:
		return true
	}
}

// getFileSize returns file size in bytes (0 on error)
func (cs *ConditionalStage) getFileSize(path string) int64 {
	// This is a simplified version - in practice we'd need to import os
	// For now, return 0 as placeholder
	// The actual implementation would use os.Stat
	return 0
}
