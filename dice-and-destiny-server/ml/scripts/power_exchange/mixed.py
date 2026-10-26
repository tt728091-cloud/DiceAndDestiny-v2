"""Characters that differ in board and defense: 4 default boards x 4 defenses, priced flat 40 (damage on each board's
own staircase average), each picking its damage/cards split against the other characters."""
import sys, json, itertools
import numpy as np
import field
from field import Board, DEFENSES, DEF_AVG, surv, HMAX, WINDOW, OUT
BOARDS_USED = ['Standard · Standard D6', 'Spellbook · Loom', 'Rainbow · Grove', 'Combo · Standard D6']
B = {l: Board(l) for l in BOARDS_USED}
DEFS = ['none', 'light', 'holdfast', 'heavy']
BASE_CARDS = 15

def score2(a, b, window=WINDOW):
    """Builds are (board label, target index, defense, cards)."""
    sa = surv(a[1], b[2], B[a[0]]); sb = surv(b[1], a[2], B[b[0]])
    n = max(len(sa), len(sb)); tot = cnt = 0
    for d in range(-window, window + 1):
        ha, hb = min(a[3] + d, HMAX), min(b[3] + d, HMAX)
        if ha < 1 or hb < 1: continue
        ka, kb = 1 - sa[:, hb], 1 - sb[:, ha]
        ka = np.concatenate([ka, np.full(n - len(ka), ka[-1])]); kb = np.concatenate([kb, np.full(n - len(kb), kb[-1])])
        pa, pb = np.diff(ka, prepend=0.0), np.diff(kb, prepend=0.0)
        tot += float(np.sum(pa * (1 - kb + 0.5 * pb))) + 0.5 * (1 - ka[-1]) * (1 - kb[-1]); cnt += 1
    return tot / cnt

def strategies(label, d, budget, c=40, step=0.25):
    bd = B[label]; a0 = float(bd.averages[bd.index(6.2)]); out = []
    for t in np.arange(6.2, 20 + 1e-9, step):
        ti = bd.index(t); left = budget - c * (bd.averages[ti] - a0) - c * DEF_AVG[d]
        if left < 0: break
        b = (label, ti, d, BASE_CARDS + int(left // 10))
        if b not in out: out.append(b)
    return out

def solve(budget, c=40, iters=5000, eta=2.0):
    chars = [(l, d) for l in BOARDS_USED for d in DEFS]
    S = {ch: strategies(ch[0], ch[1], budget, c) for ch in chars}
    chars = [ch for ch in chars if S[ch]]
    M = {}
    for p, q in itertools.combinations(chars, 2):
        m = np.array([[score2(a, b) for b in S[q]] for a in S[p]]); M[p, q], M[q, p] = m, 1 - m.T
    y = {p: np.full(len(S[p]), 1 / len(S[p])) for p in chars}; avg = {p: np.zeros(len(S[p])) for p in chars}
    for it in range(iters):
        u = {p: sum(M[p, q] @ y[q] for q in chars if q != p) / (len(chars) - 1) for p in chars}
        for p in chars:
            y[p] = y[p] * np.exp(eta * (u[p] - y[p] @ u[p])); y[p] /= y[p].sum()
        if it >= iters // 2:
            for p in chars: avg[p] += y[p]
    for p in chars: avg[p] /= avg[p].sum()
    vals = {p: float(avg[p] @ (sum(M[p, q] @ avg[q] for q in chars if q != p) / (len(chars) - 1))) for p in chars}
    pairs = {(p, q): float(avg[p] @ M[p, q] @ avg[q]) for p, q in itertools.combinations(chars, 2)}
    return vals, pairs

for budget in [150, 400, 900]:
    vals, pairs = solve(budget)
    lo = min(pairs, key=pairs.get); hi = max(pairs, key=pairs.get)
    print(f'+{budget} XP: characters {min(vals.values()):.3f}-{max(vals.values()):.3f}; matchups {pairs[lo]:.3f}-{pairs[hi]:.3f}; lopsided (<0.40 or >0.60): {sum(1 for v in pairs.values() if v < 0.4 or v > 0.6)} of {len(pairs)}', flush=True)
    for p in sorted(vals, key=vals.get):
        print(f'    {p[0]:24s} {p[1]:9s} {vals[p]:.3f}')
    print(f'    lowest matchup: {lo} {pairs[lo]:.3f}')
