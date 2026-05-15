# AGENTS.md

This repository is the active `content-hub` Go workspace rooted at the repository root.

## Repository Shape

- Active project: repo root Go module `content-hub`
- Only active documented operator surface: browser-based Chinese React + Vite web control plane on `8123`
- Only active documented operator intake path: browser upload / paste workflow
- Workflow/template management is a real browser-backed capability on that same surface
- Default automated result path stops at draft + render; review/publish are optional later manual steps
- Root `README.md` is the current high-level product/runtime document

## Rule File Discovery

- Root `AGENTS.md` exists and is the primary instruction file for this repository.
- No `.cursorrules` file was found.
- No `.cursor/rules/` directory was found.
- No `.github/copilot-instructions.md` file was found.

## Core Agent Rules

- Assume repo root is the default working project unless the task explicitly points elsewhere.
- Treat the React + Vite web control plane on port `8123` as the only active documented operator surface for the root Go runtime.
- Describe the operator UI as Chinese-first unless a task explicitly targets localization details.
- Treat browser upload / paste as the only supported, documented, and operator-facing intake path.
- Present workflow/template management as a browser-first, componentized capability on that same surface.
- Treat Material UI and React Flow as the chosen frontend foundations unless the task explicitly changes frontend architecture.
- Treat business configuration as DB-backed runtime state, not file-first operator setup.
- Draft + render are the default automated result path.
- Review/publish are optional later manual steps, not part of the default automated chain.
- Folder-intake remains backend/internal compatibility only unless a task explicitly targets it.
- Do not present static shell or RSS/collector/ingestion HTTP surfaces as current supported operator entrypoints.
- Preserve Chinese user-facing documentation where it already exists; bilingual docs are normal here.
- Prefer minimal, local edits over broad cleanup unless the task explicitly asks for broader normalization.
- Run the narrowest relevant test first, then broaden as needed.
- Do not invent lint, formatter, or build workflows that are not defined by observed files.

## Build / Run / Test Commands

### Active Root Project (Go)

Run from repository root.

- Install dependencies: `go mod download`
- Build server binary: `go build ./cmd/server`
- Run server / web control plane: `go run ./cmd/server`
- Web control plane service verification: `go test ./service -run 'TestSource|TestFolder|TestRewrite|TestBuildWebControlRuntime|TestWorkflowTemplate|TestTemplateDefinition|TestWebControlPlaneService'`
- Web control plane transport verification: `go test ./transport/http/... -run 'TestAPI|TestAdminFrontend|TestRewrite'`
- Web control plane integration verification: `go test ./integration -run 'TestWebControlPlanePasteToRenderedResult|TestWebControlPlaneUploadToRenderedResultWithWorkflowTemplate|TestReactControlPlanePasteToRenderedResultWithWorkflowTemplate|TestRewritePipelineMainlineMaterializesDraft'`
- Run all tests: `go test ./...`
- Run all tests with race and coverage: `make test`
- Run one package: `go test ./service`
- Run one exact test: `go test ./service -run TestRewriteOrchestratorRunsPipelineAndCreatesDraft`
- Run rewrite service tests: `go test ./service -run TestRewrite`
- Run rewrite integration test: `go test ./integration -run TestRewritePipelineMainlineMaterializesDraft`
- Run verbose package test: `go test -v ./transport/http/handlers -run 'TestFolder|TestRewrite'`
- Build via Makefile: `make build`
- Run built server: `make run`
- Clean build artifacts: `make clean`
- Lint: no repo-defined lint script or GolangCI config was found.

Notes:

- Go version from `go.mod`: `1.25.0`
- SQLite uses `github.com/mattn/go-sqlite3`, so CGO and a local C toolchain are required.
- `Makefile` builds `bin/server` with `CGO_ENABLED=1` and runs tests with `-race` and coverage.
- Many collector and integration tests rely on fixtures under `testdata/`.

## Code Style Guidelines

### Architecture And Boundaries

- Root Go code follows explicit package boundaries such as `domain`, `infra`, `service`, `collector`, `transport`, and `cmd`.
- Keep transport concerns in `transport/`, persistence and adapters in `infra/`, orchestration in `service/`, and core models/errors in `domain/`.
- `service/rewrite_*` owns rewrite orchestration and stage execution.
- `infra/llm/` owns prompt rendering, structured decode, and provider adapters.
- `collector/` stops at imported source article creation and bridge ingestion; it does not own rewrite profile selection or rewrite execution.
- Avoid introducing cross-package helper layers unless existing boundaries clearly require them.
- Preserve public interfaces unless the task explicitly changes behavior across layers.

### Imports And Module Structure

- Go: group standard library imports first, then local module imports, then third-party imports as formatted by `gofmt`.
- Go: use short lowercase package names; import paths are module-relative like `content-hub/service`.

### Formatting

- Go: follow `gofmt` exactly; keep functions compact and structurally simple.
- Keep wrapping and whitespace close to surrounding code; do not reformat unrelated files.

### Types And Data Models

- Go: prefer explicit structs and narrow interfaces; return concrete structs or pointers when the surrounding package already does so.
- Go: use typed domain errors such as `domain.AppError` when callers need semantic handling or HTTP mapping.
- Go: thread `context.Context` through service, collector, and transport paths that already expect it.

### Naming Conventions

- Go: exported identifiers use `PascalCase`; unexported identifiers use `camelCase`; package names stay lowercase.
- Preserve constructor and factory naming such as `NewServer`, `NewWorkflowEngine`, or `NewXxxHandler`.

### Error Handling

- Do not swallow errors silently; propagate them or convert them into explicit domain results.
- In Go, wrap low-level failures with context, for example `fmt.Errorf("load workspace config: %w", err)`.
- Use `domain.AppError` for transport-facing validation, not-found, conflict, external, and internal failures when callers need stable HTTP semantics.
- In handlers, prefer centralized error mapping such as `handlers.HandleError` over ad hoc JSON error responses when the path already supports it.

### Testing Conventions

- Root Go tests are colocated with packages, plus integration coverage under `integration/`.
- When fixing a bug, update the nearest existing test or add a close regression test instead of creating a disconnected test file.
- Prefer fixture-driven tests for collector parsing and normalization behavior.
- Prefer `go test ./service -run TestRewrite...` for rewrite service verification before broader `go test ./...` runs.
- Prefer `go test ./integration -run 'TestWebControlPlanePasteToRenderedResult|TestWebControlPlaneUploadToRenderedResultWithWorkflowTemplate|TestRewritePipelineMainlineMaterializesDraft'` when validating the default automated mainline.
- Start with a package-level or exact-test run before `go test ./...`.

## Practical Workflow For Agents

- Read the nearest README, config, and tests for the target area before editing.
- Make the smallest change that fits the existing architecture.
- When describing operator workflows, default to the browser-based Chinese React + Vite web control plane on `8123`.
- When describing frontend foundations for workflow/template management, keep Material UI and React Flow as the default stack unless the task explicitly changes that architecture.
- Re-run the most specific relevant test, then broaden as needed.
- Mention build or lint results only when that workflow actually exists in the target area.

## Common Mistakes To Avoid

- Treating compatibility-only surfaces as the live operator workflow.
- Presenting folder-intake as the default operator entrypoint.
- Claiming lint, formatter, or build workflows exist when the repository does not define them.
- Mixing unrelated cleanup with active-runtime feature work without an explicit request.
