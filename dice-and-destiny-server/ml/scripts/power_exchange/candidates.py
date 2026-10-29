import json, itertools
import numpy as np
import field
from balance_search import *
for p in [0.10, 0.15, 0.20, 0.25, 0.30, 0.37]:
    field.PCT[f'ward{int(round(p*100))}'] = p
search_flat = {(x['high'], x['defense']): x for x in json.load(open(f'{OUT}/balance_search.json'))}
search_pct = {(x['high'], x['defense']): x for x in json.load(open(f'{OUT}/balance_pct.json'))}
def dense(high, d, r, g, xs=range(300, 1501, 10)):
    worst, inside = 0, 0
    for X in xs:
        bs = builds(X, high, d, r, g); w = 0
        for i, j in itertools.combinations(range(4), 2):
            a, dr, l = wld(bs[i], bs[j]); w = max(w, abs(a + dr / 2 - 0.5))
        worst = max(worst, w); inside += w <= 0.1
    return worst, inside / len(xs)
cands = [(9, 'light', 'flat'), (12, 'light', 'flat'), (12, 'half', 'flat'), (10, 'ward15', 'pct'), (12, 'ward25', 'pct'), (13, 'ward37', 'pct'), (10, 'ward37', 'pct')]
for high, d, kind in cands:
    x = (search_flat if kind == 'flat' else search_pct)[(high, d)]
    r, g = x['r'], x['g']
    # plain formula: r = 6.91/high_avg, g = 1 - p (pct) or fitted-free estimate 1 - D/mean(m) for flat
    r0 = at_avg(LOW)[1] / at_avg(high)[1]
    g0 = 1 - field.PCT[d] if d in field.PCT else 1 - field.DEF_AVG[d] / np.sqrt(at_avg(LOW)[1] * at_avg(high)[1])
    w0, _ = evaluate(high, d, r0, g0, [400, 650, 900])
    dw, din = dense(high, d, r, g)
    print(f'high {at_avg(high)[1]:5.2f} {d:7s}: fitted r {r:.3f} g {g:.3f} -> 400/650/900 worst {50-100*x["worst"]:.1f}-{50+100*x["worst"]:.1f}; '
          f'plain formula r {r0:.3f} g {g0:.3f} -> {50-100*w0:.1f}-{50+100*w0:.1f}; every 10 XP 300-1500: worst {50-100*dw:.1f}-{50+100*dw:.1f}, inside 40-60 at {100*din:.0f}% of levels', flush=True)
