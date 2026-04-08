# Task Plan: Create repository AGENTS.md

## Goal
Create or improve `/home/mcag/Documents/editor-auto-washing/AGENTS.md` with accurate build/lint/test commands, single-test workflows, and repository-specific code style guidance based on the actual codebase and any Cursor/Copilot rules.

## Current Phase
Phase 1

## Phases

### Phase 1: Requirements & Discovery
- [x] Understand user intent
- [x] Identify constraints and requirements
- [x] Document findings in findings.md
- **Status:** complete

### Phase 2: Planning & Structure
- [x] Define technical approach
- [x] Define target AGENTS.md structure and coverage
- [x] Document decisions with rationale
- **Status:** complete

### Phase 3: Repository Analysis
- [x] Identify project roots and active subprojects
- [x] Locate commands for build/lint/test/single-test execution
- [x] Infer coding conventions from config and source files
- **Status:** complete

### Phase 4: Drafting AGENTS.md
- [x] Draft repository overview and workflow guidance
- [x] Draft commands and single-test instructions
- [x] Draft code style and error-handling guidance
- **Status:** complete

### Phase 5: Verification & Delivery
- [x] Verify AGENTS.md against repository files
- [x] Ensure Cursor/Copilot guidance is included if present
- [x] Deliver completed file to user
- **Status:** complete

## Key Questions
1. How should the root AGENTS.md balance three active subprojects without overstating repo-wide standards?
2. Which commands are explicitly encoded in project config versus inferred from README/source layout?
3. How much architectural context is useful before the document becomes too long for agents?

## Decisions Made
| Decision | Rationale |
|----------|-----------|
| Create AGENTS.md at repository root | User explicitly requested a root-level agent guide for this repository |
| Structure AGENTS.md as a workspace guide with subproject sections | Repository contains multiple active codebases with different languages and toolchains |
| Prefer explicit commands from `package.json`, `pyproject.toml`, and READMEs; label inferred conventions as conventions, not rules | Prevents inventing tooling that the repo does not actually configure |

## Errors Encountered
| Error | Attempt | Resolution |
|-------|---------|------------|
|       | 1       |            |

## Notes
- Re-read this plan before major decisions.
- Prefer direct evidence from config files and tests over assumptions.
- Keep AGENTS.md concise but comprehensive, around 150 lines as requested.
- Final AGENTS.md created at repo root and verified at 161 lines.
