"""Does a defense's spread matter, or only its average? Three 3.5 defenses: steady (3 or 4), Holdfast (0-6),
swingy (0 or 7); and two 2.0 ones. Priced by face average (flat 40), they play as characters with no defense
and Light Guard. Then the same priced by effective prevention (what it really stops against the field's hits)."""
import field
from field import DEFENSES, DEF_AVG
import numpy as np
DEFENSES['steady35'] = np.array([0, 0, 0, .5, .5]); DEFENSES['swing35'] = np.array([.5, 0, 0, 0, 0, 0, 0, .5])
DEFENSES['steady2'] = np.array([0, 0, 1.0]); DEFENSES['swing2'] = np.array([.5, 0, 0, 0, .5])
for k in ['steady35', 'swing35', 'steady2', 'swing2']:
    DEF_AVG[k] = float(np.dot(np.arange(len(DEFENSES[k])), DEFENSES[k]))
from chars import *
from level_scan import line
budgets = [150, 250, 400, 600, 900]
flat = level_price(40, 0, 40)
for group in [['none', 'light', 'holdfast', 'steady35', 'swing35'], ['none', 'light', 'light2', 'steady2', 'swing2']]:
    line('face price: ' + ','.join(group), [solve(group, b, cost_fn=flat) for b in budgets])
    for b in [250, 600]:
        r = solve(group, b, cost_fn=flat)
        print('    +%d: ' % b + '  '.join(f"{p} {x['value']:.3f}" for p, x in r['chars'].items()))
# effective prevention against a no-defense field hit at several levels
print('\neffective prevention E[min(hit, roll)] against the Standard board:')
for T in [6.2, 9, 12, 16]:
    o = BOARD.odds[BOARD.index(T)]
    row = []
    for d in ['light', 'light2', 'steady2', 'swing2', 'holdfast', 'steady35', 'swing35', 'heavy', 'bulwark']:
        eff = BOARD.averages[BOARD.index(T)] - mean_through(BOARD.index(T), d)
        row.append(f'{d} {eff:.2f}')
    print(f'  {T:5}: ' + '  '.join(row))
