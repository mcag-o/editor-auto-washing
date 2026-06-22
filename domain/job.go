package domain

import (
	"content-hub/pkg/id"
	"time"
)

type JobRun struct {
	ID           string     `json:"id"`
	Topic        string     `json:"topic"`
	Status       string     `json:"status"`
	ArtifactPath *string    `json:"artifact_path"`
	Result       *string    `json:"result"`
	StartedAt    *time.Time `json:"started_at"`
	CompletedAt  *time.Time `json:"completed_at"`
	CreatedAt    time.Time  `json:"created_at"`
	UpdatedAt    time.Time  `json:"updated_at"`
}

type JobEvent struct {
	ID        string    `json:"id"`
	JobID     string    `json:"job_id"`
	Status    string    `json:"status"`
	Message   string    `json:"message"`
	Detail    string    `json:"detail"`
	CreatedAt time.Time `json:"created_at"`
}

func NewJobRun(topic string) *JobRun {
	now := time.Now().UTC()
	return &JobRun{
		ID:        id.New(),
		Topic:     topic,
		Status:    "pending",
		CreatedAt: now,
		UpdatedAt: now,
	}
}

func NewJobRunWithArtifact(topic, artifactPath string) *JobRun {
	job := NewJobRun(topic)
	job.ArtifactPath = &artifactPath
	return job
}

func NewJobEvent(jobID, status, message string) *JobEvent {
	return &JobEvent{
		ID:        id.New(),
		JobID:     jobID,
		Status:    status,
		Message:   message,
		CreatedAt: time.Now().UTC(),
	}
}
