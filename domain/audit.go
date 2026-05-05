package domain

import (
	"content-hub/pkg/id"
	"strings"
	"time"
)

type AuditLog struct {
	ID         string         `json:"id"`
	Actor      string         `json:"actor"`
	Action     string         `json:"action"`
	Resource   string         `json:"resource"`
	ResourceID string         `json:"resource_id"`
	Result     string         `json:"result"`
	Message    string         `json:"message"`
	Metadata   map[string]any `json:"metadata"`
	CreatedAt  time.Time      `json:"created_at"`
}

func NewAuditLog(actor, action string) *AuditLog {
	return &AuditLog{
		ID:        id.New(),
		Actor:     strings.TrimSpace(actor),
		Action:    strings.TrimSpace(action),
		Metadata:  map[string]any{},
		CreatedAt: time.Now().UTC(),
	}
}

func (l AuditLog) Validate() error {
	if strings.TrimSpace(l.Actor) == "" {
		return NewValidationErr("actor is required", nil)
	}
	if strings.TrimSpace(l.Action) == "" {
		return NewValidationErr("action is required", nil)
	}
	return nil
}
