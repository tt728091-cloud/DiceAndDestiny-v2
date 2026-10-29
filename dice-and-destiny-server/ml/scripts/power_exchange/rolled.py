"""Rolled defenses only (flat amounts on die faces, like Holdfast). Two kinds:
- block: some faces prevent the whole attack (amount 99), the rest nothing;
- Holdfast-shaped: small amounts on every face.
Prices are percentages of total XP: cards = XP/10 x (6.2/damage) x (1 - defense cost share), rounded to nearest."""
import itertools, json, sys
import numpy as np
import field
from field import OUT
from eight import wld, at_avg
import balance_search  # registers half/light15/... defenses

def block(n_dice, faces):
    """n dice; each die blocks everything on `faces` of its 6 faces."""
    p_none = (1 - faces / 6) ** n_dice
    v = np.zeros(100); v[0] = p_none; v[99] = 1 - p_none
    return v
ROLLED = {
    'block1': block(1, 1),   # one die, a 6 blocks the attack: 16.7%
    'block2': block(2, 1),   # two dice, any 6: 30.6%
    'block1b': block(1, 2),  # one die, 5-6: 33.3%
    'block3': block(3, 1),   # three dice, any 6: 42.1%
}
for k, v in ROLLED.items():
    field.DEFENSES[k] = v; field.DEF_AVG[k] = float(v[99])  # 'average' here = chance to block
LOW = 7
def cards(X, s): return max(1, int(np.floor(X * s / 10 + 0.5)))
def builds(X, high, d, keep_def):
    il, ih = at_avg(LOW)[0], at_avg(high)[0]
    kl, kh = 6.2 / at_avg(LOW)[1], 6.2 / at_avg(high)[1]
    return [(il, 'none', cards(X, kl)), (il, d, cards(X, kl * keep_def)), (ih, 'none', cards(X, kh)), (ih, d, cards(X, kh * keep_def))]
def worst_at(X, high, d, keep_def):
    bs = builds(X, high, d, keep_def); w = 0
    for i, j in itertools.combinations(range(4), 2):
        a, dr, l = wld(bs[i], bs[j]); w = max(w, abs(a + dr / 2 - 0.5))
    return w
XS = list(range(400, 1501, 10))
def profile(high, d, keep_def, xs=XS):
    return np.array([worst_at(X, high, d, keep_def) for X in xs])
if __name__ == '__main__':
    rows = []
    for d in ['block1', 'block2', 'block1b', 'block3']:
        share = field.DEF_AVG[d]
        for high in [9, 10, 11, 12, 13, 14]:
            best = None
            for kd in np.arange(1 - share - 0.06, 1 - share + 0.061, 0.01):
                p = profile(high, d, kd, XS[::2])
                key = (np.mean(p <= 0.1), -p.max())
                if best is None or key > best[0]: best = (key, kd)
            kd = best[1]; p = profile(high, d, kd)
            at3 = max(worst_at(X, high, d, kd) for X in [400, 650, 900])
            rows.append(dict(defense=d, share=share, high=at_avg(high)[1], keep_def=kd, inside=float(np.mean(p <= 0.1)), worst=float(p.max()), at3=at3))
            print(f'{d:8s} blocks {100*share:4.1f}% | high {at_avg(high)[1]:5.2f} | defense costs {100*(1-kd):4.1f}% of the rest | inside 40-60 at {100*np.mean(p<=0.1):3.0f}% of XP 400-1500, worst {50-100*p.max():.0f}-{50+100*p.max():.0f} | at 400/650/900 {50-100*at3:.0f}-{50+100*at3:.0f}', flush=True)
    json.dump(rows, open(f'{OUT}/rolled.json', 'w'), indent=1)
