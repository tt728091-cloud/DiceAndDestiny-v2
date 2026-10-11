# Relic, Consumable & Gold Catalog — Slay the Spire Translation (Review Draft v2)

Status: **research / brainstorm only — nothing implemented.** "Relic" is a working
term; all item names are original placeholders (the "Inspired by" notes name the
Slay the Spire item only for reference — never ship StS names).

Sources: Slay the Spire 1 relic list and Slay the Spire 2 Early Access relic data
(slaythespire.wiki.gg `Module:Relics/StS2_data`, StS2 Ancients/Neow pages, patch
notes through v0.110). StS2 numbers move often; treat them as mid‑2026 values.

---

## 0. Decisions from review (2026‑10‑09)

1. **Wounds carry over between battles.** Removed cards stay removed for the run.
   Out‑of‑battle healing exists (bandages, rests, relics) — a deliberate change
   from StS, where healing is just HP.
2. **Relics are run‑scoped.** They stay with the character until that game/run
   ends, like StS. Run length is not decided yet.
3. **Energy scales in fine steps.** Because 5 energy ≈ 1 card, recurring bonuses
   can be +1, +2, +3 per round instead of a literal +5. Stack many small items.
4. **Battle‑start statuses from relics are fine,** every battle, for any pool.
5. **Information relics are possible** if done correctly (§7).
6. **Gold is a run currency** for items/consumables/services, and may convert to
   XP (§9).
7. **Consumables are in scope** (§8).
8. **Healing is allowed, in battle and out.** The old "revive trade only, health
   never rises" rule is removed from AGENTS.md. The engine's revive‑trade
   validation still enforces it until healing is implemented.
9. **Max health = every card you own that is still in play** (deck + hand +
   discard). There is no separate cap. "+max health" items add cards; healing
   only restores cards that were removed (wounds).
10. **Gold and XP stay separate for now.** Gold → XP conversion is a TODO, but
    gold‑bought items that grant an XP burst are fine.
11. **Plate** (temporary battle‑only health cards) is a promising candidate.
12. Ideas were also mined from the **Dice Throne reference spreadsheet**
    (`dice-throne-reference/`: 122 statuses, passives, companions, card
    mechanics) — see §5‑DT. Names stay original.
13. **Undefendable** means only that the target can't use a defensive ability
    against the attack. Prevention cards (and other prevention effects) still
    work.
14. **Companions are both:** relics can grant them, and future characters may
    unlock companions through skills/ability progression.

---

## 1. Slay the Spire in brief

- **StS1 acquisition:** Starter relic → Common/Uncommon/Rare (50/33/17%) from
  elites, chests, shops → Boss relic (pick 1 of 3) after each act → Shop‑only
  and Event‑only relics. 10–20 relics per run, never duplicated.
- **StS2 changes:** Boss relics removed; each act opens with an **Ancient** NPC
  who heals you and forces a pick of 3 themed, high‑power relics (many with
  drawbacks). **Neow** opens each run with 2 clean offers + 1 cursed offer that
  is a stronger version of a clean one. Starter relics get upgraded versions.
  New shapes: **enchantment** relics (permanently modify cards), **quest**
  relics (transform after 5 elite kills), **delayed** relics (3 relics after 5
  combats), **decaying** relics (wax relics melt every 3 combats; a candle gives
  energy for 5 combats). Character relics have private pools.
- **Patterns:** flat power vs build‑arounds; big upside + drawback aimed at a
  *different* system; counters; turn‑N timers; event hooks that bind relics to
  card keywords; cheap rule‑number tweaks; pool hygiene (no dupes, gate on deck,
  remove late‑useless); readability (counter on icon, flash on trigger, greyed
  when spent, drawback in the same tooltip).

## 2. Translation rules

| Slay the Spire | Dice and Destiny |
|---|---|
| 1 energy (one‑shot) | 5 energy |
| +1 energy every turn | **+1 to +3 Income energy** per round (fine‑grained); +5 is top tier |
| Turn | Round ("start of turn" ≈ Ongoing Effects entry or Income) |
| Draw | Draw (our Income is only 1 card — +1 per round doubles it) |
| Block / HP / relic damage | ÷5, round, min 1 (StS ~80 HP ↔ our ~16 cards) |
| Block | Protect / Ward / **Plate** (temporary health cards, §3) |
| Max HP | **+health card** permanently added to the deck |
| Heal | **Mend** a wound (restore a removed card) — out of battle by default |
| Strength / Dexterity | +damage on offensive abilities / +prevention on defensive abilities |
| Vulnerable / Weak | New statuses **Exposed** / **Sapped** |
| Exhaust | Removed pile / sacrifice |
| Shuffle | **Draw pile runs empty** (we never reshuffle) |
| Retain hand / keep energy | Already native → translate to **hand limit / energy cap** |
| Gold, shop, rest, map, potions | Gold, shop, rest, map, **consumables** (run layer) |
| *(none)* | **Dice** — a whole family StS can't have |

## 3. Things that change versus StS (read before tuning)

1. **Banked energy:** timed bursts are more flexible for us; 75 cap is the limit.
2. **One attack per actor per round:** "per hit" relics are stronger; "every 10
   attacks" counters become every 3–4. Card counters ≈ every 4–6 cards.
3. **+1 damage is a big unit** relative to ~16 health; flat "+1 to everything"
   is Rare, or narrow it.
4. **Deck = health:** removal costs health (deck thinning is a trade‑off), and
   adding junk cards adds health — StS curse‑card drawbacks become buffs.
   Our drawbacks should remove cards, raise wound pressure, cost gold/XP, cut
   income, or restrict choices.
5. **Wounds persist:** health is now a run resource, like StS HP. Healing,
   max‑health, and "don't take wounds" effects are a major reward axis.
6. **New concept — Plate (temporary health):** battle‑only cards placed in the
   discard pile; damage selects them first (discard‑first order); when removed
   they vanish and **do not count as wounds**. This is our StS Block analogue
   and needs no change to the health rule.
7. **Rule guardrails** (AGENTS.md):
   - Every charge, counter‑buff or expiring effect must be a visible status.
   - Healing must be an explicitly authored effect and only restores removed cards.
   - Reaction moments are opt‑in per card.
   - Enemy choices are never exposed unless authoritatively revealed.
   - Items marked ⚠️ touch one of these rules (mainly reaction moments and
     information).

## 4. Legend

- **Rarity:** S Starter · C Common · U Uncommon · R Rare · B Boss/Ancient ·
  Sh Shop · E Event · Q Quest · CH Character pool
- **Build:** ✅ fits today's data (battle construction, starting statuses,
  status triggers at Ongoing Effects entry / Offensive entry / Offensive exit,
  battle‑long ability modifiers, dice loadouts) · 🔌 needs an engine hook
  (Income H‑07, card drawn H‑08, die rolled H‑10, final pool H‑12, damage commit
  H‑24, hand‑limit H‑25, battle end H‑26, card‑played counters, defensive
  modifiers) · 🗺️ needs the run layer (rewards, map, shop, rest, gold, wounds
  across battles) · 🆕 needs a new status or concept · ⚠️ touches a current rule

---

## 5. Relic catalog

### A. Battle start

| # | Name | Effect | Inspired by | Rarity | Build |
|---|---|---|---|---|---|
| A1 | Kindling Pouch | +5 energy at battle start. | Lantern | C | ✅ |
| A2 | Ember Flask | +10 energy at battle start. | Very Hot Cocoa | U | ✅ |
| A3 | Early Riser's Band | +1 card in opening hand. | Ring of the Snake | S | ✅ |
| A4 | Packed Satchel | +2 cards in opening hand. | Bag of Preparation | C | ✅ |
| A5 | Ready Hand Gloves | Your first Income each battle draws +2. | — | C | 🔌 |
| A6 | Primed Reserves | +15 energy at battle start; your first Income gives no energy. | Bread | U | ✅ |
| A7 | Battle Woad | Start each battle with **Fury 2** (+2 damage to your next attack). | Akabeko | C | ✅ 🆕 |
| A8 | Opening Ward | Start each battle with 2 Protect. | Anchor | C | ✅ |
| A9 | Plated Vest | Start each battle with 2 **Plate** cards. | Thread and Needle | U | 🔌 🆕 |
| A10 | Second Look | Opening hand: discard any number, draw that many. | Gambling Chip | U | 🔌 |
| A11 | Favored Card | Choose a card; it always starts in your opening hand. | Bottled relics | U | 🔌 |
| A12 | Twin Favorites | Choose two cards; both always start in your opening hand. | — | R | 🔌 |
| A13 | Spark Gift | The first card you play each battle costs 0. | Vexing Puzzlebox | U | 🔌 |
| A14 | Echo Opener | The first card you play each battle resolves twice. | Throwing Axe | R | 🔌 |
| A15 | Scout's Primer | The first card you play each battle also draws 1. | Swift enchant | C | 🔌 |
| A16 | Mustering Drum | At battle start, a chosen card in your deck auto‑plays for free. | Electric Shrymp (Imbued) | R | 🔌 |
| A17 | Grand Entrance | Round 1: 2 damage to each enemy. | Festive Popper | U | ✅ |
| A18 | War Banner | Round 1: +1 offensive roll. | — | C | ✅ |
| A19 | Giant's Sling | Elite/boss battles: +1 damage to all your attacks. | Sling of Courage | U | ✅ |
| A20 | Champion's Resolve | Boss battles: mend 2 wounds at battle start. | Pantograph | U | 🗺️ |
| A21 | Sharp Start | Your first offensive ability each battle gains choose_tier and +1 damage. | — | C | ✅ |

### B. Recurring income, caps and limits

| # | Name | Effect | Inspired by | Rarity | Build |
|---|---|---|---|---|---|
| B1 | Copper Stipend | +1 Income energy each round. | Energy relics (scaled) | C | 🔌 H‑07 |
| B2 | Silver Stipend | +2 Income energy each round. | — | U | 🔌 |
| B3 | Gold Stipend | +3 Income energy each round. | — | R | 🔌 |
| B4 | Crown Stipend | +5 Income energy each round. (Drawback family §J) | — | B | 🔌 |
| B5 | Accelerating Coil | +1 Income energy per round elapsed this battle (max +4). | — | R | 🔌 |
| B6 | Studious Lens | +1 Income card every 2nd round. | — | U | 🔌 |
| B7 | Scholar's Satchel | +1 Income card every round. | Pael's Blood | B | 🔌 |
| B8 | Overflow Basin | Energy cap +25 (75 → 100). | Ice Cream (remapped) | C | ✅ |
| B9 | Bottomless Coffer | No energy cap; −1 Income energy. | — | R | ✅/🔌 |
| B10 | Deep Pockets | Hand limit +1. | Runic Pyramid (remapped) | C | ✅ |
| B11 | Bag of Holding | Hand limit +2; opening hand −1. | — | U | ✅ |
| B12 | Steady Trickle | If you hold ≤10 energy at Income, +5 energy. | — | C | 🔌 |
| B13 | Miser's Interest | At Income, +1 energy per 15 energy held (max +3). | — | U | 🔌 |
| B14 | Lean Rations | If your hand is empty at Income, draw +2. | Unceasing Top | C | 🔌 |
| B15 | Patience Stone | If you spent no energy last round, +5 energy at Income. | Art of War | C | 🔌 |
| B16 | Quiet Hours | If you played no cards last round, +1 Income card. | Pocketwatch | C | 🔌 |
| B17 | Brimming Cup | If you end a round at the energy cap, draw 1. | Pael's Tears (inverted) | U | 🔌 |
| B18 | Mercy Bell | Each round no attack targeted you, +2 energy. | — | C | 🔌 |

### C. Round‑N timers (countdown statuses at Ongoing Effects entry)

| # | Name | Effect | Inspired by | Rarity | Build |
|---|---|---|---|---|---|
| C1 | Dawn Candle | Round 2: +10 energy. | Candelabra | U | ✅ |
| C2 | Third Bell | Round 3: +15 energy. | Chandelier | R | ✅ |
| C3 | Round‑Two Draught | Round 2: draw 2. | — | C | ✅ |
| C4 | Tidewall Cleat | Round 2: gain 3 Protect. | Horn Cleat | U | ✅ (Protect cap) |
| C5 | Third Watch Helm | Round 3: gain 4 Plate. | Captain's Wheel | R | 🔌 🆕 |
| C6 | Rising Rouge | Round 3: +1 damage and +1 prevention for the rest of the battle. | Sparkling Rouge | U | ✅/🔌 |
| C7 | Late Bloom | From round 3: +2 Income energy each round. | Pael's Flesh | U | 🔌 |
| C8 | Slow Fuse | Round 4: 3 damage to each enemy and +10 energy. | — | R | ✅ |
| C9 | Seventh Toll | Round 6: 8 damage to each enemy. | Stone Calendar | R | ✅ |
| C10 | Mid‑Fight Rally | Round 3: remove all negative statuses from you. | — | U | ✅ |
| C11 | Second Front | Round 2: +1 offensive roll and +1 defensive die. | — | U | ✅/🔌 |

### D. Periodic triggers

| # | Name | Effect | Inspired by | Rarity | Build |
|---|---|---|---|---|---|
| D1 | Sunwheel Flower | Every 3rd round: +5 energy. | Happy Flower | C | ✅ |
| D2 | Swinging Weight | Every 3rd round: draw 1. | Pendulum | C | ✅ |
| D3 | Slow Bloom | Every 4th round: draw 2. | Pollinous Core | U | ✅ |
| D4 | Fading Incense | Every 4th round: gain **Veil** (fully prevent one attack this round). | Incense Burner | R | ✅ 🆕 |
| D5 | Grain Glass | Each round: 1 damage to each enemy. | Mercury Hourglass | R | ✅ |
| D6 | Tide Clock | Every 2nd round: +1 offensive roll. | Inserter | R | ✅ |
| D7 | Pulse Stone | Every 2nd round: 1 Protect. | Sai | C | ✅ |
| D8 | Rhythm Drum | Every 3rd round: your attack deals +2. | — | C | ✅ |
| D9 | Furnace Heart | Each round +1 damage to your attacks (stacks); every other round each enemy gains +1 damage. | Brimstone | Sh | ✅ |
| D10 | Struggle Ember | Each round from round 3: damage to each enemy = ⌊round ÷ 3⌋. | Mr. Struggles | R | ✅ |
| D11 | Plating Forge | Every 2nd round: gain 1 Plate. | Orichalcum | U | 🔌 🆕 |
| D12 | Clockwork Mender | Every 3rd round, mend 1 wound taken this battle. | Regeneration | R | 🔌 |

### E. Offense and ability modifiers

| # | Name | Effect | Inspired by | Rarity | Build |
|---|---|---|---|---|---|
| E1 | Whetted Edge | +1 damage to all offensive abilities. | Vajra | R | ✅ |
| E2 | Signature Move | Choose one offensive ability: +1 damage for the run. | Strike Dummy | C | ✅ |
| E3 | Low Blow | Tier‑1 activations: +2 damage. | Wrist Blade | U | ✅ |
| E4 | Crowning Strike | Highest‑tier activations: +2 damage. | — | U | ✅ |
| E5 | Heavy Boot | Attacks that would remove exactly 1 card remove 2. | The Boot | U | 🔌 |
| E6 | Tally Blade | Every 4th offensive ability deals double damage. | Pen Nib | R | 🔌 |
| E7 | Spiteful Edge | +2 damage against an enemy who targeted you this round. | — | U | 🔌 |
| E8 | Executioner's Mark | +2 damage against enemies at or below half health. | — | U | 🔌 |
| E9 | Bully's Knuckles | +1 damage against enemies with a negative status. | — | U | 🔌 |
| E10 | Opening Gambit | First attack against each enemy: +2 damage. | — | C | 🔌 |
| E11 | Overkill Chain | Overage damage carries to another enemy. | — | R | 🔌 |
| E12 | Cleaving Edge | Your offensive ability also deals 1 damage to each other enemy. | — | R | 🔌 |
| E13 | Bargain Blade | Offensive ability costs −5 energy (min 0). | — | U | ✅ |
| E14 | Blood‑Drunk Axe | +1 damage per 4 wounds you carry (max +3). | — | R | 🔌 🗺️ |
| E15 | Cornered Beast | While at or below half health: +1 damage. | Red Skull | C | 🔌 |
| E16 | Master's Polish | Upgraded abilities: +1 damage. | Miniature Cannon | U | ✅ |
| E17 | Chain Gauntlet | If you played 2+ cards before your offensive roll: +1 damage. | — | U | 🔌 |
| E18 | Echo Strike | Once per battle, your offensive ability resolves twice. | Necronomicon | R | 🔌 |
| E19 | Humble Pick | Choosing a lower tier than you qualify for: +5 energy per tier skipped. | — | U | 🔌 |
| E20 | Pierce Band | Your attacks ignore 1 prevention from the target's defensive ability. | — | R | 🔌 |
| E21 | Free Choice | All your offensive abilities gain choose_tier. | — | C | ✅ |
| E22 | Split Shot | Your offensive ability may split its damage between two enemies. | — | R | 🔌 |
| E23 | Status Edge | Each on‑hit status your attacks apply gets +1 stack. | Snecko Skull (general) | R | 🔌 |

### F. Offensive dice

| # | Name | Effect | Inspired by | Rarity | Build |
|---|---|---|---|---|---|
| F1 | Lucky Throw | Round 1: +1 offensive roll. | — | C | ✅ |
| F2 | Gambler's Spare | +1 offensive roll every round. | Energy relics | B | ✅ |
| F3 | Loaded Pip | Once per battle: set one offensive die to any face. | — | U | ✅ 🆕 |
| F4 | Nudging Thumb | Once per round: adjust one offensive die ±1. | — | R | 🔌 |
| F5 | Mirror Pip | Once per battle: copy one die's face onto another. | — | U | 🔌 |
| F6 | Flipping Coin | Once per battle: flip one die to its opposite face. | — | C | 🔌 |
| F7 | Sixth Die | Roll 6 offensive dice; −1 offensive roll. | — | B | 🔌 |
| F8 | Keen Die | Replace one die with a Sword on faces 1–4. | Dice loadout | U | ✅ |
| F9 | Shieldbearer Die | Replace one die with a Shield on faces 3–5. | — | U | ✅ |
| F10 | Gilded Die | Replace one die with a Coin on faces 5–6. | — | U | ✅ |
| F11 | Chameleon Die | Replace one die with a 6 face that counts as any symbol. | — | R | 🔌 🆕 |
| F12 | Straight Edge | Your first straight each battle: +10 energy. | — | U | 🔌 H‑12 |
| F13 | Triplet Charm | Three of a kind in your final pool: draw 1. | — | U | 🔌 |
| F14 | Pair Purse | A pair in your final pool: +1 energy. | — | C | 🔌 |
| F15 | Five‑Alike Crown | Five of a kind: +20 energy and draw 2. | — | U | 🔌 |
| F16 | Coin Hoard | 2+ Coins in your final pool: +5 energy. | — | C | 🔌 |
| F17 | Sword Tally | Every 10 Swords in final pools: +5 energy. | Nunchaku | C | 🔌 |
| F18 | Keep‑High Charm | Your last reroll each round uses keep‑higher. | — | R | 🔌 |
| F19 | Safety Net | If your final pool qualifies nothing, reroll all dice once for free. | — | U | 🔌 |
| F20 | Consolation Coin | If no offensive ability qualified this round: +5 energy and draw 1. | Art of War | C | 🔌 |
| F21 | Reroll Rebate | Each round you leave your last roll unused: +2 energy. | — | C | 🔌 |
| F22 | Hot Hand | If your first roll qualifies an ability: +3 energy. | — | C | 🔌 |
| F23 | Roll Bank | Unused rolls become **Spare Roll** (cap 2), spendable in a later round. | — | R | 🔌 🆕 |
| F24 | Held Breath | One die kept at the end of Offense starts on that face next round. | Runic Pyramid | R | 🔌 |
| F25 | Fickle Eye | +1 Income card every round; each round one random offensive die is locked to a random face. | Snecko Eye | B | 🔌 |
| F26 | Long Stride | A large straight gives your ability +2 damage. | — | U | 🔌 |

### G. Defensive dice

| # | Name | Effect | Inspired by | Rarity | Build |
|---|---|---|---|---|---|
| G1 | Spare Shield Die | Your first defensive ability each battle rolls +1 die. | — | U | 🔌 |
| G2 | Calm Fingers | Once per round: reroll one defensive die. | — | U | 🔌 (`reroll_defense` exists) |
| G3 | Stout Arm | Every defensive roll uses +1 die. | — | B | 🔌 |
| G4 | Coin Catcher | Defensive Coins grant +2 energy each, above the gain limit. | — | C | 🔌 |
| G5 | Lucky Guard | A defensive roll that prevents 0 prevents 1. | — | C | 🔌 |
| G6 | Doubled Shield | First defensive roll each battle: prevention doubled. | Vambrace | U | 🔌 |
| G7 | Mirror Guard | Once per battle, copy a defensive die's face. | — | U | 🔌 |
| G8 | Second Stance | Once per battle, use two defensive abilities against the same attack. | — | R | 🔌 |
| G9 | Steady Footing | +1 prevention on every defensive ability. | Oddly Smooth Stone | R | 🔌 |

### H. Damage prevention and protection

| # | Name | Effect | Inspired by | Rarity | Build |
|---|---|---|---|---|---|
| H1 | Iron Rib | The first attack each round that would remove your cards removes 1 fewer. | Tungsten Rod | R | 🔌 |
| H2 | Paper Ward | Incoming attacks of 2–3 damage are reduced to 1. | Torii | R | 🔌 |
| H3 | Unyielding Heart | You can't lose more than 4 cards per round. | Beating Remnant | B | 🔌 |
| H4 | Last Stand Charm | Once per battle, the attack that would defeat you is fully prevented. | Lizard Tail | R | 🔌 |
| H5 | Shell Charm | The first damage source against you each battle is reduced by 3. | Fossilized Helix | U | 🔌 |
| H6 | Keepsake | Choose a card each battle; damage removes it last. | — | U | 🔌 H‑21 |
| H7 | Ranks Reversed | Damage takes from your draw pile before your discard. | — | U | 🔌 H‑21 |
| H8 | Back‑Pocket Saves | Cards saved by your defensive abilities go to your hand. | — | R | ✅ |
| H9 | Deeper Guard | Protect cap 2 → 4. | — | U | ✅ |
| H10 | Lasting Guard | Unused Protect lasts one extra round. | Calipers / Sturdy Clamp | U | ✅ |
| H11 | Riposte Buckler | Fully preventing an attack deals 1 damage to its attacker. | Parrying Shield | U | 🔌 |
| H12 | Thorn Collar | When an attack removes your cards, its attacker takes 1 damage. | Bronze Scales | U | 🔌 |
| H13 | Guardian's Rebate | Fully preventing an attack: +5 energy. | — | C | 🔌 |
| H14 | Crowd Shield | When 2+ attacks target you in a round, the smallest is reduced by 1. | — | U | 🔌 |
| H15 | Bastion Scab | When you lose cards, gain 1 Protect next round. | Self‑Forming Clay | U | 🔌 |
| H16 | Prevention Echo | Your first prevention card each battle prevents +2. | — | C | 🔌 |
| H17 | Calm Mind | If you didn't attack this round, your defenses prevent +1. | Ripple Basin | C | 🔌 |
| H18 | Decoy Doll | Each battle add a **Decoy** to your discard (Plate); when removed it deals 1 damage to the attacker. | — | U | 🔌 🆕 |
| H19 | Iron Hide | Start each battle with 4 Plate; your opening hand is −1. | — | R | 🔌 🆕 |
| H20 | Bracing Plate | Start each battle with **Ward 3** (persists until it prevents). | Diamond Diadem | U | ✅ 🆕 |

### I. Health, wounds and healing (run)

| # | Name | Effect | Inspired by | Rarity | Build |
|---|---|---|---|---|---|
| I1 | Field Kit | After each battle, mend 1 wound. | Burning Blood | S | 🗺️ |
| I2 | Surgeon's Kit | After each battle, mend 2 wounds (upgraded Field Kit). | Black Blood | B | 🗺️ |
| I3 | Last‑Gasp Tonic | After a battle where you lost half your health or more, mend 3. | Meat on the Bone | U | 🗺️ |
| I4 | Victory Feast | After elite/boss battles, mend all wounds taken in that battle. | — | R | 🗺️ |
| I5 | Down Pillow | Resting mends 2 extra wounds. | Regal Pillow | C | 🗺️ |
| I6 | Hearty Rations | +1 / +2 / +3 max health: permanent health cards added to the deck. | Strawberry / Pear / Mango | C/U/R | 🗺️ |
| I7 | Feast Bowl | +2 max health and mend all wounds. | Lee's Waffle | R | 🗺️ |
| I8 | Growing Root | Every 3 battles, +1 max health. | Face of Cleric | U | 🗺️ |
| I9 | Leech Fang | Mend 1 wound at battle end for each enemy you defeated (max 2). | — | U | 🗺️ |
| I10 | Scar Ledger | +1 energy at battle start per wound you carry (max +10). | — | U | ✅ 🗺️ |
| I11 | Hardened Resolve | While you carry 5+ wounds: +1 damage. | — | U | ✅ 🗺️ |
| I12 | Mending Moss | Entering a shop or rest site mends 1. | Meal Ticket | C | 🗺️ |
| I13 | Golden Salve Jar | The first time you gain gold after each battle, mend 1. | Bloody Idol | U | 🗺️ |
| I14 | Rebirth Ember | Once per run, when defeated, mend to half health and win or flee the battle. | Lizard Tail (run‑level) | R | 🗺️ |
| I15 | Glancing Blows | The first card removed each battle returns after the battle (no wound). | — | U | 🗺️ |
| I16 | Hardy Constitution | Wounds from normal battles heal after battle; only elite/boss wounds persist. | — | B | 🗺️ |
| I17 | Patchwork Doll | At rest: turn 2 wounds into one junk "Scar" card (health back, a dead draw). | — | U | 🗺️ |
| I18 | Medic's Bag | Bandages and other mending consumables mend +1. | Sacred Bark (scoped) | C | 🗺️ |
| I19 | Iron Constitution | Max health +3; you can't mend at rest sites. | Mark of the Bloom | B | 🗺️ |
| I20 | Blood Bank | Each battle you finish with no new wounds stores 1 **Reserve**; spend Reserves to mend later. | — | U | 🗺️ 🆕 |

### J. Top‑tier relics with drawbacks (the "energy relic" family)

Upside options: **+3 or +5 Income energy**, **+1 Income card**, **+1 offensive
roll each round**, **+2 hand limit + 1 card**, **sixth die**. Pair each with a
drawback in a *different* system.

| # | Name | Upside | Drawback | Inspired by |
|---|---|---|---|---|
| J1 | Hollow Crown | +5 Income energy | No XP from battles. | Ectoplasm |
| J2 | Dry Purse Crown | +5 Income energy | No gold from battles. | Ectoplasm |
| J3 | Dripping Kettle | +5 Income energy | Can't mend at rest sites. | Coffee Dripper |
| J4 | Rusted Anvil | +5 Income energy | Can't upgrade cards at rest/after battle. | Fusion Hammer |
| J5 | Cracked Diadem | +5 Income energy | Reward choices show 2 fewer options. | Busted Crown |
| J6 | Philosopher's Ash | +5 Income energy | Every enemy's attacks deal +1 damage. | Philosopher's Stone |
| J7 | Choking Collar | +5 Income energy | Play at most 2 cards per round. | Velvet Choker |
| J8 | Elite Collar | +5 Income energy | Only in elite/boss battles. | Slaver's Collar |
| J9 | Thin Blood | +5 Income energy | Start each battle with 2 cards removed (become wounds). | Mark of Pain (inverted) |
| J10 | Teetotaler's Oath | +3 Income energy | You can't use consumables. | Sozu |
| J11 | Heavy Coil | +1 offensive roll each round | Hand limit −2. | — |
| J12 | Tight Purse | +1 Income card each round | Energy cap 75 → 40. | — |
| J13 | Fogged Lens | +5 Income energy | Enemy attack threat lists show counts only, not which cards (until Defense). | Runic Dome ⚠️ |
| J14 | Waning Candle | +5 Income energy | Burns out after 4 battles. | Pumpkin Candle |
| J15 | Pact of Seven | +5 Income energy for 7 battles | Then −2 max health. | — |
| J16 | Bloodthirst Pact | Your attacks +2 damage | Enemy attacks against you +1. | — |
| J17 | Wanderer's Burden | +1 Income card, +3 Income energy | Your opening hand is 2 cards. | — |
| J18 | Gilded Cage | +2 hand limit and +1 Income card | Shops cost 50% more. | — |
| J19 | Whispered Bargain | +5 Income energy | The enemy chooses your first round's target. | Whispering Earring |
| J20 | Brittle Shell | +5 Income energy | Defensive abilities cost +5 energy. | Spiked Gauntlets |

### K. Event triggers (our event vocabulary)

| # | Name | Effect | Inspired by | Rarity | Build |
|---|---|---|---|---|---|
| K1 | Wake‑Up Call | First time you lose cards each battle: draw 2. | Centennial Puzzle | C | 🔌 |
| K2 | Pain Reader | Whenever an attack removes your cards: draw 1. | Runic Cube | B | 🔌 |
| K3 | Ashen Offering | Whenever you sacrifice a card: 1 damage to each enemy. | Charon's Ashes | R | 🔌 |
| K4 | Scatter Chimes | Hand‑limit discard: 1 damage to a random enemy per card. | Tingsha | U | 🔌 H‑25 |
| K5 | Discard Wraps | Hand‑limit discard: 1 Protect per card. | Tough Bandages | U | 🔌 |
| K6 | Drifting Kite | First card you discard each round: +5 energy. | Hovering Kite | U | 🔌 |
| K7 | Empty Well | First time your draw pile empties: +15 energy and 2 Protect. | Sundial / Abacus | U | 🔌 |
| K8 | Deep Recall | First time your draw pile empties: return 3 random discards to the draw pile (authored recovery). | — | R | 🔌 |
| K9 | Victor's Horn | Enemy defeated: +5 energy and draw 1. | Gremlin Horn | U | 🔌 |
| K10 | Grave Collector | Enemy defeated: +15 gold. | — | C | 🗺️ |
| K11 | Status Leech | Whenever a negative status on you expires or is removed: +2 energy. | — | U | 🔌 |
| K12 | Purifier's Charm | Whenever you remove a status from an enemy: draw 1. | — | U | 🔌 |
| K13 | Saved Spark | Each card saved by prevention: +1 energy (max 5 per round). | — | C | 🔌 |
| K14 | Domino Mark | When an enemy is defeated, the other enemies become Exposed. | — | U | 🔌 🆕 |
| K15 | Blindsider | When an enemy's attack is cancelled by Blind: +10 energy. | — | U | 🔌 |
| K16 | Close Shave | When an enemy attack is fully prevented, your next attack +2. | — | U | 🔌 |
| K17 | Lifeline | Once per battle, when you drop to 3 cards or fewer: gain Ward 3. | — | U | 🔌 |
| K18 | Pattern Seeker | Use an offensive and a defensive ability and play a card in one round: +5 energy. | Rainbow Ring | R | 🔌 |
| K19 | Salvager's Hook | When a Plate/Decoy card is removed: +2 energy. | — | C | 🔌 |
| K20 | Revenge Ledger | Each card you lose this battle: +1 damage on your next attack (max +3). | — | U | 🔌 |

### L. Counters (icon shows the number)

| # | Name | Effect | Inspired by | Rarity | Build |
|---|---|---|---|---|---|
| L1 | Rhythm Sticks | Every 3 offensive abilities: +5 energy. | Nunchaku | U | 🔌 |
| L2 | Scribe's Inkpot | Every 6 cards played: draw 1. | Ink Bottle | U | 🔌 |
| L3 | Iron Club Charm | Every 4 cards played: +5 energy. | Iron Club | U | 🔌 |
| L4 | Ringing Shield | Every 4 defensive abilities: 2 Protect. | Tuning Fork | U | 🔌 |
| L5 | Momentum Band | The 2nd card you play in a round: +1 damage to your next attack this round. | Shuriken / Kunai | U | 🔌 |
| L6 | Letter Blade | The 3rd card you play in a round: 1 damage to each enemy. | Letter Opener | U | 🔌 |
| L7 | Generous Scarf | The 3rd card you play each round costs 0. | Brilliant Scarf | R | 🔌 |
| L8 | Coin Tally | Every 6 Coins in final pools: draw 1. | — | C | 🔌 |
| L9 | Shield Tally | Every 5 Shields on defense: 2 Protect. | — | C | 🔌 |
| L10 | Battle‑Scarred Medal | Every 10 wounds received this run: +1 permanent damage. | — | R | 🗺️ |
| L11 | Veteran's Notches | Every 5 battles won: +1 Income energy for the run (max +3). | — | R | 🗺️ |

### M. Self statuses at battle start

| # | Name | Effect | Rarity | Build |
|---|---|---|---|---|
| M1 | Thornmail | Start with **Thorns 1** for the battle. | U | ✅ 🆕 |
| M2 | Focus Stone | Start with **Composure 1** (spend for +1 roll in any round). | C | ✅ 🆕 |
| M3 | Fortune Charm | Start with **Fortune 2** (spend to reroll a single die any time). | U | ✅ 🆕 |
| M4 | Warding Talisman | Start with **Hallow 1** (negate the next negative status). | U | ✅ 🆕 |
| M5 | Regrowth Seed | Start with **Regrowth 3**; lose a stack when you lose cards; after the battle mend 1 per stack left. | R | ✅ 🆕 🗺️ |
| M6 | Calm Aura | Start with **Calm** (the first attack against you is −2). | C | ✅ 🆕 |
| M7 | Haste Charm | Start with **Haste 1** (your next card costs 0). | U | ✅ 🆕 |
| M8 | Hunter's Eye | Start with **Mark** on one enemy (+1 damage from you all battle). | U | ✅ 🆕 |

### N. Enemy debuffs at battle start

| # | Name | Effect | Rarity | Build |
|---|---|---|---|---|
| N1 | Marked Quarry | Each enemy starts **Exposed 1** (next attack against it +1). | U | ✅ 🆕 |
| N2 | Sapping Veil | Each enemy starts **Sapped 1** (its first attack −1). | U | ✅ 🆕 |
| N3 | Venom Censer | Each enemy starts with 1 Poison. | U | ✅ |
| N4 | Tangle Snare | Round 1: one random enemy is Entangled. | C | ✅ |
| N5 | Smoke Pellets | Round 1: every enemy is Blinded. | R | ✅ |
| N6 | Bleeding Hook | Your first attack each battle applies 1 Bleed. | U | ✅ |
| N7 | Jinx Coin | A random enemy starts with 2 Curse Count. | U | ✅ |
| N8 | Jinx Lantern | Elites and bosses start Sapped 2. | U | ✅ 🆕 |
| N9 | Rust Powder | Round 1: enemy defensive abilities prevent 1 less. | U | 🔌 |
| N10 | Fear Totem | Minion enemies start with 1 card removed. | C | ✅ |

### O. Status amplifiers and character pools

| # | Name | Effect | Pool | Build |
|---|---|---|---|---|
| O1 | Fang Gland | Whenever you apply Poison, apply 1 more (cap 3). | Venom | 🔌 |
| O2 | Catalyst Vial | Catalyst cap +1. | Venom | ✅ |
| O3 | Rotting Tooth | Poison deals damage on 1–5 instead of 1–4. | Venom | ✅ |
| O4 | Toxic Cascade | When a poisoned enemy is defeated, its Poison spreads to another enemy. | Venom | 🔌 |
| O5 | Fast Incubation | Incubation converts 2 Poison instead of 1. | Venom | ✅ |
| O6 | Heavy Toll | Curse Count deals damage every 2 Count instead of 3. | Curse | ✅ |
| O7 | Deep Ledger | Grave Debt cap 5 → 7. | Curse | ✅ |
| O8 | Rich Dividend | Black Dividend energy cap 10 → 15. | Curse | ✅ |
| O9 | Opening Knock | Start each battle with Three Knocks. | Curse | ✅ |
| O10 | Deepening Wound | Bleed deals damage twice before losing a stack. | Blade Warden | ✅ |
| O11 | Twin Bane | When you apply Bleed, also apply Entangle. | Blade Warden | 🔌 |
| O12 | Blinding Dust | Your Blind cancels on 1–3 instead of 1–2. | Any | ✅ |
| O13 | Iron Will | You can't be Entangled. | Any | ✅ |
| O14 | Clear Sight | The first Blind on you each battle has no effect. | Any | ✅ |
| O15 | Heavier Swing | Strong Swing Ready gives +3 instead of +2. | Adventurer | ✅ |
| O16 | Coin‑Sense Gauntlet | Guard's Coin result gives +5 extra energy. | Adventurer | ✅ |
| O17 | Ledger Lens | Take Stock draws 1 more. | Adventurer | ✅ |

### P. Card flow, hand and timing

| # | Name | Effect | Inspired by | Rarity | Build |
|---|---|---|---|---|---|
| P1 | Echoing Tome | Your first card costing 15+ energy each battle resolves twice. | Necronomicon | R | 🔌 |
| P2 | Ivory Rebate | Playing a card costing 15+ refunds 5 energy. | Ivory Tile | R | 🔌 |
| P3 | Thrifty Ring | Cards costing exactly 5 cost 3. | — | R | 🔌 |
| P4 | Discount Band | The first card you play each round costs 2 less. | — | U | 🔌 |
| P5 | Recall Hook | Once per battle: return a card from discard to hand. | — | U | ✅ |
| P6 | Fast Hands | Hand‑limit discards go to the top of your draw pile instead. | — | U | 🔌 |
| P7 | Ready Sleeve | Choose a card at battle start; you may play it once for 0 at any time. | — | R | 🔌 |
| P8 | Peek Glass | You can see the top 2 cards of your draw pile. | Frozen Eye | C | 🔌 |
| P9 | Sorting Lens | At Income, look at the top 2 cards; keep one on top, put the other on the bottom. | Melange | U | 🔌 |
| P10 | Echo Seal | Once per battle, play a card a second time by paying its cost again. | — | R | 🔌 |
| P11 | Flexible Timing | Once per round, a "Before" card may be played "After" (or the reverse). | — | R | 🔌 ⚠️ |
| P12 | Reaction Charm | Your Any‑time cards may also be played in the offensive reaction moment. | — | B | 🔌 ⚠️ |
| P13 | Card Pocket | One card in hand each round is safe from damage. | — | U | 🔌 |

### Q. Information (see §7 for rules)

| # | Name | Effect | Rarity | Build |
|---|---|---|---|---|
| Q1 | Bestiary Page | Full enemy ability boards, deck sizes and rules on hover from battle 1. | C | 🔌 |
| Q2 | Spyglass | One random enemy commits its plan first; you see it, then finish planning. | R | 🔌 |
| Q3 | Omen Bones | At round start, reveal which enemies will target you this round. | U | 🔌 |
| Q4 | Dice Whisper | After the first roll, see one enemy's kept dice. | R | 🔌 |
| Q5 | Card Reader | See each enemy's hand size and the top card of its draw pile. | C | 🔌 |
| Q6 | Weakness Glass | See enemy defensive abilities and their expected prevention against your attack. | C | 🔌 |
| Q7 | Fortune Teller's Deck | See the enemies in upcoming map battles. | C | 🗺️ |
| Q8 | Echo Map | See the contents of unknown map rooms. | C | 🗺️ |
| Q9 | Tactician's Hourglass | You plan after all enemies commit (full reveal); −1 offensive roll. | B | 🔌 |

### R. Encounters and enemies

| # | Name | Effect | Inspired by | Rarity | Build |
|---|---|---|---|---|---|
| R1 | Trophy Rack | Elites drop an extra relic. | Black Star | B | 🗺️ |
| R2 | Hunter's Trap | Elites start with 2 cards removed. | Preserved Insect | U | ✅ |
| R3 | Lucky Start | Enemies in your next 3 battles start at half health. | Neow's Lament | E | 🗺️ |
| R4 | Intimidation Banner | Minion enemies start Sapped. | — | C | ✅ 🆕 |
| R5 | Frail Hunt Pelt | 6 random future battles: enemies start at 1 health. | Fur Coat | B | 🗺️ |
| R6 | Escape Rope Charm | Once per run, flee a normal battle with no new wounds. | — | U | 🗺️ |
| R7 | Reward Poster | Elite kills give +50 gold. | — | C | 🗺️ |
| R8 | Champion's Challenge | Elites gain +2 health but drop 2 relic choices. | — | U | 🗺️ |
| R9 | Peace Flag | Normal battles may be skipped once per act (no rewards). | — | C | 🗺️ |

### S. Gold

| # | Name | Effect | Inspired by | Rarity | Build |
|---|---|---|---|---|---|
| S1 | Golden Idol Coin | +25% gold. | Golden Idol | U | 🗺️ |
| S2 | Lucky Purse | +10 gold per enemy defeated. | Amethyst Aubergine | C | 🗺️ |
| S3 | Piggy Vault | +12 gold per map node until you next spend gold. | Maw Bank | C | 🗺️ |
| S4 | Old Coin Purse | +200 gold now. | Old Coin | C | 🗺️ |
| S5 | Merchant's Seal | Shop prices −25%. | Membership Card | Sh | 🗺️ |
| S6 | Restocker's Bell | Shop restocks after each purchase; prices −10%. | The Courier | Sh | 🗺️ |
| S7 | Alchemist's Exchange | Better gold → XP conversion rate. | — | U | 🗺️ |
| S8 | Scholar's Grant | Gain XP equal to 10% of gold earned. | — | U | 🗺️ |
| S9 | Interest Ledger | After each battle, +10% of your gold (max 30). | — | U | 🗺️ |
| S10 | Bargain Hunter | One random shop item is free each visit. | — | R | 🗺️ |
| S11 | Gambler's Coin | After a battle, flip: double the gold reward or lose it. | — | C | 🗺️ |
| S12 | Gilded Pips | Each Coin in your final offensive pool: +1 gold. | — | C | 🔌 🗺️ |
| S13 | Tax Collector | Enemies drop gold equal to their leftover energy. | — | U | 🗺️ |
| S14 | Greed's Bargain | +333 gold; −2 max health. | Cursed Pearl | E | 🗺️ |
| S15 | Pawnbroker's Ring | Sell relics at shops. | — | Sh | 🗺️ |
| S16 | Card Pawn | Sell cards for gold (removal costs health). | — | Sh | 🗺️ |

### T. Map, rest and events

| # | Name | Effect | Inspired by | Rarity | Build |
|---|---|---|---|---|---|
| T1 | Trail Boots | Ignore map paths 3 times. | Wing Boots | U | 🗺️ |
| T2 | Calm Bracelet | Unknown rooms never contain normal fights. | Juzu Bracelet | C | 🗺️ |
| T3 | Treasure Nose | Every 4th unknown room is a chest. | Tiny Chest | C | 🗺️ |
| T4 | Camp Tent | Take two rest actions. | Miniature Tent | R | 🗺️ |
| T5 | Forge Hammer | Rest action: upgrade 2 cards or 1 ability. | — | U | 🗺️ |
| T6 | Training Weights | Rest action: +1 permanent damage (max 3). | Girya | U | 🗺️ |
| T7 | Gardener's Spade | Rest action: dig for a relic. | Shovel | R | 🗺️ |
| T8 | Monk's Pipe | Rest action: remove a card with an XP refund. | Peace Pipe | U | 🗺️ |
| T9 | Dream Journal | Resting also offers a card reward. | Dream Catcher | C | 🗺️ |
| T10 | Rested Vigor | The battle after a rest: +10 energy at start. | Ancient Tea Set | C | 🗺️ |
| T11 | Campfire Stories | Rest action: gain 15 XP. | — | C | 🗺️ |
| T12 | Event Compass | Events offer an extra option. | — | U | 🗺️ |

### U. Card trees, XP and ability progression

| # | Name | Effect | Inspired by | Rarity | Build |
|---|---|---|---|---|---|
| U1 | Golden Ledger | +25% XP from battles. | Golden Idol | U | 🗺️ |
| U2 | Tree Eggs (×3) | Bought offense / defense / utility cards arrive one step upgraded. | Molten / Toxic / Frozen Egg | U | 🗺️ |
| U3 | Star Chart | Upgrade 3 cards one step for free. | Astrolabe | B | 🗺️ |
| U4 | Grindstone / War Pigment | Upgrade 2 random offensive / defensive cards. | Whetstone / War Paint | C | 🗺️ |
| U5 | Fresh Ledger | Remove 2 cards for a full XP refund (costs health). | Empty Cage | B | 🗺️ |
| U6 | Twin Reflection | Duplicate a deck card (+1 health), ignoring the copy limit. | Dolly's Mirror | R | 🗺️ |
| U7 | Fresh Start | Replace all starter cards with random pool cards. | Pandora's Box | B | 🗺️ |
| U8 | Wide Eyes | Reward choices offer +1 option. | Question Card | U | 🗺️ |
| U9 | Second Thoughts | Reroll each reward once. | Driftwood | U | 🗺️ |
| U10 | Borrowed Lore | Cards from one other pool become available. | Prismatic Shard | B | 🗺️ |
| U11 | Mentor's Discount | Ability upgrades cost 50% less XP. | — | U | 🗺️ |
| U12 | Branch Grafter | One card may take a forbidden tree connection. | — | R | 🗺️ |
| U13 | Fair Trader | Downgrades refund 100%. | — | C | 🗺️ |
| U14 | Apprentice's Notes | Each card bought refunds 1 XP. | Lucky Fysh | C | 🗺️ |
| U15 | Master's Seal | One ability gets a free top‑tier upgrade. | Archaic Tooth | B | 🗺️ |
| U16 | Skip‑Reward Bowl | Skipping a card reward gives +1 max health instead. | Singing Bowl | U | 🗺️ |
| U17 | Steady Upgrades | Every 3 battles, upgrade a random card one step. | Fishing Rod | U | 🗺️ |
| U18 | Card Enchanter's Kit | Apply a permanent card **Inscription** (+1 damage, −2 cost, draws 1, …) to up to 3 cards. | StS2 enchantments | R | 🗺️ 🆕 |

### V. Consumable interaction

| # | Name | Effect | Inspired by | Rarity | Build |
|---|---|---|---|---|---|
| V1 | Bandolier | +2 consumable slots. | Potion Belt | C | 🗺️ |
| V2 | Alchemist's Satchel | Battle start: fill one empty slot with a random consumable. | Delicate Frond | U | 🗺️ |
| V3 | Potent Brew | Consumables are twice as effective. | Sacred Bark | B | 🗺️ |
| V4 | Herbalist's Pouch | Every battle reward includes a consumable. | White Beast Statue | U | 🗺️ |
| V5 | Abstainer's Buckle | +1 damage while you hold no consumables. | Belt Buckle | U | 🗺️ |
| V6 | Thrifty Cork | The first consumable each battle has a 50% chance not to be used up. | — | R | 🗺️ |
| V7 | Brewer's Cauldron | Gain 5 random consumables now. | Cauldron | Sh | 🗺️ |
| V8 | Tonic Rush | Using a consumable: +2 energy. | Reptile Trinket | C | 🗺️ |

### W. Quest, delayed, decaying and evolving

| # | Name | Effect | Inspired by | Rarity | Build |
|---|---|---|---|---|---|
| W1 | Sealed Wager | After 4 battles: choose 1 of 3 relics. | Wongo's Mystery Ticket | E | 🗺️ |
| W2 | Unforged Blade | After defeating 5 elites, becomes Keen Whetted Edge (+2 damage). | Sword of Stone | Q | 🗺️ |
| W3 | Wax Charms | Gain 4 relics; the leftmost melts every 3 battles. | Toy Box | B | 🗺️ |
| W4 | Battle Journal | Every 5 cards bought: +1 max health. | Book of Five Rings | C | 🗺️ |
| W5 | Seed Pod | After 3 battles, becomes a random Rare relic. | — | E | 🗺️ |
| W6 | Hatchling Egg | Evolves by play style: deal 50 damage → offensive relic; prevent 50 → defensive relic; play 40 cards → card‑flow relic. | — | Q | 🗺️ |
| W7 | Grudge Stone | After 20 wounds taken, becomes +2 damage. | — | Q | 🗺️ |
| W8 | Heirloom | Its bonus grows each act (+1 / +2 / +3 energy at battle start). | — | R | 🗺️ |
| W9 | Hunter's Contract | Defeat a named enemy type for 150 gold. | — | Q | 🗺️ |
| W10 | Ember Tea Tin | +1 damage for the next 5 battles. | Ember Tea | E | 🗺️ |

### X. Starter relics (with upgraded versions)

| # | Name | Base | Upgraded | Pool |
|---|---|---|---|---|
| X1 | Early Riser's Band | +1 card in opening hand. | +1 Income card in rounds 1–3. | Adventurer |
| X2 | Field Kit | After each battle, mend 1 wound. | Mend 2. | Adventurer |
| X3 | Kindling Pouch | Round 1: +5 energy. | Rounds 1 and 2: +10 energy. | Adventurer |
| X4 | Worn Shield Strap | First defensive ability each battle prevents +1. | Each round's first defensive ability prevents +1. | Adventurer |
| X5 | Traveler's Die | Round 1: +1 offensive roll. | +1 offensive roll in rounds 1 and 3. | Adventurer |
| X6 | Venom Starter | Battle start: 1 Poison on a random enemy. | 1 Poison on all enemies. | Venom |
| X7 | Curse Starter | Battle start: 1 Curse Count on each enemy. | 3 Curse Count. | Curse |
| X8 | Blade Warden Starter | First attack each battle applies 1 Bleed. | Each round's first attack applies 1 Bleed. | Blade Warden |

### Y. Set bonuses

| # | Name | Effect | Build |
|---|---|---|---|
| Y1 | Traveler's Set | Own 3 travel‑themed relics: +1 Income card. | 🗺️ |
| Y2 | Smith's Set | Own 3 forge relics: +1 damage. | 🗺️ |
| Y3 | Healer's Set | Own 3 healing relics: after each battle, mend 1 more. | 🗺️ |
| Y4 | Gambler's Set | Own 3 dice relics: +1 offensive roll in round 1. | 🗺️ |

### Z. Run‑start offers (clean vs cursed pairs)

| # | Clean | Cursed |
|---|---|---|
| Z1 | +50 XP | +120 XP, −1 max health. |
| Z2 | +100 gold | +250 gold; your first 3 battles start with Sapped. |
| Z3 | Upgrade 1 card | Upgrade 3 cards; start each battle with Entangle. |
| Z4 | +1 max health | +3 max health; wounds can't be mended at the first rest. |
| Z5 | A random Common relic | A random Rare relic; −15% gold. |
| Z6 | 2 consumables | 5 consumables; −1 hand limit for the first act. |

---

## 5‑DT. Dice Throne–inspired items

Mined from `docs/game-design/dice-throne-reference/`, which has 122 statuses
plus passives, companions, potions, charge cards, forms and spellbooks across
about 40 heroes. Dice Throne is much closer to our game than Slay the Spire
(dice, abilities, energy‑like CP, upkeep statuses), so many of its *token*
mechanics become clean relics: "start each battle with N tokens of X" or "gain
X when Y".

**Translation:**
- 1 CP ≈ 5 energy.
- Dice Throne damage, healing and prevention ÷2 (their ~50 HP vs our ~20 cards),
  minimum 1. "+3 dmg" → +2, "Heal 2" → heal 1, "prevent 3" → prevent 2.
- "½ of a die" → ⌈die ÷ 3⌉ (1–2).
- **Names are original;** the "Inspired by" column cites the Dice Throne
  mechanic only.

**New mechanics this section introduces:**
- **Undefendable attacks:** the target can't use a defensive ability against
  the attack. Prevention cards, statuses and relic prevention still work.
- **Dodge rolls:** spend a token, roll, and on success the whole attack is
  prevented.
- **Companions:** familiars with their own health.
- **Ability sockets:** attachments on your abilities that are spent when the
  ability activates.
- **Charge‑and‑explode cards.**
- **Alternating forms.**
- **A spellbook:** prepare a card now, cast it later.
- **Spend‑anytime token menus.**

### DA. Attack modifiers

| # | Name | Effect | Inspired by | Rarity | Build |
|---|---|---|---|---|---|
| DA1 | Sure‑Shot Sigil | Once per battle, make your attack undefendable. | Accuracy | R | 🔌 🆕 |
| DA2 | Bolt Knuckle | After your final roll each round, roll 1 die; on a 6 your attack is undefendable. | Guard Break | U | 🔌 🆕 |
| DA3 | Updraft Charm | Once per battle, roll 2 dice; any 6 makes your attack undefendable or fully prevents an attack against you. | Flight | R | 🔌 🆕 |
| DA4 | Sly Dagger | Once per battle, add ⌈one die ÷ 2⌉ to your attack's damage. | Sneak Attack | C | 🔌 |
| DA5 | Cartridge Belt | Every 2nd round gain **Chambered** (cap 2); spend after your attack for +⌈die ÷ 3⌉ damage. | Reload / Quick Draw | U | ✅ 🆕 |
| DA6 | Twin Comet Ring | Once per battle, roll 2 dice and add ⌈higher ÷ 2⌉ damage. | Cosmic Ray | U | 🔌 |
| DA7 | Finisher's Crest | If your attack deals 5+ damage, +2 damage (once per round). | Crit | U | 🔌 |
| DA8 | Discipline Sash | If you used fewer than all your rolls, gain **Resolve** (cap 2); spend 1 for +1 damage or 2 for +3. | Honor + Bushido | U | ✅ 🆕 |
| DA9 | Breath Beads | Start each battle with **Poise 2**; spend one to prevent 1 or to add 1 damage. | Chi | U | ✅ 🆕 |
| DA10 | Fury Totem | Once per round, discard a card from hand for +2 damage on your attack. | Rage | U | 🔌 |
| DA11 | Shock Coil | Each card you lose gives **Charge** (cap 6); +1 damage per 2 Charge. At 6: 3 damage to an enemy, draw 2, +10 energy, reset. | Kinetic Energy / Energy Absorption | R | 🔌 🆕 |
| DA12 | Galvanic Battery | When an attack removes your cards, gain **Spark** (1, or 2 if it removed 4+); spend for +1 damage each (max 2 per round). | Ionic Drive | U | 🔌 🆕 |
| DA13 | Sparking Charm | Once per battle, +1 damage per status on you (max 3). | Crackle | U | 🔌 |
| DA14 | Price on the Head | At battle start, mark an enemy; attacks against it deal +1 and the attacker gains 5 energy. | Bounty | U | ✅ 🆕 |
| DA15 | Clockwork Mites | Your attacks add **Mite** (cap 3). At Ongoing Effects the target rolls per Mite (6 removes one). You may detonate any time for 1/2/3 damage at 1/2/3 Mites. | Nanite | R | ✅ 🆕 |
| DA16 | Shinobi Knot | Gain 1 **Knot** each battle. Spend after your attack and roll: 1–3 +1 damage, 4–5 +2 damage, 6 your choice of +2, undefendable or 1 Poison. | Ninjutsu | R | 🔌 🆕 |
| DA17 | Second Wave Charm | Once per battle, after your attack resolves, perform another offensive roll against the same enemy. | Combo / Glide | B | 🔌 |
| DA18 | Fresh Throw Token | Once per battle, restart your offensive roll with full rolls. | Déjà Vu | U | 🔌 |
| DA19 | Leech Fang | Once per round, when your attack removes 2+ cards, heal 1. | Steal Health | R | 🔌 |
| DA20 | Crown of Fives | Five of a kind makes your attack undefendable. | Ultimate abilities | U | 🔌 🆕 |
| DA21 | Siphon Charm | Your attacks steal 5 energy from the target when they remove a card. | Pillage | U | 🔌 |

### DB. Dodges, counters and survival

| # | Name | Effect | Inspired by | Rarity | Build |
|---|---|---|---|---|---|
| DB1 | Sidestep Charm | Start each battle with **Slip 2** (cap 3). Spend one and roll 1 die: 1–2 fully prevents the attack. Multiple Slips may be spent on one attack. | Evasive | U | 🔌 🆕 |
| DB2 | Haze Cloak | Once per battle, roll 1 die: 1–3 fully prevents one attack. | Smoke Bomb status | U | 🔌 |
| DB3 | Tumbler's Wrap | **Tumble 2**: spend and roll 1–3 to halve an attack (rounded up). | Agility | U | 🔌 🆕 |
| DB4 | Halving Aegis | Start each battle with **Halve 1**: halve one attack (rounded up). | Protect (½) / Force Field | U | ✅ 🆕 |
| DB5 | Reprisal Seal | Once per battle, after an attack removes your cards, deal half that damage back (rounded up). | Retribution | U | 🔌 |
| DB6 | Counter Blade | Once per battle when attacked, roll 1 die and deal ⌈die ÷ 3⌉ to the attacker. | Back Strike | C | 🔌 |
| DB7 | Redirect Cloak | Once per battle, prevent 2 from an attack and deal 2 to any enemy. | Wind Shear | U | 🔌 |
| DB8 | Heirloom Plating | Once per battle, prevent 2. | Vibranium Suit | C | ✅ |
| DB9 | Mirror Image | Once per battle when attacked, the attacker picks 1 of 3 face‑down cards; 2 of 3 make the attack miss. | Illusion | R | 🔌 |
| DB10 | Guardian Wisp | Once per run, when you would be defeated, you stay at 1 card. | Blessing of Divinity | R | 🔌 🗺️ |
| DB11 | Hollow Vessel | Once per run, when defeated, heal 5 cards but gain no Income energy for the rest of that battle. | Resurrect | B | 🔌 🗺️ |
| DB12 | Phase Shroud | The first attack against you each battle deals no damage (its other effects still apply). | Shadows | R | 🔌 |
| DB13 | Sunlit Shield | Once per battle, change one defensive die to a 6. | Radiance | U | 🔌 |
| DB14 | Status Ward | Once per battle, block an incoming negative status. | Dryad | C | 🔌 |
| DB15 | Purifying Bead | Start each battle with **Purity 2**; spend one to remove a status from yourself. | Cleanse | U | ✅ 🆕 |
| DB16 | Absorbing Buckler | Each card you lose gives +1 to your next defensive ability's prevention (max +2). | Energy Absorption (defensive) | U | 🔌 |

### DC. Dice manipulation

| # | Name | Effect | Inspired by | Rarity | Build |
|---|---|---|---|---|---|
| DC1 | Fate's Thumb | Twice per battle, change an even die ±1. | Probability Manipulation | U | 🔌 |
| DC2 | Glimmer Charm | Start with **Glimmer 3**: spend 1 to reroll a die, or 3 to set a die to 6. | Cheer | R | ✅ 🆕 |
| DC3 | Beguiling Eye | Once per battle, roll 1 die; on 5–6, force an enemy to reroll one revealed die. | Mesmerize | R | 🔌 ⚠️ |
| DC4 | Saboteur's Pin | Once per battle, force a reroll of one die in an enemy's defensive roll. | Infiltration | U | 🔌 |
| DC5 | Gale Gatherer | Once per battle, borrow an enemy's die for your offensive roll (6 dice). | Tornado | R | 🔌 |
| DC6 | Loaned Die | Once per battle, an enemy must use one of your dice this round. | Reality Warp | R | 🔌 |
| DC7 | Echo Die | Roll an extra special die with your first roll each round. Set it aside, then use it to replace one die or add ⌈value ÷ 3⌉ damage. | Manifest | R | 🔌 🆕 |
| DC8 | Rime Trap | Round 1: each enemy's lowest die after its first roll is locked. | Dice Cube | U | 🔌 |
| DC9 | Thorn Snare | Round 1: enemies take 1 damage per reroll beyond their first (max 2). | Barbed Vine | U | 🔌 |
| DC10 | Strangling Coil | Once per battle, an enemy must pay 5 energy per reroll next round. | Constrict | U | 🔌 |
| DC11 | Doubt Seed | Your attacks apply **Doubt** (cap 3). At Offensive entry the target rolls 1 die per stack; each 1–3 costs it a roll. | Influence | U | ✅ 🆕 |
| DC12 | Null Sigil | Once per battle, an enemy's 6s count as blank this round. | Hex | U | 🔌 |
| DC13 | Brittle Touch | Once per battle, an enemy may reroll at most 2 dice after its first roll, and its attack is −1. | Decrep‑ify | U | 🔌 |
| DC14 | Toll Purse | Any time: pay 5 energy to reroll one die; pay 15 energy to draw a card. | Paladin passive | R | 🔌 |
| DC15 | Roll Allotment | At round start, choose a safe charge (roll 2 dice) or a risky one (roll 5; a 1 costs you something) to fill a charge relic (DG2). | High Stakes | U | 🔌 |

### DD. Enemy afflictions (applied by relics)

| # | Name | Effect | Inspired by | Rarity | Build |
|---|---|---|---|---|---|
| DD1 | Rotting Coin | One enemy has **Decay 1** all battle: its attacks deal −1. | Wither | R | ✅ 🆕 |
| DD2 | White Flag Banner | Once per battle, an enemy's attack this round deals no damage (its statuses still apply). | Parlay | R | 🔌 |
| DD3 | Tripwire | Once per battle, an enemy must pay 10 energy or skip its next offense. | Knockdown | R | 🔌 🆕 |
| DD4 | Ringing Maul | Once per battle, an enemy skips its next Income. | Concussion | U | 🔌 |
| DD5 | Dazing Club | Once per battle, an enemy can't defend against your attack or play cards this round. | Stun | R | 🔌 🆕 |
| DD6 | Tangling Ribbons | Once per battle, an enemy can't play cards next round. | Crimson Bands | U | 🔌 |
| DD7 | Silencing Ward | An enemy's next attack applies no statuses. | Paralyze | U | 🔌 |
| DD8 | Sealing Rune | At battle start, seal one enemy offensive ability (never its only one) for 2 rounds. | Spellbound | R | 🔌 🆕 |
| DD9 | Blasting Cask | At battle start an enemy gets **Cask**. At Ongoing Effects it rolls: 1–2 takes 2 damage and the Cask is consumed; 6 lets it pass the Cask to any actor. | Powder Keg | U | ✅ 🆕 |
| DD10 | Ember Brand | An enemy starts with **Smolder**: 1 damage at each Ongoing Effects until cleansed. | Burn | R | ✅ 🆕 |
| DD11 | Ticking Fuse | An enemy gets a 2‑round **Fuse**: it rolls at Ongoing Effects and a 6 defuses it; otherwise it takes 3 damage when the fuse runs out. | Time Bomb | U | ✅ 🆕 |
| DD12 | Lingering Venom | Your first attack applies **Lingering Toxin**: 2 damage at the end of the round. | Delayed Poison | U | ✅ 🆕 |
| DD13 | Soot Sack | Enemies gain **Soot** when they gain energy; at 4 Soot, their next gain is −5. | Coal | C | 🔌 🆕 |
| DD14 | Discord Chime | Once per battle, an enemy loses 5 energy or takes 1 damage (its choice). | Disruption | C | 🔌 |
| DD15 | Quarry Mark | One enemy has **Quarry**. Attackers roll 1 die: 1 removes Quarry, 2–5 +1 damage, 6 +2 damage. | Prey | U | 🔌 🆕 |
| DD16 | Smoldering Brand | An enemy takes 1 damage at each Ongoing Effects, then rolls: 5–6 removes it. | Phoenix Burn | U | ✅ 🆕 |
| DD17 | Haunted Chest | Enemies holding **Grave Coin** take 1 damage per coin at Ongoing Effects; you are immune. | Cursed Doubloon | R | ✅ 🆕 |

### DE. Passive engines

| # | Name | Effect | Inspired by | Rarity | Build |
|---|---|---|---|---|---|
| DE1 | Fading Starburst | Start with **Glow 3**: each Ongoing Effects deals Glow damage to each enemy, then Glow drops by 1. | Cosmic Flare | R | ✅ 🆕 |
| DE2 | Gleaming Halo | While you hold **Halo**: 1 damage to each enemy at each Ongoing Effects (cap 2, removable). | Holy Presence | R | ✅ 🆕 |
| DE3 | Trickster's Pouch | At each Ongoing Effects roll 1 die: 1 lose 5 energy; 2–5 choose heal 1, +5 energy, or 1 damage to an enemy; 6 +10 energy. | Bag of Tricks | U | ✅ |
| DE4 | Springwater Flask | Once per battle, roll 1 die and heal 1 (1–3) or 2 (4–6). | Wellspring / Donut | U | 🔌 |
| DE5 | Mulled Cup | Every 3rd round roll 1 die: 1–2 deal 1 damage to an enemy; 3–4 heal 1 and +5 energy; 5–6 heal 1 and draw 1. | Egg Nog | U | ✅ |
| DE6 | Sprout Charm | Each round grow a Sprout: **Seedling** (spend: reroll a die) → **Sapling** (spend: heal 1, +5 energy) → **Grove Elder** (spend: +2 damage or block a status). | Treant spirits / Fertilize | R | ✅ 🆕 |
| DE7 | Twin Phase Pendant | Alternate forms each round. **Calm:** +1 Income card. **Fury:** no Income, +2 damage, +1 roll. | Jean Grey / Dark Phoenix, Pale Lady flip | B | 🔌 🆕 |
| DE8 | Turncoat Coin | Start Cursed: take 1 unpreventable damage at each Ongoing Effects for 3 rounds; enemies whose attack whiffs gain a Cask. Afterward flip: +2 damage for the battle. | Cursed Pirate | B | ✅ 🆕 |
| DE9 | Overflow Vault | Energy above the cap is stored as **Surplus** (max 5, 5 energy each); spend 5 Surplus to use a second offensive ability this round. | The Freezer / Glide | R | 🔌 🆕 |
| DE10 | Tome of Preparation | Once per round, **Prepare** a card from hand onto the Tome (small effect now); later **Cast** it for free at any time. | Book of Vishanti | R | 🔌 🆕 |
| DE11 | Grave Dial | Gain **Remains** when enemies lose cards. Spend 2/3/6 to summon (DF3), or 10 for a one‑time heal 5. | Corpse dial | R | 🔌 🆕 |
| DE12 | Gravedigger's Spade | Each round you may seal one of your offensive abilities for the round to gain 5–10 energy (by tier). | Unearth | U | 🔌 |
| DE13 | Dread Lantern | Pass the Lantern to an enemy. While an enemy holds it, gain **Dread** at round end; at 4 Dread, that enemy loses its next roll. | Haunted Head | U | ✅ 🆕 |
| DE14 | Drillmaster's Ledger | While 5+ cards in your deck are upgraded: +1 damage. | Red Room Training | U | ✅ |
| DE15 | Borrowed Glory | Once per battle, copy a positive status an enemy has. | Conjure | R | 🔌 |
| DE16 | Strategist's Tokens | Start with **Plan 3** (cap 5). Spend 1: +5 energy or reroll a die. Spend 3: draw 1. Spend 4: gain 2 Protect. | Tactical Advantage / Support | R | ✅ 🆕 |
| DE17 | Gleaner's Basket | Each round you prevent all damage, gain **Gleanings**; spend 2 to heal 1. | Dissolution | U | 🔌 🆕 |

### DF. Companions (new family)

| # | Name | Effect | Inspired by | Rarity | Build |
|---|---|---|---|---|---|
| DF1 | Loyal Hound | A familiar with 5 health. You may send an attack's full damage to it. While active, +1 damage on your attacks, and once per round it can take 1 damage to let you reroll a die. When downed it heals 1 per round and returns at 5. | Nyra | B | 🔌 🆕 |
| DF2 | Returning Maul | Discard a card to throw it (+2 damage and the enemy can't defend) or to retrieve it (gain 2 Protect). | Mjölnir | R | 🔌 🆕 |
| DF3 | Bone Retinue | Spend Remains (DE11) to summon a **Bone Soldier** (+1 damage), **Bone Seer** (enemy −1 damage) or **Bone Brute** (+2 damage, or sacrifice it to prevent 2 and deal 2). | Undead | R | 🔌 🆕 |
| DF4 | Tinker's Kit | Gain **Gears** each round; build bots: **Spark Bot** (+2 damage), **Patch Bot** (heal 1 after you lose 3+ cards), **Mite Bot** (detonates Mites). | Artificer bots / Synth | R | 🔌 🆕 |
| DF5 | Imp Satchel | A deck of 6 imps; draw one every 2nd round. Each is a one‑time effect: prevent 2, +2 damage and heal 1, an extra roll, undefendable, +1 damage and Soot, +2 damage. | Krampus Rejects | R | 🔌 🆕 |
| DF6 | Crow Familiar | Moves between you and an enemy. On an enemy it drains 5 energy per round; on you it rerolls a die. | Nevermore | R | 🔌 🆕 |

### DG. Sockets and charges

| # | Name | Effect | Inspired by | Rarity | Build |
|---|---|---|---|---|---|
| DG1 | Brewer's Rack | Each ability gains 1 socket. At round start, attach a brew (Brawler +1 damage / Bulk prevent 1 / Clear remove a status / Venom 1 Poison); spend it when that ability activates. You may discard a brew to reroll a die. | Alchemist potions / The Lab | R | 🔌 🆕 |
| DG2 | Capacitor Charms | 3 charms with charge slots filled by dice results at round start. A full charm **bursts** for its effect (3 damage / draw 2 / +15 energy), then resets. | Gambit Ace cards | R | 🔌 🆕 |
| DG3 | Bolt Sigil | Each round you may **charge** one ability; when used, it deals +1 damage or gains its bonus effect. | Storm Lightning | U | 🔌 🆕 |
| DG4 | Inscribed Ability | One ability permanently gains a socket that refills every battle. | — | U | 🗺️ 🆕 |

---

## 6. Rarity, pool and offer rules

- **Rarity weights** like StS: Common/Uncommon/Rare at 50/33/17.
- **Sources:**
  - Run start: 3 offers, including one clean/cursed pair.
  - Normal battle: XP and gold, with a small chance of a Common relic.
  - Elite battle: choose 1 of 3 relics.
  - Boss / act milestone: choose 1 of 3 drawback‑family relics (B tier).
  - Shops sell relics and consumables. Events give event relics. Quest relics are tracked.
- **Pool hygiene:**
  - No duplicate relics, and skipped relics go back into the pool.
  - Character relics only appear for their character.
  - Gate offers on deck contents (no Venom amplifiers without Poison).
  - Remove late‑useless relics in the final act.
- **Readability:**
  - The relic bar shows icons, counters and a trigger flash, and greys out spent relics.
  - Hover shows the full rules, with any drawback in the same text.
  - Any buff, charge or expiring effect also appears as a status.

## 7. Information relics — doing them correctly

- The reveal must be **authoritative**: the server publishes the revealed fact to
  that viewer's safe state. The client never infers it.
- Prefer **commit‑order** designs (Q2, Q9), where an enemy commits before you and
  the reveal is a real rule. Avoid peeking at undecided choices.
- Every reveal must follow the attack‑hover contract: name, tier, rules, dice and
  result. A partial reveal says clearly what is hidden.
- Pay for information with tempo: −1 roll, one random enemy only, or once per battle.

## 8. Consumables (potion equivalents)

**Rules sketch:**
- Slots: 3 to start, more from relics.
- Use them in normal card windows, but never during Ongoing Effects.
- Some are usable only out of battle.
- Any lasting effect becomes a status.
- Healing works both in and out of battle; in‑battle healing reuses the battle's wound ledger.

| # | Name | Effect | When |
|---|---|---|---|
| CN1 | Linen Bandage | Mend 1 wound. | Out of battle |
| CN2 | Sutured Bandage | Mend 2 wounds. | Out of battle |
| CN3 | Battlefield Gauze | Mend 1 card removed this battle. | Battle |
| CN4 | Elixir of Renewal | Mend all wounds. | Out of battle (Rare) |
| CN5 | Vital Draught | +1 max health permanently. | Out of battle |
| CN6 | Small Tonic | +5 energy. | Battle |
| CN7 | Energy Tonic | +10 energy. | Battle |
| CN8 | Overflow Draught | +20 energy; cap +20 this battle. | Battle |
| CN9 | Scroll of Insight | Draw 2. | Battle |
| CN10 | Scroll of Recall | Return 2 discards to hand. | Battle |
| CN11 | Scroll of Sorting | Look at the top 4 cards and reorder them. | Battle |
| CN12 | Luck Token | Reroll one die. | Battle |
| CN13 | Weighted Pip | Set one die to any face. | Battle |
| CN14 | Roll Token | +1 offensive roll this round. | Battle |
| CN15 | Flip Token | Flip one die. | Battle |
| CN16 | Ward Potion | Gain Ward 4. | Battle |
| CN17 | Fog Flask | Fully prevent one attack. | Battle |
| CN18 | Iron Draught | Gain 3 Protect. | Battle |
| CN19 | Plate Oil | Gain 3 Plate. | Battle |
| CN20 | Fury Draught | +3 damage on your next attack. | Battle |
| CN21 | Fire Flask | 3 damage to one enemy. | Battle |
| CN22 | Shrapnel Bomb | 2 damage to each enemy. | Battle |
| CN23 | Purging Draught | Remove your negative statuses. | Battle |
| CN24 | Venom Vial | 2 Poison on an enemy. | Battle |
| CN25 | Blinding Powder | Blind an enemy. | Battle |
| CN26 | Tangle Net | Entangle an enemy. | Battle |
| CN27 | Bleed Shard | 2 Bleed on an enemy. | Battle |
| CN28 | Exposing Dust | Exposed 2 on an enemy. | Battle |
| CN29 | Calming Tea | Sapped 2 on an enemy. | Battle |
| CN30 | Escape Rope | Flee a normal battle (no rewards). | Battle |
| CN31 | Liquid Courage | Your attack this round gains choose_tier +1 tier. | Battle |
| CN32 | Second Chance Vial | Retry your offensive roll from scratch. | Battle |
| CN33 | XP Tome | +15 XP. | Out of battle |
| CN34 | Gold Pouch | +60 gold. | Out of battle |
| CN35 | Whetting Oil | One card counts as upgraded until the end of the next battle. | Out of battle |
| CN36 | Changeling Dust | Transform a card. | Out of battle |
| CN37 | Relic Polish | Advance a counter relic by 3. | Any |
| CN38 | Mirror Draught | Copy the last consumable you used. | Battle |
| CN39 | Fortune Cookie | Choose 1 of 3 random consumables. | Out of battle |
| CN40 | Ration Pack | Add 2 temporary health cards for the next battle. | Out of battle |
| CN41 | Brawler's Brew | +2 damage on your next attack. *(Alchemist‑style)* | Battle |
| CN42 | Bulk Brew | Prevent 2 from one attack. | Battle |
| CN43 | Clear Brew | Remove a status from yourself, or heal 1. | Battle |
| CN44 | Rewind Sand | Restart your offensive roll with full rolls. | Battle |
| CN45 | Pickpocket's Glove | Steal 5 energy from an enemy. | Battle |
| CN46 | Extortion Note | Choose an enemy die; it pays 10 energy or rerolls it. | Battle |
| CN47 | Leech Draught | An enemy loses 2 cards; you heal 1. | Battle |
| CN48 | Honey Cake | Roll 1 die; heal 1 (1–3) or 2 (4–6). | Any |
| CN49 | Cask Flask | Give an enemy a Cask (DD9). | Battle |
| CN50 | Spy's Tonic | Look at an enemy's hand. | Battle |
| CN51 | Seer's Tea | Your next Income draws 2; put one on top or bottom of your deck. | Battle |
| CN52 | Mirror Shard | Copy a positive status from an enemy. | Battle |
| CN53 | Retrieval Charm | Return a card costing 10 or less from discard to your hand. | Battle |
| CN54 | Dodge Powder | Gain Slip 1 (DB1). | Battle |
| CN55 | Fuse Bomb | Give an enemy a 1‑round Fuse (DD11). | Battle |
| CN56 | Scholar's Draught | +25 XP (shop item; gold buys an XP burst). | Out of battle |

## 9. Gold economy sketch

- **Sources:** battle drops (scaled by encounter), elite and boss bonuses, events,
  selling relics, cards or consumables, quest bounties, gold relics (§S).
- **Sinks:** relics, consumables, card removal service, healer (mend wounds for
  gold), upgrade service, shop rerolls, and **XP‑burst items** (CN33, CN56,
  S8).
- **Gold and XP stay separate for now.** Direct gold → XP conversion (S7) is a
  TODO.
- **Tension:** gold buys run power now (items, healing). XP converts into
  permanent deck/ability growth during the run.

## 10. Build order

1. **Data only (✅):** about 60 items. Battle‑start resources and statuses,
   countdown and periodic statuses, offensive‑entry roll changes, battle‑long
   ability modifiers, dice loadouts, status amplifiers, enemy starting debuffs.
2. **Engine hooks (🔌):**
   - Income calculation.
   - Card played/drawn counters.
   - Damage commit and the event bus.
   - Defensive modifiers.
   - Die rolled / final pool.
   - Hand‑limit discard.
   - Battle end.
   - New concepts: Plate, Ward, Veil, Exposed, Sapped, Fury, Focus, Fortune,
     Thorns.
   - New Dice Throne–derived concepts: undefendable attacks, dodge rolls,
     spend‑anytime token menus, companions, ability sockets, charge/burst
     relics, forms, the Prepare/Cast spellbook, and a general authored heal
     step.
3. **Run layer (🗺️):** relic inventory and rewards, wounds across battles,
   mending, gold, shop, rest sites, map, events, consumable slots.

## 11. Remaining open questions and TODOs

1. **TODO — Gold → XP.** Base conversion rate and where conversion happens. Gold
   and XP stay separate until this is decided.
2. **Relics per run.** Target count (StS is 10–20) and boss‑tier frequency once
   run length is known.
3. **Healing implementation.** Replace the engine's revive‑trade validation with a
   general authored heal step that works for cards, statuses, relics and
   consumables.
