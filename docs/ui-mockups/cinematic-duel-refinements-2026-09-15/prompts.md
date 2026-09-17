# Prompts

Tool: built-in image_gen, reference edit workflow.

Reference: ../minimal-offensive-2026-09-15/01-cinematic-duel.png

## Shared prompt

Use case: ui-mockup; edit/refine the supplied Cinematic Duel game screenshot.
Make ONE widescreen 16:9 finished screenshot, not a collage. Preserve the reference's cinematic side-on ruined cathedral, fog, restrained blue-grey and brass palette, small Venom and Blade Warden full-body characters on opposite sides, character positions, health/status placement, and generous empty central battlefield. This must look like the same game and same scene, with an improved lower UI, not a different visual direction.
User changes: completely remove the lower-left Your Piles panel and ALL visible Deck, Hand, Discard, Removed counters. Also remove Enemy Piles from right utilities. Keep the five actual hand cards. Keep full ability effects hidden until hover, but ALWAYS show exact dice recipes directly on all four ability controls so the player knows what to roll for.
Battle state: Venom 7/24 HP, energy 1, Bleed 2. Blade Warden 12/20 HP, energy 5, Poison 2. Use same diamond energy symbol on both sides; Bleed should be a clear blood drop not flame. Small concise status labels allowed. Top track Effects · Income · Offensive · Defensive · Damage; Offensive highlighted; Round 4 · Planning.
Four named abilities, in this exact left-to-right order with these REQUIRED legible dice targets:
"Needlefang" with "3 / 4 / 5 Fang" (three alternative tiers, NOT a total sum).
"Venom Gland" with "2 Gland + 1 Coil".
"Fever Spike" with TWO recipe lines: "2 Fang + 1 Gland + 1 Coil" and "2 Fang + 2 Gland + 1 Coil". Both must be visible and clearly alternative tiers.
"Terminal Bite" with "3 Coil".
Use Fang icon shaped like a curved tooth, Gland icon like a venom sac, Coil icon like a spiral; consistent with the recipe names. Recipe typography is readable ivory text, not microscopic dim lettering. Ability icon can shrink to make room for recipes. A small information mark or unobtrusive footer "Hover abilities for details" signals full hover details. Do not print full effect paragraphs on idle ability controls.
Five small hand cards at bottom with art, names and costs unchanged: Extract 0, Incubate 0, Coagulate 1, Shock Dose 2, Measured Dose 2. Restrained "Roll 5 dice" action and "3 rolls left" plus smaller "Skip", unless this variant explicitly specifies a rolled state. Right-side utility controls reduced to a small "Enemy abilities" button, "Log", "Inspect" and gear; no giant menu column.
Most of center stays open for future dice, bleed and tearing-card animations; no floating random dice in pre-roll samples. No pile counters anywhere, no giant character portraits, no new HUD panels across the center, no promotional title, no watermark. Modify only UI design and the explicit variant state; preserve cinematic battlefield art and overall composition closely.

## Medallions with recipes

Variant 1: closest refinement to original. Retain four circular ability medallions but make them about 25% smaller; spread across a wider horizontal strip just above the hand. Each has its name and complete text recipe below; Fever Spike has two smaller but readable lines with an "or" between them. Needlefang uses three separate tiny tier chips "3 Fang", "4 Fang", "5 Fang". Keep the original small centered five-card hand. The former piles area becomes quiet bare cathedral foreground, not a replacement panel. Idle pre-roll state, no tooltip.

## Compact recipe tiles

Variant 2: Replace circular medallions with four slim horizontal ability tiles in one spacious row immediately above the cards, each tiny icon on left, name and full dice recipe on right. Fever Spike tile slightly wider to fit both alternative recipe lines; no large empty boxes. Thin brass separators and soft charcoal translucent backgrounds only. Needlefang's target reads "3 / 4 / 5 Fang". The action Roll 5 dice and 3 rolls left sits at lower left, in part of the space freed by removing piles. Tiny utility buttons lower right. Keep five-card hand centered. Idle pre-roll, no tooltip. Elegant practical typography and strongest recipe readability.

## Symbol recipe strips

Variant 3: show dice targets primarily as small clearly distinct tooth/sac/spiral symbols grouped under each ability, with readable text labels too. Four shallow centered ability plaques above the hand. Needlefang presents three compact alternatives labeled "3 Fang", "4 Fang", "5 Fang"; Venom Gland shows two sac glyphs plus one spiral and text "2 Gland + 1 Coil"; Fever Spike shows two alternative rows of glyphs alongside text "2 Fang + 1 Gland + 1 Coil" and "2 Fang + 2 Gland + 1 Coil"; Terminal Bite three spirals and "3 Coil". Use neat consistent pictographic die-face badges, no actual rolled dice in the center. All icons are subdued ivory, no ready highlights yet. Small dice-symbol key along lower left "Fang · Gland · Coil". Compact Roll 5 dice at lower right. Keep the battlefield and cards as original.

### Targeted correction

Precise UI correction to this screenshot; preserve all environment, characters, card art, UI positions and text except the recipe glyphs inside Fever Spike. Its illustrated glyph recipes currently omit Coil. Replace the Fever Spike glyph groups with compact COUNT TIMES ICON badges. First row must show "2×" tooth icon, "+" "1×" venom sac icon, "+" "1×" spiral icon. Second row must show "2×" tooth icon, "+" "2×" venom sac icon, "+" "1×" spiral icon. Keep the accompanying exact text recipes readable: "2 Fang + 1 Gland + 1 Coil" and "2 Fang + 2 Gland + 1 Coil". You may use two lines per tier within the existing tile for clarity. Preserve all other ability recipes and screenshot. This is important: both Fever Spike rows MUST visibly include a spiral Coil symbol. Do not change any rules.

## Needlefang hover details

Variant 4: same cinematic scene and a refined slim four-tile recipe bar above the cards; ALL FOUR required recipes remain visible, including both Fever Spike tiers. Demonstrate hovering Needlefang: subtle cursor and muted brass highlight on the leftmost ability, plus ONE neatly typeset narrow dark tooltip immediately above it, positioned in the lower-left portion of the battlefield, away from both characters and not spanning the middle. Tooltip exact text:
"Needlefang"
"Choose a qualified tier"
"3 Fang → 4 damage + 1 Poison"
"4 Fang → 3 damage + 2 Poison"
"5 Fang → 2 damage + 3 Poison"
"Poison overflow applies Incubation if none exists."
"0 energy · Once per segment · One enemy"
The tooltip is transient and about 25% screen width; size text legibly. Small quiet "Pin details" affordance. It shows the full base ability detail faithfully, no invented bonuses, and does not cover the recipes or hand. Pre-roll state with 3 rolls left. Rest of central area remains empty.

## After-roll matching

Variant 5: demonstrate the same cinematic interface just after the FIRST roll, so phase label is "Round 4 · Offensive". Exactly FIVE dice sit in one modest row in the lower-center battlefield ABOVE the ability bar: Fang, Fang, Fang, Gland, Coil; each has a tooth, sac or spiral symbol and small clear label. No ordinary numbered-pip dice. Three Fang dice subtly marked "Held"; other two unheld, with tiny lock affordances. Roll button becomes "Reroll 2 dice" with "2 rolls left". Four slim ability tiles below dice still show ALL exact target recipes from the shared spec. Needlefang's "3 Fang" tier is highlighted softly with "Ready", while 4 Fang and 5 Fang stay dim. Fever Spike's "2 Fang + 1 Gland + 1 Coil" base row is softly highlighted "Ready"; its 2-Gland row remains dim. Venom Gland is unqualified and has tiny "Need 1 Gland"; Terminal Bite is unqualified and has tiny "Need 2 Coil". Muted warm-gold border and checkmarks, not bright neon. Do not select or execute an ability yet. HP and effects unchanged. Five hand cards remain below. Keep remaining middle/upper center open; no huge dice tray rectangle.

### Targeted correction

Precise small UI corrections only; preserve this screenshot's composition, characters, cards, battlefield, five dice and all recipe text. In Needlefang's unqualified "4 Fang" and "5 Fang" rows REMOVE their grey checkmarks and use an empty small hollow circle instead. In Fever Spike's unqualified "2 Fang + 2 Gland + 1 Coil" row REMOVE its grey checkmark and use an empty small hollow circle. Only the gold Ready rows should have checkmarks. Change the padlock beneath Gland die and beneath Coil die to visibly OPEN padlocks (unheld); leave the first three Fang dice Held and locked. All text, recipe values, Roll 2 / 2 rolls left, health etc otherwise unchanged. No other edits.

