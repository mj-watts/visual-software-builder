import asyncio
import base64
import pytest
from pydantic import ValidationError
from app.models import Draft
from app.store import Store
from app.runner import Runner
from app.settings import Settings
from app.runs import queue_run
from app.main import seed

def test_recovery_and_seed(tmp_path):
    store=Store(tmp_path/'seed.sqlite3')
    seed(store)
    seed(store)
    assert len(store.all())==3
    ticket=store.all()[0]
    ticket.status='running'
    store.save(ticket)
    store.recover()
    assert store.get(ticket.id).status=='failed'
    store.close()

def test_worker_failure(tmp_path):
    class BrokenAgent:
        async def run(self,ticket):
            raise RuntimeError('failure')
    store=Store(tmp_path/'worker.sqlite3')
    ticket=store.create(Draft(title='title',prompt='prompt'))
    settings=Settings(artifacts=tmp_path/'runs')
    queue_run(store,ticket,settings)
    runner=Runner(store,settings,0)
    runner.demo=BrokenAgent()
    asyncio.run(runner.execute(ticket))
    assert store.get(ticket.id).status=='failed'
    store.close()

def test_images():
    valid='data:image/png;base64,'+base64.b64encode(b'pixels').decode()
    assert Draft(title='title',prompt='prompt',images=[valid]).images==[valid]
    for value in ['data:image/png;base64,','data:image/png;base64,%%%','data:image/png;base64,'+'a'*6_666_669]:
        with pytest.raises(ValidationError):
            Draft(title='title',prompt='prompt',images=[value])

def test_patch_invalid(client):
    ticket=client.post('/api/tickets',json={'title':'test','prompt':'test'}).json()
    assert client.patch('/api/tickets/'+ticket['id'],json={'title':' '}).status_code==422
