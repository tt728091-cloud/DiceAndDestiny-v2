"""Group Dice Throne heroes by die layout and normalize board activation costs.

Reads ../dice-throne-reference.csv and prints JSON with, per hero, the die
layout (symbol slots A, B, C, D in face order) and each board ability's
activation pattern written in those slots, with the chance to hit the cheapest
tier within 3 roll attempts of 5 dice while holding dice optimally for it.
"""
import csv
import functools
import itertools
import json
import os
import re
import sys
from collections import Counter

HERE = os.path.dirname(os.path.abspath(__file__))
CSV = os.path.join(HERE, "..", "dice-throne-reference.csv")
LETTERS = "ABCD"


def norm(name):
    return re.sub(r"[^a-z]", "", name.lower())


def parse_faces(text):
    text = text.strip()
    if "-" in text:
        lo, hi = text.split("-")
        return list(range(int(lo), int(hi) + 1))
    return [int(x) for x in re.split(r"[;,]\s*", text) if x]


def load():
    rows = list(csv.DictReader(open(CSV, encoding="utf-8")))
    heroes = {}
    for r in rows:
        h = heroes.setdefault(r["character"], {"faces": {}, "abilities": []})
        if r["record_type"] in ("Character Die", "Die Face") and r["die_face"]:
            for f in parse_faces(r["die_face"]):
                h["faces"][f] = r["name"]
        elif r["record_type"] in ("Hero Board Ability", "Ability"):
            h["abilities"].append(r)
    return heroes


def layout(faces):
    """Symbols in face order -> slot letters and the layout name like 3-2-1."""
    order = []
    for f in range(1, 7):
        if faces[f] not in order:
            order.append(faces[f])
    slots = {norm(s): LETTERS[i] for i, s in enumerate(order)}
    sizes = [sum(1 for f in range(1, 7) if faces[f] == s) for s in order]
    face_sets = {LETTERS[i]: frozenset(f for f in range(1, 7) if faces[f] == s) for i, s in enumerate(order)}
    return "-".join(map(str, sizes)), slots, face_sets, order


def parse_tier(tier, slots):
    tier = tier.strip()
    low = tier.lower()
    if "straight" in low:
        return {"Small Straight": "SS", "Large Straight": "LS", "Largest Straight": "XLS"}[tier.title()]
    parts = {}
    for part in tier.split("+"):
        part = part.strip()
        m = re.fullmatch(r"\[([^\]]+)\]\s*(?:×\s*(\d+))?", part)
        if m:
            sym, n = m.group(1), int(m.group(2) or 1)
        else:
            m = re.fullmatch(r"(\d+)?\s*([A-Za-z][A-Za-z ]*)", part)
            sym, n = m.group(2), int(m.group(1) or 1)
        parts[slots[norm(sym)]] = parts.get(slots[norm(sym)], 0) + n
    return "+".join(f"{k}{parts[k]}" for k in sorted(parts))


def parse_activation(req, slots):
    req = req.strip()
    m = re.fullmatch(r"(\d+(?:/\d+)+)\s+(\w+)", req)  # Loki "3/4/5 Scepter"
    if m:
        letter = slots[norm(m.group(2))]
        return [f"{letter}{n}" for n in m.group(1).split("/")]
    return [parse_tier(t, slots) for t in req.split(" / ")]


def predicate(tier, face_sets):
    if tier == "SS":
        return lambda d: any({s, s + 1, s + 2, s + 3} <= set(d) for s in (1, 2, 3))
    if tier == "XLS":  # Storm: six dice in sequence; impossible on five dice alone
        return lambda d: False
    if tier == "LS":
        return lambda d: {1, 2, 3, 4, 5} <= set(d) or {2, 3, 4, 5, 6} <= set(d)
    need = [(face_sets[p[0]], int(p[1:])) for p in tier.split("+")]
    return lambda d: all(sum(1 for x in d if x in fs) >= n for fs, n in need)


@functools.lru_cache(None)
def roll_dist(n):
    dist = Counter()
    for r in itertools.product(range(1, 7), repeat=n):
        dist[tuple(sorted(r))] += 1
    total = 6 ** n
    return [(k, v / total) for k, v in dist.items()]


def chance(pred, rolls=3, dice=5):
    states = list(roll_dist(dice))
    states = [s for s, _ in states]
    value = {s: 1.0 if pred(s) else 0.0 for s in states}
    for _ in range(rolls - 1):
        nxt = {}
        for s in states:
            if value[s] == 1.0:
                nxt[s] = 1.0
                continue
            best = 0.0
            keeps = set()
            for mask in range(1 << dice):
                keeps.add(tuple(s[i] for i in range(dice) if mask >> i & 1))
            for keep in keeps:
                p = 0.0
                for out, w in roll_dist(dice - len(keep)):
                    p += w * value[tuple(sorted(keep + out))]
                best = max(best, p)
            nxt[s] = best
        value = nxt
    return sum(w * value[s] for s, w in roll_dist(dice))


def main():
    heroes = load()
    cache = {}
    out = []
    for name, h in heroes.items():
        lay, slots, face_sets, order = layout(h["faces"])
        face_key = tuple(sorted((k, tuple(sorted(v))) for k, v in face_sets.items()))
        seen_forms = set()
        abilities = []
        for r in h["abilities"]:
            kind = r["record_subtype"].split()[0]
            req = r["activation_requirement"]
            if kind == "Passive":
                continue
            if kind == "Defensive":
                m = re.search(r"(\d)(?:–(\d))?", req)
                pattern = f"DEF {m.group(0)}d"
                tiers = []
            else:
                tiers = parse_activation(req, slots)
                pattern = " / ".join(tiers)
            key = (kind, pattern)
            form = r["character_form"]
            entry = {"kind": kind, "name": r["name"], "pattern": pattern, "form": form, "raw": req}
            if tiers:
                ck = (face_key, tiers[0])
                if ck not in cache:
                    cache[ck] = chance(predicate(tiers[0], face_sets))
                entry["p_min_tier"] = round(cache[ck], 4)
            abilities.append(entry)
        out.append({"hero": name, "layout": lay, "symbols": dict(zip(LETTERS, order)), "abilities": abilities})
        print(name, file=sys.stderr)
    json.dump(out, sys.stdout, indent=1, ensure_ascii=False)


if __name__ == "__main__":
    main()
