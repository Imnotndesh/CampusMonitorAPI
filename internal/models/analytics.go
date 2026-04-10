package models

import "time"

type DailyCoverage struct {
	Day     time.Time `json:"day"`
	HasData bool      `json:"has_data"`
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
