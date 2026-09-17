# Current revision

The player-facing rules were reworked on September 8, 2026. Read [status revision 2](status-rework-v2.md) first. The notes below retain the implementation boundaries and numeric audit; the draft JSON is authoritative for current proposed mechanics.

# Type expansion design audit — September 7, 2026

## Scope and preserved state

The fourteen new dossiers are Flame, Tide, Storm, Frost, Stone, Verdant, Blood, Beast, Swarm, Construct, Grave, Arcane, Radiant and Fortune. Each has six abilities, 24 cards, a five-D6 map, four signature engine states, two worked sequences and at least three named partner interactions. Additional named card overlays (Ice Seal, Anchor) have explicit scope, limits and expiry in their type’s engine rules.

Curse and Venom’s reviewed panel HTML is preserved exactly and checked by SHA-256. Venom’s playable implementation is unchanged by this design task. Its durable development handoff is `../../game-design/venom-development-handoff.md`.

These 336 new cards are pools for later creature construction. They do not establish 24-card decks, new playable characters or balance-tested matchups. The new engine behaviors are proposed rules, not existing runtime capabilities.

## Sources used

- Existing local Curse and Venom dossiers: reference amounts, accepted Venom decisions and the physical-roll/overlay distinction.
- [Battle Hooks Field Guide](../../battle-hooks-local/index.html): H-03 collection, H-04/H-10/H-18 physical roll gaps, H-07 Income proposals, H-11 kept dice, H-12/H-13 qualification, H-15 revalidation, H-19/H-20 prevention/scaling, H-21/H-22 health-card selection/reveal, H-24 commit, H-26 completion.
- [Current turn structure](../../game-design/turn-structure.md): accepted costs, simultaneous proposal batches, child consequences, cancellation scope, cards as health, overage, and pending defeat.
- Active `battle_v1` status definitions: Poison and Volatile Poison roll rules; **Bleed is deterministic in this game**, not the superficially similar rolled external reference mechanic. Entangle remains the existing status.
- [Dice Throne reference CSV](../../game-design/dice-throne-reference/dice-throne-reference.csv): 1,260 records across 40 characters. Selected status, defense, stored-resource, sacrifice, summon and upgrade examples informed the pattern review. The new cards use original text and the local game's timing/currency; they do not import the external rules wholesale.
- Creature working design: complete a coherent pool before selecting creature decks, progression or health totals.

## Mechanical audit decisions

- **Events have owners.** Pulse rolls the target’s persistent die, so Curse/Storm has a real enemy-roll interaction. Fortune does not steal Favor from an enemy die owner. Physical rerolls can activate overlays; setting a face and Echoing damage cannot.
- **Shared dice remain future work.** Current playable Venom uses effect dice. The proposed Curse/Venom physical-die interaction requires the explicit persistent-die selection contract; the webpage does not claim that adapter is implemented.
- **Distinct event meanings.** Ward cancels a proposed application; Cleanse removes an existing stack. Only the latter earns Flow/Grace. Prevention and actual health-card loss are checked at commit, not against an unmodified initial damage amount.
- **Bounded engines.** Charge rewards voluntary commands, not every effect die. Catalyst corrections, Soaked retries and card corrections share a result budget. Brood, Echo, Pulse and Web have explicit source or trigger limits. Changing Module programs never resets used programs.
- **Real transfers and recovery.** Transfers/conversions are atomic and revalidate room on both sides. Corpse records do not move cards. Restoration keeps instance identity, deletes old Corpse links, excludes sacrifice/exhaustion costs, and shares a two-card-per-round allowance across types.
- **No turn-denial chain.** Chill charges Energy to begin rerolling while preserving a free initial roll; Entangle retains the existing roll reduction. Bound dice retain legal use after their initial result. Ice Seal only targets public cards and does not prevent normal health removal.
- **Finite rescue.** Death Anchor provides one rescue per battle, before final defeat and within the recovery limit. Beacon now chooses the next Income draw from discard.
- **Mixed recipes are feasible.** Each creature retains five combat dice. Raw-number combinations work across affinities; named type symbols remain namespaced. Graft adds Leaf to one face. Rune stores an already-rolled number to set a different die; it no longer adds a symbol.
- **Cross-type links are not universally positive.** Boon and Curse on one face create a reward/risk tradeoff. Most named links identify a specific generator, consumer or shared trigger, and the interaction index does not promise every pair is equally strong.

## Numeric checks

Initial qualification odds enumerate all 6^5 = 7,776 outcomes for each Offensive recipe, before rerolls, overlays or mixed dice. Basic 3/4/5 common-symbol thresholds occur on 50% / 18.75% / 3.125% of initial rolls. These are overlapping qualification probabilities, not mutually exclusive tier frequencies. Actual mixed-dice success rates must be recomputed when a creature is assembled.

Damage baselines distinguish delayed status value: one ordinary Poison stack has unmodified expected lifetime damage 2; a fresh Burn stack in this draft ticks once for 1; existing Bleed at 1/2/3 stacks totals 1/3/6 without interference. Volatile Poison’s unmodified expected lifetime damage is 8, so future mixed builds must respect Venom’s conversion costs and bonus-check limits. Neither expected lifetime value nor initial qualification chance is a match win-rate estimate.

## Validation and remaining review

The static validator checks counts, distinct names, card costs/timing labels, feasible recipes, all sixteen type routes, unique HTML IDs, properly nested HTML, local links, JavaScript syntax, partner counts and unchanged reference panels. The renderer is checked for repeatable output. Results are recorded in `validation-results.json`.

Fresh browser screenshots and visual/browser-interaction testing were **not completed**: the Browser URL policy blocked the existing local-file codex tab. No alternate browser surface, local-server workaround or indirect browser command was used. The user’s supplied index screenshot is preserved unchanged. This limits visual QA; static checks do not establish rendered layout or local-storage behavior in the user’s browser.

No Godot/gameplay tests are appropriate evidence that these new mechanics work: this task changes design documents and the review webpage only. Future work after user review should implement reusable hooks, choose actual mixed-character decks/loadouts, then run gameplay and balance tests.
