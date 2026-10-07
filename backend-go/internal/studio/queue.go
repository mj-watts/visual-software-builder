package studio

import (
	"context"
	"fmt"
	"net/http"
	"sort"
	"time"
)

func (a *App) runRoutes() {
	a.route("POST /api/tickets/{id}/run", 202, func(r *http.Request) (any, error) {
		t, err := a.ticket(r.PathValue("id"))
		if err != nil {
			return nil, err
		}
		if err = editable(t); err != nil {
			return nil, err
		}
		return a.queue(t)
	})
	a.route("POST /api/tickets/{id}/retry", 202, a.retry)
	a.mux.HandleFunc("POST /api/tickets/{id}/cancel", a.cancelHTTP)
	a.route("GET /api/tickets/{id}/runs", 200, func(r *http.Request) (any, error) {
		t, err := a.ticket(r.PathValue("id"))
		if err != nil {
			return nil, err
		}
		return queryJSON[Run](a.store, `SELECT data FROM runs WHERE ticket_id=? ORDER BY rowid DESC`, t.ID)
	})
	a.route("GET /api/runs/{id}/events", 200, func(r *http.Request) (any, error) {
		_, err := a.run(r.PathValue("id"))
		if err != nil {
			return nil, err
		}
		return a.store.events(0, r.PathValue("id"))
	})
}
func (a *App) run(id string) (Run, error) {
	return oneJSON[Run](a.store, `SELECT data FROM runs WHERE id=?`, id)
}
func (a *App) buildRun(t Ticket) (Run, error) {
	run := Run{ID: newID(), TicketID: t.ID, Snapshot: t.Draft, Status: "queued", CreatedAt: now(), TestPreset: "none", TestDirectory: ".", Files: []ChangedFile{}, Tests: "not-requested"}
	if t.Provider == "demo" {
		return run, nil
	}
	if !a.config.enabled(t.Provider) {
		return run, problem(409, "This provider is not connected. Configure server credentials and its CLI.")
	}
	w, err := oneJSON[Workspace](a.store, `SELECT data FROM workspaces WHERE id=?`, t.WorkspaceID)
	if err != nil {
		return run, problem(409, "Select a registered Git repository before running a real agent.")
	}
	w, err = resolveWorkspace(w, a.config)
	if err != nil {
		return run, problem(409, err.Error())
	}
	base, err := git(w.Path, "rev-parse", "HEAD")
	if err != nil {
		return run, err
	}
	run.WorkspacePath = w.Path
	run.BaseCommit = trim(base)
	run.TestPreset = w.TestPreset
	run.TestDirectory = w.TestDirectory
	return run, nil
}
func markQueued(t Ticket, run Run) Ticket {
	t.Status = "queued"
	t.QueuedAt = run.CreatedAt
	t.LatestRunID = run.ID
	t.Response = ""
	t.Files = []ChangedFile{}
	return t
}
func (a *App) queue(t Ticket) (Ticket, error) {
	run, err := a.buildRun(t)
	if err != nil {
		return t, err
	}
	t = markQueued(t, run)
	if err = a.store.enqueue([]Run{run}, []Ticket{t}); err != nil {
		return t, err
	}
	return t, a.store.event(run, "status", "Queued")
}
func (a *App) retry(r *http.Request) (any, error) {
	t, err := a.ticket(r.PathValue("id"))
	if err != nil {
		return nil, err
	}
	if t.Status != "failed" && t.Status != "cancelled" {
		return nil, problem(409, "Only failed or cancelled tickets can be retried.")
	}
	return a.queue(t)
}
func (a *App) cancel(r *http.Request) (any, error) {
	t, err := a.ticket(r.PathValue("id"))
	if err != nil {
		return nil, err
	}
	if t.Status != "running" && t.Status != "queued" {
		return nil, problem(409, "Only queued or running tickets can be cancelled.")
	}
	if cancel := a.active[t.ID]; cancel != nil {
		cancel()
	}
	run, err := a.run(t.LatestRunID)
	if err != nil {
		return nil, err
	}
	if err = a.finish(&run, "cancelled", "Cancelled before execution or during run. Partial changes are retained for review.", run.Files); err != nil {
		return nil, err
	}
	return a.ticket(t.ID)
}
func (a *App) queueGroup(r *http.Request) (any, error) {
	g, err := a.group(r.PathValue("id"))
	if err != nil {
		return nil, err
	}
	tickets, err := a.groupCandidates(g)
	if err != nil {
		return nil, err
	}
	runs, err := a.groupSnapshots(tickets)
	if err != nil {
		return nil, err
	}
	batch := newID()
	previous := ""
	for i := range runs {
		runs[i].GroupRunID = batch
		runs[i].PreviousRunID = previous
		previous = runs[i].ID
		tickets[i] = markQueued(tickets[i], runs[i])
	}
	if err = a.store.enqueue(runs, tickets); err != nil {
		return nil, err
	}
	for _, run := range runs {
		if err = a.store.event(run, "status", "Queued in group; waiting for preceding prompts and tests."); err != nil {
			return nil, err
		}
	}
	return tickets, nil
}
func (a *App) groupCandidates(g Group) ([]Ticket, error) {
	all, err := a.store.tickets(g.ProjectID, false)
	if err != nil {
		return nil, err
	}
	out := []Ticket{}
	for _, t := range all {
		if t.GroupID != g.ID {
			continue
		}
		if t.Status == "running" || t.Status == "queued" {
			return nil, problem(409, "This group already has queued or running tickets.")
		}
		if t.Status == "todo" {
			out = append(out, t)
		}
	}
	if len(out) == 0 {
		return nil, problem(409, "This group has no Todo tickets to run.")
	}
	return out, nil
}
func (a *App) groupSnapshots(tickets []Ticket) ([]Run, error) {
	runs := []Run{}
	for _, t := range tickets {
		run, err := a.buildRun(t)
		if err != nil {
			return nil, err
		}
		if t.Provider != "demo" && (!t.AllowTests || run.TestPreset == "none") {
			return nil, problem(409, "Group runs require tests on every real-agent ticket. Enable Run configured tests and select a repository test preset.")
		}
		runs = append(runs, run)
	}
	return runs, nil
}
func (a *App) finish(run *Run, status, response string, files []ChangedFile) error {
	run.Status = status
	run.FinishedAt = now()
	run.Response = response
	run.Files = files
	if err := a.store.saveRun(*run); err != nil {
		return err
	}
	t, err := a.store.ticket(run.TicketID, false)
	if err != nil {
		return err
	}
	if t.LatestRunID == run.ID {
		t.Status = status
		t.Response = response
		t.Files = files
		if err = a.store.saveTicket(t); err != nil {
			return err
		}
	}
	return a.store.event(*run, "status", status)
}
func (a *App) blocked(run Run) bool {
	if run.PreviousRunID == "" {
		return false
	}
	prev, err := a.run(run.PreviousRunID)
	if err != nil {
		return true
	}
	return prev.Status != "done" || (prev.Snapshot.Provider != "demo" && prev.Tests != "passed")
}
func (a *App) recover() error {
	tickets, err := a.store.tickets("", false)
	if err != nil {
		return err
	}
	for _, t := range tickets {
		if t.Status != "running" {
			continue
		}
		run, err := a.run(t.LatestRunID)
		if err != nil {
			t.Status = "failed"
			t.Response = "Run interrupted by server restart. Retry this ticket."
			if err = a.store.saveTicket(t); err != nil {
				return err
			}
			continue
		}
		run.Error = "Run interrupted by server restart. Retry this ticket to start a fresh attempt."
		if err = a.finish(&run, "failed", run.Error, run.Files); err != nil {
			return err
		}
	}
	return nil
}
func (a *App) next() (Ticket, error) {
	tickets, err := a.store.tickets("", false)
	if err != nil {
		return Ticket{}, err
	}
	sort.SliceStable(tickets, func(i, j int) bool { return queuedBefore(tickets[i].QueuedAt, tickets[j].QueuedAt) })
	for _, t := range tickets {
		if t.Status == "queued" {
			return t, nil
		}
	}
	return Ticket{}, nil
}
func (a *App) Run(parent context.Context) {
	a.mu.Lock()
	if a.closed {
		a.mu.Unlock()
		return
	}
	ctx, cancel := context.WithCancel(parent)
	a.stop = cancel
	a.worker.Add(1)
	a.mu.Unlock()
	defer a.worker.Done()
	defer cancel()
	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			a.dispatch(ctx)
		}
	}
}
func (a *App) dispatch(ctx context.Context) {
	a.mu.Lock()
	t, err := a.next()
	a.mu.Unlock()
	if err != nil {
		logError(err)
		return
	}
	if t.ID == "" {
		return
	}
	a.execute(ctx, t)
}
func logError(err error) {
	if err != nil {
		fmt.Printf("Go backend: %v\n", err)
	}
}

func queuedBefore(first, second string) bool {
	a, errA := time.Parse(time.RFC3339Nano, first)
	b, errB := time.Parse(time.RFC3339Nano, second)
	if errA != nil || errB != nil {
		return first < second
	}
	return a.Before(b)
}
func (a *App) cancelHTTP(w http.ResponseWriter, r *http.Request) {
	a.mu.Lock()
	t, err := a.cancelTarget(r.PathValue("id"))
	if err != nil {
		a.mu.Unlock()
		writeError(w, err)
		return
	}
	done := a.done[t.ID]
	if done == nil {
		value, err := a.cancel(r)
		a.mu.Unlock()
		if err != nil {
			writeError(w, err)
			return
		}
		writeJSON(w, 200, value)
		return
	}
	a.active[t.ID]()
	a.mu.Unlock()
	select {
	case <-r.Context().Done():
		return
	case <-done:
	}
	a.mu.Lock()
	value, err := a.ticket(t.ID)
	a.mu.Unlock()
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, 200, value)
}

func (a *App) cancelTarget(id string) (Ticket, error) {
	t, err := a.ticket(id)
	if err != nil {
		return t, err
	}
	if t.Status != "queued" && t.Status != "running" {
		return t, problem(409, "Only queued or running tickets can be cancelled.")
	}
	return t, nil
}
