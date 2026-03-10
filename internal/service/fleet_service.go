package service

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"time"

	"CampusMonitorAPI/internal/logger"
	"CampusMonitorAPI/internal/models"
	"CampusMonitorAPI/internal/mqtt"
	"CampusMonitorAPI/internal/repository"

	_ "github.com/google/uuid"
)

type FleetService struct {
	fleetRepo     *repository.FleetRepository
	probeRepo     *repository.ProbeRepository
	commandRepo   *repository.CommandRepository
	telemetryRepo *repository.TelemetryRepository
	alertRepo     *repository.AlertRepository
	mqttClient    *mqtt.Client
	log           *logger.Logger

	// In-memory tracking for active rollouts
	activeRollouts map[string]*models.FleetRolloutStatus
	rolloutMux     sync.RWMutex
}

func NewFleetService(
	fleetRepo *repository.FleetRepository,
	probeRepo *repository.ProbeRepository,
	commandRepo *repository.CommandRepository,
	telemetryRepo *repository.TelemetryRepository,
	alertRepo *repository.AlertRepository,
	mqttClient *mqtt.Client,
	log *logger.Logger,
) *FleetService {
	return &FleetService{
		fleetRepo:      fleetRepo,
		probeRepo:      probeRepo,
		commandRepo:    commandRepo,
		telemetryRepo:  telemetryRepo,
		alertRepo:      alertRepo,
		mqttClient:     mqttClient,
		log:            log,
		activeRollouts: make(map[string]*models.FleetRolloutStatus),
	}
}

// EnrollProbe enables fleet management for a probe
func (s *FleetService) EnrollProbe(ctx context.Context, probeID string, req *models.FleetEnrollRequest, user string) error {
	s.log.Info("Enrolling probe %s into fleet management", probeID)

	// Verify probe exists
	_, err := s.probeRepo.GetByID(ctx, probeID)
	if err != nil {
		return fmt.Errorf("probe not found: %w", err)
	}

	// If template specified, apply it
	if req.ConfigTemplateID != nil {
		if err := s.applyTemplate(ctx, probeID, *req.ConfigTemplateID, req); err != nil {
			s.log.Warn("Failed to apply template during enrollment: %v", err)
			// Continue with enrollment even if template fails
		}
	}

	// Enroll in fleet management
	err = s.fleetRepo.EnrollProbe(ctx, probeID, req, user)
	if err != nil {
		return err
	}

	// Send initial fleet config to probe
	cmdReq := &models.FleetCommandRequest{
		CommandType: "fleet_enroll",
		Payload: map[string]interface{}{
			"groups":             req.Groups,
			"location":           req.Location,
			"tags":               req.Tags,
			"maintenance_window": req.MaintenanceWindow,
		},
	}

	// Send to single probe
	_, err = s.SendFleetCommand(ctx, &models.FleetCommandRequest{
		CommandType: cmdReq.CommandType,
		Payload:     cmdReq.Payload,
		ProbeIDs:    []string{probeID},
		Strategy:    "immediate",
	}, user)

	// Update probe status to active
	status := "active"
	s.probeRepo.Update(ctx, probeID, &models.UpdateProbeRequest{Status: &status})

	return err
}

// UnenrollProbe removes probe from fleet management
func (s *FleetService) UnenrollProbe(ctx context.Context, probeID string) error {
	s.log.Info("Unenrolling probe %s from fleet management", probeID)

	// Send unenroll command to probe
	_, err := s.SendFleetCommand(ctx, &models.FleetCommandRequest{
		CommandType: "fleet_unenroll",
		Payload:     map[string]interface{}{},
		ProbeIDs:    []string{probeID},
		Strategy:    "immediate",
	}, "system")

	if err != nil {
		s.log.Warn("Failed to send unenroll command: %v", err)
	}

	// Remove from fleet database
	return s.fleetRepo.UnenrollProbe(ctx, probeID)
}

// GetFleetProbe retrieves fleet-managed probe details
func (s *FleetService) GetFleetProbe(ctx context.Context, probeID string) (*models.FleetProbe, error) {
	return s.fleetRepo.GetFleetProbe(ctx, probeID)
}

// ListFleetProbes lists all fleet-managed probes
func (s *FleetService) ListFleetProbes(ctx context.Context, group string) ([]models.FleetProbe, error) {
	return s.fleetRepo.ListFleetProbes(ctx, true, group)
}

// UpdateFleetProbe updates fleet probe metadata
func (s *FleetService) UpdateFleetProbe(ctx context.Context, probeID string, req *models.FleetUpdateRequest) error {
	s.log.Info("Updating fleet probe %s", probeID)

	// If groups changed, update MQTT subscriptions
	if req.Groups != nil {
		go s.updateProbeGroups(probeID, *req.Groups)
	}

	return s.fleetRepo.UpdateFleetProbe(ctx, probeID, req)
}

// SendFleetCommand sends a command to multiple probes with rollout strategy
func (s *FleetService) SendFleetCommand(ctx context.Context, req *models.FleetCommandRequest, user string) (*models.FleetCommand, error) {
	s.log.Info("Sending fleet command: type=%s, strategy=%s", req.CommandType, req.Strategy)

	// Resolve target probes
	targetProbes, err := s.resolveTargets(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("failed to resolve targets: %w", err)
	}

	if len(targetProbes) == 0 {
		return nil, fmt.Errorf("no probes match targeting criteria")
	}

	// Create fleet command record
	fleetCmd := &models.FleetCommand{
		CommandType:         req.CommandType,
		Payload:             req.Payload,
		IssuedBy:            user,
		TargetGroups:        req.Groups,
		TargetProbes:        targetProbes,
		TotalTargets:        len(targetProbes),
		Status:              "pending",
		CompletionThreshold: req.CompletionPercent,
		TimeoutSeconds:      req.AckTimeoutSeconds,
		ScheduledFor:        nil,
	}

	if req.Schedule != nil && !req.Schedule.ExecuteAt.IsZero() {
		fleetCmd.ScheduledFor = &req.Schedule.ExecuteAt
		fleetCmd.Status = "scheduled"
	}

	if err := s.fleetRepo.CreateFleetCommand(ctx, fleetCmd); err != nil {
		return nil, err
	}

	// If scheduled, don't send immediately
	if fleetCmd.ScheduledFor != nil {
		s.log.Info("Command scheduled for %v", fleetCmd.ScheduledFor)
		go s.scheduleCommand(fleetCmd, req)
		return fleetCmd, nil
	}

	// Execute rollout based on strategy
	go s.executeRollout(fleetCmd, req, user)

	return fleetCmd, nil
}

// GetFleetCommandStatus retrieves the status of a fleet command
func (s *FleetService) GetFleetCommandStatus(ctx context.Context, commandID string) (*models.FleetRolloutStatus, error) {
	// Check in-memory active rollouts first
	s.rolloutMux.RLock()
	if status, exists := s.activeRollouts[commandID]; exists {
		s.rolloutMux.RUnlock()
		return status, nil
	}
	s.rolloutMux.RUnlock()

	// Get from database
	cmd, err := s.fleetRepo.GetFleetCommand(ctx, commandID)
	if err != nil {
		return nil, err
	}

	// Build status from database
	status := &models.FleetRolloutStatus{
		CommandID:   cmd.ID,
		CommandType: cmd.CommandType,
		IssuedAt:    cmd.IssuedAt,
		Status:      cmd.Status,
		Progress: models.RolloutProgress{
			Total:        cmd.TotalTargets,
			Acknowledged: cmd.AcksReceived,
			Completed:    cmd.CompletedCount,
			Failed:       cmd.FailedCount,
			Pending:      cmd.TotalTargets - (cmd.AcksReceived + cmd.CompletedCount + cmd.FailedCount),
		},
		Timeline: models.RolloutTimeline{
			StartedAt:   cmd.IssuedAt,
			CompletedAt: cmd.CompletedAt,
		},
	}

	if status.Progress.Total > 0 {
		status.Progress.Percentage = float64(status.Progress.Completed+status.Progress.Failed) / float64(status.Progress.Total) * 100
	}

	return status, nil
}

// ListFleetCommands lists recent fleet commands
func (s *FleetService) ListFleetCommands(ctx context.Context, status string, limit int) ([]models.FleetCommand, error) {
	return s.fleetRepo.ListFleetCommands(ctx, status, limit)
}

// CancelFleetCommand cancels a pending or in-progress command
func (s *FleetService) CancelFleetCommand(ctx context.Context, commandID string) error {
	s.log.Info("Cancelling fleet command %s", commandID)

	// Remove from active rollouts
	s.rolloutMux.Lock()
	delete(s.activeRollouts, commandID)
	s.rolloutMux.Unlock()

	// Update status in database
	// TODO: Implement database status update for cancellation

	return nil
}

// Template Management

func (s *FleetService) CreateTemplate(ctx context.Context, template *models.FleetConfigTemplate, user string) error {
	template.CreatedBy = user
	return s.fleetRepo.CreateTemplate(ctx, template)
}

func (s *FleetService) GetTemplate(ctx context.Context, id int) (*models.FleetConfigTemplate, error) {
	return s.fleetRepo.GetTemplate(ctx, id)
}

func (s *FleetService) ListTemplates(ctx context.Context) ([]models.FleetConfigTemplate, error) {
	return s.fleetRepo.ListTemplates(ctx)
}

func (s *FleetService) ApplyTemplate(ctx context.Context, templateID int, probeIDs []string, user string) error {
	s.log.Info("Applying template %d to %d probes", templateID, len(probeIDs))

	template, err := s.fleetRepo.GetTemplate(ctx, templateID)
	if err != nil {
		return err
	}

	// Update template usage count
	s.fleetRepo.UpdateTemplateUsage(ctx, templateID)

	// For each probe, send config update
	for _, probeID := range probeIDs {
		// Resolve template variables with probe data
		config := s.resolveTemplateVariables(template.Config, probeID)

		// Send config update command
		_, err := s.SendFleetCommand(ctx, &models.FleetCommandRequest{
			CommandType: "fleet_config",
			Payload: map[string]interface{}{
				"config":  config,
				"version": templateID,
			},
			ProbeIDs: []string{probeID},
			Strategy: "immediate",
		}, user)

		if err != nil {
			s.log.Error("Failed to apply template to probe %s: %v", probeID, err)
		}

		// Update probe's config template reference
		s.fleetRepo.UpdateFleetProbe(ctx, probeID, &models.FleetUpdateRequest{
			ConfigTemplateID: &templateID,
		})
	}

	return nil
}

func (s *FleetService) DeleteTemplate(ctx context.Context, id int) error {
	return s.fleetRepo.DeleteTemplate(ctx, id)
}

// Group Management

func (s *FleetService) CreateGroup(ctx context.Context, name, description string) (*models.FleetGroup, error) {
	group := &models.FleetGroup{
		Name:        name,
		Description: description,
	}

	err := s.fleetRepo.CreateGroup(ctx, group)
	return group, err
}

func (s *FleetService) ListGroups(ctx context.Context) ([]models.FleetGroup, error) {
	return s.fleetRepo.ListGroups(ctx)
}

func (s *FleetService) DeleteGroup(ctx context.Context, groupID string) error {
	return s.fleetRepo.DeleteGroup(ctx, groupID)
}

// Fleet Status

func (s *FleetService) GetFleetStatus(ctx context.Context) (*models.FleetStatusResponse, error) {
	return s.fleetRepo.GetFleetStatus(ctx)
}

// ProcessCommandResult handles command results from probes (called by your existing CommandService)
func (s *FleetService) ProcessCommandResult(ctx context.Context, probeID, commandID, status string, result map[string]interface{}) error {
	s.log.Debug("Processing fleet command result: probe=%s, cmd=%s, status=%s", probeID, commandID, status)

	// Update probe stats
	s.fleetRepo.UpdateFleetCommandStats(ctx, probeID, commandID, status)

	// Update fleet command status
	if strings.HasPrefix(commandID, "cmd_") {
		// This is a fleet command
		s.updateFleetCommandProbeStatus(ctx, commandID, probeID, status, result)
	}

	// Handle specific command types
	switch commandID {
	case "fleet_config":
		if status == "completed" {
			// Increment config version
			s.fleetRepo.UpdateFleetProbe(ctx, probeID, &models.FleetUpdateRequest{})
		}
	case "fleet_ota":
		if version, ok := result["version"].(string); ok && status == "completed" {
			s.fleetRepo.UpdateFirmwareVersion(ctx, probeID, version)
		}
	}

	return nil
}

// Private helper methods

func (s *FleetService) resolveTargets(ctx context.Context, req *models.FleetCommandRequest) ([]string, error) {
	targetMap := make(map[string]bool)

	if req.TargetAll {
		// Get all managed probes
		probes, err := s.fleetRepo.ListFleetProbes(ctx, true, "")
		if err != nil {
			return nil, err
		}
		for _, p := range probes {
			targetMap[p.ProbeID] = true
		}
	}

	// Add groups
	for _, group := range req.Groups {
		probes, err := s.fleetRepo.ListFleetProbes(ctx, true, group)
		if err != nil {
			return nil, err
		}
		for _, p := range probes {
			targetMap[p.ProbeID] = true
		}
	}

	// Add specific probes
	for _, pid := range req.ProbeIDs {
		targetMap[pid] = true
	}

	// Remove excluded probes
	for _, pid := range req.ExcludeProbes {
		delete(targetMap, pid)
	}

	// Convert to slice
	targets := make([]string, 0, len(targetMap))
	for pid := range targetMap {
		targets = append(targets, pid)
	}

	return targets, nil
}

func (s *FleetService) executeRollout(cmd *models.FleetCommand, req *models.FleetCommandRequest, user string) {
	ctx := context.Background()

	// Initialize rollout status
	rollout := &models.FleetRolloutStatus{
		CommandID:   cmd.ID,
		CommandType: cmd.CommandType,
		IssuedAt:    time.Now(),
		Status:      "in_progress",
		Progress: models.RolloutProgress{
			Total: cmd.TotalTargets,
		},
		Timeline: models.RolloutTimeline{
			StartedAt: time.Now(),
		},
	}

	s.rolloutMux.Lock()
	s.activeRollouts[cmd.ID] = rollout
	s.rolloutMux.Unlock()

	// Update command status
	s.fleetRepo.UpdateFleetCommandStatus(ctx, cmd.ID, map[string]int{
		"acks_received": 0,
		"completed":     0,
		"failed":        0,
	})

	// Determine which probes to target based on rollout percentage
	targetProbes := cmd.TargetProbes
	if req.RolloutPercentage > 0 && req.RolloutPercentage < 100 {
		// Only target percentage of probes
		limit := (req.RolloutPercentage * len(targetProbes)) / 100
		if limit < 1 {
			limit = 1
		}
		targetProbes = targetProbes[:limit]
	}

	// Execute based on strategy
	switch req.Strategy {
	case "canary":
		s.canaryRollout(ctx, cmd, req, targetProbes, user)
	case "staggered":
		s.staggeredRollout(ctx, cmd, req, targetProbes, user)
	case "maintenance":
		s.maintenanceRollout(ctx, cmd, req, targetProbes, user)
	default: // immediate
		s.immediateRollout(ctx, cmd, req, targetProbes, user)
	}
}

func (s *FleetService) immediateRollout(ctx context.Context, cmd *models.FleetCommand, req *models.FleetCommandRequest, probes []string, user string) {
	for _, probeID := range probes {
		s.sendCommandToProbe(ctx, cmd, req, probeID, user)
	}
}

func (s *FleetService) canaryRollout(ctx context.Context, cmd *models.FleetCommand, req *models.FleetCommandRequest, probes []string, user string) {
	if len(probes) == 0 {
		return
	}

	canaryCount := req.CanaryCount
	if canaryCount <= 0 {
		canaryCount = 5
	}
	if canaryCount > len(probes) {
		canaryCount = len(probes)
	}

	// Send to canary group first
	canaryProbes := probes[:canaryCount]
	for _, probeID := range canaryProbes {
		s.sendCommandToProbe(ctx, cmd, req, probeID, user)
	}

	// Wait for canary results
	time.Sleep(30 * time.Second)

	// Check canary success rate
	s.rolloutMux.RLock()
	rollout := s.activeRollouts[cmd.ID]
	s.rolloutMux.RUnlock()

	if rollout != nil {
		canarySuccess := rollout.Progress.Completed
		canaryFailed := rollout.Progress.Failed

		if canaryFailed > canarySuccess || canarySuccess < canaryCount/2 {
			s.log.Warn("Canary rollout failed, pausing rollout")
			return
		}
	}

	// Roll out to rest
	if len(probes) > canaryCount {
		restProbes := probes[canaryCount:]
		for _, probeID := range restProbes {
			s.sendCommandToProbe(ctx, cmd, req, probeID, user)
			time.Sleep(100 * time.Millisecond) // Small delay
		}
	}
}

func (s *FleetService) staggeredRollout(ctx context.Context, cmd *models.FleetCommand, req *models.FleetCommandRequest, probes []string, user string) {
	batchSize := req.BatchSize
	if batchSize <= 0 {
		batchSize = 10
	}
	delay := req.StaggerDelay
	if delay <= 0 {
		delay = 1000 // 1 second default
	}

	for i := 0; i < len(probes); i += batchSize {
		end := i + batchSize
		if end > len(probes) {
			end = len(probes)
		}

		batch := probes[i:end]
		for _, probeID := range batch {
			s.sendCommandToProbe(ctx, cmd, req, probeID, user)
		}

		// Wait before next batch
		if end < len(probes) {
			time.Sleep(time.Duration(delay) * time.Millisecond)
		}
	}
}

func (s *FleetService) maintenanceRollout(ctx context.Context, cmd *models.FleetCommand, req *models.FleetCommandRequest, probes []string, user string) {
	// Group probes by maintenance window
	probesByWindow := make(map[string][]string)

	for _, probeID := range probes {
		fp, err := s.fleetRepo.GetFleetProbe(ctx, probeID)
		if err != nil {
			continue
		}

		if fp.MaintenanceWindow != nil {
			windowKey := fp.MaintenanceWindow.Start + "-" + fp.MaintenanceWindow.End
			probesByWindow[windowKey] = append(probesByWindow[windowKey], probeID)
		} else {
			// No window, execute now
			s.sendCommandToProbe(ctx, cmd, req, probeID, user)
		}
	}

	// Schedule for maintenance windows
	for window, windowProbes := range probesByWindow {
		// Parse window times and schedule
		go s.scheduleForMaintenanceWindow(cmd, req, windowProbes, window, user)
	}
}

func (s *FleetService) sendCommandToProbe(ctx context.Context, cmd *models.FleetCommand, req *models.FleetCommandRequest, probeID, user string) {
	// Create individual command
	_ = &models.CommandRequest{
		ProbeID:     probeID,
		CommandType: req.CommandType,
		Payload:     req.Payload,
	}

	// Use your existing command service to send
	// This assumes you have access to commandService or mqttClient directly
	payload, _ := json.Marshal(map[string]interface{}{
		"command":    req.CommandType,
		"command_id": cmd.ID,
		"payload":    req.Payload,
		"timestamp":  time.Now().Unix(),
	})

	topic := fmt.Sprintf("campus/probes/%s/command", probeID)
	err := s.mqttClient.Publish(topic, payload)

	// Track status
	probeStatus := &models.FleetCommandProbeStatus{
		CommandID: cmd.ID,
		ProbeID:   probeID,
		Status:    "sent",
	}

	if err != nil {
		probeStatus.Status = "failed"
		probeStatus.ErrorMessage = err.Error()
	}

	err = s.fleetRepo.SaveProbeCommandStatus(ctx, probeStatus)
	if err != nil {
		return
	}

	// Update rollout progress
	s.updateRolloutProgress(cmd.ID)
}

func (s *FleetService) updateFleetCommandProbeStatus(ctx context.Context, commandID, probeID, status string, result map[string]interface{}) {
	probeStatus := &models.FleetCommandProbeStatus{
		CommandID: commandID,
		ProbeID:   probeID,
		Status:    status,
		Result:    result,
	}

	err := s.fleetRepo.SaveProbeCommandStatus(ctx, probeStatus)
	if err != nil {
		return
	}
	s.updateRolloutProgress(commandID)
}

func (s *FleetService) updateRolloutProgress(commandID string) {
	s.rolloutMux.RLock()
	rollout, exists := s.activeRollouts[commandID]
	s.rolloutMux.RUnlock()

	if !exists {
		return
	}

	ctx := context.Background()

	// Get updated stats from DB using a direct query since we don't have a repo method
	var acks, completed, failed int
	err := s.fleetRepo.DB().QueryRowContext(ctx, `
		SELECT 
			COUNT(CASE WHEN status IN ('acknowledged', 'processing', 'completed') THEN 1 END) as acks,
			COUNT(CASE WHEN status = 'completed' THEN 1 END) as completed,
			COUNT(CASE WHEN status = 'failed' THEN 1 END) as failed
		FROM fleet_command_probes
		WHERE command_id = $1
	`, commandID).Scan(&acks, &completed, &failed)

	if err != nil {
		s.log.Error("Failed to get rollout stats: %v", err)
		return
	}

	// Update rollout
	s.rolloutMux.Lock()
	rollout.Progress.Acknowledged = acks
	rollout.Progress.Completed = completed
	rollout.Progress.Failed = failed
	rollout.Progress.Pending = rollout.Progress.Total - (acks + completed + failed)
	if rollout.Progress.Total > 0 {
		rollout.Progress.Percentage = float64(completed+failed) / float64(rollout.Progress.Total) * 100
	}
	s.rolloutMux.Unlock()

	// Update DB
	err = s.fleetRepo.UpdateFleetCommandStatus(ctx, commandID, map[string]int{
		"acks_received": acks,
		"completed":     completed,
		"failed":        failed,
	})
	if err != nil {
		return
	}

	// Check if complete
	if completed+failed >= rollout.Progress.Total {
		s.rolloutMux.Lock()
		rollout.Status = "completed"
		now := time.Now()
		rollout.Timeline.CompletedAt = &now
		s.rolloutMux.Unlock()
	}
}
func (s *FleetService) GetUnenrolledProbes(ctx context.Context) ([]models.Probe, error) {
	return s.fleetRepo.GetUnenrolledProbes(ctx)
}
func (s *FleetService) applyTemplate(ctx context.Context, probeID string, templateID int, req *models.FleetEnrollRequest) error {
	template, err := s.fleetRepo.GetTemplate(ctx, templateID)
	if err != nil {
		return err
	}

	// Resolve variables
	config := s.resolveTemplateVariables(template.Config, probeID)

	// Send config to probe via MQTT
	cmdReq := &models.FleetCommandRequest{
		CommandType: "fleet_config",
		Payload: map[string]interface{}{
			"config":  config,
			"version": templateID,
		},
		ProbeIDs: []string{probeID},
		Strategy: "immediate",
	}

	_, err = s.SendFleetCommand(ctx, cmdReq, "system")
	return err
}

func (s *FleetService) resolveTemplateVariables(config map[string]interface{}, probeID string) map[string]interface{} {
	result := make(map[string]interface{})

	// Recursively resolve ${variable} placeholders
	var resolve func(interface{}) interface{}
	resolve = func(val interface{}) interface{} {
		switch v := val.(type) {
		case string:
			// Replace ${probe_id} with actual probe ID
			if strings.Contains(v, "${probe_id}") {
				return strings.ReplaceAll(v, "${probe_id}", probeID)
			}
			// Add more variable replacements as needed
			return v
		case map[string]interface{}:
			m := make(map[string]interface{})
			for k, v2 := range v {
				m[k] = resolve(v2)
			}
			return m
		case []interface{}:
			arr := make([]interface{}, len(v))
			for i, v2 := range v {
				arr[i] = resolve(v2)
			}
			return arr
		default:
			return v
		}
	}

	for k, v := range config {
		result[k] = resolve(v)
	}

	return result
}

func (s *FleetService) scheduleCommand(cmd *models.FleetCommand, req *models.FleetCommandRequest) {
	delay := time.Until(*cmd.ScheduledFor)
	if delay > 0 {
		time.Sleep(delay)
	}

	// Execute rollout
	s.executeRollout(cmd, req, cmd.IssuedBy)
}

func (s *FleetService) scheduleForMaintenanceWindow(cmd *models.FleetCommand, req *models.FleetCommandRequest, probes []string, window string, user string) {
	// Parse window (e.g., "02:00-04:00")
	parts := strings.Split(window, "-")
	if len(parts) != 2 {
		return
	}

	startTime := parts[0]
	_ = parts[1]

	// Calculate next occurrence
	now := time.Now()
	location := time.Local

	// Parse start time
	startHour, startMin := 0, 0
	fmt.Sscanf(startTime, "%d:%d", &startHour, &startMin)

	nextStart := time.Date(now.Year(), now.Month(), now.Day(),
		startHour, startMin, 0, 0, location)

	if nextStart.Before(now) {
		nextStart = nextStart.Add(24 * time.Hour)
	}

	// Sleep until maintenance window
	delay := time.Until(nextStart)
	if delay > 0 {
		time.Sleep(delay)
	}

	// Send commands
	for _, probeID := range probes {
		s.sendCommandToProbe(context.Background(), cmd, req, probeID, user)
	}
}

func (s *FleetService) updateProbeGroups(probeID string, groups []string) {
	// This would trigger an MQTT message to update group subscriptions
	// Implementation depends on your MQTT client
}
