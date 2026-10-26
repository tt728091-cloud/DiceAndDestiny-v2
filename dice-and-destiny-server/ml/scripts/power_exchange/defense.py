from race import *
import sys
REF = sys.argv[1] if len(sys.argv) > 1 else 'Standard · Standard D6'
die = BOARDS[REF]['die']
B = BOARDS[REF]
def solve_target(lo, hi, T, h, w=2):
    """C has defense lo and more damage; R has defense hi at T. Average damage C needs for 0.5."""
    r, ra = board_at(REF, T); prev_s = None; prev_a = None
    for i, t in enumerate(B['targets']):
        if t < T - 1e-9: continue
        c = np.array(B['odds'][i]); ca = B['averages'][i]
        s = window_score(c, r, prev(die, lo), prev(die, hi), h, 0, w)
        if s >= 0.5:
            if prev_s is None: return ca - ra
            return (prev_a + (ca - prev_a) * (0.5 - prev_s) / (s - prev_s)) - ra
        prev_s, prev_a = s, ca
    return float('nan')
out = []
pairs = [('none', 'light'), ('light', 'holdfast'), ('none', 'holdfast')]
for lo, hi in pairs:
    print(f'\n## {lo} -> {hi}: blanks the upgrade is worth (model/prediction) | extra average damage that matches it (same health)')
    hs = [10, 15, 20, 30, 40]
    print('T     ' + ' '.join(f'  H={h:<16}' for h in hs))
    for T in [5, 6.2, 7, 8, 10, 12, 14, 16, 19]:
        r, ra = board_at(REF, T)
        cells = []
        for h in hs:
            k, s0 = solve_blanks(r, r, prev(die, hi), prev(die, lo), h)
            kp, _, _ = predict_blanks(r, r, prev(die, hi), prev(die, lo), h)
            x = solve_target(lo, hi, T, h) if T <= 15 else float('nan')
            cells.append(f'{k:5.1f}/{kp:5.1f} +{x:4.2f}dmg')
            out.append(dict(lo=lo, hi=hi, T=T, h=h, blanks=k, pred=kp, dmg=x, s0=s0))
        print(f'{T:<5} ' + ' '.join(cells))
json.dump(out, open(f'{OUT}/defense.json', 'w'), indent=1)
