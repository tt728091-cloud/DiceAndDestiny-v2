"""The user's 2 x 2 builds: damage 7 or 12, no defense or Holdfast, every leftover XP in blank cards (10 XP each),
priced flat: 40 XP a damage point above 6.2, 40 XP per point of average prevention. Every build plays every other."""
import sys, json
import numpy as np
from field import *

CARD, RATE, BASE_CARDS, BASE_T = 10, 40, 15, 6.2
A0 = float(BOARD.averages[BOARD.index(BASE_T)])

def at_avg(x):
    i = int(np.argmin(np.abs(BOARD.averages - x)))
    return i, float(BOARD.averages[i])

def build(dmg, d, budget):
    i, a = at_avg(dmg)
    left = budget - RATE * (a - A0) - RATE * DEF_AVG[d]
    return (i, d, BASE_CARDS + int(left // CARD)), a, left

def wld(a, b, window=0):
    """a's win, draw and loss chances against b (exact health by default)."""
    sa, sb = surv(a[0], b[1]), surv(b[0], a[1])
    n = max(len(sa), len(sb)); W = D = L = 0; cnt = 0
    for s in range(-window, window + 1):
        ha, hb = min(a[2] + s, HMAX), min(b[2] + s, HMAX)
        ka, kb = 1 - sa[:, hb], 1 - sb[:, ha]
        ka = np.concatenate([ka, np.full(n - len(ka), ka[-1])]); kb = np.concatenate([kb, np.full(n - len(kb), kb[-1])])
        pa, pb = np.diff(ka, prepend=0.0), np.diff(kb, prepend=0.0)
        w = float(np.sum(pa * (1 - kb))); dr = float(np.sum(pa * pb)) + (1 - ka[-1]) * (1 - kb[-1])
        W += w; D += dr; L += 1 - w - dr; cnt += 1
    return W / cnt, D / cnt, L / cnt

NAMES = {(7, 'none'): 'Low dmg, no defense', (12, 'none'): 'High dmg, no defense',
         (7, 'holdfast'): 'Low dmg, Holdfast', (12, 'holdfast'): 'High dmg, Holdfast'}

def table(budget, defense='holdfast', window=0, out=None):
    keys = [(7, 'none'), (12, 'none'), (7, defense), (12, defense)]
    bs = {k: build(k[0], k[1], budget) for k in keys}
    print(f'\n## {budget} XP above the base (15 cards, 6.2, no defense)')
    for k in keys:
        (i, d, h), a, left = bs[k]
        spent_dmg = RATE * (a - A0); spent_def = RATE * DEF_AVG[d]
        print(f'  {NAMES.get(k, str(k)):22s} {a:5.2f} dmg, {d:9s} {h:3d} cards  (damage {spent_dmg:4.0f} XP, defense {spent_def:4.0f} XP, cards {CARD*(h-BASE_CARDS):4.0f} XP)')
    print('  row vs column: win / draw / loss  (score = win + half the draws)')
    header = ''.join(f'{NAMES.get(k, str(k)):>24s}' for k in keys)
    print(f'  {"":22s}{header}')
    res = {}
    for r in keys:
        cells = []
        for c in keys:
            if r == c:
                cells.append(f'{"—":>24s}'); continue
            w, dr, l = wld(bs[r][0], bs[c][0], window)
            res[(r, c)] = (w, dr, l)
            cells.append(f'{100*w:5.0f}/{100*dr:3.0f}/{100*l:3.0f} ({w+dr/2:.2f})'.rjust(24))
        print(f'  {NAMES.get(r, str(r)):22s}' + ''.join(cells))
    avg = {r: np.mean([res[(r, c)][0] + res[(r, c)][1] / 2 for c in keys if c != r]) for r in keys}
    print('  average score against the other three: ' + '  '.join(f'{NAMES.get(r, str(r))} {v:.2f}' for r, v in avg.items()))
    if out is not None:
        out.append(dict(budget=budget, builds={NAMES.get(k, str(k)): dict(damage=bs[k][1], defense=k[1], cards=bs[k][0][2]) for k in keys},
                        wld={f'{NAMES.get(r, str(r))} vs {NAMES.get(c, str(c))}': v for (r, c), v in res.items()}))
    return bs, res

if __name__ == '__main__':
    out = []
    for b in [int(x) for x in sys.argv[1:]] or [400, 600, 900]:
        table(b, out=out)
    json.dump(out, open(f'{OUT}/eight.json', 'w'), indent=1)
