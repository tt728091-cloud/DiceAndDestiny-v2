"""The 2 x 2 with a percentage defense (Ward: prevents 37% of each hit) and proportional prices:
cards = power budget / (10 x damage multiplier x ward multiplier). Every build has the same 'power XP'."""
import itertools
from eight import *
from eight import wld
keys = [(7, 'none'), (12, 'none'), (7, 'ward37'), (12, 'ward37')]
names = {(7, 'none'): 'Low dmg, no defense', (12, 'none'): 'High dmg, no defense', (7, 'ward37'): 'Low dmg, Ward 37%', (12, 'ward37'): 'High dmg, Ward 37%'}
i7, a7 = at_avg(7); i12, a12 = at_avg(12)
for low_cards in [20, 30, 45, 70]:
    # low damage, no defense gets low_cards; everyone else the cards of equal power
    power = low_cards * a7
    bs = {}
    for k in keys:
        i, a = (i7, a7) if k[0] == 7 else (i12, a12)
        w = 1 / (1 - 0.37) if k[1] == 'ward37' else 1.0
        bs[k] = (i, k[1], int(round(power / (a * w))))
    s = {(r, c): score(bs[r], bs[c]) for r, c in itertools.permutations(keys, 2)}
    lo = min(s.values()); hi = max(s.values())
    print(f'cards ' + ', '.join(f'{names[k]} {bs[k][2]}' for k in keys) + f' | matchups {lo:.2f}-{hi:.2f}')
