package models

import (
	"time"
)

// FleetProbe extends the base Probe with fleet management metadata
type FleetProbe struct {
	ProbeID             string                 `json:"probe_id" db:"probe_id"`
	Managed             bool                   `json:"managed" db:"managed"`
	ManagedSince        *time.Time             `json:"managed_since,omitempty" db:"managed_since"`
	ManagedBy           string                 `json:"managed_by,omitempty" db:"managed_by"`
	Groups              []string               `json:"groups,omitempty" db:"groups"`
	Location            string                 `json:"location" db:"location"`
	Tags                map[string]interface{} `json:"tags,omitempty" db:"tags"`
	ConfigVersion       int                    `json:"config_version" db:"config_version"`
	ConfigTemplateID    *int                   `json:"config_template_id,omitempty" db:"config_template_id"`
	MaintenanceWindow   *MaintenanceWindow     `json:"maintenance_window,omitempty" db:"maintenance_window"`
	AutoUpdateEnabled   bool                   `json:"auto_update_enabled" db:"auto_update_enabled"`
	LastCommandID       string                 `json:"last_command_id,omitempty" db:"last_command_id"`
	LastCommandStatus   string                 `json:"last_command_status,omitempty" db:"last_command_status"`
	LastCommandTime     *time.Time             `json:"last_command_time,omitempty" db:"last_command_time"`
	CommandsReceived    int                    `json:"commands_received" db:"commands_received"`
	CommandsCompleted   int                    `json:"commands_completed" db:"commands_completed"`
	CommandsFailed      int                    `json:"commands_failed" db:"commands_failed"`
	ConsecutiveFailures int                    `json:"consecutive_failures" db:"consecutive_failures"`
	CurrentFirmware     string                 `json:"current_firmware" db:"current_firmware"`
	TargetFirmware      string                 `json:"target_firmware,omitempty" db:"target_firmware"`
	LastOTAAttempt      *time.Time             `json:"last_ota_attempt,omitempty" db:"last_ota_attempt"`
	OTAAttempts         int                    `json:"ota_attempts" db:"ota_attempts"`
	CreatedAt           time.Time              `json:"created_at" db:"created_at"`
	UpdatedAt           time.Time              `json:"updated_at" db:"updated_at"`
	Status              string                 `json:"status" db:"status"`
	LastSeen            time.Time              `json:"last_seen" db:"last_seen"`
	MQTTConnected       bool                   `json:"mqtt_connected"`
}

type MaintenanceWindow struct {
	Start    string `json:"start" db:"start"`
	End      string `json:"end" db:"end"`
	Timezone string `json:"timezone" db:"timezone"`
}

// FleetConfigTemplate represents a reusable configuration template
type FleetConfigTemplate struct {
	ID              int                    `json:"id" db:"id"`
	Name            string                 `json:"name" db:"name"`
	Description     string                 `json:"description" db:"description"`
	WiFi            map[string]interface{} `json:"wifi,omitempty" db:"wifi"`
	MQTT            map[string]interface{} `json:"mqtt,omitempty" db:"mqtt"`
	ScanSettings    map[string]interface{} `json:"scan_settings,omitempty" db:"scan_settings"`
	DefaultTags     map[string]interface{} `json:"default_tags,omitempty" db:"default_tags"`
	DefaultGroups   []string               `json:"default_groups,omitempty" db:"default_groups"`
	DefaultLocation string                 `json:"default_location,omitempty" db:"default_location"`
	Config          map[string]interface{} `json:"config" db:"config"`
	Variables       []string               `json:"variables,omitempty" db:"variables"`
	CreatedBy       string                 `json:"created_by" db:"created_by"`
	CreatedAt       time.Time              `json:"created_at" db:"created_at"`
	UpdatedAt       time.Time              `json:"updated_at" db:"updated_at"`
	UsageCount      int                    `json:"usage_count" db:"usage_count"`
}

// FleetGroup represents a group of probes for targeting
type FleetGroup struct {
	ID          string    `json:"id" db:"id"`
	Name        string    `json:"name" db:"name"`
	Description string    `json:"description" db:"description"`
	ProbeCount  int       `json:"probe_count" db:"probe_count"`
	CreatedAt   time.Time `json:"created_at" db:"created_at"`
	UpdatedAt   time.Time `json:"updated_at" db:"updated_at"`
}

// FleetCommandRequest extends CommandRequest for fleet operations
type FleetCommandRequest struct {
	CommandType       string                 `json:"command_type" binding:"required"`
	Payload           map[string]interface{} `json:"payload,omitempty"`
	TargetAll         bool                   `json:"target_all"`
	Groups            []string               `json:"groups,omitempty"`
	ProbeIDs          []string               `json:"probe_ids,omitempty"`
	ExcludeProbes     []string               `json:"exclude_probes,omitempty"`
	Strategy          string                 `json:"strategy"`
	StaggerDelay      int                    `json:"stagger_delay"`
	BatchSize         int                    `json:"batch_size"`
	CanaryCount       int                    `json:"canary_count"`
	Schedule          *ScheduleConfig        `json:"schedule,omitempty"`
	RolloutPercentage int                    `json:"rollout_percentage"` // 0-100
	ContinueOnError   bool                   `json:"continue_on_error"`
	RequireAck        bool                   `json:"require_ack"`
	AckTimeoutSeconds int                    `json:"ack_timeout_seconds"`
	CompletionPercent int                    `json:"completion_threshold"`
}

type ScheduleConfig struct {
	Type      string    `json:"type"`
	ExecuteAt time.Time `json:"execute_at,omitempty"`
	Cron      string    `json:"cron,omitempty"`
	Timezone  string    `json:"timezone,omitempty"`
}

// FleetCommand represents a fleet-wide command with tracking
type FleetCommand struct {
	ID          string                 `json:"id" db:"id"`
	CommandType string                 `json:"command_type" db:"command_type"`
	Payload     map[string]interface{} `json:"payload" db:"payload"`
	IssuedBy    string                 `json:"issued_by" db:"issued_by"`
	IssuedAt    time.Time              `json:"issued_at" db:"issued_at"`

	// Targeting snapshot
	TargetGroups []string `json:"target_groups" db:"target_groups"`
	TargetProbes []string `json:"target_probes" db:"target_probes"`
	TotalTargets int      `json:"total_targets" db:"total_targets"`

	// Status tracking
	Status         string `json:"status" db:"status"`
	AcksReceived   int    `json:"acks_received" db:"acks_received"`
	CompletedCount int    `json:"completed_count" db:"completed_count"`
	FailedCount    int    `json:"failed_count" db:"failed_count"`

	// Completion criteria
	CompletionThreshold int `json:"completion_threshold" db:"completion_threshold"`
	TimeoutSeconds      int `json:"timeout_seconds" db:"timeout_seconds"`

	// Schedule
	ScheduledFor *time.Time `json:"scheduled_for,omitempty" db:"scheduled_for"`

	// Metadata
	Metadata    map[string]interface{} `json:"metadata,omitempty" db:"metadata"`
	CompletedAt *time.Time             `json:"completed_at,omitempty" db:"completed_at"`
}

// FleetCommandProbeStatus tracks per-probe status for a fleet command
type FleetCommandProbeStatus struct {
	CommandID      string                 `json:"command_id" db:"command_id"`
	ProbeID        string                 `json:"probe_id" db:"probe_id"`
	CommandType    string                 `json:"command_type" db:"command_type"`
	Status         string                 `json:"status" db:"status"`
	SentAt         *time.Time             `json:"sent_at,omitempty" db:"sent_at"`
	AcknowledgedAt *time.Time             `json:"acknowledged_at,omitempty" db:"acknowledged_at"`
	CompletedAt    *time.Time             `json:"completed_at,omitempty" db:"completed_at"`
	IssuedAt       time.Time              `json:"issued_at" db:"issued_at"`
	Result         map[string]interface{} `json:"result,omitempty" db:"result"`
	ErrorMessage   string                 `json:"error_message,omitempty" db:"error_message"`
	RetryCount     int                    `json:"retry_count" db:"retry_count"`
}

// FleetEnrollRequest for enrolling a probe into fleet management
type FleetEnrollRequest struct {
	Groups            []string               `json:"groups,omitempty"`
	Location          string                 `json:"location"`
	Tags              map[string]interface{} `json:"tags,omitempty"`
	ConfigTemplateID  *int                   `json:"config_template_id,omitempty"`
	MaintenanceWindow *MaintenanceWindow     `json:"maintenance_window,omitempty"`
	AutoUpdateEnabled bool                   `json:"auto_update_enabled"`
}

// FleetUpdateRequest for updating fleet probe metadata
type FleetUpdateRequest struct {
	Groups            *[]string              `json:"groups,omitempty"`
	Location          *string                `json:"location,omitempty"`
	Tags              map[string]interface{} `json:"tags,omitempty"`
	ConfigTemplateID  *int                   `json:"config_template_id,omitempty"`
	MaintenanceWindow *MaintenanceWindow     `json:"maintenance_window,omitempty"`
	AutoUpdateEnabled *bool                  `json:"auto_update_enabled,omitempty"`
}

type GroupSummary struct {
	ID         string `json:"id"`
	Name       string `json:"name"`
	ProbeCount int    `json:"probe_count"`
	Online     int    `json:"online"`
	AlertCount int    `json:"alert_count"`
}

// FleetRolloutStatus tracks the progress of a fleet rollout
type FleetRolloutStatus struct {
	CommandID   string                     `json:"command_id"`
	CommandType string                     `json:"command_type"`
	IssuedAt    time.Time                  `json:"issued_at"`
	Status      string                     `json:"status"`
	Payload     map[string]interface{}     `json:"payload,omitempty"`
	Progress    RolloutProgress            `json:"progress"`
	Timeline    RolloutTimeline            `json:"timeline"`
	Targets     []FleetCommandTargetStatus `json:"targets,omitempty"` // <-- add this
}
type FleetCommandTargetStatus struct {
	ProbeID         string                 `json:"probe_id" db:"probe_id"`
	Status          string                 `json:"status" db:"status"`
	ResponsePayload map[string]interface{} `json:"response_payload,omitempty" db:"result"`
	Error           string                 `json:"error,omitempty" db:"error_message"`
	UpdatedAt       *time.Time             `json:"updated_at,omitempty" db:"completed_at"` // or whichever timestamp is most recent
}
type RolloutProgress struct {
	Total        int     `json:"total"`
	Sent         int     `json:"sent"`
	Acknowledged int     `json:"acknowledged"`
	Completed    int     `json:"completed"`
	Failed       int     `json:"failed"`
	Pending      int     `json:"pending"`
	Percentage   float64 `json:"percentage"`
}

type GroupRolloutStatus struct {
	GroupID   string          `json:"group_id"`
	GroupName string          `json:"group_name"`
	Progress  RolloutProgress `json:"progress"`
}

type RolloutTimeline struct {
	StartedAt           time.Time  `json:"started_at"`
	FirstAckAt          *time.Time `json:"first_ack_at,omitempty"`
	FirstCompleteAt     *time.Time `json:"first_complete_at,omitempty"`
	EstimatedCompleteAt *time.Time `json:"estimated_complete_at,omitempty"`
	CompletedAt         *time.Time `json:"completed_at,omitempty"`
}

// FleetBroadcastMessage for real-time fleet updates
type FleetBroadcastMessage struct {
	Type      string      `json:"type"`
	CommandID string      `json:"command_id,omitempty"`
	ProbeID   string      `json:"probe_id,omitempty"`
	Timestamp time.Time   `json:"timestamp"`
	Data      interface{} `json:"data"`
}
type FleetStatusResponse struct {
	ManagedProbes  int       `json:"total_managed"`
	Online         int       `json:"online"`
	Offline        int       `json:"offline"`
	InMaintenance  int       `json:"in_maintenance"`
	Groups         int       `json:"groups"`
	Templates      int       `json:"templates"`
	ActiveRollouts int       `json:"active_rollouts"`
	LastCommand    string    `json:"last_command,omitempty"`
	LastUpdated    time.Time `json:"last_updated"`
}
