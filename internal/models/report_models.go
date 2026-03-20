package models

import "time"

type ReportType string

const (
	ReportTypeAlerts          ReportType = "alerts"
	ReportTypeAnalytics       ReportType = "analytics"
	ReportTypeFleet           ReportType = "fleet"
	ReportTypeProbes          ReportType = "probes"
	ReportTypeCompliance      ReportType = "compliance"
	ReportTypeFirmwareVersion ReportType = "firmware_version"
	ReportTypeOutage          ReportType = "outage"
	ReportTypeCommandSuccess  ReportType = "command_success"
	ReportTypeNetworkBaseline ReportType = "network_baseline"
	ReportTypeSiteSurvey      ReportType = "site_survey"
)

type ReportRequest struct {
	Type     ReportType `json:"type" binding:"required"`
	From     time.Time  `json:"from"`
	To       time.Time  `json:"to"`
	ProbeIDs []string   `json:"probe_ids"`
	Building string     `json:"building,omitempty"`
	Floor    string     `json:"floor,omitempty"`
	Groups   []string   `json:"groups"`
	Format   string     `json:"format"`
}
type NetworkBaselineReport struct {
	Period     TimeRange          `json:"period"`
	RSSI       MetricDistribution `json:"rssi"`
	Latency    MetricDistribution `json:"latency"`
	PacketLoss MetricDistribution `json:"packet_loss"`
	Throughput MetricDistribution `json:"throughput"`
}
type MetricDistribution struct {
	Min         float64 `json:"min"`
	Max         float64 `json:"max"`
	Avg         float64 `json:"avg"`
	P50         float64 `json:"p50"`
	P95         float64 `json:"p95"`
	P99         float64 `json:"p99"`
	StdDev      float64 `json:"std_dev"`
	SampleCount int     `json:"sample_count"`
}
type ReportResponse struct {
	Type        ReportType  `json:"type"`
	GeneratedAt time.Time   `json:"generated_at"`
	Period      TimeRange   `json:"period,omitempty"`
	Data        interface{} `json:"data"`
}

type TimeRange struct {
	From time.Time `json:"from"`
	To   time.Time `json:"to"`
}

type AlertReport struct {
	Summary AlertSummary        `json:"summary"`
	Alerts  []AlertHistoryEntry `json:"alerts"`
}

type AlertSummary struct {
	Total         int                  `json:"total"`
	BySeverity    map[string]int       `json:"by_severity"`
	ByType        map[string]int       `json:"by_type"`
	ActiveCount   int                  `json:"active_count"`
	ResolvedCount int                  `json:"resolved_count"`
	Timeline      []AlertTimelinePoint `json:"timeline,omitempty"`
}

type AlertTimelinePoint struct {
	Bucket time.Time `json:"bucket"`
	Count  int       `json:"count"`
}

type AlertHistoryEntry struct {
	ID           int        `json:"id"`
	ProbeID      string     `json:"probe_id"`
	AlertType    string     `json:"alert_type"`
	Severity     string     `json:"severity"`
	Message      string     `json:"message"`
	TriggeredAt  time.Time  `json:"triggered_at"`
	ResolvedAt   *time.Time `json:"resolved_at,omitempty"`
	Acknowledged bool       `json:"acknowledged"`
}

type AnalyticsReport struct {
	Period            TimeRange             `json:"period"`
	Overall           OverallMetrics        `json:"overall"`
	RSSITimeSeries    []TimeSeriesPoint     `json:"rssi_time_series,omitempty"`
	LatencyTimeSeries []TimeSeriesPoint     `json:"latency_time_series,omitempty"`
	ChannelDist       []ChannelDistribution `json:"channel_distribution,omitempty"`
	TopAPs            []APAnalysis          `json:"top_aps,omitempty"`
	Congestion        []CongestionAnalysis  `json:"congestion,omitempty"`
}

type OverallMetrics struct {
	AvgRSSI        float64 `json:"avg_rssi"`
	MinRSSI        int     `json:"min_rssi"`
	MaxRSSI        int     `json:"max_rssi"`
	AvgLatency     float64 `json:"avg_latency"`
	AvgPacketLoss  float64 `json:"avg_packet_loss"`
	AvgDNSTime     float64 `json:"avg_dns_time"`
	SampleCount    int     `json:"sample_count"`
	StabilityScore float64 `json:"stability_score"`
}

type TimeSeriesPoint struct {
	Timestamp time.Time `json:"timestamp"`
	Value     float64   `json:"value"`
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

type FleetReport struct {
	Summary  FleetSummary        `json:"summary"`
	Groups   []FleetGroupSummary `json:"groups"`
	Commands FleetCommandSummary `json:"commands"`
}

type FleetSummary struct {
	TotalProbes   int     `json:"total_probes"`
	ManagedProbes int     `json:"managed_probes"`
	ActiveProbes  int     `json:"active_probes"`
	StaleProbes   int     `json:"stale_probes"`
	HealthScore   float64 `json:"health_score"`
}

type FleetGroupSummary struct {
	ID         string `json:"id"`
	Name       string `json:"name"`
	ProbeCount int    `json:"probe_count"`
	Online     int    `json:"online"`
	AlertCount int    `json:"alert_count"`
}

type FleetCommandSummary struct {
	Total     int `json:"total"`
	Pending   int `json:"pending"`
	Completed int `json:"completed"`
	Failed    int `json:"failed"`
}

type ProbeStatusReport struct {
	Summary ProbeStatusSummary `json:"summary"`
	Probes  []ProbeStatusEntry `json:"probes"`
}

type ProbeStatusSummary struct {
	Total   int `json:"total"`
	Active  int `json:"active"`
	Offline int `json:"offline"`
	Pending int `json:"pending"`
}

type ProbeStatusEntry struct {
	ProbeID         string    `json:"probe_id"`
	Location        string    `json:"location"`
	Building        string    `json:"building"`
	Floor           string    `json:"floor"`
	Status          string    `json:"status"`
	LastSeen        time.Time `json:"last_seen"`
	FirmwareVersion string    `json:"firmware_version"`
	RSSI            *int      `json:"rssi,omitempty"`
	Latency         *int      `json:"latency,omitempty"`
	PacketLoss      *float64  `json:"packet_loss,omitempty"`
}

type ComplianceReport struct {
	Thresholds   ComplianceThresholds `json:"thresholds"`
	NonCompliant []NonCompliantProbe  `json:"non_compliant"`
	Compliant    int                  `json:"compliant_count"`
	TotalProbes  int                  `json:"total_probes"`
}

type ComplianceThresholds struct {
	MinRSSI       int     `json:"min_rssi"`
	MaxLatency    int     `json:"max_latency_ms"`
	MaxPacketLoss float64 `json:"max_packet_loss"`
}

type NonCompliantProbe struct {
	ProbeID    string   `json:"probe_id"`
	Location   string   `json:"location"`
	RSSI       *int     `json:"rssi,omitempty"`
	Latency    *int     `json:"latency,omitempty"`
	PacketLoss *float64 `json:"packet_loss,omitempty"`
	Reason     string   `json:"reason"`
}

type FirmwareVersionReport struct {
	GeneratedAt    time.Time       `json:"generated_at"`
	Summary        FirmwareSummary `json:"summary"`
	ByVersion      []FirmwareGroup `json:"by_version"`
	OutdatedProbes []OutdatedProbe `json:"outdated_probes,omitempty"`
}

type FirmwareSummary struct {
	TotalProbes    int    `json:"total_probes"`
	UniqueVersions int    `json:"unique_versions"`
	MostCommon     string `json:"most_common_version"`
	UpToDateCount  int    `json:"up_to_date_count"`
}

type FirmwareGroup struct {
	Version  string   `json:"version"`
	Count    int      `json:"count"`
	ProbeIDs []string `json:"probe_ids,omitempty"`
}

type OutdatedProbe struct {
	ProbeID        string    `json:"probe_id"`
	CurrentVersion string    `json:"current_version"`
	TargetVersion  string    `json:"target_version,omitempty"`
	LastSeen       time.Time `json:"last_seen"`
}

type OutageReport struct {
	Period  TimeRange     `json:"period"`
	Summary OutageSummary `json:"summary"`
	Outages []ProbeOutage `json:"outages"`
}

type OutageSummary struct {
	TotalOutages      int           `json:"total_outages"`
	TotalDowntime     time.Duration `json:"total_downtime"`
	AffectedProbes    int           `json:"affected_probes"`
	AvgOutageDuration time.Duration `json:"avg_outage_duration"`
	LongestOutage     time.Duration `json:"longest_outage"`
}

type ProbeOutage struct {
	ProbeID  string        `json:"probe_id"`
	Start    time.Time     `json:"start"`
	End      *time.Time    `json:"end,omitempty"`
	Duration time.Duration `json:"duration"`
	Reason   string        `json:"reason,omitempty"`
}

type CommandSuccessReport struct {
	Period  TimeRange          `json:"period"`
	Overall CommandRateSummary `json:"overall"`
	ByType  []CommandTypeRate  `json:"by_type"`
}

type CommandRateSummary struct {
	Total       int     `json:"total"`
	Succeeded   int     `json:"succeeded"`
	Failed      int     `json:"failed"`
	Pending     int     `json:"pending"`
	SuccessRate float64 `json:"success_rate"`
}

type CommandTypeRate struct {
	CommandType string  `json:"command_type"`
	Total       int     `json:"total"`
	Succeeded   int     `json:"succeeded"`
	Failed      int     `json:"failed"`
	SuccessRate float64 `json:"success_rate"`
}

// SiteSurveyReport defines the structure of a site survey report.
type SiteSurveyReport struct {
	Building        string         `json:"building"`
	Floor           string         `json:"floor"`
	GeneratedAt     time.Time      `json:"generated_at"`
	Heatmap         []HeatmapPoint `json:"heatmap"`
	ChannelUsage    []ChannelUsage `json:"channel_usage"`
	APList          []APCoverage   `json:"ap_list"`
	Recommendations []string       `json:"recommendations"`
}

// HeatmapPoint represents a single location's signal strength.
type HeatmapPoint struct {
	Location string  `json:"location"`
	X        float64 `json:"x,omitempty"`
	Y        float64 `json:"y,omitempty"`
	RSSI     float64 `json:"rssi"`
}

// ChannelUsage represents per-channel utilization data.
type ChannelUsage struct {
	Channel     int     `json:"channel"`
	Utilization float64 `json:"utilization"`
	ProbeCount  int     `json:"probe_count"`
}

// APCoverage represents an access point's coverage details.
type APCoverage struct {
	BSSID        string    `json:"bssid"`
	FirstSeen    time.Time `json:"first_seen,omitempty"`
	LastSeen     time.Time `json:"last_seen,omitempty"`
	Channel      int       `json:"channel"`
	AvgRSSI      float64   `json:"avg_rssi"`
	ProbesSeen   int       `json:"probes_seen"`
	TotalSamples int       `json:"total_samples,omitempty"`
}
