# Minion bestiary

Twenty boss factions each receive ten original minion designs: 200 individually generated transparent creature images. Each set shares materials, palette and ecology with its boss while varying species, anatomy, silhouette and intended encounter role. Original boss art remains unchanged.

Open [the searchable local catalog](index.html) to filter by boss or search names, roles and descriptions. Click a creature for its full-resolution PNG. The light/dark toggle helps inspect transparent edges. This catalog works locally without a server or network access.

## Assets and editable profiles

- PNGs: `dice-and-destiny-client/assets/battle/minions/<boss_id>/<minion_id>.png`
- Godot visual profiles: `dice-and-destiny-client/content/battle_visuals/minions/<boss_id>/<minion_id>.tres`
- Lightweight index: `dice-and-destiny-client/content/battle_visuals/minion_catalog.json`
- Full prompts, descriptions and generation provenance: `<boss_id>/manifest.json` beside this document.

Visual profiles contain the creature's artwork, facing, default height and ground anchor. The encounter chooses its environment independently. Profile IDs include the boss faction prefix to remain unique. Every sprite can also be reused outside its associated faction.

These are visual concepts and placement-ready assets. Descriptive roles such as ambusher or support express design intent; numerical stats, decks, abilities, progression and enemy AI have not been invented or added.

## Preview in Godot

From the repository root:

```bash
./scripts/godot.sh res://devtools/battle_art_gallery.tscn
```

Choose a minion faction in the collection selector, then choose a creature and environment independently. **Two copies** shows independently placed instances. Minion images load only when selected, avoiding loading 200 textures into normal battles.

## Generation and validation

Artwork was produced with the built-in image-generation tool, one image per creature, using the associated boss as a style/material reference. The source images and complete prompts are recorded in each faction manifest. No programmatic recoloring or miniature copies were used to manufacture variants.

`scripts/build_minion_catalog.py` rebuilds the HTML catalog, Godot profiles and index from completed manifest entries. It requires Python with Pillow to read alpha bounds for placement anchors; it never changes image pixels. Original generated files are preserved at the provenance paths; selected PNGs are copied into this project.

`scripts/verify_minion_assets.py` checks the full inventory, unique IDs/names/file hashes, resolution, transparency, contained silhouettes and provenance. Its report is [validation.json](validation.json). The Godot check `res://tests/presentation/verify_minion_art_catalog.gd` loads every profile and exercises all twenty faction selectors, independent environments and repeated creature placements.

Completed validation: **200 / 200 sprites**, **20 / 20 factions**, **200 / 200 visual profiles**. The PNG set totals approximately **351.7 MiB**. Five images received framing revisions during review; selected outputs preserve their original generated pixels. Asset checks and the Godot minion catalog, enemy catalog and scenery checks passed. The minion gallery also passed with the graphical renderer; [placement-preview.png](placement-preview.png) shows two reusable minion instances on an independently selected environment. HTML filter/search/backdrop logic and all asset links were checked; interactive browser validation was unavailable because the browser rejected local file URLs.
