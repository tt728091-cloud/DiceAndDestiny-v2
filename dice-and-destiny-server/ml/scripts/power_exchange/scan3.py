import field
from chars import *
from level_scan import line, budgets, four
print('budgets', budgets)
f = level_price(25, 4, 10000); f.name = 'percent of level: 4 x level XP a point'
line(f.name, [solve(four, b, cost_fn=f) for b in budgets])
for c in [30, 50]:
    line(f'flat {c}', [solve(four, b, cost_fn=level_price(c, 0, c)) for b in budgets])
for label in ['Spellbook · Loom', 'Rainbow · Grove', 'Combo · Standard D6']:
    field.use_board(label)
    line(f'flat 40 on {label}', [solve(four, b, cost_fn=level_price(40, 0, 40)) for b in budgets])
