import asyncio
import json
import subprocess
import sys
import time
from pathlib import Path

import pytest
from fastapi.testclient import TestClient
from app.main import create_app
from app.settings import Settings
from app.repository import prepare_worktree, collect_diff
from app.process import run_process


def git(path,*args):
    return subprocess.check_output(['git','-C',str(path),*args],text=True).strip()

@pytest.fixture
def repo(tmp_path):
    path=tmp_path/'repo';path.mkdir()
    git(path,'init');git(path,'config','user.email','test@example.com');git(path,'config','user.name','Test')
    (path/'old.txt').write_text('old\n');git(path,'add','.');git(path,'commit','-m','initial')
    return path

@pytest.fixture
def real_client(tmp_path,repo):
    settings=Settings(roots=[tmp_path],artifacts=tmp_path/'runs',timeout=2,enabled={'codex':True,'claude':True})
    async def adapter(ticket,run,emit):
        await emit('agent','Starting fake coding agent')
        (Path(run.worktree)/'old.txt').write_text('new\n')
        (Path(run.worktree)/'added.txt').write_text('added\n')
        await asyncio.sleep(.15)
        return 'Implemented changes'
    with TestClient(create_app(tmp_path/'db',delay=.01,settings=settings,adapters={'codex':adapter,'claude':adapter})) as client:
        yield client


def setup_run(client,repo,**extra):
    workspace=client.post('/api/workspaces',json={'path':str(repo),'name':'Example'}).json()
    ticket=client.post('/api/tickets',json={'title':'Change text','prompt':'Update old text','provider':'codex','workspace_id':workspace['id'],'permission':'workspace-write',**extra}).json()
    return ticket


def finish(client,ticket):
    for _ in range(300):
        result=client.get('/api/tickets/'+ticket['id']).json()
        if result['status'] in ['done','failed','cancelled']:
            return result
        time.sleep(.01)
    pytest.fail('Run did not finish')


def test_isolated_real_run_and_history(real_client,repo):
    ticket=setup_run(real_client,repo)
    assert real_client.post('/api/tickets/'+ticket['id']+'/run').status_code==202
    result=finish(real_client,ticket)
    assert result['status']=='done'
    assert {f['path'] for f in result['files']}=={'old.txt','added.txt'}
    assert (repo/'old.txt').read_text()=='old\n'
    runs=real_client.get('/api/tickets/'+ticket['id']+'/runs').json()
    assert runs[0]['base_commit']==git(repo,'rev-parse','HEAD')
    assert Path(runs[0]['worktree']).is_dir()
    assert real_client.get('/api/runs/'+runs[0]['id']+'/events').json()


def test_workspace_boundaries(real_client,repo,tmp_path):
    assert real_client.post('/api/workspaces',json={'path':str(tmp_path),'name':'Not git'}).status_code==422
    assert real_client.post('/api/workspaces',json={'path':'/etc','name':'Outside'}).status_code==422
    ticket=real_client.post('/api/tickets',json={'title':'test','prompt':'test','provider':'codex'}).json()
    assert real_client.post('/api/tickets/'+ticket['id']+'/run').status_code==409
    assert real_client.get('/api/workspaces').json()==[]


def test_cancel_and_retry_keep_history(real_client,repo):
    ticket=setup_run(real_client,repo)
    real_client.post('/api/tickets/'+ticket['id']+'/run')
    assert real_client.post('/api/tickets/'+ticket['id']+'/cancel').status_code==200
    result=finish(real_client,ticket)
    assert result['status']=='cancelled'
    assert real_client.post('/api/tickets/'+ticket['id']+'/retry').status_code==202
    assert finish(real_client,ticket)['status']=='done'
    runs=real_client.get('/api/tickets/'+ticket['id']+'/runs').json()
    assert len(runs)==2
    assert {r['status'] for r in runs}=={'cancelled','done'}
    assert real_client.post('/api/tickets/'+ticket['id']+'/retry').status_code==409


def test_read_only_cannot_claim_edits(real_client,repo):
    ticket=setup_run(real_client,repo,permission='read-only')
    real_client.post('/api/tickets/'+ticket['id']+'/run')
    assert finish(real_client,ticket)['status']=='failed'


def test_git_diff_includes_staged_committed_untracked_and_binary(repo,tmp_path):
    base=git(repo,'rev-parse','HEAD')
    work=prepare_worktree(repo,tmp_path/'isolated',base)
    (work/'old.txt').write_text('edited\n');git(work,'add','old.txt');git(work,'commit','-m','agent')
    (work/'unicode name é.txt').write_text('new\n')
    (work/'binary.bin').write_bytes(b'\x00\x01\x02')
    files=collect_diff(work,base)
    assert {f.path for f in files}=={'old.txt','unicode name é.txt','binary.bin'}
    assert '+edited' in next(f.diff for f in files if f.path=='old.txt')


def test_process_timeout_cancel_and_nonzero(tmp_path):
    async def scenario():
        seen=[]
        async def emit(kind,text):seen.append(text)
        with pytest.raises(TimeoutError):
            await run_process([sys.executable,'-c','import time;time.sleep(10)'],tmp_path,'',emit,.05,{})
        with pytest.raises(RuntimeError):
            await run_process([sys.executable,'-c','raise SystemExit(3)'],tmp_path,'',emit,2,{})
        task=asyncio.create_task(run_process([sys.executable,'-c','import time;time.sleep(10)'],tmp_path,'',emit,10,{}))
        await asyncio.sleep(.05);task.cancel()
        with pytest.raises(asyncio.CancelledError):await task
        await run_process([sys.executable,'-c','print("event")'],tmp_path,'',emit,2,{})
        assert 'event' in seen
    asyncio.run(scenario())


def test_active_cancel_timeout_and_test_gates(tmp_path,repo,monkeypatch):
    from app.runner import TEST_COMMANDS
    settings=Settings(roots=[tmp_path],artifacts=tmp_path/'runs',timeout=.5,enabled={'codex':True})
    started=__import__('threading').Event()
    async def slow(ticket,run,emit):
        started.set()
        await asyncio.sleep(10)
    with TestClient(create_app(tmp_path/'cancel.db',settings=settings,adapters={'codex':slow})) as c:
        ticket=setup_run(c,repo)
        c.post('/api/tickets/'+ticket['id']+'/run')
        assert started.wait(2)
        assert c.post('/api/tickets/'+ticket['id']+'/cancel').json()['status']=='cancelled'
        c.post('/api/tickets/'+ticket['id']+'/retry')
        assert finish(c,ticket)['status']=='failed'
        assert 'timed out' in c.get('/api/tickets/'+ticket['id']).json()['response']
    async def success(ticket,run,emit):return 'Reviewed'
    monkeypatch.setitem(TEST_COMMANDS,'pytest',[sys.executable,'-c','import os; print(os.getenv("ANTHROPIC_API_KEY")); print("tests passed")'])
    with TestClient(create_app(tmp_path/'tests.db',settings=settings,adapters={'codex':success})) as c:
        ticket=setup_run(c,repo,allow_tests=True)
        c.post('/api/tickets/'+ticket['id']+'/run')
        assert finish(c,ticket)['status']=='failed'  # no configured test command
        workspace=c.get('/api/workspaces').json()[0]
        c.post('/api/workspaces',json={**workspace,'test_preset':'pytest'})
        c.post('/api/tickets/'+ticket['id']+'/retry')
        assert finish(c,ticket)['status']=='done'
        assert c.get('/api/tickets/'+ticket['id']+'/runs').json()[0]['tests']=='passed'
        events=c.get('/api/runs/'+c.get('/api/tickets/'+ticket['id']+'/runs').json()[0]['id']+'/events').json()
        assert any(e['message']=='None' for e in events)
        monkeypatch.setitem(TEST_COMMANDS,'pytest',[sys.executable,'-c','raise SystemExit(1)'])
        other=setup_run(c,repo,allow_tests=True)
        c.post('/api/workspaces',json={**workspace,'test_preset':'pytest'})
        c.post('/api/tickets/'+other['id']+'/run')
        assert finish(c,other)['status']=='failed'
        assert c.get('/api/tickets/'+other['id']+'/runs').json()[0]['tests']=='failed'


def test_missing_and_origin_controls(real_client):
    assert real_client.get('/api/config').json()['providers'][1]['available']
    assert real_client.get('/api/tickets/missing/runs').status_code==404
    assert real_client.get('/api/runs/missing/events').status_code==404
    assert real_client.post('/api/tickets/missing/cancel').status_code==404
    assert real_client.post('/api/tickets',headers={'Origin':'https://untrusted.example'},json={}).status_code==403
    assert real_client.get('/api/config',headers={'Host':'untrusted.example'}).status_code==400


def test_sse_replay_and_recovery(tmp_path):
    from app.store import Store
    from app.models import Draft
    from app.runs import queue_run
    from app.execution_api import event_stream
    store=Store(tmp_path/'recover.db')
    ticket=store.create(Draft(title='Title',prompt='Prompt'))
    queue_run(store,ticket,Settings())
    run=store.run(ticket.latest_run_id)
    ticket.status='running';store.save(ticket);store.recover()
    assert store.run(run.id).status=='failed'
    class Request:
        headers={'last-event-id':'1'}
        app=type('App',(),{'state':type('State',(),{'store':store})()})()
        async def is_disconnected(self):return False
    async def read():
        stream=event_stream(Request(),0)
        frame=await anext(stream)
        assert 'id: 2' in frame and 'data:' in frame
        assert await anext(stream)==': keepalive\n\n'
        await stream.aclose()
    asyncio.run(read());store.close()


def test_git_preparation_does_not_block_cancel_or_health(tmp_path,repo,monkeypatch):
    import threading
    from app import runner
    entered=threading.Event()
    original=runner.prepare_worktree
    def slower(*args):
        entered.set();time.sleep(.15);return original(*args)
    monkeypatch.setattr(runner,'prepare_worktree',slower)
    async def unused(ticket,run,emit):pytest.fail('Cancelled work must not call the agent')
    settings=Settings(roots=[tmp_path],artifacts=tmp_path/'runs',enabled={'codex':True})
    with TestClient(create_app(tmp_path/'responsive.db',settings=settings,adapters={'codex':unused})) as c:
        ticket=setup_run(c,repo)
        c.post('/api/tickets/'+ticket['id']+'/run')
        assert entered.wait(2)
        assert c.get('/api/health').json()['mode']=='agents'
        assert c.post('/api/tickets/'+ticket['id']+'/cancel').json()['status']=='cancelled'
        run=c.get('/api/tickets/'+ticket['id']+'/runs').json()[0]
        assert Path(run['worktree']).is_dir()
