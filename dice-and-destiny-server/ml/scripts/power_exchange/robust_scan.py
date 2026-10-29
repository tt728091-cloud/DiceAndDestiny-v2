import json
import numpy as np
from robust import *
XS2 = list(range(400, 1501, 10))
rows = []
for mode in ['floor', 'nearest']:
    MODE['round'] = mode
    for high in [9, 10, 11, 12, 13, 14]:
        for p in [0.10, 0.15, 0.20, 0.25, 0.30, 0.37]:
            d = f'ward{int(round(p*100))}'
            r = at_avg(LOW)[1] / at_avg(high)[1]; g = 1 - p
            prof = profile(high, d, r, g, XS2)
            at3 = max(worst_at(high, d, r, g, X) for X in [400, 650, 900])
            rows.append(dict(mode=mode, high=at_avg(high)[1], p=p, inside=float(np.mean(prof <= 0.1)), worst=float(prof.max()), p90=float(np.quantile(prof, 0.9)), at3=float(at3)))
            print(f'{mode:7s} high {at_avg(high)[1]:5.2f} {int(p*100):2d}%: inside 40-60 at {100*np.mean(prof <= 0.1):3.0f}% of XP 400-1500, 90% of levels within {50-100*np.quantile(prof,0.9):.0f}-{50+100*np.quantile(prof,0.9):.0f}, worst {50-100*prof.max():.0f} | at 400/650/900 {50-100*at3:.0f}-{50+100*at3:.0f}', flush=True)
json.dump(rows, open(f'{OUT}/robust_scan.json', 'w'), indent=1)
