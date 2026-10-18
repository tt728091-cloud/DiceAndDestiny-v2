# Dice Throne ability-board layouts by die layout

Research toward ability-board templates for Dice and Destiny. It compares
**what each ability costs to activate**, not what it does, across all 40 heroes
in `dice-throne-reference.csv`, grouped by the order of symbols on their die.

Regenerate the data with
`python3 -I analysis/ability_layouts.py > analysis/layouts.json`.

## Notation

- **Die layout** lists how many faces each symbol has, in face order.
  `3-2-1` means faces 1–3 are one symbol, 4–5 a second and 6 a third.
- **Slots** name the symbols in face order: **A** is the symbol on face 1,
  then B, C and D. The symbol on face 6 is always the rarest; we call it the
  **apex** symbol (C on three-symbol dice, D on four-symbol dice).
- `A3` means three A symbols. `A2+C2` means two A and two C at the same time.
  `A3 / A4 / A5` is a **ladder**: one ability with tiers that get stronger with
  more matching dice. `SS` and `LS` are the small and large straight.
  **Rainbow** means one of each symbol (`A1+B1+C1+D1`).
- **Odds** are the chance of reaching the cheapest tier with 5 dice and
  3 roll attempts, holding dice in the best way for that one ability. This
  matches our battle rules (3 rolls). Odds depend only on **how many faces each
  symbol has**, not on where those faces sit on the die.

Hero forms that share one board layout are counted once: Pale Lady/Werewolf and
Cursed Pirate front/back have the same costs on both sides.

## The die layouts

| Layout | Heroes | Who |
|---|---|---|
| 3-2-1 | 26 | Pale Lady, Raveness, Alchemist, Headless Horseman, Necromancer, Cursed Pirate, Gunslinger, Artificer, Tactician, Samurai, Vampire Lord, Wolverine, Storm, Iceman, Cyclops, Rogue, Jean Grey, Miles Morales, Thor, Treant, Ninja, Moon Elf, Barbarian, Captain Marvel, Black Panther, Santa |
| 3-1-1-1 | 6 | Seraph, Gambit, Scarlet Witch, Pyromancer, Deadpool, Krampus |
| 2-2-1-1 | 6 | Huntress, Psylocke, Loki, Doctor Strange, Shadow Thief, Paladin |
| 2-3-1 | 1 | Black Widow |
| 2-1-2-1 | 1 | Monk |

Our `battle_v1` standard die is 3-2-1: Sword (1–3), Shield (4–5), Gold coin (6).

**Five orders, three families.** Because odds depend on face counts only,
2-3-1 plays like 3-2-1 with the common symbol moved to the middle (Black
Widow's ladder is on her 3-face Batons), and 2-1-2-1 plays like 2-2-1-1 (Monk
has two 2-face symbols and two 1-face symbols). The order still matters for
straights (which symbols a straight contains, so what you give up to chase
one) and for number-based effects such as "set a die to 6". It does not change
the odds of symbol costs. So a template can be written once per family and
reused with any symbol order:

| Family | Layouts | Heroes |
|---|---|---|
| {3,2,1} | 3-2-1, 2-3-1 | 27 |
| {3,1,1,1} | 3-1-1-1 | 6 |
| {2,2,1,1} | 2-2-1-1, 2-1-2-1 | 7 |

## What every board shares

Almost every board is a fixed **core** plus 2–4 **flex** abilities.

| Core piece | How common | Odds |
|---|---|---|
| Ultimate: apex ×5 | 40 / 40 | 1.3% |
| Apex ×4 | 39 / 40 (not Shadow Thief, who has apex ×3) | 10% |
| Small and large straight | 40 / 40. 17 combine them into one tiered ability, 23 split them | SS 62%, LS 26% |
| Ladder on the most common symbol | 36 / 40 as ×3/×4/×5; Deadpool uses ×4/×5; Alchemist, Doctor Strange and Paladin have none | 3-face: 98% / 88% / 51%. 2-face: 84% / 54% / 17% |
| One defensive ability | 38 / 40 (Miles Morales and Shadow Thief have two) | Dice: 3d ×17, 4d ×11, 5d ×7, 1d ×3, 2d ×3, 1–5d ×1 |

Board size is 7 or 8 offensive abilities, ultimate included. A combined
straight frees a slot, so a combined board with 7 abilities usually has the
same number of flex abilities as a split board with 8.

### Flex difficulty mix

Sorting each flex ability by odds (**Easy** ≥ 75%, **Medium** 50–75%,
**Hard** < 50%) shows a clear habit: the most common mix in {3,2,1} is **one
Easy, one Medium and one Hard** (8 heroes). Then three Medium (5 boards with
Easy+Medium+Medium or Medium×3), two Medium (Gunslinger, Samurai, Artificer)
and a Hard-heavy mix (Moon Elf, Santa, Iceman, Headless Horseman).

In 3-2-1, abilities that use only the 2-face B symbol (`B3`, `B4`, `B3+C1`,
`B2+C2`, `B3+C2`) are about half pure utility (heal, draw, gain resources),
and the rest mostly small undefendable or rolled damage. B is the "engine"
symbol; A carries the main damage and C the big hits.

## How many combinations exist

- 40 heroes have **39 distinct offensive boards**. Only Gunslinger and Samurai
  have exactly the same costs.
- The variety is almost all in the flex abilities. Distinct flex patterns:
  **18** in 3-2-1, **11** in 3-1-1-1 and **13** in 2-2-1-1.
- The core comes in only a few shapes (straights split or combined; one, two or
  no ladders), shown under the templates below.

## Flex menus by family

Odds for each cost on its own family's die. "Used by" counts heroes.

### {3,2,1} — A = 3 faces, B = 2 faces, C = 1 face (apex)

| Cost | Odds | Used by | Notes |
|---|---|---|---|
| A1+B1+C1 | 90% | 1 | Thor |
| B3 | 84% | 5 | Usually utility |
| A3+C1 | 80% | 1 | Necromancer |
| A1+B3 | 79% | 4 | |
| A1+B2+C1 | 77% | 5 (+ Black Widow) | |
| A3+B2 | 66% | 3 (+ Black Widow, A/B swapped) | Full house |
| B3+C1 | 60% | 5 | Usually utility |
| A2+C2 | 58% | 11 | Most common flex |
| A1+B1+C2 | 58% | 1 | |
| A2+B2+C1 | 57% | 2 | |
| A2+B3 | 56% | 11 | Full house, the other way |
| B4 | 54% | 5 | |
| A4+C1 | 51% | 2 | |
| B2+C2 | 47% | 2 | |
| A2+B1+C2 | 40% | 1 | |
| A3+C2 | 35% | 11 | The standard Hard finisher |
| A1+C3 | 31% | 4 | |
| B3+C2 | 21% | 1 | |
| Second ladder B3/B4/B5 | 84% / 54% / 17% | 1 (Barbarian; Necromancer uses B3/B5) | |

### {3,1,1,1} — A = 3 faces, B, C, D = 1 face each (D is the apex)

| Cost | Odds | Used by |
|---|---|---|
| A3+B1 / A3+D1 | 80% | 2 |
| A2+B1+D1 | 71% | 1 |
| C2 (one 1-face pair) | 70% | 2 |
| A1+B2 | 67% | 1 |
| **Rainbow A1+B1+C1+D1** | 61% | **5** |
| A2+B2 / A2+C2 (two pair) | 58% | 4 |
| C2+D1 | 54% | 1 |
| C3 | 35% | 2 |
| A1+D3 | 31% | 1 |

The rainbow is the signature of this family: four symbols make "one of each"
a 61% ability, and 5 of 6 heroes use it.

### {2,2,1,1} — A, B = 2 faces each, C, D = 1 face each (D is the apex)

| Cost | Odds | Used by |
|---|---|---|
| B2/B3/B4 ladder | 97% / 84% / 54% | 1 (Shadow Thief) |
| A2+B2 | 79% | 1 |
| C2 | 70% | 4 |
| Rainbow A1+B1+C1+D1 | 65% | 3 |
| A3+C1, A3+D1, B3+D1 (triple + kicker) | 60% | 4 |
| B4 | 54% | 1 |
| A2+C2, B2+D2 | 47% | 3 |
| A2+B2+C1, A2+B2+D1 | 43% | 2 |
| A3+B2 (full house of the two 2-face symbols) | 42% | 1 |
| D3 | 35% | 1 |

Monk (2-1-2-1) in the same terms: Zen (2-face) ×3 84%, Fist ×3 + Palm 60%,
Palm (1-face) ×3 35%.

## Every board

Flex abilities exclude the ladder, straights, apex ×4 and the ultimate.

### 3-2-1

| Hero | Symbols | Ladder | Straights | Flex abilities (odds) | Apex ×4 | Ult | Defense |
|---|---|---|---|---|---|---|---|
| Pale Lady | A=Slash · B=Crescent · C=Full Moon | A3/A4/A5 | SS/LS (one) | B3+C1 60%; A2+C2 58%; B4 54% | ✓ | ✓ | 4d |
| Raveness | A=Talon · B=Wing · C=Raven Eye | A3/A4/A5 | SS + LS | B4 54%; A2+B3 56%; A2+C2 58% | ✓ | ✓ | 5d |
| Alchemist | A=Solution · B=Composition · C=Crucible | — | SS/LS (one) | A3+B2 66%; B3 84%; A1+C3 31%; A4+C1 51% | ✓ | ✓ | 5d |
| Headless Horseman | A=Axe · B=Horseshoe · C=Scare | A3/A4/A5 | SS/LS (one) | B3+C1 60%; A3+B2 66%; A3+C2 35% | ✓ | ✓ | 1–5d |
| Necromancer | A=Scythe · B=Bones · C=Soul | A3/A4/A5, B3/B5 | SS/LS (one) | A3+C1 80%; A1+B1+C2 58%; B3+C2 21% | ✓ | ✓ | 4d |
| Cursed Pirate | A=Cutlass · B=Booty · C=Skull | A3/A4/A5 | SS/LS (one) | B3 84%; A1+B2+C1 77%; A1+C3 31% | ✓ | ✓ | 4d |
| Gunslinger | A=Bullet · B=Dash · C=Bullseye | A3/A4/A5 | SS + LS | A2+C2 58%; A2+B3 56% | ✓ | ✓ | 1d |
| Artificer | A=Wrench · B=Gear · C=Bolt | A3/A4/A5 | SS + LS | B3+C1 60%; A2+B3 56% | ✓ | ✓ | 4d |
| Tactician | A=Saber · B=Flag · C=Medal | A3/A4/A5 | SS + LS | A2+C2 58%; A1+B3 79%; A3+C2 35% | ✓ | ✓ | 4d |
| Samurai | A=Katana · B=Kabuto · C=Rising Sun | A3/A4/A5 | SS + LS | A2+C2 58%; A2+B3 56% | ✓ | ✓ | 3d |
| Vampire Lord | A=Claw · B=Gaze · C=Droplet | A3/A4/A5 | SS + LS | B3 84%; B3+C1 60%; A3+C2 35% | ✓ | ✓ | 3d |
| Wolverine | A=Claw · B=Adamantium · C=Mask | A3/A4/A5 | SS/LS (one) | B3+C1 60%; A2+C2 58%; A2+B3 56%; A3+C2 35% | ✓ | ✓ | 3d |
| Storm | A=Wind · B=Rain · C=Cyclone | A3/A4/A5 | SS/LS (one) + Largest Straight (6 dice) | A1+B2+C1 77%; A2+B3 56%; A3+C2 35% | ✓ | ✓ | 5d |
| Iceman | A=Ice · B=Slide · C=Frost | A3/A4/A5 | SS + LS | B4 54%; A1+C3 31%; A2+B3 56% | ✓ | ✓ | 3d |
| Cyclops | A=Fist · B=Pouch · C=Blast | A3/A4/A5 | SS + LS | A4+C1 51%; A1+B3 79%; A3+C2 35% | ✓ | ✓ | 3d |
| Rogue | A=Touch · B=Soar · C=Ion | A3/A4/A5 | SS/LS (one) | A2+C2 58%; A1+B3 79%; B2+C2 47% | ✓ | ✓ | 4d |
| Jean Grey | A=Astral · B=Control · C=Phoenix | A3/A4/A5 | SS/LS (one) | B3 84%; A1+B2+C1 77%; A3+C2 35% | ✓ | ✓ | 2d |
| Miles Morales | A=Thwip · B=Web · C=Spider | A3/A4/A5 | SS/LS (one) | A2+C2 58%; A2+B3 56%; A1+B2+C1 77% | ✓ | ✓ | 3d, 2d |
| Thor | A=Hammer · B=Worthy · C=Thunder | A3/A4/A5 | SS + LS | A1+B1+C1 90%; A2+B3 56%; A3+C2 35% | ✓ | ✓ | 3d |
| Treant | A=Branch · B=Leaf · C=Spirit | A3/A4/A5 | SS + LS | B2+C2 47%; A2+B3 56% | ✓ | ✓ | 3d |
| Ninja | A=Ninjatō · B=Shuriken · C=Mask | A3/A4/A5 | SS + LS | B4 54%; A3+B2 66%; A1+B2+C1 77% | ✓ | ✓ | 3d |
| Moon Elf | A=Arrow · B=Foot · C=Moon | A3/A4/A5 | SS + LS | A3+C2 35%; A2+B3 56%; A1+C3 31% | ✓ | ✓ | 5d |
| Barbarian | A=Sword · B=Life · C=Pow | A3/A4/A5, B3/B4/B5 | SS + LS | A2+C2 58%; A3+C2 35% | ✓ | ✓ | 3d |
| Captain Marvel | A=Photon · B=Ascend · C=Hala Star | A3/A4/A5 | SS + LS | A2+C2 58%; A1+B3 79%; A2+B1+C2 40% | ✓ | ✓ | 4d |
| Black Panther | A=Slash · B=Vibranium · C=Panther | A3/A4/A5 | SS/LS (one) | A2+C2 58%; B3 84%; A2+B2+C1 57% | ✓ | ✓ | 3d |
| Santa | A=Axe · B=Bell · C=Star | A3/A4/A5 | SS + LS | A3+C2 35%; B4 54%; A2+B2+C1 57% | ✓ | ✓ | 4d |

### 2-3-1

| Hero | Symbols | Ladder | Straights | Flex abilities (odds) | Apex ×4 | Ult | Defense |
|---|---|---|---|---|---|---|---|
| Black Widow | A=Espionage · B=Batons · C=Widow | B3/B4/B5 | SS + LS | A2+B1+C1 77%; A3+B2 56% | ✓ | ✓ | 3d |

### 3-1-1-1

| Hero | Symbols | Ladder | Straights | Flex abilities (odds) | Apex ×4 | Ult | Defense |
|---|---|---|---|---|---|---|---|
| Seraph | A=Blade · B=Wings · C=Cross · D=Angelic Pendant | A3/A4/A5 | SS + LS | A3+B1 80%; C2+D1 54%; A1+B1+C1+D1 61% | ✓ | ✓ | 1d |
| Gambit | A=Staff · B=Heart/Diamond · C=Spade/Club · D=Super Charge | A3/A4/A5 | SS/LS (one) | A1+B2 67%; A2+C2 58%; A2+B1+D1 71% | ✓ | ✓ | 3d |
| Scarlet Witch | A=Warp · B=Chaos · C=Spell · D=Crown | A3/A4/A5 | SS/LS (one) | C2 70%; A2+B2 58%; A1+B1+C1+D1 61%; A1+D3 31% | ✓ | ✓ | 5d |
| Pyromancer | A=Flame · B=Blaze · C=Fiery Soul · D=Meteor | A3/A4/A5 | SS + LS | C2 70%; A3+D1 80%; A1+B1+C1+D1 61% | ✓ | ✓ | 5d |
| Deadpool | A=Pew · B=Knife · C=Joke · D=Mask | A4/A5 | SS + LS | C3 35%; A2+B2 58%; A1+B1+C1+D1 61% | ✓ | ✓ | 4d |
| Krampus | A=Switch · B=Jingle · C=Present · D=Horns | A3/A4/A5 | SS + LS | C3 35%; A1+B1+C1+D1 61%; A2+B2 58% | ✓ | ✓ | 3d |

### 2-2-1-1

| Hero | Symbols | Ladder | Straights | Flex abilities (odds) | Apex ×4 | Ult | Defense |
|---|---|---|---|---|---|---|---|
| Huntress | A=Spear · B=Claw · C=Bonded Soul · D=Saber Tooth | A3/A4/A5 | SS/LS (one) | B3+D1 60%; C2 70%; A3+C1 60%; A2+B2+C1 43% | ✓ | ✓ | 3d |
| Psylocke | A=Blade · B=Psi · C=Mind · D=Starlight | A3/A4/A5 | SS/LS (one) | A1+B1+C1+D1 65%; A2+B2+D1 43%; A2+C2 47% | ✓ | ✓ | 3d |
| Loki | A=Scepter · B=Illusion · C=Lies · D=Mischief | A3/A4/A5 | SS + LS | C2 70%; A2+B2 79%; A1+B1+C1+D1 65% | ✓ | ✓ | 1d |
| Doctor Strange | A=Mandala · B=Tome · C=Mantle · D=Amulet | — | SS/LS (one) | B2+D2 47%; B4 54%; A1+B1+C1+D1 65%; A2+C2 47% | ✓ | ✓ | 2d |
| Shadow Thief | A=Dagger · B=Bag · C=Card · D=Shadow | A3/A4/A5, B2/B3/B4 | SS + LS | D3 35%; C2 70% | — | ✓ | 4d, 5d |
| Paladin | A=Sword · B=Helmet · C=Life · D=Prayer | — | SS/LS (one) | B3+D1 60%; A3+B2 42%; A3+D1 60%; C2 70% | ✓ | ✓ | 3d |

### 2-1-2-1

| Hero | Symbols | Ladder | Straights | Flex abilities (odds) | Apex ×4 | Ult | Defense |
|---|---|---|---|---|---|---|---|
| Monk | A=Fist · B=Palm · C=Zen · D=Lotus | A3/A4/A5 | SS + LS | C3 84%; A3+B1 60%; B3 35% | ✓ | ✓ | 4d |

## The odd boards

- **Paladin** (2-2-1-1) looks random but is symmetrical. On his die the most
  common symbol has only 2 faces, so a Sword ladder would be a weak
  84% / 54% / 17%. Instead the board treats Sword and Helmet as two equal main
  symbols: a mirrored pair of "triple plus apex" abilities (Sword ×3 + Prayer
  and Helmet ×3 + Prayer, both 60%), a full house of the two (Sword ×3 +
  Helmet ×2, 42%) and a pair of the odd single-face symbol (Life ×2, 70%).
  **"Two mains, no ladder."**
- **Doctor Strange** (2-2-1-1) also has no ladder. Every flex ability mixes
  symbols (Tome ×2 + Amulet ×2, Tome ×4, rainbow, Mandala ×2 + Mantle ×2).
  **"Spellbook."**
- **Alchemist** (3-2-1) is the only 3-2-1 without a ladder. A full house
  (A3+B2, 66%) takes its place, and it adds A4+C1. **"Combo."**
- **Shadow Thief** (2-2-1-1) has two ladders, the second on Bag starting at
  ×2 (97%). It uses apex ×3 instead of apex ×4, has only two flex abilities
  and has two defenses.
- **Barbarian** and **Necromancer** (3-2-1) add a second ladder on the 2-face
  symbol (Barbarian B3/B4/B5; Necromancer B3/B5, which skips ×4).
- **Deadpool** (3-1-1-1) starts his ladder at ×4 (88% / 51%).
- **Storm** has a combined small/large straight plus a separate **Largest
  Straight** (six dice in sequence, confirmed on the board photo). It is
  impossible with five dice alone and needs an extra die from her abilities or
  cards.

## Template proposal

A template is the **core**, a **straight style** (split or combined), a
**ladder style**, and a number of **flex** abilities picked from the family's
menu by difficulty band. Templates use roles instead of symbols (**main** =
the most common symbol, **side** = the 2-face symbol, **apex** = the face-6
symbol, **odd** = other 1-face symbols), so each one works for any symbol order
in its family.

### {3,2,1} templates (our standard die)

With our die, main = Sword, side = Shield, apex = Gold coin.

| Template | Board | Flex | Based on |
|---|---|---|---|
| **Standard** | 8: main ladder, SS, LS, 3 flex, apex ×4, ult | Easy + Medium + Hard, e.g. main ×1 + side ×3, main ×2 + apex ×2, main ×3 + apex ×2 | Tactician, Cyclops, Thor, Captain Marvel, Vampire Lord |
| **Compact** | 7: main ladder, SS/LS in one, 3 flex, apex ×4, ult | Same bands as Standard | Pale Lady, Horseman, Pirate, Rogue, Jean Grey, Miles, Black Panther, Storm |
| **Lean** | 7: main ladder, SS, LS, 2 flex, apex ×4, ult | Two Medium: main ×2 + apex ×2 and main ×2 + side ×3 | Gunslinger, Samurai (identical), Artificer, Treant |
| **Wide** | 8: main ladder, SS/LS in one, 4 flex, apex ×4, ult | Easy/Medium-heavy, one Hard | Wolverine |
| **Double ladder** | 8: main ladder, side ladder ×3/×4/×5, SS, LS, 2 flex, apex ×4, ult | Medium + Hard | Barbarian (Necromancer variant) |
| **Combo** (no ladder) | 7: full house main ×3 + side ×2 replaces the ladder, SS/LS in one, 3 flex, apex ×4, ult | Easy side ×3, Medium main ×4 + apex, Hard main ×1 + apex ×3 | Alchemist |

### {3,1,1,1} templates

| Template | Board | Flex | Based on |
|---|---|---|---|
| **Rainbow** | 8: main ladder, SS, LS, rainbow, odd pair or triple, two pair (main ×2 + odd ×2) or triple + kicker, apex ×4, ult | Rainbow 61% + one Easy/Medium + one Medium/Hard | Krampus, Deadpool, Pyromancer, Seraph |
| **Wide Rainbow** | 8: main ladder, SS/LS in one, 4 flex | Odd pair, two pair, rainbow, main ×1 + apex ×3 | Scarlet Witch |
| **Suits** (no rainbow) | 7: main ladder, SS/LS in one, 3 flex | main ×1 + odd ×2, main ×2 + odd ×2, main ×2 + odd + apex | Gambit |

### {2,2,1,1} templates

| Template | Board | Flex | Based on |
|---|---|---|---|
| **Standard** | 8: main ladder (weaker, 84% / 54% / 17%), SS, LS or SS/LS, odd pair (70%), rainbow (65%), one Hard combination, apex ×4, ult | Easy/Medium + rainbow + Hard | Loki, Huntress, Psylocke, Monk |
| **Two Mains** (no ladder) | 7: SS/LS in one, mirrored main ×3 + apex for both 2-face symbols, full house of the two, odd pair, apex ×4, ult | 60%, 60%, 42%, 70% | Paladin |
| **Spellbook** (no ladder) | 7: SS/LS in one, 4 mixed flex | side ×4, side ×2 + apex ×2, rainbow, main ×2 + odd ×2 | Doctor Strange |
| **Thief** | 7: main ladder, side ladder from ×2, SS, LS, odd pair, apex ×3 (in place of ×4), ult, two defenses | Odd pair only | Shadow Thief |

### Guardrails from the data

- Every template keeps the apex ×5 ultimate and (except Thief) apex ×4.
- Every board has both straights. Straight odds (62% / 26%) are the same on
  every die, so they are the stable middle of any board.
- Pick flex abilities by band, not by symbol: one Easy (≥ 75%), one Medium
  (50–75%) and one Hard (< 50%) is the most common Dice Throne mix.
- A ladder on a 2-face symbol is much weaker (17% for ×5). That is why the
  2-2-1-1 boards are the ones that drop or double their ladders.
- Defense: 3 dice is the norm, then 4, then 5. 1–2 dice defenses are
  special-mechanic boards (Gunslinger, Seraph, Loki, Jean Grey, Doctor
  Strange).

Our ability requirements already support symbol counts, straights and exact
face sets, and tiers express ladders (see `../ability-configuration.md`), so
every cost in this report can be authored today. Templates would only need a
way to save and pick these board skeletons.

## Pricing damage from odds

`analysis/board_balance.py` prices a board from one anchor damage and a
steepness `alpha`:

    damage = anchor × (anchor odds ÷ ability odds) ^ alpha

It then plays the board exactly: after each roll the player keeps the dice
that give the best expected result, then uses the strongest ability they hit.
It reports the average damage per turn, how often each ability is the one used
and its **worth** (how much the average drops without it).

Abilities on one board are not alternatives picked before rolling. The player
takes whatever the dice reach, and the cheap abilities are a safety net, so
"equal chase average" (alpha = 1) is not the fair point. On the 3-2-1
Standard board with 3 Sword = 10:

| alpha | 5 Coin ultimate | Average per turn | Turns with no hit | Result |
|---|---|---|---|---|
| 0 (flat) | 10 | 10.0 | 0.1% | Rare abilities never used; their worth is 0 |
| 0.2 (Dice Throne) | 24 | 11.5 | 0.6% | Every ability used; worth spread evenly |
| 0.5 | 86 | 14.9 | 4.4% | Rare abilities start to carry the board |
| 1 (equal chase average) | 741 | 31.7 | 12.2% | Players gamble on 4–5 Coin; those two are most of the board's worth |

Dice Throne's printed damage on 3-2-1 boards fits alpha ≈ 0.2 (172
abilities): basic 4, small straight 7, large straight 8, ultimate 12. Its
rare abilities also carry strong statuses, which this damage-only fit leaves
out.

Fairness then has two levels:

- **Inside a board:** every ability is the one used on a reasonable share of
  turns, and no single ability holds most of the board's worth.
- **Between boards:** scale each board's anchor until all templates have the
  same average damage per turn. Miss rates may differ: the average already
  counts the misses, so a riskier board earns higher hits for the same
  average.

### The Starter's board

`python3 -I board_balance.py --board starter` plays the Starter's printed
damage (`adventurer_v1/combatants/starter.yaml`): **5.9 damage per turn**,
1.2% of turns with no hit. Counting damage only:

| Ability | Cost | Damage | Odds | Used |
|---|---|---|---|---|
| Strike | 3 / 4 / 5 Swords | 4 / 5 / 6 | 98% / 88% / 51% | 7% / 19% / 27% |
| Guarded Strike | 2 Swords + 3 Shields | 5 (+2 Protect) | 56% | 3% |
| Measured Strike | 2 Swords + 2 Coins | 4 (+5 energy) | 58% | 1% |
| Small Straight | SS | 6 | 62% | 25% |
| Large Straight | LS | 8 | 26% | 15% |
| Decisive Blow | 4 Coins | 10 | 10% | 2% |

On damage alone Guarded Strike and Measured Strike are almost never the best
choice; their value is their extras. If 2 Protect counts as 2 damage, Guarded
Strike becomes the most used ability (27%). Measured Strike stays rare unless
5 energy is worth 2 or more damage (2 → 14%, 3 → 30%). Usage swings that much
on these values, so non-damage effects need agreed damage equivalents before
any board can be priced. The Starter's numbers match Dice Throne's printed
basics closely (basic 4/5/6, large straight 8), with a cheaper small straight
(6 against a median of 7).

**Exchange rates (agreed 2026-10-15):** 2 Protect = 2 damage (1 per Protect)
and 5 energy = 1 damage (0.2 per energy). They live in `EXCHANGE` in
`board_balance.py`. At these rates the Starter is worth **6.2 per turn**:

| Ability | Value | Used | Worth |
|---|---|---|---|
| Strike 3 / 4 / 5 | 4 / 5 / 6 | 8% / 13% / 14% | 0.21 / 0.10 / 0.08 |
| Guarded Strike | 7 | 27% | 0.32 |
| Measured Strike | 5 | 3% | 0.04 |
| Small Straight | 6 | 17% | 0.25 |
| Large Straight | 8 | 16% | 0.24 |
| Decisive Blow | 10 | 2% | 0.03 |

Guarded Strike is the board's best ability but does not dominate. Measured
Strike is close to dead: at 4 damage it loses to the 5-Sword Strike and the
small straight for the same odds. At 5 damage it is used on 14% of turns; at
6 it becomes the top ability (30%) and pushes Guarded Strike down to 19%.

## Caveats

- The costs come from the transcribed spreadsheet. Several heroes (Paladin,
  Monk and others) have no source photo in `sources/`, so their rows were not
  checked against a board here. Storm's Largest Straight was checked against
  the photo.
- Odds assume you chase one ability from the first roll. Real play picks the
  target mid-roll, uses cards and has other dice effects, so treat the numbers
  as relative difficulty rather than exact hit rates.
