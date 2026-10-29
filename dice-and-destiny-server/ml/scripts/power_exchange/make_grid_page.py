"""Full permutation tables: markdown (docs) and compact JSON for the page."""
import json, os
OUT = os.path.join(os.path.dirname(os.path.abspath(__file__)), '../../runs/exchange')
g = json.load(open(os.path.join(OUT, 'full_grid.json')))
def lab(b): return f"{b['damage']:.1f}{' H' if b['defense'] == 'holdfast' else ''}"
md = ['# Every build against every build', '',
      'Research, 2026-10-24 (`full_grid.py`). Builds: average damage 6.2 to 18, with or without Holdfast, every leftover XP in 10-XP blank cards (no starting deck).',
      'Prices: board 40 XP per point above 6.2, Holdfast 140 XP. Race model, exact card counts. Each cell is the row build\'s score against the column build (wins + half the draws), in %. "H" = Holdfast. A build that costs more than the total XP is left out.', '']
compact = {'levels': g['levels'], 't': {}}
for X in g['levels']:
    T = g['tables'][str(X)]; B = T['builds']; M = T['matrix']
    idx = [i for i, b in enumerate(B) if b['ok']]
    md += [f'## {X} XP', '', '| Build | Board XP | Holdfast XP | Cards | Mean score |', '|---|---|---|---|---|']
    for i in idx:
        b = B[i]
        md.append(f"| {lab(b)} | {b['board']} | {140 if b['defense'] == 'holdfast' else 0} | {b['cards']} | {100 * T['means'][i]:.0f}% |")
    md += ['', '| Row vs column | ' + ' | '.join(lab(B[j]) for j in idx) + ' |', '|---|' + '---|' * len(idx)]
    for i in idx:
        md.append(f'| **{lab(B[i])}** ({B[i]["cards"]} cards) | ' + ' | '.join('—' if i == j else f"{round(100 * (M[i][j][0] + M[i][j][1] / 2))}" for j in idx) + ' |')
    md.append('')
    compact['t'][X] = {'b': [[b['damage'], 1 if b['defense'] == 'holdfast' else 0, b['cards'], b['board'], 1 if b['ok'] else 0] for b in B],
                       'm': [[None if c is None else [round(1000 * c[0]), round(1000 * c[1]), round(1000 * c[2]), c[3], c[4]] for c in row] for row in M],
                       'mean': T['means']}
open('/Users/daddymere/games/Dice-and-Destiny-v2/docs/game-design/power-pricing-full-grid.md', 'w').write('\n'.join(md) + '\n')
s = json.dumps(compact, separators=(',', ':'))
open(os.path.join(OUT, 'grid_compact.json'), 'w').write(s)
print(len(s), 'bytes compact')
