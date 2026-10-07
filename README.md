# Swimlane Studio

Build through a prompt board: create a ticket, paste images and move it into **Doing**. A FIFO worker executes one ticket at a time. Review responses, actual file diffs and previous attempts in the right-hand pane.

Frontend: Vue 3, TypeScript, Vite, Reka UI and Lucide. Default backend: Go (`net/http`), OpenAPI, SQLite and supervised agent CLI processes. The original Python/FastAPI backend remains in `backend/`. Keep the services separate and bind to localhost; this is a single-user local app.

## Run

Prerequisites: Node 22.12+, Go 1.26+, a C compiler (for the SQLite driver), Git and Google Chrome for UI tests. Python 3.11+ is needed only for the original backend or Pytest repository tests.

Go backend (default):

```sh
cd backend-go
go run ./cmd/server -import ../backend/swimlane.sqlite3
```

The optional `-import` flag makes a consistent SQLite backup of the Python database **only when the Go database does not exist**. It preserves projects, screenshots, groups, tickets, deletion markers, run history and events. The source is opened read-only and never overwritten. Go uses `backend-go/swimlane.sqlite3`; Python keeps `backend/swimlane.sqlite3`. Subsequent changes are independent; there is no ongoing synchronisation. For a fresh Go installation with no Python database, omit `-import`.

Original Python backend (still supported):

```sh
cd backend
python3 -m venv .venv
.venv/bin/pip install -r requirements.lock.txt
.venv/bin/python start.py
```

Frontend, in another terminal:

```sh
cd frontend
npm ci
npm run dev
```

Open <http://127.0.0.1:5173>. Go Swagger: <http://127.0.0.1:8080/docs> and [Go schema snapshot](docs/go-openapi.json). Python Swagger remains <http://127.0.0.1:8000/docs>. Vite proxies `/api` to the backend. Vite defaults to Go on port 8080. To use Python instead, start Vite with `SWIMLANE_API_URL=http://127.0.0.1:8000 npm run dev`.

## Enable real agents

Demo tickets remain simulated. Codex and Claude tickets run real coding CLIs with **server-only API credentials**. This version does not import desktop subscription login sessions. Configure one provider to begin.

1. Install a current Codex CLI supporting `exec --ignore-user-config --ignore-rules --json`, or Claude Code **2.1.248+** with `--bare --restricted`.
2. Copy `backend-go/.env.example` to `backend-go/.env` (or `backend/.env.example` to `backend/.env` for Python) and fill the chosen provider key and repository roots. The file is ignored by Git. Uvicorn does not load this file automatically; start it with the helper below.
3. In **Repositories**, register a local Git repository root with at least one commit. Optionally set a test preset and relative test directory.
4. Create a ticket, select the provider and repository, and choose read-only or allow edits. Enable repository tests only for trusted code.
5. Run or drag the saved ticket to Doing. Review actual changes and run history when it finishes.

```sh
cd backend-go
go run ./cmd/server
```

Environment variables are also accepted directly. `CODEX_API_KEY` (or `OPENAI_API_KEY`) is used for Codex; `ANTHROPIC_API_KEY` for Claude. Readiness means a CLI and key are configured; authentication is verified only when the provider executes. No keys are sent to the frontend or stored in tickets. Restart the backend after changing environment configuration.

Configuration:

| Variable | Default / purpose |
| --- | --- |
| `SWIMLANE_WORKSPACE_ROOTS` | This application directory; colon-separated absolute allowed repository roots |
| `SWIMLANE_ARTIFACTS` | `backend-go/.swimlane` for Go, `backend/.swimlane` for Python; retained worktrees and image attachments |
| `SWIMLANE_RUN_TIMEOUT` | 600 seconds, maximum 3600; agent and test execution deadline |
| `SWIMLANE_DB` | `swimlane.sqlite3` in the chosen backend working directory |

Real calls use your provider account and may incur API charges. Repository text and pasted images are sent to the selected provider.

## Execution and review

Each queued run stores a prompt/image/permission snapshot and the repository's committed HEAD. It creates a detached Git worktree; uncommitted changes in your original checkout are excluded. The source checkout is never merged or overwritten by the host. Worktree registration updates Git metadata. Worktrees remain available after success, cancellation or failure, and their paths appear in run review. Merge/reject and worktree removal are phase 3 work.

- **Codex:** native read-only or workspace-write sandbox, noninteractive approvals set to never, isolated Codex configuration home and no user execution rules. Provider keys are excluded from generated shell-command environments.
- **Claude:** restricted file tools confined to the worktree; read-only runs expose Read/Glob/Grep, edit runs add Edit/Write. Shell tools, inherited MCP servers and slash commands are disabled. Bare mode uses only server API credentials.
- **Tests:** explicit ticket opt-in runs a fixed Vitest (`npm test -- --run`) or Pytest (`python3 -m pytest -q`) argv in the configured relative directory. Test children do not inherit provider keys. Tests execute trusted local code under your user account, not inside an OS/container sandbox. Dependencies must already be available in the worktree/environment; missing dependencies cause a Failed attempt. There is no automatic dependency installation.

Done requires a successful provider completion event, successful opted-in tests and a readable diff. Staged, unstaged, committed and untracked changes are compared to the queued base commit; binary changes are reported without embedded binary data. Diff limits are 200 files and 500 KB per file; oversized diffs fail the run and retain its worktree. Agent output is capped at 5 MB, activity at 1,000 entries plus status events per attempt, and individual activity messages at 10,000 characters.

Use **Cancel run** for queued/running work. Child process groups are terminated; partial diffs remain reviewable. **Retry ticket** starts a fresh worktree from current committed HEAD while preserving old attempts. Tickets stay read-only after running; create a new ticket to change the prompt or permissions. A graceful backend shutdown cancels active work; after an unexpected restart, interrupted runs become Failed and queued runs resume.

Git checkout/diff operations have their own 30-second per-command limit; cancellation waits for an in-flight Git operation to finish before preserving the result. Live updates use replayable SSE with a two-second polling fallback. Cross-origin execution requests and non-local Host headers are rejected. This is not a multi-user or hostile-code isolation service; use trusted repositories, one backend worker, and localhost.

## Verify

```sh
cd frontend
npm run build
npm run test:coverage
npm run quality
npm run test:e2e
```

Playwright uses **Google Chrome only**. It starts an isolated Go backend on 8081, a frontend on 5174 and a fresh temporary database, with provider credentials blanked. Set `SWIMLANE_E2E_BACKEND=python` to exercise the preserved Python service instead. No paid calls occur. UI tests cover board workflows, images/diffs, cancel/retry/history and repository settings.

```sh
cd backend-go
go test -race -coverprofile=coverage.out ./internal/studio
go run ./cmd/quality
go vet ./...
go build ./cmd/server
```

The Go suite uses temporary SQLite databases, real temporary Git repositories and fake Codex/Claude executables. Its CRAP report is [docs/go-quality.json](docs/go-quality.json); every API function must score under 10. The Go API embeds the matching categorised OpenAPI contract and serves `/docs` and `/openapi.json` without a Python service.

Original Python checks:

```sh
cd backend
.venv/bin/python -m pytest --cov=app --cov-report=json:coverage.json --cov-fail-under=80 tests
.venv/bin/python ../scripts/backend-quality.py
```

Backend tests exercise real temporary Git worktrees, actual diffs and real supervised processes using fake Codex/Claude executables. They test CLI protocols, image payloads, environment handling, failures, timeout/cancellation, test gates, SSE replay and persistence. Live provider authentication/model execution needs your configured credentials and is not covered by the offline suite.

CRAP gates require every authored function to score **under 10**, combining cyclomatic complexity and executable-line coverage. Frontend: ESLint and Vitest V8. Backend: Radon and pytest-cov. Reports are in `docs/`; generate coverage before running quality gates. Template-generated Vue functions are covered but excluded from source complexity analysis.

[Plan](docs/PLAN.md) · [Gherkin specification](docs/board.feature) · [Backend interchangeability](docs/backends.feature) · [Validation](docs/VALIDATION.md)

Provider references: [Codex noninteractive execution](https://learn.chatgpt.com/docs/non-interactive-mode), [Claude CLI and restricted mode](https://code.claude.com/docs/en/cli-reference), [Claude programmatic execution](https://code.claude.com/docs/en/headless).

### Ticket groups
Drag a ticket onto a group's header or cards to join it, including a collapsed group. Drag between groups to change membership, or anywhere in its current column outside a group to ungroup it. These drops preserve the ticket's status, results and unsaved prompt edits; they do not start a run. Dropping a Todo ticket onto the Doing lane outside a group still queues it.

Use **Add group** in Todo to create a named group. Choose a circular colour swatch or use the custom colour picker. **Add ticket** inside a group starts a draft with that membership; the detail pane's **Group** selector can organise existing tickets. Save changes to persist a Todo draft's group. Completed and active tickets can be regrouped immediately without changing the execution snapshot.

A ticket keeps its group across Todo, Doing and Done. Empty groups appear in Todo; other lanes show groups containing matching tickets. The circular minus button hides a group's tickets in that lane until expanded with the plus button. The circular three-dot menu edits its name/colour or removes the group while keeping all tickets, responses and diffs. Names, colours and membership are persisted in SQLite; collapse is a view setting for the current page.

### Run a group
Choose **Run group** from the three-dot menu in Todo. It snapshots all saved Todo tickets in that group in their displayed order (oldest ticket first), queues them atomically, and runs one at a time. Completed, failed and cancelled tickets are excluded. Save any pending changes in the selected ticket first. Search/status filters do not change the sequence; it uses the entire group.

Every real-agent ticket must have **Run configured tests** enabled and its repository must have a Vitest or Pytest preset. The next prompt starts only when the preceding attempt completes successfully with passing tests. Demo groups simulate results and show **Tests: simulated**. A failure, timeout or cancellation marks unstarted followers Cancelled with a group-stopped explanation; retry explicitly after resolving the cause. Queue dependencies survive backend restarts. Another group run is blocked while that group's tickets are queued or running.

Each ticket retains the existing execution model: its own isolated worktree from the repository's committed HEAD. Group runs control ordering and test gates; they do not merge one ticket's changes into the next worktree.

### Project details
The board header shows the current project's title and description. Use the pencil **Edit project** action to change them, or **Add a project description** when empty. The title also appears in the breadcrumb. Both fields are saved locally in SQLite. Titles can contain up to 120 characters and descriptions up to 2,000 characters.

### Projects and deleted tickets
Click **Workspace** or the top-left logo to open the projects page. The project name in the breadcrumb opens a dropdown listing active projects with a highlight and checkmark on the current one; selecting a project opens its board. The dropdown also has **Manage projects**, and supports keyboard navigation and Escape. Unsaved ticket edits require an explicit discard before switching projects. Create a project with a title and description, open another board, edit project details, or delete a project. Upload or paste a PNG, JPEG or WebP screenshot (up to 5 MB) in the project editor to add a card preview; click its editor preview to enlarge it or use **Remove screenshot** to clear it. Screenshots are saved locally in SQLite and retained through project deletion and restore. Each project has its own tickets and groups; repository settings and the FIFO execution worker are shared. The current selection is remembered in this browser. Existing data migrates into **My first project** (retaining any title and description you already saved).

Select a ticket and use the trash icon in its details header to delete it. Deletion is recoverable: **Manage projects** contains deleted projects and deleted tickets for the current project, with **Restore** actions. Project deletion hides its board without removing prompts, images, responses, diffs, history or repository worktrees. Restoring a project does not restore tickets separately deleted from it. Queued/running tickets and projects containing them cannot be deleted until those runs are cancelled or finished. Unsaved ticket changes require an explicit discard before opening project management.

Click a ticket, then **Shift-click** another to select the inclusive range in board display order. **Ctrl-click** (or **Cmd-click** on Mac) toggles individual tickets. The bottom selection bubble shows the count, **Delete all** and a clear-selection button. Deleting requires confirmation, lists the selected tickets and warns about unsaved edits to a selected ticket. Cancel keeps the selection; successful deletion clears it. Delete up to 200 tickets per action. Any queued/running ticket blocks the entire batch, and deleted tickets remain recoverable on the projects page. Changing projects, search or status filters clears selection; collapsed groups are excluded from new ranges.

Project APIs are `/api/projects` and `/api/projects/{id}` (create/list, read/update/delete), with `POST /api/projects/{id}/restore`. Ticket and group lists/creation accept `project_id` (default `default` for compatibility); ticket deletion is `DELETE /api/tickets/{id}`, restoration is `POST /api/tickets/{id}/restore`, and `GET /api/tickets?project_id=...&deleted=true` lists deleted tickets. These are local application boundaries, not multi-user authorization.

The SQLite migration adds a projects table and a deletion marker while leaving legacy tables and ticket JSON intact. Before upgrading the local database, a backup was saved at `backend/backups/before-project-management-2026-10-06.sqlite3`; rollback requires restoring that backup and using the previous application version. New project/deletion state requires the upgraded application.
