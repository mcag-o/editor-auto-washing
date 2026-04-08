package domain

import "time"

type WorkspaceArticle struct {
	ID            string    `json:"id"`
	Title         string    `json:"title"`
	Status        string    `json:"status"`
	StatusHistory []string  `json:"status_history"`
	Notes         string    `json:"notes"`
	CreatedAt     time.Time `json:"created_at"`
	UpdatedAt     time.Time `json:"updated_at"`
}

func NewWorkspaceArticle(id, title string) *WorkspaceArticle {
	now := time.Now().UTC()
	return &WorkspaceArticle{
		ID:            id,
		Title:         title,
		Status:        "draft",
		StatusHistory: []string{"draft"},
		CreatedAt:     now,
		UpdatedAt:     now,
	}
}

var ValidWorkspaceStatusTransitions = map[string][]string{
	"draft":           {"rendered", "failed"},
	"rendered":        {"review_pending", "failed"},
	"review_pending":  {"approved", "review_rejected", "failed"},
	"review_rejected": {"draft", "failed"},
	"approved":        {"published", "failed"},
	"failed":          {"draft"},
	"published":       {},
}

func (w *WorkspaceArticle) CanTransitionTo(newStatus string) bool {
	allowed, exists := ValidWorkspaceStatusTransitions[w.Status]
	if !exists {
		return false
	}
	for _, s := range allowed {
		if s == newStatus {
			return true
		}
	}
	return false
}
