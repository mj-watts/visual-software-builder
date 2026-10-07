import asyncio
from fastapi import APIRouter, HTTPException, Request, Query
from fastapi.responses import StreamingResponse
from .models import WorkspaceInput, Workspace, Run, RunEvent, Ticket
from .repository import resolve_workspace
from .runs import require_run, queue_run

router = APIRouter()


def ticket_for(request, id):
    from .main import require_ticket
    return require_ticket(request.app.state.store, id)


@router.get('/api/config', tags=['System'])
async def config(request: Request):
    settings = request.app.state.settings
    return {'providers': settings.providers(), 'workspace_roots': [str(p) for p in settings.roots],
            'timeout_seconds': settings.timeout}


@router.get('/api/workspaces', tags=['Workspaces'], response_model=list[Workspace])
async def workspaces(request: Request):
    return request.app.state.store.workspaces()


@router.post('/api/workspaces', tags=['Workspaces'], response_model=Workspace, status_code=201)
async def register(value: WorkspaceInput, request: Request):
    try:
        workspace = resolve_workspace(value, request.app.state.settings)
    except (ValueError, OSError) as error:
        raise HTTPException(422, str(error)) from error
    return request.app.state.store.save_workspace(workspace)


@router.get('/api/tickets/{ticket_id}/runs', tags=['Runs'], response_model=list[Run])
async def history(ticket_id: str, request: Request):
    ticket_for(request, ticket_id)
    return request.app.state.store.runs(ticket_id)


@router.get('/api/runs/{run_id}/events', tags=['Runs'], response_model=list[RunEvent])
async def activity(run_id: str, request: Request):
    require_run(request.app.state.store, run_id)
    return request.app.state.store.events(run_id=run_id)


@router.post('/api/tickets/{ticket_id}/cancel', tags=['Runs'], response_model=Ticket)
async def cancel(ticket_id: str, request: Request):
    ticket = ticket_for(request, ticket_id)
    if ticket.status not in ['queued', 'running']:
        raise HTTPException(409, 'Only queued or running tickets can be cancelled.')
    return await request.app.state.runner.cancel(ticket)


@router.post('/api/tickets/{ticket_id}/retry', tags=['Runs'], response_model=Ticket, status_code=202)
async def retry(ticket_id: str, request: Request):
    ticket = ticket_for(request, ticket_id)
    if ticket.status not in ['failed', 'cancelled']:
        raise HTTPException(409, 'Only failed or cancelled tickets can be retried.')
    return queue_run(request.app.state.store, ticket, request.app.state.settings)


def cursor_for(request, after):
    header = request.headers.get('last-event-id', '')
    return max(after, int(header)) if header.isdigit() else after


async def event_stream(request, after):
    cursor = cursor_for(request, after)
    while not await request.is_disconnected():
        for event in request.app.state.store.events(after=cursor):
            cursor = event.id
            yield f'id: {cursor}\ndata: {event.model_dump_json()}\n\n'
        yield ': keepalive\n\n'
        await asyncio.sleep(.5)


@router.get('/api/events', tags=['Runs'], responses={200: {'content': {'text/event-stream': {}}}})
async def events(request: Request, after: int = Query(default=0, ge=0)):
    return StreamingResponse(event_stream(request, after), media_type='text/event-stream',
                             headers={'Cache-Control': 'no-cache', 'X-Accel-Buffering': 'no'})
