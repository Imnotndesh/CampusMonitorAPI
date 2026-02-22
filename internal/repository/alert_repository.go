package repository

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	"CampusMonitorAPI/internal/models"
)

// IAlertRepository defines the operations for managing network alerts.
type IAlertRepository interface {
	Create(ctx context.Context, alert *models.Alert) error
	GetByID(ctx context.Context, id uint) (*models.Alert, error)
	GetActive(ctx context.Context) ([]models.Alert, error) // Added this method!
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

// Create inserts a new alert record into the database.
func (r *AlertRepository) Create(ctx context.Context, alert *models.Alert) error {
	var metadataJSON []byte
	var err error
	if alert.Metadata != nil {
		metadataJSON, err = json.Marshal(alert.Metadata)
		if err != nil {
			return err
		}
	}

	query := `
		INSERT INTO alerts (
			probe_id, alert_type, severity, message, 
			threshold_value, actual_value, triggered_at, 
			resolved_at, acknowledged, metadata
		) VALUES ($1, $2, $3, $4, $5, $6, COALESCE($7, now()), $8, $9, $10)
		RETURNING id, triggered_at
	`

	var triggeredAt time.Time
	if alert.TriggeredAt.IsZero() {
		triggeredAt = time.Now()
	} else {
		triggeredAt = alert.TriggeredAt
	}

	err = r.db.QueryRowContext(ctx, query,
		alert.ProbeID,
		alert.AlertType,
		alert.Severity,
		alert.Message,
		alert.ThresholdValue,
		alert.ActualValue,
		triggeredAt,
		alert.ResolvedAt,
		alert.Acknowledged,
		metadataJSON,
	).Scan(&alert.ID, &alert.TriggeredAt)

	return err
}

func (r *AlertRepository) GetByID(ctx context.Context, id uint) (*models.Alert, error) {
	query := `
		SELECT id, probe_id, alert_type, severity, message, 
		       threshold_value, actual_value, triggered_at, 
		       resolved_at, acknowledged, metadata
		FROM alerts
		WHERE id = $1
	`

	var a models.Alert
	var metadataJSON []byte

	err := r.db.QueryRowContext(ctx, query, id).Scan(
		&a.ID, &a.ProbeID, &a.AlertType, &a.Severity, &a.Message,
		&a.ThresholdValue, &a.ActualValue, &a.TriggeredAt,
		&a.ResolvedAt, &a.Acknowledged, &metadataJSON,
	)
	if err != nil {
		return nil, err
	}

	if len(metadataJSON) > 0 {
		_ = json.Unmarshal(metadataJSON, &a.Metadata)
	}

	return &a, nil
}

// GetActive fetches ALL unresolved alerts across the entire system
func (r *AlertRepository) GetActive(ctx context.Context) ([]models.Alert, error) {
	query := `
		SELECT id, probe_id, alert_type, severity, message, 
		       threshold_value, actual_value, triggered_at, 
		       resolved_at, acknowledged, metadata
		FROM alerts
		WHERE resolved_at IS NULL
		ORDER BY triggered_at DESC
	`

	rows, err := r.db.QueryContext(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("failed to query active alerts: %w", err)
	}
	defer rows.Close()

	var alerts []models.Alert
	for rows.Next() {
		var a models.Alert
		var metadataJSON []byte

		err := rows.Scan(
			&a.ID, &a.ProbeID, &a.AlertType, &a.Severity, &a.Message,
			&a.ThresholdValue, &a.ActualValue, &a.TriggeredAt,
			&a.ResolvedAt, &a.Acknowledged, &metadataJSON,
		)
		if err != nil {
			return nil, err
		}

		if len(metadataJSON) > 0 {
			_ = json.Unmarshal(metadataJSON, &a.Metadata)
		}
		alerts = append(alerts, a)
	}
	return alerts, rows.Err()
}

// GetActiveByProbe fetches unresolved alerts for a SPECIFIC probe
func (r *AlertRepository) GetActiveByProbe(ctx context.Context, probeID string) ([]models.Alert, error) {
	query := `
		SELECT id, probe_id, alert_type, severity, message, 
		       threshold_value, actual_value, triggered_at, 
		       resolved_at, acknowledged, metadata
		FROM alerts
		WHERE probe_id = $1 AND resolved_at IS NULL
		ORDER BY triggered_at DESC
	`

	rows, err := r.db.QueryContext(ctx, query, probeID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var alerts []models.Alert
	for rows.Next() {
		var a models.Alert
		var metadataJSON []byte

		err := rows.Scan(
			&a.ID, &a.ProbeID, &a.AlertType, &a.Severity, &a.Message,
			&a.ThresholdValue, &a.ActualValue, &a.TriggeredAt,
			&a.ResolvedAt, &a.Acknowledged, &metadataJSON,
		)
		if err != nil {
			return nil, err
		}

		if len(metadataJSON) > 0 {
			_ = json.Unmarshal(metadataJSON, &a.Metadata)
		}
		alerts = append(alerts, a)
	}
	return alerts, rows.Err()
}

// GetHistory fetches all alerts (both active and resolved)
func (r *AlertRepository) GetHistory(ctx context.Context, limit int, offset int) ([]models.Alert, error) {
	query := `
		SELECT id, probe_id, alert_type, severity, message, 
		       threshold_value, actual_value, triggered_at, 
		       resolved_at, acknowledged, metadata
		FROM alerts
		ORDER BY triggered_at DESC
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
		var metadataJSON []byte

		err := rows.Scan(
			&a.ID, &a.ProbeID, &a.AlertType, &a.Severity, &a.Message,
			&a.ThresholdValue, &a.ActualValue, &a.TriggeredAt,
			&a.ResolvedAt, &a.Acknowledged, &metadataJSON,
		)
		if err != nil {
			return nil, err
		}

		if len(metadataJSON) > 0 {
			_ = json.Unmarshal(metadataJSON, &a.Metadata)
		}
		alerts = append(alerts, a)
	}
	return alerts, rows.Err()
}

func (r *AlertRepository) Acknowledge(ctx context.Context, id uint) error {
	query := `UPDATE alerts SET acknowledged = true WHERE id = $1`
	_, err := r.db.ExecContext(ctx, query, id)
	return err
}

func (r *AlertRepository) Resolve(ctx context.Context, id uint) error {
	// Instead of 'status = RESOLVED', we set the 'resolved_at' timestamp
	query := `UPDATE alerts SET resolved_at = $1 WHERE id = $2`
	_, err := r.db.ExecContext(ctx, query, time.Now(), id)
	return err
}

func (r *AlertRepository) Delete(ctx context.Context, id uint) error {
	query := `DELETE FROM alerts WHERE id = $1`
	_, err := r.db.ExecContext(ctx, query, id)
	return err
}

func (r *AlertRepository) DeleteOld(ctx context.Context, olderThan time.Duration) (int64, error) {
	query := `DELETE FROM alerts WHERE resolved_at IS NOT NULL AND resolved_at < $1`
	cutoff := time.Now().Add(-olderThan)
	result, err := r.db.ExecContext(ctx, query, cutoff)
	if err != nil {
		return 0, err
	}
	return result.RowsAffected()
}

func (r *AlertRepository) GetStatistics(ctx context.Context) (map[string]int, error) {
	query := `
		SELECT severity, COUNT(*) 
		FROM alerts 
		WHERE resolved_at IS NULL 
		GROUP BY severity
	`
	rows, err := r.db.QueryContext(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	stats := make(map[string]int)
	for rows.Next() {
		var severity string
		var count int
		if err := rows.Scan(&severity, &count); err != nil {
			return nil, err
		}
		stats[severity] = count
	}
	return stats, nil
}
