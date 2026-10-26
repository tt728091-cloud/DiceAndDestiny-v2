"""Holdfast Starter at 6.2/7.5/9/11: the damage (same cards) or the cards (same damage) Light Guard and no defense need."""
from three import *
B_ = BOARDS[REF]
def m(T, D): return stats(defend(odds(T), prev(die, D)))[0]
def even_damage(x, yD, h):
    """Average damage y (defense yD, same cards h) needs to be even with x: interpolated along the staircase."""
    ps = pa = None
    for i, t in enumerate(B_['targets']):
        s = sc((t, yD, h), x); a = B_['averages'][i]
        if s >= 0.5:
            return a if ps is None else pa + (a - pa) * (0.5 - ps) / (s - ps), t
        ps, pa = s, a
    return float('nan'), None
rows = []
for h in [15, 20, 30]:
    print(f'\n## {h} cards each')
    print('Holdfast avg | eff. prevention L / H | Light needs avg (+%)      | No defense needs avg (+%)  | check L-N | cards at same damage: Light, None | rounds')
    for TH in [6.2, 7.5, 9.0, 11.0]:
        H = (TH, 'holdfast', h); aH = avg(TH)
        aL, tL = even_damage(H, 'light', h); aN, tN = even_damage(H, 'none', h)
        effL, effH = aH - m(TH, 'light'), aH - m(TH, 'holdfast')
        cross = sc((tL, 'light', h), (tN, 'none', h)) if tL and tN else float('nan')
        cL = even_health(H, TH, 'light'); cN = even_health(H, TH, 'none')
        r = rounds(H, (TH, 'none', h))
        print(f'   {aH:5.2f}    |   {effL:.2f} / {effH:.2f}          | {aL:5.2f} (+{aL-aH:.2f}, +{100*(aL/aH-1):3.0f}%)  | {aN:5.2f} (+{aN-aH:.2f}, +{100*(aN/aH-1):3.0f}%) | {cross:.3f}     | {cL:5.1f} (+{100*(cL/h-1):3.0f}%), {cN:5.1f} (+{100*(cN/h-1):3.0f}%) | {r:.1f}')
        rows.append(dict(h=h, TH=TH, aH=aH, aL=aL, aN=aN, effL=effL, effH=effH, cross=cross, cL=cL, cN=cN))
json.dump(rows, open(f'{OUT}/defense_price.json', 'w'), indent=1)
