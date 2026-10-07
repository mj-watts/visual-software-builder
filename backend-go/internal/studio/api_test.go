package studio

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"
	"time"
)

func testApp(t *testing.T) *App {
	t.Helper()
	a, err := New(Config{Database: filepath.Join(t.TempDir(), "test.sqlite3"), Roots: []string{t.TempDir()}, Artifacts: t.TempDir(), Timeout: time.Second, DemoDelay: time.Millisecond})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { a.Close() })
	return a
}
func call(t *testing.T, a *App, method, path string, body any, status int) map[string]any {
	t.Helper()
	b, _ := json.Marshal(body)
	r := httptest.NewRequest(method, path, bytes.NewReader(b))
	r.Host = "testserver"
	w := httptest.NewRecorder()
	a.Handler().ServeHTTP(w, r)
	if w.Code != status {
		t.Fatalf("%s %s got %d: %s", method, path, w.Code, w.Body.String())
	}
	var value map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &value); err != nil {
		t.Fatal(err)
	}
	return value
}
func list(t *testing.T, a *App, path string) []map[string]any {
	t.Helper()
	w := httptest.NewRecorder()
	a.Handler().ServeHTTP(w, localRequest("GET", path))
	if w.Code != 200 {
		t.Fatalf("%s: %s", path, w.Body.String())
	}
	var v []map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &v); err != nil {
		t.Fatal(err)
	}
	return v
}
func ticket(t *testing.T, a *App, query string, extra map[string]any) map[string]any {
	b := map[string]any{"title": "Task", "prompt": "Build something"}
	for k, v := range extra {
		b[k] = v
	}
	return call(t, a, "POST", "/api/tickets"+query, b, 201)
}
func TestProjectsTicketsAndGroups(t *testing.T) {
	a := testApp(t)
	p := call(t, a, "POST", "/api/projects", map[string]any{"name": "Second", "screenshot": "data:image/png;base64,aW1hZ2U="}, 201)
	pid := p["id"].(string)
	q := "?project_id=" + pid
	g := call(t, a, "POST", "/api/groups"+q, map[string]any{"name": "Navigation", "color": "#0a7542"}, 201)
	gid := g["id"].(string)
	v := ticket(t, a, q, map[string]any{"group_id": gid})
	id := v["id"].(string)
	if len(list(t, a, "/api/tickets")) != 0 {
		t.Fatal("project leak")
	}
	call(t, a, "POST", "/api/tickets", map[string]any{"title": "Wrong", "prompt": "Build", "group_id": gid}, 422)
	call(t, a, "PATCH", "/api/tickets/"+id, map[string]any{"title": "Renamed"}, 200)
	call(t, a, "DELETE", "/api/tickets/"+id, nil, 200)
	call(t, a, "DELETE", "/api/groups/"+gid, nil, 200)
	restored := call(t, a, "POST", "/api/tickets/"+id+"/restore", nil, 200)
	if restored["group_id"] != "" || restored["title"] != "Renamed" {
		t.Fatal(restored)
	}
	call(t, a, "DELETE", "/api/projects/"+pid, nil, 200)
	call(t, a, "GET", "/api/tickets/"+id, nil, 404)
	restored = call(t, a, "POST", "/api/projects/"+pid+"/restore", nil, 200)
	if restored["screenshot"] != p["screenshot"] {
		t.Fatal("lost screenshot")
	}
}
func TestValidationAndAtomicDeletion(t *testing.T) {
	a := testApp(t)
	for _, extra := range []map[string]any{{"title": " "}, {"provider": "bad"}, {"images": []string{"data:image/svg+xml;base64,eA=="}}, {"permission": "root"}, {"images": nil}, {"provider": nil}} {
		b := map[string]any{"title": "Task", "prompt": "Build"}
		for k, v := range extra {
			b[k] = v
		}
		call(t, a, "POST", "/api/tickets", b, 422)
	}
	v := ticket(t, a, "", nil)
	id := v["id"].(string)
	call(t, a, "POST", "/api/tickets/delete", map[string]any{"ids": []string{id, "missing"}}, 404)
	if len(list(t, a, "/api/tickets")) != 1 {
		t.Fatal("partial delete")
	}
	call(t, a, "POST", "/api/tickets/delete", map[string]any{"ids": []string{id}}, 200)
	if len(list(t, a, "/api/tickets?deleted=true")) != 1 {
		t.Fatal("not recoverable")
	}
	call(t, a, "POST", "/api/tickets/"+id+"/restore", nil, 200)
	call(t, a, "POST", "/api/tickets/"+id+"/run", nil, 202)
	call(t, a, "PATCH", "/api/tickets/"+id, map[string]any{"prompt": "Changed"}, 409)
	call(t, a, "DELETE", "/api/projects/default", nil, 409)
	call(t, a, "POST", "/api/tickets/"+id+"/cancel", nil, 200)
}
func TestSequentialDemoAndHistory(t *testing.T) {
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
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go a.Run(ctx)
	for _, v := range []map[string]any{first, second} {
		id := v["id"].(string)
		waitTicket(t, a, id, "done")
		runs := list(t, a, "/api/tickets/"+id+"/runs")
		if runs[0]["tests"] != "simulated" {
			t.Fatal(runs)
		}
		if len(list(t, a, "/api/runs/"+runs[0]["id"].(string)+"/events")) < 3 {
			t.Fatal("missing events")
		}
	}
	runs := list(t, a, "/api/tickets/"+second["id"].(string)+"/runs")
	previous := list(t, a, "/api/tickets/"+first["id"].(string)+"/runs")
	if runs[0]["previous_run_id"] != previous[0]["id"] {
		t.Fatal("missing dependency")
	}
}
func waitTicket(t *testing.T, a *App, id, status string) map[string]any {
	t.Helper()
	deadline := time.Now().Add(4 * time.Second)
	for time.Now().Before(deadline) {
		v := call(t, a, "GET", "/api/tickets/"+id, nil, 200)
		if v["status"] == status {
			return v
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Fatal("worker did not reach " + status)
	return nil
}
func TestSecurityAndOpenAPI(t *testing.T) {
	a := testApp(t)
	for _, h := range []struct {
		key, value string
		status     int
	}{{"Origin", "https://evil.test", 403}, {"Host", "evil.test", 400}} {
		r := httptest.NewRequest("POST", "/api/tickets", bytes.NewBufferString("{}"))
		r.Host = "testserver"
		if h.key == "Host" {
			r.Host = h.value
		} else {
			r.Header.Set(h.key, h.value)
		}
		w := httptest.NewRecorder()
		a.Handler().ServeHTTP(w, r)
		if w.Code != h.status {
			t.Fatal(w.Code)
		}
	}
	schema := call(t, a, "GET", "/openapi.json", nil, 200)
	paths := schema["paths"].(map[string]any)
	if len(paths) < 20 {
		t.Fatal("missing API docs")
	}
	call(t, a, "GET", "/api/health", nil, 200)
	v := ticket(t, a, "", map[string]any{"provider": "codex"})
	call(t, a, "POST", "/api/tickets/"+v["id"].(string)+"/run", nil, 409)
}

func localRequest(method, path string) *http.Request {
	r := httptest.NewRequest(method, path, nil)
	r.Host = "testserver"
	return r
}
