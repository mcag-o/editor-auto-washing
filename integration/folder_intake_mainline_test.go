package integration

import (
	"content-hub/domain"
	llminfra "content-hub/infra/llm"
	workspaceinfra "content-hub/infra/workspace"
	"content-hub/service"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestFolderIntakeMainlineCreatesRenderedOutput(t *testing.T) {
	root := t.TempDir()
	loader := workspaceinfra.NewLoader()
	require.NoError(t, loader.Save(root, domain.DefaultWorkspaceSettings()))
	require.NoError(t, os.WriteFile(filepath.Join(root, workspaceinfra.WorkspaceSecretsFileName), []byte("env:\n  LLM_API_KEY: test\nwechat:\n  main: token\n"), 0o600))

	templateDir := filepath.Join(root, "templates")
	require.NoError(t, os.MkdirAll(templateDir, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(templateDir, "daily-intelligence.html"), []byte(`<html><body><h1>{{TITLE}}</h1><div>{{BODY_SECTIONS}}</div><footer>{{CTA}}</footer></body></html>`), 0o644))

	repos, cleanup, err := service.BuildRuntimeRepos(root)
	require.NoError(t, err)
	defer func() {
		require.NoError(t, cleanup())
	}()

	repos.LLMClient = llminfra.StaticClient{Response: domain.LLMResponse{
		Content:      `{"title":"Rewritten Folder Title","body":"Folder mainline draft body.","template":"daily-intelligence","meta":{"digest":"Folder digest","author":"Integration Bot"},"sections":[{"cn":"Main Section","blocks":[{"type":"card","title":"Key Point","body":["Mainline supporting detail."],"source":"Folder Intake"}]}],"conclusion":"Closing thought.","cta":"Keep watching."}`,
		Model:        "static-integration-model",
		FinishReason: "stop",
	}}

	require.NoError(t, repos.RewritePipelineProfileRepo.Upsert(t.Context(), &domain.RewritePipelineProfile{
		ID:                    "profile-folder-mainline",
		Name:                  "Folder Intake Mainline",
		TargetType:            "wechat-longform",
		SourceProfile:         "folder-default",
		Version:               "v1",
		Description:           "Mainline folder rewrite profile",
		DefaultLLMProfile:     "rewrite-default",
		MaterializationPolicy: "workspace-draft",
		Enabled:               true,
		Stages: []domain.RewriteStageDefinition{{
			Name:      "generate_draft",
			Type:      "generate_draft",
			PromptRef: "generate_draft@v1",
			Enabled:   true,
		}},
	}))

	require.NoError(t, repos.PromptTemplateRepo.Upsert(t.Context(), &domain.PromptTemplate{
		Key:            "generate_draft",
		Version:        "v1",
		SystemTemplate: "You rewrite imported articles into drafts.",
		UserTemplate:   "Rewrite {{title}} into a polished draft.",
		Description:    "Integration rewrite prompt",
	}))

	require.NoError(t, repos.LLMProfileRepo.Upsert(t.Context(), &domain.LLMProfile{
		Name:        "rewrite-default",
		Provider:    "openai",
		Model:       "static-integration-model",
		Temperature: 0.2,
		MaxTokens:   512,
		TimeoutSec:  30,
	}))

	inbox := filepath.Join(root, "folder-inbox")
	archive := filepath.Join(inbox, "SyncOver")
	require.NoError(t, os.MkdirAll(archive, 0o755))
	sourcePath := filepath.Join(inbox, "article.md")
	require.NoError(t, os.WriteFile(sourcePath, []byte("# Folder Source Title\n\nSource body from folder intake."), 0o644))

	runtime, err := service.BuildFolderIntakeRuntime(repos, service.FolderIntakeConfig{
		WatchDir:              inbox,
		ArchiveDir:            archive,
		Concurrency:           1,
		TargetType:            "wechat-longform",
		SourceProfile:         "folder-default",
		RenderPlatform:        "wechat",
		RewriteProfileVersion: "v1",
	})
	require.NoError(t, err)

	run, err := runtime.Scanner.ScanOnce(t.Context(), runtime.WatchDir, runtime.ArchiveDir)
	require.NoError(t, err)
	require.Equal(t, domain.ImportRunStatusCompleted, run.Status)
	require.Equal(t, 1, run.ImportedCount)

	pendingDocs, err := repos.SourceDocumentRepo.ListByStatus(t.Context(), domain.SourceDocumentStatusPending, 10)
	require.NoError(t, err)
	require.Len(t, pendingDocs, 1)

	processed, err := runtime.Scheduler.ProcessPending(t.Context())
	require.NoError(t, err)
	require.Len(t, processed, 1)

	doc, err := repos.SourceDocumentRepo.GetByID(t.Context(), pendingDocs[0].ID)
	require.NoError(t, err)
	require.Equal(t, domain.SourceDocumentStatusCompleted, doc.Status)
	require.NotEmpty(t, doc.ArchivedPath)
	require.FileExists(t, doc.ArchivedPath)
	require.Contains(t, doc.ArchivedPath, filepath.Join("SyncOver", "article."))
	require.NoFileExists(t, sourcePath)

	workspace, err := repos.WorkspaceRepo.GetByID(t.Context(), doc.WorkspaceArticleID)
	require.NoError(t, err)
	require.Equal(t, domain.ArticleWorkspaceStatusRendered, workspace.Status)
	require.Equal(t, "Folder Source Title", workspace.Title)

	rewriteRun, err := repos.RewritePipelineRunRepo.GetByID(t.Context(), doc.RewriteRunID)
	require.NoError(t, err)
	require.Equal(t, domain.RewriteRunSucceeded, rewriteRun.Status)
	require.Equal(t, workspace.ID, rewriteRun.FinalDraftID)

	draft, err := repos.DraftRepo.GetByID(t.Context(), workspace.ID)
	require.NoError(t, err)
	require.Equal(t, "daily-intelligence", draft.Template)
	require.Equal(t, "Rewritten Folder Title", draft.Headline["title"])
	require.Equal(t, []string{"Folder mainline draft body."}, domain.DraftParagraphs(draft.Headline["body"]))
	require.Equal(t, "Folder digest", draft.Meta["digest"])
	require.Equal(t, "Integration Bot", draft.Meta["author"])
	require.Len(t, draft.Sections, 1)

	assets, err := repos.AssetRepo.List(t.Context(), draft.ID, "wechat")
	require.NoError(t, err)
	require.Len(t, assets, 1)
	require.NotEmpty(t, assets[0].ArtifactPath)
	require.FileExists(t, assets[0].ArtifactPath)
	require.Contains(t, assets[0].Content, "Rewritten Folder Title")
}
