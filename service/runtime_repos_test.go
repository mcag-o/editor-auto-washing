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
	if repos.RewritePipelineProfileRepo == nil {
		t.Fatal("expected RewritePipelineProfileRepo to be wired")
	}
	if repos.RewriteStageRunRepo == nil {
		t.Fatal("expected RewriteStageRunRepo to be wired")
	}
	if repos.PromptTemplateRepo == nil {
		t.Fatal("expected PromptTemplateRepo to be wired")
	}
	if repos.LLMProfileRepo == nil {
		t.Fatal("expected LLMProfileRepo to be wired")
	}
}

func TestBuildRuntimeReposExposesRSSRepos(t *testing.T) {
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
	if repos.RSSSubscriptionRepo == nil {
		t.Fatal("expected RSSSubscriptionRepo to be wired")
	}
	if repos.RSSItemRepo == nil {
		t.Fatal("expected RSSItemRepo to be wired")
	}
}

func TestBuildRuntimeReposExposesFolderIntakeRepos(t *testing.T) {
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
	if repos.SourceDocumentRepo == nil {
		t.Fatal("expected SourceDocumentRepo to be wired")
	}
	if repos.ImportRunRepo == nil {
		t.Fatal("expected ImportRunRepo to be wired")
	}
}
