"""The user's system, solved. Builds i = (offense O_i, defense D_i, cards H_i).
Rounds for j to defeat i:  R_ij = H_i / m(O_j, D_i)   (m = average damage of j that gets through i's defense).
Fair pair: R_ij = R_ji  <=>  ln H_i - ln H_j = ln m(O_j, D_i) - ln m(O_i, D_j).
A: card counts chosen freely (no prices): least-squares best fit (the 'line of best fit').
B: prices: flat (additive) vs percentage of total XP (multiplicative), fitted to the same equations."""
import itertools, sys
import numpy as np
from field import BOARD, mean_through, DEF_AVG

def at_avg(x):
    i = int(np.argmin(np.abs(BOARD.averages - x))); return i, float(BOARD.averages[i])

def m_exact(O, D):
    return mean_through(at_avg(O)[0], D)

def m_linear(O, D):
    return max(O - DEF_AVG[D], 0.05)

def residuals(builds, lnH, m):
    """log kill-time ratio for every pair: ln R_ij - ln R_ji (0 = perfectly even)."""
    out = {}
    for (i, a), (j, b) in itertools.combinations(enumerate(builds), 2):
        out[(i, j)] = (lnH[i] - np.log(m(b[0], a[1]))) - (lnH[j] - np.log(m(a[0], b[1])))
    return out

def best_fit(builds, m):
    """Least squares over ln H (one degree of freedom is the overall size, fixed by ln H_0 = 0)."""
    n = len(builds); rows, rhs = [], []
    for (i, a), (j, b) in itertools.combinations(enumerate(builds), 2):
        r = np.zeros(n); r[i], r[j] = 1, -1; rows.append(r); rhs.append(np.log(m(b[0], a[1])) - np.log(m(a[0], b[1])))
    A = np.array(rows); y = np.array(rhs)
    A = np.vstack([A, np.eye(n)[0] * 10]); y = np.append(y, 0)  # pin ln H_0 = 0
    lnH, *_ = np.linalg.lstsq(A, y, rcond=None)
    return lnH

NAMES = ['1 Low O, no D', '2 Low O, Holdfast', '3 High O, no D', '4 High O, Holdfast']
B4 = [(7, 'none'), (7, 'holdfast'), (12, 'none'), (12, 'holdfast')]

if __name__ == '__main__':
    for label, m in [('linear (O - D)', m_linear), ('exact dice', m_exact)]:
        lnH = best_fit(B4, m)
        H = np.exp(lnH)
        print(f'\n## best fit, {label}: relative cards ' + ', '.join(f'{n} {h/H[0]:.3f}' for n, h in zip(NAMES, H)))
        for (i, j), e in residuals(B4, lnH, m).items():
            print(f'   {NAMES[i]:20s} vs {NAMES[j]:20s} rounds-to-defeat ratio {np.exp(e):.3f}  ({100*(np.exp(abs(e))-1):.1f}% apart)')
