# Swimlane Studio implementation plan

## Product
A local workspace where a ticket is the prompt and moving it into In progress starts an agent run. The board and right inspector stay visible together. A calm paper-and-ink interface uses Vue 3, TypeScript, Lucide icons and Reka UI accessible dialogs.

## Phase 1 — runnable prototype (completed)
1. Describe acceptance criteria in Gherkin and write failing tests.
2. Build an independent FastAPI API with OpenAPI, SQLite persistence, validated ticket/image inputs, immutable queued prompts and a FIFO worker.
3. Build a Vue board with draggable tickets, search, status filters, create/edit, keyboard-accessible run action, agent selection and queue visibility.
4. Add pasted image thumbnails with a larger preview, agent response and clickable unified file diffs.
5. Verify Vitest coverage, Python integration tests, Chrome-only Playwright UI journeys, production build and per-function CRAP checks.

The demo adapter simulates work and returns illustrative diffs. It never edits a repository or calls a paid model. Codex and Claude choices are visible but disabled until adapters are implemented. Tickets and pasted images are stored locally in SQLite. One backend process owns one worker; use one Uvicorn worker.

## Phase 2 — real coding agents (implemented)
A text-generation API alone is insufficient for a coding workspace. An agent runner needs repository access, file tools, bounded execution, test commands and a reviewable working copy. Implement Codex/Claude adapters behind the existing Agent protocol. Add server-only credentials, workspace selection, isolated Git worktrees, execution permissions, cancellation/timeouts and actual Git diffs. Start with one provider and one repository. Stream run events over SSE, preserve run history and support retries explicitly.

## Ticket grouping — implemented
Named, coloured groups persist in SQLite and follow individual tickets across lanes. Groups support per-lane collapse, creating a ticket within a group, rename/recolour and removal that retains tickets. Group metadata can change on locked tickets without modifying run snapshots. Ungrouped tickets remain supported. Run group snapshots saved Todo tickets in board order and queues durable predecessor dependencies atomically. Real attempts require enabled configured tests; failed or cancelled predecessors stop followers. Worktrees remain isolated per ticket.

## Project overview — implemented
The current local project has a persisted title and description, edited through the board header. The status filter shares the group menu surface and supports keyboard navigation. Multiple projects remain future work.

## Phase 3 — shared application
Add authentication, project permissions, database migrations, a separate durable worker, object storage for attachments and concurrent run isolation. Add merge/reject workflows after user review. Never mark a failed run Done.

## Architecture
`frontend/` → HTTP `/api` → `backend/app/` → SQLite + FIFO queue → Agent adapter.
Vite proxies `/api` during development. FastAPI exposes `/docs` and `/openapi.json`. Replayable SSE refreshes run state, with polling as a fallback. Real attempts retain detached Git worktrees, run history and actual diffs. Both services are separately runnable; no backend source is bundled into the client.

## Quality
CRAP = complexity² × (1 − coverage)³ + complexity. The gates measure each authored function using cyclomatic complexity and its covered executable lines; all scores must be strictly under 10. Frontend coverage uses Vitest V8; Python uses pytest-cov and Radon. UI journeys run against real FastAPI in Google Chrome only. Scores cover source functions, not Gherkin scenarios or test helpers. Local-demo limitations are documented instead of implying production readiness.

## Phase 2 delivery notes
Codex and Claude CLI adapters are implemented with API-key authentication configured only on the server. Repository registration is restricted to configured roots. Read-only/edit permissions are snapshotted per attempt; test presets require explicit opt-in. No merge/reject automation or dependency installation is included. Offline tests validate provider protocols using fake executable processes; live provider calls require user credentials. See README for exact configuration and execution boundaries.
