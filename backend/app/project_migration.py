"""Additive SQLite migration; the legacy singleton and ticket JSON remain intact.

Rollback: restore the pre-upgrade SQLite backup and previous application version.
New project/deletion state requires the new application version.
"""
from .models import ManagedProject, Project


def migrate_schema(db):
    columns = {row[1] for row in db.execute('PRAGMA table_info(tickets)')}
    if 'deleted' not in columns:
        db.execute('ALTER TABLE tickets ADD COLUMN deleted INTEGER NOT NULL DEFAULT 0')
    db.execute('CREATE TABLE IF NOT EXISTS projects(id TEXT PRIMARY KEY, data TEXT NOT NULL)')
    db.commit()


def migrate_project(db):
    if db.execute('SELECT count(*) FROM projects').fetchone()[0]:
        return
    row = db.execute('SELECT data FROM project WHERE id=1').fetchone()
    legacy = Project.model_validate_json(row[0]) if row else Project()
    value = ManagedProject(**legacy.model_dump(), id='default')
    db.execute('INSERT INTO projects(id,data) VALUES (?,?)', (value.id, value.model_dump_json()))
    db.commit()
