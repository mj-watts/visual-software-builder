package studio

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"time"
)

func (a *App) execute(parent context.Context, t Ticket) {
	ctx, cancel := context.WithTimeout(parent, a.config.Timeout)
	defer cancel()
	a.mu.Lock()
	run, err := a.begin(t, cancel)
	a.mu.Unlock()
	if err != nil {
		logError(err)
		return
	}
	if run.Status != "running" {
		return
	}
	defer a.endActive(t.ID)
	response, files, err := a.perform(ctx, &run)
	a.mu.Lock()
	defer a.mu.Unlock()
	if err == nil {
		logError(a.finish(&run, "done", response, files))
		return
	}
	status, message := failureMessage(err)
	run.Error = a.config.redact(message)
	// Git operations have their own deadline and finish before finalising cancellation.
	a.mu.Unlock()
	partial := a.partialDiff(run)
	a.mu.Lock()
	logError(a.finish(&run, status, run.Error, partial))
}
func (a *App) begin(t Ticket, cancel context.CancelFunc) (Run, error) {
	latest, err := a.store.ticket(t.ID, false)
	if err != nil {
		return Run{}, err
	}
	if latest.Status != "queued" {
		return Run{}, nil
	}
	run, err := a.ensureRun(latest)
	if err != nil {
		return Run{}, err
	}
	latest.LatestRunID = run.ID

	if a.blocked(run) {
		message := "Group stopped: the preceding prompt did not complete with passing tests. This prompt was not executed. Retry it explicitly after resolving the earlier failure."
		return run, a.finish(&run, "cancelled", message, run.Files)
	}
	run.StartedAt = now()
	run.Status = "running"
	latest.Status = "running"
	if err = a.store.saveRun(run); err != nil {
		return run, err
	}
	if err = a.store.saveTicket(latest); err != nil {
		return run, err
	}
	if err = a.store.event(run, "status", "Running"); err != nil {
		return run, err
	}
	a.active[t.ID] = cancel
	a.done[t.ID] = make(chan struct{})
	return run, nil
}
func failureMessage(err error) (string, string) {
	if errors.Is(err, context.Canceled) {
		return "cancelled", "Run cancelled. Partial changes are retained for review."
	}
	if errors.Is(err, context.DeadlineExceeded) {
		return "failed", "Run timed out. Partial changes are retained for review."
	}
	return "failed", err.Error()
}
func (a *App) partialDiff(run Run) []ChangedFile {
	if run.Worktree == "" {
		return []ChangedFile{}
	}
	files, err := collectDiff(run.Worktree, run.BaseCommit)
	if err != nil {
		return []ChangedFile{}
	}
	return files
}
func (a *App) persist(run Run) error { a.mu.Lock(); defer a.mu.Unlock(); return a.store.saveRun(run) }
func (a *App) emit(run Run, kind, message string) error {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.store.event(run, kind, a.config.redact(message))
}
func (a *App) perform(ctx context.Context, run *Run) (string, []ChangedFile, error) {
	if run.Snapshot.Provider == "demo" {
		if run.GroupRunID != "" {
			run.Tests = "simulated"
		}
		return demo(ctx, *run, a.config.DemoDelay)
	}
	if err := a.prepare(ctx, run); err != nil {
		return "", nil, err
	}
	emit := func(k, v string) error { return a.emit(*run, k, v) }
	response, err := cliAgent(ctx, a.config, *run, emit)
	if err != nil {
		return "", nil, err
	}
	if err = a.test(ctx, run, emit); err != nil {
		return "", nil, err
	}
	files, err := collectDiff(run.Worktree, run.BaseCommit)
	if err != nil {
		return "", nil, err
	}
	if run.Snapshot.Permission == "read-only" && len(files) > 0 {
		return "", nil, fmt.Errorf("Read-only run modified files. Changes retained; run marked Failed.")
	}
	return response, files, nil
}
func (a *App) prepare(ctx context.Context, run *Run) error {
	a.mu.Lock()
	w, err := oneJSON[Workspace](a.store, `SELECT data FROM workspaces WHERE id=?`, run.Snapshot.WorkspaceID)
	a.mu.Unlock()
	if err != nil {
		return err
	}
	if _, err = resolveWorkspace(w, a.config); err != nil {
		return err
	}
	run.Worktree = filepath.Join(a.config.Artifacts, run.ID, "worktree")
	if err = a.persist(*run); err != nil {
		return err
	}
	if err = prepareWorktree(run.WorkspacePath, run.Worktree, run.BaseCommit); err != nil {
		return err
	}
	if ctx.Err() != nil {
		return ctx.Err()
	}
	return a.emit(*run, "workspace", "Isolated worktree ready at "+run.Worktree)
}

var testCommands = map[string][]string{"vitest": {"npm", "test", "--", "--run"}, "pytest": {"python3", "-m", "pytest", "-q"}}

func (a *App) test(ctx context.Context, run *Run, emit emitFunc) error {
	if !run.Snapshot.AllowTests {
		return nil
	}
	argv, ok := testCommands[run.TestPreset]
	if !ok {
		return fmt.Errorf("Select a test preset for the repository before enabling tests.")
	}
	return a.runTests(ctx, run, emit, argv)
}
func (a *App) runTests(ctx context.Context, run *Run, emit emitFunc, argv []string) error {
	dir, err := relativeDirectory(run.Worktree, run.TestDirectory)
	if err != nil {
		return err
	}
	run.Tests = "running"
	if err = a.persist(*run); err != nil {
		return err
	}
	if err = emit("test", "Running "+run.TestPreset+" tests"); err != nil {
		return err
	}
	env, err := childEnvironment(a.config, "tests", *run)
	if err != nil {
		return err
	}
	if err = runProcess(ctx, argv, dir, "", env, emit); err != nil {
		run.Tests = "failed"
		return err
	}
	run.Tests = "passed"
	if err = a.persist(*run); err != nil {
		return err
	}
	return emit("test", "Tests passed")
}
func demo(ctx context.Context, run Run, delay time.Duration) (string, []ChangedFile, error) {
	timer := time.NewTimer(delay)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return "", nil, ctx.Err()
	case <-timer.C:
	}
	response := fmt.Sprintf("Simulated run complete.\n\nI reviewed the prompt for “%s” and prepared an illustrative implementation and test diff.\n\nThis is a demo response. No AI provider was called and no repository files were changed. Connect a coding agent adapter to perform real work.", run.Snapshot.Title)
	files := []ChangedFile{{Path: "src/components/Feature.vue", Additions: 3, Deletions: 1, Diff: "--- a/src/components/Feature.vue\n+++ b/src/components/Feature.vue\n@@ -1,3 +1,5 @@\n <template>\n-  <section />\n+  <section aria-label=\"Feature\">\n+    <h2>Ready for your next idea</h2>\n+  </section>\n </template>"}, {Path: "src/components/Feature.test.ts", Additions: 3, Diff: "--- /dev/null\n+++ b/src/components/Feature.test.ts\n@@ -0,0 +1,3 @@\n+it('renders the feature heading', () => {\n+  expect(wrapper.text()).toContain('Ready for your next idea')\n+})"}}
	return response, files, nil
}

func (a *App) ensureRun(t Ticket) (Run, error) {
	run, err := a.run(t.LatestRunID)
	if err == nil {
		return run, nil
	}
	queued, err := a.queue(t)
	if err != nil {
		return Run{}, err
	}
	return a.run(queued.LatestRunID)
}

func (a *App) endActive(id string) {
	a.mu.Lock()
	defer a.mu.Unlock()
	delete(a.active, id)
	if done := a.done[id]; done != nil {
		close(done)
		delete(a.done, id)
	}
}
