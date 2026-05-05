package service

import (
	"content-hub/domain"
	"content-hub/pkg/repo"
	"context"
	"strings"
)

type ArticleQueryFilter struct {
	Status string
	Limit  int
}

type ArticleQueryService struct {
	repo repo.SourceDocumentRepo
}

func NewArticleQueryService(r repo.SourceDocumentRepo) *ArticleQueryService {
	return &ArticleQueryService{repo: r}
}

func (s *ArticleQueryService) ListSourceDocuments(ctx context.Context, filter ArticleQueryFilter) ([]domain.SourceDocument, error) {
	if s.repo == nil {
		return nil, domain.NewInternalErr("article query service is not configured", nil)
	}
	status := strings.TrimSpace(filter.Status)
	if status == "" {
		return nil, domain.NewValidationErr("status is required", nil)
	}
	return s.repo.ListByStatus(ctx, status, filter.Limit)
}

func (s *ArticleQueryService) GetSourceDocument(ctx context.Context, id string) (*domain.SourceDocument, error) {
	if s.repo == nil {
		return nil, domain.NewInternalErr("article query service is not configured", nil)
	}
	id = strings.TrimSpace(id)
	if id == "" {
		return nil, domain.NewValidationErr("source document id is required", nil)
	}
	return s.repo.GetByID(ctx, id)
}
