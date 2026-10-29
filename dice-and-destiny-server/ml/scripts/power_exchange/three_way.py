from three import *
import sys
def solve_c(A, Bb, cdef='light'):
    """Light Guard build (T_C, H_C) even with both A and B: scan T_C, find where the two needed healths cross."""
    prev_d = None
    rows = []
    for T in np.arange(5.0, 16.01, 0.1):
        T = round(T, 1)
        ha = even_health(A, T, cdef); hb = even_health(Bb, T, cdef)
        d = ha - hb
        rows.append((T, ha, hb))
        if prev_d is not None and d * prev_d <= 0:
            t0, ha0, hb0 = rows[-2]
            f = prev_d / (prev_d - d)
            return t0 + f * (T - t0), ha0 + f * (ha - ha0), avg(t0) + f * (avg(T) - avg(t0))
        prev_d = d
    return None
for TA, HA in [(8, 20), (10, 20), (12, 20), (10, 15), (10, 30)]:
    A = (TA, 'none', HA)
    hb = even_health(A, 6.2, 'holdfast'); Bb = (6.2, 'holdfast', hb)
    c = solve_c(A, Bb)
    if c is None:
        print(f'A none {avg(TA):.2f} dmg, {HA} cards | B Holdfast 6.18 dmg, {hb:.1f} cards | C: no Light Guard build ties both'); continue
    tc, hc, ac = c
    C = (round(tc, 1), 'light', hc)
    checks = f'A-B {sc(A,Bb):.3f}  C-A {sc(C,A):.3f}  C-B {sc(C,Bb):.3f}'
    print(f'A none {avg(TA):.2f} dmg, {HA} cards | B Holdfast 6.18 dmg, {hb:.1f} cards | C Light Guard {ac:.2f} dmg, {hc:.1f} cards   [{checks}]')
