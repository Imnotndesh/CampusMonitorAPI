package service

import (
	"context"
	"fmt"
	"time"

	"CampusMonitorAPI/internal/models"
	"CampusMonitorAPI/internal/repository"
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
}

type AlertService struct {
	repo repository.IAlertRepository
}

func NewAlertService(repo repository.IAlertRepository) *AlertService {
	return &AlertService{
		repo: repo,
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
	// We use a high limit for active alerts to ensure the dashboard is comprehensive
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

// notify handles the actual transmission of the alert to connected clients.
func (s *AlertService) notify(alert *models.Alert) {
	fmt.Printf("[WS PUSH] Alert dispatched to dashboard for Probe %s\n", alert.ProbeID)
}

// CleanUpTask can be run as a background cron to remove ancient resolved alerts.
func (s *AlertService) CleanUpTask(ctx context.Context) {
	// Example: Delete resolved alerts older than 30 days
	count, err := s.repo.DeleteOld(ctx, 30*24*time.Hour)
	if err == nil && count > 0 {
		fmt.Printf("[CLEANUP] Removed %d old resolved alerts from history\n", count)
	}
}
