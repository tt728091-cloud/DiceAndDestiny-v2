# Full Battle 01: Blade Warden vs. Venom Goblin

> Status: Target-rules simulation. This is a coding template, not an executable current-engine fixture.

## Content

- Player: `../content-library/combatants/blade_warden.yaml`
- Enemy: `../content-library/combatants/venom_goblin.yaml`
- Shared content: `../content-library/`

## Scripted Randomness

All random results are scripted so the battle can later become a deterministic integration fixture.

The script controls:

- player choices required by the scenario
- enemy ability and Defensive selections, independently of future AI policy
- player combat dice
- player Basic Defense dice
- enemy Offensive D100 results
- enemy Basic Defense dice
- Poison dice
- random damage-card selections
- random draws and reshuffles
- enemy fallback ability choices

The enemy's scripted Defensive selections intentionally override its default
`defensive_selection` preference order. This keeps the fixture focused on rules
resolution; a separate AI-policy fixture can test how an enemy chooses among
multiple legal defenses.

## Initial State

| State | Blade Warden | Venom Goblin |
|---|---:|---:|
| Maximum/active health cards | 20 | 12 |
| Deck | 16 | 10 |
| Hand | 4 | 2 |
| Discard | 0 | 0 |
| Removed | 0 | 0 |
| Energy | 2 | 1 |
| Statuses | None | None |

Player opening hand:

```text
Sharpen Blade
Tip It
Emergency Ward
Antidote
```

Enemy opening hand:

```text
Emergency Ward
Battle Focus
```

## Round 1

### Ongoing Effects

Neither actor has a status trigger. The segment auto-completes.

### Income

```text
Player draws Battle Focus and gains 1 energy: energy 2 -> 3
Enemy draws Loaded Die and gains 1 energy: energy 1 -> 2
```

### Offensive

The player plays Sharpen Blade on Sword Cut:

```text
pay 1 energy: 3 -> 2
move Sharpen Blade: hand -> discard
add battle-duration exact-pair Bleed bonus to Sword Cut
```

Player roll sequence:

```text
Roll 1 (five dice):
  1 Sword, 4 Shield, 6 Gold Coin, 6 Gold Coin, 5 Shield
Keep:
  1 Sword, 4 Shield

Roll 2 (reroll the other three dice):
  1 Sword, 2 Sword, 5 Shield
Keep from this roll:
  1 Sword, 2 Sword

Roll 3 (reroll the remaining 5 Shield):
  3 Sword

Final dice:
  1 Sword, 1 Sword, 2 Sword, 3 Sword, 4 Shield
```

The player has exactly four Swords and an exact pair of face 1:

```text
select Sword Cut four-Sword tier
propose 6 damage
Sharpen Blade modifier proposes 1 Bleed
```

Enemy D100 result: `19`.

```text
three-roll chart -> Venom Strike on simulated roll two
reveal: 1 Sword, 2 Sword, 4 Shield, 5 Shield, 6 Gold Coin
propose 3 damage and 2 Poison
```

Both actors pass the Offensive reaction window. Commitments finalize.

### Defensive

The player selects Basic Defense against Venom Strike and rolls `2`:

```text
3 damage - 2 prevention = 1 final damage to Player
```

The enemy selects Basic Defense against Sword Cut and rolls `3`:

```text
6 damage - 3 prevention = 3 final damage to Enemy
```

Both pass the Basic Defense reaction window.

### Damage Resolution

The engine randomly selects the specific cards that are at risk, following the
deck -> discard -> hand zone priority. It reveals these proposed losses before
removing anything:

```text
Player proposed loss from deck:
  Loaded Die

Enemy proposed losses from deck:
  Tip It
  Emergency Ward
  Battle Focus
```

The player and enemy can see each card, its current zone, and the damage source
that selected it. The player passes the damage reaction window. The enemy then
passes. Only after both have passed does the batch commit.

Commit:

```text
Player moves Loaded Die: deck -> removed, and receives 2 Poison.
Enemy moves Tip It, Emergency Ward, and Battle Focus: deck -> removed,
and receives 1 Bleed.
```

End of Round 1:

| State | Blade Warden | Venom Goblin |
|---|---:|---:|
| Deck | 14 | 6 |
| Hand | 4 | 3 |
| Discard | 1 | 0 |
| Removed | 1 | 3 |
| Active health | 19 | 9 |
| Energy | 2 | 2 |
| Statuses | Poison 2 | Bleed 1 |

## Round 2

### Ongoing Effects

Poison rolls for the player:

```text
face 2 -> propose 1 damage
face 6 -> propose removing 1 Poison stack
```

During the Poison-roll reaction window, the player plays Antidote:

```text
pay 1 energy: 2 -> 1
move Antidote: hand -> discard
remove Poison completely
```

The two Poison rolls were already collected, so they still finish. Face 2 produces one damage; face 6's stack removal becomes a no-op because Antidote already removed Poison.

Bleed simultaneously proposes one damage to the enemy and removes its own final stack.

The child damage batch randomly selects and reveals the cards that would be lost:

```text
Player proposed loss from deck:
  Battle Focus

Enemy proposed loss from deck:
  Loaded Die
```

Neither card has been removed yet. The player passes, and then the enemy passes.
The child batch commits only after both passes:

```text
Player moves Battle Focus: deck -> removed.
Enemy moves Loaded Die: deck -> removed.
```

### Income

```text
Player draws Loaded Die and gains 1 energy: 1 -> 2
Enemy draws Battle Focus and gains 1 energy: 2 -> 3
```

### Offensive

Player roll sequence:

```text
Roll 1 (five dice):
  1 Sword, 4 Shield, 5 Shield, 6 Gold Coin, 2 Sword
Keep:
  1 Sword, 4 Shield, 5 Shield

Roll 2 (reroll the other two dice):
  1 Sword, 3 Sword
Keep from this roll:
  1 Sword

Roll 3 (reroll the remaining 3 Sword):
  1 Sword

Final dice:
  1 Sword, 1 Sword, 1 Sword, 4 Shield, 5 Shield
```

Sword Cut qualifies at exactly three Swords:

```text
propose 5 damage
three-of-a-kind bonus proposes 1 Bleed
Sharpen Blade exact-pair bonus does not trigger on a triple
```

Enemy D100 result: `52`.

```text
three-roll chart -> Greedy Blow on simulated roll one
reveal: 1 Sword, 4 Shield, 4 Shield, 6 Gold Coin, 6 Gold Coin
propose 7 damage
```

The player plays Tip It on one enemy face-6 die:

```text
pay 1 energy: 2 -> 1
move Tip It: hand -> discard
change one 6 Gold Coin -> 5 Shield
```

Modified enemy dice:

```text
1 Sword, 4 Shield, 4 Shield, 5 Shield, 6 Gold Coin
```

The engine rechecks every enemy ability. None are valid, so the enemy uses no Offensive ability. The next reaction round closes with all passes.

### Defensive

The player has no incoming source and auto-completes.

The enemy selects Protect against Sword Cut:

```text
pay 1 energy: 3 -> 2
5 damage scaled by 1/2, rounded down = 2 final damage
```

### Damage Resolution

The engine randomly selects and reveals two proposed enemy deck losses:

```text
Enemy proposed losses from deck:
  Tip It
  Battle Focus
```

The cards remain in the deck while the damage reaction window is open. The
enemy plays Emergency Ward:

```text
pay 1 energy: 2 -> 1
move Emergency Ward: hand -> discard
prevent 3 from the Sword Cut source
2 damage -> 0 damage
release both proposed card removals
```

The player passes after the response, and the enemy passes. Preventing damage
does not cancel Sword Cut's separate Bleed proposal. Commit applies one Bleed
to the enemy and removes no cards. The revealed Tip It and Battle Focus remain
in the enemy deck because their proposed removals were released.

End of Round 2:

| State | Blade Warden | Venom Goblin |
|---|---:|---:|
| Deck | 12 | 4 |
| Hand | 3 | 3 |
| Discard | 3 | 1 |
| Removed | 2 | 4 |
| Active health | 18 | 8 |
| Energy | 1 | 1 |
| Statuses | None | Bleed 1 |

## Round 3

### Ongoing Effects

Enemy Bleed proposes one damage and removes its final stack. The damage child
batch randomly selects and reveals one enemy deck card:

```text
Enemy proposed loss from deck:
  Emergency Ward
```

The card remains in the deck during the reaction window. The player passes, and
then the enemy passes. The child batch commits and moves Emergency Ward from the
enemy deck to `removed`.

### Income

```text
Player draws Emergency Ward and gains 1 energy: 1 -> 2
Enemy draws Tip It and gains 1 energy: 1 -> 2
```

### Offensive

Player roll sequence:

```text
Roll 1 (five dice):
  1 Sword, 3 Sword, 4 Shield, 5 Shield, 6 Gold Coin
Keep:
  1 Sword, 3 Sword

Roll 2 (reroll the other three dice):
  1 Sword, 2 Sword, 5 Shield
Keep from this roll:
  1 Sword, 2 Sword

Roll 3 (reroll the remaining 5 Shield):
  3 Sword

Final dice:
  1 Sword, 1 Sword, 2 Sword, 3 Sword, 3 Sword
```

```text
exactly 5 Swords -> Sword Cut proposes 7 damage
no three-of-a-kind -> base Bleed bonus does not trigger
exact pair exists -> Sharpen Blade modifier proposes 1 Bleed
```

Enemy D100 result: `33`.

```text
three-roll chart -> Crushing Advance on simulated roll one
reveal: 1 Sword, 2 Sword, 4 Shield, 5 Shield, 5 Shield
propose 5 damage
```

Both actors pass the Offensive reaction window.

### Defensive

Player Basic Defense roll: `3`.

```text
Crushing Advance: 5 - 3 = 2 final damage to Player
```

Enemy Basic Defense roll: `2`.

```text
Sword Cut: 7 - 2 = 5 final damage to Enemy
```

Both actors pass the defense-roll reaction window.

### Damage Resolution

The engine randomly selects and reveals every proposed removal. No card changes
zones yet:

```text
Player proposed losses from deck:
  Tip It
  Battle Focus

Enemy proposed losses from deck:
  Battle Focus
  Battle Focus

Enemy proposed loss from discard:
  Emergency Ward

Enemy proposed losses from hand:
  Loaded Die
  Battle Focus
```

The player passes after reviewing all seven cards. The enemy then passes. The
batch commits and moves each named card from its displayed zone to `removed`.
The same batch also applies one Bleed to the enemy.

End of Round 3:

| State | Blade Warden | Venom Goblin |
|---|---:|---:|
| Deck | 9 | 0 |
| Hand | 4 | 2 |
| Discard | 3 | 0 |
| Removed | 4 | 10 |
| Active health | 16 | 2 |
| Energy | 2 | 2 |
| Statuses | None | Bleed 1 |

## Round 4

### Ongoing Effects

Bleed proposes one damage to the enemy and removes its final stack. The enemy
has no deck or discard cards, so the engine randomly selects and reveals a card
from its hand:

```text
Enemy proposed loss from hand:
  Tip It
```

Tip It remains visible in the enemy hand during the reaction window. The player
passes, and then the enemy passes. The child batch commits and moves Tip It from
the enemy hand to `removed`.

Enemy active health becomes `1`.

### Income

```text
Player draws Battle Focus and gains 1 energy: 2 -> 3
Enemy cannot draw because deck and discard are empty; gains 1 energy: 2 -> 3
```

### Offensive

Player roll sequence:

```text
Roll 1 (five dice):
  1 Sword, 4 Shield, 5 Shield, 6 Gold Coin, 6 Gold Coin
Keep:
  1 Sword, 4 Shield

Roll 2 (reroll the other three dice):
  1 Sword, 2 Sword, 5 Shield
Keep from this roll:
  1 Sword, 2 Sword

Roll 3 (reroll the remaining 5 Shield):
  3 Sword

Final dice:
  1 Sword, 1 Sword, 2 Sword, 3 Sword, 4 Shield
```

```text
exactly 4 Swords -> Sword Cut proposes 6 damage
exact pair -> Sharpen Blade modifier proposes 1 Bleed
```

Enemy D100 result: `90`.

```text
three-roll chart no-ability range -> enemy passes
```

Both pass the Offensive reaction window.

### Defensive

The player has no incoming source and auto-completes.

The enemy selects Basic Defense and rolls `1`:

```text
Sword Cut: 6 - 1 = 5 final damage
```

### Damage Resolution

The enemy has one remaining hand card. The engine reveals exactly which card is
at risk before opening the reaction window:

```text
final damage: 5
proposed loss from enemy hand: Battle Focus
overage: 4
```

Battle Focus remains in the enemy hand while reactions are possible. The enemy
has no legal response and passes. The player passes. The batch then moves Battle
Focus from the enemy hand to `removed`; overage has no additional default
effect. The enemy becomes pending defeat.

Sword Cut's Bleed proposal also commits, but scheduled Bleed activation never occurs because the battle ends during this segment's exit.

Damage Resolution finishes all work, marks the Venom Goblin defeated, and performs the final battle-completion check.

## Result

```text
Battle result: Victory
Completed rounds: 4
Blade Warden remaining active health: 16
Venom Goblin remaining active health: 0
```

Final player state:

| State | Blade Warden |
|---|---:|
| Deck | 8 |
| Hand | 5 |
| Discard | 3 |
| Removed | 4 |
| Active health | 16 |
| Energy | 3 |
| Statuses | None |

Sharpen Blade's ability modifier is battle-duration only and clears after victory. The player's card zones, missing health, energy, and any authorized run-level results are eligible to update current run player state.

## Contracts Exercised

- one-card/one-health across every zone
- Income and hand growth
- battle-duration ability upgrades
- Offensive exact symbol tiers and independent number-pattern bonuses
- enemy D100 planning
- enemy ability invalidation after die modification
- selection-first Defensive abilities
- ability-owned Defensive dice
- source-specific prevention
- damage reaction cards
- status application by abilities
- automatic Poison and Bleed triggers
- status removal by a card
- fixed trigger batches despite mid-window status removal
- deck -> discard -> hand damage priority
- overage presentation
- pending defeat and segment-exit battle completion
- cleanup of battle-only ability modifiers
