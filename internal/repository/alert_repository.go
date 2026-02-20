package repository

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"CampusMonitorAPI/internal/models"
)

// IAlertRepository defines the operations for managing network alerts.
type IAlertRepository interface {
	Create(ctx context.Context, alert *models.Alert) error
	GetByID(ctx context.Context, id uint) (*models.Alert, error)
	GetActiveByProbe(ctx context.Context, probeID string) ([]models.Alert, error)
	GetHistory(ctx context.Context, limit int, offset int) ([]models.Alert, error)
	Acknowledge(ctx context.Context, id uint) error
	Resolve(ctx context.Context, id uint) error
	Delete(ctx context.Context, id uint) error
	DeleteOld(ctx context.Context, olderThan time.Duration) (int64, error)
	GetStatistics(ctx context.Context) (map[string]int, error)
}

type AlertRepository struct {
	db *sql.DB
}

func NewAlertRepository(db *sql.DB) *AlertRepository {
	return &AlertRepository{db: db}
}

// Create inserts a new alert record and returns the generated ID and timestamp.
func (r *AlertRepository) Create(ctx context.Context, alert *models.Alert) error {
	query := `
		INSERT INTO alerts (
			probe_id, category, severity, metric_key, message, 
			status, occurrences, threshold_value, actual_value, 
			created_at, updated_at
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)
		RETURNING id
	`

	now := time.Now()
	alert.CreatedAt = now
	alert.UpdatedAt = now
	alert.Status = models.StatusActive

	err := r.db.QueryRowContext(
		ctx, query,
		alert.ProbeID,
		alert.Category,
		alert.Severity,
		alert.MetricKey,
		alert.Message,
		alert.Status,
		alert.Occurrences,
		alert.ThresholdValue,
		alert.ActualValue,
		alert.CreatedAt,
		alert.UpdatedAt,
	).Scan(&alert.ID)

	if err != nil {
		return fmt.Errorf("failed to create alert: %w", err)
	}

	return nil
}

// GetByID retrieves a single alert by its primary key.
func (r *AlertRepository) GetByID(ctx context.Context, id uint) (*models.Alert, error) {
	query := `
		SELECT id, probe_id, category, severity, metric_key, message, 
		       status, occurrences, threshold_value, actual_value, 
		       created_at, updated_at
		FROM alerts
		WHERE id = $1
	`

	alert := &models.Alert{}
	err := r.db.QueryRowContext(ctx, query, id).Scan(
		&alert.ID,
		&alert.ProbeID,
		&alert.Category,
		&alert.Severity,
		&alert.MetricKey,
		&alert.Message,
		&alert.Status,
		&alert.Occurrences,
		&alert.ThresholdValue,
		&alert.ActualValue,
		&alert.CreatedAt,
		&alert.UpdatedAt,
	)

	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get alert by id: %w", err)
	}

	return alert, nil
}

// GetActiveByProbe returns all alerts for a probe that are not resolved.
func (r *AlertRepository) GetActiveByProbe(ctx context.Context, probeID string) ([]models.Alert, error) {
	query := `
		SELECT id, probe_id, category, severity, metric_key, message, 
		       status, occurrences, threshold_value, actual_value, 
		       created_at, updated_at
		FROM alerts
		WHERE probe_id = $1 AND status != $2
		ORDER BY created_at DESC
	`

	rows, err := r.db.QueryContext(ctx, query, probeID, models.StatusResolved)
	if err != nil {
		return nil, fmt.Errorf("failed to query active alerts: %w", err)
	}
	defer rows.Close()

	var alerts []models.Alert
	for rows.Next() {
		var a models.Alert
		err := rows.Scan(
			&a.ID, &a.ProbeID, &a.Category, &a.Severity, &a.MetricKey, &a.Message,
			&a.Status, &a.Occurrences, &a.ThresholdValue, &a.ActualValue,
			&a.CreatedAt, &a.UpdatedAt,
		)
		if err != nil {
			return nil, err
		}
		alerts = append(alerts, a)
	}

	return alerts, nil
}

// GetHistory returns a paginated list of all alerts.
func (r *AlertRepository) GetHistory(ctx context.Context, limit int, offset int) ([]models.Alert, error) {
	query := `
		SELECT id, probe_id, category, severity, metric_key, message, 
		       status, occurrences, threshold_value, actual_value, 
		       created_at, updated_at
		FROM alerts
		ORDER BY created_at DESC
		LIMIT $1 OFFSET $2
	`

	rows, err := r.db.QueryContext(ctx, query, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("failed to query alert history: %w", err)
	}
	defer rows.Close()

	var alerts []models.Alert
	for rows.Next() {
		var a models.Alert
		err := rows.Scan(
			&a.ID, &a.ProbeID, &a.Category, &a.Severity, &a.MetricKey, &a.Message,
			&a.Status, &a.Occurrences, &a.ThresholdValue, &a.ActualValue,
			&a.CreatedAt, &a.UpdatedAt,
		)
		if err != nil {
			return nil, err
		}
		alerts = append(alerts, a)
	}
	return alerts, nil
}

// Acknowledge marks an alert as acknowledged.
func (r *AlertRepository) Acknowledge(ctx context.Context, id uint) error {
	query := `UPDATE alerts SET status = $1, updated_at = $2 WHERE id = $3`
	_, err := r.db.ExecContext(ctx, query, models.StatusAcknowledged, time.Now(), id)
	return err
}

// Resolve marks an alert as resolved.
func (r *AlertRepository) Resolve(ctx context.Context, id uint) error {
	query := `UPDATE alerts SET status = $1, updated_at = $2 WHERE id = $3`
	_, err := r.db.ExecContext(ctx, query, models.StatusResolved, time.Now(), id)
	return err
}

// Delete removes an alert record from the database.
func (r *AlertRepository) Delete(ctx context.Context, id uint) error {
	query := `DELETE FROM alerts WHERE id = $1`
	_, err := r.db.ExecContext(ctx, query, id)
	return err
}

// DeleteOld removes resolved alerts older than the specified duration.
func (r *AlertRepository) DeleteOld(ctx context.Context, olderThan time.Duration) (int64, error) {
	query := `DELETE FROM alerts WHERE status = $1 AND updated_at < $2`
	cutoff := time.Now().Add(-olderThan)
	result, err := r.db.ExecContext(ctx, query, models.StatusResolved, cutoff)
	if err != nil {
		return 0, err
	}
	return result.RowsAffected()
}

// GetStatistics returns a count of active alerts grouped by severity.
func (r *AlertRepository) GetStatistics(ctx context.Context) (map[string]int, error) {
	query := `
		SELECT severity, COUNT(*) 
		FROM alerts 
		WHERE status != $1 
		GROUP BY severity
	`
	rows, err := r.db.QueryContext(ctx, query, models.StatusResolved)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	stats := make(map[string]int)
	for rows.Next() {
		var sev string
		var count int
		if err := rows.Scan(&sev, &count); err != nil {
			return nil, err
		}
		stats[sev] = count
	}
	return stats, nil
}
