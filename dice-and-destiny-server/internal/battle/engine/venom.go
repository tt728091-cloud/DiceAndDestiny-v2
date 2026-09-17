package engine

import (
	"fmt"
	"strconv"
	"strings"

	"diceanddestiny/server/internal/battle/command"
	"diceanddestiny/server/internal/battle/event"
	"diceanddestiny/server/internal/battle/state"
	"diceanddestiny/server/internal/content"
)

const stageVenomStatus = "venom_status_reaction"

func venomRuntime(b *state.Battle) *state.VenomRuntime {
	if b.Settled.Venom == nil {
		b.Settled.Venom = &state.VenomRuntime{Provoked: map[string]int{}, Used: map[string]bool{}, CatalystUsed: map[string]bool{}, AttackChecks: map[string][]state.SettledEffectRoll{}}
	}
	v := b.Settled.Venom
	if v.Round != b.Segment.Round {
		v.Round = b.Segment.Round
		v.Provoked = map[string]int{}
		v.CatalystUsed = map[string]bool{}
		v.AttackChecks = map[string][]state.SettledEffectRoll{}
		for k := range v.Used {
			if !strings.HasPrefix(k, "battle:") {
				delete(v.Used, k)
			}
		}
		v.Maturing = nil
		v.MaturationDone = false
	}
	return v
}
func hasVenomMechanics(b *state.Battle) bool {
	if b.Settled.Venom != nil {
		return true
	}
	for actor := range b.Actors {
		if venomActor(b, actor) || stacks(b, actor, "catalyst") > 0 || stacks(b, actor, "incubation") > 0 {
			return true
		}
	}
	return false
}
func venomActor(b *state.Battle, id string) bool {
	return containsString(b.Settled.Actors[id].OffensiveAbilityIDs, "needlefang")
}
func stacks(b *state.Battle, actor, status string) int {
	return statusStackCount(b.Actors[actor].Statuses, status)
}
func enemyOf(b *state.Battle, actor string) string { return first(otherActorIDs(b, actor)) }
func toxin(id string) bool                         { return id == "poison" || id == "volatile_poison" }

func applyVenomStatus(b *state.Battle, lib content.BattleLibrary, source string, app state.SettledStatusApplication) {
	overflow := app.StatusID == "poison" && venomActor(b, source) && stacks(b, app.TargetActorID, "poison")+app.Stacks > 3
	applyStatus(b, lib, app.TargetActorID, app.StatusID, app.Stacks)
	if overflow && stacks(b, app.TargetActorID, "incubation") == 0 {
		venomRuntime(b).Queue = append(venomRuntime(b).Queue, state.VenomWork{Kind: "application", SourceActorID: source, TargetActorID: app.TargetActorID, StatusID: "incubation", Stacks: 1, RequirePoison: true})
	}
}
func captureIncubation(b *state.Battle) {
	if !hasVenomMechanics(b) {
		return
	}
	v := venomRuntime(b)
	v.Maturing = nil
	v.MaturationDone = false
	for _, id := range sortedSettledActorIDs(b) {
		for _, s := range b.Actors[id].Statuses {
			if s.DefinitionID == "incubation" {
				v.Maturing = append(v.Maturing, s.InstanceID)
			}
		}
	}
}
func queueMaturation(b *state.Battle) {
	v := venomRuntime(b)
	if v.MaturationDone {
		return
	}
	v.MaturationDone = true
	for _, instance := range v.Maturing {
		actor, status := findStatusInstance(b, instance)
		if status == "incubation" {
			v.Queue = append(v.Queue, state.VenomWork{Kind: "conversion", TargetActorID: actor, InstanceID: instance})
		}
	}
}

// startVenomWork suspends the exact current input. Child rolls use the ordinary
// status reveal and damage machinery, then return to this continuation.
func (e Engine) startVenomWork(b *state.Battle, lib content.BattleLibrary, advance bool) ([]event.Event, error) {
	v := venomRuntime(b)
	if v.Active != nil || len(v.Queue) == 0 {
		return nil, nil
	}
	v.Resume = &state.VenomResume{Stage: b.Settled.Stage, Window: b.Settled.Window, Flow: b.Flow, Trigger: b.Settled.TriggerBatch, Damage: b.Settled.PendingDamage, Advance: advance}
	b.Settled.Window = nil
	b.Flow = state.NewSegmentFlowState(b.Segment)
	b.Flow.Entered = true
	return e.nextVenomWork(b, lib)
}
func (e Engine) nextVenomWork(b *state.Battle, lib content.BattleLibrary) ([]event.Event, error) {
	v := venomRuntime(b)
	if len(v.Queue) == 0 {
		resume := v.Resume
		v.Active = nil
		v.Resume = nil
		b.Settled.Stage = resume.Stage
		b.Settled.Window = resume.Window
		b.Flow = resume.Flow
		b.Settled.TriggerBatch = resume.Trigger
		b.Settled.PendingDamage = resume.Damage
		if resume.Advance {
			return e.advanceSettledSegment(b)
		}
		return nil, nil
	}
	work := v.Queue[0]
	v.Queue = v.Queue[1:]
	v.Active = &work
	b.Settled.PendingDamage = nil
	b.Settled.TriggerBatch = nil
	if work.Kind == "damage" {
		source := newSettledDamageSource(b, work.SourceActorID, work.TargetActorID, work.SourceContentID, work.Damage)
		batch, err := e.buildDamageBatch(b, []state.SettledDamageSource{source})
		if err != nil {
			return nil, err
		}
		b.Settled.PendingDamage = batch
		openSettledWindow(b, "card-damage", stageOngoingDamage, "damage_response", []command.Type{command.TypeCommitInteraction, command.TypePass})
		return []event.Event{damageRevealEvent(b, batch)}, nil
	}
	if work.Kind == "provoke" {
		b.Settled.Sequence++
		batch := &state.SettledTriggerBatch{ID: fmt.Sprintf("provoke-%d", b.Settled.Sequence), Rolls: work.Rolls, Reactable: true}
		for i := range batch.Rolls {
			die := lib.Dice[batch.Rolls[i].Die.DieID]
			n, err := e.namedIntn(b, "status_effect_dice", die.SideCount)
			if err != nil {
				return nil, err
			}
			face := die.Faces[n]
			batch.Rolls[i].Die.Face = face.Number
			batch.Rolls[i].Die.Value = face.Number
			batch.Rolls[i].Die.Symbols = []string{face.Symbol}
			batch.Rolls[i].Resolved = true
		}
		b.Settled.TriggerBatch = batch
		openStatusEffectReveal(b, true)
		return []event.Event{settledEvent(event.TypeInteractionWindowOpened, b, "", map[string]any{"batch_id": batch.ID, "rolls": batch.Rolls})}, nil
	}
	if automaticVenomStatus(work) {
		return e.resolveVenomStatus(b, lib)
	}
	openSettledWindow(b, "venom-status", stageVenomStatus, "status_response", []command.Type{command.TypeCommitInteraction, command.TypePass})
	return []event.Event{settledEvent(event.TypeInteractionWindowOpened, b, work.SourceActorID, map[string]any{"venom_work": work})}, nil
}
func (e Engine) handleVenomStatus(b *state.Battle, lib content.BattleLibrary, cmd command.Command) ([]event.Event, error) {
	if cmd.Type != command.TypePass {
		var payload command.CommitInteractionPayload
		if err := command.DecodePayload(cmd, &payload); err != nil {
			return nil, err
		}
		if err := e.playSettledReactionCard(b, lib, cmd.ActorID, payload.Commitment); err != nil {
			return nil, err
		}
		advanceSettledReactionPriority(b, cmd.ActorID, true)
		return nil, nil
	}
	if advanceSettledReactionPriority(b, cmd.ActorID, false) {
		return nil, nil
	}
	closeSettledWindow(b)
	return e.resolveVenomStatus(b, lib)
}

func automaticVenomStatus(work state.VenomWork) bool {
	return work.Kind == "conversion" || (work.Kind == "application" && work.StatusID == "incubation")
}

func (e Engine) resolveVenomStatus(b *state.Battle, lib content.BattleLibrary) ([]event.Event, error) {
	v := venomRuntime(b)
	work := *v.Active
	incubationBefore := stacks(b, work.TargetActorID, "incubation")
	applicationBefore := stacks(b, work.TargetActorID, work.StatusID)
	poisonBefore := stacks(b, work.TargetActorID, "poison")
	volatileBefore := stacks(b, work.TargetActorID, "volatile_poison")
	if work.Kind == "application" {
		if (work.StatusID != "incubation" || stacks(b, work.TargetActorID, "incubation") == 0) && (!work.RequirePoison || stacks(b, work.TargetActorID, "poison") > 0) {
			applyVenomStatus(b, lib, work.SourceActorID, state.SettledStatusApplication{TargetActorID: work.TargetActorID, StatusID: work.StatusID, Stacks: work.Stacks})
		}
	} else if work.Kind == "conversion" {
		original := true
		if work.InstanceID != "" {
			actor, status := findStatusInstance(b, work.InstanceID)
			original = actor == work.TargetActorID && status == "incubation"
		}
		if original && stacks(b, work.TargetActorID, "poison") > 0 && stacks(b, work.TargetActorID, "volatile_poison") < 3 {
			removeStatus(b, work.TargetActorID, "poison", 1)
			applyStatus(b, lib, work.TargetActorID, "volatile_poison", 1)
			if work.Accelerant {
				rolls := captureToxins(b, lib, work.TargetActorID, []string{"volatile_poison"}, 1, false)
				if len(rolls) > 0 {
					v.Queue = append(v.Queue, state.VenomWork{Kind: "provoke", Rolls: rolls})
				}
			}
		}
		if original && work.InstanceID != "" {
			removeStatus(b, work.TargetActorID, "incubation", 1)
		}
	}
	data := map[string]any{"venom_work": work}
	if work.Kind == "application" && work.StatusID != "incubation" {
		data["status_application"] = map[string]any{"target_actor_id": work.TargetActorID, "status_id": work.StatusID, "before": applicationBefore, "after": stacks(b, work.TargetActorID, work.StatusID)}
	}
	if work.Kind == "application" && work.StatusID == "incubation" {
		data["incubation_application"] = map[string]any{"target_actor_id": work.TargetActorID, "before": incubationBefore, "after": stacks(b, work.TargetActorID, "incubation")}
	}
	if work.Kind == "conversion" {
		poisonAfter := stacks(b, work.TargetActorID, "poison")
		volatileAfter := stacks(b, work.TargetActorID, "volatile_poison")
		data["poison_conversion"] = map[string]any{
			"target_actor_id": work.TargetActorID,
			"poison_before":   poisonBefore, "poison_after": poisonAfter,
			"volatile_before": volatileBefore, "volatile_after": volatileAfter,
			"converted": poisonAfter == poisonBefore-1 && volatileAfter == volatileBefore+1,
		}
	}
	events := []event.Event{settledEvent(event.TypeProposalBatchCommitted, b, work.SourceActorID, data)}
	next, err := e.nextVenomWork(b, lib)
	return append(events, next...), err
}

func captureToxins(b *state.Battle, lib content.BattleLibrary, target string, choices []string, limit int, reserve bool) []state.SettledEffectRoll {
	v := venomRuntime(b)
	if reserve {
		limit = min(limit, 2-v.Provoked[target])
	}
	counts := map[string]int{}
	var rolls []state.SettledEffectRoll
	for _, id := range choices {
		if len(rolls) >= limit {
			break
		}
		if !toxin(id) || counts[id] >= stacks(b, target, id) {
			continue
		}
		counts[id]++
		def := lib.Statuses[id]
		if len(def.Triggers) == 0 {
			continue
		}
		trigger := def.Triggers[0]
		instance := ""
		for _, s := range b.Actors[target].Statuses {
			if s.DefinitionID == id {
				instance = s.InstanceID
			}
		}
		rolls = append(rolls, state.SettledEffectRoll{ActorID: target, SourceContentType: "status", SourceContentID: id, StatusID: id, StatusInstanceID: instance, TriggerID: trigger.ID, OperationID: trigger.Operations[0].ID, OperationIndex: 0, Die: state.RolledDie{Index: len(rolls), DieID: "standard_d6"}})
	}
	if reserve {
		v.Provoked[target] += len(rolls)
	}
	return rolls
}
func toxinChoices(b *state.Battle, target string, n int) [][]string {
	n = min(n, 2-venomRuntime(b).Provoked[target])
	var result [][]string
	for p := 0; p <= min(n, stacks(b, target, "poison")); p++ {
		for v := 0; v <= min(n-p, stacks(b, target, "volatile_poison")); v++ {
			if p+v == 0 {
				continue
			}
			var row []string
			for i := 0; i < v; i++ {
				row = append(row, "volatile_poison")
			}
			for i := 0; i < p; i++ {
				row = append(row, "poison")
			}
			result = append(result, row)
		}
	}
	return result
}
func defaultToxins(b *state.Battle, target string, n int) []string {
	options := toxinChoices(b, target, n)
	var best []string
	for _, o := range options {
		if len(o) > len(best) {
			best = o
		}
	}
	return best
}

// This check runs after ordinary roll reactions, once per holder per batch.
// A retry is revealed in the same batch and cannot create another spend.
func (e Engine) automaticCatalyst(b *state.Battle, lib content.BattleLibrary) ([]event.Event, bool, error) {
	if !hasVenomMechanics(b) {
		return nil, false, nil
	}
	batch := b.Settled.TriggerBatch
	v := venomRuntime(b)
	var events []event.Event
	for _, holder := range sortedSettledActorIDs(b) {
		key := batch.ID + ":" + holder
		if v.CatalystUsed[key] {
			continue
		}
		v.CatalystUsed[key] = true
		if stacks(b, holder, "catalyst") == 0 {
			continue
		}
		chosen := -1
		for _, status := range []string{"volatile_poison", "poison"} {
			for i, roll := range batch.Rolls {
				// Captured toxin rolls still resolve after cleansing. Catalyst must
				// inspect those rolls too, rather than the enemy's current stacks.
				if roll.ActorID == holder || roll.SourceContentID != status || roll.Rerolled {
					continue
				}
				if (status == "volatile_poison" && roll.Die.Face == 6) || (status == "poison" && roll.Die.Face >= 5) {
					chosen = i
					break
				}
			}
			if chosen >= 0 {
				break
			}
		}
		if chosen < 0 {
			continue
		}
		removeStatus(b, holder, "catalyst", 1)
		roll := &batch.Rolls[chosen]
		previousFace := roll.Die.Face
		die := lib.Dice[roll.Die.DieID]
		n, err := e.namedIntn(b, "status_effect_dice", die.SideCount)
		if err != nil {
			return nil, false, err
		}
		face := die.Faces[n]
		roll.Die.Face = face.Number
		roll.Die.Value = face.Number
		roll.Die.Symbols = []string{face.Symbol}
		roll.Rerolled = true
		events = append(events, settledEvent(event.TypeDiceRolled, b, roll.ActorID, map[string]any{
			"source_type": "catalyst", "holder": holder, "forced": true,
			"die_index": chosen, "face_before": previousFace, "status_id": roll.SourceContentID,
			"rolls": []state.SettledEffectRoll{*roll},
		}))
	}
	if len(events) > 0 {
		openStatusEffectReveal(b, true)
		return events, true, nil
	}
	return nil, false, nil
}

func (e Engine) prepareVenomAttacks(b *state.Battle, lib content.BattleLibrary) {
	if !hasVenomMechanics(b) {
		return
	}
	v := venomRuntime(b)
	for _, actor := range sortedSettledActorIDs(b) {
		runtime := b.Settled.Actors[actor]
		id := runtime.SelectedAbilityID
		if id != "terminal_bite" && id != "fever_spike" {
			continue
		}
		key := "attack:" + actor
		if v.Used[key] {
			continue
		}
		v.Used[key] = true
		var source *state.SettledDamageSource
		for i := range b.Settled.OffensiveSources {
			if b.Settled.OffensiveSources[i].SourceActorID == actor && b.Settled.OffensiveSources[i].SourceContentID == id {
				source = &b.Settled.OffensiveSources[i]
				break
			}
		}
		if source == nil {
			continue
		}
		n := 1
		tier, _ := qualifiedTier(lib.Abilities[id], runtime.FinalDice)
		if id == "terminal_bite" || tier.ID == "greater" {
			n = 2
		}
		choices := runtime.SelectedToxins
		if len(choices) == 0 {
			choices = defaultToxins(b, source.TargetActorID, n)
		}
		rolls := captureToxins(b, lib, source.TargetActorID, choices, n, true)
		v.AttackChecks[source.ID] = rolls
		if id == "terminal_bite" {
			applyStatus(b, lib, actor, "catalyst", 1)
			if v.Used["formula:"+actor] {
				source.BaseAmount += len(rolls)
			}
		}
	}
}
func venomChoiceKey(actor, choice string) string { return actor + ":" + choice }
func parseDieChoice(choice string) (string, int, int) {
	parts := strings.Split(choice, ":")
	if len(parts) != 3 {
		return "", -1, 0
	}
	i, _ := strconv.Atoi(parts[1])
	f, _ := strconv.Atoi(parts[2])
	return parts[0], i, f
}

// Credit prevention in response order using the final source amount. A later
// scale-to-zero or canceled source cannot award Catalyst for damage not prevented.
func commitMoltRewards(b *state.Battle, lib content.BattleLibrary, batch *state.SettledDamageBatch) {
	v := venomRuntime(b)
	for _, reward := range v.MoltRewards {
		for _, source := range batch.Sources {
			if source.ID != reward.SourceID {
				continue
			}
			source.ReactionPrevention = reward.PriorPrevention
			if settledSourceAmount(source) > 0 {
				applyStatus(b, lib, reward.ActorID, "catalyst", 1)
			}
		}
	}
	v.MoltRewards = nil
}
