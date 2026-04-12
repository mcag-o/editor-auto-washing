package workspace

import "content-hub/domain"

type DiagnosticReport struct {
	ErrorsList   []string `json:"errors"`
	WarningsList []string `json:"warnings"`
}

func (r DiagnosticReport) HasErrors() bool {
	return len(r.ErrorsList) > 0
}

func (r DiagnosticReport) Errors() []string {
	return r.ErrorsList
}

type Validator struct{}

func NewValidator() *Validator {
	return &Validator{}
}

func (v *Validator) Validate(resolved domain.ResolvedWorkspaceSettings) DiagnosticReport {
	workspace := resolved.Workspace
	report := DiagnosticReport{}

	if _, ok := workspace.ProviderProfiles[workspace.DefaultProviderProfile]; !ok {
		report.ErrorsList = append(report.ErrorsList, "missing default provider profile: "+workspace.DefaultProviderProfile)
	}
	if _, ok := workspace.ArticleProfiles[workspace.DefaultArticleProfile]; !ok {
		report.ErrorsList = append(report.ErrorsList, "missing default article profile: "+workspace.DefaultArticleProfile)
	}
	if _, ok := workspace.PublishProfiles[workspace.DefaultPublishProfile]; !ok {
		report.ErrorsList = append(report.ErrorsList, "missing default publish profile: "+workspace.DefaultPublishProfile)
	}

	for name, profile := range workspace.ProviderProfiles {
		if profile.SecretRef != "" && resolved.Secrets[profile.SecretRef] == "" {
			report.ErrorsList = append(report.ErrorsList, "missing secret for provider profile "+name+": "+profile.SecretRef)
		}
	}
	for name, profile := range workspace.PublishProfiles {
		if profile.SecretRef != "" && resolved.Secrets[profile.SecretRef] == "" {
			report.ErrorsList = append(report.ErrorsList, "missing secret for publish profile "+name+": "+profile.SecretRef)
		}
	}

	return report
}
