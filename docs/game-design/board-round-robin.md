# Board round robin

Results of `go run ./cmd/board-lab -roundrobin` (2026-10-22). Every default board plays every other with the cards taken out: blank decks (health only), no riders, the same defense on both sides. The scenarios are no defense, Light Guard (a weak one-die defense) and Holdfast (the one in use). The question is whether, on abilities and dice alone, every board gets a fair share against the field, and whether any board is so weak or so strong that it should be fixed or removed. See the B7 results in `dice-and-board-selection-plan.md` for how this was reached.

**How it is scored.** Each cell uses the race model, the exact chance of winning when both sides attack every round with the board's priced odds per tier and the other side's defense roll comes off. It is averaged over starting health 12–30, because a single deck size can favour boards whose hits happen to divide it evenly. In the pure duels the model matched real battles within their noise. Here 48 sampled pairings were played again as real battles (table at the end). A board's **overall** score is its mean against the other 22 boards over all three defenses. A matchup is **lopsided** when one side wins 60% or more.

## Findings

- **Priced now:** overall scores run 0.442–0.556, and 26 of the 759 pairings (board pairs × defenses) are lopsided.
  - **All 26 are under Holdfast.** None happen with no defense or Light Guard.
  - **All are won by the five hard-hitting boards:** Spellbook on the Loom Die (12), Combo on the standard die (6), Two Mains on the Forge Die (4), Combo on the Hearth Die (2) and Spellbook on the Forge Die (2).
  - **The worst is Standard on the Loom Die against Spellbook on the Loom Die, at 0.319.**
  - **The weakest boards overall** are Standard on the Loom Die (0.442) and Lean on the standard die (0.443). Both are boards whose rounding left them about 2% below the 6.2 target. Next come Two Mains on the Loom Die (0.471) and the Grove Die boards (0.473–0.480).
- **No board needs removing.** Moving each board's damage by at most about 3% ("Tuned") brings every board to 0.475–0.524 overall with **no lopsided pairing under any defense**. What is left is mild rock-paper-scissors:
  - the hard hitters do better against Holdfast and slightly worse with no defense, with the largest gap 0.063 (Two Mains on the Forge Die);
  - the Grove Die boards do the reverse.
- **Whole damage limits how close a board can get.** One more point on a common combo moves a board's average about 3% and its overall score about 6 points. So ±0.025 overall is as close as whole numbers allow.
- **One rule does almost as well as tuning each board.** "Priced on damage through the defenses" aims every board's average damage through the three defenses, rolled on the standard die, at the reference board's. It gives 0.474–0.529 overall with 2 lopsided pairings, against the tuned field's 0.475–0.524 and 0. It is a formula, so it prices any board the Board Builder can make, not just these 23.
- **Its two misses are Grove Die boards**, and they come from **Holdfast itself**. On the Grove Die (3-1-1-1) Holdfast averages the same 3.5 but is swingier: the second odd face prevents 3. A swingier defense lets more damage through, and it costs Grove boards about 2 points against the Holdfast field. Rolled like the standard die's, Rainbow goes from 0.441 to 0.460 against that field.
- **The results hold at other deck sizes.** Longer battles widen every gap:

  | Health per cell | Priced now | Through-defense rule | Tuned |
  |---|---|---|---|
  | 15 only | 0.463–0.542, 18 lopsided | 0.484–0.519, 0 | 0.482–0.517, 0 |
  | 12–30 (used below) | 0.442–0.556, 26 lopsided | 0.474–0.529, 2 | 0.475–0.524, 0 |
  | 20–40 | 0.426–0.571, 53 lopsided | 0.468–0.537, 5 | 0.469–0.530, 0 |

## Results

`go run ./cmd/board-lab -roundrobin` · every default board against every other · blank decks (health only), no riders, the same defense on both sides · race model (both sides attack every round with the priced odds per tier, through the other's defense roll), each cell averaged over starting health 12–30 · score = wins + ½ draws.

Defenses: **no defense**; **Light Guard** (1 die: 1–2 prevent nothing, 3–4 prevent 1, 5–6 prevent 2; 1 on average); **Holdfast** (3 of your dice by role, 3.5 on average, up to 6, or 9 on a 3-1-1-1 die). A matchup below 0.40 (the other side wins 60% or more) counts as lopsided.

**Model check:** 48 sampled pairings played as real battles (1000 each, so ±0.031 per cell): typical gap from the model 0.014, largest 0.043.

**Priced now** — each board's mean score against the other 22 boards:

| # | Board | Avg / turn | no defense | Light Guard | Holdfast | Overall | Worst matchup | Lopsided losses |
|---|---|---|---|---|---|---|---|---|
| 1 | Lean · Standard D6 | 6.10 | 0.462 | 0.451 | 0.417 | **0.443** | 0.321 vs Spellbook · Loom (Holdfast) | 5 of 66 |
| 2 | Lean · Hearth | 6.23 | 0.498 | 0.494 | 0.482 | **0.492** | 0.376 vs Spellbook · Loom (Holdfast) | 2 of 66 |
| 3 | Standard · Standard D6 | 6.23 | 0.497 | 0.494 | 0.485 | **0.492** | 0.382 vs Spellbook · Loom (Holdfast) | 1 of 66 |
| 4 | Standard · Hearth | 6.23 | 0.496 | 0.493 | 0.484 | **0.491** | 0.381 vs Spellbook · Loom (Holdfast) | 1 of 66 |
| 5 | Compact · Standard D6 | 6.23 | 0.497 | 0.494 | 0.485 | **0.492** | 0.382 vs Spellbook · Loom (Holdfast) | 1 of 66 |
| 6 | Compact · Hearth | 6.23 | 0.496 | 0.493 | 0.484 | **0.491** | 0.381 vs Spellbook · Loom (Holdfast) | 1 of 66 |
| 7 | Wide · Standard D6 | 6.23 | 0.497 | 0.494 | 0.485 | **0.492** | 0.382 vs Spellbook · Loom (Holdfast) | 1 of 66 |
| 8 | Wide · Hearth | 6.25 | 0.502 | 0.500 | 0.494 | **0.499** | 0.389 vs Spellbook · Loom (Holdfast) | 1 of 66 |
| 9 | Double Ladder · Standard D6 | 6.32 | 0.521 | 0.521 | 0.519 | **0.520** | 0.411 vs Spellbook · Loom (Holdfast) | 0 of 66 |
| 10 | Double Ladder · Hearth | 6.32 | 0.521 | 0.520 | 0.519 | **0.520** | 0.412 vs Spellbook · Loom (Holdfast) | 0 of 66 |
| 11 | Combo · Standard D6 | 6.31 | 0.518 | 0.532 | 0.585 | **0.545** | 0.480 vs Spellbook · Loom (Holdfast) | 0 of 66 |
| 12 | Combo · Hearth | 6.23 | 0.499 | 0.506 | 0.539 | **0.514** | 0.432 vs Spellbook · Loom (Holdfast) | 0 of 66 |
| 13 | Rainbow · Grove | 6.21 | 0.493 | 0.485 | 0.441 | **0.473** | 0.351 vs Spellbook · Loom (Holdfast) | 3 of 66 |
| 14 | Wide Rainbow · Grove | 6.32 | 0.520 | 0.519 | 0.496 | **0.512** | 0.403 vs Spellbook · Loom (Holdfast) | 0 of 66 |
| 15 | Suits · Grove | 6.23 | 0.499 | 0.492 | 0.451 | **0.480** | 0.360 vs Spellbook · Loom (Holdfast) | 3 of 66 |
| 16 | Standard · Forge | 6.29 | 0.514 | 0.514 | 0.513 | **0.513** | 0.404 vs Spellbook · Loom (Holdfast) | 0 of 66 |
| 17 | Standard · Loom | 6.08 | 0.460 | 0.449 | 0.418 | **0.442** | 0.319 vs Spellbook · Loom (Holdfast) | 5 of 66 |
| 18 | Two Mains · Forge | 6.22 | 0.499 | 0.512 | 0.561 | **0.524** | 0.454 vs Spellbook · Loom (Holdfast) | 0 of 66 |
| 19 | Two Mains · Loom | 6.08 | 0.467 | 0.467 | 0.479 | **0.471** | 0.375 vs Spellbook · Loom (Holdfast) | 2 of 66 |
| 20 | Spellbook · Forge | 6.17 | 0.488 | 0.499 | 0.536 | **0.508** | 0.426 vs Spellbook · Loom (Holdfast) | 0 of 66 |
| 21 | Spellbook · Loom | 6.31 | 0.521 | 0.539 | 0.608 | **0.556** | 0.502 vs Double Ladder · Standard D6 (no defense) | 0 of 66 |
| 22 | Ladder Pair · Forge | 6.31 | 0.518 | 0.516 | 0.511 | **0.515** | 0.404 vs Spellbook · Loom (Holdfast) | 0 of 66 |
| 23 | Ladder Pair · Loom | 6.31 | 0.518 | 0.517 | 0.511 | **0.515** | 0.404 vs Spellbook · Loom (Holdfast) | 0 of 66 |

Overall scores span 0.442–0.556; 26 of the 759 pairings (board pairs × defenses) are lopsided.

**Priced on damage through the defenses** (one rule for any board, not tuned against the field): every board's target set so that its average damage per turn through the three defenses, rolled on the standard die and averaged over them, matches the Standard template's on the standard die priced now (4.75).

**Priced on damage through the defenses** — each board's mean score against the other 22 boards:

| # | Board | Avg / turn | Change | no defense | Light Guard | Holdfast | Overall | Worst matchup | Lopsided losses |
|---|---|---|---|---|---|---|---|---|---|
| 1 | Lean · Standard D6 | 6.23 | +2.1% | 0.502 | 0.498 | 0.486 | **0.495** | 0.418 vs Two Mains · Forge (Holdfast) | 0 of 66 |
| 2 | Lean · Hearth | 6.23 | +0.1% | 0.502 | 0.499 | 0.490 | **0.497** | 0.423 vs Two Mains · Forge (Holdfast) | 0 of 66 |
| 3 | Standard · Standard D6 | 6.23 | +0.0% | 0.501 | 0.498 | 0.492 | **0.497** | 0.426 vs Two Mains · Forge (Holdfast) | 0 of 66 |
| 4 | Standard · Hearth | 6.23 | +0.0% | 0.499 | 0.497 | 0.491 | **0.496** | 0.425 vs Two Mains · Forge (Holdfast) | 0 of 66 |
| 5 | Compact · Standard D6 | 6.23 | +0.0% | 0.501 | 0.498 | 0.492 | **0.497** | 0.426 vs Two Mains · Forge (Holdfast) | 0 of 66 |
| 6 | Compact · Hearth | 6.23 | +0.0% | 0.499 | 0.497 | 0.491 | **0.496** | 0.425 vs Two Mains · Forge (Holdfast) | 0 of 66 |
| 7 | Wide · Standard D6 | 6.23 | +0.0% | 0.501 | 0.498 | 0.492 | **0.497** | 0.426 vs Two Mains · Forge (Holdfast) | 0 of 66 |
| 8 | Wide · Hearth | 6.24 | -0.2% | 0.504 | 0.501 | 0.493 | **0.499** | 0.426 vs Two Mains · Forge (Holdfast) | 0 of 66 |
| 9 | Double Ladder · Standard D6 | 6.31 | -0.1% | 0.523 | 0.523 | 0.521 | **0.522** | 0.452 vs Two Mains · Forge (Holdfast) | 0 of 66 |
| 10 | Double Ladder · Hearth | 6.31 | -0.2% | 0.522 | 0.522 | 0.521 | **0.522** | 0.452 vs Two Mains · Forge (Holdfast) | 0 of 66 |
| 11 | Combo · Standard D6 | 6.13 | -2.7% | 0.480 | 0.481 | 0.495 | **0.485** | 0.428 vs Two Mains · Forge (Holdfast) | 0 of 66 |
| 12 | Combo · Hearth | 6.18 | -0.7% | 0.492 | 0.496 | 0.525 | **0.504** | 0.459 vs Two Mains · Forge (Holdfast) | 0 of 66 |
| 13 | Rainbow · Grove | 6.22 | +0.2% | 0.499 | 0.493 | 0.453 | **0.481** | 0.396 vs Two Mains · Forge (Holdfast) | 1 of 66 |
| 14 | Wide Rainbow · Grove | 6.20 | -2.0% | 0.489 | 0.484 | 0.448 | **0.474** | 0.394 vs Two Mains · Forge (Holdfast) | 1 of 66 |
| 15 | Suits · Grove | 6.24 | +0.2% | 0.505 | 0.500 | 0.463 | **0.489** | 0.405 vs Two Mains · Forge (Holdfast) | 0 of 66 |
| 16 | Standard · Forge | 6.21 | -1.3% | 0.496 | 0.492 | 0.481 | **0.490** | 0.414 vs Two Mains · Forge (Holdfast) | 0 of 66 |
| 17 | Standard · Loom | 6.20 | +1.9% | 0.492 | 0.490 | 0.483 | **0.488** | 0.417 vs Two Mains · Forge (Holdfast) | 0 of 66 |
| 18 | Two Mains · Forge | 6.22 | +0.0% | 0.502 | 0.516 | 0.569 | **0.529** | 0.482 vs Double Ladder · Standard D6 (no defense) | 0 of 66 |
| 19 | Two Mains · Loom | 6.10 | +0.2% | 0.472 | 0.475 | 0.495 | **0.481** | 0.429 vs Two Mains · Forge (Holdfast) | 0 of 66 |
| 20 | Spellbook · Forge | 6.17 | +0.0% | 0.491 | 0.503 | 0.544 | **0.513** | 0.470 vs Double Ladder · Standard D6 (no defense) | 0 of 66 |
| 21 | Spellbook · Loom | 6.15 | -2.6% | 0.485 | 0.497 | 0.540 | **0.507** | 0.465 vs Double Ladder · Standard D6 (no defense) | 0 of 66 |
| 22 | Ladder Pair · Forge | 6.31 | +0.0% | 0.521 | 0.521 | 0.518 | **0.520** | 0.450 vs Two Mains · Forge (Holdfast) | 0 of 66 |
| 23 | Ladder Pair · Loom | 6.31 | +0.0% | 0.522 | 0.521 | 0.518 | **0.521** | 0.450 vs Two Mains · Forge (Holdfast) | 0 of 66 |

Overall scores span 0.474–0.529; 2 of the 759 pairings (board pairs × defenses) are lopsided.

**Tuned:** every board's pricing target moved, all at once over 3 rounds, until its overall score is as close to 0.5 as whole damage allows. Change is the average damage per turn against the board priced now.

**Tuned** — each board's mean score against the other 22 boards:

| # | Board | Avg / turn | Change | no defense | Light Guard | Holdfast | Overall | Worst matchup | Lopsided losses |
|---|---|---|---|---|---|---|---|---|---|
| 1 | Lean · Standard D6 | 6.23 | +2.1% | 0.497 | 0.492 | 0.478 | **0.489** | 0.418 vs Two Mains · Forge (Holdfast) | 0 of 66 |
| 2 | Lean · Hearth | 6.25 | +0.3% | 0.501 | 0.498 | 0.489 | **0.496** | 0.430 vs Two Mains · Forge (Holdfast) | 0 of 66 |
| 3 | Standard · Standard D6 | 6.23 | +0.0% | 0.496 | 0.493 | 0.484 | **0.491** | 0.426 vs Two Mains · Forge (Holdfast) | 0 of 66 |
| 4 | Standard · Hearth | 6.23 | +0.0% | 0.495 | 0.492 | 0.483 | **0.490** | 0.425 vs Two Mains · Forge (Holdfast) | 0 of 66 |
| 5 | Compact · Standard D6 | 6.23 | +0.0% | 0.496 | 0.493 | 0.484 | **0.491** | 0.426 vs Two Mains · Forge (Holdfast) | 0 of 66 |
| 6 | Compact · Hearth | 6.23 | +0.0% | 0.495 | 0.492 | 0.483 | **0.490** | 0.425 vs Two Mains · Forge (Holdfast) | 0 of 66 |
| 7 | Wide · Standard D6 | 6.26 | +0.4% | 0.504 | 0.502 | 0.495 | **0.500** | 0.436 vs Two Mains · Forge (Holdfast) | 0 of 66 |
| 8 | Wide · Hearth | 6.25 | +0.0% | 0.501 | 0.499 | 0.493 | **0.498** | 0.435 vs Two Mains · Forge (Holdfast) | 0 of 66 |
| 9 | Double Ladder · Standard D6 | 6.31 | -0.1% | 0.519 | 0.518 | 0.514 | **0.517** | 0.452 vs Two Mains · Forge (Holdfast) | 0 of 66 |
| 10 | Double Ladder · Hearth | 6.31 | -0.2% | 0.518 | 0.517 | 0.513 | **0.516** | 0.452 vs Two Mains · Forge (Holdfast) | 0 of 66 |
| 11 | Combo · Standard D6 | 6.13 | -2.7% | 0.476 | 0.476 | 0.487 | **0.480** | 0.428 vs Two Mains · Forge (Holdfast) | 0 of 66 |
| 12 | Combo · Hearth | 6.19 | -0.6% | 0.488 | 0.492 | 0.519 | **0.500** | 0.460 vs Two Mains · Forge (Holdfast) | 0 of 66 |
| 13 | Rainbow · Grove | 6.32 | +1.9% | 0.519 | 0.519 | 0.495 | **0.511** | 0.444 vs Two Mains · Forge (Holdfast) | 0 of 66 |
| 14 | Wide Rainbow · Grove | 6.29 | -0.5% | 0.512 | 0.510 | 0.480 | **0.501** | 0.429 vs Two Mains · Forge (Holdfast) | 0 of 66 |
| 15 | Suits · Grove | 6.28 | +0.9% | 0.513 | 0.509 | 0.473 | **0.498** | 0.421 vs Two Mains · Forge (Holdfast) | 0 of 66 |
| 16 | Standard · Forge | 6.29 | +0.0% | 0.513 | 0.513 | 0.513 | **0.513** | 0.451 vs Two Mains · Forge (Holdfast) | 0 of 66 |
| 17 | Standard · Loom | 6.20 | +1.9% | 0.487 | 0.484 | 0.475 | **0.482** | 0.417 vs Two Mains · Forge (Holdfast) | 0 of 66 |
| 18 | Two Mains · Forge | 6.22 | +0.0% | 0.498 | 0.511 | 0.561 | **0.524** | 0.482 vs Rainbow · Grove (no defense) | 0 of 66 |
| 19 | Two Mains · Loom | 6.10 | +0.2% | 0.468 | 0.470 | 0.488 | **0.475** | 0.429 vs Two Mains · Forge (Holdfast) | 0 of 66 |
| 20 | Spellbook · Forge | 6.17 | +0.0% | 0.487 | 0.498 | 0.536 | **0.507** | 0.470 vs Double Ladder · Standard D6 (no defense) | 0 of 66 |
| 21 | Spellbook · Loom | 6.15 | -2.6% | 0.481 | 0.492 | 0.532 | **0.502** | 0.465 vs Double Ladder · Standard D6 (no defense) | 0 of 66 |
| 22 | Ladder Pair · Forge | 6.31 | +0.0% | 0.517 | 0.515 | 0.511 | **0.514** | 0.450 vs Two Mains · Forge (Holdfast) | 0 of 66 |
| 23 | Ladder Pair · Loom | 6.31 | +0.0% | 0.518 | 0.516 | 0.511 | **0.515** | 0.450 vs Two Mains · Forge (Holdfast) | 0 of 66 |

Overall scores span 0.475–0.524; 0 of the 759 pairings (board pairs × defenses) are lopsided.

**Priced now, no defense** — row board's score against the column board, in %:

| # | Board | 1 | 2 | 3 | 4 | 5 | 6 | 7 | 8 | 9 | 10 | 11 | 12 | 13 | 14 | 15 | 16 | 17 | 18 | 19 | 20 | 21 | 22 | 23 | Mean |
|---|---|---|---|---|---|---|---|---|---|---|---|---|---|---|---|---|---|---|---|---|---|---|---|---|---|
| 1 | Lean · Standard D6 | — | 46 | 47 | 47 | 47 | 47 | 47 | 46 | 44 | 44 | 45 | 47 | 47 | 44 | 46 | 45 | 50 | 47 | 50 | 48 | 45 | 45 | 44 | 46.2 |
| 2 | Lean · Hearth | 54 | — | 50 | 50 | 50 | 50 | 50 | 50 | 48 | 48 | 48 | 50 | 51 | 48 | 50 | 49 | 54 | 50 | 53 | 51 | 48 | 48 | 48 | 49.8 |
| 3 | Standard · Standard D6 | 53 | 50 | — | 50 | 50 | 50 | 50 | 50 | 48 | 48 | 48 | 50 | 50 | 48 | 50 | 48 | 54 | 50 | 53 | 51 | 48 | 48 | 48 | 49.7 |
| 4 | Standard · Hearth | 53 | 50 | 50 | — | 50 | 50 | 50 | 49 | 48 | 48 | 48 | 50 | 50 | 48 | 50 | 48 | 53 | 50 | 53 | 51 | 48 | 48 | 48 | 49.6 |
| 5 | Compact · Standard D6 | 53 | 50 | 50 | 50 | — | 50 | 50 | 50 | 48 | 48 | 48 | 50 | 50 | 48 | 50 | 48 | 54 | 50 | 53 | 51 | 48 | 48 | 48 | 49.7 |
| 6 | Compact · Hearth | 53 | 50 | 50 | 50 | 50 | — | 50 | 49 | 48 | 48 | 48 | 50 | 50 | 48 | 50 | 48 | 53 | 50 | 53 | 51 | 48 | 48 | 48 | 49.6 |
| 7 | Wide · Standard D6 | 53 | 50 | 50 | 50 | 50 | 50 | — | 50 | 48 | 48 | 48 | 50 | 50 | 48 | 50 | 48 | 54 | 50 | 53 | 51 | 48 | 48 | 48 | 49.7 |
| 8 | Wide · Hearth | 54 | 50 | 50 | 51 | 50 | 51 | 50 | — | 48 | 48 | 48 | 50 | 51 | 48 | 50 | 49 | 54 | 50 | 53 | 51 | 48 | 49 | 48 | 50.2 |
| 9 | Double Ladder · Standard D6 | 56 | 52 | 52 | 52 | 52 | 52 | 52 | 52 | — | 50 | 50 | 52 | 53 | 50 | 52 | 51 | 56 | 52 | 55 | 53 | 50 | 50 | 50 | 52.1 |
| 10 | Double Ladder · Hearth | 56 | 52 | 52 | 52 | 52 | 52 | 52 | 52 | 50 | — | 50 | 52 | 53 | 50 | 52 | 51 | 56 | 52 | 55 | 53 | 50 | 50 | 50 | 52.1 |
| 11 | Combo · Standard D6 | 55 | 52 | 52 | 52 | 52 | 52 | 52 | 52 | 50 | 50 | — | 52 | 53 | 50 | 52 | 51 | 55 | 52 | 55 | 53 | 50 | 50 | 50 | 51.8 |
| 12 | Combo · Hearth | 53 | 50 | 50 | 50 | 50 | 50 | 50 | 50 | 48 | 48 | 48 | — | 51 | 48 | 50 | 49 | 53 | 50 | 53 | 51 | 48 | 48 | 48 | 49.9 |
| 13 | Rainbow · Grove | 53 | 49 | 50 | 50 | 50 | 50 | 50 | 49 | 47 | 47 | 47 | 49 | — | 47 | 49 | 48 | 53 | 49 | 53 | 51 | 47 | 48 | 47 | 49.3 |
| 14 | Wide Rainbow · Grove | 56 | 52 | 52 | 52 | 52 | 52 | 52 | 52 | 50 | 50 | 50 | 52 | 53 | — | 52 | 51 | 56 | 52 | 55 | 53 | 50 | 50 | 50 | 52.0 |
| 15 | Suits · Grove | 54 | 50 | 50 | 50 | 50 | 50 | 50 | 50 | 48 | 48 | 48 | 50 | 51 | 48 | — | 49 | 54 | 50 | 53 | 51 | 48 | 48 | 48 | 49.9 |
| 16 | Standard · Forge | 55 | 51 | 52 | 52 | 52 | 52 | 52 | 51 | 49 | 49 | 49 | 51 | 52 | 49 | 51 | — | 55 | 51 | 54 | 52 | 49 | 50 | 50 | 51.4 |
| 17 | Standard · Loom | 50 | 46 | 46 | 47 | 46 | 47 | 46 | 46 | 44 | 44 | 45 | 47 | 47 | 44 | 46 | 45 | — | 47 | 50 | 48 | 44 | 44 | 44 | 46.0 |
| 18 | Two Mains · Forge | 53 | 50 | 50 | 50 | 50 | 50 | 50 | 50 | 48 | 48 | 48 | 50 | 51 | 48 | 50 | 49 | 53 | — | 53 | 51 | 48 | 48 | 48 | 49.9 |
| 19 | Two Mains · Loom | 50 | 47 | 47 | 47 | 47 | 47 | 47 | 47 | 45 | 45 | 45 | 47 | 47 | 45 | 47 | 46 | 50 | 47 | — | 48 | 45 | 45 | 45 | 46.7 |
| 20 | Spellbook · Forge | 52 | 49 | 49 | 49 | 49 | 49 | 49 | 49 | 47 | 47 | 47 | 49 | 49 | 47 | 49 | 48 | 52 | 49 | 52 | — | 47 | 47 | 47 | 48.8 |
| 21 | Spellbook · Loom | 55 | 52 | 52 | 52 | 52 | 52 | 52 | 52 | 50 | 50 | 50 | 52 | 53 | 50 | 52 | 51 | 56 | 52 | 55 | 53 | — | 50 | 50 | 52.1 |
| 22 | Ladder Pair · Forge | 55 | 52 | 52 | 52 | 52 | 52 | 52 | 51 | 50 | 50 | 50 | 52 | 52 | 50 | 52 | 50 | 56 | 52 | 55 | 53 | 50 | — | 50 | 51.8 |
| 23 | Ladder Pair · Loom | 56 | 52 | 52 | 52 | 52 | 52 | 52 | 52 | 50 | 50 | 50 | 52 | 53 | 50 | 52 | 50 | 56 | 52 | 55 | 53 | 50 | 50 | — | 51.8 |

**Priced now, Light Guard** — row board's score against the column board, in %:

| # | Board | 1 | 2 | 3 | 4 | 5 | 6 | 7 | 8 | 9 | 10 | 11 | 12 | 13 | 14 | 15 | 16 | 17 | 18 | 19 | 20 | 21 | 22 | 23 | Mean |
|---|---|---|---|---|---|---|---|---|---|---|---|---|---|---|---|---|---|---|---|---|---|---|---|---|---|
| 1 | Lean · Standard D6 | — | 46 | 46 | 46 | 46 | 46 | 46 | 45 | 43 | 43 | 43 | 45 | 47 | 43 | 46 | 44 | 50 | 44 | 49 | 46 | 42 | 44 | 44 | 45.1 |
| 2 | Lean · Hearth | 54 | — | 50 | 50 | 50 | 50 | 50 | 49 | 47 | 47 | 46 | 49 | 51 | 48 | 50 | 48 | 54 | 48 | 53 | 50 | 46 | 48 | 48 | 49.4 |
| 3 | Standard · Standard D6 | 54 | 50 | — | 50 | 50 | 50 | 50 | 49 | 47 | 47 | 46 | 49 | 51 | 48 | 50 | 48 | 54 | 48 | 53 | 50 | 46 | 48 | 48 | 49.4 |
| 4 | Standard · Hearth | 54 | 50 | 50 | — | 50 | 50 | 50 | 49 | 47 | 47 | 46 | 49 | 51 | 47 | 50 | 48 | 54 | 48 | 52 | 49 | 46 | 48 | 48 | 49.3 |
| 5 | Compact · Standard D6 | 54 | 50 | 50 | 50 | — | 50 | 50 | 49 | 47 | 47 | 46 | 49 | 51 | 48 | 50 | 48 | 54 | 48 | 53 | 50 | 46 | 48 | 48 | 49.4 |
| 6 | Compact · Hearth | 54 | 50 | 50 | 50 | 50 | — | 50 | 49 | 47 | 47 | 46 | 49 | 51 | 47 | 50 | 48 | 54 | 48 | 52 | 49 | 46 | 48 | 48 | 49.3 |
| 7 | Wide · Standard D6 | 54 | 50 | 50 | 50 | 50 | 50 | — | 49 | 47 | 47 | 46 | 49 | 51 | 48 | 50 | 48 | 54 | 48 | 53 | 50 | 46 | 48 | 48 | 49.4 |
| 8 | Wide · Hearth | 55 | 51 | 51 | 51 | 51 | 51 | 51 | — | 48 | 48 | 47 | 49 | 51 | 48 | 51 | 49 | 55 | 49 | 53 | 50 | 46 | 48 | 48 | 50.0 |
| 9 | Double Ladder · Standard D6 | 57 | 53 | 53 | 53 | 53 | 53 | 53 | 52 | — | 50 | 49 | 51 | 54 | 50 | 53 | 51 | 57 | 51 | 55 | 52 | 48 | 50 | 50 | 52.1 |
| 10 | Double Ladder · Hearth | 57 | 53 | 53 | 53 | 53 | 53 | 53 | 52 | 50 | — | 49 | 51 | 54 | 50 | 53 | 51 | 57 | 51 | 55 | 52 | 48 | 50 | 50 | 52.0 |
| 11 | Combo · Standard D6 | 57 | 54 | 54 | 54 | 54 | 54 | 54 | 53 | 51 | 51 | — | 52 | 55 | 51 | 54 | 52 | 58 | 52 | 56 | 53 | 49 | 52 | 52 | 53.2 |
| 12 | Combo · Hearth | 55 | 51 | 51 | 51 | 51 | 51 | 51 | 51 | 49 | 49 | 48 | — | 52 | 49 | 51 | 49 | 55 | 49 | 54 | 51 | 47 | 49 | 49 | 50.6 |
| 13 | Rainbow · Grove | 53 | 49 | 49 | 49 | 49 | 49 | 49 | 49 | 46 | 46 | 45 | 48 | — | 47 | 49 | 47 | 54 | 47 | 52 | 49 | 45 | 47 | 47 | 48.5 |
| 14 | Wide Rainbow · Grove | 57 | 52 | 52 | 53 | 52 | 53 | 52 | 52 | 50 | 50 | 49 | 51 | 53 | — | 53 | 51 | 57 | 51 | 55 | 52 | 48 | 50 | 50 | 51.9 |
| 15 | Suits · Grove | 54 | 50 | 50 | 50 | 50 | 50 | 50 | 49 | 47 | 47 | 46 | 49 | 51 | 47 | — | 48 | 54 | 48 | 52 | 49 | 45 | 48 | 48 | 49.2 |
| 16 | Standard · Forge | 56 | 52 | 52 | 52 | 52 | 52 | 52 | 51 | 49 | 49 | 48 | 51 | 53 | 49 | 52 | — | 56 | 50 | 54 | 51 | 47 | 50 | 50 | 51.4 |
| 17 | Standard · Loom | 50 | 46 | 46 | 46 | 46 | 46 | 46 | 45 | 43 | 43 | 42 | 45 | 46 | 43 | 46 | 44 | — | 44 | 48 | 45 | 42 | 43 | 43 | 44.9 |
| 18 | Two Mains · Forge | 56 | 52 | 52 | 52 | 52 | 52 | 52 | 51 | 49 | 49 | 48 | 51 | 53 | 49 | 52 | 50 | 56 | — | 54 | 51 | 48 | 50 | 50 | 51.2 |
| 19 | Two Mains · Loom | 51 | 47 | 47 | 48 | 47 | 48 | 47 | 47 | 45 | 45 | 44 | 46 | 48 | 45 | 48 | 46 | 52 | 46 | — | 47 | 43 | 45 | 45 | 46.7 |
| 20 | Spellbook · Forge | 54 | 50 | 50 | 51 | 50 | 51 | 50 | 50 | 48 | 48 | 47 | 49 | 51 | 48 | 51 | 49 | 55 | 49 | 53 | — | 46 | 48 | 48 | 49.9 |
| 21 | Spellbook · Loom | 58 | 54 | 54 | 54 | 54 | 54 | 54 | 54 | 52 | 52 | 51 | 53 | 55 | 52 | 55 | 53 | 58 | 52 | 57 | 54 | — | 52 | 52 | 53.9 |
| 22 | Ladder Pair · Forge | 56 | 52 | 52 | 52 | 52 | 52 | 52 | 52 | 50 | 50 | 48 | 51 | 53 | 50 | 52 | 50 | 57 | 50 | 55 | 52 | 48 | — | 50 | 51.6 |
| 23 | Ladder Pair · Loom | 56 | 52 | 52 | 52 | 52 | 52 | 52 | 52 | 50 | 50 | 48 | 51 | 53 | 50 | 52 | 50 | 57 | 50 | 55 | 52 | 48 | 50 | — | 51.7 |

**Priced now, Holdfast** — row board's score against the column board, in %:

| # | Board | 1 | 2 | 3 | 4 | 5 | 6 | 7 | 8 | 9 | 10 | 11 | 12 | 13 | 14 | 15 | 16 | 17 | 18 | 19 | 20 | 21 | 22 | 23 | Mean |
|---|---|---|---|---|---|---|---|---|---|---|---|---|---|---|---|---|---|---|---|---|---|---|---|---|---|
| 1 | Lean · Standard D6 | — | 43 | 43 | 44 | 43 | 44 | 43 | 43 | 40 | 40 | **34** | **38** | 48 | 43 | 47 | 41 | 50 | **36** | 44 | **38** | **32** | 41 | 41 | 41.7 |
| 2 | Lean · Hearth | 57 | — | 50 | 50 | 50 | 50 | 50 | 49 | 46 | 46 | **40** | 44 | 54 | 49 | 53 | 47 | 56 | 42 | 50 | 45 | **38** | 47 | 47 | 48.2 |
| 3 | Standard · Standard D6 | 57 | 50 | — | 50 | 50 | 50 | 50 | 49 | 47 | 47 | 40 | 45 | 54 | 49 | 53 | 47 | 56 | 43 | 51 | 45 | **38** | 47 | 47 | 48.5 |
| 4 | Standard · Hearth | 56 | 50 | 50 | — | 50 | 50 | 50 | 49 | 47 | 47 | 40 | 45 | 54 | 49 | 53 | 47 | 56 | 43 | 50 | 45 | **38** | 47 | 47 | 48.4 |
| 5 | Compact · Standard D6 | 57 | 50 | 50 | 50 | — | 50 | 50 | 49 | 47 | 47 | 40 | 45 | 54 | 49 | 53 | 47 | 56 | 43 | 51 | 45 | **38** | 47 | 47 | 48.5 |
| 6 | Compact · Hearth | 56 | 50 | 50 | 50 | 50 | — | 50 | 49 | 47 | 47 | 40 | 45 | 54 | 49 | 53 | 47 | 56 | 43 | 50 | 45 | **38** | 47 | 47 | 48.4 |
| 7 | Wide · Standard D6 | 57 | 50 | 50 | 50 | 50 | 50 | — | 49 | 47 | 47 | 40 | 45 | 54 | 49 | 53 | 47 | 56 | 43 | 51 | 45 | **38** | 47 | 47 | 48.5 |
| 8 | Wide · Hearth | 57 | 51 | 51 | 51 | 51 | 51 | 51 | — | 48 | 48 | 41 | 46 | 55 | 50 | 54 | 48 | 57 | 43 | 51 | 46 | **39** | 48 | 48 | 49.4 |
| 9 | Double Ladder · Standard D6 | 60 | 54 | 53 | 53 | 53 | 53 | 53 | 52 | — | 50 | 43 | 48 | 57 | 52 | 56 | 51 | 60 | 46 | 54 | 48 | 41 | 51 | 51 | 51.9 |
| 10 | Double Ladder · Hearth | 60 | 54 | 53 | 53 | 53 | 53 | 53 | 52 | 50 | — | 43 | 48 | 57 | 52 | 57 | 51 | 60 | 46 | 54 | 48 | 41 | 51 | 51 | 51.9 |
| 11 | Combo · Standard D6 | 66 | 60 | 60 | 60 | 60 | 60 | 60 | 59 | 57 | 57 | — | 55 | 63 | 58 | 62 | 57 | 66 | 52 | 60 | 55 | 48 | 57 | 57 | 58.5 |
| 12 | Combo · Hearth | 62 | 56 | 55 | 55 | 55 | 55 | 55 | 54 | 52 | 52 | 45 | — | 59 | 54 | 58 | 53 | 62 | 48 | 56 | 50 | 43 | 53 | 53 | 53.9 |
| 13 | Rainbow · Grove | 52 | 46 | 46 | 46 | 46 | 46 | 46 | 45 | 43 | 43 | **37** | 41 | — | 45 | 49 | 43 | 51 | **39** | 46 | 41 | **35** | 43 | 43 | 44.1 |
| 14 | Wide Rainbow · Grove | 57 | 51 | 51 | 51 | 51 | 51 | 51 | 50 | 48 | 48 | 42 | 46 | 55 | — | 54 | 48 | 57 | 44 | 51 | 47 | 40 | 49 | 49 | 49.6 |
| 15 | Suits · Grove | 53 | 47 | 47 | 47 | 47 | 47 | 47 | 46 | 44 | 43 | **38** | 42 | 51 | 46 | — | 44 | 52 | **40** | 47 | 42 | **36** | 44 | 44 | 45.1 |
| 16 | Standard · Forge | 59 | 53 | 53 | 53 | 53 | 53 | 53 | 52 | 49 | 49 | 43 | 47 | 57 | 52 | 56 | — | 59 | 45 | 53 | 48 | 40 | 50 | 50 | 51.3 |
| 17 | Standard · Loom | 50 | 44 | 44 | 44 | 44 | 44 | 44 | 43 | 40 | 40 | **34** | **38** | 49 | 43 | 48 | 41 | — | **36** | 44 | **38** | **32** | 41 | 41 | 41.8 |
| 18 | Two Mains · Forge | 64 | 58 | 57 | 57 | 57 | 57 | 57 | 57 | 54 | 54 | 48 | 52 | 61 | 56 | 60 | 55 | 64 | — | 58 | 53 | 45 | 55 | 55 | 56.1 |
| 19 | Two Mains · Loom | 56 | 50 | 49 | 50 | 49 | 50 | 49 | 49 | 46 | 46 | **40** | 44 | 54 | 49 | 53 | 47 | 56 | 42 | — | 44 | **38** | 47 | 47 | 47.9 |
| 20 | Spellbook · Forge | 62 | 55 | 55 | 55 | 55 | 55 | 55 | 54 | 52 | 52 | 45 | 50 | 59 | 53 | 58 | 52 | 62 | 47 | 56 | — | 43 | 53 | 53 | 53.6 |
| 21 | Spellbook · Loom | 68 | 62 | 62 | 62 | 62 | 62 | 62 | 61 | 59 | 59 | 52 | 57 | 65 | 60 | 64 | 60 | 68 | 55 | 62 | 57 | — | 60 | 60 | 60.8 |
| 22 | Ladder Pair · Forge | 59 | 53 | 53 | 53 | 53 | 53 | 53 | 52 | 49 | 49 | 43 | 47 | 57 | 51 | 56 | 50 | 59 | 45 | 53 | 47 | 40 | — | 50 | 51.1 |
| 23 | Ladder Pair · Loom | 59 | 53 | 53 | 53 | 53 | 53 | 53 | 52 | 49 | 49 | 43 | 47 | 57 | 51 | 56 | 50 | 59 | 45 | 53 | 47 | 40 | 50 | — | 51.1 |

**Priced on damage through the defenses, no defense** — row board's score against the column board, in %:

| # | Board | 1 | 2 | 3 | 4 | 5 | 6 | 7 | 8 | 9 | 10 | 11 | 12 | 13 | 14 | 15 | 16 | 17 | 18 | 19 | 20 | 21 | 22 | 23 | Mean |
|---|---|---|---|---|---|---|---|---|---|---|---|---|---|---|---|---|---|---|---|---|---|---|---|---|---|
| 1 | Lean · Standard D6 | — | 50 | 50 | 50 | 50 | 50 | 50 | 50 | 48 | 48 | 52 | 51 | 50 | 51 | 50 | 51 | 51 | 50 | 53 | 51 | 52 | 48 | 48 | 50.2 |
| 2 | Lean · Hearth | 50 | — | 50 | 50 | 50 | 50 | 50 | 50 | 48 | 48 | 52 | 51 | 50 | 51 | 50 | 51 | 51 | 50 | 53 | 51 | 52 | 48 | 48 | 50.2 |
| 3 | Standard · Standard D6 | 50 | 50 | — | 50 | 50 | 50 | 50 | 50 | 48 | 48 | 52 | 51 | 50 | 51 | 50 | 50 | 51 | 50 | 53 | 51 | 51 | 48 | 48 | 50.1 |
| 4 | Standard · Hearth | 50 | 50 | 50 | — | 50 | 50 | 50 | 50 | 48 | 48 | 52 | 51 | 50 | 51 | 49 | 50 | 51 | 50 | 53 | 51 | 51 | 48 | 48 | 49.9 |
| 5 | Compact · Standard D6 | 50 | 50 | 50 | 50 | — | 50 | 50 | 50 | 48 | 48 | 52 | 51 | 50 | 51 | 50 | 50 | 51 | 50 | 53 | 51 | 51 | 48 | 48 | 50.1 |
| 6 | Compact · Hearth | 50 | 50 | 50 | 50 | 50 | — | 50 | 50 | 48 | 48 | 52 | 51 | 50 | 51 | 49 | 50 | 51 | 50 | 53 | 51 | 51 | 48 | 48 | 49.9 |
| 7 | Wide · Standard D6 | 50 | 50 | 50 | 50 | 50 | 50 | — | 50 | 48 | 48 | 52 | 51 | 50 | 51 | 50 | 50 | 51 | 50 | 53 | 51 | 51 | 48 | 48 | 50.1 |
| 8 | Wide · Hearth | 50 | 50 | 50 | 50 | 50 | 50 | 50 | — | 48 | 48 | 52 | 51 | 50 | 51 | 50 | 51 | 51 | 50 | 53 | 51 | 52 | 48 | 48 | 50.4 |
| 9 | Double Ladder · Standard D6 | 52 | 52 | 52 | 52 | 52 | 52 | 52 | 52 | — | 50 | 54 | 53 | 52 | 53 | 52 | 53 | 53 | 52 | 55 | 53 | 54 | 50 | 50 | 52.3 |
| 10 | Double Ladder · Hearth | 52 | 52 | 52 | 52 | 52 | 52 | 52 | 52 | 50 | — | 54 | 53 | 52 | 53 | 52 | 53 | 53 | 52 | 55 | 53 | 53 | 50 | 50 | 52.2 |
| 11 | Combo · Standard D6 | 48 | 48 | 48 | 48 | 48 | 48 | 48 | 48 | 46 | 46 | — | 49 | 48 | 49 | 48 | 48 | 49 | 48 | 51 | 49 | 50 | 46 | 46 | 48.0 |
| 12 | Combo · Hearth | 49 | 49 | 49 | 49 | 49 | 49 | 49 | 49 | 47 | 47 | 51 | — | 49 | 50 | 49 | 50 | 50 | 49 | 52 | 50 | 51 | 47 | 47 | 49.2 |
| 13 | Rainbow · Grove | 50 | 50 | 50 | 50 | 50 | 50 | 50 | 50 | 48 | 48 | 52 | 51 | — | 51 | 49 | 50 | 51 | 50 | 53 | 51 | 51 | 48 | 48 | 49.9 |
| 14 | Wide Rainbow · Grove | 49 | 49 | 49 | 49 | 49 | 49 | 49 | 49 | 47 | 47 | 51 | 50 | 49 | — | 48 | 49 | 50 | 49 | 52 | 50 | 50 | 47 | 47 | 48.9 |
| 15 | Suits · Grove | 50 | 50 | 50 | 51 | 50 | 51 | 50 | 50 | 48 | 48 | 52 | 51 | 51 | 52 | — | 51 | 51 | 50 | 53 | 51 | 52 | 48 | 48 | 50.5 |
| 16 | Standard · Forge | 49 | 49 | 50 | 50 | 50 | 50 | 50 | 49 | 47 | 47 | 52 | 50 | 50 | 51 | 49 | — | 50 | 49 | 52 | 51 | 51 | 48 | 47 | 49.6 |
| 17 | Standard · Loom | 49 | 49 | 49 | 49 | 49 | 49 | 49 | 49 | 47 | 47 | 51 | 50 | 49 | 50 | 49 | 50 | — | 49 | 52 | 50 | 51 | 47 | 47 | 49.2 |
| 18 | Two Mains · Forge | 50 | 50 | 50 | 50 | 50 | 50 | 50 | 50 | 48 | 48 | 52 | 51 | 50 | 51 | 50 | 51 | 51 | — | 53 | 51 | 51 | 48 | 48 | 50.2 |
| 19 | Two Mains · Loom | 47 | 47 | 47 | 47 | 47 | 47 | 47 | 47 | 45 | 45 | 49 | 48 | 47 | 48 | 47 | 48 | 48 | 47 | — | 48 | 49 | 45 | 45 | 47.2 |
| 20 | Spellbook · Forge | 49 | 49 | 49 | 49 | 49 | 49 | 49 | 49 | 47 | 47 | 51 | 50 | 49 | 50 | 49 | 49 | 50 | 49 | 52 | — | 51 | 47 | 47 | 49.1 |
| 21 | Spellbook · Loom | 48 | 48 | 49 | 49 | 49 | 49 | 49 | 48 | 46 | 47 | 50 | 49 | 49 | 50 | 48 | 49 | 49 | 49 | 51 | 49 | — | 47 | 47 | 48.5 |
| 22 | Ladder Pair · Forge | 52 | 52 | 52 | 52 | 52 | 52 | 52 | 52 | 50 | 50 | 54 | 53 | 52 | 53 | 52 | 52 | 53 | 52 | 55 | 53 | 53 | — | 50 | 52.1 |
| 23 | Ladder Pair · Loom | 52 | 52 | 52 | 52 | 52 | 52 | 52 | 52 | 50 | 50 | 54 | 53 | 52 | 53 | 52 | 53 | 53 | 52 | 55 | 53 | 53 | 50 | — | 52.2 |

**Priced on damage through the defenses, Light Guard** — row board's score against the column board, in %:

| # | Board | 1 | 2 | 3 | 4 | 5 | 6 | 7 | 8 | 9 | 10 | 11 | 12 | 13 | 14 | 15 | 16 | 17 | 18 | 19 | 20 | 21 | 22 | 23 | Mean |
|---|---|---|---|---|---|---|---|---|---|---|---|---|---|---|---|---|---|---|---|---|---|---|---|---|---|
| 1 | Lean · Standard D6 | — | 50 | 50 | 50 | 50 | 50 | 50 | 50 | 47 | 48 | 52 | 50 | 51 | 51 | 50 | 51 | 51 | 48 | 52 | 49 | 50 | 48 | 48 | 49.8 |
| 2 | Lean · Hearth | 50 | — | 50 | 50 | 50 | 50 | 50 | 50 | 48 | 48 | 52 | 50 | 51 | 52 | 50 | 51 | 51 | 48 | 52 | 50 | 50 | 48 | 48 | 49.9 |
| 3 | Standard · Standard D6 | 50 | 50 | — | 50 | 50 | 50 | 50 | 50 | 48 | 48 | 52 | 50 | 51 | 51 | 50 | 51 | 51 | 48 | 52 | 50 | 50 | 48 | 48 | 49.8 |
| 4 | Standard · Hearth | 50 | 50 | 50 | — | 50 | 50 | 50 | 50 | 47 | 48 | 52 | 50 | 50 | 51 | 50 | 50 | 51 | 48 | 52 | 49 | 50 | 48 | 48 | 49.7 |
| 5 | Compact · Standard D6 | 50 | 50 | 50 | 50 | — | 50 | 50 | 50 | 48 | 48 | 52 | 50 | 51 | 51 | 50 | 51 | 51 | 48 | 52 | 50 | 50 | 48 | 48 | 49.8 |
| 6 | Compact · Hearth | 50 | 50 | 50 | 50 | 50 | — | 50 | 50 | 47 | 48 | 52 | 50 | 50 | 51 | 50 | 50 | 51 | 48 | 52 | 49 | 50 | 48 | 48 | 49.7 |
| 7 | Wide · Standard D6 | 50 | 50 | 50 | 50 | 50 | 50 | — | 50 | 48 | 48 | 52 | 50 | 51 | 51 | 50 | 51 | 51 | 48 | 52 | 50 | 50 | 48 | 48 | 49.8 |
| 8 | Wide · Hearth | 50 | 50 | 50 | 50 | 50 | 50 | 50 | — | 48 | 48 | 52 | 50 | 51 | 52 | 50 | 51 | 51 | 49 | 53 | 50 | 50 | 48 | 48 | 50.1 |
| 9 | Double Ladder · Standard D6 | 53 | 52 | 52 | 53 | 52 | 53 | 52 | 52 | — | 50 | 54 | 52 | 53 | 54 | 52 | 53 | 53 | 50 | 55 | 52 | 52 | 50 | 50 | 52.3 |
| 10 | Double Ladder · Hearth | 52 | 52 | 52 | 52 | 52 | 52 | 52 | 52 | 50 | — | 54 | 52 | 53 | 54 | 52 | 53 | 53 | 50 | 54 | 52 | 52 | 50 | 50 | 52.2 |
| 11 | Combo · Standard D6 | 48 | 48 | 48 | 48 | 48 | 48 | 48 | 48 | 46 | 46 | — | 49 | 49 | 50 | 48 | 49 | 49 | 47 | 51 | 48 | 49 | 46 | 46 | 48.1 |
| 12 | Combo · Hearth | 50 | 50 | 50 | 50 | 50 | 50 | 50 | 50 | 48 | 48 | 51 | — | 50 | 51 | 50 | 50 | 51 | 48 | 52 | 49 | 50 | 48 | 48 | 49.6 |
| 13 | Rainbow · Grove | 49 | 49 | 49 | 50 | 49 | 50 | 49 | 49 | 47 | 47 | 51 | 50 | — | 51 | 49 | 50 | 50 | 48 | 52 | 49 | 50 | 47 | 47 | 49.3 |
| 14 | Wide Rainbow · Grove | 49 | 48 | 49 | 49 | 49 | 49 | 49 | 48 | 46 | 46 | 50 | 49 | 49 | — | 48 | 49 | 49 | 47 | 51 | 48 | 49 | 46 | 46 | 48.4 |
| 15 | Suits · Grove | 50 | 50 | 50 | 50 | 50 | 50 | 50 | 50 | 48 | 48 | 52 | 50 | 51 | 52 | — | 51 | 51 | 48 | 52 | 50 | 50 | 48 | 48 | 50.0 |
| 16 | Standard · Forge | 49 | 49 | 49 | 50 | 49 | 50 | 49 | 49 | 47 | 47 | 51 | 50 | 50 | 51 | 49 | — | 50 | 48 | 52 | 49 | 50 | 47 | 47 | 49.2 |
| 17 | Standard · Loom | 49 | 49 | 49 | 49 | 49 | 49 | 49 | 49 | 47 | 47 | 51 | 49 | 50 | 51 | 49 | 50 | — | 48 | 51 | 49 | 49 | 47 | 47 | 49.0 |
| 18 | Two Mains · Forge | 52 | 52 | 52 | 52 | 52 | 52 | 52 | 51 | 50 | 50 | 53 | 52 | 52 | 53 | 52 | 52 | 52 | — | 54 | 51 | 52 | 50 | 50 | 51.6 |
| 19 | Two Mains · Loom | 48 | 48 | 48 | 48 | 48 | 48 | 48 | 47 | 45 | 46 | 49 | 48 | 48 | 49 | 48 | 48 | 49 | 46 | — | 47 | 48 | 46 | 46 | 47.5 |
| 20 | Spellbook · Forge | 51 | 50 | 50 | 51 | 50 | 51 | 50 | 50 | 48 | 48 | 52 | 51 | 51 | 52 | 50 | 51 | 51 | 49 | 53 | — | 51 | 48 | 48 | 50.3 |
| 21 | Spellbook · Loom | 50 | 50 | 50 | 50 | 50 | 50 | 50 | 50 | 48 | 48 | 51 | 50 | 50 | 51 | 50 | 50 | 51 | 48 | 52 | 49 | — | 48 | 48 | 49.7 |
| 22 | Ladder Pair · Forge | 52 | 52 | 52 | 52 | 52 | 52 | 52 | 52 | 50 | 50 | 54 | 52 | 53 | 54 | 52 | 53 | 53 | 50 | 54 | 52 | 52 | — | 50 | 52.1 |
| 23 | Ladder Pair · Loom | 52 | 52 | 52 | 52 | 52 | 52 | 52 | 52 | 50 | 50 | 54 | 52 | 53 | 54 | 52 | 53 | 53 | 50 | 54 | 52 | 52 | 50 | — | 52.1 |

**Priced on damage through the defenses, Holdfast** — row board's score against the column board, in %:

| # | Board | 1 | 2 | 3 | 4 | 5 | 6 | 7 | 8 | 9 | 10 | 11 | 12 | 13 | 14 | 15 | 16 | 17 | 18 | 19 | 20 | 21 | 22 | 23 | Mean |
|---|---|---|---|---|---|---|---|---|---|---|---|---|---|---|---|---|---|---|---|---|---|---|---|---|---|
| 1 | Lean · Standard D6 | — | 50 | 49 | 50 | 49 | 50 | 49 | 49 | 46 | 47 | 49 | 46 | 54 | 54 | 53 | 51 | 50 | 42 | 49 | 44 | 45 | 47 | 47 | 48.6 |
| 2 | Lean · Hearth | 50 | — | 50 | 50 | 50 | 50 | 50 | 50 | 47 | 47 | 50 | 47 | 54 | 54 | 53 | 51 | 51 | 42 | 50 | 45 | 45 | 47 | 47 | 49.0 |
| 3 | Standard · Standard D6 | 51 | 50 | — | 50 | 50 | 50 | 50 | 50 | 47 | 47 | 50 | 47 | 54 | 54 | 53 | 51 | 51 | 43 | 50 | 45 | 45 | 47 | 47 | 49.2 |
| 4 | Standard · Hearth | 50 | 50 | 50 | — | 50 | 50 | 50 | 50 | 47 | 47 | 50 | 47 | 54 | 54 | 53 | 51 | 51 | 43 | 50 | 45 | 45 | 47 | 47 | 49.1 |
| 5 | Compact · Standard D6 | 51 | 50 | 50 | 50 | — | 50 | 50 | 50 | 47 | 47 | 50 | 47 | 54 | 54 | 53 | 51 | 51 | 43 | 50 | 45 | 45 | 47 | 47 | 49.2 |
| 6 | Compact · Hearth | 50 | 50 | 50 | 50 | 50 | — | 50 | 50 | 47 | 47 | 50 | 47 | 54 | 54 | 53 | 51 | 51 | 43 | 50 | 45 | 45 | 47 | 47 | 49.1 |
| 7 | Wide · Standard D6 | 51 | 50 | 50 | 50 | 50 | 50 | — | 50 | 47 | 47 | 50 | 47 | 54 | 54 | 53 | 51 | 51 | 43 | 50 | 45 | 45 | 47 | 47 | 49.2 |
| 8 | Wide · Hearth | 51 | 50 | 50 | 50 | 50 | 50 | 50 | — | 47 | 47 | 50 | 47 | 54 | 54 | 53 | 51 | 51 | 43 | 50 | 45 | 45 | 48 | 48 | 49.3 |
| 9 | Double Ladder · Standard D6 | 54 | 53 | 53 | 53 | 53 | 53 | 53 | 53 | — | 50 | 53 | 50 | 56 | 57 | 56 | 54 | 54 | 45 | 53 | 48 | 48 | 50 | 50 | 52.1 |
| 10 | Double Ladder · Hearth | 53 | 53 | 53 | 53 | 53 | 53 | 53 | 53 | 50 | — | 53 | 50 | 56 | 57 | 55 | 54 | 54 | 45 | 53 | 48 | 48 | 50 | 50 | 52.1 |
| 11 | Combo · Standard D6 | 51 | 50 | 50 | 50 | 50 | 50 | 50 | 50 | 47 | 47 | — | 47 | 54 | 55 | 53 | 51 | 51 | 43 | 50 | 45 | 45 | 48 | 48 | 49.5 |
| 12 | Combo · Hearth | 54 | 53 | 53 | 53 | 53 | 53 | 53 | 53 | 50 | 50 | 53 | — | 57 | 57 | 56 | 54 | 54 | 46 | 53 | 48 | 49 | 51 | 51 | 52.5 |
| 13 | Rainbow · Grove | 46 | 46 | 46 | 46 | 46 | 46 | 46 | 46 | 44 | 44 | 46 | 43 | — | 51 | 49 | 47 | 47 | **40** | 46 | 42 | 42 | 44 | 44 | 45.3 |
| 14 | Wide Rainbow · Grove | 46 | 46 | 46 | 46 | 46 | 46 | 46 | 46 | 43 | 43 | 45 | 43 | 49 | — | 49 | 46 | 46 | **39** | 45 | 42 | 42 | 43 | 43 | 44.8 |
| 15 | Suits · Grove | 47 | 47 | 47 | 47 | 47 | 47 | 47 | 47 | 44 | 45 | 47 | 44 | 51 | 51 | — | 48 | 48 | 41 | 47 | 43 | 43 | 45 | 45 | 46.3 |
| 16 | Standard · Forge | 49 | 49 | 49 | 49 | 49 | 49 | 49 | 49 | 46 | 46 | 49 | 46 | 53 | 54 | 52 | — | 50 | 41 | 49 | 44 | 44 | 46 | 46 | 48.1 |
| 17 | Standard · Loom | 50 | 49 | 49 | 49 | 49 | 49 | 49 | 49 | 46 | 46 | 49 | 46 | 53 | 54 | 52 | 50 | — | 42 | 49 | 44 | 44 | 47 | 47 | 48.3 |
| 18 | Two Mains · Forge | 58 | 58 | 57 | 57 | 57 | 57 | 57 | 57 | 55 | 55 | 57 | 54 | 60 | 61 | 59 | 59 | 58 | — | 57 | 53 | 53 | 55 | 55 | 56.9 |
| 19 | Two Mains · Loom | 51 | 50 | 50 | 50 | 50 | 50 | 50 | 50 | 47 | 47 | 50 | 47 | 54 | 55 | 53 | 51 | 51 | 43 | — | 45 | 46 | 48 | 48 | 49.5 |
| 20 | Spellbook · Forge | 56 | 55 | 55 | 55 | 55 | 55 | 55 | 55 | 52 | 52 | 55 | 52 | 58 | 58 | 57 | 56 | 56 | 47 | 55 | — | 50 | 53 | 53 | 54.4 |
| 21 | Spellbook · Loom | 55 | 55 | 55 | 55 | 55 | 55 | 55 | 55 | 52 | 52 | 55 | 51 | 58 | 58 | 57 | 56 | 56 | 47 | 54 | 50 | — | 52 | 52 | 54.0 |
| 22 | Ladder Pair · Forge | 53 | 53 | 53 | 53 | 53 | 53 | 53 | 52 | 50 | 50 | 52 | 49 | 56 | 57 | 55 | 54 | 53 | 45 | 52 | 47 | 48 | — | 50 | 51.8 |
| 23 | Ladder Pair · Loom | 53 | 53 | 53 | 53 | 53 | 53 | 53 | 52 | 50 | 50 | 52 | 49 | 56 | 57 | 55 | 54 | 53 | 45 | 52 | 47 | 48 | 50 | — | 51.8 |

**Tuned, no defense** — row board's score against the column board, in %:

| # | Board | 1 | 2 | 3 | 4 | 5 | 6 | 7 | 8 | 9 | 10 | 11 | 12 | 13 | 14 | 15 | 16 | 17 | 18 | 19 | 20 | 21 | 22 | 23 | Mean |
|---|---|---|---|---|---|---|---|---|---|---|---|---|---|---|---|---|---|---|---|---|---|---|---|---|---|
| 1 | Lean · Standard D6 | — | 50 | 50 | 50 | 50 | 50 | 49 | 50 | 48 | 48 | 52 | 51 | 48 | 49 | 48 | 49 | 51 | 50 | 53 | 51 | 52 | 48 | 48 | 49.7 |
| 2 | Lean · Hearth | 50 | — | 50 | 51 | 50 | 51 | 50 | 50 | 48 | 48 | 52 | 51 | 48 | 49 | 49 | 49 | 51 | 50 | 53 | 51 | 52 | 48 | 48 | 50.1 |
| 3 | Standard · Standard D6 | 50 | 50 | — | 50 | 50 | 50 | 49 | 50 | 48 | 48 | 52 | 51 | 48 | 48 | 48 | 48 | 51 | 50 | 53 | 51 | 51 | 48 | 48 | 49.6 |
| 4 | Standard · Hearth | 50 | 49 | 50 | — | 50 | 50 | 49 | 49 | 48 | 48 | 52 | 51 | 48 | 48 | 48 | 48 | 51 | 50 | 53 | 51 | 51 | 48 | 48 | 49.5 |
| 5 | Compact · Standard D6 | 50 | 50 | 50 | 50 | — | 50 | 49 | 50 | 48 | 48 | 52 | 51 | 48 | 48 | 48 | 48 | 51 | 50 | 53 | 51 | 51 | 48 | 48 | 49.6 |
| 6 | Compact · Hearth | 50 | 49 | 50 | 50 | 50 | — | 49 | 49 | 48 | 48 | 52 | 51 | 48 | 48 | 48 | 48 | 51 | 50 | 53 | 51 | 51 | 48 | 48 | 49.5 |
| 7 | Wide · Standard D6 | 51 | 50 | 51 | 51 | 51 | 51 | — | 50 | 49 | 49 | 53 | 52 | 49 | 49 | 49 | 49 | 52 | 51 | 53 | 52 | 52 | 49 | 49 | 50.4 |
| 8 | Wide · Hearth | 50 | 50 | 50 | 51 | 50 | 51 | 50 | — | 48 | 48 | 52 | 51 | 48 | 49 | 49 | 49 | 51 | 50 | 53 | 51 | 52 | 49 | 48 | 50.1 |
| 9 | Double Ladder · Standard D6 | 52 | 52 | 52 | 52 | 52 | 52 | 51 | 52 | — | 50 | 54 | 53 | 50 | 51 | 51 | 51 | 53 | 52 | 55 | 53 | 54 | 50 | 50 | 51.9 |
| 10 | Double Ladder · Hearth | 52 | 52 | 52 | 52 | 52 | 52 | 51 | 52 | 50 | — | 54 | 53 | 50 | 51 | 51 | 50 | 53 | 52 | 55 | 53 | 53 | 50 | 50 | 51.8 |
| 11 | Combo · Standard D6 | 48 | 48 | 48 | 48 | 48 | 48 | 47 | 48 | 46 | 46 | — | 49 | 46 | 46 | 46 | 46 | 49 | 48 | 51 | 49 | 50 | 46 | 46 | 47.6 |
| 12 | Combo · Hearth | 49 | 49 | 49 | 49 | 49 | 49 | 48 | 49 | 47 | 47 | 51 | — | 47 | 48 | 48 | 48 | 50 | 49 | 52 | 50 | 51 | 47 | 47 | 48.8 |
| 13 | Rainbow · Grove | 52 | 52 | 52 | 52 | 52 | 52 | 51 | 52 | 50 | 50 | 54 | 53 | — | 51 | 51 | 51 | 53 | 52 | 55 | 53 | 54 | 50 | 50 | 51.9 |
| 14 | Wide Rainbow · Grove | 51 | 51 | 52 | 52 | 52 | 52 | 51 | 51 | 49 | 49 | 54 | 52 | 49 | — | 50 | 50 | 52 | 51 | 54 | 52 | 53 | 50 | 49 | 51.2 |
| 15 | Suits · Grove | 52 | 51 | 52 | 52 | 52 | 52 | 51 | 51 | 49 | 49 | 54 | 52 | 49 | 50 | — | 50 | 52 | 51 | 54 | 53 | 53 | 50 | 50 | 51.3 |
| 16 | Standard · Forge | 51 | 51 | 52 | 52 | 52 | 52 | 51 | 51 | 49 | 50 | 54 | 52 | 49 | 50 | 50 | — | 52 | 51 | 54 | 52 | 53 | 50 | 50 | 51.3 |
| 17 | Standard · Loom | 49 | 49 | 49 | 49 | 49 | 49 | 48 | 49 | 47 | 47 | 51 | 50 | 47 | 48 | 48 | 48 | — | 49 | 52 | 50 | 51 | 47 | 47 | 48.7 |
| 18 | Two Mains · Forge | 50 | 50 | 50 | 50 | 50 | 50 | 49 | 50 | 48 | 48 | 52 | 51 | 48 | 49 | 49 | 49 | 51 | — | 53 | 51 | 51 | 48 | 48 | 49.8 |
| 19 | Two Mains · Loom | 47 | 47 | 47 | 47 | 47 | 47 | 47 | 47 | 45 | 45 | 49 | 48 | 45 | 46 | 46 | 46 | 48 | 47 | — | 48 | 49 | 45 | 45 | 46.8 |
| 20 | Spellbook · Forge | 49 | 49 | 49 | 49 | 49 | 49 | 48 | 49 | 47 | 47 | 51 | 50 | 47 | 48 | 47 | 48 | 50 | 49 | 52 | — | 51 | 47 | 47 | 48.7 |
| 21 | Spellbook · Loom | 48 | 48 | 49 | 49 | 49 | 49 | 48 | 48 | 46 | 47 | 50 | 49 | 46 | 47 | 47 | 47 | 49 | 49 | 51 | 49 | — | 47 | 47 | 48.1 |
| 22 | Ladder Pair · Forge | 52 | 52 | 52 | 52 | 52 | 52 | 51 | 51 | 50 | 50 | 54 | 53 | 50 | 50 | 50 | 50 | 53 | 52 | 55 | 53 | 53 | — | 50 | 51.7 |
| 23 | Ladder Pair · Loom | 52 | 52 | 52 | 52 | 52 | 52 | 51 | 52 | 50 | 50 | 54 | 53 | 50 | 51 | 50 | 50 | 53 | 52 | 55 | 53 | 53 | 50 | — | 51.8 |

**Tuned, Light Guard** — row board's score against the column board, in %:

| # | Board | 1 | 2 | 3 | 4 | 5 | 6 | 7 | 8 | 9 | 10 | 11 | 12 | 13 | 14 | 15 | 16 | 17 | 18 | 19 | 20 | 21 | 22 | 23 | Mean |
|---|---|---|---|---|---|---|---|---|---|---|---|---|---|---|---|---|---|---|---|---|---|---|---|---|---|
| 1 | Lean · Standard D6 | — | 50 | 50 | 50 | 50 | 50 | 49 | 49 | 47 | 48 | 52 | 50 | 47 | 48 | 48 | 48 | 51 | 48 | 52 | 49 | 50 | 48 | 48 | 49.2 |
| 2 | Lean · Hearth | 50 | — | 50 | 51 | 50 | 51 | 50 | 50 | 48 | 48 | 52 | 51 | 48 | 49 | 49 | 49 | 51 | 49 | 53 | 50 | 51 | 48 | 48 | 49.8 |
| 3 | Standard · Standard D6 | 50 | 50 | — | 50 | 50 | 50 | 49 | 49 | 48 | 48 | 52 | 50 | 47 | 48 | 48 | 48 | 51 | 48 | 52 | 50 | 50 | 48 | 48 | 49.3 |
| 4 | Standard · Hearth | 50 | 49 | 50 | — | 50 | 50 | 49 | 49 | 47 | 48 | 52 | 50 | 47 | 48 | 48 | 48 | 51 | 48 | 52 | 49 | 50 | 48 | 48 | 49.2 |
| 5 | Compact · Standard D6 | 50 | 50 | 50 | 50 | — | 50 | 49 | 49 | 48 | 48 | 52 | 50 | 47 | 48 | 48 | 48 | 51 | 48 | 52 | 50 | 50 | 48 | 48 | 49.3 |
| 6 | Compact · Hearth | 50 | 49 | 50 | 50 | 50 | — | 49 | 49 | 47 | 48 | 52 | 50 | 47 | 48 | 48 | 48 | 51 | 48 | 52 | 49 | 50 | 48 | 48 | 49.2 |
| 7 | Wide · Standard D6 | 51 | 50 | 51 | 51 | 51 | 51 | — | 50 | 48 | 48 | 52 | 51 | 48 | 49 | 49 | 49 | 52 | 49 | 53 | 50 | 51 | 49 | 49 | 50.2 |
| 8 | Wide · Hearth | 51 | 50 | 51 | 51 | 51 | 51 | 50 | — | 48 | 48 | 52 | 51 | 48 | 49 | 49 | 49 | 51 | 49 | 53 | 50 | 51 | 48 | 48 | 49.9 |
| 9 | Double Ladder · Standard D6 | 53 | 52 | 52 | 53 | 52 | 53 | 52 | 52 | — | 50 | 54 | 52 | 50 | 51 | 51 | 50 | 53 | 50 | 55 | 52 | 52 | 50 | 50 | 51.8 |
| 10 | Double Ladder · Hearth | 52 | 52 | 52 | 52 | 52 | 52 | 52 | 52 | 50 | — | 54 | 52 | 50 | 51 | 51 | 50 | 53 | 50 | 54 | 52 | 52 | 50 | 50 | 51.7 |
| 11 | Combo · Standard D6 | 48 | 48 | 48 | 48 | 48 | 48 | 48 | 48 | 46 | 46 | — | 49 | 46 | 47 | 47 | 46 | 49 | 47 | 51 | 48 | 49 | 46 | 46 | 47.6 |
| 12 | Combo · Hearth | 50 | 49 | 50 | 50 | 50 | 50 | 49 | 49 | 48 | 48 | 51 | — | 47 | 48 | 48 | 48 | 51 | 48 | 52 | 49 | 50 | 48 | 48 | 49.2 |
| 13 | Rainbow · Grove | 53 | 52 | 53 | 53 | 53 | 53 | 52 | 52 | 50 | 50 | 54 | 53 | — | 51 | 51 | 51 | 53 | 51 | 55 | 52 | 53 | 50 | 50 | 51.9 |
| 14 | Wide Rainbow · Grove | 52 | 51 | 52 | 52 | 52 | 52 | 51 | 51 | 49 | 49 | 53 | 52 | 49 | — | 50 | 50 | 52 | 50 | 54 | 51 | 52 | 49 | 49 | 51.0 |
| 15 | Suits · Grove | 52 | 51 | 52 | 52 | 52 | 52 | 51 | 51 | 49 | 49 | 53 | 52 | 49 | 50 | — | 50 | 52 | 50 | 54 | 51 | 52 | 49 | 49 | 50.9 |
| 16 | Standard · Forge | 52 | 51 | 52 | 52 | 52 | 52 | 51 | 51 | 50 | 50 | 54 | 52 | 49 | 50 | 50 | — | 53 | 50 | 54 | 51 | 52 | 50 | 50 | 51.3 |
| 17 | Standard · Loom | 49 | 49 | 49 | 49 | 49 | 49 | 48 | 49 | 47 | 47 | 51 | 49 | 47 | 48 | 48 | 47 | — | 48 | 51 | 49 | 49 | 47 | 47 | 48.4 |
| 18 | Two Mains · Forge | 52 | 51 | 52 | 52 | 52 | 52 | 51 | 51 | 50 | 50 | 53 | 52 | 49 | 50 | 50 | 50 | 52 | — | 54 | 51 | 52 | 50 | 50 | 51.1 |
| 19 | Two Mains · Loom | 48 | 47 | 48 | 48 | 48 | 48 | 47 | 47 | 45 | 46 | 49 | 48 | 45 | 46 | 46 | 46 | 49 | 46 | — | 47 | 48 | 46 | 46 | 47.0 |
| 20 | Spellbook · Forge | 51 | 50 | 50 | 51 | 50 | 51 | 50 | 50 | 48 | 48 | 52 | 51 | 48 | 49 | 49 | 49 | 51 | 49 | 53 | — | 51 | 48 | 48 | 49.8 |
| 21 | Spellbook · Loom | 50 | 49 | 50 | 50 | 50 | 50 | 49 | 49 | 48 | 48 | 51 | 50 | 47 | 48 | 48 | 48 | 51 | 48 | 52 | 49 | — | 48 | 48 | 49.2 |
| 22 | Ladder Pair · Forge | 52 | 52 | 52 | 52 | 52 | 52 | 51 | 52 | 50 | 50 | 54 | 52 | 50 | 51 | 51 | 50 | 53 | 50 | 54 | 52 | 52 | — | 50 | 51.5 |
| 23 | Ladder Pair · Loom | 52 | 52 | 52 | 52 | 52 | 52 | 51 | 52 | 50 | 50 | 54 | 52 | 50 | 51 | 51 | 50 | 53 | 50 | 54 | 52 | 52 | 50 | — | 51.6 |

**Tuned, Holdfast** — row board's score against the column board, in %:

| # | Board | 1 | 2 | 3 | 4 | 5 | 6 | 7 | 8 | 9 | 10 | 11 | 12 | 13 | 14 | 15 | 16 | 17 | 18 | 19 | 20 | 21 | 22 | 23 | Mean |
|---|---|---|---|---|---|---|---|---|---|---|---|---|---|---|---|---|---|---|---|---|---|---|---|---|---|
| 1 | Lean · Standard D6 | — | 49 | 49 | 50 | 49 | 50 | 48 | 49 | 46 | 47 | 49 | 46 | 49 | 50 | 51 | 47 | 50 | 42 | 49 | 44 | 45 | 47 | 47 | 47.8 |
| 2 | Lean · Hearth | 51 | — | 51 | 51 | 51 | 51 | 49 | 50 | 48 | 48 | 50 | 47 | 50 | 51 | 52 | 48 | 51 | 43 | 50 | 45 | 46 | 48 | 48 | 48.9 |
| 3 | Standard · Standard D6 | 51 | 49 | — | 50 | 50 | 50 | 49 | 49 | 47 | 47 | 50 | 47 | 49 | 50 | 51 | 47 | 51 | 43 | 50 | 45 | 45 | 47 | 47 | 48.4 |
| 4 | Standard · Hearth | 50 | 49 | 50 | — | 50 | 50 | 49 | 49 | 47 | 47 | 50 | 47 | 49 | 50 | 51 | 47 | 51 | 43 | 50 | 45 | 45 | 47 | 47 | 48.3 |
| 5 | Compact · Standard D6 | 51 | 49 | 50 | 50 | — | 50 | 49 | 49 | 47 | 47 | 50 | 47 | 49 | 50 | 51 | 47 | 51 | 43 | 50 | 45 | 45 | 47 | 47 | 48.4 |
| 6 | Compact · Hearth | 50 | 49 | 50 | 50 | 50 | — | 49 | 49 | 47 | 47 | 50 | 47 | 49 | 50 | 51 | 47 | 51 | 43 | 50 | 45 | 45 | 47 | 47 | 48.3 |
| 7 | Wide · Standard D6 | 52 | 51 | 51 | 51 | 51 | 51 | — | 50 | 48 | 48 | 51 | 48 | 50 | 51 | 52 | 48 | 52 | 44 | 51 | 46 | 46 | 49 | 48 | 49.5 |
| 8 | Wide · Hearth | 51 | 50 | 51 | 51 | 51 | 51 | 50 | — | 48 | 48 | 51 | 48 | 50 | 51 | 52 | 48 | 52 | 43 | 51 | 46 | 46 | 48 | 48 | 49.3 |
| 9 | Double Ladder · Standard D6 | 54 | 52 | 53 | 53 | 53 | 53 | 52 | 52 | — | 50 | 53 | 49 | 52 | 53 | 54 | 50 | 54 | 45 | 53 | 48 | 48 | 50 | 50 | 51.4 |
| 10 | Double Ladder · Hearth | 53 | 52 | 53 | 53 | 53 | 53 | 52 | 52 | 50 | — | 53 | 49 | 52 | 53 | 54 | 50 | 54 | 45 | 53 | 48 | 48 | 50 | 50 | 51.3 |
| 11 | Combo · Standard D6 | 51 | 50 | 50 | 50 | 50 | 50 | 49 | 49 | 47 | 47 | — | 47 | 49 | 51 | 52 | 48 | 51 | 43 | 50 | 45 | 45 | 48 | 48 | 48.7 |
| 12 | Combo · Hearth | 54 | 53 | 53 | 53 | 53 | 53 | 52 | 52 | 51 | 51 | 53 | — | 52 | 54 | 54 | 51 | 54 | 46 | 53 | 48 | 49 | 51 | 51 | 51.9 |
| 13 | Rainbow · Grove | 51 | 50 | 51 | 51 | 51 | 51 | 50 | 50 | 48 | 48 | 51 | 48 | — | 51 | 52 | 48 | 51 | 44 | 50 | 47 | 47 | 49 | 49 | 49.5 |
| 14 | Wide Rainbow · Grove | 50 | 49 | 50 | 50 | 50 | 50 | 49 | 49 | 47 | 47 | 49 | 46 | 49 | — | 51 | 47 | 50 | 43 | 49 | 45 | 46 | 47 | 47 | 48.0 |
| 15 | Suits · Grove | 49 | 48 | 49 | 49 | 49 | 49 | 48 | 48 | 46 | 46 | 48 | 46 | 48 | 49 | — | 46 | 49 | 42 | 48 | 44 | 45 | 46 | 46 | 47.3 |
| 16 | Standard · Forge | 53 | 52 | 53 | 53 | 53 | 53 | 52 | 52 | 50 | 50 | 52 | 49 | 52 | 53 | 54 | — | 54 | 45 | 52 | 48 | 48 | 50 | 50 | 51.3 |
| 17 | Standard · Loom | 50 | 49 | 49 | 49 | 49 | 49 | 48 | 48 | 46 | 46 | 49 | 46 | 49 | 50 | 51 | 46 | — | 42 | 49 | 44 | 44 | 47 | 47 | 47.5 |
| 18 | Two Mains · Forge | 58 | 57 | 57 | 57 | 57 | 57 | 56 | 57 | 55 | 55 | 57 | 54 | 56 | 57 | 58 | 55 | 58 | — | 57 | 53 | 53 | 55 | 55 | 56.1 |
| 19 | Two Mains · Loom | 51 | 50 | 50 | 50 | 50 | 50 | 49 | 49 | 47 | 47 | 50 | 47 | 50 | 51 | 52 | 48 | 51 | 43 | — | 45 | 46 | 48 | 48 | 48.8 |
| 20 | Spellbook · Forge | 56 | 55 | 55 | 55 | 55 | 55 | 54 | 54 | 52 | 52 | 55 | 52 | 53 | 55 | 56 | 52 | 56 | 47 | 55 | — | 50 | 53 | 53 | 53.6 |
| 21 | Spellbook · Loom | 55 | 54 | 55 | 55 | 55 | 55 | 54 | 54 | 52 | 52 | 55 | 51 | 53 | 54 | 55 | 52 | 56 | 47 | 54 | 50 | — | 52 | 52 | 53.2 |
| 22 | Ladder Pair · Forge | 53 | 52 | 53 | 53 | 53 | 53 | 51 | 52 | 50 | 50 | 52 | 49 | 51 | 53 | 54 | 50 | 53 | 45 | 52 | 47 | 48 | — | 50 | 51.1 |
| 23 | Ladder Pair · Loom | 53 | 52 | 53 | 53 | 53 | 53 | 52 | 52 | 50 | 50 | 52 | 49 | 51 | 53 | 54 | 50 | 53 | 45 | 52 | 47 | 48 | 50 | — | 51.1 |

**Model check, every sampled pairing:**

| Board | Opponent | Defense | Health | Model | Battles | Gap |
|---|---|---|---|---|---|---|
| Standard · Standard D6 | Wide Rainbow · Grove | no defense | 24 | 0.486 | 0.473 | -0.013 |
| Two Mains · Loom | Standard · Loom | no defense | 22 | 0.476 | 0.489 | +0.013 |
| Standard · Standard D6 | Rainbow · Grove | no defense | 16 | 0.489 | 0.489 | +0.000 |
| Double Ladder · Hearth | Standard · Forge | no defense | 25 | 0.497 | 0.496 | -0.001 |
| Rainbow · Grove | Standard · Hearth | no defense | 18 | 0.540 | 0.539 | -0.001 |
| Wide Rainbow · Grove | Combo · Hearth | no defense | 27 | 0.533 | 0.510 | -0.023 |
| Wide Rainbow · Grove | Lean · Standard D6 | no defense | 29 | 0.570 | 0.560 | -0.010 |
| Suits · Grove | Combo · Standard D6 | no defense | 16 | 0.543 | 0.532 | -0.011 |
| Double Ladder · Hearth | Compact · Hearth | no defense | 13 | 0.551 | 0.594 | +0.043 |
| Suits · Grove | Lean · Hearth | no defense | 27 | 0.498 | 0.501 | +0.003 |
| Two Mains · Forge | Combo · Standard D6 | no defense | 27 | 0.460 | 0.454 | -0.006 |
| Lean · Standard D6 | Wide · Standard D6 | no defense | 17 | 0.474 | 0.464 | -0.010 |
| Standard · Forge | Ladder Pair · Forge | no defense | 20 | 0.501 | 0.505 | +0.004 |
| Combo · Standard D6 | Wide Rainbow · Grove | no defense | 26 | 0.531 | 0.524 | -0.007 |
| Two Mains · Loom | Lean · Hearth | no defense | 18 | 0.532 | 0.524 | -0.008 |
| Standard · Standard D6 | Wide · Hearth | no defense | 25 | 0.478 | 0.486 | +0.008 |
| Ladder Pair · Forge | Compact · Hearth | Light Guard | 30 | 0.529 | 0.514 | -0.015 |
| Ladder Pair · Loom | Combo · Standard D6 | Light Guard | 21 | 0.488 | 0.514 | +0.026 |
| Lean · Standard D6 | Double Ladder · Hearth | Light Guard | 24 | 0.426 | 0.407 | -0.019 |
| Spellbook · Loom | Standard · Hearth | Light Guard | 22 | 0.555 | 0.557 | +0.002 |
| Spellbook · Loom | Combo · Standard D6 | Light Guard | 24 | 0.505 | 0.489 | -0.016 |
| Rainbow · Grove | Compact · Hearth | Light Guard | 22 | 0.491 | 0.501 | +0.010 |
| Wide Rainbow · Grove | Spellbook · Forge | Light Guard | 14 | 0.533 | 0.551 | +0.018 |
| Ladder Pair · Forge | Two Mains · Forge | Light Guard | 14 | 0.514 | 0.497 | -0.017 |
| Compact · Hearth | Compact · Standard D6 | Light Guard | 28 | 0.498 | 0.510 | +0.012 |
| Standard · Standard D6 | Suits · Grove | Light Guard | 30 | 0.502 | 0.522 | +0.020 |
| Wide Rainbow · Grove | Standard · Forge | Light Guard | 25 | 0.506 | 0.509 | +0.003 |
| Rainbow · Grove | Lean · Standard D6 | Light Guard | 27 | 0.542 | 0.537 | -0.005 |
| Ladder Pair · Forge | Combo · Hearth | Light Guard | 29 | 0.511 | 0.527 | +0.017 |
| Compact · Standard D6 | Double Ladder · Hearth | Light Guard | 19 | 0.478 | 0.463 | -0.015 |
| Spellbook · Forge | Two Mains · Forge | Light Guard | 17 | 0.493 | 0.476 | -0.017 |
| Lean · Standard D6 | Lean · Hearth | Light Guard | 29 | 0.447 | 0.443 | -0.004 |
| Wide · Standard D6 | Ladder Pair · Loom | Holdfast | 25 | 0.472 | 0.464 | -0.008 |
| Two Mains · Forge | Spellbook · Forge | Holdfast | 29 | 0.533 | 0.529 | -0.004 |
| Wide Rainbow · Grove | Standard · Forge | Holdfast | 15 | 0.484 | 0.504 | +0.020 |
| Combo · Hearth | Rainbow · Grove | Holdfast | 12 | 0.567 | 0.554 | -0.013 |
| Compact · Hearth | Compact · Standard D6 | Holdfast | 15 | 0.499 | 0.490 | -0.009 |
| Lean · Hearth | Standard · Forge | Holdfast | 28 | 0.464 | 0.464 | +0.000 |
| Double Ladder · Hearth | Combo · Hearth | Holdfast | 23 | 0.479 | 0.499 | +0.020 |
| Wide · Hearth | Standard · Loom | Holdfast | 17 | 0.566 | 0.582 | +0.015 |
| Combo · Standard D6 | Double Ladder · Hearth | Holdfast | 15 | 0.555 | 0.555 | -0.000 |
| Double Ladder · Standard D6 | Standard · Loom | Holdfast | 30 | 0.620 | 0.607 | -0.013 |
| Standard · Forge | Wide · Standard D6 | Holdfast | 26 | 0.531 | 0.531 | +0.001 |
| Two Mains · Forge | Combo · Standard D6 | Holdfast | 12 | 0.481 | 0.491 | +0.010 |
| Spellbook · Forge | Standard · Loom | Holdfast | 21 | 0.619 | 0.614 | -0.006 |
| Wide · Hearth | Combo · Standard D6 | Holdfast | 16 | 0.423 | 0.410 | -0.012 |
| Two Mains · Loom | Combo · Hearth | Holdfast | 13 | 0.456 | 0.453 | -0.004 |
| Compact · Hearth | Lean · Hearth | Holdfast | 25 | 0.502 | 0.520 | +0.019 |
