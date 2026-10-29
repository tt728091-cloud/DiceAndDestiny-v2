"""Card value report: each hero with its deck against the same hero never
playing a card (balance-lab -block-cards=all), paired battle by battle (seed
and seat), and the Starter with one card type blocked at a time.

    python3 ml/scripts/card_value_report.py RESULTS_DIR [hero ...]

RESULTS_DIR holds balance-lab -out files: <hero>.json with the players
lookahead@rounds=1@budget=400 and greedy, each also with @block-cards=all,
and each-<card>.json with lookahead@rounds=1@budget=400@block-cards=<card>
(paired against starter.json). See docs/game-design/balance-lab.md, "Cards
on top of the board"."""
import json
import math
import sys
from pathlib import Path

HERE = Path(sys.argv[1]) if len(sys.argv) > 1 else Path("ml/runs/cards")
BOARDS = {
    "starter": (["adventurer_strike", "guarded_strike", "measured_strike", "adventurer_small_straight", "adventurer_large_straight", "decisive_blow"], ["adventurer_guard"]),
    "curse": (["hexbrand", "grasp_of_the_sarcophagus", "funeral_rattle", "eclipse_of_the_black_star"], ["hexward_rebuttal", "misfortune_repaid"]),
    "venom": (["needlefang", "venom_gland", "fever_spike", "terminal_bite"], ["shedskin", "barbed_mantle"]),
    "blade_warden": (["sword_cut", "shield_bash", "golden_edge", "perfect_form", "venom_strike"], ["basic_defense", "protect"]),
}


def battle(r, hero):
    """One battle's numbers per offensive turn."""
    offense, defense = BOARDS[hero]
    dealt, taken = r.get("dealt") or {}, r.get("taken") or {}
    ht, ft = max(1, r["hero_turns"]), max(1, r["foe_turns"])
    attack = sum(v["base"] for k, v in dealt.items() if k in offense)
    other = sum(v["base"] for k, v in dealt.items() if k not in offense)
    hits = sum(v["hits"] for k, v in dealt.items() if k in offense)
    return {
        "score": {"win": 1.0, "draw": 0.5}.get(r["result"], 0.0),
        "rounds": r["rounds"],
        "turns": r["hero_turns"],
        "attack": attack / ht,  # the board's attacks before defense (with card bonuses)
        "hit": hits / ht,
        "other": other / ht,  # statuses, retaliation, card damage
        "dealt": sum(v["final"] for v in dealt.values()) / ht,  # everything that landed
        "incoming": sum(v["base"] for v in taken.values()) / ft,
        "rolled": sum(v["prevention"] for v in taken.values()) / ft,
        "reaction": sum(v["reaction_prevention"] for v in taken.values()) / ft,
        "taken": sum(v["final"] for v in taken.values()) / ft,
        "net": (sum(v["final"] for v in dealt.values()) - sum(v["final"] for v in taken.values())) / max(1, r["rounds"]),
    }


def players(path):
    out = {}
    for s in json.loads(Path(path).read_text()):
        if "(seat" in s["player"]:
            continue
        out[s["player"]] = s
    return out


def keyed(summary, hero):
    return {(r["seed"], r["hero_seat"]): battle(r, hero) for r in summary["records"] if r["result"] != "error"}


def mean(xs):
    return sum(xs) / len(xs)


def paired(a, b, field):
    keys = sorted(set(a) & set(b))
    d = [a[k][field] - b[k][field] for k in keys]
    m = mean(d)
    sd = math.sqrt(sum((x - m) ** 2 for x in d) / max(1, len(d) - 1))
    return m, sd / math.sqrt(len(d)), len(d)


def fmt(m, se):
    return f"{m:+.2f} ±{1.96 * se:.2f}"


FIELDS = [("score", "win score"), ("attack", "attack before defense / turn"), ("hit", "attacks landed / turn"), ("other", "other damage / turn"),
          ("dealt", "damage landed / turn"), ("incoming", "incoming before defense / foe turn"), ("rolled", "prevented by defense roll / foe turn"),
          ("reaction", "prevented by cards, Protect / foe turn"), ("taken", "damage taken / foe turn"), ("net", "net health swing / round")]


def report(hero):
    path = HERE / f"{hero}.json"
    if not path.exists():
        return
    ps = players(path)
    print(f"\n## {hero} mirror (greedy foe with its deck)")
    for base in ["lookahead@rounds=1@budget=400", "greedy"]:
        deck, none = ps.get(base), ps.get(base + "@block-cards=all")
        if not deck or not none:
            continue
        a, b = keyed(deck, hero), keyed(none, hero)
        print(f"\n{base}: {len(a)} battles with the deck, {len(b)} without cards; errors {deck.get('errors', 0)}/{none.get('errors', 0)}")
        print(f"{'':42s} {'deck':>7s} {'no cards':>9s} {'cards add (95%)':>16s}")
        for f, label in FIELDS:
            m, se, _ = paired(a, b, f)
            print(f"{label:42s} {mean([v[f] for v in a.values()]):7.2f} {mean([v[f] for v in b.values()]):9.2f} {fmt(m, se):>16s}")
        print(f"{'rounds':42s} {mean([v['rounds'] for v in a.values()]):7.2f} {mean([v['rounds'] for v in b.values()]):9.2f}")
        n = len(a)
        cards = deck["cards_played"]
        print("cards per battle:", ", ".join(f"{k} {v / n:.2f}" for k, v in sorted(cards.items(), key=lambda kv: -kv[1])))


def each():
    paths = sorted(HERE.glob("each-*.json"))
    if not paths or not (HERE / "starter.json").exists():
        return
    full = players(HERE / "starter.json")["lookahead@rounds=1@budget=400"]
    a = keyed(full, "starter")
    n = len(a)
    turns = mean([v["turns"] for v in a.values()])
    rounds = mean([v["rounds"] for v in a.values()])
    print("\n## Starter, one card type blocked at a time (the search; what the card adds)")
    print(f"{'card':14s} {'plays/battle':>12s} {'win score':>14s} {'attack/turn':>14s} {'landed/turn':>14s} {'taken/foe turn':>15s} {'net/round':>14s} {'health per play':>16s}")
    for name, s in (item for path in paths for item in players(path).items()):
        card = name.split("block-cards=")[1]
        b = keyed(s, "starter")
        plays = full["cards_played"].get(card, 0) / n
        row = {f: paired(a, b, f) for f in ["score", "attack", "dealt", "taken", "net"]}
        per_play = row["net"][0] * rounds / plays if plays else float("nan")
        per_play_se = row["net"][1] * rounds / plays if plays else float("nan")
        print(f"{card:14s} {plays:12.2f} {fmt(*row['score'][:2]):>14s} {fmt(*row['attack'][:2]):>14s} {fmt(*row['dealt'][:2]):>14s} {fmt(-row['taken'][0], row['taken'][1]):>15s} {fmt(*row['net'][:2]):>14s} {fmt(per_play, per_play_se):>16s}")
    print(f"(taken/foe turn shown as damage the card saves; {turns:.2f} turns and {rounds:.2f} rounds per battle with the full deck)")


for hero in sys.argv[2:] or BOARDS:
    report(hero)
each()
