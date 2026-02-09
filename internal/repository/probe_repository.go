package repository

import (
	"CampusMonitorAPI/internal/models"
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	"github.com/lib/pq"
)

type ProbeRepository struct {
	db *sql.DB
}

func NewProbeRepository(db *sql.DB) *ProbeRepository {
	return &ProbeRepository{db: db}
}

func (r *ProbeRepository) Create(ctx context.Context, probe *models.Probe) error {
	query := `
		INSERT INTO probes (
			probe_id, location, building, floor, department, 
			status, firmware_version, last_seen, metadata
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
		RETURNING created_at, updated_at
	`

	err := r.db.QueryRowContext(
		ctx, query,
		probe.ProbeID,
		probe.Location,
		probe.Building,
		probe.Floor,
		probe.Department,
		probe.Status,
		probe.FirmwareVersion,
		probe.LastSeen,
		probe.Metadata,
	).Scan(&probe.CreatedAt, &probe.UpdatedAt)

	if err != nil {
		if pqErr, ok := err.(*pq.Error); ok && pqErr.Code == "23505" {
			return fmt.Errorf("probe with ID %s already exists", probe.ProbeID)
		}
		return fmt.Errorf("failed to create probe: %w", err)
	}

	return nil
}

func (r *ProbeRepository) GetByID(ctx context.Context, probeID string) (*models.Probe, error) {
	query := `
        SELECT probe_id, location, building, floor, department, 
               status, firmware_version, last_seen, created_at, updated_at, metadata
        FROM probes
        WHERE probe_id = $1`

	var probe models.Probe
	var metadataJSON []byte

	err := r.db.QueryRowContext(ctx, query, probeID).Scan(
		&probe.ProbeID,
		&probe.Location,
		&probe.Building,
		&probe.Floor,
		&probe.Department,
		&probe.Status,
		&probe.FirmwareVersion,
		&probe.LastSeen,
		&probe.CreatedAt,
		&probe.UpdatedAt,
		&metadataJSON,
	)

	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("probe not found")
		}
		return nil, fmt.Errorf("failed to scan probe: %w", err)
	}

	if len(metadataJSON) > 0 {
		if err := json.Unmarshal(metadataJSON, &probe.Metadata); err != nil {
			return nil, fmt.Errorf("failed to unmarshal metadata: %w", err)
		}
	}

	return &probe, nil
}

func (r *ProbeRepository) GetAll(ctx context.Context) ([]models.Probe, error) {
	query := `
		SELECT probe_id, location, building, floor, department, 
			   status, firmware_version, last_seen, 
			   created_at, updated_at, metadata
		FROM probes
		ORDER BY created_at DESC
	`

	rows, err := r.db.QueryContext(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("failed to query probes: %w", err)
	}
	defer rows.Close()

	probes := []models.Probe{}
	for rows.Next() {
		var probe models.Probe
		var metadataJSON []byte

		err := rows.Scan(
			&probe.ProbeID,
			&probe.Location,
			&probe.Building,
			&probe.Floor,
			&probe.Department,
			&probe.Status,
			&probe.FirmwareVersion,
			&probe.LastSeen,
			&probe.CreatedAt,
			&probe.UpdatedAt,
			&metadataJSON,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan probe: %w", err)
		}
		if len(metadataJSON) > 0 {
			if err := json.Unmarshal(metadataJSON, &probe.Metadata); err != nil {
				return nil, fmt.Errorf("failed to unmarshal metadata: %w", err)
			}
		}

		probes = append(probes, probe)
	}

	return probes, nil
}
func (r *ProbeRepository) Update(ctx context.Context, probeID string, updates *models.UpdateProbeRequest) error {
	query := `
       UPDATE probes
       SET location = COALESCE($2, location),
           building = COALESCE($3, building),
           floor = COALESCE($4, floor),
           department = COALESCE($5, department),
           status = COALESCE($6, status),
           metadata = COALESCE($7, metadata),
           updated_at = NOW()
       WHERE probe_id = $1
    `
	var metadataArg interface{}

	if updates.Metadata != nil {
		metadataJSON, err := json.Marshal(updates.Metadata)
		if err != nil {
			return fmt.Errorf("failed to marshal metadata updates: %w", err)
		}
		metadataArg = metadataJSON
	} else {
		metadataArg = nil
	}

	result, err := r.db.ExecContext(
		ctx, query,
		probeID,
		updates.Location,
		updates.Building,
		updates.Floor,
		updates.Department,
		updates.Status,
		metadataArg,
	)

	if err != nil {
		return fmt.Errorf("failed to update probe: %w", err)
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get affected rows: %w", err)
	}

	if rows == 0 {
		return fmt.Errorf("probe %s not found", probeID)
	}

	return nil
}

func (r *ProbeRepository) Delete(ctx context.Context, probeID string) error {
	query := `DELETE FROM probes WHERE probe_id = $1`

	result, err := r.db.ExecContext(ctx, query, probeID)
	if err != nil {
		return fmt.Errorf("failed to delete probe: %w", err)
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get affected rows: %w", err)
	}

	if rows == 0 {
		return fmt.Errorf("probe %s not found", probeID)
	}

	return nil
}

func (r *ProbeRepository) UpdateLastSeen(ctx context.Context, probeID string, timestamp time.Time) error {
	query := `
		UPDATE probes
		SET last_seen = $2, updated_at = NOW()
		WHERE probe_id = $1
	`

	_, err := r.db.ExecContext(ctx, query, probeID, timestamp)
	if err != nil {
		return fmt.Errorf("failed to update last_seen: %w", err)
	}

	return nil
}

func (r *ProbeRepository) GetActive(ctx context.Context) ([]models.Probe, error) {
	query := `
		SELECT probe_id, location, building, floor, department, 
			   status, firmware_version, last_seen, 
			   created_at, updated_at, metadata
		FROM probes
		WHERE status = 'active'
		ORDER BY last_seen DESC
	`

	rows, err := r.db.QueryContext(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("failed to query active probes: %w", err)
	}
	defer rows.Close()

	probes := []models.Probe{}
	for rows.Next() {
		var probe models.Probe
		err := rows.Scan(
			&probe.ProbeID,
			&probe.Location,
			&probe.Building,
			&probe.Floor,
			&probe.Department,
			&probe.Status,
			&probe.FirmwareVersion,
			&probe.LastSeen,
			&probe.CreatedAt,
			&probe.UpdatedAt,
			&probe.Metadata,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan probe: %w", err)
		}
		probes = append(probes, probe)
	}

	return probes, nil
}

func (r *ProbeRepository) GetByBuilding(ctx context.Context, building string) ([]models.Probe, error) {
	query := `
		SELECT probe_id, location, building, floor, department, 
			   status, firmware_version, last_seen, 
			   created_at, updated_at, metadata
		FROM probes
		WHERE building = $1
		ORDER BY floor, location
	`

	rows, err := r.db.QueryContext(ctx, query, building)
	if err != nil {
		return nil, fmt.Errorf("failed to query probes by building: %w", err)
	}
	defer rows.Close()

	probes := []models.Probe{}
	for rows.Next() {
		var probe models.Probe
		err := rows.Scan(
			&probe.ProbeID,
			&probe.Location,
			&probe.Building,
			&probe.Floor,
			&probe.Department,
			&probe.Status,
			&probe.FirmwareVersion,
			&probe.LastSeen,
			&probe.CreatedAt,
			&probe.UpdatedAt,
			&probe.Metadata,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan probe: %w", err)
		}
		probes = append(probes, probe)
	}

	return probes, nil
}

func (r *ProbeRepository) UpdateFirmwareVersion(ctx context.Context, probeID, version string) error {
	query := `
		UPDATE probes
		SET firmware_version = $2, updated_at = NOW()
		WHERE probe_id = $1
	`

	result, err := r.db.ExecContext(ctx, query, probeID, version)
	if err != nil {
		return fmt.Errorf("failed to update firmware version: %w", err)
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get affected rows: %w", err)
	}

	if rows == 0 {
		return fmt.Errorf("probe %s not found", probeID)
	}

	return nil
}

func (r *ProbeRepository) GetStale(ctx context.Context, threshold time.Duration) ([]models.Probe, error) {
	query := `
		SELECT probe_id, location, building, floor, department, 
			   status, firmware_version, last_seen, 
			   created_at, updated_at, metadata
		FROM probes
		WHERE last_seen < $1 AND status = 'active'
		ORDER BY last_seen ASC
	`

	staleTime := time.Now().Add(-threshold)
	rows, err := r.db.QueryContext(ctx, query, staleTime)
	if err != nil {
		return nil, fmt.Errorf("failed to query stale probes: %w", err)
	}
	defer rows.Close()

	probes := []models.Probe{}
	for rows.Next() {
		var probe models.Probe
		err := rows.Scan(
			&probe.ProbeID,
			&probe.Location,
			&probe.Building,
			&probe.Floor,
			&probe.Department,
			&probe.Status,
			&probe.FirmwareVersion,
			&probe.LastSeen,
			&probe.CreatedAt,
			&probe.UpdatedAt,
			&probe.Metadata,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan probe: %w", err)
		}
		probes = append(probes, probe)
	}

	return probes, nil
}

func (r *ProbeRepository) AutoDiscover(ctx context.Context, probeID string) error {
	query := `
		INSERT INTO probes (
			probe_id, status, location, building, floor, department, 
			firmware_version, last_seen, created_at, updated_at
		) VALUES (
			$1, 'pending', 'unknown', 'unknown', 'unknown', 'unknown', 
			'unknown', NOW(), NOW(), NOW()
		)
		ON CONFLICT (probe_id) DO NOTHING
	`

	_, err := r.db.ExecContext(ctx, query, probeID)
	if err != nil {
		return fmt.Errorf("failed to auto-discover probe: %w", err)
	}

	return nil
}
