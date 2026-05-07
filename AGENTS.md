# AGENTS.md

This repository is a post-migration workspace with one active Go codebase at the repository root and archived reference projects under `Archive/`. Treat root Go code as the live runtime unless the task explicitly targets historical parity or migration research.

## Repository Shape

- Active project: repo root Go module `content-hub`
- Only active documented operator surface: browser-based Chinese React + Vite web control plane on `8123` in the root Go runtime
- Only active documented operator intake path: browser upload / paste workflow in the root Go runtime
- Workflow/template management is a real browser-backed capability on that same surface
- Default automated result path stops at draft + render; review/publish are optional later manual steps
- Archived reference projects:
  - `Archive/ArticleWashing/` - legacy Python content-hub implementation
  - `Archive/DataCollection/` - legacy Node.js collector implementation
- Root `README.md` is the current high-level product/runtime document.
- Many docs still discuss migration state. Verify whether a file is describing current behavior or historical reference behavior before editing it.

## Rule File Discovery

- Root `AGENTS.md` exists and is the primary instruction file for this repository.
- No `.cursorrules` file was found.
- No `.cursor/rules/` directory was found.
- No `.github/copilot-instructions.md` file was found.

## Core Agent Rules

- Assume repo root is the default working project unless the task explicitly points into `Archive/`.
- Do not treat archived docs as source of truth for current commands, APIs, or architecture.
- Treat the React + Vite web control plane on port `8123` as the only active documented operator surface for the root Go runtime, and describe its operator UI as Chinese-first unless a task explicitly targets localization details. Treat browser upload / paste as the only supported, documented, and operator-facing active-runtime intake path, and present workflow/template management as a real browser-first, componentized capability on that same surface. Treat Material UI and React Flow as the chosen frontend foundations when describing or extending that UI unless the task explicitly changes frontend architecture. Treat business configuration as DB-backed runtime state, not file-first operator setup. Draft + render are the default automated result path. Review/publish are optional later manual steps, not part of the default automated chain. Folder-intake remains backend/internal compatibility only unless a task explicitly targets it. Do not present legacy static shell, RSS/collector/ingestion HTTP, or CLI surfaces as current supported operator entrypoints unless the task explicitly targets historical behavior or development/debug support.
- Preserve Chinese user-facing documentation where it already exists; bilingual docs are normal here.
- Prefer minimal, local edits over wide migration-era cleanup unless the task explicitly asks for broader normalization.
- When touching path-sensitive docs or metadata, check whether the target should reference root runtime code or archived legacy code.
- Run the narrowest relevant test first, then a broader suite for the area you changed.
- Do not invent lint, formatter, or build workflows that are not defined by observed files.

## Build / Run / Test Commands

### Active Root Project (Go)

Run from repository root.

- Install dependencies: `go mod download`
- Build server binary: `go build ./cmd/server`
- Build CLI binary: `go build ./cmd/cli`
- Run server / web control plane: `go run ./cmd/server`
- Run CLI: `go run ./cmd/cli`
- Run TUI: `go run ./cmd/tui --api http://localhost:8123`
- Web control plane service verification: `go test ./service -run 'TestSource|TestFolder|TestRewrite|TestBuildWebControlRuntime|TestWorkflowTemplate|TestTemplateDefinition|TestWebControlPlaneService'`
- Web control plane transport verification: `go test ./transport/http/... -run 'TestAPI|TestAdminFrontend|TestRewrite'`
- Web control plane integration verification: `go test ./integration -run 'TestWebControlPlanePasteToRenderedResult|TestWebControlPlaneUploadToRenderedResultWithWorkflowTemplate|TestReactControlPlanePasteToRenderedResultWithWorkflowTemplate|TestRewritePipelineMainlineMaterializesDraft'`
- Run all tests: `go test ./...`
- Run all tests with race and coverage: `make test`
- Run one package: `go test ./service`
- Run one exact test: `go test ./service -run TestRewriteOrchestratorRunsPipelineAndCreatesDraft`
- Run rewrite service tests: `go test ./service -run TestRewrite`
- Run rewrite orchestrator test: `go test ./service -run TestRewriteOrchestratorRunsPipelineAndCreatesDraft`
- Run one integration test: `go test ./integration -run TestFolderIntakeMainlineCreatesRenderedOutput`
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

### Archived Python Reference (`Archive/ArticleWashing/`)

Use only when validating migration parity, reading historical API shape, or updating archive docs.

- Install dependencies: `python3 -m pip install -r "Archive/ArticleWashing/requirements.txt"`
- Run API: `PYTHONPATH="Archive/ArticleWashing/src" uvicorn content_hub.interfaces.api.main:app --reload`
- Run standalone entry: `python3 "Archive/ArticleWashing/main.py"`
- Run all tests: `PYTHONPATH="Archive/ArticleWashing/src" python3 -m unittest discover -s "Archive/ArticleWashing/tests/content_hub" -p "test_*.py"`
- Run one test file: `PYTHONPATH="Archive/ArticleWashing/src" python3 -m unittest discover -s "Archive/ArticleWashing/tests/content_hub" -p "test_workflow.py"`
- Run one exact test case: `PYTHONPATH="Archive/ArticleWashing/src" python3 -m unittest "Archive.ArticleWashing.tests.content_hub.test_workflow.WorkflowEngineTestCase.test_executes_registered_nodes_and_persists_output"`
- Build package: `python3 -m build "Archive/ArticleWashing"`
- Lint: no repo-defined lint command or linter config was found.

Notes:
- Python requirement from `Archive/ArticleWashing/pyproject.toml`: `>=3.10,<3.13`
- Tests use `unittest`, not `pytest`.
- Build backend is `hatchling`; install `build` first if `python3 -m build` is unavailable.

### Archived Node Collector Reference (`Archive/DataCollection/`)

Use only for migration comparison, fixtures, or legacy collector behavior.

- Install dependencies: `npm install`
- Run all tests: `npm test`
- Watch tests: `npm run test:watch`
- Run one test file: `npx vitest run test/core/httpClient.test.js`
- Run one test by name: `npx vitest run -t "retries retryable failures once before succeeding"`
- Run one file and one test name: `npx vitest run test/cli/run.test.js -t "parses collect-many arguments"`
- Run collector CLI: `npm run collect`
- List sources: `npm run sources:list`
- Check sources: `npm run sources:check`
- Run ops CLI: `npm run ops`
- Run smoke script: `npm run smoke`
- Build: no build script exists.
- Lint: no lint script or lint config was found.

Notes:
- Run Node commands from `Archive/DataCollection/`.
- Node requirement from `Archive/DataCollection/package.json`: `>=20`
- Project uses ESM via `"type": "module"`.

## Code Style Guidelines

Match the active area you are editing. Do not normalize root Go code and archived Python/Node code into one artificial style.

### Architecture And Boundaries

- Root Go code follows explicit package boundaries such as `domain`, `infra`, `service`, `collector`, `transport`, and `cmd`.
- Keep transport concerns in `transport/`, persistence and adapters in `infra/`, orchestration in `service/`, and core models/errors in `domain/`.
- `service/rewrite_*` owns rewrite orchestration and stage execution.
- `infra/llm/` owns prompt rendering, structured decode, and provider adapters.
- `collector/` stops at imported source article creation and bridge ingestion; it does not own rewrite profile selection or rewrite execution.
- Avoid introducing cross-package helper layers unless existing boundaries clearly require them.
- Preserve public interfaces unless the task explicitly changes behavior across layers.

### Imports And Module Structure

- Go: group standard library imports first, then local module imports, then third-party imports as already formatted by `gofmt`.
- Go: use short lowercase package names; import paths are module-relative like `content-hub/service`.
- Python archive: prefer `from __future__ import annotations`, then stdlib, third-party, and local imports.
- Node archive: use ESM `import` and `export`; keep explicit local `.js` suffixes in relative imports.

### Formatting

- Go: follow `gofmt` exactly; keep functions compact and structurally simple.
- Python archive: use 4-space indentation and normal PEP 8 spacing.
- JavaScript archive: use 2-space indentation, single quotes, and semicolons.
- Keep wrapping and whitespace close to surrounding code; do not reformat unrelated files.

### Types And Data Models

- Go: prefer explicit structs and narrow interfaces; return concrete structs or pointers when the surrounding package already does so.
- Go: use typed domain errors such as `domain.AppError` when callers need semantic handling or HTTP mapping.
- Go: thread `context.Context` through service, collector, and transport paths that already expect it.
- Python archive: add type annotations by default and prefer modern annotations like `list[str]` and `Path | None`.
- Python archive: use `@dataclass` for internal state and Pydantic models at FastAPI boundaries.
- JavaScript archive: keep the current JavaScript-plus-runtime-validation style; do not introduce TypeScript unless explicitly requested.

### Naming Conventions

- Go: exported identifiers use `PascalCase`; unexported identifiers use `camelCase`; package names stay lowercase.
- Preserve constructor and factory naming such as `NewServer`, `NewWorkflowEngine`, or `NewXxxHandler`.
- Python archive: functions and variables use `snake_case`; classes use `PascalCase`.
- JavaScript archive: functions and variables use `camelCase`; classes use `PascalCase`.
- Keep test names behavior-oriented and aligned with the local framework.

### Error Handling

- Do not swallow errors silently; propagate them or convert them into explicit domain results.
- In Go, wrap low-level failures with context, for example `fmt.Errorf("load workspace config: %w", err)`.
- Use `domain.AppError` for transport-facing validation, not-found, conflict, external, and internal failures when callers need stable HTTP semantics.
- In handlers, prefer centralized error mapping such as `handlers.HandleError` over ad hoc JSON error responses when the path already supports it.
- Python archive commonly uses built-in exceptions such as `ValueError` and `KeyError` unless a nearby richer exception type already exists.
- JavaScript archive should prefer established domain errors over raw `Error` where the legacy code already defines them.

### Testing Conventions

- Root Go tests are colocated with packages, plus integration coverage under `integration/`.
- When fixing a bug, update the nearest existing test or add a close regression test instead of creating a disconnected test file.
- Prefer fixture-driven tests for collector parsing, normalization, and migration-reference behavior.
- Prefer `go test ./service -run TestRewrite...` for rewrite service verification before broader `go test ./...` runs.
- Prefer `go test ./integration -run 'TestWebControlPlanePasteToRenderedResult|TestWebControlPlaneUploadToRenderedResultWithWorkflowTemplate|TestRewritePipelineMainlineMaterializesDraft'` when validating the default automated mainline.
- Start with a package-level or exact-test run before `go test ./...`.
- Archived Python tests use `unittest.TestCase` under `Archive/ArticleWashing/tests/content_hub/`.
- Archived Node tests use Vitest under `Archive/DataCollection/test/`.

## Migration-Aware Editing Guidance

- Expect stale references to old top-level paths such as `ArticleWashing-Go/`, `ArticleWashing/`, and `DataCollection/` in docs and metadata.
- Some references to `Archive/DataCollection/src/platforms/*.js` are intentional migration-reference fields, not broken links.
- Archived documentation may intentionally preserve historical commands; do not "modernize" archive docs unless the task is archive maintenance.
- If you update path-sensitive docs in root, verify whether examples should point to root runtime files or to `Archive/...` as historical references.
- Check `.gitignore`, docs, and config metadata carefully after any path-structure changes.

## Practical Workflow For Agents

- Confirm whether the task targets active root Go code or archived reference material.
- Read the nearest README, config, and tests for the target area before editing.
- Make the smallest change that fits the existing architecture.
- When describing operator workflows, default to the browser-based Chinese React + Vite web control plane on `8123`; describe the CLI as development/debug support unless the task explicitly centers CLI behavior. When describing frontend foundations for workflow/template management, keep Material UI and React Flow as the default stack unless the task explicitly changes that architecture.
- Re-run the most specific relevant test, then broaden as needed.
- Mention build or lint results only when that workflow actually exists in the target area.
- If docs claim migration completeness, operational exposure, or parity, verify the claim against code before editing it.

## Common Mistakes To Avoid

- Treating archived projects as the live runtime.
- Running legacy commands from repo root when they only work inside `Archive/` subprojects.
- Rewriting intentional migration-reference paths as if they were accidental leftovers.
- Claiming lint, formatter, or build workflows exist when the repository does not define them.
- Mixing archive cleanup with active-runtime feature work without an explicit request.
