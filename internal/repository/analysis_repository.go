package repository

import (
	"context"
	"database/sql"
	"fmt"
	"time"
)

type AnalyticsRepository struct {
	db *sql.DB
}

func NewAnalyticsRepository(db *sql.DB) *AnalyticsRepository {
	return &AnalyticsRepository{db: db}
}

type TimeSeriesPoint struct {
	Timestamp time.Time `json:"timestamp"`
	Value     float64   `json:"value"`
}

type HeatmapData struct {
	Building string  `json:"building"`
	Floor    string  `json:"floor"`
	Location string  `json:"location"`
	AvgRSSI  float64 `json:"avg_rssi"`
	Count    int     `json:"count"`
}

type ChannelDistribution struct {
	Channel     int     `json:"channel"`
	ProbeCount  int     `json:"probe_count"`
	AvgRSSI     float64 `json:"avg_rssi"`
	Utilization float64 `json:"utilization"`
}

type APAnalysis struct {
	BSSID           string    `json:"bssid"`
	FirstSeen       time.Time `json:"first_seen"`
	LastSeen        time.Time `json:"last_seen"`
	ProbesConnected int       `json:"probes_connected"`
	AvgRSSI         float64   `json:"avg_rssi"`
	Channel         int       `json:"channel"`
	TotalSamples    int       `json:"total_samples"`
}

type CongestionAnalysis struct {
	Hour            time.Time `json:"hour"`
	AvgNeighbors    float64   `json:"avg_neighbors"`
	AvgOverlap      float64   `json:"avg_overlap"`
	AvgUtilization  float64   `json:"avg_utilization"`
	PeakUtilization float64   `json:"peak_utilization"`
	CongestedProbes int       `json:"congested_probes"`
}

type PerformanceMetrics struct {
	Period         string  `json:"period"`
	AvgRSSI        float64 `json:"avg_rssi"`
	MinRSSI        int     `json:"min_rssi"`
	MaxRSSI        int     `json:"max_rssi"`
	AvgLatency     float64 `json:"avg_latency"`
	MinLatency     float64 `json:"min_latency"`
	MaxLatency     float64 `json:"max_latency"`
	P50Latency     float64 `json:"p50_latency"`
	P95Latency     float64 `json:"p95_latency"`
	P99Latency     float64 `json:"p99_latency"`
	AvgPacketLoss  float64 `json:"avg_packet_loss"`
	AvgDNSTime     float64 `json:"avg_dns_time"`
	StabilityScore float64 `json:"stability_score"`
	SampleCount    int     `json:"sample_count"`
}

type ProbeComparison struct {
	ProbeID       string  `json:"probe_id"`
	Location      string  `json:"location"`
	AvgRSSI       float64 `json:"avg_rssi"`
	AvgLatency    float64 `json:"avg_latency"`
	AvgPacketLoss float64 `json:"avg_packet_loss"`
	LinkQuality   float64 `json:"link_quality"`
	UptimePercent float64 `json:"uptime_percent"`
	SampleCount   int     `json:"sample_count"`
}

type NetworkHealth struct {
	Timestamp     time.Time `json:"timestamp"`
	TotalProbes   int       `json:"total_probes"`
	ActiveProbes  int       `json:"active_probes"`
	StaleProbes   int       `json:"stale_probes"`
	AvgRSSI       float64   `json:"avg_rssi"`
	AvgLatency    float64   `json:"avg_latency"`
	AvgPacketLoss float64   `json:"avg_packet_loss"`
	HealthScore   float64   `json:"health_score"`
}

type AnomalyDetection struct {
	ProbeID       string    `json:"probe_id"`
	Timestamp     time.Time `json:"timestamp"`
	MetricType    string    `json:"metric_type"`
	Value         float64   `json:"value"`
	ExpectedValue float64   `json:"expected_value"`
	Deviation     float64   `json:"deviation"`
	Severity      string    `json:"severity"`
}

func (r *AnalyticsRepository) GetRSSITimeSeries(ctx context.Context, probeID string, start, end time.Time, interval string) ([]TimeSeriesPoint, error) {
	query := fmt.Sprintf(`
		SELECT 
			time_bucket('%s', timestamp) as bucket,
			AVG(rssi) as avg_rssi
		FROM telemetry
		WHERE timestamp >= $1
		  AND timestamp <= $2
		  AND rssi IS NOT NULL
	`, interval)

	args := []interface{}{start, end}
	if probeID != "" && probeID != "all" {
		query += " AND probe_id = $3"
		args = append(args, probeID)
	}

	query += " GROUP BY bucket ORDER BY bucket"

	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to get RSSI time series: %w", err)
	}
	defer rows.Close()

	points := []TimeSeriesPoint{}
	for rows.Next() {
		var p TimeSeriesPoint
		if err := rows.Scan(&p.Timestamp, &p.Value); err != nil {
			return nil, fmt.Errorf("failed to scan time series point: %w", err)
		}
		points = append(points, p)
	}

	return points, nil
}

func (r *AnalyticsRepository) GetLatencyTimeSeries(ctx context.Context, probeID string, start, end time.Time, interval string) ([]TimeSeriesPoint, error) {
	query := fmt.Sprintf(`
		SELECT 
			time_bucket('%s', timestamp) as bucket,
			AVG(latency) as avg_latency
		FROM telemetry
		WHERE timestamp >= $1
		  AND timestamp <= $2
		  AND latency IS NOT NULL
	`, interval)

	args := []interface{}{start, end}
	if probeID != "" && probeID != "all" {
		query += " AND probe_id = $3"
		args = append(args, probeID)
	}
	query += " GROUP BY bucket ORDER BY bucket"

	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to get latency time series: %w", err)
	}
	defer rows.Close()

	points := []TimeSeriesPoint{}
	for rows.Next() {
		var p TimeSeriesPoint
		if err := rows.Scan(&p.Timestamp, &p.Value); err != nil {
			return nil, fmt.Errorf("failed to scan time series point: %w", err)
		}
		points = append(points, p)
	}

	return points, nil
}

func (r *AnalyticsRepository) GetHeatmapData(ctx context.Context, start, end time.Time) ([]HeatmapData, error) {
	query := `
		SELECT 
			p.building,
			p.floor,
			p.location,
			AVG(t.rssi) as avg_rssi,
			COUNT(*) as count
		FROM telemetry t
		JOIN probes p ON t.probe_id = p.probe_id
		WHERE t.timestamp >= $1
		  AND t.timestamp <= $2
		  AND t.rssi IS NOT NULL
		GROUP BY p.building, p.floor, p.location
		ORDER BY p.building, p.floor, p.location
	`
	rows, err := r.db.QueryContext(ctx, query, start, end)
	if err != nil {
		return nil, fmt.Errorf("failed to get heatmap: %w", err)
	}
	defer rows.Close()

	heatmap := []HeatmapData{}
	for rows.Next() {
		var h HeatmapData
		if err := rows.Scan(&h.Building, &h.Floor, &h.Location, &h.AvgRSSI, &h.Count); err != nil {
			return nil, err
		}
		heatmap = append(heatmap, h)
	}
	return heatmap, nil
}

func (r *AnalyticsRepository) GetChannelDistribution(ctx context.Context, start, end time.Time) ([]ChannelDistribution, error) {
	query := `
		SELECT 
			channel,
			COUNT(DISTINCT probe_id) as probe_count,
			AVG(rssi) as avg_rssi,
			AVG(utilization) as avg_utilization
		FROM telemetry
		WHERE timestamp >= $1
		  AND timestamp <= $2
		  AND channel IS NOT NULL
		GROUP BY channel
		ORDER BY channel
	`
	rows, err := r.db.QueryContext(ctx, query, start, end)
	if err != nil {
		return nil, fmt.Errorf("failed to get channels: %w", err)
	}
	defer rows.Close()

	dist := []ChannelDistribution{}
	for rows.Next() {
		var c ChannelDistribution
		var u sql.NullFloat64
		if err := rows.Scan(&c.Channel, &c.ProbeCount, &c.AvgRSSI, &u); err != nil {
			return nil, err
		}
		if u.Valid {
			c.Utilization = u.Float64
		}
		dist = append(dist, c)
	}
	return dist, nil
}

func (r *AnalyticsRepository) GetAPAnalysis(ctx context.Context, start, end time.Time) ([]APAnalysis, error) {
	query := `
		SELECT 
			bssid,
			MIN(timestamp) as first_seen,
			MAX(timestamp) as last_seen,
			COUNT(DISTINCT probe_id) as probes_connected,
			AVG(rssi) as avg_rssi,
			MODE() WITHIN GROUP (ORDER BY channel) as channel,
			COUNT(*) as total_samples
		FROM telemetry
		WHERE timestamp >= $1
		  AND timestamp <= $2
		  AND bssid IS NOT NULL
		GROUP BY bssid
		ORDER BY probes_connected DESC, total_samples DESC
	`
	rows, err := r.db.QueryContext(ctx, query, start, end)
	if err != nil {
		return nil, fmt.Errorf("failed to get APs: %w", err)
	}
	defer rows.Close()

	res := []APAnalysis{}
	for rows.Next() {
		var a APAnalysis
		var c sql.NullInt64
		if err := rows.Scan(&a.BSSID, &a.FirstSeen, &a.LastSeen, &a.ProbesConnected, &a.AvgRSSI, &c, &a.TotalSamples); err != nil {
			return nil, err
		}
		if c.Valid {
			a.Channel = int(c.Int64)
		}
		res = append(res, a)
	}
	return res, nil
}

func (r *AnalyticsRepository) GetCongestionAnalysis(ctx context.Context, start, end time.Time) ([]CongestionAnalysis, error) {
	query := `
		SELECT 
			time_bucket('1 hour', timestamp) as hour,
			AVG(neighbors) as avg_neighbors,
			AVG(overlap) as avg_overlap,
			AVG(utilization) as avg_utilization,
			MAX(utilization) as peak_utilization,
			COUNT(DISTINCT CASE WHEN utilization > 70 THEN probe_id END) as congested_probes
		FROM telemetry
		WHERE timestamp >= $1
		  AND timestamp <= $2
		  AND neighbors IS NOT NULL
		GROUP BY hour
		ORDER BY hour
	`
	rows, err := r.db.QueryContext(ctx, query, start, end)
	if err != nil {
		return nil, fmt.Errorf("failed to get congestion: %w", err)
	}
	defer rows.Close()

	res := []CongestionAnalysis{}
	for rows.Next() {
		var c CongestionAnalysis
		var au, pu sql.NullFloat64
		if err := rows.Scan(&c.Hour, &c.AvgNeighbors, &c.AvgOverlap, &au, &pu, &c.CongestedProbes); err != nil {
			return nil, err
		}
		if au.Valid {
			c.AvgUtilization = au.Float64
		}
		if pu.Valid {
			c.PeakUtilization = pu.Float64
		}
		res = append(res, c)
	}
	return res, nil
}

func (r *AnalyticsRepository) GetPerformanceMetrics(ctx context.Context, probeID string, start, end time.Time) (*PerformanceMetrics, error) {
	whereClause := "timestamp >= $1 AND timestamp <= $2 AND latency IS NOT NULL"
	args := []interface{}{start, end}
	if probeID != "" && probeID != "all" {
		whereClause += " AND probe_id = $3"
		args = append(args, probeID)
	}

	query := fmt.Sprintf(`
		SELECT 
			AVG(rssi) as avg_rssi,
			MIN(rssi) as min_rssi,
			MAX(rssi) as max_rssi,
			AVG(latency) as avg_latency,
			MIN(latency) as min_latency,
			MAX(latency) as max_latency,
			PERCENTILE_CONT(0.5) WITHIN GROUP (ORDER BY latency) as p50_latency,
			PERCENTILE_CONT(0.95) WITHIN GROUP (ORDER BY latency) as p95_latency,
			PERCENTILE_CONT(0.99) WITHIN GROUP (ORDER BY latency) as p99_latency,
			AVG(packet_loss) as avg_packet_loss,
			AVG(dns_time) as avg_dns_time,
			COUNT(*) as sample_count
		FROM telemetry
		WHERE %s
	`, whereClause)

	metrics := &PerformanceMetrics{
		Period: fmt.Sprintf("%s to %s", start.Format("2006-01-02"), end.Format("2006-01-02")),
	}

	var avgRSSI, avgLat, minLat, maxLat, p50, p95, p99, avgLoss, avgDNS sql.NullFloat64
	var minRSSI, maxRSSI sql.NullInt64

	err := r.db.QueryRowContext(ctx, query, args...).Scan(
		&avgRSSI, &minRSSI, &maxRSSI,
		&avgLat, &minLat, &maxLat,
		&p50, &p95, &p99,
		&avgLoss, &avgDNS,
		&metrics.SampleCount,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to get performance metrics: %w", err)
	}

	if avgRSSI.Valid {
		metrics.AvgRSSI = avgRSSI.Float64
	}
	if minRSSI.Valid {
		metrics.MinRSSI = int(minRSSI.Int64)
	}
	if maxRSSI.Valid {
		metrics.MaxRSSI = int(maxRSSI.Int64)
	}

	if avgLat.Valid {
		metrics.AvgLatency = avgLat.Float64
	}
	if minLat.Valid {
		metrics.MinLatency = minLat.Float64
	}
	if maxLat.Valid {
		metrics.MaxLatency = maxLat.Float64
	}

	if p50.Valid {
		metrics.P50Latency = p50.Float64
	}
	if p95.Valid {
		metrics.P95Latency = p95.Float64
	}
	if p99.Valid {
		metrics.P99Latency = p99.Float64
	}

	if avgLoss.Valid {
		metrics.AvgPacketLoss = avgLoss.Float64
	}
	if avgDNS.Valid {
		metrics.AvgDNSTime = avgDNS.Float64
	}
	metrics.StabilityScore = calculateStabilityScore(metrics.AvgLatency, metrics.AvgPacketLoss)

	return metrics, nil
}

func (r *AnalyticsRepository) GetProbeComparison(ctx context.Context, probeIDs []string, start, end time.Time) ([]ProbeComparison, error) {
	query := `
		SELECT 
			t.probe_id,
			p.location,
			AVG(t.rssi) as avg_rssi,
			AVG(t.latency) as avg_latency,
			AVG(t.packet_loss) as avg_packet_loss,
			AVG(t.link_quality) as avg_link_quality,
			(COUNT(*) * 100.0 / EXTRACT(EPOCH FROM ($3 - $2)) * 30) as uptime_percent,
			COUNT(*) as sample_count
		FROM telemetry t
		JOIN probes p ON t.probe_id = p.probe_id
		WHERE t.probe_id = ANY($1)
		  AND t.timestamp >= $2
		  AND t.timestamp <= $3
		GROUP BY t.probe_id, p.location
		ORDER BY avg_rssi DESC
	`

	rows, err := r.db.QueryContext(ctx, query, probeIDs, start, end)
	if err != nil {
		return nil, fmt.Errorf("failed to get probe comparison: %w", err)
	}
	defer rows.Close()

	comparison := []ProbeComparison{}
	for rows.Next() {
		var pc ProbeComparison
		var linkQuality sql.NullFloat64
		if err := rows.Scan(&pc.ProbeID, &pc.Location, &pc.AvgRSSI, &pc.AvgLatency, &pc.AvgPacketLoss, &linkQuality, &pc.UptimePercent, &pc.SampleCount); err != nil {
			return nil, fmt.Errorf("failed to scan probe comparison: %w", err)
		}
		if linkQuality.Valid {
			pc.LinkQuality = linkQuality.Float64
		}
		comparison = append(comparison, pc)
	}

	return comparison, nil
}

func (r *AnalyticsRepository) GetNetworkHealth(ctx context.Context) (*NetworkHealth, error) {
	const activeWindow = 5 * time.Minute

	query := `
		WITH metrics AS (
			SELECT 
				COUNT(DISTINCT probe_id) as active_count,
				AVG(latency) as avg_latency,
				AVG(rssi) as avg_rssi,
				AVG(packet_loss) as avg_loss
			FROM telemetry
			WHERE timestamp >= NOW() - $1::interval
		),
		total AS (
			SELECT COUNT(*) as total_count FROM probes
		)
		SELECT 
			t.total_count,
			COALESCE(m.active_count, 0),
			COALESCE(m.avg_rssi, 0),
			COALESCE(m.avg_latency, 0),
			COALESCE(m.avg_loss, 0)
		FROM total t, metrics m
	`

	health := &NetworkHealth{
		Timestamp: time.Now(),
	}

	var total, active int
	var rssi, latency, loss sql.NullFloat64

	err := r.db.QueryRowContext(ctx, query, activeWindow.String()).Scan(
		&total,
		&active,
		&rssi,
		&latency,
		&loss,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to get network health: %w", err)
	}

	health.TotalProbes = total
	health.ActiveProbes = active
	health.StaleProbes = total - active

	if rssi.Valid {
		health.AvgRSSI = rssi.Float64
	}
	if latency.Valid {
		health.AvgLatency = latency.Float64
	}
	if loss.Valid {
		health.AvgPacketLoss = loss.Float64
	}

	score := 100.0
	if health.AvgLatency > 50 {
		score -= (health.AvgLatency - 50) * 0.5
	}
	score -= health.AvgPacketLoss * 5

	if score < 0 {
		score = 0
	}
	health.HealthScore = score

	return health, nil
}

func (r *AnalyticsRepository) DetectAnomalies(ctx context.Context, probeID string, hours int) ([]AnomalyDetection, error) {
	query := `
		WITH stats AS (
			SELECT 
				AVG(rssi) as avg_rssi,
				STDDEV(rssi) as stddev_rssi,
				AVG(latency) as avg_latency,
				STDDEV(latency) as stddev_latency,
				AVG(packet_loss) as avg_packet_loss,
				STDDEV(packet_loss) as stddev_packet_loss
			FROM telemetry
			WHERE probe_id = $1
			  AND timestamp >= NOW() - INTERVAL '1 hour' * $2
		),
		recent AS (
			SELECT timestamp, rssi, latency, packet_loss
			FROM telemetry
			WHERE probe_id = $1
			  AND timestamp >= NOW() - INTERVAL '15 minutes'
		)
		SELECT 
			timestamp,
			'rssi' as metric_type,
			rssi as value,
			s.avg_rssi as expected_value,
			ABS(rssi - s.avg_rssi) / NULLIF(s.stddev_rssi, 0) as deviation
		FROM recent r, stats s
		WHERE ABS(rssi - s.avg_rssi) > 2 * s.stddev_rssi
		  AND s.stddev_rssi > 0
		UNION ALL
		SELECT 
			timestamp,
			'latency' as metric_type,
			latency as value,
			s.avg_latency as expected_value,
			ABS(latency - s.avg_latency) / NULLIF(s.stddev_latency, 0) as deviation
		FROM recent r, stats s
		WHERE ABS(latency - s.avg_latency) > 2 * s.stddev_latency
		  AND s.stddev_latency > 0
		  AND latency IS NOT NULL
		UNION ALL
		SELECT 
			timestamp,
			'packet_loss' as metric_type,
			packet_loss as value,
			s.avg_packet_loss as expected_value,
			ABS(packet_loss - s.avg_packet_loss) / NULLIF(s.stddev_packet_loss, 0) as deviation
		FROM recent r, stats s
		WHERE ABS(packet_loss - s.avg_packet_loss) > 2 * s.stddev_packet_loss
		  AND s.stddev_packet_loss > 0
		  AND packet_loss IS NOT NULL
		ORDER BY timestamp DESC
	`

	rows, err := r.db.QueryContext(ctx, query, probeID, hours)
	if err != nil {
		return nil, fmt.Errorf("failed to detect anomalies: %w", err)
	}
	defer rows.Close()

	anomalies := []AnomalyDetection{}
	for rows.Next() {
		var a AnomalyDetection
		a.ProbeID = probeID

		if err := rows.Scan(&a.Timestamp, &a.MetricType, &a.Value, &a.ExpectedValue, &a.Deviation); err != nil {
			return nil, fmt.Errorf("failed to scan anomaly: %w", err)
		}

		if a.Deviation > 4 {
			a.Severity = "critical"
		} else if a.Deviation > 3 {
			a.Severity = "high"
		} else {
			a.Severity = "medium"
		}

		anomalies = append(anomalies, a)
	}

	return anomalies, nil
}

func (r *AnalyticsRepository) GetRoamingAnalysis(ctx context.Context, probeID string, start, end time.Time) ([]APAnalysis, error) {
	query := `
		WITH ap_transitions AS (
			SELECT 
				timestamp,
				bssid,
				LAG(bssid) OVER (ORDER BY timestamp) as prev_bssid,
				rssi,
				channel
			FROM telemetry
			WHERE probe_id = $1
			  AND timestamp >= $2
			  AND timestamp <= $3
			  AND bssid IS NOT NULL
			ORDER BY timestamp
		)
		SELECT 
			bssid,
			MIN(timestamp) as first_seen,
			MAX(timestamp) as last_seen,
			1 as probes_connected,
			AVG(rssi) as avg_rssi,
			MODE() WITHIN GROUP (ORDER BY channel) as channel,
			COUNT(*) as total_samples
		FROM ap_transitions
		WHERE bssid != prev_bssid OR prev_bssid IS NULL
		GROUP BY bssid
		ORDER BY first_seen
	`

	rows, err := r.db.QueryContext(ctx, query, probeID, start, end)
	if err != nil {
		return nil, fmt.Errorf("failed to get roaming analysis: %w", err)
	}
	defer rows.Close()

	roaming := []APAnalysis{}
	for rows.Next() {
		var ap APAnalysis
		var channel sql.NullInt64
		if err := rows.Scan(&ap.BSSID, &ap.FirstSeen, &ap.LastSeen, &ap.ProbesConnected, &ap.AvgRSSI, &channel, &ap.TotalSamples); err != nil {
			return nil, fmt.Errorf("failed to scan roaming analysis: %w", err)
		}
		if channel.Valid {
			ap.Channel = int(channel.Int64)
		}
		roaming = append(roaming, ap)
	}

	return roaming, nil
}
func calculateStabilityScore(latency, packetLoss float64) float64 {
	score := 100.0
	if latency > 40 {
		score -= (latency - 40) * 0.5
	}
	score -= packetLoss * 5.0
	if score < 0 {
		return 0
	}

	return score
}
