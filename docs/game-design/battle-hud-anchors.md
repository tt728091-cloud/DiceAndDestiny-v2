# Fighter HUDs and animation anchors

The live battle gives **every actor** a compact `ActorProfile` beneath their
fighter: name and Energy, health with current/max, status symbols and counts,
then deck/hand/discard/removed symbols and counts. Status names and catalog rules
are available on hover. Existing queued ability additions remain separate `+N`
previews and do not change authoritative stacks.

`battle_icons.gd` owns the vector symbol set. Every current status definition has
its own symbol, with an unknown-status fallback. New definitions should add an
entry here. `status_icon_strip.gd` addresses cells by status ID and reserves a
slot for incoming or disappearing statuses throughout the current presentation.
Its `text` property is diagnostic/legacy compatibility, not visible UI.

## Player pile inspection

Every deck, hand, discard, and removed counter retains its name/count/rules
hover. Only the player’s deck, discard, and removed counters open the read-only
`pile_browser.gd` panel. Hand and enemy counters never open a card inventory.
Use the viewer’s authoritative `*_composition` maps, with one compact header per
copy sorted by card name; never infer contents from initial decklists or reveal
draw order. Hovering a header shows the full card from the battle’s pinned catalog.
Full previews use `BattleCard.STANDARD_SIZE` (185×248 design pixels), shared
with the player’s hand, for both actors’ damage lists and the pile browser.
Full previews stay beside their list, never over its rows. Damage-list previews
prefer the right side, use the left at the right screen edge, and move above
neighboring damage lists when necessary to keep their rows visible.
Tabs switch piles; Close, Escape, and clicking outside dismiss the panel. Empty
piles have an explicit message. A board rebuild retains the selected pile and
refreshes its contents from the new snapshot.

`tests/presentation/verify_pile_browser.gd` checks real pointer input, private pile
access, duplicates, empty/scrolling lists, previews, live refresh and dismissal
in Planning and Damage Reaction at 1080p/720p with one and four enemies. Set
`DICE_AND_DESTINY_PILE_SCREENSHOTS` to a workspace-local directory for captures.

## Positioning

`battle_scenery.gd` records each fighter's local `ground_anchor` and `actor_id`.
`fighter_profile_dock.gd` transforms that point into its parent's coordinates on
every frame. Move/scale the fighter to move its HUD, or change the dock's
`attachment_offset` to reposition just the HUD. The live compositor offsets the
encounter's normal placements to reserve room above the hand. The player stands
farther left, leaving four full-width enemy lanes with a single dice row each.
Player placement reserves space for fully expanded curse grids even when dice
are clean. Four-enemy encounters add only a further fixed leftward offset, at the same height; curse
changes never reposition the fighter. Multi-enemy
placement creates a separate dock and profile per actor; selecting a different
enemy only changes focus and the ability inspector. Single enemies occupy the
right side of the battlefield; the attached profile, dice and
effect endpoints follow that same placement across phases. Every enemy retains its own
dice tray beneath its HUD, including while another enemy is selected.

Both sides use `BattleDiceTray.hud_compact` and the shared `HUD_DIE_SIZE`
(56×60 design pixels), with independently sized 14-point curse-face numerals,
opaque face chips and bright marked borders.
They are children of the fighter HUD dock, so status wrapping, movement and
resizing move both together. Player dice have their own bottom-left row, with
Roll/Skip on a separate row above and abilities below. Five or more enemies use
three-plus-two dice rows per actor to preserve the shared die size. Enemy attack details and the combat log use the independent right
utility rail. Face-selection pop-ups are clamped above the hand region.

Resolve a dice tray through `screen.dice_dock(actor_id)` each frame. Its first
child is the live `BattleDiceTray`; use the actual button/face control's
`get_global_rect()` for trails and targeting. `_enemy_dice_dock` is only an alias
for the focused enemy and must not be used to target other actors.

## Effects must use semantic endpoints

Use `screen.actor_anchor_rect(actor_id, kind, status_id)` or the resolved profile's
`anchor_rect(kind, status_id)`. Kinds are `health`, `energy`, `deck`, `hand`,
`discard`, `removed`, `name`, `status`, and `pending_status`. Rectangles use canvas
coordinates. For example:

```gdscript
var destination: Rect2 = screen.actor_anchor_rect(target_actor, "status", "poison")
var local_tip := get_global_transform_with_canvas().affine_inverse() * destination.get_center()
```

Resolve **every frame**. Do not retain a profile reference across a screen
rebuild, cache screen pixels, infer status rows from text, or point at the center
of the entire status strip. A short-lived effect inside the rebuilt root may
retain its profile; effects that survive rebuilding resolve via the actor map.
The 1920×1080 board transform, letterboxing, window resizing, fighter movement,
and status wrapping are thereby shared by all endpoints.

Card gains/cleanses, status changes, defense gains, Effects gathering, Catalyst,
Blind, Curse Count, Second Knell, Refusal, Dividend, Bloom, and damage-card pile
returns use these anchors. Income and damage counts update the same controls.
Same-phase combat redraws preserve settled lane scale before the first draw.

## Attack intents and defense rolls

`attack_intent.gd` presents each authoritative damage source above its attacker.
The source ID, not enemy focus or array position, selects the defense and card
target. Clicking the fighter selects its first available attack; separate intent
buttons remain available when an attacker has more than one source. The defense
rail appears after selection. Queued and completed sources cannot receive a
second defense. Each effect symbol has an explicit ×count, including a single stack. Repeated
applications of the same status to the same target are summed. Bespoke Curse
follow-ups use structured catalog quantities rather than numbers parsed from
rules text; conditional Blind is marked ×1? and per-die marks ×1/die.
Hovering individual symbols explains each status application;
locked player abilities and revealed status-only abilities also receive intents.
Unrevealed opponent choices remain hidden.

`screen.attack_anchor_rect(source_id)` resolves the live damage numeral in canvas
coordinates. `screen.ability_intent(actor_id, ability_id)` also supports the
preview of a locked ability before a damage source exists. These controls track
the fighter every frame. Card prevention and attack-bonus flights use these
anchors instead of the removed central attack panels.

Defense dice reuse the existing authoritative faces and shared defense clocks,
while travelling, bouncing, and rotating across the battlefield. They settle in
separate source slots; crowded encounters use the open area below enemy HUDs.
Per-die prevention originates at that landed die's result and ends at its attack
intent. Status gains originate at the granting die and resolve through the actor's
status anchor. Fixed defenses use the same effect area and destinations without
inventing a roll. Damage-card reveals retain their fitted grid, but have no attack
box around them. Saved-card feedback and card-to-intent trails share the played
card rather than drawing duplicate reveals.

## Verification

Run all Godot checks with the repository launcher. New coverage:

- `tests/presentation/verify_actor_hud_anchors.gd`: 1/2/3/4/5 enemies, 1280×720,
  1920×1080, 1600×1000, 2560×1080; independent panels, status symbols/tooltips,
  non-overlap, actor movement, HUD offsets, off-focus card effects, live resource
  endpoints, compact dice/grid footprints, off-focus Curse trails to exact face
  chips, and survival across root rebuilds.
- `tests/presentation/verify_attack_intents.gd`: 1–4 enemies at 1080p and 720p,
  real fighter/intent/defense/card clicks, movement tracking, authoritative die
  faces, prevention counters, status-only previews, hidden opponent choices, and
  live card-trail destinations. Set `DICE_AND_DESTINY_INTENT_SCREENSHOTS` to a
  workspace-local directory for graphical captures.
- Existing status, Curse, defense, Effects, damage continuity/handoff, pile-return,
  card-gain/cleanse, enemy-selection and cinematic-layout regression checks.
- `scripts/verify_battle_authority.gd` and server `go test ./...`.

Set `DICE_AND_DESTINY_HUD_SCREENSHOTS` to a directory under this workspace's
`.godot` directory and run the anchor test without `--headless` for image captures.

## Shared fanned hand

`presentation/cards/fanned_hand.gd` owns the bottom hand region for every
character. The region stays fixed while slot transforms fan and slide cards;
only the card tops remain visible when it is collapsed. Entering any part of
the expanded region reveals the hand over 0.24 seconds; leaving collapses it
over 0.30 seconds. Hover raises a card for reading. Energy appears at the upper
left and the title starts to its right in the shared `BattleCard` renderer.

Pointer selection uses stable fan geometry so overlapping or raised cards do
not steal neighbouring headers. Native keyboard activation remains available.
Targeting, hand-limit discards, and the actual Income presentation hold the hand
open. Slots move independently of card-local draw animations, and redraws carry
forward the current reveal progress.

`tests/presentation/verify_fanned_hand.gd` checks all three playable characters
at 1080p/720p, empty/single/6/10-card hands, real pointer selection, disabled cards,
keyboard activation, lifted-header clicks, toggles, draw animations, and redraws.
Set `DICE_AND_DESTINY_FAN_SCREENSHOTS` to a workspace-local output directory to
capture expanded, collapsed, and hovered views in a graphical run.

### Entering defense from a revealed attack

During Offensive Reaction, an incoming intent can open its defense choices while
reaction cards remain playable. Inspecting an intent does not pass priority.
Committing a defense uses the existing reaction pass, preserves opponent responses,
and submits a fresh authoritative defense command when Defense Selection opens.
The choice retains the attacker, attack content, target, Catalyst option, and an
exact source ID when already available. Canceled attacks or newly unavailable
choices ask for a new selection; they never retarget another enemy automatically.

`defense_previews` is a viewer-only presentation field, separate from immediately
submit-ready `legal_actions`. It is available only at the revealed reaction
checkpoint to the actor holding priority. The client closes offensive card input
after commitment, carries the choice across opponent priority, and revalidates it
against the resulting legal actions. Defense rolls and their animation anchors
continue through the normal defense flow. Manual Pass remains available for
skipping a response; it is not required before choosing a defense. No-choice
windows retain their existing automatic progression.

### Damage stacks and removal playback

Damage cards are partitioned by exact source ID and placed in independent stacks
inside an actor-bound area below that actor's HUD (and enemy dice). A compact
header shows the source's damage; each 24-pixel card header retains its cost and
name. Moving over a header shows a full, non-intercepting card above the stack;
moving away restores it. Large batches scroll within the actor's area rather
than shrinking type. The ordinary 4/2/3 damage example keeps three separate
stacks and all nine headers visible.

`combat_card_reveal.gd` keeps the original BattleCard children and public removal
identities for prevention feedback. Saved/released cards are excluded from
committed stacks. Only authoritative committed damage starts
`damage_stack_tear.gd`: every stack shares the batch clock, splits its exposed
card headers, and draws a green trail to the target ActorProfile's live
`anchor_rect("removed")`. Counter and health playback use the same accepted
removals and actor baseline. Large losses allow at least 40 ms per count step;
the stacks animate concurrently, without adding per-source waits or prompts.

Verification: `verify_damage_card_layout.gd` covers 1–4 enemies, three simultaneous
sources, full-deck losses, scrolling, hover restoration, and moving HUD anchors
at 1080p and 720p. `verify_damage_stack_commit.gd` checks a shared removal clock,
individual counter steps, synchronized health, exact totals, and automatic exit.
The damage response, blocked-damage, handoff, and health-continuity regressions
cover prevention and the existing phase flow.

## Attack ability hover contract

An attack's hover explains the choice before the player commits a defense. Every
player/enemy intent uses `BattlePresentationCatalog.attack_ability_tooltip` and
`attack_intent_button.gd`: ability name, selected tier/requirements and benefits,
revealed offensive dice, full pinned rules, target, current damage, and relevant
status/defense information. Intent symbols share the whole-ability hover instead
of intercepting it with a short label. The fighter's selection target uses the
same popup. Wrapped popups use Godot's viewport-aware tooltip placement.

Ability authors must supply `presentation.rules_text` explaining the dice-based
calculation, miss conditions, and conditional effects, with matching structured
qualification tiers and operations. Brine Lash, for example, explains 2 damage
per Brine (3), keeping threes during rerolls, and a miss with no Brine. Use the
revealed tier, not a guess based on current damage: cards and prevention can
change that number. After dice edits, the authoritative rerender refreshes the
recipe and faces; defense animation updates the current damage. Never reveal a
hidden opponent selection to populate a tooltip.

Regression: `tests/presentation/verify_attack_ability_tooltips.gd` covers shared
catalog rules, tiers, dice changes, viewport placement, and offensive/defensive
hover behavior, alongside `verify_attack_intents.gd` for source selection.

## Defense choices beside the selected attack

The player's existing defensive ability controls appear immediately left of the
selected source's overhead intent, including previews during offensive reaction.
The ability rail carries the source ID and resolves that intent's live rectangle;
actor movement and viewport scaling must move the choices with it. If another
intent occupies that space, shift the choices just below it so every attack
remains clickable. The dice and roll controls keep their lower-left placement.

Keep defense controls above fighter selection hit targets in both draw order and
GUI sibling order. Preserve every ability, paid option, availability state, and
exact source-targeted command. Once defense selection ends, rebuild the ordinary
rail; do not leave the floating controls behind. Validate with
`verify_defense_choice_position.gd`, `verify_attack_intents.gd`, and
`verify_defense_entry.gd`.

### Complete defense outcomes

Present every outcome of a defense together on the Defense board. Hexward
Rebuttal's prevention and Curse must launch from the landed defense die in the
same animation interval: prevention targets the incoming attack, and Curse
targets the receiving dice before their existing roll/face-mark animation.
Use green launch trails; keep the purple face highlights on the receiving tray.
Do not show detached defense-Curse text panels or replay the effect in Damage.

The authority still finalizes after legal defense responses. Until then, the
landed roll remains a preview. When the finalized result arrives, retain the
previous defense board while presenting its complete outcome, then expose the
next defense wave or Damage snapshot. Preserve independent source IDs and run
simultaneous defense follow-ups together. Do not reveal later offensive Curse
marks or damage counters from the same authority response early.

Validate direct placement, rolled expansion, player/enemy recipients, both
submission paths, multiple incoming attacks, redraws and no replay with
`verify_defense_curse_feedback.gd`.

### Damage and attack follow-ups

Brine Surge is included in the joint attack reveal: show the authoritative
boosted total immediately, without a separate card reveal, damage-count hold,
or trail. Append its damage contribution and card name to the bottom of the
attack hover details, alongside the full ability rules. Keep that explanation
through defense and clear it with the attack's round. Validate with
`verify_brine_surge_feedback.gd`.

Show the upcoming attack effect beneath its recipient's damage-card stack, with
the ability name and quantity (for example, `Hexbrand · 2 Curse`). Keep this cue
visible while cards tear and health/Removed counters update. Once those updates
finish, launch the green trail from that same cue into the receiving dice, then
play their Curse checks and face highlights. Keep the board, stack anchors and
Damage presentation alive throughout; do not insert a separate Curse screen or
purple origin/outcome panels. Multiple source follow-ups run concurrently, each
retaining its own recipient and stack origin. Fully blocked damage still shows
and resolves applicable follow-ups. Only advance after all follow-ups finish.

`verify_hexbrand_curse_sequence.gd` checks the continuous tear/counter/Curse
sequence, pending quantities, both viewport sizes, physical dice previews,
multiple sources, blocked attacks, redraws and duplicate-event suppression.

### Status expiration

Queued expiration feedback must not hide or consume an active HUD status during
an earlier presentation beat. Malediction’s Refusal remains visible through
Damage and Effects, then expires at Income. Its real status cell fades while a
green trail connects it to a nearby expiration caption; the caption then fades
out. Place the caption beside the actor's HUD, or just below their dice when
neighboring actors leave no lateral room. Never cover another actor's HUD or
invent a dice roll for an unused status expiring.

`verify_maledictions_refusal_feedback.gd` checks application, trigger outcomes,
Damage → Effects → expiration → Income ordering, continuous status visibility,
redraws, local placement and fades for player/enemy recipients at 1920 and 1280
widths, including four-enemy encounters.
