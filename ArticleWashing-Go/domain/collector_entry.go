package domain

import (
	"content-hub/pkg/id"
	"time"
)

const (
	CollectorEntryDiscovered    = "discovered"
	CollectorEntryPendingDetail = "pending_detail"
	CollectorEntryFetchedDetail = "fetched_detail"
	CollectorEntryDetailFailed  = "detail_failed"
)

type CollectorEntry struct {
	ID             string     `json:"id"`
	RunID          string     `json:"run_id"`
	SourceID       string     `json:"source_id"`
	ExternalID     string     `json:"external_id"`
	CanonicalURL   string     `json:"canonical_url"`
	Title          string     `json:"title"`
	Summary        string     `json:"summary"`
	Author         string     `json:"author"`
	Status         string     `json:"status"`
	Rank           *int       `json:"rank"`
	PublishedAt    *time.Time `json:"published_at"`
	RawJSON        []byte     `json:"raw_json"`
	NormalizedJSON []byte     `json:"normalized_json"`
	MetadataJSON   []byte     `json:"metadata_json"`
	CreatedAt      time.Time  `json:"created_at"`
	UpdatedAt      time.Time  `json:"updated_at"`
}

func NewCollectorEntry(runID, sourceID, externalID, title, canonicalURL string) *CollectorEntry {
	now := time.Now().UTC()
	return &CollectorEntry{
		ID:             id.New(),
		RunID:          runID,
		SourceID:       sourceID,
		ExternalID:     externalID,
		CanonicalURL:   canonicalURL,
		Title:          title,
		Status:         CollectorEntryPendingDetail,
		RawJSON:        []byte("{}"),
		NormalizedJSON: []byte("{}"),
		MetadataJSON:   []byte("{}"),
		CreatedAt:      now,
		UpdatedAt:      now,
	}
}
