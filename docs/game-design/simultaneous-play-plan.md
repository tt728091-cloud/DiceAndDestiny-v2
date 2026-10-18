# Simultaneous play: secret plays, lock-in and reveal

Status: **planned, not started** (written 2026-10-18 from a design review with the user). Read this whole page before changing code. Confirm the open questions in §8 with the user first.

## 1. Why

Everything in a round is meant to be **simultaneous**. The game is single player today: one human against AI enemies. Later it may be played by several people on a local network, and later still as PvP on a cloud server. The order of plays, lock-ins, reveals and rolls therefore has to be right in the authority now, for any number of human and AI participants.

The user's main goal is to **avoid interruptions and back-and-forth**. Nobody reacts to a play the moment it happens. Plays made before the roll stay secret, everyone locks in, everything is revealed at once, and then the dice roll.

## 2. The rules the user set

1. **Before the roll, everyone plays in secret.**
   - In Offense and in Defense, every participant may play cards until they lock in. This includes cards that affect other participants (for example, Dispel on an enemy's Fortified).
   - Other participants don't see these plays until the reveal: not the card, its target, the energy spent, changes in hand size, or its effects.
   - Example from the user: I play Dispel on the enemy and the enemy plays Dispel on me. Neither of us knows. When we both lock in for our defense rolls, both Dispels resolve at the same time, then the rolls start.
2. **No unlocking.** A card never undoes another participant's lock-in or makes them re-choose. Everything played before the roll is revealed afterwards. That includes +damage, status removal and the like.
3. **Lock-in, reveal, then roll.**
   - When every participant has locked in, the secret plays are revealed and resolved together. Then everyone's rolls happen together.
   - Offense: after the reveal, everyone rolls and rerolls at the same time, then locks in an ability. The existing post-offensive reveal (the reaction moment for enemy dice) follows when everyone has locked in.
   - Defense: every defense roll, against every incoming attack, is done before any "after defense" card can be played.
4. **After your first roll, only self-affecting plays.**
   - Once the rolls have started, participants may keep rolling and play cards that affect only themselves (their own dice, draws, energy, their own statuses).
   - They can't change anyone else, including another participant's dice and statuses. Enemy dice can change only in the post-offensive reveal reaction, which works like that today.
5. **"Any time" = Before + After together, never a third "middle" timing.** An any-time card is legal wherever a Before card or an After card would be, under that moment's rules.
   - So an any-time card that targets an enemy is playable in the Before part, secretly, and not after rolls have started.
   - See §8 Q1 for what "After" means in Offense.
6. **"After defense" opens only when all defense rolls are final.** If several enemies attack me, all my defenses against all of them must resolve, along with everyone else's, before any After-defense card is playable.
7. **Defense rolls are simultaneous for every participant.** Today the Defense hub opens for one human at a time; that's wrong.
8. **Map conditions come back each round.** Fortified, Conqueror, Lieutenant's Favor and the Lieutenant marker are `restore_each_round` statuses. They return to their battle-start stacks at every round's Ongoing Effects. Already built: see "Dependencies".
   - Fortified can only be beaten by removing it **before** the defense rolls. It's gone for that round's defense rolls, and Ongoing Effects puts it back next round.
   - This is intended. It's why the Dispel tree's "after your defense roll" column (Dispel, Strip, Shatter, Nullify) is the weaker one. **Don't change the Dispel or Salve trees.**
9. **Keep the `dispel_immune` status flag** (unused today, kept for the future). No status should be immune today.

## 3. Target sequence

### Offense

1. **Offensive entry**: entry triggers resolve (Entangle). Each participant's public snapshot is taken.
2. **Before**: every participant (all humans at once; AI by policy) plays cards in secret. Effects on themselves happen at once but are hidden from others (§8 Q3). Effects on others are queued.
3. **Lock-in**: a participant locks in by committing their first roll (or passing without rolling). They can't play Before cards after that.
4. **Reveal 1**, once everyone is locked in: the secret plays are revealed. Queued effects on others resolve in a fixed order (§8 Q4). Then everyone's first roll happens.
5. **Rolling**: everyone keeps, rerolls and plays self-only cards at the same time (own-dice cards such as Nudge and Try Again; draw and energy). Each participant then locks in an ability and targets.
6. **Reveal 2**: the existing post-offensive reveal, then the opt-in reaction moment (enemy-dice cards such as Disrupt). The attacks are finalized (Lieutenant's Favor adds its damage here).

### Defense

1. **Defense entry**: attacks, their reserved cards and their pending statuses are shown, as today.
2. **Before**: every participant at once plays cards in secret (Dispel on an enemy, Salve on yourself, prevention on an incoming attack) and **chooses their defenses** (§8 Q2).
3. **Lock-in**: a participant locks in by committing their defense rolls (or none). Their defense energy is spent then.
4. **Reveal**, once everyone is locked in: the secret plays are revealed and queued effects resolve. Then **all** chosen defense rolls happen together, each keeping its roll review and opt-in defense reaction moment (Second Guard). For example, a revealed Dispel removes Fortified, so the enemy's defense rolls without the extra die. Its choice stands; nobody re-chooses.
5. **Defended statuses land.** Each attack's queued statuses apply when the defense against it resolves, as today, but always before the After step opens.
6. **After**, once every participant's defense rolls are final: every participant at once plays After-defense cards (Salve on a status that just landed, before it ticks at Ongoing Effects). It ends when everyone has passed. §8 Q5 asks whether these plays are secret too.
7. **Commit**: remaining damage and the statuses of undefended attacks commit once, as today. Next comes Ongoing Effects (map conditions return here).

## 4. What the code does today

These are the findings of the review, with code references.

- **Offense: AI enemies decide everything at entry.**
  - `progressSettledOffensive` → `planSettledAI` (`internal/battle/engine/settled_engine.go`) picks the AI's ability and its **final dice** (from reveal profiles) at Offensive entry, and sets `PlanningCommitted`.
  - Under rule 2 (no unlocking), that's acceptable as long as queued effects on the AI resolve at Reveal 1, before its attack is finalized.
- **Offense: human planning is already simultaneous.**
  - The planning window opens for every external actor at once (`openSettledWindowForActors(... external ...)`).
  - Part of the privacy exists already: `SettledPlanningPublic` and `refreshPlanningPublicCounts` hide dice, selections and ability modifiers from other viewers during planning (`snapshot.go`, `SettledPlanningPrivate`).
  - But energy, hand, deck and discard counts are refreshed **live** after each card play, and a card's effects on others, such as status removal, apply **at once and are visible**. No plays are queued.
- **Offense: any-time cards can affect enemies mid-rolling.** In `card_program.go`, `programWindowOpen` treats `offensive_planning` (any time) as open for the whole of planning.
  - Enemy **dice** are already held back until the reaction stage (`offensive_die` targets need `stageOffensiveReact` when the die isn't yours).
  - Enemy **statuses** are not held back. Unravel and the other any-time Dispels can remove an enemy buff after you've rolled. That breaks rule 4.
- **Defense: the hub is one human at a time.**
  - `unifiedDefenseHub` (`unified_defense.go`) opens the window for the **first** external actor that hasn't passed, and returns.
  - Other humans wait until that player passes.
- **Defense: the AI chooses and rolls when the first human commits a defense.**
  - `handleDefenseSelectionCommand` → `afterDefenseSelections` → `selectAIDefenses` makes every AI choose (at random among legal and affordable defenses) at that moment.
  - `resolveDefenseRollsAndOpenReaction` then rolls everyone's selections together. If the human passes without rolling, the AI chooses and rolls then.
  - So a card played **before** your first defense roll already beats Fortified today: an any-time Dispel such as Unravel removes it before the AI rolls. One played **after** your roll is too late.
- **Defense: After cards open between your own defenses.**
  - `programWindowOpen` opens `defense_after_roll` as soon as `defenseRolled` finds **one** finished defense of yours.
  - With two incoming attacks you defend one, may play After cards, then defend the second. That breaks rule 6.
  - The AI's defenses also roll only alongside the human's first defense; there's no shared "everyone rolls everything" step.
- **Before-defense plays aren't secret either.** They resolve at once and are visible, and AI defense choices are made after they resolve. For a fair PvP version, AI and human choices must use only public information, and other participants' secret plays must stay hidden.
- **Unified defense is a pinned battle rule** (`Settled.UnifiedDefense`, `docs/game-design/unified-defense.md`). Legacy saves and replays keep their original flow.

## 5. Work to do

1. **A new pinned battle rule**, for example `simultaneous_play`, alongside `unified_defense`. New battles from the menu, campaigns and expeditions get it. Old saves, replays and recorded fixtures keep their flow (see how `unified_defense` is pinned and passed through native startup and ML simulation/replay).
2. **Queued, hidden plays.**
   - Record each Before play, with its choices made at play time, in authority state.
   - Apply self-only effects at once if §8 Q3 says so, otherwise queue them.
   - Queue every effect on another participant.
   - Hide all of it from other viewers until the reveal: card identity, targets, energy, pile counts and status changes. Extend the `PlanningPublic` approach to statuses and to Defense, and filter the per-viewer events.
   - Card plays must still be visible statuses where AGENTS.md requires it.
3. **Lock-in state per participant** for each step (Offense Before, Offense Rolling, Defense Before, Defense After), with reveal points that fire only when every participant is locked in or has passed. Never wait on one particular participant except at these points.
4. **The reveal resolver**: resolve the queued effects in the fixed order (§8 Q4). A card whose target is gone does nothing; its energy stays spent and the card goes to its normal destination. Publish feedback events the client can animate as one reveal.
5. **Offense timing.**
   - After a participant's first roll, cards may affect only that participant (rule 4); apply this to every effect kind, not just dice.
   - "Any time" means Before + After, each under its own rules (rule 5).
   - Update `content.CardTimingTags`, the generated `Play: …` text if the wording changes, and `docs/game-design/card-configuration.md`.
6. **Defense flow.**
   - The hub opens for **all** humans at once.
   - Each participant chooses defenses for all their incoming attacks before locking in (§8 Q2).
   - AI choices are made from public information at lock-in.
   - All defense rolls happen after the reveal, and their reactions run simultaneously.
   - After cards open only once every defense roll is final.
   - Keep: the per-attack reservations, the release order, saved-card destinations, statuses landing on the defended attack, the single final commit, main Pass ending a participant's step, and auto-complete when only Pass remains.
7. **Client (Godot)**:
   - Show your own secret plays as pending.
   - Show a "waiting for others" state after you lock in.
   - Animate the reveal.
   - Keep the hover/tooltip contracts in AGENTS.md (attack intent hover, visible statuses, card-first prevention targeting, reserved card lists).
   - The client never computes outcomes.
8. **Docs**: update `unified-defense.md` (or replace its flow section), `card-configuration.md`, `card-value-rules.md` (only if timing rules change prices; they shouldn't) and AGENTS.md's "Unified defense experiment" bullets.

## 6. Acceptance criteria

**Offense**
- With the new rule, two human participants (tests may drive two external actors through authority commands) can both play Before cards. Neither one's view shows the other's plays, energy change, hand-size change or status changes until both have locked in.
- A Before Dispel played on an enemy's buff takes effect at Reveal 1, before anyone's first roll. Both sides' plays resolve in the documented order, and both are shown in the reveal feedback.
- After a participant's first roll, no card can target another participant's statuses, cards, dice or damage. Enemy dice are still changeable in the post-offensive reaction moment, as today. Any-time cards that target enemies are legal only in Before.
- Removing Lieutenant's Favor before Reveal 2 lowers the Warlord's attack; removing it in Defense doesn't.

**Defense**
- The Defense step opens for every human participant at once. No human waits for another to pass.
- Nobody's defense rolls until every participant is locked in.
- A Dispel played before the defense rolls removes Fortified, and the enemy's defense rolls without the extra die. Fortified is back at the next round's Ongoing Effects.
- A Dispel played after the defense rolls doesn't change that round's defense rolls.
- With two incoming attacks, both of my defenses (and all enemy defenses) resolve before any After-defense card is legal.
- The defended attacks' statuses are on me when the After step opens, so an After Salve can remove poison before it ticks.
- Undefended attacks' statuses and the remaining damage commit once at the end. There's no second damage phase.
- AI defense choices use only public information; a hidden play by the human doesn't affect the AI's choice except through its revealed effect.

**General**
- Each participant's view never contains another participant's unrevealed plays. Add a test in the style of `TestViewNeverLeaksHiddenState`.
- Saves and reloads in the middle of Before, after one participant has locked in, and during the After step keep the queued plays and lock-ins exactly. They never reroll or re-resolve.
- Legacy saves and battles pinned to `unified_defense` without the new rule keep their current flow.
- Single-player battles, campaigns and expeditions still complete, and the seeded region tests stay green. If the seeds' outcomes change, find new seeds with the env-gated probes; don't weaken the assertions.
- Pointer tests: play a secret Before card, lock in, watch the reveal, then roll; and in Defense with two attacks, After cards stay unavailable until both defenses are done. Cover manual Pass and automatic progression.

## 7. Out of scope

- Don't change any card tree, card price or status rule (Salve and Dispel were rebuilt on 2026-10-17 to match this design).
- Making Lieutenant's Favor dispellable during Defense (the user declined).
- AI enemies playing cards (they don't today). The design must allow it, but no AI card policy is needed now.
- Networking. Build the authority so it would work for several clients; don't add transport.
- A Siege Dispel card (a later idea).

## 8. Open questions: ask the user before coding

1. **What "After" means in Offense.** The proposal: **Before** is until your first roll. **After** is from your first roll until you lock in your ability, and it allows self-only cards (Nudge, Try Again, draw, energy). That's the only way own-dice cards can work. The user said no cards should be played "in the middle" of rolling; we read that as no cards that **affect others** in the middle. Confirm.
2. **Defense choices.** Does each participant choose defenses for **all** their incoming attacks before locking in? (Proposed; it fits "no back and forth".) Or may they pick a later defense after seeing an earlier roll?
3. **Self-only Before effects.** Should they resolve at once for their owner (you see your drawn cards and new energy, hidden from others), or wait for the reveal like everything else? Proposed: draws and energy resolve at once, privately, because you need them to plan; anything that changes a status, die or damage waits for the reveal.
4. **Reveal order.** Proposed: all queued effects resolve in a fixed order, participants by seat (the human first, then enemies in order) and each participant's plays in the order played. A card whose target has gone does nothing. The alternative is to resolve everything against the pre-reveal state at once. Confirm which.
5. **Are After-defense plays secret too** (revealed when everyone passes), or does each resolve when played? Prevention cards today show their saved cards at once.
6. **What the opponents see when a participant locks in.** Proposed: only "locked in" and the public counts from the start of the step.

## 9. Dependencies and starting point

- **Base branch:** this work depends on uncommitted work on branch `t3code/2e688e7a`:
  - the `restore_each_round` status lifecycle (`content.StatusLifecycle.RestoreEachRound`; `recordRestoredStatuses` and `restoreRoundStatuses` in `settled_engine.go`; `SettledActorRuntime.RestoreStatuses`; `TestMapConditionsReturnEachRound`);
  - the rebuilt Salve and Dispel trees (authored revisions 105 and 106);
  - the map card trees.

  Start the new worktree from that branch **after it's committed**; uncommitted edits aren't copied into a new worktree.
- **Key files:**
  - `dice-and-destiny-server/internal/battle/engine/settled_engine.go`: Offensive, Defensive, AI planning, `afterDefenseSelections`, `resolveDefenseRollsAndOpenReaction`, `finalizeDefenses`.
  - `engine/unified_defense.go`: the hub, Pass, card plays, defended statuses.
  - `engine/card_program.go`: `programWindowOpen`, `defenseRolled`, target gating.
  - `engine/legal_actions.go`.
  - `internal/battle/state/settled.go`.
  - `internal/battle/state/venom.go` (`SettledPlanningPrivate`).
  - `internal/battle/snapshot/snapshot.go`: the per-viewer view.
  - `internal/content/card_timing.go`.
  - Client battle screens under `dice-and-destiny-client/app/`.
  - Docs: `unified-defense.md`, `card-configuration.md`.
- **Existing tests to keep and extend:**
  - Go: `engine/unified_defense_test.go`, `card_timing_test.go`, `planning_card_public_state_test.go`, `planning_test.go`, `expedition_statuses_test.go`.
  - Godot: `verify_unified_defense.gd`, `verify_steady_guard_targeting.gd`, `verify_card_timing_choices.gd`, `verify_defense_*.gd`, `verify_damage_card_layout.gd` (with `DICE_AND_DESTINY_UNIFIED_LAYOUT=1`), `verify_expedition_map.gd`, `verify_expedition_statuses.gd`.
- **Project rules** (AGENTS.md):
  - Always run Godot through `./scripts/godot.sh`.
  - Build native code with `dice-and-destiny-server/scripts/build_native.sh`, or let the launcher rebuild stale artifacts.
  - Standard checks: `go test ./...` and `verify_battle_authority.gd`.
  - Script tests never write to `content/authored/`.
  - Commit or push only when the user asks, after showing the diff.
  - No Dice Throne names.
