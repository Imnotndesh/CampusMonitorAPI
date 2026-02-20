package service

import (
	"CampusMonitorAPI/internal/service"
	"context"
	"fmt"
	"sync"

	"CampusMonitorAPI/internal/models"
)

// MetricWindow maintains a sliding buffer of recent telemetry values.
type MetricWindow struct {
	values []float64
	size   int
}

func NewMetricWindow(size int) *MetricWindow {
	// If size is 0 or 1, we treat it as 1 to avoid division/indexing issues
	if size < 1 {
		size = 1
	}
	return &MetricWindow{
		values: make([]float64, 0, size),
		size:   size,
	}
}

// Push adds a new value to the window and slides it if full.
func (w *MetricWindow) Push(val float64) {
	if len(w.values) >= w.size {
		w.values = w.values[1:]
	}
	w.values = append(w.values, val)
}

// IsConsistentlyBelow returns true if every value in the window is less than the threshold.
func (w *MetricWindow) IsConsistentlyBelow(threshold float64) bool {
	if len(w.values) < w.size {
		return false
	}
	for _, v := range w.values {
		if v >= threshold {
			return false
		}
	}
	return true
}

// IsConsistentlyAbove returns true if every value in the window is greater than the threshold.
func (w *MetricWindow) IsConsistentlyAbove(threshold float64) bool {
	if len(w.values) < w.size {
		return false
	}
	for _, v := range w.values {
		if v <= threshold {
			return false
		}
	}
	return true
}

// ProbeState tracks the performance windows for a specific probe.
type ProbeState struct {
	RSSIWindow    *MetricWindow
	LatencyWindow *MetricWindow
}

// IAlertEvaluator defines the interface for analyzing telemetry in real-time.
type IAlertEvaluator interface {
	Evaluate(ctx context.Context, telemetry models.Telemetry) error
	UpdateConfig(newCfg models.AlertConfig)
	ResetProbe(probeID string)
}

type AlertEvaluator struct {
	config       models.AlertConfig
	probeStates  map[string]*ProbeState
	alertService service.IAlertService
	mu           sync.RWMutex
}

func NewAlertEvaluator(cfg models.AlertConfig, alertSvc service.IAlertService) *AlertEvaluator {
	return &AlertEvaluator{
		config:       cfg,
		probeStates:  make(map[string]*ProbeState),
		alertService: alertSvc,
	}
}

// Evaluate processes incoming telemetry through the sliding windows.
func (e *AlertEvaluator) Evaluate(ctx context.Context, telemetry models.Telemetry) error {
	e.mu.Lock()
	state, exists := e.probeStates[telemetry.ProbeID]
	if !exists {
		state = &ProbeState{
			RSSIWindow:    NewMetricWindow(e.config.RSSIOccurrences),
			LatencyWindow: NewMetricWindow(e.config.LatencyWindow),
		}
		e.probeStates[telemetry.ProbeID] = state
	}
	e.mu.Unlock()

	state.RSSIWindow.Push(float64(*telemetry.RSSI))
	if state.RSSIWindow.IsConsistentlyBelow(e.config.RSSIThreshold) {
		err := e.dispatch(ctx, telemetry, models.CategorySignal, models.SeverityWarning,
			"rssi", e.config.RSSIThreshold, float64(*telemetry.RSSI),
			fmt.Sprintf("Sustained Low Signal: %d consecutive samples below %.0fdBm",
				e.config.RSSIOccurrences, e.config.RSSIThreshold))
		if err != nil {
			return err
		}
	}

	state.LatencyWindow.Push(float64(*telemetry.Latency))
	if state.LatencyWindow.IsConsistentlyAbove(e.config.LatencyThreshold) {
		err := e.dispatch(ctx, telemetry, models.CategoryNetwork, models.SeverityCritical,
			"latency", e.config.LatencyThreshold, float64(*telemetry.Latency),
			fmt.Sprintf("High Network Latency: %d consecutive samples above %.0fms",
				e.config.LatencyWindow, e.config.LatencyThreshold))
		if err != nil {
			return err
		}
	}

	return nil
}

// dispatch creates the Alert object and hands it to the AlertService for WS push and storage.
func (e *AlertEvaluator) dispatch(ctx context.Context, t models.Telemetry, cat, sev, key string, thresh, actual float64, msg string) error {
	alert := &models.Alert{
		ProbeID:        t.ProbeID,
		Category:       cat,
		Severity:       sev,
		MetricKey:      key,
		ThresholdValue: thresh,
		ActualValue:    actual,
		Message:        msg,
		Status:         models.StatusActive,
		Occurrences:    e.config.RSSIOccurrences,
	}

	// Persist and Notify via WebSockets
	return e.alertService.Dispatch(ctx, alert)
}

// UpdateConfig allows the client to change parameters at runtime.
func (e *AlertEvaluator) UpdateConfig(newCfg models.AlertConfig) {
	e.mu.Lock()
	defer e.mu.Unlock()
	if newCfg.RSSIOccurrences != e.config.RSSIOccurrences ||
		newCfg.LatencyWindow != e.config.LatencyWindow {
		e.probeStates = make(map[string]*ProbeState)
	}

	e.config = newCfg
}

// ResetProbe clears the in-memory state for a probe (e.g., after maintenance).
func (e *AlertEvaluator) ResetProbe(probeID string) {
	e.mu.Lock()
	defer e.mu.Unlock()
	delete(e.probeStates, probeID)
}
