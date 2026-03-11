package repository

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"CampusMonitorAPI/internal/models"

	"github.com/lib/pq"
)

type FleetRepository struct {
	db *sql.DB
}

func (r *FleetRepository) DB() *sql.DB {
	return r.db
}
func NewFleetRepository(db *sql.DB) *FleetRepository {
	return &FleetRepository{db: db}
}

func (r *FleetRepository) EnrollProbe(ctx context.Context, probeID string, req *models.FleetEnrollRequest, user string) error {
	query := `
		INSERT INTO fleet_probes (
			probe_id, managed, managed_since, managed_by, groups, location, tags,
			config_template_id, maintenance_window, auto_update_enabled,
			current_firmware, created_at, updated_at
		) VALUES ($1, true, NOW(), $2, $3, $4, $5, $6, $7, $8, 
			-- FIX: Changed $1 to $9 to completely isolate the parameter inference
			COALESCE((SELECT firmware_version FROM probes WHERE probe_id = $9), 'unknown'),
			NOW(), NOW())
		ON CONFLICT (probe_id) DO UPDATE SET
			managed = true,
			managed_since = COALESCE(fleet_probes.managed_since, NOW()),
			managed_by = $2,
			groups = $3,
			location = $4,
			tags = $5,
			config_template_id = $6,
			maintenance_window = $7,
			auto_update_enabled = $8,
			updated_at = NOW()
	`

	groupsJSON, _ := json.Marshal(req.Groups)
	tagsJSON, _ := json.Marshal(req.Tags)
	maintWindowJSON, _ := json.Marshal(req.MaintenanceWindow)

	_, err := r.db.ExecContext(ctx, query,
		probeID,               // $1
		user,                  // $2
		groupsJSON,            // $3
		req.Location,          // $4
		tagsJSON,              // $5
		req.ConfigTemplateID,  // $6
		maintWindowJSON,       // $7
		req.AutoUpdateEnabled, // $8
		probeID,               // $9
	)

	return err
}

func (r *FleetRepository) UnenrollProbe(ctx context.Context, probeID string) error {
	query := `DELETE FROM fleet_probes WHERE probe_id = $1`
	_, err := r.db.ExecContext(ctx, query, probeID)
	return err
}

func (r *FleetRepository) GetFleetProbe(ctx context.Context, probeID string) (*models.FleetProbe, error) {
	query := `
		SELECT 
			fp.probe_id, fp.managed, fp.managed_since, fp.managed_by,
			fp.groups, fp.location, fp.tags, fp.config_version,
			fp.config_template_id, fp.maintenance_window, fp.auto_update_enabled,
			fp.last_command_id, fp.last_command_status, fp.last_command_time,
			fp.commands_received, fp.commands_completed, fp.commands_failed,
			fp.consecutive_failures, fp.current_firmware, fp.target_firmware,
			fp.last_ota_attempt, fp.ota_attempts, fp.created_at, fp.updated_at,
			p.status, p.last_seen
		FROM fleet_probes fp
		JOIN probes p ON fp.probe_id = p.probe_id
		WHERE fp.probe_id = $1
	`

	var fp models.FleetProbe
	var groupsJSON, tagsJSON, maintWindowJSON []byte
	var configTemplateID sql.NullInt64
	var lastCommandTime, lastOTA, lastSeen pq.NullTime
	var status sql.NullString
	var lastCmdID, lastCmdStatus, currFw, targetFw sql.NullString

	err := r.db.QueryRowContext(ctx, query, probeID).Scan(
		&fp.ProbeID, &fp.Managed, &fp.ManagedSince, &fp.ManagedBy,
		&groupsJSON, &fp.Location, &tagsJSON, &fp.ConfigVersion,
		&configTemplateID, &maintWindowJSON, &fp.AutoUpdateEnabled,
		&lastCmdID, &lastCmdStatus, &lastCommandTime,
		&fp.CommandsReceived, &fp.CommandsCompleted, &fp.CommandsFailed,
		&fp.ConsecutiveFailures, &currFw, &targetFw,
		&lastOTA, &fp.OTAAttempts, &fp.CreatedAt, &fp.UpdatedAt,
		&status, &lastSeen,
	)

	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, fmt.Errorf("fleet probe not found")
		}
		return nil, err
	}
	if lastCmdID.Valid {
		fp.LastCommandID = lastCmdID.String
	}
	if lastCmdStatus.Valid {
		fp.LastCommandStatus = lastCmdStatus.String
	}
	if currFw.Valid {
		fp.CurrentFirmware = currFw.String
	}
	if targetFw.Valid {
		fp.TargetFirmware = targetFw.String
	}

	if configTemplateID.Valid {
		tid := int(configTemplateID.Int64)
		fp.ConfigTemplateID = &tid
	}
	if lastCommandTime.Valid {
		fp.LastCommandTime = &lastCommandTime.Time
	}
	if lastOTA.Valid {
		fp.LastOTAAttempt = &lastOTA.Time
	}
	if lastSeen.Valid {
		fp.LastSeen = lastSeen.Time
	}
	if status.Valid {
		fp.Status = status.String
	}

	err = json.Unmarshal(groupsJSON, &fp.Groups)
	if err != nil {
		return nil, err
	}
	err = json.Unmarshal(tagsJSON, &fp.Tags)
	if err != nil {
		return nil, err
	}
	err = json.Unmarshal(maintWindowJSON, &fp.MaintenanceWindow)
	if err != nil {
		return nil, err
	}

	return &fp, nil
}

func (r *FleetRepository) ListFleetProbes(ctx context.Context, managedOnly bool, group string) ([]models.FleetProbe, error) {
	query := `
		SELECT 
			fp.probe_id, fp.managed, fp.managed_since, fp.managed_by,
			fp.groups, fp.location, fp.tags, fp.config_version,
			fp.config_template_id, fp.maintenance_window, fp.auto_update_enabled,
			fp.last_command_id, fp.last_command_status, fp.last_command_time,
			fp.commands_received, fp.commands_completed, fp.commands_failed,
			fp.consecutive_failures, fp.current_firmware, fp.target_firmware,
			fp.last_ota_attempt, fp.ota_attempts, fp.created_at, fp.updated_at,
			p.status, p.last_seen
		FROM fleet_probes fp
		JOIN probes p ON fp.probe_id = p.probe_id
		WHERE 1=1
	`

	var args []interface{}
	argCount := 1

	if managedOnly {
		query += " AND fp.managed = true"
	}

	if group != "" {
		query += fmt.Sprintf(" AND fp.groups @> $%d", argCount)
		groupJSON, _ := json.Marshal([]string{group})
		args = append(args, groupJSON)
		argCount++
	}

	query += " ORDER BY fp.updated_at DESC"

	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var probes []models.FleetProbe
	for rows.Next() {
		var fp models.FleetProbe
		var groupsJSON, tagsJSON, maintWindowJSON []byte
		var configTemplateID sql.NullInt64
		var lastCommandTime, lastOTA, lastSeen pq.NullTime
		var status sql.NullString
		var lastCmdID, lastCmdStatus, currFw, targetFw sql.NullString

		err := rows.Scan(
			&fp.ProbeID, &fp.Managed, &fp.ManagedSince, &fp.ManagedBy,
			&groupsJSON, &fp.Location, &tagsJSON, &fp.ConfigVersion,
			&configTemplateID, &maintWindowJSON, &fp.AutoUpdateEnabled,
			&lastCmdID, &lastCmdStatus, &lastCommandTime,
			&fp.CommandsReceived, &fp.CommandsCompleted, &fp.CommandsFailed,
			&fp.ConsecutiveFailures, &currFw, &targetFw,
			&lastOTA, &fp.OTAAttempts, &fp.CreatedAt, &fp.UpdatedAt,
			&status, &lastSeen,
		)
		if err != nil {
			return nil, err
		}

		if lastCmdID.Valid {
			fp.LastCommandID = lastCmdID.String
		}
		if lastCmdStatus.Valid {
			fp.LastCommandStatus = lastCmdStatus.String
		}
		if currFw.Valid {
			fp.CurrentFirmware = currFw.String
		}
		if targetFw.Valid {
			fp.TargetFirmware = targetFw.String
		}

		if configTemplateID.Valid {
			tid := int(configTemplateID.Int64)
			fp.ConfigTemplateID = &tid
		}
		if lastCommandTime.Valid {
			fp.LastCommandTime = &lastCommandTime.Time
		}
		if lastOTA.Valid {
			fp.LastOTAAttempt = &lastOTA.Time
		}
		if lastSeen.Valid {
			fp.LastSeen = lastSeen.Time
		}
		if status.Valid {
			fp.Status = status.String
		}

		err = json.Unmarshal(groupsJSON, &fp.Groups)
		if err != nil {
			return nil, err
		}
		json.Unmarshal(tagsJSON, &fp.Tags)
		json.Unmarshal(maintWindowJSON, &fp.MaintenanceWindow)

		probes = append(probes, fp)
	}

	return probes, nil
}

func (r *FleetRepository) UpdateFleetProbe(ctx context.Context, probeID string, req *models.FleetUpdateRequest) error {
	query := `
		UPDATE fleet_probes SET
			groups = COALESCE($2, groups),
			location = COALESCE($3, location),
			tags = COALESCE($4, tags),
			config_template_id = COALESCE($5, config_template_id),
			maintenance_window = COALESCE($6, maintenance_window),
			auto_update_enabled = COALESCE($7, auto_update_enabled),
			updated_at = NOW()
		WHERE probe_id = $1
	`

	var groupsJSON, tagsJSON, maintWindowJSON []byte
	if req.Groups != nil {
		groupsJSON, _ = json.Marshal(req.Groups)
	}
	if req.Tags != nil {
		tagsJSON, _ = json.Marshal(req.Tags)
	}
	if req.MaintenanceWindow != nil {
		maintWindowJSON, _ = json.Marshal(req.MaintenanceWindow)
	}

	_, err := r.db.ExecContext(ctx, query,
		probeID,
		groupsJSON,
		req.Location,
		tagsJSON,
		req.ConfigTemplateID,
		maintWindowJSON,
		req.AutoUpdateEnabled,
	)

	return err
}

func (r *FleetRepository) UpdateFleetCommandStats(ctx context.Context, probeID string, commandID string, status string) error {
	query := `
		UPDATE fleet_probes SET
			last_command_id = $2,
			last_command_status = $3::text,               -- explicit cast
			last_command_time = NOW(),
			commands_received = commands_received + 1,
			consecutive_failures = CASE WHEN $3::text = 'failed' THEN consecutive_failures + 1 ELSE 0 END,
			commands_completed = CASE WHEN $3::text = 'completed' THEN commands_completed + 1 ELSE commands_completed END,
			commands_failed = CASE WHEN $3::text = 'failed' THEN commands_failed + 1 ELSE commands_failed END,
			updated_at = NOW()
		WHERE probe_id = $1
	`

	_, err := r.db.ExecContext(ctx, query, probeID, commandID, status)
	return err
}

func (r *FleetRepository) UpdateFirmwareVersion(ctx context.Context, probeID, version string) error {
	query := `
		UPDATE fleet_probes SET
			current_firmware = $2,
			updated_at = NOW()
		WHERE probe_id = $1
	`

	_, err := r.db.ExecContext(ctx, query, probeID, version)
	return err
}

func (r *FleetRepository) SetTargetFirmware(ctx context.Context, probeID, version string) error {
	query := `
		UPDATE fleet_probes SET
			target_firmware = $2,
			last_ota_attempt = NOW(),
			ota_attempts = ota_attempts + 1,
			updated_at = NOW()
		WHERE probe_id = $1
	`

	_, err := r.db.ExecContext(ctx, query, probeID, version)
	return err
}

// Config Templates

func (r *FleetRepository) CreateTemplate(ctx context.Context, template *models.FleetConfigTemplate) error {
	query := `
		INSERT INTO fleet_templates (
			name, description, config, variables, created_by
		) VALUES ($1, $2, $3, $4, $5)
		RETURNING id, created_at
	`

	configJSON, _ := json.Marshal(template.Config)
	variablesJSON, _ := json.Marshal(template.Variables)

	err := r.db.QueryRowContext(ctx, query,
		template.Name,
		template.Description,
		configJSON,
		variablesJSON,
		template.CreatedBy,
	).Scan(&template.ID, &template.CreatedAt)

	return err
}

func (r *FleetRepository) GetTemplate(ctx context.Context, id int) (*models.FleetConfigTemplate, error) {
	query := `
		SELECT id, name, description, config, variables, created_by,
		       created_at, updated_at, usage_count
		FROM fleet_templates
		WHERE id = $1
	`

	var t models.FleetConfigTemplate
	var configJSON, variablesJSON []byte

	err := r.db.QueryRowContext(ctx, query, id).Scan(
		&t.ID, &t.Name, &t.Description, &configJSON, &variablesJSON,
		&t.CreatedBy, &t.CreatedAt, &t.UpdatedAt, &t.UsageCount,
	)

	if err != nil {
		return nil, err
	}

	json.Unmarshal(configJSON, &t.Config)
	json.Unmarshal(variablesJSON, &t.Variables)

	return &t, nil
}

func (r *FleetRepository) ListTemplates(ctx context.Context) ([]models.FleetConfigTemplate, error) {
	query := `
		SELECT id, name, description, created_by, created_at, updated_at, usage_count
		FROM fleet_templates
		ORDER BY usage_count DESC, name
	`

	rows, err := r.db.QueryContext(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var templates []models.FleetConfigTemplate
	for rows.Next() {
		var t models.FleetConfigTemplate
		err := rows.Scan(
			&t.ID, &t.Name, &t.Description, &t.CreatedBy,
			&t.CreatedAt, &t.UpdatedAt, &t.UsageCount,
		)
		if err != nil {
			return nil, err
		}
		templates = append(templates, t)
	}

	return templates, nil
}

func (r *FleetRepository) UpdateTemplateUsage(ctx context.Context, id int) error {
	query := `UPDATE fleet_templates SET usage_count = usage_count + 1 WHERE id = $1`
	_, err := r.db.ExecContext(ctx, query, id)
	return err
}

func (r *FleetRepository) DeleteTemplate(ctx context.Context, id int) error {
	// Check if template is in use
	var count int
	err := r.db.QueryRowContext(ctx,
		"SELECT COUNT(*) FROM fleet_probes WHERE config_template_id = $1", id,
	).Scan(&count)

	if err != nil {
		return err
	}

	if count > 0 {
		return fmt.Errorf("template is in use by %d probes", count)
	}

	_, err = r.db.ExecContext(ctx, "DELETE FROM fleet_templates WHERE id = $1", id)
	return err
}

// Fleet Commands

func (r *FleetRepository) CreateFleetCommand(ctx context.Context, cmd *models.FleetCommand) error {
	query := `
		INSERT INTO fleet_commands (
			id, command_type, payload, issued_by, target_groups, target_probes,
			total_targets, status, completion_threshold, timeout_seconds, scheduled_for,
			metadata
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12)
		RETURNING issued_at
	`

	id := generateCommandID()
	payloadJSON, _ := json.Marshal(cmd.Payload)
	groupsJSON, _ := json.Marshal(cmd.TargetGroups)
	probesJSON, _ := json.Marshal(cmd.TargetProbes)
	metadataJSON, _ := json.Marshal(cmd.Metadata)

	err := r.db.QueryRowContext(ctx, query,
		id, cmd.CommandType, payloadJSON, cmd.IssuedBy,
		groupsJSON, probesJSON, cmd.TotalTargets, cmd.Status,
		cmd.CompletionThreshold, cmd.TimeoutSeconds, cmd.ScheduledFor,
		metadataJSON,
	).Scan(&cmd.IssuedAt)

	if err != nil {
		return err
	}

	cmd.ID = id
	return nil
}

func (r *FleetRepository) GetFleetCommand(ctx context.Context, commandID string) (*models.FleetCommand, error) {
	query := `
		SELECT id, command_type, payload, issued_by, issued_at,
		       target_groups, target_probes, total_targets, status,
		       acks_received, completed_count, failed_count,
		       completion_threshold, timeout_seconds, scheduled_for,
		       metadata, completed_at
		FROM fleet_commands
		WHERE id = $1
	`

	var cmd models.FleetCommand
	var payloadJSON, groupsJSON, probesJSON, metadataJSON []byte
	var scheduledFor pq.NullTime
	var completedAt pq.NullTime

	err := r.db.QueryRowContext(ctx, query, commandID).Scan(
		&cmd.ID, &cmd.CommandType, &payloadJSON, &cmd.IssuedBy, &cmd.IssuedAt,
		&groupsJSON, &probesJSON, &cmd.TotalTargets, &cmd.Status,
		&cmd.AcksReceived, &cmd.CompletedCount, &cmd.FailedCount,
		&cmd.CompletionThreshold, &cmd.TimeoutSeconds, &scheduledFor,
		&metadataJSON, &completedAt,
	)

	if err != nil {
		return nil, err
	}

	err = json.Unmarshal(payloadJSON, &cmd.Payload)
	if err != nil {
		return nil, err
	}
	err = json.Unmarshal(groupsJSON, &cmd.TargetGroups)
	if err != nil {
		return nil, err
	}
	err = json.Unmarshal(probesJSON, &cmd.TargetProbes)
	if err != nil {
		return nil, err
	}
	err = json.Unmarshal(metadataJSON, &cmd.Metadata)
	if err != nil {
		return nil, err
	}

	if scheduledFor.Valid {
		cmd.ScheduledFor = &scheduledFor.Time
	}
	if completedAt.Valid {
		cmd.CompletedAt = &completedAt.Time
	}

	return &cmd, nil
}

func (r *FleetRepository) ListFleetCommands(ctx context.Context, status string, limit int) ([]models.FleetCommand, error) {
	query := `
		SELECT id, command_type, issued_by, issued_at, status,
		       total_targets, acks_received, completed_count, failed_count,
		       completed_at
		FROM fleet_commands
		WHERE 1=1
	`

	var args []interface{}
	argCount := 1

	if status != "" {
		query += fmt.Sprintf(" AND status = $%d", argCount)
		args = append(args, status)
		argCount++
	}

	query += fmt.Sprintf(" ORDER BY issued_at DESC LIMIT $%d", argCount)
	args = append(args, limit)

	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var commands []models.FleetCommand
	for rows.Next() {
		var cmd models.FleetCommand
		var completedAt pq.NullTime

		err := rows.Scan(
			&cmd.ID, &cmd.CommandType, &cmd.IssuedBy, &cmd.IssuedAt, &cmd.Status,
			&cmd.TotalTargets, &cmd.AcksReceived, &cmd.CompletedCount, &cmd.FailedCount,
			&completedAt,
		)
		if err != nil {
			return nil, err
		}

		if completedAt.Valid {
			cmd.CompletedAt = &completedAt.Time
		}

		commands = append(commands, cmd)
	}

	return commands, nil
}

func (r *FleetRepository) UpdateFleetCommandStatus(ctx context.Context, commandID string, stats map[string]int) error {
	query := `
		UPDATE fleet_commands SET
			acks_received = $2,
			completed_count = $3,
			failed_count = $4,
			status = CASE 
				WHEN $3::int + $4::int >= total_targets THEN 'completed'
				WHEN $2::int > 0 THEN 'in_progress'
				ELSE status
			END,
			completed_at = CASE 
				WHEN $3::int + $4::int >= total_targets THEN NOW()
				ELSE completed_at
			END
		WHERE id = $1
	`

	_, err := r.db.ExecContext(ctx, query,
		commandID,
		stats["acks_received"],
		stats["completed"],
		stats["failed"],
	)

	return err
}

// Per-probe command status

func (r *FleetRepository) SaveProbeCommandStatus(ctx context.Context, status *models.FleetCommandProbeStatus) error {
	query := `
		INSERT INTO fleet_command_probes (
			command_id, probe_id, status, result, error_message, retry_count
		) VALUES ($1, $2, $3, $4, $5, $6)
		ON CONFLICT (command_id, probe_id) DO UPDATE SET
			status = $3,
			result = $4,
			error_message = $5,
			retry_count = $6,
			sent_at = CASE WHEN $3 = 'sent' AND fleet_command_probes.sent_at IS NULL THEN NOW() ELSE fleet_command_probes.sent_at END,
			acknowledged_at = CASE WHEN $3 = 'acknowledged' THEN NOW() ELSE fleet_command_probes.acknowledged_at END,
			completed_at = CASE WHEN $3 IN ('completed', 'failed') THEN NOW() ELSE fleet_command_probes.completed_at END
	`

	resultJSON, _ := json.Marshal(status.Result)

	_, err := r.db.ExecContext(ctx, query,
		status.CommandID, status.ProbeID, status.Status, resultJSON,
		status.ErrorMessage, status.RetryCount,
	)

	return err
}

func (r *FleetRepository) GetProbeCommandStatus(ctx context.Context, commandID, probeID string) (*models.FleetCommandProbeStatus, error) {
	query := `
		SELECT command_id, probe_id, status, result, error_message,
		       retry_count, sent_at, acknowledged_at, completed_at
		FROM fleet_command_probes
		WHERE command_id = $1 AND probe_id = $2
	`

	var status models.FleetCommandProbeStatus
	var resultJSON []byte
	var sentAt, acknowledgedAt, completedAt pq.NullTime

	err := r.db.QueryRowContext(ctx, query, commandID, probeID).Scan(
		&status.CommandID, &status.ProbeID, &status.Status, &resultJSON,
		&status.ErrorMessage, &status.RetryCount,
		&sentAt, &acknowledgedAt, &completedAt,
	)

	if err != nil {
		return nil, err
	}

	err = json.Unmarshal(resultJSON, &status.Result)
	if err != nil {
		return nil, err
	}

	if sentAt.Valid {
		status.SentAt = &sentAt.Time
	}
	if acknowledgedAt.Valid {
		status.AcknowledgedAt = &acknowledgedAt.Time
	}
	if completedAt.Valid {
		status.CompletedAt = &completedAt.Time
	}

	return &status, nil
}

func (r *FleetRepository) ListProbeCommands(ctx context.Context, probeID string, limit int) ([]models.FleetCommandProbeStatus, error) {
	query := `
		SELECT fcp.command_id, fcp.probe_id, fcp.status, fcp.result,
		       fcp.error_message, fcp.sent_at, fcp.acknowledged_at, fcp.completed_at,
		       fc.command_type, fc.issued_at, fcp.retry_count
		FROM fleet_command_probes fcp
		JOIN fleet_commands fc ON fcp.command_id = fc.id
		WHERE fcp.probe_id = $1
		ORDER BY fc.issued_at DESC
		LIMIT $2
	`

	rows, err := r.db.QueryContext(ctx, query, probeID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var statuses []models.FleetCommandProbeStatus
	for rows.Next() {
		var status models.FleetCommandProbeStatus
		var resultJSON []byte
		var sentAt, acknowledgedAt, completedAt pq.NullTime

		err := rows.Scan(
			&status.CommandID, &status.ProbeID, &status.Status, &resultJSON,
			&status.ErrorMessage, &sentAt, &acknowledgedAt, &completedAt,
			&status.CommandType, &status.IssuedAt, &status.RetryCount,
		)
		if err != nil {
			return nil, err
		}

		json.Unmarshal(resultJSON, &status.Result)

		if sentAt.Valid {
			status.SentAt = &sentAt.Time
		}
		if acknowledgedAt.Valid {
			status.AcknowledgedAt = &acknowledgedAt.Time
		}
		if completedAt.Valid {
			status.CompletedAt = &completedAt.Time
		}

		statuses = append(statuses, status)
	}

	return statuses, nil
}

// Groups

func (r *FleetRepository) CreateGroup(ctx context.Context, group *models.FleetGroup) error {
	query := `
		INSERT INTO fleet_groups (id, name, description)
		VALUES ($1, $2, $3)
		RETURNING created_at
	`

	group.ID = generateGroupID(group.Name)

	err := r.db.QueryRowContext(ctx, query,
		group.ID, group.Name, group.Description,
	).Scan(&group.CreatedAt)

	return err
}

func (r *FleetRepository) ListGroups(ctx context.Context) ([]models.FleetGroup, error) {
	query := `
		SELECT g.id, g.name, g.description, g.created_at, g.updated_at,
		       COUNT(fp.probe_id) as probe_count
		FROM fleet_groups g
		LEFT JOIN fleet_probes fp ON fp.groups @> jsonb_build_array(g.id)
		GROUP BY g.id, g.name, g.description, g.created_at, g.updated_at
		ORDER BY g.name
	`

	rows, err := r.db.QueryContext(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var groups []models.FleetGroup
	for rows.Next() {
		var g models.FleetGroup
		err := rows.Scan(
			&g.ID, &g.Name, &g.Description, &g.CreatedAt, &g.UpdatedAt,
			&g.ProbeCount,
		)
		if err != nil {
			return nil, err
		}
		groups = append(groups, g)
	}

	return groups, nil
}

func (r *FleetRepository) DeleteGroup(ctx context.Context, groupID string) error {
	// Check if any probes are in this group
	var count int
	err := r.db.QueryRowContext(ctx,
		"SELECT COUNT(*) FROM fleet_probes WHERE groups @> jsonb_build_array($1)",
		groupID,
	).Scan(&count)

	if err != nil {
		return err
	}

	if count > 0 {
		return fmt.Errorf("group is in use by %d probes", count)
	}

	_, err = r.db.ExecContext(ctx, "DELETE FROM fleet_groups WHERE id = $1", groupID)
	return err
}

// Fleet Status

func (r *FleetRepository) GetFleetStatus(ctx context.Context) (*models.FleetStatusResponse, error) {
	status := &models.FleetStatusResponse{
		LastUpdated: time.Now(),
	}

	// Get counts
	err := r.db.QueryRowContext(ctx, `
		SELECT 
			COUNT(*) as total,
			COUNT(CASE WHEN managed THEN 1 END) as managed,
			COUNT(CASE WHEN p.status = 'active' THEN 1 END) as active,
			COUNT(CASE WHEN p.last_seen < NOW() - INTERVAL '5 minutes' AND p.status = 'active' THEN 1 END) as stale
		FROM fleet_probes fp
		RIGHT JOIN probes p ON fp.probe_id = p.probe_id
	`).Scan(&status.TotalProbes, &status.ManagedProbes, &status.ActiveProbes, &status.StaleProbes)

	if err != nil {
		return nil, err
	}

	// Get command stats
	err = r.db.QueryRowContext(ctx, `
		SELECT 
			COUNT(CASE WHEN issued_at > NOW() - INTERVAL '24 hours' THEN 1 END) as today,
			COUNT(CASE WHEN status = 'pending' THEN 1 END) as pending,
			COUNT(CASE WHEN status = 'in_progress' THEN 1 END) as in_progress
		FROM fleet_commands
	`).Scan(&status.CommandsToday, &status.CommandsPending, &status.CommandsInProgress)

	if err != nil {
		return nil, err
	}

	// Get group summaries
	rows, err := r.db.QueryContext(ctx, `
		SELECT 
			g.id, g.name,
			COUNT(fp.probe_id) as probe_count,
			COUNT(CASE WHEN p.status = 'active' AND p.last_seen > NOW() - INTERVAL '5 minutes' THEN 1 END) as online,
			COUNT(DISTINCT a.id) as alert_count
		FROM fleet_groups g
		LEFT JOIN fleet_probes fp ON fp.groups @> jsonb_build_array(g.id)
		LEFT JOIN probes p ON fp.probe_id = p.probe_id
		LEFT JOIN alerts a ON a.probe_id = p.probe_id AND a.resolved_at IS NULL
		GROUP BY g.id, g.name
		ORDER BY g.name
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	for rows.Next() {
		var gs models.GroupSummary
		err := rows.Scan(&gs.ID, &gs.Name, &gs.ProbeCount, &gs.Online, &gs.AlertCount)
		if err != nil {
			return nil, err
		}
		status.Groups = append(status.Groups, gs)
	}

	// Calculate health score
	status.HealthScore = calculateFleetHealthScore(status)

	return status, nil
}

// Helper functions

func generateCommandID() string {
	return fmt.Sprintf("cmd_%d", time.Now().UnixNano())
}

func generateGroupID(name string) string {
	return strings.ToLower(strings.ReplaceAll(name, " ", "_"))
}

func calculateFleetHealthScore(status *models.FleetStatusResponse) float64 {
	if status.TotalProbes == 0 {
		return 100.0
	}

	score := 100.0

	// Managed probes are good
	managedRatio := float64(status.ManagedProbes) / float64(status.TotalProbes)
	score *= managedRatio

	// Active ratio
	activeRatio := float64(status.ActiveProbes) / float64(status.TotalProbes)
	score *= activeRatio

	// Stale probes penalty
	stalePenalty := float64(status.StaleProbes) / float64(status.TotalProbes) * 50
	score -= stalePenalty

	// Command backlog penalty
	if status.CommandsPending > 10 {
		score -= 10
	}
	if status.CommandsInProgress > 5 {
		score -= 5
	}

	if score < 0 {
		score = 0
	}

	return score
}

// GetUnenrolledProbes returns probes that are not currently in the fleet_probes table.
func (r *FleetRepository) GetUnenrolledProbes(ctx context.Context) ([]models.Probe, error) {
	query := `
		SELECT p.probe_id, p.location, p.building, p.floor, p.department, 
			   p.status, p.firmware_version, p.last_seen, p.created_at, p.updated_at
		FROM probes p
		WHERE NOT EXISTS (
			SELECT 1 FROM fleet_probes fp WHERE fp.probe_id = p.probe_id
		)
		ORDER BY p.last_seen DESC
	`

	rows, err := r.db.QueryContext(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("failed to query unenrolled probes: %w", err)
	}
	defer rows.Close()

	var probes []models.Probe
	for rows.Next() {
		var p models.Probe
		err := rows.Scan(
			&p.ProbeID, &p.Location, &p.Building, &p.Floor, &p.Department,
			&p.Status, &p.FirmwareVersion, &p.LastSeen, &p.CreatedAt, &p.UpdatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan probe: %w", err)
		}
		probes = append(probes, p)
	}
	return probes, nil
}

// GetProbeCommandStatuses retrieves the per‑probe statuses for a given fleet command.
func (r *FleetRepository) GetProbeCommandStatuses(ctx context.Context, commandID string) ([]models.FleetCommandTargetStatus, error) {
	query := `
        SELECT probe_id, status, result, error_message, 
               COALESCE(completed_at, acknowledged_at, sent_at) as last_update
        FROM fleet_command_probes
        WHERE command_id = $1
        ORDER BY probe_id
    `
	rows, err := r.db.QueryContext(ctx, query, commandID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var targets []models.FleetCommandTargetStatus
	for rows.Next() {
		var t models.FleetCommandTargetStatus
		var resultJSON []byte
		var lastUpdate pq.NullTime

		err := rows.Scan(
			&t.ProbeID,
			&t.Status,
			&resultJSON,
			&t.Error,
			&lastUpdate,
		)
		if err != nil {
			return nil, err
		}

		if len(resultJSON) > 0 && string(resultJSON) != "null" {
			if err := json.Unmarshal(resultJSON, &t.ResponsePayload); err != nil {

			}
		}
		if lastUpdate.Valid {
			t.UpdatedAt = &lastUpdate.Time
		}

		targets = append(targets, t)
	}
	return targets, nil
}
