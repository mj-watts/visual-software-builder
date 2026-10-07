package studio

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/mattn/go-sqlite3"
	"os"
	"path/filepath"
	"strings"
)

type Store struct{ db *sql.DB }

func openStore(path string) (*Store, error) {
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return nil, err
	}
	db, err := sql.Open("sqlite3", path+"?_busy_timeout=5000&_foreign_keys=on")
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(1)
	s := &Store{db}
	if err = s.migrate(); err != nil {
		db.Close()
		return nil, err
	}
	return s, nil
}

var schema = []string{
	`CREATE TABLE IF NOT EXISTS tickets(number INTEGER PRIMARY KEY AUTOINCREMENT,data TEXT NOT NULL)`,
	`CREATE TABLE IF NOT EXISTS project(id INTEGER PRIMARY KEY CHECK(id=1),data TEXT NOT NULL)`,
	`CREATE TABLE IF NOT EXISTS projects(id TEXT PRIMARY KEY,data TEXT NOT NULL)`,
	`CREATE TABLE IF NOT EXISTS groups(id TEXT PRIMARY KEY,data TEXT NOT NULL)`,
	`CREATE TABLE IF NOT EXISTS workspaces(id TEXT PRIMARY KEY,data TEXT NOT NULL)`,
	`CREATE TABLE IF NOT EXISTS runs(id TEXT PRIMARY KEY,ticket_id TEXT NOT NULL,data TEXT NOT NULL)`,
	`CREATE TABLE IF NOT EXISTS events(id INTEGER PRIMARY KEY AUTOINCREMENT,run_id TEXT,ticket_id TEXT,type TEXT,message TEXT,created_at TEXT)`,
	`CREATE INDEX IF NOT EXISTS run_ticket ON runs(ticket_id)`, `CREATE INDEX IF NOT EXISTS event_run ON events(run_id)`,
}

func (s *Store) migrate() error {
	for _, q := range schema {
		if _, err := s.db.Exec(q); err != nil {
			return err
		}
	}
	var n int
	if err := s.db.QueryRow(`SELECT count(*) FROM pragma_table_info('tickets') WHERE name='deleted'`).Scan(&n); err != nil {
		return err
	}
	if n == 0 {
		if _, err := s.db.Exec(`ALTER TABLE tickets ADD COLUMN deleted INTEGER NOT NULL DEFAULT 0`); err != nil {
			return err
		}
	}
	return s.migrateProject()
}
func (s *Store) migrateProject() error {
	var n int
	if err := s.db.QueryRow(`SELECT count(*) FROM projects`).Scan(&n); err != nil {
		return err
	}
	if n > 0 {
		return nil
	}
	p := ManagedProject{Project: Project{Name: "My first project"}, ID: "default"}
	var data string
	err := s.db.QueryRow(`SELECT data FROM project WHERE id=1`).Scan(&data)
	if err == nil {
		if err = json.Unmarshal([]byte(data), &p.Project); err != nil {
			return err
		}
	} else if !errors.Is(err, sql.ErrNoRows) {
		return err
	}
	return s.saveProject(p)
}

// ImportDatabase uses SQLite's online backup API, including committed WAL pages.
// An existing destination is never overwritten. Source data is opened read-only.
func ImportDatabase(source, dest string) error {
	if _, err := os.Stat(dest); err == nil {
		return nil
	} else if !os.IsNotExist(err) {
		return err
	}
	if _, err := os.Stat(source); err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(dest), 0700); err != nil {
		return err
	}
	src, err := sql.Open("sqlite3", "file:"+source+"?mode=ro&_busy_timeout=5000")
	if err != nil {
		return err
	}
	defer src.Close()
	dst, err := sql.Open("sqlite3", dest)
	if err != nil {
		return err
	}
	defer dst.Close()
	err = backupConnections(src, dst)
	if err != nil {
		os.Remove(dest)
		return err
	}
	return os.Chmod(dest, 0600)
}
func backupConnections(src, dst *sql.DB) error {
	ctx := context.Background()
	sc, err := src.Conn(ctx)
	if err != nil {
		return err
	}
	defer sc.Close()
	dc, err := dst.Conn(ctx)
	if err != nil {
		return err
	}
	defer dc.Close()
	return sc.Raw(func(source any) error {
		return dc.Raw(func(destination any) error {
			backup, err := destination.(*sqlite3.SQLiteConn).Backup("main", source.(*sqlite3.SQLiteConn), "main")
			if err != nil {
				return err
			}
			done, stepErr := backup.Step(-1)
			finishErr := backup.Finish()
			if stepErr != nil {
				return stepErr
			}
			if !done {
				return errors.New("database backup could not complete; source may be busy")
			}
			return finishErr
		})
	})
}
func queryJSON[T any](s *Store, q string, args ...any) ([]T, error) {
	rows, err := s.db.Query(q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	values := []T{}
	for rows.Next() {
		var raw string
		if err = rows.Scan(&raw); err != nil {
			return nil, err
		}
		var v T
		if err = json.Unmarshal([]byte(raw), &v); err != nil {
			return nil, err
		}
		values = append(values, v)
	}
	return values, rows.Err()
}
func oneJSON[T any](s *Store, q string, args ...any) (T, error) {
	var v T
	var raw string
	err := s.db.QueryRow(q, args...).Scan(&raw)
	if errors.Is(err, sql.ErrNoRows) {
		return v, missing("Record")
	}
	if err != nil {
		return v, err
	}
	err = json.Unmarshal([]byte(raw), &v)
	return v, err
}
func execJSON(exec interface {
	Exec(string, ...any) (sql.Result, error)
}, q string, value any, args ...any) error {
	data, err := json.Marshal(value)
	if err != nil {
		return err
	}
	_, err = exec.Exec(q, append([]any{string(data)}, args...)...)
	return err
}
func (s *Store) saveProject(v ManagedProject) error {
	return execJSON(s.db, `INSERT INTO projects(data,id) VALUES(?,?) ON CONFLICT(id) DO UPDATE SET data=excluded.data`, v, v.ID)
}
func (s *Store) saveGroup(v Group) error {
	return execJSON(s.db, `INSERT INTO groups(data,id) VALUES(?,?) ON CONFLICT(id) DO UPDATE SET data=excluded.data`, v, v.ID)
}
func (s *Store) saveWorkspace(v Workspace) error {
	return execJSON(s.db, `INSERT INTO workspaces(data,id) VALUES(?,?) ON CONFLICT(id) DO UPDATE SET data=excluded.data`, v, v.ID)
}
func (s *Store) saveTicket(v Ticket) error {
	return execJSON(s.db, `UPDATE tickets SET data=? WHERE number=?`, v, strings.TrimPrefix(v.ID, "SW-"))
}
func (s *Store) saveRun(v Run) error {
	return execJSON(s.db, `INSERT INTO runs(data,id,ticket_id) VALUES(?,?,?) ON CONFLICT(id) DO UPDATE SET data=excluded.data`, v, v.ID, v.TicketID)
}
func (s *Store) tickets(project string, deleted bool) ([]Ticket, error) {
	values, err := queryJSON[Ticket](s, `SELECT data FROM tickets WHERE deleted=? ORDER BY number`, deleted)
	out := []Ticket{}
	for _, v := range values {
		normalizeTicket(&v)
		if project == "" || v.ProjectID == project {
			out = append(out, v)
		}
	}
	return out, err
}
func normalizeTicket(v *Ticket) {
	if v.ProjectID == "" {
		v.ProjectID = "default"
	}
	if v.Permission == "" {
		v.Permission = "read-only"
	}
	if v.Provider == "" {
		v.Provider = "demo"
	}
	if v.Images == nil {
		v.Images = []string{}
	}
	if v.Files == nil {
		v.Files = []ChangedFile{}
	}
}
func (s *Store) ticket(id string, deleted bool) (Ticket, error) {
	v, err := oneJSON[Ticket](s, `SELECT data FROM tickets WHERE number=? AND deleted=?`, strings.TrimPrefix(id, "SW-"), deleted)
	normalizeTicket(&v)
	return v, err
}
func (s *Store) create(d Draft, project string) (Ticket, error) {
	tx, err := s.db.Begin()
	if err != nil {
		return Ticket{}, err
	}
	defer tx.Rollback()
	result, err := tx.Exec(`INSERT INTO tickets(data) VALUES('{}')`)
	if err != nil {
		return Ticket{}, err
	}
	n, err := result.LastInsertId()
	if err != nil {
		return Ticket{}, err
	}
	v := Ticket{Draft: d, ID: fmt.Sprintf("SW-%03d", n), ProjectID: project, Status: "todo", Files: []ChangedFile{}, CreatedAt: now()}
	if err = execJSON(tx, `UPDATE tickets SET data=? WHERE number=?`, v, n); err != nil {
		return v, err
	}
	return v, tx.Commit()
}
func (s *Store) setDeleted(ids []string, deleted bool) error {
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	for _, id := range ids {
		if _, err = tx.Exec(`UPDATE tickets SET deleted=? WHERE number=?`, deleted, strings.TrimPrefix(id, "SW-")); err != nil {
			return err
		}
	}
	return tx.Commit()
}
func (s *Store) enqueue(runs []Run, tickets []Ticket) error {
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	for i, run := range runs {
		if err = execJSON(tx, `INSERT INTO runs(data,id,ticket_id) VALUES(?,?,?)`, run, run.ID, run.TicketID); err != nil {
			return err
		}
		if err = execJSON(tx, `UPDATE tickets SET data=? WHERE number=?`, tickets[i], strings.TrimPrefix(tickets[i].ID, "SW-")); err != nil {
			return err
		}
	}
	return tx.Commit()
}
func (s *Store) event(run Run, kind, message string) error {
	var n int
	if err := s.db.QueryRow(`SELECT count(*) FROM events WHERE run_id=?`, run.ID).Scan(&n); err != nil {
		return err
	}
	if n >= 1000 && kind != "status" {
		return nil
	}
	chars := []rune(message)
	if len(chars) > 10000 {
		message = string(chars[:10000])
	}
	_, err := s.db.Exec(`INSERT INTO events(run_id,ticket_id,type,message,created_at) VALUES(?,?,?,?,?)`, run.ID, run.TicketID, kind, message, now())
	return err
}
func (s *Store) events(after int, run string) ([]RunEvent, error) {
	rows, err := s.db.Query(`SELECT id,run_id,ticket_id,type,message,created_at FROM events WHERE id>? AND (?='' OR run_id=?) ORDER BY id LIMIT 1000`, after, run, run)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []RunEvent{}
	for rows.Next() {
		var e RunEvent
		if err = rows.Scan(&e.ID, &e.RunID, &e.TicketID, &e.Type, &e.Message, &e.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, e)
	}
	return out, rows.Err()
}
func (s *Store) removeGroup(id string) error {
	values, err := queryJSON[Ticket](s, `SELECT data FROM tickets ORDER BY number`)
	if err != nil {
		return err
	}
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	for _, v := range values {
		if v.GroupID != id {
			continue
		}
		v.GroupID = ""
		if err = execJSON(tx, `UPDATE tickets SET data=? WHERE number=?`, v, strings.TrimPrefix(v.ID, "SW-")); err != nil {
			return err
		}
	}
	if _, err = tx.Exec(`DELETE FROM groups WHERE id=?`, id); err != nil {
		return err
	}
	return tx.Commit()
}
