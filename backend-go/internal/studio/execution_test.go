package studio

import (
	"bufio"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func write(t *testing.T, path, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0700); err != nil {
		t.Fatal(err)
	}
}
func gitTest(t *testing.T, path string, args ...string) string {
	t.Helper()
	v, err := git(path, args...)
	if err != nil {
		t.Fatal(err)
	}
	return trim(v)
}
func repository(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	gitTest(t, root, "init")
	gitTest(t, root, "config", "user.email", "test@example.com")
	gitTest(t, root, "config", "user.name", "Test")
	write(t, filepath.Join(root, "old.txt"), "old\n")
	write(t, filepath.Join(root, "test_sample.py"), "def test_sample():\n    assert True\n")
	write(t, filepath.Join(root, ".gitignore"), "__pycache__/\n.pytest_cache/\n")
	gitTest(t, root, "add", ".")
	gitTest(t, root, "commit", "-m", "Initial")
	return root
}
func realApp(t *testing.T, repo, provider, script string) *App {
	t.Helper()
	bin := filepath.Join(t.TempDir(), provider)
	write(t, bin, "#!/bin/sh\ncat >/dev/null\n"+script+"\n")
	a, err := New(Config{Database: filepath.Join(t.TempDir(), "real.db"), Roots: []string{repo}, Artifacts: t.TempDir(), Timeout: 3 * time.Second, Binaries: map[string]string{provider: bin}, Keys: map[string]string{provider: "test-secret"}})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { a.Close() })
	return a
}
func start(t *testing.T, a *App) {
	t.Helper()
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	go a.Run(ctx)
}
func workspace(t *testing.T, a *App, repo, preset string) string {
	return call(t, a, "POST", "/api/workspaces", map[string]any{"path": repo, "name": "Example", "test_preset": preset}, 201)["id"].(string)
}

const codexSuccess = `printf 'new\n' > old.txt
printf 'added\n' > 'unicode name é.txt'
printf '%s\n' '{"type":"item.completed","item":{"type":"agent_message","text":"Implemented test-secret"}}' '{"type":"turn.completed"}'`

func TestRealAgentIsolationDiffAndTests(t *testing.T) {
	repo := repository(t)
	a := realApp(t, repo, "codex", codexSuccess)
	// The fixed pytest preset uses the already installed Python test environment.
	venv, err := filepath.Abs("../../../backend/.venv/bin")
	if err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", venv+string(os.PathListSeparator)+os.Getenv("PATH"))
	wid := workspace(t, a, repo, "pytest")
	v := ticket(t, a, "", map[string]any{"provider": "codex", "workspace_id": wid, "permission": "workspace-write", "allow_tests": true})
	id := v["id"].(string)
	call(t, a, "POST", "/api/tickets/"+id+"/run", nil, 202)
	start(t, a)
	result := waitTicket(t, a, id, "done")
	if len(result["files"].([]any)) != 2 || strings.Contains(result["response"].(string), "test-secret") {
		t.Fatal(result)
	}
	old, _ := os.ReadFile(filepath.Join(repo, "old.txt"))
	if string(old) != "old\n" {
		t.Fatal("source modified")
	}
	runs := list(t, a, "/api/tickets/"+id+"/runs")
	if runs[0]["tests"] != "passed" || runs[0]["base_commit"] != gitTest(t, repo, "rev-parse", "HEAD") {
		t.Fatal(runs)
	}
	call(t, a, "POST", "/api/tickets/"+id+"/retry", nil, 409)
	// Binary, staged and committed changes are included in the diff.
	tree := runs[0]["worktree"].(string)
	write(t, filepath.Join(tree, "binary.bin"), "\x00\x01")
	gitTest(t, tree, "add", "old.txt")
	gitTest(t, tree, "commit", "-m", "Agent commit")
	files, err := collectDiff(tree, runs[0]["base_commit"].(string))
	if err != nil || len(files) != 3 {
		t.Fatal(files, err)
	}
}
func TestReadOnlyFailuresAndRetry(t *testing.T) {
	repo := repository(t)
	a := realApp(t, repo, "codex", codexSuccess)
	wid := workspace(t, a, repo, "none")
	v := ticket(t, a, "", map[string]any{"provider": "codex", "workspace_id": wid})
	id := v["id"].(string)
	call(t, a, "POST", "/api/tickets/"+id+"/run", nil, 202)
	start(t, a)
	result := waitTicket(t, a, id, "failed")
	if !strings.Contains(result["response"].(string), "Read-only") {
		t.Fatal(result)
	}
	call(t, a, "POST", "/api/tickets/"+id+"/retry", nil, 202)
	waitTicket(t, a, id, "failed")
	if len(list(t, a, "/api/tickets/"+id+"/runs")) != 2 {
		t.Fatal("lost history")
	}
	call(t, a, "POST", "/api/tickets/"+id+"/cancel", nil, 409)
}
func TestProviderFailureCompletionAndClaude(t *testing.T) {
	repo := repository(t)
	for _, script := range []string{`echo '{"type":"turn.failed"}'`, `echo '{"type":"diagnostic"}'`, `echo 'not json'; echo 'test-secret' >&2; exit 3`} {
		a := realApp(t, repo, "codex", script)
		wid := workspace(t, a, repo, "none")
		v := ticket(t, a, "", map[string]any{"provider": "codex", "workspace_id": wid})
		id := v["id"].(string)
		call(t, a, "POST", "/api/tickets/"+id+"/run", nil, 202)
		start(t, a)
		waitTicket(t, a, id, "failed")
	}
	a := realApp(t, repo, "claude", `echo '{"type":"assistant","message":{"content":[{"type":"text","text":"Reviewing"}]}}'; echo '{"type":"result","result":"Reviewed","is_error":false}'`)
	wid := workspace(t, a, repo, "none")
	v := ticket(t, a, "", map[string]any{"provider": "claude", "workspace_id": wid, "images": []string{"data:image/png;base64,aW1hZ2U="}})
	id := v["id"].(string)
	call(t, a, "POST", "/api/tickets/"+id+"/run", nil, 202)
	start(t, a)
	waitTicket(t, a, id, "done")
}
func TestCancelTimeoutAndProcessLimits(t *testing.T) {
	repo := repository(t)
	a := realApp(t, repo, "codex", "sleep 10")
	wid := workspace(t, a, repo, "none")
	v := ticket(t, a, "", map[string]any{"provider": "codex", "workspace_id": wid})
	id := v["id"].(string)
	call(t, a, "POST", "/api/tickets/"+id+"/run", nil, 202)
	start(t, a)
	waitTicket(t, a, id, "running")
	call(t, a, "POST", "/api/tickets/"+id+"/cancel", nil, 200)
	waitTicket(t, a, id, "cancelled")
	b := testApp(t)
	b.config.Timeout = 10 * time.Millisecond
	b.config.DemoDelay = time.Second
	v = ticket(t, b, "", nil)
	id = v["id"].(string)
	call(t, b, "POST", "/api/tickets/"+id+"/run", nil, 202)
	start(t, b)
	result := waitTicket(t, b, id, "failed")
	if !strings.Contains(result["response"].(string), "timed out") {
		t.Fatal(result)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Millisecond)
	defer cancel()
	err := runProcess(ctx, []string{"/bin/sh", "-c", "trap '' TERM; sleep 10"}, t.TempDir(), "", []string{}, func(k, v string) error { return nil })
	if err == nil {
		t.Fatal("timeout ignored")
	}
	python, err := exec.LookPath("python3")
	if err != nil {
		t.Fatal(err)
	}
	err = runProcess(context.Background(), []string{python, "-c", "print(('x'*900000+'\\n')*6)"}, t.TempDir(), "", []string{}, func(k, v string) error { return nil })
	if err == nil {
		t.Fatal("output budget ignored")
	}
}
func TestGroupStopsAndEnforcesTests(t *testing.T) {
	a := testApp(t)
	g := call(t, a, "POST", "/api/groups", map[string]any{"name": "Group"}, 201)
	gid := g["id"].(string)
	first := ticket(t, a, "", map[string]any{"group_id": gid})
	second := ticket(t, a, "", map[string]any{"group_id": gid})
	w := httptest.NewRecorder()
	a.Handler().ServeHTTP(w, localRequest("POST", "/api/groups/"+gid+"/run"))
	if w.Code != 202 {
		t.Fatal(w.Body.String())
	}
	call(t, a, "POST", "/api/groups/"+gid+"/run", nil, 409)
	call(t, a, "POST", "/api/tickets/"+first["id"].(string)+"/cancel", nil, 200)
	start(t, a)
	result := waitTicket(t, a, second["id"].(string), "cancelled")
	if !strings.Contains(result["response"].(string), "Group stopped") {
		t.Fatal(result)
	}
	repo := repository(t)
	b := realApp(t, repo, "codex", codexSuccess)
	wid := workspace(t, b, repo, "none")
	g = call(t, b, "POST", "/api/groups", map[string]any{"name": "Real"}, 201)
	ticket(t, b, "", map[string]any{"group_id": g["id"], "provider": "codex", "workspace_id": wid})
	call(t, b, "POST", "/api/groups/"+g["id"].(string)+"/run", nil, 409)
}
func TestSSEReplayAndDocs(t *testing.T) {
	a := testApp(t)
	v := ticket(t, a, "", nil)
	call(t, a, "POST", "/api/tickets/"+v["id"].(string)+"/run", nil, 202)
	call(t, a, "POST", "/api/tickets/"+v["id"].(string)+"/cancel", nil, 200)
	server := httptest.NewServer(a.Handler())
	defer server.Close()
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	req, _ := http.NewRequestWithContext(ctx, "GET", server.URL+"/api/events", nil)
	req.Header.Set("Last-Event-ID", "1")
	response, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	scanner := bufio.NewScanner(response.Body)
	if !scanner.Scan() || scanner.Text() != "id: 2" {
		t.Fatal("bad replay", scanner.Text())
	}
	response.Body.Close()
	call(t, a, "GET", "/api/events?after=-1", nil, 422)
	call(t, a, "GET", "/api/events?after=x", nil, 422)
	w := httptest.NewRecorder()
	a.Handler().ServeHTTP(w, localRequest("GET", "/docs"))
	if !strings.Contains(w.Body.String(), "SwaggerUIBundle") {
		t.Fatal(w.Body.String())
	}
	schema := call(t, a, "GET", "/openapi.json", nil, 200)
	for path, raw := range schema["paths"].(map[string]any) {
		for method := range raw.(map[string]any) {
			r := localRequest(strings.ToUpper(method), strings.ReplaceAll(strings.ReplaceAll(strings.ReplaceAll(path, "{id}", "missing"), "{ticket_id}", "missing"), "{run_id}", "missing"))
			_, pattern := a.mux.Handler(r)
			if pattern == "" {
				t.Fatal("unimplemented", method, path)
			}
		}
	}
}
func TestImportPersistenceRecoveryAndSeed(t *testing.T) {
	a := testApp(t)
	if err := a.Seed(); err != nil {
		t.Fatal(err)
	}
	if err := a.Seed(); err != nil {
		t.Fatal(err)
	}
	if len(list(t, a, "/api/tickets")) != 3 {
		t.Fatal("bad seed")
	}
	v := ticket(t, a, "", nil)
	id := v["id"].(string)
	call(t, a, "POST", "/api/tickets/"+id+"/run", nil, 202)
	a.mu.Lock()
	tkt, err := a.store.ticket(id, false)
	if err != nil {
		t.Fatal(err)
	}
	tkt.Status = "running"
	if err = a.store.saveTicket(tkt); err != nil {
		t.Fatal(err)
	}
	a.mu.Unlock()
	dest := filepath.Join(t.TempDir(), "import.db")
	if err = ImportDatabase(a.config.Database, dest); err != nil {
		t.Fatal(err)
	}
	b, err := New(Config{Database: dest})
	if err != nil {
		t.Fatal(err)
	}
	defer b.Close()
	result := call(t, b, "GET", "/api/tickets/"+id, nil, 200)
	if result["status"] != "failed" {
		t.Fatal(result)
	}
	if err = ImportDatabase("missing", dest); err != nil {
		t.Fatal("existing destination overwritten", err)
	}
	if err = ImportDatabase("missing", filepath.Join(t.TempDir(), "new")); err == nil {
		t.Fatal("missing import ignored")
	}
	call(t, b, "PUT", "/api/project", map[string]any{"name": "Preserved", "description": "Details"}, 200)
	call(t, b, "GET", "/api/config", nil, 200)
}
func TestWorkspaceBoundaries(t *testing.T) {
	repo := repository(t)
	a := realApp(t, repo, "codex", codexSuccess)
	for _, body := range []map[string]any{{"name": "Outside", "path": "/etc"}, {"name": "Escape", "path": repo, "test_directory": ".."}, {"name": "Absolute", "path": repo, "test_directory": "/tmp"}, {"name": "Missing", "path": repo, "test_directory": "missing"}, {"name": "Invalid", "path": repo, "test_preset": "shell"}} {
		call(t, a, "POST", "/api/workspaces", body, 422)
	}
	call(t, a, "GET", "/api/tickets/missing/runs", nil, 404)
	call(t, a, "GET", "/api/runs/missing/events", nil, 404)
	v := ticket(t, a, "", map[string]any{"provider": "codex"})
	call(t, a, "POST", "/api/tickets/"+v["id"].(string)+"/run", nil, 409)
}
func TestLegacyMigration(t *testing.T) {
	path := filepath.Join(t.TempDir(), "legacy.db")
	s, err := openStore(path)
	if err != nil {
		t.Fatal(err)
	}
	_, err = s.db.Exec(`DELETE FROM projects;INSERT INTO project(id,data) VALUES(1,'{"name":"Legacy","description":"Keep"}')`)
	if err != nil {
		t.Fatal(err)
	}
	s.db.Close()
	a, err := New(Config{Database: path})
	if err != nil {
		t.Fatal(err)
	}
	defer a.Close()
	v := call(t, a, "GET", "/api/project", nil, 200)
	if v["name"] != "Legacy" {
		t.Fatal(v)
	}
}
func TestAdapterCommands(t *testing.T) {
	run := Run{Worktree: filepath.Join(t.TempDir(), "worktree"), Snapshot: defaultDraft()}
	run.Snapshot.Prompt = "Build"
	run.Snapshot.Images = []string{"data:image/png;base64,aW1hZ2U="}
	c := Config{Binaries: map[string]string{"codex": "codex", "claude": "claude"}, Keys: map[string]string{"codex": "secret", "claude": "secret"}}
	argv, input, err := codexCommand(c, run)
	if err != nil || !strings.Contains(strings.Join(argv, " "), "--image") || !strings.Contains(input, "Build") {
		t.Fatal(argv, input, err)
	}
	run.Snapshot.Permission = "workspace-write"
	argv, input, err = claudeCommand(c, run)
	if err != nil || !strings.Contains(strings.Join(argv, " "), "Edit,Write") {
		t.Fatal(argv, err)
	}
	var value map[string]any
	if err = json.Unmarshal([]byte(input), &value); err != nil {
		t.Fatal(err)
	}
	for _, provider := range []string{"codex", "claude", "tests"} {
		env, err := childEnvironment(c, provider, run)
		if err != nil {
			t.Fatal(err)
		}
		if provider == "tests" && strings.Contains(strings.Join(env, " "), "secret") {
			t.Fatal("tests inherit keys")
		}
	}
	result := agentResult{}
	if err = consumeEvent(c, "claude", &result, "stdout", `{"type":"result","result":"secret","permission_denials":[{}]}`, func(k, v string) error { return nil }); err != nil || result.failure == "" || result.response != "[redacted]" {
		t.Fatal(result, err)
	}
}
