package memory

import (
	"content-hub/domain"
	"context"
	"testing"
)

func TestArticleRepo_CreateAndGet(t *testing.T) {
	p := NewProvider()
	repo := p.ArticleRepo()
	ctx := context.Background()

	doc, err := domain.NewContentDocument("Test Article", "Body content", "markdown")
	if err != nil {
		t.Fatalf("failed to create document: %v", err)
	}

	if err := repo.Create(ctx, doc); err != nil {
		t.Fatalf("failed to create: %v", err)
	}

	got, err := repo.GetByID(ctx, doc.ID)
	if err != nil {
		t.Fatalf("failed to get: %v", err)
	}
	if got.Title != doc.Title {
		t.Errorf("expected title %q, got %q", doc.Title, got.Title)
	}
	if got.Body != doc.Body {
		t.Errorf("expected body %q, got %q", doc.Body, got.Body)
	}
}

func TestArticleRepo_GetNotFound(t *testing.T) {
	p := NewProvider()
	repo := p.ArticleRepo()
	ctx := context.Background()

	_, err := repo.GetByID(ctx, "nonexistent")
	if err == nil {
		t.Fatal("expected error for nonexistent article")
	}

}

func TestArticleRepo_Update(t *testing.T) {
	p := NewProvider()
	repo := p.ArticleRepo()
	ctx := context.Background()

	doc, _ := domain.NewContentDocument("Test", "original", "markdown")
	repo.Create(ctx, doc)

	if err := repo.Update(ctx, doc.ID, "updated body"); err != nil {
		t.Fatalf("failed to update: %v", err)
	}

	got, _ := repo.GetByID(ctx, doc.ID)
	if got.Body != "updated body" {
		t.Errorf("expected body %q, got %q", "updated body", got.Body)
	}
}

func TestArticleRepo_Delete(t *testing.T) {
	p := NewProvider()
	repo := p.ArticleRepo()
	ctx := context.Background()

	doc, _ := domain.NewContentDocument("ToDelete", "body", "markdown")
	repo.Create(ctx, doc)

	if err := repo.Delete(ctx, doc.ID); err != nil {
		t.Fatalf("failed to delete: %v", err)
	}

	_, err := repo.GetByID(ctx, doc.ID)
	if err == nil {
		t.Fatal("expected error after delete")
	}
}

func TestArticleRepo_List(t *testing.T) {
	p := NewProvider()
	repo := p.ArticleRepo()
	ctx := context.Background()

	doc1, _ := domain.NewContentDocument("Alpha", "body1", "markdown")
	doc2, _ := domain.NewContentDocument("Beta", "body2", "markdown")
	repo.Create(ctx, doc1)
	repo.Create(ctx, doc2)

	docs, err := repo.List(ctx, domain.ListQuery{TitleQuery: "Al"})
	if err != nil {
		t.Fatalf("failed to list: %v", err)
	}
	if len(docs) != 1 {
		t.Errorf("expected 1 doc, got %d", len(docs))
	}
	if docs[0].Title != "Alpha" {
		t.Errorf("expected Alpha, got %s", docs[0].Title)
	}
}

func TestTemplateRepo_CreateAndGet(t *testing.T) {
	p := NewProvider()
	repo := p.TemplateRepo()
	ctx := context.Background()

	tpl, err := domain.NewTemplateAsset("email", "welcome", "Hello {{name}}")
	if err != nil {
		t.Fatalf("failed to create template: %v", err)
	}

	if err := repo.Create(ctx, tpl); err != nil {
		t.Fatalf("failed to create: %v", err)
	}

	got, err := repo.GetByID(ctx, tpl.ID)
	if err != nil {
		t.Fatalf("failed to get: %v", err)
	}
	if got.Category != "email" {
		t.Errorf("expected category %q, got %q", "email", got.Category)
	}
	if got.Name != "welcome" {
		t.Errorf("expected name %q, got %q", "welcome", got.Name)
	}
}

func TestTemplateRepo_ListCategories(t *testing.T) {
	p := NewProvider()
	repo := p.TemplateRepo()
	ctx := context.Background()

	tpl1, _ := domain.NewTemplateAsset("email", "welcome", "content1")
	tpl2, _ := domain.NewTemplateAsset("blog", "post", "content2")
	tpl3, _ := domain.NewTemplateAsset("email", "farewell", "content3")
	repo.Create(ctx, tpl1)
	repo.Create(ctx, tpl2)
	repo.Create(ctx, tpl3)

	cats, err := repo.ListCategories(ctx)
	if err != nil {
		t.Fatalf("failed to list categories: %v", err)
	}
	if len(cats) != 2 {
		t.Errorf("expected 2 categories, got %d", len(cats))
	}
	if cats[0] != "blog" || cats[1] != "email" {
		t.Errorf("expected [blog, email], got %v", cats)
	}
}

func TestTemplateRepo_ListByCategory(t *testing.T) {
	p := NewProvider()
	repo := p.TemplateRepo()
	ctx := context.Background()

	tpl1, _ := domain.NewTemplateAsset("email", "welcome", "content1")
	tpl2, _ := domain.NewTemplateAsset("blog", "post", "content2")
	repo.Create(ctx, tpl1)
	repo.Create(ctx, tpl2)

	emails, err := repo.List(ctx, "email")
	if err != nil {
		t.Fatalf("failed to list: %v", err)
	}
	if len(emails) != 1 {
		t.Errorf("expected 1 template, got %d", len(emails))
	}
}

func TestTx_CommitPersistsChanges(t *testing.T) {
	p := NewProvider()
	ctx := context.Background()

	doc, _ := domain.NewContentDocument("Tx Test", "original", "markdown")

	tx := p.BeginTx()
	tx.ArticleRepo().Create(ctx, doc)

	if err := tx.Commit(); err != nil {
		t.Fatalf("commit failed: %v", err)
	}

	got, err := p.ArticleRepo().GetByID(ctx, doc.ID)
	if err != nil {
		t.Fatalf("article not found after commit: %v", err)
	}
	if got.Title != "Tx Test" {
		t.Errorf("expected title %q, got %q", "Tx Test", got.Title)
	}
}

func TestTx_RollbackDiscardsChanges(t *testing.T) {
	p := NewProvider()
	ctx := context.Background()

	tx := p.BeginTx()

	doc, _ := domain.NewContentDocument("Rollback Test", "body", "markdown")
	tx.ArticleRepo().Create(ctx, doc)

	if err := tx.Rollback(); err != nil {
		t.Fatalf("rollback failed: %v", err)
	}

	_, err := p.ArticleRepo().GetByID(ctx, doc.ID)
	if err == nil {
		t.Fatal("expected error after rollback, article should not exist")
	}
}

func TestTx_RollbackRestoresDeleted(t *testing.T) {
	p := NewProvider()
	ctx := context.Background()

	doc, _ := domain.NewContentDocument("To Preserve", "body", "markdown")
	p.ArticleRepo().Create(ctx, doc)

	tx := p.BeginTx()
	tx.ArticleRepo().Delete(ctx, doc.ID)

	tx.Rollback()

	got, err := p.ArticleRepo().GetByID(ctx, doc.ID)
	if err != nil {
		t.Fatalf("article should exist after rollback: %v", err)
	}
	if got.Title != "To Preserve" {
		t.Errorf("expected title %q, got %q", "To Preserve", got.Title)
	}
}

func TestTx_Isolation(t *testing.T) {
	p := NewProvider()
	ctx := context.Background()

	doc1, _ := domain.NewContentDocument("Pre-existing", "body", "markdown")
	p.ArticleRepo().Create(ctx, doc1)

	tx := p.BeginTx()

	doc2, _ := domain.NewContentDocument("Tx Only", "body", "markdown")
	tx.ArticleRepo().Create(ctx, doc2)

	docs, _ := p.ArticleRepo().List(ctx, domain.ListQuery{})
	if len(docs) != 1 {
		t.Errorf("expected 1 article in provider during tx, got %d", len(docs))
	}

	tx.Commit()

	docs, _ = p.ArticleRepo().List(ctx, domain.ListQuery{})
	if len(docs) != 2 {
		t.Errorf("expected 2 articles after commit, got %d", len(docs))
	}
}

func TestTx_TemplateOperations(t *testing.T) {
	p := NewProvider()
	ctx := context.Background()

	tpl, _ := domain.NewTemplateAsset("report", "monthly", "Report content")

	tx := p.BeginTx()
	tx.TemplateRepo().Create(ctx, tpl)

	got, err := tx.TemplateRepo().GetByID(ctx, tpl.ID)
	if err != nil {
		t.Fatalf("template not found in tx: %v", err)
	}
	if got.Content != "Report content" {
		t.Errorf("expected content, got %q", got.Content)
	}

	tx.Rollback()

	_, err = p.TemplateRepo().GetByID(ctx, tpl.ID)
	if err == nil {
		t.Fatal("template should not exist after rollback")
	}
}

func TestProvider_AllReposReturnNonNil(t *testing.T) {
	p := NewProvider()

	if p.ArticleRepo() == nil {
		t.Error("ArticleRepo is nil")
	}
	if p.TemplateRepo() == nil {
		t.Error("TemplateRepo is nil")
	}
	if p.DraftRepo() == nil {
		t.Error("DraftRepo is nil")
	}
	if p.AssetRepo() == nil {
		t.Error("AssetRepo is nil")
	}
	if p.ReviewRepo() == nil {
		t.Error("ReviewRepo is nil")
	}
	if p.PublishRepo() == nil {
		t.Error("PublishRepo is nil")
	}
	if p.JobRepo() == nil {
		t.Error("JobRepo is nil")
	}
	if p.JobEventRepo() == nil {
		t.Error("JobEventRepo is nil")
	}
	if p.IngestionRepo() == nil {
		t.Error("IngestionRepo is nil")
	}
	if p.WorkspaceRepo() == nil {
		t.Error("WorkspaceRepo is nil")
	}
}

func TestTx_AllReposReturnNonNil(t *testing.T) {
	p := NewProvider()
	tx := p.BeginTx()

	if tx.ArticleRepo() == nil {
		t.Error("tx.ArticleRepo is nil")
	}
	if tx.TemplateRepo() == nil {
		t.Error("tx.TemplateRepo is nil")
	}
	if tx.DraftRepo() == nil {
		t.Error("tx.DraftRepo is nil")
	}
	if tx.AssetRepo() == nil {
		t.Error("tx.AssetRepo is nil")
	}
	if tx.ReviewRepo() == nil {
		t.Error("tx.ReviewRepo is nil")
	}
	if tx.PublishRepo() == nil {
		t.Error("tx.PublishRepo is nil")
	}
	if tx.JobRepo() == nil {
		t.Error("tx.JobRepo is nil")
	}
	if tx.JobEventRepo() == nil {
		t.Error("tx.JobEventRepo is nil")
	}
	if tx.IngestionRepo() == nil {
		t.Error("tx.IngestionRepo is nil")
	}
	if tx.WorkspaceRepo() == nil {
		t.Error("tx.WorkspaceRepo is nil")
	}
}
