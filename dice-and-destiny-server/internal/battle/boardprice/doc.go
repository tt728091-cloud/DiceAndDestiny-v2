// Package boardprice prices player ability boards: a die, a board template
// and the player's slot picks become abilities whose damage pays more the
// harder they are to roll, while every board averages the same per turn.
//
// # Menus
//
// LoadMenus reads the Board Builder's choices, all data:
// content/progression_v1/board_templates.yaml (pricing numbers, difficulty
// bands, dice, defenses, each family's flex cost menu and the templates) and
// content/progression_v1/riders.yaml (riders and the exchange rates every
// rider is priced at). Costs are written in die roles (content.DieRoles), so
// one template serves every symbol order of its family. Every menu entry
// carries an unlock rule; the first pass accepts only "open".
//
// # Pricing
//
// Price is the agreed method, ported from
// docs/game-design/dice-throne-reference/analysis/board_balance.py (parity
// tests reproduce its Starter, Lean and Monk numbers):
//
//  1. chase odds: the chance to roll a tier with 5 dice and 3 rolls when
//     every keep chases that tier alone;
//  2. relative payoff (1 ÷ chase) ^ alpha (alpha 0.2), a guaranteed hit is 1;
//  3. the board is played exactly (every hand, every distinct keep, backward
//     induction over rolls left, the most valuable qualified tier used, ties
//     sharing) for each tier's use share; the multiplier is target ÷
//     Σ(use × relative) and each tier's value is relative × multiplier;
//  4. damage is value − rider (riders at the exchange rates), never below the
//     minimum;
//  5. every harder tier of one ability deals at least the ladder step (1)
//     more than the tier below it: a ladder too flat for that is spread
//     around its average and the multiplier solved again so the board still
//     averages the target (the reference script has no step; parity runs
//     without it);
//  6. rounded by playing every monotone floor/ceiling rounding that keeps
//     the steps and keeping the one within 2% of the target whose use
//     shares drift least.
//
// Qualification is the engine's own (engine.QualifiedAbilityTier). Reports
// are cached by board key.
//
// # Interface for the AI work (keep stable)
//
//   - Price(die, board, options) (Report, error): Options carry the target,
//     alpha and rates (Menus.Options gives the agreed ones). The Report gives
//     per tier: chase, relative, use (exact and whole), value, rider value,
//     damage and whole damage; and Σ, the multiplier (and the stepped
//     board's damage multiplier), the averages and the no-hit rates, plus
//     the keep policy after rolls 1 and 2.
//   - Builder.RandomBoard(rng, family) returns a seeded random Choice: a die,
//     a template and slot picks (cost and rider) plus defenses.
//     Builder.Generate turns a priced choice into content ability
//     definitions with the die's symbol IDs.
//   - Exchange rates live only in riders.yaml (Menus.Rates); never copy them
//     into code.
//
// The package must not import internal/battle/lookahead (the AI work).
package boardprice
