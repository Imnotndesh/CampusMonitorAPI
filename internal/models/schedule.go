package models

import (
	"encoding/json"
	"time"
)

type ScheduledTask struct {
	ID          string                 `json:"id" db:"id"`
	ProbeID     string                 `json:"probe_id" db:"probe_id"`
	CommandType string                 `json:"command_type" db:"command_type"`
	Payload     map[string]interface{} `json:"payload,omitempty" db:"payload"`
	Schedule    ScheduleSpec           `json:"schedule" db:"schedule"`
	CreatedAt   time.Time              `json:"created_at" db:"created_at"`
	UpdatedAt   time.Time              `json:"updated_at" db:"updated_at"`
	LastRun     *time.Time             `json:"last_run,omitempty" db:"last_run"`
	NextRun     *time.Time             `json:"next_run,omitempty" db:"next_run"`
	Enabled     bool                   `json:"enabled" db:"enabled"`
}

type ScheduleSpec struct {
	Type      string     `json:"type"`                 // "one-time" or "recurring"
	ExecuteAt *time.Time `json:"execute_at,omitempty"` // for one-time or first occurrence
	Cron      string     `json:"cron,omitempty"`       // e.g., "@daily", "@hourly", "@weekly"
	Timezone  string     `json:"timezone,omitempty"`   // optional, default UTC
}

// Scan implements sql.Scanner for ScheduleSpec (since it's stored as JSONB)
func (s *ScheduleSpec) Scan(value interface{}) error {
	bytes, ok := value.([]byte)
	if !ok {
		return nil
	}
	return json.Unmarshal(bytes, s)
}
