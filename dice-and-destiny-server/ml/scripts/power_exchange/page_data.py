"""Exact tables for the pricing page: 4 builds (low/high damage x no defense/defense, the rest in blank cards) at
total XP levels, under each pricing scheme, every pairing by the race model at exact card counts (win/draw/loss),
plus each build's mean score against the other three at every XP from 300 to 1500. No base deck: cards = what is
left of the total XP / 10, as on the user's worksheet."""
import json, itertools
import numpy as np
import field
from field import mean_through, OUT
from eight import wld, at_avg

PAIRS = [(7, 12), (9, 15), (11, 18)]
LEVELS = [400, 650, 900, 1200, 1500]
CURVE = list(range(300, 1501, 10))

def m(O, D):
    return mean_through(at_avg(O)[0], D)

def fitted_shares(lo, hi, dname):
    offs, defs = [lo, hi], ['none', dname]
    f = {O: float(np.exp(np.mean([np.log(m(6.2, d) / m(O, d)) for d in defs]))) for O in offs}
    g = {D: float(np.exp(np.mean([np.log(m(o, D) / m(o, 'none')) for o in offs]))) for D in defs}
    return f, g

def scheme_flat(lo, hi):
    a0 = at_avg(6.2)[1]
    def cards(O, D, X):
        a = at_avg(O)[1]
        oxp, dxp = 40 * (a - a0), 40 * field.DEF_AVG.get(D, 0)
        return int((X - oxp - dxp) // 10), oxp, dxp
    return cards

def scheme_pct(lo, hi, dname):
    f, g = fitted_shares(lo, hi, dname)
    def cards(O, D, X):
        oxp = X * (1 - f[O]); dxp = (X - oxp) * (1 - g[D])
        return int((X - oxp - dxp) // 10), oxp, dxp
    return cards, f, g

def scheme_ward(lo, hi, p=0.37):
    def cards(O, D, X):
        a = at_avg(O)[1]
        oxp = X * (1 - 6.2 / a); dxp = (X - oxp) * (p if D != 'none' else 0)
        return int((X - oxp - dxp) // 10), oxp, dxp
    return cards

SCHEMES = {
    'flat': dict(label='Flat 40 XP a point, Holdfast', defense='holdfast'),
    'pct': dict(label='Percentage, Holdfast', defense='holdfast'),
    'pctlight': dict(label='Percentage, Light Guard', defense='light'),
    'ward': dict(label='Percentage, 37% defense', defense='ward37'),
}

def cards_fn(key, lo, hi):
    d = SCHEMES[key]['defense']
    if key == 'flat': return scheme_flat(lo, hi), None
    if key in ('pct', 'pctlight'):
        fn, f, g = scheme_pct(lo, hi, d); return fn, dict(f=f, g=g)
    return scheme_ward(lo, hi), None

out = {'levels': LEVELS, 'curve': CURVE, 'pairs': [f'{a}-{b}' for a, b in PAIRS], 'schemes': {}, 'notes': {}}
for key, meta in SCHEMES.items():
    out['schemes'][key] = {'label': meta['label'], 'defense': meta['defense'], 'data': {}}
    for lo, hi in PAIRS:
        fn, shares = cards_fn(key, lo, hi)
        d = meta['defense']
        keys = [(lo, 'none'), (lo, d), (hi, 'none'), (hi, d)]
        def builds_at(X):
            res = []
            for O, D in keys:
                c, oxp, dxp = fn(O, D, X)
                res.append(dict(damage=at_avg(O)[1], defense=D, cards=c, oxp=oxp, dxp=dxp, ti=at_avg(O)[0]))
            return res
        tables = {}
        for X in LEVELS:
            bs = builds_at(X)
            mat = [[None] * 4 for _ in range(4)]
            for i, j in itertools.permutations(range(4), 2):
                a, b = bs[i], bs[j]
                if a['cards'] < 1 or b['cards'] < 1: continue
                w, dr, l = wld((a['ti'], a['defense'], a['cards']), (b['ti'], b['defense'], b['cards']))
                ra = a['cards'] / mean_through(b['ti'], a['defense'])  # rounds b needs to defeat a
                mat[i][j] = dict(w=round(w, 4), d=round(dr, 4), l=round(l, 4), rounds=round(ra, 2))
            tables[X] = dict(builds=[{k: (round(v, 2) if isinstance(v, float) else v) for k, v in b.items() if k != 'ti'} for b in bs], matrix=mat)
        curve = [[] for _ in range(4)]
        lo_c, hi_c = [], []
        for X in CURVE:
            bs = builds_at(X)
            ps = []
            for i in range(4):
                if bs[i]['cards'] < 1: curve[i].append(None); continue
                vals = []
                for j in range(4):
                    if j == i or bs[j]['cards'] < 1: continue
                    w, dr, l = wld((bs[i]['ti'], bs[i]['defense'], bs[i]['cards']), (bs[j]['ti'], bs[j]['defense'], bs[j]['cards']))
                    vals.append(w + dr / 2)
                ps += vals
                curve[i].append(round(float(np.mean(vals)), 4) if vals else None)
            lo_c.append(round(min(ps), 4) if ps else None); hi_c.append(round(max(ps), 4) if ps else None)
        out['schemes'][key]['data'][f'{lo}-{hi}'] = dict(tables=tables, curve=curve, low=lo_c, high=hi_c, shares=shares)
        print(key, lo, hi, 'done', flush=True)
json.dump(out, open(f'{OUT}/page_data.json', 'w'))
print('written', f'{OUT}/page_data.json')
