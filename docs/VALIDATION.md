# Go backend validation — 7 October 2026

The frontend now defaults to the standalone Go backend on localhost:8080. The Python backend and its source remain unchanged. Vite's live `/api/health` proxy returned `backend: go`, and Go's categorised Swagger page was verified at `/docs`.

| Check | Result |
| --- | --- |
| Go API/storage/worker/process/adapter tests with race detection | 22 passed |
| Go API statement coverage | 86.0% |
| Go API authored-function CRAP | Maximum 9.788 across 154 functions, all strictly under 10 |
| `go vet ./...` and server build | Passed |
| Frontend TypeScript and production build | Passed |
| Vitest unit/component tests | 70 passed |
| Chrome UI tests against isolated Go/SQLite service | All 14 passed; final cancellation/group/drag changes rechecked with 3 passing journeys |
| Preserved Python tests | 49 passed |
| SQLite import | 2 projects, 17 tickets, 1 group, 14 runs and 42 events retained |

Go tests exercise actual temporary Git worktrees, committed/staged/untracked/binary/Unicode diffs, source isolation, fake Codex/Claude executables, image payloads, secret redaction and test environment filtering, successful and failing Vitest/Pytest presets, output limits, timeout and process-group cancellation, retained partial diffs returned by cancellation, FIFO ordering, group stop gates, snapshots/history, SSE replay, local-origin/Host checks, bulk deletion/isolation/restore, SQLite import, legacy migration, restart recovery and seed idempotence. API routes are checked against every operation in the embedded OpenAPI contract. Paid provider authentication/model calls were not performed.

The database import uses SQLite's online backup API, including committed WAL content. Source data is read-only; the Go database is separate. All existing JSON payloads and history were compared after import; one ticket's JSON serialisation differed while its field values were identical. Future edits are independent between backends. An existing target is not overwritten on subsequent imports.

Go CRAP uses AST cyclomatic complexity and statement coverage in `backend-go/internal/studio`. The report excludes tests and command-line startup/measurement utilities, consistent with the existing Python app-only and frontend source gates. Gherkin scenarios are in `docs/backends.feature`. The Go OpenAPI snapshot is embedded from the Python contract, with a Go-specific title; HTTP handler registration is checked in tests. The old Python schema stays available independently.

## Previous Python/frontend validation


Project switcher: the breadcrumb opens an accessible dropdown of active projects, highlights and checks the selected project, opens the chosen board, and offers Manage projects. Unit and Chrome tests verify switching, current selection, Escape dismissal and unsaved-draft keep/discard behavior. The 3 relevant Chrome project journeys passed; the frontend build and CRAP gate passed.

Ticket multiselection: Chrome verifies inclusive ranges across grouped and ungrouped tickets, native Mac Ctrl-click and Cmd-click toggles, bottom action bar, cancel/confirm deletion and restore. Vitest covers reversed/additive ranges, missing anchors, selection retention and failed deletion preserving unsaved edits. Python checks that bulk deletion is atomic, project-scoped, recoverable and rejected as a whole for active or missing tickets. Backend max CRAP is 8; frontend max CRAP is 9. Existing single-ticket deletion and execution contracts remain available.

Projects page and screenshot previews: Workspace and the logo open a full page, and opening a project returns to its board. Chrome verifies screenshot upload, preview cards, edit preservation and reload/delete/restore persistence. Vitest also verifies pasted screenshots, removal and invalid upload feedback. Python validates screenshot payloads and both project API contracts. Existing projects use an empty screenshot by default, with no database schema change.

Project and ticket management: Chrome verifies project creation/editing/switching, separate boards, persisted selection, recoverable project/ticket deletion, disabled deletion during a run, and completed response/history restoration. Unit tests cover unsaved-draft discard controls and stale responses during project switches. Python checks migration of the singleton project, isolation/cross-project group validation, deletion conflicts, final-project deletion, restores, and restoring a ticket after its group is removed. A copy of the existing local database was migrated and verified with all 13 tickets and 2 groups retained; a pre-upgrade SQLite backup was saved before restarting the local API.

Group dragging is verified in Chrome: ungrouped to grouped, between groups including collapsed headers, back out by dropping on the current column outside a group, and completed-ticket regrouping with reload persistence. The dedicated removal target has been removed. Vitest checks column drops ungroup Todo, running and completed tickets without starting a run, group drop bubbling never starts a run, unknown drag IDs are ignored, and another selected ticket's unsaved draft stays intact. No backend execution behavior changed.

Queue refresh regression: an open but stalled event stream no longer leaves completed tickets shown as Queued. A two-second reconciliation refresh updates the board and selected run history regardless of stream connection state. A Chrome test suppresses all live events while two demo tickets execute sequentially, then verifies both reach Done and the selected response appears without a page reload. Vitest verifies the fallback timer is removed when the connection is stopped. The existing local board was also checked: SW-008 and SW-009 now appear completed in Done.

Verified locally on 6 October 2026. Project details, the status menu and all existing Chrome journeys were verified against the real local API.

| Check | Result |
| --- | --- |
| TypeScript check and Vite production build | Passed |
| Vitest unit/component tests | 70 passed |
| Frontend line coverage | 95.56% |
| Frontend branch coverage | 91.58% |
| Frontend authored-function CRAP | Maximum 9.000, all strictly under 10 |
| Python API/worker/provider/process tests | 49 passed |
| Backend line coverage | 97.40% |
| Backend function CRAP | Maximum 8.000, all strictly under 10 |
| Playwright, Google Chrome only | Previous full suite: 12 passed; column-drop change: 4 relevant journeys passed; projects page: 2 relevant journeys passed; multiselection and regressions: 4 journeys passed |

Chrome journeys cover creation, editing, reload persistence, run completion, prompt locking, file diff dialogs, dragging tickets, sequential queuing, pasted image previews/removal, search/status filters, a 390px overflow check, cancellation/retry with previous-run diff review, repository settings/path validation, and grouped ticket creation, collapse/expand, lane movement, rename, reload, reassignment removal, and saved custom colours with readable header contrast, and Run group with sequential completion simulated test labels, waiting/running progress bars and completion ticks, project title/description persistence, keyboard menu dismissal/focus and search padding. The Chrome suite uses the real FastAPI service with real SQLite persistence and demo runs; no API request fixtures or paid model calls are used.

Python tests cover real temporary Git worktrees; staged, committed, untracked, binary and Unicode-path diffs; source-checkout isolation; configuration/credential handling; actual supervised fake Codex/Claude processes and their output protocols; image delivery; failure/incomplete completion detection; timeout/cancellation; trusted test presets; history/retry; restart recovery; SSE replay; local-origin boundaries; responsive cancellation during Git preparation, group validation/persistence, compatibility with old tickets, and grouping changes that preserve completed results and run snapshots, atomic group validation, real test-gated successors, failure/cancellation stops, and durable sequence dependencies, and project field validation/restart persistence.

Actual paid provider authentication and model execution are not verified. Run with configured server API keys and a registered repository for live validation. Tests/dependencies, merge/reject and multi-user deployment are distinct concerns; the README states the current execution boundaries.

The Gherkin specification describes acceptance behavior; Python, Vitest and Playwright implement checks without a separate Cucumber runner. CRAP reports apply to authored production functions in backend/app and frontend/src. OpenAPI is regenerated from FastAPI. The offline provider tests use fake executables, not live model responses.
