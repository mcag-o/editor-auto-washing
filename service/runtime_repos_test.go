package service

import (
	"reflect"
	"strings"
	"testing"
)

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

func TestBuildRuntimeReposDoesNotExposeRSSRepos(t *testing.T) {
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
	repoType := reflect.TypeOf(*repos)
	for i := range repoType.NumField() {
		field := repoType.Field(i)
		if strings.HasPrefix(field.Name, "RSS") {
			t.Fatalf("expected RuntimeRepos to omit RSS fields, found %s", field.Name)
		}
	}
}

func TestBuildRuntimeReposOmitsLegacyInputCompatibilityRepos(t *testing.T) {
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
	repoType := reflect.TypeOf(*repos)
	if _, ok := repoType.FieldByName("InputDocumentRepo"); ok {
		t.Fatal("expected RuntimeRepos to omit browser intake compatibility repo field")
	}
	if _, ok := repoType.FieldByName("ImportRunRepo"); ok {
		t.Fatal("expected RuntimeRepos to omit browser intake import-run compatibility field")
	}
}

func TestBuildRuntimeReposExposesWebControlPlaneRepos(t *testing.T) {
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
	if repos.BusinessConfigRepo == nil {
		t.Fatal("expected BusinessConfigRepo to be wired")
	}
	if repos.SystemControlStateRepo == nil {
		t.Fatal("expected SystemControlStateRepo to be wired")
	}
	if repos.AuditLogRepo == nil {
		t.Fatal("expected AuditLogRepo to be wired")
	}
}

func TestBuildRuntimeReposExposesWorkflowTemplateRepos(t *testing.T) {
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
	if repos.WorkflowDefinitionRepo == nil {
		t.Fatal("expected WorkflowDefinitionRepo to be wired")
	}
	if repos.TemplateDefinitionRepo == nil {
		t.Fatal("expected TemplateDefinitionRepo to be wired")
	}
	if repos.WorkflowRunRepo == nil {
		t.Fatal("expected WorkflowRunRepo to be wired")
	}
	if repos.WorkflowCheckpointRepo == nil {
		t.Fatal("expected WorkflowCheckpointRepo to be wired")
	}
}
