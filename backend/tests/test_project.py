def test_project_fields_persist(client):
    assert client.get('/api/project').json()=={'name':'My first project','description':'','screenshot':''}
    value={'screenshot':'','name':'Navbar builder','description':'Build a clear, accessible navigation.\nAnimate the logo.'}
    assert client.put('/api/project',json=value).json()==value
    assert client.get('/api/project').json()==value
    assert client.put('/api/project',json={'name':'  Changed  ','description':' '}).json()=={'name':'Changed','description':'','screenshot':''}


def test_invalid_project_fields(client):
    for value in [{'name':' '},{'name':'x'*121},{'name':'Valid','description':'x'*2001}]:
        assert client.put('/api/project',json=value).status_code==422
    assert client.get('/api/project').json()['name']=='My first project'


def test_project_survives_store_restart(tmp_path):
    from app.store import Store
    from app.models import Project
    path=tmp_path/'db';store=Store(path)
    store.save_project(Project(name='My app',description='Description'))
    store.close();store=Store(path)
    assert store.project().description=='Description'
    store.close()
