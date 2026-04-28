package domain

import "testing"

func TestNewSourceDocumentDefaultsToDiscovered(t *testing.T) {
	doc := NewSourceDocument("article.md", "/inbox/article.md", "md", "Title", "Body", "hash-1")
	if doc.Status != SourceDocumentStatusDiscovered {
		t.Fatalf("expected discovered status, got %s", doc.Status)
	}
	if doc.FileType != "md" {
		t.Fatalf("unexpected file type: %s", doc.FileType)
	}
	if err := doc.Validate(); err != nil {
		t.Fatalf("expected new source document to validate: %v", err)
	}
	if doc.Metadata == nil {
		t.Fatal("expected metadata to be initialized")
	}
	if doc.ID == "" {
		t.Fatal("expected new source document to have an id")
	}
	if doc.OriginalFilename != "article.md" {
		t.Fatalf("unexpected original filename: %s", doc.OriginalFilename)
	}
	if doc.OriginalPath != "/inbox/article.md" {
		t.Fatalf("unexpected original path: %s", doc.OriginalPath)
	}
	if doc.Hash != "hash-1" {
		t.Fatalf("unexpected hash: %s", doc.Hash)
	}
	if doc.Title != "Title" {
		t.Fatalf("unexpected title: %s", doc.Title)
	}
	if doc.Body != "Body" {
		t.Fatalf("unexpected body: %s", doc.Body)
	}
}

func TestSourceDocumentValidateRequiresCoreFields(t *testing.T) {
	doc := SourceDocument{}
	if err := doc.Validate(); err == nil {
		t.Fatal("expected validate to reject empty document")
	}
}

func TestSourceDocumentValidateRejectsUnsupportedStatus(t *testing.T) {
	doc := SourceDocument{
		SourceType:       "folder",
		OriginalFilename: "article.md",
		OriginalPath:     "/inbox/article.md",
		FileType:         "md",
		Title:            "Title",
		Body:             "Body",
		Hash:             "hash-1",
		Status:           "unknown",
	}
	if err := doc.Validate(); err == nil {
		t.Fatal("expected validate to reject unsupported status")
	}
}

func TestNewImportRunDefaultsToPending(t *testing.T) {
	run := NewImportRun("folder")
	if run.Status != ImportRunStatusPending {
		t.Fatalf("expected pending status, got %s", run.Status)
	}
	if err := run.Validate(); err != nil {
		t.Fatalf("expected new import run to validate: %v", err)
	}
	if run.Metadata == nil {
		t.Fatal("expected metadata to be initialized")
	}
	if run.ID == "" {
		t.Fatal("expected new import run to have an id")
	}
	if run.SourceType != "folder" {
		t.Fatalf("unexpected source type: %s", run.SourceType)
	}
}

func TestImportRunValidateRequiresCoreFields(t *testing.T) {
	run := ImportRun{}
	if err := run.Validate(); err == nil {
		t.Fatal("expected validate to reject empty import run")
	}
}

func TestImportRunValidateRejectsUnsupportedStatus(t *testing.T) {
	run := ImportRun{SourceType: "folder", Status: "unknown"}
	if err := run.Validate(); err == nil {
		t.Fatal("expected validate to reject unsupported status")
	}
}
