package repository

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"CampusMonitorAPI/internal/models"
)

type OAuthAccountRepository struct {
	db *sql.DB
}

func NewOAuthAccountRepository(db *sql.DB) *OAuthAccountRepository {
	return &OAuthAccountRepository{db: db}
}

// Create inserts a new OAuth account.
func (r *OAuthAccountRepository) Create(ctx context.Context, acc *models.OAuthAccount) error {
	query := `
		INSERT INTO oauth_accounts (user_id, provider, provider_user_id, access_token, refresh_token, expires_at, created_at)
		VALUES ($1, $2, $3, $4, $5, $6, NOW())
		RETURNING id, created_at
	`
	err := r.db.QueryRowContext(ctx, query,
		acc.UserID, acc.Provider, acc.ProviderUserID, acc.AccessToken, acc.RefreshToken, acc.ExpiresAt,
	).Scan(&acc.ID, &acc.CreatedAt)
	if err != nil {
		return fmt.Errorf("failed to create oauth account: %w", err)
	}
	return nil
}

// GetByProvider finds an OAuth account by provider and provider user ID.
func (r *OAuthAccountRepository) GetByProvider(ctx context.Context, provider models.OAuthProvider, providerUserID string) (*models.OAuthAccount, error) {
	query := `
		SELECT id, user_id, provider, provider_user_id, access_token, refresh_token, expires_at, created_at
		FROM oauth_accounts
		WHERE provider = $1 AND provider_user_id = $2
	`
	var acc models.OAuthAccount
	err := r.db.QueryRowContext(ctx, query, provider, providerUserID).Scan(
		&acc.ID, &acc.UserID, &acc.Provider, &acc.ProviderUserID,
		&acc.AccessToken, &acc.RefreshToken, &acc.ExpiresAt, &acc.CreatedAt,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil // not an error, just not found
		}
		return nil, fmt.Errorf("failed to get oauth account: %w", err)
	}
	return &acc, nil
}

// GetByUserID returns all OAuth accounts for a user.
func (r *OAuthAccountRepository) GetByUserID(ctx context.Context, userID int) ([]models.OAuthAccount, error) {
	query := `
		SELECT id, user_id, provider, provider_user_id, access_token, refresh_token, expires_at, created_at
		FROM oauth_accounts
		WHERE user_id = $1
	`
	rows, err := r.db.QueryContext(ctx, query, userID)
	if err != nil {
		return nil, fmt.Errorf("failed to query oauth accounts: %w", err)
	}
	defer rows.Close()

	var accounts []models.OAuthAccount
	for rows.Next() {
		var acc models.OAuthAccount
		err := rows.Scan(
			&acc.ID, &acc.UserID, &acc.Provider, &acc.ProviderUserID,
			&acc.AccessToken, &acc.RefreshToken, &acc.ExpiresAt, &acc.CreatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan oauth account: %w", err)
		}
		accounts = append(accounts, acc)
	}
	return accounts, nil
}

// UpdateTokens updates access and refresh tokens for an account.
func (r *OAuthAccountRepository) UpdateTokens(ctx context.Context, id int, accessToken, refreshToken string, expiresAt *time.Time) error {
	query := `
		UPDATE oauth_accounts
		SET access_token = $2, refresh_token = $3, expires_at = $4
		WHERE id = $1
	`
	_, err := r.db.ExecContext(ctx, query, id, accessToken, refreshToken, expiresAt)
	if err != nil {
		return fmt.Errorf("failed to update oauth tokens: %w", err)
	}
	return nil
}

// Delete removes an OAuth account.
func (r *OAuthAccountRepository) Delete(ctx context.Context, id int) error {
	query := `DELETE FROM oauth_accounts WHERE id = $1`
	_, err := r.db.ExecContext(ctx, query, id)
	return err
}
