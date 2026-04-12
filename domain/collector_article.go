package domain

import (
	"content-hub/pkg/id"
	"time"
)

const (
	CollectorArticleBridgePending   = "bridge_pending"
	CollectorArticleBridgeSucceeded = "bridge_succeeded"
	CollectorArticleBridgeFailed    = "bridge_failed"
)

type CollectorArticle struct {
	ID             string     `json:"id"`
	EntryID        string     `json:"entry_id"`
	RunID          string     `json:"run_id"`
	SourceID       string     `json:"source_id"`
	ExternalID     string     `json:"external_id"`
	CanonicalURL   string     `json:"canonical_url"`
	Title          string     `json:"title"`
	Body           string     `json:"body"`
	Summary        string     `json:"summary"`
	Author         string     `json:"author"`
	BridgeStatus   string     `json:"bridge_status"`
	WorkspaceID    string     `json:"workspace_id"`
	PublishedAt    *time.Time `json:"published_at"`
	RawJSON        []byte     `json:"raw_json"`
	NormalizedJSON []byte     `json:"normalized_json"`
	MetadataJSON   []byte     `json:"metadata_json"`
	CreatedAt      time.Time  `json:"created_at"`
	UpdatedAt      time.Time  `json:"updated_at"`
}

func NewCollectorArticle(entryID, runID, sourceID, externalID, title, canonicalURL string) *CollectorArticle {
	now := time.Now().UTC()
	return &CollectorArticle{
		ID:             id.New(),
		EntryID:        entryID,
		RunID:          runID,
		SourceID:       sourceID,
		ExternalID:     externalID,
		CanonicalURL:   canonicalURL,
		Title:          title,
		BridgeStatus:   CollectorArticleBridgePending,
		RawJSON:        []byte("{}"),
		NormalizedJSON: []byte("{}"),
		MetadataJSON:   []byte("{}"),
		CreatedAt:      now,
		UpdatedAt:      now,
	}
}
