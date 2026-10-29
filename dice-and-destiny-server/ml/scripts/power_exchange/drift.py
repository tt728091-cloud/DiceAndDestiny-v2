"""RPS width: start from the even family (same damage T, cards = N x (T - prevention)) priced at c = 10N per damage or
prevention point, then let one character move damage into cards (or back) at equal XP: -1 damage point = +N cards.
Report that character's worst and best score against the others."""
from field import *
from families import at_avg
DEFS = ['none', 'light', 'holdfast', 'heavy']
for N in [4, 5]:
    print(f'\n## N={N} (c = {10*N} XP a point): one character drifts; worst / best score against the others')
    print('T      defense    ' + '  '.join(f'{d:+d}dmg' .rjust(11) for d in [-3, -2, -1, 1, 2, 3]))
    for T in [7.5, 9, 11, 14, 17]:
        i, a = at_avg(T)
        fam = [(i, d, int(round(N * (a - DEF_AVG[d])))) for d in DEFS if a - DEF_AVG[d] >= 2.5]
        for k, (ti, d, h) in enumerate(fam):
            cells = []
            for delta in [-3, -2, -1, 1, 2, 3]:
                j, a2 = at_avg(a + delta)
                h2 = h + int(round(-(a2 - a) * N))
                if h2 < 3 or a2 < 5.05:
                    cells.append('      -    '); continue
                s = [score((j, d, h2), o) for kk, o in enumerate(fam) if kk != k]
                cells.append(f'{min(s):.2f}/{max(s):.2f}'.rjust(11))
            print(f'{a:5.2f}  {d:9s} {h:3d}c ' + '  '.join(cells))
