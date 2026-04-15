package service

import "testing"

func TestBuildRuntimeReposExposesRewriteRepos(t *testing.T) {
	repos, cleanup, err := BuildRuntimeRepos(t.TempDir())
	if cleanup != nil {
		defer func() {
			if closeErr := cleanup(); closeErr != nil {
				t.Fatalf("cleanup returned error: %v", closeErr)
			}
		}()
	}
	if err != nil {
		t.Fatalf("BuildRuntimeRepos returned error: %v", err)
	}
	if repos.RewritePipelineRunRepo == nil {
		t.Fatal("expected RewritePipelineRunRepo to be wired")
	}
	if repos.PromptTemplateRepo == nil {
		t.Fatal("expected PromptTemplateRepo to be wired")
	}
}
