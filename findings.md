# Findings & Decisions

## Requirements
- Create or improve `/home/mcag/Documents/editor-auto-washing/AGENTS.md`.
- Include build, lint, and test commands, with special attention to running a single test.
- Include repository-specific code style guidance: imports, formatting, types, naming, error handling, and related conventions.
- Include Cursor rules from `.cursor/rules/` or `.cursorrules` if present.
- Include Copilot rules from `.github/copilot-instructions.md` if present.
- Keep the file about 150 lines long.
- Deeply explore the project before writing.

## Research Findings
- Root `README.md` is empty.
- Repository root is multi-project rather than a single app; the active executable/code-bearing subprojects are `DataCollection` and `ArticleWashing`.
- No existing root `AGENTS.md`, `.cursorrules`, `.cursor/rules/`, or `.github/copilot-instructions.md` were found.
- `DataCollection` is a Node 20+ ESM project using npm and Vitest. `package.json` defines `test`, `test:watch`, `collect`, and `smoke`, but no `build` or `lint` scripts.
- `DataCollection` single-test workflow is via Vitest CLI, e.g. `npx vitest run test/core/httpClient.test.js` or `npx vitest run -t "test name"`.
- `DataCollection` style conventions from source: explicit `.js` local imports, single quotes, semicolons, 2-space indentation, `camelCase`, uppercase error codes, `async`/`await`, and typed domain errors.
- `ArticleWashing` is a Python `src`-layout package built with Hatchling. README documents `unittest` discovery; no repo-defined lint tool was found.
- `ArticleWashing` single-test workflow is `python3 -m unittest discover -p "test_file.py"` for one file, or running the test file directly with `ClassName.test_method` for one case.
- `ArticleWashing` style conventions from source: `from __future__ import annotations`, typed-by-default modern Python, dataclasses for internal/domain models, Pydantic models at FastAPI boundaries, `snake_case` functions, `PascalCase` classes, and lightweight exception handling using built-in exceptions.
- `结构化文章` was a separate Node ESM structured-article project; its core templates, rendering, validation, pipeline, and CLI entry have now been merged into `ArticleWashing`.
- Recent commits use short informal messages, often in Chinese or terse labels, which suggests a lightweight repo workflow but does not materially affect AGENTS guidance.

## Technical Decisions
| Decision | Rationale |
|----------|-----------|
| Treat the repo as a multi-project workspace in AGENTS.md | There is no single dominant application; agents need subproject-specific command guidance |
| Center code-style guidance on the strongest recurring conventions across the three active subprojects | This avoids inventing nonexistent repo-wide formatter/linter rules while still giving agents actionable defaults |
| Treat missing Cursor/Copilot rule files as absent unless later discovery finds them | The user only asked to include them if they exist |

## Issues Encountered
| Issue | Resolution |
|-------|------------|
| Root repository appears heterogeneous and partially archival | Focus analysis on executable subprojects with package/test configuration |

## Resources
- `/home/mcag/Documents/editor-auto-washing/DataCollection/package.json`
- `/home/mcag/Documents/editor-auto-washing/DataCollection/src/`
- `/home/mcag/Documents/editor-auto-washing/DataCollection/test/`
- `/home/mcag/Documents/editor-auto-washing/ArticleWashing/pyproject.toml`
- `/home/mcag/Documents/editor-auto-washing/ArticleWashing/README.md`
- `/home/mcag/Documents/editor-auto-washing/ArticleWashing/knowledge/structured_templates/`
- `/home/mcag/Documents/editor-auto-washing/ArticleWashing/examples/article.sample.json`

## Visual/Browser Findings
- None.
