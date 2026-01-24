// internal/database/database.go

package database

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"CampusMonitorAPI/internal/config"

	_ "github.com/lib/pq"
)

type Database struct {
	DB  *sql.DB
	cfg *config.DatabaseConfig
}

func New(cfg *config.DatabaseConfig) (*Database, error) {
	dsn := fmt.Sprintf(
		"host=%s port=%d user=%s password=%s dbname=%s sslmode=%s",
		cfg.Host,
		cfg.Port,
		cfg.User,
		cfg.Password,
		cfg.Database,
		cfg.SSLMode,
	)

	db, err := sql.Open("postgres", dsn)
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	db.SetMaxOpenConns(cfg.MaxOpenConns)
	db.SetMaxIdleConns(cfg.MaxIdleConns)
	db.SetConnMaxLifetime(cfg.ConnMaxLifetime)
	db.SetConnMaxIdleTime(cfg.ConnMaxIdleTime)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := db.PingContext(ctx); err != nil {
		return nil, fmt.Errorf("failed to ping database: %w", err)
	}

	return &Database{
		DB:  db,
		cfg: cfg,
	}, nil
}

func (d *Database) Close() error {
	return d.DB.Close()
}

func (d *Database) Health(ctx context.Context) error {
	ctx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()

	if err := d.DB.PingContext(ctx); err != nil {
		return fmt.Errorf("database health check failed: %w", err)
	}

	var result int
	if err := d.DB.QueryRowContext(ctx, "SELECT 1").Scan(&result); err != nil {
		return fmt.Errorf("database query check failed: %w", err)
	}

	return nil
}

func (d *Database) Stats() sql.DBStats {
	return d.DB.Stats()
}

func (d *Database) BeginTx(ctx context.Context, opts *sql.TxOptions) (*sql.Tx, error) {
	return d.DB.BeginTx(ctx, opts)
}
