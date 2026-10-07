# Go backend

A standalone Go implementation of Swimlane's Python/FastAPI API. The Python service is retained in `../backend`; Go does not call it or require Python to run (unless a repository opts into Pytest).

```sh
go run ./cmd/server -import ../backend/swimlane.sqlite3
```

Go 1.26+ and a C compiler are required by `github.com/mattn/go-sqlite3`. The server defaults to `127.0.0.1:8080`, with Swagger at `/docs` and the API contract at `/openapi.json`. Override the address with `-addr 127.0.0.1:8081`; disable sample tickets with `-seed=false`.

The optional import creates a consistent online SQLite backup only if the target database is absent. It opens the source read-only and preserves all tables, IDs and history. Never run two queue workers against the same database. The default Go and Python databases are separate and do not synchronise after import.

Copy `.env.example` to `.env` for server-only keys, repository roots and execution settings. Settings have the same names and behaviour as Python. `SWIMLANE_DB` selects the Go database. No key is returned by the API; child tests receive no provider credentials. Real calls require configured CLI binaries and may incur provider charges. The application remains a localhost, single-user service for trusted repositories.

```sh
go test -race -coverprofile=coverage.out ./internal/studio
go run ./cmd/quality
go vet ./...
go build -o bin/swimlane-go ./cmd/server
```

`internal/studio` has typed API models, SQLite persistence, endpoint handlers, a single FIFO worker, Git isolation, CLI adapters, supervised process groups and replayable SSE. Runs snapshot the prompt, permissions and committed HEAD before queueing. Group dependencies persist across restarts. Worktrees and partial changes are retained for review. OpenAPI and seed data are embedded in the binary.

See the [root README](../README.md) for frontend setup, execution safeguards and rollback to Python.
