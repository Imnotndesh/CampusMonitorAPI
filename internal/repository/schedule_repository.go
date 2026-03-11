package repository

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	"CampusMonitorAPI/internal/models"

	"github.com/google/uuid"
)

type ScheduleRepository struct {
	db *sql.DB
}

func NewScheduleRepository(db *sql.DB) *ScheduleRepository {
	return &ScheduleRepository{db: db}
}

func (r *ScheduleRepository) Create(ctx context.Context, task *models.ScheduledTask) error {
	query := `
        INSERT INTO scheduled_tasks (id, probe_id, command_type, payload, schedule, next_run, enabled)
        VALUES ($1, $2, $3, $4, $5, $6, $7)
        RETURNING created_at, updated_at
    `
	if task.ID == "" {
		task.ID = uuid.New().String()
	}
	payloadJSON, _ := json.Marshal(task.Payload)
	scheduleJSON, _ := json.Marshal(task.Schedule)

	err := r.db.QueryRowContext(ctx, query,
		task.ID, task.ProbeID, task.CommandType, payloadJSON, scheduleJSON, task.NextRun, task.Enabled,
	).Scan(&task.CreatedAt, &task.UpdatedAt)
	if err != nil {
		return fmt.Errorf("failed to create scheduled task: %w", err)
	}
	return nil
}

func (r *ScheduleRepository) GetByID(ctx context.Context, id string) (*models.ScheduledTask, error) {
	query := `
        SELECT id, probe_id, command_type, payload, schedule, created_at, updated_at, last_run, next_run, enabled
        FROM scheduled_tasks
        WHERE id = $1
    `
	var task models.ScheduledTask
	var payloadJSON, scheduleJSON []byte
	var lastRun, nextRun sql.NullTime
	err := r.db.QueryRowContext(ctx, query, id).Scan(
		&task.ID, &task.ProbeID, &task.CommandType, &payloadJSON, &scheduleJSON,
		&task.CreatedAt, &task.UpdatedAt, &lastRun, &nextRun, &task.Enabled,
	)
	if err != nil {
		return nil, err
	}
	if len(payloadJSON) > 0 {
		json.Unmarshal(payloadJSON, &task.Payload)
	}
	if len(scheduleJSON) > 0 {
		json.Unmarshal(scheduleJSON, &task.Schedule)
	}
	if lastRun.Valid {
		task.LastRun = &lastRun.Time
	}
	if nextRun.Valid {
		task.NextRun = &nextRun.Time
	}
	return &task, nil
}

func (r *ScheduleRepository) ListByProbe(ctx context.Context, probeID string) ([]models.ScheduledTask, error) {
	query := `
        SELECT id, probe_id, command_type, payload, schedule, created_at, updated_at, last_run, next_run, enabled
        FROM scheduled_tasks
        WHERE probe_id = $1
        ORDER BY next_run NULLS LAST
    `
	rows, err := r.db.QueryContext(ctx, query, probeID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var tasks []models.ScheduledTask
	for rows.Next() {
		var task models.ScheduledTask
		var payloadJSON, scheduleJSON []byte
		var lastRun, nextRun sql.NullTime
		err := rows.Scan(
			&task.ID, &task.ProbeID, &task.CommandType, &payloadJSON, &scheduleJSON,
			&task.CreatedAt, &task.UpdatedAt, &lastRun, &nextRun, &task.Enabled,
		)
		if err != nil {
			return nil, err
		}
		json.Unmarshal(payloadJSON, &task.Payload)
		json.Unmarshal(scheduleJSON, &task.Schedule)
		if lastRun.Valid {
			task.LastRun = &lastRun.Time
		}
		if nextRun.Valid {
			task.NextRun = &nextRun.Time
		}
		tasks = append(tasks, task)
	}
	return tasks, nil
}

func (r *ScheduleRepository) Update(ctx context.Context, task *models.ScheduledTask) error {
	query := `
        UPDATE scheduled_tasks
        SET command_type = $2, payload = $3, schedule = $4, next_run = $5, enabled = $6, updated_at = NOW()
        WHERE id = $1
        RETURNING updated_at
    `
	payloadJSON, _ := json.Marshal(task.Payload)
	scheduleJSON, _ := json.Marshal(task.Schedule)
	return r.db.QueryRowContext(ctx, query,
		task.ID, task.CommandType, payloadJSON, scheduleJSON, task.NextRun, task.Enabled,
	).Scan(&task.UpdatedAt)
}

func (r *ScheduleRepository) Delete(ctx context.Context, id string) error {
	query := `DELETE FROM scheduled_tasks WHERE id = $1`
	_, err := r.db.ExecContext(ctx, query, id)
	return err
}

func (r *ScheduleRepository) UpdateLastRun(ctx context.Context, id string, lastRun time.Time, nextRun *time.Time) error {
	query := `UPDATE scheduled_tasks SET last_run = $2, next_run = $3, updated_at = NOW() WHERE id = $1`
	_, err := r.db.ExecContext(ctx, query, id, lastRun, nextRun)
	return err
}
