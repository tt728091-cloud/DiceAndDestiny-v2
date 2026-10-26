"""Search for the balancing point: high damage level, defense size and percentage prices so that every pairing of the
four builds (low 7 / high damage x no defense / defense, the rest in 10-XP blank cards) lands inside 40-60 at every
XP level. Exact card counts (floor), race model, win + half the draws.

cards = floor(XP / 10 x share), share = f(damage) x g(defense): the board keeps f of your XP for cards, the defense
keeps g of what is left. f(low 7) is fixed at 6.2 / 6.91 (so the base 6.2 is free); the search sets f(high) and g."""
import sys, json, itertools
import numpy as np
import field
from field import BOARD, surv, mean_through, OUT
from eight import wld, at_avg

def face_die(values):
    """One die whose six faces prevent these amounts."""
    v = np.zeros(max(values) + 1)
    for x in values:
        v[x] += 1 / 6
    return v

def steady(x):
    v = np.zeros(int(x) + 1); v[int(x)] = 1.0; return v

# Defense candidates (average prevention per roll). light/light2/holdfast exist in field already.
EXTRA = {
    'half': face_die([0, 0, 0, 1, 1, 1]),           # 0.5
    'light15': face_die([0, 1, 1, 2, 2, 3]),        # 1.5
    'light25': face_die([1, 2, 2, 3, 3, 4]),        # 2.5
    'light3': np.convolve(np.convolve([1/3, 1/3, 1/3], [1/3, 1/3, 1/3]), [1/3, 1/3, 1/3]),  # 3.0
    'steady1': steady(1), 'steady2': steady(2), 'steady3': steady(3),
}
for k, v in EXTRA.items():
    field.DEFENSES[k] = v
    field.DEF_AVG[k] = float(np.dot(np.arange(len(v)), v))

LOW = 7
F_LOW = 6.2 / at_avg(LOW)[1]

def cards(X, share):
    return max(1, int(X * share / 10 + 1e-9))

def builds(X, high, dname, r, g):
    il, ih = at_avg(LOW)[0], at_avg(high)[0]
    fl, fh = F_LOW, F_LOW * r
    return [(il, 'none', cards(X, fl)), (il, dname, cards(X, fl * g)), (ih, 'none', cards(X, fh)), (ih, dname, cards(X, fh * g))]

def evaluate(high, dname, r, g, levels):
    worst = 0.0; scores = []
    for X in levels:
        bs = builds(X, high, dname, r, g)
        for i, j in itertools.combinations(range(4), 2):
            w, d, l = wld(bs[i], bs[j])
            s = w + d / 2
            scores.append(s)
            worst = max(worst, abs(s - 0.5))
            if worst > 0.25:  # hopeless, stop early
                return worst, scores
    return worst, scores

def search(high, dname, levels, coarse=0.02, fine=0.004):
    best = (9, None, None)
    for r in np.arange(0.30, 1.0001, coarse):
        for g in np.arange(0.40, 1.0001, coarse):
            w, _ = evaluate(high, dname, r, g, levels)
            if w < best[0]: best = (w, r, g)
    _, r0, g0 = best
    for r in np.arange(r0 - coarse, r0 + coarse + 1e-9, fine):
        for g in np.arange(g0 - coarse, g0 + coarse + 1e-9, fine):
            if g > 1 or r > 1: continue
            w, _ = evaluate(high, dname, r, g, levels)
            if w < best[0]: best = (w, r, g)
    return best

if __name__ == '__main__':
    levels = [400, 650, 900]
    highs = [float(a) for a in sys.argv[1].split(',')] if len(sys.argv) > 1 else [9, 10, 11, 12, 13, 14]
    defs = sys.argv[2].split(',') if len(sys.argv) > 2 else ['half', 'light', 'light15', 'light2', 'light25', 'light3', 'holdfast', 'steady1', 'steady2', 'steady3']
    rows = []
    for high in highs:
        for dname in defs:
            w, r, g = search(high, dname, levels)
            rows.append(dict(high=high, high_avg=at_avg(high)[1], defense=dname, prevent=field.DEF_AVG[dname], worst=w, r=r, g=g))
            print(f'high {at_avg(high)[1]:5.2f} | {dname:9s} ({field.DEF_AVG[dname]:.1f}) | worst pairing {50-100*w:4.1f}-{50+100*w:4.1f} | high keeps {r:.3f} of low\'s share, defense keeps {g:.3f}', flush=True)
    json.dump(rows, open(f'{OUT}/balance_search.json', 'w'), indent=1)
