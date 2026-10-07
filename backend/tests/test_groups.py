def test_group_lifecycle(client):
    group = client.post('/api/groups', json={'name': 'Navigation', 'color': 'orange'}).json()
    assert group['id']
    ticket = client.post('/api/tickets', json={'title': 'Links', 'prompt': 'Add links', 'group_id': group['id']}).json()
    assert ticket['group_id'] == group['id']
    assert client.patch('/api/groups/' + group['id'], json={'name': 'Navbar', 'color': 'blue'}).json()['name'] == 'Navbar'
    client.post(f"/api/tickets/{ticket['id']}/run")
    assert client.put(f"/api/tickets/{ticket['id']}/group", json={'group_id': ''}).json()['group_id'] == ''
    assert client.put(f"/api/tickets/{ticket['id']}/group", json={'group_id': group['id']}).status_code == 200
    assert client.delete('/api/groups/' + group['id']).status_code == 200
    assert client.get('/api/tickets/' + ticket['id']).json()['group_id'] == ''
    assert client.get('/api/groups').json() == []


def test_group_boundaries(client):
    assert client.post('/api/groups', json={'name': ' ', 'color': 'orange'}).status_code == 422
    assert client.post('/api/groups', json={'name': 'Test', 'color': 'invalid'}).status_code == 422
    assert client.post('/api/tickets', json={'title': 'Test', 'prompt': 'Test', 'group_id': 'missing'}).status_code == 404
    ticket = client.post('/api/tickets', json={'title': 'Test', 'prompt': 'Test'}).json()
    assert ticket['group_id'] == ''
    assert client.patch('/api/tickets/' + ticket['id'], json={'group_id': 'missing'}).status_code == 404
    assert client.put('/api/tickets/missing/group', json={'group_id': ''}).status_code == 404
    assert client.put('/api/tickets/' + ticket['id'] + '/group', json={'group_id': 'missing'}).status_code == 404
    assert client.patch('/api/groups/missing', json={'name': 'Test', 'color': 'blue'}).status_code == 404
    assert client.delete('/api/groups/missing').status_code == 404


def test_finished_group_changes_preserve_history(client):
    import time
    group = client.post('/api/groups', json={'name': 'Hero', 'color': 'green'}).json()
    ticket = client.post('/api/tickets', json={'title': 'Hero', 'prompt': 'Build hero', 'group_id': group['id']}).json()
    client.post(f"/api/tickets/{ticket['id']}/run")
    for _ in range(100):
        result = client.get('/api/tickets/' + ticket['id']).json()
        if result['status'] == 'done':
            break
        time.sleep(.01)
    assert result['status'] == 'done'
    client.delete('/api/groups/' + group['id'])
    ungrouped = client.get('/api/tickets/' + ticket['id']).json()
    assert ungrouped['group_id'] == ''
    assert ungrouped['response'] == result['response']
    assert ungrouped['files'] == result['files']
    history = client.get(f"/api/tickets/{ticket['id']}/runs").json()
    assert history[0]['snapshot']['group_id'] == group['id']


def test_groups_persist_and_old_tickets_migrate(tmp_path):
    from app.store import Store
    from app.models import Group
    path = tmp_path / 'groups.sqlite3'
    store = Store(path)
    store.save_group(Group(id='g1', name='Persisted', color='plum'))
    store.db.execute('INSERT INTO tickets(data) VALUES (?)', ('{"id":"SW-001","title":"Old","prompt":"Prompt","created_at":"today"}',))
    store.db.commit()
    store.close()
    reopened = Store(path)
    assert reopened.groups()[0].name == 'Persisted'
    assert reopened.get('SW-001').group_id == ''
    reopened.close()


def test_custom_group_colour(client):
    group = client.post('/api/groups', json={'name': 'Custom', 'color': '#a1B2c3'}).json()
    assert group['color'] == '#a1B2c3'
    assert client.get('/api/groups').json()[0]['color'] == '#a1B2c3'
    assert client.patch('/api/groups/' + group['id'], json={'name': 'Custom', 'color': '#ffffff'}).json()['color'] == '#ffffff'
    for color in ['#abc', '#12345678', 'red', 'var(--x)', '#xxxxxx', 'orange;background:red']:
        assert client.post('/api/groups', json={'name': 'Invalid', 'color': color}).status_code == 422
