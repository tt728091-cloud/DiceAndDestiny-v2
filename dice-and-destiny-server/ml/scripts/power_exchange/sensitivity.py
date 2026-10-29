from race import *
from math import erf, sqrt, log
import random
Phi = lambda z: 0.5 * (1 + erf(z / sqrt(2)))
REF = 'Standard · Standard D6'; die = BOARDS[REF]['die']
random.seed(3)
rows = []
labels = list(BOARDS)
for _ in range(1500):
    la = random.choice(labels); lb = random.choice(labels)
    ta = random.choice([5, 6.2, 7, 8, 10, 12, 14, 16]); tb = ta + random.choice([0, 0, 0.5, 1, 2, -1])
    tb = min(max(tb, 5), 20)
    da = random.choice(['none', 'light', 'holdfast']); db = random.choice(['none', 'light', 'holdfast'])
    ha = random.choice(range(10, 41)); hb = max(6, ha + random.choice([0, 0, -3, 3, -6, 6]))
    A, aa = board_at(la, ta); Bo, ba = board_at(lb, tb)
    pa, pb = prev(BOARDS[la]['die'], da), prev(BOARDS[lb]['die'], db)
    s = score(A, Bo, pa, pb, ha, hb)
    ma, m2a = stats(defend(A, pb)); mb, m2b = stats(defend(Bo, pa))
    if ma < 0.3 or mb < 0.3: continue
    oa, ob = (m2a / ma - 1) / 2, (m2b / mb - 1) / 2
    Na, Nb = (hb + oa) / ma, (ha + ob) / mb          # rounds each needs to kill (staying-power rule)
    cva, cvb = sqrt(max(m2a - ma * ma, 0)) / ma, sqrt(max(m2b - mb * mb, 0)) / mb
    # first-passage spread: sd(N) ~ cv * sqrt(N); difference of the two kill times
    sd = sqrt(cva ** 2 * Na + cvb ** 2 * Nb + 1 / 6)   # +1/6: whole rounds
    z = (Nb - Na) / sd
    rows.append(dict(s=s, pred=Phi(z), simple=0.5 + 0.0, Na=Na, Nb=Nb, z=z, edge=log(Nb / Na), rounds=(Na + Nb) / 2, cv=(cva + cvb) / 2))
err = np.array([r['s'] - r['pred'] for r in rows])
print('pairs', len(rows), 'mean abs err', np.mean(np.abs(err)).round(4), 'p90', np.quantile(np.abs(err), .9).round(4), 'max', np.max(np.abs(err)).round(3))
# winner prediction accuracy
ok = np.mean([(r['s'] - 0.5) * (r['Nb'] - r['Na']) >= 0 for r in rows if abs(r['s'] - 0.5) > 0.02])
print('sign agreement (|s-0.5|>0.02):', ok.round(3))
for lo, hi in [(0, 2.5), (2.5, 4), (4, 7), (7, 50)]:
    e = [abs(r['s'] - r['pred']) for r in rows if lo <= r['rounds'] < hi]
    print(f'rounds {lo}-{hi}: n={len(e)} mean abs err {np.mean(e):.3f}')
