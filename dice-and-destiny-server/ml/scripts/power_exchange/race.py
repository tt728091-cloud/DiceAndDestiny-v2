"""Race model (port of cmd/board-lab/duel.go) and exchange-rate solvers."""
import json, os
import numpy as np

# The board-lab dump (go run ./cmd/board-lab -exchange-dump <path>); EXCHANGE_DIR holds it and the outputs.
OUT = os.environ.get('EXCHANGE_DIR', os.path.join(os.path.dirname(os.path.abspath(__file__)), '../../runs/exchange'))
D = json.load(open(os.path.join(OUT, 'odds.json')))
BOARDS = {b['label']: b for b in D['boards']}
DEF = D['defenses']

def board_at(label, target):
    b = BOARDS[label]
    i = int(round((target - b['targets'][0]) / (b['targets'][1] - b['targets'][0])))
    i = max(0, min(i, len(b['targets']) - 1))
    return np.array(b['odds'][i]), b['averages'][i]

def defend(odds, prevent):
    out = np.zeros(len(odds))
    for k, pk in enumerate(prevent):
        if pk == 0: continue
        for d, pd in enumerate(odds):
            out[max(0, d - k)] += pd * pk
    return out

def kill_rounds(odds, health, rounds=400):
    alive = np.zeros(health); alive[0] = 1
    out = []
    for _ in range(rounds):
        nxt = np.zeros(health); killed = 0.0
        for d, pd in enumerate(odds):
            if pd == 0: continue
            if d >= health:
                killed += pd * alive.sum(); continue
            nxt[d:] += pd * alive[:health - d]
            killed += pd * alive[health - d:].sum()
        out.append(killed); alive = nxt
        if alive.sum() < 1e-12: break
    return np.array(out)

def race_rounds(a, b):
    n = max(len(a), len(b)); a = np.pad(a, (0, n - len(a))); b = np.pad(b, (0, n - len(b)))
    their_left = 1 - np.concatenate([[0], np.cumsum(b)[:-1]])
    return float(np.sum(a * (their_left - 0.5 * b)))

def mean_rounds(kr):
    return float(np.sum(kr * np.arange(1, len(kr) + 1)))

def score(c_odds, r_odds, c_prev, r_prev, hc, hr):
    """Challenger's score: c deals through r's defense to r's health hr."""
    return race_rounds(kill_rounds(defend(c_odds, r_prev), hr), kill_rounds(defend(r_odds, c_prev), hc))

def window_score(c_odds, r_odds, c_prev, r_prev, h, k, w=2):
    """Score averaged over base healths h-w..h+w; k>0 gives the rival k blanks, k<0 the challenger."""
    tot = 0
    hs = range(h - w, h + w + 1)
    for b in hs:
        hc, hr = (b, b + k) if k >= 0 else (b - k, b)
        tot += score(c_odds, r_odds, c_prev, r_prev, hc, hr)
    return tot / len(hs)

def solve_blanks(c_odds, r_odds, c_prev, r_prev, h, w=2, kmax=200):
    """Blanks the rival needs so the challenger scores 0.5 (negative: the challenger needs them)."""
    f = lambda k: window_score(c_odds, r_odds, c_prev, r_prev, h, k, w)
    s0 = f(0)
    if abs(s0 - 0.5) < 1e-9: return 0.0, s0
    step = 1 if s0 > 0.5 else -1
    prev, k = s0, 0
    while abs(k) < kmax:
        k += step; s = f(k)
        if (s - 0.5) * (prev - 0.5) <= 0:
            return (k - step) + step * (prev - 0.5) / (prev - s), s0
        prev = s
    return float('nan'), s0

def stats(odds):
    x = np.arange(len(odds)); m = float(np.sum(x * odds)); m2 = float(np.sum(x * x * odds))
    return m, m2

_kr_cache = {}
_orig_kill_rounds = kill_rounds
def kill_rounds(odds, health, rounds=400):
    key = (odds.tobytes(), health)
    r = _kr_cache.get(key)
    if r is None:
        r = _orig_kill_rounds(odds, health, rounds); _kr_cache[key] = r
    return r

def prev(die, d): return np.array(DEF[die][d])

def predict_blanks(c_odds, r_odds, c_prev, r_prev, h):
    """Staying-power rule: rounds to kill = (H + overshoot) / m, even when equal."""
    mc, m2c = stats(defend(c_odds, r_prev)); mr, m2r = stats(defend(r_odds, c_prev))
    oc, orr = (m2c / mc - 1) / 2, (m2r / mr - 1) / 2
    # rival health H+k: (H+k+oc)/mc = (H+orr)/mr
    return (h + orr) * mc / mr - h - oc, mc, mr
