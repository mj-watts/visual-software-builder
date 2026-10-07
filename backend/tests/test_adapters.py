import asyncio
import json
import os
import sys
from pathlib import Path
from types import SimpleNamespace

import pytest
from app.adapters import CliAgent, child_environment, codex_command, claude_command, redact
from app.models import Draft, Run
from app.settings import Settings

@pytest.fixture
def context(tmp_path):
    worktree=tmp_path/'run'/'worktree';worktree.mkdir(parents=True)
    ticket=Draft(title='Example',prompt='Build it',permission='workspace-write',images=['data:image/png;base64,cGl4ZWxz'])
    run=Run(id='one',ticket_id='SW-001',snapshot=ticket,worktree=str(worktree),created_at='now')
    settings=Settings(artifacts=tmp_path,keys={'codex':'secret-test-key','claude':'another-secret'},binaries={'codex':'codex','claude':'claude'})
    return ticket,run,settings


def test_commands_and_credentials(context):
    ticket,run,settings=context
    argv,text=codex_command(settings,ticket,run)
    assert '--sandbox' in argv and '--image' in argv
    assert 'secret-test-key' not in argv
    assert Path(argv[argv.index('--image')+1]).read_bytes()==b'pixels'
    env=child_environment(settings,'codex',run)
    assert env['CODEX_API_KEY']=='secret-test-key'
    assert env['CODEX_HOME'].endswith('codex-home')
    assert 'ANTHROPIC_API_KEY' not in child_environment(settings,'tests',run)
    argv,text=claude_command(settings,ticket,run)
    assert '--restricted' in argv and '--bare' in argv
    assert 'Bash' not in argv[argv.index('--tools')+1]
    assert json.loads(text)['message']['content'][1]['source']['data']=='cGl4ZWxz'
    ticket.permission='read-only'
    argv,_=claude_command(settings,ticket,run)
    assert 'Write' not in argv[argv.index('--tools')+1]
    assert child_environment(settings,'claude',run)['ANTHROPIC_API_KEY']=='another-secret'
    assert redact('secret-test-key',settings)=='[redacted]'


@pytest.mark.parametrize('provider', ['codex','claude'])
def test_real_subprocess_protocol(context,tmp_path,provider):
    ticket,run,settings=context
    script=tmp_path/'fake-agent'
    events=[{'type':'item.completed','item':{'type':'agent_message','text':'Implemented secret-test-key'}},{'type':'turn.completed'}] if provider=='codex' else [{'type':'assistant','message':{'content':[{'type':'text','text':'Working'}]}},{'type':'result','result':'Implemented','is_error':False}]
    script.write_text(f'#!{sys.executable}\nimport json,sys\nsys.stdin.read()\n'+ '\n'.join('print('+repr(json.dumps(e))+',flush=True)' for e in events)+'\n')
    script.chmod(0o755);settings.binaries[provider]=str(script)
    async def scenario():
        seen=[]
        async def emit(kind,message):seen.append(message)
        response=await CliAgent(settings,provider)(ticket,run,emit)
        assert 'Implemented' in response
        assert all('secret-test-key' not in value for value in seen)
    asyncio.run(scenario())


def test_provider_events_failure_and_unfinished(context,monkeypatch):
    ticket,run,settings=context
    async def fake_process(argv,cwd,text,emit,timeout,env):
        await emit('stdout','not json')
        await emit('stderr','secret-test-key')
    monkeypatch.setattr('app.adapters.run_process',fake_process)
    async def scenario():
        seen=[]
        async def emit(kind,message):seen.append(message)
        with pytest.raises(RuntimeError,match='completion event'):
            await CliAgent(settings,'codex')(ticket,run,emit)
        agent=CliAgent(settings,'codex')
        await agent.consume('stdout',json.dumps({'type':'turn.failed'}),emit)
        assert agent.failure
        agent.codex_event({'type':'error'})
        agent=CliAgent(settings,'claude')
        agent.claude_event({'type':'result','is_error':True,'permission_denials':[{}]})
        assert agent.failure
        assert '[redacted]' in seen
        async def failed_process(argv,cwd,text,emit,timeout,env):
            await emit('stdout',json.dumps({'type':'turn.failed'}))
        monkeypatch.setattr('app.adapters.run_process',failed_process)
        with pytest.raises(RuntimeError,match='unsuccessful'):
            await CliAgent(settings,'codex')(ticket,run,emit)
        async def empty_process(argv,cwd,text,emit,timeout,env):
            await emit('stdout',json.dumps({'type':'turn.completed'}))
        monkeypatch.setattr('app.adapters.run_process',empty_process)
        assert 'without a text summary' in await CliAgent(settings,'codex')(ticket,run,emit)
    asyncio.run(scenario())


def test_env_file_is_data_and_preserves_existing_values(tmp_path,monkeypatch):
    from app.settings import load_env
    path=tmp_path/'env'
    load_env(path)
    path.write_text('# comment\n\nSWIMLANE_TEST_VALUE="first"\nINVALID LINE\nBAD KEY=no\n')
    monkeypatch.delenv('SWIMLANE_TEST_VALUE',raising=False)
    load_env(path)
    assert os.environ['SWIMLANE_TEST_VALUE']=='first'
    path.write_text('SWIMLANE_TEST_VALUE=second')
    load_env(path)
    assert os.environ['SWIMLANE_TEST_VALUE']=='first'
