# Tactics Sandbox: progress log

Implementation log for `tactics-sandbox-plan.md` (the plan is the source of truth). Branch: `t3/tactics-sandbox-poc`.

## Status

| Milestone | State |
|---|---|
| M0 · Read and scaffold | done |
| M1 · `internal/battle/tactics` (pure) | done |
| M2 · Authority | done |
| M3 · Enemy behaviour | done |
| M4 · Client | — |
| M5 · Verify and document | — |

## M0 · Content pack and schema

- `content/tactics_v1/` (only tactics battles load it):
  - `rules.yaml`: every number of the plan (§7.4), the terrain table, the tiebreak order, the default delivery and the AI tuning. Strict loader: `tactics.LoadPack` / `LoadRules` (unknown keys and out-of-range values are refused). `tactics.ApplyOverrides` merges the setup screen's per-battle overrides by key and re-validates.
  - `arenas/`: Open Field, Doorway, Pillar Hall, Swamp Crossing, all 12 × 9, each with a player start and four enemy starts (used in order).
  - `pack.yaml`: the ability copies, the heroes and the enemies the setup screen offers.
  - `abilities/`: Magic Missile, Fireball, Charge, Evade (new); Brine Slap and Crown Strike (backup melee, new); copies of Brine Lash (range 4), Tide Crown (area 3×3, range 5) and Slip the Current (on the move).
  - `statuses/running.yaml`: Running (`on_the_move_defenses_only`), expires at the next round's Ongoing Effects like Protect.
  - `combatants/`: Starter Movement, Adventurer Movement, and the sandbox Ribbon Eel, Bell Diver, Brine Mask and Drowned Regent (same names, decks, dice and defenses as the shipped ones; a `tactics` block with base speed and role).
- Schema (`internal/content`): optional `delivery` and `on_the_move` on abilities, `tactics` on combatants and the `on_the_move_defenses_only` status modifier. All are omitted from JSON when unset, so every shipped catalog serializes byte-for-byte as before (`TestShippedCatalogJSONIgnoresTacticsFields`). Validation is strict: area needs `area`, a charge needs `type: direct`, ranges ≥ 1, `on_the_move` only on defenses, `delivery` only on attacks.
- **Copies keep their shipped IDs.** `ApplyTacticsPack` replaces `brine_lash`, `tide_crown` and `slip_the_current` with the pack's copies in a tactics battle's catalog only; the shipped files and every other catalog are untouched. A tactics ability whose ID already exists must be listed in `pack.yaml` `ability_copies` (and vice versa), so an accidental clash is refused. Same IDs were needed because ability names must be unique in a catalog and the plan keeps the names.
- A scripted (single-ability) combatant with a `tactics` block may have two attacks (its backup melee); every other one still needs exactly one.

## M1 · The pure package `internal/battle/tactics`

No engine imports. Files: `config.go` (rules, arenas, loaders), `frac.go` (exact times), `grid.go` (terrain, costs, distance, corner rule), `los.go` (supercover line of sight), `path.go` (re-pathing cost field), `sim.go` (the simulation), `budget.go` (Move budgets and Running).

How the simulation works (`tactics.Simulate`):

- Time is an exact fraction (`Frac`, int64 numerator/denominator, compared by cross-multiplying). A mover with Move budget M decides its next square **at a moment t, against the positions at t**, then enters it at t + c/M (c = the square's Move cost). With no waits it enters its k-th square at exactly C/M.
- Every entry due at the same moment is resolved together: an opponent still holding the square blocks it unless it leaves at that same moment (half-open windows); opponents entering the same square at the same moment are separated by the tiebreak chain (`tiebreak_order`: squares moved, Move rate, Move paid, then seeded random); opponents swapping head-on both wait. A mover that fails waits in its square and decides again at once, so its retry comes one step later. A wait with no route retries after 1/M (time, no Move).
- Pathing: shortest path by Move cost over (square, diagonal parity) — the parity only matters for the alternating rule — recomputed at every decision; squares held by opponents at that moment are blocked, allies are passable. Equal-cost routes prefer the step nearest the aim (straight line), then the fixed direction order N, NE, E, SE, S, SW, W, NW.
- Intentions: Move to (stop on arrival), Pursue (head for the nearest free square next to the target's current square; wait while adjacent), Flee (best distance gain among affordable steps; ties: cheaper terrain, then direction order; no gain → wait), Hold.
- Allies may share a square in passing; if two end on one square, the one that entered it first keeps it and the other is limited to the last square of its own path nobody holds at the end, and the round is resolved again (repeated until stable; limits only ever shrink, so it terminates).
- The seeded random tiebreak is a hash of the round's seed, the moment, the square and the creature: resolving the round again gives the same answer, and the engine spends one battle random draw per round on the seed.
- The result is a timeline (steps with exact times, waits with reasons, tiebreaks with the deciding rule, charge contacts, stops), final squares, paths and Move spent.

Tests (`go test ./internal/battle/tactics/`): §11 items 1–7, 10 (rules side) and 14 at the pure level, plus `TestRandomRoundsKeepEveryInvariant`, which resolves 3,000 random arenas and plans (both diagonal rules, corner cutting on and off, terrain modifiers) and checks that opponents never hold a square at the same time, allies never end together, every step is a legal neighbour paid at exactly its cost and rate, nothing passes t = 1 or the budget, and the result is deterministic.

## M2 · Authority

The engine integration lives in `engine/tactics.go` with small hooks in the existing flow; state in `state/tactics.go` (`Settled.Tactics` + `Settled.TacticsRuntime`); the viewer-safe view in `snapshot/tactics.go` (`snapshot.tactics`).

- **Pinning.** `start_battle` takes an optional `tactics` setup (arena, rules with overrides already applied, every seat's start square). The authority refuses it unless unified defense and simultaneous play are both pinned, and only a tactics battle may have five seats (a hero and four enemies). The arena, the rules and every creature's speed and role are pinned in the battle.
- **Offense.** At Offensive entry every creature's square is recorded as the round's start. Legal actions offer only targets in range and in line of sight from those squares: creatures for direct attacks, creatures within reach + 1 for a Charge, squares for an area attack (`target_square` in the ability payload; a wall square is never a target). Skipping the offense (Pass in Offense Before) records a Sprint. An area attack's square stays private to its owner until the offense reveal, like every other planning choice.
- **Movement step.** After the offense reveal the attacks are recorded with their delivery and a new secret simultaneous step, `movement`, opens for everyone at once. A plan is one command, `tactics_movement_plan`: intention (move to a square, pursue or flee a creature, hold), the exact cards discarded for Move, map Move cards played for their amount (with the terrain for a choose-terrain card), and energy; `lock_in` locks it in, otherwise it replaces the draft. The authority validates every plan, prices it (budget, Running) and keeps a preview path assuming everyone else holds; legal actions offer the current plan locked in and every plan one change away (another intention, a card or map card in or out, another energy amount). A fixed Charge (`charge_allows_extra_move: false`) is locked in automatically. Plans, their events and their legal actions are visible only to their owner; the others see a lock-in marker.
- **Resolution.** When everyone is locked in: committed cards go to discard (played map cards to their play destination), energy is spent, one battle random draw seeds the round's tiebreaks, `tactics.Simulate` resolves the movement, plans that ran get Running, and attacks connect: direct always, area against every opponent of the caster standing in its area at t = 1, a charge if the charger touched its target. The attacker's targets become the creatures hit, so an area attack makes one damage source per creature, each with its own reservations and defense. An attack that misses does nothing (`tacticsMissed`). The resolution (timeline, start and final squares, budgets, Running, the revealed plans with other participants' discarded cards shown as a count) is public. Then the usual Defense opens.
- **Running.** Read by its `on_the_move_defenses_only` modifier, never by ID: only `on_the_move` defenses are offered or accepted; `snapshot.tactics.disabled_defenses` lists the others with the reason. Running expires at the next round's Ongoing Effects (a trigger in its content file, like Protect).
- **Refusals.** `mlsim` (environment and both encoders), the learned v1/v2/v3 policies, the session's scripted minions, the look-ahead opponent and every balance-lab player return `mlsim.ErrTacticsBattle` for a tactics battle.
- **Runner and ops** (`internal/battle/tacticsbattle`, dispatched from `learned/runtime.go`): `tactics_catalog` (heroes, enemies, arenas, rules, published map Move cards), `tactics_start`, `tactics_open` (resume) and `tactics_submit`. Battles are stored with the workspace's battle root (`<battles>/tactics/`) beside a small file holding the setup and each enemy seat's pinned scripted behaviour. After every hero command the runner lets each enemy answer from its own viewer-safe view and legal actions until only the hero has a decision. A per-request write-behind cache keeps the checkpoint in memory and writes it once, so a hero command with four enemies answering takes well under a second (it was over two seconds when every enemy view reloaded the checkpoint from disk).

Tests: `engine/tactics_test.go` (legality, connections, one source per creature hit, charge contact, Sprint, map cards with ignore/choose terrain, energy, spent commitments, Hold and Charge limits, Running and the on-the-move filter and expiry, viewer safety, save/reload during movement and defense, determinism, the rule needing the other two), `battle/tactics_authority_test.go`, the refusal tests in `mlsim`, `learned` and `lookahead`, and `tacticsbattle/runner_test.go` (catalog; a whole battle through the ops, then resume).

## M3 · Enemy behaviour

`tacticsbattle/policy.go` (`scriptedPolicy`), reading only the enemy seat's own viewer-safe view and legal actions, with each seat's scripted settings pinned at start:

- **Offense:** the shipped single-ability roll policy (lock in by rolling, keep the target faces, reroll the rest while rolls remain, play the configured card — Brine Surge — once a round on the attack it will use). A ranged or area enemy takes its ranged/area attack when it has a legal target (an area attack only at the player's current square), then its backup melee; otherwise no attack. Enemies never sprint.
- **Movement:** melee enemies Pursue the nearest opponent (the player). Ranged and area enemies Flee when adjacent, Move to the nearest square in range and line of sight when out of range or sight (cheapest in Move, then nearest in a straight line), otherwise Hold. An enemy commits the fewest cards, up to `ai.max_cards_per_round`, that bring it into range by the end of the round if everyone else held (checked with the same preview the player sees), and never its last card in hand. No energy.
- **Defense:** its defense on each incoming attack while it is legal (Running leaves only on-the-move defenses: the sandbox Ribbon Eel keeps Slip the Current), otherwise the main Pass.

Tests: `policy_test.go` (pursuit and the one-card rule, flee / hold / move into range and sight, range before backup melee, area aim, defense or pass) and `battles_test.go`, which plays 20 whole battles through the runner — every arena, both heroes, each enemy alone and in mixes up to all four — with a simple scripted hero: no refused command, no stuck decision, the slowest hero command about 80 ms.

**Finding (for the user):** in those 20 battles enemies used Brine Lash 49 times and Tide Crown 29 times but their backup melee (Brine Slap, Crown Strike) never. A ranged or area attack also reaches adjacent squares, and the backup melee qualifies on exactly the same dice, so "prefer range, then melee" never reaches the melee. See "Questions for the user".

## Defaults to confirm (plan §13)

All live in `content/tactics_v1/rules.yaml` or the combatant files and can be overridden per battle from the setup screen.

1. Charge does not count as running (`charge_counts_as_running: false`) and cannot add cards or energy (`charge_allows_extra_move: false`).
2. On-the-move defenses are usable whether or not you ran (Evade, sandbox Slip the Current).
3. Area attacks don't hit the caster's allies (`area_hits_allies: false`).
4. Creatures block line of sight (`los_blocked_by_creatures: true`); corners are permissive (both squares must block).
5. During defense, tokens stay put with path and ghost previews; full playback after the commit.
6. Arenas 12 × 9; enemy speeds (Ribbon Eel 2, others 1) and backup melee (Brine Slap 1 per Brine; Crown Strike = Tide Crown −2, minimum 1); Evade (2 dice: 4–5 prevent 1, 6 prevents 2; once per defensive segment, like Guard; saved cards to discard).
7. Enemy AI commits at most 1 card per round (`ai.max_cards_per_round: 1`).
8. Map Move cards give their full map amount (`map_card_move_multiplier: 1`).
9. Diagonals: Chebyshev (`diagonal_rule`), no corner cutting (`corner_cutting: false`).

## Interpretations to confirm

Small gaps the plan does not settle; each is the most natural reading and is listed so it can be changed.

- **Alternating diagonals** apply to every grid distance (range, flee distance) and to movement cost (every second diagonal step costs double, times the terrain cost), not only to movement.
- **Corner cutting** is about walls (impassable terrain) only; creatures never stop a diagonal step past them.
- **Charge already adjacent** at the start of movement connects at t = 0.
- **Hold never runs**, even while sprinting: a Hold plan commits nothing and does not move, so it has no Move budget.
- **Area targets** must be a square you could see: in range, in line of sight, and not a wall. The caster may aim at its own square; it is never hit by its own area attack.
- **An attack that does not connect** (dodged, out of reach) has no effect at all; any energy it cost stays spent.
- **Last card:** an enemy "never commits its last card" read as its last card in hand.
- **Defeated creatures** leave the grid: they no longer block movement or line of sight.
- **Map Move cards** may be played for their amount (paying their printed energy cost) or discarded like any card for +1 Move. Their map-only extra energy cost (`energy_cost`) is not charged in battle.
- **Running** has negative polarity, so a status-removal card could remove it; no sandbox deck holds one today.

## Questions for the user

1. **Backup melee is never used.** Ranged and area attacks also reach adjacent squares and the backup melee qualifies on the same dice, so the §8 preference never picks it. Options: a minimum range for ranged/area attacks (e.g. 2), or the AI preferring its backup melee when adjacent, or dropping the backup melee.
