package sqlite

import (
	"content-hub/domain"
	"context"
	"fmt"
	"os"
	"testing"
)

func newTestTemplateRepo(t *testing.T) *templateRepo {
	t.Helper()
	dbPath := fmt.Sprintf("%s/test_template_%d.db", t.TempDir(), os.Getpid())
	p, err := NewProvider(dbPath)
	if err != nil {
		t.Fatalf("failed to create provider: %v", err)
	}
	t.Cleanup(func() {
		p.Close()
	})
	return p.TemplateRepo().(*templateRepo)
}

func TestTemplateRepo_CreateAndGetByID(t *testing.T) {
	r := newTestTemplateRepo(t)
	ctx := context.Background()

	tmpl, err := domain.NewTemplateAsset("news", "Breaking News", "{{title}}\n{{body}}")
	if err != nil {
		t.Fatalf("failed to create template: %v", err)
	}
	tmpl.Metadata["version"] = "1.0"

	if err := r.Create(ctx, tmpl); err != nil {
		t.Fatalf("Create() error = %v", err)
	}

	got, err := r.GetByID(ctx, tmpl.ID)
	if err != nil {
		t.Fatalf("GetByID() error = %v", err)
	}

	if got.ID != tmpl.ID {
		t.Errorf("GetByID() ID = %v, want %v", got.ID, tmpl.ID)
	}
	if got.Category != tmpl.Category {
		t.Errorf("GetByID() Category = %v, want %v", got.Category, tmpl.Category)
	}
	if got.Name != tmpl.Name {
		t.Errorf("GetByID() Name = %v, want %v", got.Name, tmpl.Name)
	}
	if got.Content != tmpl.Content {
		t.Errorf("GetByID() Content = %v, want %v", got.Content, tmpl.Content)
	}
	if got.Metadata["version"] == nil {
		t.Errorf("GetByID() Metadata version = nil, want 1.0")
	}
}

func TestTemplateRepo_GetByID_NotFound(t *testing.T) {
	r := newTestTemplateRepo(t)
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

func TestTemplateRepo_List(t *testing.T) {
	r := newTestTemplateRepo(t)
	ctx := context.Background()

	tmpl1, _ := domain.NewTemplateAsset("news", "Breaking News", "content1")
	tmpl2, _ := domain.NewTemplateAsset("news", "Daily Digest", "content2")
	tmpl3, _ := domain.NewTemplateAsset("tutorial", "How To", "content3")

	r.Create(ctx, tmpl1)
	r.Create(ctx, tmpl2)
	r.Create(ctx, tmpl3)

	t.Run("list all", func(t *testing.T) {
		templates, err := r.List(ctx, "")
		if err != nil {
			t.Fatalf("List() error = %v", err)
		}
		if len(templates) != 3 {
			t.Errorf("List() count = %d, want 3", len(templates))
		}
	})

	t.Run("list by category", func(t *testing.T) {
		templates, err := r.List(ctx, "news")
		if err != nil {
			t.Fatalf("List() error = %v", err)
		}
		if len(templates) != 2 {
			t.Errorf("List() count = %d, want 2", len(templates))
		}
		for _, tmpl := range templates {
			if tmpl.Category != "news" {
				t.Errorf("List() unexpected category = %v", tmpl.Category)
			}
		}
	})
}

func TestTemplateRepo_ListCategories(t *testing.T) {
	r := newTestTemplateRepo(t)
	ctx := context.Background()

	tmpl1, _ := domain.NewTemplateAsset("news", "Breaking", "content1")
	tmpl2, _ := domain.NewTemplateAsset("tutorial", "Guide", "content2")
	tmpl3, _ := domain.NewTemplateAsset("news", "Digest", "content3")

	r.Create(ctx, tmpl1)
	r.Create(ctx, tmpl2)
	r.Create(ctx, tmpl3)

	categories, err := r.ListCategories(ctx)
	if err != nil {
		t.Fatalf("ListCategories() error = %v", err)
	}

	if len(categories) != 2 {
		t.Errorf("ListCategories() count = %d, want 2", len(categories))
	}

	expected := map[string]bool{"news": true, "tutorial": true}
	for _, cat := range categories {
		if !expected[cat] {
			t.Errorf("ListCategories() unexpected category = %v", cat)
		}
	}
}

func TestTemplateRepo_Update(t *testing.T) {
	r := newTestTemplateRepo(t)
	ctx := context.Background()

	tmpl, _ := domain.NewTemplateAsset("news", "Test Template", "Original content")
	r.Create(ctx, tmpl)

	if err := r.Update(ctx, tmpl.ID, "Updated content"); err != nil {
		t.Fatalf("Update() error = %v", err)
	}

	got, err := r.GetByID(ctx, tmpl.ID)
	if err != nil {
		t.Fatalf("GetByID() error = %v", err)
	}
	if got.Content != "Updated content" {
		t.Errorf("Update() content = %v, want 'Updated content'", got.Content)
	}
}

func TestTemplateRepo_Update_NotFound(t *testing.T) {
	r := newTestTemplateRepo(t)
	ctx := context.Background()

	err := r.Update(ctx, "nonexistent", "new content")
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

func TestTemplateRepo_Delete(t *testing.T) {
	r := newTestTemplateRepo(t)
	ctx := context.Background()

	tmpl, _ := domain.NewTemplateAsset("news", "Delete Test", "content")
	r.Create(ctx, tmpl)

	if err := r.Delete(ctx, tmpl.ID); err != nil {
		t.Fatalf("Delete() error = %v", err)
	}

	_, err := r.GetByID(ctx, tmpl.ID)
	if err == nil {
		t.Fatal("GetByID() after delete error = nil, want not found")
	}
}

func TestTemplateRepo_Delete_NotFound(t *testing.T) {
	r := newTestTemplateRepo(t)
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
