package service

import (
	"content-hub/infra/config"
	"content-hub/infra/formatter"
	llminfra "content-hub/infra/llm"
	"content-hub/infra/sqlite"
	workspaceinfra "content-hub/infra/workspace"
	"content-hub/pkg/repo"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

type RuntimeRepos struct {
	ArticleRepo                repo.ArticleRepo
	TemplateRepo               repo.TemplateRepo
	DraftRepo                  repo.DraftRepo
	AssetRepo                  repo.AssetRepo
	ReviewRepo                 repo.ReviewRepo
	Formatter                  DraftFormatter
	RenderedDir                string
	PublishRepo                repo.PublishRepo
	JobRepo                    repo.JobRepo
	JobEventRepo               repo.JobEventRepo
	IngestionRepo              repo.IngestionRepo
	WorkspaceRepo              repo.WorkspaceRepo
	BundleImportTxStarter      repo.BundleImportTxStarter
	CollectorSourceRepo        repo.CollectorSourceRepo
	CollectorRunRepo           repo.CollectorRunRepo
	CollectorEntryRepo         repo.CollectorEntryRepo
	CollectorArticleRepo       repo.CollectorArticleRepo
	CollectorAttemptRepo       repo.CollectorAttemptRepo
	CollectorSchedulerRepo     repo.CollectorSchedulerStateRepo
	RewritePipelineProfileRepo repo.RewritePipelineProfileRepo
	RewritePipelineRunRepo     repo.RewritePipelineRunRepo
	RewriteStageRunRepo        repo.RewriteStageRunRepo
	RSSSubscriptionRepo        repo.RSSSubscriptionRepo
	RSSPullRunRepo             repo.RSSPullRunRepo
	RSSItemRepo                repo.RSSItemRepo
	SourceDocumentRepo         repo.SourceDocumentRepo
	ImportRunRepo              repo.ImportRunRepo
	PromptTemplateRepo         repo.PromptTemplateRepo
	LLMProfileRepo             repo.LLMProfileRepo
	LLMClient                  llminfra.Client
}

func BuildRuntimeRepos(root string) (*RuntimeRepos, func() error, error) {
	workspaceSvc := NewWorkspaceConfigService(workspaceinfra.NewLoader(), workspaceinfra.NewValidator())
	resolved, err := workspaceSvc.Init(root)
	if err != nil {
		return nil, nil, err
	}
	runtimeCfg, err := workspaceSvc.RuntimeConfig(root)
	if err != nil {
		return nil, nil, err
	}
	return buildRuntimeReposFromResolved(runtimeCfg, resolved.Paths.RenderedDir, resolved.Paths.TemplateDirs)
}

func BuildStandaloneRuntimeRepos(cfg config.Config) (*RuntimeRepos, func() error, error) {
	if err := cfg.Validate(); err != nil {
		return nil, nil, err
	}
	renderedDir := cfg.Storage.BasePath
	if renderedDir != "" {
		renderedDir = filepath.Join(renderedDir, "rendered")
	}
	var templateDirs []string
	if cfg.Template.PromptDir != "" {
		templateDirs = []string{cfg.Template.PromptDir}
	}
	return buildRuntimeReposFromResolved(cfg, renderedDir, templateDirs)
}

func buildRuntimeReposFromResolved(runtimeCfg config.Config, renderedDir string, templateDirs []string) (*RuntimeRepos, func() error, error) {
	if err := os.MkdirAll(filepath.Dir(runtimeCfg.Database.Path), 0o755); err != nil {
		return nil, nil, fmt.Errorf("create runtime db dir: %w", err)
	}
	sqliteProvider, err := sqlite.NewProvider(runtimeCfg.Database.Path)
	if err != nil {
		return nil, nil, err
	}
	wechatFormatter := formatter.NewWechatHtmlFormatter(templateDirs)
	return &RuntimeRepos{
		ArticleRepo:                sqliteProvider.ArticleRepo(),
		TemplateRepo:               sqliteProvider.TemplateRepo(),
		DraftRepo:                  sqliteProvider.DraftRepo(),
		AssetRepo:                  sqliteProvider.AssetRepo(),
		ReviewRepo:                 sqliteProvider.ReviewRepo(),
		Formatter:                  wechatFormatter,
		RenderedDir:                renderedDir,
		PublishRepo:                sqliteProvider.PublishRepo(),
		JobRepo:                    sqliteProvider.JobRepo(),
		JobEventRepo:               sqliteProvider.JobEventRepo(),
		IngestionRepo:              sqliteProvider.IngestionRepo(),
		WorkspaceRepo:              sqliteProvider.WorkspaceRepo(),
		BundleImportTxStarter:      sqliteProvider,
		CollectorSourceRepo:        sqliteProvider.CollectorSourceRepo(),
		CollectorRunRepo:           sqliteProvider.CollectorRunRepo(),
		CollectorEntryRepo:         sqliteProvider.CollectorEntryRepo(),
		CollectorArticleRepo:       sqliteProvider.CollectorArticleRepo(),
		CollectorAttemptRepo:       sqliteProvider.CollectorAttemptRepo(),
		CollectorSchedulerRepo:     sqliteProvider.CollectorSchedulerStateRepo(),
		RewritePipelineProfileRepo: sqliteProvider.RewritePipelineProfileRepo(),
		RewritePipelineRunRepo:     sqliteProvider.RewritePipelineRunRepo(),
		RewriteStageRunRepo:        sqliteProvider.RewriteStageRunRepo(),
		RSSSubscriptionRepo:        sqliteProvider.RSSSubscriptionRepo(),
		RSSPullRunRepo:             sqliteProvider.RSSPullRunRepo(),
		RSSItemRepo:                sqliteProvider.RSSItemRepo(),
		SourceDocumentRepo:         sqliteProvider.SourceDocumentRepo(),
		ImportRunRepo:              sqliteProvider.ImportRunRepo(),
		PromptTemplateRepo:         sqliteProvider.PromptTemplateRepo(),
		LLMProfileRepo:             sqliteProvider.LLMProfileRepo(),
		LLMClient:                  llminfra.NewProvider(runtimeCfg.LLM.BaseURL, runtimeCfg.LLM.APIKey, runtimeCfg.LLM.Model, time.Duration(runtimeCfg.LLM.TimeoutSec)*time.Second),
	}, sqliteProvider.Close, nil
}
