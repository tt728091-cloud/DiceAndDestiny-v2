# Unified Defense experiment

Baseline: `de0ed80` on `codex/battle-ui-and-gameplay-update`.
Experiment: `codex/unified-defense-experiment`.

## Simultaneous play (new battles)

New battles from the menu, campaigns and expeditions also pin `simultaneous_play`
(`Settled.SimultaneousPlay`, requested with `simultaneous_play` at start and kept
only with `unified_defense`). Its full design is `simultaneous-play-plan.md`.
Everything below about card ownership, reservations, prevention and the single
commit still applies; the order of play changes:

- **Offense.** Every participant plays Before cards in secret, then locks in by
  pressing Roll (or skips their offense). Once everyone has locked in, the
  secret plays are revealed and resolve together (seat order, then play order),
  then everyone's first roll happens. While rolling, cards may only affect
  their owner (own dice, draws, energy, own statuses and ability bonuses) and
  stay hidden from the others. Once every ability is locked in, the abilities
  are revealed and the attacks are set; play does not stop here.
- **Between the attacks and the defenses (Offense After = Defense Before).**
  One step, opened for every participant at once. Each incoming attack shows
  the cards it threatens from the start. You can:
  - play After-offense, any-time and opt-in reaction cards (such as Disrupt)
    and Before-defense cards, with any target — except your own attack, which
    is final once you lock your ability in (no Nudge or Try Again on it);
  - choose a defense for an attack (one defense can cover all remaining
    attacks when its usage limit allows; an attack can be left undefended).
    **It rolls and resolves the moment you choose it** — damage prevented,
    energy gained — and you can choose the next one while its dice are still
    rolling. Its dice then rest above the attack's badge, where cards that
    reroll defense dice (Second Guard, Full Parry) target them; the defense
    re-resolves by the difference.

  Prevention cards on your own incoming attack (Steady Guard) also resolve
  at once. A Before-defense card plays on an attack only until that attack
  has its defense (rolled, or none); any-time cards play throughout. Protect needs no action: when the attacks are committed it
  prevents 1 damage per stack at random across your remaining incoming attack
  damage (`auto_prevention`), sends the saved cards to discard and is spent. All of this is visible to you only; the others see it at
  the reveal. Plays that touch anyone else (statuses, enemy dice, another
  participant's damage) wait in hand until the reveal, as before. There is no
  roll or lock-in button: you lock in with **Pass**, or automatically once
  nothing is left to choose and your dice have landed. AI-controlled enemies
  choose their defenses when the step opens, before any hidden play exists;
  theirs roll at the reveal.

  Once everyone has locked in, the reveal resolves the queued plays (seat
  order, then play order). A play that changed an attack's dice, ability
  bonuses or the attacker's statuses (a Disrupt on an enemy's dice, a
  Dispel on Lieutenant's Favor) re-scores that attack at once: the same
  ability at the tier its dice now reach, or a miss — never a different
  ability. Its reserved cards follow the new amount (freed cards stay where
  they are); a defense already rolled against it keeps what it prevented.
  This Offense's bonuses are spent then. Abilities with their own rolls or
  special effects keep their score. Then the AI defenses roll and resolve,
  and every defended attack's statuses land.
- **Defense After.** Opens for everyone once the reveal is done: After and
  any-time cards resolve when played and are visible (an After Salve can
  remove the poison before it ticks); they no longer change the attacks. It
  completes on its own when nothing is playable. Then the remaining damage
  and the undefended attacks' statuses commit once. A Dispel played before
  the reveal on someone's Fortified does not take back a die they already
  rolled.
- **Privacy.** During a secret step the others see each participant as the
  step began: energy, pile counts, statuses, ability bonuses and, in Defense,
  their incoming attacks and threatened cards (`SettledSimultaneous.PublicDamage`),
  plus a "locked in" marker. Secret card plays, draws, defense rolls and
  prevention are never in their events, snapshot or legal actions. Your own
  cards still waiting for the reveal stay dimmed in hand (no on-screen text
  lists them). One
  `secret_plays_revealed` event lists every play at the reveal; the combat log
  shows it, with no on-screen pop-up.
- **Which battles.** The rule needs every participant's cards to support
  secret plays: card programs and cards that only touch their owner. Battles
  with Venom or Curse mechanic cards, or an enemy-targeting legacy card such as
  Alchemist's Gamble, keep the unified flow below.

## Player flow (unified defense without simultaneous play)

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
  `original` (default) or `discard`. Steady Guard/Guard explicitly use discard
  and Bulwark/Guard+ use original. A played prevention card still
  goes to discard, even if it saves itself; protecting another source does not
  clear that card's own outstanding reservation. Protect sends the cards it
  saves to discard (`auto_prevention_destination`) and acts on its own at the
  final commit. Discard counts as health and never reshuffles.
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
simulation/replay. New battles from the menu enable it, together with
`simultaneous_play` where every participant's cards support it. Older saved battles and
recorded fixtures keep the old flow. Starting a new battle is required to try it;
opening an old save deliberately does not migrate its in-progress checkpoint.

Relevant checks:

```bash
cd dice-and-destiny-server && go test ./...
cd ..
./scripts/godot.sh --headless --script res://scripts/verify_battle_authority.gd
./scripts/godot.sh --headless --script res://tests/presentation/verify_unified_defense.gd
./scripts/godot.sh --headless --script res://tests/presentation/verify_steady_guard_targeting.gd
DICE_AND_DESTINY_UNIFIED_LAYOUT=1 ./scripts/godot.sh --headless --script res://tests/presentation/verify_damage_card_layout.gd
./scripts/godot.sh --headless --script res://tests/presentation/verify_simultaneous_play.gd
```

`internal/battle/engine/simultaneous_play_test.go` covers the simultaneous rule
with two external participants (secret plays, lock-ins, reveal order, self-only
rolling, the After steps, Fortified and Lieutenant's Favor timing, the single
commit, AI choices from public information, per-viewer privacy and save/reload);
`internal/battle/learned/simultaneous_play_test.go` plays complete encounters.

The unified tests cover separate sources, reverse restoration priority, card and
Protect destinations, stable saves, excess damage, blocked attack statuses,
specialized prevention, complete encounters, frozen policies, native pointer
Pass, and automatic completion eligibility. Layout tests cover 720p/1080p and
one through four enemies. The normal launcher isolates all script-test state.

To abandon the experiment, switch back to the baseline branch after preserving
any later local work. The baseline branch and its pushed commit are unchanged.
