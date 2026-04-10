package domain

import "time"

const (
	IngestionStatusPending       = "pending"
	IngestionStatusImported      = "imported"
	IngestionStatusFailed        = "failed"
	IngestionStatusRoutingFailed = "routing_failed"
)

type IngestionRecord struct {
	ID               string    `json:"id"`
	SourceType       string    `json:"source_type"`
	BundleFile       string    `json:"bundle_file"`
	OriginalLocation string    `json:"original_location"`
	RoutedPath       string    `json:"routed_path"`
	Payload          []byte    `json:"payload"`
	Status           string    `json:"status"`
	ErrorMessage     string    `json:"error_message,omitempty"`
	ImportedItems    int       `json:"imported_items"`
	CreatedArticles  int       `json:"created_articles"`
	Retried          bool      `json:"retried"`
	CreatedAt        time.Time `json:"created_at"`
	UpdatedAt        time.Time `json:"updated_at"`
}

type IngestionSource struct {
	SourceType        string   `json:"sourceType"`
	Platform          string   `json:"platform"`
	CanonicalPlatform string   `json:"canonicalPlatform"`
	DisplayName       string   `json:"displayName"`
	SourceURL         string   `json:"sourceUrl"`
	FetchedAt         string   `json:"fetchedAt"`
	Aliases           []string `json:"aliases"`
	Success           bool     `json:"success"`
	ItemCount         int      `json:"itemCount"`
	Warnings          []string `json:"warnings"`
	Error             any      `json:"error"`
}

type IngestionItem struct {
	SourceType        string         `json:"sourceType"`
	Platform          string         `json:"platform"`
	CanonicalPlatform string         `json:"canonicalPlatform"`
	Title             string         `json:"title"`
	URL               string         `json:"url"`
	Summary           string         `json:"summary"`
	Author            string         `json:"author"`
	PublishTime       string         `json:"publishTime"`
	Tags              []string       `json:"tags"`
	Category          string         `json:"category"`
	Rank              *int           `json:"rank"`
	Hot               any            `json:"hot"`
	Metadata          map[string]any `json:"metadata"`
	Raw               any            `json:"raw"`
}

type IngestionFailure struct {
	SourceType        string   `json:"sourceType"`
	Platform          string   `json:"platform"`
	CanonicalPlatform string   `json:"canonicalPlatform"`
	DisplayName       string   `json:"displayName"`
	SourceURL         string   `json:"sourceUrl"`
	FetchedAt         string   `json:"fetchedAt"`
	Warnings          []string `json:"warnings"`
	Error             any      `json:"error"`
}

type IngestionBundle struct {
	BundleVersion string             `json:"bundleVersion"`
	GeneratedAt   string             `json:"generatedAt"`
	Sources       []IngestionSource  `json:"sources"`
	Items         []IngestionItem    `json:"items"`
	Failures      []IngestionFailure `json:"failures"`
}

type IngestionFileResult struct {
	RecordID         string `json:"record_id"`
	FileName         string `json:"file_name"`
	Status           string `json:"status"`
	ImportedItems    int    `json:"imported_items"`
	CreatedArticles  int    `json:"created_articles"`
	ErrorMessage     string `json:"error_message,omitempty"`
	OriginalLocation string `json:"original_location"`
	RoutedPath       string `json:"routed_path"`
}

type IngestionRunResult struct {
	ScannedFiles         int                   `json:"scanned_files"`
	ImportedFiles        int                   `json:"imported_files"`
	RoutingFailedFiles   int                   `json:"routing_failed_files"`
	FailedFiles          int                   `json:"failed_files"`
	TotalImportedItems   int                   `json:"total_imported_items"`
	TotalCreatedArticles int                   `json:"total_created_articles"`
	FileResults          []IngestionFileResult `json:"file_results"`
}

type IngestionStatusView struct {
	Record   IngestionRecord          `json:"record"`
	Articles []ArticleWorkspaceRecord `json:"articles"`
}

type CollectResult struct {
	Bundle   *IngestionBundle `json:"bundle"`
	Failures []CollectError   `json:"failures"`
	Duration string           `json:"duration"`
}

type CollectError struct {
	Platform string `json:"platform"`
	Code     string `json:"code"`
	Message  string `json:"message"`
}

type PlatformInfo struct {
	ID          string `json:"id"`
	DisplayName string `json:"display_name"`
	Enabled     bool   `json:"enabled"`
	SourceType  string `json:"source_type"`
	SourceURL   string `json:"source_url"`
}
