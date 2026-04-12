package sqlite

import (
	"content-hub/domain"
	"context"
	"fmt"
	"os"
	"testing"
)

func newTestArticleRepo(t *testing.T) *articleRepo {
	t.Helper()
	dbPath := fmt.Sprintf("%s/test_%d.db", t.TempDir(), os.Getpid())
	p, err := NewProvider(dbPath)
	if err != nil {
		t.Fatalf("failed to create provider: %v", err)
	}
	t.Cleanup(func() {
		p.Close()
	})
	return p.ArticleRepo().(*articleRepo)
}

func TestArticleRepo_CreateAndGetByID(t *testing.T) {
	r := newTestArticleRepo(t)
	ctx := context.Background()

	doc, err := domain.NewContentDocument("Test Article", "Hello world", "markdown")
	if err != nil {
		t.Fatalf("failed to create document: %v", err)
	}
	doc.Summary = "A test article"
	doc.Metadata["tags"] = []string{"test"}

	if err := r.Create(ctx, doc); err != nil {
		t.Fatalf("Create() error = %v", err)
	}

	got, err := r.GetByID(ctx, doc.ID)
	if err != nil {
		t.Fatalf("GetByID() error = %v", err)
	}

	if got.ID != doc.ID {
		t.Errorf("GetByID() ID = %v, want %v", got.ID, doc.ID)
	}
	if got.Title != doc.Title {
		t.Errorf("GetByID() Title = %v, want %v", got.Title, doc.Title)
	}
	if got.Body != doc.Body {
		t.Errorf("GetByID() Body = %v, want %v", got.Body, doc.Body)
	}
	if got.Format != doc.Format {
		t.Errorf("GetByID() Format = %v, want %v", got.Format, doc.Format)
	}
	if got.Summary != doc.Summary {
		t.Errorf("GetByID() Summary = %v, want %v", got.Summary, doc.Summary)
	}
	if got.Metadata["tags"] == nil {
		t.Errorf("GetByID() Metadata tags = nil, want [test]")
	}
}

func TestArticleRepo_GetByID_NotFound(t *testing.T) {
	r := newTestArticleRepo(t)
	ctx := context.Background()

	_, err := r.GetByID(ctx, "nonexistent")
	if err == nil {
		t.Fatal("GetByID() error = nil, want not found error")
	}
	appErr, ok := err.(*domain.AppError)
	if !ok {
		t.Fatalf("GetByID() error type = %T, want *domain.AppError", err)
	}
	if appErr.Code != domain.ErrNotFound {
		t.Errorf("GetByID() error code = %v, want %v", appErr.Code, domain.ErrNotFound)
	}
}

func TestArticleRepo_List(t *testing.T) {
	r := newTestArticleRepo(t)
	ctx := context.Background()

	doc1, _ := domain.NewContentDocument("Go Programming", "Body 1", "markdown")
	doc2, _ := domain.NewContentDocument("Python Basics", "Body 2", "markdown")
	doc3, _ := domain.NewContentDocument("Go Concurrency", "Body 3", "markdown")

	r.Create(ctx, doc1)
	r.Create(ctx, doc2)
	r.Create(ctx, doc3)

	t.Run("list all", func(t *testing.T) {
		docs, err := r.List(ctx, domain.ListQuery{})
		if err != nil {
			t.Fatalf("List() error = %v", err)
		}
		if len(docs) != 3 {
			t.Errorf("List() count = %d, want 3", len(docs))
		}
	})

	t.Run("list with title query", func(t *testing.T) {
		docs, err := r.List(ctx, domain.ListQuery{TitleQuery: "Go"})
		if err != nil {
			t.Fatalf("List() error = %v", err)
		}
		if len(docs) != 2 {
			t.Errorf("List() count = %d, want 2", len(docs))
		}
		for _, d := range docs {
			if d.Title != "Go Programming" && d.Title != "Go Concurrency" {
				t.Errorf("List() unexpected title = %v", d.Title)
			}
		}
	})

	t.Run("list with limit", func(t *testing.T) {
		docs, err := r.List(ctx, domain.ListQuery{Limit: 1})
		if err != nil {
			t.Fatalf("List() error = %v", err)
		}
		if len(docs) != 1 {
			t.Errorf("List() count = %d, want 1", len(docs))
		}
	})
}

func TestArticleRepo_Update(t *testing.T) {
	r := newTestArticleRepo(t)
	ctx := context.Background()

	doc, _ := domain.NewContentDocument("Update Test", "Original body", "markdown")
	r.Create(ctx, doc)

	if err := r.Update(ctx, doc.ID, "Updated body"); err != nil {
		t.Fatalf("Update() error = %v", err)
	}

	got, err := r.GetByID(ctx, doc.ID)
	if err != nil {
		t.Fatalf("GetByID() error = %v", err)
	}
	if got.Body != "Updated body" {
		t.Errorf("Update() body = %v, want 'Updated body'", got.Body)
	}
}

func TestArticleRepo_Update_NotFound(t *testing.T) {
	r := newTestArticleRepo(t)
	ctx := context.Background()

	err := r.Update(ctx, "nonexistent", "new body")
	if err == nil {
		t.Fatal("Update() error = nil, want not found error")
	}
	appErr, ok := err.(*domain.AppError)
	if !ok {
		t.Fatalf("Update() error type = %T, want *domain.AppError", err)
	}
	if appErr.Code != domain.ErrNotFound {
		t.Errorf("Update() error code = %v, want %v", appErr.Code, domain.ErrNotFound)
	}
}

func TestArticleRepo_Delete(t *testing.T) {
	r := newTestArticleRepo(t)
	ctx := context.Background()

	doc, _ := domain.NewContentDocument("Delete Test", "Body", "markdown")
	r.Create(ctx, doc)

	if err := r.Delete(ctx, doc.ID); err != nil {
		t.Fatalf("Delete() error = %v", err)
	}

	_, err := r.GetByID(ctx, doc.ID)
	if err == nil {
		t.Fatal("GetByID() after delete error = nil, want not found")
	}
}

func TestArticleRepo_Delete_NotFound(t *testing.T) {
	r := newTestArticleRepo(t)
	ctx := context.Background()

	err := r.Delete(ctx, "nonexistent")
	if err == nil {
		t.Fatal("Delete() error = nil, want not found error")
	}
	appErr, ok := err.(*domain.AppError)
	if !ok {
		t.Fatalf("Delete() error type = %T, want *domain.AppError", err)
	}
	if appErr.Code != domain.ErrNotFound {
		t.Errorf("Delete() error code = %v, want %v", appErr.Code, domain.ErrNotFound)
	}
}
