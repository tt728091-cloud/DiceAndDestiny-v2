"""Fair price of each block defense at every XP level, the same way as Holdfast (holdfast_curve.py): a no-defense
character and a character with the defense each pick their damage (board 40 XP a point above 6.2) and fill the rest
with 10-XP cards (no starting deck); the fair price makes the defense win half the time."""
import json, sys
import numpy as np
import rolled  # registers block1 / block1b / block2 / block3
import chars as C
from chars import solve, BOARD
from field import OUT
C.BASE_CARDS = 0
A0 = float(BOARD.averages[BOARD.index(6.2)])
def run(X, d, price):
    return solve(['none', d], X, c=40, defense_cost=lambda q: price if q == d else 0, iters=4000, step=0.25)
def fair(X, d):
    lo, hi = 0.0, X * 0.8
    for _ in range(13):
        mid = (lo + hi) / 2
        if run(X, d, mid)['chars'][d]['value'] > 0.5: lo = mid
        else: hi = mid
    return (lo + hi) / 2
defs = sys.argv[1].split(',') if len(sys.argv) > 1 else ['block1', 'block1b', 'block2']
levels = [300, 400, 500, 650, 800, 900, 1100, 1300, 1500]
rows = []
for d in defs:
    for X in levels:
        F = fair(X, d); r = run(X, d, F)
        n, h = r['chars']['none'], r['chars'][d]
        left = X - 40 * (h['damage'] - A0)
        rows.append(dict(defense=d, X=X, fair=F, share_total=F / X, share_left=F / left, none=n, d=h))
        print(f"{d:8s} {X:5d} XP: fair {F:5.0f} XP = {100*F/X:4.1f}% of total = {100*F/left:4.1f}% of XP left after its board | none {n['damage']:.1f} dmg/{n['cards']:.0f}c, {d} {h['damage']:.1f} dmg/{h['cards']:.0f}c", flush=True)
json.dump(rows, open(f'{OUT}/block_curve.json', 'w'), indent=1)
