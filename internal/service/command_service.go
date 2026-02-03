// internal/service/command_service.go

package service

import (
	"context"
	"fmt"

	"CampusMonitorAPI/internal/logger"
	"CampusMonitorAPI/internal/models"
	"CampusMonitorAPI/internal/mqtt"
	"CampusMonitorAPI/internal/repository"
)

type CommandService struct {
	commandRepo *repository.CommandRepository
	mqttClient  *mqtt.Client
	log         *logger.Logger
}

func NewCommandService(
	commandRepo *repository.CommandRepository,
	mqttClient *mqtt.Client,
	log *logger.Logger,
) *CommandService {
	return &CommandService{
		commandRepo: commandRepo,
		mqttClient:  mqttClient,
		log:         log,
	}
}

func (s *CommandService) IssueCommand(ctx context.Context, req *models.CommandRequest) (*models.Command, error) {
	s.log.Info("Issuing command: type=%s, probe=%s", req.CommandType, req.ProbeID)

	cmd := &models.Command{
		ProbeID:     req.ProbeID,
		CommandType: req.CommandType,
		Payload:     req.Payload,
		Status:      "pending",
	}

	if err := s.commandRepo.Create(ctx, cmd); err != nil {
		s.log.Error("Failed to create command: %v", err)
		return nil, err
	}

	switch req.CommandType {
	case "deep_scan":
		duration := 2
		if d, ok := req.Payload["duration"].(float64); ok {
			duration = int(d)
		}
		if err := s.mqttClient.SendDeepScan(req.ProbeID, duration); err != nil {
			s.log.Error("Failed to send deep scan command: %v", err)
			s.commandRepo.UpdateStatus(ctx, cmd.ID, "failed", map[string]interface{}{"error": err.Error()})
			return nil, err
		}

	case "config_update":
		config := mqtt.ConfigUpdateCommand{}
		if ri, ok := req.Payload["report_interval"].(float64); ok {
			config.ReportInterval = int(ri)
		}
		if srv, ok := req.Payload["mqtt_server"].(string); ok {
			config.MQTTServer = srv
		}
		if port, ok := req.Payload["mqtt_port"].(float64); ok {
			config.MQTTPort = int(port)
		}
		if err := s.mqttClient.SendConfigUpdate(req.ProbeID, config); err != nil {
			s.log.Error("Failed to send config update: %v", err)
			s.commandRepo.UpdateStatus(ctx, cmd.ID, "failed", map[string]interface{}{"error": err.Error()})
			return nil, err
		}

	case "restart":
		if err := s.mqttClient.SendRestart(req.ProbeID); err != nil {
			s.log.Error("Failed to send restart command: %v", err)
			s.commandRepo.UpdateStatus(ctx, cmd.ID, "failed", map[string]interface{}{"error": err.Error()})
			return nil, err
		}

	case "ota_update":
		url, _ := req.Payload["url"].(string)
		version, _ := req.Payload["version"].(string)
		if url == "" || version == "" {
			return nil, fmt.Errorf("ota_update requires url and version")
		}
		if err := s.mqttClient.SendOTAUpdate(req.ProbeID, url, version); err != nil {
			s.log.Error("Failed to send OTA update: %v", err)
			s.commandRepo.UpdateStatus(ctx, cmd.ID, "failed", map[string]interface{}{"error": err.Error()})
			return nil, err
		}

	default:
		return nil, fmt.Errorf("unknown command type: %s", req.CommandType)
	}

	s.commandRepo.UpdateStatus(ctx, cmd.ID, "sent", nil)
	s.log.Info("Command sent successfully: id=%d", cmd.ID)

	return cmd, nil
}

func (s *CommandService) GetCommandHistory(ctx context.Context, probeID string) ([]models.Command, error) {
	return s.commandRepo.GetByProbeID(ctx, probeID, 50)
}

func (s *CommandService) ProcessCommandResult(ctx context.Context, commandID int, result map[string]interface{}) error {
	s.log.Info("Processing command result: id=%d", commandID)

	status := "completed"
	if success, ok := result["success"].(bool); ok && !success {
		status = "failed"
	}

	return s.commandRepo.UpdateStatus(ctx, commandID, status, result)
}

func (s *CommandService) GetPendingCommands(ctx context.Context) ([]models.Command, error) {
	return s.commandRepo.GetPending(ctx)
}
