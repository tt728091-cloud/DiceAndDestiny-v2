# Reusable battle habitats and fighters

The live battle composes environment art and independent transparent fighters. It no longer uses the baked Venom/Blade Warden `wasteland/duel.png`, nor chooses a background by checking the two character IDs. Existing combat rules remain authoritative and separate from visual configuration.

## Editable resources

All resources live under `dice-and-destiny-client/content/battle_visuals/` and can be edited in Godot's Inspector or as text:

| Resource | Purpose |
| --- | --- |
| `fighters/*.tres` | A reusable visual profile: definition ID, display name, body texture, optional portrait, native facing, default height, and ground anchor. |
| `habitats/*.tres` | A clean environment texture and optional ambient shade. Contains no fighters or UI. |
| `encounters/*.tres` | One habitat plus an array of independently positioned fighter instances. |
| `library.tres` | Registered fighter profiles, independently selectable habitats, and the default encounter layout. |

`default_duel.tres` binds its two placements to the existing presentation actor IDs `blade` and `goblin`. Those are compatibility bindings for the current duel protocol, not artwork selections. The renderer resolves each bound actor's actual `definition_id` through the visual library. Changing a combatant definition selects its corresponding artwork without a new renderer branch.

Each placement has a unique `instance_id`, an optional `actor_id`, a ground position, optional height override, facing direction, and draw order. Coordinates are in the existing 1920 × 1080 battle design space; the whole composition scales with the battle's letterboxing. The ground anchor is a normalized point in the image, normally between the feet. Positioning uses that anchor, preserving both contact point and aspect ratio when size or facing changes. Higher draw orders appear in front of lower ones, all behind the interface.

For unbound previews, a placement can reference a visual `profile` directly. For bound placements, the actual actor identity always wins; an unknown actor is never silently shown as the preview creature. The renderer accepts an arbitrary number of placements, including repeated uses of the same profile. The battle HUD and authority still implement a duel; multiple active opponents need their own gameplay/targeting work.

## Change a habitat or placement

Assign `encounter_visual` before adding the battle screen to the tree, or use its `set_encounter_visual()` method for an immediate visual update. The selected resource survives battle redraws. Duplicate shared resources before modifying them at runtime:

```gdscript
var layout: BattleEncounterVisual = preload(
    "res://content/battle_visuals/encounters/default_duel.tres"
).duplicate(true)
layout.habitat = preload("res://content/battle_visuals/habitats/mushroom_cavern.tres")
layout.fighters[1].ground_position = Vector2(1435, 585)
layout.fighters[1].height = 520.0
battle_screen.set_encounter_visual(layout)
```

This changes scenery only. It does not turn a Blade Warden into a Spore Cantor or alter health, cards, targeting, or abilities. For the application's global default, change `library.tres`'s default encounter instead. Per-encounter selection belongs to the caller that constructs the battle screen; it is not inferred from the enemy's preferred habitat.

## Add an enemy visual profile

1. Add a transparent full-body texture under `assets/battle/fighters/`.
2. Duplicate a fighter `.tres`, assigning its definition ID, name, texture, facing, default height, and contact anchor.
3. Register that resource in `library.tres`'s `fighters` array. The HUD uses this same registry for the displayed name and portrait.
4. Bind a placement to the actor ID in an encounter, or reference the profile directly for an art preview. Duplicate a placement with a new instance ID to show another copy of the creature.

Spore Cantor now has a visual profile and standalone artwork. A playable Spore Cantor still needs its combat definition/deck/abilities in the authority catalog, following `dice-and-destiny-server/content/battle_v1/combatants/`; this change does not invent its mechanics.

## Included compositions and preview

The complete 20-enemy extraction and independent enemy/environment gallery are documented in [battle-enemy-art-library.md](battle-enemy-art-library.md).

- `default_duel.tres`: the existing wasteland with separate Venom and Blade Warden layers resolved from actual actors.
- `spore_cantor.tres`: Venom and one Spore Cantor in the mushroom cavern; visual preview with explicit profiles.
- `spore_cantor_pair.tres`: the same habitat with two independently sized and positioned Spore Cantors sharing one texture.

From the repository root:

```bash
./scripts/godot.sh res://devtools/battle_scenery_preview.tscn
```

The preview is also editable as a scene in Godot. Assign a different encounter resource to its `encounter` property to view that composition. Rerun the preview after editing nested resource values. It does not start or change a saved battle.

## Verification

```bash
./scripts/godot.sh --headless --script res://tests/presentation/verify_battle_scenery.gd
./scripts/godot.sh --script res://tests/presentation/verify_battle_scenery.gd
./scripts/godot.sh --headless --script res://tests/presentation/verify_player_roll_animation.gd
```

The composition test checks real alpha transparency, independent habitat and actor swaps, repeated profiles, mirrored ground anchors, draw order, immutable shared resources, mouse-input transparency, live battle integration, HUD profile reuse, redraw retention, and resizing. Its graphical run captures `.godot/spore-cantor.png`, `.godot/spore-cantor-pair.png`, and `.godot/layered-live-battle.png` for visual review.

## Artwork provenance

Prepared with the built-in image-generation tool in reference-image editing mode. The existing duel and supplied Spore Cantor mockup were used as visual references; the resulting independent layers are newly rendered artwork, not pixel-identical crops. No gameplay UI from the mockup is included in the assets. Original references remain intact.

| Output under `dice-and-destiny-client/assets/battle/` | Reference | Prompt specification |
| --- | --- | --- |
| `fighters/venom.png` | `wasteland/duel.png` | Isolate the full-body black-haired alchemist, ragged black cloak, pale sleeves, green potion and hanging lantern; preserve pose, facing right, camera angle and dark watercolor/ink style; true transparent alpha, no scenery, other character, text or UI. |
| `fighters/blade_warden.png` | `wasteland/duel.png` | Isolate the full-body antlered skeletal Warden, pale mask, bone armor, burgundy cloth and sword angled down-left; preserve design, facing left and watercolor/ink style; true transparent alpha, no scenery, alchemist, text or UI. |
| `fighters/spore_cantor.png` | `docs/ui-mockups/dark-watercolor-monster-habitats-2026-09-15/04-spore-cantor.png` | Isolate the full-body Spore Cantor with hollow pipe torso, layered mauve mushroom caps, ragged fungal skin, root arms and rooted base; preserve three-quarter view facing left and dark watercolor style; true transparent alpha, no environment or UI. |
| `habitats/mushroom_cavern.png` | Same Spore Cantor mockup | Reconstruct an environment-only 16:9 underground mushroom arena with shelf fungi, hanging mycelium, sparse green spores and open pale floor; remove both characters and every UI element; preserve dark watercolor style and ground-level battle camera. |

The existing clean `wasteland/arena.png` is reused as the wasteland habitat.
