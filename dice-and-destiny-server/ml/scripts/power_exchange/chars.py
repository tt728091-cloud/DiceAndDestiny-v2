"""Character equilibrium: each defense is a character; at a total XP budget each picks its split between board damage
and blank cards (10 XP each) to do best against the other characters. Prices are fair when every character, playing
its best split, scores about 0.5 against the field, and the matchups between them stay out of the lopsided zone."""
import sys, json
import numpy as np
from field import *

BASE_CARDS, BASE_TARGET, CARD_XP = 15, 6.2, 10
A0 = float(BOARD.averages[BOARD.index(BASE_TARGET)])


def strategies(d, budget, c, kdef, step=0.25, tmax=20.0, defense_cost=None, cost_fn=None):
    dcost = defense_cost(d) if defense_cost else kdef * c * DEF_AVG[d]
    out = []
    for t in np.arange(BASE_TARGET, tmax + 1e-9, step):
        ti = BOARD.index(t)
        spend = cost_fn(float(BOARD.averages[ti]), d) if cost_fn else dcost + c * (BOARD.averages[ti] - A0)
        left = budget - spend
        if left < -1e-9:
            break
        h = BASE_CARDS + int(left // CARD_XP + 1e-9)
        if h > HMAX - WINDOW:
            continue
        b = (ti, d, h)
        if b not in out:
            out.append(b)
    return out


def solve(chars, budget, c=40, kdef=1.0, iters=6000, eta=2.0, defense_cost=None, step=0.25, cost_fn=None):
    S = {d: strategies(d, budget, c, kdef, step=step, defense_cost=defense_cost, cost_fn=cost_fn) for d in chars}
    chars = [d for d in chars if S[d]]
    M = {}
    for p in chars:
        for q in chars:
            if p < q:
                m = np.array([[score(a, b) for b in S[q]] for a in S[p]])
                M[p, q], M[q, p] = m, 1 - m.T
    y = {p: np.full(len(S[p]), 1.0 / len(S[p])) for p in chars}
    avg = {p: np.zeros(len(S[p])) for p in chars}
    for it in range(iters):
        u = {p: sum(M[p, q] @ y[q] for q in chars if q != p) / (len(chars) - 1) for p in chars}
        for p in chars:
            y[p] = y[p] * np.exp(eta * (u[p] - y[p] @ u[p]))
            y[p] /= y[p].sum()
        if it >= iters // 2:
            for p in chars:
                avg[p] += y[p]
    for p in chars:
        avg[p] /= avg[p].sum()
    res = {'budget': budget, 'c': c, 'k': kdef, 'chars': {}, 'pairs': {}}
    for p in chars:
        u = sum(M[p, q] @ avg[q] for q in chars if q != p) / (len(chars) - 1)
        v = float(avg[p] @ u)
        T = float(sum(avg[p][i] * BOARD.averages[b[0]] for i, b in enumerate(S[p])))
        H = float(sum(avg[p][i] * b[2] for i, b in enumerate(S[p])))
        top = int(np.argmax(avg[p]))
        res['chars'][p] = dict(value=v, exploit=float(u.max() - v), damage=T, cards=H,
                               top=(float(BOARD.averages[S[p][top][0]]), S[p][top][2], float(avg[p][top])))
    for p in chars:
        for q in chars:
            if p < q:
                res['pairs'][f'{p}-{q}'] = float(avg[p] @ M[p, q] @ avg[q])
    return res


def show(res):
    vals = '  '.join(f"{p}: {r['value']:.3f} ({r['damage']:.1f} dmg, {r['cards']:.0f}c)" for p, r in res['chars'].items())
    pv = list(res['pairs'].values())
    lo = min(res['pairs'], key=res['pairs'].get)
    ex = max(r['exploit'] for r in res['chars'].values())
    print(f"  +{res['budget']:5.0f} XP | {vals} | matchups {min(pv):.3f}-{max(pv):.3f} (lowest {lo}) | eq. error {ex:.3f}", flush=True)


def level_price(c0, slope, cmax, base_level=None):
    """Board XP for a level L = average damage + average prevention: P'(L) = min(cmax, c0 + slope (L - base))."""
    L0 = A0 if base_level is None else base_level
    def P(L):
        x = L - L0
        if slope <= 0:
            return c0 * x
        xk = (cmax - c0) / slope  # where the ramp reaches cmax
        if x <= xk:
            return c0 * x + slope * x * x / 2
        return c0 * xk + slope * xk * xk / 2 + cmax * (x - xk)
    def cost(avg, d):
        return P(avg + DEF_AVG[d]) - P(A0)
    cost.name = f'level price {c0:g}->{cmax:g} (+{slope:g}/pt)'
    return cost


if __name__ == '__main__':
    c = float(sys.argv[1]) if len(sys.argv) > 1 else 50
    k = float(sys.argv[2]) if len(sys.argv) > 2 else 1.0
    chars = ['none', 'light', 'holdfast', 'heavy']
    print(f'## c={c:g} XP a damage point, defenses {k:g} x c per prevention point')
    for b in [250, 400, 600, 900, 1300, 1800]:
        show(solve(chars, b, c, k))
