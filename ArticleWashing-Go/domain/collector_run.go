package domain

import (
	"content-hub/pkg/id"
	"time"
)

const (
	CollectorRunPending   = "pending"
	CollectorRunRunning   = "running"
	CollectorRunSucceeded = "succeeded"
	CollectorRunFailed    = "failed"

	CollectorSourceRunPending   = "pending"
	CollectorSourceRunRunning   = "running"
	CollectorSourceRunSucceeded = "succeeded"
	CollectorSourceRunFailed    = "failed"

	CollectorStageHotlist = "hotlist"
	CollectorStageDetail  = "detail"

	CollectorErrorNetworkTimeout  = "network_timeout"
	CollectorErrorAuthMissing     = "auth_missing"
	CollectorErrorAuthExpired     = "auth_expired"
	CollectorErrorUpstreamBlocked = "upstream_blocked"
	CollectorErrorParseFailed     = "parse_failed"
	CollectorErrorSchemaChanged   = "schema_changed"
	CollectorErrorRateLimited     = "rate_limited"
	CollectorErrorStorageFailed   = "storage_failed"
	CollectorErrorBridgeFailed    = "bridge_failed"

	CollectorAttemptPending   = "pending"
	CollectorAttemptRunning   = "running"
	CollectorAttemptSucceeded = "succeeded"
	CollectorAttemptFailed    = "failed"
)

type CollectorRun struct {
	ID           string     `json:"id"`
	Trigger      string     `json:"trigger"`
	Status       string     `json:"status"`
	StartedAt    *time.Time `json:"started_at"`
	CompletedAt  *time.Time `json:"completed_at"`
	ErrorCode    string     `json:"error_code"`
	ErrorMessage string     `json:"error_message"`
	MetadataJSON []byte     `json:"metadata_json"`
	CreatedAt    time.Time  `json:"created_at"`
	UpdatedAt    time.Time  `json:"updated_at"`
}

type CollectorSourceRun struct {
	ID              string     `json:"id"`
	RunID           string     `json:"run_id"`
	SourceID        string     `json:"source_id"`
	Stage           string     `json:"stage"`
	Status          string     `json:"status"`
	StartedAt       *time.Time `json:"started_at"`
	CompletedAt     *time.Time `json:"completed_at"`
	ErrorCode       string     `json:"error_code"`
	ErrorMessage    string     `json:"error_message"`
	DiscoveredCount int        `json:"discovered_count"`
	StoredCount     int        `json:"stored_count"`
	MetadataJSON    []byte     `json:"metadata_json"`
	CreatedAt       time.Time  `json:"created_at"`
	UpdatedAt       time.Time  `json:"updated_at"`
}

type CollectorAttempt struct {
	ID                 string     `json:"id"`
	RunID              string     `json:"run_id"`
	SourceRunID        string     `json:"source_run_id"`
	EntryID            string     `json:"entry_id"`
	ArticleID          string     `json:"article_id"`
	Stage              string     `json:"stage"`
	AttemptNumber      int        `json:"attempt_number"`
	Status             string     `json:"status"`
	RequestURL         string     `json:"request_url"`
	RequestMethod      string     `json:"request_method"`
	ResponseStatusCode int        `json:"response_status_code"`
	ErrorCode          string     `json:"error_code"`
	ErrorMessage       string     `json:"error_message"`
	RawJSON            []byte     `json:"raw_json"`
	StartedAt          *time.Time `json:"started_at"`
	CompletedAt        *time.Time `json:"completed_at"`
	CreatedAt          time.Time  `json:"created_at"`
}

func NewCollectorRun(trigger string) *CollectorRun {
	now := time.Now().UTC()
	return &CollectorRun{
		ID:           id.New(),
		Trigger:      trigger,
		Status:       CollectorRunPending,
		MetadataJSON: []byte("{}"),
		CreatedAt:    now,
		UpdatedAt:    now,
	}
}

func NewCollectorSourceRun(runID, sourceID, stage string) *CollectorSourceRun {
	now := time.Now().UTC()
	return &CollectorSourceRun{
		ID:           id.New(),
		RunID:        runID,
		SourceID:     sourceID,
		Stage:        stage,
		Status:       CollectorSourceRunPending,
		MetadataJSON: []byte("{}"),
		CreatedAt:    now,
		UpdatedAt:    now,
	}
}

func NewCollectorAttempt(runID, sourceRunID, entryID, stage string) *CollectorAttempt {
	now := time.Now().UTC()
	return &CollectorAttempt{
		ID:            id.New(),
		RunID:         runID,
		SourceRunID:   sourceRunID,
		EntryID:       entryID,
		Stage:         stage,
		AttemptNumber: 1,
		Status:        CollectorAttemptPending,
		RequestMethod: "GET",
		RawJSON:       []byte("{}"),
		CreatedAt:     now,
	}
}
