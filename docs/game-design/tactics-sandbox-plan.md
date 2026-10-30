# Tactics Sandbox: simultaneous grid movement in battle (proof of concept)

Status: **plan, ready to build** (2026-10-29). Written from a design review with the user. Every rule below was decided by the user unless it is marked *default*: a default is my best call where the user did not decide, so it must stay configurable, and it is listed again in §13 so the user can confirm it while playtesting.

The implementer works in their own worktree and branch, keeps a progress log in `docs/game-design/tactics-sandbox-progress.md`, and asks the user before changing any decided rule.

---

## 1. Why

Battles are currently Slay the Spire style: everyone is always in range, so there is no reason to play an archer or a melee fighter. The user wants to try a top-down, D&D-style **square grid** in battle where:

- range and line of sight matter (melee versus ranged versus area attacks);
- movement is **paid with the deck** (a card discarded = 1 Move), so the deck is health, stamina and positioning at once;
- movement is **simultaneous** and secret like everything else in battle, and a real simulation settles who goes where, including collisions and paths that cross;
- giving up your attack to sprint, charging an enemy, dodging a fireball, or running and losing your normal defenses are all real choices.

This is a **proof of concept in its own menu tab**. Shipping battles (menu battles, campaigns, expeditions, scenarios) must not change. If the user dislikes the result, the sandbox can be deleted without touching anything else.

## 2. Ground rules for the implementer

- Read `AGENTS.md` first and follow it: the `./scripts/godot.sh` launcher, native builds, worktree isolation, the visible-status rule, the hover contract, the draw/discard and damage rules.
- **The Go authority owns every rule.** Godot only renders the viewer-safe state and submits commands. It never computes paths, collisions, ranges, line of sight or hits.
- **Every number lives in configuration** (`content/tactics_v1/`), never in code. Loaders are strict.
- **Deterministic and seeded.** The same seed and commands give the same result. Random tiebreaks use the battle's RNG.
- **Secret until the reveal.** Movement plans never appear in another viewer's snapshot, events or legal actions before the movement reveal. Use the existing `secret_actor_id` pattern from simultaneous play.
- **No changes to shipping content or flows.** New ability fields are optional and ignored outside the tactics ruleset. The sandbox uses copies of characters and enemies (§7), so published ability files and the learned-policy training fingerprints stay untouched.
- **Keep shipping tests green:** `go test ./...`, `verify_battle_authority.gd`, `simultaneous_play_test.go`, `learned/simultaneous_play_test.go`, `verify_simultaneous_play.gd`, and the expedition tests named in `AGENTS.md`.

## 3. The round

A tactics battle pins `unified_defense`, `simultaneous_play` and a new `tactics` rule (`Settled.Tactics` plus a `TacticsRuntime` in `SettledRuntime`, with the arena and the rules config pinned into the battle at start so YAML edits never change a running battle). The tactics rule is only accepted together with the other two.

Each round:

1. **Ongoing Effects, Income:** unchanged.
2. **Offense (simultaneous, unchanged flow):** secret Before plays, lock in (Roll, or skip), one reveal, rolls, abilities and targets lock in and are revealed. New in tactics:
   - **Targets must be legal at this moment**, measured from the round's starting positions (§5.4): in range and in line of sight. Range and line of sight are measured **only here**; later movement never changes whether a direct attack lands.
   - An **area** ability chooses a target **square**, not a creature.
   - A **Charge** ability chooses a target within its charge reach (§5.6).
   - **Skipping offense means Sprint** (§4.3). In the sandbox the skip control reads "Sprint (skip attack)".
3. **Movement (new step, secret):** every participant sees the revealed attacks, including area squares, then secretly locks in one movement plan: an intention plus the exact cards and energy committed (§4). The only card plays here are movement commitments. No Before/After windows, and ordinary battle cards cannot be played.
4. **Movement resolves** in the authority (§5). The result (timeline, final squares, which attacks connect) becomes public.
5. **Defense (`defense_before`, `defense_after`: unchanged flow)** opens **only for attacks that connected** (§5.5), with their threatened cards shown. A creature that ran this round can only use on-the-move defenses (§6).
6. **One final commit**, then **one playback** that shows movement, strikes, defense results and deaths together (§9.4).

Nobody is stopped by being hit: hits never end or shorten movement. A creature killed by a hit keeps its movement for the round and falls at its final square when damage is committed (the playback shows it hit, running, then falling).

## 4. Movement plans

### 4.1 Intentions

Each participant picks exactly one:

| Intention | Meaning |
|---|---|
| **Move to** a square | Shortest path (by Move cost) to that square. |
| **Pursue** a creature | Each step, head for the nearest free square adjacent to the target's **current** position. Stop while adjacent; step again if the target moves away. |
| **Flee from** a creature | Each step, take the neighbouring square that most increases distance from that creature's current position. Ties: lower terrain cost, then a fixed direction order. If no step increases the distance, wait. |
| **Hold** | Stay put. Commits nothing. |

A Charge (§5.6) replaces the intention for that round.

### 4.2 Paying for Move

- **Base speed:** 1 Move per round, free (config `base_speed`).
- **Discard any card:** +1 Move (config `card_move`). The card goes to the discard pile and still counts as health.
- **Energy:** 5 energy = +1 Move (config `energy_per_move`).
- **Map Move cards** (cards with `map_effect.kind: move`: Trailblaze, Forced March, Know the Land, Slip Away…) can be **played** in the movement step for their `amount`, paying their energy cost; then they go to discard. In the sandbox only, this deliberately breaks the normal rule that map cards are never legal in battle. Config `map_card_move_multiplier` (*default* 1).
  - `ignore_terrain`: every square costs 1 for this mover this round.
  - `choose_terrain`: one terrain type, chosen when played, costs 1 for this mover this round.
  - Other map modifiers (`ignore_night`, `carry_over`, `explore_*`, `sneak`, `cross_cost`) do nothing in battle.
  - These modifiers apply only within the round's movement they were committed to. They are not ongoing effects, so they need no status.
- **Commit exact cards.** The player picks the specific cards and the energy amount. Everything committed is spent even if the mover ends up needing less.
- The plan's **Move budget** is base + cards + energy + map amounts (+ Sprint, §4.3).

### 4.3 Sprint (skipping offense)

A participant who skips offense this round sprints: base speed × 2 (config `sprint_base_multiplier`) and +1 extra Move per committed card, map cards included (config `sprint_card_bonus`). The user wants giving up your whole attack to be a big benefit; tune these in playtests.

### 4.4 Running

A plan whose Move budget exceeds base speed **runs** this round (Sprint always runs). Running applies a visible status **Running** at the movement reveal. It expires at the end of the round, and its rules text reads "Ran this round: only on-the-move defenses". See §6. A Charge does **not** count as running (*default*, config `charge_counts_as_running: false`).

## 5. Movement resolution (authority)

New pure Go package `internal/battle/tactics`: grid, distance, line of sight, pathing, the simulation and collisions. No engine imports; the engine calls it. It must be heavily unit-tested on its own.

### 5.1 Grid

- **Squares.** Distance is Chebyshev: a diagonal step counts as 1, as in the D&D 5e default. Config `diagonal_rule: chebyshev | alternating` (*default* chebyshev; alternating is the 5-10-5 variant).
- **Diagonals may not cut a blocked corner:** a diagonal step is illegal if either orthogonal neighbour it passes is a wall (*default*, config `corner_cutting: false`).
- **Terrain types** from config: `floor` (cost 1), `swamp` (cost 2), `wall` (impassable, blocks line of sight). Each square has one terrain type.

### 5.2 Time and speed

- Movement happens over one round, time **t ∈ [0, 1]**.
- A mover with Move budget **M** moves at rate M: entering a square of cost c takes c/M time. With cumulative cost C after k steps, it enters its k-th square at t = C/M.
- Times are **exact fractions in integer arithmetic** (compare by cross-multiplying, int64). There are no ticks and no rounding, so any speed works.
- A mover stops when its intention is satisfied, its budget is used up, or t would pass 1.
- **Occupancy:** a creature holds a square from the moment it enters until it leaves, the half-open window [enter, exit). The starting square is held from 0; the final square is held through the end of the round.

### 5.3 Simulation and collisions

Event-driven: a queue of "next step" attempts ordered by time. At each time t, take every mover with a step due at t:

1. Each mover picks its next square from its intention, re-pathing against current positions (shortest path by Move cost; walls blocked; squares held by **opposing** creatures at t blocked; allies passable).
2. **Blocked by an opponent** (the square is held by an opposing creature at t): re-path around if a route exists; otherwise wait. A wait retries at t + 1/M and uses time but no Move.
3. **Same square, same moment.** If two opposing creatures want the same square at the same t, the tiebreak chain decides:
   1. More squares moved so far this round
   2. Higher Move rate M
   3. More Move paid with cards and energy
   4. Seeded random

   The loser waits in its square and retries at its next step.
4. **No swapping through an opponent:** if A steps into B's square while B steps into A's square at the same t, and they are opponents, both wait. Crossing diagonals that only touch at a corner is allowed.
5. **Allies** pass through each other but cannot end the round on the same square. A mover whose final square is held by an ally stops at the last free square of its path.
6. Paths that cross collide **only when their occupancy windows overlap**. A fast creature that enters and leaves a square before a slow one arrives does not block it.

The result is a **timeline**: one record per step (creature, from, to, t as numerator/denominator), plus waits, tiebreaks and charge contacts. It is stored in the battle state, shown in the debug view (§9.5) and drives the playback.

### 5.4 Range and line of sight

- **Range** is Chebyshev distance between squares. Melee is range 1 (adjacent, diagonals included).
- **Line of sight** runs between square centres using a supercover line (every square the line touches). Blocked by walls, and by creatures other than the attacker and the target (config `los_blocked_by_creatures`, *default* true). A line that passes exactly through a corner is blocked only if both squares at that corner block (permissive corners).
- Both are checked **once, at the offense lock-in, from the round's starting positions**.

### 5.5 How attacks connect

| Delivery | Chosen at offense | Connects |
|---|---|---|
| **Direct** melee/ranged (homing, like Magic Missile) | A creature in range and in line of sight | Always, if the roll qualifies. Later movement doesn't matter. |
| **Area** (Fireball) | A square in range and in line of sight | Hits every opponent of the caster standing in the area at **t = 1** (config `area_hits_allies`, *default* false). Moving out of it dodges. |
| **Charge** (§5.6) | A creature within charge reach and in line of sight | If the charger becomes adjacent to the target at any time during movement. |

An area attack makes **one damage source per creature hit**, each with its own reservations, badge and defense, following the unified-defense rule that attacks are never pooled. The engine needs this as new work: today offensive targeting is exactly one enemy.

Attacks that do not connect are shown as a miss ("dodged" / "out of reach") and open no defense.

### 5.6 Charge

A Charge ability has a built-in charge range R (`delivery.charge.range`). At offense the target must be within R + 1 squares and in line of sight. In the movement step the charger's plan is fixed to "Pursue the target with Move budget R". It uses up its movement for the round and can't add cards or energy (*default*, config `charge_allows_extra_move: false`). It connects at the first moment it becomes adjacent. If the target outruns it or it is blocked, it misses.

## 6. Defenses while running

- A defensive ability may be tagged `on_the_move: true` (*default*: such defenses are usable whether or not you ran).
- A creature with **Running** (§4.4) may choose only on-the-move defenses this round. Normal defenses show as disabled, with the reason, in legal actions and the UI.
- The sandbox adds **Evade** for the player (§7.2) and tags Ribbon Eel's sandbox Slip the Current as on-the-move (§7.3), so both sides can test it.
- Cards that prevent damage stay playable as normal. The restriction covers defensive *abilities* only.

## 7. Content: `content/tactics_v1/`

### 7.1 Ability schema: `delivery`

Add an optional `delivery` block to the ability definition. Range is part of the ability itself, as the user requires:

```yaml
delivery:
  type: direct            # direct | area
  range: 1                # Chebyshev squares; melee = 1
  line_of_sight: true
  area: {shape: square, radius: 1}   # area only; square radius 1 = 3×3
  charge: {range: 4}                 # optional: makes a direct ability a Charge
```

Defensive abilities may set `on_the_move: true`. Validation is strict: area needs `area`, charge needs `type: direct`, ranges ≥ 1. In a tactics battle, an offensive ability without `delivery` defaults to direct melee range 1. Outside tactics battles, `delivery` and `on_the_move` are ignored. Show delivery in the ability's rules text and in the shared attack tooltip ("Range 6, line of sight", "Area 3×3, range 5", "Charge 4").

### 7.2 Player characters (copies; the originals stay untouched)

**Starter Movement** (`starter_movement`) and **Adventurer Movement** (`adventurer_movement`) copy `adventurer_v1/combatants/starter.yaml` and `adventurer.yaml`, with these board changes. Requirements and damage are reused, so the power level stays the same and only the delivery changes:

| Slot | Ability | Requirements / effect | Delivery |
|---|---|---|---|
| Offense | Strike, Guarded Strike, Measured Strike, Decisive Blow | unchanged | direct melee, range 1 |
| Offense | **Magic Missile** (replaces Small Straight) | Small Straight's requirement, 6 damage | direct ranged, range 6, line of sight |
| Offense | **Fireball** (replaces Large Straight) | Large Straight's requirement, 8 damage | area 3×3, range 5, line of sight |
| Offense | **Charge** (new) | 3/4/5 Swords → 3/4/5 damage, choose tier | direct melee, charge range 4 |
| Defense | Guard (Starter); Guard and Guard+ (Adventurer) | unchanged | normal (not on the move) |
| Defense | **Evade** (new) | Roll 2 dice: each 4–5 prevents 1, each 6 prevents 2; saved cards go to discard | `on_the_move: true` |

The decks are copied unchanged. The setup screen (§9.1) can add map Move cards for testing. These characters are sandbox-only: not campaign- or expedition-eligible, and not priced in board XP. Write full `presentation.rules_text` for every new ability (hover contract).

### 7.3 Enemies (sandbox copies with delivery and speed)

Copies of the Drowned Oracle set and the Regent, e.g. `tactics_brine_mask`, with the same names, decks, dice and defenses. Each has a `tactics` block (base speed, role). Ranged and area enemies get a **weaker backup melee** ability on the same dice faces, because the user wants them to prefer range but not be helpless up close:

| Enemy | Role | Base speed | Offense | Defense |
|---|---|---|---|---|
| Ribbon Eel | melee pursuer | 2 | Coiling Snare, melee | Slip the Current, **on the move** |
| Bell Diver | melee bruiser | 1 | Sunken Toll, melee | Brass Helm |
| Brine Mask | ranged | 1 | Brine Lash → direct ranged, range 4, line of sight (2 per Brine); backup **Brine Slap** melee (1 per Brine) | Salt Veil |
| Drowned Regent | area caster | 1 | Tide Crown → area 3×3, range 5; backup **Crown Strike** melee (Tide Crown's tiers −2 damage, minimum 1) | Coral Mantle |

The speeds and backup numbers are *defaults* for playtesting and live in config.

### 7.4 Rules config: `content/tactics_v1/rules.yaml`

Holds `base_speed`, `card_move`, `energy_per_move`, `map_card_move_multiplier`, `sprint_base_multiplier`, `sprint_card_bonus`, `diagonal_rule`, `corner_cutting`, the terrain table (cost, passable, blocks line of sight), `los_blocked_by_creatures`, `area_hits_allies`, `charge_counts_as_running`, `charge_allows_extra_move`, the tiebreak order, and enemy AI tuning (§8). The setup screen can override any of these for one battle; the overrides are pinned into that battle.

### 7.5 Arenas: `content/tactics_v1/arenas/*.yaml`

Hand-made, **12 × 9 squares** (*default*). Each arena has a terrain grid, a player start square, and up to four enemy start squares (fixed, used in order).

1. **Open Field:** all floor.
2. **Doorway:** a wall across the middle with a one-square door (blocking and choke points).
3. **Pillar Hall:** scattered pillars (line of sight and charging around obstacles).
4. **Swamp Crossing:** a band of swamp across the middle, so you can test map cards that ignore or choose terrain.

## 8. Enemy behaviour

Scripted. The learned, search and neural-network players are **not** used in tactics battles. `mlsim` and the policy paths must refuse a tactics battle rather than mis-handle it.

- **Offense:** keep the existing single-ability roll policy. Among the abilities whose roll qualifies, a ranged or area enemy prefers its ranged/area ability when a legal target exists, then its backup melee if adjacent; otherwise no attack. Only legal targets (range and line of sight).
- **Area aim:** aim at the target's **current** square (the user's choice: it forces the player to decide whether to spend to dodge).
- **Movement:**
  - Melee enemies: Pursue the player.
  - Ranged and area enemies: if adjacent to the player, Flee from them; if out of range or line of sight, Move to the nearest square with range and line of sight; otherwise Hold.
- **Spending:** an enemy may commit up to `ai.max_cards_per_round` cards (*default* 1) when that gets it into range next round. It never commits its last card.
- **Defense:** the existing policy, limited to legal (on-the-move) defenses when Running.

## 9. Client (Godot): `app/screens/tactics/`

### 9.1 Menu and setup

- A new main-menu button **"Tactics Sandbox (prototype)"** in `app/boot/battle_bootstrap.gd`, always visible.
- The setup screen picks:
  - the hero (Starter Movement or Adventurer Movement), plus optional map Move cards to add;
  - 1–4 enemies from §7.3, in any mix;
  - the arena;
  - optional rule overrides;
  - the seed (random by default).
- Start creates the battle through a new authority op, following the pattern of the existing ops in `learned/runtime.go`.

### 9.2 Board

- Top-down **2D**: a square grid filling the battlefield area, tokens for creatures, and terrain drawn simply. No 2.5D.
- Reuse the existing battle HUD pieces where possible: hand, dice tray, ability board, piles, status bars, the shared attack tooltip presenter.
- Attack badges and threatened-card lists anchor to the target's token. Area attacks highlight their squares. Hovering a creature shows its range, statuses and Move budget. The hover contract and keep-popups-in-the-viewport rules from `AGENTS.md` apply.

### 9.3 Movement step UI

- Pick an intention:
  - **Move to:** click a square.
  - **Pursue / Flee:** click a creature, then choose which.
  - **Hold:** a button.
- Commit cards by clicking them in hand. Each shows +1, or its map amount, or +2 while sprinting. Add energy in 5-energy steps.
- A **path preview** shows the projected path and arrival, assuming everyone else holds. It's labelled as a preview; the authority computes it.
- A Move budget meter shows base + committed. A **Running** warning appears as soon as the budget exceeds base speed.
- **Lock in.** Auto-complete when only Pass remains, as elsewhere.

### 9.4 Defense step and playback

- **During defense,** tokens stay at their starting squares. Each creature shows its revealed path and a ghost at its final square, so the player can see they left the fireball. Connected attacks show their badges. Missed attacks show "dodged" or "out of reach".
- **After the final commit, one playback** shows everything together, along the resolved timeline:
  - Movement plays in time order.
  - Direct hits strike at t = 0 (the target is slashed, then keeps running).
  - Charge hits land at contact.
  - Area hits land at t = 1.
  - Defense results show with each hit, and lethal hits make the creature fall at its final square.
- Playback has a speed control and a **Replay round** button. The user's wish is that the reveal shows attacks, movement and defense results together; this split, with a plan view during defense and a full playback after the commit, is my reading of it (§13).

### 9.5 Debug timeline

A toggleable panel listing the resolved timeline: each step with its time as a fraction and a percentage, waits, tiebreak decisions (which rule decided), charge contacts, and area hits. It's for checking the collision rules during playtests.

### 9.6 Save and resume

Tactics battles persist through the authority like any battle and resume from the sandbox tab. Use a **separate active-battle pointer** (e.g. `active_tactics_battle.json` beside `active_battle.json`) so the sandbox never hijacks the normal battle resume.

## 10. Milestones

- **M0 · Read and scaffold:** read the docs in §12. Add the content pack, the `delivery` schema, the strict loaders for rules and arenas, and the Starter Movement and Adventurer Movement characters and enemy copies. Shipping tests stay green.
- **M1 · `internal/battle/tactics` (pure):** grid, distance rules, line of sight, pathing with terrain, intentions, the exact-time simulation, collisions, the tiebreak chain, and the timeline. This is the heart of the work: test it exhaustively (§11) before integrating.
- **M2 · Authority:** pin the tactics ruleset, positions in state, offense legality (range, line of sight, area squares, charge reach), Sprint, the movement step (secret lock-in, legal actions, commands, viewer safety), resolution, connections, multi-target area damage sources, Running and the on-the-move defense filter, the snapshot/view, save/resume, and the start/catalog ops.
- **M3 · Enemy behaviour (§8).**
- **M4 · Client:** menu tab, setup, grid board, movement UI, path preview, defense-step plan view, playback, debug timeline, resume.
- **M5 · Verify and document:** the tests in §11, then **physical playtesting** (run the game, play several battles in each arena with both heroes and enemy mixes, and take screenshots). Add a short `AGENTS.md` section for the sandbox and keep the progress log current.

## 11. Acceptance criteria and tests

Go (`internal/battle/tactics` and engine), at minimum:

1. Chebyshev and alternating distances; corner cutting refused; swamp costs 2; walls impassable.
2. Line of sight: walls block; creatures block when configured; permissive corners; attacker and target never block themselves.
3. Exact times: speeds 1–10 with mixed terrain produce the expected entry times, with no rounding anywhere.
4. Crossing paths **without** a collision when the windows don't overlap; **with** one when they do.
5. Same square at the same moment: each tiebreak level decides one case, down to the seeded random (deterministic for a seed).
6. A head-on swap between opponents is refused (both wait); a swap between allies is allowed; allies never end on the same square.
7. Pursue catches a slower fleeing target, and a faster one escapes. Flee in a corner waits. Pursuit re-paths around a blocker.
8. A direct attack lands even though the target ran out of range. An area attack misses a target that left the area and hits one that stayed. A charge connects mid-movement and misses when outrun.
9. Area damage creates one damage source per creature hit, each defended separately.
10. Sprint multipliers; map Move cards with `ignore_terrain` and `choose_terrain`; energy → Move; committed cards are spent even if unused.
11. Running is applied as a visible status, normal defenses are refused while Running, Evade and Slip the Current are allowed, and the status expires at the end of the round.
12. Viewer safety: no movement plan in another viewer's snapshot, events or legal actions before the reveal.
13. Save and reload during the movement step and during defense keep plans, timeline and positions identical.
14. Same seed and commands give an identical timeline and outcome.
15. The tactics ruleset is refused without unified defense and simultaneous play; `mlsim` and the policies refuse tactics battles.
16. All shipping tests listed in §2 stay green.

Godot: `tests/presentation/verify_tactics_sandbox.gd`, pointer-driven:
- set up a battle;
- choose targets in range;
- Sprint;
- pick each intention by clicking;
- commit a card and energy;
- lock in;
- defend (including Running with Evade only);
- run the playback;
- resume a saved battle;
- win and lose.

Keep popups inside the viewport for 1–4 enemies and at both screen edges. Run windowed screenshot checks with `inspect_game.py pointer`, never the real mouse (see the windowed-test note in project memory and the playbooks).

## 12. Read first

- `AGENTS.md` (all of it)
- `docs/game-design/simultaneous-play-plan.md` and `docs/game-design/unified-defense.md` (the flow you're extending)
- `docs/game-design/ability-configuration.md` and `docs/game-design/turn-structure.md` (ability and combatant schemas)
- `docs/game-design/battle-hud-anchors.md` (UI contract)
- `docs/game-design/card-configuration.md` (card timing)
- `docs/game-design/expedition.md` §map cards, and `internal/battle/expedition/map_cards.go` (map Move effects)
- Code:
  - `internal/battle/engine/simultaneous.go`, `unified_defense.go`, `legal_actions.go`, `offensive_flow.go`
  - `internal/battle/state/settled.go`
  - `internal/battle/learned/runtime.go`, `session.go` (ops, session start)
  - `app/boot/battle_bootstrap.gd` (menu)
  - `app/screens/battle/` (HUD to reuse)

## 13. Defaults to confirm with the user during playtesting

These weren't decided by the user. Build them as configured defaults and list them in the progress log:

1. Charge does not count as running, and can't be extended with cards.
2. On-the-move defenses are usable whether or not you ran.
3. Area attacks don't hit the caster's allies.
4. Creatures block line of sight; corners are permissive.
5. During defense, tokens stay put with path and ghost previews; full playback after the commit (§9.4).
6. Arena size 12 × 9; enemy speeds and backup-melee numbers (§7.3); Evade's numbers (§7.2).
7. Enemy AI commits at most 1 card per round.
8. Map Move cards give their full map amount (multiplier 1).

## 14. Out of scope

- Hex grids and 2.5D or 3D presentation.
- Campaign, expedition, site-map or mansion integration, including rests and Strain.
- Objectives such as chests.
- Opportunity attacks and disengage costs (running's defense penalty replaces them).
- Curving or wall-piercing missiles, and line-of-sight variants beyond §5.4.
- Knockback or other forced movement.
- The neural-network, search and learned players in tactics battles.
- Multiplayer, and pricing the sandbox characters in board XP.

If a rule is missing or two rules conflict, **stop and ask the user**. Don't invent a rule.
