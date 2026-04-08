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
	db           *sql.DB
	articleRepo  repo.ArticleRepo
	templateRepo repo.TemplateRepo
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
				return fmt.Errorf("exec migration %s: %w", file, err)
			}
		}
	}

	return nil
}
