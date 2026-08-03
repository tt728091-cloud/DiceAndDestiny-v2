# Mechanics-aware observation schema v2

Schema identifiers:

- environment: `dice-and-destiny-ml-env-v2`
- observation: `dice-and-destiny-observation-v2`
- action candidates: `dice-and-destiny-action-candidates-v2`

The v2 family is separate from the accepted v1 family. The v1 dimensions,
checkpoint hashes, export format, Python encoder, and native loader remain
valid and are not reinterpreted.

## Shape and authored capacities

The observation contains 2,560 viewer/context features followed by 128 legal
candidate slots of 128 features each. Unused candidate slots are zero and
masked. Authored capacities are checked at runtime and overflow fails closed:

- 10 current dice
- 8 authored symbols
- 12 authored abilities on the acting board
- 6 distinct activation/conditional tiers per ability. Repeated instances of
  the same battle-long authored modifier share one requirement tier and scale
  its generic operation summary by authoritative multiplicity.
- 2 conjunctive requirements per tier
- 128 exact authority candidates

These capacities cover the pinned battle-v1 catalog, including a runtime
ability modifier. They are limits, not truncation rules.

## Mechanics represented directly

The context exposes the viewer seat, round/segment/priority, public actor
resources, viewer-private hand counts and definitions, and the complete current
dice decision:

- rolls used, maximum rolls, and rolls remaining;
- stable die index, die definition linkage, face, value, and symbols;
- kept and rerollable identity;
- public symbol counts.

The acting character's authored offensive and defensive board is ordered into
exact slots. Every ability slot exposes type, qualification and selection,
energy cost, usage limit, target cardinality/selector class, and every authored
or active modifier tier. Every tier exposes its requirements, current progress,
exact authority-equivalent qualification, and generic operation summary
(damage, prevention, status, resources, rolls, and modifiers).

Legal candidates retain their complete original authority command envelopes.
The schema adds command type, targets, selected die identities, card costs and
effects, and an exact one-hot reference to the corresponding authored board
slot. IDs are used for joins and deterministic ordering, not as small lossy
buckets carrying primary meaning.

The v2 action mask excludes observable non-progress and finite-state cycle
candidates: all provisional keep-subset commands, idempotent ability or target
selections, plus a planning draw-card when the deck is empty and it could
recycle the discard pile indefinitely. Reroll candidates already express the
desired changed-dice subset, so automated v2 play loses no roll choice by
omitting keep-only state changes. The authority still reports the complete
legal-action list and v1 remains unchanged, while a v2 deterministic policy
cannot loop on these otherwise legal commands.

## Privacy boundary

Encoding starts from `FromBattleForViewer`. During hidden planning, the model
cannot receive the opponent's hand, card instances, planning dice, kept dice,
roll history, qualified abilities, or private commitment. The public content
catalog omits combatant AI policy. Future authority RNG state and random cursor
are not part of the observation or mechanics teacher.

The mechanics teacher's one-roll lookahead enumerates only the public faces of
the rerolled dice. It never reads, clones, or advances the authority's future
RNG stream. Its source contains no character, ability, card, die, or symbol
special cases.

## Parity contract

Go and Python independently implement v2. Parity transport retains both raw
viewer-safe state and the Go float32/mask payload; Python requires bit-exact
equality for every feature and candidate mask. Exported v2 actors use the same
Go encoder through `EncodeDecisionV2ForRuntime`. Native loading requires an
explicit export SHA-256 and rejects v1 formats or dimensions.
