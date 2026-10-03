# Curse playable character — 24 September 2026

The user explicitly authorized implementing the reviewed Curse design. Curse is available in the startup character dropdown, alongside Blade Warden and Venom. Its starting build has 24 health/cards, four offensive abilities, two defensive abilities, five owned D6s, a starting hand of four, two starting Energy, one card/one Energy Income, and a six-card hand limit.

## Implemented rules

- All settled gameplay rolls use persistent owned dice: offense, defense, Poison/Volatile Poison, Blind, card rolls, forced rolls, and retries. Extra results remain separate from saved offensive results unless a card explicitly changes offense.
- Each actor shares a no-replacement selection bag across Effects checks. Independent actions outside Effects start separate bags. Exhausted bags refill. Entombed dice take priority; competing bound dice are randomly selected. Retries retain identity.
- Cursed faces persist without replacing native numbers or symbols. Physical cursed results add uncapped negative Curse Count. Face setting cannot trigger Count or release Entombment.
- Entombment allows at most two bound dice per owner. Bound dice must join an ordinary reroll if the player chooses to continue; stopping remains legal. A physical cursed result releases the binding immediately.
- Effects resolves automatically. Ordinary effects and maturation precede one Curse conversion per combatant. Both Count totals are captured before conversion damage. Three Count normally deals one damage, with the remainder retained. Curse Bloom's later rolls cannot restart conversion. Lethal damage stops subsequent Curse work.
- Three Knocks is a separate, cleanseable, one-stack negative status. It doubles every converted damage group at the next Effects and expires even when conversion is skipped.
- Grave Interest, Black Tax, and Stored Calamity share one prepared conversion-mode slot per target. Prepared effects are publicly displayed and expire as specified. Each named Curse card may be used once per player per round.
- Cursed Entangle rolls all five owned dice for marks before the next offense, discards those results, and reduces ordinary maximum rolls by one. Ordinary Entangle does not add another reduction.
- Authoritative choice checkpoints support Grasp's Entomb selection, Eclipse's number, Widen the Crack's adjacent face, Misfortune Repaid's die, and the opposing player's Tomb's Choice/Misfortune's Choice decisions. Save/clone preserves pending choices, physical identities, marks, bags, and preparations.
- Second Knell precedes Catalyst and shares its one-effect-retry budget. Black Dividend rewards at most once per round and twice in total. Malediction's Refusal protects Count cleansing, never Count payment.

## Runtime locations

- Content: `dice-and-destiny-server/content/curse_v1/` (24 cards, six abilities, three statuses, three symbols, one die, one combatant).
- Rules: `internal/battle/engine/curse.go`, `curse_cards.go`, `owned_dice.go`, and their integration into the settled engine.
- Persistent state: `internal/battle/state/curse.go`; public snapshots expose owned dice, preparations, and active choice metadata without exposing selection bags.
- UI: dropdown, card-choice controls, mandatory Curse choice panel, die-mark labels/tooltips, prepared-effect labels, combat log, ability summaries, and fighter visual profile.
- Legacy chart opponents use real physical rolls in Curse-enabled battles. Existing learned models retain their weights and frozen feature positions. The scripted minion policy respects Entombment and can answer mandatory enemy choices.

## Verification

- `cd dice-and-destiny-server && go test ./...` passed, including existing authority, replay, snapshots, content, learned models, and minion tests.
- Focused Curse engine suite passed for all 24 cards and all six abilities; additionally checks every face of Hexward Rebuttal and every two-die combination of Misfortune Repaid.
- Coverage includes selection reuse beyond five dice, competing Entombed dice, physical triggers versus face setting, all-30-face saturation, all conversion modes, remainder retention, both combatants converting, delayed Bloom Count, Count cleansing, card costs, automatic retry priority, saved choices, and lethal termination.
- `TestCurseFullGamesAgainstPreservedModels`: 12 complete matches, all six preserved models in both seats; no authority rejects, invalid actions, truncation, or fallback.
- `TestCurseMinionEncounters`: 20 complete matches across single/two Brine Mask encounters, both seats, five reproducible seeds; no authority rejects, invalid actions, truncation, or fallback.
- `./scripts/godot.sh --headless --script res://scripts/verify_battle_authority.gd`: native startup and 5D6 authority smoke test passed.
- `./scripts/godot.sh --script res://tests/phase3/verify_curse_character.gd` passed a full graphical match with no script errors. It exercises the dropdown, actual native match, graphical choice buttons, defense rolls, all six loaded abilities, rematch character retention, and full-match completion. Captures are written only when `DICE_AND_DESTINY_CURSE_SCREENSHOTS` is set.

Automated correctness and complete matches do not establish competitive balance; no model retraining or balance claim is made.

## Fighter artwork

Built-in imagegen generated the project asset at:

`/Users/daddymere/games/Dice-and-Destiny-v2/dice-and-destiny-client/assets/battle/fighters/curse.png`

Transparent alpha was checked. The fighter profile is `dice-and-destiny-client/content/battle_visuals/fighters/curse.tres`; Curse cards reuse this illustration. No existing image was replaced.

Final generation prompt:

> Use case: stylized-concept. Asset type: transparent full-body 2D fighter sprite for the dark fantasy dice game Dice and Destiny. Subject: Curse, a sinister ancient mummy sorcerer wrapped in aged ivory funerary linen, tattered black shroud, cracked funerary mask with dim violet eyes, long trailing bandage ribbons. Full-body standing combat stance, three-quarter view facing right (the player stands on left side of battlefield), one hand raised to cast a curse, both feet entirely visible at bottom, strong readable silhouette. Detailed hand-painted ink and watercolor fantasy RPG illustration with dark outlines, weathered fabric, muted bone and charcoal, restrained violet curse glow. Center character with modest transparent margins, portrait framing. Genuinely transparent alpha background, no backdrop, no ground plane, no text, no UI, no borders, no extra characters. One finished game-ready sprite.

The reviewed HTML remains the design history. Its older “not implemented” statements describe the earlier review, superseded by the user's explicit implementation request and this handoff. Existing unrelated workspace changes were preserved.

## Shared character UI review — September 24, 2026

The screenshot's missing outcomes came from a Venom-only rendering allowlist. Ability tiles now use the shared renderer for every offensive/defensive definition. Requirements, damage, status effects, conditional bonuses, costs, and bespoke Curse follow-ups are visible without opening the rules dialog. Unsupported future operation types fall back to the authored rules instead of silently omitting an effect.

Hexbrand uses Needlefang's inline tier control: every tier remains visible, only legal tiers are clickable, and each shows its damage and Curse amount. Long names, wrapped outcomes, defense rules, and selected-attack summaries size their tiles to fit. Grasp and Eclipse no longer report “No offensive effect pending” after selection.

Defense results now show Hexward's Curse application and Misfortune Repaid's conditional die choice and once-per-defense Omen reward. Fixed a presentation-only stack-limit interpretation that hid uncapped Curse Count gains. Automatic Effects explains the conversion's Count total, damage, and retained Count, using the existing damage/card-loss presentation. Income and hand presentation already share the common character layout; all 24 Curse card plaques were checked at hand and removal sizes.

Validation passed through the workspace Godot launcher:

- `verify_curse_layout.gd`: planning, legal Hexbrand tier boundaries and actual mouse selection, selected attacks, both defense abilities and results, double Omen, damage, automatic Effects/Three Knocks, and all 24 cards at 1920×1080 and 1280×720. Passed in the graphical runtime; screenshots inspected.
- `verify_shared_ability_layout.gd`: all Venom, Curse, and Blade Warden ability panels, text containment, and a future custom-operation fallback. Graphical side-by-side comparison inspected.
- Existing regression tests: `verify_needlefang_tiers.gd`, `verify_offensive_benefits.gd`, `verify_inline_ability_choices.gd`, `verify_defense_result_animation.gd`, `verify_automatic_effects.gd`, and `verify_damage_handoff_layout.gd`.
- `res://scripts/verify_battle_authority.gd` and `git diff --check`.

Local review screenshots are in `dice-and-destiny-client/.godot/curse-ui-review/`, including `curse-planning-1920.png`, `character-comparison.png`, and defense, selected-attack, damage, and Effects captures at both window sizes. These are disposable development output, not game assets. This review changed client presentation only; character mechanics remain as documented above.

## Cursed-face visibility review — September 24, 2026

Replaced the comma-separated mark text beneath each die with a fixed six-face map. Cursed numbers have a purple fill and light outline; uncursed numbers remain dim. Each map identifies its die and shows the exact number of cursed faces out of six. Entombment has a separate BOUND label. The map remains available when enemy offensive results are hidden and throughout roll animations. Clean trays stay compact. Integer normalization removes the misleading `1.0,3.0` formatting, sorts and deduplicates face labels, and resets stale marks when owned-die data changes.

Dice docks now reserve their actual content height before positioning roll controls, abilities, enemy attacks, and the log. Curses cannot make the new map overlap those controls.

Validation:

- `verify_cursed_face_maps.gd` passed in the graphical runtime. Covers all 64 face combinations, JSON numeric values, clean/partial/full coverage, both owners, hidden enemy rolls, repeated redraws, animation, Entombment/release, and clearing stale state. Geometry checked and screenshots inspected at 1920×1080 and 1280×720.
- `verify_player_roll_animation.gd`, `verify_curse_layout.gd`, and `verify_battle_authority.gd` passed; `git diff --check` passed.
- Two older suites remain failing: `verify_cinematic_layout.gd` (attack-summary/source-selection assertions) and `verify_fixed_action_footer.gd` (bottom-position assertion). Reproduced both failure sets using a disposable reconstruction of the pre-change dice and fixed-offset layout; the current layout introduced no additional failure categories. Updated the cinematic test's obsolete 113-pixel offset assertion to check actual dice height plus spacing. These pre-existing failures are not represented as passing checks.

Review captures: `dice-and-destiny-client/.godot/curse-face-map-review/curse-face-maps-1920.png` and `curse-face-maps-1280.png`. Test logs and baseline comparison fixtures are disposable ignored development output. No authority rules or saved battles were changed.

## Dual symbols on cursed faces — September 24, 2026

Removed the whole-die purple tint. The lower six-face map retains its purple highlights. Revealed dice now show their original symbol and an additional purple Curse symbol (⌁) side by side only when the displayed face is marked. Numbers and kept/hover styles are preserved; Venom's illustrated normal symbol also remains visible. Hidden results show neither symbol. Animated faces update the negative symbol with the currently displayed face, then settle on the authoritative result. This is presentation only; the existing Curse Count rules are unchanged.

`verify_curse_dual_faces.gd` passed in the graphical runtime across all six faces of standard, Venom, Curse, and Brine dice, checking marked/unmarked results, kept state, animation, clearing marks, accessibility text, hidden faces, and geometry at both window sizes. The screenshot example's rolls 4, 3, 6, 1, 3 with only face 1 cursed now show both symbols only on die 4. `verify_cursed_face_maps.gd`, `verify_player_roll_animation.gd`, `verify_curse_layout.gd`, and `git diff --check` also passed.

Review images: `dice-and-destiny-client/.godot/curse-dual-face-review/curse-dual-faces-1920.png` and `curse-dual-faces-1280.png`.

## Target-panel Curse follow-ups — September 27, 2026

Target attack panels now include bespoke after-damage effects alongside ordinary pending statuses. Hexbrand's five-Skull tier visibly says “Then apply 3 Curse after damage (even if blocked)” beneath its damage amount during defense selection, defense responses, and damage review. The count comes from the attacker's selected tier, not rolled Skull count or modified damage. Funeral Rattle uses the same shared follow-up formatter. Damage playback prefers the recorded attacker tier when available. This changes presentation only.

`verify_curse_attack_followups.gd` passed headlessly and graphically: all three tiers, both viewport sizes, all three review stages, fully blocked damage, lower tiers chosen with five Skulls, modified damage, existing Poison alongside Curse, incoming/outgoing attacker isolation, and full/sparse recorded actor state. Existing `verify_defense_result_animation.gd` and `verify_pending_attack_statuses.gd` also passed. Screenshots inspected in `.godot/curse-followup-review/`; `git diff --check` passed.

## Curse application feedback — September 27, 2026

Mark the Number now reveals the played card and sends a short purple trail to the affected die's exact numbered face chip, which pulses on arrival. Its caption identifies the target, die, face, and direct placement on a randomly selected clean die, explicitly stating that no roll occurred. All public Curse face-placement events share this feedback. Public Curse-specific rolls receive a separate animated preview that settles to the authority's result; saved offensive dice remain untouched. Already-cursed results explain the Count gain, clean results explain no Count, and Entombment releases are named. Multiple outcomes play sequentially, survive board rebuilds, pause during snapshot/history inspection, and delay automatic opponent progression until shown. Existing card-gain notices share the queue so their reveals cannot cover each other.

The authority now labels newly marked faces as `direct` or `rolled`; this is event metadata only, with no changes to targeting, randomness, damage, or Curse rules. Ability choices retain their ability names without fabricating card reveals. Unfocused enemies receive named feedback without highlighting a different enemy's dice.

Validation: `go test ./...`, `verify_battle_authority.gd`, `verify_card_gains.gd`, and `verify_curse_dual_faces.gd` passed. New `verify_curse_feedback.gd` passed graphically using a real native Mark the Number play, followed by rolled expansion, duplicate-event suppression, redraw persistence, inspection pause, automatic cleanup, and sequential multi-die placement on the player. Direct and rolled outcomes were visually inspected at 1920×1080 and 1280×720; notices fit without covering the dice. Native regression coverage verifies that the initial card placement emits no roll and a Lesser Expand publishes the actual roll before its new mark. `git diff --check` passed.

Review captures: `dice-and-destiny-client/.godot/curse-feedback-review/direct-1920.png`, `direct-1280.png`, and `rolled-1280.png`. Existing workspace battle saves and developer history were preserved.

## Malediction’s Refusal feedback — September 27, 2026

Malediction’s Refusal now presents both parts of its play in one short sequence: the card reveal and trail to the exact newly cursed face, followed by a trail and pulse on the target's persistent Prepared label. The second caption explains the next Count-cleanse roll, cancellation on a Curse result, and expiry at the next Income. Preparation feedback is matched to a fresh card play and the authoritative preparation; it still appears when a Curse expansion finds an already cursed face, and does not repeat for unrelated later rolls. Prepared-label anchors are rebuilt with the board so redraws retain the correct target. This change is presentation only.

`verify_maledictions_refusal_feedback.gd` passed graphically using real native plays of Mark the Number followed by Malediction’s Refusal, matching the user's scenario. Checks cover exact new face identification, both feedback steps, preparation persistence, no invented roll, redraws, duplicate delivery, already-marked expansion, no replay on unrelated rolls, and automatic completion. Screenshots were inspected at 1920×1080 and 1280×720 in `.godot/refusal-feedback-review/`. `verify_curse_feedback.gd`, `verify_card_gains.gd`, `verify_battle_authority.gd`, and `git diff --check` passed. Existing battle saves were preserved.

## Defense-origin Curse animation — September 27, 2026

Defense Curse events now carry the originating defender, ability, and incoming attack ID. This covers Hexward Rebuttal and Misfortune Repaid's later die choice, including rolled expansions. The client preserves the defense effect's board position across the transition into Damage and displays a brief named ability cue there. A purple trail travels from that cue to the exact affected die-face chip. Newly applied defensive marks remain visually clean until the trail arrives, then light up and pulse; existing marks and authoritative dice data remain unchanged. The original defense roll/effect/hold timing is preserved. Early animation cleanup restores the final authoritative face display.

Validation: `go test ./...` passed, including new provenance checks for both Curse defenses on clean and already-cursed dice. `verify_defense_curse_feedback.gd` passed headlessly and graphically for player/enemy defense origins at 1920×1080 and 1280×720: defense-to-Damage transition, origin retention, exact face targeting, delayed visual marking, old-mark preservation, redraws, duplicate suppression, and early cleanup. `verify_defense_result_animation.gd`, `verify_maledictions_refusal_feedback.gd`, `verify_curse_dual_faces.gd`, `verify_battle_authority.gd`, and `git diff --check` passed. Flight and arrival screenshots inspected in `.godot/defense-curse-review/`. Existing workspace battle saves/history were preserved.

## Hexbrand damage-to-Curse sequence — September 27, 2026

Hexbrand's target-panel follow-up now includes the ordinary Curse rule alongside its tier-specific count: clean dice receive face 1; when all five dice are cursed, further applications roll to expand. The ability's detailed rules explain random clean-die selection and that an already-cursed rolled result adds Count without adding another mark. The explanation also appears when viewing an older pinned Hexbrand definition.

Ordinary Curse events now carry application numbers, totals, and their rule type. Hexbrand additionally identifies its source attack and ability. Its three applications are presented immediately after the matching damage removal, before next-round Effects/Income, rather than waiting for every queued segment. Each numbered application sends a purple trail from a named Hexbrand cue in the attack panel to its exact die-face chip. Future marks stay visually hidden until their individual arrival. A rolled expansion displays the real result separately from saved offensive dice and explains why face 5 can follow two face-1 placements. Existing marks remain visible. Presentation cursors distinguish the damage and Curse steps; history inspection has an explicit presentation continuation.

Validation: `go test ./...` passed, including deterministic native-engine reproduction of three existing marks followed by two face-1 placements and a rolled face 5, with exact application/source metadata. `verify_hexbrand_curse_sequence.gd` passed graphically at 1920×1080 and 1280×720, checking the advance warning, numbered direct/rolled steps, individually timed marks, damage→Curse→Effects ordering, presentation cursors, layout after transitions, and automatic completion. Also passed: `verify_curse_attack_followups.gd`, `verify_defense_curse_feedback.gd`, `verify_maledictions_refusal_feedback.gd`, `verify_curse_feedback.gd`, `verify_pending_attack_statuses.gd`, `verify_battle_authority.gd`, and `git diff --check`. Preview and three-step screenshots inspected in `.godot/hexbrand-sequence-review/`. Existing workspace saves/history were preserved; Curse resolution rules were not changed.

## Brine Surge attack feedback — September 27, 2026

The joint offensive reveal now publishes each attack-modifying card's actual damage contribution, calculated against the current dice. Brine Surge reveals beside Brine Mask's profile and sends the existing short card-effect trail into Brine Lash. The attack holds its original damage until arrival, then displays the boosted total and keeps “Includes +1 from Brine Surge.” The card caption explicitly shows the arithmetic (4 + 1 = 5 damage), and the combat log records the named cause. No hidden opponent planning is exposed early. Repeated joint reveals do not replay the same card; automatic progression and command submission wait for card feedback. Damage rules and costs are unchanged.

Validation: `go test ./...` passed. Native minion tests verify contributions across one through five Brine symbols, no bonus on a miss, and expiry next round. `verify_brine_surge_feedback.gd` reproduces a real automatic Brine Surge with two threes, tests the staged 4→5 display, card identity, exact attack trail target, persistent explanation, redraw and re-reveal deduplication, and cleanup. Graphical captures were inspected at 1920×1080 and 1280×720 in `.godot/surge-review/`. `verify_card_gains.gd`, `verify_hexbrand_curse_sequence.gd`, `verify_maledictions_refusal_feedback.gd`, `verify_battle_authority.gd`, and `git diff --check` passed. The test also exposed and fixed a combat-log null-dice error when an actor skips offense. Existing saves and developer history were preserved.

## No Safe Keep forced-reroll feedback — September 27, 2026

No Safe Keep now identifies its physical offensive reroll in the public Curse event, including the prior face and originating card. Its existing purple card reveal travels to the actual affected die in the tray. That die holds its old face until the trail arrives, cycles faces and tumbles briefly, then settles on the authoritative result. Other dice remain fixed. The caption names the target and die, shows the before→after result, explains any Curse Count gain or Entombment release, and reports the rechecked attack or required reselection. Extra Curse rolls retain their separate previews. The sequence lasts 2.8 seconds, survives board rebuilds, pauses during inspection, and uses the existing feedback gate to delay opponent progression. Early cleanup restores the final face and upright orientation; the underlying dice and roll budget are never changed by presentation.

Validation: `go test ./...` passed, including source/baseline metadata coverage for No Safe Keep. `verify_no_safe_keep_feedback.gd` uses native seed 153 to play Mark the Number and then a legally eligible No Safe Keep against Brine Mask. It verifies the held baseline, card-to-die trail, physical face cycling, unchanged neighboring dice and authority data, redraw continuity, inspection pause, exact result, deduplication, completion, same-face rerolls, and early cleanup. Graphical flight/rolling/settled captures were inspected at 1920×1080 and 1280×720 in `.godot/no-safe-keep-review/`. `verify_curse_feedback.gd`, `verify_brine_surge_feedback.gd`, `verify_battle_authority.gd`, and `git diff --check` passed. Existing workspace saves/history were preserved.

## Fixed Curse conversion damage counter — September 27, 2026

The red Curse conversion box now keeps its damage value throughout playback. It no longer switches from damage to an interpolated remaining-Count value after card removal. Curse-only Effects end when the last card finishes dissolving (5.2 seconds for two cards, previously 7.2), and the retained Count updates directly on completion. Mixed Effects retain the timing needed for real Poison/Catalyst rolls and other status changes; Curse's damage box remains fixed beside them.

Validation: `verify_curse_conversion_counter.gd` passed headlessly and graphically at 1920×1080 and 1280×720. The seven-Count fixture checks two removed cards, a constant upright “2” through every sampled timestamp including the former closeout interval, direct settlement to one retained Count, and automatic advance after card dissolution. It also verifies that real Poison dice still animate in a mixed batch. `verify_automatic_effects.gd` (including Catalyst), `verify_effects_timeline.gd`, `verify_effects_exhausted_cards.gd`, `verify_battle_authority.gd`, and `git diff --check` passed. Review captures are in `.godot/curse-counter-review/`. No game rules or saved battles were changed.

## Grave Interest status lifecycle — September 27, 2026

New battles apply Grave Interest ×1 as a normal negative status, with ordinary status rules/tooltip and the existing card-to-status reveal/trail. There is no Grave Interest preparation label or hidden preparation record. It remains mutually exclusive with other Curse conversion modes. At the next Effects it expires: with at least three Count, the first group is consumed to apply Grave Debt ×1; remaining complete groups still deal damage and the remainder is retained. With fewer than three Count it expires without penalty. Cleansing Grave Interest cancels the conversion change. Grave Debt is also a normal negative status: the next Income gains one less Energy, floored at zero, then consumes the debt.

Effects gather animates the named status from its profile into a brief purple trigger panel, which pulses and explains either the three Count consumption and resulting debt or the insufficient-Count expiry. Income highlights the actual reduced gain and names the consumed Grave Debt. The combat log records both transitions. Income events now publish the actual energy gain, including zero, preventing a phantom +1 animation. Neither transition invents a dice roll.

Validation: `go test ./...` and `verify_battle_authority.gd` passed. New engine tests cover real status application, cap, clone persistence, 0/2/3/7 Count, normal damage from remaining groups, trigger/expiry metadata, cleansing, and 0/1/2 base Energy income. `verify_grave_interest_status.gd` passed graphically at 1280×720 and 1920×1080, including real native card play, normal status placement, no Prepared label, source-to-Effects gather, trigger/expiry text, debt placement, actual zero-gain Income feedback, and cleanup. Captures inspected in `.godot/grave-review/`. Existing card-gain, automatic Effects, and fixed Curse counter tests passed. Existing saved battles retain their pinned rules; a new battle is required for the new status definitions. Legacy pinned catalogs keep their original preparation behavior. Saved battles and developer history were preserved.

## Call the Mark inline targeting — September 27, 2026

Call the Mark no longer opens the generic card-choice dialog. Selecting the card pulses only the visible, legally eligible offensive dice. A die with one legal cursed face plays the card when clicked; a die with several choices unfolds compact face buttons to the left of the enemy tray (inward/right for the player's tray). Each face shows its native symbol plus Curse and submits the exact authority-provided action. The selection remains on the battlefield with Cancel/Escape, can switch dice, survives ordinary redraws, and clears when its choices become invalid. Auto-pass and opponent scheduling wait while targeting. Authority seat IDs are mapped only for display; the original action remains untouched for submission. Costs and rules are unchanged.

Validation: `verify_call_the_mark_targeting.gd` passed headlessly and graphically at 1280×720 and 1920×1080, testing eligible dice, multiple faces, both tray sides, redraw persistence, Escape cancellation without submission, single-face play, exact selected-face submission, invalidated choices, and a real native Call the Mark play against Brine Mask (one Energy spent and correct final face). Captures inspected in `.godot/mark-target-review/`. Existing `verify_no_safe_keep_feedback.gd`, `verify_card_gains.gd`, `verify_battle_authority.gd`, and `git diff --check` passed. Existing saves/history were preserved; this UI change works with existing battles after restarting the client.

## Call the Mark chosen-face flip — September 27, 2026

The authority now publishes Call the Mark's physical die index, prior face, and selected face as an `offensive_face_set` feedback event. The existing feedback queue holds the previous displayed face, turns the actual tray die over once in 0.58 seconds, and briefly highlights the chosen result; total feedback is 1.15 seconds. Only the old and selected numbers appear. A small adjacent caption names Call the Mark, the die, and the before→after change. There is no rolling preview or additional card dialog. Progress survives redraws, pauses during inspection, and blocks automatic progression until finished. Cleanup restores the final face, scale, and orientation even if interrupted. Count, Entombment, roll attempts, and all other dice retain their existing rules.

Validation: `go test ./...` passed, including exact public metadata plus unchanged Count/Entombment assertions. The extended `verify_call_the_mark_targeting.gd` passed with a real native card play, all-timestamp checks against random/intermediate faces, old-face folding and chosen-face unfolding, untouched neighboring dice/authority data, redraw continuity, inspection pause, duplicate-event suppression, and early cleanup. Graphical captures inspected at 1280×720 and 1920×1080 in `.godot/mark-target-review/flip-*.png`. `verify_no_safe_keep_feedback.gd`, `verify_curse_dual_faces.gd`, `verify_battle_authority.gd`, and `git diff --check` passed. Existing saves/history were preserved; restart the client to use the new native feedback event and animation.

## Prevention saves-card animation — September 27, 2026

Spiteful Ward and the shared damage-prevention feedback now connect the played card to the specific released removal cards with green trails and SAVED badges. Those cards hold their original revealed positions, then gently shrink and travel toward the owning actor's authority-published destination counter (their current pile by default; discard for an explicit effect such as Protect). The counter pulses and a short label names the saved count and zone. Unsaved cards hold their slots until the saved cards leave, then slide into the remaining grid layout. The sequence lasts 2.6 seconds and gates automatic continuation; repeated events do not replay it, and redraws retain the captured positions and start time.

Saved cards remain in their current deck, discard, or hand under the default card-prevention rule; prevention never rewinds a card play or draw. Pending removals are still included in zone totals, so the animation does not add a false increase to those totals. The played reaction card still moves to discard normally. No game rules or saved state were changed.

Validation: the expanded `verify_damage_response_flow.gd` passed headlessly and graphically, including a legal native Spiteful Ward play against Brine Mask at 1920×1080 and 1280×720, exact saved-card origins, all three original-zone endpoints, held unsaved slots, visible travel, destination labels clear of profile text, unchanged authority counts, grid restoration, deduplication, and continuation timing. Graphical captures inspected in `.godot/ward-feedback-review/`. Focused Go tests `TestDamageRemovesMarkedReactionCardFromItsCurrentZone` and `TestCurseDefensiveCards`, `verify_fully_blocked_damage.gd`, `verify_battle_authority.gd`, and `git diff --check` passed. Restart the client to use this presentation update; existing battles remain usable.

## Count gain and Grave Interest conversion explanation — September 27, 2026

The workspace authority transcript confirmed the reported Round 3→4 sequence: Brine Mask lost three health (9→6), then D5 rolled its already-cursed face 4 during the post-damage Curse applications and gained one Count (3→4). At Effects, the previously played Grave Interest spent the first three Count on the next Income's Energy penalty, leaving one Count and dealing no Curse damage. Health remaining at six was correct. This older saved battle emits conversion mode metadata but lacks the newer explicit Grave Interest trigger event.

The Effects presentation now recognizes that legacy conversion and displays a purple Grave Interest trigger explaining the three Count spent, reduced next Income, retained Count, and damage amount. It does not invent a Grave Debt status for old pinned rules. The combat log also names this replacement. Count gain feedback holds the prior displayed Count until the actual cursed roll is revealed, then draws a purple trail from that die to the status and displays the before→after Count. Legacy post-damage rolls are scheduled between damage and Effects, with duplicate roll/mark feedback merged and the originating round retained. New Spiteful Ward follow-up events identify the source card and attack, so their animation can name the correct cause rather than Hexbrand. Gameplay calculations are unchanged.

Validation: a public-event fixture extracted from the exact recorded sequence drives `verify_curse_count_transition.gd`, passing headlessly and graphically at 1280×720 and 1920×1080. It verifies 9→6 health, staged 3→4 Count, die-to-status feedback, damage→Curse→Effects ordering, redraw continuity, and the legacy 4→1 conversion with zero damage. Graphical captures were inspected in `.godot/count-transition-review/`. Modern Grave Interest engine and presentation cases now explicitly include four Count. Full `go test ./...`, `verify_grave_interest_status.gd`, `verify_curse_conversion_counter.gd`, `verify_hexbrand_curse_sequence.gd`, `verify_no_safe_keep_feedback.gd`, `verify_battle_authority.gd`, and `git diff --check` passed. Existing saves, history, and the running game were preserved; restart the client for the updated presentation.

## Unquiet Hands inline targeting and separate check — September 27, 2026

Unquiet Hands now uses battlefield targeting instead of the card-choice dialog. The selected hand card stays outlined while legally eligible enemy dice pulse; clicking a die submits that exact legal action, including when only one target is available. Cancel/Escape spends nothing, redraws preserve selection, stale choices clear, and automatic continuation waits. The shared selector accepts the planning action's die index and target without requiring revealed offensive dice. Call the Mark retains its existing face-selection behavior.

A 2.6-second card-to-die cue now leads into a separate rolling preview, followed by the actual native symbol and Curse symbol on a hit. The original tray remains unchanged, whether unrevealed or showing saved offensive results. Hits travel from the preview to the Count status, which increments on arrival; misses explicitly say “Clean face · no Count.” The generic inferred Count-gain notice is suppressed for this card so it cannot reveal the reward before the roll. Card text and the inline hint explain the check, +1 Count on a cursed face, and unchanged offensive result; older pinned battles receive the same presentation clarification. New public roll events identify Unquiet Hands as their source. No gameplay rules, costs, or play windows changed.

Validation: `verify_unquiet_hands_feedback.gd` uses recorded responses from real native Mark the Number → Unquiet Hands plays (hit and miss), tests both 1280×720 and 1920×1080, card selection, legal target sets, cancellation, exact action submission, stale-choice removal, rolling frames, unchanged hidden/revealed dice, result symbols, timed Count arrival, inspection pause, redraws, duplicate suppression, and cleanup. Graphical captures inspected in `.godot/unquiet-review/`. The engine regression verifies saved offensive dice/history and face marks are unchanged and source/result metadata is published. Full `go test ./...`, `verify_call_the_mark_targeting.gd`, `verify_curse_feedback.gd`, `verify_curse_count_transition.gd`, and `verify_battle_authority.gd` passed. Existing workspace saves/history were preserved; restart the game to load the update.

## Second Knell status and visible retry — September 27, 2026

New battles apply Second Knell as a normal, cleanseable negative status capped at one. A physical cursed result consumes it and rerolls that same owned die once; each cursed hit adds Count, the final result is used, and the existing shared retry limit remains intact. Clean results leave it armed. Unused Second Knell expires after the next Effects. The card and status explain these rules. Older pinned catalogs retain their compatible preparation implementation; saved battles/history are preserved.

Public trigger events carry the original result, retry, Count baseline/final values, die identity, and roll context. Private offensive retry faces are retained internally until joint reveal, including across state clones. Presentation gives the retry its own short beat: original roll and Count arrival, status fade/trail, same-die retry, then a separate Count arrival or explicit clean miss. Separate checks preserve the saved offensive tray. When Unquiet Hands starts the check its card remains visible and points to the selected die. Paired owned-roll events do not animate twice, expiry never rolls, and retries precede subsequent Effects/Income. Combat log entries explain consumption, final face, and Count changes.

Validation: full `go test ./...` passed; focused Second Knell tests also verify normal status application/cap/expiry/cleanse, both retry outcomes across Curse/offensive/status streams, private-face retention, one-retry limits, and the Unquiet Hands cause. `verify_second_knell_status.gd` passed headlessly and graphically at 1280×720 and 1920×1080, including real native card application, sequential Count arrival, status fade and die linkage, hits/misses, unchanged extra-check offensive dice, inspection pause, redraw persistence, duplicate suppression, automatic continuation, and ordering before bundled Effects/Income. Captures inspected in `.godot/second-knell-review/`. Grave Interest, Unquiet Hands, No Safe Keep, Count transition, authority smoke, and whitespace checks passed. Restart the client to load updated native/presentation code; start a new battle for the new status definition rather than the old pinned preparation rules.

## Blind Omen / shared Blind check feedback — September 27, 2026

Blind's existing checkpoint already rolled and consumed the status correctly, but the client ignored the status dice event and had no explicit resolved-outcome event. The authority now includes the selected ability in the roll event, exposes the public pending Blind check in snapshots, and emits `blind_resolved` with the final reaction-adjusted face and actual cancellation result before advancing to defense. Gameplay thresholds and reaction opportunities remain unchanged.

The battlefield now presents a short Blind roll with a status-to-die trail, holds the authoritative face visibly during the real reaction window, then plays a separate confirmed-outcome beat. Faces 1–2 show a purple Blind pulse/slash, cancellation text, and the named ability changing to no offensive ability. Faces 3–6 show a dissipating pulse and explicitly retain the attack. Both explain Blind consumption. The result beat never rolls the die again. Header timing stays on Blind Check until feedback completes, auto-pass/opponent progression waits, ordinary redraws retain elapsed playback, inspection pauses it, and history has an explicit presentation-continue control. The combat log records the final face and outcome.

Validation: full `go test ./...` passed. Focused native tests cover all six faces, source metadata, result ordering, status consumption, public pending-check snapshots, and a reaction changing face 1 to 5. `verify_blind_feedback.gd` passed headlessly and graphically at 1280×720 and 1920×1080, checking rolling frames, provisional vs. confirmed outcomes, reaction-window visibility, cancellation/miss paths, automatic continuation, duplicate suppression, redraws, inspection pause, and viewport bounds. Screenshots inspected in `.godot/blind-feedback-review/`. Second Knell, Brine Surge, authority smoke, and whitespace checks passed. Restart the game to load the update; existing battles and pinned card rules remain usable.


### Malediction’s Refusal status — September 27, 2026

- New catalogs apply a normal negative, single-stack Malediction’s Refusal status alongside the card’s ordinary Curse application. It appears in the normal status list, can be removed through normal status removal, and prevents duplicate guards.
- Consume it before the next Curse Count cleanse attempt. Roll a random owned die; the final cursed result blocks the cleanse, while a clean result permits it. Count costs/conversion and face-mark removal do not trigger it. Second Knell retries are resolved before deciding the cleanse outcome.
- It survives Effects and expires at the next Income. Public trigger/expiry events drive an explicit status-consumption, die-check, and cleanse-result sequence; expiry has no invented roll. Combat log includes the outcome and Count before/after.
- Existing pinned catalogs retain legacy preparation rules. Restart and start a new battle to use the new status definition; saves/history are preserved.
- Validation: full Go suite, authority smoke, Refusal lifecycle/cleanse/retry tests, and graphical Refusal feedback checks at 1280×720 and 1920×1080.

### Number-choice dice fold-out — September 27, 2026

Shared Misfortune and Rotten Numeral now open a board-local six-face cube net beside the focused enemy dice. Ivory pips, dark face tiles, gold edges, and purple hover/card highlighting match the battle UI. All six faces fit without scrolling; unavailable legal choices are disabled. Choosing a face submits the existing authority action unchanged. The caption distinguishes choosing the number from the game's random selection of affected dice. Cancel, Escape, or clicking the selected card again dismisses the selection without spending resources. Selection pauses automatic progression.

Validated graphical layout at 1280×720 and 1920×1080, native submissions for both cards, exact resource costs and face marks, cancellation, stale legality, plus Call the Mark and Unquiet Hands regression checks. No rule/catalog change; existing battles can use this UI after restarting.

### Simultaneous multi-face Curse feedback — September 27, 2026

The shared Curse notice groups contiguous face applications from the same source and recipient into one animation, including card applications, defense effects, and attack follow-ups. All precise face-chip trails travel together, all new marks reveal on the same arrival, and the connections remain visible through the result hold. A grouped effect takes about 2.8 seconds total instead of 2.1 seconds per face. Multiple marks on one die retain separate endpoints. Rolled expansion checks have concurrent labeled previews and aggregate Count feedback; direct marks do not invent rolls. Distinct status triggers, deliberate face sets, and forced rerolls retain their own presentation.

Validated native Shared Misfortune/Rotten Numeral plays, five-face Eclipse feedback, mixed direct/rolled Hexbrand marks, redraw anchoring, viewport fit, single-beat completion, and defense/Second Knell regressions. This is presentation-only and works with existing battles after restarting.

### Curse expansion rolls on their physical dice — September 27, 2026

Curse-application checks now animate directly over the affected tray slots, including single expansion checks. Their results settle concurrently and short pink trails run from each die/result to its face chip directly below. The detached side-roll previews are removed for these effects. Repeated checks of the same physical die share the slot and retain separate result cells and trail origins. Direct marks stay unrolled. Temporary overlays leave saved offensive faces and game state intact and disappear with the shared animation.

Validated mixed direct/rolled applications, three concurrent results including two on one die, exact die/face anchoring, original offensive results, existing blank slots, viewport layouts at 1280×720 and 1920×1080, and Curse/number-choice/Unquiet Hands/defense regressions. Existing battles are supported after restart.

### Remaining dice-choice lists — September 27, 2026

- Confirmed Rotten Numeral and Shared Misfortune use the existing six-face cube net, including a native card play and authority validation. The old scrolling dialog in the report is no longer the current route.
- Widen the Crack, Chosen Instrument, and No Safe Keep now select highlighted physical tray dice. Grasp of the Sarcophagus and Misfortune Repaid use the same board interaction for their mandatory follow-up die choice.
- Eclipse of the Black Star and Widen the Crack's adjacent-number follow-up now use the six-face foldout with unavailable numbers dimmed. Mandatory choices cannot be canceled after their parent card/ability has committed.
- Call the Mark's face choices now use the same cross-shaped six-face net. Steady Hand and Forked Tongue also use tray selection followed by that net. Their legal authority choices are forwarded unchanged; opening/canceling optional targeting does not play a card.
- `verify_remaining_dice_choices.gd` covers the new routes, viewport bounds, stale choice rejection, absence of modal dialogs, and exact command submission. Existing number-foldout, Call the Mark (including native play/flip), and Unquiet Hands checks remain passing.
- Client-only presentation changes. Restart the running game to load them; existing battles remain compatible.

### Persistent Entombed dice — September 27, 2026

Entombment now has a gold chain frame on the physical tray die and a high-contrast ENTOMBED badge directly below it, replacing the easy-to-miss BOUND text beneath the face grid. The symbol, result, and all six Curse face chips stay readable. Newly applied bindings close around the die over 0.85 seconds; the screen retains the application timestamp across redraws. Previously saved bindings load settled. The marker remains on hidden/blank results and while rolling, and disappears when the authoritative owned die is released. Tooltips explain forced rerolls, subset priority, and the cursed-roll release condition. No gameplay rules changed.

Verified all 64 Curse-face combinations, both player/enemy trays at 1280×720 and 1920×1080, exact-die application, redraw timing, release cleanup, layout bounds, and existing targeting/Call the Mark regressions. Existing battles are compatible after restarting the client.

### Widen the Crack: die roll to adjacent face — September 27, 2026

Widen the Crack keeps its highlighted tray-die selection and now animates the check on the chosen physical die. A check never draws a face-placement trail or changes saved offensive results. Clean/cursed outcomes and any Count gain are shown with the roll; duplicate generic Count-gain feedback is suppressed. The follow-up uses only the eligible face chips beneath that same die, replacing the separate number foldout for this card. Gold-edged, purple-glowing chips distinguish selectable faces from existing marks. One eligible face highlights briefly and submits automatically; two await an exact legal click. The purple die-to-face trail appears only when the new adjacent mark is actually applied. A check with no eligible adjacent face explains that outcome and continues without a picker.

Validated native clean/cursed card plays and one/two adjacent options, authority acceptance, preservation of offensive values and existing marks, selector timing and exact chip geometry, automatic selection, and absence of a false placement trail. Graphical checks passed at 1280×720 and 1920×1080 (screenshots in `.godot/widen-review`). Remaining dice-choice, shared Curse feedback, Hexbrand, and Unquiet Hands regressions passed. Presentation-only; existing battles are compatible after restarting.

### Black Dividend enemy status — September 27, 2026

New catalogs apply Black Dividend as a real, negative, single-stack enemy status. Source ownership, reward limits, and expiry are stored against that status instance; removing it disables rewards, and an unrelated replacement instance cannot inherit the old caster. The first physical cursed result each round grants the caster 1 Energy, with at most two rewards total. The status remains after its first reward, is consumed after its second, and otherwise expires after the next Effects. Direct face changes do not trigger it. Old pinned catalogs retain their preparation rules for compatibility.

Public trigger events identify the cursed die, recipient, Energy before/after, reward number, and consumption. Private offensive faces are held until joint reveal. The client lists Black Dividend among enemy statuses, then highlights the triggering die and status, sends a purple trail to the caster's Energy counter, and states whether the status remains or is consumed. Expiry has explicit feedback without an invented roll. Card resource feedback suppresses duplicate Dividend rewards. The combat log records trigger and expiry details.

Validation: full Go suite, native authority smoke, native card application, status/trigger/expiry layouts at 1280×720 and 1920×1080, roll-before-reward ordering, duplicate event suppression, and shared Curse feedback passed. Focused rules checks cover clean results, same-round limits, second reward consumption, Effects expiry, removal, instance ownership, save cloning, private reveal, and old catalog compatibility. Screenshots are in `.godot/dividend-review/`. Restart and start a new battle for the new catalog; existing saves and developer history are preserved.

### Curse Bloom enemy status — September 27, 2026

Curse Bloom now applies a real negative, single-stack enemy status in new catalogs. At the next Curse Count damage resolution it consumes before any follow-up roll. If that damage actually removes health and cursed dice exist, it makes the existing three sequential Lesser Expand attempts (fully cursed dice Surge). Blocked/released damage and an empty cursed-dice pool produce explicit no-expansion feedback. Untriggered statuses expire after the next Effects; removing the status stops its trigger. Existing pinned catalogs preserve legacy preparation rules.

The client shows Curse Bloom in the ordinary enemy status list, animates consumption/expiry, and ties follow-up rolls and face marks to the status and conversion damage source. All three expansion attempts use one concurrent dice/face animation; the status cue precedes them. New Count remains for the following Effects. Conversion follow-up presentation retains the Effects segment rather than showing Damage Resolution. The combat log records consumption, damage, attempts, and expiry.

Validation: full Go suite, authority smoke, native card application, status/blocked/no-dice/expiry visuals at 1280×720 and 1920×1080, grouped three-roll feedback, duplicate suppression, and Hexbrand regression passed. Rules tests cover single-stack application, cloning, removal, expiry, accepted versus blocked/released damage, ordinary attack exclusion, one-shot consumption, fully cursed Surges, and old catalog compatibility. Screenshots are in `.godot/bloom-review/`. Restart and start a new battle to load the new status catalog; existing saves/history remain intact.
