"""The big table with block defenses. Builds: damage 6.2-18 x {no defense, Holdfast, 1-die block (a 6), 2-dice block
(any 6)}, the rest in 10-XP blank cards (no starting deck). Prices: board 40 XP per point above 6.2; Holdfast 140 XP;
block defenses a share of the XP left after the board (1-die 15.5%, 2-dice 28%), from block_curve.py."""
import json
import numpy as np
import rolled  # registers block defenses
from field import OUT
from eight import wld, at_avg
A0 = at_avg(6.2)[1]
LEVELS = [400, 650, 900, 1200, 1500]
DMGS = [6.2, 8, 10, 12, 14, 16, 18]
DEFS = [('none', None), ('holdfast', ('flat', 140)), ('block1', ('share', 0.155)), ('block2', ('share', 0.28))]
out = {'levels': LEVELS, 'defs': [d for d, _ in DEFS], 't': {}}
for X in LEVELS:
    B = []
    for T in DMGS:
        i, a = at_avg(T)
        board = 40 * (a - A0)
        for d, price in DEFS:
            left = X - board
            dxp = 0 if price is None else (price[1] if price[0] == 'flat' else price[1] * left)
            n = int((left - dxp) // 10)
            B.append(dict(d=a, f=d, c=n, bx=round(board), dx=round(dxp), ok=n >= 1, ti=i))
    M = []
    for A in B:
        row = []
        for C in B:
            if A is C or not A['ok'] or not C['ok']:
                row.append(None); continue
            w, dr, l = wld((A['ti'], A['f'], A['c']), (C['ti'], C['f'], C['c']))
            row.append([round(1000 * w), round(1000 * dr)])
        M.append(row)
    out['t'][X] = {'b': [[round(b['d'], 2), DEFS.index(next(x for x in DEFS if x[0] == b['f'])), b['c'], b['bx'], b['dx'], 1 if b['ok'] else 0] for b in B], 'm': M}
    print(X, 'done', flush=True)
s = json.dumps(out, separators=(',', ':'))
open(f'{OUT}/grid2_compact.json', 'w').write(s)
print(len(s), 'bytes')
