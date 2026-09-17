# Dark watercolor wasteland battle UI

Implemented from `dark-watercolor-wasteland-left-dice-v2.png`. Actual rendered preview: `wasteland-battle-implemented-2026-09-15.png`.

## Live layout

- Dark watercolor wasteland, apothecary and bone-armored opponent; neutral empty arena with the actual character portraits for other matchups.
- Compact upper-corner health, energy, card-zone and status HUDs.
- Parchment round label and five live phase indicators.
- Left column: five live dice, roll/reroll and Skip, then compact parchment abilities. Needlefang's three tiers retain individual legality and click targets. Fang, Gland and Coil have matching vector icons, with numeric die values preserved.
- Five portrait cards fit the bottom hand; additional cards remain horizontally scrollable. Costs, rules, card targeting and disabled states still come from the authority.
- Enemy abilities, Log, Inspect and Settings along the lower right. History joins that row when enabled. Disable auto-pass remains in Settings.
- Defense, Effects, damage and card/status animations retain their existing command and timing behavior. Long results scroll independently from the left action controls.
- Native button labels and inspection IDs preserved, including the full accessible Skip label. No card rules or RNG changes.

## Validation

Passed the cinematic layout test at 1920×1080, 1280×720 and 1600×1000, including real pointer clicks for Settings, auto-pass toggle and individual Needlefang tiers. Added non-overlap checks for the left dice/roll/ability column and separate roll/skip targets.

Passed battle-scene interactions, Needlefang tiers, automatic pass, quiet offensive reactions, native offensive roll/reroll/skip controls for both characters and seats, defense-result animation, automatic Effects and damage-response flow. Rendered screenshots inspected for planning, defense, Effects and damage. Enemy-only defense and Antidote feedback also checked.

The native offensive-control test now explicitly pumps the model worker while holding automatic acknowledgements; its previous frame timing could race past the reaction stage being inspected. A quiet skipped-offense handoff now retains its explanatory hint.

## Assets and generation

Built-in image-generation tool used (not the CLI). Assets are saved in `dice-and-destiny-client/assets/battle/wasteland/`: `duel.png`, `arena.png`, `card_illustrations.png`, `parchment.png`. `fang.svg`, `gland.svg` and `coil.svg` are native vector UI assets. The six atlas illustrations are category art reused by related cards; they are not unique art for every card. No controls or status values are baked into the background. Original reference and previous cathedral artwork are preserved.

### Prompt: duel background

Edit this reference into a production game background plate, widescreen 16:9. Preserve exactly the dark watercolor ink wasteland scene, fractured slate and pale cracked earth, lanterns, two figures and their positions: small ragged apothecary survivor at x36% feet y64%, towering bone armored sword adversary at x73% feet y49%. REMOVE ALL user interface everywhere: all text, headings, health bars, numbers, icons, phase timeline, dice, buttons, parchment ability panels, playing cards, bottom utility controls. Fill all those removed areas with continuous natural wasteland painting. Top 15% dark almost black rocky atmospheric sky; bottom dark rough charcoal slate. Retain open arena center, leftmost22% clear of characters to place live dice and abilities. NO lettering, NO cards, NO UI, NO dice, NO terrain faces, NO skull-shaped rocks. Actual character mask allowed. Match source style and composition faithfully. Save usable background art.

### Prompt: card atlas

Create a production game art atlas based on reference's card art. EXACT 3 columns by 2 rows uniform grid, cells fill entire image with no gutters, each cell a separate portrait illustration, no borders no text no labels no numbers. Dark watercolor and ink on weathered parchment, muted bone gray charcoal olive green burgundy. Top-left surgical hook and fang dagger on cloth (Extract/Deep Puncture), top-middle green glass poison vial with brass stopper and root (Incubate), top-right scales with hanging green vials (Measured Dose). Bottom-left bone-handled dagger with lightning (Shock Dose), bottom-middle red thorny coagulated heart (Coagulate), bottom-right healing antidote glass vial with pale glowing liquid. Objects centered in each equal cell and never cross boundaries. High detail tactile watercolor texture, grim survival fantasy, match reference.

### Prompt: empty arena

Edit this game background plate: remove both characters completely, the left apothecary and the right giant bone armored figure, including their weapons, shadows and equipment. Fill naturally with continuous pale cracked slate wasteland. Preserve all other scene composition lighting dark watercolor ink texture, black sky, mineral spires, lanterns and open center. Empty environment only, no people no monsters no faces no UI no text. Same widescreen aspect.

### Prompt: parchment texture

Production UI texture asset: one blank rectangular aged ivory parchment panel fills entire wide 3:1 image. Dark watercolor survival fantasy, soft beige off-white center with subtle natural paper fiber mottling, thin irregular charcoal weathered border and chipped dark taupe edges. Center very light and even for black text readability. Straight rectangle not rolled scroll. No text no letters no icons no symbols no decorations no objects. Edge width approximately 3% of height, fine restrained distress. Entire image is this single texture, no background outside panel.
