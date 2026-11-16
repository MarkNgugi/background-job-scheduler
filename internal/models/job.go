package models

import (
	"encoding/json"
	"time"
)

type JobStatus string

const (
	StatusPending   JobStatus = "pending"
	StatusRunning   JobStatus = "running"
	StatusCompleted JobStatus = "completed"
	StatusFailed    JobStatus = "failed"
	StatusCancelled JobStatus = "cancelled"
)

type Job struct {
	ID                       int64           `json:"id"`
	Name                     string          `json:"name"`
	Type                     string          `json:"type"`
	Payload                  json.RawMessage `json:"payload"`
	Status                   JobStatus       `json:"status"`
	ScheduledAt              time.Time       `json:"scheduled_at"`
	CreatedAt                time.Time       `json:"created_at"`
	UpdatedAt                time.Time       `json:"updated_at"`
	StartedAt                *time.Time      `json:"started_at,omitempty"`
	CompletedAt              *time.Time      `json:"completed_at,omitempty"`
	Attempts                 int             `json:"attempts"`
	MaxRetries               int             `json:"max_retries"`
	LastError                string          `json:"last_error,omitempty"`
	RecurringIntervalSeconds int64           `json:"recurring_interval_seconds"`
}
