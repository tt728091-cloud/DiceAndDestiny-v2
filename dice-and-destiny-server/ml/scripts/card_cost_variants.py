"""Writes a scratch copy of the published cards with cost variants of
existing cards under new IDs (<card>_e<energy>), for balance-lab
-authored-root. The real authored directory is only read.

    python3 ml/scripts/card_cost_variants.py OUT_DIR card:energy,energy,... [card:...]

Example: python3 ml/scripts/card_cost_variants.py ml/runs/energy/authored nudge:8,9,10 take_stock:0,1
"""
import copy
import json
import shutil
import sys
from pathlib import Path

SOURCE = Path(__file__).resolve().parents[2] / "content" / "authored"

out = Path(sys.argv[1])
out.mkdir(parents=True, exist_ok=True)
cards = json.loads((SOURCE / "authored_cards.json").read_text())
for spec in sys.argv[2:]:
    card, costs = spec.split(":")
    base = cards["cards"][card]
    for energy in (int(e) for e in costs.split(",")):
        variant = copy.deepcopy(base)
        variant["id"] = f"{card}_e{energy}"
        variant["name"] = f"{base['name']} ({energy})"
        variant["cost"] = dict(variant["cost"], energy=energy)
        cards["cards"][variant["id"]] = variant
(out / "authored_cards.json").write_text(json.dumps(cards))
shutil.copy(SOURCE / "economy_admin.json", out / "economy_admin.json")
print(f"wrote {out / 'authored_cards.json'}")
