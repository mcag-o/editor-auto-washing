package domain

import "time"

const (
	ArticleWorkspaceStatusImported       = "imported"
	ArticleWorkspaceStatusRewritePending = "rewrite_pending"
	ArticleWorkspaceStatusRewriting      = "rewriting"
	ArticleWorkspaceStatusRewriteFailed  = "rewrite_failed"
	ArticleWorkspaceStatusDraft          = "draft"
	ArticleWorkspaceStatusRendered       = "rendered"
	ArticleWorkspaceStatusReviewPending  = "review_pending"
	ArticleWorkspaceStatusApproved       = "approved"
	ArticleWorkspaceStatusReviewRejected = "review_rejected"
	ArticleWorkspaceStatusPublished      = "published"
	ArticleWorkspaceStatusFailed         = "failed"
)

type WorkspacePaths struct {
	DataDir           string   `yaml:"data_dir" json:"data_dir"`
	IncomingDir       string   `yaml:"incoming_dir" json:"incoming_dir"`
	ArticlesDir       string   `yaml:"articles_dir" json:"articles_dir"`
	DraftsDir         string   `yaml:"drafts_dir" json:"drafts_dir"`
	TemplateDirs      []string `yaml:"template_dirs" json:"template_dirs"`
	RenderedDir       string   `yaml:"rendered_dir" json:"rendered_dir"`
	ReviewsDir        string   `yaml:"reviews_dir" json:"reviews_dir"`
	PublishRecordsDir string   `yaml:"publish_records_dir" json:"publish_records_dir"`
	LogsDir           string   `yaml:"logs_dir" json:"logs_dir"`
}

type ProviderProfile struct {
	Provider    string  `yaml:"provider" json:"provider"`
	Model       string  `yaml:"model" json:"model"`
	SecretRef   string  `yaml:"secret_ref" json:"secret_ref"`
	BaseURL     string  `yaml:"base_url" json:"base_url"`
	Temperature float64 `yaml:"temperature" json:"temperature"`
	MaxTokens   int     `yaml:"max_tokens" json:"max_tokens"`
	Enabled     bool    `yaml:"-" json:"enabled"`
	EnabledSet  *bool   `yaml:"enabled" json:"-"`
}

type ArticleProfile struct {
	Style            string `yaml:"style" json:"style"`
	OutputFormat     string `yaml:"output_format" json:"output_format"`
	Template         string `yaml:"template" json:"template"`
	Length           string `yaml:"length" json:"length"`
	ImagePolicy      string `yaml:"image_policy" json:"image_policy"`
	AllowAutoPublish bool   `yaml:"allow_auto_publish" json:"allow_auto_publish"`
}

type PublishProfile struct {
	Platform         string `yaml:"platform" json:"platform"`
	Account          string `yaml:"account" json:"account"`
	SecretRef        string `yaml:"secret_ref" json:"secret_ref"`
	AllowAutoPublish bool   `yaml:"allow_auto_publish" json:"allow_auto_publish"`
	RetryCount       int    `yaml:"retry_count" json:"retry_count"`
	FallbackToReview bool   `yaml:"-" json:"fallback_to_review"`
	FallbackSet      *bool  `yaml:"fallback_to_review" json:"-"`
}

type ReviewPolicy struct {
	DefaultMode         string   `yaml:"default_mode" json:"default_mode"`
	AutoPublishProfiles []string `yaml:"auto_publish_profiles" json:"auto_publish_profiles"`
	BlockingErrors      []string `yaml:"blocking_errors" json:"blocking_errors"`
}

type AutomationPolicy struct {
	AutoImport                  bool   `yaml:"auto_import" json:"auto_import"`
	AutoGenerate                bool   `yaml:"auto_generate" json:"auto_generate"`
	AutoPublish                 bool   `yaml:"auto_publish" json:"auto_publish"`
	IntervalSeconds             int    `yaml:"interval_seconds" json:"interval_seconds"`
	IncomingDir                 string `yaml:"incoming_dir" json:"incoming_dir"`
	ProcessedDir                string `yaml:"processed_dir" json:"processed_dir"`
	FailedDir                   string `yaml:"failed_dir" json:"failed_dir"`
	AlertWarningThreshold       *int   `yaml:"alert_warning_threshold" json:"alert_warning_threshold"`
	AlertCriticalThreshold      *int   `yaml:"alert_critical_threshold" json:"alert_critical_threshold"`
	AlertWebhookCooldownSeconds *int   `yaml:"alert_webhook_cooldown_seconds" json:"alert_webhook_cooldown_seconds"`
}

type WorkspaceSettings struct {
	Name                   string                     `yaml:"name" json:"name"`
	Paths                  WorkspacePaths             `yaml:"paths" json:"paths"`
	ProviderProfiles       map[string]ProviderProfile `yaml:"provider_profiles" json:"provider_profiles"`
	ArticleProfiles        map[string]ArticleProfile  `yaml:"article_profiles" json:"article_profiles"`
	PublishProfiles        map[string]PublishProfile  `yaml:"publish_profiles" json:"publish_profiles"`
	ReviewPolicy           ReviewPolicy               `yaml:"review_policy" json:"review_policy"`
	Automation             AutomationPolicy           `yaml:"automation" json:"automation"`
	DefaultProviderProfile string                     `yaml:"default_provider_profile" json:"default_provider_profile"`
	DefaultArticleProfile  string                     `yaml:"default_article_profile" json:"default_article_profile"`
	DefaultPublishProfile  string                     `yaml:"default_publish_profile" json:"default_publish_profile"`
}

type ResolvedWorkspacePaths struct {
	Root                   string   `json:"root"`
	ConfigFile             string   `json:"config_file"`
	SecretsFile            string   `json:"secrets_file"`
	DataDir                string   `json:"data_dir"`
	IncomingDir            string   `json:"incoming_dir"`
	ProcessedDir           string   `json:"processed_dir"`
	FailedDir              string   `json:"failed_dir"`
	ArticlesDir            string   `json:"articles_dir"`
	DraftsDir              string   `json:"drafts_dir"`
	TemplateDirs           []string `json:"template_dirs"`
	RenderedDir            string   `json:"rendered_dir"`
	ReviewsDir             string   `json:"reviews_dir"`
	PublishRecordsDir      string   `json:"publish_records_dir"`
	LogsDir                string   `json:"logs_dir"`
	AutomationIncomingDir  string   `json:"automation_incoming_dir"`
	AutomationProcessedDir string   `json:"automation_processed_dir"`
	AutomationFailedDir    string   `json:"automation_failed_dir"`
}

type ResolvedWorkspaceSettings struct {
	Workspace WorkspaceSettings      `json:"workspace"`
	Paths     ResolvedWorkspacePaths `json:"paths"`
	Secrets   map[string]string      `json:"secrets"`
}

func DefaultWorkspaceSettings() WorkspaceSettings {
	return WorkspaceSettings{
		Name: "content-workspace",
		Paths: WorkspacePaths{
			DataDir:           "workspace_data",
			IncomingDir:       "incoming",
			ArticlesDir:       "articles",
			DraftsDir:         "drafts",
			TemplateDirs:      []string{"templates", "knowledge/structured_templates"},
			RenderedDir:       "rendered",
			ReviewsDir:        "reviews",
			PublishRecordsDir: "publish_records",
			LogsDir:           "logs",
		},
		ProviderProfiles: map[string]ProviderProfile{
			"default": {
				Provider:    "openai-compatible",
				Model:       "",
				SecretRef:   "env.LLM_API_KEY",
				BaseURL:     "",
				Temperature: 0.7,
				MaxTokens:   4096,
				Enabled:     true,
				EnabledSet:  boolPtr(true),
			},
		},
		ArticleProfiles: map[string]ArticleProfile{
			"wechat-daily": {
				Style:            "news-rewrite",
				OutputFormat:     "html",
				Template:         "daily-intelligence",
				Length:           "medium",
				ImagePolicy:      "none",
				AllowAutoPublish: false,
			},
		},
		PublishProfiles: map[string]PublishProfile{
			"wechat-review": {
				Platform:         "wechat",
				Account:          "main",
				SecretRef:        "wechat.main",
				AllowAutoPublish: false,
				RetryCount:       1,
				FallbackToReview: true,
				FallbackSet:      boolPtr(true),
			},
		},
		ReviewPolicy: ReviewPolicy{
			DefaultMode:         "review_required",
			AutoPublishProfiles: []string{},
			BlockingErrors:      []string{"missing_secret", "render_failed", "validation_failed"},
		},
		Automation: AutomationPolicy{
			AutoImport:      false,
			AutoGenerate:    false,
			AutoPublish:     false,
			IntervalSeconds: 1800,
		},
		DefaultProviderProfile: "default",
		DefaultArticleProfile:  "wechat-daily",
		DefaultPublishProfile:  "wechat-review",
	}
}

func boolPtr(value bool) *bool {
	return &value
}

type WorkspaceArticle struct {
	ID            string    `json:"id"`
	Title         string    `json:"title"`
	Status        string    `json:"status"`
	StatusHistory []string  `json:"status_history"`
	Notes         string    `json:"notes"`
	CreatedAt     time.Time `json:"created_at"`
	UpdatedAt     time.Time `json:"updated_at"`
}

type ArticleWorkspaceSource struct {
	IngestionID string `json:"ingestion_id"`
	BundleFile  string `json:"bundle_file"`
	SourceType  string `json:"source_type"`
	Platform    string `json:"platform"`
	URL         string `json:"url"`
}

type ArticleWorkspaceLifecycleEntry struct {
	Status    string    `json:"status"`
	Notes     string    `json:"notes"`
	CreatedAt time.Time `json:"created_at"`
}

type ArticleWorkspaceRecord struct {
	ID               string                           `json:"id"`
	Title            string                           `json:"title"`
	Summary          string                           `json:"summary"`
	Status           string                           `json:"status"`
	StatusHistory    []string                         `json:"status_history"`
	LifecycleHistory []ArticleWorkspaceLifecycleEntry `json:"lifecycle_history"`
	Source           ArticleWorkspaceSource           `json:"source"`
	Metadata         map[string]any                   `json:"metadata"`
	Notes            string                           `json:"notes"`
	CreatedAt        time.Time                        `json:"created_at"`
	UpdatedAt        time.Time                        `json:"updated_at"`
}

func NewWorkspaceArticle(id, title string) *WorkspaceArticle {
	now := time.Now().UTC()
	return &WorkspaceArticle{
		ID:            id,
		Title:         title,
		Status:        ArticleWorkspaceStatusDraft,
		StatusHistory: []string{ArticleWorkspaceStatusDraft},
		CreatedAt:     now,
		UpdatedAt:     now,
	}
}

func NewArticleWorkspaceRecord(id, title, summary string, source ArticleWorkspaceSource, metadata map[string]any) *ArticleWorkspaceRecord {
	now := time.Now().UTC()
	if metadata == nil {
		metadata = map[string]any{}
	}
	return &ArticleWorkspaceRecord{
		ID:            id,
		Title:         title,
		Summary:       summary,
		Status:        ArticleWorkspaceStatusImported,
		StatusHistory: []string{ArticleWorkspaceStatusImported},
		LifecycleHistory: []ArticleWorkspaceLifecycleEntry{{
			Status:    ArticleWorkspaceStatusImported,
			Notes:     "created from ingestion bundle",
			CreatedAt: now,
		}},
		Source:    source,
		Metadata:  metadata,
		CreatedAt: now,
		UpdatedAt: now,
	}
}

var ValidWorkspaceStatusTransitions = map[string][]string{
	ArticleWorkspaceStatusImported:       {ArticleWorkspaceStatusRewritePending, ArticleWorkspaceStatusDraft, ArticleWorkspaceStatusFailed},
	ArticleWorkspaceStatusRewritePending: {ArticleWorkspaceStatusRewriting, ArticleWorkspaceStatusFailed},
	ArticleWorkspaceStatusRewriting:      {ArticleWorkspaceStatusDraft, ArticleWorkspaceStatusRewriteFailed, ArticleWorkspaceStatusFailed},
	ArticleWorkspaceStatusRewriteFailed:  {ArticleWorkspaceStatusRewritePending, ArticleWorkspaceStatusFailed},
	ArticleWorkspaceStatusDraft:          {ArticleWorkspaceStatusRendered, ArticleWorkspaceStatusFailed},
	ArticleWorkspaceStatusRendered:       {ArticleWorkspaceStatusReviewPending, ArticleWorkspaceStatusFailed},
	ArticleWorkspaceStatusReviewPending:  {ArticleWorkspaceStatusApproved, ArticleWorkspaceStatusReviewRejected, ArticleWorkspaceStatusFailed},
	ArticleWorkspaceStatusReviewRejected: {ArticleWorkspaceStatusDraft, ArticleWorkspaceStatusFailed},
	ArticleWorkspaceStatusApproved:       {ArticleWorkspaceStatusPublished, ArticleWorkspaceStatusFailed},
	ArticleWorkspaceStatusFailed:         {ArticleWorkspaceStatusDraft},
	ArticleWorkspaceStatusPublished:      {},
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

func (w *ArticleWorkspaceRecord) CanTransitionTo(newStatus string) bool {
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

func ValidateWorkspaceTransition(currentStatus, newStatus string) error {
	allowed, exists := ValidWorkspaceStatusTransitions[currentStatus]
	if !exists {
		return NewConflictErr("unknown status: " + currentStatus)
	}
	for _, status := range allowed {
		if status == newStatus {
			return nil
		}
	}
	return NewConflictErr("invalid transition")
}
