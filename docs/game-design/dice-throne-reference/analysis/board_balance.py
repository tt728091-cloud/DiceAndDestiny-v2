"""Turn a die and an ability board into the damage each ability should deal.

1. Price: each tier's relative payoff is (1 / p) ** alpha, where p is its
   chase odds, the chance to roll it when chasing it alone (5 dice, 3 rolls).
   That anchors the curve on a guaranteed hit (p = 1). alpha sets how much more
   harder tiers pay: 0 pays them all the same, 1 gives every tier the same
   chase average. Dice Throne's printed damage fits about 0.2.
2. Play: the board is played exactly. After each roll the player keeps the
   dice that maximize the expected damage of the best tier they will end on,
   which gives each tier's use share (how often it is the one used; the shares
   and the misses add up to 100%).
3. Scale: the sum of use share x payoff is the board's average per turn.
   Every payoff is multiplied by target / average, which keeps the same best
   play, so the board then averages exactly the target. The multiplier is
   what a guaranteed hit is worth on this board.
4. Round: damage is whole cards, so every monotone floor/ceiling rounding is
   played again. The one whose average stays within 2% of the target and
   whose use shares drift least from the exact board is reported.

The report also gives each tier's worth: how much the exact average drops
without it.

    python3 -I board_balance.py --board lean --target 6.2
    python3 -I board_balance.py --die 2-2-1-1 --costs "A3/A4/A5, A2+B2, SS/LS, D4, D5"

With --board starter (or --printed for a custom board) it also shows the
current values (side effects at agreed exchange rates) and their average.
"""
import argparse
import os
import sys
from collections import Counter

sys.path.insert(0, os.path.dirname(os.path.abspath(__file__)))
from ability_layouts import chance, predicate, roll_dist

# Dice by face layout: symbol slots A, B, C, D in face order. The 3-2-1 die is
# ours: Sword 1-3 (A), Shield 4-5 (B), Gold coin 6 (C).
DICE = {
    "3-2-1": {"A": frozenset({1, 2, 3}), "B": frozenset({4, 5}), "C": frozenset({6})},
    "2-2-1-1": {"A": frozenset({1, 2}), "B": frozenset({3, 4}), "C": frozenset({5}), "D": frozenset({6})},
    "3-1-1-1": {"A": frozenset({1, 2, 3}), "B": frozenset({4}), "C": frozenset({5}), "D": frozenset({6})},
    # Monk's die: Fist 1-2, Palm 3, Zen 4-5, Lotus 6 (2-2-1-1 by count, with its own straights).
    "2-1-2-1": {"A": frozenset({1, 2}), "B": frozenset({3}), "C": frozenset({4, 5}), "D": frozenset({6})},
}
SYMBOLS = {
    "3-2-1": {"A": "Sword", "B": "Shield", "C": "Coin"},
    "2-1-2-1": {"A": "Fist", "B": "Palm", "C": "Zen", "D": "Lotus"},
}

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

# The 3-2-1 templates, each with the flex costs its heroes use most. Straights
# are one tiered ability or two; that only changes the board's layout, not play.
TEMPLATES_321 = {
    "lean": [  # Gunslinger, Samurai; the Starter
        ("Ladder", ["A3", "A4", "A5"]),
        ("2 Sword + 3 Shield", ["A2+B3"]),
        ("2 Sword + 2 Coin", ["A2+C2"]),
        ("Straights", ["SS", "LS"]),
        ("4 Coin", ["C4"]),
        ("Ultimate 5 Coin", ["C5"]),
    ],
    # Lean boards that swap 2 Sword + 2 Coin for another cost.
    "lean_artificer": [
        ("Ladder", ["A3", "A4", "A5"]),
        ("2 Sword + 3 Shield", ["A2+B3"]),
        ("3 Shield + Coin", ["B3+C1"]),
        ("Straights", ["SS", "LS"]),
        ("4 Coin", ["C4"]),
        ("Ultimate 5 Coin", ["C5"]),
    ],
    "lean_treant": [
        ("Ladder", ["A3", "A4", "A5"]),
        ("2 Sword + 3 Shield", ["A2+B3"]),
        ("2 Shield + 2 Coin", ["B2+C2"]),
        ("Straights", ["SS", "LS"]),
        ("4 Coin", ["C4"]),
        ("Ultimate 5 Coin", ["C5"]),
    ],
    "lean_black_widow": [
        ("Ladder", ["A3", "A4", "A5"]),
        ("2 Sword + 3 Shield", ["A2+B3"]),
        ("Sword + 2 Shield + Coin", ["A1+B2+C1"]),
        ("Straights", ["SS", "LS"]),
        ("4 Coin", ["C4"]),
        ("Ultimate 5 Coin", ["C5"]),
    ],
    "standard": STANDARD_321,  # 18 boards; Compact is the same with one straight ability
    "wide": [  # Wolverine
        ("Ladder", ["A3", "A4", "A5"]),
        ("3 Shield + Coin", ["B3+C1"]),
        ("2 Sword + 2 Coin", ["A2+C2"]),
        ("2 Sword + 3 Shield", ["A2+B3"]),
        ("3 Sword + 2 Coin", ["A3+C2"]),
        ("Straights", ["SS", "LS"]),
        ("4 Coin", ["C4"]),
        ("Ultimate 5 Coin", ["C5"]),
    ],
    "double_ladder": [  # Barbarian
        ("Ladder", ["A3", "A4", "A5"]),
        ("Shield ladder", ["B3", "B4", "B5"]),
        ("2 Sword + 2 Coin", ["A2+C2"]),
        ("3 Sword + 2 Coin", ["A3+C2"]),
        ("Straights", ["SS", "LS"]),
        ("4 Coin", ["C4"]),
        ("Ultimate 5 Coin", ["C5"]),
    ],
    "combo": [  # Alchemist: no ladder
        ("Full house", ["A3+B2"]),
        ("3 Shield", ["B3"]),
        ("4 Sword + Coin", ["A4+C1"]),
        ("Sword + 3 Coin", ["A1+C3"]),
        ("Straights", ["SS", "LS"]),
        ("4 Coin", ["C4"]),
        ("Ultimate 5 Coin", ["C5"]),
    ],
}

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


def label(tier, symbols):
    if tier in ("SS", "LS"):
        return tier
    return " + ".join(f"{p[1:]} {symbols.get(p[0], p[0])}" for p in tier.split("+"))


def tiers_of(board, faces):
    out = []
    for name, tiers in board:
        for t in tiers:
            out.append((name, t, predicate(t, faces)))
    return out


def parse_costs(text):
    """"A3/A4/A5, A2+B3, SS/LS" -> one ability per comma, tiers split by /."""
    return [(part.strip(), part.strip().split("/")) for part in text.split(",") if part.strip()]


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


def drift(a, b):
    """Share of turns that end on a different tier (total variation distance)."""
    keys = set(a) | set(b)
    return sum(abs(a[k] - b[k]) for k in keys) / 2


def whole_numbers(tiers, odds, damage, target, used, tolerance=0.02):
    """Play every monotone floor/ceiling rounding and keep the closest match."""
    order = sorted(range(len(damage)), key=lambda i: -odds[i])  # easiest first
    combos = []

    def walk(k, floor, cur):
        if k == len(order):
            combos.append(list(cur))
            return
        i = order[k]
        for v in sorted({int(damage[i]), int(damage[i]) + 1} if damage[i] % 1 else {int(damage[i])}):
            if v >= max(floor, 1):  # a harder tier never pays less than an easier one
                cur[i] = v
                walk(k + 1, v, cur)

    walk(0, 0, [0] * len(damage))
    best = None
    for c in combos:
        ev, u = solve(tiers, c)
        near = abs(ev - target) <= tolerance * target
        key = (not near, drift(u, used) if near else abs(ev - target))
        if best is None or key < best[0]:
            best = (key, c, ev, u)
    return best[1], best[2], best[3], len(combos)


def main():
    ap = argparse.ArgumentParser()
    ap.add_argument("--die", choices=list(DICE), default="3-2-1")
    ap.add_argument("--board", choices=["starter"] + list(TEMPLATES_321), default="standard")
    ap.add_argument("--costs", help='a custom board, e.g. "A3/A4/A5, A2+B3, SS/LS, C4, C5"')
    ap.add_argument("--printed", help='the custom board\'s current values in the same layout, e.g. "4/5/6, 7, 6/8, 10, 12"')
    ap.add_argument("--target", type=float, default=6.2, help="average damage per turn to scale to")
    ap.add_argument("--alpha", type=float, default=0.2)
    ap.add_argument("--no-marginal", action="store_true", help="skip the slower without-this-tier runs")
    args = ap.parse_args()

    faces = DICE[args.die]
    symbols = SYMBOLS.get(args.die, {})
    printed = None
    if args.costs:
        board, args.board = parse_costs(args.costs), "custom"
        if args.printed:
            printed = [float(v) for _, vs in parse_costs(args.printed) for v in vs]
            if len(printed) != sum(len(t) for _, t in board):
                sys.exit("--printed needs one value per tier in --costs")
    elif args.board == "starter":
        board = [(n, t) for n, t, _ in STARTER]
        printed = [float(d) for _, _, ds in STARTER for d in ds]
    else:
        board = TEMPLATES_321[args.board]
    tiers = tiers_of(board, faces)

    odds = [chance(ok) for _, _, ok in tiers]
    if not all(odds):
        sys.exit("a tier can never be rolled: " + ", ".join(t for (_, t, _), p in zip(tiers, odds) if not p))
    relative = [(1 / p) ** args.alpha for p in odds]
    base, used = solve(tiers, relative)
    scale = args.target / base  # the same best play, so the average scales too
    damage = [r * scale for r in relative]
    whole, whole_ev, whole_used, tried = whole_numbers(tiers, odds, damage, args.target, used)

    print(f"die={args.die}  board={args.board}  alpha={args.alpha:g}  target={args.target:g}")
    print(f"sum of use x relative payoff = {base:.3f}, so multiplier = {args.target:g} / {base:.3f} = {scale:.2f} "
          f"(what a guaranteed hit is worth here)")
    print(f"exact damage: average {args.target:.2f}, no hit {used[None]:.1%}")
    print(f"whole damage: average {whole_ev:.2f}, no hit {whole_used[None]:.1%}, "
          f"{drift(whole_used, used):.0%} of turns end on a different tier ({tried} roundings tried)")
    if printed:
        printed_ev, printed_used = solve(tiers, printed)
        print(f"printed now:  average {printed_ev:.2f}, no hit {printed_used[None]:.1%}")
    head = f"{'ability':24s} {'tier':22s} {'chase':>6s} {'relative':>8s} {'used':>6s} {'damage':>6s} {'whole':>5s} {'used':>6s}"
    head += f" {'worth':>5s}" if not args.no_marginal else ""
    head += f" {'printed':>7s} {'used':>6s}" if printed else ""
    print(head)
    for i, (name, t, _) in enumerate(tiers):
        print(f"{name:24s} {label(t, symbols):22s} {odds[i]:6.1%} {relative[i]:8.3f} {used[i]:6.1%} "
              f"{damage[i]:6.1f} {whole[i]:5d} {whole_used[i]:6.1%}", end="")
        if not args.no_marginal:
            removed = list(damage)
            removed[i] = 0.0
            print(f" {args.target - solve(tiers, removed)[0]:5.2f}", end="")
        if printed:
            print(f" {printed[i]:7g} {printed_used[i]:6.1%}", end="")
        print()


if __name__ == "__main__":
    main()
