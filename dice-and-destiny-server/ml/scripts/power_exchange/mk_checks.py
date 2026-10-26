"""Real-battle checks of the character equilibria: each character's most-played build at a few budgets, every pair
played by board-lab -exchange-check. Writes the plan with the race model's exact-health prediction."""
import json, itertools, sys
from chars import *
plan = []
def add(tag, a, b):
    plan.append(dict(name=f'{tag} {a[1]} vs {b[1]}', template='standard', die='standard_d6',
                     hero_target=float(BOARD.targets[a[0]]), hero_health=a[2], hero_defense=a[1],
                     rival_target=float(BOARD.targets[b[0]]), rival_health=b[2], rival_defense=b[1],
                     expect=score(a, b, window=0), expect_window=score(a, b)))
cost = level_price(40, 0, 40)
for budget, chars in [(150, ['none', 'light', 'holdfast']), (400, ['none', 'light', 'holdfast', 'heavy']),
                      (900, ['none', 'light', 'holdfast', 'heavy']), (600, ['light2', 'bulwark', 'none', 'holdfast'])]:
    S = {d: strategies(d, budget, 40, 1.0, cost_fn=cost) for d in chars}
    r = solve(chars, budget, cost_fn=cost)
    top = {}
    for d in chars:
        a, h, w = r['chars'][d]['top']
        top[d] = next(b for b in S[d] if abs(BOARD.averages[b[0]] - a) < 1e-9 and b[2] == h)
    pairs = itertools.combinations(chars, 2) if budget != 600 else [('light2', 'holdfast'), ('bulwark', 'none'), ('bulwark', 'light2'), ('bulwark', 'holdfast')]
    for p, q in pairs:
        add(f'+{budget} XP flat 40:', top[p], top[q])
json.dump(plan, open(f'{OUT}/price-checks-plan.json', 'w'), indent=1)
for p in plan:
    print(f"{p['name']:45s} hero {p['hero_target']:.2f}/{p['hero_health']}c rival {p['rival_target']:.2f}/{p['rival_health']}c  model {p['expect']:.3f}")
