"""Measure per-function CRAP using Radon complexity and pytest line coverage."""
import json
from pathlib import Path
from radon.complexity import cc_visit

root=Path(__file__).resolve().parents[1]
coverage=json.loads((root/'backend/coverage.json').read_text())['files']
rows=[]
seen=set()
def inspect(blocks, file, covered, executable):
    for block in blocks:
        if hasattr(block,'methods'):
            inspect(block.methods,file,covered,executable)
            continue
        key=(file,block.lineno,block.name)
        if key in seen:
            continue
        seen.add(key)
        lines=set(range(block.lineno,block.endline+1)) & executable
        rate=len(lines & covered)/len(lines) if lines else 1
        score=block.complexity**2 * (1-rate)**3 + block.complexity
        rows.append({'file':file,'function':block.name,'line':block.lineno,'complexity':block.complexity,'coverage':round(rate,3),'crap':round(score,3)})
        inspect(getattr(block,'closures',[]),file,covered,executable)

for file in (root/'backend/app').glob('*.py'):
    relative='app/'+file.name
    data=coverage[relative]
    covered=set(data['executed_lines'])
    inspect(cc_visit(file.read_text()),relative,covered,covered | set(data['missing_lines']))
(root/'docs/backend-quality.json').write_text(json.dumps(rows,indent=2)+'\n')
fail=[row for row in rows if row['crap']>=10]
print(f'{len(rows)} Python functions; maximum CRAP {max(r["crap"] for r in rows):.3f}')
if fail:
    print(json.dumps(fail,indent=2))
    raise SystemExit(1)
