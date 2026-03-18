package repository

import (
	"CampusMonitorAPI/internal/models"
	"context"
	"database/sql"
	"errors"
	"fmt"
)

type TOTPRepository struct {
	db *sql.DB
}

func NewTOTPRepository(db *sql.DB) *TOTPRepository {
	return &TOTPRepository{db: db}
}

// CreateOrUpdate inserts or updates the TOTP secret for a user.
func (r *TOTPRepository) CreateOrUpdate(ctx context.Context, userID int, secret string, enabled bool) error {
	query := `
		INSERT INTO totp_secrets (user_id, secret, enabled, created_at)
		VALUES ($1, $2, $3, NOW())
		ON CONFLICT (user_id) DO UPDATE SET
			secret = EXCLUDED.secret,
			enabled = EXCLUDED.enabled,
			created_at = EXCLUDED.created_at
	`
	_, err := r.db.ExecContext(ctx, query, userID, secret, enabled)
	if err != nil {
		return fmt.Errorf("failed to upsert totp secret: %w", err)
	}
	return nil
}

// GetByUserID retrieves the TOTP secret for a user.
func (r *TOTPRepository) GetByUserID(ctx context.Context, userID int) (*models.TOTPSecret, error) {
	query := `
		SELECT user_id, secret, enabled, created_at, last_used
		FROM totp_secrets
		WHERE user_id = $1
	`
	var secret models.TOTPSecret
	err := r.db.QueryRowContext(ctx, query, userID).Scan(
		&secret.UserID, &secret.Secret, &secret.Enabled, &secret.CreatedAt, &secret.LastUsed,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to get totp secret: %w", err)
	}
	return &secret, nil
}

// UpdateLastUsed updates the last_used timestamp.
func (r *TOTPRepository) UpdateLastUsed(ctx context.Context, userID int) error {
	query := `UPDATE totp_secrets SET last_used = NOW() WHERE user_id = $1`
	_, err := r.db.ExecContext(ctx, query, userID)
	return err
}

// Delete removes TOTP for a user.
func (r *TOTPRepository) Delete(ctx context.Context, userID int) error {
	query := `DELETE FROM totp_secrets WHERE user_id = $1`
	_, err := r.db.ExecContext(ctx, query, userID)
	return err
}
