// Package util provides utility functions including stage tracking for operations.
package util

import (
	"fmt"
	"strings"
	"sync"
	"time"
)

// StageStatus represents the status of a stage
type StageStatus string

const (
	// StageStatusRunning indicates the stage is currently executing
	StageStatusRunning StageStatus = "running"
	// StageStatusCompleted indicates the stage completed successfully
	StageStatusCompleted StageStatus = "completed"
	// StageStatusFailed indicates the stage failed
	StageStatusFailed StageStatus = "failed"
)

// Stage represents an operation stage with timing and metrics
type Stage struct {
	Name      string                 `json:"name"`
	StartTime time.Time              `json:"start_time"`
	EndTime   *time.Time             `json:"end_time,omitempty"`
	Status    StageStatus            `json:"status"`
	Metrics   map[string]interface{} `json:"metrics,omitempty"`
	Error     error                  `json:"error,omitempty"`
}

// Duration returns the duration of the stage
func (s *Stage) Duration() time.Duration {
	if s.EndTime == nil {
		return time.Since(s.StartTime)
	}
	return s.EndTime.Sub(s.StartTime)
}

// StageTracker tracks operation stages with timing and metrics
type StageTracker struct {
	targetName   string
	currentStage *Stage
	stages       []*Stage
	mu           sync.RWMutex
}

// NewStageTracker creates a new stage tracker for a target
func NewStageTracker(targetName string) *StageTracker {
	return &StageTracker{
		targetName: targetName,
		stages:     make([]*Stage, 0),
	}
}

// StartStage begins a new stage and logs a visual marker
func (st *StageTracker) StartStage(stageName string) {
	st.mu.Lock()
	defer st.mu.Unlock()

	// End current stage if any
	if st.currentStage != nil && st.currentStage.Status == StageStatusRunning {
		now := time.Now()
		st.currentStage.EndTime = &now
		st.currentStage.Status = StageStatusCompleted
	}

	// Create new stage
	stage := &Stage{
		Name:      stageName,
		StartTime: time.Now(),
		Status:    StageStatusRunning,
		Metrics:   make(map[string]interface{}),
	}

	st.currentStage = stage
	st.stages = append(st.stages, stage)

	// Log visual marker
	LogStageMarker(st.targetName, stageName, true)
}

// EndStage completes the current stage with optional metrics and logs a summary
func (st *StageTracker) EndStage(metrics map[string]interface{}) {
	st.mu.Lock()
	defer st.mu.Unlock()

	if st.currentStage == nil {
		return
	}

	// Mark as completed
	now := time.Now()
	st.currentStage.EndTime = &now
	st.currentStage.Status = StageStatusCompleted

	// Merge metrics
	if metrics != nil {
		for k, v := range metrics {
			st.currentStage.Metrics[k] = v
		}
	}

	// Log completion summary
	LogStageSummary(st.targetName, st.currentStage.Name, st.currentStage.Duration(), st.currentStage.Metrics)

	st.currentStage = nil
}

// FailStage marks the current stage as failed with an error
func (st *StageTracker) FailStage(err error) {
	st.mu.Lock()
	defer st.mu.Unlock()

	if st.currentStage == nil {
		return
	}

	// Mark as failed
	now := time.Now()
	st.currentStage.EndTime = &now
	st.currentStage.Status = StageStatusFailed
	st.currentStage.Error = err

	// Log failure
	Error("[%s] Stage %s failed: %v", st.targetName, st.currentStage.Name, err)

	st.currentStage = nil
}

// GetCurrentStage returns the currently running stage (if any)
func (st *StageTracker) GetCurrentStage() *Stage {
	st.mu.RLock()
	defer st.mu.RUnlock()
	return st.currentStage
}

// GetAllStages returns all stages (completed and current)
func (st *StageTracker) GetAllStages() []*Stage {
	st.mu.RLock()
	defer st.mu.RUnlock()

	// Create a copy to avoid race conditions
	result := make([]*Stage, len(st.stages))
	copy(result, st.stages)
	return result
}

// GetCompletedStages returns only completed or failed stages
func (st *StageTracker) GetCompletedStages() []*Stage {
	st.mu.RLock()
	defer st.mu.RUnlock()

	result := make([]*Stage, 0)
	for _, stage := range st.stages {
		if stage.Status != StageStatusRunning {
			result = append(result, stage)
		}
	}
	return result
}

// LogStageMarker outputs a visual stage boundary marker
func LogStageMarker(targetName, stageName string, isStart bool) {
	marker := strings.Repeat("=", 50)
	stageUpper := strings.ToUpper(stageName)

	if isStart {
		Info("[%s] %s", targetName, marker)
		Info("[%s] %s", targetName, stageUpper)
		Info("[%s] %s", targetName, marker)
	} else {
		Info("[%s] %s END", targetName, marker)
	}
}

// LogStageSummary outputs a stage completion summary with metrics
func LogStageSummary(targetName, stageName string, duration time.Duration, metrics map[string]interface{}) {
	Info("[%s] Stage '%s' completed in %v", targetName, stageName, duration)

	if len(metrics) > 0 {
		Info("[%s] Stage metrics:", targetName)
		for key, value := range metrics {
			Info("[%s]   - %s: %v", targetName, key, formatMetricValue(value))
		}
	}
}

// formatMetricValue formats a metric value for display
func formatMetricValue(value interface{}) string {
	switch v := value.(type) {
	case int64:
		// Check if it looks like bytes (common for size metrics)
		if v > 1024 {
			return FormatBytes(v)
		}
		return fmt.Sprintf("%d", v)
	case float64:
		// Check if it's a percentage
		if v >= 0 && v <= 100 {
			return fmt.Sprintf("%.1f%%", v)
		}
		return fmt.Sprintf("%.2f", v)
	case time.Duration:
		return v.String()
	default:
		return fmt.Sprintf("%v", v)
	}
}
