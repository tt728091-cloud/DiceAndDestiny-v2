"""Final page data: the balancing points found by robust_scan, each with its prices, exact tables at the XP levels
and every 10 XP from 300 to 1500. Cards are rounded to the nearest whole card."""
import json, itertools
import numpy as np
import field
from field import mean_through, OUT
from eight import wld, at_avg
for p in [0.10, 0.15, 0.20, 0.25, 0.30, 0.37]:
    field.PCT[f'ward{int(round(p*100))}'] = p
import balance_search  # registers the extra flat defenses ('half' etc.)
import rolled  # registers the block defenses
LOW = 7
A_LOW = at_avg(LOW)[1]
LEVELS = [400, 650, 900, 1200, 1500]
CURVE = list(range(300, 1501, 10))
CONFIGS = {
    'block2': dict(label='Recommended: roll 2 dice, any 6 blocks the attack; high 9', high=9, defense='block2', keep_def=0.704, kind='block'),
    'block1b': dict(label='Roll 1 die, 5–6 blocks; high 9', high=9, defense='block1b', keep_def=0.677, kind='block'),
    'block3': dict(label='Roll 3 dice, any 6 blocks (Holdfast strength); high 9', high=9, defense='block3', keep_def=0.579, kind='block'),
    'holdfast': dict(label='Holdfast at its best price; high 8', high=8, defense='holdfast', keep_def=0.52, kind='small'),
}
def cards(X, s):
    return max(1, int(np.floor(X * s / 10 + 0.5)))
out = {'levels': LEVELS, 'curve': CURVE, 'pairs': ['main'], 'schemes': {}}
for key, c in CONFIGS.items():
    a_hi = at_avg(c['high'])[1]
    keep_low, keep_high, keep_def = 6.2 / A_LOW, 6.2 / a_hi, c['keep_def']
    spec = [(LOW, 'none', keep_low), (LOW, c['defense'], keep_low * keep_def), (c['high'], 'none', keep_high), (c['high'], c['defense'], keep_high * keep_def)]
    def builds_at(X):
        res = []
        for O, D, s in spec:
            n = cards(X, s)
            keep_o = keep_low if O == LOW else keep_high
            oxp = X * (1 - keep_o); dxp = (X - oxp) * (1 - (keep_def if D != 'none' else 1))
            res.append(dict(damage=at_avg(O)[1], defense=D, cards=n, oxp=oxp, dxp=dxp, ti=at_avg(O)[0]))
        return res
    tables = {}
    for X in LEVELS:
        bs = builds_at(X); mat = [[None] * 4 for _ in range(4)]
        for i, j in itertools.permutations(range(4), 2):
            a, b = bs[i], bs[j]
            w, dr, l = wld((a['ti'], a['defense'], a['cards']), (b['ti'], b['defense'], b['cards']))
            mat[i][j] = dict(w=round(w, 4), d=round(dr, 4), l=round(l, 4), rounds=round(a['cards'] / mean_through(b['ti'], a['defense']), 2))
        tables[X] = dict(builds=[{k: (round(v, 2) if isinstance(v, float) else v) for k, v in b.items() if k != 'ti'} for b in bs], matrix=mat)
    curve = [[] for _ in range(4)]; lo_c, hi_c = [], []
    for X in CURVE:
        bs = builds_at(X); ps = []
        for i in range(4):
            vals = []
            for j in range(4):
                if j == i: continue
                w, dr, l = wld((bs[i]['ti'], bs[i]['defense'], bs[i]['cards']), (bs[j]['ti'], bs[j]['defense'], bs[j]['cards']))
                vals.append(w + dr / 2)
            ps += vals; curve[i].append(round(float(np.mean(vals)), 4))
        lo_c.append(round(min(ps), 4)); hi_c.append(round(max(ps), 4))
    out['schemes'][key] = dict(label=c['label'], defense=c['defense'], lo=A_LOW, hi=a_hi, share=1 - c['keep_def'], kind=c['kind'],
                               keep=dict(low=keep_low, high=keep_high, defense=keep_def),
                               data={'main': dict(tables=tables, curve=curve, low=lo_c, high=hi_c, shares=None)})
    allp = np.array(lo_c); print(key, 'every 10 XP 300-1500: worst %.3f, inside %.0f%%; 400+: inside %.0f%%' % (min(lo_c), 100 * np.mean(np.minimum(np.array(lo_c), 1 - np.array(hi_c)) >= 0.4), 100 * np.mean([min(l, 1 - h) >= 0.4 for X, l, h in zip(CURVE, lo_c, hi_c) if X >= 400])), flush=True)
json.dump(out, open(f'{OUT}/page_data3.json', 'w'))
