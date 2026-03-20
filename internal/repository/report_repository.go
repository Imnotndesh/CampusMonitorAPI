package repository

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"CampusMonitorAPI/internal/models"

	"github.com/lib/pq"
)

type ReportRepository struct {
	alertRepo     *AlertRepository
	telemetryRepo *TelemetryRepository
	probeRepo     *ProbeRepository
	commandRepo   *CommandRepository
	fleetRepo     *FleetRepository
	analyticsRepo *AnalyticsRepository
	db            *sql.DB
}

func NewReportRepository(
	alertRepo *AlertRepository,
	telemetryRepo *TelemetryRepository,
	probeRepo *ProbeRepository,
	commandRepo *CommandRepository,
	fleetRepo *FleetRepository,
	analyticsRepo *AnalyticsRepository,
	db *sql.DB,
) *ReportRepository {
	return &ReportRepository{
		alertRepo:     alertRepo,
		telemetryRepo: telemetryRepo,
		probeRepo:     probeRepo,
		commandRepo:   commandRepo,
		fleetRepo:     fleetRepo,
		analyticsRepo: analyticsRepo,
		db:            db,
	}
}

func (r *ReportRepository) AlertReportData(ctx context.Context, from, to time.Time, probeIDs []string) (*models.AlertReport, error) {

	query := `
		SELECT id, probe_id, alert_type, severity, message, threshold_value, actual_value, triggered_at, resolved_at, acknowledged
		FROM alerts
		WHERE triggered_at BETWEEN $1 AND $2
	`
	args := []interface{}{from, to}
	if len(probeIDs) > 0 {
		query += " AND probe_id = ANY($3)"
		args = append(args, pq.Array(probeIDs))
	}
	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var alerts []models.AlertHistoryEntry
	summary := models.AlertSummary{
		BySeverity: make(map[string]int),
		ByType:     make(map[string]int),
	}
	timeline := make(map[time.Time]int)

	for rows.Next() {
		var a models.AlertHistoryEntry
		var resolvedAt sql.NullTime
		var threshold, actual sql.NullFloat64
		err := rows.Scan(&a.ID, &a.ProbeID, &a.AlertType, &a.Severity, &a.Message,
			&threshold, &actual, &a.TriggeredAt, &resolvedAt, &a.Acknowledged)
		if err != nil {
			return nil, err
		}
		if resolvedAt.Valid {
			a.ResolvedAt = &resolvedAt.Time
		}
		alerts = append(alerts, a)

		summary.BySeverity[a.Severity]++
		summary.ByType[a.AlertType]++
		if a.ResolvedAt == nil {
			summary.ActiveCount++
		} else {
			summary.ResolvedCount++
		}
		bucket := a.TriggeredAt.Truncate(time.Hour)
		timeline[bucket]++
	}

	summary.Total = len(alerts)
	var timelinePoints []models.AlertTimelinePoint
	for t, cnt := range timeline {
		timelinePoints = append(timelinePoints, models.AlertTimelinePoint{Bucket: t, Count: cnt})
	}
	summary.Timeline = timelinePoints

	return &models.AlertReport{
		Summary: summary,
		Alerts:  alerts,
	}, nil
}

func (r *ReportRepository) AnalyticsReportData(ctx context.Context, from, to time.Time, probeIDs []string) (*models.AnalyticsReport, error) {
	// Overall metrics using analyticsRepo (it supports empty probeID for all)
	perf, err := r.analyticsRepo.GetPerformanceMetrics(ctx, "", from, to)
	if err != nil {
		return nil, err
	}
	overall := models.OverallMetrics{
		AvgRSSI:        perf.AvgRSSI,
		MinRSSI:        perf.MinRSSI,
		MaxRSSI:        perf.MaxRSSI,
		AvgLatency:     perf.AvgLatency,
		AvgPacketLoss:  perf.AvgPacketLoss,
		AvgDNSTime:     perf.AvgDNSTime,
		SampleCount:    perf.SampleCount,
		StabilityScore: perf.StabilityScore,
	}

	// RSSI time series
	rssiTS, err := r.analyticsRepo.GetRSSITimeSeries(ctx, "", from, to, "1 hour")
	if err != nil {
		rssiTS = []TimeSeriesPoint{}
	}
	modelRSSI := make([]models.TimeSeriesPoint, len(rssiTS))
	for i, p := range rssiTS {
		modelRSSI[i] = toModelTimeSeriesPoint(p)
	}

	// Latency time series
	latTS, err := r.analyticsRepo.GetLatencyTimeSeries(ctx, "", from, to, "1 hour")
	if err != nil {
		latTS = []TimeSeriesPoint{}
	}
	modelLat := make([]models.TimeSeriesPoint, len(latTS))
	for i, p := range latTS {
		modelLat[i] = toModelTimeSeriesPoint(p)
	}

	// Channel distribution
	channelDist, err := r.analyticsRepo.GetChannelDistribution(ctx, from, to)
	if err != nil {
		channelDist = []ChannelDistribution{}
	}
	modelChannels := make([]models.ChannelDistribution, len(channelDist))
	for i, c := range channelDist {
		modelChannels[i] = toModelChannelDistribution(c)
	}

	// Top APs
	aps, err := r.analyticsRepo.GetAPAnalysis(ctx, from, to)
	if err != nil {
		aps = []APAnalysis{}
	}
	if len(aps) > 10 {
		aps = aps[:10]
	}
	modelAPs := make([]models.APAnalysis, len(aps))
	for i, ap := range aps {
		modelAPs[i] = toModelAPAnalysis(ap)
	}

	// Congestion analysis
	congestion, err := r.analyticsRepo.GetCongestionAnalysis(ctx, from, to)
	if err != nil {
		congestion = []CongestionAnalysis{}
	}
	modelCongestion := make([]models.CongestionAnalysis, len(congestion))
	for i, c := range congestion {
		modelCongestion[i] = toModelCongestionAnalysis(c)
	}

	return &models.AnalyticsReport{
		Period:            models.TimeRange{From: from, To: to},
		Overall:           overall,
		RSSITimeSeries:    modelRSSI,
		LatencyTimeSeries: modelLat,
		ChannelDist:       modelChannels,
		TopAPs:            modelAPs,
		Congestion:        modelCongestion,
	}, nil
}

func (r *ReportRepository) FleetReportData(ctx context.Context) (*models.FleetReport, error) {
	// Get fleet status (summary counts)
	status, err := r.fleetRepo.GetFleetStatus(ctx)
	if err != nil {
		return nil, err
	}

	// Get total probes count
	var totalProbes int
	err = r.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM probes").Scan(&totalProbes)
	if err != nil {
		return nil, err
	}

	// Get active probes (last seen within 5 minutes)
	var activeProbes int
	err = r.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM probes WHERE status = 'active' AND last_seen > NOW() - INTERVAL '5 minutes'").Scan(&activeProbes)
	if err != nil {
		return nil, err
	}

	staleProbes := totalProbes - activeProbes

	// Compute health score
	healthScore := 100.0
	if totalProbes > 0 {
		healthScore = float64(activeProbes)/float64(totalProbes)*100 - float64(staleProbes)/float64(totalProbes)*10
		if healthScore < 0 {
			healthScore = 0
		}
	}

	// Command summary
	cmdStats, err := r.commandRepo.GetStatistics(ctx)
	if err != nil {
		return nil, err
	}
	cmdSummary := models.FleetCommandSummary{
		Total:     0,
		Pending:   cmdStats["pending"] + cmdStats["sent"] + cmdStats["processing"],
		Completed: cmdStats["completed"],
		Failed:    cmdStats["failed"],
	}
	cmdSummary.Total = cmdSummary.Pending + cmdSummary.Completed + cmdSummary.Failed

	// Get groups with counts
	groupQuery := `
        SELECT 
            g.id, g.name,
            COUNT(DISTINCT fp.probe_id) as probe_count,
            COUNT(CASE WHEN p.status = 'active' AND p.last_seen > NOW() - INTERVAL '5 minutes' THEN 1 END) as online,
            COUNT(DISTINCT a.id) as alert_count
        FROM fleet_groups g
        LEFT JOIN fleet_probes fp ON fp.groups @> jsonb_build_array(g.id)
        LEFT JOIN probes p ON fp.probe_id = p.probe_id
        LEFT JOIN alerts a ON a.probe_id = p.probe_id AND a.resolved_at IS NULL
        GROUP BY g.id, g.name
        ORDER BY g.name
    `
	rows, err := r.db.QueryContext(ctx, groupQuery)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var groupSummaries []models.FleetGroupSummary
	for rows.Next() {
		var g models.FleetGroupSummary
		var id, name string
		var probeCount, online, alertCount int
		if err := rows.Scan(&id, &name, &probeCount, &online, &alertCount); err != nil {
			return nil, err
		}
		g.ID = id
		g.Name = name
		g.ProbeCount = probeCount
		g.Online = online
		g.AlertCount = alertCount
		groupSummaries = append(groupSummaries, g)
	}

	return &models.FleetReport{
		Summary: models.FleetSummary{
			TotalProbes:   totalProbes,
			ManagedProbes: status.ManagedProbes,
			ActiveProbes:  activeProbes,
			StaleProbes:   staleProbes,
			HealthScore:   healthScore,
		},
		Groups:   groupSummaries,
		Commands: cmdSummary,
	}, nil
}
func (r *ReportRepository) ProbeStatusReportData(ctx context.Context) (*models.ProbeStatusReport, error) {
	probes, err := r.probeRepo.GetAll(ctx)
	if err != nil {
		return nil, err
	}

	summary := models.ProbeStatusSummary{
		Total:   len(probes),
		Active:  0,
		Offline: 0,
		Pending: 0,
	}
	var entries []models.ProbeStatusEntry
	for _, p := range probes {

		now := time.Now()
		var statusText string
		switch p.Status {
		case "active":
			if now.Sub(p.LastSeen) < 5*time.Minute {
				statusText = "online"
				summary.Active++
			} else {
				statusText = "offline"
				summary.Offline++
			}
		case "pending":
			statusText = "pending"
			summary.Pending++
		default:
			statusText = p.Status
		}

		var rssi *int
		var latency *int
		var packetLoss *float64
		if tel, err := r.telemetryRepo.GetLatestByProbe(ctx, p.ProbeID); err == nil && tel != nil {
			rssi = tel.RSSI
			latency = tel.Latency
			packetLoss = tel.PacketLoss
		}

		entries = append(entries, models.ProbeStatusEntry{
			ProbeID:         p.ProbeID,
			Location:        p.Location,
			Building:        p.Building,
			Floor:           p.Floor,
			Status:          statusText,
			LastSeen:        p.LastSeen,
			FirmwareVersion: p.FirmwareVersion,
			RSSI:            rssi,
			Latency:         latency,
			PacketLoss:      packetLoss,
		})
	}
	return &models.ProbeStatusReport{
		Summary: summary,
		Probes:  entries,
	}, nil
}

func (r *ReportRepository) ComplianceReportData(ctx context.Context, thresholds models.ComplianceThresholds) (*models.ComplianceReport, error) {
	probes, err := r.probeRepo.GetAll(ctx)
	if err != nil {
		return nil, err
	}

	nonCompliant := []models.NonCompliantProbe{}
	compliant := 0
	for _, p := range probes {
		tel, err := r.telemetryRepo.GetLatestByProbe(ctx, p.ProbeID)
		if err != nil || tel == nil {
			continue
		}
		reasons := []string{}
		if tel.RSSI != nil && *tel.RSSI < thresholds.MinRSSI {
			reasons = append(reasons, fmt.Sprintf("RSSI too low (%d < %d)", *tel.RSSI, thresholds.MinRSSI))
		}
		if tel.Latency != nil && *tel.Latency > thresholds.MaxLatency {
			reasons = append(reasons, fmt.Sprintf("Latency too high (%d > %d)", *tel.Latency, thresholds.MaxLatency))
		}
		if tel.PacketLoss != nil && *tel.PacketLoss > thresholds.MaxPacketLoss {
			reasons = append(reasons, fmt.Sprintf("Packet loss too high (%.2f > %.2f)", *tel.PacketLoss, thresholds.MaxPacketLoss))
		}
		if len(reasons) > 0 {
			nonCompliant = append(nonCompliant, models.NonCompliantProbe{
				ProbeID:    p.ProbeID,
				Location:   p.Location,
				RSSI:       tel.RSSI,
				Latency:    tel.Latency,
				PacketLoss: tel.PacketLoss,
				Reason:     reasons[0],
			})
		} else {
			compliant++
		}
	}
	return &models.ComplianceReport{
		Thresholds:   thresholds,
		NonCompliant: nonCompliant,
		Compliant:    compliant,
		TotalProbes:  len(probes),
	}, nil
}

func (r *ReportRepository) FirmwareVersionReportData(ctx context.Context) (*models.FirmwareVersionReport, error) {
	probes, err := r.probeRepo.GetAll(ctx)
	if err != nil {
		return nil, err
	}
	versionCount := make(map[string]int)
	versionProbes := make(map[string][]string)
	for _, p := range probes {
		fw := p.FirmwareVersion
		versionCount[fw]++
		versionProbes[fw] = append(versionProbes[fw], p.ProbeID)
	}

	var groups []models.FirmwareGroup
	for v, cnt := range versionCount {
		groups = append(groups, models.FirmwareGroup{
			Version:  v,
			Count:    cnt,
			ProbeIDs: versionProbes[v],
		})
	}
	mostCommon := ""
	maxCnt := 0
	for v, cnt := range versionCount {
		if cnt > maxCnt {
			maxCnt = cnt
			mostCommon = v
		}
	}

	outdated := []models.OutdatedProbe{}
	for _, p := range probes {
		if p.FirmwareVersion != mostCommon {
			outdated = append(outdated, models.OutdatedProbe{
				ProbeID:        p.ProbeID,
				CurrentVersion: p.FirmwareVersion,
				TargetVersion:  mostCommon,
				LastSeen:       p.LastSeen,
			})
		}
	}
	return &models.FirmwareVersionReport{
		GeneratedAt: time.Now(),
		Summary: models.FirmwareSummary{
			TotalProbes:    len(probes),
			UniqueVersions: len(versionCount),
			MostCommon:     mostCommon,
			UpToDateCount:  len(probes) - len(outdated),
		},
		ByVersion:      groups,
		OutdatedProbes: outdated,
	}, nil
}

func (r *ReportRepository) OutageReportData(ctx context.Context, from, to time.Time, probeIDs []string) (*models.OutageReport, error) {
	var outages []models.ProbeOutage
	probesToCheck := probeIDs
	if len(probesToCheck) == 0 {
		// Get all probes
		probes, err := r.probeRepo.GetAll(ctx)
		if err != nil {
			return nil, err
		}
		for _, p := range probes {
			probesToCheck = append(probesToCheck, p.ProbeID)
		}
	}

	for _, pid := range probesToCheck {
		// Query timestamps within range
		query := `
            SELECT timestamp FROM telemetry
            WHERE probe_id = $1 AND timestamp BETWEEN $2 AND $3
            ORDER BY timestamp
        `
		rows, err := r.db.QueryContext(ctx, query, pid, from, to)
		if err != nil {
			continue
		}
		var timestamps []time.Time
		for rows.Next() {
			var ts time.Time
			if err := rows.Scan(&ts); err != nil {
				continue
			}
			timestamps = append(timestamps, ts)
		}
		rows.Close()

		if len(timestamps) == 0 {
			// Whole period is an outage
			outages = append(outages, models.ProbeOutage{
				ProbeID:  pid,
				Start:    from,
				End:      &to,
				Duration: to.Sub(from),
				Reason:   "No telemetry",
			})
			continue
		}
		// Check gap from start to first timestamp
		if timestamps[0].Sub(from) > 5*time.Minute {
			startOutage := from
			endOutage := timestamps[0]
			duration := endOutage.Sub(startOutage)
			outages = append(outages, models.ProbeOutage{
				ProbeID:  pid,
				Start:    startOutage,
				End:      &endOutage,
				Duration: duration,
				Reason:   "No telemetry",
			})
		}
		// Check gaps between timestamps
		for i := 1; i < len(timestamps); i++ {
			gap := timestamps[i].Sub(timestamps[i-1])
			if gap > 5*time.Minute {
				startOutage := timestamps[i-1]
				endOutage := timestamps[i]
				duration := endOutage.Sub(startOutage)
				outages = append(outages, models.ProbeOutage{
					ProbeID:  pid,
					Start:    startOutage,
					End:      &endOutage,
					Duration: duration,
					Reason:   "Telemetry gap",
				})
			}
		}
		// Gap from last timestamp to end
		if to.Sub(timestamps[len(timestamps)-1]) > 5*time.Minute {
			startOutage := timestamps[len(timestamps)-1]
			endOutage := to
			duration := endOutage.Sub(startOutage)
			outages = append(outages, models.ProbeOutage{
				ProbeID:  pid,
				Start:    startOutage,
				End:      &endOutage,
				Duration: duration,
				Reason:   "No telemetry",
			})
		}
	}

	// Summary calculation (unchanged)
	summary := models.OutageSummary{
		TotalOutages:   len(outages),
		TotalDowntime:  0,
		AffectedProbes: 0,
		LongestOutage:  0,
	}
	probeSet := make(map[string]bool)
	for _, o := range outages {
		summary.TotalDowntime += o.Duration
		probeSet[o.ProbeID] = true
		if o.Duration > summary.LongestOutage {
			summary.LongestOutage = o.Duration
		}
	}
	summary.AffectedProbes = len(probeSet)
	if summary.TotalOutages > 0 {
		summary.AvgOutageDuration = summary.TotalDowntime / time.Duration(summary.TotalOutages)
	}

	return &models.OutageReport{
		Period:  models.TimeRange{From: from, To: to},
		Summary: summary,
		Outages: outages,
	}, nil
}

// GetNetworkBaselineReportData fetches data for the network baseline report
func (r *ReportRepository) GetNetworkBaselineReportData(ctx context.Context, from, to time.Time) (*models.NetworkBaselineReport, error) {
	// Get overall metrics (which already includes latency percentiles)
	perf, err := r.analyticsRepo.GetPerformanceMetrics(ctx, "", from, to)
	if err != nil {
		return nil, err
	}

	// Build RSSI distribution – we need percentiles for RSSI (not available in PerformanceMetrics)
	// We'll query directly
	query := `
        SELECT 
            percentile_cont(0.5) WITHIN GROUP (ORDER BY rssi) as p50_rssi,
            percentile_cont(0.95) WITHIN GROUP (ORDER BY rssi) as p95_rssi,
            percentile_cont(0.99) WITHIN GROUP (ORDER BY rssi) as p99_rssi,
            STDDEV(rssi) as rssi_stddev
        FROM telemetry
        WHERE timestamp BETWEEN $1 AND $2
          AND rssi IS NOT NULL
    `
	var p50RSSI, p95RSSI, p99RSSI, rssiStdDev sql.NullFloat64
	err = r.db.QueryRowContext(ctx, query, from, to).Scan(&p50RSSI, &p95RSSI, &p99RSSI, &rssiStdDev)
	if err != nil {
		// not fatal, proceed with zeros
	}

	// Build Packet Loss distribution (percentiles)
	query = `
        SELECT 
            percentile_cont(0.5) WITHIN GROUP (ORDER BY packet_loss) as p50_loss,
            percentile_cont(0.95) WITHIN GROUP (ORDER BY packet_loss) as p95_loss,
            percentile_cont(0.99) WITHIN GROUP (ORDER BY packet_loss) as p99_loss,
            STDDEV(packet_loss) as loss_stddev
        FROM telemetry
        WHERE timestamp BETWEEN $1 AND $2
          AND packet_loss IS NOT NULL
    `
	var p50Loss, p95Loss, p99Loss, lossStdDev sql.NullFloat64
	_ = r.db.QueryRowContext(ctx, query, from, to).Scan(&p50Loss, &p95Loss, &p99Loss, &lossStdDev)

	// Build Throughput distribution (if available)
	query = `
        SELECT 
            AVG(throughput) as avg_throughput,
            percentile_cont(0.5) WITHIN GROUP (ORDER BY throughput) as p50_throughput,
            percentile_cont(0.95) WITHIN GROUP (ORDER BY throughput) as p95_throughput,
            STDDEV(throughput) as throughput_stddev
        FROM telemetry
        WHERE timestamp BETWEEN $1 AND $2
          AND throughput IS NOT NULL
    `
	var avgThroughput, p50Throughput, p95Throughput, throughputStdDev sql.NullFloat64
	_ = r.db.QueryRowContext(ctx, query, from, to).Scan(&avgThroughput, &p50Throughput, &p95Throughput, &throughputStdDev)

	// Build RSSI distribution struct
	rssiDist := models.MetricDistribution{
		Min:         float64(perf.MinRSSI),
		Max:         float64(perf.MaxRSSI),
		Avg:         perf.AvgRSSI,
		SampleCount: perf.SampleCount,
	}
	if p50RSSI.Valid {
		rssiDist.P50 = p50RSSI.Float64
	}
	if p95RSSI.Valid {
		rssiDist.P95 = p95RSSI.Float64
	}
	if p99RSSI.Valid {
		rssiDist.P99 = p99RSSI.Float64
	}
	if rssiStdDev.Valid {
		rssiDist.StdDev = rssiStdDev.Float64
	}

	// Latency distribution
	latencyDist := models.MetricDistribution{
		Min:         perf.MinLatency,
		Max:         perf.MaxLatency,
		Avg:         perf.AvgLatency,
		P50:         perf.P50Latency,
		P95:         perf.P95Latency,
		P99:         perf.P99Latency,
		SampleCount: perf.SampleCount,
	}

	// Packet Loss distribution
	lossDist := models.MetricDistribution{
		Avg:         perf.AvgPacketLoss,
		SampleCount: perf.SampleCount,
	}
	if p50Loss.Valid {
		lossDist.P50 = p50Loss.Float64
	}
	if p95Loss.Valid {
		lossDist.P95 = p95Loss.Float64
	}
	if p99Loss.Valid {
		lossDist.P99 = p99Loss.Float64
	}
	if lossStdDev.Valid {
		lossDist.StdDev = lossStdDev.Float64
	}

	// Throughput distribution
	throughputDist := models.MetricDistribution{
		SampleCount: perf.SampleCount, // rough approximation
	}
	if avgThroughput.Valid {
		throughputDist.Avg = avgThroughput.Float64
	}
	if p50Throughput.Valid {
		throughputDist.P50 = p50Throughput.Float64
	}
	if p95Throughput.Valid {
		throughputDist.P95 = p95Throughput.Float64
	}
	if throughputStdDev.Valid {
		throughputDist.StdDev = throughputStdDev.Float64
	}

	return &models.NetworkBaselineReport{
		Period:     models.TimeRange{From: from, To: to},
		RSSI:       rssiDist,
		Latency:    latencyDist,
		PacketLoss: lossDist,
		Throughput: throughputDist,
	}, nil
}

func (r *ReportRepository) CommandSuccessReportData(ctx context.Context, from, to time.Time, probeIDs []string) (*models.CommandSuccessReport, error) {
	query := `
        SELECT command_type, status, COUNT(*) as cnt
        FROM commands
        WHERE issued_at BETWEEN $1 AND $2
    `
	args := []interface{}{from, to}
	if len(probeIDs) > 0 {
		query += " AND probe_id = ANY($3)"
		args = append(args, pq.Array(probeIDs))
	}
	query += " GROUP BY command_type, status"

	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	typeCount := make(map[string]map[string]int)
	for rows.Next() {
		var cmdType, status string
		var cnt int
		if err := rows.Scan(&cmdType, &status, &cnt); err != nil {
			return nil, err
		}
		if _, ok := typeCount[cmdType]; !ok {
			typeCount[cmdType] = make(map[string]int)
		}
		typeCount[cmdType][status] = cnt
	}

	var byType []models.CommandTypeRate
	var totalAll, succeededAll, failedAll, pendingAll int
	for cmdType, statMap := range typeCount {
		total := statMap["pending"] + statMap["sent"] + statMap["processing"] + statMap["completed"] + statMap["failed"]
		succeeded := statMap["completed"]
		failed := statMap["failed"]
		pending := statMap["pending"] + statMap["sent"] + statMap["processing"]
		var rate float64
		if total > 0 {
			rate = float64(succeeded) / float64(total) * 100
		}
		byType = append(byType, models.CommandTypeRate{
			CommandType: cmdType,
			Total:       total,
			Succeeded:   succeeded,
			Failed:      failed,
			SuccessRate: rate,
		})
		totalAll += total
		succeededAll += succeeded
		failedAll += failed
		pendingAll += pending
	}
	overallRate := 0.0
	if totalAll > 0 {
		overallRate = float64(succeededAll) / float64(totalAll) * 100
	}
	overall := models.CommandRateSummary{
		Total:       totalAll,
		Succeeded:   succeededAll,
		Failed:      failedAll,
		Pending:     pendingAll,
		SuccessRate: overallRate,
	}

	return &models.CommandSuccessReport{
		Period:  models.TimeRange{From: from, To: to},
		Overall: overall,
		ByType:  byType,
	}, nil
}

func toModelTimeSeriesPoint(p TimeSeriesPoint) models.TimeSeriesPoint {
	return models.TimeSeriesPoint{Timestamp: p.Timestamp, Value: p.Value}
}

func toModelChannelDistribution(c ChannelDistribution) models.ChannelDistribution {
	return models.ChannelDistribution{
		Channel:     c.Channel,
		ProbeCount:  c.ProbeCount,
		AvgRSSI:     c.AvgRSSI,
		Utilization: c.Utilization,
	}
}

func toModelAPAnalysis(a APAnalysis) models.APAnalysis {
	return models.APAnalysis{
		BSSID:           a.BSSID,
		FirstSeen:       a.FirstSeen,
		LastSeen:        a.LastSeen,
		ProbesConnected: a.ProbesConnected,
		AvgRSSI:         a.AvgRSSI,
		Channel:         a.Channel,
		TotalSamples:    a.TotalSamples,
	}
}

func toModelCongestionAnalysis(c CongestionAnalysis) models.CongestionAnalysis {
	return models.CongestionAnalysis{
		Hour:            c.Hour,
		AvgNeighbors:    c.AvgNeighbors,
		AvgOverlap:      c.AvgOverlap,
		AvgUtilization:  c.AvgUtilization,
		PeakUtilization: c.PeakUtilization,
		CongestedProbes: c.CongestedProbes,
	}
}

// GetSiteSurveyReportData fetches data for site survey report for a specific building/floor.
func (r *ReportRepository) GetSiteSurveyReportData(ctx context.Context, building, floor string, from, to time.Time) (*models.SiteSurveyReport, error) {
	// Default to last 24h if no range provided
	if from.IsZero() {
		from = time.Now().Add(-24 * time.Hour)
	}
	if to.IsZero() {
		to = time.Now()
	}

	// 1. Get all probes in this building/floor
	probes, err := r.probeRepo.GetByBuildingAndFloor(ctx, building, floor)
	if err != nil {
		return nil, err
	}
	probeIDs := make([]string, len(probes))
	for i, p := range probes {
		probeIDs[i] = p.ProbeID
	}

	// 2. Heatmap data: RSSI per location (already grouped by building/floor/location)
	heatmapData, err := r.analyticsRepo.GetHeatmapData(ctx, from, to)
	if err != nil {
		return nil, err
	}
	var heatmapPoints []models.HeatmapPoint
	for _, h := range heatmapData {
		if h.Building == building && h.Floor == floor {
			heatmapPoints = append(heatmapPoints, models.HeatmapPoint{
				Location: h.Location,
				RSSI:     h.AvgRSSI,
			})
		}
	}

	// 3. Channel usage – we can use GetChannelDistribution (global) or filter by probeIDs.
	// For simplicity, we'll use the global distribution; it's still useful.
	channelDist, err := r.analyticsRepo.GetChannelDistribution(ctx, from, to)
	if err != nil {
		return nil, err
	}
	var channelUsage []models.ChannelUsage
	for _, c := range channelDist {
		channelUsage = append(channelUsage, models.ChannelUsage{
			Channel:     c.Channel,
			Utilization: c.Utilization,
			ProbeCount:  c.ProbeCount,
		})
	}

	// 4. AP list filtered by probes in this building/floor
	// We need to query telemetry for these probes only.
	apList, err := r.getAPsForProbes(ctx, probeIDs, from, to)
	if err != nil {
		return nil, err
	}

	// 5. Generate recommendations based on data
	recommendations := generateSiteSurveyRecommendations(channelUsage, heatmapPoints)

	return &models.SiteSurveyReport{
		Building:        building,
		Floor:           floor,
		GeneratedAt:     time.Now(),
		Heatmap:         heatmapPoints,
		ChannelUsage:    channelUsage,
		APList:          apList,
		Recommendations: recommendations,
	}, nil
}

// Helper: get APs for a specific list of probe IDs
func (r *ReportRepository) getAPsForProbes(ctx context.Context, probeIDs []string, from, to time.Time) ([]models.APCoverage, error) {
	if len(probeIDs) == 0 {
		return []models.APCoverage{}, nil
	}

	// We can reuse the APAnalysis query but filter by probe_id IN (...)
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
        WHERE timestamp BETWEEN $1 AND $2
          AND bssid IS NOT NULL
          AND probe_id = ANY($3)
        GROUP BY bssid
        ORDER BY probes_connected DESC, total_samples DESC
    `
	rows, err := r.db.QueryContext(ctx, query, from, to, pq.Array(probeIDs))
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var aps []models.APCoverage
	for rows.Next() {
		var ap models.APCoverage
		var channel sql.NullInt64
		err := rows.Scan(
			&ap.BSSID,
			&ap.FirstSeen,
			&ap.LastSeen,
			&ap.ProbesSeen,
			&ap.AvgRSSI,
			&channel,
			&ap.TotalSamples,
		)
		if err != nil {
			return nil, err
		}
		if channel.Valid {
			ap.Channel = int(channel.Int64)
		}
		aps = append(aps, ap)
	}
	return aps, nil
}

// Helper recommendation function
func generateSiteSurveyRecommendations(channelUsage []models.ChannelUsage, heatmap []models.HeatmapPoint) []string {
	var recs []string

	// Check channel utilization
	for _, cu := range channelUsage {
		if cu.Utilization > 80 {
			recs = append(recs, fmt.Sprintf("Channel %d is heavily utilized (%.1f%%). Consider moving some APs to a different channel.", cu.Channel, cu.Utilization))
		}
	}

	// Check for low signal areas (RSSI < -75)
	lowSignalCount := 0
	for _, h := range heatmap {
		if h.RSSI < -75 {
			lowSignalCount++
		}
	}
	if lowSignalCount > len(heatmap)/3 {
		recs = append(recs, "Several areas have weak signal. Consider adding additional APs or relocating existing ones.")
	}

	if len(recs) == 0 {
		recs = append(recs, "No critical issues detected. Network health appears good.")
	}
	return recs
}
