# Configuring damage prevention

Add `saved_card_destination` at the top level of any card or defensive ability
definition, alongside `id`, `name`, and `type`. This is independent of the played
card's `play.destination`.

```json
"saved_card_destination": "discard"
```

Saved cards move to discard, regardless of whether they were threatened in the
draw pile, hand, or discard. They still count as health. Cards already in discard
stay there exactly once. Suggested card text: **Saved cards go to discard.**

```json
"saved_card_destination": "original"
```

Saved cards stay in their current piles. Suggested card text: **Return saved
cards to their piles.** Threatened cards are reservations, so no physical return
is needed. If a card has since been played or drawn, protection follows its live
pile; it never undoes that action. A played Bulwark (Steady Guard tree) still goes to
discard even if it saves itself. Protecting another attack does not clear its own reservation.

Omitting the field defaults to `original`. Invalid values reject the catalog
with a `saved_card_destination` error. No engine or UI changes are needed to make
new variants: copy a definition, choose a unique ID/name, set this field, and add
the ID to the combatant's decklist or defensive ability board. Update
`presentation.effect_summary` (cards) and `presentation.rules_text` to match.

The destination applies to the definition's flat prevention and damage scaling,
including nested defensive dice outcomes. It only affects cards newly saved by
that effect; a later effect cannot move cards already saved by an earlier one.
Each incoming attack keeps its own reservations. Selection order remains hand →
draw → discard for prevention in the combined defense flow, independent of the
destination. No prevention duplicates cards or changes health.

## Adventurer test loadout

| Definition | Display name | Destination | Amount |
| --- | --- | --- | --- |
| `steady_guard` | Steady Guard | `discard` | Prevent 1 from one source |
| `adventurer_guard` | Guard | `discard` | Roll 3; Sword blocks 1, Shield blocks 2 |
| `adventurer_guard_plus` | Guard+ | `original` | Same roll and prevention as Guard |

Both Guards grant at most 1 energy if any Coin is rolled, and each can be used
once per defensive segment. An attack can still be defended only once. Both are
available as choices for testing; progression can later replace one with the other.
The 12-card starter deck contains three Steady Guard; its other nine cards are unchanged.
The Steady Guard card tree adds Brace (`discard`, prevent 3) and
Bulwark (`original`, prevent 3); tree cards live in the authored
content store, so the shipped starter uses the base card.

Files live under `dice-and-destiny-server/content/adventurer_v1/`, in `cards/`,
`abilities/`, and `combatants/adventurer.yaml`. Catalogs are pinned when battles
start: restart the game and start a new battle to test edited content. Existing
battles keep their saved definitions. Pre-experiment saves also retain the legacy
flow, where rolled defense reduces damage before cards are revealed.

## Enemy defenses

| Definition | Display name | Destination | Amount |
| --- | --- | --- | --- |
| `salt_veil` | Salt Veil (Brine Mask) | `discard` | Roll 1D6; prevent half, rounded up |

Salt Veil lives in `dice-and-destiny-server/content/minions_v1/abilities/`.
`TestSaltVeilSavesCardsToDiscard` checks saves from all three piles.

Regression tests flip only this configuration on the same IDs and verify both
destinations, all three piles, real card plays and defensive rolls, persisted
catalogs, played-card costs, and idempotent reconciliation.
