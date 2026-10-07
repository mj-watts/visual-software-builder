import time
import pytest
from fastapi.testclient import TestClient
from app.main import create_app


def create(client, **extra):
    return client.post('/api/tickets', json={'title':'Add navigation','prompt':'Add accessible links',**extra})

def wait_done(client, id):
    for _ in range(100):
        ticket = client.get('/api/tickets/' + id).json()
        if ticket['status'] == 'done':
            return ticket
        time.sleep(0.01)
    pytest.fail('worker did not complete')

def test_create_and_edit(client):
    ticket = create(client).json()
    assert ticket['status'] == 'todo'
    assert client.patch('/api/tickets/'+ticket['id'],json={'title':'New title'}).json()['title']=='New title'
    assert any(t['id']==ticket['id'] for t in client.get('/api/tickets').json())

def test_queue_completes_and_has_diff(client):
    ticket = create(client).json()
    assert client.post('/api/tickets/'+ticket['id']+'/run').status_code == 202
    result = wait_done(client,ticket['id'])
    assert result['files'][0]['diff'].startswith('---')
    assert 'simulated' in result['response'].lower()

def test_invalid_and_missing(client):
    assert create(client,title=' ').status_code==422
    assert client.get('/api/tickets/nope').status_code==404
    assert client.patch('/api/tickets/nope',json={'title':'test'}).status_code==404
    assert client.post('/api/tickets/nope/run').status_code==404
    assert create(client,provider='invalid').status_code==422
    assert create(client,images=['data:image/svg+xml;base64,abc']).status_code==422

def test_unconfigured_provider(client):
    ticket=create(client,provider='codex').json()
    assert client.post('/api/tickets/'+ticket['id']+'/run').status_code==409

def test_duplicate_run_and_lock(client):
    ticket=create(client).json()
    client.post('/api/tickets/'+ticket['id']+'/run')
    assert client.post('/api/tickets/'+ticket['id']+'/run').status_code==409
    assert client.patch('/api/tickets/'+ticket['id'],json={'prompt':'changed'}).status_code==409
    wait_done(client,ticket['id'])
    assert client.post('/api/tickets/'+ticket['id']+'/run').status_code==409

def test_persistence(tmp_path):
    path=tmp_path/'persist.sqlite3'
    with TestClient(create_app(path,delay=0)) as c:
        ticket=create(c).json()
    with TestClient(create_app(path,delay=0)) as c:
        assert c.get('/api/tickets/'+ticket['id']).json()['title']=='Add navigation'

def test_health_and_openapi(client):
    assert client.get('/api/health').json()['mode']=='demo'
    assert '/api/tickets/{ticket_id}/run' in client.get('/openapi.json').json()['paths']
