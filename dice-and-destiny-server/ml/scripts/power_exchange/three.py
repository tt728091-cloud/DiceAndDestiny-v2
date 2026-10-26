"""Three Starters: A no defense + damage, B Holdfast, C Light Guard + damage. Where are all three even?"""
from race import *
REF = 'Standard · Standard D6'; die = 'standard_d6'; B = BOARDS[REF]
W = 2
def odds(T): return board_at(REF, T)[0]
def avg(T): return board_at(REF, T)[1]
def sc(x, y):
    """x's score against y; builds are (target, defense, health); health may be fractional (interpolated)."""
    def at(hx, hy):
        return np.mean([score(odds(x[0]), odds(y[0]), prev(die, x[1]), prev(die, y[1]), hx + d, hy + d) for d in range(-W, W + 1)])
    fx, fy = x[2] - int(x[2]), y[2] - int(y[2])
    hx, hy = int(x[2]), int(y[2])
    # bilinear in the two healths
    return ((1-fx)*(1-fy)*at(hx, hy) + fx*(1-fy)*at(hx+1, hy) + (1-fx)*fy*at(hx, hy+1) + fx*fy*at(hx+1, hy+1))
def even_health(x, yT, yD, lo=3, hi=150):
    """Health y needs (target yT, defense yD) to be even with x."""
    f = lambda h: sc(x, (yT, yD, h)) - 0.5   # decreasing in h
    if f(lo) < 0: return float('nan')
    for _ in range(40):
        mid = (lo + hi) / 2
        if f(mid) > 0: lo = mid
        else: hi = mid
    return (lo + hi) / 2
def rule_health(x, yT, yD):
    """Product rule: H_y = H_x * m(x->y) / m(y->x), exact damage-through odds."""
    mxy = stats(defend(odds(x[0]), prev(die, yD)))[0]; myx = stats(defend(odds(yT), prev(die, x[1])))[0]
    return x[2] * mxy / myx
def rounds(x, y):
    return mean_rounds(kill_rounds(defend(odds(x[0]), prev(die, y[1])), int(round(y[2]))))
