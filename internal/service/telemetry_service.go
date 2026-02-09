package service

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"CampusMonitorAPI/internal/logger"
	"CampusMonitorAPI/internal/models"
	"CampusMonitorAPI/internal/repository"
)

type TelemetryService struct {
	telemetryRepo *repository.TelemetryRepository
	probeRepo     *repository.ProbeRepository
	log           *logger.Logger
}

func NewTelemetryService(
	telemetryRepo *repository.TelemetryRepository,
	probeRepo *repository.ProbeRepository,
	log *logger.Logger,
) *TelemetryService {
	return &TelemetryService{
		telemetryRepo: telemetryRepo,
		probeRepo:     probeRepo,
		log:           log,
	}
}

func (s *TelemetryService) ProcessMessage(ctx context.Context, payload []byte) error {
	s.log.Debug("Processing telemetry message: %d bytes", len(payload))

	var rawData map[string]interface{}
	if err := json.Unmarshal(payload, &rawData); err != nil {
		s.log.Error("Failed to unmarshal telemetry: %v", err)
		return fmt.Errorf("invalid JSON: %w", err)
	}

	probeID, ok := rawData["pid"].(string)
	if !ok {
		return fmt.Errorf("missing probe_id")
	}

	// Auto-register unknown probes
	_, err := s.probeRepo.GetByID(ctx, probeID)
	if err != nil {
		s.log.Info("Unknown probe detected: %s, auto-registering", probeID)

		probe := &models.Probe{
			ProbeID:         probeID,
			Location:        "Unknown",
			Building:        "Unknown",
			Floor:           "Unknown",
			Department:      "Unknown",
			Status:          "unknown",
			FirmwareVersion: "unknown",
			LastSeen:        time.Now(),
		}

		if createErr := s.probeRepo.Create(ctx, probe); createErr != nil {
			s.log.Error("Failed to auto-register probe: %v", createErr)
		} else {
			s.log.Info("Auto-registered probe: %s with status 'unknown'", probeID)
		}
	}

	telemetryType, ok := rawData["type"].(string)
	if !ok {
		return fmt.Errorf("missing or invalid 'type' field")
	}

	var telemetry *models.Telemetry
	var parseErr error

	switch telemetryType {
	case "light":
		telemetry, parseErr = s.parseLightTelemetry(rawData)
	case "enhanced":
		telemetry, parseErr = s.parseEnhancedTelemetry(rawData)
	default:
		return fmt.Errorf("unknown telemetry type: %s", telemetryType)
	}

	if parseErr != nil {
		s.log.Error("Failed to parse telemetry: %v", parseErr)
		return parseErr
	}

	telemetry.ReceivedAt = time.Now()

	if err := s.telemetryRepo.Insert(ctx, telemetry); err != nil {
		s.log.Error("Failed to insert telemetry: %v", err)
		return err
	}

	s.log.Info("Telemetry stored: probe=%s, type=%s, rssi=%v",
		telemetry.ProbeID, telemetry.Type, telemetry.RSSI)

	if err := s.probeRepo.UpdateLastSeen(ctx, telemetry.ProbeID, telemetry.Timestamp); err != nil {
		s.log.Warn("Failed to update probe last_seen: %v", err)
	}

	return nil
}

func (s *TelemetryService) parseLightTelemetry(data map[string]interface{}) (*models.Telemetry, error) {
	probeID, ok := data["pid"].(string)
	if !ok {
		return nil, fmt.Errorf("missing probe_id")
	}

	epoch, ok := data["epoch"].(float64)
	if !ok {
		return nil, fmt.Errorf("missing epoch timestamp")
	}

	timestamp := time.Unix(int64(epoch), 0)

	telemetry := &models.Telemetry{
		Timestamp: timestamp,
		ProbeID:   probeID,
		Type:      "light",
	}

	if val, ok := data["rssi"].(float64); ok {
		rssi := int(val)
		telemetry.RSSI = &rssi
	}

	if val, ok := data["lat"].(float64); ok {
		latency := int(val)
		telemetry.Latency = &latency
	}

	if val, ok := data["loss"].(float64); ok {
		telemetry.PacketLoss = &val
	}

	if val, ok := data["dns"].(float64); ok {
		dns := int(val)
		telemetry.DNSTime = &dns
	}

	if val, ok := data["ch"].(float64); ok {
		channel := int(val)
		telemetry.Channel = &channel
	}

	if val, ok := data["cong"].(float64); ok {
		congestion := int(val)
		telemetry.Congestion = &congestion
	}

	if val, ok := data["bssid"].(string); ok {
		telemetry.BSSID = &val
	}

	if val, ok := data["neighbors"].(float64); ok {
		neighbors := int(val)
		telemetry.Neighbors = &neighbors
	}

	if val, ok := data["overlap"].(float64); ok {
		overlap := int(val)
		telemetry.Overlap = &overlap
	}

	return telemetry, nil
}

func (s *TelemetryService) parseEnhancedTelemetry(data map[string]interface{}) (*models.Telemetry, error) {
	telemetry, err := s.parseLightTelemetry(data)
	if err != nil {
		return nil, err
	}

	telemetry.Type = "enhanced"

	if val, ok := data["snr"].(float64); ok {
		telemetry.SNR = &val
	}

	if val, ok := data["qual"].(float64); ok {
		telemetry.LinkQuality = &val
	}

	if val, ok := data["util"].(float64); ok {
		telemetry.Utilization = &val
	}

	if val, ok := data["phy"].(string); ok {
		telemetry.PhyMode = &val
	}

	if val, ok := data["tput"].(float64); ok {
		throughput := int(val)
		telemetry.Throughput = &throughput
	}

	if val, ok := data["noise"].(float64); ok {
		noise := int(val)
		telemetry.NoiseFloor = &noise
	}

	if val, ok := data["up"].(float64); ok {
		uptime := int(val)
		telemetry.Uptime = &uptime
	}

	return telemetry, nil
}

func (s *TelemetryService) GetTelemetry(ctx context.Context, req *models.TelemetryQueryRequest) (*models.TelemetryQueryResponse, error) {
	s.log.Debug("Querying telemetry: probes=%v, type=%s", req.ProbeIDs, req.Type)

	data, totalCount, err := s.telemetryRepo.Query(ctx, req)
	if err != nil {
		return nil, err
	}

	response := &models.TelemetryQueryResponse{
		Data:       data,
		TotalCount: totalCount,
		Limit:      req.Limit,
		Offset:     req.Offset,
	}

	s.log.Debug("Query returned %d records (total: %d)", len(data), totalCount)

	return response, nil
}

func (s *TelemetryService) GetProbeStats(ctx context.Context, probeID string, hours int) ([]models.StatsResponse, error) {
	s.log.Debug("Getting stats for probe %s (last %d hours)", probeID, hours)

	stats, err := s.telemetryRepo.GetHourlyStats(ctx, probeID, hours)
	if err != nil {
		return nil, err
	}

	return stats, nil
}

func (s *TelemetryService) GetLatestTelemetry(ctx context.Context, probeID string, limit int) ([]models.Telemetry, error) {
	return s.telemetryRepo.GetLatest(ctx, probeID, limit)
}
