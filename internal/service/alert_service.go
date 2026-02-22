package service

import (
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
	hub  *websocket.Hub
}

func NewAlertService(repo repository.IAlertRepository, hub *websocket.Hub) *AlertService {
	return &AlertService{
		repo: repo,
		hub:  hub,
	}
}

func (s *AlertService) Dispatch(ctx context.Context, alert *models.Alert) error {
	err := s.repo.Create(ctx, alert)
	if err != nil {
		return fmt.Errorf("failed to persist alert history: %w", err)
	}
	s.notify(alert)
	return nil
}

func (s *AlertService) Acknowledge(ctx context.Context, id uint) error {
	return s.repo.Acknowledge(ctx, id)
}

func (s *AlertService) Resolve(ctx context.Context, id uint) error {
	return s.repo.Resolve(ctx, id)
}

func (s *AlertService) DeleteAlert(ctx context.Context, id uint) error {
	return s.repo.Delete(ctx, id)
}

func (s *AlertService) GetActiveAlerts(ctx context.Context) ([]models.Alert, error) {
	// FIX: Uses the new repository method specifically for active alerts
	return s.repo.GetActive(ctx)
}

func (s *AlertService) GetProbeAlerts(ctx context.Context, probeID string) ([]models.Alert, error) {
	return s.repo.GetActiveByProbe(ctx, probeID)
}

func (s *AlertService) GetAlertHistory(ctx context.Context, limit, offset int) ([]models.Alert, error) {
	return s.repo.GetHistory(ctx, limit, offset)
}

func (s *AlertService) notify(alert *models.Alert) {
	if s.hub != nil {
		s.hub.Broadcast("ALERT", alert)
	}
}

func (s *AlertService) CleanUpTask(ctx context.Context) {
	count, err := s.repo.DeleteOld(ctx, 30*24*time.Hour)
	if err == nil && count > 0 {
		fmt.Printf("[CLEANUP] Removed %d old resolved alerts from history\n", count)
	}
}

func (s *AlertService) SendTestAlert(ctx context.Context) error {
	// FIX: Replaced 'Category' with 'AlertType' to match the database and struct definition
	testAlert := &models.Alert{
		ID:          int(time.Now().UnixNano() / 1e6),
		ProbeID:     "TEST-PROBE-01",
		AlertType:   "SYSTEM",
		Severity:    "INFO",
		Message:     "Simulation: This is a test alert to verify real-time notifications.",
		TriggeredAt: time.Now(),
	}

	s.notify(testAlert)
	return nil
}
