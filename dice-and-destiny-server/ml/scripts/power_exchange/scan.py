import sys, json
from study import *
budgets = [100, 200, 400, 700, 1000, 1500]
out = []
for c in [30, 40, 50, 60, 80]:
    for k in [0.8, 1.0, 1.25]:
        cost = flat(c, k)
        rows = []
        for b in budgets:
            r = analyse(cost, b, verbose=False)
            rows.append(r)
        worst_def = min(min(s for s, _ in r['best_by_defense'].values()) for r in rows if len(r['best_by_defense']) == 6)
        line = f'c={c:3d} k={k:4.2f} | weakest defense vs eq (all-6 budgets) {worst_def:.3f} | best-of-defense worst pairing: ' + ' '.join(f'{r["best_min"]:.2f}' for r in rows) + ' | within eq: ' + ' '.join(f'{r["support_min"]:.2f}' for r in rows)
        print(line, flush=True)
        out.append(dict(c=c, k=k, rows=rows))
json.dump(out, open(f'{OUT}/scan_flat.json', 'w'))
