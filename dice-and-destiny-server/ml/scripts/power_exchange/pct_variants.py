"""Percentage pricing (pct_solution) on variants of the 2 x 2: which defense and damage range reach an equal plane?
Predicted mismatch: half of |ln[(1 - D/O_low) / (1 - D/O_high)]| (0 for a percentage defense)."""
import itertools, json
import numpy as np
import field
from field import score, OUT
from eight import wld, at_avg
from pct_solution import shares, build, m

def run(lo, hi, dname, label, Xs=(400, 600, 900, 1500)):
    f, g = shares([lo, hi], ['none', dname])
    keys = [(lo, 'none'), (lo, dname), (hi, 'none'), (hi, dname)]
    D = 3.5 if dname == 'ward37' else field.DEF_AVG[dname]
    pred = 0 if dname == 'ward37' else 0.5 * abs(np.log((1 - D / lo) / (1 - D / hi)))
    cells = []
    for X in Xs:
        bs = {k: build(k[0], k[1], X, f, g) for k in keys}
        s = [score(bs[r], bs[c]) for r, c in itertools.permutations(keys, 2)]
        cells.append(f'{min(s):.2f}-{max(s):.2f}')
    print(f'{label:44s} predicted gap {100*(np.exp(pred)-1):4.1f}% | defense costs {100*(1-g[dname]):4.1f}% | scores at ' + '  '.join(f'{X}: {c}' for X, c in zip(Xs, cells)), flush=True)

print('every pairing of the four builds, smoothed over +-2 cards; equal plane = all near 0.50')
import sys
if len(sys.argv) > 1: run(7, 12, 'holdfast', '7 vs 12 dmg, Holdfast (3.5)')
if len(sys.argv) > 1: run(7, 12, 'light2', '7 vs 12 dmg, Double Light Guard (2)')
if len(sys.argv) > 1: run(7, 12, 'light', '7 vs 12 dmg, Light Guard (1)')
if len(sys.argv) > 1: run(9, 12, 'holdfast', '9 vs 12 dmg, Holdfast')
if len(sys.argv) > 1: run(10, 14, 'holdfast', '10 vs 14 dmg, Holdfast')
if len(sys.argv) > 1: run(12, 18, 'holdfast', '12 vs 18 dmg, Holdfast')
run(7, 12, 'ward37', '7 vs 12 dmg, Ward (prevents 37% of each hit)')
run(7, 18, 'ward37', '7 vs 18 dmg, Ward 37%')
