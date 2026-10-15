# Implementation handoff: the Expedition (Mage Knight-style exploration map)

**Written:** 2026-10-12, at the end of the design review with the user.
**For:** an engineer (human or LLM) starting with **no prior context**. Read this whole document before doing anything.
**Repository:** `/Users/daddymere/games/Dice-and-Destiny-v2` (Go authority in `dice-and-destiny-server/`, Godot 4 client in `dice-and-destiny-client/`).

---

## 0. How to use this document

- This is the **complete brief**. It tells you what to build, the rules you must not break, where the existing code lives, the order to build things in, how to test, and what "done" means.
- The **design** lives in [`expedition.md`](expedition.md) (with pictures in [`expedition/`](expedition/)). That document is the source of truth for *game rules*. This document is the source of truth for *how to implement and verify them*. If the two disagree, stop and report it rather than guessing.
- **Keep a progress log** at `docs/game-design/expedition-progress.md`. Update it at the end of every milestone, and whenever you make a decision this document doesn't cover. Record:
  - milestone status;
  - files changed;
  - tests added, and the commands you ran with their results;
  - screenshots taken, and what you saw in them;
  - decisions made;
  - open issues.

  If your context resets, the log plus this document must be enough to continue.
- **When something isn't specified:** choose the option most consistent with `expedition.md` and the rules in section 3, record it under "Decisions made during implementation" in the progress log, and mention it in your final report. Ask the user only when you are blocked on a product decision that would be expensive to reverse.
- **Never claim something works because the code looks right.** Every acceptance criterion must be proven by running it (section 8).

---

## 1. Goal in one paragraph

Replace the campaign's linear encounter list with an explorable hex map, built tile by tile as the player explores. One deck of cards pays for walking, using sites and fighting. A region lasts a fixed number of Day/Night rounds, and each round lasts one pass through the deck. Monsters, Lieutenants and the Warlord sit on the map. Every fight runs in the **existing battle system, whose rules stay unchanged**: the battle simply starts from the player's map state (hand, draw pile, spent cards, wounds, energy) and hands the result back to the map. The work is **done** when the user can start a new campaign, play a full region end to end in the real game with the mouse, and every acceptance criterion in section 7 is proven by automated tests *and* by a recorded live play-through.

---

## 2. Required reading (in this order)

1. `AGENTS.md` (repo root): mandatory repo rules. Read every section.
2. `docs/game-design/expedition.md`: the full design, with pictures. Read it all, including the decision log at the end.
3. `docs/game-design/campaign.md`: the current campaign (saves, Prepare screen, wounds) that you are extending.
4. `docs/game-design/turn-structure.md`: battle round structure: Effects → Income → Offensive → Defense. Note: damage order is **discard → draw → hand** in code; lines ~1843 and ~1879 of this doc still say otherwise and are out of date.
5. `docs/game-design/unified-defense.md`, `battle-wounds.md`, `prevention-configuration.md`: how battles, wounds and prevention work today.
6. `docs/game-design/card-configuration.md` and `card-trees.md`: card programs, timing, and card trees (you will add map effects and new tree branches).
7. `docs/game-design/ability-configuration.md`: dice abilities and defensive rolls (needed for Ambush and Fortified).
8. Existing handoff docs, for this repo's verification standard and tone: `docs/game-design/venom-development-handoff.md`, `curse-development-handoff.md`, `godot-client-battle-implementation-prompt.md`.

---

## 3. Non-negotiable rules

### Repo rules (from `AGENTS.md`, summarized — read the original)

- **Run Godot only through `./scripts/godot.sh`** from the repo root, never a raw `godot --path`. Script tests: `./scripts/godot.sh --headless --script res://path/to/test.gd` (they get disposable state, including an empty authored root).
- **Native builds** only via `dice-and-destiny-server/scripts/build_native.sh` (the launcher runs it with `--if-needed`). Never copy artifacts from another worktree.
- No fixed inspector ports, shared `/tmp` paths or shared `user://` saves. Don't point `DICE_AND_DESTINY_*_ROOT` at another checkout. Don't touch another worktree's `.godot`, build or runtime folders.
- **Only the authority** (Go) decides outcomes and awards XP, gold or items. The client never computes rewards, costs, legality or hidden information.
- **Visible statuses:** any buff, debuff or ongoing effect that can be consumed or expires (Ambush, Fortified, Lieutenant buffs, Conqueror, "Shaken: hand size −1", and so on) must be an **authoritative status** authored in the content catalog, shown on the status bar with its icon, count and full rules text, including exactly when it expires. A button alone is never enough. Verify:
  - acquiring it;
  - display and hover;
  - using it against the right target;
  - that it can't be used twice;
  - that it persists through save/reload;
  - that it expires correctly through the real phase transition;
  - pointer input as well as authority commands.
- **Draw/discard:** battle draws never recycle discard into the draw pile. Deck, hand and discard all count as health; only removal loses health. On the map, the only reshuffle is the **explicit rest** (an authored recovery outside battle).
- **Damage-card order** stays discard → draw → hand.
- **Campaign rules:**
  - Campaign progress lives only in the named campaign save (`<loadout root>/campaigns/<id>.json`), never in character templates or editor saves.
  - Campaign decks use only card-tree cards.
  - Wounds carry over between battles.
- **Authored content:** published cards and card trees live in the tracked `dice-and-destiny-server/content/authored/` (for example `authored_cards.json`). Script tests use an empty disposable authored root and must never write to the tracked directory. Tests that need trees copy them in, as `verify_campaign.gd` does in `_seed_card_trees()`.
- **Card and variant names must be original.** Never reuse Dice Throne card names. Placeholder names are fine, but mark them as placeholders in the progress log.
- **Standard checks** must pass at the end: `cd dice-and-destiny-server && go test ./...` and, from the repo root, `./scripts/godot.sh --headless --script res://scripts/verify_battle_authority.gd`.

### Design rules (from `expedition.md` — the user decided these; don't change them)

- **Battles stay as they are.** The only battle changes allowed are:
  - starting a battle from preset piles and energy;
  - the **Ambush** and **Fortified** statuses (and the Warlord's buff statuses), which change dice counts as specified;
  - reading the end state back.

  Battles stay simultaneous: nobody "goes first".
- **The bridge:**
  - At battle start, map hand → opening hand **exactly as it is** (no top-up, no opening draw); map draw pile → battle draw pile; map Spent pile → battle discard; wounds → removed; current map energy → starting energy.
  - At battle end, the leftovers go back to the map. The player keeps **at most the energy they brought in**. **A fight ends the turn.**
- **Hand symmetry:** one hand size (4) and hand limit (6) for both map and battle. Any change applies to both.
- **Turn:**
  - Move (1 card = 1 Move; 5 energy = 1 Move; pay the cost of the hex you enter), then **one action** (use a site, challenge an adjacent enemy, or enter an adventure site).
  - The turn ends when the player chooses, but only after they have **played or discarded at least 1 card** (a fight counts).
  - Unplayed cards stay in hand; draw up to hand size at end of turn.
  - Ordinary cards **can't** be turned into energy on the map. Cards whose own effect gains energy (Second Wind) do give energy.
- **Clock:**
  - A region lasts **4 rounds** (Day, Night, Day, Night; configurable). A round lasts one pass through the deck.
  - **Rest** happens when the draw pile is empty at end of turn, or when the player chooses to **make camp** (forfeiting the unplayed cards). At rest:
    1. Spent cards are reshuffled into the draw pile (wounds stay out).
    2. Energy resets to the character's base.
    3. Camp services run (mend rolls, deck changes at camp prices).
    4. The Warlord marches 2 hexes; roaming Lieutenants step.
    5. Day ↔ Night flips. Draw up to hand size.
  - After the last round, do the normal rest (reshuffle, energy reset, camp services), then the Warlord comes to the player's hex. It's a forced fight starting with a fresh hand, with no Ambush, and he's not Fortified in the open.
- **Explore:** pay 2 Move next to an empty tile slot to place the next tile from the region's stack (countryside tiles first, then core tiles).
- **Fog of war:**
  - Hexes in sight (2 by day, 1 at night) show everything.
  - Revealed hexes out of sight show greyed terrain. Sites you've seen stay known; roaming enemies are shown where last seen.
  - Never-seen contents are unknown.

  **The client must never receive hidden contents.**
- **Encounters:**
  - **Provoked:** moving from one hex adjacent to an enemy group to another hex adjacent to the same group forces a fight now, never an Ambush.
  - **Challenge:** your action against an adjacent group.
  - **Garrison:** entering a keep, tower or occupied site. Defenders inside walls are **Fortified**.
- **Ambush** requires all of:
  - the player starts the fight (never provoked);
  - **unseen**: the player began the turn outside the target's sight (enemies see like the player: 2 hexes by day, 1 at night), or attacks from cover (a forest or ruins hex);
  - the target's **habit** allows it now: Unwary = always; Sleeper = at night; Night hunter = by day; Always alert = never.

  Effect: a visible **Ambush** status on the player, used up on the first offensive roll. That roll uses 5 + 3 = 8 dice; the player keeps at most 5; each reroll returns to 5 dice total (keep 3 → reroll 2). Max rolls is still 3.
- **Fortified:** a visible status on defenders: their defensive roll uses **+1 die**. Siege gear removes it.
- **Defeat (not the Warlord):**
  - The player's wounds return to their **pre-battle** state **plus 2 defeat wounds**; it can never take the last live card.
  - The player is driven back to the hex they came from, and the enemy stays.
  - The round ends with a forced rest where they stand.
- **Losing to the Warlord ends the run.** Nothing else does.
- **The Warlord:**
  - Has a goal on a core tile and starts ~5 hexes from it on a route laid out at region build.
  - Marches 2 hexes per rest. Extra rests (early camps, defeats) speed him up.
  - Can be challenged on the road.
  - Has his own habit and weaknesses like any enemy. He's Fortified only in a tower or keep.
  - Each living Lieutenant gives him a visible buff status.
  - If he reaches his goal: the site's services are lost, he's Fortified there, and he gains a visible **Conqueror** status (+1 defensive die).
  - Beating him early pays **+20 XP per unused round** plus **one extra relic choice** (if relics aren't implemented yet, record the extra choice as a pending reward and say so).
- **Economy:**
  - **XP** only grows the deck (buy cards, tree upgrades, ability upgrades).
  - **Gold** buys gear, consumables and healers. A trainer converts 3 gold → 1 XP. Gold never upgrades a card directly. *(2026-10-14: the trainer was removed; gold never buys XP. See [phase-0-handoff.md](phase-0-handoff.md), task 0.1.)*
  - **Influence** is never stored: it's cards played at a site this turn (1 card = 1 Influence).
  - Deck changes happen only at sites and cost XP + Influence: camp 3, village 2, monastery 1, town 0.
  - Mending:
    - camp: 1 roll per wound per bandage (or 3 Influence), success on 4+ on a d6;
    - village: 2 Influence per wound;
    - monastery: 1 Influence per wound, usable mid-round;
    - town healer: gold.
- **Night:**
  - Forest costs 3 (2 by day); sight is 1.
  - Sleepers can be ambushed at night and night hunters by day; night hunters roam (1 hex per turn) at night.
  - Grottos and ruins pay more at night but have stronger guardians.
  - Villages and towns close at night except the inn (a safe rest, +1 to camp mend rolls). Monasteries stay open.
- **Cards** come in three kinds:
  - **battle** (today's cards; worth 1 Move or 1 Influence on the map);
  - **map** (new; a map effect, **unplayable in battle** but still health);
  - **both**.

  Card trees gain map and "both" branches. Map effects that cost energy are allowed but rare, and must be big, because map energy refills only at rest.
- **Every number is configuration, not code** (section 5.2).

---

## 4. What exists today (code map)

Paths are relative to the repo root.

**Campaign saves and progress (Go, package `loadout`):**
- `dice-and-destiny-server/internal/battle/loadout/campaign_save.go`:
  - `CampaignSave{version,id,name,character,created_at,migrated_from,sheet Progress}`, stored at `<root>/campaigns/<id>.json`.
  - Functions: `ReadCampaignSave`, `ListCampaignSaves`, `NewCampaignSave`, `DeleteCampaignSave`, `BuyCampaign` (checks revision, forces tree-cards-only, checks expected cost).
- `.../loadout/progression.go`:
  - `Progress` with JSON fields `decklist`, `collection`, `ability_board`, `xp`, `total_budget`, `deck_value`, `revision`, `earned_xp`, `campaign`, `wounds`, `wound_value`.
  - `writeJSON` does an atomic temp-file + `Sync` + `os.Rename`. All functions hold the package mutex `progressMu`.
- `.../loadout/campaign.go`: the linear encounter model (`Encounter`, `Campaign`, `CampaignProgress`, `NextEncounter`, `RecordCampaignBattle`, `NonTreeCards`, `IsTreeCard`).
- `.../loadout/deck.go`: `Entry{card_id,count,tree}`.

**Runtime ops (Go, package `learned`):**
- `dice-and-destiny-server/internal/battle/learned/runtime.go`: `HandleRuntimeRequest` dispatches JSON `op`s. Responses are `{"ok":true,"result":…}` or `{"ok":false,"accepted":false,"error":…}`. The campaign ops are `campaign_status`, `campaign_new`, `campaign_delete`, `campaign_loadout`, `campaign_purchase`, and `reset` with `encounter`.
- `.../learned/campaign.go`: `campaignStatus`, `campaignLoadout`, `ResetCampaignEncounter`, `recordCampaignBattle` (called once when the battle is terminal, from `updateTerminalTelemetry`).
- `.../learned/session.go`:
  - `NewSession` caps `OpponentCount` at 0–2 (line ~102).
  - `resetLoadout` builds the deck plus wounds and passes `SeatWounds` (~222–229).
  - A second copy of the opponent sits in `seat-c` (~282).

**Battle setup (Go):**
- `dice-and-destiny-server/internal/battle/state/battle.go`:
  - `ActorSetup` (~265) has `Deck`, `Hand`, `Discard`, `Removed`, `Resources` (`StartingHandSize`, `MaxHandSize`, `StartingEnergyPoints`, `MaxEnergyPoints`, `EnergyPoints`), `Statuses`, `DiceLoadout`.
  - `NewBattleFromSetup` (~370) copies the piles and energy into the actor state.
- `.../internal/battle/participant_assembler.go`: `assembleSettledParticipantsWithCatalog` (~233) creates card instances (`seat-a-card-001`, …), sets starting energy and `MaxRolls: 3` (~264), and turns `starting_statuses` into statuses.
- `.../internal/battle/deck_loadout_assembler.go`: `WithDeckLoadouts` and `WithCarriedWounds` (~63; moves matching copies from Deck to Removed). They are chained in `.../internal/battle/mlsim/environment.go` (~245), whose `ResetRequest` has `seat_wounds` (~63) and whose `ReplayRecord` carries it (~81).
- `.../internal/battle/engine/settled_engine.go`: `initializeSettled` (~134) draws `StartingHandSize` cards per actor. **Setting a seat's `StartingHandSize = 0` in a wrapper skips the opening draw** without forcing `Initialized` (which would skip setup events).
- `.../internal/battle/authority.go`: `InspectBattleState(battleID)` (~182) returns the full `state.Battle`. When terminal, read each actor's four piles, `Resources.EnergyPoints`, `WinnerActorID` and `Wounds`. Snapshots (`internal/battle/snapshot/snapshot.go`) expose `DeckComposition`, `DiscardComposition` and `RemovedComposition`; the campaign uses `RemovedComposition`.
- Dev-only precedent for preset piles: `internal/battle/scenario/builder.go` (`ActorOverride{CardZones, Energy, Statuses}`, behind the `scenario_tools` build tag). Don't ship that; follow the `WithCarriedWounds` pattern instead.

**Dice and statuses (Go):**
- Dice:
  - Five d6 per actor (`dice_loadout`). Validation requires exactly 5 loadout dice (`internal/content/battle_library.go:~676`); keep the loadout at 5 and add Ambush dice as a temporary roll bonus.
  - Offensive flow in `settled_engine.go`: `progressSettledOffensive` (~398), `handleOffensivePlanningCommand` (~1299: planning roll/keep/reroll/ability), `rollCombatDice` (~2539), `validateRerollIndices` (~3284).
  - Legal actions are in `engine/legal_actions.go:55-80`.
  - Several places assume dice indices fit inside 5 `FinalDice`: curse indices, legal actions, `diceEvent`, and the client.
- Defensive roll: `rollSelectedDefense` (~2232) rolls `max(1, DiceCount)` from a bag that refills, so a 6th die is safe. The 1–5 range is validated in `internal/content/ability_configuration.go:~129, ~252`. The per-face outcomes must handle the extra face.
- Statuses:
  - Schema `BattleStatusDefinition` (`battle_library.go:~214`): `presentation{rules_text,glyph,art_key}`, `activation_mode`, `polarity`, `stacking`, `lifecycle`, `triggers`.
  - Examples: `content/battle_v1/statuses/blind.yaml`, `content/adventurer_v1/statuses/strong_swing_ready.yaml`.
  - Expiry is enforced in code (for example `expireOffensiveModifiers` ~3446, `card_mechanics.go` `expireMechanicStatuses`). `applyStatus`/`removeStatus` are at ~2893/~2907.
  - The only start-of-battle mechanism today is a combatant's `starting_statuses`; set `ActorSetup.Statuses` from your wrapper instead.
- Client status UI: `dice-and-destiny-client/presentation/battle/status_icon_strip.gd`, data from `content/battle_presentation_catalog.gd` (`status(id)`). Icon shapes are hard-coded in `presentation/battle/battle_icons.gd:~48`, so add new status IDs there.

**Opponents:**
- The **engine** accepts 2–4 seats with independent definitions (mixed enemies work at engine level; see `engine/multi_minion_test.go`).
- The **campaign session** limits a battle to **one opponent definition with 1–2 copies** (`learned/session.go`, `campaign.go`, client `_minion_opponent`).
- Lifting that limit (mixed groups, more than 2 enemies) is a **separate follow-up push**. See section 9.
- Existing minions: `dice-and-destiny-server/content/minions_v1/combatants/drowned_oracle_{brine_mask,bell_diver,ribbon_eel}.yaml` (scripted, `single_ability_policy`). **No boss-tier combatant exists.**

**Healing:** out-of-battle changes to the save's `wounds` don't touch the engine. The engine's revive-trade validation (`internal/content/card_program.go:~190, ~336`) only matters for in-battle card effects; leave it alone.

**Client (Godot, all screens built in code — no `.tscn` except the battle and bootstrap scenes):**
- `dice-and-destiny-client/app/boot/battle_bootstrap.gd`: the main menu ("Campaign" button ~104, `_open_campaign` ~263).
- `app/screens/campaign/campaign_screen.gd`:
  - the save list and new-campaign UI;
  - the play view with `_render_path` (the linear `EncounterPath`, to be replaced by the map for expedition saves);
  - `_start_next` (builds the gateway, starts the battle, instantiates `battle_screen.tscn`).
  - Controls use `set_meta("campaign_control", id)` and register with `/root/GameInspector` (`register_control`, ~358).
- `app/screens/campaign/campaign_prepare.gd`: the Prepare screen (deck, abilities, tree trades through `campaign_loadout` and `campaign_purchase`).
- `app/screens/battle/battle_screen.gd`: campaign badge (~1045), `_build_campaign_completion` (~2684), `_return_to_campaign` (~2709).
- `app/screens/map/README.md`: an **empty folder reserved for map screens. Put the expedition map screen here.**
- `app/screens/character/character_style.gd`: shared palette and helpers (`box()`, `theme()`, `label`, `chip`, `stat`). `presentation/battle/cinematic_theme.gd` has the HUD colors.
- Native bridge:
  1. `dice-and-destiny-server/adapters/gdextension/go_export/main.go` exports `HandleLearnedBattleJSON`.
  2. The C++ `NativeBattleAuthority` (`adapters/gdextension/src/native_battle_authority.cpp`) exposes it as `learned_battle_request`.
  3. The autoload `local_client/learned_battle/learned_battle_runtime.gd` (`_request`, `campaign_status()`, `campaign_loadout()`, `start_battle(..., encounter, campaign_save)`, `encounter_model_key` → `minion:<def>:<count>`) calls it.
  4. `learned_battle_gateway.gd` carries `encounter_id` and `campaign_save`.
- Art: there are **no hex, tile or map assets**. Terrain can be drawn as colored hex polygons with icons (`presentation/battle/battle_icons.gd` draws icons as inline SVG paths). Habitat art in `assets/battle/habitats/` may be reused.

**Tests to learn from:**
- Go: `internal/battle/learned/campaign_test.go` (`playCampaignBattle` plays real battles; `campaignLoadoutRoot`, `startCampaign`, `readSave`, `editSave`).
- Engine tests in `internal/battle/engine/` (`wounds_test.go`, `draw_exhaustion_test.go`, `hand_limit_test.go`, `energy_cap_test.go`, `multi_minion_test.go`, `strong_swing_test.go`).
- Godot: `dice-and-destiny-client/tests/presentation/verify_campaign.gd` and `verify_campaign_prepare.gd`. They:
  - extend `verify_character_deck_editing.gd` → `verify_character_creation.gd`, which provides `_click(button)` (real pointer events via `root.push_input`), `_text`, `_capture` (screenshots only when an env var is set and not headless), and `_expect`;
  - set `root.size = Vector2i(1280, 720)`;
  - play battles through the gateway until `battle_result`;
  - end with `print("…: PASSED")` and `quit(1 if failed else 0)`.
- Status test example: `tests/presentation/verify_curse_bloom_status.gd`.

**Debug inspector:**
- Start the game with `DICE_AND_DESTINY_INSPECTOR=1 ./scripts/godot.sh`, then use `python3 dice-and-destiny-client/devtools/inspect_game.py`. Commands: `health`, `state`, `controls`, `errors`, `screenshot [--out]`, `activate <control_id>`, `wait …`, `play …`.
- `activate` only presses registered buttons (no coordinate clicks). `state` needs a node in group `inspectable_battle_screen` with `inspection_state()`. **Give the map screen its own inspection state and register all its controls**, including per-hex controls (or a "move to hex q,r" control), so a live run can be driven and checked.

---

## 5. What to build

### 5.1 Campaign modes

- New campaigns created from the menu are **Expedition** campaigns (add a `mode` to the save, defaulting to `expedition` for new saves).
- **Existing linear saves must keep working unchanged** (`mode` absent → `linear`), and the existing campaign tests must keep passing.
- The campaign screen opens the map for expedition saves and the encounter path for linear ones.

### 5.2 Configuration (no numbers in code)

Create `dice-and-destiny-server/content/progression_v1/expedition.yaml`. Load it strictly, in the style of `LoadCampaign` and `LoadEconomy`: `KnownFields(true)`, `schema_version`, and range validation with clear errors. It holds every rule number: rounds, hand size and limit, energy per Move, explore cost, terrain costs by Day/Night, sight, ambush bonus dice and max kept, fortified extra defense dice, defeat wounds, camp mending (roll needed, inn bonus), Warlord start distance and march per rest, the early-strike reward, Influence per deck change by site, the mending costs, gold drops and prices, and the trainer rate. `expedition.md` §14 has a sketch and the first values. *(2026-10-14: the trainer was removed; gold never buys XP. See [phase-0-handoff.md](phase-0-handoff.md), task 0.1.)*

**Per-thing data:**
- Character base energy, hand size/limit and energy carry-out: on the character definition or sheet.
- Enemy habit, XP and gold: on the encounter group definition.
- Card map effects: in the card's catalog definition.
- Region tiles, sites, groups, Lieutenants, the Warlord and his goal: in the region definition.

Suggested layout: a new content pack `dice-and-destiny-server/content/expedition_v1/` containing `tiles/`, `sites/`, `groups/`, `events/`, `gear/` and `regions/`. Every loader validates references (unknown tile, site, group, card or combatant IDs are errors).

### 5.3 Authority (Go)

Create a package (suggested `internal/battle/expedition`) that owns the map rules as pure, deterministic functions over a serializable state, plus runtime ops in `learned`.

**State** (stored inside the campaign save, written atomically with the existing locking):
- mode, region ID, **seed and RNG position**;
- round index and Day/Night;
- placed tiles (slot → tile ID);
- seen hexes and remembered contents;
- player hex;
- piles (hand, draw, spent; wounds stay in the existing `wounds` field);
- current energy;
- gold, inventory (gear, consumables);
- map statuses;
- enemy groups (position, defeated, last seen, patrol state);
- Lieutenants;
- Warlord (route, index, goal taken, buffs);
- site states (occupied, freed, closed at night, used);
- event states;
- turn state (Move spent, cards played or discarded this turn, action used);
- a pending battle context (provoked/challenge/garrison, Ambush earned, Fortified, energy brought in, origin hex);
- results.

Use a **revision** number for optimistic concurrency, like `BuyCampaign`.

**Map geometry:**
- Axial hex coordinates (q, r), pointy-top.
- A tile is a 7-hex "flower" (center plus 6 neighbors).
- Flowers tile the plane with center offsets (2,1), (−1,3), (−3,2), (−2,−1), (1,−3), (3,−2).
- A region defines its allowed tile slots (the shape) and its tile stack: start tile fixed, countryside tiles shuffled, then core tiles shuffled.
- Explore places the next stack tile into an empty slot adjacent to the player's hex.
- Generation and all random draws come from the save's seed, so the same seed always gives the same game.

**Ops** (add to `HandleRuntimeRequest`; keep the `{ok, result}` envelope):
- `expedition_status`: the **viewer-safe** view. Fog-filtered; never the tile stack, unseen contents or RNG state. It includes legal actions and their costs, so the client never computes legality.
- `expedition_act` with `kind`:
  - `move` (target hex; payment: card IDs from hand and/or energy);
  - `explore` (slot; payment);
  - `play_map_card` (card, target);
  - `use_site` (service; Influence cards; gold);
  - `event_choice`;
  - `challenge` (target group);
  - `enter_site`;
  - `discard` (to satisfy the 1-card rule);
  - `end_turn`;
  - `make_camp`;
  - `camp_service` (mend roll with bandage or Influence; deck change);
  - `deck_change` (reuse the campaign purchase rules, but allowed only at a site or camp with the Influence cost paid).

  Each action is validated by the authority, rejected with a clear error if illegal, and saved atomically.
- **Battle start:** reuse the session reset path with an expedition battle context. Provoked or forced fights start immediately as part of the action that triggered them. The battle's result is recorded once when terminal (as `recordCampaignBattle` does), and the map state updates per the bridge rules.

### 5.4 The battle bridge

- Add a wrapper `WithPresetPiles` (next to `WithCarriedWounds` in `internal/battle/deck_loadout_assembler.go`) that, for the human seat:
  - assigns card instance IDs into `Hand`, `Deck`, `Discard` and `Removed` from the map piles and wounds;
  - sets `Resources.EnergyPoints` (clamped to the cap);
  - sets `Resources.StartingHandSize = 0` so there's no opening draw;
  - adds start statuses (Ambush on the player; Fortified, Lieutenant buffs and Conqueror on enemies).
- Add matching fields to `mlsim.ResetRequest` and `ReplayRecord` so replays reproduce the start.
- **End of battle:** read the human seat's piles and energy from the terminal battle (`InspectBattleState` or snapshot compositions):
  - removed → wounds;
  - discard → Spent;
  - hand → hand;
  - deck → draw pile;
  - energy = min(end energy, energy brought in).

  On defeat, apply the defeat rule instead (pre-battle wounds + 2, never the last card; pushed back; forced rest).
- Battle rules otherwise unchanged: Income still draws 1 per round from the draw pile, nothing reshuffles, and damage order is unchanged.
- **Map cards are unplayable in battle:** they must never be legal to play, but they count as health and can be removed by damage. Add a card flag (e.g. `battle_playable: false`, or derive it from "has a map effect and no battle program") and enforce it in the engine's legal actions.

### 5.5 New battle statuses

Author each in content with presentation, rules text stating its exact expiry, polarity, stacking and lifecycle. Implement the engine behavior. Add client icons.

1. **Ambush** (player, positive, used on the first offensive roll):
   - The first planning roll rolls 5 + `ambush.bonus_dice` dice; the player keeps at most 5; later rerolls roll (5 − kept).
   - This touches `TypePlanningRoll`, `rollCombatDice`, the keep/reroll validation, legal actions, the dice event, the AI path if relevant, and the client dice UI (show 8 dice, allow choosing up to 5).
   - Keep `FinalDice` at 5 after the choice.
2. **Fortified** (enemy, persistent for the battle): the defensive roll uses +1 die (`rollSelectedDefense`); per-face outcomes handle the extra face. It's removed by Siege gear before the battle starts.
3. **Lieutenant buffs** on the Warlord (one per living Lieutenant). Use existing status operations where possible (for example a damage or prevent modifier), and record what you chose.
4. **Conqueror** (Warlord): +1 defensive die (same mechanism as Fortified).
5. **Map statuses** (e.g. "Shaken: hand size −1"), if any content uses them. They apply to both map and battle (hand symmetry).

### 5.6 Client (Godot)

New screen(s) under `dice-and-destiny-client/app/screens/map/`, built in code in the style of the campaign screens:

- **Map:**
  - Pointy-top hex map from the authority's view: tiles, terrain colors, site and enemy icons, the player pawn, Warlord and patrol markers, and fog (in-sight hexes highlighted, out-of-sight hexes greyed, "?" for never-seen contents), plus empty tile slots you can explore.
  - It must fit **1280×720** and **1920×1080**; pan if the region is larger.
- **Hover tooltips:**
  - hex: terrain, Day/Night move cost, site details, and services with costs;
  - enemy group: members, habit, Fortified, XP and gold;
  - Warlord: next step, goal, buffs.
- **HUD:**
  - round ("Day 2 · round 3 of 4");
  - the hand (each card's battle and map text, map cards marked "map only");
  - draw, Spent and wound counts, with hover lists;
  - energy (with base), gold, XP, inventory;
  - map statuses;
  - "cards played/discarded this turn: n (need ≥ 1)".
- **Actions:**
  - select a hex to see the path cost; choose the payment (cards and/or energy); confirm;
  - explore on empty slots;
  - site panel (services, Influence payment by choosing cards);
  - event dialog;
  - challenge and enter buttons;
  - end turn; make camp;
  - a rest summary (reshuffle, energy reset, mending roll results, Warlord movement, Day/Night).
- **Deck changes:** reuse `campaign_prepare.gd`, opened from a site or camp, with the site's Influence cost shown and enforced by the authority.
- **Battle:**
  - launch the existing `battle_screen.tscn` with the expedition context;
  - the campaign badge shows the region and round;
  - at the end, a result panel shows map consequences (cards returned, wounds, energy kept, rewards);
  - "Continue" returns to the map.
- **Region end:** victory (rewards, wounds mended as the existing campaign rule does on clearing) and defeat by the Warlord (run over; the save is marked ended).
- **Inspector:**
  - register every control (including a way to target any hex);
  - expose `inspection_state()` with the map view (player hex, round, piles, energy, gold, legal actions);
  - add the map screen to an inspectable group so `inspect_game.py state` and `activate` can drive a full live play-through.

### 5.7 First content set (placeholders allowed; record names as placeholders)

- **One region**, built as a start tile, 5 countryside tiles and 2 core tiles. It must contain at least:
  - 2 villages, 1 monastery, 1 town on a core tile (the Warlord's goal);
  - 2 ruins, 1 grotto, 3 event sites;
  - 1 keep with a **fortified Lieutenant**, 1 **roaming Lieutenant**;
  - minion groups: Brine Mask ×1, Brine Mask ×2, Bell Diver, Ribbon Eel — with a mix of habits so every habit appears;
  - 1 occupied village (freed by beating its garrison);
  - lake, mountain, marsh, forest, hills and plains terrain.
- **Encounter-group rule for this push:**
  - Each group is one combatant definition with 1–2 copies (the current session limit).
  - Region generation must **never** place two groups so that one hex touches both: validate this, and fail the build if it happens.
  - Mixed groups and multi-group battles are the follow-up push.
- **Lieutenants:** existing minions (e.g. Ribbon Eel, Bell Diver) with a Lieutenant status.
- **The Warlord:** no boss combatant exists. Author a **placeholder Warlord combatant** in `content/minions_v1/combatants/` following the existing minion schema. Give it a larger deck so it's tougher, and an original placeholder name. Report it clearly so the user can replace it.
- **Map cards:**
  - A **Pathfinder** tree:
    - Trailblaze (base): Move 2;
    - Forced March: Move 4 ignoring terrain costs, **costs 10 energy** — the one big energy-costing map effect;
    - Lantern Road: Move 2, ignore night terrain penalties;
    - Scout Ahead: see contents of hexes up to 3 away this turn.
  - **Both** branches on the Steady Guard tree:
    - Warden's March: battle Prevent 1 + map Move 2;
    - Sentry's Watch: battle Prevent 1 + map +1 sight this turn.
  - Take Stock gets map "draw 2"; Second Wind gets map "+10 energy".
  - Names are placeholders. Card trees are authored in the tracked `content/authored/` JSON; follow the existing format, and keep tests on the disposable authored root.
- **Gear:**
  - Bandage (consumable: one camp mend roll);
  - Lantern (+1 sight at night);
  - Siege Hooks (consumable: remove Fortified from the next garrison fight).
- **Events:** 3 scenes with 2–3 choices each. They must cover a gold reward, an XP reward, a wound or fight penalty, a choice needing Influence, and one needing a die roll.

---

## 6. Milestones

Do them in order. A milestone is complete only when its tests pass and its evidence is in the progress log.

**M0 — Baseline**
- Create a branch from `main` (e.g. `expedition-map`); `git status` must be clean.
- Run the standard checks and the existing campaign Godot tests (`verify_campaign.gd`, `verify_campaign_prepare.gd`). Record the results. If anything fails before you start, record it and tell the user; don't "fix" unrelated failures silently.

**M1 — Configuration and content loading**
- `expedition.yaml` and the `expedition_v1` pack (tiles, sites, groups, events, gear, region), with strict loaders and reference validation.
- Card schema support for map effects and battle-unplayable map cards.
- Go tests:
  - every loader accepts the shipped content;
  - every loader rejects unknown fields, bad ranges, missing references, and adjacent-group placements.

**M2 — Map rules on the authority (no battles yet)**
- All of §5.3 except battles: generation, exploring, fog, movement and payment, terrain by Day/Night, the 1-card rule, end turn and draw, rest and make camp, Warlord and patrol movement, sites and Influence, deck changes at sites, mending rolls, gold and inventory, events, and the deadline trigger.
- Persistence in the campaign save; viewer-safe `expedition_status`.
- Go tests for each rule (section 8.1), including determinism (same seed → identical state after the same actions) and save/reload equivalence.

**M3 — The battle bridge**
- `WithPresetPiles`, replay fields, end-state read-back, the energy carry-out cap, forced vs voluntary fights, "a fight ends the turn", the defeat rule, and Warlord victory/defeat (region won / run ended).
- Go tests that play **real battles** through the session (like `playCampaignBattle`) and assert:
  - exact pile and energy hand-off in both directions;
  - no opening draw;
  - Income drawing from the map draw pile;
  - damage hitting discard first;
  - map cards never legal in battle;
  - defeat wounds;
  - the carry-out cap.

**M4 — Ambush, Fortified, Lieutenant buffs, Conqueror**
- Content definitions, engine behavior and client icons.
- The full `AGENTS.md` status checklist for each (section 8.3).

**M5 — Client map screen**
- Everything in §5.6, wired to the real authority (no fake data in production paths).
- Pointer-driven Godot tests (section 8.2).

**M6 — Content complete and full-region play**
- The §5.7 content.
- A seeded full-region play-through test (automated) that reaches each ending.
- A live play-through in the graphical game (section 8.4).

**M7 — Docs and wrap-up**
- Update `docs/game-design/campaign.md` and `expedition.md` (implementation notes; leave the design decisions as decided).
- Add an "Expedition" section to `AGENTS.md`, in the style of "Campaign progression", with the rules future work must keep.
- Add follow-ups to `TODO.md` (section 9).
- Finish the progress log and write the final report (section 10).

---

## 7. Acceptance criteria (definition of done)

**Every** item must be proven by at least one automated test **and** — for anything the player sees or does — by the live play-through evidence in section 8.4. List each criterion with its proof in the final report.

**Setup and saves**
1. Starting a new campaign from the main menu creates an Expedition save. Existing linear campaign saves still open and play exactly as before (the existing campaign tests pass unchanged).
2. The same seed and the same actions produce the identical map and state. Quitting and relaunching at any point (mid-turn, mid-rest, mid-battle) restores the identical state.

**Map and turns**

3. The region starts with only the start tile revealed. Exploring costs 2 Move next to an empty slot and places the next tile from the stack (countryside before core).
4. Fog works:
   - in-sight hexes show contents (2 by day, 1 at night);
   - out-of-sight revealed hexes show greyed terrain with remembered sites;
   - never-seen contents are hidden;
   - the client payload **never** contains hidden contents, the tile stack or the RNG state.
5. Movement:
   - costs the entered hex's terrain cost for the current Day/Night;
   - is payable with cards (1 each) and/or energy (5 each);
   - lakes and mountains are impassable;
   - illegal moves are rejected by the authority with a clear error, and the UI never offers them.
6. Ordinary cards can't be converted to energy on the map. Second Wind's map effect gives +10. Take Stock's map effect draws 2.
7. A turn can't end until at least 1 card has been played or discarded this turn (a fight counts). Unplayed cards stay in hand. End of turn draws up to hand size.
8. Rest:
   - when the draw pile is empty at end of turn, or on Make camp, Spent cards are reshuffled into the draw pile;
   - wounds stay out;
   - energy resets to base (up or down);
   - camp services are offered;
   - the Warlord moves 2 hexes and patrols step;
   - Day ↔ Night flips;
   - the round counter advances.

   After round 4 the Warlord fight starts.
9. Night rules: forest costs 3, sight is 1, towns and villages are closed except the inn, grottos and ruins have better loot and stronger guardians, and night hunters roam per turn.

**Battles**

10. A battle starts with exactly the map hand (no opening draw), the map draw pile, Spent as discard, wounds as removed, and the current energy. Income draws from that draw pile, nothing reshuffles, and damage takes discard first.
11. After a battle:
    - the piles come back exactly as the battle left them;
    - energy is min(end, brought in);
    - the turn is over;
    - victory awards the group's XP and gold from the authority only.
12. Provoked fights start immediately and never have Ambush. Challenges and garrison entries are the player's action.
13. Ambush is granted only when the player starts the fight, unseen (by the turn-start sight rule or cover), and the target's habit allows it. Its effect is exactly 8 dice on the first offensive roll, keep at most 5, rerolls back to 5 total, max 3 rolls. Each habit is tested in both Day and Night.
14. Fortified defenders roll +1 defensive die. Siege Hooks remove it. Conqueror gives the Warlord +1 defensive die. Each living Lieutenant's buff is on the Warlord, and killing the Lieutenant removes it.
15. All new statuses pass the `AGENTS.md` checklist: acquisition, status-bar icon, count and full rules on hover (including expiry), correct target, no repeat use, persistence through save/reload, and correct expiry through the real phase transition — by authority command **and** by pointer.
16. Map cards can't be played in battle (never in legal actions, and the UI doesn't offer them), but they count as health and can be removed by damage.
17. Losing a normal battle:
    - restores the pre-battle wounds plus 2 (never the last live card);
    - moves the player back to the origin hex;
    - leaves the enemy in place;
    - forces a rest.

    Losing to the Warlord ends the run and marks the save ended.

**Economy and sites**

18. Influence is paid only with cards played at the site that turn. Deck changes are only possible at sites or camp, cost XP plus the configured Influence (camp 3, village 2, monastery 1, town 0), and use the existing purchase rules (tree cards only, expected cost, revision).
19. Mending:
    - camp: one d6 roll per wound per bandage (or 3 Influence), success on 4+ (+1 at an inn);
    - village: 2 Influence per wound;
    - monastery: 1 per wound, usable mid-round;
    - town healer: gold.

    Results come from the seeded RNG on the authority.
20. Gold comes only from the authority (enemies, treasure, events). Shops sell the gear. The trainer converts 3 gold → 1 XP. Gold can't buy or upgrade cards directly. *(2026-10-14: the trainer was removed; gold never buys XP. See [phase-0-handoff.md](phase-0-handoff.md), task 0.1.)*
21. Event sites present their scenes. Each choice's requirements and outcomes are enforced by the authority.

**The Warlord**

22. The Warlord follows his route 2 hexes per rest. Extra rests (Make camp, defeats) speed him up. He can be challenged on the road. Reaching his goal fortifies him there and adds Conqueror. Beating him early awards +20 XP per unused round plus the extra relic choice (or a recorded pending reward).

**Configuration**

23. Every number in `expedition.md` §14 is read from configuration. Changing a value in `expedition.yaml` (verify at least: rounds per region, hand size, energy per Move, terrain cost, defeat wounds) changes behavior **without code changes**, proven by tests that load modified config.

**Client quality**

24. The map screen, HUD, site and event panels, rest summary and battle result panel are readable and fully inside the viewport at **1280×720 and 1920×1080**: no overlapping or clipped text. Proven by screenshots that you **open and look at**.
25. Every action in criteria 3–22 can be done with the **mouse** in the real game (pointer tests), and every control is registered with the inspector.

**Regression**

26. `cd dice-and-destiny-server && go test ./...` passes; `./scripts/godot.sh --headless --script res://scripts/verify_battle_authority.gd` passes; all existing Godot campaign and status tests pass; `git diff --check` is clean; no files under the tracked `content/authored/` were written by tests.

---

## 8. Testing and validation requirements

**You must physically run everything. Do not assume code works because it compiles or looks right.** A criterion is proven only by a test you ran and saw pass, or by evidence you captured and inspected. If a test fails, fix the cause; never weaken, skip or delete a test to make it pass. If something can't be verified, say so plainly in the final report — don't claim it.

### 8.1 Go tests (authority)

- Unit tests for every rule in section 3, with **seeded** RNG, covering:
  - movement costs Day/Night and payment combinations;
  - impassable terrain;
  - explore placement and stack order;
  - fog and viewer safety (assert hidden fields are absent from the JSON);
  - the 1-card rule;
  - end-turn draw;
  - rest (every step);
  - Make camp;
  - the round limit and deadline;
  - Warlord march, speed-up and goal capture;
  - patrols and night hunters;
  - habits × Day/Night × unseen/cover for Ambush;
  - provoke detection;
  - site services and Influence;
  - deck change costs by site;
  - mending odds and inn bonus;
  - gold drops and prices;
  - the trainer rate; *(2026-10-14: the trainer was removed; gold never buys XP. See [phase-0-handoff.md](phase-0-handoff.md), task 0.1.)*
  - events;
  - defeat rule edge cases (last card, already heavily wounded);
  - the energy carry-out cap;
  - config-driven behavior (load a modified config and assert the change).
- Integration tests that play **real battles** through the session and check the bridge in both directions.
- A **seeded full-region test** that plays from new campaign to Warlord victory, and another to Warlord defeat, asserting state at each step.
- **Save/reload:** serialize and reload after every action in the full-region test, and assert that state and legal actions are identical.

### 8.2 Godot tests (client, pointer-driven)

- New tests under `dice-and-destiny-client/tests/presentation/`, e.g. `verify_expedition_map.gd`, `verify_expedition_battle_bridge.gd`, `verify_expedition_statuses.gd`. Follow `verify_campaign.gd`: real `_click` pointer events, controls found by metadata, `root.size` set, battles played through the gateway, and a `PASSED` line plus exit code.
- Cover with the **mouse**:
  - new campaign → map;
  - select a hex, pay with chosen cards and energy, move;
  - explore;
  - a site service with Influence;
  - an event choice;
  - challenge → the real battle screen → result panel → Continue → map shows returned piles;
  - a provoked fight;
  - end turn (blocked until 1 card is used);
  - Make camp and the rest summary;
  - deck change at a site via Prepare;
  - the Warlord fight;
  - region victory and run-ended screens.
- Run each at 1280×720 and 1920×1080 in **graphical** mode with screenshots enabled (env-gated, like `_capture`). **Open the PNGs and check them**: nothing clipped, overlapping or outside the viewport. Record what you checked in the progress log.

### 8.3 Status checklist (per new status)

For Ambush, Fortified, each Lieutenant buff, Conqueror, and any map status, prove:
- it's gained in the right situations, and not in the wrong ones;
- the status-bar icon and count;
- the hover shows the full rules, including expiry;
- it applies to the right target;
- it can't be reused;
- it persists through save/reload;
- it expires at the real phase transition;
- all of the above by authority command **and** by pointer.

### 8.4 Live play-through (required evidence)

- Start the real game: `DICE_AND_DESTINY_INSPECTOR=1 ./scripts/godot.sh` from the repo root, in the normal workspace (not a script run).
- Use `python3 dice-and-destiny-client/devtools/inspect_game.py` (`health`, `state`, `controls`, `activate`, `screenshot --out …`, `errors`) to play a full region:
  - new expedition;
  - at least one explore, site service, event, deck change at a site, ambushed fight, provoked fight, and defeat with forced rest;
  - a night round;
  - Make camp;
  - the Warlord fight.
- Take a screenshot at each step. Save them in an ignored folder under `dice-and-destiny-client/.godot/runtime/` (e.g. `.godot/runtime/expedition-evidence/`). **Look at every screenshot.**
- After each step, use `inspect_game.py state` to confirm the authority state matches what the screen shows.
- `inspect_game.py errors` must show no new errors.
- If the inspector can't do something, extend the inspector registration on your screens. Don't skip the step.

### 8.5 Standard checks

Run them at the end of every milestone and before the final report:

```bash
cd dice-and-destiny-server && go test ./...
cd .. && ./scripts/godot.sh --headless --script res://scripts/verify_battle_authority.gd
./scripts/godot.sh --headless --script res://tests/presentation/verify_campaign.gd
./scripts/godot.sh --headless --script res://tests/presentation/verify_campaign_prepare.gd
# plus every new expedition test
git diff --check
```

---

## 9. Out of scope (record in `TODO.md` as follow-ups)

- **Mixed enemy groups and more than 2 enemies per battle**, including two groups pulled into one battle. The engine supports multiple seats; the session, encounter model, mlsim observations and client layout don't. This is a separate push.
- Grievous wounds (crippled cards).
- Companions and recruiting.
- More regions, the between-region camp/healer stop, and run-scoped relics (if not already present; store pending relic rewards).
- Map abilities on more cards and trees, and gear cards beyond the first set.
- Replacing the placeholder Warlord, and real art for tiles and sites.
- Content versioning for campaign saves (already in `TODO.md`).

---

## 10. Work process and final report

**Process:**
1. Read section 2 and `AGENTS.md` fully. Inspect `git status` and preserve anything present.
2. Work on your own branch. Commit locally at the end of each verified milestone with a clear message. **Never push** unless the user asks.
3. Build in thin vertical slices with tests first where practical. Keep the authority pure and deterministic.
4. After each milestone: run section 8.5, update the progress log, and review your own diff (`git diff`) for leftovers, debug code and accidental changes to tracked authored content.
5. If you discover the design can't work as written, stop and report with a concrete proposal rather than inventing a different design.

**Final report** (to the user, and copied into the progress log):
1. **Verdict first:** is a full region playable end to end in the real game with the mouse?
2. A table of every acceptance criterion (section 7) → the test name(s) and the live evidence (screenshot paths) that prove it, marked PASS, or FAIL/NOT VERIFIED with the reason.
3. The commands you ran and their exact results.
4. Decisions made during implementation, placeholders created (Warlord, card names), and anything that differs from `expedition.md`.
5. Known issues and follow-ups (section 9).
