package repository

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"CampusMonitorAPI/internal/models"
)

type TelemetryRepository struct {
	db *sql.DB
}

func NewTelemetryRepository(db *sql.DB) *TelemetryRepository {
	return &TelemetryRepository{db: db}
}

func (r *TelemetryRepository) Insert(ctx context.Context, telemetry *models.Telemetry) error {
	query := `
		INSERT INTO telemetry (
			timestamp, probe_id, type, rssi, latency, packet_loss, 
			dns_time, channel, bssid, neighbors, overlap, congestion,
			snr, link_quality, utilization, phy_mode, throughput, 
			noise_floor, uptime, metadata
		) VALUES (
			$1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12,
			$13, $14, $15, $16, $17, $18, $19, $20
		)
	`

	// Handle metadata: convert to JSON or use sql.NullString for NULL
	var metadataVal interface{}
	if telemetry.Metadata != nil && len(telemetry.Metadata) > 0 {
		metadataJSON, err := json.Marshal(telemetry.Metadata)
		if err != nil {
			return fmt.Errorf("failed to marshal metadata: %w", err)
		}
		metadataVal = metadataJSON
	} else {
		metadataVal = nil // SQL NULL
	}

	_, err := r.db.ExecContext(
		ctx, query,
		telemetry.Timestamp,
		telemetry.ProbeID,
		telemetry.Type,
		telemetry.RSSI,
		telemetry.Latency,
		telemetry.PacketLoss,
		telemetry.DNSTime,
		telemetry.Channel,
		telemetry.BSSID,
		telemetry.Neighbors,
		telemetry.Overlap,
		telemetry.Congestion,
		telemetry.SNR,
		telemetry.LinkQuality,
		telemetry.Utilization,
		telemetry.PhyMode,
		telemetry.Throughput,
		telemetry.NoiseFloor,
		telemetry.Uptime,
		metadataVal,
	)

	if err != nil {
		return fmt.Errorf("failed to insert telemetry: %w", err)
	}

	return nil
}

// GetLatestByProbe retrieves the single most recent telemetry reading for a given probe.
func (r *TelemetryRepository) GetLatestByProbe(ctx context.Context, probeID string) (*models.Telemetry, error) {
	query := `
		SELECT timestamp, probe_id, type, rssi, latency, packet_loss, dns_time, 
		       channel, bssid, neighbors, overlap, congestion, snr, link_quality, 
		       utilization, phy_mode, throughput, noise_floor, uptime, received_at, metadata
		FROM telemetry
		WHERE probe_id = $1
		ORDER BY timestamp DESC
		LIMIT 1
	`

	var t models.Telemetry
	var metadataJSON []byte

	err := r.db.QueryRowContext(ctx, query, probeID).Scan(
		&t.Timestamp,
		&t.ProbeID,
		&t.Type,
		&t.RSSI,
		&t.Latency,
		&t.PacketLoss,
		&t.DNSTime,
		&t.Channel,
		&t.BSSID,
		&t.Neighbors,
		&t.Overlap,
		&t.Congestion,
		&t.SNR,
		&t.LinkQuality,
		&t.Utilization,
		&t.PhyMode,
		&t.Throughput,
		&t.NoiseFloor,
		&t.Uptime,
		&t.ReceivedAt,
		&metadataJSON,
	)

	if err != nil {
		// sql.ErrNoRows is expected if a probe hasn't sent telemetry yet
		return nil, err
	}

	// Parse the JSONB metadata
	if len(metadataJSON) > 0 {
		_ = json.Unmarshal(metadataJSON, &t.Metadata)
	}

	return &t, nil
}

func (r *TelemetryRepository) InsertBatch(ctx context.Context, telemetries []models.Telemetry) error {
	if len(telemetries) == 0 {
		return nil
	}

	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback()

	stmt, err := tx.PrepareContext(ctx, `
		INSERT INTO telemetry (
			timestamp, probe_id, type, rssi, latency, packet_loss, 
			dns_time, channel, bssid, neighbors, overlap, congestion,
			snr, link_quality, utilization, phy_mode, throughput, 
			noise_floor, uptime, metadata
		) VALUES (
			$1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12,
			$13, $14, $15, $16, $17, $18, $19, $20
		)
	`)
	if err != nil {
		return fmt.Errorf("failed to prepare statement: %w", err)
	}
	defer stmt.Close()

	for _, t := range telemetries {
		var metadataVal interface{}
		if t.Metadata != nil && len(t.Metadata) > 0 {
			metadataJSON, err := json.Marshal(t.Metadata)
			if err != nil {
				return fmt.Errorf("failed to marshal metadata: %w", err)
			}
			metadataVal = metadataJSON
		} else {
			metadataVal = nil
		}

		_, err := stmt.ExecContext(
			ctx,
			t.Timestamp, t.ProbeID, t.Type, t.RSSI, t.Latency, t.PacketLoss,
			t.DNSTime, t.Channel, t.BSSID, t.Neighbors, t.Overlap, t.Congestion,
			t.SNR, t.LinkQuality, t.Utilization, t.PhyMode, t.Throughput,
			t.NoiseFloor, t.Uptime, metadataVal,
		)
		if err != nil {
			return fmt.Errorf("failed to insert telemetry batch: %w", err)
		}
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("failed to commit transaction: %w", err)
	}

	return nil
}

func (r *TelemetryRepository) Query(ctx context.Context, req *models.TelemetryQueryRequest) ([]models.Telemetry, int, error) {
	var conditions []string
	var args []interface{}
	argCount := 1

	if len(req.ProbeIDs) > 0 {
		placeholders := make([]string, len(req.ProbeIDs))
		for i, probeID := range req.ProbeIDs {
			placeholders[i] = fmt.Sprintf("$%d", argCount)
			args = append(args, probeID)
			argCount++
		}
		conditions = append(conditions, fmt.Sprintf("probe_id IN (%s)", strings.Join(placeholders, ",")))
	}

	if req.Type != "" {
		conditions = append(conditions, fmt.Sprintf("type = $%d", argCount))
		args = append(args, req.Type)
		argCount++
	}

	if req.StartTime != nil {
		conditions = append(conditions, fmt.Sprintf("timestamp >= $%d", argCount))
		args = append(args, *req.StartTime)
		argCount++
	}

	if req.EndTime != nil {
		conditions = append(conditions, fmt.Sprintf("timestamp <= $%d", argCount))
		args = append(args, *req.EndTime)
		argCount++
	}

	whereClause := ""
	if len(conditions) > 0 {
		whereClause = "WHERE " + strings.Join(conditions, " AND ")
	}

	countQuery := fmt.Sprintf("SELECT COUNT(*) FROM telemetry %s", whereClause)
	var totalCount int
	err := r.db.QueryRowContext(ctx, countQuery, args...).Scan(&totalCount)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to count telemetry: %w", err)
	}

	limit := req.Limit
	if limit <= 0 {
		limit = 100
	}
	offset := req.Offset
	if offset < 0 {
		offset = 0
	}

	query := fmt.Sprintf(`
		SELECT timestamp, probe_id, type, rssi, latency, packet_loss,
			   dns_time, channel, bssid, neighbors, overlap, congestion,
			   snr, link_quality, utilization, phy_mode, throughput,
			   noise_floor, uptime, received_at, metadata
		FROM telemetry
		%s
		ORDER BY timestamp DESC
		LIMIT $%d OFFSET $%d
	`, whereClause, argCount, argCount+1)

	args = append(args, limit, offset)

	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to query telemetry: %w", err)
	}
	defer rows.Close()

	var telemetries []models.Telemetry
	for rows.Next() {
		var t models.Telemetry
		var metadataJSON sql.NullString

		err := rows.Scan(
			&t.Timestamp, &t.ProbeID, &t.Type, &t.RSSI, &t.Latency, &t.PacketLoss,
			&t.DNSTime, &t.Channel, &t.BSSID, &t.Neighbors, &t.Overlap, &t.Congestion,
			&t.SNR, &t.LinkQuality, &t.Utilization, &t.PhyMode, &t.Throughput,
			&t.NoiseFloor, &t.Uptime, &t.ReceivedAt, &metadataJSON,
		)
		if err != nil {
			return nil, 0, fmt.Errorf("failed to scan telemetry: %w", err)
		}

		if metadataJSON.Valid && metadataJSON.String != "" {
			if err := json.Unmarshal([]byte(metadataJSON.String), &t.Metadata); err != nil {
				return nil, 0, fmt.Errorf("failed to unmarshal metadata: %w", err)
			}
		}

		telemetries = append(telemetries, t)
	}

	return telemetries, totalCount, nil
}

func (r *TelemetryRepository) GetLatest(ctx context.Context, probeID string, limit int) ([]models.Telemetry, error) {
	if limit <= 0 {
		limit = 10
	}

	query := `
		SELECT timestamp, probe_id, type, rssi, latency, packet_loss,
			   dns_time, channel, bssid, neighbors, overlap, congestion,
			   snr, link_quality, utilization, phy_mode, throughput,
			   noise_floor, uptime, received_at, metadata
		FROM telemetry
		WHERE probe_id = $1
		ORDER BY timestamp DESC
		LIMIT $2
	`

	rows, err := r.db.QueryContext(ctx, query, probeID, limit)
	if err != nil {
		return nil, fmt.Errorf("failed to get latest telemetry: %w", err)
	}
	defer rows.Close()

	telemetries := []models.Telemetry{}
	for rows.Next() {
		var t models.Telemetry
		var metadataJSON sql.NullString

		err := rows.Scan(
			&t.Timestamp, &t.ProbeID, &t.Type, &t.RSSI, &t.Latency, &t.PacketLoss,
			&t.DNSTime, &t.Channel, &t.BSSID, &t.Neighbors, &t.Overlap, &t.Congestion,
			&t.SNR, &t.LinkQuality, &t.Utilization, &t.PhyMode, &t.Throughput,
			&t.NoiseFloor, &t.Uptime, &t.ReceivedAt, &metadataJSON,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan telemetry: %w", err)
		}

		if metadataJSON.Valid && metadataJSON.String != "" {
			if err := json.Unmarshal([]byte(metadataJSON.String), &t.Metadata); err != nil {
				return nil, fmt.Errorf("failed to unmarshal metadata: %w", err)
			}
		}

		telemetries = append(telemetries, t)
	}

	return telemetries, nil
}

func (r *TelemetryRepository) GetStats(ctx context.Context, probeID string, start, end time.Time) (*models.StatsResponse, error) {
	query := `
		SELECT 
			COUNT(*) as sample_count,
			AVG(rssi) as avg_rssi,
			MIN(rssi) as min_rssi,
			MAX(rssi) as max_rssi,
			AVG(latency) as avg_latency,
			AVG(packet_loss) as avg_packet_loss,
			MODE() WITHIN GROUP (ORDER BY bssid) as most_common_ap,
			MODE() WITHIN GROUP (ORDER BY channel) as most_common_channel
		FROM telemetry
		WHERE probe_id = $1 
		  AND timestamp >= $2 
		  AND timestamp <= $3
		  AND rssi IS NOT NULL
	`

	stats := &models.StatsResponse{
		ProbeID: probeID,
		Period:  fmt.Sprintf("%s to %s", start.Format("2006-01-02"), end.Format("2006-01-02")),
	}

	var avgRSSI, avgLatency, avgPacketLoss sql.NullFloat64
	var minRSSI, maxRSSI sql.NullInt64
	var mostCommonAP sql.NullString
	var mostCommonChan sql.NullInt64

	err := r.db.QueryRowContext(ctx, query, probeID, start, end).Scan(
		&stats.SampleCount,
		&avgRSSI,
		&minRSSI,
		&maxRSSI,
		&avgLatency,
		&avgPacketLoss,
		&mostCommonAP,
		&mostCommonChan,
	)

	if err != nil {
		return nil, fmt.Errorf("failed to get stats: %w", err)
	}

	if avgRSSI.Valid {
		stats.AvgRSSI = avgRSSI.Float64
	}
	if minRSSI.Valid {
		stats.MinRSSI = int(minRSSI.Int64)
	}
	if maxRSSI.Valid {
		stats.MaxRSSI = int(maxRSSI.Int64)
	}
	if avgLatency.Valid {
		stats.AvgLatency = avgLatency.Float64
	}
	if avgPacketLoss.Valid {
		stats.AvgPacketLoss = avgPacketLoss.Float64
	}
	if mostCommonAP.Valid {
		stats.MostCommonAP = mostCommonAP.String
	}
	if mostCommonChan.Valid {
		stats.MostCommonChan = int(mostCommonChan.Int64)
	}

	return stats, nil
}

func (r *TelemetryRepository) GetHourlyStats(ctx context.Context, probeID string, hours int) ([]models.StatsResponse, error) {
	query := `
		SELECT 
			hour,
			probe_id,
			sample_count,
			avg_rssi,
			min_rssi,
			max_rssi,
			avg_latency,
			avg_packet_loss,
			most_common_bssid as most_common_ap,
			most_common_channel
		FROM telemetry_hourly
		WHERE probe_id = $1
		  AND hour >= NOW() - INTERVAL '1 hour' * $2
		ORDER BY hour DESC
	`

	rows, err := r.db.QueryContext(ctx, query, probeID, hours)
	if err != nil {
		return nil, fmt.Errorf("failed to get hourly stats: %w", err)
	}
	defer rows.Close()

	var stats []models.StatsResponse
	for rows.Next() {
		var s models.StatsResponse
		var hour time.Time
		var avgRSSI, avgLatency, avgPacketLoss sql.NullFloat64
		var minRSSI, maxRSSI sql.NullInt64
		var mostCommonAP sql.NullString
		var mostCommonChan sql.NullInt64

		err := rows.Scan(
			&hour,
			&s.ProbeID,
			&s.SampleCount,
			&avgRSSI,
			&minRSSI,
			&maxRSSI,
			&avgLatency,
			&avgPacketLoss,
			&mostCommonAP,
			&mostCommonChan,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan hourly stats: %w", err)
		}

		s.Period = hour.Format("2006-01-02 15:00")
		if avgRSSI.Valid {
			s.AvgRSSI = avgRSSI.Float64
		}
		if minRSSI.Valid {
			s.MinRSSI = int(minRSSI.Int64)
		}
		if maxRSSI.Valid {
			s.MaxRSSI = int(maxRSSI.Int64)
		}
		if avgLatency.Valid {
			s.AvgLatency = avgLatency.Float64
		}
		if avgPacketLoss.Valid {
			s.AvgPacketLoss = avgPacketLoss.Float64
		}
		if mostCommonAP.Valid {
			s.MostCommonAP = mostCommonAP.String
		}
		if mostCommonChan.Valid {
			s.MostCommonChan = int(mostCommonChan.Int64)
		}

		stats = append(stats, s)
	}

	return stats, nil
}
