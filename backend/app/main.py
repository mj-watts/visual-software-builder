import asyncio
import os
from contextlib import asynccontextmanager, suppress
from fastapi import FastAPI, HTTPException
from .models import Draft, Changes, Ticket, ChangedFile, Project
from .store import Store
from .runner import Runner
from .settings import Settings
from .execution_api import router
from .runs import queue_run
from starlette.middleware.trustedhost import TrustedHostMiddleware

def require_ticket(store, ticket_id):
    ticket = store.get(ticket_id)
    if ticket is None:
        raise HTTPException(404, 'Ticket not found')
    from .projects_api import require_project
    require_project(store, ticket.project_id)
    return ticket

def require_editable(ticket):
    if ticket.status != 'todo':
        raise HTTPException(409, 'Only Todo tickets can be edited or started')

def seed(store):
    if store.db.execute('SELECT count(*) FROM tickets').fetchone()[0] or not store.managed_project('default'):
        return
    store.create(Draft(title='Build a welcoming project overview', prompt='Create a project overview with a clear introduction, recent activity and a quick way to start a new task. Keep the design calm and accessible.'))
    store.create(Draft(title='Bring the navigation to life', prompt='Add a subtle transition to the navigation. Respect reduced-motion preferences and keep keyboard focus visible.'))
    ticket = store.create(Draft(title='Add a contact section', prompt='Create a contact section with email and social links. Use semantic markup and a responsive layout.'))
    ticket.status = 'done'
    ticket.response = 'Simulated example: a contact section was prepared with semantic links and a responsive layout. These are illustrative diffs; no repository was changed.'
    ticket.files = [ChangedFile(path='src/components/Contact.vue', additions=4, deletions=1, diff='--- a/src/components/Contact.vue\n+++ b/src/components/Contact.vue\n@@ -1,3 +1,6 @@\n <template>\n-  <div>Contact</div>\n+  <section aria-labelledby="contact-title">\n+    <h2 id="contact-title">Let’s talk</h2>\n+    <a href="mailto:hello@example.com">Email us</a>\n+  </section>\n </template>')]
    store.save(ticket)

def create_app(path=None, delay=3, with_seed=False, settings=None, adapters=None):
    @asynccontextmanager
    async def lifespan(app):
        store = Store(path or os.environ.get('SWIMLANE_DB', 'swimlane.sqlite3'))
        app.state.store = store
        store.recover()
        if with_seed:
            seed(store)
        app.state.settings = settings or Settings.from_env()
        app.state.runner = Runner(store, app.state.settings, delay, adapters)
        task = asyncio.create_task(app.state.runner.loop())
        yield
        task.cancel()
        with suppress(asyncio.CancelledError):
            await task
        store.close()

    api = FastAPI(title='Swimlane Studio API', version='0.2.0', lifespan=lifespan, openapi_tags=[
        {'name': 'Projects', 'description': 'Create, edit, delete and restore projects, including their screenshots.'},
        {'name': 'Tickets', 'description': 'Manage ticket prompts, images and results, including recoverable deletion.'},
        {'name': 'Groups', 'description': 'Organise tickets into groups and change membership.'},
        {'name': 'Runs', 'description': 'Start tickets or groups, cancel or retry runs, and inspect history and live events.'},
        {'name': 'Workspaces', 'description': 'Register and list local Git repositories used by coding agents.'},
        {'name': 'System', 'description': 'Check API health, available providers and server configuration.'},
    ])
    api.include_router(router)
    from .groups_api import router as groups_router
    api.include_router(groups_router)
    from .projects_api import router as projects_router
    api.include_router(projects_router)
    api.add_middleware(TrustedHostMiddleware, allowed_hosts=['127.0.0.1', 'localhost', 'testserver'])

    @api.middleware('http')
    async def local_origin(request, call_next):
        origin = request.headers.get('origin')
        allowed = {'http://127.0.0.1:5173', 'http://127.0.0.1:5174', 'http://localhost:5173', 'http://localhost:5174', 'http://127.0.0.1:8000', 'http://localhost:8000'}
        if origin and origin not in allowed and request.method != 'GET':
            from fastapi.responses import JSONResponse
            return JSONResponse({'detail': 'Cross-origin execution requests are blocked.'}, status_code=403)
        return await call_next(request)

    @api.get('/api/health', tags=['System'])
    async def health():
        return {'status': 'ok', 'mode': 'agents' if any(api.state.settings.enabled.values()) else 'demo'}

    @api.get('/api/project', tags=['Projects'], response_model=Project)
    async def project():
        from .projects_api import require_project
        require_project(api.state.store, 'default')
        return api.state.store.project()

    @api.put('/api/project', tags=['Projects'], response_model=Project)
    async def update_project(value: Project):
        from .projects_api import require_project
        require_project(api.state.store, 'default')
        return api.state.store.save_project(value)

    @api.get('/api/tickets', tags=['Tickets'], response_model=list[Ticket])
    async def tickets(project_id: str = 'default', deleted: bool = False):
        from .projects_api import require_project
        require_project(api.state.store, project_id)
        return api.state.store.all(deleted=deleted, project_id=project_id)

    @api.post('/api/tickets', tags=['Tickets'], response_model=Ticket, status_code=201)
    async def create(draft: Draft, project_id: str = 'default'):
        from .groups_api import validate_group
        from .projects_api import require_project
        require_project(api.state.store, project_id)
        validate_group(api.state.store, draft.group_id, project_id)
        return api.state.store.create(draft, project_id)

    @api.get('/api/tickets/{ticket_id}', tags=['Tickets'], response_model=Ticket)
    async def get(ticket_id: str):
        return require_ticket(api.state.store, ticket_id)

    @api.patch('/api/tickets/{ticket_id}', tags=['Tickets'], response_model=Ticket)
    async def update(ticket_id: str, changes: Changes):
        ticket = require_ticket(api.state.store, ticket_id)
        require_editable(ticket)
        data = ticket.model_dump()
        data.update(changes.model_dump(exclude_none=True))
        try:
            changed = Ticket.model_validate(data)
        except ValueError as error:
            raise HTTPException(422, 'Invalid ticket fields or attachments') from error
        from .groups_api import validate_group
        validate_group(api.state.store, changed.group_id, ticket.project_id)
        return api.state.store.save(changed)

    @api.post('/api/tickets/{ticket_id}/run', tags=['Runs'], response_model=Ticket, status_code=202)
    async def run(ticket_id: str):
        ticket = require_ticket(api.state.store, ticket_id)
        require_editable(ticket)
        return queue_run(api.state.store, ticket, api.state.settings)

    return api

app = create_app(with_seed=True)
