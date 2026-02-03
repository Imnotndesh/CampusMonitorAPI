// internal/service/probe_service.go

package service

import (
	"context"
	"fmt"
	"time"

	"CampusMonitorAPI/internal/logger"
	"CampusMonitorAPI/internal/models"
	"CampusMonitorAPI/internal/repository"
)

type ProbeService struct {
	probeRepo *repository.ProbeRepository
	log       *logger.Logger
}

func NewProbeService(
	probeRepo *repository.ProbeRepository,
	log *logger.Logger,
) *ProbeService {
	return &ProbeService{
		probeRepo: probeRepo,
		log:       log,
	}
}

func (s *ProbeService) RegisterProbe(ctx context.Context, req *models.CreateProbeRequest) (*models.Probe, error) {
	s.log.Info("Registering new probe: %s", req.ProbeID)

	existing, err := s.probeRepo.GetByID(ctx, req.ProbeID)
	if err == nil && existing != nil {
		return nil, fmt.Errorf("probe %s already exists", req.ProbeID)
	}

	probe := &models.Probe{
		ProbeID:         req.ProbeID,
		Location:        req.Location,
		Building:        req.Building,
		Floor:           req.Floor,
		Department:      req.Department,
		Status:          "active",
		FirmwareVersion: req.FirmwareVersion,
		LastSeen:        time.Now(),
		Metadata:        req.Metadata,
	}

	if err := s.probeRepo.Create(ctx, probe); err != nil {
		s.log.Error("Failed to register probe: %v", err)
		return nil, err
	}

	s.log.Info("Probe registered successfully: %s", req.ProbeID)
	return probe, nil
}

func (s *ProbeService) GetProbe(ctx context.Context, probeID string) (*models.Probe, error) {
	return s.probeRepo.GetByID(ctx, probeID)
}

func (s *ProbeService) ListProbes(ctx context.Context) ([]models.Probe, error) {
	return s.probeRepo.GetAll(ctx)
}

func (s *ProbeService) UpdateProbe(ctx context.Context, probeID string, req *models.UpdateProbeRequest) (*models.Probe, error) {
	s.log.Info("Updating probe: %s", probeID)

	if err := s.probeRepo.Update(ctx, probeID, req); err != nil {
		s.log.Error("Failed to update probe: %v", err)
		return nil, err
	}

	probe, err := s.probeRepo.GetByID(ctx, probeID)
	if err != nil {
		return nil, err
	}

	s.log.Info("Probe updated successfully: %s", probeID)
	return probe, nil
}

func (s *ProbeService) DeleteProbe(ctx context.Context, probeID string) error {
	s.log.Warn("Deleting probe: %s", probeID)

	if err := s.probeRepo.Delete(ctx, probeID); err != nil {
		s.log.Error("Failed to delete probe: %v", err)
		return err
	}

	s.log.Info("Probe deleted successfully: %s", probeID)
	return nil
}

func (s *ProbeService) GetActiveProbes(ctx context.Context) ([]models.Probe, error) {
	return s.probeRepo.GetActive(ctx)
}

func (s *ProbeService) GetProbesByBuilding(ctx context.Context, building string) ([]models.Probe, error) {
	return s.probeRepo.GetByBuilding(ctx, building)
}

func (s *ProbeService) CheckStaleProbes(ctx context.Context, threshold time.Duration) ([]models.Probe, error) {
	s.log.Debug("Checking for stale probes (threshold: %v)", threshold)

	staleProbes, err := s.probeRepo.GetStale(ctx, threshold)
	if err != nil {
		return nil, err
	}

	if len(staleProbes) > 0 {
		s.log.Warn("Found %d stale probes", len(staleProbes))
	}

	return staleProbes, nil
}

func (s *ProbeService) UpdateFirmwareVersion(ctx context.Context, probeID, version string) error {
	s.log.Info("Updating firmware version for probe %s: %s", probeID, version)

	if err := s.probeRepo.UpdateFirmwareVersion(ctx, probeID, version); err != nil {
		s.log.Error("Failed to update firmware version: %v", err)
		return err
	}

	return nil
}
