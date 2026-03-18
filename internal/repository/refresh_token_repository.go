package repository

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"CampusMonitorAPI/internal/models"
)

type RefreshTokenRepository struct {
	db *sql.DB
}

func NewRefreshTokenRepository(db *sql.DB) *RefreshTokenRepository {
	return &RefreshTokenRepository{db: db}
}

// Create stores a new refresh token (hash only).
func (r *RefreshTokenRepository) Create(ctx context.Context, token *models.RefreshToken) error {
	query := `
		INSERT INTO refresh_tokens (user_id, token_hash, expires_at, created_at, revoked)
		VALUES ($1, $2, $3, NOW(), false)
		RETURNING id, created_at
	`
	err := r.db.QueryRowContext(ctx, query,
		token.UserID, token.TokenHash, token.ExpiresAt,
	).Scan(&token.ID, &token.CreatedAt)
	if err != nil {
		return fmt.Errorf("failed to create refresh token: %w", err)
	}
	return nil
}

// GetByTokenHash retrieves a token by its hash.
func (r *RefreshTokenRepository) GetByTokenHash(ctx context.Context, tokenHash string) (*models.RefreshToken, error) {
	query := `
		SELECT id, user_id, token_hash, expires_at, created_at, revoked
		FROM refresh_tokens
		WHERE token_hash = $1
	`
	var token models.RefreshToken
	err := r.db.QueryRowContext(ctx, query, tokenHash).Scan(
		&token.ID, &token.UserID, &token.TokenHash, &token.ExpiresAt, &token.CreatedAt, &token.Revoked,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to get refresh token: %w", err)
	}
	return &token, nil
}

// Revoke marks a token as revoked.
func (r *RefreshTokenRepository) Revoke(ctx context.Context, id int) error {
	query := `UPDATE refresh_tokens SET revoked = true WHERE id = $1`
	_, err := r.db.ExecContext(ctx, query, id)
	return err
}

// RevokeAllForUser revokes all tokens for a user (logout everywhere).
func (r *RefreshTokenRepository) RevokeAllForUser(ctx context.Context, userID int) error {
	query := `UPDATE refresh_tokens SET revoked = true WHERE user_id = $1`
	_, err := r.db.ExecContext(ctx, query, userID)
	return err
}

// DeleteExpired removes expired tokens older than a given time.
func (r *RefreshTokenRepository) DeleteExpired(ctx context.Context, olderThan time.Time) error {
	query := `DELETE FROM refresh_tokens WHERE expires_at < $1`
	_, err := r.db.ExecContext(ctx, query, olderThan)
	return err
}
