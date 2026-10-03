# Dice and Destiny Agent Instructions

## Godot workspace isolation

- Always run Godot from the repository root through `./scripts/godot.sh`.
- Never invoke this project with a raw `godot --path ...` command. Doing so bypasses per-workspace logs and per-process test-state isolation.
- Do not add fixed inspector ports, shared `/tmp` paths, or shared Godot `user://` development-save paths.
- Do not point `DICE_AND_DESTINY_*_ROOT` variables at another checkout. The launcher assigns roots for the current worktree automatically.
- A new worktree needs no manual setup. The launcher builds missing or stale native artifacts and performs the initial Godot import automatically.

Standard commands:

```bash
# Run the game
./scripts/godot.sh

# Open the editor
./scripts/godot.sh --editor

# Run a Godot script test with disposable process-local state
./scripts/godot.sh --headless --script res://path/to/test.gd

# Run with the debug inspector
DICE_AND_DESTINY_INSPECTOR=1 ./scripts/godot.sh

# Connect to the inspector for this worktree; no port or token is required
python3 dice-and-destiny-client/devtools/inspect_game.py health
```

Normal development state belongs beneath `dice-and-destiny-client/.godot/runtime/` and must remain uncommitted. Scripted Godot tests receive temporary state that the launcher removes when the process exits.

## Developer history, snapshots, and fresh battles

- Developer history and snapshot tooling are runtime opt-ins. Set both flags before starting Godot; setting them after the process starts does not enable the native authority features.
- Pass the flags to `./scripts/godot.sh`. Never replace the launcher with a raw `godot --path ...` invocation.
- The persistent active-battle pointer for a normal workspace run is `dice-and-destiny-client/.godot/runtime/workspace/client/user/active_battle.json`.
- To start a fresh battle, delete only that active-battle pointer. This intentionally preserves developer snapshots, history timelines, and prior server battle records.
- Do not use a broad `find ... -name active_battle.json -delete` beneath `.godot/runtime/`; it can modify disposable state owned by concurrently running script tests in the same worktree.
- Do not delete the workspace `server/snapshots` or `server/history` directories unless the user explicitly requests destructive removal of saved developer diagnostics.

Run with developer history and snapshots:

```bash
DICE_AND_DESTINY_ENABLE_HISTORY=1 \
DICE_AND_DESTINY_ENABLE_SNAPSHOTS=1 \
./scripts/godot.sh
```

Start a fresh battle while preserving saved snapshots and history:

```bash
rm -f dice-and-destiny-client/.godot/runtime/workspace/client/user/active_battle.json

DICE_AND_DESTINY_ENABLE_HISTORY=1 \
DICE_AND_DESTINY_ENABLE_SNAPSHOTS=1 \
./scripts/godot.sh
```

Run the same developer configuration with the debug inspector:

```bash
DICE_AND_DESTINY_ENABLE_HISTORY=1 \
DICE_AND_DESTINY_ENABLE_SNAPSHOTS=1 \
DICE_AND_DESTINY_INSPECTOR=1 \
./scripts/godot.sh
```

## Native builds

- Build native Go/C++ artifacts with `dice-and-destiny-server/scripts/build_native.sh`.
- Do not copy native artifacts from another worktree. Each worktree builds and loads its own ignored artifacts.
- The native build script serializes duplicate builds inside one worktree; builds in separate worktrees may run concurrently.

## Verification

Run checks relevant to the change. The standard authority checks are:

```bash
cd dice-and-destiny-server && go test ./...
cd .. && ./scripts/godot.sh --headless --script res://scripts/verify_battle_authority.gd
```

Use `./scripts/godot.sh` for every additional Godot test.

## Git worktrees

- Start parallel Codex tasks in separate Worktree environments based on the intended baseline branch.
- Keep each task on its own branch before committing or pushing; Git cannot check out the same branch in multiple worktrees simultaneously.
- Do not alter or clean another worktree's `.godot`, build, save, or runtime directories.

## Ability hover contract

- Every revealed attack intent, for player and enemies, must expose the selected ability's name, qualification/tier, full rules, revealed offensive dice, and current attack result on hover. Use the battle's pinned catalog and viewer-safe authority state; never infer the selected tier from damage or expose unrevealed enemy choices.
- Author `presentation.rules_text` for new abilities with their dice-to-damage/effect calculation, miss conditions, and conditional effects. Keep structured tier requirements and operations authoritative. Use the shared attack tooltip presenter, including over intent symbols; do not replace it with a short defense-selection hint.
- Keep pop-ups wrapped and inside the viewport for 1–4 enemies. Validate offensive reaction and defense selection, live dice edits, selected tiers, and both screen edges. See `docs/game-design/battle-hud-anchors.md` for the UI contract.

## Mandatory visible status effects

- Any new buff, debuff, stored protection, charge, or other ongoing combat effect that can be consumed or expires MUST be represented as an authoritative status effect on the affected actor. Never implement it only as a hidden runtime counter, boolean, or button.
- Author its polarity, stack/cap rules, consumption conditions, and exact expiration checkpoint in the content catalog. Show its icon/count and full rules on the character's status bar, including when and how it expires.
- Actions that spend the effect must read and consume that same status instance. Offer a clearly labeled, reachable control at the applicable reaction/defense step; a button alone is not a substitute for the status.
- Verify acquisition, public status display/hover, use against the selected target, prevention of repeat use, persistence through saves, and unused expiration through the real phase transition. Validate pointer input as well as authority commands, including manual Pass and automatic progression after the choice is resolved.

## Default draw and discard rule

- Ordinary draws must never shuffle or move discard back into the draw pile, for any character or draw source (opening hand, Income, cards, or abilities). Draw only the available deck cards; an empty/short deck resolves without blocking progression or losing health.
- Cards in deck, hand, and discard all count as health. Only removal loses health. Discard remains eligible for damage removal.
- Returning discarded cards to the draw pile requires an explicitly authored recovery effect; never add implicit recycling as a fallback. Keep pile tooltips and regression tests consistent with this rule.

## Default damage prevention destination

- A revealed damage card saved by prevention moves from its current zone to discard immediately when released. This preserves health but does not make it available to draw. Apply this shared default to every damage-prevention card and status, including Brace and Protect.
- Keep already-discarded cards in discard without duplication, follow cards played from hand to their live zone, and never move a permanently removed card back. Repeated reconciliation must not move a saved card twice.
- Publish the saved card's destination for animations; saved-card flights and pile counts must agree with authority. A future upgraded effect returning cards to deck must explicitly author that exception.

## Default damage-card selection order

- For every actor and ordinary damage source, select cards from discard first, then draw pile, then hand. Exhaust each pile before sampling the next; never combine discard and draw into one random pool.
- Random selection stays without replacement within each pile. Preserve reveal-before-commit, prevention-to-discard, health totals, and overage behavior. Different pile targeting requires an explicitly authored effect; do not infer exceptions from character or card names.

## Damage-prevention card targeting

- Start with the card click. If exactly one incoming source has remaining damage and a current legal card action, play against it immediately; do not require the player to preselect an attack.
- With multiple viable sources, select the card and highlight each eligible attack and its damage-list control. Let the player choose the source next or cancel without spending. Count sources, not enemies; one enemy can have multiple attacks.
- Never target outgoing or fully prevented damage, reuse an earlier source selection, or submit stale target commands. Preserve the full ability tooltip and existing prevention/saved-card animations.
