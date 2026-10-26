import json, itertools, sys
import numpy as np
import field, balance_search as B
from balance_search import at_avg, wld, LOW
for p in [0.10, 0.15, 0.20, 0.25, 0.30, 0.37]:
    field.PCT[f'ward{int(round(p*100))}'] = p
MODE = {'round': 'floor'}
def cards(X, share):
    v = X * share / 10
    return max(1, int(v + 1e-9) if MODE['round'] == 'floor' else int(np.floor(v + 0.5)))
B.cards = cards
def worst_at(high, d, r, g, X):
    bs = B.builds(X, high, d, r, g); w = 0
    for i, j in itertools.combinations(range(4), 2):
        a, dr, l = wld(bs[i], bs[j]); w = max(w, abs(a + dr / 2 - 0.5))
    return w
def profile(high, d, r, g, xs):
    return np.array([worst_at(high, d, r, g, X) for X in xs])
XS = list(range(300, 1501, 10))
def summary(p, xs=XS):
    out = []
    for lo, hi in [(300, 390), (400, 690), (700, 990), (1000, 1500)]:
        sel = [v for X, v in zip(xs, p) if lo <= X <= hi]
        out.append(f'{lo}-{hi}: {100*np.mean(np.array(sel) <= 0.1):3.0f}% in (worst {50-100*max(sel):.0f})')
    return ' | '.join(out)
if __name__ == '__main__':
    cands = [(9, 'light'), (12, 'half'), (10, 'ward15'), (10, 'ward37'), (13, 'ward37')]
    for high, d in cands:
        r = at_avg(LOW)[1] / at_avg(high)[1]
        g = 1 - field.PCT[d] if d in field.PCT else {'light': 0.86, 'half': 0.928}[d]
        for mode in ['floor', 'nearest']:
            MODE['round'] = mode
            p = profile(high, d, r, g, XS)
            print(f'high {at_avg(high)[1]:5.2f} {d:7s} {mode:7s}: {summary(p)}', flush=True)
