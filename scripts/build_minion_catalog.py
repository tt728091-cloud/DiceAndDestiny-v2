#!/usr/bin/env python3
"""Build the local art catalog and lightweight Godot references from reviewed art.

This does not generate or edit images, or add combat mechanics.
"""
from pathlib import Path
import html
import json
import struct
from PIL import Image

ROOT = Path(__file__).resolve().parents[1]
CLIENT = ROOT / "dice-and-destiny-client"
DOCS = ROOT / "docs/minion-bestiary"


def build():
    bosses = json.loads((ROOT / "docs/battle-art-extraction-manifest.json").read_text())["entries"]
    factions = []
    for boss in bosses:
        manifest_path = DOCS / boss["id"] / "manifest.json"
        if not manifest_path.exists():
            continue
        manifest = json.loads(manifest_path.read_text())
        faction = {"boss_id": boss["id"], "boss_name": boss["name"],
                   "theme": manifest["theme"], "minions": []}
        for minion in manifest["minions"]:
            asset = ROOT / minion["relative_asset_path"]
            if not asset.exists() or minion["status"] != "completed":
                continue
            width, height = struct.unpack(">II", asset.read_bytes()[16:24])
            # Read alpha only; keep every generated PNG's pixels unchanged.
            # Alpha=1 canvas residue must not determine the contact point.
            with Image.open(asset) as image:
                bounds = image.getchannel("A").point(lambda a: 255 if a >= 64 else 0).getbbox()
            ground_y = bounds[3] / height
            definition_id = boss["id"] + "_" + minion["id"]
            profile = CLIENT / "content/battle_visuals/minions" / boss["id"] / (minion["id"] + ".tres")
            profile.parent.mkdir(parents=True, exist_ok=True)
            texture_path = "res://" + asset.relative_to(CLIENT).as_posix()
            profile.write_text(
                '[gd_resource type="Resource" script_class="FighterVisualProfile" load_steps=3 format=3]\n\n'
                '[ext_resource type="Script" path="res://content/battle_visuals/fighter_visual_profile.gd" id="1"]\n'
                f'[ext_resource type="Texture2D" path="{texture_path}" id="2"]\n\n'
                '[resource]\nscript = ExtResource("1")\n'
                f'definition_id = {json.dumps(definition_id)}\n'
                f'display_name = {json.dumps(minion["name"])}\n'
                'texture = ExtResource("2")\nportrait = ExtResource("2")\nfaces_left = true\n'
                f'default_height = {min(320, 420 * height / width):.1f}\n'
                f'ground_anchor = Vector2(0.5, {ground_y:.6f})\n')
            faction["minions"].append({
                "id": definition_id, "name": minion["name"], "role": minion["role"],
                "description": minion["description"], "texture": texture_path,
                "profile": "res://" + profile.relative_to(CLIENT).as_posix(),
                "width": width, "height": height,
                "ground_anchor": [0.5, round(ground_y, 6)],
            })
        factions.append(faction)
    catalog = {"version": 1, "kind": "visual_concepts", "factions": factions}
    (CLIENT / "content/battle_visuals/minion_catalog.json").write_text(json.dumps(catalog, indent=2) + "\n")
    count = sum(len(f["minions"]) for f in factions)
    cards = []
    for faction in factions:
        for m in faction["minions"]:
            asset_url = "../../dice-and-destiny-client/" + m["texture"].removeprefix("res://")
            search = " ".join([faction["boss_name"], m["name"], m["role"], m["description"]]).lower()
            cards.append(f'''<article data-boss="{html.escape(faction['boss_id'])}" data-search="{html.escape(search)}">
<a href="{html.escape(asset_url)}" target="_blank"><img loading="lazy" src="{html.escape(asset_url)}" alt="{html.escape(m['name'])}"></a>
<div class="copy"><small>{html.escape(faction['boss_name'])} · {html.escape(m['role'])}</small><h2>{html.escape(m['name'])}</h2><p>{html.escape(m['description'])}</p></div></article>''')
    options = ''.join(f'<option value="{f["boss_id"]}">{html.escape(f["boss_name"])} ({len(f["minions"])})</option>' for f in factions)
    page = '''<!doctype html><html lang="en"><meta charset="utf-8"><meta name="viewport" content="width=device-width,initial-scale=1">
<title>Dice &amp; Destiny · Minion bestiary</title><style>
:root{color-scheme:dark;font-family:Georgia,serif;background:#111412;color:#e6decc}*{box-sizing:border-box}body{margin:0}header{padding:36px 4vw 22px;background:#1a201c;border-bottom:1px solid #5b5b43}h1{font-weight:normal;margin:0 0 12px;font-size:38px}header p{max-width:850px;line-height:1.5;color:#bcbda9}.tools{display:flex;gap:12px;flex-wrap:wrap;align-items:center;margin-top:22px}select,input{font:16px system-ui;background:#111712;color:#e6decc;border:1px solid #73765b;border-radius:5px;padding:10px}input{min-width:260px}#count{font:14px system-ui;color:#bebf9e}main{padding:28px 4vw;display:grid;grid-template-columns:repeat(auto-fill,minmax(270px,1fr));gap:22px}article{border:1px solid #414b3c;border-radius:8px;overflow:hidden;background:#1c211e}article[hidden]{display:none}article a{display:block;background:radial-gradient(ellipse at center,#535649,#272e29)}img{display:block;width:100%;height:290px;object-fit:contain;padding:12px}.copy{padding:18px}small{font:12px system-ui;color:#b7bb8e}h2{font-size:23px;font-weight:normal;margin:10px 0}p{font-size:15px;line-height:1.5;margin:0;color:#c7c9ba}body.light article a{background:#dedccf}button{padding:10px;background:#48523b;color:white;border:1px solid #7b8468;border-radius:5px;cursor:pointer}footer{padding:20px 4vw;color:#b5b99f}a{color:#d5ddb3}
</style><header><h1>Minion bestiary</h1><p>200 distinct creatures across 20 boss factions. Each is a separate transparent sprite with its own visual profile. Roles describe design intent; combat abilities and balance remain to be defined.</p><div class="tools"><label>Faction <select id="boss"><option value="">All factions</option>OPTIONS</select></label><input id="search" type="search" placeholder="Search name, role, anatomy…" aria-label="Search creatures"><button id="backdrop">Light / dark backdrop</button><span id="count"></span></div></header><main>CARDS</main><footer>Click any creature to open its full-resolution PNG. <a href="README.md">Production notes</a></footer><script>
const cards=[...document.querySelectorAll('article')], boss=document.querySelector('#boss'), search=document.querySelector('#search');
function filter(){let shown=0;const q=search.value.trim().toLowerCase();for(const c of cards){c.hidden=!!((boss.value&&c.dataset.boss!==boss.value)||(q&&!c.dataset.search.includes(q)));if(!c.hidden)shown++}document.querySelector('#count').textContent=shown+' creatures shown'}
boss.addEventListener('change',filter);search.addEventListener('input',filter);document.querySelector('#backdrop').addEventListener('click',()=>document.body.classList.toggle('light'));filter();
</script></html>'''.replace('OPTIONS', options).replace('CARDS', '\n'.join(cards))
    (DOCS / "index.html").write_text(page)
    print(f"Built {count} minion profiles across {len(factions)} factions.")


if __name__ == "__main__":
    build()
