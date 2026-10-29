from race import *
labels = list(BOARDS)
def field_sd(T, scale, d='holdfast'):
    ods = {l: board_at(l, T)[0] for l in labels}
    means = []
    for a in labels:
        tot = n = 0
        for b in labels:
            if a == b: continue
            pa, pb = prev(BOARDS[a]['die'], d), prev(BOARDS[b]['die'], d)
            for h0 in range(12, 31, 3):
                h = max(3, round(h0 * scale))
                tot += score(ods[a], ods[b], pa, pb, h, h); n += 1
        means.append(tot / n)
    return np.std(means), min(means), max(means)
r, _ = board_at('Standard · Standard D6', 6.2)
for d in ['holdfast', 'none']:
    P = prev('standard_d6', d); m0 = stats(defend(r, P))[0]
    for T in [6.2, 10, 14, 20]:
        m = stats(defend(board_at('Standard · Standard D6', T)[0], P))[0]
        a = field_sd(T, 1.0, d); b = field_sd(T, m / m0, d)
        print(f'{d:8s} T={T:4}: health 12-30 sd {a[0]:.3f} ({a[1]:.3f}-{a[2]:.3f}) | health scaled x{m/m0:.2f} (same hits to kill) sd {b[0]:.3f} ({b[1]:.3f}-{b[2]:.3f})')
