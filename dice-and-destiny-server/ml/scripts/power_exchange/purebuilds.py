"""Pure builds, exact health (no smoothing): for each character, its good builds (within 0.01 of its equilibrium
value); how lopsided are the best and worst pairings between good builds of different characters?"""
import itertools
from chars import *
import chars as C
cost = level_price(40, 0, 40)
four = ['none', 'light', 'holdfast', 'heavy']
for budget in [150, 250, 400, 600, 900]:
    S = {d: strategies(d, budget, 40, 1.0, cost_fn=cost) for d in four}
    S = {d: s for d, s in S.items() if s}
    ch = list(S)
    r = solve(ch, budget, cost_fn=cost)
    # each build's value against the others' equilibrium mixtures, smoothed (as in the equilibrium)
    good = {}
    for d in ch:
        vals = []
        for b in S[d]:
            v = np.mean([np.mean([score(b, o) for o in S[q]]) for q in ch if q != d])
            vals.append(v)
        # good = the five builds of that character with the best average against everyone
        good[d] = [S[d][i] for i in np.argsort(vals)[-5:]]
    exact = []
    for p, q in itertools.combinations(ch, 2):
        for a in good[p]:
            for b in good[q]:
                exact.append((score(a, b, window=0), a, b))
    s = np.array([e[0] for e in exact])
    lop = np.mean((s < 0.4) | (s > 0.6))
    print(f'+{budget}: {len(exact)} pairings of good builds, exact health: {s.min():.2f}-{s.max():.2f}, lopsided {lop:.0%}; smoothed over +-2 cards the same pairings: ', end='')
    sm = np.array([score(a, b) for _, a, b in exact])
    print(f'{sm.min():.2f}-{sm.max():.2f}, lopsided {np.mean((sm < 0.4) | (sm > 0.6)):.0%}')
