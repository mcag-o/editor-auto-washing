package service

import "content-hub/domain"

type WebControlRuntime struct {
	Config   *BusinessConfigService
	Control  *ControlStateService
	Audit    *AuditLogService
	Intake   *WebIntakeService
	Articles *ArticleQueryService
}

func BuildWebControlRuntime(repos *RuntimeRepos) (*WebControlRuntime, error) {
	if repos == nil {
		return nil, domain.NewInternalErr("web control runtime repos are required", nil)
	}

	audit := NewAuditLogService(repos.AuditLogRepo)

	return &WebControlRuntime{
		Config:   NewBusinessConfigService(repos.BusinessConfigRepo),
		Control:  NewControlStateService(repos.SystemControlStateRepo),
		Audit:    audit,
		Intake:   NewWebIntakeService(repos.SourceDocumentRepo, repos.AuditLogRepo),
		Articles: NewArticleQueryService(repos.SourceDocumentRepo),
	}, nil
}
