// internal/service/command_service.go
package service

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"CampusMonitorAPI/internal/logger"
	"CampusMonitorAPI/internal/models"
	"CampusMonitorAPI/internal/mqtt"
	"CampusMonitorAPI/internal/repository"
)

type CommandService struct {
	commandRepo      *repository.CommandRepository
	probeRepo        *repository.ProbeRepository
	telemetryService *TelemetryService
	mqttClient       *mqtt.Client
	log              *logger.Logger
	pingStatus       map[string]bool
	pingStatusMux    sync.RWMutex
}

const StaleThreshold = 60 * time.Second

func NewCommandService(
	commandRepo *repository.CommandRepository,
	mqttClient *mqtt.Client,
	probeRepo *repository.ProbeRepository,
	telemetryService *TelemetryService,
	log *logger.Logger,
) *CommandService {
	return &CommandService{
		commandRepo:      commandRepo,
		mqttClient:       mqttClient,
		probeRepo:        probeRepo,
		telemetryService: telemetryService,
		log:              log,
		pingStatus:       make(map[string]bool),
	}
}

func (s *CommandService) UpdateResultByID(ctx context.Context, commandID int, result map[string]interface{}) error {
	status := "completed"
	err := s.commandRepo.UpdateStatus(ctx, commandID, status, result)
	if err != nil {
		s.log.Error("Failed to update command %d: %v", commandID, err)
		return err
	}

	s.log.Info("Command %d manually updated via API", commandID)
	return nil
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
	if req.CommandType != "ping" {
		checkCtx, cancel := context.WithTimeout(ctx, 6*time.Second)
		defer cancel()

		if err := s.VerifyProbeConnectivity(checkCtx, req.ProbeID); err != nil {
			s.log.Warn("Connectivity check failed for %s: %v", req.ProbeID, err)
			return nil, fmt.Errorf("cannot send %s: %v", req.CommandType, err)
		}
	}

	switch req.CommandType {
	case "deep_scan":
		duration := 5
		if d, ok := req.Payload["duration"].(float64); ok {
			duration = int(d)
		}
		err = s.mqttClient.SendDeepScan(req.ProbeID, cmd.ID, duration)

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
		err = s.mqttClient.SendConfigUpdate(req.ProbeID, cmd.ID, config)

	case "get_config":
		err = s.mqttClient.SendGetConfig(req.ProbeID, cmd.ID)

	case "set_wifi":
		ssid, _ := req.Payload["ssid"].(string)
		password, _ := req.Payload["password"].(string)
		if ssid == "" || password == "" {
			err = fmt.Errorf("set_wifi requires ssid and password")
		} else {
			err = s.mqttClient.SendSetWifi(req.ProbeID, cmd.ID, ssid, password)
		}

	case "set_mqtt":
		broker, _ := req.Payload["broker"].(string)
		port := 1883
		if p, ok := req.Payload["port"].(float64); ok {
			port = int(p)
		}
		user, _ := req.Payload["user"].(string)
		password, _ := req.Payload["password"].(string)

		if broker == "" {
			err = fmt.Errorf("set_mqtt requires broker")
		} else {
			err = s.mqttClient.SendSetMqtt(req.ProbeID, cmd.ID, broker, port, user, password)
		}

	case "rename_probe":
		newID, _ := req.Payload["new_id"].(string)
		if newID == "" {
			err = fmt.Errorf("rename_probe requires new_id")
		} else {
			err = s.mqttClient.SendRenameProbe(req.ProbeID, cmd.ID, newID)
		}

	case "restart":
		delay := 2000
		if d, ok := req.Payload["delay"].(float64); ok {
			delay = int(d)
		}
		err = s.mqttClient.SendRestart(req.ProbeID, cmd.ID, delay)

	case "ota_update":
		url, _ := req.Payload["url"].(string)
		if url == "" {
			err = fmt.Errorf("ota_update requires url")
		} else {
			err = s.mqttClient.SendOTAUpdate(req.ProbeID, cmd.ID, url)
		}

	case "factory_reset":
		err = s.mqttClient.SendFactoryReset(req.ProbeID, cmd.ID)

	case "ping":
		err = s.mqttClient.SendPing(req.ProbeID, cmd.ID)

	case "get_status":
		err = s.mqttClient.SendGetStatus(req.ProbeID, cmd.ID)

	default:
		s.log.Info("Sending custom command: %s", req.CommandType)
		err = s.mqttClient.SendRawCommand(req.ProbeID, cmd.ID, req.CommandType, req.Payload)
	}

	if err != nil {
		s.log.Error("Failed to send command via MQTT: %v", err)
		updateErr := s.commandRepo.UpdateStatus(ctx, cmd.ID, "failed", map[string]interface{}{"error": err.Error()})
		if updateErr != nil {
			return nil, updateErr
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

	if err := s.mqttClient.BroadcastCommand(cmd.ID, commandType, params); err != nil {
		s.log.Error("Failed to broadcast command: %v", err)
		updateErr := s.commandRepo.UpdateStatus(ctx, cmd.ID, "failed", map[string]interface{}{"error": err.Error()})
		if updateErr != nil {
			return updateErr
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

func (s *CommandService) ProcessCommandResult(ctx context.Context, payload []byte) error {
	var result struct {
		ProbeID   string                 `json:"probe_id"`
		Command   string                 `json:"command"`
		Status    string                 `json:"status"`
		Result    map[string]interface{} `json:"result"`
		CommandID string                 `json:"command_id"`
	}

	if err := json.Unmarshal(payload, &result); err != nil {
		s.log.Error("Failed to unmarshal command result: %v", err)
		return err
	}

	s.log.Info("Processing result: Probe=%s Cmd=%s Status=%s CommandID=%s", result.ProbeID, result.Command, result.Status, result.CommandID)

	if result.CommandID != "" {
		cmdID := 0
		if _, err := fmt.Sscanf(result.CommandID, "%d", &cmdID); err == nil && cmdID > 0 {
			err := s.commandRepo.UpdateStatus(ctx, cmdID, result.Status, result.Result)
			if err != nil {
				s.log.Warn("Failed to update command %d: %v", cmdID, err)
			}
		}
	} else {
		err := s.commandRepo.UpdateLatestResult(ctx, result.ProbeID, result.Command, result.Status, result.Result)
		if err != nil {
			s.log.Warn("Could not link result to a specific command history entry: %v", err)
		}
	}

	if result.Status == "completed" {
		switch result.Command {
		case "deep_scan":
			if err := s.commandRepo.PruneOldScans(ctx, result.ProbeID, 5); err != nil {
				s.log.Warn("Failed to prune old deep scans: %v", err)
			}
			go func() {
				bgCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
				defer cancel()

				if err := s.telemetryService.RecordDeepScanAsTelemetry(bgCtx, result.ProbeID, result.Result); err != nil {
					s.log.Error("Failed to record deep scan telemetry: %v", err)
				}
			}()
			s.log.Info("Deep scan completed for %s", result.ProbeID)

		case "config_update", "set_wifi", "set_mqtt":
			s.log.Info("Probe %s configuration updated successfully", result.ProbeID)

		case "rename_probe":
			if newID, ok := result.Result["new_id"].(string); ok && newID != "" {
				s.log.Info("Probe %s renamed to %s", result.ProbeID, newID)
			}

		case "ota_update":
			s.log.Info("Probe %s OTA update status: %s", result.ProbeID, result.Status)
			if progress, ok := result.Result["progress"].(float64); ok {
				s.log.Info("OTA Progress: %.0f%%", progress)
			}

		case "get_status":
			s.handleStatusUpdate(ctx, result.ProbeID, result.Result)

		case "get_config":
			s.log.Info("Probe %s config retrieved", result.ProbeID)

		case "ping":
			_ = s.probeRepo.UpdateLastSeen(ctx, result.ProbeID, time.Now())

		case "factory_reset":
			s.log.Warn("Probe %s performed a factory reset", result.ProbeID)
		}
	} else if result.Status == "processing" {
		if result.Command == "ota_update" {
			if progress, ok := result.Result["progress"].(float64); ok {
				s.log.Info("Probe %s OTA progress: %.0f%%", result.ProbeID, progress)
			}
		}
	}

	return nil
}

func (s *CommandService) VerifyProbeConnectivity(ctx context.Context, probeID string) error {
	probe, err := s.probeRepo.GetByID(ctx, probeID)
	if err != nil {
		return fmt.Errorf("probe lookup failed: %w", err)
	}
	if time.Since(probe.LastSeen) < StaleThreshold {
		return nil
	}

	s.log.Info("Attempting to ping %s (last seen: %v)", probeID, probe.LastSeen)

	tempCmd := &models.Command{
		ProbeID:     probeID,
		CommandType: "ping",
		Status:      "pending",
	}
	if err := s.commandRepo.Create(ctx, tempCmd); err != nil {
		return fmt.Errorf("failed to create ping command: %w", err)
	}

	if err := s.mqttClient.SendPing(probeID, tempCmd.ID); err != nil {
		return fmt.Errorf("failed to send wake-up ping: %w", err)
	}

	ticker := time.NewTicker(500 * time.Millisecond)
	defer ticker.Stop()
	timeout := time.After(5 * time.Second)

	initialLastSeen := probe.LastSeen

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-timeout:
			return fmt.Errorf("probe unreachable: no response to ping after 5s")
		case <-ticker.C:
			p, err := s.probeRepo.GetByID(ctx, probeID)
			if err == nil && p.LastSeen.After(initialLastSeen) {
				s.log.Info("Probe %s is back online!", probeID)
				return nil
			}
		}
	}
}

func (s *CommandService) handleStatusUpdate(ctx context.Context, probeID string, data map[string]interface{}) {
	if err := s.probeRepo.UpdateLastSeen(ctx, probeID, time.Now()); err != nil {
		s.log.Error("Failed to update last_seen from status report: %v", err)
	}
}

func (s *CommandService) DeleteCommand(ctx context.Context, commandID int) error {
	return s.commandRepo.Delete(ctx, commandID)
}

func (s *CommandService) StartBackgroundPinger(ctx context.Context) {
	ticker := time.NewTicker(30 * time.Second)
	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				s.pingAllProbes(ctx)
			}
		}
	}()
}

func (s *CommandService) pingAllProbes(ctx context.Context) {
	probes, err := s.probeRepo.GetAll(ctx)
	if err != nil {
		s.log.Error("Failed to get probes for ping: %v", err)
		return
	}

	for _, probe := range probes {
		go func(probeID string) {
			tempCmd := &models.Command{
				ProbeID:     probeID,
				CommandType: "ping",
				Status:      "pending",
			}

			if err := s.commandRepo.Create(ctx, tempCmd); err != nil {
				s.setPingStatus(probeID, false)
				return
			}

			if err := s.mqttClient.SendPing(probeID, tempCmd.ID); err != nil {
				s.setPingStatus(probeID, false)
				return
			}

			time.Sleep(3 * time.Second)

			cmd, err := s.commandRepo.GetByID(ctx, tempCmd.ID)
			if err == nil && cmd.Status == "completed" {
				s.setPingStatus(probeID, true)
			} else {
				s.setPingStatus(probeID, false)
			}
		}(probe.ProbeID)
	}
}

func (s *CommandService) setPingStatus(probeID string, status bool) {
	s.pingStatusMux.Lock()
	defer s.pingStatusMux.Unlock()
	s.pingStatus[probeID] = status
}

func (s *CommandService) GetPingStatus(probeID string) bool {
	s.pingStatusMux.RLock()
	defer s.pingStatusMux.RUnlock()
	return s.pingStatus[probeID]
}
