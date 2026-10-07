package studio

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log"
	"net"
	"net/http"
	"reflect"
	"strconv"
	"strings"
	"sync"
)

type App struct {
	mu     sync.Mutex
	store  *Store
	config Config
	mux    *http.ServeMux
	active map[string]context.CancelFunc
	done   map[string]chan struct{}
	worker sync.WaitGroup
	stop   context.CancelFunc
	closed bool
}

func New(c Config) (*App, error) {
	c.defaults()
	s, err := openStore(c.Database)
	if err != nil {
		return nil, err
	}
	a := &App{store: s, config: c, mux: http.NewServeMux(), active: map[string]context.CancelFunc{}, done: map[string]chan struct{}{}}
	if err = a.recover(); err != nil {
		s.db.Close()
		return nil, err
	}
	a.routes()
	return a, nil
}
func (a *App) Close() error {
	a.mu.Lock()
	a.closed = true
	if a.stop != nil {
		a.stop()
	}
	for _, cancel := range a.active {
		cancel()
	}
	a.mu.Unlock()
	a.worker.Wait()
	return a.store.db.Close()
}

type endpoint func(*http.Request) (any, error)

func (a *App) route(pattern string, status int, fn endpoint) {
	a.mux.HandleFunc(pattern, func(w http.ResponseWriter, r *http.Request) {
		a.mu.Lock()
		value, err := fn(r)
		a.mu.Unlock()
		if err != nil {
			writeError(w, err)
			return
		}
		writeJSON(w, status, value)
	})
}
func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(value); err != nil {
		log.Printf("write response: %v", err)
	}
}
func writeError(w http.ResponseWriter, err error) {
	code := 500
	message := "Internal server error"
	var ae *apiError
	if errors.As(err, &ae) {
		code = ae.Code
		message = ae.Message
	} else {
		log.Printf("API error: %v", err)
	}
	writeJSON(w, code, map[string]string{"detail": message})
}
func decode(r *http.Request, value any) error {
	d := json.NewDecoder(io.LimitReader(r.Body, 41000000))
	var raw json.RawMessage
	if err := d.Decode(&raw); err != nil {
		return problem(422, "Invalid request body")
	}
	var extra any
	if d.Decode(&extra) != io.EOF {
		return problem(422, "Invalid request body")
	}
	if string(raw) == "null" {
		return problem(422, "Invalid request body")
	}
	if err := rejectNullFields(raw, reflect.TypeOf(value).Elem()); err != nil {
		return err
	}
	if err := json.Unmarshal(raw, value); err != nil {
		return problem(422, "Invalid request body")
	}
	return nil
}
func rejectNullFields(raw []byte, kind reflect.Type) error {
	if kind.Kind() != reflect.Struct {
		return nil
	}
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(raw, &fields); err != nil {
		return problem(422, "Invalid request body")
	}
	for i := 0; i < kind.NumField(); i++ {
		field := kind.Field(i)
		key := strings.Split(field.Tag.Get("json"), ",")[0]
		if string(fields[key]) == "null" {
			return problem(422, "Field "+key+" cannot be null")
		}
	}
	return nil
}

func (a *App) Handler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !localHost(r.Host) {
			writeError(w, problem(400, "Invalid host header"))
			return
		}
		if !allowedOrigin(r.Header.Get("Origin")) && r.Method != "GET" {
			writeError(w, problem(403, "Cross-origin execution requests are blocked."))
			return
		}
		a.mux.ServeHTTP(w, r)
	})
}
func localHost(host string) bool {
	if h, _, err := net.SplitHostPort(host); err == nil {
		host = h
	}
	return host == "localhost" || host == "127.0.0.1" || host == "testserver"
}

func allowedOrigin(origin string) bool {
	return origin == "" || strings.HasPrefix(origin, "http://127.0.0.1:") && allowedPort(origin) || strings.HasPrefix(origin, "http://localhost:") && allowedPort(origin)
}
func allowedPort(origin string) bool {
	for _, p := range []string{":5173", ":5174", ":8000", ":8001", ":8080", ":8081"} {
		if strings.HasSuffix(origin, p) {
			return true
		}
	}
	return false
}
func projectID(r *http.Request) string {
	if id := r.URL.Query().Get("project_id"); id != "" {
		return id
	}
	return "default"
}
func queryBool(r *http.Request, key string) (bool, error) {
	v := r.URL.Query().Get(key)
	if v == "" {
		return false, nil
	}
	b, err := strconv.ParseBool(v)
	if err != nil {
		return false, problem(422, "Invalid boolean query parameter")
	}
	return b, nil
}
func (a *App) project(id string, deleted bool) (ManagedProject, error) {
	v, err := oneJSON[ManagedProject](a.store, `SELECT data FROM projects WHERE id=?`, id)
	if err != nil {
		return v, err
	}
	if v.Deleted && !deleted {
		return v, missing("Project")
	}
	return v, nil
}
func (a *App) ticket(id string) (Ticket, error) {
	t, err := a.store.ticket(id, false)
	if err != nil {
		return t, err
	}
	_, err = a.project(t.ProjectID, false)
	return t, err
}
func (a *App) group(id string) (Group, error) {
	g, err := oneJSON[Group](a.store, `SELECT data FROM groups WHERE id=?`, id)
	if err != nil {
		return g, err
	}
	if g.ProjectID == "" {
		g.ProjectID = "default"
	}
	_, err = a.project(g.ProjectID, false)
	return g, err
}
func (a *App) validateGroup(id, project string) error {
	if id == "" {
		return nil
	}
	g, err := a.group(id)
	if err != nil {
		return err
	}
	if g.ProjectID != project {
		return problem(422, "Group belongs to another project")
	}
	return nil
}
func (a *App) routes() {
	a.projectRoutes()
	a.ticketRoutes()
	a.groupRoutes()
	a.runRoutes()
	a.docsRoutes()
	a.route("GET /api/health", 200, func(r *http.Request) (any, error) {
		mode := "demo"
		if a.config.enabled("codex") || a.config.enabled("claude") {
			mode = "agents"
		}
		return map[string]string{"status": "ok", "mode": mode, "backend": "go"}, nil
	})
	a.route("GET /api/config", 200, func(r *http.Request) (any, error) {
		return map[string]any{"providers": a.config.providers(), "workspace_roots": a.config.Roots, "timeout_seconds": a.config.Timeout.Seconds()}, nil
	})
	a.route("GET /api/workspaces", 200, func(r *http.Request) (any, error) {
		return queryJSON[Workspace](a.store, `SELECT data FROM workspaces ORDER BY id`)
	})
	a.route("POST /api/workspaces", 201, a.registerWorkspace)
	a.mux.HandleFunc("GET /api/events", a.eventStream)
}
