# Configurable abilities and Ability Creation

Ability Creation is available from Character Creation, beside Card Creation. It opens with a blank draft. Enter an ID and name, then choose **Offensive** or **Defensive** directly below Name. Add requirements/effects, validate and preview its rules, publish it, and assign it through Character boards. **New blank ability** starts another definition without a template; editing an existing definition and creating a copy remain optional shortcuts. Publication and assignment affect future battles. An active battle continues using its pinned catalog.

The form groups fields into Identity, Cost & usage and Targeting (or Defense) panels with two-column captions. Lists such as activation tiers, effects and follow-ups render as numbered item cards with **Remove** and an **+ Add** control; dropdowns show display names while saving the same IDs. The preview shows the ability name, type and energy badges, and the generated rules. Character boards show Offense and Defense columns of check boxes; incompatible abilities stay disabled with a reason.

Switching a new draft's type exposes the matching controls and keeps separate offensive/defensive settings for that draft while the editor is open. Only the active type is published. New blank ability clears both sets. A published ID keeps its existing type; enter a new ID or create a copy to change types. Blank definitions cannot publish until named and given offensive tiers or defensive effects.

## Audit of current character abilities

The audit covers 27 definitions, including Guard+. Blade Warden's seven original definitions already use the shared operation interpreter. Adventurer's qualification, status, resource and prevention operations were also mostly declarative. The remaining tier selection, Venom choices and Curse follow-ups previously depended on specific ability IDs. Those behaviors now have explicit configuration. Original Blade Warden files retain their training-catalog format; copies and edits use configuration version 1 without changing the policy's training fingerprint.

| Character | Abilities | Configurable behavior |
| --- | --- | --- |
| Adventurer | Strike | Ordered symbol thresholds, damage per tier, explicit choice of any qualified tier |
| Adventurer | Guarded Strike | Mixed symbol requirements, damage, immediate positive self-status and stacks |
| Adventurer | Measured Strike | Mixed requirements, damage, energy gain |
| Adventurer | Small Straight, Large Straight | Number pattern, damage, additional operations |
| Adventurer | Decisive Blow | Symbol minimum and damage |
| Adventurer | Guard, Guard+ | Number/type of dice, exhaustive face outcomes, prevention per face, capped energy gain, saved-card destination |
| Venom | Needlefang | Selectable tier requirements, damage and Poison per tier |
| Venom | Venom Gland | Requirements, enemy status, self-status, stack amounts |
| Venom | Fever Spike | Requirements and damage/check count per tier |
| Venom | Terminal Bite | Damage, toxin-check count, reward after checks have been captured |
| Venom | Shedskin | Dice/outcomes, Incubation application, optional status ID/payment stacks/additional prevention |
| Venom | Barbed Mantle | Incoming-ability restriction, dice/outcomes, Poison/Catalyst, conditional status threshold, result/cap condition, fallback status and stacks |
| Curse | Hexbrand | Selectable tiers, damage, tier-specific sequential Curse count after damage |
| Curse | Funeral Rattle | Damage, cursed-dice roll limit, Count threshold, resulting status/stacks |
| Curse | Grasp of the Sarcophagus | Requirements, ordinary Curse count, Entomb choice and per-owner limit |
| Curse | Eclipse of the Black Star | Requirements, number choice across owned dice, fallback roll count |
| Curse | Hexward Rebuttal | Dice/outcomes, prevention, unconditional Curse follow-up count |
| Curse | Misfortune Repaid | Cost, dice/outcomes, any-face condition, once-per-activation Count award, prevention-gated die choice, initial cursed face |
| Blade Warden | Sword Cut | Exact symbol-count tiers, damage, conditional number-pattern status bonus |
| Blade Warden | Shield Bash | Mixed symbol requirements, damage and Entangle |
| Blade Warden | Golden Edge | Requirements, damage and energy |
| Blade Warden | Perfect Form | Exact face set and damage |
| Blade Warden | Venom Strike | Requirements, damage and Poison |
| Blade Warden | Basic Defense | Die/count, rolled-face prevention |
| Blade Warden | Protect | Energy cost, incoming-damage fraction and floor rounding |

A new name/ID does not require an engine branch. Character passives remain character rules: for example, Venom's Poison-overflow behavior belongs to Venom, rather than being inferred from the presence of a specifically named attack. Existing statuses retain their catalog rules, caps and expiration; assigning an application to an ability does not duplicate or redefine the status.

## Definition fields

- `configuration_version: 1` opts into authored behavior. `schema_version` remains 1.
- `id`, `name`, `type`, `presentation`, `cost.energy`, `usage.maximum_per_segment` describe identity, type, rules/art and cost/limits. The native creator generates rules from the actual operations. Editing a published ability cannot change its type; create a different ID instead.
- Offensive `targeting` selects exactly one enemy or self. Effects may use `self`, `source_actor`, `selected_targets`, `target_actor`, or `enemy`. In ordinary offensive operations `enemy` addresses opponents; use `selected_targets` for the selected enemy. A follow-up's `enemy` is the selected opponent, and a defensive operation's `enemy` is the attacker.
- `qualification.activation_tiers` is ordered. Each tier has an ID, requirements and operations. `choose_tier` allows choosing any qualifying tier instead of the default highest qualifying tier. `conditional_bonuses` supplies additional operations when its requirements match.
- Requirements support symbol counts (minimum/maximum/exact), number patterns (three of a kind, exact pair, pair or better, small/large straight), and exact face sets. Character assignment checks the actual dice definitions for at least one possible qualifying roll.
- Defensive `selection` requires one incoming `damage_source`. `offensive_abilities_only` restricts the eligible sources without relying on the ability's name.
- Defensive `resolution.operations` contains a fixed defense or one top-level `roll_dice` operation with 1–5 dice. Its outcomes must cover all faces exactly once. `energy_gain_limit` caps the energy awarded by the complete defense roll; zero means uncapped.
- `saved_card_destination` is `original` (default) or `discard`, following the existing prevention contract. Reservation release never restores removed cards or rewinds a card play. Card programs continue to support their broader explicit pile-movement/sacrifice destinations; those are not implicit ability-prevention destinations.
- `optional_payment` names a status, number of stacks to spend, and additional prevention. The status is spent before rolling through the normal defense command. Legal actions and the UI use the configured values. The old wire field `spend_catalyst` remains for save/protocol compatibility; it now means the declared optional payment.
- `hooks` add operations at explicit checkpoints. A hook can restrict itself to a tier. Defensive hooks can require any of specified faces and/or actual prevention from the resolved defense. A defensive hook runs once for the completed activation, not once per matching die. Offensive follow-ups run for their affected target/source.

Offensive energy is reserved when the attack is committed. A reaction-driven reselection charges or refunds only the difference; retaining the same selection does not charge again. The reservation survives save/reload. Ordinary resource changes, card draws, damage reservations and health continue through the existing authority paths.

## Shared operations and checkpoints

Cards, abilities and statuses use the common effect interpreter for damage, drawing, resources, status application/removal and dice outcomes. Card programs call the same draw/resource/status primitives. Ordinary draws never recycle discard.

The `special_effect` operation exposes existing owned-die mechanics through explicit parameters:

| Kind | Parameters |
| --- | --- |
| `curse` | `amount` ordinary Curse applications |
| `roll_cursed` | `amount` maximum distinct cursed dice |
| `entomb_choice` | Curse `amount`, Entomb `limit` (1–5) |
| `curse_face_choice` | Fallback roll `amount` when all faces are already cursed |
| `curse_die_choice` | Initial `face` (1–6); partly cursed dice expand and full dice Surge |
| `status_threshold` | `status_id`, `threshold`, `result_status_id`, `stacks` |
| `conditional_status` | Required status/threshold, result status/stacks, result-stack `limit`, fallback status/stacks |

Card Creation also advertises `curse`, `roll_cursed`, `status_threshold` and `conditional_status` program steps, which invoke the same shared implementations. Existing card mechanics retain their choice orchestration and reuse the owned-die machinery. This is one effect implementation with source-specific timing and input orchestration, rather than separate card and ability rules.

Hook timing:

- `before_defense`: after offensive effects/reveal, before advancing to defense; supports non-damaging abilities such as Grasp and Eclipse.
- `after_provoke`: after the attack's toxin checks are captured, before their eventual resolution. This preserves Terminal Bite's existing check/reward ordering. Requires a tier containing both damage and Provoke.
- `after_damage`: after remaining damage is committed, including when all damage was prevented. Requires an attack source. Effects on a defeated target are skipped; self rewards can still resolve.
- `after_defense`: after the selected defense resolves, before its follow-up choices are completed. `faces_any` means at least one die, not a reward per die. `requires_prevention` measures a real reduction from the resolution, not merely a prevention operation printed in the definition.

Hooks support draw, energy, apply/remove status stacks, shared special effects and no-op. Damage and prevention belong in activation/resolution operations. Input-dependent card operations (selecting a card, editing a selected offensive die, or selecting another ability) are rejected on an ability instead of silently lacking their required input.

## Creator validation and storage

The guided panes expose identity/cost/targeting, requirements/effects, follow-ups, character boards and advanced JSON. The effect picker filters obvious offense/defense/hook conflicts. Native validation is authoritative and blocks publication for invalid references, unknown fields/effects, invalid conditions, overlapping/incomplete outcomes, incompatible timing/targets, impossible dice counts, unsupported repeat contexts and excessive nesting. The rule preview is regenerated from the same draft. Tab navigation never requires a publishable definition. Readable but incomplete JSON returns to the guided forms for editing. Malformed JSON or incompatible field shapes remain in a separate preserved buffer; other tabs show the last readable draft, and publication remains blocked until the JSON is corrected or explicitly discarded. Opening JSON without changing it does not mark a draft dirty. Back protects actual unpublished edits with a compact, readable confirmation row.

Character boards require 1–20 offensive and 1–20 defensive definitions, no duplicates, matching types and compatible character access/dice. Publishing an edited definition checks reachability for its assigned catalog boards. The creator does not grant extra offensive turns merely because a usage cap is larger; the battle's normal action economy still applies.

The workspace-local `authored_abilities.json` holds definitions, boards and revisions. Writes are atomic and reject stale revisions. Board revisions synchronize assignments into the shared loadout for Sandbox and Progression, while later XP upgrades are not repeatedly overwritten by an old assignment. Card and ability overlays are validated together, so references can cross between authored content. Active saves/replays continue to use their original pinned definitions and version-zero compatibility behavior.

## Verification

Automated coverage includes:

- Renaming, publishing and assigning each character's complete board, followed by completed native battles for all four characters, with no invalid actions or authority rejections.
- Shared draw/resource/status results across card, ability and status sources; both branches of configurable conditional-status effects.
- Renamed tier selection, Provoke, configurable rewards, Entomb limits, choice save/reload, and once-per-defense face conditions.
- Offensive energy reservation, insufficient energy, cancellation/refund and persistence without duplicate payment.
- Both Sandbox and Progression assignments, catalog pinning across later publication, stale revisions, and rejected malformed native requests.
- Godot pointer-driven publication and board assignment, native battle use, generated rules, validation of every template, and invalid-JSON preservation.
- Pointer navigation out of blank/incomplete/malformed JSON, Keep editing and Discard actions, and confirmation layout at 1024, 1440 and 1920 pixels wide (`verify_ability_json_navigation.gd`).
- Existing card creation, specialized card creation, defense feedback, tier selection, unified defense and full-battle regressions.

The supported vocabulary covers the mechanics of the current abilities. A genuinely new mechanic, input type or phase still needs an engine implementation and validation/presentation support. Once implemented as a shared effect, it can be exposed to both cards and abilities; merely inventing a new operation name in JSON is intentionally rejected.
