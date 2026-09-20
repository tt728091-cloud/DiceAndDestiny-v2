# Brine Mask

Select **Venom** and **Brine Mask · minion · keeps every 3** in the setup menu.
The existing `drowned_oracle/brine_mask.png` art renders at its 320-pixel minion height.

- **16 health cards:** 16 copies of Brine Surge. Normal damage removes cards using the existing health/deck rules. There are no removal combos or special damage reactions.
- **Brine Lash:** five six-sided dice; keep every 3 and reroll all other faces, with three total rolls normally. Each 3 deals 2 damage (2/4/6/8/10). Zero 3s is a miss. An all-3 result stops early because every die is already kept. Existing effects that change the roll budget remain authoritative.
- **Salt Veil:** automatically roll 1D6 and prevent half the result, rounded up: 1–2 block 1, 3–4 block 2, 5–6 block 3. The defense roll is shown in battle and uses the normal defense reaction window. No energy cost or ability choice. Poison and other ongoing damage use the ordinary rules.
- **Brine Surge:** 1 energy adds 1 damage to this round's attack. Automatically play at most one after finishing the rolls, only with a qualified attack and an affordable card in hand. It goes to discard; it does not remove health. The bonus expires before next round and produces no damage if the attack is invalidated.
- Start with one card in hand and zero energy. Normal income draws one card and grants one energy; hand limit three. Excess hand cards are automatically discarded at the normal hand-limit checkpoint.

The reusable `single_ability_policy` content configuration supplies a target face and optional automatic card. Its single attack and defense come from the ability board. The Go controller submits ordinary legal commands against the same authority as the player, including real keep/reroll commands; it never synthesizes successful dice. It passes optional reaction windows. No learned weights are loaded.

Content is in `dice-and-destiny-server/content/minions_v1`. The pack loads only for matchups needing it, preserving existing trained opponents' frozen content vocabulary. The existing local battle session/transport is shared with trained opponents; `controller_kind: single_ability` identifies this controller. Generic ability modifiers now support `duration: round` as well as `battle`.

Verification:

```sh
cd dice-and-destiny-server
go test ./...
cd ..
./scripts/godot.sh --headless --script res://scripts/verify_battle_authority.gd
./scripts/godot.sh --headless --script res://tests/presentation/verify_brine_mask.gd
```

The targeted Go tests exhaust all 7,776 five-die outcomes, complete 200 Venom games across both seats, check every attack tier, misses, card cost/expiry, last-target invalidation, and catalog isolation. The Godot test starts from the dropdown, plays a complete battle through the screen, checks rolled defense/art/symbols, rematches, and switches between trained and minion opponents. Optional graphical captures use `DICE_AND_DESTINY_MINION_SCREENSHOTS` pointing at a directory in this workspace.

## Two Brine Masks encounter

Select **Two Brine Masks · allied minion encounter** in the opponent menu.
This starts Venom (or Blade Warden) against two independent copies of the
same minion. Each keeps its own 16-card health deck, hand, energy, dice,
Brine Surge modifier, statuses, and defense roll. The masks share a team;
neither can target the other.

- Choose Mask 1 or Mask 2 before selecting an attack or enemy-targeted card.
- Both surviving masks commit an attack in the same round. A miss creates no
  incoming attack.
- In defense, assign a defense to every incoming attack before any rolls start.
  Energy and optional Catalyst costs are reserved for each choice. The same
  defense can be chosen once per source. **Pass All Remaining** explicitly
  skips all unassigned attacks and preserves choices already made.
- All chosen defense dice roll together. Queued results are retained while
  source-specific reaction windows and effect animations resolve in order. Both incoming panels stay visible,
  showing Needs defense, Queued, and each completed dice result. After each
  effect animation finishes, a sole **Continue** action advances automatically,
  including after the last defense. Playable reaction cards still pause normally.
- Defense effects apply to the selected source and its attacker. The two
  remaining damage amounts are then resolved together through the existing
  damage/removal and reaction rules.
- A defeated mask no longer rolls, receives income, or takes reaction turns.
  Combat continues against the survivor. Victory requires defeating both;
  losing the player ends the encounter even when both masks are alive.
  Simultaneous elimination of every team is a draw.
- Rematch restores both masks and preserves the chosen player character/seat.

All encounters use the original battle screen: the same HUD, hand, dice
controls, effects, and card-loss animations. During offense, clickable enemy
nameplates select the target of attacks and cards. After offense, incoming
attacks remain visible in stacked panels on the left, with Venom's outgoing
attack and its target's defense on the right. Each incoming panel owns defense
choice buttons bound to that attack, so no enemy-view switching is needed.
Damage panels show each source's independent amount and card losses; clicking
a panel targets reaction cards without hiding the others. Effects show both
enemies in named rows on one shared animation timeline, including all toxin
dice and removals. Defense playback advances once effects finish unless there
is a playable response or automatic progression is explicitly disabled.

The upper-right profile stays aligned with the player's profile, and both dice
rows share one baseline. Both masks appear together, with the right mask in
the foreground and the left mask behind it. The stacked central layout is sized
for the two-mask encounter; it does not introduce extra HUD columns.

Verification: `go test ./...`, the authority Godot script, and
`res://tests/presentation/verify_brine_pair.gd`. The pair simulation covers
60 complete seeded games across both player seats, deterministic replay,
separate defenses/skips, and continuation after a minion falls. Engine tests
cover team victory/draw, costs per defense, rejection of duplicate defenses
and friendly/dead targets, separate prevention, suspended inputs after defeat,
and Venom card/status targeting. The graphical test exercises menu selection,
a complete encounter, both defense rolls, rematch, and switching back to
single-enemy mode. `verify_enemy_selector.gd` checks original layout positions,
five-enemy nameplate spacing and mouse selection, kept-dice preservation, targeted cards/attacks, actual
defense-button submission, handled/missed attacks, and simultaneously visible defense, damage, and Effects panels with unique card losses.
