# Dice and Destiny Agent Instructions

## Godot workspace isolation

- Always run Godot from the repository root through `./scripts/godot.sh`.
- Never invoke this project with a raw `godot --path ...` command. Doing so bypasses per-workspace logs and per-process test-state isolation.
- Do not add fixed inspector ports, shared `/tmp` paths, or shared Godot `user://` development-save paths.
- Do not point `DICE_AND_DESTINY_*_ROOT` variables at another checkout. The launcher assigns roots for the current worktree automatically.
- A new worktree needs no manual setup. The launcher builds missing or stale native artifacts and runs the Godot import automatically: on a brand-new worktree, and whenever a checkout or pull brings an asset whose imported file this checkout has not built yet (a `.import` whose `dest_files` is missing).

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

- Defensive abilities and legacy cards configure `saved_card_destination` in their catalog definition; program cards (all shipped General and Adventurer cards) configure `destination` on each prevention effect. `original` preserves the live pile; `discard` moves newly saved cards to discard. Omission defaults to `original`. Never decide this by card/ability ID or upgrade name. See `docs/game-design/prevention-configuration.md`.
- In the Steady Guard tree, Steady Guard and Brace explicitly use discard; Bulwark uses original. Guard uses discard and Guard+ uses original. Reservations do not move cards, so original-pile prevention must not rewind a draw or play.
- A played prevention card goes to its normal play destination (usually discard), even if it saves itself. If it remains threatened by an unprotected source, removal still follows it into that destination. Protect retains its explicitly authored saved-to-discard status rule.
- Never duplicate cards. Healing is allowed: removed cards may return to a live pile through an explicitly authored heal effect (card, ability, status, relic, consumable or out-of-battle service) — never as an implicit fallback. Healing only restores cards that were removed; each healed card leaves the wound that removed it. Wounds are planned to carry over between battles in a run. The legacy revive-trade validation (a card had to remove at least as many cards as it revives) was dropped on 2026-10-16; Revive and Rebirth heal. Repeated reconciliation and save/reload must preserve pile membership and health.
- Publish the saved card's live destination for animations, including the played card's final destination after its effects resolve. Saved-card flights and pile counts must agree with authority. Any different destination requires an explicit effect.

## Default damage-card selection order

- For every actor and ordinary damage source, select cards from discard first, then draw pile, then hand. Exhaust each pile before sampling the next; never combine discard and draw into one random pool.
- Random selection stays without replacement within each pile. Preserve reveal-before-commit, prevention destination rules, health totals, and overage behavior. Different pile targeting requires an explicitly authored effect; do not infer exceptions from character or card names.

## Damage-prevention card targeting

- Start with the card click. If exactly one incoming source has remaining damage and a current legal card action, play against it immediately; do not require the player to preselect an attack.
- With multiple viable sources, select the card and highlight each eligible enemy attack badge on the battlefield. The badge is the only card target; its threatened-card list hangs beneath it, and the lower-left Incoming Attacks list is for defense selection only. Let the player choose the source next or cancel without spending. Count sources, not enemies; one enemy can have multiple attacks.
- Never target outgoing or fully prevented damage, reuse an earlier source selection, or submit stale target commands. Preserve the full ability tooltip and existing prevention/saved-card animations.

## Unified defense experiment

- New battles launched from the game menu use the pinned `unified_defense` rule. Legacy saves/replays retain their original flow. See `docs/game-design/unified-defense.md`.
- Reveal pending removals when attacks enter Defense, before rolling defenses. Each attack owns separate, stable card reservations; never pool attacks for source-specific prevention or display the same card in two attack lists.
- Rolled defensive abilities release cards in hand → draw → discard priority, randomly within each live pile. Cards and defensive abilities send released cards to their configured destination; statuses follow their explicitly authored rule above. Never undo a card play or draw when releasing a reservation.
- The main Pass ends that participant's remaining defenses and card opportunities. Applying a completed defensive roll returns to the same segment; it must not act as the main Pass. Auto-complete when only Pass remains, after the usual review time.
- An attack's queued statuses apply when the defense rolled against it resolves, still inside Defense, so "after" cards can interact with them; undefended attacks' statuses stay pending. Commit remaining damage and remaining queued statuses once at the end. Do not open a second damage-response phase or reselect the already revealed cards.
- Card timing is authored as Before / After / Any time for each segment's own turn (see `docs/game-design/card-configuration.md`). The offensive reaction and the defense roll review are separate opt-in reaction moments, off by default, because play pauses there whenever a card is playable; never add them to ordinary cards. The Ongoing Effects segment accepts no card plays.

## Campaign progression

- The menu's **Campaign** runs the battle → XP → deck change → next battle loop. Each campaign is a named save (`<loadout root>/campaigns/`) holding its own copy of a starting character sheet plus `bonus_xp`; never write campaign progress to the character template or the editors' progression saves, and never let them write to a campaign save. Encounters, XP rewards, and eligible (General-type) characters live in `dice-and-destiny-server/content/progression_v1/campaign.yaml`. See `docs/game-design/campaign.md`.
- Only the authority awards XP: the session records a campaign battle's result once, when it ends (victory only), in that campaign's save. Never compute or grant rewards from the client. Between battles the campaign uses its own Prepare screen (deck, abilities, tree trades that keep copies in the deck); keep Character Creation as the admin editor.
- Wounds carry over between campaign battles: after a victory the human's removed pile becomes the save's `wounds`, and the next battle starts with them in the removed pile (still counted in max health). Wounded copies are frozen until mended: they can't be played, sold, upgraded, traded down or stored, in either mode, and Prepare shows them disabled with the reason. Defeats and draws keep the pre-battle wounds, and clearing the campaign's last encounter mends them all.
- Campaign decks use only card-tree cards (bases, variants, shared cards). Legacy and standalone cards may be sold or stored but never added or fought with; the authority refuses a campaign battle whose deck holds one.

## Expedition

- New campaigns are **Expeditions** by default (`mode: expedition`): a seeded hex region explored tile by tile, with battles started from the map's piles. See `docs/game-design/expedition.md` and the implementation log `docs/game-design/expedition-progress.md`. Linear campaigns stay available and unchanged.
- **The map is authority state.** Every rule lives in `dice-and-destiny-server/internal/battle/expedition` and runs through the `expedition_status` / `expedition_act` / `expedition_deck_change` ops; the state is saved inside the campaign save. The Godot map screen (`app/screens/map/`) renders the viewer-safe `View` and submits actions; it never computes costs, outcomes, random draws or hidden contents. The view must never contain the tile stack, the core deal, the RNG position, the Warlord's route or unseen contents (`TestViewNeverLeaksHiddenState`), and its lists are never null.
- **Numbers live in configuration**, never in code: `content/progression_v1/expedition.yaml` (rounds, hand, Move, terrain by Day/Night, sight, Ambush, Fortified, Conqueror, defeat, camp healing, site mending, the Warlord, Influence and gold) and the region pack `content/expedition_v1/`. Loaders are strict; the shipped content must pass `go test ./internal/battle/expedition/`.
- **Battles start from the map**: the map hand (no opening draw), the map draw pile, Spent as discard, wounds as removed, the current energy; map cards are never legal in battle but count as health. Afterwards the piles come back as the battle left them but never better than they went in — worst to best (removed, Spent, draw, hand), each tier with the better ones holds at most what it held going in, the excess picked at random (seeded) and pushed down to the next pile with room, so wounds never shrink in battle — energy is min(end, brought in), the turn ends, and only the authority records the result once. Provoked fights never have Ambush; Ambush/Fortified/Conqueror/Lieutenant's Favor are visible statuses read by their `modifiers`, never by ID.
- **Keep these rules:** a turn needs at least one card played or discarded (a fight counts); a normal defeat restores the pre-battle piles and wounds plus one defeat wound of 2 damage (never the last live card), pushes back and forces a rest; a draw records nothing; losing to the Warlord ends the run; deck changes happen only at sites or camp for XP plus the configured Influence (cards played there that turn), paid once per visit: the first change opens the deck and further changes cost XP only until camp breaks or, at a site, the turn ends; gold comes only from the authority and never buys XP.
- **Wounds are groupings of damage** — the battle's wound ledger carried to the map: one wound per hit, defeat or event, holding the cards it removed; a wound healed to zero is gone. The view's `wounds` are the groupings (id, damage, cards, source, when taken, treated/bandaged) and `wound_cards` the health lost. Camp healing is basic aid 1d3 per untreated wound, then a bandage 1d6 per treated wound, once each per wound per camp, reset at the next camp; the numbers live in `camp_healing`; site mending heals one point per payment. Every rest (no cards left in hand or draw pile at the end of a turn, Make camp, defeat; never just because the draw pile is empty while cards remain in hand) reshuffles Spent, resets energy and draws the fresh hand; break camp does not draw.
- **Time is a world clock** (`clock` in the save, `expedition/clock.go`): one round is the Day or Night half of a month, the calendar is `calendar` in `expedition.yaml`, and each region names its start month in `calendar_start`. The round and Day/Night are derived from the clock and only the rest advances it; every time-based rule reads it, and it never uses the RNG. Seasonal effects are empty until Phase 4 and the loader refuses them.
- **Quests** are data in `dice-and-destiny-server/content/quests_v1/` (one folder per set, one quest per file), loaded strictly by `internal/battle/quest` and shown read-only by the `quest_catalog` op and Character Creation's Quests tab. They are description only until Phase 5: no expedition rule, timer, reward or view field reads them, and `expedition` never imports `quest`. Scenario quests refer to the scenario's numbers and content (config keys, group/site/event/choice ids) rather than copying them; see `docs/game-design/quest-format.md`.
- **Map cards** resolve in `expedition/map_cards.go`. Effects that outlast the play are visible map statuses or marks; map-only statuses (modifiers that only the map reads) never enter a battle. Energy can pay for Influence at the configured rate.
- **Tests to keep green:** `go test ./internal/battle/expedition/ ./internal/battle/learned/` (rules, ops, the bridge, the seeded full-region runs with save/reload checks) and `./scripts/godot.sh --headless --script res://tests/presentation/verify_expedition_map.gd` plus `verify_expedition_statuses.gd` (pointer). Seeds used by the region tests are fixed; if a rule or the proxy changes their outcome, find new seeds with the env-gated probes rather than weakening the assertions.

## Card pricing

- Price every General card, new or changed, with `docs/game-design/card-value-rules.md`. In those rules, 1 unit is 5 energy, 1 card drawn or 1 Move, and 1 health is 2 units. XP = 5 × (2 + net units), half points up, with a floor of 10, plus 1 XP for each 5 energy a card costs less than an equal-net card in its tree. A base card nets +1 unit and costs 15 XP.
- On the Expedition map the units differ: 2 Move = 1 unit, 5 energy = 1 unit, 1 Influence = 1 unit, and a card is priced on the better of its battle net and its map net (`card-value-rules.md` §5).
- Price a tree's variants relative to its pinned base. When a base's energy or effect changes, its upgrades shift with it, except that free (Swift) nodes stay free.
- Keep the doc's tables in step with any change to the published trees.
- To measure cards in real battles, or after changing a card's energy, effect or XP, the energy economy, the AI or the opponents, follow `docs/game-design/balancing-playbook.md`: what triggers a balancing pass, which test answers which question, the commands, and the current measured prices.

## Authored content storage

- Published cards, card trees, abilities/boards, and admin card prices and pool types live in the tracked `dice-and-destiny-server/content/authored/` directory. The launcher points `DICE_AND_DESTINY_AUTHORED_ROOT` at it for normal runs; commit its changes to share them.
- Player decks, progression, collections, and admin budgets stay in the uncommitted loadout root.
- Script tests receive an empty disposable authored root and must never write to the tracked directory.
