package models

import (
	"time"
)

type UserRole string

const (
	RoleUser  UserRole = "user"
	RoleAdmin UserRole = "admin"
)

type User struct {
	ID           int       `json:"id" db:"id"`
	Username     string    `json:"username" db:"username"`
	Email        string    `json:"email" db:"email"`
	PasswordHash string    `json:"-" db:"password_hash"`
	Role         UserRole  `json:"role" db:"role"`
	CreatedAt    time.Time `json:"created_at" db:"created_at"`
	UpdatedAt    time.Time `json:"updated_at" db:"updated_at"`
}

type OAuthProvider string

const (
	ProviderGoogle   OAuthProvider = "google"
	ProviderGitHub   OAuthProvider = "github"
	ProviderPocketID OAuthProvider = "pocketid"
)

type OAuthAccount struct {
	ID             int           `json:"id" db:"id"`
	UserID         int           `json:"user_id" db:"user_id"`
	Provider       OAuthProvider `json:"provider" db:"provider"`
	ProviderUserID string        `json:"provider_user_id" db:"provider_user_id"`
	AccessToken    string        `json:"-" db:"access_token"`
	RefreshToken   string        `json:"-" db:"refresh_token"`
	ExpiresAt      *time.Time    `json:"expires_at" db:"expires_at"`
	CreatedAt      time.Time     `json:"created_at" db:"created_at"`
}

type TOTPSecret struct {
	UserID    int        `json:"user_id" db:"user_id"`
	Secret    string     `json:"-" db:"secret"`
	Enabled   bool       `json:"enabled" db:"enabled"`
	CreatedAt time.Time  `json:"created_at" db:"created_at"`
	LastUsed  *time.Time `json:"last_used" db:"last_used"`
}

type RefreshToken struct {
	ID        int       `json:"id" db:"id"`
	UserID    int       `json:"user_id" db:"user_id"`
	TokenHash string    `json:"-" db:"token_hash"`
	ExpiresAt time.Time `json:"expires_at" db:"expires_at"`
	CreatedAt time.Time `json:"created_at" db:"created_at"`
	Revoked   bool      `json:"revoked" db:"revoked"`
}

type OAuthState struct {
	State       string    `json:"state" db:"state"`
	RedirectURI string    `json:"redirect_uri" db:"redirect_uri"`
	ExpiresAt   time.Time `json:"expires_at" db:"expires_at"`
}
