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
	ID             int                    `json:"id" db:"id"`
	ProbeID        string                 `json:"probe_id" db:"probe_id"`
	AlertType      string                 `json:"alert_type" db:"alert_type"`
	Severity       string                 `json:"severity" db:"severity"`
	Message        string                 `json:"message" db:"message"`
	ThresholdValue *float64               `json:"threshold_value" db:"threshold_value"`
	ActualValue    *float64               `json:"actual_value" db:"actual_value"`
	TriggeredAt    time.Time              `json:"triggered_at" db:"triggered_at"`
	ResolvedAt     *time.Time             `json:"resolved_at" db:"resolved_at"`
	Acknowledged   bool                   `json:"acknowledged" db:"acknowledged"`
	Metadata       map[string]interface{} `json:"metadata" db:"metadata"`
}

// AlertConfig defines the program-defined defaults
type AlertConfig struct {
	RSSIThreshold    float64 `json:"rssi_threshold"`
	RSSIOccurrences  int     `json:"rssi_occurrences"`
	LatencyThreshold float64 `json:"latency_threshold"`
	LatencyWindow    int     `json:"latency_window"`
	HeartbeatTimeout int     `json:"heartbeat_timeout"`
}

// TODO: Make this part of a config or something
var DEFAULT_ALERT_CONFIG = AlertConfig{
	RSSIThreshold:    -85.0,
	RSSIOccurrences:  3,
	LatencyThreshold: 500.0,
	LatencyWindow:    3,
	HeartbeatTimeout: 60,
}
