"""The two even families, checked by the race model at many levels.
Family 1 (same cards): damage = T* - average prevention. Family 2 (same damage T): cards = N x (T - prevention)."""
from field import *
DEFS = ['none', 'light', 'light2', 'holdfast', 'heavy', 'bulwark']
def at_avg(target_avg):
    """Staircase target whose whole average is closest to target_avg (and the average it lands on)."""
    i = int(np.argmin(np.abs(BOARD.averages - target_avg)))
    return i, float(BOARD.averages[i])
def report(builds, label):
    names = [f'{d}' for (_, d, _) in builds]
    m = matrix(builds)
    off = m[~np.eye(len(m), dtype=bool)]
    worst = np.unravel_index(np.argmin(m), m.shape)
    print(f'{label}: scores {off.min():.3f}-{off.max():.3f}; worst {names[worst[0]]} vs {names[worst[1]]}')
    return m
if __name__ == '__main__':
    print('## Family 1: same cards, damage = T* - average prevention')
    for H in [15, 20, 30, 45]:
        for Ts in [9, 11, 14, 17, 20]:
            bs = []
            for d in DEFS:
                want = Ts - DEF_AVG[d]
                if want < 5.0: continue
                i, a = at_avg(want); bs.append((i, d, H))
            desc = ' '.join(f'{d}:{BOARD.averages[i]:.1f}' for i, d, _ in bs)
            report(bs, f'  {H:2d} cards, T*={Ts:4.1f} [{desc}]')
    print('\n## Family 2: same damage T, cards = N x (T - prevention)')
    for N in [3, 4, 5, 6]:
        for T in [6.2, 7.5, 9, 11, 14, 17, 20]:
            i, a = at_avg(T)
            bs = []
            for d in DEFS:
                if a - DEF_AVG[d] < 1.5: continue
                h = int(round(N * (a - DEF_AVG[d])))
                if h < 4: continue
                bs.append((i, d, h))
            desc = ' '.join(f'{d}:{h}c' for _, d, h in bs)
            report(bs, f'  N={N} T={a:5.2f} [{desc}]')
