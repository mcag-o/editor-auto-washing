package service

import (
	"content-hub/domain"
	"content-hub/pkg/repo"
	"context"
)

type ContentService struct {
	articleRepo repo.ArticleRepo
	publishRepo repo.PublishRepo
}

func NewContentService(articleRepo repo.ArticleRepo, publishRepo repo.PublishRepo) *ContentService {
	return &ContentService{articleRepo: articleRepo, publishRepo: publishRepo}
}

func (s *ContentService) CreateDocument(ctx context.Context, title, body, format string) (*domain.ContentDocument, error) {
	doc, err := domain.NewContentDocument(title, body, format)
	if err != nil {
		return nil, err
	}
	if err := s.articleRepo.Create(ctx, doc); err != nil {
		return nil, domain.NewInternalErr("failed to create document", err)
	}
	return doc, nil
}

func (s *ContentService) GetDocument(ctx context.Context, id string) (*domain.ContentDocument, error) {
	return s.articleRepo.GetByID(ctx, id)
}

func (s *ContentService) ListDocuments(ctx context.Context, q domain.ListQuery) ([]domain.ContentDocument, error) {
	return s.articleRepo.List(ctx, q)
}

func (s *ContentService) UpdateDocument(ctx context.Context, id, body string) (*domain.ContentDocument, error) {
	if err := s.articleRepo.Update(ctx, id, body); err != nil {
		return nil, err
	}
	return s.articleRepo.GetByID(ctx, id)
}

func (s *ContentService) DeleteDocument(ctx context.Context, id string) error {
	return s.articleRepo.Delete(ctx, id)
}

func (s *ContentService) GetPublishHistory(ctx context.Context, title string) ([]domain.PublishRecord, error) {
	return s.publishRepo.ListByArticle(ctx, title)
}
