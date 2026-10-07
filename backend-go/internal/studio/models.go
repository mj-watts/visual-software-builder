package studio

import (
	"crypto/rand"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"slices"
	"strings"
	"time"
	"unicode/utf8"
)

type Draft struct {
	Title       string   `json:"title"`
	Prompt      string   `json:"prompt"`
	Provider    string   `json:"provider"`
	Images      []string `json:"images"`
	WorkspaceID string   `json:"workspace_id"`
	Permission  string   `json:"permission"`
	AllowTests  bool     `json:"allow_tests"`
	GroupID     string   `json:"group_id"`
}
type ChangedFile struct {
	Path      string `json:"path"`
	Additions int    `json:"additions"`
	Deletions int    `json:"deletions"`
	Diff      string `json:"diff"`
}
type Ticket struct {
	Draft
	ID          string        `json:"id"`
	ProjectID   string        `json:"project_id"`
	Status      string        `json:"status"`
	Response    string        `json:"response"`
	Files       []ChangedFile `json:"files"`
	CreatedAt   string        `json:"created_at"`
	QueuedAt    string        `json:"queued_at"`
	LatestRunID string        `json:"latest_run_id"`
}
type Project struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	Screenshot  string `json:"screenshot"`
}
type ManagedProject struct {
	Project
	ID      string `json:"id"`
	Deleted bool   `json:"deleted"`
}
type Group struct {
	Name      string `json:"name"`
	Color     string `json:"color"`
	ID        string `json:"id"`
	ProjectID string `json:"project_id"`
}
type Workspace struct {
	Path          string `json:"path"`
	Name          string `json:"name"`
	TestPreset    string `json:"test_preset"`
	TestDirectory string `json:"test_directory"`
	ID            string `json:"id"`
}
type Run struct {
	ID            string        `json:"id"`
	TicketID      string        `json:"ticket_id"`
	Snapshot      Draft         `json:"snapshot"`
	Status        string        `json:"status"`
	GroupRunID    string        `json:"group_run_id"`
	PreviousRunID string        `json:"previous_run_id"`
	BaseCommit    string        `json:"base_commit"`
	WorkspacePath string        `json:"workspace_path"`
	Worktree      string        `json:"worktree"`
	TestPreset    string        `json:"test_preset"`
	TestDirectory string        `json:"test_directory"`
	CreatedAt     string        `json:"created_at"`
	StartedAt     string        `json:"started_at"`
	FinishedAt    string        `json:"finished_at"`
	Response      string        `json:"response"`
	Files         []ChangedFile `json:"files"`
	Error         string        `json:"error"`
	Tests         string        `json:"tests"`
}
type RunEvent struct {
	ID        int    `json:"id"`
	RunID     string `json:"run_id"`
	TicketID  string `json:"ticket_id"`
	Type      string `json:"type"`
	Message   string `json:"message"`
	CreatedAt string `json:"created_at"`
}

func now() string { return time.Now().UTC().Format(time.RFC3339Nano) }
func newID() string {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		panic(err)
	}
	return hex.EncodeToString(b[:])
}
func defaultDraft() Draft {
	return Draft{Provider: "demo", Permission: "read-only", Images: []string{}}
}
func textLength(value string, min, max int) bool {
	n := utf8.RuneCountInString(value)
	return n >= min && n <= max
}
func validateText(value string, min, max int) error {
	if !textLength(value, min, max) || strings.TrimSpace(value) == "" {
		return problem(422, "Invalid or blank text field")
	}
	return nil
}
func validateImage(image string) error {
	header, encoded, ok := strings.Cut(image, ",")
	if !ok || !slices.Contains([]string{"data:image/png;base64", "data:image/jpeg;base64", "data:image/webp;base64"}, header) {
		return problem(422, "Use PNG, JPEG or WebP images")
	}
	if len(encoded) > 6666668 {
		return problem(422, "Image exceeds 5 MB")
	}
	decoded, err := base64.StdEncoding.Strict().DecodeString(encoded)
	if err != nil || len(decoded) == 0 || len(decoded) > 5000000 {
		return problem(422, "Image is invalid, empty or exceeds 5 MB")
	}
	return nil
}
func (d *Draft) validate() error {
	if err := validateText(d.Title, 1, 160); err != nil {
		return err
	}
	if err := validateText(d.Prompt, 1, 20000); err != nil {
		return err
	}
	return d.validateOptions()
}
func (d *Draft) validateOptions() error {
	if !slices.Contains([]string{"demo", "codex", "claude"}, d.Provider) {
		return problem(422, "Invalid provider")
	}
	if !slices.Contains([]string{"read-only", "workspace-write"}, d.Permission) {
		return problem(422, "Invalid permission")
	}
	if len(d.Images) > 6 || !textLength(d.GroupID, 0, 80) {
		return problem(422, "Too many images or invalid group identifier")
	}
	for _, image := range d.Images {
		if err := validateImage(image); err != nil {
			return err
		}
	}
	d.Title = strings.TrimSpace(d.Title)
	d.Prompt = strings.TrimSpace(d.Prompt)
	return nil
}
func (p *Project) validate() error {
	if err := validateText(p.Name, 1, 120); err != nil {
		return err
	}
	if !textLength(p.Description, 0, 2000) {
		return problem(422, "Description exceeds 2000 characters")
	}
	if p.Screenshot != "" {
		if err := validateImage(p.Screenshot); err != nil {
			return err
		}
	}
	p.Name = strings.TrimSpace(p.Name)
	p.Description = strings.TrimSpace(p.Description)
	return nil
}
func (g *Group) validate() error {
	if err := validateText(g.Name, 1, 80); err != nil {
		return err
	}
	if !validColor(g.Color) {
		return problem(422, "Invalid group colour")
	}
	g.Name = strings.TrimSpace(g.Name)
	return nil
}
func validColor(color string) bool {
	if slices.Contains([]string{"orange", "blue", "green", "amber", "plum"}, color) {
		return true
	}
	if len(color) != 7 || color[0] != '#' {
		return false
	}
	_, err := hex.DecodeString(color[1:])
	return err == nil
}
func idle(t Ticket) error {
	if t.Status == "queued" || t.Status == "running" {
		return problem(409, "Cancel queued or running tickets before deleting.")
	}
	return nil
}
func editable(t Ticket) error {
	if t.Status != "todo" {
		return problem(409, "Only Todo tickets can be edited or started")
	}
	return nil
}

type apiError struct {
	Code    int
	Message string
}

func (e *apiError) Error() string            { return e.Message }
func problem(code int, message string) error { return &apiError{code, message} }
func missing(what string) error              { return problem(404, fmt.Sprintf("%s not found", what)) }
