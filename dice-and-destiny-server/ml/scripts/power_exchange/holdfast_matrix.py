"""Holdfast's fair price one-on-one: a Holdfast buyer with damage T_self against a no-defense opponent with damage
T_opp, both at the same total XP (board 40 XP a point above 6.2, the rest in 10-XP cards, no starting deck). The fair
price is the XP that makes the matchup 50/50 (smoothed over +-2 cards)."""
import json
import numpy as np
from field import score, OUT
from eight import at_avg
A0 = at_avg(6.2)[1]
def build(T, d, X, extra=0):
    i, a = at_avg(T)
    return (i, d, max(1, int((X - 40 * (a - A0) - extra) // 10)))
def fair(X, Ts, To):
    opp = build(To, 'none', X)
    f = lambda F: score(build(Ts, 'holdfast', X, F), opp)
    lo, hi = 0.0, X - 40 * (at_avg(Ts)[1] - A0) - 10
    if f(hi) > 0.5: return float('inf')
    if f(lo) < 0.5: return float('nan')
    for _ in range(18):
        mid = (lo + hi) / 2
        if f(mid) > 0.5: lo = mid
        else: hi = mid
    return (lo + hi) / 2
rows = []
for X in [400, 650, 900, 1200]:
    print(f'\n## {X} XP: fair Holdfast price in XP (and in cards); rows = buyer damage, columns = opponent damage (no defense)')
    opps = [7, 9, 12, 15, 18]
    print('buyer ' + ''.join(f'{"vs "+str(o):>16s}' for o in opps))
    for Ts in [7, 9, 12, 15]:
        cells = []
        for To in opps:
            F = fair(X, Ts, To)
            rows.append(dict(X=X, self=Ts, opp=To, fair=F))
            cells.append(f'{"more than all" if F == float("inf") else ("never" if F != F else f"{F:4.0f} ({F/10:.0f}c)"):>16s}')
        print(f'{Ts:5d} ' + ''.join(cells), flush=True)
json.dump(rows, open(f'{OUT}/holdfast_matrix.json', 'w'), indent=1, default=str)
