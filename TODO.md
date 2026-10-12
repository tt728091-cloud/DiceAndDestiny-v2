# Project TODOs

## Per-battle diagnostic bundle

- [ ] Persist an exportable diagnostic bundle for every played battle.

The bundle should be keyed by battle ID and contain:

- submitted player commands, including rejected commands;
- authoritative events in sequence order;
- client and inspector errors;
- presentation beats and player acknowledgements;
- final authority checkpoint and relevant random-state metadata;
- screenshots captured at important segment transitions and errors;
- a concise human-readable battle summary.

Suggested layout:

```text
battle-id/
  commands.jsonl
  authority-events.jsonl
  client-errors.jsonl
  presentation.jsonl
  final-checkpoint.json
  screenshots/
  summary.txt
```

Acceptance criteria:

- The bundle is written incrementally so a crash does not erase earlier evidence.
- Private authority data remains separate from viewer-safe client data.
- A battle ID is sufficient to locate and export the bundle.
- Normal gameplay does not require the inspector to be enabled.
- Diagnostics can be disabled or retention-limited for release builds.

## Content versioning for campaign saves

- [ ] Version the game content and tie campaign saves to it.

Campaign saves store card and ability IDs, not definitions, so admin edits to cards, trees and abilities reach campaigns already in progress. That is intended while cards are being tuned (decided 2026-10-10). Before release, review:

- stamping each campaign save with the content version it started on;
- shipping compatibility upgrades for each new version (renamed, repriced or removed cards and abilities, changed tree paths);
- refusing, or explicitly migrating, saves that a breaking change would invalidate.


## Expedition follow-ups

Out of scope of the first Expedition build (2026-10-12; see `docs/game-design/expedition-progress.md`):

- [ ] **Mixed enemy groups and more than 2 enemies per battle**, including two groups pulled into one battle. The engine supports multiple seats; the session, encounter model, mlsim observations and client layout pin one minion type with 1–2 copies.
- [ ] **Grievous wounds** (crippled cards) and the town healer for them.
- [ ] **Companions and recruiting.**
- [ ] **More regions, the between-region camp/healer stop, and run-scoped relics.** The region victory records `pending_relic_choices`; nothing offers relics yet.
- [ ] **Map abilities on more cards and trees, and gear beyond** Bandage, Lantern and Siege Hooks.
- [ ] **Replace the placeholder Warlord** ("The Drowned Regent") and add real art for tiles and sites. Balance every number in `expedition.yaml` and the Warlord in playtesting (the current tuning only makes him beatable).
- [ ] **Content versioning for campaign saves** now also covers the region pack and `expedition.yaml` (see above).
- [ ] **Mid-battle relaunch** restarts the pending battle from its start with the same seed (the learned runtime keeps battles in memory); persisting battle progress would make a relaunch resume mid-battle.
- [ ] **A live event scene through the inspector**: the shipped region's only event near the start sits behind a group; the pointer test shows it, the live play-through did not reach it.
- [ ] `tests/phase3/verify_offensive_roll_controls.gd` fails on the pristine client (stale "Roll 5 Dice" label); unrelated to the Expedition, recorded during M4.
