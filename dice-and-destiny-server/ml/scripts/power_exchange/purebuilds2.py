import itertools, sys
from chars import *
import chars as C
four = ['none', 'light', 'holdfast', 'heavy']
def run(tag, cost, budgets=(150, 250, 400, 600, 900), base=15, show_worst=False):
    C.BASE_CARDS = base
    cells = []
    for budget in budgets:
        S = {d: strategies(d, budget, 40, 1.0, cost_fn=cost) for d in four}
        S = {d: s for d, s in S.items() if s}; ch = list(S)
        good = {}
        for d in ch:
            vals = [np.mean([np.mean([score(b, o) for o in S[q]]) for q in ch if q != d]) for b in S[d]]
            good[d] = [S[d][i] for i in np.argsort(vals)[-5:]]
        ex = [(score(a, b, window=0), a, b) for p, q in itertools.combinations(ch, 2) for a in good[p] for b in good[q]]
        s = np.array([e[0] for e in ex])
        cells.append(f'{s.min():.2f}-{s.max():.2f} ({np.mean((s < 0.4) | (s > 0.6)):.0%})')
        if show_worst:
            w = min(ex, key=lambda e: min(e[0], 1 - e[0]))
            f = lambda b: f'{b[1]} {BOARD.averages[b[0]]:.1f}/{b[2]}c'
            print(f'   +{budget} worst: {f(w[1])} vs {f(w[2])} = {w[0]:.2f}')
    C.BASE_CARDS = 15
    print(f'{tag:34s} ' + '  '.join(cells), flush=True)
print('good-build pairings, exact health: range (share lopsided) at +150 +250 +400 +600 +900 XP')
run('flat 40', level_price(40, 0, 40), show_worst=True)
run('level price 25->40 (+5/pt)', level_price(25, 5, 40))
run('flat 30', level_price(30, 0, 30))
run('flat 40, base 25 cards', level_price(40, 0, 40), base=25)
