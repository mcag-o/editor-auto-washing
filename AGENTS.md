# AGENTS.md

This repository is a multi-project workspace, not a single application. Identify the active subproject before running commands or editing files. Use `ArticleWashing/` for core service, API, workflow, and structured article work; use `DataCollection/` for crawling, adapters, and scheduling.

## Repository Framing

- Pick the subproject first, then follow that subproject's toolchain, tests, and style.
- Root-level assumptions are usually wrong in this workspace; verify commands against the target project.
- Prefer observed commands and file layout over generic framework habits.
- If the task touches structured article formatting, templates, validation, or publishing flow, start in `ArticleWashing/`.

## Workspace Overview

- `ArticleWashing/`: Python service-first content hub and the recommended main development directory.
- `DataCollection/`: Node.js ESM collector for hotlists, source ingestion, adapters, and scheduling.
- Root `README.md` is workspace navigation only; subproject READMEs and metadata define the real toolchains.
- Structured article capability now lives in `ArticleWashing/`.

## Rule-File Discovery Results

- No `.cursorrules` file was found.
- No `.cursor/rules/` directory was found.
- No `.github/copilot-instructions.md` file was found.

## General Rules For Agents

- Do not assume root-level build, lint, or test commands exist for the whole workspace.
- Run commands from the relevant subproject unless a command explicitly uses repo-root-relative paths.
- Edit the active project instead of introducing cross-project coupling for convenience.
- Put structured article rendering, formatting, validation, templates, and review-gated publishing work in `ArticleWashing/`.
- Preserve existing language and naming in user-facing docs, examples, and CLI output; Chinese content is normal in this repository.
- Keep automation local to the subproject unless multiple projects truly need the same behavior.
- Before broad refactors, check whether the target code is current runtime code or a compatibility surface.
- Do not invent lint, formatter, build, or dev server workflows that are not defined by observed files.

## Build / Run / Test Commands

### `ArticleWashing/` (Python)

Run from repository root unless noted otherwise.

- Install dependencies: `python3 -m pip install -r "ArticleWashing/requirements.txt"`
- Run API: `PYTHONPATH="ArticleWashing/src" uvicorn content_hub.interfaces.api.main:app --reload`
- Run standalone entry: `python3 "ArticleWashing/main.py"`
- Run all tests: `PYTHONPATH="ArticleWashing/src" python3 -m unittest discover -s "ArticleWashing/tests/content_hub" -p "test_*.py"`
- Run one test file: `PYTHONPATH="ArticleWashing/src" python3 -m unittest discover -s "ArticleWashing/tests/content_hub" -p "test_workflow.py"`
- Run one test case: `PYTHONPATH="ArticleWashing/src" python3 "ArticleWashing/tests/content_hub/test_workflow.py" WorkflowEngineTestCase.test_executes_registered_nodes_and_persists_output`
- Build package: `python3 -m build "ArticleWashing"`
- Lint: no repo-defined lint command or linter config was found.

Notes:
- Python requirement from `ArticleWashing/pyproject.toml`: `>=3.10,<3.13`
- Tests use `unittest`, not `pytest`
- Root-run commands need `PYTHONPATH="ArticleWashing/src"`
- Build backend is `hatchling`; install `build` first if `python3 -m build` is unavailable

### `DataCollection/` (Node.js + Vitest)

Run from `DataCollection/`.

- Install dependencies: `npm install`
- Run all tests: `npm test`
- Watch tests: `npm run test:watch`
- Run one test file: `npx vitest run test/core/httpClient.test.js`
- Run one test by name: `npx vitest run -t "retries retryable failures once before succeeding"`
- Run one file and one test name: `npx vitest run test/cli/run.test.js -t "parses collect-many arguments"`
- Run collector CLI: `npm run collect`
- Run smoke script: `npm run smoke`
- Build: no build script exists.
- Lint: no lint script or lint config was found.

Notes:
- Node requirement from `DataCollection/package.json`: `>=20`
- Project uses ESM via `"type": "module"`
- Some runtime flows depend on network access, cookies, proxy environment variables, or optional Playwright fallback

## Code Style Guidelines

These are observed project conventions. Match the target subproject instead of normalizing the workspace.

### Cross-Project Guidance

- Match the target subproject's conventions instead of normalizing Python and Node code into one blended style.
- Keep modules focused; both active projects mostly use small, explicit files.
- Prefer readable data flow over new abstractions.
- Preserve public interfaces unless the task explicitly changes them.
- Avoid new dependencies when the project already has a simple local pattern.

### Imports And Module Structure

- In Node projects, use ESM `import` and `export`, not CommonJS.
- In `DataCollection/`, keep explicit local `.js` suffixes in relative imports.
- In `ArticleWashing/`, follow the common import order: `from __future__ import annotations`, stdlib, third-party, local modules.
- Keep Python changes within the existing layered package boundaries and prefer named exports in JS modules.

### Formatting

- `DataCollection/` uses 2-space indentation, single quotes, and semicolons.
- `ArticleWashing/` uses 4-space indentation and standard PEP 8 style.
- Do not reformat unrelated files to force personal preferences.
- Keep spacing and wrapping close to the surrounding file; no repo-wide formatter config was found.

### Types And Data Models

- Add Python type annotations by default in `ArticleWashing/`.
- Use modern Python typing already present in the codebase, such as `list[str]`, `dict[str, Any]`, and `Path | None`.
- Use `@dataclass` for internal runtime state and domain settings in `ArticleWashing/`.
- Use Pydantic `BaseModel` at FastAPI request and response boundaries in `ArticleWashing/`.
- Keep `DataCollection/` in its current JavaScript plus runtime-validation style; do not introduce TypeScript.

### Naming Conventions

- Python: `snake_case` for functions, methods, and variables; `PascalCase` for classes.
- JavaScript: `camelCase` for functions, variables, and object properties; `PascalCase` for classes.
- Preserve `createXxxCrawler` factory naming in `DataCollection/`.
- Keep test names descriptive and behavior-oriented.
- Keep filenames aligned with local conventions instead of renaming for cross-project consistency.

### Error Handling

- Use structured, domain-appropriate errors when callers need metadata or semantics.
- In `DataCollection/`, prefer established domain errors such as `CollectorError`, `UpstreamHttpError`, `ParseError`, and `UnsupportedPlatformError` over raw `Error` where applicable.
- In `ArticleWashing/`, simple built-in exceptions like `ValueError` and `KeyError` are common unless a nearby richer exception type already exists.
- Do not swallow exceptions silently; convert them into explicit results or propagate them.

### Async, I/O, And Side Effects

- Prefer `async` and `await` in JS instead of promise chains.
- Keep filesystem and network access near service, repository, or adapter boundaries.
- Preserve file-backed persistence assumptions in `ArticleWashing/` unless the task intentionally changes storage design.
- Preserve retry and timeout behavior in `DataCollection/` when editing HTTP client or crawler code.

### Testing Conventions

- `ArticleWashing/` tests live under `ArticleWashing/tests/content_hub/` and use `unittest.TestCase`.
- `DataCollection/` tests use Vitest and should stay concise and behavior-focused.
- Run the narrowest relevant test first, then the relevant full suite.
- When fixing a bug, update the nearest existing test or add a close regression test instead of creating a disconnected test file.

## Architecture Notes That Matter During Edits

- `ArticleWashing/` is the preferred main runtime for ongoing service development.
- `ArticleWashing/` is layered: `bootstrap`, `domain`, `application`, `infrastructure`, `runtime`, `interfaces`.
- `ArticleWashing/src/content_hub/interfaces/api/main.py` is structurally sensitive because it centralizes FastAPI routes and related tests check that shape.
- Extend `content_hub` core paths before adding new behavior to compatibility shims.
- Structured article formatting, templates, examples, and CLI entry points live in `ArticleWashing/`.
- `DataCollection/` is an independent collector project, not a support folder for Python runtime code.

## Practical Workflow

- Identify the target subproject before editing or running commands.
- Re-read the nearest README, metadata file, and local tests if the task touches commands or runtime behavior.
- Run the narrowest relevant test command first.
- Make the smallest change that matches the local style and architecture boundary.
- Re-run the targeted test, then the relevant broader suite.
- Mention lint or build results only when that subproject actually defines them.

## Common Mistakes To Avoid

- Running root-level commands that only exist inside one subproject.
- Editing `DataCollection/` for structured article rendering, validation, formatting, or template work that belongs in `ArticleWashing/`.
- Adding cross-project imports or shared abstractions without a clear repository-level need.
- Normalizing Python and Node code into one artificial style.
- Claiming lint, build, or formatter workflows exist when the repository does not define them.
- Treating historical compatibility code as the main development surface without checking current boundaries first.
