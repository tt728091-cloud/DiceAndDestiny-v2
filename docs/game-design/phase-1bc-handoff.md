# Phase 1B + 1C handoff: the world clock and the quest format

**Status:** ready to implement (written 2026-10-18).
**Plan of record:** [`open-world-plan.html`](open-world-plan.html), §4 "Time: months and seasons" and §6 "Towns, quests and prosperity". Phase 1 is the second step of its roadmap; Phase 0 is done (commits `2349150`–`572b7da`).
**Scope:** 1B (the world clock and calendar) and 1C (the quest format, on paper). **1A (the XP price table and balance) is being done in other threads; don't touch it.**

---

## Prompt for the implementing model

> You are working in the Dice and Destiny repository. Read `AGENTS.md` first and follow it exactly, especially: run Godot only through `./scripts/godot.sh`, keep the Go server the authority, keep every number in configuration, and never let the view leak hidden state.
>
> Then read `docs/game-design/phase-1bc-handoff.md` and implement Phase 1B (the world clock and calendar) and Phase 1C (the quest format, on paper), with every test the handoff lists. Work on a new branch named `phase-1-clock-and-quests` created from local `main`. Commit each numbered task separately with a clear message. Do not push or merge unless the user asks.
>
> Before changing code, confirm each "where it lives" pointer in the handoff against the current code; line numbers may have moved. The handoff makes a few design choices (marked **Decision**). Follow them. If one contradicts the code, `AGENTS.md` or the plan page, stop and report it rather than guessing.
>
> Don't touch the 1A work: price tables, `cmd/balance-lab`, `cmd/card-balance`, `cmd/battle-review`, `internal/battle/lookahead`, the look-ahead policy files, or card/enemy prices.
>
> When you finish, run the full verification list in the handoff, do the live check, and reply with the final report it describes.

---

## Background (read this first)

**What exists today.** The opening scenario is the Salt Marches Expedition (milestones M0–M19; see `docs/game-design/expedition.md` and `expedition-progress.md`). All of its rules live in `dice-and-destiny-server/internal/battle/expedition/`, and its numbers in `content/progression_v1/expedition.yaml` and the region pack `content/expedition_v1/`. The Godot map (`dice-and-destiny-client/app/screens/map/expedition_screen.gd`) only renders the viewer-safe `View` and submits actions through the `expedition_status` / `expedition_act` / `expedition_deck_change` ops (`internal/battle/learned/runtime.go`, handlers in `learned/campaign.go`).

**Time today.** `State` (`expedition/state.go`) stores `Round int` and `Night bool`. A round is one pass through the deck. Round 1 is Day; every rest (`expedition/actions.go`, step "5. Day <-> Night") does `s.Round++` and flips `s.Night`. After `rounds_per_region` (4) rounds plus any `ExtraRounds` (the `extend_time` map card), the deadline fight starts. `View` exposes `round`, `rounds`, `night` and `day_label` ("Day 1 · round 1 of 4"); the chart cartouche shows "Day · 1/4" (`chart_day_label()`).

**What the plan decided about time (§4):**
- One round = one pass through your deck = the Day or the Night half of a month. A month is a Day round plus a Night round; a year is 12 months (24 rounds).
- Four seasons of three months each. Seasonal effects (terrain, sight, events) come in **Phase 4**; Phase 1 adds only their configuration skeleton.
- Time runs the same everywhere. Every time-based rule counts the same world clock.
- The opening scenario starts in late summer and runs into autumn (4 rounds = 2 months).
- World time is stored as a world clock, not inside one hero's deck, so a later multi-hero save doesn't need a new format.

**What the plan decided about quests (§6):** every quest is a small graph, `quest → stages`, `stage → objective · timer · outcomes`, `outcome → next stage or end + consequences`. Quests are data from a fixed vocabulary of verbs, like card programs. Phase 1C writes the format, a strict loader and validator, describes the Salt Marches in it plus two paper quests, and shows them in a read-only graph view. **Nothing runs from quest data yet**: the scenario's Go rules stay exactly as they are.

---

## Phase 1B: The world clock and calendar

### Task 1B.1: Calendar configuration

**Where it lives:** `content/progression_v1/expedition.yaml`; the strict config loader in `expedition/config.go` (`Config`, its range checks, `SightRange`); `expedition/config_test.go`.

**Do:** add a `calendar:` section to `expedition.yaml`, loaded strictly like everything else.

```yaml
# The world clock: one round is the Day or the Night half of a month.
# Seasonal effects arrive in Phase 4; until then every season's effects list is empty.
calendar:
  seasons:
    - {id: spring, name: Spring, rules_text: "...", effects: []}
    - {id: summer, name: Summer, rules_text: "...", effects: []}
    - {id: autumn, name: Autumn, rules_text: "...", effects: []}
    - {id: winter, name: Winter, rules_text: "...", effects: []}
  months:                      # in calendar order; month 1 opens the year
    - {id: early_spring, name: Early Spring, season: spring}
    - {id: mid_spring,   name: Midspring,    season: spring}
    - {id: late_spring,  name: Late Spring,  season: spring}
    - {id: early_summer, name: Early Summer, season: summer}
    # … through late_winter: 12 months, three per season
```

- **Decision: month names are season-based placeholders** ("Early Summer", "Midsummer", "Late Summer", …). They read clearly and avoid inventing a fantasy calendar now; they're configuration, so the user can rename them later.
- **Decision: the year starts in spring.** Month 1 = Early Spring, so Late Summer is month 6.
- `rules_text` is what the season tooltip shows. For now, write one short sentence of flavour plus "No seasonal map effects yet." Don't promise effects that don't exist.
- `effects` is the Phase 4 hook. The loader must refuse any non-empty `effects` list with a clear error ("seasonal effects arrive in Phase 4"). That keeps the skeleton honest: nothing in the config can claim an effect the engine doesn't apply.
- Validation: unique season and month ids; every month names a known season; every season has at least one month; at least one month; names non-empty. Don't hard-code 12 or 3 in the loader. The shipped-content test asserts 12 months, three per season.

**Tests (`config_test.go`):** the shipped config loads with 12 months and four seasons; a config with an unknown season on a month, a duplicate id, an empty month list or a non-empty `effects` list is refused; an unknown key under `calendar` is refused by the strict loader.

### Task 1B.2: The world clock in the authority state

**Where it lives:** `State` in `expedition/state.go`; `New` in `expedition/generate.go`; the rest in `expedition/actions.go` (the round advance and `dayName`); `s.rounds(r)` in `map_cards.go`; every reader of `s.Round` / `s.Night` (grep `s\.Round\|s\.Night` in the package; there are about twenty, in `battle.go`, `generate.go`, `actions.go`, `view.go`, `state.go`).

**Do:**

1. Add a world clock as its own object in the save, separate from the hero's piles:

   ```go
   // WorldClock is world time, not the hero's: it counts half-months since the
   // world began (0 = Day of month 1, year 1). Odd halves are Night.
   type WorldClock struct {
       Half int `json:"half"`
   }
   ```

   with methods that read it through the calendar config: `Year()`, `Month(cfg)` (the month entry), `Season(cfg)`, `Night()`. Put the arithmetic in one small file (`expedition/clock.go`) so later phases (quest timers, critical-wound durations, Dark Tiers) all call the same code.

2. `State` gets `Clock WorldClock \`json:"clock"\`` plus `RegionStart int \`json:"region_start"\``: the clock half at which this region's first round began.

3. **Replace the stored round counter.** `Round` and `Night` stop being stored truths and become derived:
   - `s.round() = s.Clock.Half - s.RegionStart + 1` (the scenario's round; it stays the deadline counter, still compared against `s.rounds(r)`);
   - `s.night() = s.Clock.Night()`.

   Swap every `s.Round` / `s.Night` reader for the derived value. Records that stamp a round (`Wound.Round`, `CampState.Round`, `EventResult.Round`, the pending battle ID) keep their current JSON fields and values, now computed from the clock.

4. The rest advances time through exactly one function, `s.advanceClock(r)`, which increments `Clock.Half` by one. Leave a comment saying the trigger (one hero's deck pass) is the single-hero rule, so multi-hero play can change the trigger without touching the format.

5. **The region's start date lives on the region.** Add `calendar_start: {month: late_summer}` to `content/expedition_v1/regions/salt_marches.yaml` and to the region loader in `pack.go`. A region always starts on a Day half; the loader refuses an unknown month. `New` sets `Clock.Half` to the first Day half of that month in year 1, and `RegionStart` to the same value.

6. **Old saves still load.** A save written before this change has `round`/`night` and no `clock`. On load, if `clock` is absent, set `RegionStart` from the region's `calendar_start` and `Clock.Half = RegionStart + round - 1`. Night is then consistent automatically: round 1 was always Day and every rest flipped it. If a legacy `night` value disagrees, trust `round` and log nothing to the player. Drop the legacy fields after migration (they're read-only, never written). Follow the pattern of `TestLegacySaveWithFlatWoundsHealsAndFights` in `healing_test.go` and the `wounds migrated` path in `view.go`.

7. **Don't change random-number consumption.** The clock must never draw from the RNG. The seeded full-region runs in `internal/battle/learned/` and the expedition tests must keep their exact outcomes; if one changes, the refactor changed a rule, so find out why rather than picking a new seed.

**What the Salt Marches look like afterwards (asserted by tests):**

| Round | Clock | Shown as |
| --- | --- | --- |
| 1 | Late Summer, Day | Late Summer · Day |
| 2 | Late Summer, Night | Late Summer · Night |
| 3 | Early Autumn, Day | Early Autumn · Day |
| 4 | Early Autumn, Night | Early Autumn · Night |
| deadline | Midautumn, Day | The deadline (Midautumn) |

An `extend_time` card simply lets the clock run one half further before the deadline.

**Tests (new `clock_test.go`, plus edits):**
- Clock arithmetic: half 0 is Year 1, Early Spring, Day; half 23 is Late Winter, Night; half 24 is Year 2, Early Spring, Day; the season of every month matches the config.
- A new Salt Marches expedition starts at Late Summer, Day, round 1; after each rest the clock and round follow the table above; the deadline still fires after round 4 (and after round 5 with `extend_time`).
- A legacy save JSON (no `clock`, `round: 3`, `night: false`) loads at Early Autumn, Day, round 3, and plays on normally through a rest.
- A save round-trip preserves `clock` and `region_start`.
- Every existing expedition and learned test still passes unchanged, including `TestViewNeverLeaksHiddenState` and the seeded full-region runs.

### Task 1B.3: The clock in the view and on the map

**Where it lives:** `View` and `State.View` in `expedition/view.go` (`DayLabel`, the `day := (s.Round + 1) / 2` block); the client's `chart_day_label()`, `_render()` (the save label tooltip), the rest overlay heading (`"Night falls" / "Day breaks" · round %d`) and the log line `"%s %d begins."` in `actions.go`.

**Do:**

1. Keep `round`, `rounds`, `night` and `day_label` in the view (the client and tests read them). Add a `clock` object:

   ```json
   "clock": {"year": 1, "month": "late_summer", "month_name": "Late Summer",
             "season": "summer", "season_name": "Summer",
             "season_rules": "…", "season_effects": [], "night": false,
             "label": "Late Summer · Day"}
   ```

   `season_effects` is always a list, never null (AGENTS.md: view lists are never null).
2. `day_label` becomes "Late Summer · Day · round 1 of 4"; at the deadline it stays "The deadline".
3. The rest's log line and the `RestSummary` say which half begins, e.g. "Night falls in Late Summer." / "Early Autumn begins." Add `month_name_after` (or the whole clock) to the rest summary so the overlay doesn't compute it.
4. **The map shows the calendar** (plan §4 acceptance: "the current month, season and Day/Night are always visible, and each season's effects are shown with their rules"):
   - The chart cartouche's day line becomes "Late Summer · Day · 1/4". Keep the sun/moon glyph.
   - Hovering the cartouche shows a tooltip with the month, the season, the year, Day or Night with today's sight and the terrain reminder already shown in the legend, and the season's `rules_text` and effects list ("No seasonal map effects yet"). Use the existing tooltip style of the map; keep it inside the viewport.
   - The rest overlay heading names the new half and month, e.g. "Night falls · Late Summer · round 2".
   - The client renders only the view's strings; it never computes the month or season.
5. Include `clock` in the screen's inspector state dictionary (`expedition_screen.gd`, the block that already lists `round`, `night`, `day_label`, `chart_day`), so the live check can read it.

**Tests:**
- Go: the view of a new expedition has the `clock` object with the values above; `day_label` matches; `season_effects` is `[]`, not null.
- Godot: update the three cartouche assertions (`tests/presentation/verify_expedition_chart.gd` ~line 78; `verify_expedition_map.gd` ~lines 393 and 592) to the new label, and add checks that (a) the cartouche reads "Late Summer · Day · 1/4" at the start, (b) after two rests it reads "Early Autumn · Day · 3/4", and (c) the cartouche tooltip contains the month, the season and the season's rules text. Drive the rests through the real pointer path the test already uses, not authority shortcuts.

### Task 1B.4: Docs for the clock

- `docs/game-design/expedition.md` §5 "The clock": describe the world clock, the calendar config, the region's `calendar_start`, and the table above. Add a dated line to §17's decision log ("2026-10: the round counter is derived from a world clock; the Salt Marches start in late summer").
- `docs/game-design/expedition-progress.md`: add numbered implementation notes for the clock (the half-month unit, the legacy migration, no RNG use).
- `AGENTS.md`, Expedition section: one bullet: time is a world clock (`calendar` in `expedition.yaml`, `calendar_start` on the region); every time-based rule reads it; seasonal effects are empty until Phase 4 and the loader refuses them.

---

## Phase 1C: The quest format, on paper

**Description only.** The loader, validator and graph view exist so the format can be judged. No rule, timer, reward or view field of the running expedition may read quest data in this phase.

### Task 1C.1: The format and the vocabulary

**Where it lives (new):** quest files in `dice-and-destiny-server/content/quests_v1/`, with one subfolder per set (`salt_marches/`, `paper/`); a new Go package `internal/battle/quest/` (loader, validator, describer). The quest package may import `expedition` (for its `Pack` and `Config`, to check references); `expedition` must not import `quest`.

**One quest per file.** Use the strict YAML style of the region pack (`schema_version: 1`, `decoder.KnownFields(true)` as in `expedition/pack.go`). A sketch; the exact field names are yours as long as the doc you write in 1C.5 matches:

```yaml
schema_version: 1
id: stop_the_drowned_regent
name: Stop the Drowned Regent
set: salt_marches
kind: main                 # main | side | small | personal
status: scenario           # scenario = describes rules the Go code runs today; paper = design only
region: salt_marches       # optional; checked against the region pack
roles: {}                  # template roles for generated worlds, e.g. {person: {type: person}, site: {type: site}}
hook:    {verb: scenario_start, text: "The Drowned Regent marches on Greywharf."}
trigger: [{verb: scenario_start}]
start: stop_him
stages:
  - id: stop_him
    text: "Beat the Regent before the last night ends."
    objective: {verb: defeat_group, group: drowned_regent}
    timer: {type: urgent, rounds: {config: rounds_per_region}}
    outcomes:
      success:
        text: "The Regent falls; the world opens."
        consequences:
          - {verb: config_reward, key: warlord.early_reward, per: unused_round}
          - {verb: open_world}
        next: end
      failure:
        text: "You lose the deadline fight."
        consequences: [{verb: end_run}]
        next: end
```

**The vocabulary.** Every hook, trigger, objective, timer and consequence is a verb from a fixed list the validator knows. Start with these, then add only what the five Salt Marches files and the two paper quests actually need:

| Part | Verbs (starting list) |
| --- | --- |
| Hook | `scenario_start`, `rumour`, `request` (from a person or role), `event`, `raid`, `quest_failed` |
| Trigger | `scenario_start`, `season` / `month`, `reputation_tier`, `prosperity`, `site_state`, `quest_outcome` |
| Objective | `defeat_group`, `liberate_site`, `reach_site`, `resolve_event`, `deliver_item`, `be_at_by`, `spend_influence`, `survey_tiles`, `clear_dungeon` |
| Timer | `none`; `seasonal` (`before: <season>`); `urgent` (`rounds: N` or a config reference, counted `from: quest_start` or `stage_start`) |
| Outcome | `success`, `failure`, `abandoned`; each with `text`, `consequences` and `next: <stage id> \| end` |
| Consequence | `xp`, `gold`, `gear`, `reputation`, `prosperity`, `town_condition` (with `until`), `remove_enemy_status`, `site_state`, `spawn`, `start_quest`, `open_world`, `end_run`, `config_reward`, `event_outcome` |

Rules for the format:
- **Decision: refer, don't copy.** When a number or text already lives in `expedition.yaml` or the region pack, the quest refers to it (a config key, a group/site/event id, an event choice id) instead of repeating it. The validator resolves every reference. This keeps the scenario files from drifting when the scenario is tuned. Paper quests have no existing source, so their numbers are written in place and marked as placeholders.
- Timers count world-clock halves (rounds), the clock from 1B. `seasonal` names a season id from the calendar config.
- Roles: a paper quest declares roles (`{person}`, `{captor}`, `{site}`, `{town}`) and uses them as `role: person` in verbs. Hand-made scenario quests use real ids and no roles.
- Each verb in the vocabulary carries an engine status in the Go code ("run by today's scenario code" or "not yet implemented"). The describer and the graph view show it, so nobody mistakes a paper verb for a working one.

### Task 1C.2: Strict loader and validator

**Do:** `quest.LoadQuests(root string, cfg expedition.Config, pack expedition.Pack) (Catalog, error)`. Like the region pack, any problem refuses the whole catalog with an error naming the file and the field.

The validator refuses:
- unknown fields, unknown verbs, missing required fields, a wrong `schema_version`;
- duplicate quest ids, duplicate stage ids within a quest;
- a `start` or `next` that names no stage; a stage no path from `start` reaches; a cycle (**Decision:** no cycles in Phase 1; a retry is a new stage);
- a stage with no `success` outcome; a stage with a timer but no `failure` outcome (a failure may simply `next: end`);
- a reference to an unknown region, site, group, event, event choice, season, month or config key, or a `config` reference whose value isn't a number;
- a role used but not declared, or declared but never used; a role on a `status: scenario` quest;
- an `urgent` timer with no rounds, a `seasonal` timer with an unknown season.

**Tests (`internal/battle/quest/`):** one small invalid fixture per rule above, each refused with an error that names the problem; the shipped catalog loads (`TestShippedQuestsLoad`); the loader never writes anything.

### Task 1C.3: Describe the Salt Marches and two paper quests

**Salt Marches files (`content/quests_v1/salt_marches/`, `status: scenario`):**
- `stop_the_drowned_regent`: main quest; urgent, `rounds_per_region` rounds; success = beat the Regent (early-strike reward per unused round by config reference, then `open_world`); failure = lose the deadline fight → `end_run`.
- One side quest per Lieutenant: today the groups with `lieutenant: true` are `bell_marshal` and `eel_warden`. Success removes one stack of the Lieutenant's Favor from the Warlord (`remove_enemy_status`, referring to `statuses.lieutenant` in config; see `livingLieutenants` in `expedition/battle.go`). No timer; when the main quest ends, they end as `abandoned`.
- `free_saltreed`: liberate `saltreed_village` (its `occupied_by` group); success frees it so it trades like any village.
- Three small quests for the event sites (`shrine_site` → `wayside_shrine`, `barge_site` → `wrecked_barge`, `hermit_site` → `hermits_fire`): objective `resolve_event`; each choice of the event file becomes a labelled outcome edge whose consequence is `event_outcome` referring to that choice. Don't copy the gold/XP/wound numbers.

**Paper quests (`content/quests_v1/paper/`, `status: paper`):**
- `kidnapped_daughter`: roles `{person}` (her father), `{captive}`, `{captor}` (a group), `{site}` (the captor's lair), `{town}`. Hook: a request from `{person}`. Stage 1: rescue her (`defeat_group` `{captor}` at `{site}`), urgent, 3 rounds from stage start (placeholder). Success: reputation with `{person}` and prosperity for `{town}`. **Failure branch:** stage 2, she has become a werewolf and roams; hunt her (or an authored alternative) with its own success and failure outcomes. This is the example that proves failure branches work.
- `spring_farms`: roles `{town}`, `{farms}` (a site), `{infestation}` (a group). Hook: a request from the farmers. Trigger: autumn or winter. Stage: clear the farms, seasonal timer `before: spring`. Success: prosperity. Failure (plain, no branch): `town_condition: food_shortage`, prices rise `until: next_season`.

Mark every paper number as a placeholder in a YAML comment.

**Drift test (`TestSaltMarchesQuestsMatchScenario`):** the main quest's timer resolves to `rounds_per_region`; the Lieutenant side quests cover exactly the groups with `lieutenant: true`; the liberation quest matches the sites with `occupied_by`; the event quests cover every event site and every choice of each event. If someone adds a Lieutenant or an event choice to the scenario, this test fails until the quest files follow.

### Task 1C.4: The read-only graph view

**Server:**
- A describer in the quest package turns verbs into display text ("Defeat the Bell Marshal", "Urgent: 4 rounds", "+30 XP per unused round (warlord.early_reward)", "Not yet implemented" badges). The client never interprets verbs; it shows these strings.
- A new read-only runtime op `quest_catalog` (dispatch in `learned/runtime.go`, next to the expedition ops; handler alongside them in `learned/campaign.go` or a new `learned/quests.go`). It loads the expedition config, pack and quest catalog from the content root and returns `{sets: [...], quests: [...]}`; each quest has its header (id, name, kind, status, roles, hook, trigger) and a graph: `nodes` (stages plus end nodes, each with title, objective text, timer text, engine-status badges) and `edges` (from, to, outcome, label, consequence texts). Every list is a list, never null. It reads only content, never a save.
- Note: other threads have small uncommitted edits in `runtime.go` and `battle_bootstrap.gd` in the main checkout. Keep your edits to those files minimal and self-contained so the merge stays easy.

**Client (new `dice-and-destiny-client/app/screens/quests/quest_graph_viewer.gd`):**
- Open it from Character Creation, which is the admin editor: add an entry beside the card tree and ability workshops (`character_creation.gd`, the tab handling ~line 211 and `_open_card_trees()` ~line 1533). If you find a better admin home, use it and say so in the report. It doesn't go on the main menu.
- Left: the quest list grouped by set (Salt Marches, Paper), with kind and status chips. Right: the selected quest's graph, laid out left to right by depth, branches stacked vertically. Stage cards show the title, objective and timer chip; edges are coloured by outcome (success, failure, abandoned) and labelled. Clicking or hovering a stage shows its full text, outcomes and consequences. Read-only: no editing, no saving.
- Reuse what fits from the card tree canvas (`app/screens/character/card_tree_canvas.gd`, `card_tree_layout.gd`) and the shared style (`character_style.gd`) rather than inventing a new look.
- Must work at 1920×1080 and 1280×720: no overlapping cards, no text outside its card, the graph pans or scrolls when it's larger than the area.

**Tests:**
- Go: the op returns every shipped quest; the kidnapped daughter's graph has a `failure` edge from the rescue stage to the werewolf stage; no list in the response is null.
- Godot (new `tests/presentation/verify_quest_graph.gd`, headless): open the viewer through Character Creation with the pointer, check that both sets and all shipped quests are listed, select the kidnapped daughter and assert the failure edge leads to the werewolf stage node, select the main quest and assert its timer chip says urgent with 4 rounds, and assert no two stage cards overlap and every card is inside the viewer.

### Task 1C.5: Docs for the quest format

- New `docs/game-design/quest-format.md`: the file layout, every field, the full vocabulary table with each verb's parameters and engine status, the validator's rules, the "refer, don't copy" rule, and the three example quests. It's the reference the future quest editor (Phase 5E) is built from.
- `docs/game-design/expedition.md`: one paragraph pointing to the quest description of the scenario, saying the Go rules still run it.
- `AGENTS.md`: a short "Quests" bullet: quest files live in `content/quests_v1/`, are loaded strictly by `internal/battle/quest`, are description only until Phase 5, and refer to scenario numbers rather than copying them.

---

## Verification

Run all of these from the repository root, following `AGENTS.md`. The launcher rebuilds stale native artifacts after Go changes.

```bash
cd dice-and-destiny-server && go test ./...
cd .. && ./scripts/godot.sh --headless --script res://scripts/verify_battle_authority.gd
cd dice-and-destiny-server && go test ./internal/battle/expedition/ ./internal/battle/learned/ ./internal/battle/quest/
cd .. && ./scripts/godot.sh --headless --script res://tests/presentation/verify_expedition_map.gd
./scripts/godot.sh --headless --script res://tests/presentation/verify_expedition_statuses.gd
./scripts/godot.sh --headless --script res://tests/presentation/verify_expedition_chart.gd
./scripts/godot.sh --headless --script res://tests/presentation/verify_quest_graph.gd
```

Notes:
- Use headless runs for verdicts. Windowed Godot tests are sensitive to the real mouse moving over the window.
- If a test was already failing on `main` before your work, check it on a clean `main` checkout, say so in the report and don't count it as a regression.
- Seeds used by the region tests are fixed. The clock must not change any seeded outcome (it uses no randomness). If one changes, find the rule you changed by accident.

**Live check:** start the game with the inspector (`DICE_AND_DESTINY_INSPECTOR=1 ./scripts/godot.sh`):
- start a new expedition and confirm the cartouche reads "Late Summer · Day · 1/4" and its tooltip shows the season and its rules;
- play to the second rest and confirm "Early Autumn · Day · 3/4";
- load an expedition save made before this change (or a test fixture copied into a campaign save) and confirm it shows the right month for its round;
- open the quest graph viewer from Character Creation, view the main quest and the kidnapped daughter.

Save screenshots in the ignored evidence folder `dice-and-destiny-client/.godot/runtime/expedition-evidence/`.

---

## Acceptance criteria

1. The world clock is a separate object in the save; the round and Day/Night are derived from it; the rest advances it through one function; nothing else stores time.
2. The Salt Marches start in Late Summer on a Day half and follow the table above; the deadline still comes after round 4 (or later with `extend_time`).
3. Saves written before this change load with the correct clock and play on.
4. The calendar (months, seasons, rules text, empty effects) lives in configuration; the loader refuses non-empty seasonal effects and malformed calendars.
5. The map always shows the month, season and Day/Night, and the tooltip shows the season's rules; the client computes none of it.
6. All shipped quests (the seven Salt Marches files and the two paper quests) load through the strict loader; every validator rule has a failing fixture; the drift test ties the scenario quests to the live scenario content.
7. The read-only graph view shows every quest's stages and labelled outcome edges, including the daughter's failure branch.
8. No expedition rule, view field or test outcome depends on quest data.
9. Every command in the verification list passes, apart from any pre-existing failure you've proven exists on `main`.
10. `expedition.md`, `expedition-progress.md`, `quest-format.md` and `AGENTS.md` match the new behaviour.

## Out of scope

- 1A: price tables, enemy budgets, the balance lab, the look-ahead player, fight logs.
- Seasonal effects themselves (Phase 4), the story skip to late fall and the hometown (Phase 3 / start of Phase 4).
- Running quests: timers that tick, quest logs, branches that change the world, reputation, prosperity (Phase 5).
- Moving the scenario's Go rules onto quest data.
- A quest editor (Phase 5E).
- Pushing or merging the branch.

## Final report

Reply with:
1. a summary of each task (1B.1–1B.4, 1C.1–1C.5) and its commit;
2. the test commands you ran, with pass/fail results;
3. the final quest vocabulary, and any verb you added beyond the starting list and why;
4. anything you found that contradicts this handoff, the plan page or `AGENTS.md`, and how you handled it;
5. any design choice you had to make that this handoff didn't cover;
6. the paths to the live-check screenshots.
