"""Closed form: a defense preventing a share p of every hit, offense priced as a share of total XP.
cards = X/10 x (6.2 / O) x (1 - p). Then H x O / (1 - p) = 10 X / 6.2 for everyone, so the user's equation #1
(rounds to defeat each other equal) holds for every pair at every X."""
import itertools
from field import mean_through, BOARD
from eight import wld, at_avg
P = 0.37
names = {(7, 0): '1 Low O, no D', (7, P): '2 Low O, Ward 37%', (12, 0): '3 High O, no D', (12, P): '4 High O, Ward 37%'}
for X in [400, 600]:
    print(f'\n## {X} XP')
    bs = {}
    for (O, p), n in names.items():
        a = at_avg(O)[1]
        oxp = X * (1 - 6.2 / a); dxp = (X - oxp) * p; cards = int((X - oxp - dxp) // 10)
        bs[(O, p)] = (at_avg(O)[0], 'ward37' if p else 'none', cards)
        print(f'  {n:20s} offense {a:5.2f}: {oxp:4.0f} XP ({100*(1-6.2/a):.1f}%) + defense {dxp:4.0f} XP + {cards} cards = {oxp+dxp+10*cards:.0f} XP')
    for r in names:
        row = []
        for c in names:
            if r == c: row.append(' ' * 27); continue
            w, d, l = wld(bs[r], bs[c])
            rounds = bs[r][2] / mean_through(bs[c][0], bs[r][1])
            row.append(f'{rounds:4.1f}r {100*w:3.0f}/{100*d:3.0f}/{100*l:3.0f} {w+d/2:.2f}'.ljust(27))
        print(f'  {names[r]:20s} ' + ''.join(row))
