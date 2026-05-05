package service

import (
	"content-hub/domain"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestArticleQueryServiceListsSourceDocumentsByStatus(t *testing.T) {
	repos, cleanup, err := BuildRuntimeRepos(t.TempDir())
	if cleanup != nil {
		defer func() {
			require.NoError(t, cleanup())
		}()
	}
	require.NoError(t, err)

	pendingA := domain.NewSourceDocument("pending-a.md", "pending-a.md", "md", "Pending A", "Body A", "hash-pending-a")
	pendingA.Status = domain.SourceDocumentStatusPending
	pendingB := domain.NewSourceDocument("pending-b.md", "pending-b.md", "md", "Pending B", "Body B", "hash-pending-b")
	pendingB.Status = domain.SourceDocumentStatusPending
	completed := domain.NewSourceDocument("completed.md", "completed.md", "md", "Completed", "Body C", "hash-completed")
	completed.Status = domain.SourceDocumentStatusCompleted
	now := time.Now().UTC()
	completed.CompletedAt = &now

	require.NoError(t, repos.SourceDocumentRepo.Create(t.Context(), pendingA))
	require.NoError(t, repos.SourceDocumentRepo.Create(t.Context(), pendingB))
	require.NoError(t, repos.SourceDocumentRepo.Create(t.Context(), completed))

	svc := NewArticleQueryService(repos.SourceDocumentRepo)

	docs, err := svc.ListSourceDocuments(t.Context(), ArticleQueryFilter{
		Status: domain.SourceDocumentStatusPending,
		Limit:  1,
	})

	require.NoError(t, err)
	require.Len(t, docs, 1)
	require.Equal(t, domain.SourceDocumentStatusPending, docs[0].Status)
	require.NotEqual(t, completed.ID, docs[0].ID)
}

func TestArticleQueryServiceGetsSourceDocumentDetailByID(t *testing.T) {
	repos, cleanup, err := BuildRuntimeRepos(t.TempDir())
	if cleanup != nil {
		defer func() {
			require.NoError(t, cleanup())
		}()
	}
	require.NoError(t, err)

	doc := domain.NewSourceDocument("article.json", "article.json", "json", "Title", "Body", "hash-detail")
	doc.Status = domain.SourceDocumentStatusPending
	doc.Summary = "Summary"
	doc.Metadata["source_profile"] = "web-upload"

	require.NoError(t, repos.SourceDocumentRepo.Create(t.Context(), doc))

	svc := NewArticleQueryService(repos.SourceDocumentRepo)

	loaded, err := svc.GetSourceDocument(t.Context(), doc.ID)

	require.NoError(t, err)
	require.NotNil(t, loaded)
	require.Equal(t, doc.ID, loaded.ID)
	require.Equal(t, "Title", loaded.Title)
	require.Equal(t, "Body", loaded.Body)
	require.Equal(t, "Summary", loaded.Summary)
	require.Equal(t, domain.SourceDocumentStatusPending, loaded.Status)
	require.Equal(t, "web-upload", loaded.Metadata["source_profile"])
}
