"""Validate and snapshot executions before publishing them to the FIFO worker."""
import uuid
from fastapi import HTTPException
from .models import Draft, Run
from .repository import resolve_workspace, git
from .store import now


def require_run(store, id):
    run = store.run(id)
    if not run:
        raise HTTPException(404, 'Run not found')
    return run


def validate_target(store, ticket, settings):
    if ticket.provider == 'demo':
        return None
    if not settings.enabled.get(ticket.provider):
        raise HTTPException(409, 'This provider is not connected. Configure server credentials and its CLI.')
    workspace = store.workspace(ticket.workspace_id)
    if not workspace:
        raise HTTPException(409, 'Select a registered Git repository before running a real agent.')
    try:
        return resolve_workspace(workspace, settings)
    except (ValueError, OSError) as error:
        raise HTTPException(409, str(error)) from error


def build_run(store, ticket, settings):
    workspace = validate_target(store, ticket, settings)
    run = Run(id=uuid.uuid4().hex, ticket_id=ticket.id,
              snapshot=Draft.model_validate(ticket.model_dump()), created_at=now())
    if workspace:
        run.workspace_path = workspace.path
        run.base_commit = git(workspace.path, 'rev-parse', 'HEAD').decode().strip()
        run.test_preset = workspace.test_preset
        run.test_directory = workspace.test_directory
    return run


def mark_queued(ticket, run):
    ticket.status = 'queued'
    ticket.queued_at = run.created_at
    ticket.latest_run_id = run.id
    ticket.response = ''
    ticket.files = []
    return ticket


def queue_run(store, ticket, settings):
    run = build_run(store, ticket, settings)
    mark_queued(ticket, run)
    # One transaction: a queued ticket always has a durable snapshot.
    store.enqueue(run, ticket)
    store.event(run, 'status', 'Queued')
    return ticket


def finish_run(store, run, status, response, files=None):
    run.status = status
    run.finished_at = now()
    run.response = response
    if files is not None:
        run.files = files
    store.save_run(run)
    ticket = store.get(run.ticket_id)
    ticket.status = status
    ticket.response = response
    ticket.files = run.files
    store.save(ticket)
    store.event(run, 'status', status.capitalize())
