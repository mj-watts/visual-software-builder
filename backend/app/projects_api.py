from uuid import uuid4
from fastapi import APIRouter, HTTPException, Request
from .models import Project, ManagedProject, Ticket
from pydantic import BaseModel, Field

router = APIRouter(prefix='/api')


class TicketDeletion(BaseModel):
    ids: list[str] = Field(min_length=1, max_length=200)
    project_id: str = 'default'


@router.post('/tickets/delete', tags=['Tickets'])
async def delete_tickets(value: TicketDeletion, request: Request):
    from .main import require_ticket
    store = request.app.state.store
    require_project(store, value.project_id)
    ids = list(dict.fromkeys(value.ids))
    tickets = [require_ticket(store, id) for id in ids]
    if any(t.project_id != value.project_id for t in tickets):
        raise HTTPException(422, 'Selected tickets must belong to the current project.')
    require_idle(tickets)
    store.delete_tickets(ids)
    return {'deleted': ids}


def require_project(store, id, include_deleted=False):
    value = store.managed_project(id, include_deleted)
    if value is None:
        raise HTTPException(404, 'Project not found')
    return value


def require_idle(tickets):
    if any(t.status in ['queued', 'running'] for t in tickets):
        raise HTTPException(409, 'Cancel queued or running tickets before deleting.')


@router.get('/projects', tags=['Projects'], response_model=list[ManagedProject])
async def projects(request: Request, include_deleted: bool = False):
    return request.app.state.store.projects(include_deleted)


@router.post('/projects', tags=['Projects'], response_model=ManagedProject, status_code=201)
async def create(value: Project, request: Request):
    return request.app.state.store.save_managed_project(ManagedProject(**value.model_dump(), id=uuid4().hex))


@router.get('/projects/{id}', tags=['Projects'], response_model=ManagedProject)
async def get(id: str, request: Request):
    return require_project(request.app.state.store, id)


@router.put('/projects/{id}', tags=['Projects'], response_model=ManagedProject)
async def update(id: str, value: Project, request: Request):
    store = request.app.state.store
    require_project(store, id)
    return store.save_managed_project(ManagedProject(**value.model_dump(), id=id))


@router.delete('/projects/{id}', tags=['Projects'], response_model=ManagedProject)
async def delete(id: str, request: Request):
    store = request.app.state.store
    value = require_project(store, id)
    require_idle(store.all(project_id=id))
    value.deleted = True
    return store.save_managed_project(value)


@router.post('/projects/{id}/restore', tags=['Projects'], response_model=ManagedProject)
async def restore(id: str, request: Request):
    store = request.app.state.store
    value = require_project(store, id, include_deleted=True)
    value.deleted = False
    return store.save_managed_project(value)


@router.delete('/tickets/{id}', tags=['Tickets'])
async def delete_ticket(id: str, request: Request):
    from .main import require_ticket
    store = request.app.state.store
    ticket = require_ticket(store, id)
    require_idle([ticket])
    store.set_ticket_deleted(id, True)
    return {'deleted': id}


@router.post('/tickets/{id}/restore', tags=['Tickets'], response_model=Ticket)
async def restore_ticket(id: str, request: Request):
    store = request.app.state.store
    ticket = store.get(id, deleted=True)
    if ticket is None:
        raise HTTPException(404, 'Deleted ticket not found')
    require_project(store, ticket.project_id)
    store.set_ticket_deleted(id, False)
    return ticket
