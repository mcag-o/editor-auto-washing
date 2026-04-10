package domain

import "time"

type AutomationRunResult struct {
	Mode         string         `json:"mode" yaml:"mode"`
	JobID        string         `json:"job_id,omitempty" yaml:"job_id,omitempty"`
	State        string         `json:"state" yaml:"state"`
	Stopped      bool           `json:"stopped" yaml:"stopped"`
	RunsExecuted int            `json:"runs_executed" yaml:"runs_executed"`
	Summary      map[string]any `json:"summary,omitempty" yaml:"summary,omitempty"`
	UpdatedAt    time.Time      `json:"updated_at" yaml:"updated_at"`
}

type AutomationStatusSnapshot struct {
	State            string         `json:"state" yaml:"state"`
	QueueDepth       int            `json:"queue_depth" yaml:"queue_depth"`
	LastCommand      string         `json:"last_command,omitempty" yaml:"last_command,omitempty"`
	LastJobID        string         `json:"last_job_id,omitempty" yaml:"last_job_id,omitempty"`
	LastRunSucceeded bool           `json:"last_run_succeeded" yaml:"last_run_succeeded"`
	Summary          map[string]any `json:"summary,omitempty" yaml:"summary,omitempty"`
	UpdatedAt        time.Time      `json:"updated_at" yaml:"updated_at"`
}

type AutomationHealthReport struct {
	Status    string            `json:"status" yaml:"status"`
	Checks    map[string]string `json:"checks" yaml:"checks"`
	UpdatedAt time.Time         `json:"updated_at" yaml:"updated_at"`
}

type AutomationStopResult struct {
	Stopped   bool      `json:"stopped" yaml:"stopped"`
	Reason    string    `json:"reason" yaml:"reason"`
	UpdatedAt time.Time `json:"updated_at" yaml:"updated_at"`
}
