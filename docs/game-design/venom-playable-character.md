# Venom playable character

Implemented September 7, 2026 from the reviewed Venom Type Codex design.

## Starting a match

Run `./scripts/godot.sh` from the repository root. Choose Blade Warden or Venom, choose a trained Blade Warden opponent, choose a seat, and start. Rematch retains the chosen character and model.

The six existing opponent choices remain available: CP193, CP38, CP480, optimized v3, decision-quality v2, and original v1. Their model files and trained weights are unchanged. The compatibility layer preserves their original feature positions and treats explicitly registered new Venom identifiers as unknown categories. The models can complete matches against Venom, but have not been retrained to understand its new mechanics strategically.

## First playable loadout

Venom uses five D6: Fang on 1–3, Gland on 4–5, and Coil on 6. The starter deck contains one copy of each of the 24 reviewed cards. Under the existing cards-as-health rules, that gives Venom 24 starting health. This is a first playable balance baseline, not a balance certification.

| Ability | Implemented result |
| --- | --- |
| Needlefang | Choose a qualified tier: 3 Fang = 4 damage + 1 Poison; 4 Fang = 3 damage + 2 Poison; 5 Fang = 2 damage + 3 Poison. |
| Venom Gland | 2 Gland + 1 Coil: apply 1 Poison and gain 2 Catalyst. |
| Fever Spike | 2 Fang + 1 Gland + 1 Coil: 3 damage and Provoke 1. With 2 Gland: 4 damage and Provoke 2. |
| Terminal Bite | 3 Coil: gain 1 Catalyst, then deal 4 damage and Provoke 2. Checks resolve after the attack. |
| Shedskin | Free defense; roll 2 Venom dice. Each Fang prevents 1; each Gland gains 1 Catalyst; any Coil applies 1 Incubation to the attacker if absent, even without Poison. Optionally spend 1 Catalyst before rolling for 2 extra prevention. |
| Barbed Mantle | Pay 1 energy against an Offensive ability; roll 1 Venom die. Fang prevents 2 and applies 1 Poison. Gland prevents 3 and gains 1 Catalyst. Coil prevents 1 and applies Incubation if the attacker has Poison and no Incubation; otherwise it applies 1 Poison. |

All 24 cards have authoritative timing, costs, prerequisites, targets, and legal choices. The client displays those choices, including toxin selection, die changes, optional Catalyst payments, and Needlefang tiers. Venom Lens increases the displayed Needlefang damage as well as the actual attack.

## Status interactions

- Poison and Volatile Poison retain their existing independent caps of 3. Poison rolls 1–4 for 1 damage, 5–6 to clear. Volatile rolls 1–4 for 2 damage, 5 for no effect, and 6 to clear.
- A Venom Poison application that exceeds the normal Poison cap proposes 1 Incubation if none exists. Volatile Poison being full does not block this proposal. Filling Poison to exactly 3 without excess does not trigger overflow.
- Incubation has cap 1. After the next Ongoing Effects toxin checks and their damage, it converts one surviving Poison to Volatile Poison if there is room, then expires. Cleansing or replacing the original marker prevents that original conversion. Ordinary Incubation applications require Poison at commit; Shedskin explicitly permits seeding the marker before Poison exists.
- Catalyst has cap 3 and operates automatically after toxin rolls (after responses where a roll occurs outside Effects). Once per holder per batch, it spends one stack to reroll an eligible enemy Volatile 6 first, otherwise an enemy Poison 5–6. It never rerolls Volatile 5 or the holder’s own toxins. Each toxin die permits one physical reroll; the new result can still clear the stack.
- Provoke performs immediate normal toxin checks. Venom sources share a limit of two bonus checks per target per round. Cards reserve their checks when accepted; offensive abilities capture them after final revalidation and late cancellation. Ordinary Ongoing checks and Catalyst rerolls do not consume that allowance.
- Costs are paid when an action is accepted. A later failed conversion does not refund energy or Catalyst. Ordinary Poison applications outside Effects open response windows, and interrupted planning resumes without revealing uncommitted dice. Incubation applications and Poison-to-Volatile conversions resolve immediately in every segment, with inline status feedback. Effects resolves without participant commands.

## Content and verification

The character lives in the separate `dice-and-destiny-server/content/venom_v1` pack. The frozen `battle_v1` pack remains unchanged so existing model content hashes continue to validate.

Verified with the full Go test suite, native authority verification, the existing Blade Warden Godot startup/rematch checks, and the new Venom Godot full-game test. Automated integration tests complete Venom matches against all six models from both seats (12 games), checking for authority rejects, invalid actions, truncation, and model fallbacks. Focused tests cover every card’s executable choices and the new status interactions.

Relevant commands, from the repository root:

```sh
(cd dice-and-destiny-server && go test ./...)
./scripts/godot.sh --headless --script res://scripts/verify_battle_authority.gd
./scripts/godot.sh --headless --script res://tests/phase3/verify_venom_character.gd
./scripts/godot.sh --headless --script res://tests/phase3/verify_learned_battle.gd
./scripts/godot.sh --headless --script res://tests/phase3/verify_learned_battle_v2.gd
```

## Automatic Effects — September 15, 2026

Effects is now fully automatic in the authority, including rolling, Catalyst, damage, recovery, Bleed decay, and Incubation conversion. No card or manual dice action is legal during Effects. General dice cards retain their existing legal uses outside Effects; Agitate/Provoke outside Effects retain their response flow.

Bitter Reagent now costs 1 Energy and gains 1 Catalyst. Measured Dose costs 2 Energy and gains 2 Catalyst. Both are Offensive planning setup cards and require room under cap 3. Antidote moves its old Effects timing to Offensive planning and retains Defensive reactions. Coagulate, Emergency Molt, and Antivenom Draught no longer prevent damage during Effects; use their remaining Damage Resolution timing.

The client receives a public `effects_resolved` event with before/after profile values and frozen outcome events. One visual sequence shows original dice, Catalyst retries, actual card losses, recovered statuses, and conversion. It has no gameplay buttons and ignores Disable auto-pass; developer snapshot/history review can freeze playback. Nonempty damaging sequences last about 7 seconds, empty Effects about 1 second. New battles use the revised card definitions; saved battles retain their pinned content.

Six existing Blade Warden models remain unchanged. Their original training content version is retained, with an explicit allowlist for the reviewed Antidote timing change. The models were not retrained for the new timing.
