package repository

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"

	"CampusMonitorAPI/internal/models"
)

type CommandRepository struct {
	db *sql.DB
}

func NewCommandRepository(db *sql.DB) *CommandRepository {
	return &CommandRepository{db: db}
}

// ... Create (Keep your existing Create method) ...
func (r *CommandRepository) Create(ctx context.Context, cmd *models.Command) error {
	query := `
       INSERT INTO commands (probe_id, command_type, payload, status)
       VALUES ($1, $2, $3, $4)
       RETURNING id, issued_at
    `
	var payloadJSON []byte
	if cmd.Payload != nil {
		var err error
		payloadJSON, err = json.Marshal(cmd.Payload)
		if err != nil {
			return fmt.Errorf("failed to marshal command payload: %w", err)
		}
	} else {
		payloadJSON = []byte("{}")
	}
	err := r.db.QueryRowContext(
		ctx, query,
		cmd.ProbeID,
		cmd.CommandType,
		payloadJSON,
		cmd.Status,
	).Scan(&cmd.ID, &cmd.IssuedAt)

	if err != nil {
		return fmt.Errorf("failed to create command: %w", err)
	}

	return nil
}

func (r *CommandRepository) GetByID(ctx context.Context, commandID int) (*models.Command, error) {
	query := `
       SELECT id, probe_id, command_type, payload, issued_at, 
              executed_at, status, result
       FROM commands
       WHERE id = $1
    `

	var payloadBytes, resultBytes []byte // Buffer for JSON data
	cmd := &models.Command{}

	err := r.db.QueryRowContext(ctx, query, commandID).Scan(
		&cmd.ID,
		&cmd.ProbeID,
		&cmd.CommandType,
		&payloadBytes, // Scan into bytes first
		&cmd.IssuedAt,
		&cmd.ExecutedAt,
		&cmd.Status,
		&resultBytes, // Scan into bytes first
	)

	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("command %d not found", commandID)
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get command: %w", err)
	}

	// Manually Unmarshal JSON
	if payloadBytes != nil {
		_ = json.Unmarshal(payloadBytes, &cmd.Payload)
	}
	if resultBytes != nil {
		_ = json.Unmarshal(resultBytes, &cmd.Result)
	}

	return cmd, nil
}

func (r *CommandRepository) GetByProbeID(ctx context.Context, probeID string, limit int) ([]models.Command, error) {
	query := `
       SELECT id, probe_id, command_type, payload, issued_at, 
              executed_at, status, result
       FROM commands
       WHERE probe_id = $1
       ORDER BY issued_at DESC
       LIMIT $2
    `

	rows, err := r.db.QueryContext(ctx, query, probeID, limit)
	if err != nil {
		return nil, fmt.Errorf("failed to query commands: %w", err)
	}
	defer rows.Close()

	commands := []models.Command{}
	for rows.Next() {
		var cmd models.Command
		var payloadBytes, resultBytes []byte // Buffer

		err := rows.Scan(
			&cmd.ID,
			&cmd.ProbeID,
			&cmd.CommandType,
			&payloadBytes, // Scan bytes
			&cmd.IssuedAt,
			&cmd.ExecutedAt,
			&cmd.Status,
			&resultBytes, // Scan bytes
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan command: %w", err)
		}

		// Unmarshal
		if payloadBytes != nil {
			_ = json.Unmarshal(payloadBytes, &cmd.Payload)
		}
		if resultBytes != nil {
			_ = json.Unmarshal(resultBytes, &cmd.Result)
		}

		commands = append(commands, cmd)
	}

	return commands, nil
}

func (r *CommandRepository) GetPending(ctx context.Context) ([]models.Command, error) {
	query := `
       SELECT id, probe_id, command_type, payload, issued_at, 
              executed_at, status, result
       FROM commands
       WHERE status IN ('pending', 'sent')
       ORDER BY issued_at ASC
    `

	rows, err := r.db.QueryContext(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("failed to query pending commands: %w", err)
	}
	defer rows.Close()

	commands := []models.Command{}
	for rows.Next() {
		var cmd models.Command
		var payloadBytes, resultBytes []byte

		err := rows.Scan(
			&cmd.ID,
			&cmd.ProbeID,
			&cmd.CommandType,
			&payloadBytes,
			&cmd.IssuedAt,
			&cmd.ExecutedAt,
			&cmd.Status,
			&resultBytes,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan command: %w", err)
		}

		if payloadBytes != nil {
			_ = json.Unmarshal(payloadBytes, &cmd.Payload)
		}
		if resultBytes != nil {
			_ = json.Unmarshal(resultBytes, &cmd.Result)
		}

		commands = append(commands, cmd)
	}

	return commands, nil
}

// ... UpdateStatus, DeleteOld, GetStatistics (Keep existing) ...
func (r *CommandRepository) UpdateStatus(ctx context.Context, commandID int, status string, result map[string]interface{}) error {
	query := `
       UPDATE commands
       SET status = $2,
           result = $3,
           executed_at = CASE 
               WHEN $2 IN ('completed', 'failed') THEN NOW()
               ELSE executed_at
           END
       WHERE id = $1
    `
	var resultJSON []byte
	if result != nil {
		var err error
		resultJSON, err = json.Marshal(result)
		if err != nil {
			return fmt.Errorf("failed to marshal command result: %w", err)
		}
	} else {
		resultJSON = []byte("{}")
	}

	res, err := r.db.ExecContext(ctx, query, commandID, status, resultJSON)
	if err != nil {
		return fmt.Errorf("failed to update command status: %w", err)
	}

	rows, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get affected rows: %w", err)
	}

	if rows == 0 {
		return fmt.Errorf("command %d not found", commandID)
	}

	return nil
}

func (r *CommandRepository) DeleteOld(ctx context.Context, olderThanDays int) (int64, error) {
	query := `
       DELETE FROM commands
       WHERE issued_at < NOW() - ($1 || ' days')::INTERVAL
         AND status IN ('completed', 'failed')
    `

	result, err := r.db.ExecContext(ctx, query, olderThanDays)
	if err != nil {
		return 0, fmt.Errorf("failed to delete old commands: %w", err)
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return 0, fmt.Errorf("failed to get affected rows: %w", err)
	}

	return rows, nil
}

func (r *CommandRepository) GetStatistics(ctx context.Context) (map[string]int, error) {
	query := `
       SELECT status, COUNT(*) as count
       FROM commands
       GROUP BY status
    `

	rows, err := r.db.QueryContext(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("failed to get command statistics: %w", err)
	}
	defer rows.Close()

	stats := make(map[string]int)
	for rows.Next() {
		var status string
		var count int
		if err := rows.Scan(&status, &count); err != nil {
			return nil, fmt.Errorf("failed to scan statistics: %w", err)
		}
		stats[status] = count
	}

	return stats, nil
}
func (r *CommandRepository) UpdateLatestResult(ctx context.Context, probeID string, cmdType string, status string, result map[string]interface{}) error {
	query := `
        UPDATE commands
        SET status = $3,
            result = $4,
            executed_at = NOW()
        WHERE id = (
            SELECT id FROM commands 
            WHERE probe_id = $1 
              AND command_type = $2
            ORDER BY 
              CASE WHEN status IN ('pending', 'sent', 'processing') THEN 0 ELSE 1 END,
              issued_at DESC
            LIMIT 1
        )
    `

	var resultJSON []byte
	var err error

	if result != nil {
		resultJSON, err = json.Marshal(result)
		if err != nil {
			return fmt.Errorf("failed to marshal result: %w", err)
		}
	} else {
		resultJSON = []byte("{}")
	}

	res, err := r.db.ExecContext(ctx, query, probeID, cmdType, status, resultJSON)
	if err != nil {
		return fmt.Errorf("db error updating command: %w", err)
	}

	rows, _ := res.RowsAffected()
	if rows == 0 {
		return fmt.Errorf("no matching command record found")
	}

	return nil
}
func (r *CommandRepository) Delete(ctx context.Context, commandID int) error {
	query := `DELETE FROM commands WHERE id = $1`
	res, err := r.db.ExecContext(ctx, query, commandID)
	if err != nil {
		return fmt.Errorf("failed to delete command: %w", err)
	}

	rows, _ := res.RowsAffected()
	if rows == 0 {
		return fmt.Errorf("command not found")
	}
	return nil
}

// PruneOldScans keeps only the latest N completed deep scans for a probe
func (r *CommandRepository) PruneOldScans(ctx context.Context, probeID string, keepCount int) error {
	query := `
		DELETE FROM commands
		WHERE probe_id = $1
		  AND command_type = 'deep_scan'
		  AND status = 'completed'
		  AND id NOT IN (
			  SELECT id FROM commands
			  WHERE probe_id = $1
				AND command_type = 'deep_scan'
				AND status = 'completed'
			  ORDER BY issued_at DESC
			  LIMIT $2
		  )
	`
	_, err := r.db.ExecContext(ctx, query, probeID, keepCount)
	if err != nil {
		return fmt.Errorf("failed to prune old scans: %w", err)
	}
	return nil
}
