import itertools
from chars import *
four = ['none', 'light', 'holdfast', 'heavy']
budgets = [100, 150, 200, 250, 300, 400, 500, 600, 900]
def evaluate(cost):
    worst_v, lop_n, lop_tot = 0, 0, 0
    per = []
    for budget in budgets:
        r = solve(four, budget, cost_fn=cost, iters=4000)
        vs = [x['value'] for x in r['chars'].values()]
        dev = max(abs(v - 0.5) for v in vs); worst_v = max(worst_v, dev)
        per.append(dev)
    return worst_v, per
rows = []
for c0, length in [(40, 0), (35, 2), (30, 2), (30, 3), (25, 3), (25, 4), (20, 4), (20, 6), (25, 6), (30, 5)]:
    slope = 0 if length == 0 else (40 - c0) / length
    cost = level_price(c0, slope, 40)
    w, per = evaluate(cost)
    print(f'{c0:2d} XP at level 6.2, reaching 40 after {length} points | worst character |value-0.5| {w:.3f} | by budget ' + ' '.join(f'{p:.3f}' for p in per), flush=True)
