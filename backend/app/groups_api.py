from uuid import uuid4
from fastapi import APIRouter, HTTPException, Request
from .models import Group, GroupInput, GroupAssignment, Ticket

router = APIRouter(prefix='/api')

def require_group(store, id):
    group = store.group(id)
    if group is None:
        raise HTTPException(404, 'Group not found')
    from .projects_api import require_project
    require_project(store, group.project_id)
    return group

def validate_group(store, id, project_id='default'):
    if id:
        group = require_group(store, id)
        if group.project_id != project_id:
            raise HTTPException(422, 'Group belongs to another project')

@router.get('/groups', tags=['Groups'], response_model=list[Group])
async def groups(request: Request, project_id: str = 'default'):
    from .projects_api import require_project
    require_project(request.app.state.store, project_id)
    return request.app.state.store.groups(project_id)

@router.post('/groups', tags=['Groups'], response_model=Group, status_code=201)
async def create(value: GroupInput, request: Request, project_id: str = 'default'):
    from .projects_api import require_project
    require_project(request.app.state.store, project_id)
    return request.app.state.store.save_group(Group(**value.model_dump(), id=str(uuid4()), project_id=project_id))

@router.patch('/groups/{id}', tags=['Groups'], response_model=Group)
async def update(id: str, value: GroupInput, request: Request):
    store = request.app.state.store
    group = require_group(store, id)
    return store.save_group(Group(**value.model_dump(), id=id, project_id=group.project_id))

@router.delete('/groups/{id}', tags=['Groups'])
async def delete(id: str, request: Request):
    store = request.app.state.store
    require_group(store, id)
    store.delete_group(id)
    return {'deleted': id}

@router.put('/tickets/{id}/group', tags=['Groups'], response_model=Ticket)
async def assign(id: str, value: GroupAssignment, request: Request):
    from .main import require_ticket
    store = request.app.state.store
    ticket = require_ticket(store, id)
    validate_group(store, value.group_id, ticket.project_id)
    ticket.group_id = value.group_id
    return store.save(ticket)

@router.post('/groups/{id}/run', tags=['Runs'], response_model=list[Ticket], status_code=202)
async def run_group(id: str, request: Request):
    from .group_runs import queue_group
    store = request.app.state.store
    require_group(store, id)
    return queue_group(store, id, request.app.state.settings)
