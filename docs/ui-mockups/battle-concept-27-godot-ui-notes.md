# Battle Concept 27 UI Notes

Reference image: `battle-concept-27-mirrored-player-profile.png`

This mockup represents the preferred battle screen direction for the Godot UI. The screen uses a dark fantasy board-game style with stone panels, gold trim, blue player accents, and orange enemy accents. The goal is to show all important combat information without turning the screen into a text-heavy dashboard.

## Overall Layout

The screen is split into three main areas:

- Left column: player-focused information.
- Center playfield: round/segment state, ability options, battle focus, and player hand.
- Right column: enemy-focused information and combat history.

The left and right columns mirror each other visually so the player can quickly compare their state against the selected enemy.

## Top Bar

The top bar shows the battle segments in order: Effects, Income, Offensive, Defensive, and Damage. The active segment is highlighted. The bar also includes the current round, such as `Round 3 - Offensive`.

## Left Player Column

The upper-left box shows the player's card zones as four compact tiles:

- Deck count.
- Hand count.
- Discard count.
- Removed count.

Below that is the player profile box. It should match the internal layout of the selected enemy profile box:

- Player portrait on the left.
- Player name/title.
- Current health number and health bar.
- Status effect rows with icon, name, and count.
- Current energy shown as a side stat/gem.

Below the profile is the player dice command box:

- Five offensive dice are displayed.
- Each die face has a symbol and a small number in the upper-right corner.
- Roll and Pass buttons sit in the same box.
- Rolls used and rolls left are shown near the controls.

The bottom-left box is intentionally blank for now. It should reserve space for a future feature while keeping the left column balanced with the right column combat log.

## Center Playfield

The center keeps the battle focus visually open with a circular rune/sigil area. It can show warnings, targeting, incoming attacks, or selected ability feedback without covering the main UI.

Enemy abilities sit across the upper center. Player abilities sit across the lower center. Each side supports 10 visible abilities. Ability tiles should be compact and mostly icon-driven:

- Ability number/name.
- Main symbol or icon.
- Required dice/symbol combination.
- Examples include pair, two pair, three of a kind, full house, small straight, large straight, five sixes, sword plus eye, shield pair, and fire plus shield.

The player hand is shown along the bottom center as overlapping cards. Cards should show:

- Energy cost.
- Card name.
- Card art.
- Bottom symbol/effect strip.

## Right Enemy Column

The upper-right box lists all enemies in the encounter. Each enemy entry includes portrait, name, health number/bar, and selection state. The currently selected enemy is highlighted.

Below that is the selected enemy profile box:

- Enemy portrait.
- Enemy name/title.
- Current health number and health bar.
- Status effect rows with icon, name, and count.
- Current energy shown as a side stat/gem.

Below the profile is the enemy revealed dice box:

- Five revealed enemy dice.
- Each die face has a symbol and a small number in the upper-right corner.
- The selected enemy ability and required dice/symbol combo are shown beneath the dice.

The bottom-right box is the combat log. It uses compact icon rows to show recent combat events, such as damage, healing, shields, resource changes, dice results, and a short final event line.

## Godot Setup Notes

Build the screen as a fixed battle HUD with three columns: left panel, flexible center, and right panel. The left and right panel boxes should share matching dimensions where possible. The center should scale wider on large screens, while the side columns stay readable and stable.

Use reusable scene/components for:

- Actor profile box.
- Dice tray.
- Ability tile.
- Card zone tile.
- Enemy roster entry.
- Combat log row.
- Hand card.

The actor profile component should be shared by player and enemy views, with different accent colors and data bindings.
