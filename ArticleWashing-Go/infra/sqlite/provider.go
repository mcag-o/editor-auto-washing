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
	db            *sql.DB
	articleRepo   repo.ArticleRepo
	templateRepo  repo.TemplateRepo
	draftRepo     repo.DraftRepo
	assetRepo     repo.AssetRepo
	publishRepo   repo.PublishRepo
	reviewRepo    repo.ReviewRepo
	jobRepo       repo.JobRepo
	jobEventRepo  repo.JobEventRepo
	ingestionRepo repo.IngestionRepo
	workspaceRepo repo.WorkspaceRepo
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
	p.reviewRepo = &reviewRepo{db: db}
	p.publishRepo = &publishRepo{db: db}
	p.jobRepo = &jobRepo{db: db}
	p.jobEventRepo = &jobEventRepo{db: db}
	p.ingestionRepo = &ingestionRepo{db: db}
	p.workspaceRepo = &articleWorkspaceRepo{db: db}

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
