package service

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"CampusMonitorAPI/internal/logger"
	"CampusMonitorAPI/internal/models"
	"CampusMonitorAPI/internal/mqtt"
	"CampusMonitorAPI/internal/repository"

	"github.com/google/uuid"
)

type ScheduleService struct {
	scheduleRepo *repository.ScheduleRepository
	probeRepo    *repository.ProbeRepository
	mqttClient   *mqtt.Client
	log          *logger.Logger
}

func NewScheduleService(
	scheduleRepo *repository.ScheduleRepository,
	probeRepo *repository.ProbeRepository,
	mqttClient *mqtt.Client,
	log *logger.Logger,
) *ScheduleService {
	return &ScheduleService{
		scheduleRepo: scheduleRepo,
		probeRepo:    probeRepo,
		mqttClient:   mqttClient,
		log:          log,
	}
}

// Create schedules a task on a probe and stores it in the database.
func (s *ScheduleService) Create(ctx context.Context, req *models.ScheduledTask) error {
	s.log.Info("Creating scheduled task for probe %s: %s", req.ProbeID, req.CommandType)

	// Verify probe exists
	_, err := s.probeRepo.GetByID(ctx, req.ProbeID)
	if err != nil {
		return fmt.Errorf("probe not found: %w", err)
	}

	// Set ID if not provided
	if req.ID == "" {
		req.ID = uuid.New().String()
	}

	// Set next_run based on schedule
	if err := s.calcNextRun(req); err != nil {
		return err
	}

	// Send to probe via MQTT
	if err := s.sendScheduleToProbe(ctx, req, "create"); err != nil {
		return err
	}

	// Store in database
	return s.scheduleRepo.Create(ctx, req)
}

// List returns all scheduled tasks for a probe.
func (s *ScheduleService) List(ctx context.Context, probeID string) ([]models.ScheduledTask, error) {
	return s.scheduleRepo.ListByProbe(ctx, probeID)
}

// Get retrieves a specific scheduled task.
func (s *ScheduleService) Get(ctx context.Context, id string) (*models.ScheduledTask, error) {
	return s.scheduleRepo.GetByID(ctx, id)
}

// Update modifies an existing scheduled task.
func (s *ScheduleService) Update(ctx context.Context, id string, req *models.ScheduledTask) error {
	s.log.Info("Updating scheduled task %s", id)

	existing, err := s.scheduleRepo.GetByID(ctx, id)
	if err != nil {
		return err
	}

	// Update fields
	existing.CommandType = req.CommandType
	existing.Payload = req.Payload
	existing.Schedule = req.Schedule
	existing.Enabled = req.Enabled
	if err := s.calcNextRun(existing); err != nil {
		return err
	}

	// Send update to probe (delete old, then create new)
	if err := s.sendScheduleToProbe(ctx, existing, "delete"); err != nil {
		s.log.Warn("Failed to delete old schedule on probe: %v", err)
	}
	if err := s.sendScheduleToProbe(ctx, existing, "create"); err != nil {
		return err
	}

	return s.scheduleRepo.Update(ctx, existing)
}

// Delete removes a scheduled task.
func (s *ScheduleService) Delete(ctx context.Context, id string) error {
	s.log.Info("Deleting scheduled task %s", id)

	task, err := s.scheduleRepo.GetByID(ctx, id)
	if err != nil {
		return err
	}

	// Send cancellation to probe
	if err := s.sendScheduleToProbe(ctx, task, "delete"); err != nil {
		s.log.Warn("Failed to send delete command to probe: %v", err)
	}

	return s.scheduleRepo.Delete(ctx, id)
}

// HandleCommandResult is called by CommandService when a command result arrives.
// If the command ID matches a scheduled task, update last_run and next_run.
func (s *ScheduleService) HandleCommandResult(ctx context.Context, commandID, status string, result map[string]interface{}) {
	// Check if commandID looks like a task ID (UUID format)
	if _, err := uuid.Parse(commandID); err != nil {
		return // not a task ID
	}

	task, err := s.scheduleRepo.GetByID(ctx, commandID)
	if err != nil {
		s.log.Warn("Command result for unknown task ID: %s", commandID)
		return
	}

	now := time.Now().UTC()
	if status == "completed" || status == "failed" {
		task.LastRun = &now
		if task.Enabled && task.Schedule.Type == "recurring" {
			next := s.computeNextRecurring(task, now)
			task.NextRun = &next
		} else {
			if task.Schedule.Type == "one-time" {
				task.Enabled = false
				task.NextRun = nil
			}
		}
		if err := s.scheduleRepo.UpdateLastRun(ctx, task.ID, *task.LastRun, task.NextRun); err != nil {
			s.log.Error("Failed to update last_run for task %s: %v", task.ID, err)
		} else {
			s.log.Info("Updated last_run for scheduled task %s", task.ID)
		}
	}
}

// calcNextRun computes the next execution time based on the schedule spec (for a new or updated task).
func (s *ScheduleService) calcNextRun(task *models.ScheduledTask) error {
	if !task.Enabled {
		task.NextRun = nil
		return nil
	}

	now := time.Now().UTC()
	switch task.Schedule.Type {
	case "one-time":
		if task.Schedule.ExecuteAt == nil {
			return fmt.Errorf("execute_at required for one-time schedule")
		}
		if task.Schedule.ExecuteAt.Before(now) {
			task.NextRun = nil
		} else {
			task.NextRun = task.Schedule.ExecuteAt
		}
	case "recurring":
		if task.Schedule.Cron == "" {
			return fmt.Errorf("cron required for recurring schedule")
		}
		next := s.computeNextRecurring(task, now)
		task.NextRun = &next
	default:
		return fmt.Errorf("invalid schedule type: %s", task.Schedule.Type)
	}
	return nil
}

// computeNextRecurring calculates the next execution time for a recurring task after a given reference time.
func (s *ScheduleService) computeNextRecurring(task *models.ScheduledTask, after time.Time) time.Time {
	base := after
	if task.Schedule.ExecuteAt != nil {
		base = *task.Schedule.ExecuteAt
	}

	switch task.Schedule.Cron {
	case "@hourly":
		// Next whole hour after `after`
		next := time.Date(after.Year(), after.Month(), after.Day(), after.Hour()+1, 0, 0, 0, time.UTC)
		return next
	case "@daily":
		// Use the hour/minute/second from base, but the next day after `after` if needed
		next := time.Date(after.Year(), after.Month(), after.Day(), base.Hour(), base.Minute(), base.Second(), 0, time.UTC)
		if next.Before(after) || next.Equal(after) {
			next = next.AddDate(0, 0, 1)
		}
		return next
	case "@weekly":
		// Use the weekday and time from base
		// Find the next occurrence of that weekday after `after`
		daysUntil := (base.Weekday() - after.Weekday() + 7) % 7
		next := time.Date(after.Year(), after.Month(), after.Day(), base.Hour(), base.Minute(), base.Second(), 0, time.UTC)
		if daysUntil == 0 {
			if next.Before(after) || next.Equal(after) {
				daysUntil = 7
			}
		}
		if daysUntil > 0 {
			next = next.AddDate(0, 0, int(daysUntil))
		}
		return next
	default:
		// fallback: add 24h
		return after.Add(24 * time.Hour)
	}
}

func (s *ScheduleService) sendScheduleToProbe(ctx context.Context, task *models.ScheduledTask, action string) error {
	// Build the payload for the firmware
	schedulePayload := map[string]interface{}{
		"operation":  task.CommandType,
		"parameters": task.Payload,
	}

	if action == "create" || action == "update" {
		scheduleSpec := map[string]interface{}{}
		if task.Schedule.ExecuteAt != nil {
			scheduleSpec["at"] = task.Schedule.ExecuteAt.Unix()
		}
		if task.Schedule.Cron != "" {
			scheduleSpec["cron"] = task.Schedule.Cron
		}
		schedulePayload["schedule"] = scheduleSpec
	} else if action == "delete" {
		schedulePayload = map[string]interface{}{
			"operation": "cancel",
			"schedule": map[string]interface{}{
				"id": task.ID,
			},
		}
	}

	// Construct the full MQTT message
	msg := map[string]interface{}{
		"command":    "fleet_schedule",
		"command_id": task.ID,
		"payload":    schedulePayload,
		"timestamp":  time.Now().Unix(),
	}
	data, err := json.Marshal(msg)
	if err != nil {
		return err
	}

	topic := fmt.Sprintf("campus/probes/%s/command", task.ProbeID)
	return s.mqttClient.Publish(topic, data)
}
