package engine

import (
	"encoding/json"
	"errors"
	"fmt"
	"sort"

	"diceanddestiny/server/internal/battle/command"
	"diceanddestiny/server/internal/battle/damage"
	"diceanddestiny/server/internal/battle/event"
	"diceanddestiny/server/internal/battle/operation"
	"diceanddestiny/server/internal/battle/state"
	"diceanddestiny/server/internal/content"
)

// Simultaneous play (docs/game-design/simultaneous-play-plan.md).
//
// Offense: Before (secret plays) → everyone locks in by rolling or skipping →
// Reveal 1 (queued plays resolve, then every first roll) → rolling (self-only
// plays, hidden) → every ability locks in → Reveal 2 → attacks are set.
//
// Offense After and Defense Before are one step, between the attacks and the
// defenses: secret plays (After-offense and Before-defense cards, any target)
// and a defense for each incoming attack → everyone locks in → reveal (queued
// plays, re-scoring any attack they changed, then every defense roll) →
// review (opt-in reaction cards) → defenses resolve and their statuses land →
// After-defense (visible) → one final commit.

func simultaneousPlay(b *state.Battle) bool {
	return b != nil && b.Settled != nil && b.Settled.SimultaneousPlay
}

func simultaneousStep(b *state.Battle) string {
	if !simultaneousPlay(b) || b.Settled.Simultaneous == nil {
		return ""
	}
	return b.Settled.Simultaneous.Step
}

// simultaneousSecret reports a step whose plays other participants can't see.
func simultaneousSecret(b *state.Battle) bool {
	switch simultaneousStep(b) {
	case state.SimultaneousOffenseBefore, state.SimultaneousOffenseRolling, state.SimultaneousDefenseBefore:
		return true
	}
	return false
}

// simultaneousBefore reports a Before step, where plays on statuses, dice,
// damage or other participants wait for the reveal.
func simultaneousBefore(b *state.Battle) bool {
	step := simultaneousStep(b)
	return (step == state.SimultaneousOffenseBefore || step == state.SimultaneousDefenseBefore) && !b.Settled.Simultaneous.Resolving
}

// simultaneousWindow reports a window every participant answers at once.
func simultaneousWindow(b *state.Battle) bool {
	return simultaneousStep(b) != "" && b.Settled.Window != nil
}

// SimultaneousPlayEligible reports whether every participant's cards work
// under simultaneous play: card programs and cards that only touch their
// owner. Mechanic cards (Venom, Curse) and cards that target another
// participant through a legacy selector keep the battle on the unified flow.
func SimultaneousPlayEligible(b *state.Battle) bool {
	library, err := settledLibrary(b)
	if err != nil {
		return false
	}
	for _, runtime := range b.Settled.Actors {
		for _, instance := range runtime.CardInstances {
			if !simultaneousCardSupported(library.Cards[instance.DefinitionID]) {
				return false
			}
		}
	}
	return true
}

func simultaneousCardSupported(definition content.BattleCardDefinition) bool {
	if definition.Program != nil {
		return true
	}
	if definition.Mechanic != nil {
		return false
	}
	switch definition.Targeting.Selector {
	case "general_choice", "venom_choice", "curse_choice", "one_enemy":
		return false
	}
	return true
}

// simultaneousWindowOpen maps a card's authored windows onto the current
// simultaneous step. Offense any time covers Before, the rolls (self only)
// and After; Offense After and Defense Before share one step.
func simultaneousWindowOpen(b *state.Battle, windows []string) (bool, bool) {
	var open []string
	switch simultaneousStep(b) {
	case state.SimultaneousOffenseBefore:
		open = []string{"offensive_before_roll", "offensive_planning"}
	case state.SimultaneousOffenseRolling:
		open = []string{"offensive_after_roll", "offensive_planning"}
	case state.SimultaneousDefenseBefore:
		// defense_reaction: rerolls of a defense already rolled this step.
		open = []string{"offensive_reaction", "offensive_after_roll", "offensive_planning", "defense_before_roll", "defense_selection", "defense_reaction"}
	case state.SimultaneousDefenseAfter:
		open = []string{"defense_after_roll", "defense_selection", "damage_reaction"}
	default:
		return false, false
	}
	for _, w := range windows {
		if content.ProgramContains(open, w) {
			return true, true
		}
	}
	return false, true
}

// simultaneousBeforeDefenseOnly reports a card the step between the attacks
// and the defenses opens only as a Before-defense card: it plays on an attack
// until that attack has its defense (rolled, or none).
func simultaneousBeforeDefenseOnly(b *state.Battle, windows []string) bool {
	if simultaneousStep(b) != state.SimultaneousDefenseBefore || !content.ProgramContains(windows, "defense_before_roll") {
		return false
	}
	for _, w := range windows {
		if w != "defense_before_roll" && content.ProgramContains([]string{"offensive_reaction", "offensive_after_roll", "offensive_planning", "defense_selection", "defense_reaction"}, w) {
			return false
		}
	}
	return true
}

// simultaneousDefers reports whether a program step played now waits for the
// reveal: everything except private changes to the player's own piles and
// resources.
func simultaneousDefers(b *state.Battle, actor string, s content.CardStep, targets []programChoice) bool {
	if !simultaneousBefore(b) {
		return false
	}
	switch s.Effect {
	case "prevent":
		// Prevention on your own incoming attack resolves at once, for you.
		for _, t := range targets {
			if source := batchSourceByID(b.Settled.PendingDamage, t.Source); source == nil || source.TargetActorID != actor {
				return true
			}
		}
		return false
	case "draw", "energy", "look_deck", "order_deck", "move_cards", "sacrifice", "extra_roll", "choice", "reroll_defense":
		for _, t := range targets {
			if t.Actor != "" && t.Actor != actor {
				return true
			}
		}
		return false
	}
	return true
}

// settledCardDefers reports a legacy card that waits for the reveal: any card
// that does more than draw or gain resources.
func settledCardDefers(definition content.BattleCardDefinition) bool {
	for _, op := range definition.Operations {
		if op.Type != "draw_cards" && op.Type != "gain_resource" {
			return true
		}
	}
	return false
}

func beginSimultaneousStep(b *state.Battle, step string) {
	sim := b.Settled.Simultaneous
	if sim == nil {
		sim = &state.SettledSimultaneous{}
		b.Settled.Simultaneous = sim
	}
	sim.Step = step
	sim.Locked = map[string]bool{}
	if sim.Rolling == nil {
		sim.Rolling = map[string]bool{}
	}
	sim.Queue = nil
	sim.SecretWounds = nil
	sim.Resolving = false
	sim.Public = nil
	if step == state.SimultaneousOffenseBefore {
		sim.Rolling = map[string]bool{}
		sim.Revealed = nil
	}
	if step == state.SimultaneousDefenseBefore {
		sim.Revealed = nil
	}
	if step == state.SimultaneousOffenseBefore || step == state.SimultaneousOffenseRolling || step == state.SimultaneousDefenseBefore {
		sim.Public = captureSimultaneousPublic(b)
	}
	sim.PublicDamage = nil
	if step == state.SimultaneousDefenseBefore && b.Settled.PendingDamage != nil {
		sim.PublicDamage = cloneDamageBatch(b.Settled.PendingDamage)
	}
}

// endSimultaneousStep leaves the simultaneous steps; the last reveal stays
// visible until the next one.
func endSimultaneousStep(b *state.Battle) {
	if sim := b.Settled.Simultaneous; sim != nil {
		sim.Step = ""
		sim.Locked = nil
		sim.Rolling = nil
		sim.Queue = nil
		sim.Public = nil
		sim.PublicDamage = nil
		sim.SecretWounds = nil
		sim.Resolving = false
	}
}

func cloneDamageBatch(batch *state.SettledDamageBatch) *state.SettledDamageBatch {
	raw, _ := json.Marshal(batch)
	var clone state.SettledDamageBatch
	_ = json.Unmarshal(raw, &clone)
	return &clone
}

func captureSimultaneousPublic(b *state.Battle) map[string]state.SimultaneousPublic {
	public := make(map[string]state.SimultaneousPublic, len(b.Actors))
	for id, actor := range b.Actors {
		public[id] = state.SimultaneousPublic{
			EnergyPoints: actor.Resources.EnergyPoints,
			HandCount:    len(actor.Cards.Hand), DeckCount: len(actor.Cards.Deck),
			DiscardCount: len(actor.Cards.Discard), RemovedCount: len(actor.Cards.Removed),
			Statuses:         append([]state.StatusState(nil), actor.Statuses...),
			AbilityModifiers: append([]state.RuntimeAbilityModifier(nil), b.Settled.Actors[id].AbilityModifiers...),
		}
	}
	return public
}

// secretPending reports a card played in secret that waits in hand for the reveal.
func secretPending(b *state.Battle, actor, card string) bool {
	if !simultaneousPlay(b) || b.Settled.Simultaneous == nil {
		return false
	}
	for _, play := range b.Settled.Simultaneous.Queue {
		if play.ActorID == actor && play.CardID == card && !play.Immediate {
			return true
		}
	}
	return false
}

func protectQueued(b *state.Battle, actor string) bool {
	if !simultaneousPlay(b) || b.Settled.Simultaneous == nil {
		return false
	}
	for _, play := range b.Settled.Simultaneous.Queue {
		if play.ActorID == actor && play.ProtectSourceID != "" {
			return true
		}
	}
	return false
}

func markSecretEvents(events []event.Event, actor string) {
	for i := range events {
		if events[i].Type != event.TypeSimultaneousLockedIn {
			events[i].SecretActorID = actor
		}
	}
}

func lockedInEvent(b *state.Battle, actor string) event.Event {
	return settledEvent(event.TypeSimultaneousLockedIn, b, actor, map[string]any{"step": simultaneousStep(b)})
}

func nextSecretSequence(b *state.Battle) int {
	b.Settled.Simultaneous.Sequence++
	return b.Settled.Simultaneous.Sequence
}

func queueSecretProgramPlay(b *state.Battle, actor, card, definition string, x *state.CardExecution) {
	sim := b.Settled.Simultaneous
	play := state.SecretPlay{Sequence: nextSecretSequence(b), Step: sim.Step, ActorID: actor, CardID: card, CardDefinitionID: definition, Immediate: len(x.Deferred) == 0}
	for _, step := range x.Deferred {
		play.Steps = append(play.Steps, step)
		for _, key := range step.Targets {
			var c programChoice
			if json.Unmarshal([]byte(key), &c) == nil && c.Label != "" {
				play.Targets = append(play.Targets, c.Label)
			}
		}
	}
	sim.Queue = append(sim.Queue, play)
}

func queueSecretLegacyPlay(b *state.Battle, library content.BattleLibrary, actor, card string, definition content.BattleCardDefinition, targetIDs []string, abilityID string, dieIndex int, statusID string) {
	sim := b.Settled.Simultaneous
	play := state.SecretPlay{Sequence: nextSecretSequence(b), Step: sim.Step, ActorID: actor, CardID: card, CardDefinitionID: definition.ID, Legacy: &state.SecretLegacyPlay{TargetIDs: append([]string(nil), targetIDs...), AbilityID: abilityID, DieIndex: dieIndex, StatusID: statusID}}
	switch {
	case statusID != "" && library.Statuses[statusID].Name != "":
		play.Targets = []string{library.Statuses[statusID].Name}
	case abilityID != "":
		play.Targets = []string{library.Abilities[abilityID].Name}
	case definition.Targeting.Selector == "one_incoming_damage_source" && len(targetIDs) == 1:
		if source := effectDamageSourceByID(b, targetIDs[0]); source != nil {
			play.Targets = []string{programSourceName(b, library, *source)}
		}
	}
	sim.Queue = append(sim.Queue, play)
}

func queueSecretImmediatePlay(b *state.Battle, actor, card, definition string, targets []string) {
	sim := b.Settled.Simultaneous
	sim.Queue = append(sim.Queue, state.SecretPlay{Sequence: nextSecretSequence(b), Step: sim.Step, ActorID: actor, CardID: card, CardDefinitionID: definition, Immediate: true, Targets: targets})
}

// finishProgramCard sends a resolved program card to its play destination.
func finishProgramCard(b *state.Battle, actor, card string, definition content.BattleCardDefinition) {
	a := b.Actors[actor]
	zone := programCardZone(b, actor, card)
	if zone != "" && zone != operation.ZoneRemoved {
		moveCard(&a.Cards, card, zone, operation.CardZone(definition.Play.Destination))
		b.Actors[actor] = a
	}
	retainPlayedPreventionCard(b, actor, card)
}

// retainPlayedPreventionCard keeps a card that saved itself in its live pile.
func retainPlayedPreventionCard(b *state.Battle, actor, card string) {
	if batch := b.Settled.PendingDamage; batch != nil {
		for i := range batch.Removals {
			r := &batch.Removals[i]
			if r.TargetActorID == actor && r.CardID == card && r.Released {
				damage.RetainPreventedCard(b, r)
			}
		}
	}
}

func simultaneousRequired(b *state.Battle) []string {
	if b.Settled.Window == nil {
		return nil
	}
	return b.Settled.Window.RequiredActorIDs
}

func simultaneousAllLocked(b *state.Battle) bool {
	for _, id := range simultaneousRequired(b) {
		if !b.Settled.Simultaneous.Locked[id] {
			return false
		}
	}
	return true
}

func simultaneousAllPassed(b *state.Battle) bool {
	window := b.Settled.Window
	if window == nil {
		return true
	}
	for _, id := range window.RequiredActorIDs {
		if !window.Passes[id] {
			return false
		}
	}
	return true
}

func liveExternalActorIDs(b *state.Battle) []string {
	var ids []string
	for _, id := range externalSettledActorIDs(b) {
		if b.Actors[id].DefeatState != state.ActorDefeated {
			ids = append(ids, id)
		}
	}
	return ids
}

// revealSecretPlays resolves the queue in seat order, then play order, and
// publishes one reveal of every play. A play whose targets are all gone does
// nothing; its energy stays spent and the card goes to its destination.
func (e Engine) revealSecretPlays(b *state.Battle, library content.BattleLibrary) ([]event.Event, error) {
	sim := b.Settled.Simultaneous
	step := sim.Step
	plays := append([]state.SecretPlay(nil), sim.Queue...)
	seat := map[string]int{}
	for i, id := range sortedSettledActorIDs(b) {
		seat[id] = i
	}
	sort.SliceStable(plays, func(i, j int) bool {
		if seat[plays[i].ActorID] != seat[plays[j].ActorID] {
			return seat[plays[i].ActorID] < seat[plays[j].ActorID]
		}
		return plays[i].Sequence < plays[j].Sequence
	})
	sim.Queue = nil
	sim.Resolving = true
	var revealed []state.RevealedPlay
	var events []event.Event
	for _, play := range plays {
		outcome := "resolved"
		if !play.Immediate {
			applied, next, err := e.resolveSecretPlay(b, library, play)
			if err != nil {
				sim.Resolving = false
				return nil, err
			}
			if !applied {
				outcome = "no_effect"
			}
			events = append(events, next...)
		}
		revealed = append(revealed, state.RevealedPlay{Sequence: play.Sequence, Step: play.Step, ActorID: play.ActorID, CardID: play.CardID, CardDefinitionID: play.CardDefinitionID, Targets: append([]string(nil), play.Targets...), Outcome: outcome})
	}
	sim.Resolving = false
	sim.SecretWounds = nil
	sim.Revealed = revealed
	if len(revealed) == 0 {
		return events, nil
	}
	return append([]event.Event{settledEvent(event.TypeSecretPlaysRevealed, b, "", map[string]any{"step": step, "plays": revealed})}, events...), nil
}

func sourceAmounts(b *state.Battle) map[string]int {
	amounts := map[string]int{}
	if batch := b.Settled.PendingDamage; batch != nil {
		for _, s := range batch.Sources {
			amounts[s.ID] = settledSourceAmount(s)
		}
	}
	return amounts
}

func damageChangeEvents(b *state.Battle, actor string, amounts map[string]int, data map[string]any) []event.Event {
	var events []event.Event
	if batch := b.Settled.PendingDamage; batch != nil {
		for _, s := range batch.Sources {
			if before, ok := amounts[s.ID]; ok && before != settledSourceAmount(s) {
				changed := map[string]any{"source_id": s.ID, "damage_before": before, "damage_after": settledSourceAmount(s), "target_actor_id": s.TargetActorID}
				for k, v := range data {
					changed[k] = v
				}
				events = append(events, settledEvent(event.TypeDamageModified, b, actor, changed))
			}
		}
	}
	return events
}

func (e Engine) resolveSecretPlay(b *state.Battle, library content.BattleLibrary, play state.SecretPlay) (bool, []event.Event, error) {
	inputs := attackInputs(b)
	applied, events, err := e.resolveQueuedPlay(b, library, play)
	if err != nil || play.Step != state.SimultaneousDefenseBefore {
		return applied, events, err
	}
	// A play that changed an attack's dice, bonuses or statuses re-scores it
	// before any defense rolls.
	amounts := sourceAmounts(b)
	for _, attacker := range sortedSettledActorIDs(b) {
		if before, ok := inputs[attacker]; ok && before != attackInput(b, attacker) {
			if err := e.rescoreAttack(b, library, attacker); err != nil {
				return false, nil, err
			}
		}
	}
	if changed := damageChangeEvents(b, play.ActorID, amounts, map[string]any{"card_instance_id": play.CardID, "card_definition_id": play.CardDefinitionID, "revealed": true, "rescored": true}); len(changed) > 0 {
		events = append(events, changed...)
	}
	return applied, events, nil
}

func (e Engine) resolveQueuedPlay(b *state.Battle, library content.BattleLibrary, play state.SecretPlay) (bool, []event.Event, error) {
	amounts := sourceAmounts(b)
	if play.ProtectSourceID != "" {
		if !roundPreventionEligible(b, library, play.ActorID, play.ProtectSourceID) {
			return false, nil, nil
		}
		next, err := e.spendRoundPrevention(b, library, play.ActorID, command.InteractionCommitmentData{ChoiceID: "spend_round_prevention", ProposalIDs: []string{play.ProtectSourceID}})
		return err == nil, next, err
	}
	definition := library.Cards[play.CardDefinitionID]
	data := map[string]any{"card_instance_id": play.CardID, "card_definition_id": play.CardDefinitionID, "revealed": true}
	if play.Legacy != nil {
		l := play.Legacy
		stacksBefore := stacks(b, play.ActorID, l.StatusID)
		if err := validateCardTargets(b, library, play.ActorID, definition, l.TargetIDs, l.AbilityID, l.DieIndex, l.StatusID); err != nil {
			a := b.Actors[play.ActorID]
			if zone := programCardZone(b, play.ActorID, play.CardID); zone != "" && zone != operation.ZoneRemoved {
				moveCard(&a.Cards, play.CardID, zone, operation.CardZone(definition.Play.Destination))
				b.Actors[play.ActorID] = a
			}
			return false, []event.Event{settledEvent(event.TypeCardPlayed, b, play.ActorID, data)}, nil
		}
		if err := e.resolveSettledCard(b, library, play.ActorID, play.CardID, definition, l.TargetIDs, l.AbilityID, l.DieIndex, l.StatusID, false); err != nil {
			return false, nil, err
		}
		addCardCleanseOutcome(data, b, library, play.ActorID, play.CardDefinitionID, l.StatusID, stacksBefore)
		events := damageChangeEvents(b, play.ActorID, amounts, data)
		if len(events) == 0 {
			events = []event.Event{settledEvent(event.TypeCardPlayed, b, play.ActorID, data)}
		}
		return true, events, nil
	}
	x := &state.CardExecution{CardID: play.CardID, Paid: true}
	applied := false
	for _, deferred := range play.Steps {
		var s content.CardStep
		if err := json.Unmarshal(deferred.Step, &s); err != nil {
			return false, nil, err
		}
		targets := validSecretTargets(b, library, play.ActorID, play.CardID, s, deferred.Targets)
		if len(targets) == 0 {
			continue
		}
		if err := e.applyProgramStep(b, library, play.ActorID, play.CardID, s, targets, x); err != nil {
			return false, nil, err
		}
		applied = true
	}
	finishProgramCard(b, play.ActorID, play.CardID, definition)
	data["program"], data["complete"], data["program_dice_changes"] = true, true, x.Feedback
	events := damageChangeEvents(b, play.ActorID, amounts, data)
	if len(events) == 0 {
		events = []event.Event{settledEvent(event.TypeCardPlayed, b, play.ActorID, data)}
	}
	return applied, events, nil
}

// attackPending reports an attack that is set but not yet final: between the
// attacks and the defense rolls, its dice and bonuses can still change.
func attackPending(b *state.Battle, actor string) bool {
	if simultaneousStep(b) != state.SimultaneousDefenseBefore || b.Settled.Actors[actor].SelectedAbilityID == "" {
		return false
	}
	for _, s := range b.Settled.OffensiveSources {
		if s.SourceActorID == actor {
			return true
		}
	}
	return false
}

// attackInputs records what each pending attack is scored from.
func attackInputs(b *state.Battle) map[string]string {
	inputs := map[string]string{}
	for _, actor := range sortedSettledActorIDs(b) {
		if attackPending(b, actor) {
			inputs[actor] = attackInput(b, actor)
		}
	}
	return inputs
}

func attackInput(b *state.Battle, actor string) string {
	rt := b.Settled.Actors[actor]
	faces := make([]int, len(rt.FinalDice))
	for i, d := range rt.FinalDice {
		faces[i] = d.Face
	}
	raw, _ := json.Marshal([]any{faces, rt.SelectedTierID, rt.AbilityModifiers, b.Actors[actor].Statuses})
	return string(raw)
}

// rescoreAttack scores a set attack again from its current dice, bonuses and
// statuses: the same ability at the tier the dice now reach, or a miss. Only
// its damage and pending statuses change; reserved cards follow the new
// amount. Abilities with their own rolls or special effects keep their score.
func (e Engine) rescoreAttack(b *state.Battle, library content.BattleLibrary, attacker string) error {
	batch := b.Settled.PendingDamage
	rt := b.Settled.Actors[attacker]
	ability := library.Abilities[rt.SelectedAbilityID]
	if batch == nil || rt.SelectedAbilityID == "" || !rescorable(ability) {
		return nil
	}
	if tier, ok := qualifiedTier(ability, rt.FinalDice); !ok {
		rt.SelectedTierID = ""
	} else if !chooseAbilityTier(ability) || !tierMet(ability, rt.SelectedTierID, rt.FinalDice) {
		rt.SelectedTierID = tier.ID
	}
	b.Settled.Actors[attacker] = rt
	damageTo := map[string]int{}
	statusesTo := map[string][]state.SettledStatusApplication{}
	if ops, ok := resolvedOffensiveOperations(b, library, attacker); ok {
		ctx := effectContext{SourceActorID: attacker, SourceContentID: ability.ID, SourceContentType: "ability", TargetActorIDs: rt.SelectedTargetIDs}
		for _, op := range ops {
			switch {
			case op.OnePerStatusStack || op.Repeat == "one_per_status_stack":
				continue // no stacks in an attack's context, as when it was set
			case op.Type == "deal_damage", op.Type == "apply_status" && op.ApplicationTiming != "immediate":
			default:
				continue // immediate effects already happened when it was set
			}
			targets, err := effectTargets(b, ctx, op.Target)
			if err != nil {
				return err
			}
			for _, target := range targets {
				if op.Type == "apply_status" {
					statusesTo[target] = append(statusesTo[target], state.SettledStatusApplication{SourceActorID: attacker, TargetActorID: target, StatusID: op.StatusID, Stacks: max(1, op.StackCount)})
					continue
				}
				amount, err := operationAmount(op, 0)
				if err != nil {
					return err
				}
				damageTo[target] += amount
			}
		}
	}
	for i := range batch.Sources {
		s := &batch.Sources[i]
		if s.SourceActorID != attacker || s.SourceContentID != ability.ID {
			continue
		}
		s.BaseAmount = damageTo[s.TargetActorID]
		for _, old := range s.StatusApplications {
			for j, a := range batch.Applications {
				if a == old {
					batch.Applications = append(batch.Applications[:j], batch.Applications[j+1:]...)
					break
				}
			}
		}
		s.StatusApplications = statusesTo[s.TargetActorID]
		batch.Applications = append(batch.Applications, s.StatusApplications...)
	}
	return e.reconcileUnifiedDamage(b, true)
}

// rescorable reports an ability whose score is only damage and statuses;
// resource gains, like immediate statuses, already happened when it was set.
func rescorable(ability content.BattleAbilityDefinition) bool {
	if ability.Qualification == nil {
		return false
	}
	var ops []content.BattleOperation
	for _, tier := range ability.Qualification.ActivationTiers {
		ops = append(ops, tier.Operations...)
	}
	for _, bonus := range ability.Qualification.ConditionalBonuses {
		ops = append(ops, bonus.Operations...)
	}
	for _, op := range ops {
		switch op.Type {
		case "deal_damage", "apply_status", "gain_resource", "noop":
		default:
			return false
		}
	}
	return true
}

func tierMet(ability content.BattleAbilityDefinition, tierID string, dice []state.RolledDie) bool {
	if ability.Qualification == nil {
		return false
	}
	for _, tier := range ability.Qualification.ActivationTiers {
		if tier.ID == tierID {
			return requirementsMet(tier.Requirements, dice)
		}
	}
	return false
}

// settleAttacks ends the step between offense and defense: the attacks are
// final, so this Offense's bonuses are spent and expire.
func settleAttacks(b *state.Battle) {
	for _, actor := range sortedSettledActorIDs(b) {
		if ability := b.Settled.Actors[actor].SelectedAbilityID; ability != "" {
			consumeProgramBonuses(b, actor, ability)
		}
	}
	expireOffensiveModifiers(b)
}

// validSecretTargets keeps the chosen targets that are still legal now.
func validSecretTargets(b *state.Battle, library content.BattleLibrary, actor, card string, s content.CardStep, keys []string) []programChoice {
	live := map[string]bool{}
	for _, c := range programCandidates(b, library, actor, card, s, nil) {
		live[programTargetKey(c)] = true
	}
	var out []programChoice
	for _, key := range keys {
		var c programChoice
		if json.Unmarshal([]byte(key), &c) == nil && live[programTargetKey(c)] {
			out = append(out, c)
		}
	}
	return out
}

// --- Offense ---------------------------------------------------------------

// openOffenseBefore starts the secret Before step after Offensive entry.
func (e Engine) openOffenseBefore(b *state.Battle, library content.BattleLibrary) ([]event.Event, error) {
	beginSimultaneousStep(b, state.SimultaneousOffenseBefore)
	for _, id := range sortedSettledActorIDs(b) {
		if b.Actors[id].Controller == state.ControllerAI {
			b.Settled.Simultaneous.Locked[id] = true
		}
	}
	external := liveExternalActorIDs(b)
	if len(external) == 0 {
		return e.revealOffenseBefore(b, library)
	}
	openSettledWindowForActors(b, "offensive-planning", stageOffensivePlan, "planning", offensivePlanningCommands(), external, true)
	events := make([]event.Event, 0, len(external))
	for _, actorID := range external {
		events = append(events, event.NewRollRequested(actorID, b.Segment.Current, fmt.Sprintf("roll-r%d-%s", b.Segment.Round, actorID), b.Flow.PendingInput[actorID].ID))
	}
	return events, nil
}

func offensivePlanningCommands() []command.Type {
	return []command.Type{command.TypePlanningCards, command.TypePlanningRoll, command.TypePlanningKeep, command.TypePlanningReroll, command.TypePlanningAbility, command.TypePlanningTargets, command.TypePlanningPass}
}

// lockOffenseBefore locks a participant in: by rolling (the dice roll at the
// reveal) or by skipping their offense.
func (e Engine) lockOffenseBefore(b *state.Battle, library content.BattleLibrary, actor string, rolling bool) ([]event.Event, error) {
	sim := b.Settled.Simultaneous
	if sim.Locked[actor] {
		return nil, errors.New("already locked in")
	}
	if b.Settled.Actors[actor].CardExecution != nil {
		return nil, errors.New("finish or cancel the card first")
	}
	sim.Locked[actor] = true
	if rolling {
		sim.Rolling[actor] = true
	} else {
		if !reserveOffensiveCost(b, library, actor) {
			return nil, errors.New("insufficient energy for selected ability")
		}
		rt := b.Settled.Actors[actor]
		rt.PlanningCommitted = true
		b.Settled.Actors[actor] = rt
	}
	events := []event.Event{lockedInEvent(b, actor)}
	closeSettledInputForActor(b, actor, state.ActorLockedIn)
	if !simultaneousAllLocked(b) {
		return events, nil
	}
	next, err := e.revealOffenseBefore(b, library)
	return append(events, next...), err
}

// revealOffenseBefore is Reveal 1: queued plays resolve, then everyone who
// locked in by rolling rolls at once.
func (e Engine) revealOffenseBefore(b *state.Battle, library content.BattleLibrary) ([]event.Event, error) {
	closeSettledWindow(b)
	events, err := e.revealSecretPlays(b, library)
	if err != nil {
		return nil, err
	}
	var rollers []string
	for _, id := range sortedSettledActorIDs(b) {
		if !b.Settled.Simultaneous.Rolling[id] || b.Actors[id].DefeatState == state.ActorDefeated {
			continue
		}
		rolled, err := e.firstOffensiveRoll(b, library, id)
		if err != nil {
			return nil, err
		}
		events = append(events, rolled)
		if !b.Settled.Actors[id].PlanningCommitted {
			rollers = append(rollers, id)
		}
	}
	beginSimultaneousStep(b, state.SimultaneousOffenseRolling)
	if len(rollers) == 0 {
		next, err := e.revealOffense(b, library)
		return append(events, next...), err
	}
	openSettledWindowForActors(b, "offensive-planning", stageOffensivePlan, "planning", offensivePlanningCommands(), rollers, true)
	return events, nil
}

// revealOffense is Reveal 2: every ability is revealed and the attacks are
// set. Offense After is the Defense Before step that follows, so play does not
// stop here.
func (e Engine) revealOffense(b *state.Battle, library content.BattleLibrary) ([]event.Event, error) {
	closeSettledWindow(b)
	b.Settled.ReactionReplanning = false
	endSimultaneousStep(b)
	events := []event.Event{offensiveRevealEvent(b, library)}
	next, err := e.finalizeOffensiveSources(b, library)
	return append(events, next...), err
}

// --- Defense ---------------------------------------------------------------

func defenseBeforeCommands() []command.Type {
	return []command.Type{command.TypePlanningAbility, command.TypePlanningPass, command.TypeCommitInteraction}
}

// openDefenseBefore opens the step between the attacks and the defenses for
// every participant at once. AI defenders choose now, before any hidden play
// exists; their defenses roll at the reveal.
func (e Engine) openDefenseBefore(b *state.Battle, library content.BattleLibrary) ([]event.Event, error) {
	b.Settled.DefensePlans = map[string]state.SettledDefense{}
	b.Settled.DefenseSelections = map[string]state.SettledDefense{}
	b.Settled.Stage = stageDefenseSelect
	b.Flow.Stage = stageDefenseSelect
	beginSimultaneousStep(b, state.SimultaneousDefenseBefore)
	if err := e.planAIDefenses(b, library); err != nil {
		return nil, err
	}
	actors := liveExternalActorIDs(b)
	if len(actors) == 0 {
		return e.revealDefense(b, library)
	}
	openSettledWindowForActors(b, "defense-before", stageDefenseSelect, "reaction", defenseBeforeCommands(), actors, true)
	return nil, nil
}

func (e Engine) planAIDefenses(b *state.Battle, library content.BattleLibrary) error {
	for _, actorID := range sortedSettledActorIDs(b) {
		if b.Actors[actorID].Controller != state.ControllerAI {
			continue
		}
		b.Settled.Simultaneous.Locked[actorID] = true
		legal := b.Settled.Actors[actorID].DefensiveAbilityIDs
		if len(legal) == 0 || b.Actors[actorID].DefeatState == state.ActorDefeated {
			continue
		}
		for _, source := range b.Settled.OffensiveSources {
			if source.TargetActorID != actorID || settledSourceAmount(source) <= 0 {
				continue
			}
			choice := 0
			if len(legal) > 1 {
				value, err := e.namedIntn(b, "ai_defense", len(legal))
				if err != nil {
					return err
				}
				choice = value
			}
			ordered := []string{legal[choice]}
			if definition, ok := library.Combatants[b.Actors[actorID].DefinitionID]; ok && definition.AI != nil {
				ordered = append(ordered, definition.AI.DefensiveSelection.PreferenceOrder...)
			}
			ordered = append(ordered, legal...)
			for _, abilityID := range ordered {
				ability := library.Abilities[abilityID]
				if !containsString(legal, abilityID) || b.Actors[actorID].Resources.EnergyPoints < ability.Cost.Energy || ability.Usage.MaximumPerSegment > 0 && simultaneousDefenseUses(b, actorID, abilityID) >= ability.Usage.MaximumPerSegment {
					continue
				}
				if abilityOnlyDefense(ability) && library.Abilities[source.SourceContentID].Type != "offensive" {
					continue
				}
				spendEnergy(b, actorID, ability.Cost.Energy)
				rt := b.Settled.Actors[actorID]
				rt.UsedAbilities[abilityID]++
				b.Settled.Actors[actorID] = rt
				b.Settled.DefensePlans[source.ID] = state.SettledDefense{ActorID: actorID, AbilityID: abilityID, SourceID: source.ID}
				break
			}
		}
	}
	return nil
}

func simultaneousDefenseUses(b *state.Battle, actor, ability string) int {
	n := 0
	for _, plan := range b.Settled.DefensePlans {
		if plan.ActorID == actor && plan.AbilityID == ability {
			n++
		}
	}
	for _, defense := range b.Settled.DefenseHistory {
		if defense.ActorID == actor && defense.AbilityID == ability {
			n++
		}
	}
	return n
}

// defenseChosen reports an attack that already has its defense (or none).
func defenseChosen(b *state.Battle, sourceID string) bool {
	if _, ok := b.Settled.DefensePlans[sourceID]; ok {
		return true
	}
	_, ok := b.Settled.DefenseHistory[sourceID]
	return ok
}

// unplannedDefenseSources lists the attacks on an actor still waiting for a
// defense choice, in attack order.
func unplannedDefenseSources(b *state.Battle, actor string) []state.SettledDamageSource {
	var out []state.SettledDamageSource
	for _, source := range b.Settled.OffensiveSources {
		if source.TargetActorID != actor || settledSourceAmount(source) <= 0 || defenseChosen(b, source.ID) {
			continue
		}
		out = append(out, source)
	}
	return out
}

func simultaneousDefenseActions(b *state.Battle, library content.BattleLibrary, actor string, pending state.PendingInput) []command.Command {
	var actions []command.Command
	if simultaneousStep(b) == state.SimultaneousDefenseBefore {
		unplanned := unplannedDefenseSources(b, actor)
		energy := b.Actors[actor].Resources.EnergyPoints
		defendable := map[string]bool{}
		for _, abilityID := range b.Settled.Actors[actor].DefensiveAbilityIDs {
			ability := library.Abilities[abilityID]
			var eligible []string
			for _, source := range unplanned {
				if abilityOnlyDefense(ability) && library.Abilities[source.SourceContentID].Type != "offensive" {
					continue
				}
				eligible = append(eligible, source.ID)
			}
			if len(eligible) == 0 || energy < ability.Cost.Energy {
				continue
			}
			uses := simultaneousDefenseUses(b, actor, abilityID)
			if ability.Usage.MaximumPerSegment > 0 && uses >= ability.Usage.MaximumPerSegment {
				continue
			}
			for _, sourceID := range eligible {
				defendable[sourceID] = true
				actions = append(actions, legalCommand(b.ID, actor, command.TypePlanningAbility, command.PlanningAbilityPayload{PendingInputID: pending.ID, Checkpoint: planningCheckpoint(pending), AbilityID: abilityID, TargetIDs: []string{sourceID}}))
			}
			// The same defense for every remaining attack, when it can cover them all.
			if len(eligible) > 1 && energy >= ability.Cost.Energy*len(eligible) && (ability.Usage.MaximumPerSegment == 0 || uses+len(eligible) <= ability.Usage.MaximumPerSegment) {
				actions = append(actions, legalCommand(b.ID, actor, command.TypePlanningAbility, command.PlanningAbilityPayload{PendingInputID: pending.ID, Checkpoint: planningCheckpoint(pending), AbilityID: abilityID, TargetIDs: eligible}))
			}
		}
		actions = append(actions, legalCommand(b.ID, actor, command.TypePlanningPass, command.PlanningPassPayload{PendingInputID: pending.ID, Checkpoint: planningCheckpoint(pending)}))
		// "No defense" only matters while a defense could still answer the attack.
		for _, source := range unplanned {
			if defendable[source.ID] {
				actions = append(actions, legalCommand(b.ID, actor, command.TypePlanningPass, command.PlanningPassPayload{PendingInputID: pending.ID, Checkpoint: planningCheckpoint(pending), SourceID: source.ID}))
			}
		}
	} else {
		actions = append(actions, legalCommand(b.ID, actor, command.TypePlanningPass, command.PlanningPassPayload{PendingInputID: pending.ID, Checkpoint: planningCheckpoint(pending)}))
	}
	return append(actions, reactionCardActions(b, library, actor, pending)...)
}

func (e Engine) handleDefenseBefore(b *state.Battle, library content.BattleLibrary, cmd command.Command) ([]event.Event, error) {
	actor := cmd.ActorID
	switch cmd.Type {
	case command.TypeCommitInteraction:
		var payload command.CommitInteractionPayload
		if err := command.DecodePayload(cmd, &payload); err != nil {
			return nil, err
		}
		if payload.Commitment.ChoiceID == "spend_round_prevention" {
			if len(payload.Commitment.ProposalIDs) != 1 || !roundPreventionEligible(b, library, actor, payload.Commitment.ProposalIDs[0]) {
				return nil, errors.New("round protection requires an unspent grant and one incoming attack this round")
			}
			// Protect guards your own incoming attack: it resolves at once, for you.
			events, err := e.spendRoundPrevention(b, library, actor, payload.Commitment)
			if err != nil {
				return nil, err
			}
			queueSecretImmediatePlay(b, actor, "", "protect", []string{"Protect"})
			rotateSettledPending(b, actor)
			return events, nil
		}
		events, err := e.handleUnifiedCard(b, library, cmd)
		if err == nil {
			rotateSettledPending(b, actor)
			if len(events) == 1 && events[0].Type == event.TypeCardPlayed && secretPending(b, actor, first(payload.Commitment.CardIDs)) {
				events[0].Type = event.TypeCardQueued
			}
		}
		return events, err
	case command.TypePlanningAbility:
		return e.planSimultaneousDefense(b, library, cmd)
	case command.TypePlanningPass:
		var payload command.PlanningPassPayload
		if err := command.DecodePayload(cmd, &payload); err != nil {
			return nil, err
		}
		if payload.SourceID != "" {
			source := sourceBySettledID(b, payload.SourceID)
			if source == nil || source.TargetActorID != actor {
				return nil, errors.New("invalid incoming source")
			}
			if defenseChosen(b, source.ID) {
				return nil, errors.New("that attack already has a defense choice")
			}
			b.Settled.DefensePlans[source.ID] = state.SettledDefense{ActorID: actor, SourceID: source.ID}
			rotateSettledPending(b, actor)
			return []event.Event{settledEvent(event.TypeDefensePlanned, b, actor, map[string]any{"source_ids": []string{source.ID}, "skipped": true})}, nil
		}
		// The main Pass locks in; attacks still without a defense go undefended.
		return e.lockDefenseBefore(b, library, actor)
	}
	return nil, unsupportedCommand()
}

// planSimultaneousDefense rolls and resolves a defense the moment it is
// chosen, for its owner alone; the others see it at the reveal.
func (e Engine) planSimultaneousDefense(b *state.Battle, library content.BattleLibrary, cmd command.Command) ([]event.Event, error) {
	actor := cmd.ActorID
	var payload command.PlanningAbilityPayload
	if err := command.DecodePayload(cmd, &payload); err != nil {
		return nil, err
	}
	rt := b.Settled.Actors[actor]
	if !containsString(rt.DefensiveAbilityIDs, payload.AbilityID) || len(payload.TargetIDs) == 0 || payload.SpendCatalyst {
		return nil, errors.New("select one legal defense and its incoming attacks")
	}
	ability := library.Abilities[payload.AbilityID]
	seen := map[string]bool{}
	for _, id := range payload.TargetIDs {
		source := sourceBySettledID(b, id)
		if seen[id] || source == nil || source.TargetActorID != actor || settledSourceAmount(*source) <= 0 {
			return nil, errors.New("defense target is not a remaining incoming attack")
		}
		if defenseChosen(b, id) {
			return nil, errors.New("that attack already has a defense choice")
		}
		if abilityOnlyDefense(ability) && library.Abilities[source.SourceContentID].Type != "offensive" {
			return nil, errors.New("this defense requires an incoming offensive ability")
		}
		seen[id] = true
	}
	count := len(payload.TargetIDs)
	if ability.Usage.MaximumPerSegment > 0 && simultaneousDefenseUses(b, actor, payload.AbilityID)+count > ability.Usage.MaximumPerSegment {
		return nil, fmt.Errorf("ability %q has reached its segment usage limit", payload.AbilityID)
	}
	if b.Actors[actor].Resources.EnergyPoints < ability.Cost.Energy*count {
		return nil, errors.New("insufficient energy")
	}
	events := []event.Event{settledEvent(event.TypeDefensePlanned, b, actor, map[string]any{"ability_id": payload.AbilityID, "source_ids": payload.TargetIDs})}
	for _, id := range payload.TargetIDs {
		spendEnergy(b, actor, ability.Cost.Energy)
		rt = b.Settled.Actors[actor]
		rt.UsedAbilities[payload.AbilityID]++
		b.Settled.Actors[actor] = rt
		next, err := e.resolveChosenDefense(b, library, state.SettledDefense{ActorID: actor, AbilityID: payload.AbilityID, SourceID: id})
		if err != nil {
			return nil, err
		}
		events = append(events, next...)
	}
	rotateSettledPending(b, actor)
	return events, nil
}

func pendingSourceAmount(b *state.Battle, id string) int {
	if s := batchSourceByID(b.Settled.PendingDamage, id); s != nil {
		return settledSourceAmount(*s)
	}
	return 0
}

// resolveChosenDefense rolls a defense, resolves it at once and records what
// it prevented and gained, so a reroll of its dice can re-resolve it. The
// attack's statuses land at the reveal, once the attack is final.
func (e Engine) resolveChosenDefense(b *state.Battle, library content.BattleLibrary, plan state.SettledDefense) ([]event.Event, error) {
	var events []event.Event
	if defenseRollOperation(library.Abilities[plan.AbilityID]) != nil {
		rolled, err := e.rollSelectedDefense(b, library, &plan)
		if err != nil {
			return nil, err
		}
		events = append(events, rolled...)
	}
	before, energy := pendingSourceAmount(b, plan.SourceID), b.Actors[plan.ActorID].Resources.EnergyPoints
	resolved, next, err := e.resolveDefenseSelection(b, library, plan)
	if err != nil {
		return nil, err
	}
	resolved.Prevented = before - pendingSourceAmount(b, plan.SourceID)
	resolved.EnergyGained = b.Actors[plan.ActorID].Resources.EnergyPoints - energy
	b.Settled.DefenseHistory[plan.SourceID] = resolved
	return append(events, next...), nil
}

// rerollableDefense reports a defense whose result is prevention and energy,
// so a changed die can re-resolve it.
func rerollableDefense(ability content.BattleAbilityDefinition) bool {
	var check func([]content.BattleOperation) bool
	check = func(ops []content.BattleOperation) bool {
		for _, op := range ops {
			switch op.Type {
			case "prevent_damage", "gain_resource", "noop":
			case "roll_dice":
				for _, outcome := range op.Outcomes {
					if !check(outcome.Operations) {
						return false
					}
				}
			default:
				return false
			}
		}
		return true
	}
	return check(ability.Resolution.Operations)
}

// reresolveDefense applies a resolved defense again after its dice changed:
// what it prevented and gained gives way to what the new dice give.
func (e Engine) reresolveDefense(b *state.Battle, library content.BattleLibrary, defense state.SettledDefense) error {
	result, err := e.defenseResult(b, library, defense)
	if err != nil {
		return err
	}
	prevent := 0
	for _, p := range result.Preventions {
		if p.ProposalID == defense.SourceID {
			prevent += p.Amount
		}
	}
	if source := batchSourceByID(b.Settled.PendingDamage, defense.SourceID); source != nil {
		undone := settledSourceAmount(*source) + defense.Prevented
		amount := max(0, undone-prevent)
		setUnifiedSourceAmount(b, source, amount)
		defense.Prevented = undone - amount
	}
	gained := result.ResourceDeltas[defense.ActorID]
	if delta := gained - defense.EnergyGained; delta > 0 {
		gainEnergy(b, defense.ActorID, delta)
	} else if delta < 0 {
		spendEnergy(b, defense.ActorID, min(-delta, b.Actors[defense.ActorID].Resources.EnergyPoints))
	}
	defense.EnergyGained = gained
	b.Settled.DefenseHistory[defense.SourceID] = defense
	return e.reconcilePreventionDestination(b, library.Abilities[defense.AbilityID].SavedCardDestination)
}

func (e Engine) lockDefenseBefore(b *state.Battle, library content.BattleLibrary, actor string) ([]event.Event, error) {
	sim := b.Settled.Simultaneous
	if sim.Locked[actor] {
		return nil, errors.New("already locked in")
	}
	if b.Settled.Actors[actor].CardExecution != nil {
		return nil, errors.New("finish or cancel the card first")
	}
	sim.Locked[actor] = true
	events := []event.Event{lockedInEvent(b, actor)}
	closeSettledInputForActor(b, actor, state.ActorLockedIn)
	if !simultaneousAllLocked(b) {
		return events, nil
	}
	next, err := e.revealDefense(b, library)
	return append(events, next...), err
}

// revealDefense resolves the queued plays (which may re-score attacks), then
// the defenses AI participants chose at entry, lands every defended attack's
// statuses and opens After-defense.
func (e Engine) revealDefense(b *state.Battle, library content.BattleLibrary) ([]event.Event, error) {
	closeSettledWindow(b)
	events, err := e.revealSecretPlays(b, library)
	if err != nil {
		return nil, err
	}
	settleAttacks(b)
	next, err := e.finalizeSimultaneousDefenses(b, library)
	return append(events, next...), err
}

// finalizeSimultaneousDefenses rolls and resolves the planned defenses in
// seat order, then attack order; every defended attack's statuses land
// before After opens.
func (e Engine) finalizeSimultaneousDefenses(b *state.Battle, library content.BattleLibrary) ([]event.Event, error) {
	var events []event.Event
	if b.Settled.DefenseHistory == nil {
		b.Settled.DefenseHistory = map[string]state.SettledDefense{}
	}
	for _, actorID := range sortedSettledActorIDs(b) {
		for _, source := range b.Settled.OffensiveSources {
			plan, ok := b.Settled.DefensePlans[source.ID]
			if !ok || plan.ActorID != actorID || plan.AbilityID == "" {
				continue
			}
			if defenseRollOperation(library.Abilities[plan.AbilityID]) != nil {
				rolled, err := e.rollSelectedDefense(b, library, &plan)
				if err != nil {
					return nil, err
				}
				events = append(events, rolled...)
			}
			_, next, err := e.resolveDefenseSelection(b, library, plan)
			if err != nil {
				return nil, err
			}
			events = append(events, next...)
		}
	}
	for _, source := range b.Settled.OffensiveSources {
		if defense, handled := b.Settled.DefenseHistory[source.ID]; handled {
			if defense.AbilityID != "" {
				applyDefendedAttackStatuses(b, library, source.ID)
			}
			continue
		}
		b.Settled.DefenseHistory[source.ID] = state.SettledDefense{ActorID: source.TargetActorID, SourceID: source.ID, Finalized: true}
	}
	b.Settled.DefensePlans = map[string]state.SettledDefense{}
	next, err := e.openDefenseAfter(b, library)
	return append(events, next...), err
}

// openDefenseAfter opens After-defense once every defense is final.
func (e Engine) openDefenseAfter(b *state.Battle, library content.BattleLibrary) ([]event.Event, error) {
	beginSimultaneousStep(b, state.SimultaneousDefenseAfter)
	b.Settled.Stage = stageDefenseSelect
	b.Flow.Stage = stageDefenseSelect
	var actors []string
	for _, id := range liveExternalActorIDs(b) {
		if !b.Settled.DefensePassed[id] {
			actors = append(actors, id)
		}
	}
	if len(actors) == 0 || state.IsTerminalBattleStatus(b.Status) {
		return e.finishSimultaneousDefense(b, library)
	}
	openSettledWindowForActors(b, "defense-after", stageDefenseSelect, "reaction", []command.Type{command.TypeCommitInteraction, command.TypePlanningPass}, actors, true)
	return nil, nil
}

func (e Engine) handleDefenseAfter(b *state.Battle, library content.BattleLibrary, cmd command.Command) ([]event.Event, error) {
	switch cmd.Type {
	case command.TypeCommitInteraction:
		events, err := e.handleUnifiedCard(b, library, cmd)
		if err == nil {
			rotateSettledPending(b, cmd.ActorID)
		}
		return events, err
	case command.TypePlanningPass:
		b.Settled.DefensePassed[cmd.ActorID] = true
		b.Settled.Window.Passes[cmd.ActorID] = true
		b.Settled.Simultaneous.Locked[cmd.ActorID] = true
		closeSettledInputForActor(b, cmd.ActorID, state.ActorLockedIn)
		if !simultaneousAllPassed(b) {
			return []event.Event{lockedInEvent(b, cmd.ActorID)}, nil
		}
		closeSettledWindow(b)
		return e.finishSimultaneousDefense(b, library)
	}
	return nil, unsupportedCommand()
}

func (e Engine) finishSimultaneousDefense(b *state.Battle, library content.BattleLibrary) ([]event.Event, error) {
	endSimultaneousStep(b)
	if err := e.autoAIDamageResponse(b, library, b.Settled.PendingDamage); err != nil {
		return nil, err
	}
	return e.finishDamageBatch(b, library)
}
