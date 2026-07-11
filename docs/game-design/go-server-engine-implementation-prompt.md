# Prompt: Implement the Complete YAML-Driven Battle in the Go Server Engine

Work in this repository:

`/Users/daddymere/games/Dice-and-Destiny-v2`

Your task is to implement the settled battle design in the portable Golang
server engine and prove it with a deterministic, command-by-command full-battle
integration test.

## Non-negotiable scope boundary

Make changes only inside:

`/Users/daddymere/games/Dice-and-Destiny-v2/dice-and-destiny-server/`

The intended implementation areas are the Go domain packages under `internal/`,
server-owned YAML under `content/`, and server test fixtures/testdata. Changes to
`go.mod` or `go.sum` are allowed only if truly necessary.

Do not modify:

- `dice-and-destiny-client/`
- any Godot code or assets
- `adapters/gdextension/`
- `adapters/httpserver/`
- UI or presentation code
- the design documents
- any other repository directory outside `dice-and-destiny-server/`

This is a portable Go authority task. Do not add HTTP calls, curl-based tests,
Godot integration, GDExtension behavior, or client-side work.

## Authoritative design references

Read these completely before planning or editing:

1. General rules and content model:
   `/Users/daddymere/games/Dice-and-Destiny-v2/docs/game-design/turn-structure.md`
2. Deterministic four-round acceptance battle:
   `/Users/daddymere/games/Dice-and-Destiny-v2/docs/game-design/examples/full-battles/blade_warden_vs_venom_goblin.md`
3. Target YAML content library:
   `/Users/daddymere/games/Dice-and-Destiny-v2/docs/game-design/examples/content-library/`

Also inspect the existing Go implementation, especially:

- `dice-and-destiny-server/internal/battle/README.md`
- `dice-and-destiny-server/internal/battle/authority.go`
- `dice-and-destiny-server/internal/battle/engine/`
- `dice-and-destiny-server/internal/battle/state/`
- `dice-and-destiny-server/internal/battle/command/`
- `dice-and-destiny-server/internal/battle/event/`
- `dice-and-destiny-server/internal/battle/snapshot/`
- `dice-and-destiny-server/internal/battle/operation/`
- `dice-and-destiny-server/internal/content/`
- `dice-and-destiny-server/internal/battle/multi_round_integration_test.go`

When interpreting the references:

- The full-battle document is authoritative for the exact acceptance scenario,
  scripted outcomes, card identities, and final state.
- `turn-structure.md` is authoritative for general settled battle rules.
- The example content library is authoritative for the target content concepts
  and composition. Adapt it into server-owned content without editing the source
  documents.
- Open questions or explicitly future/advanced ideas are not acceptance
  requirements unless the full battle depends on them.
- Do not preserve current placeholder behavior when it conflicts with a settled
  rule merely to avoid changing existing code.

## Existing architectural seam to preserve

The portable authority already follows this boundary:

```text
JSON command
-> Authority.HandleCommandJSON
-> typed command validation
-> battle engine/domain packages
-> persisted checkpoint
-> viewer-safe events + pending input + snapshot
-> JSON result
```

Keep gameplay meaning below the transport boundary. `authority.go` should remain
an orchestrator, not become a monolithic rules engine. Preserve deterministic
commands-in/events-and-snapshots-out behavior and the in-memory repository test
path.

## Required implementation outcome

Implement the settled YAML-driven rules needed to run the complete Blade Warden
versus Venom Goblin battle through the real Go authority. This must be a generic
engine implementation driven by loaded content—not a hardcoded function that
recognizes this battle, actor IDs, card names, round numbers, or expected result.

At minimum, the implementation must support the following as reusable rules:

### Content and setup

- Strict YAML decoding with useful validation errors.
- A central symbol catalog with stable symbol IDs.
- Dice definitions in which each face has one number and one symbol.
- The reusable Standard D6: 1-3 Sword, 4-5 Shield, 6 Gold Coin.
- Reusable card, status, Offensive ability, and Defensive ability definitions.
- Cross-reference validation among combatants, cards, dice, abilities, statuses,
  and symbols.
- A shared combatant model for player and enemy data, with enemy-only AI data.
- Separate Offensive and Defensive ability boards.
- Default five-`standard_d6` loadout for these combatants.
- One instantiated card equals one point of health in deck, hand, or discard;
  removed cards are missing health.
- Initial player/enemy composition and deterministic opening state matching the
  acceptance battle.

### Turn flow

- Round order: Ongoing Effects, Income, Offensive, Defensive, Damage Resolution.
- Segment Entry, Main, and Exit behavior and deterministic stage progression.
- The final action of every segment Exit checks whether the player or all enemies
  have been defeated and completes the battle when appropriate.
- Automatic progression continues until a human actor genuinely needs input.
- AI work never creates client input that must be submitted on behalf of an AI.

### Effects, proposals, and reactions

- Same-timing effects collect into one simultaneous proposal batch.
- Parent batches commit before any resulting child batch resolves.
- Content declares its timing, lifecycle, and whether it opens a reaction window;
  do not hardcode checks for specific IDs such as `poison` in segment flow code.
- Reaction windows use the existing pending-input/checkpoint model, reject stale
  input, and require explicit human pass or response where the rules require it.
- AI passes/responds automatically and deterministically according to its policy
  or the scripted test configuration.
- Resources are spent immediately when a play is accepted. Cards move to discard
  immediately when played unless content explicitly replaces that destination.
- Spent resources stay spent when an effect is canceled or collides, unless an
  explicit reusable rule says otherwise.
- Reveals still show what was played even though the card is already discarded.

### Statuses

- Status behavior is defined through reusable YAML fields/operations, not status
  name conditionals.
- Poison uses its persistent/outcome-based lifecycle and Ongoing Effects timing.
- Bleed behaves as defined by the example content and battle.
- Entangle is consumed at Offensive Entry, reduces maximum roll batches by one,
  opens no reaction window, and has no ordinary duration tick.
- Blind is consumed when its Offensive checkpoint is reached even if its target
  ultimately selects no ability; when an ability exists, Blind performs its roll
  after ability selection, uses its complete reaction window, then is removed.
- Cleanse/Antidote-style player-activated status removal works in legal windows.
- Removing a status during a reaction window does not erase trigger work already
  collected into the current batch.
- Add focused Go tests for settled status behavior not exercised by the four-round
  battle, especially Entangle and Blind.

### Income and resources

- Default Income draws one card and gains one energy.
- Energy accumulates and has no hard cap for this target rule set.
- Hand limit is six.
- Going over the limit is allowed temporarily; discard-to-six is enforced near
  the end of Damage Resolution if cards were not used first.

### Offensive

- A human player rolls its configured pool, keeps dice, rerolls only unkept dice,
  and has the configured maximum number of roll batches.
- Roll history preserves every batch, kept dice, final number, and final symbol.
- Ability qualification supports symbol counts, exact counts, number patterns,
  and independent conditional bonuses.
- Evaluate all qualified board abilities. A human chooses any qualified ability;
  never force the highest tier or strongest ability across the board.
- Selecting an ability and any required target auto-locks the default single
  Offensive ability selection.
- Ability operations can damage, apply/remove statuses, change resources, draw or
  remove cards, and support future reusable operation expansion.
- Battle-duration ability modifiers such as Sharpen Blade apply generically and
  clear after battle.
- The enemy uses its YAML D100 chart selected by its current maximum roll count.
  One D100 result selects the simulated roll band, ability, and reveal profile.
- Validate that every enemy chart covers 1-100 exactly once without gaps or
  overlaps.
- If a reaction changes enemy dice, reevaluate every enemy board ability. Keep the
  same ability if it remains valid; use the sole valid fallback; uniformly choose
  among multiple valid fallbacks; do nothing if none remain valid.
- Blind resolution occurs at the settled post-selection checkpoint.

### Defensive

- Defensive is selection-first and must not reuse Offensive roll-to-qualify flow.
- An actor with no incoming attacks auto-completes.
- If neither side has incoming attacks, the whole segment auto-completes.
- A defense may target one source or a group, according to content.
- A selected defense may own a nested die roll.
- Basic Defense rolls 1D6 and prevents the rolled face value from one selected
  incoming source.
- Protect scales one selected source as defined by content.
- Multiple incoming and counterattack sources remain distinct through Damage
  Resolution.

### Damage and defeat

- Damage is tracked as distinct sources.
- Prevention applies to its selected source, not automatically to total damage.
- Randomly select cards without replacement using zone priority:
  deck -> discard -> hand.
- Reveal the exact card instance IDs and definitions proposed for permanent loss,
  including their current zones and damage source, before removal.
- Revealed cards remain in their zones throughout the reaction window.
- Only after all required passes/responses and batch commit do those exact cards
  move to `removed`.
- If prevention reduces damage, release excess proposed removals without moving
  those cards.
- Overage is provisional and shown before commit; prevention consumes overage
  before saving proposed card removals.
- Status application can commit with damage but does not activate until its
  content-defined trigger point.
- Damage presentation always requires the human player's explicit pass when the
  rules say the result must be seen, even if no response card is available.
- Zero active health creates pending defeat; battle completion occurs only at the
  final segment-exit check after current work finishes.

## Determinism requirement

The acceptance battle must reproduce every scripted random result in the battle
document: opening/drawn cards, every player roll and reroll, Defensive dice,
Poison dice, enemy D100 values and reveal profiles, random damage-card choices,
and any fallback choice.

Use a clean deterministic injection mechanism at the domain boundary. It may be
a typed scripted random source or a more expressive test-only random plan. It
must:

- fail loudly on an unexpected call, bound, category, or exhausted script;
- prove at test end that every scripted value was consumed;
- avoid global mutable state;
- work through the real engine paths;
- not inject already-resolved proposals or directly mutate battle state to skip
  rules;
- keep production randomness behavior intact.

Because different random domains can interleave, prefer named/domain-specific
streams for combat dice, AI D100, status/effect dice, card shuffle/draw, damage
selection, and fallback selection rather than one fragile anonymous integer
list.

## Primary acceptance test

Add a clearly named Go integration test such as:

`TestAuthorityRunsBladeWardenVsVenomGoblinFullBattle`

Place it at the battle authority/integration level. The test must use:

- the real target server YAML content;
- the real content loader and participant assembly path;
- `repository.NewInMemory()`;
- a real `Authority` with the production engine plus deterministic dependencies;
- JSON commands passed directly to `Authority.HandleCommandJSON`;
- JSON-decoded `engine.Result` values returned by that boundary.

Do not call HTTP. Do not call curl. Do not bypass the authority by invoking flow
methods directly for the primary acceptance test.

The test acts like the human client:

1. Send `start_battle` as JSON.
2. Inspect the returned events, pending input, and snapshot.
3. Build the next JSON command using the actual returned pending-input ID and
   checkpoint fields; do not hardcode ephemeral IDs.
4. Send the command through `Authority.HandleCommandJSON`.
5. Decode and evaluate the response.
6. Confirm the persisted in-memory checkpoint agrees with the response.
7. Repeat for every roll, keep, reroll, card play, ability/target selection,
   Defensive selection/roll, reaction, pass, and damage acknowledgment until the
   complete four-round battle ends.

AI decisions and AI passes must happen inside the authority. The test must not
pretend to be an enemy client just to force the documented outcome.

### What the test must assert

Assert behavior after every command, not only the final snapshot:

- command accepted/rejected status as appropriate;
- exact currently allowed commands and human pending input;
- segment, Entry/Main/Exit stage, round, window, and reaction-round progression;
- ordered emitted domain events and important payload fields;
- every roll batch, kept die, rerolled die, final face, and symbol;
- qualified ability choices and selected ability/target;
- immediate energy spending and hand -> discard movement for played cards;
- public reveal data, including already-spent cards;
- enemy D100 selection, simulated roll band, reveal dice, and fallback result;
- each incoming damage source and selected Defensive response;
- exact proposed damage-card instances while they remain in their zones;
- response/pass ordering and commit timing;
- exact cards released by prevention and exact cards moved to `removed`;
- status stack application, trigger, consumption, and removal;
- state/event persistence and monotonically sequenced events;
- battle completion only after the final Damage Resolution Exit check.

Maintain an exact ordered high-level trace or checkpoint table in the test so a
missing, duplicated, or reordered interaction fails clearly. Prefer focused
assertion helpers and readable subtests/checkpoint labels over one unreadable
giant assertion.

The final state must exactly match the battle document, including:

```text
Battle result: victory
Completed rounds: 4

Blade Warden:
  deck: 8
  hand: 5
  discard: 3
  removed: 4
  active health: 16
  energy: 3
  statuses: none

Venom Goblin:
  deck: 0
  hand: 0
  discard: 0
  removed: 12
  active health: 0
```

Also assert that Sharpen Blade's battle-duration modifier is cleared when the
battle finishes and that the final scheduled Bleed never activates after battle
completion.

## Supporting tests

Do not rely on the one large integration test for all diagnosis. Add or update
focused tests for:

- YAML strictness and every new cross-reference;
- symbol catalog and one-symbol-per-face dice validation;
- combatant composition and card-instance identity;
- ability tier and conditional-bonus qualification;
- multiple valid human abilities remaining selectable;
- D100 chart coverage and roll-count selection;
- enemy requalification after die modification;
- immediate costs and discard movement;
- generic status timing/lifecycle, including Poison, Blind, and Entangle;
- selection-first Defensive behavior and nested defense rolls;
- exact damage reveal -> reaction -> commit zone transitions;
- overage reduction before card preservation;
- hand-limit enforcement timing;
- defeat checks at every segment Exit;
- deterministic random-script validation and exhaustion.

Preserve or intentionally update existing tests to the settled rules. Do not
delete meaningful assertions merely to make the suite green.

## Work process

1. Read the design references and current implementation completely enough to
   map the gaps.
2. Run the existing server suite before editing:
   `cd dice-and-destiny-server && go test ./...`
3. Write a concise implementation plan ordered by dependency and keep it updated.
4. Implement vertical slices, running focused tests after each meaningful slice.
5. Add the full-battle test early enough that it guides the remaining work, even
   if it initially fails at the first unsupported checkpoint.
6. Continue until the entire battle passes. Do not stop after scaffolding, schema
   changes, partial rounds, or a test that manually constructs the final state.
7. Format all Go code with `gofmt`.
8. Run the final verification commands from `dice-and-destiny-server/`:

   ```text
   go test ./...
   go test -race ./...
   go vet ./...
   ```

9. Run `git diff --check` from the repository root and verify no file outside
   `dice-and-destiny-server/` was changed by this task.
10. Review your own complete diff for rule gaps, hardcoded fixture shortcuts,
    nondeterminism, stale-input vulnerabilities, incorrect viewer exposure,
    package-boundary regressions, and insufficient assertions. Fix findings
    before reporting completion.

If you discover an apparent design contradiction, first reread the relevant
reference and inspect nearby settled decisions. Make the narrowest reasonable
interpretation that preserves the documented battle and generic rule model.
Only stop for user input if two materially different interpretations remain and
choosing one would require substantial rework. Record any non-obvious assumption
in the final report; do not edit the design documents.

## Required final response

Lead with whether the complete acceptance battle passes.

Then provide:

- a concise architecture/implementation summary;
- the important server files and packages changed;
- the exact full-battle test name and what it verifies;
- a short command-by-command/checkpoint summary proving the test traverses all
  four rounds through `Authority.HandleCommandJSON`;
- exact final battle state;
- the commands run and their pass/fail results;
- the outcome of your final diff self-review;
- any remaining known gap or assumption.

Do not claim completion if any required test is skipped, weakened, flaky, or
still failing. Do not include client or Godot work as a suggested prerequisite;
this task must be complete and reviewable entirely in the Go server engine.
