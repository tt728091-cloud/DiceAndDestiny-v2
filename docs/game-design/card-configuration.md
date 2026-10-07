# Configurable card catalog

## Audit and implementation

The active catalog contains the 21 previously supported General templates, 24 Venom cards, 24 Curse cards, Alchemist’s Gamble and Brine Surge. Every card can now be opened, cloned, renamed, validated, published and equipped through Character Creation → Card Creation.

Before this change, Venom and Curse used card IDs to select behavior, with numeric values and delayed-effect values embedded in handlers. Their JSON/YAML files exposed presentation and energy costs but did not describe those mechanics. Brine Surge already had authored operations but its policy validator assumed one energy and a round-long modifier. Alchemist’s Gamble already had a data-driven outcome table, but the editor did not expose it.

The 48 specialized definitions now explicitly contain `mechanic.kind`, `params`, `windows` and usage limits. Preparation cards also contain `expiration` and `rounds`. The mechanic selector is independent of card ID/name. Delayed work retains the originating card definition from the battle’s pinned catalog. Old pinned definitions without a mechanic continue through compatibility defaults.

The editor uses the same parameter registry and native validation as the runtime. General cards and Brine Surge use composable `program` steps. Specialized cards use recipes for physical dice, toxin reactions, opponent choices and delayed triggers. Alchemist’s Gamble uses an editable numbered-outcome table.

Card Creation opens with a blank program card. **New blank card** starts a fresh draft with an empty ID, name, illustration, effects and upgrades; templates remain optional. Fill in the Card fields and add effects under **Effects & choices** before publishing. Neutral starting settings are General access, zero energy, 10 XP buy/sell, 20 copies, play from hand to discard, and offensive planning with no roll restriction. These are editable and effect compatibility still controls the available play windows. Starting over protects unpublished guided or JSON edits with Keep editing / Discard changes. `verify_blank_card_creation.gd` covers blank creation, template reset, navigation, discard protection, publication, deck saving and the authored effect in a real battle.

## Shared configuration

- Card ID, name, presentation/art, access family, energy cost (0–100).
- Play source: hand, draw pile or discard; play destination: hand, draw pile, discard or permanently removed. Removed cards cannot be played or resurrected. Removing cards records wounds and loses health.
- Timing: a nonempty subset of the recipe’s supported windows. Unsupported phase/effect combinations fail validation.
- Timing is authored as **Before**, **After** or **Any time** (or not playable) for your own Offense and Defense turn; `content.CardTimingChoices` maps each choice onto engine windows, narrowed to the windows the card's effects support:

  | Choice | Offense (your turn) | Defense (the Defense screen) |
  | --- | --- | --- |
  | Before | `offensive_before_roll`: until your first offensive roll | `defense_before_roll`: until your first defense roll |
  | After | `offensive_after_roll`: after that roll | `defense_after_roll` (+ hidden legacy `damage_reaction`): after that roll, until Pass |
  | Any time | `offensive_planning` | `defense_selection` (+ `damage_reaction`) |

  The two in-between moments are a separate **opt-in reaction** setting per segment (`content.CardReactionWindows`), off by default, because play pauses there whenever a card is playable: `offensive_reaction` (after attack dice are revealed) and `defense_reaction` (after a defense roll's dice land, before it applies). Only effects that need them use them — Disrupt and Tip It (enemy dice) and Second Guard and Hexward Retort (defense rerolls); Spined Rebuttal and Antidote keep their original review timing. "Before your first roll" counts any attack; skipping an attack or another participant's roll does not end it, and passing without rolling never reaches After. Die effects need rolled dice, so they never offer Before. Program and specialized cards add a generated `Play: …` line to their rules text (for example "Play: Offense, only as a reaction to revealed attack dice."), and the editor's status line and the deck builder show the same choices. New card-tree cards default to Any time without the reaction moments. Legacy `roll_requirement` values still work but only gate offensive planning; the editor converts them to the equivalent windows. `card_timing_test.go` and `verify_card_timing_choices.gd` cover the authority and the editor.
- In battle, program cards target card-first on the board, like built-in cards. When starting a card would immediately ask for a choice, its start actions also carry each first choice (`then` plus the choice's `kind`), so the client highlights the targets — incoming attacks for prevention, dice for die effects, ability tiles for ability bonuses, and a chooser for statuses, cards and options — and the player can cancel without contacting the authority. One viable choice plays immediately. Later choices of a started card use the same targeting. Ability bonuses may name an authored `preparation_status` (Strong Swing uses `strong_swing_ready`) instead of a generated one. A completed program card in a reaction window hands priority on like any other reaction card, and prevention publishes the shared `damage_prevented_or_modified` feedback event.
- Per-round and per-battle play limits (0 means unlimited; active, nonstacking preparations still prevent duplicates).
- XP buy/sell prices, copy limits and upgrade branches; access restrictions still apply when equipping a deck.
- Prevention recipes: saved cards may remain in their live original pile, move to discard/hand/deck, or be permanently removed. The played card still pays its own configured destination.
- Preparations: expiration at Offense exit, damage-resolution exit, Income, ongoing effects or battle end. `rounds` is 1–100; 1 means this round’s exit for exit checkpoints and next round for Income/ongoing checkpoints. Battle-end expiration ignores the round offset.

Published changes affect new battles. Running battles and replays retain their pinned catalog. Authoring never edits an active battle’s rules.

## Specialized effect parameters and shipped defaults

Most quantities allow 1–100. Optional costs and limits that explicitly permit zero allow 0–100. Face lists use 1–6, face deltas −5–5, and Second Knell retries 1–10. Status references must exist; ability references must be offensive abilities. Terminal Formula only targets the existing toxin-capture abilities Terminal Bite or Fever Spike. The editor exposes these restrictions and native validation rejects invalid references, unknown parameters, fractional numbers, invalid lists and contradictory bounds.

| Card | Family | Effect parameters |
| --- | --- | --- |
| Accelerant | venom | `convert_stacks` = 1, `cost_stacks` = 1, `cost_status` = "catalyst", `gain_stacks` = 1 |
| Agitate | venom | `checks` = 0 |
| Antivenom Draught | venom | `cleanse_stacks` = 1, `cost_stacks` = 1, `cost_status` = "catalyst", `prevent` = 2 |
| Bitter Reagent | venom | `stacks` = 1, `status_id` = "catalyst" |
| Coagulate | venom | `cost_stacks` = 1, `cost_status` = "poison", `prevent` = 3 |
| Culture Flask | venom | `stacks` = 2, `status_id` = "catalyst" |
| Deep Puncture | venom | `damage_cost` = 1, `minimum_damage` = 2, `stacks` = 1, `status_id` = "poison" |
| Distill | venom | `convert_stacks` = 1, `cost_stacks` = 1, `cost_status` = "catalyst", `gain_stacks` = 1 |
| Emergency Molt | venom | `prevent` = 2, `reward_stacks` = 1, `reward_status` = "catalyst" |
| Extract | venom | `cost_stacks` = 1, `cost_status` = "poison", `stacks` = 2, `status_id` = "catalyst" |
| Fever Cycle | venom | `checks` = 2 |
| Forked Tongue | venom | `deltas` = [-1, 1], `maximum` = 6, `minimum` = 1 |
| Incubate | venom | `cost_stacks` = 1, `cost_status` = "catalyst", `stacks` = 1 |
| Measured Dose | venom | `stacks` = 2, `status_id` = "catalyst" |
| Pinprick | venom | `stacks` = 1, `status_id` = "poison" |
| Repurpose | venom | `amount` = 2, `cost_stacks` = 1, `cost_status` = "catalyst" |
| Shock Dose | venom | `cost_stacks` = 1, `cost_status` = "volatile_poison", `damage` = 3 |
| Slow Release | venom | `cost_stacks` = 0, `cost_status` = "catalyst", `stacks` = 1 |
| Spined Rebuttal | venom | `prevent` = 1, `stacks` = 1, `status_id` = "poison" |
| Steady Hand | venom | `faces` = [1, 4] |
| Terminal Formula | venom | `ability_id` = "terminal_bite", `damage_per_check` = 1 |
| Twin Puncture | venom | `stacks` = 2, `status_id` = "poison" |
| Venom Lens | venom | `ability_id` = "needlefang", `damage` = 1 |
| Venom Reserve | venom | `amount` = 1, `cost_stacks` = 1, `cost_status` = "catalyst" |
| Black Dividend | curse | `energy` = 1, `rewards` = 2 |
| Black Fingerprint | curse | `dice` = 1, `fallback_rolls` = 1 |
| Black Tax | curse | `groups` = 1, `penalty` = 1 |
| Blind Omen | curse | `cost_count` = 3, `required_count` = 6, `stacks` = 1, `status_id` = "blind" |
| Call the Mark | curse | No numeric knobs; identity, cost, timing, usage, piles, economy and preparation duration remain editable. |
| Chosen Instrument | curse | No numeric knobs; identity, cost, timing, usage, piles, economy and preparation duration remain editable. |
| Curse Bloom | curse | `curses` = 3 |
| Curse-Eater | curse | `cost_count` = 3, `draw` = 2, `energy` = 2 |
| Grave Interest | curse | `groups` = 1, `penalty` = 1 |
| Hexward Retort | curse | `curses` = 1 |
| Malediction’s Refusal | curse | `curses` = 1 |
| Mark the Number | curse | `curses` = 1 |
| Misfortune’s Choice | curse | `cost_count` = 3, `damage` = 2, `stacks` = 1, `status_id` = "cursed_entangle" |
| No Safe Keep | curse | No numeric knobs; identity, cost, timing, usage, piles, economy and preparation duration remain editable. |
| Rotten Numeral | curse | `clean_only` = false, `dice` = 3, `faces` = [1, 2, 3, 4, 5, 6] |
| Ruin Made Flesh | curse | `cost_count` = 3, `damage` = 2, `maximum_cost` = 6 |
| Second Knell | curse | `retries` = 1 |
| Shared Misfortune | curse | `clean_only` = true, `dice` = 2, `faces` = [1, 2, 3, 4, 5, 6] |
| Spiteful Ward | curse | `curses` = 1, `prevent` = 2 |
| Stored Calamity | curse | No numeric knobs; identity, cost, timing, usage, piles, economy and preparation duration remain editable. |
| Three Knocks | curse | `multiplier` = 2 |
| Tomb’s Choice | curse | `count` = 3, `rolls` = 5 |
| Unquiet Hands | curse | `rolls` = 1 |
| Widen the Crack | curse | `deltas` = [-1, 1] |

## Other active cards

**Alchemist’s Gamble:** target self or one enemy; number of physical dice rolled; complete, nonoverlapping face-to-outcome table; one or more effects per outcome. Supported outcome effects are damage, status application, energy, draw and no effect, directed at self or the chosen target. Rolls use the target’s owned dice and preserve Curse/physical-die rules. The table uses faces 1–6; it does not replace a character’s physical dice loadout. Every face must be covered exactly once.

**Brine Surge:** exposed through the same composable ability-bonus program as General cards, including damage, qualification conditions, duration, stack behavior, cost, timing and piles. Its opponent policy reads the configured card ID and completes card choices; its one-card-per-round strategy no longer depends on a hardcoded energy price or modifier duration.

## Rules that remain mechanic semantics

Configuration changes values and composes the supported operations; it is not an arbitrary scripting language. Specialized recipes retain their defining interactions: owned physical dice, Curse conversion groups, shared Provoke allowance, Poison overflow, toxin maturation, target relationships, legal response windows, and nonstacking preparation slots. Those are game rules, not silently ignored card settings. A new kind of trigger or effect requires an engine capability before the editor can offer it.

Preparations are authoritative statuses, not hidden counters. Their visible rules describe consumption and expiration. Cleansing a preparation prevents its delayed work; use and expiration read the same status instance. Saved games preserve configured rewards and originating-card associations.

## Validation coverage

- Every active definition converts to an editable card, publishes with a new ID/name and reloads.
- Every specialized recipe exposes executable legal choices under a renamed definition.
- Full battles with renamed Venom and Curse decks against two enemies, with zero invalid/rejected authority actions.
- Configured status costs, draws, conversion quantities, delayed energy rewards, multiple Second Knell retries with all physical results retained, every roll-table face, play piles, prevention destinations, usage limits and expiration/reload.
- Editor controls, native preview, publish, equip and real battle play; invalid outcome overlaps cannot publish.
- Full `go test ./...` (battle, progression, replay, save and other server packages).
- Godot authority smoke; guided and specialized card creation; program-card battles; a full two-enemy battle; unified defense; Needlefang tier selection; Curse feedback; and Bloom, Dividend, Knell and Refusal lifecycle presentations.
- Presentation checks at 1280×720 and 1920×1080 where supported, plus a visible Card Creation render.

Implementation entry points: `internal/content/card_mechanics.go`, `internal/battle/engine/card_mechanics.go`, `internal/battle/learned/card_authoring.go`, and `app/screens/character/card_authoring.gd`.

## Admin card deletion

In Progression mode, open **Admin settings**, hover/focus a card in the Cards list, and use **Delete card…** in its preview. The confirmation names both the card and its ID. Deletion is immediate and discards pending economy edits; Cancel preserves the definition. Card Creation and the ordinary deck inspector have no delete action.

The native admin service checks for an active workspace-scoped Admin session, a current catalog revision, and dependencies again at confirmation. Owned copies must first be sold or removed from the shared deck. Starter decks, upgrade paths, opponent policies and other catalog/progression references block deletion and show a reason. No cascade removes cards from decks or changes XP. Existing admin price/type overrides for deleted cards are excluded from future settings snapshots.

Deletion atomically removes a custom definition and records its ID as deleted in `authored_cards.json`; source-pack cards cannot reappear on reload, and stale creator drafts cannot republish that ID. Active battles retain their pinned definitions. Use a new ID to create a replacement.

Admin Settings is currently a local desktop workflow, not an authenticated account/role system. Its short-lived native capability separates deletion from ordinary editing and is revoked when the panel closes; it is not a security boundary against someone controlling the local process or files.

Regression coverage: `TestAdminCardDeletionLifecycle` exercises native authorization, stale revisions, ownership/upgrade dependencies, deletion/reload, price/type overrides, and completion of an already-active battle. `verify_admin_card_deletion.gd` exercises pointer confirmation/cancellation, preview visibility, library/creator refresh, dependency messaging and session revocation. The existing admin preview tests cover multiple viewport sizes.
