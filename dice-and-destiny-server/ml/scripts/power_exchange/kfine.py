from chars import *
six = ['none', 'light', 'light2', 'holdfast', 'heavy', 'bulwark']
budgets = [150, 250, 400, 600, 900]
for k in [0.9, 0.95, 1.0, 1.05, 1.1]:
    devs, means = [], {d: [] for d in six}
    for b in budgets:
        r = solve(six, b, 40, k, iters=4000)
        for p, x in r['chars'].items(): means[p].append(x['value'])
        devs.append(max(abs(x['value'] - 0.5) for x in r['chars'].values()))
    print(f'k={k:.2f}: worst |value-0.5| by budget ' + ' '.join(f'{d:.3f}' for d in devs) + ' | mean value per defense: ' + ' '.join(f'{p} {np.mean(v):.3f}' for p, v in means.items() if v), flush=True)
