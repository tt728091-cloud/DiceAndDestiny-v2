# Exact 5d6 reroll policy tables

`dice_destiny_ml.dice_policy_table` generates exact teacher and runtime lookup
tables for five fair six-sided dice with up to two rerolls. Targets are supplied
as JSON patterns. A roll completes a target when it contains every die in any
one of that target's patterns.

## Generate the example

Run from `dice-and-destiny-server/ml`:

```bash
python3 -m dice_destiny_ml.dice_policy_table \
  --targets configs/dice-policy-example.json \
  --output /tmp/dice-policy-example-table.json
```

The example defines three ones, three twos, and the three alternative small
straights. The generator creates one objective for each target and an
`any_of_all` objective that maximizes the probability of completing any of
them.

Set `include_any_of_all` to `false` to omit the generated combined objective.
Additional named subsets can be declared explicitly:

```json
{
  "targets": [
    {"id": "three_ones", "patterns": [[1, 1, 1]]},
    {"id": "three_twos", "patterns": [[2, 2, 2]]}
  ],
  "include_any_of_all": true,
  "combined_objectives": [
    {
      "id": "either_triple",
      "name": "Maximize either triple",
      "targets": ["three_ones", "three_twos"]
    }
  ]
}
```

## Output structure

Every objective contains:

- `before_first_roll`: the probability of eventual success from a fresh roll
  followed by optimal use of up to two rerolls;
- `stages.0`: all 252 terminal roll states with no rerolls remaining;
- `stages.1`: all 252 states with one reroll remaining;
- `stages.2`: all 252 states with two rerolls remaining.

That is 756 roll-state values plus one pre-roll value, or 757 primary values per
objective. A state is keyed by its face-count vector. For example,
`1,0,0,0,0,4` means one `1` and four `6`s.

Decision-stage entries include `optimal_keeps` and an `actions` array containing
the eventual probability for every unique keep action. Each probability is
provided as an exact fraction, a decimal probability, and a percentage. Keeping
all five dice is the stop action; it rerolls zero dice.

The 252 states are not treated as equally likely. Every transition and the
pre-roll value use the correct multinomial probability for that sorted roll.

## Symbols and ranked tiers

Named symbols can map to one or more numbered faces. This 3-2-1 die maps three
faces to swords, two to shields, and one to a heart:

```json
{
  "symbols": {
    "sword": [1, 2, 3],
    "shield": [4, 5],
    "heart": [6]
  }
}
```

A minimum symbol-count target uses the symbol name rather than enumerating all
combinations of its faces:

```json
{"id": "three_shields", "symbol": "shield", "at_least": 3}
```

See `configs/dice-policy-symbol-example.json` for three nested shield tiers, the
three alternative small straights, and a `shield_tiers` ranked objective. Its
rewards `1`, `2`, and `3` express that a higher shield tier is always better
without depending on the game's damage amounts. The ranked table reports
expected reward and the exact probability of finishing with no shield tier,
exactly the three-shield tier, exactly the four-shield tier, or the five-shield
tier. The generated `any_of_all` objective separately maximizes the chance of
reaching either a small straight or any shield tier.

Generate it with:

```bash
python3 -m dice_destiny_ml.dice_policy_table \
  --targets configs/dice-policy-symbol-example.json \
  --output /tmp/dice-policy-symbol-table.json \
  --balance-output /tmp/dice-policy-symbol-balance.json
```

## Separate damage balance reports

`balance_reports` are excluded from the teacher table and written only when
`--balance-output` is supplied. Each report assigns damage to completed target
outcomes and calculates a separate policy that maximizes expected damage:

```json
{
  "balance_reports": [
    {
      "id": "offensive_cycle_damage",
      "teacher_objective": "any_of_all",
      "outcomes": [
        {"target": "three_shields", "damage": 5},
        {"target": "four_shields", "damage": 6},
        {"target": "five_shields", "damage": 7},
        {"target": "small_straight", "damage": 9}
      ]
    }
  ]
}
```

Outcomes must be ordered by nondecreasing damage. When a final roll completes
more than one outcome, the report assigns it only to the completed outcome with
the highest damage. Equal-damage ties go to the later configured outcome. This
makes the probability rows mutually exclusive and prevents the same roll's
damage from being counted twice.

The balance output contains two policies:

- `teacher_success_policy` preserves the referenced teacher objective's maximum
  success probability. Expected damage only breaks ties between actions with
  exactly equal success probabilities.
- `damage_optimized_policy` may sacrifice success probability when doing so
  increases expected damage.

Each policy has a before-first-roll summary and all 756 state/stage entries.
Each summary reports average damage, overall hit probability, the exclusive
probability of each outcome, and that outcome's contribution to the average.
State entries report that policy's optimal dice to keep. The comparison section
reports the damage gain and hit-probability change caused by switching from the
teacher policy to damage optimization.

## Plug-and-play character workbooks

The full character workbook is configuration-driven. Adding a character no
longer requires editing the probability solver, analysis generator, or workbook
builder. `configs/dice-policy-barbarian.json` is the complete reference config.

Each character config supplies:

- `character`: stable ID, display name, and the matching character name in the
  Dice Throne reference CSV;
- `symbols`: the numbered die faces belonging to each character symbol;
- `targets`, `combined_objectives`, and `balance_reports`: exact activation
  patterns, the reliability-first objective, and mutually exclusive listed-value
  outcomes;
- `character_analysis.abilities`: the displayed ability list, source-row name,
  requirement/effect text, listed value or tier values, category, recommendation,
  and value type;
- `character_analysis.tier_targets`: optional tier-threshold rows;
- `character_analysis.reports`: which balance report drives the primary summary
  and which drives the all-ability teacher;
- `character_analysis.adjustments`: optional outcome-based additions or
  subtractions such as Reckless self-damage.

From the repository root, generate the analysis JSON and complete workbook with
one command:

```bash
DICE_ANALYSIS_NODE=/path/to/node \
DICE_ANALYSIS_NODE_MODULES=/path/to/node_modules \
python3 dice-and-destiny-server/ml/scripts/generate_dice_character_workbook.py \
  --config dice-and-destiny-server/ml/configs/dice-policy-barbarian.json \
  --output /absolute/output/path/barbarian-roll-analysis.xlsx
```

`DICE_ANALYSIS_NODE_MODULES` must contain `@oai/artifact-tool`. If it is already
installed where Node can resolve it, omit that variable. The launcher creates a
disposable runtime directory and does not add `node_modules` to the repository.

The generated workbook includes the reference data, character ability summary,
overall reliability-first and value-first results, values with zero/one/two
rerolls, compact teacher choices, compact value-first choices, all legal actions,
and all 252 state frequencies with per-10,000 and per-7,776 counts.

For Monk, copy the Barbarian config to a Monk config and replace only the
character data: Monk's face-to-symbol map, ability targets, displayed metadata,
teacher objective, outcome values, and any explicit adjustments. The generator
and workbook code remain unchanged.
