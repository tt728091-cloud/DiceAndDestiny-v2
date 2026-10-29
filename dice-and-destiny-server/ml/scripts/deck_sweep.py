"""Plays deck variants through balance-lab, one run per variant, and records
each variant's deck in a manifest for card_exchange_report.py. Variants
already on disk are skipped, so an interrupted sweep resumes where it
stopped. See docs/game-design/balancing-playbook.md.

    python3 ml/scripts/deck_sweep.py PLAN.json

PLAN (paths are relative to the plan file, except "lab"):
    {
      "player": "greedy",                  # or lookahead@rounds=1@budget=400
      "battles": 1000,                     # seeds per variant; both seats play each
      "first_seed": 1,
      "workers": 15,
      "out": "greedy",                     # results directory
      "hero": "starter",                   # optional
      "opponent": "starter", "foe": "greedy",          # optional; minions use "foe": "script"
      "foe_deck": {"card": count},         # optional
      "authored_root": "../energy/authored",           # optional scratch published cards
      "lab": "ml/runs/bin/balance-lab",    # optional, relative to the server root
      "variants": {"std+0": {"steady_guard": 3, "nudge": 2, ...}, ...}
    }

Build the lab first: go build -o ml/runs/bin/balance-lab ./cmd/balance-lab
"""
import json
import subprocess
import sys
import time
from pathlib import Path

SERVER = Path(__file__).resolve().parents[2]


def deck_spec(deck):
    return ",".join(f"{card}:{n}" for card, n in deck.items() if n > 0)


def main(plan_path):
    plan_path = Path(plan_path).resolve()
    plan = json.loads(plan_path.read_text())
    base = plan_path.parent
    lab = SERVER / plan.get("lab", "ml/runs/bin/balance-lab")
    if not lab.exists():
        sys.exit(f"{lab} is missing: go build -o {lab.relative_to(SERVER)} ./cmd/balance-lab")
    out = base / plan["out"]
    out.mkdir(parents=True, exist_ok=True)
    manifest_path = out / "manifest.json"
    manifest = json.loads(manifest_path.read_text()) if manifest_path.exists() else {}
    for name, deck in plan["variants"].items():
        target = out / f"{name}.json"
        manifest[name] = {"deck": deck, "player": plan["player"], "battles": plan["battles"], "first_seed": plan.get("first_seed", 1),
                          "hero": plan.get("hero", "starter"), "opponent": plan.get("opponent", "starter"), "foe": plan.get("foe", "greedy"),
                          "foe_deck": plan.get("foe_deck"), "authored_root": plan.get("authored_root")}
        manifest_path.write_text(json.dumps(manifest, indent=1))
        if target.exists() and target.stat().st_size > 0:
            continue
        cmd = [str(lab), "-hero", plan.get("hero", "starter"), "-opponent", plan.get("opponent", "starter"), "-foe", plan.get("foe", "greedy"),
               "-players", plan["player"], "-hero-deck", deck_spec(deck), "-seats", "both", "-battles", str(plan["battles"]),
               "-first-seed", str(plan.get("first_seed", 1)), "-workers", str(plan.get("workers", 15)), "-out", str(target)]
        if plan.get("authored_root"):
            cmd += ["-authored-root", str((base / plan["authored_root"]).resolve())]
        if plan.get("foe_deck"):
            cmd += ["-foe-deck", deck_spec(plan["foe_deck"])]
        started = time.time()
        with open(out / f"{name}.txt", "w") as txt, open(out / f"{name}.err", "w") as err:
            code = subprocess.call(cmd, cwd=SERVER, stdout=txt, stderr=err)
        if code != 0 and target.exists():
            target.unlink()
        print(f"{name}: exit {code} in {time.time() - started:.0f}s", flush=True)
    print("sweep done", flush=True)


if __name__ == "__main__":
    if len(sys.argv) != 2:
        sys.exit(__doc__)
    main(sys.argv[1])
