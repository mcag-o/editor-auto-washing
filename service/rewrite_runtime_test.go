package service

import "testing"

func TestBuildRewriteRuntimeReturnsReadyServices(t *testing.T) {
	repos, cleanup, err := BuildRuntimeRepos(t.TempDir())
	if cleanup != nil {
		defer func() {
			if closeErr := cleanup(); closeErr != nil {
				t.Fatalf("cleanup returned error: %v", closeErr)
			}
		}()
	}
	if err != nil {
		t.Fatalf("BuildRuntimeRepos error: %v", err)
	}

	runtime, err := BuildRewriteRuntime(repos)
	if err != nil {
		t.Fatalf("BuildRewriteRuntime error: %v", err)
	}
	if runtime == nil {
		t.Fatal("expected rewrite runtime to be configured")
	}
	if runtime.Orchestrator == nil {
		t.Fatal("expected orchestrator to be configured")
	}
	if runtime.PromptRegistry == nil || runtime.ProfileRegistry == nil {
		t.Fatal("expected rewrite runtime dependencies to be configured")
	}
	if runtime.QualityGate == nil || runtime.StageExecutor == nil || runtime.DraftMaterializer == nil {
		t.Fatal("expected rewrite runtime services to be configured")
	}
}
