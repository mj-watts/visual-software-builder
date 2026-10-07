import asyncio
from pathlib import Path
from .adapters import CliAgent
from .agents import DemoAgent
from .process import run_process
from .repository import prepare_worktree, collect_diff, relative_directory, resolve_workspace
from .runs import finish_run
from .store import now

async def git_operation(operation, *args):
    task = asyncio.create_task(asyncio.to_thread(operation, *args))
    try:
        return await asyncio.shield(task)
    except asyncio.CancelledError:
        # Finish the bounded Git operation before finalizing the attempt.
        # HTTP/SSE stay responsive and no worker thread is left mutating its index.
        await task
        raise


TEST_COMMANDS = {'vitest': ['npm', 'test', '--', '--run'], 'pytest': ['python3', '-m', 'pytest', '-q']}

class Runner:
    def __init__(self, store, settings, delay, adapters=None):
        self.store = store
        self.settings = settings
        self.demo = DemoAgent(delay)
        self.adapters = adapters or {}
        self.active = {}

    async def loop(self):
        while True:
            ticket = self.store.next()
            if ticket:
                await self.dispatch(ticket)
            await asyncio.sleep(.1)

    async def dispatch(self, ticket):
        task = asyncio.create_task(self.execute(ticket))
        self.active[ticket.id] = task
        try:
            await task
        except asyncio.CancelledError:
            if asyncio.current_task().cancelling():
                raise
        finally:
            self.active.pop(ticket.id, None)

    async def execute(self, ticket):
        ticket = self.store.get(ticket.id)
        run = self.store.run(ticket.latest_run_id)
        if not run:  # Resume tickets queued by phase 1 after an upgrade.
            from .runs import queue_run
            queue_run(self.store, ticket, self.settings)
            run = self.store.run(ticket.latest_run_id)
        from .group_runs import blocked_reason
        blocked = blocked_reason(self.store, run)
        if blocked:
            finish_run(self.store, run, 'cancelled', blocked)
            return
        run.started_at = now()
        run.status = ticket.status = 'running'
        self.store.save_run(run)
        self.store.save(ticket)
        self.store.event(run, 'status', 'Running')
        try:
            async with asyncio.timeout(self.settings.timeout):
                response, files = await self.perform(run)
            finish_run(self.store, run, 'done', response, files)
        except asyncio.CancelledError:
            await self.failed(run, 'cancelled', 'Run cancelled. Partial changes are retained for review.')
            raise
        except TimeoutError:
            await self.failed(run, 'failed', 'Run timed out. Partial changes are retained for review.')
        except Exception as error:
            await self.failed(run, 'failed', str(error))

    async def failed(self, run, status, message):
        from .adapters import redact
        run.error = redact(message, self.settings)
        files = await self.partial_diff(run)
        finish_run(self.store, run, status, run.error, files)

    async def partial_diff(self, run):
        if not run.worktree:
            return []
        try:
            return await git_operation(collect_diff, Path(run.worktree), run.base_commit)
        except Exception:
            return []

    async def prepare(self, run):
        workspace = self.store.workspace(run.snapshot.workspace_id)
        if not workspace:
            raise ValueError('Workspace no longer exists.')
        resolve_workspace(workspace, self.settings)
        target = self.settings.artifacts / run.id / 'worktree'
        run.worktree = str(target)
        self.store.save_run(run)
        await git_operation(prepare_worktree, run.workspace_path, target, run.base_commit)
        self.store.event(run, 'workspace', f'Isolated worktree ready at {run.worktree}')

    async def perform(self, run):
        ticket = run.snapshot
        if ticket.provider == 'demo':
            if run.group_run_id:
                run.tests = 'simulated'
            return await self.demo.run(ticket)
        await self.prepare(run)
        async def emit(kind, message):
            self.store.event(run, kind, message)
        agent = self.adapters.get(ticket.provider) or CliAgent(self.settings, ticket.provider)
        response = await agent(ticket, run, emit)
        await self.test(run, emit)
        files = await git_operation(collect_diff, Path(run.worktree), run.base_commit)
        if ticket.permission == 'read-only' and files:
            raise ValueError('Read-only run modified files. Changes retained; run marked Failed.')
        return response, files

    async def test(self, run, emit):
        if not run.snapshot.allow_tests:
            return
        if run.test_preset == 'none':
            raise ValueError('Select a test preset for the repository before enabling tests.')
        from .adapters import child_environment
        directory = relative_directory(Path(run.worktree), run.test_directory)
        run.tests = 'running'
        self.store.save_run(run)
        await emit('test', 'Running ' + run.test_preset + ' tests')
        try:
            await run_process(TEST_COMMANDS[run.test_preset], directory, '', emit,
                              self.settings.timeout, child_environment(self.settings, 'tests', run))
        except BaseException:
            run.tests = 'failed'
            raise
        run.tests = 'passed'
        self.store.save_run(run)
        await emit('test', 'Tests passed')

    async def cancel(self, ticket):
        task = self.active.get(ticket.id)
        if task:
            task.cancel()
            try:
                await task
            except asyncio.CancelledError:
                pass
            return self.store.get(ticket.id)
        run = self.store.run(ticket.latest_run_id)
        finish_run(self.store, run, 'cancelled', 'Cancelled before execution.')
        return self.store.get(ticket.id)
