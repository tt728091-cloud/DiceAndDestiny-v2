import json
from chars import *
cost = level_price(40, 0, 40)
out = {}
for tag, chars_ in [('four', ['none', 'light', 'holdfast', 'heavy']), ('six', ['none', 'light', 'light2', 'holdfast', 'heavy', 'bulwark'])]:
    print(f'\n## {tag} characters, flat 40 XP a point (damage or prevention), cards 10 XP')
    rows = []
    for b in [100, 150, 200, 250, 300, 400, 500, 600, 750, 900, 1100]:
        r = solve(chars_, b, cost_fn=cost, step=0.1, iters=5000)
        rows.append(r)
        cells = []
        for p, x in r['chars'].items():
            board_xp = 40 * (x['damage'] - A0); def_xp = 40 * DEF_AVG[p]; card_xp = 10 * (x['cards'] - BASE_CARDS)
            cells.append(f"{p} {x['value']:.3f} {x['damage']:.1f}dmg {x['cards']:.0f}c")
        pv = list(r['pairs'].values())
        print(f'+{b:5d} | ' + ' | '.join(cells) + f' | matchups {min(pv):.2f}-{max(pv):.2f}', flush=True)
    out[tag] = rows
json.dump(out, open(f'{OUT}/final_tables.json', 'w'), indent=1)
