package sqlite

import (
	"content-hub/infra/sqlite/migrations"
	"content-hub/pkg/repo"
	"database/sql"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"

	_ "github.com/mattn/go-sqlite3"
)

type Provider struct {
	db                          *sql.DB
	articleRepo                 repo.ArticleRepo
	templateRepo                repo.TemplateRepo
	draftRepo                   repo.DraftRepo
	assetRepo                   repo.AssetRepo
	businessConfigRepo          repo.BusinessConfigRepo
	systemControlStateRepo      repo.SystemControlStateRepo
	auditLogRepo                repo.AuditLogRepo
	publishRepo                 repo.PublishRepo
	reviewRepo                  repo.ReviewRepo
	jobRepo                     repo.JobRepo
	jobEventRepo                repo.JobEventRepo
	ingestionRepo               repo.IngestionRepo
	workspaceRepo               repo.WorkspaceRepo
	collectorSourceRepo         repo.CollectorSourceRepo
	collectorRunRepo            repo.CollectorRunRepo
	collectorEntryRepo          repo.CollectorEntryRepo
	collectorArticleRepo        repo.CollectorArticleRepo
	collectorAttemptRepo        repo.CollectorAttemptRepo
	collectorSchedulerStateRepo repo.CollectorSchedulerStateRepo
	rewritePipelineProfileRepo  repo.RewritePipelineProfileRepo
	rewritePipelineRunRepo      repo.RewritePipelineRunRepo
	rewriteStageRunRepo         repo.RewriteStageRunRepo
	workflowRunRepo             repo.WorkflowRunRepo
	workflowCheckpointRepo      repo.WorkflowCheckpointRepo
	workflowDefinitionRepo      repo.WorkflowDefinitionRepo
	templateDefinitionRepo      repo.TemplateDefinitionRepo
	sourceDocumentRepo          repo.SourceDocumentRepo
	importRunRepo               repo.ImportRunRepo
	rssSubscriptionRepo         repo.RSSSubscriptionRepo
	rssPullRunRepo              repo.RSSPullRunRepo
	rssItemRepo                 repo.RSSItemRepo
	promptTemplateRepo          repo.PromptTemplateRepo
	llmProfileRepo              repo.LLMProfileRepo
}

func NewProvider(dbPath string) (*Provider, error) {
	dir := filepath.Dir(dbPath)
	if dir != "." && dir != "" {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return nil, fmt.Errorf("create db dir: %w", err)
		}
	}

	db, err := sql.Open("sqlite3", dbPath+"?_busy_timeout=5000&_journal_mode=WAL&_foreign_keys=ON")
	if err != nil {
		return nil, fmt.Errorf("open sqlite db: %w", err)
	}

	db.SetMaxOpenConns(1)
	db.SetMaxIdleConns(10)

	if err := db.Ping(); err != nil {
		return nil, fmt.Errorf("ping db: %w", err)
	}

	p := &Provider{db: db}

	if err := p.runMigrations(db, migrations.FS); err != nil {
		return nil, fmt.Errorf("run migrations: %w", err)
	}

	p.articleRepo = &articleRepo{db: db}
	p.templateRepo = &templateRepo{db: db}
	p.draftRepo = &draftRepo{db: db}
	p.assetRepo = &assetRepo{db: db}
	p.businessConfigRepo = &businessConfigRepo{db: db}
	p.systemControlStateRepo = &controlStateRepo{db: db}
	p.auditLogRepo = &auditLogRepo{db: db}
	p.reviewRepo = &reviewRepo{db: db}
	p.publishRepo = &publishRepo{db: db}
	p.jobRepo = &jobRepo{db: db}
	p.jobEventRepo = &jobEventRepo{db: db}
	p.ingestionRepo = &ingestionRepo{db: db}
	p.workspaceRepo = &articleWorkspaceRepo{db: db}
	p.collectorSourceRepo = &collectorSourceRepo{db: db}
	p.collectorRunRepo = &collectorRunRepo{db: db}
	p.collectorEntryRepo = &collectorEntryRepo{db: db}
	p.collectorArticleRepo = &collectorArticleRepo{db: db}
	p.collectorAttemptRepo = &collectorAttemptRepo{db: db}
	p.collectorSchedulerStateRepo = &collectorSchedulerStateRepo{db: db}
	p.rewritePipelineProfileRepo = &rewritePipelineProfileRepo{db: db}
	p.rewritePipelineRunRepo = &rewritePipelineRunRepo{db: db}
	p.rewriteStageRunRepo = &rewriteStageRunRepo{db: db}
	p.workflowRunRepo = &workflowRunRepo{db: db}
	p.workflowCheckpointRepo = &workflowCheckpointRepo{db: db}
	p.workflowDefinitionRepo = &workflowDefinitionRepo{db: db}
	p.templateDefinitionRepo = &templateDefinitionRepo{db: db}
	p.sourceDocumentRepo = &sourceDocumentRepo{db: db}
	p.importRunRepo = &importRunRepo{db: db}
	p.rssSubscriptionRepo = &rssSubscriptionRepo{db: db}
	p.rssPullRunRepo = &rssPullRunRepo{db: db}
	p.rssItemRepo = &rssItemRepo{db: db}
	p.promptTemplateRepo = &promptTemplateRepo{db: db}
	p.llmProfileRepo = &llmProfileRepo{db: db}

	return p, nil
}

func (p *Provider) Close() error {
	return p.db.Close()
}

func (p *Provider) DB() *sql.DB {
	return p.db
}

func (p *Provider) ArticleRepo() repo.ArticleRepo {
	return p.articleRepo
}

func (p *Provider) TemplateRepo() repo.TemplateRepo {
	return p.templateRepo
}

func (p *Provider) DraftRepo() repo.DraftRepo {
	return p.draftRepo
}

func (p *Provider) AssetRepo() repo.AssetRepo {
	return p.assetRepo
}

func (p *Provider) BusinessConfigRepo() repo.BusinessConfigRepo {
	return p.businessConfigRepo
}

func (p *Provider) SystemControlStateRepo() repo.SystemControlStateRepo {
	return p.systemControlStateRepo
}

func (p *Provider) AuditLogRepo() repo.AuditLogRepo {
	return p.auditLogRepo
}

func (p *Provider) PublishRepo() repo.PublishRepo {
	return p.publishRepo
}

func (p *Provider) ReviewRepo() repo.ReviewRepo {
	return p.reviewRepo
}

func (p *Provider) JobRepo() repo.JobRepo {
	return p.jobRepo
}

func (p *Provider) JobEventRepo() repo.JobEventRepo {
	return p.jobEventRepo
}

func (p *Provider) IngestionRepo() repo.IngestionRepo {
	return p.ingestionRepo
}

func (p *Provider) WorkspaceRepo() repo.WorkspaceRepo {
	return p.workspaceRepo
}

func (p *Provider) CollectorSourceRepo() repo.CollectorSourceRepo {
	return p.collectorSourceRepo
}

func (p *Provider) CollectorRunRepo() repo.CollectorRunRepo {
	return p.collectorRunRepo
}

func (p *Provider) CollectorEntryRepo() repo.CollectorEntryRepo {
	return p.collectorEntryRepo
}

func (p *Provider) CollectorArticleRepo() repo.CollectorArticleRepo {
	return p.collectorArticleRepo
}

func (p *Provider) CollectorAttemptRepo() repo.CollectorAttemptRepo {
	return p.collectorAttemptRepo
}

func (p *Provider) CollectorSchedulerStateRepo() repo.CollectorSchedulerStateRepo {
	return p.collectorSchedulerStateRepo
}

func (p *Provider) RewritePipelineProfileRepo() repo.RewritePipelineProfileRepo {
	return p.rewritePipelineProfileRepo
}

func (p *Provider) RewritePipelineRunRepo() repo.RewritePipelineRunRepo {
	return p.rewritePipelineRunRepo
}

func (p *Provider) RewriteStageRunRepo() repo.RewriteStageRunRepo {
	return p.rewriteStageRunRepo
}

func (p *Provider) WorkflowRunRepo() repo.WorkflowRunRepo {
	return p.workflowRunRepo
}

func (p *Provider) WorkflowCheckpointRepo() repo.WorkflowCheckpointRepo {
	return p.workflowCheckpointRepo
}

func (p *Provider) WorkflowDefinitionRepo() repo.WorkflowDefinitionRepo {
	return p.workflowDefinitionRepo
}

func (p *Provider) TemplateDefinitionRepo() repo.TemplateDefinitionRepo {
	return p.templateDefinitionRepo
}

func (p *Provider) SourceDocumentRepo() repo.SourceDocumentRepo {
	return p.sourceDocumentRepo
}

func (p *Provider) ImportRunRepo() repo.ImportRunRepo {
	return p.importRunRepo
}

func (p *Provider) RSSSubscriptionRepo() repo.RSSSubscriptionRepo {
	return p.rssSubscriptionRepo
}

func (p *Provider) RSSPullRunRepo() repo.RSSPullRunRepo {
	return p.rssPullRunRepo
}

func (p *Provider) RSSItemRepo() repo.RSSItemRepo {
	return p.rssItemRepo
}

func (p *Provider) PromptTemplateRepo() repo.PromptTemplateRepo {
	return p.promptTemplateRepo
}

func (p *Provider) LLMProfileRepo() repo.LLMProfileRepo {
	return p.llmProfileRepo
}

func (p *Provider) runMigrations(db *sql.DB, fsys fs.FS) error {
	entries, err := fs.ReadDir(fsys, ".")
	if err != nil {
		return fmt.Errorf("read migrations: %w", err)
	}

	var files []string
	for _, e := range entries {
		if !e.IsDir() && strings.HasSuffix(e.Name(), ".sql") {
			files = append(files, e.Name())
		}
	}
	sort.Strings(files)

	for _, file := range files {
		content, err := fs.ReadFile(fsys, file)
		if err != nil {
			return fmt.Errorf("read migration %s: %w", file, err)
		}

		statements := strings.Split(string(content), ";")
		for _, stmt := range statements {
			stmt = strings.TrimSpace(stmt)
			if stmt == "" {
				continue
			}
			if _, err := db.Exec(stmt); err != nil {
				if strings.Contains(err.Error(), "duplicate column name") {
					continue
				}
				return fmt.Errorf("exec migration %s: %w", file, err)
			}
		}
	}

	return nil
}
