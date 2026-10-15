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

## Expedition ideas from the Mage Knight review (2026-10-14)

Accepted for later; none of them changes battle rules.

- [x] **Remove the gold→XP trainer** (`gold.trainer_gold_per_xp` in `expedition.yaml`, Greywharf's trainer service, its view/client entry, tests and docs). Hard rule: gold never buys power; XP alone measures and controls character power.
- [ ] **Tile generator screen.** A UI screen that generates and shows world layouts for review, to experiment with before building the world. Start simple: no wave function collapse or dynamic filler; a finished world may stay static. See `docs/game-design/open-world-plan.html` §3.
- [ ] **Skippable cutscenes.** If the game gets cutscenes (e.g. before the opening scenario), the player may watch or skip them. The opening scenario itself is mandatory.

- [ ] **Round Tactics (maybe).** At break camp, pick one of a few Day or Night tactics, each a visible status that bends the deck for the round (Mage Knight's Rethink, Preparation, Sparing Power…). Unsure it's needed on top of the battle mechanics; revisit later.
- [ ] **End-of-turn location sites.** Ending a turn on a site pays: a glade heals 1 point of a wound, a mine yields something stored.
- [ ] **Better adventure-site information.** Some ruins show their exact reward once seen; some guardians stay unknown until entered; a garrison that beats you keeps the cards you removed, so the next attempt meets it weaker (battle setup only).
- [ ] **Energy carry-over as gear.** A gear item lets you keep more energy than normal through a rest into the next Day/Night round.
- [ ] **Trainers unlock tree nodes.** Some upgrades (e.g. a +3 energy variant) are learnable only at one site: pay its trainer gold to unlock the node, then pay XP for the upgrade as usual.
- [ ] **New map abilities, unlocked by gear.** Improvisation (e.g. boots: discard two cards for Move 3), Influence cards, flight/ignore terrain, lake and mountain crossing, Survey (look at the next two tiles and pick one), map healing.
- [ ] **Day and Night with real trade-offs per terrain.** Some terrain is easier at night (a desert or salt flat), some sites better by day, so both halves of the cycle have positives and negatives.
