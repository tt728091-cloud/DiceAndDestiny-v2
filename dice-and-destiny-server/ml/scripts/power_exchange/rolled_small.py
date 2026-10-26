import numpy as np, json
import field
from rolled import *
rows = []
for d in ['half', 'light', 'light2', 'holdfast']:
    D = field.DEF_AVG[d]
    for high in [8, 9, 10, 12]:
        best = None
        for kd in np.arange(0.50, 0.995, 0.01):
            p = profile(high, d, kd, XS[::3])
            key = (np.mean(p <= 0.1), -p.max())
            if best is None or key > best[0]: best = (key, kd)
        kd = best[1]; p = profile(high, d, kd)
        at3 = max(worst_at(X, high, d, kd) for X in [400, 650, 900])
        rows.append(dict(defense=d, prevent=D, high=at_avg(high)[1], keep_def=kd, inside=float(np.mean(p <= 0.1)), worst=float(p.max()), at3=at3))
        print(f'{d:8s} prevents {D:.1f} | high {at_avg(high)[1]:5.2f} | defense costs {100*(1-kd):4.1f}% of the rest | inside 40-60 at {100*np.mean(p<=0.1):3.0f}% of XP 400-1500, worst {50-100*p.max():.0f}-{50+100*p.max():.0f} | at 400/650/900 {50-100*at3:.0f}-{50+100*at3:.0f}', flush=True)
json.dump(rows, open(f'{OUT}/rolled_small.json', 'w'), indent=1)
