# Expedition implementation progress log

Brief: [`expedition-implementation-handoff.md`](expedition-implementation-handoff.md). Design: [`expedition.md`](expedition.md).
Branch: `expedition-map` (from `main` at `665b2d4`). Worktree: `/Users/daddymere/.t3/worktrees/Dice-and-Destiny-v2/t3code-beeade77`.

Evidence folder (ignored): `dice-and-destiny-client/.godot/runtime/expedition-evidence/`.

## Milestone status

| Milestone | Status | Commit |
| --- | --- | --- |
| M0 Baseline | done | (no code change) |
| M1 Configuration and content loading | done | see git log (`M1: …`) |
| M2 Map rules on the authority | not started | — |
| M3 Battle bridge | not started | — |
| M4 Ambush, Fortified, Lieutenant buffs, Conqueror | not started | — |
| M5 Client map screen | not started | — |
| M6 Content complete and full-region play | not started | — |
| M7 Docs and wrap-up | not started | — |

## M0 — Baseline (2026-10-12)

- `git status` was clean at `665b2d4` (== `main`). Created branch `expedition-map`.
- The first `./scripts/godot.sh` run cloned godot-cpp, built the native extension and imported the project. The import generated untracked `*.gd.uid` files for scripts committed without them (campaign_prepare.gd, card_tree_*.gd, several tests). They are Godot-generated and were left untracked; they are not part of this work.
- `./scripts/godot.sh --headless --script res://scripts/verify_battle_authority.gd` → `BATTLE AUTHORITY: start and 5D6 planning roll passed`, exit 0.
- `./scripts/godot.sh --headless --script res://tests/presentation/verify_campaign.gd` → `CAMPAIGN: PASSED`, exit 0 (note printed by the test: "no defeat occurred; the loss path was not exercised this run").
- `./scripts/godot.sh --headless --script res://tests/presentation/verify_campaign_prepare.gd` → `CAMPAIGN PREPARE: PASSED`, exit 0.
- Log: `.godot/runtime/expedition-evidence/m0/godot-baseline.log`.
- `cd dice-and-destiny-server && go test ./...` → 25 packages ok, no FAIL, 5m12s wall. Log: `.godot/runtime/expedition-evidence/m0/go-test-baseline.log`.

## Standing rule: the authority is the Go server

The user restated this mid-task (2026-10-12): the exploration map follows the battle pattern exactly. Every map rule, random draw (tiles, mending rolls, loot, events), fog filter, action legality check, reward and the battle hand-off runs in Go (`internal/battle/expedition`, exposed through `expedition_status` / `expedition_act` in `learned.HandleRuntimeRequest`, next to the `campaign_*` ops). The Godot screen under `app/screens/map/` renders the viewer-safe status and submits actions through the same native bridge the battle uses; it never computes outcomes, costs or hidden information.

## M1 — Configuration and content loading (2026-10-12)

Files:
- `dice-and-destiny-server/content/progression_v1/expedition.yaml` — every rule number (§14), plus the status IDs the map applies.
- `internal/battle/expedition/{hex.go,config.go,pack.go}` — axial geometry (flower lattice, distance, lines), the strict config loader (`KnownFields`, schema 1, ranges, terrain/site/drop/price presence, status modifier checks), and the content pack loader for `content/expedition_v1/{tiles,sites,groups,events,gear,regions}` with reference validation and the static group-adjacency rule.
- `content/expedition_v1/` — the first region "The Salt Marches": start tile + 5 countryside + 2 core tiles, 11 sites, 9 groups (incl. the placeholder Warlord), 3 events, 3 gear.
- `content/general_v1/statuses/{ambush,fortified,conqueror,lieutenants_favor,lieutenant,shaken}.yaml` — authored with rules text stating expiry; numbers are per-stack `modifiers` (new schema field).
- `content/minions_v1/{combatants/drowned_regent.yaml, abilities/tide_crown.yaml, abilities/coral_mantle.yaml, dice/regent_d6.yaml, cards/drowned_standard.yaml}` — the placeholder Warlord (24-card blank deck, 3 Toll faces, 5/7/10 damage, prevent 2/3/4).
- `internal/content/{card_map_effect.go, battle_library.go, card_program.go, card_authoring.go}` — `map_effect` on cards (kinds move/draw/energy/sight/scout), `modifiers` on statuses, `LoadoutDiceCount`, generated map text on faces/rules, map-only cards accepted by the authored overlay.
- `content/adventurer_v1/cards/{take_stock,second_wind}.yaml` and `content/authored/authored_cards.json` (revision 48): Take Stock "Map: Draw 2", Second Wind "Map: +10 Energy"; new `pathfinder_paths` tree (Trailblaze, Forced March, Lantern Road, Scout Ahead); Steady Guard gains Warden's March and Sentry's Watch.

Tests (all run and passed):
- `go test ./internal/battle/expedition/` — `TestLoadConfigAcceptsShippedValues`, `TestLoadConfigRejectsBadValues` (18 cases: unknown field, schema, ranges, missing terrain/site/price, bad range shapes, unknown/unsuitable statuses), `TestConfigValuesDriveTerrainAndSight`, `TestHexGeometry`, `TestLoadPackAcceptsShippedRegion` (asserts the §5.7 inventory), `TestLoadPackRejectsBadContent` (31 cases incl. missing refs and bad ranges), `TestLoadPackRejectsTouchingGroups`, `TestAuthoredMapCardsAndTrees`.
- `go test ./internal/content/` — `TestValidateCardMapEffect`, `TestMapOnlyCardsAreBlankBattleCards`, `TestStatusModifiersValidation`; `TestProgramCardFacesAreShort` updated for the two "both" faces.
- `go test ./internal/battle/learned/ ./internal/battle/loadout/... ./internal/battle/engine/ ./internal/battle/` → ok.
- Godot: `verify_campaign.gd` PASSED, `verify_campaign_prepare.gd` PASSED, `verify_battle_authority.gd` passed (log `.godot/runtime/expedition-evidence/m1-godot.log`).

## Decisions made during implementation

1. **Tile stack and the Warlord's goal.** The region's slots are typed (countryside or core). Countryside tiles form a seeded stack drawn in order into any empty countryside slot the player explores. Core tiles are dealt (seeded shuffle) to the core slots when the region is built, and a core slot can be explored only once the countryside stack is empty ("countryside tiles first, then core tiles"). Reason: the Warlord's route must be "laid out at region build" ~5 hexes from his goal, which is a site on a core tile; dealing the core tiles at build is the only way the goal hex exists before it is explored. The player never learns a core slot's tile before exploring it (viewer-safe status omits it).
2. **Group adjacency is a static content rule for open groups.** The region loader checks every pair of tiles in every possible relative flower offset (and within each tile): no hex may touch two different *open* groups (groups standing on the map, which provoke). In practice this means open groups sit at tile centres. Garrisons, occupiers and guardians wait inside their sites and fight only when the player enters, so they never pull a second group into a battle. Patrol steps are guarded at run time: a step that would bring a group within two hexes of another living open group, onto the player, a site or impassable terrain is skipped (the group waits). This makes "never place two groups so that one hex touches both" a load-time guarantee independent of the seed.
3. **Movement is one hex per `move` action.** The authority moves one adjacent hex at a time (so provoke detection is exact); the payment (cards and/or energy) converts into this turn's Move pool, the entered hex's cost is charged from the pool, and leftover Move is lost at end of turn. The client may chain steps along a path it displays, but every step is validated by the authority and the status lists each adjacent hex's cost and whether it provokes.
4. **Influence is a per-turn, per-site pool.** Cards played at a site (`use_site` with `cards`) go to Spent and add 1 Influence each to `turn.influence`, valid only at that site this turn; services (mending, deck changes) consume it. This is literally "cards played at a site this turn" and lets the Prepare screen reuse `deck_change` without choosing cards inside Prepare.
5. **Rest is an explicit camp phase.** Rest steps 1–2 (reshuffle Spent, energy reset) run immediately; the save enters `phase: camp` where camp services are offered (`camp_service`, `deck_change`); `break_camp` runs steps 4–5 (Warlord march, patrols, Day↔Night, draw) and writes a rest summary into the state for the client. After the last round the Warlord deadline fight starts on `break_camp`.
6. **Pending battles are persisted in the save.** A fight (challenge, provoke, garrison, deadline) writes a `pending_battle` context (opponent, count, kind, ambush, fortified, energy brought in, origin hex, seed). The client initializes the runtime with that opponent and calls `reset` with the save; the session records the result once when terminal. Quitting mid-battle keeps the pending battle; relaunching restarts that battle from its start with the same seed (the learned runtime keeps battles in memory only, as the existing campaign does). Recorded as a limitation of criterion 2 (mid-battle = identical battle start, not identical mid-battle progress).
7. **Statuses carry their numbers as stacks.** Fortified and Conqueror add one defensive die per stack; Ambush adds one bonus die per stack on the first offensive roll; the Lieutenant buff adds one damage per stack to every damaging attack. `expedition.yaml` holds the stack counts (`fortified.extra_defense_dice`, `conqueror.extra_defense_dice`, `ambush.bonus_dice`, one Lieutenant-buff stack per living Lieutenant), so one number lives in one place and the status bar shows it as the count. `ambush.max_kept` must equal the five-dice loadout (validated), because the engine collapses the first roll back to the loadout size.
8. **Ambush first roll = roll 5+3, then choose exactly 5.** After the eight-dice first roll the only legal planning command is `planning_keep` with exactly five indices, which collapses the tray to those five (consuming no roll); the normal keep/reroll flow then continues (max 3 rolls). Keeping K and rerolling 5−K fresh dice is equivalent to choosing any five and rerolling 5−K of them, so this matches "keep at most 5; each reroll returns to 5 dice total" without letting any other effect address a sixth die. Ambush expires at the end of round 1's Offensive segment if it was never used (stated in its rules text).
9. **Map cards are blank battle cards with a `map_effect`.** A card with `map_effect` and no program/mechanic is a blank card (never legal in battle, still health, removable by damage); "both" cards keep their program. No new engine gate is needed beyond the existing blank-card behaviour, which the tests assert explicitly.
10. **The Warlord never provokes.** He marches on an agenda; the player challenges him from an adjacent hex. Provoke detection applies to minion groups only, so a hex that touches a minion group and the Warlord never pulls two enemies into one fight. Minion groups are validated statically (decision 2). If his route crosses a hex holding a minion group he shares it; if it is the player's hex he waits one step short.
11. **Entering a site is the action and costs no Move.** `enter_site` from an adjacent hex moves the player onto the keep/tower/ruins/grotto/occupied-village hex and starts the garrison or guardian fight; a defeat pushes the player back to the hex they came from. Event sites trigger when the player moves onto them (Slay-the-Spire "?" rooms) and must be resolved before any other action; each is one-shot.
12. **Deck changes keep copies in place.** An upgrade/trade-down replaces the copy in hand first, then draw pile, then Spent; a sold or stored copy leaves Spent first, then draw, then hand; a bought card goes on top of the draw pile (it adds health and time to this round, as the design says). Wounded copies never move.

## Placeholders created

- **Warlord combatant** `drowned_regent` ("The Drowned Regent") in `content/minions_v1/combatants/`, with `tide_crown` / `coral_mantle` abilities, `regent_d6` and the blank `drowned_standard` deck (24 cards). Replace with a real boss.
- **Card names** (all original, placeholders): Trailblaze, Forced March, Lantern Road, Scout Ahead (Pathfinder tree); Warden's March, Sentry's Watch (Steady Guard tree).
- **Region and site names**: The Salt Marches, Meadow Crossing, Reed Shallows, Cairn Hills, Saltreed Fen, Stillwater, Brinehold March, Greywharf Strand, Drowned Reach; Hollow Ford, Saltreed, Stillwater Monastery, Greywharf, Drowned Cairn, Sunken Chapel, Kelp Grotto, Brinehold Keep; events Wayside Shrine, Wrecked Barge, Hermit's Fire; groups Eel Warden, Bell Marshal, Brine squatters.
- **Gear**: Bandage (10 gold), Lantern (40), Siege Hooks (20).

## Open issues

(none yet)
