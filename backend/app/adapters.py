"""Provider-specific CLI protocols; no shell strings or browser credentials."""
import base64
import json
import os
from pathlib import Path
from typing import Protocol
from .process import run_process

class CodingAgent(Protocol):
    async def __call__(self, ticket, run, emit) -> str: ...


def prompt_for(ticket):
    return ('Work only in this isolated working directory. Do not commit, push or merge. '
            'Follow the repository instructions. Do not start background services. '
            'Summarize your changes and any limitations. Tests will be run separately by the host if enabled.\n\n'
            + ticket.prompt)


def child_environment(settings, provider, run):
    env = {key: os.environ[key] for key in ['PATH', 'HOME', 'TMPDIR', 'LANG'] if key in os.environ}
    key = settings.keys.get(provider, '')
    if provider == 'codex':
        home = Path(run.worktree).parent / 'codex-home'
        home.mkdir(exist_ok=True)
        env.update(CODEX_HOME=str(home), CODEX_API_KEY=key)
    elif provider == 'claude':
        env['ANTHROPIC_API_KEY'] = key
    return env


def redact(text, settings):
    for key in filter(None, settings.keys.values()):
        text = text.replace(key, '[redacted]')
    return text


def write_images(ticket, run):
    directory = Path(run.worktree).parent / 'attachments'
    directory.mkdir(exist_ok=True)
    paths = []
    for index, data in enumerate(ticket.images):
        header, encoded = data.split(',', 1)
        extension = {'data:image/png;base64': 'png', 'data:image/jpeg;base64': 'jpg', 'data:image/webp;base64': 'webp'}[header]
        path = directory / f'reference-{index}.{extension}'
        path.write_bytes(base64.b64decode(encoded, validate=True))
        paths.append(path)
    return paths


def codex_command(settings, ticket, run):
    argv = [settings.binaries['codex'], '-a', 'never', 'exec', '--ignore-user-config', '--ignore-rules',
            '--sandbox', ticket.permission, '-c', 'shell_environment_policy.exclude=["CODEX_API_KEY","OPENAI_API_KEY","ANTHROPIC_API_KEY"]', '--ephemeral', '--json', '--color', 'never', '-C', run.worktree]
    for image in write_images(ticket, run):
        argv += ['--image', str(image)]
    return argv + ['-'], prompt_for(ticket)


def claude_command(settings, ticket, run):
    tools = 'Read,Glob,Grep'
    if ticket.permission == 'workspace-write':
        tools += ',Edit,Write'
    argv = [settings.binaries['claude'], '--print', '--bare', '--restricted', '--disable-slash-commands',
            '--no-session-persistence', '--strict-mcp-config', '--mcp-config', '{"mcpServers":{}}',
            '--permission-mode', 'dontAsk', '--tools', tools, '--allowedTools', tools,
            '--input-format', 'stream-json', '--output-format', 'stream-json', '--verbose']
    content = [{'type': 'text', 'text': prompt_for(ticket)}]
    content += [claude_image(image) for image in ticket.images]
    return argv, json.dumps({'type': 'user', 'message': {'role': 'user', 'content': content}}) + '\n'


def claude_image(data):
    header, encoded = data.split(',', 1)
    return {'type': 'image', 'source': {'type': 'base64', 'media_type': header[5:].split(';')[0], 'data': encoded}}


class CliAgent:
    def __init__(self, settings, provider):
        self.settings = settings
        self.provider = provider
        self.response = ''
        self.completed = False
        self.failure = ''

    async def __call__(self, ticket, run, emit):
        builder = codex_command if self.provider == 'codex' else claude_command
        argv, text = builder(self.settings, ticket, run)
        async def consume(kind, line):
            await self.consume(kind, line, emit)
        await run_process(argv, run.worktree, text, consume, self.settings.timeout,
                          child_environment(self.settings, self.provider, run))
        if self.failure:
            raise RuntimeError(self.failure)
        if not self.completed:
            raise RuntimeError('Provider ended without a successful completion event.')
        return self.response or 'Agent completed without a text summary. Review the actual Git diff.'

    async def consume(self, kind, line, emit):
        if kind == 'stderr':
            await emit('diagnostic', redact(line, self.settings))
            return
        try:
            data = json.loads(line)
        except json.JSONDecodeError:
            await emit('diagnostic', redact(line, self.settings))
            return
        messages = self.codex_event(data) if self.provider == 'codex' else self.claude_event(data)
        for message in messages:
            await emit('agent', redact(message, self.settings))

    def codex_event(self, data):
        kind = data.get('type', '')
        if kind == 'turn.completed':
            self.completed = True
        if kind in ['turn.failed', 'error']:
            self.failure = 'Codex reported an unsuccessful run. See activity for details.'
        item = data.get('item', {})
        if item.get('type') == 'agent_message':
            self.response = redact(item.get('text', ''), self.settings)
            return [self.response]
        return []

    def claude_event(self, data):
        if data.get('type') == 'result':
            self.completed = True
            self.response = redact(data.get('result', ''), self.settings)
            if data.get('is_error') or data.get('permission_denials'):
                self.failure = 'Claude reported an error or a denied tool permission. See activity for details.'
            return [self.response] if self.response else []
        content = data.get('message', {}).get('content', [])
        return [item['text'] for item in content if item.get('type') == 'text']
