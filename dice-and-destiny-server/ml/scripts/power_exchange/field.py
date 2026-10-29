"""Field engine for the pricing study: builds (damage target, defense, cards) priced by a scheme, every build
against every other by the race model (vectorised over health), and the field's equilibrium.

A build is (target, defense, cards). The board is the Standard template on the standard die at that target on its
staircase (odds from board-lab -exchange-dump). Cards are blank cards: health only, 10 XP each.
"""
import json, os, math
import numpy as np

OUT = os.environ.get('EXCHANGE_DIR', os.path.join(os.path.dirname(os.path.abspath(__file__)), '../../runs/exchange'))
_D = json.load(open(os.path.join(OUT, 'odds.json')))
BOARDS = {b['label']: b for b in _D['boards']}
REF = 'Standard · Standard D6'
HMAX = 260
WINDOW = 2  # each score is averaged over both healths shifted by -WINDOW..WINDOW


def _roles(main, side, apex):
    """Three dice of the standard die (3 main, 2 side, 1 apex faces), each face preventing its role's amount."""
    face = [main] * 3 + [side] * 2 + [apex]
    dist = np.array([1.0])
    for _ in range(3):
        nxt = np.zeros(len(dist) + max(face))
        for k, p in enumerate(dist):
            for f in face:
                nxt[k + f] += p / 6
        dist = nxt
    return dist


def _light(n):
    d = np.array([1.0])
    for _ in range(n):
        d = np.convolve(d, [1 / 3, 1 / 3, 1 / 3])
    return d


# Defenses by average prevention per roll. holdfast is the shipped one; the others are hypothetical except light,
# the lab's Light Guard.
DEFENSES = {
    'none': np.array([1.0]),
    'light': _light(1),          # 1 die: 0/1/2, average 1
    'light2': _light(2),         # 2 such dice, average 2
    'holdfast': _roles(1, 2, 0),  # average 3.5
    'heavy': _roles(1, 2, 2),    # average 4.5
    'bulwark': _roles(1, 3, 2),  # average 5.5
}
DEF_AVG = {k: float(np.dot(np.arange(len(v)), v)) for k, v in DEFENSES.items()}

# Percentage defenses (hypothetical): prevent a share of each hit, rounded at random so the share is exact.
PCT = {'ward37': 0.37}


def defend_pct(odds, p):
    out = np.zeros(len(odds))
    for d, pd in enumerate(odds):
        x = p * d
        lo = int(np.floor(x)); f = x - lo
        out[d - lo] += pd * (1 - f)
        if f > 0:
            out[d - lo - 1] += pd * f
    return out


class Board:
    def __init__(self, label=REF):
        b = BOARDS[label]
        self.targets = np.array(b['targets'])
        self.averages = np.array(b['averages'])
        self.odds = [np.array(o) for o in b['odds']]

    def index(self, target):
        return int(np.argmin(np.abs(self.targets - target)))


BOARD = Board()


def use_board(label):
    """Price and play every build on another default board (and its die's defense odds are unchanged: standard die)."""
    global BOARD
    BOARD = Board(label)
    import sys
    for mod in ('chars', 'study', 'families', 'drift'):
        m = sys.modules.get(mod)
        if m is not None:
            m.BOARD = BOARD
            if hasattr(m, 'A0'):
                m.A0 = float(BOARD.averages[BOARD.index(m.BASE_TARGET)])


def defend(odds, prevent):
    out = np.zeros(len(odds))
    for k, pk in enumerate(prevent):
        if pk == 0:
            continue
        if k == 0:
            out += odds * pk
        else:
            out[0] += odds[:k + 1].sum() * pk
            out[1:len(odds) - k] += odds[k + 1:] * pk
    return out


_surv = {}


def surv(ti, dname, board=None):
    """S[t, h]: chance that t rounds of this attack (board at target index ti, through defense dname) dealt < h."""
    board = board or BOARD
    key = (id(board), ti, dname)
    s = _surv.get(key)
    if s is not None:
        return s
    odds = defend_pct(board.odds[ti], PCT[dname]) if dname in PCT else defend(board.odds[ti], DEFENSES[dname])
    dist = np.zeros(HMAX + 1)
    dist[0] = 1.0
    rows = [np.concatenate([[0.0], np.cumsum(dist[:HMAX])])]
    for _ in range(1500):
        conv = np.convolve(dist[:HMAX], odds)
        nxt = np.zeros(HMAX + 1)
        nxt[:HMAX] = conv[:HMAX]
        nxt[HMAX] = dist[HMAX] + conv[HMAX:].sum()
        dist = nxt
        rows.append(np.concatenate([[0.0], np.cumsum(dist[:HMAX])]))
        if dist[:HMAX].sum() < 1e-10:
            break
    s = np.array(rows)
    _surv[key] = s
    return s


def mean_through(ti, dname, board=None):
    board = board or BOARD
    o = defend_pct(board.odds[ti], PCT[dname]) if dname in PCT else defend(board.odds[ti], DEFENSES[dname])
    return float(np.dot(np.arange(len(o)), o))


def score(a, b, window=WINDOW, board=None):
    """a's expected score against b. Builds are (target index, defense, cards)."""
    sa = surv(a[0], b[1], board)  # a attacking through b's defense
    sb = surv(b[0], a[1], board)
    n = max(len(sa), len(sb))
    tot = 0.0
    cnt = 0
    for d in range(-window, window + 1):
        ha, hb = a[2] + d, b[2] + d
        if ha < 1 or hb < 1:
            continue
        ha, hb = min(ha, HMAX), min(hb, HMAX)
        ka = 1 - sa[:, hb]
        kb = 1 - sb[:, ha]
        if len(ka) < n:
            ka = np.concatenate([ka, np.full(n - len(ka), ka[-1])])
        if len(kb) < n:
            kb = np.concatenate([kb, np.full(n - len(kb), kb[-1])])
        pa = np.diff(ka, prepend=0.0)
        pb = np.diff(kb, prepend=0.0)
        # whatever is left unresolved after the last round counts as a draw
        left = (1 - ka[-1]) * (1 - kb[-1])
        tot += float(np.sum(pa * (1 - kb + 0.5 * pb))) + 0.5 * left
        cnt += 1
    return tot / cnt


def matrix(builds, board=None):
    n = len(builds)
    m = np.full((n, n), 0.5)
    for i in range(n):
        for j in range(i + 1, n):
            s = score(builds[i], builds[j], board=board)
            m[i, j], m[j, i] = s, 1 - s
    return m


def equilibrium(m, iters=20000, eta=0.5):
    """Symmetric equilibrium of the zero-sum game with payoff m - 0.5 (multiplicative weights, averaged)."""
    n = len(m)
    x = np.full(n, 1.0 / n)
    avg = np.zeros(n)
    a = m - 0.5
    for t in range(iters):
        u = a @ x
        x = x * np.exp(eta * (u - x @ u))
        x /= x.sum()
        if t >= iters // 2:
            avg += x
    avg /= avg.sum()
    return avg
