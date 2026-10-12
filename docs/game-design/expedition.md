# The Expedition: Mage Knight-style exploration

**Status:** design draft for review (2026-10-12). Nothing here is implemented yet.

The Expedition replaces the campaign's linear encounter list (see [Campaign progression](campaign.md)) with a map you explore. It borrows Mage Knight's exploration — one hand of cards pays for everything, the map grows tile by tile, your deck is your clock — and hands every fight to **our existing battle system, unchanged**.

The pictures are in [`expedition/`](expedition/).

## Contents

1. [Inspiration: Mage Knight](#1-inspiration-mage-knight)
2. [The bridge: one deck, two arenas](#2-the-bridge-one-deck-two-arenas)
3. [A region](#3-a-region)
4. [A turn](#4-a-turn)
5. [The clock](#5-the-clock)
6. [Energy](#6-energy)
7. [Encounters](#7-encounters)
8. [The Warlord](#8-the-warlord)
9. [Defeat and wounds](#9-defeat-and-wounds)
10. [Economy](#10-economy)
11. [Growth and gear](#11-growth-and-gear)
12. [Between regions](#12-between-regions)
13. [Worked example](#13-worked-example)
14. [Configurable values](#14-configurable-values)
15. [What it would take to build](#15-what-it-would-take-to-build)
16. [Open questions](#16-open-questions)
17. [Decision log](#17-decision-log)

---

## 1. Inspiration: Mage Knight

![Mage Knight](expedition/01-mage-knight.png)

**What we take:** one hand pays for walking, talking and fighting; terrain costs; exploring by placing new tiles; visible monsters you can provoke; a fixed number of Day and Night rounds that each last one deck; fortified sites; experience that makes you stronger.

**What we don't take:** Mage Knight's card-based combat. Every fight is a full battle in our system.

## 2. The bridge: one deck, two arenas

![One deck, two arenas](expedition/02-one-deck-two-arenas.png)

**A battle starts from your map state exactly as it is:**

| On the map | In the battle |
| --- | --- |
| Hand | Opening hand, **as it is** — no top-up |
| Draw pile | Draw pile (Income draws from it) |
| Spent cards (played this round) | Discard |
| Wounds | Removed |
| Current energy | Starting energy |

The battle then plays by today's rules, including no recycling of discard and damage taking cards **discard → draw → hand**. When it ends, whatever is left goes back to the map, the fight **ends your turn**, and you keep at most the energy you brought in (see [Energy](#6-energy)).

So on the map:

- **Which cards you walk with** decides which cards you can't draw in the next fight — and which get wounded first, because damage takes discard first.
- **When in a round you fight** decides how many Income draws you have left.
- Health never changes from walking: Spent cards still count as health. Only removal loses health.

## 3. A region

![A region](expedition/03-region-map.png)

- **Hex map built from random tiles**, like Mage Knight: a **start tile**, then **countryside tiles** (open terrain, villages, monasteries, ruins, roaming monsters, Lieutenant keeps), then harder **core tiles** (towns and cities, the Warlord's goal, his stronghold). First scenario: start tile, about 5 countryside tiles and 2 core tiles.
- **Explore:** standing next to an empty tile position, pay **2 Move** to place the next tile from the region's stack.
- **Fog of war:**
  - In sight (2 hexes by day, 1 at night): you see everything on those hexes.
  - Revealed but out of sight: the terrain is shown greyed out. Sites you've already seen stay on your map; roaming enemies are shown where you last saw them.
  - Never seen: terrain only, contents unknown.
- **On the map:** villages, monasteries, towns, ruins, grottos, **event sites**, Lieutenants (roaming on patrol, or **fortified** in a keep), groups of minions, occupied villages to free, and the Warlord.
- **Event sites** work like Slay the Spire's "?" rooms: stepping on one starts a short scene with 2–3 choices.
  - Choosing well earns a bonus (gold, XP, gear, a mended wound); choosing badly costs something (a wound, gold, a fight).
  - Some choices need something from you: Influence, gold, a die roll, or giving up a card for the round.

## 4. A turn

![A turn](expedition/04-turn.png)

1. **Start:** your hand holds up to your **hand size** (4). Cards you didn't play last turn are still there.
2. **Move and explore:** **1 card = 1 Move**; **5 energy = 1 Move**. Pay the terrain cost of each hex you enter. Exploring costs 2 Move. Cards whose effect gains energy (Second Wind) give that energy on the map; ordinary cards can't be turned into energy.
3. **One action:** use a site (paying Influence), challenge an adjacent enemy, or enter an adventure site.
4. **A fight**, if one starts, ends your turn.
5. **End the turn** whenever you like, as long as you've **played or discarded at least 1 card** this turn (a fight counts). Keep the cards you didn't play and **draw up to your hand size**. If your draw pile is empty, you **rest**.

**Hand symmetry:** the map and battles share one hand size and one hand limit (6). Anything that raises or lowers them — a level-up, gear, an enemy's "Shaken: hand size −1" status — applies to both. Effects like that are visible statuses with their expiry shown.

**Terrain costs (first draft):**

| Terrain | Day | Night | Notes |
| --- | ---: | ---: | --- |
| Plains / coast | 1 | 1 | |
| Hills | 2 | 2 | |
| Forest | 2 | 3 | Cover: counts as "unseen" for an Ambush |
| Marsh | 3 | 3 | |
| Lake / mountain | ✖ | ✖ | Impassable without gear |

## 5. The clock

![The clock](expedition/05-clock.png)

- A region lasts **4 rounds**, alternating **Day and Night** (configurable). Each round lasts **one pass through your deck**. What's limited is cards, not turns.
- **Rest** happens when your draw pile is empty, or **whenever you choose to make camp** (you give up the cards you didn't use):
  1. Spent cards are shuffled back into your draw pile. Wounds stay out. (This is an explicit recovery outside battle; battles still never recycle.)
  2. **Energy resets** to your character's base.
  3. Camp services: each bandage (or 3 Influence) is **one roll to mend one wound** (4+ on a d6); change your deck at camp prices.
  4. The Warlord marches 2 hexes toward his goal; roaming Lieutenants take one step.
  5. Day ↔ Night. Draw up to your hand size.
- **Bigger deck = longer rounds.** Buying cards adds health and time. **Wounds shorten rounds.**
- **Night** differs, like Mage Knight. Night does **not** make ambushes automatic.

| | Day | Night |
| --- | --- | --- |
| Terrain | Normal costs | Forest costs 3 |
| Sight (you and enemies) | 2 hexes | Adjacent hexes only |
| Enemy habits | Night hunters rest: they can be ambushed | Sleepers can be ambushed; night hunters roam, moving 1 hex each turn instead of each rest |
| Grottos and ruins | Normal rewards | Better loot, stronger guardian |
| Villages and towns | All services | Closed, except the inn: a safe rest, +1 on camp mending rolls |
| Monasteries | Open | Open |
| The Warlord | His own habits and weaknesses | His own habits and weaknesses; Fortified if he's camped in a tower or keep |
- After the last round you take the normal rest, then **the Warlord comes to you**: a forced fight, starting with a fresh hand, with every buff he still has and no Ambush.

## 6. Energy

![Energy](expedition/06-energy.png)

- **Base energy** is a character stat (Starter: 10). A higher base starts and resets higher.
- **One pool per round**, shared by the map and battles. Cards refill every turn; energy refills only when you rest.
- **Battle start:** your current energy. Battle Income still adds +5 per round.
- **Battle end:** you leave with the **smaller of** what you have and **what you brought in**. This stops anyone farming an easy fight for energy. Relics and gear may raise this limit.
- **Rest:** reset to base, whether you were above it (you lose the extra) or below it (you're refilled).

## 7. Encounters

![Encounters](expedition/07-encounters.png)

- **Provoked:** walking from one hex next to an enemy to another hex next to it starts a fight **now**, with whatever you hold. Never an Ambush.
- **Challenge:** your action; attack an enemy next to you with the hand you chose to keep.
- **Garrison:** entering a keep, tower or occupied town fights its defenders. Defenders inside walls are **Fortified**.
- **Groups:** a group can mix enemy types (four goblins; a captain and two eels). Stepping next to two groups can pull both into one battle.

**Ambush:**

- You **start the fight yourself** (never when provoked), **unseen**, against an enemy whose **habit** allows it right now.
- **Unseen** means either:
  - you **began your turn outside the target's sight** — enemies see the way you do: 2 hexes by day, 1 at night — so the approach itself matters; or
  - you attack **from cover**: a forest or ruins hex.
- **Habits:**

| Habit | Can be ambushed | Example |
| --- | --- | --- |
| Unwary | Day or night | Wandering beasts |
| Sleeper | At night | A bandit camp asleep |
| Night hunter | By day | An owlbear resting at noon |
| Always alert | Never (without tools) | Guards on walls |

- Skills, gear and relics can bend these rules.
- **Effect (default):** a visible **Ambush** status, used up on your first offensive roll. That roll uses **5 + 3 = 8 dice**; you may keep at most 5; each reroll goes back to 5 dice total (keep 3, reroll 2).
- Rare, strong versions (skills, relics) may add more dice, bonus damage, or **stun the enemy for round 1**. Battles stay **simultaneous**: nobody acts first.

**Fortified:** a visible status on defenders inside walls that strengthens their **defense**. Default: their defensive roll uses **1 extra die**. Alternative to test: each shield result prevents **2 instead of 1**. Siege gear removes it.

## 8. The Warlord

**His agenda.** Each region gives the Warlord a goal on a core tile, usually conquering a town. His route is laid out when the region is built.
- He starts **about 5 hexes** from his goal and marches **2 hexes each time you rest**.
- With no extra rests he arrives at the **third rest**, just before the last round.
- Every early camp and every defeat (a forced rest) **speeds him up**.
- He's shown where you last saw him. Once you've seen him, his next step is visible.

**Lieutenants** each give him a visible buff. Kill them to remove those buffs. A Lieutenant may roam on patrol or sit fortified in a keep.

**Catch him early.** On the road he's an enemy on a hex like any other:
- challenge him from an adjacent hex, **unfortified**;
- like any enemy, **each Warlord has his own habits and weaknesses**, by day or by night (one may be a night hunter, another may sleep in a camp). Learn them, approach unseen when his habit allows it, and you earn an **Ambush**. At night he's **Fortified** only if he's camped in a tower or keep;
- beating him before the deadline pays **+20 XP for each round you didn't use**, plus **one extra relic choice**.

**If he reaches his goal**, he takes the town:
- its services are gone;
- he waits there **Fortified**, and storming him means fighting through his walls;
- he gains a visible **Conqueror** buff (+1 defensive die). This is a consequence you could have prevented, not automatic growth.

**At the deadline** (after the last round) he comes to you, wherever you are:
- he brings every buff he still has, and there's no Ambush;
- he isn't Fortified in the open field.

**Losing to the Warlord ends the run.** Nothing else does.

## 9. Defeat and wounds

![Defeat](expedition/08-defeat.png)

**Losing a normal battle:**

1. Your wounds go back to their **pre-battle** state, **plus 2 defeat wounds**. A normal defeat can never take your last card.
2. You're **driven back** to the hex you came from. The enemy stays.
3. **Forced recovery:** the round ends with a rest where you stand. Bandages, gear and camp services help you mend. The clock has moved on.

**Mending, worst to best:**
1. **Camp:** at rest, each bandage (or 3 Influence) is **one roll to mend one wound**, succeeding on 4+ on a d6. Gear and relics can improve the odds.
2. **Village:** Influence per wound.
3. **Monastery:** cheaper, and possible mid-round.
4. **Town healer:** gold; also treats grievous wounds.

**Grievous wounds (later extension):** a wound that **cripples a card** instead of removing it. The card stays in your deck and counts as health but works worse (costs more energy, prevents less) until a healer fixes it. A good fit for characters like a barbarian.

## 10. Economy

![Economy](expedition/09-economy.png)

To avoid three overlapping systems, there are **two things you save and one thing you spend on the spot**:

| | Earned from | Spent on |
| --- | --- | --- |
| **XP** — grows your deck | Fights, objectives | Buying cards, card-tree upgrades, ability upgrades. Nothing else. |
| **Gold** — buys things | Enemies, treasure, ruins, dungeons, event sites | Gear, consumables (bandages), healers. A trainer converts gold to XP at a poor rate. Gold can never upgrade a card directly. |
| **Influence** — never saved | The cards you play at a site this turn (1 card = 1 Influence) | Site services: deck changes, mending, recruiting |

**Deck changes cost time and place.** Changing your deck (buy, sell, upgrade, swap) happens only at sites, and costs XP **plus** Influence by site tier. A player can shape their deck however they like, but not rebuild it for free before every fight.

| Site | Deck change | Mend wounds | Shop |
| --- | --- | --- | --- |
| Camp (any hex, at rest) | XP + 3 Influence per change | A bandage or 3 Influence = 1 roll (4+) per wound | — |
| Village | XP + 2 Influence | 2 Influence each | Consumables (gold) |
| Monastery | XP + 1 Influence; best card-tree upgrades | 1 Influence each, mid-round | — |
| Town / city | XP only: a straight 1:1 swap, no Influence | Healer (gold), grievous wounds | Gear (gold), trainer (gold → XP) |

These costs are a first pass. Mage Knight charges 3 Influence per wound at a village and 2 at a monastery. Ours are lower because our Influence is cards from a 4-card hand.

**Gold, first pass.** XP keeps its base-10 scale (a base card costs 10 XP). Gold uses its own numbers:

| Source | Gold | | Price | Gold |
| --- | ---: | --- | --- | ---: |
| Minion group | 5–15 | | Bandage | 10 |
| Lieutenant | 30 | | Consumable | 15–30 |
| Treasure / ruins | 20–40 | | Gear | 40–100 |
| Event site | 0–30 | | Healer (grievous wound) | 50 |
| Warlord | 60 | | Trainer | 3 gold → 1 XP |

At the trainer, 30 gold buys a base card's worth of XP (10). A minion fight pays 20–30 XP, so fighting stays the main way to grow your deck, and gold is a slow back door.

## 11. Growth and gear

- **XP** buys cards and upgrades, as today, at sites.
- **Gear** comes from ruins, grottos, shops and objectives. A piece of gear may:
  - **add a card** to your deck (more health and longer rounds);
  - **improve a card**;
  - **improve an ability or your defense**.
- **Relics** are run-scoped, as already decided for the run layer.
- **Hand size**, **base energy** and the **energy carry-out limit** are character stats that levels, gear and relics can raise.
### Cards for the map, for battle, or for both

![Card kinds](expedition/13-card-kinds.png)

Every card counts as health and can always be spent for **1 Move or 1 Influence**. On top of that there are three kinds:

| Kind | In battle | On the map | Examples |
| --- | --- | --- | --- |
| **Battle cards** (today's cards) | Their battle effect | 1 Move or 1 Influence | Strong Swing, Steady Guard |
| **Map cards** (new) | No battle effect — still health, but a dead draw in a fight | A map effect stronger than 1 Move | Trailblaze (Move 2), Scout Ahead (see hexes up to 3 away) |
| **Both** | Their battle effect | Their own map effect | Take Stock (draw 2), Second Wind (+10 energy) |

**Card trees grow in both directions.** Existing trees gain **map** and **both** branches where they make sense, and a few **new trees** are purely about the map (for example a Pathfinder tree: Trailblaze → Forced March / Lantern Road / Scout Ahead). When you upgrade a copy, you choose what it becomes: stronger in battle, useful on the map, or a bit of both. Upgrading at a site costs XP plus Influence as usual, so turning your deck toward exploring or fighting takes time and place. (New card names are placeholders.)

**Map cards are unplayable in battle by default.** That's a deliberate weakness: the more you build for exploring, the weaker your fights. Abilities, gear and card upgrades can later cover the gap (for example, a map card that also gains a small battle use). Naming the game's weaknesses on purpose is what creates build paths that feel good to complete.

**Map effects that cost energy are allowed, but rare and big.** In battle, energy comes back every round (Income, cards). On the map it comes back **only when you rest**, so the energy you have at the start of a round is all you get for the round. A map effect that costs energy must therefore be **big** enough to be worth part of that pool (e.g. Move 4 and ignore terrain costs, not Move 3). Most map effects cost no energy. Energy-costing map effects belong on specific card-tree upgrades, and they need careful tuning.

**Gear** can also be a card (strong on the map, weak in battle, or the reverse), and every card it adds is more health and a longer round.

## 12. Between regions

- **Carries over:** XP, cards, gear, relics, and wounds.
- **Between regions** there's a camp and healer stop: mend wounds; treat grievous wounds with gold.

## 13. Worked example

The Starter's real 12-card deck, Day 1.

![Example: turn 1](expedition/10-example-turn-1.png)

**Turn 1:** I need 3 Move (1 for plains, 2 to explore). I walk with Take Stock, Second Wind and Nudge, and keep Strong Swing, Steady Guard and Try Again for a fight. Exploring east reveals a Monastery, Ruins and a Bell Diver.

![Example: turn 2](expedition/11-example-turn-2.png)

**Turn 2:** I can challenge the Bell Diver with the hand I kept, spend two turns walking around it, or blunder past and be provoked. I fight. The battle starts from my piles; the damage takes **Second Wind and Nudge** — the cards I walked with. My draw pile runs out and I rest. Day 2 starts with 10 live cards.

## 14. Configurable values

**Every number in this doc is a first guess that will change in playtesting.** None of them belong in code. They live in content files the authority loads, in the same style as `content/progression_v1/campaign.yaml` and `economy.yaml`:

- **Expedition rules:** one file, e.g. `content/progression_v1/expedition.yaml` — rounds, hand size, terrain costs, sight, ambush, fortified, defeat, mending, the Warlord's pace, rewards, Influence costs and gold.
- **Per character:** base energy, hand size and limit, energy carry-out — on the character sheet.
- **Per enemy:** habit, gold drop, XP — on the enemy or encounter definition.
- **Per card:** its map effect (if any) — in the card's catalog definition, next to its battle `program`.
- **Per region:** tile stack, Lieutenants, the Warlord and his goal — in the region definition.

A sketch of the rules file (names are illustrative):

```yaml
schema_version: 1
rounds_per_region: 4          # alternating day/night, day first
hand: { size: 4, limit: 6 }   # shared by map and battle
move: { energy_per_move: 5, explore_cost: 2 }
terrain:
  plains: { day: 1, night: 1 }
  forest: { day: 2, night: 3, cover: true }
  hills:  { day: 2, night: 2 }
  marsh:  { day: 3, night: 3 }
sight: { day: 2, night: 1 }
ambush: { bonus_dice: 3, max_kept: 5 }
fortified: { extra_defense_dice: 1 }
defeat: { wounds: 2 }
camp_mending: { roll_needed: 4, inn_bonus: 1 }
warlord: { start_distance: 5, march_per_rest: 2, early_reward: { xp_per_round: 20, extra_relic_choices: 1 } }
influence_per_deck_change: { camp: 3, village: 2, monastery: 1, town: 0 }
gold:
  drops: { minion_group: [5, 15], lieutenant: 30, treasure: [20, 40], event: [0, 30], warlord: 60 }
  prices: { bandage: 10, consumable: [15, 30], gear: [40, 100], healer_grievous: 50 }
  trainer_gold_per_xp: 3
```

| Value | First guess |
| --- | --- |
| Rounds per region | 4 (Day, Night, Day, Night) |
| Hand size (map and battle) | 4 |
| Hand limit (map and battle) | 6 |
| Base energy (Starter) | 10 |
| Energy per Move | 5 |
| Explore cost | 2 Move |
| Terrain costs | See [A turn](#4-a-turn) |
| Sight (you and enemies) | 2 hexes by day, 1 at night |
| Ambush bonus dice | +3 on the first offensive roll (keep at most 5) |
| Fortified | +1 defensive die (alternative: shields prevent 2) |
| Defeat wounds | 2 |
| Camp mending | 1 roll per bandage (or 3 Influence), succeeds on 4+ |
| Warlord start distance / march | ~5 hexes from his goal / 2 hexes per rest |
| Early-strike reward | +20 XP per unused round, plus 1 extra relic choice |
| Gold drops and prices | See [Economy](#10-economy) |
| Influence per deck change | Camp 3, village 2, monastery 1, town 0 |
| Tiles per region | Start + ~5 countryside + 2 core |

## 15. What it would take to build

The full implementation brief — scope, code map, milestones, acceptance criteria and testing requirements — is in [expedition-implementation-handoff.md](expedition-implementation-handoff.md). In short:

- **Battle start from map piles.** `battle.ActorSetup` already has `Hand` and `Discard` fields, but `initializeSettled` always draws the opening hand on top; that draw needs skipping. Today's `battle.WithCarriedWounds` / `seat_wounds` is the model.
- **Starting energy override** (today it is always `starting_energy`), and returning piles and energy to the map afterwards.
- **Ambush status:** rolling more than 5 dice on the first roll is a new battle rule (small, but the first one).
- **Fortified status** for garrison defenders.
- **Mixed and multiple enemy groups.** The campaign session currently pins one enemy type with 1–2 copies; mixed groups need session (and possibly engine) work, tracked separately.
- **Map state** on the authority: tiles, fog, positions, the round clock, the Warlord's agenda — saved in the campaign save, never computed by the client.
- **Healing removed cards** outside battle (allowed by the design rules; engine validation still blocks it).

## 16. Open questions

1. **Tuning after the first playtest:** every value in [Configurable values](#14-configurable-values).
2. **Content:** the first map card tree(s), the map and "both" branches for existing trees, event scenes, gear, and the first region's tiles. A first set is defined in the [implementation handoff](expedition-implementation-handoff.md).

## 17. Decision log

Decided in review (2026-10-11 to 2026-10-12):

- Exploration follows Mage Knight; battles stay exactly as they are.
- A battle starts from the map piles as they are, with no top-up. Waiting a turn to draw is the player's choice.
- One hand size and limit for the map and battles; changes apply to both.
- No per-turn timer and no automatic Warlord growth: the clock is rounds, and each round is one deck.
- Keep unplayed cards between turns; each turn must play or discard at least 1 card; a fight ends the turn and counts.
- Energy: one pool per round; resets to the character's base at rest; battles start with current energy and return at most what came in; 5 energy = 1 Move; ordinary cards can't become energy on the map.
- Making camp early is allowed and advances the clock.
- Ambush gives extra dice on the first roll (visible status); night doesn't make ambushes automatic; a turn-1 stun is reserved for rare abilities.
- Fog of war: revealed terrain shown greyed out, contents unknown until seen.
- Day and Night rounds with different rules.
- Influence is cards played at a site; deck changes cost XP plus Influence, cheaper at better sites; gold buys gear and services and never upgrades cards directly.
- Losing a normal battle costs wounds and a round but never ends the run; only the Warlord does.
- Random tiles; Lieutenants roam or sit fortified; the Warlord has an agenda and can be caught.
- Mixed groups of enemies are in scope.
- Ambush needs an unseen approach: start your turn outside the target's sight (enemies see like you do), or attack from cover.
- Fortified strengthens the defenders' defense (default +1 defensive die).
- A normal defeat costs 2 wounds on top of your pre-battle wounds.
- Camp mending: one roll per wound per bandage.
- The Warlord marches toward his goal each rest; extra rests speed him up; catching him early pays per unused round; reaching his goal fortifies him there.
- Gold comes from enemies, treasure and event sites (Slay the Spire-style "?" scenes with choices). Gold and Influence costs start as a first pass, inspired by Mage Knight, and get tuned in playtesting. Every number is configuration, not code.
- Night: night hunters roam at night and can be ambushed by day; sleepers can be ambushed at night; grottos and ruins pay more at night with stronger guardians; towns close at night except the inn.
- The Warlord is like any enemy: his own habits and weaknesses, by day or night. He's Fortified only when camped in a tower or keep.
- Cards come in three kinds — battle, map, and both — and card trees grow toward battle, the map, or both, so a player configures each copy. Some new trees are purely about the map.
- Map cards are unplayable in battle by default — a deliberate weakness for gear, abilities and upgrades to cover later.
- Map effects may cost energy, but rarely and only for big effects, because map energy refills only at rest.
- XP, cards, gear and relics carry between regions, with a camp and healer stop in between.
