from race import *
REF = 'Standard · Standard D6'; die = BOARDS[REF]['die']
def solve_k(c, r, cp, rp, h, goal, w=2):
    prev_s, k = window_score(c, r, cp, rp, h, 0, w), 0
    while k < 300:
        k += 1; s = window_score(c, r, cp, rp, h, k, w)
        if s <= goal: return k - 1 + (prev_s - goal) / (prev_s - s)
        prev_s = s
print('Mirror of the Standard board: blanks the rival needs to win 60% (the lopsided line), and that as damage points and % of health')
hs = [10, 15, 20, 30, 40]
for d in ['none', 'light', 'holdfast']:
    P = prev(die, d)
    print(f'\n## {d}')
    print('T     net  cv   ' + ' '.join(f'H={h:<20}' for h in hs))
    for T in [5, 6.2, 8, 10, 12, 15, 19]:
        r, ra = board_at(REF, T); m, m2 = stats(defend(r, P)); cv = (m2 - m * m) ** .5 / m
        cells = []
        for h in hs:
            k = solve_k(r, r, P, P, h, 0.4)
            n = mean_rounds(kill_rounds(defend(r, P), h))
            cells.append(f'{k:4.1f}bl {100*k/h:3.0f}% {k*m/h:4.2f}dm {n:3.1f}r')
        print(f'{T:<5} {m:4.2f} {cv:4.2f} ' + ' '.join(cells))
