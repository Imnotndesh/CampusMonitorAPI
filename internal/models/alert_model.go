package models

import "time"

// Alert Constants
const (
	SeverityInfo     = "INFO"
	SeverityWarning  = "WARNING"
	SeverityCritical = "CRITICAL"

	StatusActive       = "ACTIVE"
	StatusAcknowledged = "ACKNOWLEDGED"
	StatusResolved     = "RESOLVED"

	CategorySignal  = "SIGNAL"
	CategoryNetwork = "NETWORK"
	CategorySystem  = "SYSTEM"
)

// Alert represents the persistent history of a network event
type Alert struct {
	ID             uint      `gorm:"primaryKey" json:"id"`
	ProbeID        string    `gorm:"index" json:"probe_id"`
	Category       string    `json:"category"`
	Severity       string    `json:"severity"`
	MetricKey      string    `json:"metric_key"` // e.g., "rssi", "latency"
	ThresholdValue float64   `json:"threshold_value"`
	ActualValue    float64   `json:"actual_value"`
	Message        string    `json:"message"`
	Status         string    `json:"status"`
	Occurrences    int       `json:"occurrences"`
	CreatedAt      time.Time `json:"created_at"`
	UpdatedAt      time.Time `json:"updated_at"`
}

// AlertConfig defines the program-defined defaults
type AlertConfig struct {
	RSSIThreshold    float64 `json:"rssi_threshold"`
	RSSIOccurrences  int     `json:"rssi_occurrences"`
	LatencyThreshold float64 `json:"latency_threshold"`
	LatencyWindow    int     `json:"latency_window"`
	HeartbeatTimeout int     `json:"heartbeat_timeout"`
}
