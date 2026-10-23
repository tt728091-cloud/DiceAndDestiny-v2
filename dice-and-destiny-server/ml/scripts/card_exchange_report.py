"""Card exchange rates against blank cards (pure health).

Each variant replaces every copy of one card in the Starter's deck with m
blank cards (Trailblaze: a map card, never playable in battle, still
health). The card is worth as many blanks as it takes to score what the
standard deck scores: the crossing m*, from a line fitted to the variants
around it. Per copy, a card with c copies is worth m*/c blanks.

    python3 ml/scripts/card_exchange_report.py RESULTS_DIR [--rates=DIR] [--metric=health]
    python3 ml/scripts/card_exchange_report.py COSTS_DIR --costs=DIR [--now=card:energy,...]

With --costs=DIR, variants named "<card>@<energy>" (the card swapped for a
copy at another energy cost) are valued in blanks per copy against DIR's
rates and blank curves.

--metric=health compares the hero's health left at the end (for lopsided
fights such as minions, where wounds carry over) instead of the win score.

RESULTS_DIR holds balance-lab -out files, one per variant, and
manifest.json (variant name -> {"deck": {card: count}}), as written by
ml/runs/blanks/sweep.py. The standard deck is the variant "std+0"; a
20-card deck (eight blanks) is "big+8", with its swaps named "big-<card>-
to-<m>". With --rates=DIR, decks named "x-…" (swaps between cards) and
"plus-…" (extra copies) are predicted from DIR's rates and blank curve.
"""
import json
import math
import sys
from pathlib import Path

import numpy as np

STD = {"steady_guard": 3, "nudge": 2, "try_again": 2, "strong_swing": 2, "take_stock": 2, "second_wind": 1}
PRICE = {"steady_guard": 15, "nudge": 15, "try_again": 13, "strong_swing": 15, "take_stock": 15, "second_wind": 15}
BLANK = "trailblaze"


def load(root):
    manifest = json.loads((root / "manifest.json").read_text())
    out = {}
    for name, meta in manifest.items():
        path = root / f"{name}.json"
        if not path.exists() or path.stat().st_size == 0:
            continue
        summary = [s for s in json.loads(path.read_text()) if "(seat" not in s["player"]][0]
        records = [r for r in summary["records"] if r["result"] != "error"]
        if METRIC == "health":
            scores = np.array([float(r["hero_health"]) for r in records])
        else:
            scores = np.array([{"win": 1.0, "draw": 0.5}.get(r["result"], 0.0) for r in records])
        out[name] = {"deck": meta["deck"], "n": len(scores), "score": scores.mean(), "se": scores.std(ddof=1) / math.sqrt(len(scores)),
                     "rounds": float(np.mean([r["rounds"] for r in records])), "cards": summary["cards_played"],
                     "by": {(r["seed"], r["hero_seat"]): x for r, x in zip(records, scores)}}
    return out


def crossing(points, target, target_se, window=4):
    """m where a line through the points nearest the target crosses it,
    with its standard error (delta method)."""
    points = sorted(points)
    near = sorted(points, key=lambda p: abs(p[1] - target))[:window]
    near.sort()
    m = np.array([p[0] for p in near], float)
    y = np.array([p[1] for p in near])
    w = 1 / np.array([p[2] for p in near]) ** 2
    X = np.vstack([np.ones_like(m), m]).T
    cov = np.linalg.inv(X.T @ (X * w[:, None]))
    a, b = cov @ (X.T @ (w * y))
    if b <= 0:
        return float("nan"), float("nan"), b
    mstar = (target - a) / b
    # d m*/d(a, b, target)
    g = np.array([-1 / b, -(target - a) / b**2])
    var = g @ cov @ g + (target_se / b) ** 2
    return mstar, math.sqrt(var), b


def rates(data, base, prefix=""):
    """Each card's crossing against the base variant: {card: (blanks for
    all copies, se, copies)}."""
    out = {}
    ref = data[base]
    extra = ref["deck"].get(BLANK, 0)
    for card, c in STD.items():
        pts = [(v["deck"].get(BLANK, 0) - extra, v["score"], v["se"]) for k, v in data.items() if k.startswith(f"{prefix}{card}-to-")]
        if len(pts) < 2:
            continue
        mstar, se, _ = crossing(pts, ref["score"], ref["se"])
        out[card] = (mstar, se, c, sorted(pts))
    return out


def print_rates(data, base, prefix=""):
    ref = data[base]
    print(f"{base}: score {ref['score']:.3f} ±{1.96 * ref['se']:.3f} ({ref['n']} battles, {ref['rounds']:.2f} rounds)")
    print(f"{'card':14s} {'copies':>6s} {'blanks for all copies (95%)':>28s} {'blanks per copy':>16s} {'XP at 10 per blank':>19s} {'XP now':>7s}  scores by blanks")
    found = rates(data, base, prefix)
    for card, (mstar, se, c, pts) in found.items():
        line = ", ".join(f"{m}: {s:.3f}" for m, s, _ in pts)
        print(f"{card:14s} {c:6d} {mstar:12.2f} ±{1.96 * se:4.2f}{'':10s} {mstar / c:16.2f} {10 * mstar / c:19.1f} {PRICE[card]:7d}  {line}")
    return found


def curve(data, prefix="std+"):
    """Score against blanks added to the standard deck."""
    pts = sorted((v["deck"].get(BLANK, 0), v["score"]) for k, v in data.items() if k.startswith(prefix))
    return np.array([p[0] for p in pts], float), np.array([p[1] for p in pts])


def predictions(data, found, xs, ys):
    """Score each deck by its blank equivalents (cards at their measured
    rates; Steady Guard and Take Stock at 1 when not measured), read off
    the standard deck's blank curve, against what it scored."""
    per_copy = {card: (m / c) for card, (m, se, c, _) in found.items()}
    def value(deck):
        return sum(n * per_copy.get(card, 1.0) for card, n in deck.items() if card != BLANK) + deck.get(BLANK, 0)
    std_value = value(dict(STD))
    slope0 = (ys[1] - ys[0]) / (xs[1] - xs[0])
    print(f"{'deck':28s} {'blank equivalents vs standard':>30s} {'predicted':>10s} {'scored (95%)':>15s} {'XP now vs standard':>19s}")
    for name, v in sorted(data.items()):
        if not (name.startswith("x-") or name.startswith("plus-")):
            continue
        dv = value(v["deck"]) - std_value
        pred = np.interp(dv, xs, ys) if dv >= 0 else ys[0] + slope0 * dv
        xp = sum(n * PRICE.get(card, 0) for card, n in v["deck"].items() if card != BLANK) + 10 * v["deck"].get(BLANK, 0) - sum(n * PRICE[c] for c, n in STD.items())
        print(f"{name:28s} {dv:+30.2f} {pred:10.3f} {v['score']:9.3f} ±{1.96 * v['se']:.3f} {xp:+19d}")


def budget(data, found):
    """Decks named "old-…" (equal price today) and "new-…" (equal value at
    the measured rates), each valued in blank cards and scored against the
    all-blank decks ("blank-N")."""
    per_copy = {card: (m / c) for card, (m, se, c, _) in found.items()}
    blank = sorted((v["deck"].get(BLANK, 0), v["score"]) for k, v in data.items() if k.startswith("blank-") or k == "new-B4-blank15" or k == "new-B5-blank16")
    if blank:
        print("all-blank decks: " + ", ".join(f"{n}: {s:.3f}" for n, s in blank))
    xs, ys = np.array([b[0] for b in blank], float), np.array([b[1] for b in blank])
    print(f"{'deck':24s} {'cards':>5s} {'XP now':>6s} {'value in blanks':>15s} {'plays like':>10s} {'scored (95%)':>15s}")
    for name, v in sorted(data.items()):
        if not (name.startswith("old-") or name.startswith("new-") or name in ("std+0",)):
            continue
        deck = v["deck"]
        value = sum(n * per_copy.get(card, 1.0) for card, n in deck.items())
        xp = sum(n * PRICE.get(card, 10) for card, n in deck.items())
        pred = np.interp(value, xs, ys) if len(xs) > 1 else float("nan")
        print(f"{name:24s} {sum(deck.values()):5d} {xp:6d} {value:15.2f} {pred:10.3f} {v['score']:9.3f} ±{1.96 * v['se']:.3f}")


ENERGY = {"nudge": 7, "try_again": 3, "strong_swing": 10, "steady_guard": 5, "take_stock": 5, "second_wind": 10}


def cost_report(root, rates_from):
    """Variants named "<card>@<energy>" swap a card for a copy at another
    energy cost (a scratch copy of the published cards). Each cost's value
    is the card's value at its own cost (rates_from) plus the score change
    against the copy at the current cost, paired battle by battle, in blanks
    (the card's own blank slope); a cost with no control is read straight
    off the card's blank curve."""
    data = load(root)
    base = load(rates_from)
    found = rates(base, "std+0")
    if "std" in data:
        print(f"standard deck on these seeds: {data['std']['score']:.3f} ±{1.96 * data['std']['se']:.3f} ({data['std']['n']} battles)")
    for card, c in STD.items():
        points = sorted((int(k.split("@")[1]), v) for k, v in data.items() if k.startswith(card + "@"))
        if not points or card not in found:
            continue
        mstar, se, _, pts = found[card]
        m = np.array([p[0] for p in pts], float)
        slope = np.polyfit(m, [p[1] for p in pts], 1)[0]
        control = data.get(f"{card}@{ENERGY[card]}")
        print(f"\n{card} ({c} copies; now {ENERGY[card]} energy, {mstar / c:.2f} blanks per copy; one blank = {slope:.3f} score)")
        print(f"{'energy':>6s} {'score (95%)':>16s} {'vs current cost':>16s} {'change per copy':>16s} {'blanks per copy (95%)':>22s} {'XP at 10 per blank':>19s} {'plays per battle':>17s}")
        for e, v in points:
            plays = sum(v["cards"].values()) and v["cards"].get(f"{card}_e{e}", 0) / v["n"]
            if control:
                keys = sorted(set(v["by"]) & set(control["by"]))
                d = np.array([v["by"][k] - control["by"][k] for k in keys])
                diff = d.mean()
                dse = d.std(ddof=1) / math.sqrt(len(d)) if v is not control else 0.0
                per = mstar / c + diff / slope / c
                pse = math.hypot(se / c, dse / slope / c)
            else:
                cross, cse, _ = crossing([(p[0], p[1], p[2]) for p in pts], v["score"], v["se"])
                diff, dse, per, pse = float("nan"), float("nan"), cross / c, cse / c
            change, cse2 = diff / slope / c, dse / slope / c
            print(f"{e:6d} {v['score']:9.3f} ±{1.96 * v['se']:.3f} {diff:+10.3f} ±{1.96 * dse:.3f} {change:+10.2f} ±{1.96 * cse2:.2f} {per:14.2f} ±{1.96 * pse:.2f} {10 * per:19.1f} {plays:17.2f}")


def combo_report(data):
    """Decks named "pair-…" or "combo-…" change several costs at once: their
    change against the standard deck ("std"), paired battle by battle,
    against the sum of the single-card changes ("<card>@<energy>")."""
    std = data.get("std")
    if not std:
        return
    def paired(v, ref):
        keys = sorted(set(v["by"]) & set(ref["by"]))
        d = np.array([v["by"][k] - ref["by"][k] for k in keys])
        return d.mean(), d.std(ddof=1) / math.sqrt(len(d))
    rows = [(k, v) for k, v in sorted(data.items()) if k.startswith("pair-") or k.startswith("combo-")]
    if not rows:
        return
    print(f"\n{'deck':24s} {'changed costs':34s} {'sum of single changes':>22s} {'measured change (95%)':>22s}")
    for name, v in rows:
        changes, predicted, missing = [], 0.0, False
        for cid in v["deck"]:
            if "_e" in cid:
                card, e = cid.rsplit("_e", 1)
                if card not in ENERGY or int(e) == ENERGY[card]:
                    continue
                changes.append(f"{card} {ENERGY[card]}→{e}")
                single = data.get(f"{card}@{e}")
                if single is None:
                    missing = True
                    continue
                predicted += paired(single, std)[0]
        m, se = paired(v, std)
        pred = "n/a" if missing else f"{predicted:+.3f}"
        print(f"{name:24s} {', '.join(changes):34s} {pred:>22s} {m:+15.3f} ±{1.96 * se:.3f}")


def wind_report(data, found):
    """Second Wind variants ("wind-pay<e>-gain<g>"): their change against
    the standard deck, paired battle by battle, in blanks."""
    std = data.get("std")
    rows = [(k, v) for k, v in sorted(data.items()) if k.startswith("wind-")]
    if not std or not rows or "second_wind" not in found:
        return
    mstar, se, c, pts = found["second_wind"]
    slope = np.polyfit([p[0] for p in pts], [p[1] for p in pts], 1)[0]
    print(f"\nSecond Wind now: pay 10, gain 15, {mstar / c:.2f} blanks per copy")
    print(f"{'variant':22s} {'measured change (95%)':>22s} {'blanks per copy':>16s} {'plays per battle':>17s}")
    for name, v in rows:
        keys = sorted(set(v["by"]) & set(std["by"]))
        d = np.array([v["by"][k] - std["by"][k] for k in keys])
        m, dse = d.mean(), d.std(ddof=1) / math.sqrt(len(d))
        plays = sum(n for cid, n in v["cards"].items() if cid.startswith("second_wind")) / v["n"]
        print(f"{name:22s} {m:+15.3f} ±{1.96 * dse:.3f} {mstar / c + m / slope / c:16.2f} {plays:17.2f}")


def report(root, rates_from=None):
    data = load(root)
    if "std+0" in data:
        print_rates(data, "std+0")
        xs, ys = curve(data)
        if len(xs) > 1:
            print("standard deck plus blanks: " + ", ".join(f"+{int(m)}: {s:.3f}" for m, s in zip(xs, ys)))
    if "big+8" in data:
        print()
        print_rates(data, "big+8", "big-")
        xs, ys = curve(data, "big+")
        print("20-card deck (8 blanks) plus or minus blanks: " + ", ".join(f"{int(m)}: {s:.3f}" for m, s in zip(xs, ys)))
    if rates_from is not None:
        base = load(rates_from)
        found = rates(base, "std+0")
        xs, ys = curve(base)
        print()
        predictions({**base, **data}, found, xs, ys)
        if any(k.startswith("old-") or k.startswith("blank-") for k in data):
            print()
            budget({**base, **data}, found)


METRIC = "score"
args = [a for a in sys.argv[1:] if not a.startswith("--metric=") and not a.startswith("--now=")]
for a in sys.argv[1:]:
    if a.startswith("--metric="):
        METRIC = a.split("=", 1)[1]
    if a.startswith("--now="):
        for pair in a.split("=", 1)[1].split(","):
            card, e = pair.split(":")
            ENERGY[card] = int(e)
args = args or ["ml/runs/blanks/greedy"]
if len(args) == 2 and args[1].startswith("--costs="):
    print(f"## {args[0]} (card values from {args[1][8:]})")
    cost_report(Path(args[0]), Path(args[1][8:]))
    combo_report(load(Path(args[0])))
    wind_report(load(Path(args[0])), rates(load(Path(args[1][8:])), "std+0"))
elif len(args) == 2 and args[1].startswith("--rates="):
    print(f"## {args[0]} (rates from {args[1][8:]})")
    report(Path(args[0]), Path(args[1][8:]))
else:
    for arg in args:
        print(f"## {arg}")
        report(Path(arg))
