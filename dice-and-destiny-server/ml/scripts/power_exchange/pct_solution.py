"""Percentage pricing from the user's equation #1, fitted to a field of opponents.
Cards = total XP / 10 x f(O) x g(D), where (exact damage-through m from the dice)
  f(O) = geometric mean over the field's defenses D' of  m(6.2, D') / m(O, D')     (offense keeps this share)
  g(D) = geometric mean over the field's offenses O' of  m(O', D) / m(O', none)    (defense keeps this share)
So the board costs (1 - f) of your total XP and the defense costs (1 - g) of what is left; every remaining 10 XP is a card."""
import itertools, sys, json
import numpy as np
from field import BOARD, mean_through, DEF_AVG, score, OUT
from eight import wld, at_avg

def m(O, D):
    return mean_through(at_avg(O)[0], D)

def shares(offenses, defenses):
    f = {O: float(np.exp(np.mean([np.log(m(6.2, d) / m(O, d)) for d in defenses]))) for O in offenses}
    g = {D: float(np.exp(np.mean([np.log(m(o, D) / m(o, 'none')) for o in offenses]))) for D in defenses}
    return f, g

def build(O, D, X, f, g):
    return (at_avg(O)[0], D, max(1, int(X * f[O] * g[D] / 10)))

def rounds_to_defeat(a, b):
    """Rounds b needs to defeat a, the user's equation: a's cards / b's damage through a's defense."""
    return a[2] / mean_through(b[0], a[1])

if __name__ == '__main__':
    offenses, defenses = [7, 12], ['none', 'holdfast']
    f, g = shares(offenses, defenses)
    print('offense keeps of your XP: ' + ', '.join(f'{O} dmg {100*v:.1f}% (costs {100*(1-v):.1f}%)' for O, v in f.items()))
    print('defense keeps of what is left: ' + ', '.join(f'{D} {100*v:.1f}% (costs {100*(1-v):.1f}%)' for D, v in g.items()))
    names = {(7, 'none'): '1 Low O, no D', (7, 'holdfast'): '2 Low O, Holdfast', (12, 'none'): '3 High O, no D', (12, 'holdfast'): '4 High O, Holdfast'}
    keys = list(names)
    out = []
    for X in [400, 600, 900, 1500]:
        bs = {k: build(k[0], k[1], X, f, g) for k in keys}
        print(f'\n## {X} XP total')
        for k in keys:
            oxp = X * (1 - f[k[0]]); dxp = (X - oxp) * (1 - g[k[1]])
            print(f'  {names[k]:20s} offense {oxp:5.0f} XP + defense {dxp:5.0f} XP + {bs[k][2]:3d} cards ({10*bs[k][2]} XP) = {oxp+dxp+10*bs[k][2]:.0f}')
        print('  rounds to defeat (row is defeated by column) / score of row vs column, win/draw/loss %')
        for r in keys:
            cells = []
            for c in keys:
                if r == c: cells.append(' ' * 30); continue
                w, d, l = wld(bs[r], bs[c], window=0)
                cells.append(f'{rounds_to_defeat(bs[r], bs[c]):4.1f}r {100*w:3.0f}/{100*d:3.0f}/{100*l:3.0f} ({w+d/2:.2f})'.ljust(30))
            print(f'  {names[r]:20s} ' + ''.join(cells))
        sc = [wld(bs[r], bs[c], window=0) for r, c in itertools.permutations(keys, 2)]
        s = [w + d / 2 for w, d, l in sc]
        sm = [score(bs[r], bs[c]) for r, c in itertools.permutations(keys, 2)]
        print(f'  scores {min(s):.2f}-{max(s):.2f} exact cards; {min(sm):.2f}-{max(sm):.2f} smoothed over +-2 cards')
        out.append(dict(X=X, builds={names[k]: bs[k][2] for k in keys}, scores=s))
    json.dump(dict(f=f, g=g, rows=out), open(f'{OUT}/pct_solution.json', 'w'), indent=1)
