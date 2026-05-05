package domain

import (
	"content-hub/pkg/id"
	"strings"
	"time"
)

const (
	SystemStateStopped = "stopped"
	SystemStatePaused  = "paused"
	SystemStateRunning = "running"
)

var validSystemStates = map[string]struct{}{
	SystemStateStopped: {},
	SystemStatePaused:  {},
	SystemStateRunning: {},
}

type SystemControlState struct {
	ID          string         `json:"id"`
	State       string         `json:"state"`
	Reason      string         `json:"reason"`
	Metadata    map[string]any `json:"metadata"`
	UpdatedBy   string         `json:"updated_by"`
	RequestedAt *time.Time     `json:"requested_at"`
	UpdatedAt   time.Time      `json:"updated_at"`
}

func NewSystemControlState(updatedBy string) *SystemControlState {
	now := time.Now().UTC()
	return &SystemControlState{
		ID:        id.New(),
		State:     SystemStateStopped,
		Metadata:  map[string]any{},
		UpdatedBy: strings.TrimSpace(updatedBy),
		UpdatedAt: now,
	}
}

func (s SystemControlState) Validate() error {
	state := strings.TrimSpace(s.State)
	if state == "" {
		return NewValidationErr("state is required", nil)
	}
	if _, ok := validSystemStates[state]; !ok {
		return NewValidationErr("unsupported system control state", nil)
	}
	if strings.TrimSpace(s.UpdatedBy) == "" {
		return NewValidationErr("updated by is required", nil)
	}
	return nil
}
