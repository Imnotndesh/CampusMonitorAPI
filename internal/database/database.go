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

	// Automatically initialize schema if not already present
	if err := initSchema(db); err != nil {
		return nil, fmt.Errorf("failed to initialize database schema: %w", err)
	}

	return &Database{
		DB:  db,
		cfg: cfg,
	}, nil
}

// initSchema creates all necessary tables, indexes, and the TimescaleDB hypertable if they don't exist.
func initSchema(db *sql.DB) error {
	// 1. Enable TimescaleDB extension (if not already enabled)
	if _, err := db.Exec("CREATE EXTENSION IF NOT EXISTS timescaledb"); err != nil {
		return fmt.Errorf("failed to enable timescaledb extension: %w", err)
	}

	createTableQueries := []string{
		`CREATE TABLE IF NOT EXISTS users (
			id SERIAL PRIMARY KEY,
			username VARCHAR(50) UNIQUE NOT NULL,
			email VARCHAR(255) UNIQUE NOT NULL,
			password_hash VARCHAR(255),
			role VARCHAR(20) DEFAULT 'user',
			created_at TIMESTAMPTZ DEFAULT NOW(),
			updated_at TIMESTAMPTZ DEFAULT NOW()
		)`,

		`CREATE TABLE IF NOT EXISTS refresh_tokens (
			id SERIAL PRIMARY KEY,
			user_id INT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
			token_hash VARCHAR(255) NOT NULL UNIQUE,
			expires_at TIMESTAMPTZ NOT NULL,
			created_at TIMESTAMPTZ DEFAULT NOW(),
			revoked BOOLEAN DEFAULT false
		)`,

		`CREATE TABLE IF NOT EXISTS totp_secrets (
			user_id INT PRIMARY KEY REFERENCES users(id) ON DELETE CASCADE,
			secret VARCHAR(255) NOT NULL,
			enabled BOOLEAN DEFAULT false,
			created_at TIMESTAMPTZ DEFAULT NOW(),
			last_used TIMESTAMPTZ
		)`,

		`CREATE TABLE IF NOT EXISTS oauth_accounts (
			id SERIAL PRIMARY KEY,
			user_id INT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
			provider VARCHAR(50) NOT NULL,
			provider_user_id VARCHAR(255) NOT NULL,
			access_token TEXT,
			refresh_token TEXT,
			expires_at TIMESTAMPTZ,
			created_at TIMESTAMPTZ DEFAULT NOW(),
			UNIQUE(provider, provider_user_id)
		)`,

		`CREATE TABLE IF NOT EXISTS oauth_states (
			state VARCHAR(255) PRIMARY KEY,
			redirect_uri TEXT NOT NULL,
			expires_at TIMESTAMPTZ NOT NULL
		)`,

		// Probes and telemetry
		`CREATE TABLE IF NOT EXISTS probes (
			probe_id VARCHAR(50) PRIMARY KEY,
			location TEXT,
			building TEXT,
			floor TEXT,
			department TEXT,
			status VARCHAR(20),
			firmware_version VARCHAR(50),
			last_seen TIMESTAMPTZ,
			created_at TIMESTAMPTZ DEFAULT NOW(),
			updated_at TIMESTAMPTZ DEFAULT NOW(),
			metadata JSONB
		)`,

		`CREATE TABLE IF NOT EXISTS telemetry (
			timestamp TIMESTAMPTZ NOT NULL,
			probe_id VARCHAR(50) REFERENCES probes(probe_id),
			type VARCHAR(10),
			rssi INT,
			latency INT,
			packet_loss REAL,
			dns_time INT,
			channel INT,
			bssid VARCHAR(17),
			neighbors INT,
			overlap INT,
			congestion INT,
			snr REAL,
			link_quality REAL,
			utilization REAL,
			phy_mode VARCHAR(10),
			throughput INT,
			noise_floor INT,
			uptime INT,
			received_at TIMESTAMPTZ NOT NULL,
			metadata JSONB
		)`,

		// Alerts
		`CREATE TABLE IF NOT EXISTS alerts (
			id SERIAL PRIMARY KEY,
			probe_id VARCHAR(50) REFERENCES probes(probe_id),
			alert_type VARCHAR(50),
			severity VARCHAR(20),
			message TEXT,
			threshold_value FLOAT,
			actual_value FLOAT,
			triggered_at TIMESTAMPTZ,
			resolved_at TIMESTAMPTZ,
			acknowledged BOOLEAN DEFAULT false,
			metadata JSONB
		)`,

		// Commands
		`CREATE TABLE IF NOT EXISTS commands (
			id SERIAL PRIMARY KEY,
			probe_id VARCHAR(50) REFERENCES probes(probe_id),
			command_type VARCHAR(50),
			payload JSONB,
			status VARCHAR(20),
			result JSONB,
			issued_at TIMESTAMPTZ DEFAULT NOW(),
			executed_at TIMESTAMPTZ
		)`,

		// Fleet management
		`CREATE TABLE IF NOT EXISTS fleet_groups (
			id VARCHAR(50) PRIMARY KEY,
			name VARCHAR(100) NOT NULL,
			description TEXT,
			created_at TIMESTAMPTZ DEFAULT NOW(),
			updated_at TIMESTAMPTZ DEFAULT NOW()
		)`,

		`CREATE TABLE IF NOT EXISTS fleet_templates (
			id SERIAL PRIMARY KEY,
			name VARCHAR(100) NOT NULL,
			description TEXT,
			config JSONB,
			variables JSONB,
			wifi JSONB,
			mqtt JSONB,
			scan_settings JSONB,
			default_tags JSONB,
			default_groups JSONB,
			default_location TEXT,
			created_by VARCHAR(100),
			created_at TIMESTAMPTZ DEFAULT NOW(),
			updated_at TIMESTAMPTZ DEFAULT NOW(),
			usage_count INT DEFAULT 0
		)`,

		`CREATE TABLE IF NOT EXISTS fleet_probes (
			probe_id VARCHAR(50) PRIMARY KEY REFERENCES probes(probe_id) ON DELETE CASCADE,
			managed BOOLEAN DEFAULT false,
			managed_since TIMESTAMPTZ,
			managed_by VARCHAR(100),
			groups JSONB,
			location TEXT,
			tags JSONB,
			config_version INT DEFAULT 0,
			config_template_id INT REFERENCES fleet_templates(id),
			maintenance_window JSONB,
			auto_update_enabled BOOLEAN DEFAULT false,
			last_command_id VARCHAR(100),
			last_command_status VARCHAR(20),
			last_command_time TIMESTAMPTZ,
			commands_received INT DEFAULT 0,
			commands_completed INT DEFAULT 0,
			commands_failed INT DEFAULT 0,
			consecutive_failures INT DEFAULT 0,
			current_firmware VARCHAR(50),
			target_firmware VARCHAR(50),
			last_ota_attempt TIMESTAMPTZ,
			ota_attempts INT DEFAULT 0,
			created_at TIMESTAMPTZ DEFAULT NOW(),
			updated_at TIMESTAMPTZ DEFAULT NOW()
		)`,

		`CREATE TABLE IF NOT EXISTS fleet_commands (
			id VARCHAR(100) PRIMARY KEY,
			command_type VARCHAR(50) NOT NULL,
			payload JSONB,
			issued_by VARCHAR(100),
			issued_at TIMESTAMPTZ DEFAULT NOW(),
			target_groups JSONB,
			target_probes JSONB,
			total_targets INT,
			status VARCHAR(20) DEFAULT 'pending',
			acks_received INT DEFAULT 0,
			completed_count INT DEFAULT 0,
			failed_count INT DEFAULT 0,
			completion_threshold INT DEFAULT 100,
			timeout_seconds INT DEFAULT 300,
			scheduled_for TIMESTAMPTZ,
			metadata JSONB,
			completed_at TIMESTAMPTZ
		)`,

		`CREATE TABLE IF NOT EXISTS fleet_command_probes (
			command_id VARCHAR(100) REFERENCES fleet_commands(id) ON DELETE CASCADE,
			probe_id VARCHAR(50) REFERENCES probes(probe_id) ON DELETE CASCADE,
			status VARCHAR(20) DEFAULT 'pending',
			result JSONB,
			error_message TEXT,
			retry_count INT DEFAULT 0,
			sent_at TIMESTAMPTZ,
			acknowledged_at TIMESTAMPTZ,
			completed_at TIMESTAMPTZ,
			PRIMARY KEY (command_id, probe_id)
		)`,

		// Scheduled tasks
		`CREATE TABLE IF NOT EXISTS scheduled_tasks (
			id VARCHAR(50) PRIMARY KEY,
			probe_id VARCHAR(50) REFERENCES probes(probe_id),
			command_type VARCHAR(50),
			payload JSONB,
			schedule JSONB,
			last_run TIMESTAMPTZ,
			next_run TIMESTAMPTZ,
			enabled BOOLEAN DEFAULT true,
			created_at TIMESTAMPTZ DEFAULT NOW(),
			updated_at TIMESTAMPTZ DEFAULT NOW()
		)`,
	}

	for _, q := range createTableQueries {
		if _, err := db.Exec(q); err != nil {
			return fmt.Errorf("failed to create table: %w (query: %s)", err, q)
		}
	}

	// 3. Create indexes (optional but improve performance)
	indexQueries := []string{
		"CREATE INDEX IF NOT EXISTS idx_telemetry_probe_time ON telemetry (probe_id, timestamp DESC)",
		"CREATE INDEX IF NOT EXISTS idx_telemetry_timestamp ON telemetry (timestamp DESC)",
		"CREATE INDEX IF NOT EXISTS idx_alerts_probe_id ON alerts (probe_id)",
		"CREATE INDEX IF NOT EXISTS idx_alerts_triggered_at ON alerts (triggered_at DESC)",
		"CREATE INDEX IF NOT EXISTS idx_commands_probe_id ON commands (probe_id)",
		"CREATE INDEX IF NOT EXISTS idx_commands_issued_at ON commands (issued_at DESC)",
		"CREATE INDEX IF NOT EXISTS idx_fleet_probes_groups ON fleet_probes USING gin(groups)",
		"CREATE INDEX IF NOT EXISTS idx_fleet_probes_managed ON fleet_probes (managed)",
		"CREATE INDEX IF NOT EXISTS idx_fleet_commands_status ON fleet_commands (status)",
		"CREATE INDEX IF NOT EXISTS idx_fleet_commands_issued ON fleet_commands (issued_at DESC)",
	}

	for _, q := range indexQueries {
		if _, err := db.Exec(q); err != nil {
			// Non-fatal, but log
			fmt.Printf("Warning: could not create index: %v\n", err)
		}
	}

	var isHypertable bool
	err := db.QueryRow(`
		SELECT EXISTS (
			SELECT 1 FROM timescaledb_information.hypertables 
			WHERE hypertable_name = 'telemetry'
		)
	`).Scan(&isHypertable)
	if err != nil {
		return fmt.Errorf("failed to check hypertable status: %w", err)
	}
	if !isHypertable {
		if _, err := db.Exec("SELECT create_hypertable('telemetry', 'timestamp', if_not_exists => TRUE)"); err != nil {
			return fmt.Errorf("failed to create hypertable: %w", err)
		}
	}

	return nil
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
