import sqlite3
from app.store import Store


def test_bulk_delete_is_atomic_and_project_scoped(client):
    first = client.post('/api/tickets', json={'title': 'First', 'prompt': 'Build'}).json()
    second = client.post('/api/tickets', json={'title': 'Second', 'prompt': 'Build'}).json()
    ids = [first['id'], second['id']]
    stored = client.app.state.store.get(second['id'])
    stored.status = 'running'
    client.app.state.store.save(stored)
    assert client.post('/api/tickets/delete', json={'ids': ids}).status_code == 409
    assert len(client.get('/api/tickets').json()) == 2
    stored.status = 'todo'; client.app.state.store.save(stored)
    assert client.post('/api/tickets/delete', json={'ids': [first['id'], 'missing']}).status_code == 404
    assert len(client.get('/api/tickets').json()) == 2
    other = client.post('/api/projects', json={'name': 'Other'}).json()
    assert client.post('/api/tickets/delete', json={'ids': ids, 'project_id': other['id']}).status_code == 422
    assert client.post('/api/tickets/delete', json={'ids': []}).status_code == 422
    assert client.post('/api/tickets/delete', json={'ids': ids}).json()['deleted'] == ids
    assert client.get('/api/tickets').json() == []
    assert len(client.get('/api/tickets?deleted=true').json()) == 2
    assert client.post('/api/tickets/'+first['id']+'/restore').json()['prompt'] == 'Build'


def test_project_screenshot_roundtrip_validation_and_restore(client):
    image = 'data:image/png;base64,aW1hZ2U='
    project = client.post('/api/projects', json={'name': 'Preview', 'screenshot': image}).json()
    assert project['screenshot'] == image
    path = '/api/projects/' + project['id']
    assert client.get(path).json()['screenshot'] == image
    assert client.get('/api/projects').json()[-1]['screenshot'] == image
    client.delete(path)
    assert client.post(path+'/restore').json()['screenshot'] == image
    assert client.put(path, json={'name': 'Preview', 'screenshot': ''}).json()['screenshot'] == ''
    for invalid in ['https://example.com/image.png', 'data:image/png;base64,!!!']:
        assert client.put(path, json={'name': 'Preview', 'screenshot': invalid}).status_code == 422
    assert client.put('/api/project', json={'name': 'Original', 'screenshot': image}).json()['screenshot'] == image
    assert client.get('/api/project').json()['screenshot'] == image


def test_project_isolation_and_group_boundaries(client):
    project = client.post('/api/projects', json={'name': 'Second', 'description': 'Another app'}).json()
    assert project['id'] != 'default'
    query = '?project_id=' + project['id']
    group = client.post('/api/groups'+query, json={'name': 'Feature'}).json()
    ticket = client.post('/api/tickets'+query, json={'title': 'Task', 'prompt': 'Build it', 'group_id': group['id']}).json()
    assert ticket['project_id'] == project['id']
    assert client.get('/api/tickets').json() == []
    assert client.get('/api/groups').json() == []
    assert client.get('/api/tickets'+query).json()[0]['id'] == ticket['id']
    assert client.post('/api/tickets', json={'title': 'Wrong', 'prompt': 'Test', 'group_id': group['id']}).status_code == 422
    assert client.put('/api/tickets/'+ticket['id']+'/group', json={'group_id': ''}).status_code == 200
    assert client.get('/api/tickets?project_id=missing').status_code == 404


def test_ticket_delete_restore_preserves_history(client):
    ticket = client.post('/api/tickets', json={'title': 'Task', 'prompt': 'Build it'}).json()
    path = '/api/tickets/'+ticket['id']
    assert client.delete(path).status_code == 200
    assert client.get('/api/tickets').json() == []
    assert client.get(path).status_code == 404
    assert client.get('/api/tickets?deleted=true').json()[0]['id'] == ticket['id']
    assert client.post(path+'/restore').json()['status'] == 'todo'
    assert client.get(path).json()['prompt'] == 'Build it'
    assert client.delete('/api/tickets/missing').status_code == 404


def test_project_delete_restore_and_edit(client):
    project = client.post('/api/projects', json={'name': 'Second'}).json()
    path = '/api/projects/'+project['id']
    assert client.put(path, json={'name': 'Renamed', 'description': 'Details'}).json()['name'] == 'Renamed'
    ticket = client.post('/api/tickets?project_id='+project['id'], json={'title': 'Task', 'prompt': 'Build it'}).json()
    assert client.delete(path).status_code == 200
    assert len(client.get('/api/projects').json()) == 1
    assert client.get(path).status_code == 404
    assert len(client.get('/api/projects?include_deleted=true').json()) == 2
    assert client.post('/api/tickets/'+ticket['id']+'/run').status_code == 404
    assert client.post(path+'/restore').json()['name'] == 'Renamed'
    assert client.get('/api/tickets?project_id='+project['id']).json()[0]['id'] == ticket['id']
    assert client.delete('/api/projects/default').status_code == 200
    assert client.delete(path).status_code == 200
    assert client.get('/api/projects').json() == []


def test_cannot_delete_active_ticket_or_project(client):
    ticket = client.post('/api/tickets', json={'title': 'Task', 'prompt': 'Build it'}).json()
    stored = client.app.state.store.get(ticket['id'])
    stored.status = 'queued'; stored.queued_at = '2099'
    client.app.state.store.save(stored)
    assert client.delete('/api/tickets/'+ticket['id']).status_code == 409
    assert client.delete('/api/projects/default').status_code == 409


def test_legacy_project_migrates_without_losing_metadata(tmp_path):
    path = tmp_path/'legacy.db'
    db = sqlite3.connect(path)
    db.execute('CREATE TABLE project(id INTEGER PRIMARY KEY, data TEXT NOT NULL)')
    db.execute('INSERT INTO project VALUES(1,?)', ('{"name":"Existing project","description":"Preserve me"}',))
    db.commit(); db.close()
    store = Store(path)
    assert store.project().name == 'Existing project'
    assert store.projects()[0].id == 'default'
    assert store.projects()[0].description == 'Preserve me'
    store.close(); store = Store(path)
    assert len(store.projects()) == 1
    store.close()


def test_restored_ticket_survives_group_removal(client):
    group = client.post('/api/groups', json={'name': 'Feature'}).json()
    ticket = client.post('/api/tickets', json={'title': 'Task', 'prompt': 'Build', 'group_id': group['id']}).json()
    path = '/api/tickets/'+ticket['id']
    client.delete(path)
    client.delete('/api/groups/'+group['id'])
    assert client.post(path+'/restore').json()['group_id'] == ''
