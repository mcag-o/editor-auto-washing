package service

import (
	"content-hub/infra/formatter"
	"content-hub/infra/sqlite"
	workspaceinfra "content-hub/infra/workspace"
	"content-hub/pkg/repo"
	"fmt"
	"os"
	"path/filepath"
)

type RuntimeRepos struct {
	ArticleRepo            repo.ArticleRepo
	TemplateRepo           repo.TemplateRepo
	DraftRepo              repo.DraftRepo
	AssetRepo              repo.AssetRepo
	ReviewRepo             repo.ReviewRepo
	Formatter              DraftFormatter
	RenderedDir            string
	PublishRepo            repo.PublishRepo
	JobRepo                repo.JobRepo
	JobEventRepo           repo.JobEventRepo
	IngestionRepo          repo.IngestionRepo
	WorkspaceRepo          repo.WorkspaceRepo
	BundleImportTxStarter  repo.BundleImportTxStarter
	CollectorSourceRepo    repo.CollectorSourceRepo
	CollectorRunRepo       repo.CollectorRunRepo
	CollectorEntryRepo     repo.CollectorEntryRepo
	CollectorArticleRepo   repo.CollectorArticleRepo
	CollectorAttemptRepo   repo.CollectorAttemptRepo
	CollectorSchedulerRepo repo.CollectorSchedulerStateRepo
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
	if err := os.MkdirAll(filepath.Dir(runtimeCfg.Database.Path), 0o755); err != nil {
		return nil, nil, fmt.Errorf("create runtime db dir: %w", err)
	}
	sqliteProvider, err := sqlite.NewProvider(runtimeCfg.Database.Path)
	if err != nil {
		return nil, nil, err
	}
	_ = resolved
	wechatFormatter := formatter.NewWechatHtmlFormatter(resolved.Paths.TemplateDirs)
	return &RuntimeRepos{
		ArticleRepo:            sqliteProvider.ArticleRepo(),
		TemplateRepo:           sqliteProvider.TemplateRepo(),
		DraftRepo:              sqliteProvider.DraftRepo(),
		AssetRepo:              sqliteProvider.AssetRepo(),
		ReviewRepo:             sqliteProvider.ReviewRepo(),
		Formatter:              wechatFormatter,
		RenderedDir:            resolved.Paths.RenderedDir,
		PublishRepo:            sqliteProvider.PublishRepo(),
		JobRepo:                sqliteProvider.JobRepo(),
		JobEventRepo:           sqliteProvider.JobEventRepo(),
		IngestionRepo:          sqliteProvider.IngestionRepo(),
		WorkspaceRepo:          sqliteProvider.WorkspaceRepo(),
		BundleImportTxStarter:  sqliteProvider,
		CollectorSourceRepo:    sqliteProvider.CollectorSourceRepo(),
		CollectorRunRepo:       sqliteProvider.CollectorRunRepo(),
		CollectorEntryRepo:     sqliteProvider.CollectorEntryRepo(),
		CollectorArticleRepo:   sqliteProvider.CollectorArticleRepo(),
		CollectorAttemptRepo:   sqliteProvider.CollectorAttemptRepo(),
		CollectorSchedulerRepo: sqliteProvider.CollectorSchedulerStateRepo(),
	}, sqliteProvider.Close, nil
}
