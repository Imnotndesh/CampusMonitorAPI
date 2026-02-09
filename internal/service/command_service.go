// internal/service/command_service.go (Updated)

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

	var err error

	switch req.CommandType {
	case "deep_scan":
		duration := 2
		if d, ok := req.Payload["duration"].(float64); ok {
			duration = int(d)
		}
		err = s.mqttClient.SendDeepScan(req.ProbeID, duration)

	case "config_update":
		config := make(map[string]interface{})
		if ri, ok := req.Payload["report_interval"].(float64); ok {
			config["report_interval"] = int(ri)
		}
		if srv, ok := req.Payload["mqtt_server"].(string); ok {
			config["mqtt_server"] = srv
		}
		if port, ok := req.Payload["mqtt_port"].(float64); ok {
			config["mqtt_port"] = int(port)
		}
		if topic, ok := req.Payload["telemetry_topic"].(string); ok {
			config["telemetry_topic"] = topic
		}
		err = s.mqttClient.SendConfigUpdate(req.ProbeID, config)

	case "restart":
		delay := 2000
		if d, ok := req.Payload["delay"].(float64); ok {
			delay = int(d)
		}
		err = s.mqttClient.SendRestart(req.ProbeID, delay)

	case "ota_update":
		url, _ := req.Payload["url"].(string)
		version, _ := req.Payload["version"].(string)
		if url == "" || version == "" {
			err = fmt.Errorf("ota_update requires url and version")
		} else {
			err = s.mqttClient.SendOTAUpdate(req.ProbeID, url, version)
		}

	case "factory_reset":
		err = s.mqttClient.SendRawCommand(req.ProbeID, "factory_reset", map[string]interface{}{})

	case "ping":
		err = s.mqttClient.SendPing(req.ProbeID)

	case "get_status":
		err = s.mqttClient.SendGetStatus(req.ProbeID)

	default:
		s.log.Info("Sending custom command: %s", req.CommandType)
		err = s.mqttClient.SendRawCommand(req.ProbeID, req.CommandType, req.Payload)
	}

	if err != nil {
		s.log.Error("Failed to send command via MQTT: %v", err)
		err := s.commandRepo.UpdateStatus(ctx, cmd.ID, "failed", map[string]interface{}{"error": err.Error()})
		if err != nil {
			return nil, err
		}
		return nil, fmt.Errorf("failed to send command: %w", err)
	}

	err = s.commandRepo.UpdateStatus(ctx, cmd.ID, "sent", nil)
	if err != nil {
		return nil, err
	}
	s.log.Info("Command sent successfully: id=%d, type=%s, probe=%s", cmd.ID, req.CommandType, req.ProbeID)

	return cmd, nil
}

func (s *CommandService) GetCommandHistory(ctx context.Context, probeID string) ([]models.Command, error) {
	s.log.Debug("Fetching command history for probe: %s", probeID)
	return s.commandRepo.GetByProbeID(ctx, probeID, 50)
}

func (s *CommandService) GetPendingCommands(ctx context.Context) ([]models.Command, error) {
	s.log.Debug("Fetching pending commands")
	return s.commandRepo.GetPending(ctx)
}

func (s *CommandService) ProcessCommandResult(ctx context.Context, commandID int, result map[string]interface{}) error {
	s.log.Info("Processing command result: id=%d", commandID)

	status := "completed"
	if success, ok := result["success"].(bool); ok && !success {
		status = "failed"
	}

	return s.commandRepo.UpdateStatus(ctx, commandID, status, result)
}

func (s *CommandService) BroadcastCommand(ctx context.Context, commandType string, params map[string]interface{}) error {
	s.log.Info("Broadcasting command: type=%s", commandType)
	cmd := &models.Command{
		ProbeID:     "broadcast",
		CommandType: commandType,
		Payload:     params,
		Status:      "pending",
	}

	if err := s.commandRepo.Create(ctx, cmd); err != nil {
		s.log.Error("Failed to create broadcast command: %v", err)
		return err
	}

	if err := s.mqttClient.BroadcastCommand(commandType, params); err != nil {
		s.log.Error("Failed to broadcast command: %v", err)
		err := s.commandRepo.UpdateStatus(ctx, cmd.ID, "failed", map[string]interface{}{"error": err.Error()})
		if err != nil {
			return err
		}
		return err
	}

	err := s.commandRepo.UpdateStatus(ctx, cmd.ID, "sent", nil)
	if err != nil {
		return err
	}
	s.log.Info("Broadcast command sent successfully: id=%d, type=%s", cmd.ID, commandType)

	return nil
}

func (s *CommandService) GetCommandStatistics(ctx context.Context) (map[string]interface{}, error) {
	s.log.Debug("Fetching command statistics")
	stats, err := s.commandRepo.GetStatistics(ctx)
	if err != nil {
		s.log.Error("Failed to get command stats: %v", err)
		return nil, err
	}
	result := make(map[string]interface{})
	total := 0
	for k, v := range stats {
		result[k] = v
		total += v
	}
	result["total"] = total

	return result, nil
}
func (s *CommandService) DeleteOldCommands(ctx context.Context, days int) (int, error) {
	s.log.Info("Deleting commands older than %d days", days)

	count, err := s.commandRepo.DeleteOld(ctx, days)
	if err != nil {
		s.log.Error("Failed to cleanup old commands: %v", err)
		return 0, err
	}

	s.log.Info("Deleted %d old commands", count)
	return int(count), nil
}
