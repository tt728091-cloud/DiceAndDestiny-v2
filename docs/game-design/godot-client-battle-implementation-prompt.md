# Prompt: Implement the Playable Godot Battle Client

Work in this repository:

`/Users/daddymere/games/Dice-and-Destiny-v2`

Your task is to implement a focused Godot client that launches directly into a
real Blade Warden versus Venom Goblin battle and lets a human play the battle to
completion through the completed Go battle authority.

This is not a menu, campaign, matchmaking, or general game-shell task. The
entire proof is one playable battle, launched immediately when the Godot project
starts.

## Non-negotiable scope boundary

Make source changes only inside:

`/Users/daddymere/games/Dice-and-Destiny-v2/dice-and-destiny-client/`

Treat the completed Go server as read-only. Do not modify Go rules, Go tests,
server YAML, server adapters, or server save files.

You may run the existing native build script and allow it to rebuild/copy
generated native libraries needed by the client:

`/Users/daddymere/games/Dice-and-Destiny-v2/dice-and-destiny-server/scripts/build_native.sh`

Running that build is allowed; editing server source is not. Do not commit
generated server build output. Preserve every pre-existing server change in the
working tree exactly as found.

Do not modify:

- `dice-and-destiny-server/` source or content
- `docs/`
- Go battle behavior
- the GDExtension C++ or Go bridge source
- unrelated repository directories

If the existing JSON/GDExtension boundary genuinely lacks information or a
command required for the playable battle, stop and report the precise missing
field/command and the exact UI state it blocks. Do not silently expand scope
into the Go server.

## Read these references first

Read these files completely before planning or editing:

### Game and battle behavior

1. `/Users/daddymere/games/Dice-and-Destiny-v2/docs/game-design/turn-structure.md`
2. `/Users/daddymere/games/Dice-and-Destiny-v2/docs/game-design/examples/full-battles/blade_warden_vs_venom_goblin.md`
3. `/Users/daddymere/games/Dice-and-Destiny-v2/docs/game-design/examples/content-library/`
4. `/Users/daddymere/games/Dice-and-Destiny-v2/dice-and-destiny-server/content/battle_v1/`
5. `/Users/daddymere/games/Dice-and-Destiny-v2/dice-and-destiny-server/internal/battle/full_battle_authority_test.go`

The full Go test is especially useful because it demonstrates the actual JSON
command sequence, pending-input checkpoints, card instance IDs, source IDs,
reaction payloads, and snapshot fields accepted by the finished authority.

### Current client architecture

Read:

- `dice-and-destiny-client/project.godot`
- `dice-and-destiny-client/app/screens/battle/`
- `dice-and-destiny-client/app/screens/scenarios/`
- `dice-and-destiny-client/local_client/battle_gateway/`
- `dice-and-destiny-client/local_client/command_builder/`
- `dice-and-destiny-client/local_client/view_state/`
- `dice-and-destiny-client/scripts/battle_authority.gd`
- `dice-and-destiny-client/scripts/go_gdextension_battle_authority.gd`
- `dice-and-destiny-client/native/battle_authority.gdextension`
- `dice-and-destiny-client/tests/`
- `dice-and-destiny-server/adapters/gdextension/README.md`

Preserve the existing boundary:

```text
Godot presentation
-> client command builder
-> BattleGateway
-> BattleAuthority interface
-> NativeBattleAuthority GDExtension
-> Go HandleCommandJSON
-> events + snapshot + pending_input
-> client view state
-> Godot presentation
```

Godot owns presentation and user intent. Go remains the only authority for
legal commands, randomness, card zones, dice outcomes, ability qualification,
damage, statuses, AI behavior, segment progression, and victory/defeat.

## Confirmed visual references

Use these exact images as visual references for the corresponding states:

| UI state | Reference image |
| --- | --- |
| Shared battle shell | `docs/ui-mockups/battle-concept-27-mirrored-player-profile.png` |
| Effects | `docs/ui-mockups/battle-concept-33-effects-poison-roll.png` |
| Income | `docs/ui-mockups/battle-concept-34-income-card-draw-energy.png` |
| Offensive before roll/selection | `docs/ui-mockups/battle-concept-28-offensive-pre-ability-select.png` |
| Offensive after selection/reveal | `docs/ui-mockups/battle-concept-29-offensive-post-ability-select.png` |
| Defensive before selection | `docs/ui-mockups/battle-concept-30-defensive-pre-ability-select.png` |
| Defensive after selection/roll | `docs/ui-mockups/battle-concept-31-defensive-post-ability-select.png` |
| Damage pending removals | `docs/ui-mockups/battle-concept-32-damage-pending-removal.png` |

Also read the matching notes:

- `docs/ui-mockups/battle-concept-27-godot-ui-notes.md`
- `docs/ui-mockups/battle-segment-ui-reference-index.md`
- every `docs/ui-mockups/battle-segment-*-ui-notes.md` file

These PNGs are reference screenshots, not shippable UI textures. Do not place a
full screenshot behind invisible buttons, slice copyrighted-looking figures
out of the mockups, or fake functionality with static imagery. Build the layout
with real Godot controls and reusable components.

The target is the same general shape and visual hierarchy, not a pixel-perfect
reproduction. Favor clear interaction and state accuracy over ornamental detail.

Use a responsive 16:9 desktop layout with a 1920x1080 design baseline that
remains usable at 1280x720.

## Image and visual asset plan

Create a coherent, original dark-fantasy asset set inside the client, organized
under a clear directory such as:

`dice-and-destiny-client/assets/battle/`

Use image generation when available for the bitmap illustrations. Keep generated
assets original and stylistically consistent. Do not search for or copy
copyrighted game artwork.

### Required bitmap illustrations

1. Blade Warden portrait
   - human fighter/blade specialist
   - cool blue accent lighting
   - transparent or easily masked background
2. Venom Goblin portrait
   - poisonous goblin combatant
   - sickly green with warm enemy/orange accents
   - transparent or easily masked background
3. Shared battle-board background
   - dark stone/metal tabletop
   - subtle central rune or circular combat sigil
   - low visual noise behind controls
4. One card-back illustration
5. Six reusable card illustrations, one per definition:
   - Tip It
   - Loaded Die
   - Antidote
   - Sharpen Blade
   - Emergency Ward
   - Battle Focus

Portraits and the shared background are higher priority than elaborate card art.
If time or generation constraints require simplification, use attractive
color-block/icon card illustrations, but do not leave the interaction unclear.

### Icons and Godot-drawn visuals

Prefer small original SVGs, Godot drawing, labels, theme resources, and reusable
styled controls for:

- Sword, Shield, and Gold Coin symbols
- Poison, Bleed, Entangle, and Blind statuses
- Energy
- Deck, Hand, Discard, and Removed zones
- the ten unique equipped abilities across both actors
- die faces and keep/selected states
- pending-removal slash/crack overlay
- targeting, damage, prevention, and pass/continue indicators
- victory and defeat emblems

Do not generate separate full-screen art for every segment. All segment screens
reuse the shared shell; layout and center content change from state.

## Launch behavior

Change the Godot project startup so running the project goes directly into this
battle. Do not show the scenario launcher, title screen, character selection, or
other menus.

The battle must be started through the real authority using a normal
`start_battle` JSON command with:

```json
{
  "battle_id": "<generated-collision-safe-id>",
  "actor_id": "blade",
  "type": "start_battle",
  "payload": {
    "player": {
      "instance_id": "blade",
      "definition_id": "blade_warden"
    },
    "enemies": [
      {
        "instance_id": "goblin",
        "definition_id": "venom_goblin"
      }
    ]
  }
}
```

Generate a collision-safe battle ID for each genuinely new battle rather than
reusing a constant ID that can conflict with persisted server state.

It is acceptable—and recommended—to reopen a locally stored active battle on
restart if `open_battle` succeeds. If reopening fails, clear the stale local
record and start a new battle immediately. On victory or defeat, clear the
active-battle record and offer a simple `Play Again` action that starts another
real battle.

Use `blade` as the viewer actor ID throughout this battle. Do not retain the old
hardcoded assumption that the viewer is always named `player`.

## Randomness policy

The playable application must use normal production server randomness.

- Do not hardcode the four-round walkthrough outcomes.
- Do not force the scripted Go integration-test random sequence.
- Do not put random gameplay logic in GDScript.
- Do not locally roll dice and tell the server what was rolled.
- Do not predict enemy D100 choices.
- Render whatever legal results the authority returns.

Automated client presentation tests may use deterministic fake authority results
or an existing deterministic server configuration, but production gameplay must
always call the real native authority.

If the read-only server exposes no deterministic full-battle mode through the
native boundary, do not change the server to create one. Use:

- deterministic fake-authority fixtures for UI/presentation tests; and
- a real-native-authority smoke test that asserts structure and legal
  progression without asserting particular random faces/cards.

The fake authority must implement the existing `BattleAuthority` interface and
must only be injected into tests. It may not be used by the playable scene.

## Content presentation ownership

The server snapshot exposes stable definition IDs and authoritative instance
state, but it does not necessarily expose every display name, rules paragraph,
icon key, or ability recipe needed for a polished UI.

Create a client-owned presentation catalog keyed by the exact server IDs for the
single `battle_v1` content set. It may be JSON, `.tres` resources, or typed
GDScript data.

The catalog may contain only presentation metadata such as:

- display name
- concise rules text
- icon/art path
- color/accent
- formatted activation recipe
- card energy-cost display
- status explanation

Do not use the presentation catalog to decide whether a command is legal,
calculate damage, qualify an ability, spend energy, change zones, or resolve a
status. Enable actions only from the server's `pending_input`,
`allowed_commands`, current snapshot, qualified IDs, sources, and revealed
proposals.

Support these IDs:

### Player cards

- `tip_it`
- `loaded_die`
- `antidote`
- `sharpen_blade`
- `emergency_ward`
- `battle_focus`

### Player abilities

- Offensive: `sword_cut`, `shield_bash`, `golden_edge`, `perfect_form`
- Defensive: `basic_defense`, `protect`

### Enemy abilities

- Offensive: `jagged_slash`, `venom_strike`, `crushing_advance`, `greedy_blow`
- Defensive: `basic_defense`, `protect`

### Statuses and symbols

- Statuses: `poison`, `bleed`, `entangle`, `blind`
- Symbols: `sword`, `shield`, `gold_coin`

Use the server YAML for accurate names, costs, recipes, and rules text.

## Required client architecture

Replace the current debug-label battle screen with reusable presentation
components and a small coordinator. Keep transport, view-state translation,
presentation sequencing, and controls separate enough to test.

Recommended responsibility split:

```text
Battle bootstrap
  starts/reopens the one battle and hands the first result to the screen

Battle gateway
  submits JSON and validates that the native response is a dictionary

Command builder
  builds exact typed payload shapes from current pending-input checkpoints

Battle view state
  translates snapshots/events into presentation-safe state

Presentation director
  queues ordered returned events, presents automatic segments, then exposes the
  current pending-input controls

Battle screen
  composes reusable actor, dice, ability, hand, status, damage, and log views
```

Do not put every rule, JSON payload, layout node, and presentation transition in
one giant `battle_screen.gd` file.

## Critical event-presentation behavior

The Go authority automatically progresses until the human needs input. A single
response can therefore contain ordered events for several automatic stages or
segments—for example:

```text
Damage acknowledgment
-> damage commits
-> Ongoing Effects resolves
-> Income draws and grants energy
-> Offensive opens and returns a planning wait
```

The returned snapshot represents the final authoritative state after all those
events. If the client renders only that snapshot, Effects and Income will appear
to be skipped.

Implement an event-presentation queue:

1. Receive one authority result.
2. Preserve event sequence order.
3. Queue concise presentation beats for meaningful events.
4. Temporarily disable new authority commands while those beats play.
5. Show Effects, Income, damage commits, status changes, and battle completion
   in their corresponding center panels.
6. Apply/render the final snapshot and expose its pending input after the queue
   drains.

Animations may be simple. A short timed transition or a user-facing `Continue`
button for presentation-only pacing is acceptable. A presentation Continue must
not send a gameplay command unless the server is actually waiting for `pass`.
Make the difference visually and architecturally clear.

Never reorder server events. Track event sequence numbers and do not replay an
already-presented event when reopening a battle.

## Shared battle screen

Build the shell from the confirmed concept 27 reference:

### Top

- Effects, Income, Offensive, Defensive, Damage segment bar
- active-segment highlight
- round and current stage label

### Left/player column

- Deck, Hand, Discard, Removed counts
- Blade Warden portrait, name, current/max health, energy
- active statuses with stack counts
- player dice tray and roll controls when relevant

### Center

- segment-specific presentation area
- enemy ability row near the top when relevant
- player ability row near the bottom when relevant
- selected attack/defense detail panels
- pending damage-card rows
- hand along the bottom

### Right/enemy column

- one-enemy roster containing Venom Goblin
- Venom Goblin portrait, current/max health, energy, statuses
- revealed enemy dice and selected ability when public
- concise combat/event log

The enemy's hidden hand must remain hidden. Display only its server-provided
hand count unless exact cards are publicly revealed for damage.

## Required interactive states

Drive every state from `snapshot.stage`, `snapshot.segment`,
`pending_input["blade"]`, its `source_type/source_id`, and
`allowed_commands`. Do not infer legality from the reference walkthrough.

### Offensive planning

Use the Offensive pre-select reference.

- Before the first roll, show five blank/ready dice slots and a Roll button only
  when `planning_roll` is allowed.
- After a roll, render the returned face number and symbol for all five dice.
- Clicking a die toggles its keep selection.
- Submit kept indices through `planning_keep`.
- Reroll only unkept/selected indices through `planning_reroll`.
- Show rolls used and remaining from authoritative roll state/history.
- Highlight every ID in `qualified_abilities`.
- Keep all qualified abilities selectable; do not auto-select the strongest.
- Selecting an ability and the only enemy target submits
  `planning_select_ability` with `goblin` as the target.
- Provide `planning_pass` only when allowed.

### Planning cards

Render the viewer's actual hand instance IDs using `card_instances` to find each
definition ID.

Only make a card playable when the current pending input includes the appropriate
command and the card has a meaningful target in the current UI state.

Support:

- Battle Focus: play from hand during Offensive planning.
- Sharpen Blade: choose one owned Offensive ability.
- Loaded Die: choose one owned rolled die.
- Antidote: choose one negative self status during a legal status reaction.
- Tip It: during Offensive reaction, choose one revealed enemy face-6 die and
  submit the required planning adjustment to face 5.
- Emergency Ward: during damage response, choose one incoming damage source.

Played cards must disappear from hand immediately when the returned snapshot
moves them to discard. Do not wait for a later reveal to update the hand.

### Offensive reveal/reaction

Use the Offensive post-select reference.

- Show both actors' public final dice.
- Highlight selected abilities.
- Show selected targets and concise effect text.
- Show the enemy's D100/simulated-roll information only if the viewer-safe
  response exposes it.
- Offer legal reaction cards such as Tip It.
- Always expose the real `pass` action when required.
- After a reaction, update enemy dice and selected/fallback ability from the new
  snapshot instead of calculating fallback locally.

### Defensive selection

Use the Defensive pre-select reference.

- Show every incoming damage source separately.
- Show source actor, source ability, base/final amount, target, and statuses.
- Render `basic_defense` and `protect` from the player's Defensive board.
- Let the player select a defense and one incoming source.
- Submit `planning_select_ability` using the selected source ID as the target.
- Support Defensive pass when the authority allows it.
- Auto-completed defense states should be presented briefly but must not create
  fake controls.

### Defensive roll/reaction

Use the Defensive post-select reference.

- Show the selected defense beside the incoming attack.
- When `roll_dice` is the only allowed command, label it as the defense's owned
  roll—not an Offensive roll.
- Display the returned Basic Defense 1D6 result.
- Show resulting prevention/final damage from the returned source state.
- Require the real server pass when the defense reaction window is open.

### Effects

Use the Effects reference.

- Present Poison, Bleed, Blind, and Entangle from server events/state.
- Poison uses its own effect-die presentation, not the five Offensive dice.
- Show every collected Poison roll and its outcome.
- Offer Antidote when legal and a real pass when required.
- If a status is removed during the window, continue displaying already-collected
  work until the server advances it.
- Show Bleed damage and stack removal.
- Show Entangle reducing the available Offensive rolls when it occurs.
- Show Blind's die and reaction window at its server-defined checkpoint.

### Income

Use the Income reference.

- Present the exact viewer card instance drawn when exposed by `cards_drawn`.
- Show the card moving conceptually from deck to hand.
- Show the energy increase.
- Then transition into the next returned state.
- Do not send a command for automatic Income.

### Damage Resolution

Use the Damage reference.

- Show exact proposed removal cards for player and enemy in separate rows.
- Resolve each removal's instance/definition, original zone, and damage source.
- Keep cards visually in their current zones while the reaction window is open.
- Clearly mark them as pending, not already removed.
- Show overage.
- Offer Emergency Ward and source selection when legal.
- After prevention, visually release any removals marked released/unaccepted.
- Commit/move cards only after the authority returns committed state/events.
- Require the server's real pass/acknowledgment.

### Hand-limit choice

- When the pending purpose/stage requires choosing cards, show the number that
  must be discarded to reach six.
- Allow exactly that number of hand cards to be selected.
- Submit their instance IDs through `commit_interaction`.

### Battle completion

- Show a clear Victory or Defeat overlay from `battle_result`/snapshot status.
- Show final health and card-zone counts.
- Disable all battle commands.
- Clear the active battle record.
- Provide `Play Again`, which starts a new real-random battle.

## Command builder requirements

The existing generic pending-command builder is insufficient for this battle.
Extend the client command-builder layer with explicit builders for:

- `start_battle`
- `open_battle`
- `planning_roll`
- `planning_keep`
- `planning_reroll`
- `planning_commit_cards`
- `planning_select_ability`
- `planning_select_targets`
- `planning_pass`
- `roll_dice`
- `commit_interaction`
- `pass`

Every command must copy the current pending input's:

- `id` as `pending_input_id`
- `window_id`
- `segment` where required
- `stage`
- `iteration`
- `reaction_round`
- `planning_cycle`
- `source_id` as a roll request ID where required

Never reuse a checkpoint after a response rotates pending input. Disable a
control immediately after submission until the result returns, and build the
next command only from the new result.

Use the payload structs and helpers demonstrated in
`full_battle_authority_test.go` as the contract. Do not invent alternate JSON.

## Error handling

- Show a readable in-screen error panel when the native class is unavailable,
  JSON is invalid, content cannot load, a battle cannot start/reopen, or a
  command is rejected.
- Keep the last valid UI state visible when a command fails.
- Prevent double submission.
- Include a development-only expandable raw error/result area if useful.
- Do not automatically retry a rejected gameplay command with guessed data.

## Testing requirements

Create focused Godot-side tests without changing the Go server.

### Deterministic command-builder tests

Assert exact JSON payloads for every supported command, including current
pending IDs/checkpoints, card instance IDs, source IDs, target IDs, kept/reroll
indices, die adjustments, and hand-limit choices.

### Deterministic view-state tests

Use representative viewer-safe results for every required stage:

- Offensive before roll
- Offensive after each roll/keep/reroll
- Offensive qualified ability selection
- Offensive reveal/reaction
- Defensive selection
- Defensive owned roll
- Defensive reaction
- Poison/status reaction
- Income presentation events
- Damage pending removal
- Damage after prevention/released removals
- Hand-limit choice
- Victory

Verify hidden enemy cards are never rendered.

### Deterministic presentation tests

Inject a fake `BattleAuthority` through the existing interface and verify:

- the project coordinator consumes ordered results;
- event beats are shown in sequence;
- controls match `allowed_commands`;
- control actions emit the expected JSON;
- stale controls are disabled after submission;
- automatic Effects/Income presentations do not send gameplay commands;
- the final pending state appears only after queued event presentations drain.

### Real native-authority smoke test

Rebuild the native library and run a headless Godot smoke test through the real
GDExtension. It must:

1. start Blade Warden versus Venom Goblin using `start_battle`;
2. confirm a real snapshot and Offensive planning pending input;
3. send at least an initial legal `planning_roll` using returned checkpoint data;
4. verify five returned dice, each with a number and symbol;
5. verify the result remains accepted and persisted/openable;
6. avoid asserting exact random faces or card draws.

If feasible without adding server hooks, continue this structural smoke test
through additional legal waits. Do not make it flaky by assuming random ability
qualification.

### Scene/headless verification

Run the battle scene headlessly and ensure:

- no GDScript parse/runtime errors;
- required reusable components instantiate;
- the layout fits the configured viewport;
- every fixture state can render;
- all required controls have accessible text/tooltips;
- project startup points directly to the battle bootstrap.

## Manual playable verification

After automated checks pass:

1. rebuild the native library;
2. launch the Godot project normally;
3. confirm it enters Blade Warden versus Venom Goblin directly;
4. play through multiple rounds using real random outcomes;
5. exercise rolling, keeping/rerolling, ability selection, at least one playable
   card when drawn/legal, defense selection/roll, damage acknowledgment, and
   automatic segment presentations;
6. continue to victory or defeat if practical;
7. inspect the UI at 1920x1080 and 1280x720;
8. capture screenshots of the implemented states reached during verification.

Do not claim the UI is fully playable if any pending-input type can leave the
battle stuck without a corresponding control.

## Work process

1. Inspect `git status` and preserve all existing changes. Do not reset, clean,
   reformat, stage, or alter server work.
2. Read all required references and map server stages/commands/snapshot fields to
   client views.
3. Run the existing Go test only as a read-only verification if useful; do not
   change it.
4. Run the existing Godot tests before editing.
5. Present a concise implementation plan and asset plan.
6. Implement a thin vertical slice first:
   direct launch -> real start_battle -> Offensive pre-roll -> real roll -> render
   returned dice.
7. Expand through every pending state and automatic presentation beat.
8. Keep scenes/components reusable and presentation-only.
9. Generate/add the agreed original bitmap assets and wire them into client
   presentation resources.
10. Run focused tests continuously.
11. Rebuild the native library and complete automated plus manual verification.
12. Run `git diff --check` and confirm the task changed only
    `dice-and-destiny-client/`.
13. Review the entire diff for hardcoded outcomes, duplicated authority logic,
    stale pending IDs, hidden-information leaks, skipped automatic segments,
    dead-end waits, and monolithic UI code. Fix all findings before reporting.

## Acceptance criteria

The task is complete only when:

- running the Godot project enters the real battle directly;
- the battle is Blade Warden versus Venom Goblin;
- production play uses real Go randomness;
- all gameplay commands travel through the real GDExtension authority;
- all human pending-input types produced by this content have functional UI;
- player dice support roll, keep, and reroll;
- every qualified Offensive ability remains selectable;
- cards can be played with their required UI targets in legal windows;
- enemy reveal and fallback changes are rendered from server results;
- Defensive selection and owned defense rolls work;
- Effects and Income are visibly presented even when the server auto-advances;
- damage reveals exact pending cards before acknowledgment and removal;
- health, zones, energy, statuses, dice, abilities, and logs stay synchronized
  with viewer-safe snapshots/events;
- victory/defeat and Play Again work;
- hidden enemy information is never exposed;
- deterministic client tests and real native smoke tests pass;
- the client remains usable at both target resolutions;
- no server or documentation source was modified.

## Required final response

Lead with whether the battle is playable end to end through the real Go
authority.

Then report:

- the implemented client architecture;
- the main scenes/scripts/resources/assets added or changed;
- which bitmap assets were generated and where they live;
- the mapping from every server stage/pending command to its UI control;
- how automatic event presentation prevents Effects/Income from being skipped;
- how production randomness differs from deterministic UI tests;
- automated commands run and exact results;
- manual playthrough states exercised;
- screenshots captured;
- final diff/self-review findings;
- any remaining blocker or unimplemented pending-input type.

Do not claim completion if the UI uses a fake authority in production, hardcodes
the documented battle outcomes, bypasses pending checkpoints, exposes enemy
hidden cards, visually skips automatic segments, or cannot progress through a
server wait.

Begin by reading all references, inspecting the current dirty worktree without
changing server files, running baseline client checks, and presenting the
implementation/asset plan. Then implement the client-only task through verified
playability.
