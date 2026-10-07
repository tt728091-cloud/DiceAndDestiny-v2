# Fighter HUDs and animation anchors

The live battle gives every enemy a compact `ActorProfile` beneath its
fighter and the player a fixed bottom-right profile: name and Energy, health with current/max, status symbols and counts,
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

The live board is 1920×1080, uniformly scaled and letterboxed. The top 64%
(y < 691.2) belongs to enemies beneath the shared header; the bottom 36% belongs to the player. There is
no player fighter sprite. Existing habitat, fighter, card, and symbol assets are
preserved. `bone_frame.svg` repeats complete bone pieces with fitted tiling along the
player station, player stats, utility buttons, and attack intents. Fixed corner
joints join the edges; longer edges add bones instead of stretching a single
shaft. `bone_health_frame.svg` supplies a slimmer matching frame for every
actor’s health bar, retaining the live fill, number, and health animation anchor.

Battle panels and dark controls use opaque RGB (40, 39, 40), `#282728`;
`cinematic_theme.gd` defines `DARK_SURFACE`, and the bone-frame SVG uses the
same fill. Existing hover accents and parchment controls retain their treatments.

The lower-left charcoal station contains Roll/Skip (or Pass), the player's dice,
and the offensive ability list. Six ordinary Adventurer abilities fit without
scrolling; expanded choices and large Curse grids may scroll the ability list.
Controls never push upward into enemy space. During Defense, offensive dice and
abilities hide. Clicking an incoming attack opens its legal defensive choices
beside that exact attack. The compact menu avoids every attack number and stays
inside the battlefield; dice still roll in the lower-left station. No attack is preselected on entry. The fanned hand
stays in the center. Player stats anchor at y = 1060 in the bottom-right;
status growth extends upward, leaving the hand clear. Pending player
losses scroll above those stats, while enemy losses scroll below their own HUDs.
Each source has its own heading/cards; selecting an attack scrolls to its group.
Player attacks show damage in the pending-loss heading beneath their recipient;
no floating outgoing badge is shown before or after that heading appears. It retains the live damage, full
ability hover, and attack effect anchor. Enemy incoming badges remain selectable;
previews before pending losses exist retain their badge.
The compact top header contains only the centered round/current-segment banner
and the Enemies, Log, Inspect, and Settings buttons at the upper-right. There is
no secondary segment strip. Enemy portraits and profiles sit higher to expose
at least five full pending-card headers in ordinary encounters; larger batches
remain scrollable. Buttons remain accessible while utility panels are open.

`battle_scenery.gd` records each enemy fighter's local `ground_anchor` and
`actor_id`. `fighter_profile_dock.gd` follows that point every frame. The player
dock instead uses `fixed_bottom`, so it needs no hidden portrait. This anchor
takes precedence over fighter attachments and is reapplied synchronously on
resize and container sorting, including during income and phase rebuilds. Effects must
resolve the same live actor HUD controls in both cases.

Enemy dice are collapsed initially. Clicking an enemy name toggles its offensive
dice above the name; during Offense that click also focuses the enemy. Visibility
survives root rebuilds without changing authority state. `enemy_dice_dock.gd`
follows its profile independently, so opening dice never moves health or status
anchors. Revealed enemy defense dice always appear and cycle above the enemy
name, independently of that toggle. The defense name sits above the dice during
the roll; per-die results appear after landing, then green prevention trails reach
the affected damage amount. Opening the offensive tray moves the defense row
above it, keeping both rows separate without moving the profile.
`verify_enemy_defense_dice.gd` checks animated faces, result timing, prevention
trails, and placement at three viewport sizes with one and four enemies, plus
a native unified-defense Pass through Salt Veil into the next round.
Dice targeting temporarily unfolds the trays; Curse feedback briefly
reveals affected trays and then returns to the player's toggle preference.
Curse targeting the player temporarily reveals their face maps too; any visible
defensive result moves to the next row within the same lower-left station.
Only viewer-safe dice are used, so opening a tray cannot reveal secret results.

Both sides retain `BattleDiceTray.hud_compact` and the shared 56×60 die size,
including readable curse-face grids. Five or more enemies use a three-plus-two
dice arrangement. Empty legacy results containers ignore pointer input so they
cannot block the centrally placed enemy names, artwork, or intents. Genuine
result controls and utility panels retain their own input handling.

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

`attack_intent.gd` presents each authoritative damage source above its attacker,
or beside tall artwork when the compact header leaves insufficient space above.
New badges stay hidden until their actor anchor is available. Board rebuilds
position them after attaching all profiles, before transitions or the first draw;
asynchronous opponent results must never expose a badge at the default origin.
The source ID, not enemy focus or array position, selects the defense and card
target. Clicking the fighter selects its first available attack; separate intent
buttons remain available when an attacker has more than one source. The defense
rail appears beside the selected attack when using that overhead entry point.
During Defense, the lower-left station also lists every incoming source separately,
including multiple attacks from one enemy. Rows show the current damage, ability
name, attacker, pending effects, and `Undefended` until a defense is chosen. Queued
rolls show `Queued`; the active roll shows `Defending…`; completed defenses clear
the label, even if their roll prevents zero damage. Card-only prevention does not
pretend that a defensive ability has been used. The source history survives redraws.
Clicking a list row opens its defensive choices in a separate bone-framed popup
to the right of the station (design x=454). This popup may cover the hand, but
never inserts list content or displaces attack rows; it stays inside the viewport
and receives pointer input above cards. Utility windows temporarily hide it. Clicking an overhead intent switches back to adjacent
choices. Both routes submit the exact source ID, and source-targeted cards can
also use an eligible list row. Queued and completed sources cannot receive a
second defense. Each effect symbol has an explicit ×count, including a single stack. Repeated
applications of the same status to the same target are summed. Bespoke Curse
follow-ups use structured catalog quantities rather than numbers parsed from
rules text; conditional Blind is marked ×1? and per-die marks ×1/die.
Hovering individual symbols explains each status application;
locked player abilities and revealed status-only abilities also receive intents.
Unrevealed opponent choices remain hidden.

`screen.attack_anchor_rect(source_id)` resolves the visible damage control in canvas
coordinates: the lower-left row numeral for incoming attacks during Defense,
the recipient heading for consolidated player attacks, otherwise the intent
numeral. Scrolled-out row endpoints clamp to the list viewport. Presenters expose `attack_damage_rect()` for the same endpoint. `screen.ability_intent(actor_id, ability_id)` also supports the
preview of a locked ability before a damage source exists. These controls track
the fighter every frame. Card prevention and attack-bonus flights use these
anchors instead of the removed central attack panels.

Defense dice reuse the existing authoritative faces and shared defense clocks,
with player defense dice cycling in place in the lower-left station. They never
travel, bounce, or rotate across the field. Only the current defense for each
actor occupies its result station; completed sources retain their intent results.
Authored large enemy silhouettes remain larger and take the center lane in
three-enemy encounters. Growing enemy status strips are clamped above the
player region.
Per-die prevention originates at that landed die's result and ends at its lower-left
incoming row for player defenses, never at the overhead enemy number. The row
shares the presenter's damage animation clock; it does not predict damage or
choose released cards. The station reserves a fixed dice band before selection: dice start at (38,716),
use the same 56×60 dimensions and 6-pixel gap as offensive dice, and remain hidden
until a defense is chosen. Pass stays fixed at the upper right. There is no Apply
button: the completed roll finalizes automatically and returns to the Defense hub.
Available Protect appears in the selected attack’s defense popup alongside the
rolled defenses, with its current prevention amount and full status rules. It
remains reachable on an already-defended attack while the authority permits it,
and disappears after consumption. Clicking Protect immediately spends it on that
selected source and starts the prevention/saved-card animation; there is no
second target dialog or confirmation. Protect never occupies the action footer. The attack list starts at y=844 and does not shift for a roll or popup.
Only an explicitly revealed Curse face-map tray needs additional space. All
sources remain scrollable. Undefended attacks sort first, queued choices follow,
and completed defenses move to the bottom after the reduction animation settles.
Sort ties retain authoritative source order; completion restores the top of the
list so the next undefended attack is visible. History uses the same ordering. Saved-card flights still originate from the
right-side pending-card list and use authority-published destinations. Ability
feedback launches its saved-card trails from the local incoming amount; card
feedback retains the played card as its origin. Enemy
defenses continue to target their own attack endpoint. Status gains originate at the granting die and resolve through the actor's
status anchor. Fixed defenses use the same effect area and destinations without
inventing a roll. Damage-card reveals retain their fitted grid, but have no attack
box around them. Saved-card feedback and card-to-intent trails share the played
card rather than drawing duplicate reveals.

## Verification

Run all Godot checks with the repository launcher. New coverage:

- `verify_incoming_attack_list.gd`: actual local and overhead pointer entry,
  repeated sources per enemy, paid defenses, right-side popup without row movement,
  fixed offensive-size defense dice and far-right Pass, undefended-first ordering,
  scrolling at 720p/1080p, full rules and effects, shared reduction clocks,
  queued/completed state, and a native
  unified-defense battle with saved-card destinations and right-side origins.

- `tests/presentation/verify_first_person_layout.gd`: real name/attack clicks,
  1–4 enemies at 720p, 1080p, 16:10, and ultrawide; 64/36 ownership bounds,
  no player portrait, six visible base abilities, independent attacks/statuses,
  dice toggling across redraws and phases, compact header/utility placement, and separate threatened-card lists.
  Use `DICE_AND_DESTINY_LAYOUT_SCREENSHOTS` for graphical captures.
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
- `tests/presentation/verify_guarded_strike_transition.gd`: pointer-selects
  Guarded Strike against two Brine Masks, runs the live threaded opponent turns,
  and checks every rendered frame for stray badges through Defense at three sizes.
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
than shrinking type. The ordinary 5/2/3 damage example keeps three separate
stacks; five full headers fit before scrolling to later sources.

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

## Offensive target selection

Selecting a single-enemy offensive ability or tier prepares it locally. It never
submits against the currently inspected enemy. Eligible living enemy portraits
pulse gold until the player clicks a portrait or its name; only then submit the
current legal action for that exact target, preserving tier and other choices.
Require this explicit click even when only one enemy remains. Targetless and
multi-target authored actions retain their existing flow.

Clicking the same ability again, Cancel, or Escape cancels without spending.
Another card or submitted action clears the choice. Revalidate legal actions on
refresh and on target click so stale or defeated targets cannot be submitted.
`verify_offensive_targeting.gd` covers pointer selection, cancellation, pulsing,
and stale targets with 1, 2, and 4 enemies at three viewport sizes. The live
Guarded Strike handoff and Strike tier stability checks include the target click.

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

## Wrapped rules tooltips

Ability tiles (including their info buttons, tier controls, and defense choices),
card explanations, defense result labels, and status/pile hints use the shared
`wrapped_tooltip.gd` presenter via the tooltip button/label helpers. Text is
word-wrapped to at most 440 logical pixels, with room for popup padding and
viewport edges. Short hints keep their natural width. Measure the wrapped height
before opening the popup so Godot can position it above bottom-edge controls.
Preserve the complete rules; do not truncate them to make a tooltip fit.
Ability rows use the shared catalog tooltip builder: show the name once, omit
a recipe that merely repeats that name, and include temporary bonus/expiration
rules once. Tier controls share the same bonus wording. Keep the qualification,
base damage, miss conditions, and selected outcome available.

Empty or whitespace-only hover content must never open a popup. Normalize it
through the shared `wrapped_tooltip.gd` content check in `_get_tooltip`, before
Godot opens a window, and return null from custom builders for blank content.
This also applies to cards, statuses, actor profiles, and rich attack tooltips.
`verify_empty_tooltips.gd` exercises blank and populated pointer hovers across
all custom tooltip types, including the shared Roll/Pass button controls.

Use these helpers for new text-only battle hover controls. Revealed attack
intents retain the richer `attack_intent_button.gd` presenter described above.
`verify_wrapped_rule_tooltips.gd` checks all authored card/ability rules and actual
Guard+, info-button, card, status, and normal/boosted Small Straight hovers at
both edges and multiple sizes.

## Defense choices beside the selected attack

The player's existing defensive ability controls appear beside the selected
source's overhead intent, including previews during offensive reaction. Prefer
the left or right, then a clear row below; keep the compact bone-framed menu
inside the battlefield and leave every attack button exposed.
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
next defense wave, Damage, or Income snapshot. This hold also applies to ordinary
unified defenses (such as Salt Veil), whose finalization can commit damage and
advance the round in one response. Keep the old health visible while the green
die-to-attack trail reduces the authoritative attack amount. Release the saved
reservations on that board with the same effects clock; build feedback only
after all recipient HUD anchors are attached.
Preserve independent source IDs and run
simultaneous defense follow-ups together. Do not reveal later offensive Curse
marks or damage counters from the same authority response early.

Validate direct placement, rolled expansion, player/enemy recipients, both
submission paths, multiple incoming attacks, redraws and no replay with
`verify_defense_curse_feedback.gd`. `verify_enemy_defense_dice.gd` also checks
the native Pass sequence frame by frame and both finalization submission paths:
6 incoming → 4 pending must be visible before 12 health → 8 and next-round Income.

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

### Played draw-card feedback

Resource-gain cards played from the viewer's hand retain their clicked pose in
board coordinates and fade there while a curved green trail reaches the affected
HUD counter. Drawn cards start at the live draw-pile icon and arrive in their
individual hand slots, with staggered flights and matching counter increments.
Destination cards remain hidden until arrival; the hand stays open during the
animation. Rebuilds retain the same timeline and cleanup restores normal hand
visibility. Only public draw-event IDs or newly present IDs in the viewer-safe
hand may create flights. Short/empty decks create only the actual draws and do
not recycle discard or change health. Enemy hidden card identities stay hidden.

`verify_card_draw_flights.gd` checks pointer play, retained pose, 0/1/2-card
results, counter timing, staggered arrival, redraw continuity, cleanup, 720p and
1080p, and a native-authority Take Stock play. `verify_card_gains.gd` covers
shared status/resource effects, queueing, and duplicate-event suppression.

Card plays also fade the actor's energy number from its pre-payment value to
the post-payment value over 0.6 seconds. This feedback survives HUD rebuilds,
does not block actions, and ignores repeated events and unchanged values. Cards
that grant energy use the same fade for the later reward, after payment.
`verify_card_energy_fade.gd` checks spending, reaching zero, defense plays,
free cards, gains after payment, redraws, replacement, and event deduplication.


Cards targeting abilities, attacks, or dice keep the selected hand card raised,
straightened, and outlined while the pointer moves to the target. An ability prompt names the card
and its catalog-authored damage bonus; Cancel spends nothing. Targeting uses
single horizontal tier requirements without resizing the rows on hover.
Confirmed Strong Swing feedback retains that hand pose and curves to both the
authoritative status and the selected ability's temporary damage badge. The
hand stays open during the fade. `verify_strong_swing.gd` exercises actual hand
and ability pointer input, cancellation, layout, both trail anchors, one-time
payment, and offensive-exit expiration at 720p and 1080p.

Die-manipulation cards use the same captured hand pose, keyed by the public
played card instance (including duplicate copies). Try Again stays highlighted
while a curved trail reaches the actual die, which then tumbles and settles on
the authority's result without spending a normal roll attempt. Board rebuilds
retain the pose and the hand stays open until feedback ends. Missing poses never
fall back to a floating card at a fixed screen coordinate.
`verify_card_die_hand_pose.gd` checks native pointer play, duplicate copies,
frame-by-frame positioning, redraws, trail anchors, and every catalog card's
shared pose/fallback behavior at 1024 and 1920 pixels.

Prevention cards retain that captured hand pose through target confirmation and
board rebuilds, then fade in place. Their trails to the incoming damage amount
and saved-card list share the same clock; the hand stays open until feedback
ends. Animation headers keep their full dimensions even at a scroll boundary,
so clipped rows cannot squash their text during a flight. Brace/Brace+ pointer
and animation checks live in `verify_brace_targeting.gd` for both defense flows.


The offensive station puts its five unchanged 56×60 dice at (38,716), with
Roll and Skip stacked to their right at x=350. The ability scroll area uses all
remaining height below that band; a temporary targeting prompt reserves space
only while a card target is being selected. Temporary damage is a blue-green
`+N DMG` label beside the ability name when the full text fits; otherwise it
gets its own line beneath the name. Ability names never wrap. Long requirements
or tier groups also move intact to another line when necessary, and the row
height expands to contain them. The full duration remains in the hover rules. All six Adventurer abilities,
including Decisive Blow, fit without scrolling before and after Strong Swing.
The Strong Swing pointer/layout regression actually traverses nested ability
rows and exercises every pinned catalog ability with a bonus. It checks native
Strike and Small Straight card plays, unchanged dice sizes, nonoverlapping
controls, horizontal names, row containment, and scrollbar absence at 576p,
720p, and 1080p.


Selecting an offensive tier changes the live rail without detaching or flying
its tile. Viewer attacks never create an attacker-side floating damage badge,
including the interval before joint reveal. The recipient's pending-card
heading owns the outgoing damage and full ability hover. Preparation modifiers
already shown on card play (such as Strong Swing) contribute their full total
at attack reveal without replaying the card or a second bonus trail. Fixed
player HUDs anchor before the first draw and again during Container sorting.
`verify_attack_selection_stability.gd` follows a native four-sword Strike with
Strong Swing through opponent turns and unified Defense, checking every frame
for a stationary player HUD, no detached ability, no outgoing badge, and no
replayed preparation. It checks seven damage/seven reserved cards beneath the
recipient and preserved tier/bonus hover details at 720p and 1080p.

Round banners follow the ordered presentation events. Resource events without a
positive round inherit the preceding event round, falling back to the authority
snapshot only when no event context exists. Never display Round 0 or use a later
batch round for an earlier income animation. `verify_round_transition.gd` checks
the native Defense Pass through Income and Offense at 720p and 1080p, including
every rendered frame of the player HUD and monotonically advancing banners.


Completion results (Victory, Defeat, and Draw) render above the battlefield HUDs.
Their scroll container is also ordered after fighter controls for pointer picking;
raising visual z-order alone must not leave enemy controls intercepting the buttons.
Wound Review opens above the completion controls. `verify_wound_review.gd` checks
right-edge and overlapping-HUD pointer targets, all three completion actions, and
review open/close behavior at three viewport sizes with two and four enemies.

Each actor has one visible defensive dice result. Finalizing a defense retains
that source in its dice station after the active selection clears; historical
rolls keep their damage anchors but cannot draw dice or benefit labels over it.
The remembered source resets with the battle/round. A restored history-only
snapshot chooses one deterministic result. `verify_defense_result_labels.gd`
covers differing prevention/energy rolls, reverse selection order, finalization,
redraw, and round reset at 1024, 1280, and 1920 widths.
