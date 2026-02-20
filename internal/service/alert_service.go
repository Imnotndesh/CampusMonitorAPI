package service

import (
	"CampusMonitorAPI/internal/logger"
	"context"
	"fmt"
	"time"

	"CampusMonitorAPI/internal/models"
	"CampusMonitorAPI/internal/repository"
	"CampusMonitorAPI/internal/websocket"
)

// IAlertService defines the business logic for handling alerts.
type IAlertService interface {
	Dispatch(ctx context.Context, alert *models.Alert) error
	Acknowledge(ctx context.Context, id uint) error
	Resolve(ctx context.Context, id uint) error
	DeleteAlert(ctx context.Context, id uint) error
	GetActiveAlerts(ctx context.Context) ([]models.Alert, error)
	GetProbeAlerts(ctx context.Context, probeID string) ([]models.Alert, error)
	GetAlertHistory(ctx context.Context, limit, offset int) ([]models.Alert, error)
	SendTestAlert(ctx context.Context) error
}

type AlertService struct {
	repo repository.IAlertRepository
	hub  *websocket.Hub // Added WebSocket Hub for real-time dispatch
}

func NewAlertService(repo repository.IAlertRepository, hub *websocket.Hub) *AlertService {
	return &AlertService{
		repo: repo,
		hub:  hub,
	}
}

// Dispatch handles the "One-Shot" transition from a detected pattern to a stored/notified event.
func (s *AlertService) Dispatch(ctx context.Context, alert *models.Alert) error {
	err := s.repo.Create(ctx, alert)
	if err != nil {
		return fmt.Errorf("failed to persist alert history: %w", err)
	}
	s.notify(alert)
	if alert.Severity == models.SeverityCritical {
		fmt.Printf("[CRITICAL ALERT] %s: %s (Probe: %s)\n",
			alert.Category, alert.Message, alert.ProbeID)
	}

	return nil
}

// Acknowledge marks an alert as "Read" by the user.
func (s *AlertService) Acknowledge(ctx context.Context, id uint) error {
	err := s.repo.Acknowledge(ctx, id)
	if err != nil {
		return fmt.Errorf("failed to acknowledge alert %d: %w", id, err)
	}
	return nil
}

// Resolve marks the underlying network issue as fixed.
func (s *AlertService) Resolve(ctx context.Context, id uint) error {
	err := s.repo.Resolve(ctx, id)
	if err != nil {
		return fmt.Errorf("failed to resolve alert %d: %w", id, err)
	}
	return nil
}

// DeleteAlert removes the alert from the system.
func (s *AlertService) DeleteAlert(ctx context.Context, id uint) error {
	return s.repo.Delete(ctx, id)
}

// GetActiveAlerts retrieves all alerts that haven't been resolved yet.
func (s *AlertService) GetActiveAlerts(ctx context.Context) ([]models.Alert, error) {
	return s.repo.GetHistory(ctx, 100, 0)
}

// GetProbeAlerts fetches current issues for a specific campus probe.
func (s *AlertService) GetProbeAlerts(ctx context.Context, probeID string) ([]models.Alert, error) {
	return s.repo.GetActiveByProbe(ctx, probeID)
}

// GetAlertHistory provides the full audit trail for reporting.
func (s *AlertService) GetAlertHistory(ctx context.Context, limit, offset int) ([]models.Alert, error) {
	return s.repo.GetHistory(ctx, limit, offset)
}

// notify handles the actual transmission of the alert to connected clients via WebSockets.
func (s *AlertService) notify(alert *models.Alert) {
	if s.hub != nil {
		s.hub.Broadcast("ALERT", alert)
	}
}

// CleanUpTask can be run as a background cron to remove ancient resolved alerts.
func (s *AlertService) CleanUpTask(ctx context.Context) {
	count, err := s.repo.DeleteOld(ctx, 30*24*time.Hour)
	if err == nil && count > 0 {
		fmt.Printf("[CLEANUP] Removed %d old resolved alerts from history\n", count)
	}
}
func (s *AlertService) SendTestAlert(ctx context.Context) error {
	testAlert := &models.Alert{
		ProbeID:   "TEST-PROBE-01",
		Category:  models.CategorySystem,
		Severity:  models.SeverityInfo,
		MetricKey: "simulation",
		Message:   "Simulation: This is an ephemeral test notification (not saved to DB).",
		Status:    models.StatusActive,
		CreatedAt: time.Now(),
	}
	s.notify(testAlert)
	logger.Info("Ephemeral test alert broadcasted to WebSocket hub.")
	return nil
}
