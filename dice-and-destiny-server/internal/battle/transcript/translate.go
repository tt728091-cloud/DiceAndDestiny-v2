package transcript

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"diceanddestiny/server/internal/battle/event"
	"diceanddestiny/server/internal/battle/state"
	"diceanddestiny/server/internal/content"
)

func eventDrafts(authorityEvent event.Event, transition Transition, library content.BattleLibrary) []draft {
	battle := transition.After
	base := baseDraft(transition.Command.BattleID, transition.Battle, battle)
	base.Kind = string(authorityEvent.Type)
	base.ActorID = authorityEvent.ActorID
	base.TargetActorID = authorityEvent.TargetActorID
	base.Controller = controllerForActor(transition.Battle, battle, authorityEvent.ActorID)
	base.SourceType = string(authorityEvent.SourceType)
	base.SourceID = authorityEvent.SourceID
	base.ProposalID = authorityEvent.ProposalID
	base.WindowID = firstNonEmpty(authorityEvent.WindowID, base.WindowID)
	base.PendingInputID = firstNonEmpty(authorityEvent.PendingInputID, base.PendingInputID)
	if authorityEvent.Round != 0 {
		base.Round = authorityEvent.Round
	}
	if authorityEvent.Segment != "" {
		base.Segment = string(authorityEvent.Segment)
	}
	raw := jsonMap(authorityEvent)
	base.Details = raw

	switch authorityEvent.Type {
	case event.TypeSegmentEntered:
		base.Visibility = VisibilityPublic
		base.Summary = fmt.Sprintf("Round %d entered %s", base.Round, humanize(base.Segment))
		return []draft{{Record: base}}
	case event.TypeSegmentAdvanced:
		base.Visibility = VisibilityPublic
		base.Summary = fmt.Sprintf("Authority advanced %s to %s", humanize(string(authorityEvent.From)), humanize(string(authorityEvent.To)))
		return []draft{{Record: base}}
	case event.TypeCardsDrawn:
		base.Visibility = privateVisibility(authorityEvent.ActorID, transition.Battle, battle)
		base.Summary = fmt.Sprintf("%s drew %d card(s)", actorName(battle, authorityEvent.ActorID), maxInt(len(authorityEvent.Cards), authorityEvent.Count))
		return []draft{{Record: base}}
	case event.TypeDiscardReshuffled:
		base.Visibility = VisibilityPublic
		base.Summary = fmt.Sprintf("%s reshuffled %d discarded card(s)", actorName(battle, authorityEvent.ActorID), authorityEvent.Count)
		return []draft{{Record: base}}
	case event.TypeEnergyPointsGained:
		base.Visibility = VisibilityPublic
		base.Kind = "resource_changed"
		base.Summary = fmt.Sprintf("%s now has %d energy", actorName(battle, authorityEvent.ActorID), authorityEvent.EnergyPoints)
		base.Details["resource"] = "energy"
		base.Details["value_after"] = authorityEvent.EnergyPoints
		return []draft{{Record: base}}
	case event.TypeRollRequested:
		base.Visibility = privateVisibility(authorityEvent.ActorID, transition.Battle, battle)
		base.Kind = "die_requested"
		base.Summary = fmt.Sprintf("%s was asked to roll %s dice", actorName(battle, authorityEvent.ActorID), humanize(string(authorityEvent.Pool)))
		return []draft{{Record: base}}
	case event.TypeDiceRolled:
		rolls := rolledDiceFromEvent(authorityEvent, transition.After)
		if len(rolls) == 0 {
			base.Visibility = VisibilityDebugSystem
			base.Summary = fmt.Sprintf("%s completed a hidden roll checkpoint", actorName(battle, authorityEvent.ActorID))
			return []draft{{Record: base}}
		}
		base.Kind = "die_rolled"
		base.Visibility = privateVisibility(authorityEvent.ActorID, transition.Battle, battle)
		base.Summary = fmt.Sprintf("%s rolled %s", actorName(battle, authorityEvent.ActorID), faceList(rolls))
		base.Details["dice"] = rolls
		key := diePrivateKey(authorityEvent.ActorID, base.Segment, eventSourceID(authorityEvent))
		if base.Segment == "defensive" {
			key = "defense_die:" + authorityEvent.ActorID
		}
		return []draft{{Record: base, privateKey: key}}
	case event.TypeInteractionWindowOpened:
		rollGroups := effectRollGroups(authorityEvent.Data["rolls"])
		if len(rollGroups) == 0 {
			if battle != nil && battle.Settled != nil && battle.Settled.TriggerBatch != nil && base.Segment == "ongoing_effects" {
				batch := battle.Settled.TriggerBatch
				base.Kind = "status_trigger_batch_opened"
				base.Visibility = VisibilityDebugSystem
				base.SourceID = batch.ID
				base.Summary = fmt.Sprintf("Authority opened %d status roll(s) for %d triggered status instance(s)", len(batch.Rolls), len(batch.StatusInstanceIDs))
				base.Details = map[string]any{"batch_id": batch.ID, "status_instance_ids": batch.StatusInstanceIDs, "roll_count": len(batch.Rolls), "reactable": batch.Reactable}
				return []draft{{Record: base}}
			}
			base.Visibility = VisibilityDebugSystem
			base.Summary = fmt.Sprintf("Authority opened %s window", humanize(string(authorityEvent.Purpose)))
			return []draft{{Record: base}}
		}
		var result []draft
		for _, group := range rollGroups {
			actorID := stringValue(group[0], "actor_id")
			sourceID := firstNonEmpty(stringValue(group[0], "source_content_id"), stringValue(group[0], "status_id"))
			faces := effectRollFaces(group)
			key := diePrivateKey(actorID, base.Segment, sourceID)
			rolled := base
			rolled.Kind = "die_rolled"
			rolled.ActorID = actorID
			rolled.Controller = controllerForActor(transition.Battle, battle, actorID)
			rolled.SourceType = firstNonEmpty(stringValue(group[0], "source_content_type"), "status")
			rolled.SourceID = sourceID
			rolled.StatusID = sourceID
			rolled.Visibility = privateVisibility(actorID, transition.Battle, battle)
			rolled.Summary = fmt.Sprintf("%s rolled %s for %s", actorName(battle, actorID), intList(faces), contentName(library, rolled.SourceType, sourceID))
			rolled.Details = map[string]any{"rolls": group, "faces": faces}
			result = append(result, draft{Record: rolled, privateKey: key})

			revealed := rolled
			revealed.Kind = "die_revealed"
			revealed.Visibility = VisibilityPublic
			revealed.Summary = fmt.Sprintf("%s dice revealed for %s: %s", contentName(library, rolled.SourceType, sourceID), actorName(battle, actorID), intList(faces))
			result = append(result, draft{Record: revealed, revealKeys: []string{key}})
		}
		return result
	case event.TypeInteractionCommitted:
		base.Visibility = privateVisibility(authorityEvent.ActorID, transition.Battle, battle)
		base.Summary = fmt.Sprintf("%s committed a hidden %s choice", actorName(battle, authorityEvent.ActorID), humanize(string(authorityEvent.Purpose)))
		return []draft{{Record: base, privateKey: "interaction:" + authorityEvent.ActorID + ":" + authorityEvent.WindowID}}
	case event.TypeInteractionRevealed:
		base.Visibility = VisibilityPublic
		base.Summary = "Authority revealed the simultaneous choices"
		keys := revealKeysForInteraction(authorityEvent, transition.Before, transition.After)
		return []draft{{Record: base, revealKeys: keys}}
	case event.TypeCardPlayed:
		cardInstanceID := stringValue(authorityEvent.Data, "card_instance_id")
		cardDefinitionID := firstNonEmpty(stringValue(authorityEvent.Data, "card_definition_id"), cardDefinitionForInstance(transition.Before, transition.After, authorityEvent.ActorID, cardInstanceID))
		base.CardInstanceID = cardInstanceID
		base.CardDefinitionID = cardDefinitionID
		base.StatusID = stringValue(authorityEvent.Data, "choice_id")
		base.AbilityID = stringValue(authorityEvent.Data, "ability_id")
		base.Visibility = VisibilityPublic
		base.Summary = fmt.Sprintf("%s played %s", actorName(battle, authorityEvent.ActorID), contentName(library, "card", cardDefinitionID))
		if base.StatusID != "" {
			base.Summary += fmt.Sprintf(" targeting %s", contentName(library, "status", base.StatusID))
		}
		enrichCardDetails(base.Details, library, cardDefinitionID)
		return []draft{{Record: base, revealKeys: []string{"card:" + authorityEvent.ActorID + ":" + cardInstanceID}}}
	case event.TypeAbilitySelected:
		base.AbilityID = stringValue(authorityEvent.Data, "ability_id")
		base.Visibility = privateVisibility(authorityEvent.ActorID, transition.Battle, battle)
		base.Summary = fmt.Sprintf("%s selected %s", actorName(battle, authorityEvent.ActorID), contentName(library, "ability", base.AbilityID))
		return []draft{{Record: base, privateKey: "ability:" + authorityEvent.ActorID}}
	case event.TypeDefenseSelected:
		base.Kind = "defense_applied"
		base.AbilityID = stringValue(authorityEvent.Data, "ability_id")
		base.SourceID = stringValue(authorityEvent.Data, "source_id")
		base.Visibility = VisibilityPublic
		base.Summary = fmt.Sprintf("%s revealed %s with face %d", actorName(battle, authorityEvent.ActorID), contentName(library, "ability", base.AbilityID), intValue(authorityEvent.Data, "rolled_face"))
		return []draft{{Record: base, revealKeys: []string{"defense_choice:" + authorityEvent.ActorID, "defense_die:" + authorityEvent.ActorID}}}
	case event.TypeProposalBatchCommitted, event.TypeProposalBatchRevealed:
		base.Visibility = VisibilityPublic
		base.Summary = "Authority evaluated and committed the proposal batch"
		return append(statusOutcomeDrafts(base, authorityEvent.Data, transition, library), draft{Record: base})
	case event.TypeDamageProposed:
		base.Visibility = VisibilityPublic
		base.Kind = "damage_proposed"
		base.Summary = fmt.Sprintf("%s proposed %d damage to %s from %s", actorName(battle, authorityEvent.ActorID), authorityEvent.Amount, actorName(battle, authorityEvent.TargetActorID), contentName(library, "ability", authorityEvent.SourceID))
		return []draft{{Record: base}}
	case event.TypeDamageCardsRevealed:
		base.Visibility = VisibilityPublic
		base.Summary = "Authority revealed the cards selected for incoming damage"
		return append([]draft{{Record: base}}, damageSourceDrafts(base, authorityEvent.Data, battle, library, false)...)
	case event.TypeDamageModified:
		base.Visibility = VisibilityPublic
		base.Summary = fmt.Sprintf("%s modified incoming damage", actorName(battle, authorityEvent.ActorID))
		return []draft{{Record: base}}
	case event.TypeDamageCommitted:
		base.Visibility = VisibilityPublic
		base.Summary = "Authority committed the final damage batch"
		return append(damageSourceDrafts(base, authorityEvent.Data, battle, library, true), draft{Record: base})
	case event.TypeCardsPermanentlyRemoved:
		base.Visibility = VisibilityPublic
		base.Summary = fmt.Sprintf("%s permanently lost %d card(s) to damage", actorName(battle, authorityEvent.TargetActorID), len(authorityEvent.Cards))
		return []draft{{Record: base}}
	case event.TypeStatusChanged:
		base.Visibility = VisibilityPublic
		base.StatusID = stringValue(authorityEvent.Data, "status_id")
		base.Summary = fmt.Sprintf("%s changed on %s", contentName(library, "status", base.StatusID), actorName(battle, authorityEvent.TargetActorID))
		return []draft{{Record: base}}
	case event.TypeBattleCompleted:
		base.Visibility = VisibilityPublic
		base.Summary = battleCompletedSummary(battle)
		return []draft{{Record: base}}
	default:
		base.Visibility = publicOrSystemEventVisibility(authorityEvent.Type)
		base.Summary = humanize(string(authorityEvent.Type))
		return []draft{{Record: base}}
	}
}

func defenseRevealDrafts(transition Transition, library content.BattleLibrary) []draft {
	if transition.Before == nil || transition.After == nil || transition.Before.Settled == nil || transition.After.Settled == nil {
		return nil
	}
	if transition.Before.Settled.Stage != "defense_roll" || transition.After.Settled.Stage != "defense_reaction" {
		return nil
	}
	var result []draft
	for _, actorID := range sortedActorIDs(transition.After) {
		selection, exists := transition.After.Settled.DefenseSelections[actorID]
		if !exists {
			continue
		}
		record := baseDraft(transition.Command.BattleID, transition.Battle, transition.After)
		record.Kind = "defense_selected"
		record.ActorID = actorID
		record.Controller = controllerForActor(transition.Battle, transition.After, actorID)
		record.AbilityID = selection.AbilityID
		record.SourceID = selection.SourceID
		record.Visibility = VisibilityPublic
		record.Summary = fmt.Sprintf("%s revealed %s with face %d", actorName(transition.After, actorID), contentName(library, "ability", selection.AbilityID), selection.RolledFace)
		record.Details = map[string]any{"ability_id": selection.AbilityID, "source_id": selection.SourceID, "rolled_face": selection.RolledFace, "simultaneous_reveal": true}
		result = append(result, draft{Record: record, revealKeys: []string{"defense_choice:" + actorID, "defense_die:" + actorID}})
	}
	return result
}

func mutationDrafts(transition Transition, library content.BattleLibrary) []draft {
	if transition.Before == nil || transition.After == nil {
		return nil
	}
	var drafts []draft
	for _, actorID := range sortedActorIDs(transition.After) {
		beforeActor, beforeExists := transition.Before.Actors[actorID]
		afterActor := transition.After.Actors[actorID]
		if !beforeExists {
			continue
		}
		base := baseDraft(transition.Command.BattleID, transition.Battle, transition.After)
		base.ActorID = actorID
		base.Controller = controllerForActor(transition.Battle, transition.After, actorID)

		if beforeActor.Resources.EnergyPoints != afterActor.Resources.EnergyPoints && !hasEnergyEvent(transition.Events, actorID) {
			resource := base
			resource.Kind = "resource_changed"
			resource.Visibility = privateVisibility(actorID, transition.Battle, transition.After)
			if hasPublicActorEvent(transition.Events, actorID) {
				resource.Visibility = VisibilityPublic
			}
			resource.Summary = fmt.Sprintf("%s energy changed %d → %d", actorName(transition.After, actorID), beforeActor.Resources.EnergyPoints, afterActor.Resources.EnergyPoints)
			resource.Details = map[string]any{"resource": "energy", "value_before": beforeActor.Resources.EnergyPoints, "value_after": afterActor.Resources.EnergyPoints, "delta": afterActor.Resources.EnergyPoints - beforeActor.Resources.EnergyPoints}
			drafts = append(drafts, draft{Record: resource})
		}

		beforeHealth, afterHealth := beforeActor.CurrentHealth(), afterActor.CurrentHealth()
		if beforeHealth != afterHealth {
			health := base
			health.Kind = "health_changed"
			health.Visibility = VisibilityPublic
			health.Summary = fmt.Sprintf("%s health changed %d → %d", actorName(transition.After, actorID), beforeHealth, afterHealth)
			health.Details = map[string]any{"value_before": beforeHealth, "value_after": afterHealth, "delta": afterHealth - beforeHealth, "maximum": afterActor.Health.MaxHealth}
			drafts = append(drafts, draft{Record: health})
		}

		beforeStatuses := statusStacks(beforeActor.Statuses)
		afterStatuses := statusStacks(afterActor.Statuses)
		for _, statusID := range unionKeys(beforeStatuses, afterStatuses) {
			if beforeStatuses[statusID] == afterStatuses[statusID] {
				continue
			}
			changed := base
			changed.Kind = statusMutationKind(beforeStatuses[statusID], afterStatuses[statusID])
			changed.TargetActorID = actorID
			changed.StatusID = statusID
			changed.Visibility = VisibilityPublic
			changed.Summary = statusMutationSummary(actorName(transition.After, actorID), contentName(library, "status", statusID), beforeStatuses[statusID], afterStatuses[statusID])
			changed.Details = map[string]any{"status_id": statusID, "stacks_before": beforeStatuses[statusID], "stacks_after": afterStatuses[statusID], "stacks_changed": afterStatuses[statusID] - beforeStatuses[statusID], "operation": statusOperation(beforeStatuses[statusID], afterStatuses[statusID])}
			drafts = append(drafts, draft{Record: changed})
		}

		drafts = append(drafts, cardZoneMutationDrafts(base, actorID, beforeActor, afterActor, transition, library)...)
		drafts = append(drafts, abilityAndDieMutationDrafts(base, actorID, transition, library)...)
		drafts = append(drafts, defenseSelectionMutationDrafts(base, actorID, transition, library)...)
	}
	return drafts
}

func automaticPlanningDrafts(transition Transition, library content.BattleLibrary) []draft {
	if transition.After == nil || transition.After.Settled == nil {
		return nil
	}
	var result []draft
	for _, actorID := range sortedActorIDs(transition.After) {
		actor := transition.After.Actors[actorID]
		runtime := transition.After.Settled.Actors[actorID]
		if actor.Controller != state.ControllerAI || !runtime.PlanningCommitted {
			continue
		}
		alreadyPresent := false
		if transition.Before != nil && transition.Before.Settled != nil {
			alreadyPresent = transition.Before.Settled.Actors[actorID].PlanningCommitted
		}
		if alreadyPresent {
			continue
		}
		base := baseDraft(transition.Command.BattleID, transition.Battle, transition.After)
		base.ActorID = actorID
		base.Controller = "d100"
		if runtime.AID100 > 0 {
			d100 := base
			d100.Kind = "d100_rolled"
			d100.Visibility = VisibilityDebugSystem
			d100.Summary = fmt.Sprintf("%s rolled D100 %d and simulated %d planning roll(s)", actorName(transition.After, actorID), runtime.AID100, runtime.AISimulatedRolls)
			d100.Details = map[string]any{"d100": runtime.AID100, "simulated_rolls": runtime.AISimulatedRolls}
			result = append(result, draft{Record: d100})
		}
		if len(runtime.FinalDice) > 0 {
			rolled := base
			rolled.Kind = "die_rolled"
			rolled.Visibility = VisibilityPrivateOpponent
			rolled.Summary = fmt.Sprintf("%s rolled offensive dice %s", actorName(transition.After, actorID), faceList(runtime.FinalDice))
			rolled.Details = map[string]any{"dice": runtime.FinalDice, "rolls_used": runtime.RollsUsed, "simulated_rolls": runtime.AISimulatedRolls}
			result = append(result, draft{Record: rolled, privateKey: diePrivateKey(actorID, "offensive", "")})
		}
		if runtime.SelectedAbilityID != "" {
			selected := base
			selected.Kind = "ability_selected"
			selected.AbilityID = runtime.SelectedAbilityID
			selected.Visibility = VisibilityPrivateOpponent
			selected.Summary = fmt.Sprintf("%s selected %s", actorName(transition.After, actorID), contentName(library, "ability", runtime.SelectedAbilityID))
			selected.Details = map[string]any{"ability_id": runtime.SelectedAbilityID, "tier_id": runtime.SelectedTierID, "target_ids": runtime.SelectedTargetIDs, "qualified_abilities": runtime.QualifiedAbilityIDs}
			result = append(result, draft{Record: selected, privateKey: "ability:" + actorID})
		}
	}
	return result
}

func defenseSelectionMutationDrafts(base Record, actorID string, transition Transition, library content.BattleLibrary) []draft {
	if transition.Before == nil || transition.After == nil || transition.Before.Settled == nil || transition.After.Settled == nil {
		return nil
	}
	before, existedBefore := transition.Before.Settled.DefenseSelections[actorID]
	after, existsAfter := transition.After.Settled.DefenseSelections[actorID]
	if !existsAfter || (existedBefore && before.AbilityID == after.AbilityID && before.SourceID == after.SourceID) {
		return nil
	}
	record := base
	record.Kind = "defense_selected_private"
	record.AbilityID = after.AbilityID
	record.SourceID = after.SourceID
	record.Visibility = privateVisibility(actorID, transition.Battle, transition.After)
	record.Summary = fmt.Sprintf("%s secretly selected %s", actorName(transition.After, actorID), contentName(library, "ability", after.AbilityID))
	record.Details = map[string]any{"ability_id": after.AbilityID, "source_id": after.SourceID}
	return []draft{{Record: record, privateKey: "defense_choice:" + actorID}}
}

func statusOutcomeDrafts(base Record, data map[string]any, transition Transition, library content.BattleLibrary) []draft {
	var rolls []state.SettledEffectRoll
	decodeValue(data["rolls"], &rolls)
	var result []draft
	for _, roll := range rolls {
		definition, exists := library.Statuses[roll.SourceContentID]
		if !exists {
			continue
		}
		var operation content.BattleOperation
		for _, trigger := range definition.Triggers {
			if trigger.ID != roll.TriggerID || roll.OperationIndex < 0 || roll.OperationIndex >= len(trigger.Operations) {
				continue
			}
			rolledOperation := trigger.Operations[roll.OperationIndex]
			for _, outcome := range rolledOperation.Outcomes {
				if containsIntValue(outcome.Faces, roll.Die.Face) && len(outcome.Operations) > 0 {
					operation = outcome.Operations[0]
					break
				}
			}
		}
		redundant := false
		if strings.HasPrefix(operation.Type, "remove_status") && transition.Before != nil {
			redundant = statusStacks(transition.Before.Actors[roll.ActorID].Statuses)[roll.SourceContentID] == 0
		}
		record := base
		record.Kind = "status_outcome_evaluated"
		record.ActorID = roll.ActorID
		record.Controller = controllerForActor(transition.Battle, transition.After, roll.ActorID)
		record.SourceType = "status"
		record.SourceID = roll.SourceContentID
		record.StatusID = roll.SourceContentID
		record.Visibility = VisibilityDebugSystem
		record.Summary = fmt.Sprintf("%s face %d evaluated %s", contentName(library, "status", roll.SourceContentID), roll.Die.Face, humanize(operation.Type))
		if redundant {
			record.Summary += " (redundant because the status was already absent)"
		}
		record.Details = map[string]any{"roll": roll, "operation": operation, "redundant": redundant, "batch_id": stringValue(data, "batch_id")}
		result = append(result, draft{Record: record})
	}
	return result
}

func pendingInputDrafts(battle *state.Battle, context BattleContext) []draft {
	if battle == nil {
		return nil
	}
	actorIDs := make([]string, 0, len(battle.Flow.PendingInput))
	for actorID := range battle.Flow.PendingInput {
		actorIDs = append(actorIDs, actorID)
	}
	sort.Strings(actorIDs)
	result := make([]draft, 0, len(actorIDs))
	for _, actorID := range actorIDs {
		pending := battle.Flow.PendingInput[actorID]
		record := baseDraft(battle.ID, context, battle)
		record.Kind = "decision_offered"
		record.ActorID = actorID
		record.Controller = controllerForActor(context, battle, actorID)
		record.PendingInputID = pending.ID
		record.WindowID = pending.WindowID
		record.Visibility = VisibilityDebugSystem
		record.Summary = fmt.Sprintf("Authority offered %s a %s decision", actorName(battle, actorID), humanize(pending.Stage))
		record.Details = jsonMap(pending)
		result = append(result, draft{Record: record})
	}
	return result
}

func damageSourceDrafts(base Record, data map[string]any, battle *state.Battle, library content.BattleLibrary, committed bool) []draft {
	var sources []state.SettledDamageSource
	decodeValue(data["sources"], &sources)
	result := make([]draft, 0, len(sources))
	for _, source := range sources {
		record := base
		record.Kind = "damage_calculated"
		record.ActorID = source.SourceActorID
		record.TargetActorID = source.TargetActorID
		record.Controller = controllerForActor(BattleContext{HumanActorID: base.HumanActorID, ModelActorID: base.ModelActorID}, battle, source.SourceActorID)
		record.SourceID = source.SourceContentID
		record.ProposalID = source.ID
		record.Visibility = VisibilityPublic
		record.Summary = fmt.Sprintf("%s damage to %s: %d base - %d defense - %d reaction = %d final", contentName(library, "ability", source.SourceContentID), actorName(battle, source.TargetActorID), source.BaseAmount, source.Prevention, source.ReactionPrevention, source.FinalAmount)
		record.Details = jsonMap(source)
		record.Details["base_amount"] = source.BaseAmount
		record.Details["prevention"] = source.Prevention
		record.Details["reaction_prevention"] = source.ReactionPrevention
		record.Details["final_amount"] = source.FinalAmount
		record.Details["committed"] = committed
		result = append(result, draft{Record: record})
	}
	return result
}

func cardZoneMutationDrafts(base Record, actorID string, before, after state.ActorState, transition Transition, library content.BattleLibrary) []draft {
	beforeZones, afterZones := cardZones(before), cardZones(after)
	var result []draft
	for _, instanceID := range unionKeys(beforeZones, afterZones) {
		from, to := beforeZones[instanceID], afterZones[instanceID]
		if from == to {
			continue
		}
		definitionID := cardDefinitionForInstance(transition.Before, transition.After, actorID, instanceID)
		record := base
		record.Kind = "card_zone_changed"
		record.CardInstanceID = instanceID
		record.CardDefinitionID = definitionID
		record.Visibility = privateVisibility(actorID, transition.Battle, transition.After)
		if to == "removed" || cardPlayedPublicly(transition.Events, actorID, instanceID) {
			record.Visibility = VisibilityPublic
		}
		record.Summary = fmt.Sprintf("%s's %s moved %s → %s", actorName(transition.After, actorID), contentName(library, "card", definitionID), humanize(from), humanize(to))
		record.Details = map[string]any{"zone_before": from, "zone_after": to, "card_instance_id": instanceID, "card_definition_id": definitionID}
		key := ""
		if from == "hand" && to != "hand" && record.Visibility != VisibilityPublic {
			key = "card:" + actorID + ":" + instanceID
		}
		result = append(result, draft{Record: record, privateKey: key})
	}
	return result
}

func abilityAndDieMutationDrafts(base Record, actorID string, transition Transition, library content.BattleLibrary) []draft {
	if transition.Before.Settled == nil || transition.After.Settled == nil {
		return nil
	}
	before, beforeOK := transition.Before.Settled.Actors[actorID]
	after, afterOK := transition.After.Settled.Actors[actorID]
	if !beforeOK || !afterOK {
		return nil
	}
	var result []draft
	if before.SelectedAbilityID != after.SelectedAbilityID {
		record := base
		record.Kind = "ability_changed"
		record.AbilityID = after.SelectedAbilityID
		record.Visibility = privateVisibility(actorID, transition.Battle, transition.After)
		if hasTipStyleEvent(transition.Events, actorID) || hasInteractionReveal(transition.Events) {
			record.Visibility = VisibilityPublic
		}
		record.Summary = fmt.Sprintf("%s ability changed %s → %s", actorName(transition.After, actorID), contentName(library, "ability", before.SelectedAbilityID), contentName(library, "ability", after.SelectedAbilityID))
		record.Details = map[string]any{"ability_before": before.SelectedAbilityID, "ability_after": after.SelectedAbilityID, "qualified_after": after.QualifiedAbilityIDs}
		result = append(result, draft{Record: record, privateKey: "ability:" + actorID})
	}
	for _, authorityEvent := range transition.Events {
		if authorityEvent.Type != event.TypeCardPlayed || stringValue(authorityEvent.Data, "actor_id") != actorID || !eventDataHas(authorityEvent, "die_index") {
			continue
		}
		index := intValue(authorityEvent.Data, "die_index")
		if index < 0 || index >= len(before.FinalDice) || index >= len(after.FinalDice) || before.FinalDice[index].Face == after.FinalDice[index].Face {
			continue
		}
		record := base
		record.Kind = "die_modified"
		record.Visibility = VisibilityPublic
		record.SourceType = "card"
		record.CardDefinitionID = stringValue(authorityEvent.Data, "card_definition_id")
		record.Summary = fmt.Sprintf("%s changed %s's die %d from face %d to %d", contentName(library, "card", record.CardDefinitionID), actorName(transition.After, actorID), index+1, before.FinalDice[index].Face, after.FinalDice[index].Face)
		record.Details = map[string]any{"die_index": index, "face_before": before.FinalDice[index].Face, "face_after": after.FinalDice[index].Face, "ability_before": before.SelectedAbilityID, "ability_after": after.SelectedAbilityID, "qualified_after": after.QualifiedAbilityIDs}
		result = append(result, draft{Record: record})
	}
	return result
}

func rolledDiceFromEvent(value event.Event, battle *state.Battle) []state.RolledDie {
	if len(value.Dice) > 0 {
		return value.Dice
	}
	var effectRolls []state.SettledEffectRoll
	decodeValue(value.Data["rolls"], &effectRolls)
	if len(effectRolls) > 0 {
		result := make([]state.RolledDie, 0, len(effectRolls))
		for _, roll := range effectRolls {
			result = append(result, roll.Die)
		}
		return result
	}
	if value.Data["hidden"] == true && battle != nil && battle.Settled != nil && battle.Settled.TriggerBatch != nil {
		var result []state.RolledDie
		for _, roll := range battle.Settled.TriggerBatch.Rolls {
			if roll.ActorID == value.ActorID && roll.Resolved && roll.Die.Face > 0 {
				result = append(result, roll.Die)
			}
		}
		return result
	}
	return nil
}

func effectRollGroups(value any) [][]map[string]any {
	items := mapSlice(value)
	grouped := map[string][]map[string]any{}
	var keys []string
	for _, item := range items {
		actorID := stringValue(item, "actor_id")
		sourceID := firstNonEmpty(stringValue(item, "source_content_id"), stringValue(item, "status_id"))
		key := actorID + "|" + sourceID
		if _, ok := grouped[key]; !ok {
			keys = append(keys, key)
		}
		grouped[key] = append(grouped[key], item)
	}
	sort.Strings(keys)
	result := make([][]map[string]any, 0, len(keys))
	for _, key := range keys {
		result = append(result, grouped[key])
	}
	return result
}

func effectRollFaces(group []map[string]any) []int {
	faces := make([]int, 0, len(group))
	for _, item := range group {
		faces = append(faces, intValue(mapValue(item, "die"), "face"))
	}
	return faces
}

func revealKeysForInteraction(value event.Event, before, after *state.Battle) []string {
	keys := []string{}
	commitments := mapValue(value.Data, "commitments")
	for actorID := range commitments {
		keys = append(keys, "interaction:"+actorID+":"+value.WindowID, "ability:"+actorID, diePrivateKey(actorID, "offensive", ""))
	}
	if len(keys) == 0 && after != nil {
		for _, actorID := range sortedActorIDs(after) {
			keys = append(keys, "ability:"+actorID, diePrivateKey(actorID, "offensive", ""))
		}
	}
	return keys
}

func diePrivateKey(actorID, segmentID, sourceID string) string {
	return "die:" + actorID + ":" + segmentID + ":" + sourceID
}

func eventSourceID(value event.Event) string {
	return firstNonEmpty(value.SourceID, stringValue(value.Data, "source_id"), stringValue(value.Data, "source_content_id"))
}

func publicOrSystemEventVisibility(kind event.Type) string {
	switch kind {
	case event.TypeEnemyPlanned, event.TypeResolutionCompleted:
		return VisibilityPublic
	default:
		return VisibilityDebugSystem
	}
}

func faceList(dice []state.RolledDie) string {
	faces := make([]int, len(dice))
	for index := range dice {
		faces[index] = dice[index].Face
	}
	return intList(faces)
}

func intList(values []int) string {
	parts := make([]string, len(values))
	for index, value := range values {
		parts[index] = fmt.Sprint(value)
	}
	return "[" + strings.Join(parts, ", ") + "]"
}

func statusStacks(statuses []state.StatusState) map[string]int {
	result := map[string]int{}
	for _, status := range statuses {
		result[status.DefinitionID] += status.Stacks
	}
	return result
}

func statusMutationKind(before, after int) string {
	switch {
	case before == 0:
		return "status_applied"
	case after == 0:
		return "status_removed"
	default:
		return "status_stacks_changed"
	}
}

func statusOperation(before, after int) string {
	if after > before {
		return "apply_status"
	}
	if after == 0 {
		return "remove_status"
	}
	return "remove_status_stack"
}

func statusMutationSummary(actor, status string, before, after int) string {
	switch {
	case before == 0:
		return fmt.Sprintf("%s gained %s ×%d (%d → %d)", actor, status, after, before, after)
	case after == 0:
		return fmt.Sprintf("%s lost %s ×%d (%d → %d)", actor, status, before, before, after)
	default:
		return fmt.Sprintf("%s %s stacks changed %d → %d", actor, status, before, after)
	}
}

func cardZones(actor state.ActorState) map[string]string {
	result := map[string]string{}
	for _, pair := range []struct {
		name  string
		cards []string
	}{{"deck", actor.Cards.Deck}, {"hand", actor.Cards.Hand}, {"discard", actor.Cards.Discard}, {"removed", actor.Cards.Removed}} {
		for _, card := range pair.cards {
			result[card] = pair.name
		}
	}
	return result
}

func cardDefinitionForInstance(before, after *state.Battle, actorID, instanceID string) string {
	for _, battle := range []*state.Battle{after, before} {
		if battle == nil || battle.Settled == nil {
			continue
		}
		if instance, ok := battle.Settled.Actors[actorID].CardInstances[instanceID]; ok {
			return instance.DefinitionID
		}
	}
	return ""
}

func enrichCardDetails(details map[string]any, library content.BattleLibrary, definitionID string) {
	definition, ok := library.Cards[definitionID]
	if !ok {
		return
	}
	details["card_name"] = definition.Name
	details["energy_cost"] = definition.Cost.Energy
	details["targeting"] = definition.Targeting
	details["operations"] = definition.Operations
	details["rules_text"] = definition.Presentation.RulesText
}

func hasEnergyEvent(events []event.Event, actorID string) bool {
	for _, value := range events {
		if value.Type == event.TypeEnergyPointsGained && value.ActorID == actorID {
			return true
		}
	}
	return false
}

func hasPublicActorEvent(events []event.Event, actorID string) bool {
	for _, value := range events {
		if value.ActorID == actorID && (value.Type == event.TypeCardPlayed || value.Type == event.TypeAbilitySelected || value.Type == event.TypeDefenseSelected) {
			return true
		}
	}
	return false
}

func cardPlayedPublicly(events []event.Event, actorID, instanceID string) bool {
	for _, value := range events {
		if value.Type == event.TypeCardPlayed && value.ActorID == actorID && stringValue(value.Data, "card_instance_id") == instanceID {
			return true
		}
	}
	return false
}

func hasTipStyleEvent(events []event.Event, actorID string) bool {
	for _, value := range events {
		if value.Type == event.TypeCardPlayed && stringValue(value.Data, "actor_id") == actorID && eventDataHas(value, "die_index") {
			return true
		}
	}
	return false
}

func hasInteractionReveal(events []event.Event) bool {
	for _, value := range events {
		if value.Type == event.TypeInteractionRevealed {
			return true
		}
	}
	return false
}

func eventDataHas(value event.Event, key string) bool {
	_, ok := value.Data[key]
	return ok
}

func unionKeys[V any](left, right map[string]V) []string {
	seen := map[string]bool{}
	for key := range left {
		seen[key] = true
	}
	for key := range right {
		seen[key] = true
	}
	keys := make([]string, 0, len(seen))
	for key := range seen {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}

func jsonMap(value any) map[string]any {
	encoded, _ := json.Marshal(value)
	var result map[string]any
	_ = json.Unmarshal(encoded, &result)
	if result == nil {
		result = map[string]any{}
	}
	return result
}

func decodeValue(value any, target any) {
	encoded, _ := json.Marshal(value)
	_ = json.Unmarshal(encoded, target)
}

func mapSlice(value any) []map[string]any {
	var result []map[string]any
	decodeValue(value, &result)
	return result
}

func stringSlice(value any) []string {
	var result []string
	decodeValue(value, &result)
	return result
}

func mapValue(source map[string]any, key string) map[string]any {
	if value, ok := source[key].(map[string]any); ok {
		return value
	}
	return jsonMap(source[key])
}

func stringValue(source map[string]any, key string) string {
	if source == nil {
		return ""
	}
	value, ok := source[key]
	if !ok || value == nil {
		return ""
	}
	return fmt.Sprint(value)
}

func intValue(source map[string]any, key string) int {
	if source == nil {
		return 0
	}
	switch value := source[key].(type) {
	case int:
		return value
	case float64:
		return int(value)
	case json.Number:
		parsed, _ := value.Int64()
		return int(parsed)
	default:
		var parsed int
		_, _ = fmt.Sscan(fmt.Sprint(value), &parsed)
		return parsed
	}
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if value != "" && value != "<nil>" {
			return value
		}
	}
	return ""
}

func maxInt(left, right int) int {
	if left > right {
		return left
	}
	return right
}

func containsIntValue(values []int, target int) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
}
