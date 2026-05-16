package repo

import (
	"context"
	"testing"
	"time"

	"content-hub/domain"
)

type staticLLMProvider struct{}

func (staticLLMProvider) Generate(_ context.Context, _ domain.LLMGenerateRequest) (*domain.LLMGenerateResponse, error) {
	return &domain.LLMGenerateResponse{}, nil
}

func (staticLLMProvider) Models(_ context.Context) ([]string, error) {
	return []string{"mock-1"}, nil
}

func (staticLLMProvider) Name() string {
	return "static"
}

type sourceDocumentRepoCompileStub struct{}

func (sourceDocumentRepoCompileStub) Create(context.Context, *domain.SourceDocument) error {
	return nil
}
func (sourceDocumentRepoCompileStub) Update(context.Context, *domain.SourceDocument) error {
	return nil
}
func (sourceDocumentRepoCompileStub) UpdateIfStatus(context.Context, *domain.SourceDocument, ...string) error {
	return nil
}
func (sourceDocumentRepoCompileStub) Delete(context.Context, string) error {
	return nil
}
func (sourceDocumentRepoCompileStub) DeleteIfStatus(context.Context, string, ...string) error {
	return nil
}
func (sourceDocumentRepoCompileStub) GetByID(context.Context, string) (*domain.SourceDocument, error) {
	return nil, nil
}
func (sourceDocumentRepoCompileStub) List(context.Context, int) ([]domain.SourceDocument, error) {
	return nil, nil
}
func (sourceDocumentRepoCompileStub) FindByHash(context.Context, string) (*domain.SourceDocument, error) {
	return nil, nil
}
func (sourceDocumentRepoCompileStub) ClaimPending(context.Context, int, string, time.Time) ([]domain.SourceDocument, error) {
	return nil, nil
}
func (sourceDocumentRepoCompileStub) ListByStatus(context.Context, string, int) ([]domain.SourceDocument, error) {
	return nil, nil
}

type importRunRepoCompileStub struct{}

func (importRunRepoCompileStub) Create(context.Context, *domain.ImportRun) error { return nil }
func (importRunRepoCompileStub) Update(context.Context, *domain.ImportRun) error { return nil }
func (importRunRepoCompileStub) GetByID(context.Context, string) (*domain.ImportRun, error) {
	return nil, nil
}
func (importRunRepoCompileStub) List(context.Context, int) ([]domain.ImportRun, error) {
	return nil, nil
}

func TestSourceDocumentRepoUsesCompileContract(t *testing.T) {
	var _ SourceDocumentRepo = sourceDocumentRepoCompileStub{}
}

func TestImportRunRepoUsesCompileContract(t *testing.T) {
	var _ ImportRunRepo = importRunRepoCompileStub{}
}

func TestLLMProviderUsesLLMClientContract(t *testing.T) {
	var _ LLMProvider = staticLLMProvider{}
}
