from race import *
import sys, time
REF = sys.argv[1] if len(sys.argv) > 1 else 'Standard · Standard D6'
die = BOARDS[REF]['die']
rows = []
t0 = time.time()
for d in ['none', 'light', 'holdfast']:
    P = prev(die, d)
    for T in [5, 6.2, 7, 8, 9, 10, 12, 14, 16, 18, 19]:
        r, ra = board_at(REF, T); c, ca = board_at(REF, T + 1)
        for h in [10, 15, 20, 30, 40]:
            k, s0 = solve_blanks(c, r, P, P, h)
            kp, mc, mr = predict_blanks(c, r, P, P, h)
            rounds = mean_rounds(kill_rounds(defend(r, P), h))
            rows.append(dict(defense=d, T=T, avg=ra, davg=ca - ra, h=h, k=k, per_point=k / (ca - ra), pred=kp / (ca - ra), s0=s0, rounds=rounds, m=mr, mc=mc))
    print(d, time.time() - t0, file=sys.stderr)
json.dump(rows, open(f'{OUT}/offense-{REF.split()[0]}-{die}.json', 'w'), indent=1)
for d in ['none', 'light', 'holdfast']:
    print(f'\n## {d}: blanks per +1 average damage (model; prediction)   rounds at H')
    hs = [10, 15, 20, 30, 40]
    print('T     avg  net  ' + ' '.join(f'  H={h:<12}' for h in hs))
    for T in [5, 6.2, 7, 8, 9, 10, 12, 14, 16, 18, 19]:
        rs = [x for x in rows if x['defense'] == d and x['T'] == T]
        print(f"{T:<5} {rs[0]['avg']:.2f} {rs[0]['m']:.2f} " + ' '.join(f"{x['per_point']:5.2f}/{x['pred']:5.2f} {x['rounds']:4.1f}r" for x in rs))
