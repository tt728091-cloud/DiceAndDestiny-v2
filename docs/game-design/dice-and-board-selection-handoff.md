# Handoff: player die and board selection (first pass)

**Status:** ready to implement (written 2026-10-21).

**Plan of record:** [`dice-and-board-selection-plan.md`](dice-and-board-selection-plan.md),
phases B1–B7. The research and the pricing reference:

- [`dice-throne-reference/ability-layout-analysis.md`](dice-throne-reference/ability-layout-analysis.md)
- `dice-throne-reference/analysis/board_balance.py`

**Scope:** B1–B7. **B8 (the choice at campaign and expedition start) is a
separate handoff** after this one is merged and played.

**Running in parallel:** the AI work ([`ai-value-network-plan.md`](ai-value-network-plan.md))
runs in another thread, and so does gear (`phase-2-gear`). The ownership
rules below keep them apart.

**Decisions the user made (2026-10-21):**

1. **Open menus in the first pass.**
   - The player picks each flex slot's cost and rider freely.
   - Later the menus will be limited like gear: few choices at first, more
     as the character gains power.
   - Every menu is data with an unlock hook. Everything is unlocked for now.
2. **Where.** A Board Builder tab in Character Creation first. Campaign and
   expedition start come next (B8, not here). Rebuilding at camp comes later.
3. **Who.** A new custom adventurer owns the chosen die. Venom, Curse, Blade
   Warden, the Adventurer and the Starter are unchanged.
4. **Defenses.** They will become as wide a design space as offensive
   abilities. This pass only makes them work on any die:
   - they roll the owner's die;
   - their outcomes are written by symbol role;
   - they are priced to Guard's expected prevention.
5. **Storage.** Generated abilities belong to the player and live in their
   save, never in the tracked `content/authored/` directory.

---

## Prompt for the implementing model

> You are working in the Dice and Destiny repository. Read `AGENTS.md` first and follow it exactly, especially: run Godot only through `./scripts/godot.sh`, keep the Go server the authority, keep every number in configuration, make every ongoing battle effect a visible status, follow the ability hover contract (full `presentation.rules_text`), and never write to the tracked `content/authored/` directory from a test.
>
> Then read `docs/game-design/dice-and-board-selection-handoff.md` and implement tasks T1 to T7 with every test it lists. Also read the plan it points to, and `docs/game-design/dice-throne-reference/analysis/board_balance.py`, which is the reference implementation of the pricing. Work on a new branch named `dice-board-selection` created from local `main`. Commit each task separately with a clear message. Do not push or merge unless the user asks.
>
> Before changing code, confirm each "where it lives" pointer against the current code; line numbers may have moved. The handoff makes choices marked **Decision**. Follow them. If one contradicts the code or `AGENTS.md`, stop and report it rather than guessing. Every new name (symbols, dice, abilities, templates shown to players) must be original: never echo Dice Throne hero, ability or symbol names.
>
> Don't touch the AI work: `internal/battle/lookahead`, `internal/battle/mlsim`, `internal/battle/learned/lookahead_policy.go`, `cmd/balance-lab`, `cmd/card-balance`, `cmd/battle-review`, `ml/`, `docs/game-design/ai-value-network-plan.md` and `docs/game-design/balance-lab.md`. You may call `lookahead`'s exported API from a new command. Don't touch the gear work (Phase 2) either.
>
> When you finish, run the verification list, do the live check, and reply with the final report described at the end of the handoff.

---

## Background

### What exists (confirm before relying on it)

| Thing | Where it lives | Notes |
|---|---|---|
| Die definitions | `internal/content/battle_library.go` (`BattleDieDefinition`, about line 40); pack `dice/` folders, loaded by `LoadBattleExtension` (`battle_extension.go`) | Each face is `{number, symbol}`, one symbol per face; the number is also the value. `adventurer_v1/dice/` is empty. |
| Playable dice | `standard_d6` (battle_v1), `venom_d6`, `curse_d6` | All 3-2-1. Symbols come from each pack's `symbols.yaml`. |
| Ability schema | `BattleAbilityDefinition` (`battle_library.go`, about line 177); `validateTier` (about line 848) | Requirements: `symbol_count` (min/max/exact), `number_pattern` (three of a kind, exact pair, pair or better, small/large straight) and `exact_faces`. These cover every template cost. |
| Straights | `settled_engine.go` `requirementsMet` (about line 2898) | Fixed to faces 1–6. Every die numbers its faces 1–6, so straights work on any die. |
| Combatant die and board | `CombatantDefinition.dice_loadout` and `ability_board` (`battle_library.go`, about 431–447) | Everyone has 5 of one die. |
| Player sheet board | `loadout.Progress.Abilities` (`internal/battle/loadout/progression.go`, about line 65); applied in `learned/session.go` (about 256) and `campaign.go` (about 115) | The sheet has **no dice field** yet. |
| Playable character list | `internal/content/ability_authoring.go` (about line 91) | A hard-coded switch. The custom adventurer must be added here and wherever else the list repeats. |
| Board fits dice | `engine.AbilityFitsDice` (`engine/ability_configuration.go`, about line 158) | Searches every face combination of the loadout. |
| Qualification | `engine.QualifiedAbilityTier` (`engine/lookahead_exports.go`) | Returns the first matching tier in authored order. |
| Defense dice | `adventurer_guard.yaml` and `adventurer_guard_plus.yaml` | `roll_dice` names `dice_id: standard_d6` and lists `outcomes` by face number (1–3 prevent 1, 4–5 prevent 2, 6 more). |
| Bonus dice | `rollBonusDice` (`settled_engine.go`, about 2641) | Uses `loadout[0]`. That is fine for one-die loadouts. |
| Max rolls | `participant_assembler.go` (about 264) | Hard-coded 3. Pricing assumes 3. |
| Character Creation | `dice-and-destiny-client/app/screens/character/character_creation.gd` | Tabs include Abilities, Dice (read-only) and Ability Creation. The ability workshop (`ability_authoring.gd`) has a per-character board editor; the server side is `learned/ability_authoring.go`. |

### The pricing method (the user agreed this; don't change it)

`board_balance.py` is the reference implementation. In short:

1. **Chase odds** of a tier: the chance to qualify with 5 dice and 3 rolls
   when every keep chases that tier alone.
2. **Relative payoff** = (1 ÷ chase odds) ^ alpha, with **alpha = 0.2**. A
   guaranteed hit is 1.
3. **Play the board exactly:**
   - 252 sorted hands of 5d6, weighted by how many orderings make each;
   - every distinct keep;
   - backward induction over rolls left;
   - at the end, take the most valuable tier the dice qualify for;
   - ties share the use.

   This gives each tier's **use share** (the share of turns ending on it).
4. **Multiplier** = target ÷ Σ(use × relative). The target is **6.2** per
   turn, the Starter's level. Each tier's priced value is relative ×
   multiplier.
5. **Riders.** A tier's value counts damage plus its rider at the exchange
   rates (Protect 1 per stack, energy 0.2 per point). Damage = priced value −
   rider value, never below 1. Riders change play, so the board is always
   played with them included.
6. **Round to whole damage.** Harder tiers never pay less than easier ones.
   Pick the rounding within 2% of the target whose use shares drift least
   from the exact board, then play it again and report it.

The board is re-priced from scratch whenever the die, the template, a cost or
a rider changes.

**Parity numbers** (the Go port must reproduce these to ±0.01 on use shares
and the multiplier, and exactly on whole damage):

- `python3 -I board_balance.py --board starter --target 6.2 --no-marginal` →
  Σ = 1.130, multiplier 5.49, exact average 6.20 with 0.6% no hit, whole
  average 6.13 with 1.2% no hit.

  | Tier | Chase | Relative | Use (exact) | Value | Whole |
  |---|---|---|---|---|---|
  | 3 Sword | 98.4% | 1.003 | 4.6% | 5.5 | 5 |
  | 4 Sword | 87.9% | 1.026 | 14.1% | 5.6 | 5 |
  | 5 Sword | 51.3% | 1.143 | 21.5% | 6.3 | 6 |
  | 2 Sword + 3 Shield | 55.7% | 1.124 | 12.5% | 6.2 | 6 |
  | 2 Sword + 2 Coin | 57.6% | 1.117 | 13.3% | 6.1 | 6 |
  | SS | 61.5% | 1.102 | 17.3% | 6.0 | 6 |
  | LS | 26.1% | 1.308 | 14.8% | 7.2 | 8 |
  | 4 Coin | 10.4% | 1.571 | 1.4% | 8.6 | 8 |

- `--board lean --target 6.2 --no-marginal` gives the same board plus a
  5 Coin ultimate (chase 1.3%, relative 2.374, value 13.0, whole 12):
  Σ = 1.133, multiplier 5.47.
- **Monk die 2-1-2-1:**

  ```
  --die 2-1-2-1 --costs "A3/A4/A5, C3, A3+B1, B3, SS, D4, LS, D5"
  --printed "6/7/8, 7.66, 8.28, 10.5, 8, 10, 12.66, 21.32"
  --target 8.58 --no-marginal
  ```

  This gives multiplier 7.52 (Σ 1.140), and exact values:

  | 3/4/5 A | 3 C | 3 A + 1 B | 3 B | SS | 4 D | LS | 5 D |
  |---|---|---|---|---|---|---|---|
  | 7.8 / 8.5 / 10.7 | 7.8 | 8.3 | 9.3 | 8.3 | 11.8 | 9.8 | 17.9 |

  Its printed board plays to 8.58 per turn with 5.3% no hit.

If a number doesn't match, the Python script is right unless you can show it
has a bug. Report it either way.

---

## Ownership and the AI interface

The AI thread will consume three things from this work. Keep their shape
stable and documented in the package docs:

1. `boardprice.Price(die, board, options) (Report, error)`. Options are the
   target, alpha and rates. The report gives per tier: chase, relative, use
   (exact and whole), value, rider value, damage and whole damage. It also
   gives Σ, the multiplier, the average and the no-hit rate.
2. `boardprice.RandomBoard(rng, family)` returns a die, a template and slot
   picks. `boardprice.Generate(...)` turns a priced board into
   `BattleAbilityDefinition`s. The AI will train on random boards built this
   way.
3. **Exchange rates live in one file, `content/progression_v1/riders.yaml`.**
   - Pricing reads them from there.
   - The AI thread will point the greedy player's values at the same file
     after merge, and will later replace the rates with measured ones.
   - Never copy the rates into code.

## Tasks

### T1 — Dice catalog (B1)

- Create the custom adventurer's pack content in `adventurer_v1`, or a new
  pack if loading order requires it. **Decision:** one die per family order:
  - 3-2-1 reuses `standard_d6`;
  - new dice for 2-3-1, 3-1-1-1, 2-2-1-1 and 2-1-2-1.
- New symbols get original names and placeholder icons in the existing icon
  style. List each placeholder in the report.
- Faces are numbered 1–6 in face order. The rarest symbol is on face 6.
- **Tests:**
  - every die loads;
  - a role function (main / side / apex / odd from face counts) gives the
    expected roles for each die.

### T2 — Custom adventurer and the die on the sheet (B2)

- **A new playable character, `custom_adventurer`.** It has the Starter's
  deck rules, a default 3-2-1 die and a default priced Lean board. Add it to
  every playable-character list.
- **A dice field on the player sheet** that overrides the combatant's
  `dice_loadout`. It is always 5 of one die and is validated against the
  catalog. The battle pins it, so saves and replays keep the starting die.
- **Role-based defense outcomes (Decision 4):**
  - `roll_dice` may name `dice: owner` instead of a `dice_id`;
  - outcomes may name `roles` instead of `faces`;
  - the engine resolves roles from the die's face counts;
  - existing face-based defenses are unchanged.
- **Tests:**
  - a full authority battle completes with the custom adventurer on each new
    die;
  - save and reload keeps the die;
  - board validation (`AbilityFitsDice`) runs against the chosen die;
  - a role-based defense resolves correctly on a 2-2-1-1 die.

### T3 — Templates, riders and defenses as content (B3)

- **`content/progression_v1/board_templates.yaml`:**
  - the templates from the plan for each family;
  - each slot's job, role-based cost (for example `main3/main4/main5`,
    `main2+side3`, `SS`, `apex4`) and difficulty band;
  - the cost menu each flex slot may take.
- **`content/progression_v1/riders.yaml`:**
  - each rider's operations (Protect, energy, draw, heal, and the existing
    self-statuses) and its exchange rate;
  - which slot jobs may take it.
- **A defense menu:** role-based versions of Guard and Guard+, priced so
  their expected prevention on any die equals Guard's on the 3-2-1 die.
- **An `unlock` field on every menu entry.** The first pass ships everything
  unlocked, and the loader accepts only that value for now.
- Strict loaders that reject unknown fields and costs a die can't reach.
  Riders may only apply statuses that already exist in the catalog.

### T4 — Pricing in Go and the `board_price` op (B4)

- **Package `internal/battle/boardprice`:**
  - the exact solver without cards;
  - qualification through the engine's rules;
  - the pricing method above;
  - `RandomBoard` and the parity tests.

  It must not import `lookahead`.
- **The `board_price` op.** Given a die, template, slot picks and riders, it
  returns the report plus the keep policy after rolls 1 and 2.
  - It only reads; the client never computes prices.
- **Speed.** One board must price in well under a second, so the Board
  Builder can preview live. Cache by board key the way `lookahead/dice.go`
  does, but in your own package.

### T5 — Generated abilities (B5)

- `boardprice.Generate` builds real ability definitions with the die's
  symbol IDs.
  - Ladders become tiers of one ability.
  - Riders become operations.
  - `presentation.rules_text` lists the dice requirement, damage, rider and
    when it misses.
- **Storage (Decision 5):** in the player's loadout save next to `Abilities`.
  - A battle pins the generated definitions in its catalog.
  - Generated IDs are stable per save, so re-pricing an unchanged board
    yields the same IDs.
- **Tests:**
  - every generated ability validates and qualifies on its die;
  - the hover shows the full rules;
  - the board survives save and reload;
  - re-pricing after a rider change updates the damage.

### T6 — Board Builder tab (B6)

A Character Creation tab for the custom adventurer.

1. **Die picker:**
   - face strip with symbol roles;
   - each menu cost's chase odds.
2. **Template picker:**
   - slots with difficulty bands;
   - each template's average and no-hit rate.
3. **Slot editor.** Pick the cost and rider for each flex slot, and a rider
   for core slots where the template allows one. Every change calls
   `board_price` and shows the live preview:
   - damage per ability;
   - use bars;
   - average and no-hit rate;
   - the keep table on demand.
4. **Confirm** writes the board and die to the sheet through the authority.

Add a headless Godot test, and a pointer check through the inspector's
`pointer` route.

### T7 — Board lab (B7)

- **A new `cmd/board-lab`** that runs battles with `lookahead`'s exported
  runner and policies. It must not modify that package.
  - Hero: the custom adventurer with a given die and board.
  - Opponents: the Starter, Brine Mask and the other minions.
- **The matrix:**
  - every die × template at default picks;
  - 300 random boards from `RandomBoard`;
  - greedy players, paired seeds.
- **Write the results into the plan doc**, not `balance-lab.md`:
  - each combination's score against the Starter mirror band (0.45–0.55);
  - any rider that is always best or always worst.

  Don't tune individual abilities by hand. Report the findings instead.

## Verification

```bash
cd dice-and-destiny-server && go test ./...
cd .. && ./scripts/godot.sh --headless --script res://scripts/verify_battle_authority.gd
./scripts/godot.sh --headless --script res://tests/presentation/verify_character_creation.gd
./scripts/godot.sh --headless --script res://tests/presentation/verify_expedition_map.gd
```

Also run the new Board Builder test.

Notes:
- If a test already fails on `main`, prove it on a clean `main` checkout and
  say so.
- The `learned` package takes about 8 minutes.

**Live check** (`DICE_AND_DESTINY_INSPECTOR=1 ./scripts/godot.sh`):

1. Open Character Creation, then the Board Builder.
2. Pick the 2-2-1-1 die and a template; change a rider and watch the
   preview re-price.
3. Confirm.
4. Start a sandbox battle as the custom adventurer.
5. Hover an attack: the full rules show. Roll a defense: it resolves by
   role.

Save screenshots in `dice-and-destiny-client/.godot/runtime/board-evidence/`.

## Out of scope

- B8: the choice at campaign or expedition start.
- Rebuilding during a run, and unlock rules beyond the hook.
- Upgrades and levels.
- Wider defenses.
- Mixed dice pools.
- Changing the Adventurer, Starter, Venom, Curse or Blade Warden boards.
- The AI work, and any change to `lookahead`.
- The gear work.
- Pushing or merging.

## Final report

Reply with:

1. a summary and the commit for each task (T1–T7);
2. the test commands you ran, with pass/fail results;
3. the parity results against the Python numbers, and any differences;
4. the new dice and symbols, with their names and which icons are
   placeholders;
5. the final template, rider and defense menus;
6. the board-lab results table and anything outside the band;
7. anything that contradicted this handoff or `AGENTS.md`, and how you
   handled it;
8. design choices this handoff didn't cover;
9. the paths to the live-check screenshots.
