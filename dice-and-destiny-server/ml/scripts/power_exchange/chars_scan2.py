import json, chars as C
from chars import *
budgets = [150, 250, 400, 600, 900, 1100]
six = ['none', 'light', 'light2', 'holdfast', 'heavy', 'bulwark']
def line(tag, rows):
    cells = []
    for r in rows:
        vs = [x['value'] for x in r['chars'].values()]; pv = list(r['pairs'].values())
        lo = min(r['chars'], key=lambda p: r['chars'][p]['value']); hi = max(r['chars'], key=lambda p: r['chars'][p]['value'])
        cells.append(f"{min(vs):.2f}({lo[:5]})-{max(vs):.2f}({hi[:5]})/{min(pv):.2f}")
    print(f'{tag} | ' + '  '.join(cells), flush=True)
print('budgets', budgets, ': weakest-strongest character value / lowest matchup')
for c in [35, 40, 45]:
    line(f'six defenses c={c}', [solve(six, b, c, 1.0) for b in budgets])
for base in [20, 25]:
    C.BASE_CARDS = base
    line(f'four, base {base} cards, c=40', [solve(['none','light','holdfast','heavy'], b, 40, 1.0) for b in budgets])
    line(f'four, base {base} cards, c=50', [solve(['none','light','holdfast','heavy'], b, 50, 1.0) for b in budgets])
C.BASE_CARDS = 15
