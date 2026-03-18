package repository

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"CampusMonitorAPI/internal/models"
)

type OAuthStateRepository struct {
	db *sql.DB
}

func NewOAuthStateRepository(db *sql.DB) *OAuthStateRepository {
	return &OAuthStateRepository{db: db}
}

// Create stores an OAuth state.
func (r *OAuthStateRepository) Create(ctx context.Context, state *models.OAuthState) error {
	query := `
		INSERT INTO oauth_states (state, redirect_uri, expires_at)
		VALUES ($1, $2, $3)
	`
	_, err := r.db.ExecContext(ctx, query, state.State, state.RedirectURI, state.ExpiresAt)
	if err != nil {
		return fmt.Errorf("failed to create oauth state: %w", err)
	}
	return nil
}

// GetAndDelete retrieves a state and deletes it (one-time use).
func (r *OAuthStateRepository) GetAndDelete(ctx context.Context, state string) (*models.OAuthState, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	var oaState models.OAuthState
	err = tx.QueryRowContext(ctx, `SELECT state, redirect_uri, expires_at FROM oauth_states WHERE state = $1`, state).Scan(
		&oaState.State, &oaState.RedirectURI, &oaState.ExpiresAt,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}

	_, err = tx.ExecContext(ctx, `DELETE FROM oauth_states WHERE state = $1`, state)
	if err != nil {
		return nil, err
	}

	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return &oaState, nil
}

// CleanupExpired removes expired states.
func (r *OAuthStateRepository) CleanupExpired(ctx context.Context) error {
	query := `DELETE FROM oauth_states WHERE expires_at < NOW()`
	_, err := r.db.ExecContext(ctx, query)
	return err
}
