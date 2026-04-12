package domain

import (
	"content-hub/pkg/id"
	"fmt"
	"strings"
	"time"
)

type DraftValidationResult struct {
	Errors   []string `json:"errors"`
	Warnings []string `json:"warnings"`
}

const (
	AssetStatusReady = "ready"
)

type RenderedAssetRecord struct {
	AssetID      string         `json:"asset_id"`
	ArticleID    string         `json:"article_id"`
	Platform     string         `json:"platform"`
	Status       string         `json:"status"`
	OutputFormat string         `json:"output_format"`
	Template     string         `json:"template"`
	Content      string         `json:"content"`
	ArtifactPath string         `json:"artifact_path,omitempty"`
	Metadata     map[string]any `json:"metadata,omitempty"`
	CreatedAt    time.Time      `json:"created_at"`
}

type ArticleDraft struct {
	ID              string         `json:"id"`
	Template        string         `json:"template"`
	Meta            map[string]any `json:"meta"`
	Headline        map[string]any `json:"headline"`
	Sections        []any          `json:"sections"`
	Conclusion      string         `json:"conclusion"`
	CTA             string         `json:"cta"`
	SourceRefs      []any          `json:"source_refs"`
	TargetPlatforms []string       `json:"target_platforms"`
	ProviderProfile string         `json:"provider_profile"`
	ArticleProfile  string         `json:"article_profile"`
	PublishProfile  string         `json:"publish_profile"`
	Status          string         `json:"status"`
	CreatedAt       time.Time      `json:"created_at"`
	UpdatedAt       time.Time      `json:"updated_at"`
}

func NewArticleDraft(template string) *ArticleDraft {
	now := time.Now().UTC()
	return &ArticleDraft{
		ID:              id.New(),
		Template:        template,
		Meta:            make(map[string]any),
		Headline:        make(map[string]any),
		Sections:        []any{},
		SourceRefs:      []any{},
		TargetPlatforms: []string{},
		Status:          "draft",
		CreatedAt:       now,
		UpdatedAt:       now,
	}
}

func NewRenderedAssetRecord(articleID, platform, outputFormat, template, content, artifactPath string) *RenderedAssetRecord {
	now := time.Now().UTC()
	return &RenderedAssetRecord{
		AssetID:      id.New(),
		ArticleID:    articleID,
		Platform:     platform,
		Status:       AssetStatusReady,
		OutputFormat: outputFormat,
		Template:     template,
		Content:      content,
		ArtifactPath: artifactPath,
		Metadata:     map[string]any{},
		CreatedAt:    now,
	}
}

func DraftString(value any) string {
	if value == nil {
		return ""
	}
	return strings.TrimSpace(fmt.Sprint(value))
}

func DraftParagraphs(value any) []string {
	if value == nil {
		return nil
	}
	switch typed := value.(type) {
	case []string:
		paragraphs := make([]string, 0, len(typed))
		for _, item := range typed {
			trimmed := strings.TrimSpace(item)
			if trimmed != "" {
				paragraphs = append(paragraphs, trimmed)
			}
		}
		return paragraphs
	case []any:
		paragraphs := make([]string, 0, len(typed))
		for _, item := range typed {
			trimmed := DraftString(item)
			if trimmed != "" {
				paragraphs = append(paragraphs, trimmed)
			}
		}
		return paragraphs
	default:
		trimmed := DraftString(value)
		if trimmed == "" {
			return nil
		}
		return []string{trimmed}
	}
}

func AppendUniqueIssue(issues []string, issue string) []string {
	if strings.TrimSpace(issue) == "" {
		return issues
	}
	for _, existing := range issues {
		if existing == issue {
			return issues
		}
	}
	return append(issues, issue)
}

func (d *ArticleDraft) DisplayTitle() string {
	if d == nil {
		return ""
	}
	if title := strings.TrimSpace(DraftString(d.Headline["title"])); title != "" {
		return title
	}
	if title := strings.TrimSpace(DraftString(d.Meta["title"])); title != "" {
		return title
	}
	return ""
}
