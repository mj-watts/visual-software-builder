import time
import pytest
from fastapi.testclient import TestClient
from app.main import create_app


def setup(client, provider='demo', **fields):
    group = client.post('/api/groups', json={'name': 'Sequence', 'color': 'blue'}).json()
    tickets = [client.post('/api/tickets', json={'title': str(index), 'prompt': str(index), 'provider': provider, 'group_id': group['id'], **fields}).json() for index in range(3)]
    return group, tickets


def wait(client, tickets):
    for _ in range(300):
        results = [client.get('/api/tickets/' + t['id']).json() for t in tickets]
        if all(t['status'] in ['done', 'failed', 'cancelled'] for t in results):
            return results
        time.sleep(.01)
    pytest.fail('Sequence did not finish')


def test_group_runs_top_to_bottom(client):
    group, tickets = setup(client)
    response = client.post('/api/groups/' + group['id'] + '/run')
    assert response.status_code == 202
    assert [t['id'] for t in response.json()] == [t['id'] for t in tickets]
    assert client.post('/api/groups/' + group['id'] + '/run').status_code == 409
    assert [t['status'] for t in wait(client, tickets)] == ['done'] * 3
    runs = [client.get('/api/tickets/' + t['id'] + '/runs').json()[0] for t in tickets]
    assert runs[1]['previous_run_id'] == runs[0]['id']
    assert runs[2]['previous_run_id'] == runs[1]['id']
    assert runs[0]['finished_at'] <= runs[1]['started_at'] <= runs[2]['started_at']
    assert client.post('/api/groups/' + group['id'] + '/run').status_code == 409
    assert client.post('/api/groups/missing/run').status_code == 404


def test_failure_stops_following_prompts(tmp_path):
    with TestClient(create_app(tmp_path/'db', delay=.01)) as client:
        seen=[]
        async def fail(ticket):
            seen.append(ticket.title)
            raise ValueError('Test failure')
        client.app.state.runner.demo.run=fail
        group,tickets=setup(client)
        assert client.post('/api/groups/'+group['id']+'/run').status_code==202
        results=wait(client,tickets)
        assert [t['status'] for t in results]==['failed','cancelled','cancelled']
        assert seen==['0']
        assert 'Group stopped' in results[1]['response']


def test_invalid_group_is_not_partially_queued(client):
    group,tickets=setup(client)
    client.patch('/api/tickets/'+tickets[1]['id'],json={'provider':'codex'})
    assert client.post('/api/groups/'+group['id']+'/run').status_code==409
    assert all(client.get('/api/tickets/'+t['id']).json()['status']=='todo' for t in tickets)
    assert client.get('/api/tickets/'+tickets[0]['id']+'/runs').json()==[]


def test_cancel_stops_group(tmp_path):
    with TestClient(create_app(tmp_path/'db',delay=1)) as client:
        group,tickets=setup(client)
        client.post('/api/groups/'+group['id']+'/run')
        client.post('/api/tickets/'+tickets[0]['id']+'/cancel')
        assert [t['status'] for t in wait(client,tickets)]==['cancelled']*3

from test_phase2 import repo

def test_real_tests_gate_each_successor(tmp_path, repo, monkeypatch):
    import sys
    from pathlib import Path
    from app.settings import Settings
    from app.runner import TEST_COMMANDS
    settings=Settings(roots=[tmp_path],artifacts=tmp_path/'runs',timeout=3,enabled={'codex':True})
    seen=[]
    async def adapter(ticket,run,emit):
        seen.append(ticket.title)
        (Path(run.worktree)/'gate.txt').write_text(ticket.title)
        return 'Implemented'
    monkeypatch.setitem(TEST_COMMANDS,'pytest',[sys.executable,'-c',"from pathlib import Path; import sys; sys.exit(1 if Path('gate.txt').read_text() == '1' else 0)"])
    with TestClient(create_app(tmp_path/'db',settings=settings,adapters={'codex':adapter})) as client:
        workspace=client.post('/api/workspaces',json={'path':str(repo),'name':'Example','test_preset':'pytest'}).json()
        group,tickets=setup(client,'codex',workspace_id=workspace['id'],allow_tests=True,permission='workspace-write')
        assert client.post('/api/groups/'+group['id']+'/run').status_code==202
        results=wait(client,tickets)
        assert [t['status'] for t in results]==['done','failed','cancelled']
        assert seen==['0','1']
        runs=[client.get('/api/tickets/'+t['id']+'/runs').json()[0] for t in tickets]
        assert [run['tests'] for run in runs]==['passed','failed','not-requested']
        assert (repo/'old.txt').read_text()=='old\n'


def test_real_groups_require_explicit_tests(tmp_path,repo):
    from app.settings import Settings
    settings=Settings(roots=[tmp_path],artifacts=tmp_path/'runs',enabled={'codex':True})
    with TestClient(create_app(tmp_path/'db',settings=settings)) as client:
        workspace=client.post('/api/workspaces',json={'path':str(repo),'name':'Example','test_preset':'pytest'}).json()
        group,tickets=setup(client,'codex',workspace_id=workspace['id'])
        result=client.post('/api/groups/'+group['id']+'/run')
        assert result.status_code==409 and 'require tests' in result.json()['detail']
        assert all(client.get('/api/tickets/'+t['id']).json()['status']=='todo' for t in tickets)


def test_dependency_survives_restart_and_rejects_missing_tests(tmp_path):
    from app.store import Store
    from app.models import Draft,Run
    from app.group_runs import blocked_reason
    path=tmp_path/'db';store=Store(path)
    first=Run(id='first',ticket_id='SW-001',snapshot=Draft(title='One',prompt='One',provider='codex'),created_at='',status='done')
    second=Run(id='second',ticket_id='SW-002',snapshot=Draft(title='Two',prompt='Two'),created_at='',previous_run_id='first')
    store.save_run(first);store.save_run(second);store.close();store=Store(path)
    assert 'Group stopped' in blocked_reason(store,store.run('second'))
    first.tests='passed';store.save_run(first)
    assert blocked_reason(store,second)==''
    second.previous_run_id='missing';assert 'Group stopped' in blocked_reason(store,second)
    store.close()
