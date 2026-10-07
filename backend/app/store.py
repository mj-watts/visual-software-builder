import sqlite3
from datetime import datetime, timezone
from .models import Ticket, Draft

def now():
    return datetime.now(timezone.utc).isoformat()

class Store:
    def __init__(self, path):
        self.db = sqlite3.connect(str(path), check_same_thread=False)
        self.db.execute('CREATE TABLE IF NOT EXISTS tickets (number INTEGER PRIMARY KEY AUTOINCREMENT, data TEXT NOT NULL)')
        self.db.execute('CREATE TABLE IF NOT EXISTS project (id INTEGER PRIMARY KEY CHECK(id=1), data TEXT NOT NULL)')
        self.db.execute('CREATE TABLE IF NOT EXISTS groups (id TEXT PRIMARY KEY, data TEXT NOT NULL)')
        self.db.execute('CREATE TABLE IF NOT EXISTS workspaces (id TEXT PRIMARY KEY, data TEXT NOT NULL)')
        self.db.execute('CREATE TABLE IF NOT EXISTS runs (id TEXT PRIMARY KEY, ticket_id TEXT NOT NULL, data TEXT NOT NULL)')
        self.db.execute('CREATE TABLE IF NOT EXISTS events (id INTEGER PRIMARY KEY AUTOINCREMENT, run_id TEXT, ticket_id TEXT, type TEXT, message TEXT, created_at TEXT)')
        self.db.execute('CREATE INDEX IF NOT EXISTS run_ticket ON runs(ticket_id)')
        self.db.execute('CREATE INDEX IF NOT EXISTS event_run ON events(run_id)')
        self.db.commit()
        from .project_migration import migrate_schema, migrate_project
        migrate_schema(self.db)
        migrate_project(self.db)

    def all(self, deleted=False, project_id=None):
        tickets = [Ticket.model_validate_json(row[0]) for row in self.db.execute('SELECT data FROM tickets WHERE deleted=? ORDER BY number', (int(deleted),))]
        return [t for t in tickets if project_id is None or t.project_id == project_id]

    def get(self, id, deleted=False):
        row = self.db.execute('SELECT data FROM tickets WHERE number=? AND deleted=?', (id.removeprefix('SW-'), int(deleted))).fetchone()
        return Ticket.model_validate_json(row[0]) if row else None

    def create(self, draft: Draft, project_id='default'):
        cursor = self.db.execute("INSERT INTO tickets(data) VALUES ('{}')")
        ticket = Ticket(**draft.model_dump(), id=f'SW-{cursor.lastrowid:03}', created_at=now(), project_id=project_id)
        self.save(ticket)
        return ticket

    def save(self, ticket, commit=True):
        self.db.execute('UPDATE tickets SET data=? WHERE number=?', (ticket.model_dump_json(), ticket.id.removeprefix('SW-')))
        if commit:
            self.db.commit()
        return ticket

    def recover(self):
        for ticket in self.all():
            if ticket.status == 'running':
                ticket.status = 'failed'
                ticket.response = 'Run interrupted by server restart. Retry this ticket to start a fresh attempt.'
                self.save(ticket)
                self.recover_run(ticket)

    def recover_run(self, ticket):
        run = self.run(ticket.latest_run_id)
        if run:
            run.status = 'failed'
            run.error = ticket.response
            run.response = ticket.response
            run.finished_at = now()
            self.save_run(run)
            self.event(run, 'status', ticket.response)

    def next(self):
        queued = sorted((t for t in self.all() if t.status == 'queued'), key=lambda t: t.queued_at)
        return next(iter(queued), None)

    def close(self):
        self.db.close()

    def save_workspace(self, workspace):
        self.db.execute('INSERT OR REPLACE INTO workspaces(id,data) VALUES (?,?)', (workspace.id, workspace.model_dump_json()))
        self.db.commit()
        return workspace

    def workspaces(self):
        from .models import Workspace
        return [Workspace.model_validate_json(row[0]) for row in self.db.execute('SELECT data FROM workspaces ORDER BY id')]

    def workspace(self, id):
        return next((w for w in self.workspaces() if w.id == id), None)

    def save_run(self, run, commit=True):
        self.db.execute('INSERT INTO runs(id,ticket_id,data) VALUES (?,?,?) ON CONFLICT(id) DO UPDATE SET data=excluded.data',
                        (run.id, run.ticket_id, run.model_dump_json()))
        if commit:
            self.db.commit()
        return run

    def run(self, id):
        from .models import Run
        row = self.db.execute('SELECT data FROM runs WHERE id=?', (id,)).fetchone()
        return Run.model_validate_json(row[0]) if row else None

    def runs(self, ticket_id):
        from .models import Run
        return [Run.model_validate_json(row[0]) for row in self.db.execute('SELECT data FROM runs WHERE ticket_id=? ORDER BY rowid DESC', (ticket_id,))]

    def event(self, run, kind, message):
        count = self.db.execute('SELECT count(*) FROM events WHERE run_id=?', (run.id,)).fetchone()[0]
        if count >= 1000 and kind != 'status':
            return
        self.db.execute('INSERT INTO events(run_id,ticket_id,type,message,created_at) VALUES (?,?,?,?,?)',
                        (run.id, run.ticket_id, kind, message[:10000], now()))
        self.db.commit()

    def events(self, after=0, run_id=None):
        from .models import RunEvent
        query = 'SELECT id,run_id,ticket_id,type,message,created_at FROM events WHERE id>?'
        params = [after]
        if run_id:
            query += ' AND run_id=?'
            params.append(run_id)
        rows = self.db.execute(query + ' ORDER BY id LIMIT 1000', params)
        return [RunEvent(**dict(zip(['id','run_id','ticket_id','type','message','created_at'], row))) for row in rows]

    def enqueue(self, run, ticket):
        with self.db:
            self.save_run(run, commit=False)
            self.save(ticket, commit=False)

    def groups(self, project_id=None):
        from .models import Group
        groups = [Group.model_validate_json(row[0]) for row in self.db.execute('SELECT data FROM groups ORDER BY rowid')]
        return [g for g in groups if project_id is None or g.project_id == project_id]

    def group(self, id):
        return next((group for group in self.groups() if group.id == id), None)

    def save_group(self, group):
        self.db.execute('INSERT INTO groups(id,data) VALUES (?,?) ON CONFLICT(id) DO UPDATE SET data=excluded.data', (group.id, group.model_dump_json()))
        self.db.commit()
        return group

    def delete_group(self, id):
        with self.db:
            for ticket in self.all() + self.all(deleted=True):
                if ticket.group_id == id:
                    ticket.group_id = ''
                    self.save(ticket, commit=False)
            self.db.execute('DELETE FROM groups WHERE id=?', (id,))

    def enqueue_group(self, pairs):
        with self.db:
            for run, ticket in pairs:
                self.save_run(run, commit=False)
                self.save(ticket, commit=False)
        for run, ticket in pairs:
            self.event(run, 'status', 'Queued in group; waiting for preceding prompts and tests.')

    def project(self, id='default'):
        from .models import Project
        value = self.managed_project(id)
        return Project(**value.model_dump()) if value else None

    def save_project(self, value, id='default'):
        from .models import ManagedProject
        self.save_managed_project(ManagedProject(**value.model_dump(), id=id))
        return value

    def projects(self, include_deleted=False):
        from .models import ManagedProject
        values = [ManagedProject.model_validate_json(row[0]) for row in self.db.execute('SELECT data FROM projects ORDER BY rowid')]
        return [p for p in values if include_deleted or not p.deleted]

    def managed_project(self, id, include_deleted=False):
        return next((p for p in self.projects(include_deleted) if p.id == id), None)

    def save_managed_project(self, value):
        self.db.execute('INSERT INTO projects(id,data) VALUES (?,?) ON CONFLICT(id) DO UPDATE SET data=excluded.data', (value.id, value.model_dump_json()))
        self.db.commit()
        return value

    def set_ticket_deleted(self, id, deleted):
        self.db.execute('UPDATE tickets SET deleted=? WHERE number=?', (int(deleted), id.removeprefix('SW-')))
        self.db.commit()

    def delete_tickets(self, ids):
        with self.db:
            self.db.executemany('UPDATE tickets SET deleted=1 WHERE number=?', [(id.removeprefix('SW-'),) for id in ids])
