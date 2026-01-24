// internal/models/models.go

package models

import (
	"time"
)

type Probe struct {
	ProbeID         string                 `json:"probe_id" db:"probe_id"`
	Location        string                 `json:"location" db:"location"`
	Building        string                 `json:"building" db:"building"`
	Floor           string                 `json:"floor" db:"floor"`
	Department      string                 `json:"department" db:"department"`
	Status          string                 `json:"status" db:"status"`
	FirmwareVersion string                 `json:"firmware_version" db:"firmware_version"`
	LastSeen        time.Time              `json:"last_seen" db:"last_seen"`
	CreatedAt       time.Time              `json:"created_at" db:"created_at"`
	UpdatedAt       time.Time              `json:"updated_at" db:"updated_at"`
	Metadata        map[string]interface{} `json:"metadata" db:"metadata"`
}

type Telemetry struct {
	Timestamp time.Time `json:"timestamp" db:"timestamp"`
	ProbeID   string    `json:"probe_id" db:"probe_id"`
	Type      string    `json:"type" db:"type"`

	// Network Metrics
	RSSI       *int     `json:"rssi" db:"rssi"`
	Latency    *int     `json:"latency" db:"latency"`
	PacketLoss *float64 `json:"packet_loss" db:"packet_loss"`
	DNSTime    *int     `json:"dns_time" db:"dns_time"`
	Channel    *int     `json:"channel" db:"channel"`
	BSSID      *string  `json:"bssid" db:"bssid"`
	Neighbors  *int     `json:"neighbors" db:"neighbors"`
	Overlap    *int     `json:"overlap" db:"overlap"`
	Congestion *int     `json:"congestion" db:"congestion"`

	// Enhanced Metrics
	SNR         *float64 `json:"snr" db:"snr"`
	LinkQuality *float64 `json:"link_quality" db:"link_quality"`
	Utilization *float64 `json:"utilization" db:"utilization"`
	PhyMode     *string  `json:"phy_mode" db:"phy_mode"`
	Throughput  *int     `json:"throughput" db:"throughput"`
	NoiseFloor  *int     `json:"noise_floor" db:"noise_floor"`
	Uptime      *int     `json:"uptime" db:"uptime"`

	ReceivedAt time.Time              `json:"received_at" db:"received_at"`
	Metadata   map[string]interface{} `json:"metadata" db:"metadata"`
}

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

type Command struct {
	ID          int                    `json:"id" db:"id"`
	ProbeID     string                 `json:"probe_id" db:"probe_id"`
	CommandType string                 `json:"command_type" db:"command_type"`
	Payload     map[string]interface{} `json:"payload" db:"payload"`
	IssuedAt    time.Time              `json:"issued_at" db:"issued_at"`
	ExecutedAt  *time.Time             `json:"executed_at" db:"executed_at"`
	Status      string                 `json:"status" db:"status"`
	Result      map[string]interface{} `json:"result" db:"result"`
}

type LightTelemetryMessage struct {
	ProbeID    string  `json:"pid"`
	Type       string  `json:"type"`
	Timestamp  string  `json:"ts"`
	Epoch      int64   `json:"epoch"`
	RSSI       int     `json:"rssi"`
	Latency    int     `json:"lat"`
	PacketLoss float64 `json:"loss"`
	DNSTime    int     `json:"dns"`
	Channel    int     `json:"ch"`
	Congestion int     `json:"cong"`
	BSSID      string  `json:"bssid"`
	Neighbors  int     `json:"neighbors"`
	Overlap    int     `json:"overlap"`
}

type EnhancedTelemetryMessage struct {
	LightTelemetryMessage
	SNR         float64 `json:"snr"`
	LinkQuality float64 `json:"qual"`
	Utilization float64 `json:"util"`
	PhyMode     string  `json:"phy"`
	Throughput  int     `json:"tput"`
	NoiseFloor  int     `json:"noise"`
	Uptime      int     `json:"up"`
}

type CreateProbeRequest struct {
	ProbeID         string                 `json:"probe_id" binding:"required"`
	Location        string                 `json:"location"`
	Building        string                 `json:"building"`
	Floor           string                 `json:"floor"`
	Department      string                 `json:"department"`
	FirmwareVersion string                 `json:"firmware_version"`
	Metadata        map[string]interface{} `json:"metadata"`
}

type UpdateProbeRequest struct {
	Location   *string                `json:"location"`
	Building   *string                `json:"building"`
	Floor      *string                `json:"floor"`
	Department *string                `json:"department"`
	Status     *string                `json:"status"`
	Metadata   map[string]interface{} `json:"metadata"`
}

type TelemetryQueryRequest struct {
	ProbeIDs  []string   `form:"probe_ids"`
	Type      string     `form:"type"`
	StartTime *time.Time `form:"start_time" time_format:"2006-01-02T15:04:05Z"`
	EndTime   *time.Time `form:"end_time" time_format:"2006-01-02T15:04:05Z"`
	Limit     int        `form:"limit"`
	Offset    int        `form:"offset"`
}

type TelemetryQueryResponse struct {
	Data       []Telemetry `json:"data"`
	TotalCount int         `json:"total_count"`
	Limit      int         `json:"limit"`
	Offset     int         `json:"offset"`
}

type CommandRequest struct {
	ProbeID     string                 `json:"probe_id" binding:"required"`
	CommandType string                 `json:"command_type" binding:"required"`
	Payload     map[string]interface{} `json:"payload"`
}

type StatsResponse struct {
	ProbeID        string  `json:"probe_id"`
	Period         string  `json:"period"`
	AvgRSSI        float64 `json:"avg_rssi"`
	MinRSSI        int     `json:"min_rssi"`
	MaxRSSI        int     `json:"max_rssi"`
	AvgLatency     float64 `json:"avg_latency"`
	AvgPacketLoss  float64 `json:"avg_packet_loss"`
	SampleCount    int     `json:"sample_count"`
	MostCommonAP   string  `json:"most_common_ap"`
	MostCommonChan int     `json:"most_common_channel"`
}

type HealthResponse struct {
	Status    string    `json:"status"`
	Timestamp time.Time `json:"timestamp"`
	Services  struct {
		Database bool `json:"database"`
		MQTT     bool `json:"mqtt"`
	} `json:"services"`
}

type ProbeRepository interface {
	Create(probe *Probe) error
	GetByID(probeID string) (*Probe, error)
	GetAll() ([]Probe, error)
	Update(probeID string, updates *UpdateProbeRequest) error
	Delete(probeID string) error
	UpdateLastSeen(probeID string, timestamp time.Time) error
	GetActive() ([]Probe, error)
}

type TelemetryRepository interface {
	Insert(telemetry *Telemetry) error
	InsertBatch(telemetries []Telemetry) error
	Query(req *TelemetryQueryRequest) ([]Telemetry, int, error)
	GetLatest(probeID string, limit int) ([]Telemetry, error)
	GetStats(probeID string, start, end time.Time) (*StatsResponse, error)
	GetHourlyStats(probeID string, hours int) ([]StatsResponse, error)
}

type AlertRepository interface {
	Create(alert *Alert) error
	GetByProbeID(probeID string, limit int) ([]Alert, error)
	GetUnresolved() ([]Alert, error)
	Resolve(alertID int) error
	Acknowledge(alertID int) error
}

type CommandRepository interface {
	Create(cmd *Command) error
	GetByProbeID(probeID string, limit int) ([]Command, error)
	GetPending() ([]Command, error)
	UpdateStatus(commandID int, status string, result map[string]interface{}) error
}

type TelemetryService interface {
	ProcessMessage(payload []byte) error
	GetTelemetry(req *TelemetryQueryRequest) (*TelemetryQueryResponse, error)
	GetProbeStats(probeID string, hours int) ([]StatsResponse, error)
}

type ProbeService interface {
	RegisterProbe(req *CreateProbeRequest) (*Probe, error)
	GetProbe(probeID string) (*Probe, error)
	ListProbes() ([]Probe, error)
	UpdateProbe(probeID string, req *UpdateProbeRequest) (*Probe, error)
	DeleteProbe(probeID string) error
	GetActiveProbes() ([]Probe, error)
}

type CommandService interface {
	IssueCommand(req *CommandRequest) (*Command, error)
	GetCommandHistory(probeID string) ([]Command, error)
	ProcessCommandResult(commandID int, result map[string]interface{}) error
}

type AlertService interface {
	CheckThresholds(telemetry *Telemetry) error
	GetAlerts(probeID string) ([]Alert, error)
	GetUnresolvedAlerts() ([]Alert, error)
	ResolveAlert(alertID int) error
	AcknowledgeAlert(alertID int) error
}

type MQTTService interface {
	Connect() error
	Disconnect() error
	Subscribe(topic string, handler func(payload []byte)) error
	Publish(topic string, payload []byte) error
	IsConnected() bool
}

type WSMessage struct {
	Type      string      `json:"type"`
	ProbeID   string      `json:"probe_id,omitempty"`
	Timestamp time.Time   `json:"timestamp"`
	Data      interface{} `json:"data"`
}

type WSClient struct {
	ID           string
	ProbeID      string
	SendChannel  chan WSMessage
	CloseChannel chan bool
}
