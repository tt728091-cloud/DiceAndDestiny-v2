"""Ignore prices: choose each of the four builds' card counts to make the matchups as even as possible.
If even the best choice has big swings, no price can remove them."""
import itertools
import numpy as np
from eight import *
keys = [(7, 'none'), (12, 'none'), (7, 'holdfast'), (12, 'holdfast')]
idx = {k: at_avg(k[0])[0] for k in keys}
def sc(h):
    bs = {k: (idx[k], k[1], h[k]) for k in keys}
    return {(r, c): score(bs[r], bs[c]) for r, c in itertools.combinations(keys, 2)}
best = None
for h1 in [20, 30, 40, 60]:
    # search the other three healths on a grid around the product-rule guesses
    for h2 in range(max(5, int(h1 * 0.4)), int(h1 * 0.9) + 1):
        for h3 in range(max(5, int(h1 * 0.35)), int(h1 * 0.9) + 1, 1):
            for h4 in range(max(5, int(h1 * 0.2)), int(h1 * 0.6) + 1, 1):
                h = dict(zip(keys, [h1, h2, h3, h4]))
                s = sc(h)
                dev = max(abs(v - 0.5) for v in s.values())
                if best is None or dev < best[0]:
                    best = (dev, h, s)
    dev, h, s = best
    print(f'low dmg no defense at {h1} cards: best possible worst matchup {0.5 - dev:.2f}-{0.5 + dev:.2f} with cards ' + ', '.join(f'{NAMES[k]} {h[k]}' for k in keys), flush=True)
    for (r, c), v in s.items():
        print(f'      {NAMES[r]:22s} vs {NAMES[c]:22s} {v:.2f}')
    best = None
