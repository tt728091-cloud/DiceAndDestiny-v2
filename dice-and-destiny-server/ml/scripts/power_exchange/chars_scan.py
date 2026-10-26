import json
from chars import *
chars = ['none', 'light', 'holdfast', 'heavy']
budgets = [150, 250, 400, 600, 900, 1300]
out = []
for c in [30, 40, 50, 60]:
    for k in [0.7, 0.85, 1.0, 1.15]:
        rows = [solve(chars, b, c, k) for b in budgets]
        cells = []
        for r in rows:
            vs = [x['value'] for x in r['chars'].values()]; pv = list(r['pairs'].values())
            cells.append(f"{min(vs):.2f}-{max(vs):.2f}/{min(pv):.2f}")
        print(f'c={c} k={k:.2f} | char values / lowest matchup per budget {budgets}: ' + '  '.join(cells), flush=True)
        out.append(dict(c=c, k=k, rows=rows))
json.dump(out, open(f'{OUT}/chars_scan.json', 'w'))
