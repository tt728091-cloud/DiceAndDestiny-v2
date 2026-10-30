# Tactics Sandbox: progress log

Implementation log for `tactics-sandbox-plan.md` (the plan is the source of truth). Branch: `t3/tactics-sandbox-poc`.

## Status

| Milestone | State |
|---|---|
| M0 · Read and scaffold | done |
| M1 · `internal/battle/tactics` (pure) | done |
| M2 · Authority | — |
| M3 · Enemy behaviour | — |
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
