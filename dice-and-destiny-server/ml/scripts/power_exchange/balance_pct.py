import sys, json
import field
from balance_search import *
for p in [0.10, 0.15, 0.20, 0.25, 0.30, 0.37]:
    field.PCT[f'ward{int(round(p*100))}'] = p
rows = []
levels = [400, 650, 900]
for high in [9, 10, 11, 12, 13, 14]:
    for p in [0.10, 0.15, 0.20, 0.25, 0.30, 0.37]:
        dname = f'ward{int(round(p*100))}'
        w, r, g = search(high, dname, levels)
        rows.append(dict(high=high, high_avg=at_avg(high)[1], defense=dname, p=p, worst=w, r=r, g=g))
        print(f'high {at_avg(high)[1]:5.2f} | prevents {int(p*100)}% | worst {50-100*w:4.1f}-{50+100*w:4.1f} | r {r:.3f} g {g:.3f}', flush=True)
json.dump(rows, open(f'{OUT}/balance_pct.json', 'w'), indent=1)
