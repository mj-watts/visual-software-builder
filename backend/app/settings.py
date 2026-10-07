"""Server-only execution configuration. Keys are never returned by the API."""
import os
import shutil
from dataclasses import dataclass, field
from pathlib import Path

PROJECT = Path(__file__).resolve().parents[2]

@dataclass
class Settings:
    roots: list[Path] = field(default_factory=lambda: [PROJECT])
    artifacts: Path = PROJECT / 'backend' / '.swimlane'
    timeout: float = 600
    enabled: dict = field(default_factory=dict)
    binaries: dict = field(default_factory=dict)
    keys: dict = field(default_factory=dict, repr=False)

    @classmethod
    def from_env(cls):
        roots = os.environ.get('SWIMLANE_WORKSPACE_ROOTS', str(PROJECT)).split(os.pathsep)
        keys = {'codex': os.environ.get('CODEX_API_KEY') or os.environ.get('OPENAI_API_KEY', ''),
                'claude': os.environ.get('ANTHROPIC_API_KEY', '')}
        binaries = {provider: shutil.which(provider) for provider in keys}
        return cls(roots=[Path(root).resolve() for root in roots if root],
                   artifacts=Path(os.environ.get('SWIMLANE_ARTIFACTS', str(PROJECT / 'backend' / '.swimlane'))).resolve(),
                   timeout=max(1, min(3600, float(os.environ.get('SWIMLANE_RUN_TIMEOUT', '600')))),
                   enabled={p: bool(binaries[p] and keys[p]) for p in keys}, binaries=binaries, keys=keys)

    def providers(self):
        return [{'id': 'demo', 'name': 'Demo agent', 'available': True, 'reason': 'Simulated execution'}] + [
            {'id': p, 'name': name, 'available': self.enabled.get(p, False),
             'reason': 'Configured · authentication checked during run' if self.enabled.get(p) else f'Set {key} on the backend and install {p} CLI'}
            for p, name, key in [('codex', 'Codex', 'CODEX_API_KEY or OPENAI_API_KEY'), ('claude', 'Claude', 'ANTHROPIC_API_KEY')]]


def load_env(path):
    if not path.exists():
        return
    for raw in path.read_text().splitlines():
        line = raw.strip()
        if not line or line.startswith('#'):
            continue
        key, separator, value = line.partition('=')
        if separator and key.replace('_', '').isalnum():
            os.environ.setdefault(key, value.strip().strip('"\''))
