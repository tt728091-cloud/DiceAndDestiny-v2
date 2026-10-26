"""Pricing study: for a price scheme and an XP budget above the base character, every affordable build
(damage target, defense, blank cards at 10 XP) plays every other; the field's equilibrium shows which builds
are worth playing, whether every defense has a competitive build, and how lopsided the good builds are."""
import sys, json
import numpy as np
from field import *

BASE_CARDS = 15
BASE_TARGET = 6.2
CARD_XP = 10
DEFENSE_ORDER = ['none', 'light', 'light2', 'holdfast', 'heavy', 'bulwark']
A0 = float(BOARD.averages[BOARD.index(BASE_TARGET)])


def flat(c, k=1.0, base_def='none'):
    """c XP per point of average damage above the base; a defense costs k * c per point of average prevention
    above the base defense."""
    d0 = DEF_AVG[base_def]
    def cost(avg, d):
        return c * (avg - A0) + k * c * (DEF_AVG[d] - d0)
    cost.name = f'flat {c:g}/pt, defense {k:g}x'
    return cost


def builds_for(cost, budget, defenses=DEFENSE_ORDER, step=0.25, max_target=20.0, min_def=None):
    out = []
    for d in defenses:
        for t in np.arange(BASE_TARGET, max_target + 1e-9, step):
            ti = BOARD.index(t)
            avg = float(BOARD.averages[ti])
            left = budget - cost(avg, d)
            if left < -1e-9:
                continue
            h = BASE_CARDS + int(left // CARD_XP + 1e-9)
            if h > HMAX - WINDOW:
                continue
            b = (ti, d, h)
            if b not in out:
                out.append(b)
    return out


def describe(b):
    return f'{BOARD.averages[b[0]]:5.2f} {b[1]:8s} {b[2]:3d}c'


def analyse(cost, budget, defenses=DEFENSE_ORDER, step=0.25, verbose=True):
    bs = builds_for(cost, budget, defenses, step)
    m = matrix(bs)
    x = equilibrium(m)
    v = m @ x
    res = {'budget': budget, 'n': len(bs), 'best_by_defense': {}, 'support': []}
    for d in defenses:
        idx = [i for i, b in enumerate(bs) if b[1] == d]
        if idx:
            i = max(idx, key=lambda i: v[i])
            res['best_by_defense'][d] = (float(v[i]), describe(bs[i]))
    order = np.argsort(-x)
    for i in order:
        if x[i] < 0.02:
            break
        res['support'].append((float(x[i]), describe(bs[i]), float(v[i])))
    good = [i for i in range(len(bs)) if v[i] >= 0.45]
    sub = m[np.ix_(good, good)]
    res['good'] = len(good)
    res['good_min'] = float(sub.min())
    wi = np.unravel_index(np.argmin(sub), sub.shape)
    res['good_worst'] = (describe(bs[good[wi[0]]]), describe(bs[good[wi[1]]]))
    res['good_defenses'] = sorted({bs[i][1] for i in good}, key=DEFENSE_ORDER.index)
    res['good_damage'] = (float(min(BOARD.averages[bs[i][0]] for i in good)), float(max(BOARD.averages[bs[i][0]] for i in good)))
    best = [max([i for i, b in enumerate(bs) if b[1] == d], key=lambda i: v[i]) for d in defenses if any(b[1] == d for b in bs)]
    bm = m[np.ix_(best, best)]
    res['best_min'] = float(bm.min())
    bi = np.unravel_index(np.argmin(bm), bm.shape)
    res['best_worst'] = (describe(bs[best[bi[0]]]), describe(bs[best[bi[1]]]))
    sup = [i for i in range(len(bs)) if x[i] >= 0.02]
    res['support_min'] = float(m[np.ix_(sup, sup)].min())
    res['matrix_best'] = bm.tolist()
    res['best_builds'] = [describe(bs[i]) for i in best]
    if verbose:
        bd = '  '.join(f'{d}:{s:.3f}' for d, (s, _) in res['best_by_defense'].items())
        print(f'  +{budget:5.0f} XP  builds {len(bs):3d} | best of each defense vs the equilibrium: {bd}')
        print(f'           equilibrium: ' + '; '.join(f'{w:.0%} {d}' for w, d, _ in res['support'][:6]))
        print(f'           builds within 0.45: {len(good)} (defenses {",".join(res["good_defenses"])}, damage {res["good_damage"][0]:.1f}-{res["good_damage"][1]:.1f}); worst pairing among them {res["good_min"]:.3f} ({res["good_worst"][0]} vs {res["good_worst"][1]})')
        print(f'           best-of-each-defense worst pairing {res["best_min"]:.3f} ({res["best_worst"][0]} vs {res["best_worst"][1]}); within the equilibrium {res["support_min"]:.3f}')
    return res


if __name__ == '__main__':
    budgets = [0, 100, 200, 400, 700, 1000, 1500]
    for c in [float(a) for a in sys.argv[1:]] or [50]:
        cost = flat(c)
        print(f'\n## {cost.name}')
        for bud in budgets:
            analyse(cost, bud)
