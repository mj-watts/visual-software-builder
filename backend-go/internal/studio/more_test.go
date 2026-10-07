package studio

import (
	"context"
	"encoding/base64"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

func TestListsAssignmentsAndProjectUpdates(t *testing.T) {
	a := testApp(t)
	p := call(t, a, "POST", "/api/projects", map[string]any{"name": "Second"}, 201)
	pid := p["id"].(string)
	call(t, a, "PUT", "/api/projects/"+pid, map[string]any{"name": "Renamed", "description": "  Trimmed  "}, 200)
	call(t, a, "PUT", "/api/project", map[string]any{"name": "Default"}, 200)
	if len(list(t, a, "/api/projects")) != 2 {
		t.Fatal("project list")
	}
	call(t, a, "DELETE", "/api/projects/"+pid, nil, 200)
	if len(list(t, a, "/api/projects?include_deleted=true")) != 2 || len(list(t, a, "/api/projects")) != 1 {
		t.Fatal("deleted projects")
	}
	g := call(t, a, "POST", "/api/groups", map[string]any{"name": "Group"}, 201)
	gid := g["id"].(string)
	call(t, a, "PATCH", "/api/groups/"+gid, map[string]any{"name": "Rename", "color": "blue"}, 200)
	if len(list(t, a, "/api/groups")) != 1 {
		t.Fatal("group list")
	}
	v := ticket(t, a, "", nil)
	id := v["id"].(string)
	call(t, a, "PUT", "/api/tickets/"+id+"/group", map[string]any{"group_id": gid}, 200)
	call(t, a, "PUT", "/api/tickets/"+id+"/group", map[string]any{"group_id": ""}, 200)
	call(t, a, "PUT", "/api/tickets/"+id+"/group", map[string]any{"group_id": "missing"}, 404)
	for _, path := range []string{"/api/projects?include_deleted=bad", "/api/tickets?deleted=bad"} {
		call(t, a, "GET", path, nil, 422)
	}
	call(t, a, "GET", "/api/tickets?project_id=missing", nil, 404)
	call(t, a, "GET", "/api/groups?project_id=missing", nil, 404)
	call(t, a, "POST", "/api/groups/"+gid+"/run", nil, 409)
}
func TestInputBoundaries(t *testing.T) {
	a := testApp(t)
	for _, body := range []map[string]any{{"name": " "}, {"name": nil}, {"screenshot": nil}, {"name": strings.Repeat("x", 121)}, {"name": "Valid", "description": strings.Repeat("x", 2001)}, {"name": "Valid", "screenshot": "data:image/png;base64,!!!"}} {
		call(t, a, "POST", "/api/projects", body, 422)
	}
	for _, body := range []map[string]any{{"name": " "}, {"name": "Group", "color": "#xyzxyz"}, {"name": "Group", "color": "#123"}} {
		call(t, a, "POST", "/api/groups", body, 422)
	}
	call(t, a, "POST", "/api/tickets/delete", map[string]any{"ids": []string{}}, 422)
	v := ticket(t, a, "", nil)
	id := v["id"].(string)
	call(t, a, "PATCH", "/api/tickets/"+id, map[string]any{"images": []string{"data:image/png;base64,"}}, 422)
	call(t, a, "PATCH", "/api/tickets/"+id, map[string]any{"title": 1}, 422)
	call(t, a, "POST", "/api/tickets/delete", map[string]any{"ids": []string{id, id}, "project_id": "missing"}, 404)
	p := call(t, a, "POST", "/api/projects", map[string]any{"name": "Other"}, 201)
	call(t, a, "POST", "/api/tickets/delete", map[string]any{"ids": []string{id}, "project_id": p["id"]}, 422)
	call(t, a, "POST", "/api/tickets/delete", map[string]any{"ids": []string{id, id}}, 200)
	call(t, a, "POST", "/api/tickets/missing/restore", nil, 404)
	for _, image := range []string{"data:image/png;base64," + strings.Repeat("x", 6666669), "data:image/png;base64," + base64.StdEncoding.EncodeToString(make([]byte, 5000001))} {
		if validateImage(image) == nil {
			t.Fatal("oversize image")
		}
	}
	for _, color := range []string{"orange", "blue", "green", "amber", "plum", "#123ABC"} {
		if !validColor(color) {
			t.Fatal(color)
		}
	}
	r := localRequest("POST", "/api/projects")
	r.Body = osFileBody(t, "{} {}")
	w := httptest.NewRecorder()
	a.Handler().ServeHTTP(w, r)
	if w.Code != 422 {
		t.Fatal(w.Code)
	}
}
func osFileBody(t *testing.T, s string) *os.File {
	path := filepath.Join(t.TempDir(), "body")
	write(t, path, s)
	f, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { f.Close() })
	return f
}
func TestConfigurationAndLocalOrigins(t *testing.T) {
	for _, key := range []string{"CODEX_API_KEY", "OPENAI_API_KEY", "ANTHROPIC_API_KEY", "SWIMLANE_DB", "SWIMLANE_WORKSPACE_ROOTS", "SWIMLANE_ARTIFACTS", "SWIMLANE_RUN_TIMEOUT"} {
		t.Setenv(key, "")
	}
	bin := t.TempDir()
	write(t, filepath.Join(bin, "codex"), "#!/bin/sh\nexit 0\n")
	t.Setenv("PATH", bin)
	t.Setenv("OPENAI_API_KEY", "test-key")
	t.Setenv("SWIMLANE_RUN_TIMEOUT", "99999")
	t.Setenv("SWIMLANE_WORKSPACE_ROOTS", bin)
	t.Setenv("SWIMLANE_ARTIFACTS", bin)
	c := ConfigFromEnv()
	if !c.enabled("codex") || c.Timeout != time.Hour || len(c.Roots) != 1 {
		t.Fatal("configuration")
	}
	if c.providers()[1]["available"] != true {
		t.Fatal("provider")
	}
	path := filepath.Join(t.TempDir(), ".env")
	write(t, path, "# comment\nSWIMLANE_TEST_NEW='value'\nINVALID-NAME=ignored\n1BAD=ignored\nnot a pair\n")
	t.Cleanup(func() { os.Unsetenv("SWIMLANE_TEST_NEW") })
	if err := LoadEnv(path); err != nil {
		t.Fatal(err)
	}
	if os.Getenv("SWIMLANE_TEST_NEW") != "value" {
		t.Fatal("env parsing")
	}
	if err := LoadEnv("nonexistent-env"); err != nil {
		t.Fatal(err)
	}
	if !allowedOrigin("http://127.0.0.1:5173") || !allowedOrigin("http://localhost:8080") || allowedOrigin("https://evil:5173") || allowedOrigin("http://localhost:9999") {
		t.Fatal("origin rules")
	}
	a := testApp(t)
	a.config = c
	call(t, a, "GET", "/api/health", nil, 200)
	call(t, a, "GET", "/api/config", nil, 200)
}
func TestTestPresetsAndFailureGates(t *testing.T) {
	repo := repository(t)
	a := realApp(t, repo, "codex", `echo '{"type":"turn.completed"}'`)
	wid := workspace(t, a, repo, "none")
	first := ticket(t, a, "", map[string]any{"provider": "codex", "workspace_id": wid, "allow_tests": true})
	id := first["id"].(string)
	call(t, a, "POST", "/api/tickets/"+id+"/run", nil, 202)
	start(t, a)
	result := waitTicket(t, a, id, "failed")
	if !strings.Contains(result["response"].(string), "test preset") {
		t.Fatal(result)
	}
	bin := t.TempDir()
	write(t, filepath.Join(bin, "npm"), "#!/bin/sh\n[ -z \"$CODEX_API_KEY$OPENAI_API_KEY$ANTHROPIC_API_KEY\" ] || exit 99\nexit 0\n")
	t.Setenv("PATH", bin+string(os.PathListSeparator)+os.Getenv("PATH"))
	wid = workspace(t, a, repo, "vitest")
	second := ticket(t, a, "", map[string]any{"provider": "codex", "workspace_id": wid, "allow_tests": true})
	id = second["id"].(string)
	call(t, a, "POST", "/api/tickets/"+id+"/run", nil, 202)
	waitTicket(t, a, id, "done")
	write(t, filepath.Join(bin, "npm"), "#!/bin/sh\nexit 3\n")
	g := call(t, a, "POST", "/api/groups", map[string]any{"name": "Fail tests"}, 201)
	gid := g["id"].(string)
	first = ticket(t, a, "", map[string]any{"provider": "codex", "workspace_id": wid, "allow_tests": true, "group_id": gid})
	second = ticket(t, a, "", map[string]any{"provider": "codex", "workspace_id": wid, "allow_tests": true, "group_id": gid})
	w := httptest.NewRecorder()
	a.Handler().ServeHTTP(w, localRequest("POST", "/api/groups/"+gid+"/run"))
	if w.Code != 202 {
		t.Fatal(w.Body.String())
	}
	waitTicket(t, a, first["id"].(string), "failed")
	waitTicket(t, a, second["id"].(string), "cancelled")
	runs := list(t, a, "/api/tickets/"+first["id"].(string)+"/runs")
	if runs[0]["tests"] != "failed" {
		t.Fatal(runs)
	}
}
func TestRecoveryWithoutRunAndLegacyQueuedTicket(t *testing.T) {
	a := testApp(t)
	v := ticket(t, a, "", nil)
	id := v["id"].(string)
	a.mu.Lock()
	stored, err := a.store.ticket(id, false)
	if err != nil {
		t.Fatal(err)
	}
	stored.Status = "running"
	if err = a.store.saveTicket(stored); err != nil {
		t.Fatal(err)
	}
	if err = a.recover(); err != nil {
		t.Fatal(err)
	}
	stored.Status = "queued"
	if err = a.store.saveTicket(stored); err != nil {
		t.Fatal(err)
	}
	a.mu.Unlock()
	start(t, a)
	waitTicket(t, a, id, "done")
}
func TestProcessPumpAndResultErrors(t *testing.T) {
	err := pump(strings.NewReader("line\n"), "stdout", newAtomic(), func(k, v string) error { return context.Canceled })
	if err == nil {
		t.Fatal("emit failure ignored")
	}
	if processResult(context.Background(), context.Canceled, nil, nil) == nil || processResult(context.Background(), nil, context.Canceled, nil) == nil {
		t.Fatal("pump errors ignored")
	}
}

func newAtomic() *atomic.Int64 { return &atomic.Int64{} }

func TestCancellationReturnsPartialDiff(t *testing.T) {
	repo := repository(t)
	a := realApp(t, repo, "codex", "printf 'partial\\n' > old.txt; echo ready; sleep 10")
	wid := workspace(t, a, repo, "none")
	v := ticket(t, a, "", map[string]any{"provider": "codex", "workspace_id": wid, "permission": "workspace-write"})
	id := v["id"].(string)
	call(t, a, "POST", "/api/tickets/"+id+"/run", nil, 202)
	start(t, a)
	deadline := time.Now().Add(3 * time.Second)
	ready := false
	for time.Now().Before(deadline) {
		runs := list(t, a, "/api/tickets/"+id+"/runs")
		events := list(t, a, "/api/runs/"+runs[0]["id"].(string)+"/events")
		for _, e := range events {
			if e["message"] == "ready" {
				ready = true
			}
		}
		if ready {
			break
		}
		time.Sleep(5 * time.Millisecond)
	}
	if !ready {
		t.Fatal("agent did not start")
	}
	result := call(t, a, "POST", "/api/tickets/"+id+"/cancel", nil, 200)
	if result["status"] != "cancelled" || len(result["files"].([]any)) != 1 {
		t.Fatal(result)
	}
	call(t, a, "POST", "/api/tickets/"+id+"/cancel", nil, 409)
}
func TestQueuedTimestampOrdering(t *testing.T) {
	if !queuedBefore("2026-10-07T10:00:00Z", "2026-10-07T10:00:00.001+00:00") {
		t.Fatal("fractional timestamps reorder FIFO")
	}
	if !queuedBefore("bad-a", "bad-b") {
		t.Fatal("legacy fallback")
	}
}
