package repository

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	"CampusMonitorAPI/internal/models"
)

type AlertRepository struct {
	db *sql.DB
}

func NewAlertRepository(db *sql.DB) *AlertRepository {
	return &AlertRepository{db: db}
}

func (r *AlertRepository) Create(ctx context.Context, alert *models.Alert) error {
	query := `
		INSERT INTO alerts (
			probe_id, alert_type, severity, message,
			threshold_value, actual_value, metadata
		) VALUES ($1, $2, $3, $4, $5, $6, $7)
		RETURNING id, triggered_at
	`

	err := r.db.QueryRowContext(
		ctx, query,
		alert.ProbeID,
		alert.AlertType,
		alert.Severity,
		alert.Message,
		alert.ThresholdValue,
		alert.ActualValue,
		alert.Metadata,
	).Scan(&alert.ID, &alert.TriggeredAt)

	if err != nil {
		return fmt.Errorf("failed to create alert: %w", err)
	}

	return nil
}

func (r *AlertRepository) GetByID(ctx context.Context, alertID int) (*models.Alert, error) {
	query := `
		SELECT id, probe_id, alert_type, severity, message,
		       threshold_value, actual_value, triggered_at,
		       resolved_at, acknowledged, metadata
		FROM alerts
		WHERE id = $1
	`

	alert := &models.Alert{}
	err := r.db.QueryRowContext(ctx, query, alertID).Scan(
		&alert.ID,
		&alert.ProbeID,
		&alert.AlertType,
		&alert.Severity,
		&alert.Message,
		&alert.ThresholdValue,
		&alert.ActualValue,
		&alert.TriggeredAt,
		&alert.ResolvedAt,
		&alert.Acknowledged,
		&alert.Metadata,
	)

	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("alert %d not found", alertID)
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get alert: %w", err)
	}

	return alert, nil
}

func (r *AlertRepository) GetByProbeID(ctx context.Context, probeID string, limit int) ([]models.Alert, error) {
	query := `
		SELECT id, probe_id, alert_type, severity, message,
		       threshold_value, actual_value, triggered_at,
		       resolved_at, acknowledged, metadata
		FROM alerts
		WHERE probe_id = $1
		ORDER BY triggered_at DESC
		LIMIT $2
	`

	rows, err := r.db.QueryContext(ctx, query, probeID, limit)
	if err != nil {
		return nil, fmt.Errorf("failed to query alerts: %w", err)
	}
	defer rows.Close()

	alerts := []models.Alert{}
	for rows.Next() {
		var alert models.Alert
		err := rows.Scan(
			&alert.ID,
			&alert.ProbeID,
			&alert.AlertType,
			&alert.Severity,
			&alert.Message,
			&alert.ThresholdValue,
			&alert.ActualValue,
			&alert.TriggeredAt,
			&alert.ResolvedAt,
			&alert.Acknowledged,
			&alert.Metadata,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan alert: %w", err)
		}
		alerts = append(alerts, alert)
	}

	return alerts, nil
}

func (r *AlertRepository) GetUnresolved(ctx context.Context) ([]models.Alert, error) {
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
		return nil, fmt.Errorf("failed to query unresolved alerts: %w", err)
	}
	defer rows.Close()

	alerts := []models.Alert{}
	for rows.Next() {
		var alert models.Alert
		err := rows.Scan(
			&alert.ID,
			&alert.ProbeID,
			&alert.AlertType,
			&alert.Severity,
			&alert.Message,
			&alert.ThresholdValue,
			&alert.ActualValue,
			&alert.TriggeredAt,
			&alert.ResolvedAt,
			&alert.Acknowledged,
			&alert.Metadata,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan alert: %w", err)
		}
		alerts = append(alerts, alert)
	}

	return alerts, nil
}

func (r *AlertRepository) Resolve(ctx context.Context, alertID int) error {
	query := `
		UPDATE alerts
		SET resolved_at = NOW()
		WHERE id = $1 AND resolved_at IS NULL
	`

	result, err := r.db.ExecContext(ctx, query, alertID)
	if err != nil {
		return fmt.Errorf("failed to resolve alert: %w", err)
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get affected rows: %w", err)
	}

	if rows == 0 {
		return fmt.Errorf("alert %d not found or already resolved", alertID)
	}

	return nil
}

func (r *AlertRepository) Acknowledge(ctx context.Context, alertID int) error {
	query := `
		UPDATE alerts
		SET acknowledged = true
		WHERE id = $1
	`

	result, err := r.db.ExecContext(ctx, query, alertID)
	if err != nil {
		return fmt.Errorf("failed to acknowledge alert: %w", err)
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get affected rows: %w", err)
	}

	if rows == 0 {
		return fmt.Errorf("alert %d not found", alertID)
	}

	return nil
}
func (r *AlertRepository) GetActiveByProbe(ctx context.Context, probeID string) ([]models.Alert, error) {
	// An alert is "Active" if it hasn't been resolved yet (resolved_at IS NULL)
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
			&a.ID,
			&a.ProbeID,
			&a.AlertType,
			&a.Severity,
			&a.Message,
			&a.ThresholdValue,
			&a.ActualValue,
			&a.TriggeredAt,
			&a.ResolvedAt,
			&a.Acknowledged,
			&metadataJSON,
		)
		if err != nil {
			return nil, err
		}

		if len(metadataJSON) > 0 {
			_ = json.Unmarshal(metadataJSON, &a.Metadata)
		}

		alerts = append(alerts, a)
	}

	if err = rows.Err(); err != nil {
		return nil, err
	}

	return alerts, nil
}
func (r *AlertRepository) GetBySeverity(ctx context.Context, severity string, limit int) ([]models.Alert, error) {
	query := `
		SELECT id, probe_id, alert_type, severity, message,
		       threshold_value, actual_value, triggered_at,
		       resolved_at, acknowledged, metadata
		FROM alerts
		WHERE severity = $1 AND resolved_at IS NULL
		ORDER BY triggered_at DESC
		LIMIT $2
	`

	rows, err := r.db.QueryContext(ctx, query, severity, limit)
	if err != nil {
		return nil, fmt.Errorf("failed to query alerts by severity: %w", err)
	}
	defer rows.Close()

	alerts := []models.Alert{}
	for rows.Next() {
		var alert models.Alert
		err := rows.Scan(
			&alert.ID,
			&alert.ProbeID,
			&alert.AlertType,
			&alert.Severity,
			&alert.Message,
			&alert.ThresholdValue,
			&alert.ActualValue,
			&alert.TriggeredAt,
			&alert.ResolvedAt,
			&alert.Acknowledged,
			&alert.Metadata,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan alert: %w", err)
		}
		alerts = append(alerts, alert)
	}

	return alerts, nil
}

func (r *AlertRepository) DeleteOld(ctx context.Context, olderThan time.Duration) (int64, error) {
	query := `
		DELETE FROM alerts
		WHERE triggered_at < $1
		  AND resolved_at IS NOT NULL
	`

	cutoff := time.Now().Add(-olderThan)
	result, err := r.db.ExecContext(ctx, query, cutoff)
	if err != nil {
		return 0, fmt.Errorf("failed to delete old alerts: %w", err)
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return 0, fmt.Errorf("failed to get affected rows: %w", err)
	}

	return rows, nil
}

func (r *AlertRepository) GetStatistics(ctx context.Context) (map[string]int, error) {
	query := `
		SELECT 
			severity,
			COUNT(*) as count
		FROM alerts
		WHERE resolved_at IS NULL
		GROUP BY severity
	`

	rows, err := r.db.QueryContext(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("failed to get alert statistics: %w", err)
	}
	defer rows.Close()

	stats := make(map[string]int)
	for rows.Next() {
		var severity string
		var count int
		if err := rows.Scan(&severity, &count); err != nil {
			return nil, fmt.Errorf("failed to scan statistics: %w", err)
		}
		stats[severity] = count
	}

	return stats, nil
}
