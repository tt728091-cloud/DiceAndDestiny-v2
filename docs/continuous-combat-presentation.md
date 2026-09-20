# Continuous combat presentation

Combat UI uses two lanes in the central play area, grouped by the **recipient**:
player on the left, opponent on the right. Each authoritative damage-source ID
keeps the same transition key through selection, defense and damage. A player
clicks an incoming attack before choosing a defense; legal actions are filtered
by that source, including optional Catalyst payment.

Once established, a source frame and its unchanged labels stay visible through
defense selection, rolling, damage review, and commitment. Its rectangle expands
in place after the nested layout settles; only obsolete prompts and changing
contents fade. Dice appear as the expansion finishes, and pending-removal cards
appear inside that same frame. Frame-by-frame regression checks cover these
handoffs, including the smaller viewport and automatic commitment.

An offensive selection keeps its tile opaque while moving to the top of the
ability rail, then swaps its requirements for the authoritative outcome once.
Other abilities fade away. The hand, both dice trays, roll controls, and action
footer stay opaque across selection and the subsequent handoff. Synchronous
command submission does not rebuild the board before capturing its settled
layout. Real dice-manipulation reactions remain available there. Once those
finish, the attack travels to the receiving character's lane. Defense dice land
there; per-die prevention trails reduce the incoming number, and status trails
travel from the relevant die to the receiving profile.

Accepted player rolls and rerolls animate for 0.45 seconds in the existing dice
tray. Only rolled dice cycle faces and wobble; held dice retain their faces and
highlights. The animation ends on the authoritative result, including when a
reroll produces the same face. Its timestamp survives redraws, and gameplay
waits until it settles. Rejected commands and history inspection do not animate.

Damage cards wrap and scale beneath their source. Shared-source removals display
once. Reaction cards can save them. A `combat_damage` presentation beat also covers
commits that the authority finishes automatically, so no-choice damage cannot
skip the reveal/hold/removal sequence. Health and zone counts use the pre-commit
baseline until the cards have faded, then count down/up before the next beat.
This changes presentation only; the authority remains the source of game rules.

Ongoing Effects uses the same compact `BattleCard` and `damage_card_grid.gd`
card dimensions as damage resolution, labeled **Removed** for committed losses.
Each recipient has three reserved columns per toxin group: one card slot beneath
each Poison die and two vertically stacked slots beneath each Volatile Poison
die. Both actors' maximum toxin losses (12 dice, 18 cards) fit together without
scrolling or extra card scaling at 1920×1080 and 1280×720. Slots are reserved
before rolling, so revealing cards does not move the dice or resize the groups.

Edit the `[dice_and_destiny]` section of
`dice-and-destiny-client/project.godot` (also visible in Godot Project Settings):

| Setting | Default seconds | Purpose |
| --- | ---: | --- |
| `presentation/card_cleanse_seconds` | 3.2 | Antidote card reveal, trail to its owner’s status, status fade and card exit |
| `presentation/card_gain_seconds` | 2.4 | Card benefit reveal beside the profile, resource/status trails and exit |
| `presentation/catalyst_cue_seconds` | 0.9 | Hold the clearing face while Catalyst's green trail reaches the die |
| `presentation/catalyst_reroll_seconds` | 0.9 | Animate the forced reroll with its Catalyst label visible |
| `presentation/combat_transition_seconds` | 0.65 | Attack morph, travel and segment crossfade |
| `presentation/defense_roll_seconds` | 1.2 | Visible defense roll after arrival |
| `presentation/player_roll_seconds` | 0.45 | Player initial roll and unkept-dice reroll |
| `presentation/defense_effects_seconds` | 2.4 | Per-die outcomes and trails |
| `presentation/defense_hold_seconds` | 1.5 | Read settled defenses before automatic pass |
| `presentation/damage_reveal_seconds` | 0.65 | Card fade-in (plus a bounded 0.25-second stagger) |
| `presentation/damage_hold_seconds` | 2.4 | Minimum readable card hold |
| `presentation/damage_removal_seconds` | 0.7 | Card removal fade |
| `presentation/damage_count_seconds` | 0.35 | Health and card-count update after removal |
| `presentation/effects_gather_seconds` | 0.65 | Trails from profiles to Effects groups |
| `presentation/empty_segment_hold_seconds` | 0.8 | Brief automatic hold when a segment has nothing to resolve |

Timings are centralized in `combat_timing.gd` and `defense_timing.gd`. Elapsed
presentation clocks survive board refreshes; another priority handoff in the same
batch does not restart the reveal. History review does not submit gameplay
commands, and snapshot tools pause automatic committed-damage advancement.

Combat lanes establish their available width before measuring wrapped text for
height fitting. Same-stage defense redraws keep full-size panels and text from
the first frame, including the opponent's model handoff before the dice expand.
Lane resize notifications also size their contents before drawing. A redraw
late in the frame, after defense resolves or damage priority changes, must not
wait for the next process tick and expose narrow minimum-width columns.
Damage card grids restore their reveal/removal opacity as soon as they are
rebuilt, so reviewed cards cannot disappear for a frame during auto-pass or an
opponent priority handoff. The existing animation clock continues unchanged.

Opponent inference waits on the existing board without a thinking interstitial
or a board rebuild. Gameplay controls lock in place while utility controls remain
available. The completed result resumes normal presentation immediately, without
an artificial minimum thinking display time. A redraw during inference still
renders the current phase and hand.

Defense snapshots take precedence over the events that led to them. A
`dice_rolled` event names the defense ability but omits its incoming source and
Catalyst payment; it must not replace the complete defense record. Keeping that
record stable prevents settled rolls from replaying on eventless priority
handoffs or repeated event delivery. Actual changed faces still start new playback.

Antidote appears beside its owner's profile (left of the enemy's status list,
mirrored for the player). Its green trail arrives before the affected status
fades; unrelated statuses stay visible. The presentation uses the recorded
before/after counts and resumes its clock across board rebuilds. Automatic
handoffs wait until the card has exited.

Antidote played inside a Venom status response emits the same public card event
and exact removal counts as other timings. For Pinprick against two Poison,
the opponent's Antidote first animates 2 → 0; Pinprick then resolves 0 → 1.
The Combat log names both cards and records each status change once. Reaction
energy and pile counts remain current when the interrupted planning window
resumes, while private hand identities and dice remain concealed.

During Ongoing Effects, Catalyst draws a green trail from the recorded holder's
Catalyst status row to the exact rerolled die. The cue and reroll add 0.75 seconds
to the previous sequence; all subsequent damage, card removal, and status updates
shift together. Effects without a Catalyst reroll keep their original timing.
The trail uses the Effects playback clock, so pauses and redraws preserve it.

Card gains share one compact card presentation per result, with a short combined
benefit caption and trails to the receiving profile's resource/status values.
Rapid plays queue instead of stacking labels over the dice. The opponent and
automatic pass wait for this feedback; board redraws retain its playback. A
separate received-event watermark prevents cumulative events from replaying
while older presentation beats still await completion.

Public planning card plays refresh the authority's opponent-visible energy and
pile counts immediately. The planning baseline still conceals dice, selections,
and hand identities. This lets Battle Focus animate both its draw and Energy gain
from actual before/after values, and prevents non-drawing cards from inventing a
draw against stale hand counts. Resource notices hold their starting values from
the first render until their trails arrive, including across board redraws.

The regular Combat log accumulates viewer-safe events across responses for the
current battle, rather than displaying only the latest eight events. It records
public card plays, dice, attacks, damage, removals, and resource/status/pile changes.
Enemy draws include their count and final hand size without naming hidden cards;
private enemy selections and unrevealed dice are excluded. Repeated event delivery
does not duplicate entries. A new battle starts a new log.

Effects conversions and settled stack changes appear in this log instead of text
beneath the central panels. Effects dice, cards, and their animations remain in
the play area, and opening the log during Effects still shows its accumulated history.

When Effects damage exceeds an actor's remaining cards, the unused damage slots
show “No cards left” and “1 excess damage” instead of unexplained gaps. These
markers use the authority's committed overage and animate alongside actual lost
cards; they never duplicate a removed card. The settled roll captions and Combat
log also report excess damage. A replay fixture covers six remaining enemy cards
against three Poison and three Volatile Poison rolls (nine damage).

Ability variants are selected inside their ability tile. Shedskin presents the
free roll and optional Catalyst payment together, disabling payment when it is
unavailable. Fever Spike and Terminal Bite show legal toxin combinations in a
grid of at most three choices per row. Single-choice abilities select directly.
Each button submits the authority's exact legal action and revalidates it against
the current selected source and priority before submission. Rules inspection
remains available through the information button.

Attack summaries combine matching status contributions for the recipient in
defense selection, defense results, and damage reaction. Sword Cut plus a card
modifier adding one Bleed each displays one `Bleed ×2 pending` line. Individual
authority applications remain intact.

Damage playback starts from the previously displayed authority state. Restored
playback reconstructs only combat removals before the next Effects summary;
future Bleed/Poison removals must never be added back as combat health. The
recorded round-four Molt fixture reproduces the false 4-to-7 health increase.
The authority also follows a marked card to its current zone when committing
damage: playing Emergency Molt moves it to discard, where its accepted removal
must still cost one health and prevent that card from being removed again later.

Relevant verification:

The roll button reads the current offensive dice snapshot's maximum and used
rolls. Entangle is already consumed at offensive entry, so its reduced budget
must be visible before the first roll, and the next unaffected round immediately
returns to three rolls. Temporary reaction permissions do not change that budget.
Legacy event-only budget caches are cleared between rounds and battles.

```sh
./scripts/godot.sh --headless --script res://tests/presentation/verify_combat_flow.gd
./scripts/godot.sh --headless --script res://tests/presentation/verify_quiet_model_wait.gd
./scripts/godot.sh --headless --script res://tests/presentation/verify_defense_roll_continuity.gd
./scripts/godot.sh --headless --script res://tests/presentation/verify_player_roll_animation.gd
./scripts/godot.sh --headless --script res://tests/presentation/verify_round_roll_limits.gd
./scripts/godot.sh --headless --script res://tests/presentation/verify_combined_attack_statuses.gd
./scripts/godot.sh --headless --script res://tests/presentation/verify_damage_health_continuity.gd
./scripts/godot.sh --headless --script res://tests/presentation/verify_combat_log.gd
./scripts/godot.sh --headless --script res://tests/presentation/verify_combat_native_selection.gd
./scripts/godot.sh --headless --script res://tests/presentation/verify_defense_result_animation.gd
./scripts/godot.sh --headless --script res://tests/presentation/verify_defense_single_choice.gd
./scripts/godot.sh --headless --script res://tests/presentation/verify_inline_ability_choices.gd
./scripts/godot.sh --headless --script res://tests/presentation/verify_card_cleanse.gd
./scripts/godot.sh --script res://tests/presentation/verify_pinprick_antidote_feedback.gd
./scripts/godot.sh --headless --script res://tests/presentation/verify_card_gains.gd
./scripts/godot.sh --headless --script res://tests/presentation/verify_battle_focus_rewards.gd
./scripts/godot.sh --headless --script res://tests/presentation/verify_damage_response_flow.gd
./scripts/godot.sh --script res://tests/presentation/verify_damage_handoff_layout.gd
./scripts/godot.sh --headless --script res://tests/presentation/verify_effects_timeline.gd
./scripts/godot.sh --headless --script res://tests/presentation/verify_effects_card_grid.gd
./scripts/godot.sh --headless --script res://tests/presentation/verify_effects_exhausted_cards.gd
```
