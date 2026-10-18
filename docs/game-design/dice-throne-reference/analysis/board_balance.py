"""Price an ability board's damage from its hit odds, then check it with optimal play.

Each tier gets damage = anchor * (p_anchor / p_tier) ** alpha, where p is the
chance to roll that tier when chasing it alone (5 dice, 3 rolls). alpha = 1
gives every tier the same chase average (damage x odds); alpha = 0 gives all
tiers the same damage.

The board is then played exactly: after each roll the player keeps the dice
that maximize the expected damage of the best tier they will end on. The
report shows each tier's chase odds, its damage, how often it is the tier
actually used, and how much the turn's average damage drops without it.

    python3 -I board_balance.py --alpha 0.5 --anchor 10

With --board starter it plays the Starter's printed damage instead, and shows
the curve's damage for the same anchor and the alpha each printed number
implies. Side effects count at the exchange rates in EXCHANGE.
"""
import argparse
import math
import os
import sys
from collections import Counter

sys.path.insert(0, os.path.dirname(os.path.abspath(__file__)))
from ability_layouts import chance, predicate, roll_dist

# Our standard die: Sword 1-3 (A), Shield 4-5 (B), Gold coin 6 (C).
FACES = {"A": frozenset({1, 2, 3}), "B": frozenset({4, 5}), "C": frozenset({6})}
SYMBOLS = {"A": "Sword", "B": "Shield", "C": "Coin"}

# The 3-2-1 "Standard" template (Tactician / Cyclops / Thor shape).
STANDARD_321 = [
    ("Ladder", ["A3", "A4", "A5"]),
    ("Sword + 3 Shield", ["A1+B3"]),
    ("Small straight", ["SS"]),
    ("2 Sword + 2 Coin", ["A2+C2"]),
    ("Large straight", ["LS"]),
    ("3 Sword + 2 Coin", ["A3+C2"]),
    ("4 Coin", ["C4"]),
    ("Ultimate 5 Coin", ["C5"]),
]

# Damage equivalents for side effects (agreed 2026-10-15: 2 Protect = 2 damage,
# 5 energy = 1 damage).
EXCHANGE = {"protect": 1.0, "energy": 0.2}

# The Starter (adventurer_v1/combatants/starter.yaml): value per tier, damage
# plus side effects at the exchange rates.
STARTER = [
    ("Strike", ["A3", "A4", "A5"], [4, 5, 6]),
    ("Guarded Strike", ["A2+B3"], [5 + 2 * EXCHANGE["protect"]]),
    ("Measured Strike", ["A2+C2"], [4 + 5 * EXCHANGE["energy"]]),
    ("Small Straight", ["SS"], [6]),
    ("Large Straight", ["LS"], [8]),
    ("Decisive Blow", ["C4"], [10]),
]


def label(tier):
    if tier in ("SS", "LS"):
        return tier
    return " + ".join(f"{p[1:]} {SYMBOLS[p[0]]}" for p in tier.split("+"))


def tiers_of(board):
    out = []
    for name, tiers in board:
        for t in tiers:
            out.append((name, t, predicate(t, FACES)))
    return out


def solve(tiers, damage, rolls=3, dice=5):
    """Optimal-play values, then the exact chance each tier is the one used."""
    states = [s for s, _ in roll_dist(dice)]

    def best(s):
        hit = [(damage[i], i) for i, (_, _, ok) in enumerate(tiers) if ok(s)]
        if not hit:
            return 0.0, [None]
        top = max(d for d, _ in hit)
        return top, [i for d, i in hit if d == top]  # equal values share the use

    stop = {s: best(s) for s in states}
    value = [{s: stop[s][0] for s in states}]  # value[k][s]: k rerolls left
    policy = [None]
    for k in range(1, rolls):
        v, pol = {}, {}
        for s in states:
            keeps = {tuple(s[i] for i in range(dice) if m >> i & 1) for m in range(1 << dice)}
            choice = max(
                keeps,
                key=lambda keep: (
                    sum(w * value[k - 1][tuple(sorted(keep + o))] for o, w in roll_dist(dice - len(keep))),
                    len(keep),
                ),
            )
            v[s] = sum(w * value[k - 1][tuple(sorted(choice + o))] for o, w in roll_dist(dice - len(choice)))
            pol[s] = choice
        value.append(v)
        policy.append(pol)

    dist = Counter({s: w for s, w in roll_dist(dice)})
    for k in range(rolls - 1, 0, -1):
        nxt = Counter()
        for s, w in dist.items():
            keep = policy[k][s]
            for o, w2 in roll_dist(dice - len(keep)):
                nxt[tuple(sorted(keep + o))] += w * w2
        dist = nxt
    used = Counter()
    for s, w in dist.items():
        for i in stop[s][1]:
            used[i] += w / len(stop[s][1])
    ev = sum(w * stop[s][0] for s, w in dist.items())
    return ev, used


def main():
    ap = argparse.ArgumentParser()
    ap.add_argument("--anchor", type=float, default=10.0, help="damage of the first tier (the anchor)")
    ap.add_argument("--alpha", type=float, default=0.5)
    ap.add_argument("--board", choices=["standard", "starter"], default="standard")
    ap.add_argument("--no-marginal", action="store_true", help="skip the slower without-this-tier runs")
    args = ap.parse_args()

    if args.board == "starter":
        tiers = tiers_of([(n, t) for n, t, _ in STARTER])
        printed = [float(d) for _, _, ds in STARTER for d in ds]
        args.anchor = printed[0]
    else:
        tiers = tiers_of(STANDARD_321)
        printed = None
    odds = [chance(ok) for _, _, ok in tiers]
    curve = [args.anchor * (odds[0] / p) ** args.alpha for p in odds]
    damage = printed or curve
    ev, used = solve(tiers, damage)
    print(f"board={args.board}  alpha={args.alpha}  anchor={args.anchor:g}  average damage per turn={ev:.2f}  "
          f"no hit={used[None]:.1%}")
    if printed:
        curve_ev, _ = solve(tiers, curve)
        print(f"same board priced by the curve: average damage per turn={curve_ev:.2f}")
    extra = f" {'curve':>6s} {'implied a':>9s}" if printed else ""
    print(f"{'ability':18s} {'tier':16s} {'chase':>6s} {'damage':>7s} {'chase x dmg':>11s} {'used':>6s} {'worth':>6s}{extra}")
    for i, (name, t, _) in enumerate(tiers):
        worth = ""
        if not args.no_marginal:
            removed = list(damage)
            removed[i] = 0.0
            worth = f"{ev - solve(tiers, removed)[0]:6.2f}"
        print(f"{name:18s} {label(t):16s} {odds[i]:6.1%} {damage[i]:7.1f} {odds[i] * damage[i]:11.2f} "
              f"{used[i]:6.1%} {worth}", end="")
        if printed:
            implied = math.log(damage[i] / damage[0]) / math.log(odds[0] / odds[i]) if i else float("nan")
            print(f" {curve[i]:6.1f} {implied:9.2f}", end="")
        print()


if __name__ == "__main__":
    main()
