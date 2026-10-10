# Campaign progression

The campaign is the battle → XP → deck change → next battle loop. Open it from the main menu with **Campaign**.

## Flow

1. The **Campaign** screen shows the character's health (equipped cards), XP to spend, run number and victories. It also shows the encounter path, with each encounter marked Cleared, Next or Ahead.
2. **Fight · <encounter>** starts the next encounter. It uses the character's Progression deck and ability board, Seat A, and the unified defense rule.
3. When the battle ends, the authority records the result in the character's progression ledger:
   - **Victory:** adds the encounter's XP and advances to the next encounter. After the last encounter, the run counter increases and the next run starts again at the first encounter. XP and cards carry over.
   - **Defeat or draw:** earns nothing; the same encounter is offered again.
4. The result screen shows the reward. **Continue · Spend XP & Next Battle** returns to the Campaign screen.
5. **Deck & Card Trees** opens Character Creation, locked to Progression. There you can:
   - buy base cards;
   - climb card trees;
   - equip, store or sell copies (selling refunds the card's value).

   Equipping and storing are free and can be repeated as often as you like. The mode cannot be switched to Sandbox from the campaign, because Sandbox's free deck edits would bypass XP.

The client never names a reward amount. The battle session knows it is a campaign battle and which encounter it is. When the battle becomes terminal, it calls `loadout.RecordCampaignBattle`, which commits the XP and the campaign position in one atomic ledger write. Each battle ID is recorded once. A battle whose encounter is no longer the character's next encounter earns nothing.

## Configuration

`dice-and-destiny-server/content/progression_v1/campaign.yaml`:

| Field | Meaning |
| --- | --- |
| `characters` | Characters that may enter. Each must have the General type, so campaign decks only use General cards and trees. The first is the default. |
| `encounters[].id` / `name` / `description` | Stable ID and the text shown on the path. |
| `encounters[].opponent` | A scripted minion combatant (one with `single_ability_policy`), such as `drowned_oracle_brine_mask`. |
| `encounters[].opponent_count` | 1 or 2 copies of that minion. |
| `encounters[].xp` | XP a victory awards (base-10 scale; a base card costs 10). |

All three encounters currently use Brine Mask: Tide Pool (20 XP), Sunken Steps (25 XP) and Drowned Shrine (30 XP). To use new enemies, change each `opponent`. A new playable character (for example, Starter) also needs:

- an entry in `characters`;
- the playable-character lists in `learned.CharacterCatalogs` and `Session.resetLoadout`.

## Ledger

The campaign uses the per-character progression ledger shared with Progression mode (`<loadout root>/progression/<character>.json`):

- `campaign`: next encounter, victories, defeats, runs completed, and recent recorded battle IDs.
- `earned_xp`: total battle rewards. An admin budget override is a total fixed at the time it is saved. Rewards earned later are added to it, just like free deck edits.

The `starting_xp` allowance in `economy.yaml` is still the new-character XP.

## Tests

- `internal/battle/learned/campaign_test.go` plays real campaign battles. It covers:
  - victory XP and advancing, including run wrap-around;
  - defeat or draw earning nothing;
  - out-of-order and non-General characters being refused;
  - duplicate recording being refused;
  - spending XP between battles changing the next battle's deck;
  - rewards surviving admin budget overrides.
- `tests/presentation/verify_campaign.gd` drives the loop with the pointer: menu → Campaign → battle → reward screen → Continue → Deck & Card Trees purchase → the next encounter uses the new deck.
