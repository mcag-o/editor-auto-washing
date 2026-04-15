package domain

import "testing"

func TestDefaultRewritePipelineRunStartsPending(t *testing.T) {
	run := NewRewritePipelineRun("profile-1", "v1", "workspace-1", "collector-1", "wechat-longform", "sspai")
	if run.Status != RewriteRunPending {
		t.Fatalf("expected pending status, got %s", run.Status)
	}
	if run.CurrentStage != "" {
		t.Fatalf("expected empty current stage, got %s", run.CurrentStage)
	}
}

func TestWorkspaceTransitionsIncludeRewriteLifecycle(t *testing.T) {
	if err := ValidateWorkspaceTransition(ArticleWorkspaceStatusImported, ArticleWorkspaceStatusRewritePending); err != nil {
		t.Fatalf("expected imported -> rewrite_pending to be valid: %v", err)
	}
	if err := ValidateWorkspaceTransition(ArticleWorkspaceStatusRewriting, ArticleWorkspaceStatusDraft); err != nil {
		t.Fatalf("expected rewriting -> draft to be valid: %v", err)
	}
	if err := ValidateWorkspaceTransition(ArticleWorkspaceStatusDraft, ArticleWorkspaceStatusRewritePending); err == nil {
		t.Fatal("expected draft -> rewrite_pending to be invalid")
	}
}
