"""Every permutation: damage level x (no defense / Holdfast), the rest of the XP in 10-XP blank cards, at each total XP.
Prices: board 40 XP per point of average damage above 6.2, Holdfast 140 XP, no starting deck. Every build plays every
other build (race model, exact card counts): win / draw / loss."""
import json
import numpy as np
from field import mean_through, OUT, BOARD
from eight import wld, at_avg
A0 = at_avg(6.2)[1]
LEVELS = [400, 650, 900, 1200, 1500]
DMGS = [6.2, 8, 10, 12, 14, 16, 18]
out = {'levels': LEVELS, 'tables': {}}
for X in LEVELS:
    builds = []
    for T in DMGS:
        i, a = at_avg(T)
        for d in ['none', 'holdfast']:
            left = X - 40 * (a - A0) - (140 if d == 'holdfast' else 0)
            n = int(left // 10)
            builds.append(dict(damage=round(a, 2), defense=d, cards=n, board=round(40 * (a - A0)), ok=n >= 1, ti=i))
    M = []
    for A in builds:
        row = []
        for B in builds:
            if A is B or not A['ok'] or not B['ok']:
                row.append(None); continue
            w, dr, l = wld((A['ti'], A['defense'], A['cards']), (B['ti'], B['defense'], B['cards']))
            ra = A['cards'] / mean_through(B['ti'], A['defense']); rb = B['cards'] / mean_through(A['ti'], B['defense'])
            row.append([round(w, 3), round(dr, 3), round(l, 3), round(rb, 1), round(ra, 1)])  # rounds A needs, rounds B needs
        M.append(row)
    out['tables'][X] = dict(builds=[{k: v for k, v in b.items() if k != 'ti'} for b in builds], matrix=M)
    means = []
    for bi, b in enumerate(builds):
        vals = [c[0] + c[1] / 2 for c in M[bi] if c]
        means.append(round(float(np.mean(vals)), 3) if vals else None)
    out['tables'][X]['means'] = means
    print(X, 'done:', ', '.join(f"{b['damage']}/{'H' if b['defense']=='holdfast' else '-'}/{b['cards']}c={m}" for b, m in zip(builds, means)), flush=True)
json.dump(out, open(f'{OUT}/full_grid.json', 'w'), separators=(',', ':'))
