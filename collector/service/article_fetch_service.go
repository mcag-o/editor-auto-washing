package service

import (
	"content-hub/collector/plugin"
	"content-hub/domain"
	"content-hub/pkg/repo"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"
)

type ArticleFetchService struct {
	entries  repo.CollectorEntryRepo
	articles repo.CollectorArticleRepo
	attempts repo.CollectorAttemptRepo
	runs     repo.CollectorSourceRunReader
	plugins  *plugin.Registry
}

func NewArticleFetchService(entries repo.CollectorEntryRepo, articles repo.CollectorArticleRepo, attempts repo.CollectorAttemptRepo, runs repo.CollectorSourceRunReader, plugins *plugin.Registry) *ArticleFetchService {
	return &ArticleFetchService{entries: entries, articles: articles, attempts: attempts, runs: runs, plugins: plugins}
}

func (s *ArticleFetchService) FetchForEntry(ctx context.Context, entryID string) (*domain.CollectorArticle, error) {
	entry, err := s.entries.GetByID(ctx, entryID)
	if err != nil {
		return nil, err
	}
	if entry.Status == domain.CollectorEntryFetchedDetail {
		article, articleErr := s.articles.GetByEntryID(ctx, entry.ID)
		if articleErr == nil {
			return article, nil
		}
		if !isNotFoundErr(articleErr) {
			return nil, fmt.Errorf("lookup collector article for fetched entry: %w", articleErr)
		}
	}
	sourceRunID, err := s.resolveSourceRunID(ctx, entry.RunID, entry.SourceID)
	if err != nil {
		return nil, err
	}
	attempt := domain.NewCollectorAttempt(entry.RunID, sourceRunID, entry.ID, domain.CollectorStageDetail)
	started := time.Now().UTC()
	attempt.StartedAt = &started
	attempt.Status = domain.CollectorAttemptRunning
	attempt.RequestURL = entry.CanonicalURL
	p, err := s.plugins.Get(entry.SourceID)
	if err != nil {
		return nil, s.failEntry(ctx, entry, attempt, domain.CollectorErrorSchemaChanged, err)
	}

	rawArticle, err := p.FetchArticle(ctx, plugin.FetchArticleRequest{Entry: hotEntryFromEntry(entry)})
	if err != nil {
		return nil, s.failEntry(ctx, entry, attempt, classifyCollectorError(err), err)
	}
	normalized, err := p.NormalizeArticle(rawArticle)
	if err != nil {
		return nil, s.failEntry(ctx, entry, attempt, classifyCollectorError(err), err)
	}
	article := domain.NewCollectorArticle(entry.ID, entry.RunID, entry.SourceID, normalized.ExternalID, normalized.Title, normalized.CanonicalURL)
	article.Body = normalized.Body
	article.Summary = normalized.Summary
	article.Author = normalized.Author
	article.PublishedAt = normalized.PublishedAt
	article.RawJSON = cloneBytes(normalized.RawJSON)
	if len(article.RawJSON) == 0 {
		article.RawJSON = cloneBytes(rawArticle.Body)
	}
	article.MetadataJSON = mustJSON(normalized.Metadata)
	article.NormalizedJSON = mustJSON(normalized)
	if err := s.articles.Create(ctx, article); err != nil {
		return nil, s.failEntry(ctx, entry, attempt, domain.CollectorErrorStorageFailed, err)
	}
	originalStatus := entry.Status
	originalUpdatedAt := entry.UpdatedAt
	entry.Status = domain.CollectorEntryFetchedDetail
	entry.UpdatedAt = time.Now().UTC()
	if err := s.entries.Update(ctx, entry); err != nil {
		cleanupErr := s.articles.Delete(ctx, article.ID)
		entry.Status = originalStatus
		entry.UpdatedAt = originalUpdatedAt
		return nil, combineErrors(fmt.Errorf("update collector entry after fetch: %w", err), cleanupErr)
	}
	completed := time.Now().UTC()
	attempt.ArticleID = article.ID
	attempt.Status = domain.CollectorAttemptSucceeded
	attempt.RawJSON = cloneBytes(article.RawJSON)
	attempt.CompletedAt = &completed
	if err := s.attempts.Create(ctx, attempt); err != nil {
		cleanupErr := s.articles.Delete(ctx, article.ID)
		entry.Status = originalStatus
		entry.UpdatedAt = originalUpdatedAt
		restoreErr := s.entries.Update(ctx, entry)
		return nil, combineErrors(fmt.Errorf("record collector attempt: %w", err), cleanupErr, restoreErr)
	}
	return article, nil
}

func (s *ArticleFetchService) resolveSourceRunID(ctx context.Context, runID, sourceID string) (string, error) {
	sourceRuns, err := s.runs.ListSourceRuns(ctx, runID)
	if err != nil {
		return "", err
	}
	for _, sourceRun := range sourceRuns {
		if sourceRun.SourceID == sourceID {
			return sourceRun.ID, nil
		}
	}
	return "", domain.NewNotFoundErr("collector_source_run", sourceID)
}

func (s *ArticleFetchService) failEntry(ctx context.Context, entry *domain.CollectorEntry, attempt *domain.CollectorAttempt, code string, cause error) error {
	entry.Status = domain.CollectorEntryDetailFailed
	entry.UpdatedAt = time.Now().UTC()
	if err := s.entries.Update(ctx, entry); err != nil {
		return fmt.Errorf("update collector entry failure state: %w", err)
	}
	if attempt != nil {
		completed := time.Now().UTC()
		attempt.Status = domain.CollectorAttemptFailed
		attempt.ErrorCode = code
		attempt.ErrorMessage = cause.Error()
		attempt.CompletedAt = &completed
		if err := s.attempts.Create(ctx, attempt); err != nil {
			return fmt.Errorf("record collector attempt failure: %w", err)
		}
	}
	return cause
}

func hotEntryFromEntry(entry *domain.CollectorEntry) plugin.HotEntry {
	return plugin.HotEntry{SourceID: entry.SourceID, ExternalID: entry.ExternalID, CanonicalURL: entry.CanonicalURL, Title: entry.Title, Summary: entry.Summary, Author: entry.Author, PublishedAt: entry.PublishedAt, Rank: entry.Rank, RawJSON: cloneBytes(entry.RawJSON)}
}

func mustJSON(value any) []byte {
	body, err := json.Marshal(value)
	if err != nil {
		return []byte("{}")
	}
	return body
}

func classifyCollectorError(err error) string {
	message := strings.ToLower(err.Error())
	switch {
	case strings.Contains(message, "timeout"):
		return domain.CollectorErrorNetworkTimeout
	case strings.Contains(message, "auth"):
		return domain.CollectorErrorAuthMissing
	case strings.Contains(message, "schema changed"):
		return domain.CollectorErrorSchemaChanged
	case strings.Contains(message, "parse"):
		return domain.CollectorErrorParseFailed
	default:
		return domain.CollectorErrorSchemaChanged
	}
}

func isNotFoundErr(err error) bool {
	var appErr *domain.AppError
	return errors.As(err, &appErr) && appErr.Code == domain.ErrNotFound
}

func combineErrors(primary error, cleanup ...error) error {
	if primary == nil {
		return nil
	}
	message := primary.Error()
	for _, err := range cleanup {
		if err == nil {
			continue
		}
		message += "; cleanup failed: " + err.Error()
	}
	return fmt.Errorf("%s", message)
}
