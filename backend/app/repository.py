"""Git worktree creation and bounded, real diffs without changing the source checkout."""
import hashlib
import subprocess
from pathlib import Path
from .models import Workspace, WorkspaceInput, ChangedFile

MAX_DIFF = 500_000
MAX_FILES = 200


def git(path, *args):
    result = subprocess.run(['git', '-c', 'core.hooksPath=/dev/null', '-c', 'core.fsmonitor=false',
                             '-C', str(path), *args], capture_output=True, timeout=30)
    if result.returncode:
        raise ValueError('Git operation failed. Check the repository and saved HEAD.')
    return result.stdout


def resolve_workspace(value: WorkspaceInput, settings):
    path = Path(value.path).expanduser().resolve(strict=True)
    if not any(path.is_relative_to(root.resolve()) for root in settings.roots):
        raise ValueError('Repository is outside the configured workspace roots.')
    top = git(path, 'rev-parse', '--show-toplevel').decode().strip()
    if Path(top).resolve() != path:
        raise ValueError('Select the root of a Git repository.')
    git(path, 'rev-parse', '--verify', 'HEAD^{commit}')
    relative_directory(path, value.test_directory)
    id = 'ws-' + hashlib.sha256(str(path).encode()).hexdigest()[:16]
    return Workspace(**{**value.model_dump(), 'path': str(path), 'id': id})


def relative_directory(root, directory):
    relative = Path(directory)
    if relative.is_absolute():
        raise ValueError('Test directory must be relative to the repository.')
    target = (Path(root) / relative).resolve()
    if not target.is_relative_to(Path(root).resolve()):
        raise ValueError('Test directory must stay inside the worktree.')
    if not target.is_dir():
        raise ValueError('Test directory does not exist.')
    return target


def prepare_worktree(repo, target, base):
    target = Path(target)
    target.parent.mkdir(parents=True, exist_ok=True)
    git(repo, 'worktree', 'add', '--detach', str(target), base)
    return target


def collect_diff(worktree, base):
    untracked = git(worktree, 'ls-files', '--others', '--exclude-standard', '-z').split(b'\0')
    for name in filter(None, untracked):
        git(worktree, 'add', '--intent-to-add', '--', name.decode('utf8', errors='surrogateescape'))
    entries = list(filter(None, git(worktree, 'diff', '--no-ext-diff', '--no-textconv', '--no-renames', '--numstat', '-z', base).split(b'\0')))
    if len(entries) > MAX_FILES:
        raise ValueError('More than 200 changed files. Worktree retained; narrow the ticket before retrying.')
    return [file_diff(worktree, base, entry) for entry in entries]


def file_diff(worktree, base, entry):
    added, removed, raw_path = entry.split(b'\t', 2)
    path = raw_path.decode('utf8', errors='replace')
    content = git(worktree, 'diff', '--no-ext-diff', '--no-textconv', '--no-renames', base, '--', path)
    if len(content) > MAX_DIFF:
        raise ValueError('A file diff exceeds 500 KB. Worktree retained for manual review.')
    return ChangedFile(path=path, additions=number(added), deletions=number(removed), diff=content.decode('utf8', errors='replace'))


def number(value):
    return int(value) if value != b'-' else 0
