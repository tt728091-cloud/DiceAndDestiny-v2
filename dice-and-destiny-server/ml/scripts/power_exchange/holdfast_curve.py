"""Holdfast's fair price at every XP level. Characters: no defense and Holdfast (plus Light Guard as a check).
Each picks its damage (staircase, board 40 XP a point above 6.2) and fills the rest with 10-XP blank cards (no starting
deck). Holdfast's fair price is the XP at which, with both characters playing their best split, it scores 0.5."""
import json, sys
import numpy as np
import chars as C
from chars import solve, BOARD, DEF_AVG
from field import OUT
C.BASE_CARDS = 0
RATE = 40
def run(X, price, chars_=('none', 'holdfast'), extra=None):
    costs = {'none': 0, 'holdfast': price}
    if extra: costs.update(extra)
    return solve(list(chars_), X, c=RATE, defense_cost=lambda d: costs[d], iters=4000, step=0.25)
def fair_price(X, chars_=('none', 'holdfast'), extra=None):
    lo, hi = 0.0, X * 0.8
    for _ in range(14):
        mid = (lo + hi) / 2
        v = run(X, mid, chars_, extra)['chars']['holdfast']['value']
        if v > 0.5: lo = mid
        else: hi = mid
    return (lo + hi) / 2
if __name__ == '__main__':
    rows = []
    for X in [int(a) for a in sys.argv[1:]] or [300, 400, 500, 650, 800, 900, 1100, 1300, 1500]:
        F = fair_price(X)
        r = run(X, F)
        n, h = r['chars']['none'], r['chars']['holdfast']
        board_h = RATE * (h['damage'] - float(BOARD.averages[BOARD.index(6.2)]))
        share = F / max(1, X - board_h)
        row = dict(X=X, fair=F, share_after_board=share, share_total=F / X, none=n, holdfast=h, pair=r['pairs'])
        rows.append(row)
        print(f"{X:5d} XP: Holdfast fair price {F:5.0f} XP = {100*F/X:4.1f}% of total XP = {100*share:4.1f}% of the XP left after its board | "
              f"no defense plays {n['damage']:.1f} dmg / {n['cards']:.0f} cards, Holdfast plays {h['damage']:.1f} dmg / {h['cards']:.0f} cards | Holdfast worth {F/10:.0f} cards", flush=True)
    json.dump(rows, open(f'{OUT}/holdfast_curve' + ('_fine' if len(sys.argv) > 3 else '') + '.json', 'w'), indent=1)
