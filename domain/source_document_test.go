package domain

import (
	"testing"
	"time"
)

func TestNewSourceDocumentDefaultsToDiscovered(t *testing.T) {
	doc := NewSourceDocument("article.md", "/inbox/article.md", "md", "", "", "")
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
	if doc.Hash != "" {
		t.Fatalf("unexpected hash: %s", doc.Hash)
	}
	if doc.Title != "" {
		t.Fatalf("unexpected title: %s", doc.Title)
	}
	if doc.Body != "" {
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
		Status:           "unknown",
	}
	if err := doc.Validate(); err == nil {
		t.Fatal("expected validate to reject unsupported status")
	}
}

func TestSourceDocumentValidateDiscoveredAllowsMissingParsedContent(t *testing.T) {
	doc := SourceDocument{
		SourceType:       "folder",
		OriginalFilename: "article.md",
		OriginalPath:     "/inbox/article.md",
		FileType:         "md",
		Status:           SourceDocumentStatusDiscovered,
	}
	if err := doc.Validate(); err != nil {
		t.Fatalf("expected discovered document to validate without parsed content: %v", err)
	}
}

func TestSourceDocumentValidatePendingRejectsMissingTitle(t *testing.T) {
	doc := validParsedSourceDocument(SourceDocumentStatusPending)
	doc.Title = ""
	if err := doc.Validate(); err == nil {
		t.Fatal("expected pending document to reject missing title")
	}
}

func TestSourceDocumentValidateImportedRejectsMissingBody(t *testing.T) {
	doc := validParsedSourceDocument(SourceDocumentStatusImported)
	doc.Body = ""
	if err := doc.Validate(); err == nil {
		t.Fatal("expected imported document to reject missing body")
	}
}

func TestSourceDocumentValidatePendingRejectsMissingHash(t *testing.T) {
	doc := validParsedSourceDocument(SourceDocumentStatusPending)
	doc.Hash = ""
	if err := doc.Validate(); err == nil {
		t.Fatal("expected pending document to reject missing hash")
	}
}

func TestSourceDocumentValidateClaimedRequiresClaimMetadata(t *testing.T) {
	doc := validParsedSourceDocument(SourceDocumentStatusClaimed)
	if err := doc.Validate(); err == nil {
		t.Fatal("expected claimed document to reject missing claim metadata")
	}

	now := time.Now().UTC()
	doc.ClaimedBy = "worker-1"
	doc.ClaimedAt = &now
	if err := doc.Validate(); err != nil {
		t.Fatalf("expected claimed document with claim metadata to validate: %v", err)
	}
}

func TestSourceDocumentValidateProcessingRequiresProcessingStartTime(t *testing.T) {
	doc := validParsedSourceDocument(SourceDocumentStatusProcessing)
	if err := doc.Validate(); err == nil {
		t.Fatal("expected processing document to reject missing processing start time")
	}

	now := time.Now().UTC()
	doc.ProcessingStartedAt = &now
	if err := doc.Validate(); err != nil {
		t.Fatalf("expected processing document with start time to validate: %v", err)
	}
}

func TestSourceDocumentValidateCompletedRequiresCompletedTime(t *testing.T) {
	doc := validParsedSourceDocument(SourceDocumentStatusCompleted)
	if err := doc.Validate(); err == nil {
		t.Fatal("expected completed document to reject missing completed time")
	}

	now := time.Now().UTC()
	doc.CompletedAt = &now
	if err := doc.Validate(); err != nil {
		t.Fatalf("expected completed document with completion time to validate: %v", err)
	}
}

func TestSourceDocumentValidateFailedRequiresErrorSummary(t *testing.T) {
	doc := validParsedSourceDocument(SourceDocumentStatusFailed)
	if err := doc.Validate(); err == nil {
		t.Fatal("expected failed document to reject missing error summary")
	}

	doc.ErrorSummary = "parser failed"
	if err := doc.Validate(); err != nil {
		t.Fatalf("expected failed document with error summary to validate: %v", err)
	}
}

func validParsedSourceDocument(status string) SourceDocument {
	return SourceDocument{
		SourceType:       "folder",
		OriginalFilename: "article.md",
		OriginalPath:     "/inbox/article.md",
		FileType:         "md",
		Title:            "Title",
		Body:             "Body",
		Hash:             "hash-1",
		Status:           status,
	}
}
