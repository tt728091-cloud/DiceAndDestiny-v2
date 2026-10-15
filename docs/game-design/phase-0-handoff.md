# Phase 0 handoff: quick fixes before the open world

**Status:** ready to implement (written 2026-10-14).
**Plan of record:** [`open-world-plan.html`](open-world-plan.html), the agreed open-world design. Phase 0 is the first step of its roadmap.
**Scope:** four small, low-risk fixes to today's game. Nothing in Phase 1 or later.

---

## Prompt for the implementing model

> You are working in the Dice and Destiny repository. Read `AGENTS.md` first and follow it exactly, especially: run Godot only through `./scripts/godot.sh`, keep the Go server the authority, and keep every number in configuration.
>
> Then read `docs/game-design/phase-0-handoff.md` and implement all of Phase 0: tasks 0.1 to 0.4, with the tests each task lists. Work on a new branch named `phase-0-quick-fixes` created from `main`. Commit each task separately, with a clear message. Do not push or merge unless the user asks.
>
> Before changing code, confirm each "where it lives" pointer in the handoff against the current code; line numbers may have moved. If something in the handoff contradicts the code or `AGENTS.md`, stop and report it rather than guessing.
>
> When you finish, run the full verification list in the handoff and write the final report it describes: what changed, the test commands and their results, and any finding you did not fix. Reply with that report.

---

## Background (read this first)

The open-world plan sets some hard rules. Phase 0 brings today's game in line with them before larger work starts:

- **XP is the only driver of character power.** Spending XP is always 100% refundable; nothing anywhere loses XP.
- **Gold is for convenience and recovery.** Gold never buys XP or power.
- **Wounded cards are frozen.** A wounded copy can't be sold, upgraded, downgraded, stored or traded until it's healed, so selling can never shed a wound.
- **The Rising** (a later phase) starts at a total-XP threshold, so the server must keep a reliable total of all XP the hero has.

### How XP works today (important for tasks 0.2 and 0.3)

A character sheet is `loadout.Progress` (`dice-and-destiny-server/internal/battle/loadout/progression.go`). It behaves like a budget:

```
Budget (total_budget) = XP (unspent) + DeckValue + CollectionValue + WoundValue + UpgradeSpent
```

- `settlePurchase` charges or refunds a transaction and recomputes the values. `UpgradeSpent` is whatever is left over.
- `creditXP` (`loadout/expedition_save.go`) adds battle and map rewards to both `XP`, `EarnedXP` and `Budget`.
- Campaign saves keep their own sheet (`loadout/campaign_save.go`). Reading one calls `reconcileBudget`, which keeps the sheet's total when cards are repriced.
- Card-tree cards must have equal buy and sell values (`internal/content/card_tree.go`, "tree cards need equal buy/sell values"), so tree trades are already lossless.
- Campaign and expedition deck changes set `TreeCardsOnly`, so legacy non-tree cards can be sold there but not added.

---

## Task 0.1: Remove the gold→XP trainer

**Why:** the town trainer converts 3 gold into 1 XP, which lets gold buy power. The rule is now that gold never buys XP.

**Where it lives** (verify before editing):

| Place | What's there |
| --- | --- |
| `dice-and-destiny-server/content/progression_v1/expedition.yaml` | `gold.trainer_gold_per_xp: 3` |
| `dice-and-destiny-server/internal/battle/expedition/config.go` | `TrainerGoldPerXP` field on the gold config and its range check (`"gold.trainer_gold_per_xp"`) |
| `dice-and-destiny-server/internal/battle/expedition/actions.go` | `case "trainer":` in the site services; `Action.Gold` (commented `// trainer`, used only there) |
| `dice-and-destiny-server/internal/battle/expedition/view.go` | the town's `"trainer"` `ViewService`, and `TrainerGoldPerXP` in the view config struct |
| `dice-and-destiny-client/app/screens/map/expedition_screen.gd` | `if service.id == "trainer": action["gold"] = ...` in the site services list |
| `dice-and-destiny-server/content/expedition_v1/sites/greywharf.yaml` | the description mentions "a trainer (3 gold per XP)" |
| `dice-and-destiny-server/internal/battle/expedition/config_test.go` | asserts `TrainerGoldPerXP == 3` |
| `dice-and-destiny-server/internal/battle/expedition/rules_test.go` | `TestSitesInfluenceMendingShopsAndTrainer` exercises the trainer |

Ignore matches for "trainer" under `docs/machine-learning-battle-simulation/`. That's the ML training pipeline, unrelated.

**Do:**

1. Remove the config key, the config field and its validation. The loader is strict, so a config that still contains `trainer_gold_per_xp` must now fail to load. That is correct.
2. Remove the trainer service from the actions and the view. A `use_site` action with `service: "trainer"` must be refused with a clear error (the same path any unknown service takes).
3. Remove `Action.Gold` if nothing else uses it. A stale client that still sends `gold` must not break anything: the default JSON decoding ignores unknown fields, so confirm that rather than assume it.
4. Remove the client's trainer special case. Check that no Godot test or tooltip refers to the trainer.
5. Update Greywharf's description (it keeps its healer, its shop and XP-only deck changes).
6. **Docs:**
   - `docs/game-design/expedition.md`: remove the trainer from §10 (the economy tables and the "At the trainer…" paragraph) and from the §14 config sketch. Add a short dated note in §17's decision log: "Gold never buys XP; the trainer was removed (2026-10, open-world plan)."
   - `docs/game-design/expedition-implementation-handoff.md` is a historical brief. Don't rewrite it; add a one-line dated note where it mentions the trainer, pointing to this change.
   - `AGENTS.md`, Expedition section: where it says "gold comes only from the authority", add "and never buys XP".
   - `TODO.md`: tick the "Remove the gold→XP trainer" item.

**Tests:**
- Rename `TestSitesInfluenceMendingShopsAndTrainer` to `TestSitesInfluenceMendingAndShops`. Keep its other checks. Assert that a town's services no longer list a trainer and that `use_site` with `trainer` is refused, with gold unchanged.
- `config_test.go`: drop the trainer assertion. Add a case showing that a config containing `gold.trainer_gold_per_xp` is refused by the strict loader.
- The shipped content must still load: `go test ./internal/battle/expedition/`.

---

## Task 0.2: Make sure every XP spend is fully refundable

**Why:** the rule is that selling or undoing anything returns exactly the XP it cost. Card-tree trades already do. Audit the rest and close any gap a **campaign or expedition save** can reach.

**Transaction kinds** (`applyPurchase` in `progression.go`): `buy_card`, `buy_collection_card`, `sell_card`, `sell_collection_card`, `equip_collection_card`, `unequip_collection_card`, `tree_card`, `tree_card_deck`, `upgrade_card`, `upgrade_ability`, `downgrade_ability`.

**Known spots to check:**
- `sell_card` refunds `e.Value(...)`, but for an authored non-tree card it uses `c.Sell`, which may be lower than its buy price.
- `upgrade_card` (legacy, non-tree) costs `max(u.XP, price difference)` and has no downgrade path. If `u.XP` is larger than the price difference, the extra goes into `UpgradeSpent` and can never come back.
- `downgrade_ability` refunds `upgrade.XP` but requires `UpgradeSpent >= cost`. Check that an upgrade-then-downgrade round trip always returns the same XP.
- Store and equip moves between deck and collection must not change the total.

**Do:**
1. Work out which of these paths a campaign or expedition save can actually reach. Both set `TreeCardsOnly`, which blocks adding non-tree cards, but selling one is allowed.
2. **Fix any lossy path a campaign or expedition save can reach.** Make it lossless, or refuse it with a clear message if a lossless version isn't possible. Prefer making it lossless.
3. **Don't change the admin editors' behaviour** (the non-campaign progression saves edited in Character Creation). If you find lossy paths there, list them in your final report for the user to decide.

**Tests:** add a round-trip test, table-driven over the shipped content, for campaign and expedition sheets:
- for every card tree, buy, upgrade along each connection, then trade down and sell back;
- for every ability with an upgrade path, upgrade, then downgrade;
- after each round trip, `XP`, `Budget` and `UpgradeSpent` are exactly what they were before;
- for the expedition, use `ExpeditionDeckChange`. The Influence fee is paid in cards, not XP, so it must not affect the XP comparison.

---

## Task 0.3: A reliable total-XP figure

**Why:** later phases need the hero's total XP: the omen meter, the Rising's trigger, and the fairness check. The data mostly exists already (`Budget`, `EarnedXP`, `creditXP`); Phase 0 makes it explicit, protected and visible.

**Do:**
1. **Define the canonical figure:** total XP = `Budget` on a campaign or expedition sheet, meaning everything the hero has, spent and unspent. Document it in a short comment where `Budget` is declared, and in `docs/game-design/campaign.md`.
2. **Protect it:** a campaign or expedition write must never lower `Budget`. Add a check where those saves are written (for example `finishExpeditionWrite` and the linear campaign's write path) that refuses a write that would lower it.
   - The one legitimate exception is the repricing `reconcileBudget` already performs on read. It keeps the total by design, so confirm that and leave it alone.
3. **Expose it:** add `xp_total` (the canonical figure) to the expedition status view built in `internal/battle/learned/campaign.go` (`expeditionView`, which already sends `xp` and `earned_xp`). Do the same in the linear campaign status if it reports XP.
4. **Show it:** on the expedition map's XP tile (`expedition_screen.gd`, the `_stat_tile` for XP), add a tooltip such as "XP to spend: N · Total XP: M". Use the server's figures; don't compute it in the client.

**Tests:**
- A Go test: credit XP, spend some, sell some, take wounds, reload the save. `Budget` never drops, and `xp_total` equals it in the view.
- A Go test: a write that would lower `Budget` is refused.
- Extend `verify_expedition_map.gd` (headless) to read the XP tile's tooltip and check it contains both numbers from the view.

---

## Task 0.4: Wounded cards are frozen everywhere

**Why:** a wounded copy must not be sold, upgraded, downgraded, stored or traded until it's healed. Otherwise selling cards could shed wounds.

**What exists:**
- Wounded copies live in `Progress.Wounds`, separate from `Progress.Deck`, and `applyPurchase` only edits the deck and the collection.
- `ExpeditionDeckChange` also refuses any purchase that changes the wound list ("wounded copies cannot be traded until mended").
- The linear campaign's `BuyCampaign` (`campaign_save.go`) has no such explicit guard.

**Do:**
1. Add the same explicit guard to `BuyCampaign`: refuse any transaction that would change `Wounds`.
2. Check every transaction kind for a path that could reach a wounded copy indirectly. For example, a copy count or a tree tag that matches both a live copy and a wounded one, or a `tree_card_deck` batch `Count` larger than the live copies.
3. Make sure the Prepare screens show wounded copies as frozen. Today's campaign rule (AGENTS.md: "Wounded copies can't be played, traded or sold") should already cover this; confirm the controls are disabled with a reason and not merely refused by the server.

**Tests:** for both the linear campaign and the expedition, with a deck that holds wounded and live copies of the same card:
- every kind (sell, upgrade, trade down, store, batch trade) succeeds against the live copies and leaves the wounds untouched;
- when only wounded copies of a card remain, every kind is refused, and XP and `Budget` don't change;
- a save and reload keeps the wounds identical.

---

## Verification

Run all of these from the repository root, following `AGENTS.md`:

```bash
cd dice-and-destiny-server && go test ./...
cd .. && ./scripts/godot.sh --headless --script res://scripts/verify_battle_authority.gd
cd dice-and-destiny-server && go test ./internal/battle/expedition/ ./internal/battle/learned/
cd .. && ./scripts/godot.sh --headless --script res://tests/presentation/verify_expedition_map.gd
./scripts/godot.sh --headless --script res://tests/presentation/verify_expedition_statuses.gd
```

Notes:
- Use headless runs for verdicts. Windowed Godot tests are sensitive to the real mouse moving over the window.
- `tests/phase3/verify_offensive_roll_controls.gd` was already failing before this work (see `TODO.md`). Don't count it as a regression, but mention it.
- Seeds used by the region tests are fixed. If a change alters a seeded outcome, find a new seed with the env-gated probes instead of weakening an assertion (AGENTS.md).

**Live check:** start the game with the inspector (`DICE_AND_DESTINY_INSPECTOR=1 ./scripts/godot.sh`):
- open or start an expedition, reach Greywharf (or use a test save), and confirm the town lists no trainer;
- confirm the XP tile's tooltip shows both figures;
- try to sell a wounded copy in Prepare and confirm it's disabled with a reason.

Save screenshots in the ignored evidence folder `dice-and-destiny-client/.godot/runtime/expedition-evidence/`.

---

## Acceptance criteria

1. No config, server rule, view, client control or document offers converting gold into XP. A `trainer` service request is refused, and a config containing `trainer_gold_per_xp` fails to load.
2. In campaign and expedition saves, every transaction that spends XP can be undone for exactly the XP it cost. A round-trip test over the shipped content proves it.
3. Any lossy path found in the admin editors is listed in the final report, not silently changed.
4. Total XP (`Budget`) never decreases across campaign or expedition writes, is sent to the client as `xp_total`, and shows in the map's XP tooltip.
5. Wounded copies can't be sold, upgraded, downgraded, stored or traded in either campaign mode. Live copies of the same card still can. The Prepare screen shows the frozen state.
6. Every command in the verification list passes, apart from the known pre-existing failure.
7. The docs (`expedition.md`, `campaign.md`, `AGENTS.md`, `TODO.md`) match the new behaviour.

## Out of scope

- Anything from Phase 1 or later: the calendar, quests, gear, potions, reputation, new screens.
- Gold prices, Influence fees or other balance numbers.
- Changing the admin editors' economy (report only).
- Pushing or merging the branch.

## Final report

Reply with:
1. a summary of each task (0.1–0.4) and its commit;
2. the test commands you ran, with pass/fail results;
3. the findings from the refund audit, including any admin-editor paths left for the user to decide;
4. anything you found that contradicts this handoff or `AGENTS.md`, and how you handled it;
5. the paths to the live-check screenshots.
