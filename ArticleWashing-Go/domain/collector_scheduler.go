package domain

import (
	"time"
)

const (
	DefaultCollectorSchedulerName = "default"

	CollectorSchedulerIdle    = "idle"
	CollectorSchedulerRunning = "running"
	CollectorSchedulerStopped = "stopped"
	CollectorSchedulerFailed  = "failed"
)

type CollectorSchedulerState struct {
	Name          string     `json:"name"`
	Status        string     `json:"status"`
	LastRunID     string     `json:"last_run_id"`
	LastHeartbeat *time.Time `json:"last_heartbeat"`
	LastRunAt     *time.Time `json:"last_run_at"`
	NextRunAt     *time.Time `json:"next_run_at"`
	ErrorMessage  string     `json:"error_message"`
	MetadataJSON  []byte     `json:"metadata_json"`
	UpdatedAt     time.Time  `json:"updated_at"`
}

type CollectorRunSummary struct {
	RunID             string    `json:"run_id" yaml:"run_id"`
	Trigger           string    `json:"trigger" yaml:"trigger"`
	Status            string    `json:"status" yaml:"status"`
	SourceCount       int       `json:"source_count" yaml:"source_count"`
	SuccessfulSources int       `json:"successful_sources" yaml:"successful_sources"`
	FailedSources     int       `json:"failed_sources" yaml:"failed_sources"`
	EntryCount        int       `json:"entry_count" yaml:"entry_count"`
	StartedAt         time.Time `json:"started_at" yaml:"started_at"`
	CompletedAt       time.Time `json:"completed_at" yaml:"completed_at"`
}

type CollectorRunDetail struct {
	Run        CollectorRun         `json:"run" yaml:"run"`
	SourceRuns []CollectorSourceRun `json:"source_runs" yaml:"source_runs"`
}

type CollectorSourceHealthStatus struct {
	SourceID     string                      `json:"source_id" yaml:"source_id"`
	DisplayName  string                      `json:"display_name" yaml:"display_name"`
	Enabled      bool                        `json:"enabled" yaml:"enabled"`
	Capabilities CollectorSourceCapabilities `json:"capabilities" yaml:"capabilities"`
	Health       CollectorHealthInfo         `json:"health" yaml:"health"`
}

type CollectorSourceCapabilities struct {
	SupportsHotlist bool     `json:"supports_hotlist" yaml:"supports_hotlist"`
	SupportsArticle bool     `json:"supports_article" yaml:"supports_article"`
	AuthModes       []string `json:"auth_modes" yaml:"auth_modes"`
}

type CollectorHealthInfo struct {
	OK        bool      `json:"ok" yaml:"ok"`
	Code      string    `json:"code,omitempty" yaml:"code,omitempty"`
	Message   string    `json:"message" yaml:"message"`
	CheckedAt time.Time `json:"checked_at" yaml:"checked_at"`
}

type CollectorSchedulerStatus struct {
	Name          string     `json:"name" yaml:"name"`
	State         string     `json:"state" yaml:"state"`
	Running       bool       `json:"running" yaml:"running"`
	LastRunID     string     `json:"last_run_id" yaml:"last_run_id"`
	LastRunAt     *time.Time `json:"last_run_at" yaml:"last_run_at"`
	LastHeartbeat *time.Time `json:"last_heartbeat" yaml:"last_heartbeat"`
	NextRunAt     *time.Time `json:"next_run_at" yaml:"next_run_at"`
	UpdatedAt     time.Time  `json:"updated_at" yaml:"updated_at"`
}

type CollectorSchedulerHealthReport struct {
	Status    string            `json:"status" yaml:"status"`
	Checks    map[string]string `json:"checks" yaml:"checks"`
	UpdatedAt time.Time         `json:"updated_at" yaml:"updated_at"`
}

type CollectorSchedulerControlResult struct {
	Started   bool      `json:"started,omitempty" yaml:"started,omitempty"`
	Stopped   bool      `json:"stopped,omitempty" yaml:"stopped,omitempty"`
	State     string    `json:"state" yaml:"state"`
	Reason    string    `json:"reason,omitempty" yaml:"reason,omitempty"`
	UpdatedAt time.Time `json:"updated_at" yaml:"updated_at"`
}

func NewCollectorSchedulerState(name string) *CollectorSchedulerState {
	now := time.Now().UTC()
	if name == "" {
		name = DefaultCollectorSchedulerName
	}
	return &CollectorSchedulerState{
		Name:         name,
		Status:       CollectorSchedulerIdle,
		MetadataJSON: []byte("{}"),
		UpdatedAt:    now,
	}
}
