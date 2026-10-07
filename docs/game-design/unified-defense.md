# Unified Defense experiment

Baseline: `de0ed80` on `codex/battle-ui-and-gameplay-update`.
Experiment: `codex/unified-defense-experiment`.

## Player flow

Effects → Income → Offensive → Defense → next round.

When attacks are finalized, Defense immediately displays each incoming attack,
its pending status effects, and its own threatened-card list under the defender.
The list retains pile-origin icons and the existing hand-sized card hover preview.
Card identities are revealed in this first version. Hidden identities and
foreknowledge upgrades are deferred.

The player can play a prevention card before rolling a defense or between
completed defenses. Card targeting remains card-first: one viable source is
automatic; multiple sources highlight choices. Defense selection still targets
one attack, pays its existing cost, and follows its existing usage rules.

A defense roll retains its atomic reaction/animation checkpoint. It finishes
automatically after the review animation and returns to the same Defense screen;
there is no Apply button, even when automatic passing is disabled. Reaction cards
can be used during review, and selecting a card target pauses completion. Cards
and unused defenses remain available in the Defense hub. **Pass** finishes all of
that participant's remaining choices, including unused defenses and cards. It
does not take away another participant's turn. When Pass is the sole legal action,
the normal automatic review-and-pass behavior finishes the segment.

Damage reduction and released-card flights share one feedback clock after the
roll finalizes. The roll preview holds the previous damage amount until the
actual saved cards are known. Only the affected attack's list uses temporary
headers; their text keeps its natural height, and the list reserves its space
until the saved cards leave. Other attack lists remain live throughout.

Each attack's queued statuses land as soon as the defense rolled against it
resolves, still inside Defense, so later "after" cards (status removal and so
on) see them; the snapshot keeps those applications marked `applied` and
pending displays drop them. Attacks that are never defended keep their statuses
pending. After all participants finish, remaining removals and the remaining
queued statuses commit once. Normal hand-limit cleanup, defeat checks, and authored follow-up
work still run. The engine does not open a second Damage segment.

## Card ownership and prevention

- Each damage source reserves distinct card instances, selecting discard → draw
  → hand, without replacement, randomly within a pile.
- Reservations do not move cards. Playing a threatened hand card or drawing a
  threatened deck card leaves that instance threatened in its new pile.
- Reducing one source only releases that source's cards. For attacks of four and
  six, halving the six leaves four plus three; it never halves the combined ten.
- Rolled defensive abilities save in reverse live-pile priority: hand → draw →
  discard, randomly within the preferred pile. The `saved_card_destination` setting
  controls their destination; `original` never undoes a play or draw.
- Reductions apply in play order. Preventing three from seven and then halving
  leaves two; halving seven (rounded down) before preventing three leaves zero.
- Cards and defensive abilities use the shared `saved_card_destination` setting:
  `original` (default) or `discard`. Brace/Guard explicitly use discard and
  Brace+/Guard+ use original. A played Brace still goes to discard, even if it saves
  itself; protecting another source does not clear Brace's own outstanding
  reservation. Protect retains its explicit
  saved-to-discard status rule. Discard counts as health and never reshuffles.
- Repainting, reopening a save, or reconciling unchanged damage never rerolls
  existing reservations. Saved proposals remain released.
- Damage beyond available health has no duplicate card reservation. If later
  prevention frees cards while another attack still has unfilled excess damage,
  that excess may reserve available cards. Existing reservations stay attached
  to their original source; total outstanding damage is not silently lost.
- Unconditional attack statuses still apply if the attack is fully blocked.
  Immediate defense effects retain their existing timing.

## Compatibility and testing

The experiment is a persisted battle rule passed through native startup and ML
simulation/replay. New battles from the menu enable it. Older saved battles and
recorded fixtures keep the old flow. Starting a new battle is required to try it;
opening an old save deliberately does not migrate its in-progress checkpoint.

Relevant checks:

```bash
cd dice-and-destiny-server && go test ./...
cd ..
./scripts/godot.sh --headless --script res://scripts/verify_battle_authority.gd
./scripts/godot.sh --headless --script res://tests/presentation/verify_unified_defense.gd
./scripts/godot.sh --headless --script res://tests/presentation/verify_brace_targeting.gd
DICE_AND_DESTINY_UNIFIED_LAYOUT=1 ./scripts/godot.sh --headless --script res://tests/presentation/verify_damage_card_layout.gd
```

The unified tests cover separate sources, reverse restoration priority, card and
Protect destinations, stable saves, excess damage, blocked attack statuses,
specialized prevention, complete encounters, frozen policies, native pointer
Pass, and automatic completion eligibility. Layout tests cover 720p/1080p and
one through four enemies. The normal launcher isolates all script-test state.

To abandon the experiment, switch back to the baseline branch after preserving
any later local work. The baseline branch and its pushed commit are unchanged.
