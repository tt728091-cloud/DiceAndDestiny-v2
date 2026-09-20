package engine

import (
	"encoding/json"
	"sort"

	"diceanddestiny/server/internal/battle/command"
	"diceanddestiny/server/internal/battle/segment"
	"diceanddestiny/server/internal/battle/state"
	"diceanddestiny/server/internal/content"
)

// LegalActions returns complete, immediately submission-ready commands for
// the selected viewer's current pending input. It never exposes another
// actor's private pending input or planning state.
func (e Engine) LegalActions(battle *state.Battle, viewerActorID string) []command.Command {
	if battle == nil || battle.Settled == nil || state.IsTerminalBattleStatus(battle.Status) {
		return nil
	}
	if battle.Segment.Current == segment.OngoingEffects {
		return nil
	}
	pending, ok := battle.Flow.PendingInput[viewerActorID]
	if !ok {
		return nil
	}
	library, err := settledLibrary(battle)
	if err != nil {
		return nil
	}
	return settledLegalActions(battle, library, viewerActorID, pending)
}

func settledLegalActions(battle *state.Battle, library content.BattleLibrary, actorID string, pending state.PendingInput) []command.Command {
	window := battle.Settled.Window
	if window == nil || battle.Actors[actorID].DefeatState == state.ActorDefeated {
		return nil
	}
	var actions []command.Command
	switch window.Stage {
	case stageOffensivePlan:
		actions = append(actions, planningCardActions(battle, library, actorID, pending)...)
		runtime := battle.Settled.Actors[actorID]
		if containsCommand(window.AllowedCommands, command.TypePlanningRoll) && runtime.RollsUsed == 0 {
			actions = append(actions, legalCommand(battle.ID, actorID, command.TypePlanningRoll, command.PlanningRollPayload{PendingInputID: pending.ID, Checkpoint: planningCheckpoint(pending)}))
		}
		if containsCommand(window.AllowedCommands, command.TypePlanningKeep) && runtime.RollsUsed > 0 {
			for _, indices := range indexSubsets(allDieIndices(len(runtime.FinalDice)), true) {
				actions = append(actions, legalCommand(battle.ID, actorID, command.TypePlanningKeep, command.PlanningKeepPayload{PendingInputID: pending.ID, Checkpoint: planningCheckpoint(pending), KeptIndices: indices}))
			}
		}
		if containsCommand(window.AllowedCommands, command.TypePlanningReroll) && runtime.RollsUsed > 0 && runtime.RollsUsed < runtime.MaxRolls {
			var available []int
			for index := range runtime.FinalDice {
				if !containsInt(runtime.KeptIndices, index) {
					available = append(available, index)
				}
			}
			for _, indices := range indexSubsets(available, false) {
				actions = append(actions, legalCommand(battle.ID, actorID, command.TypePlanningReroll, command.PlanningRerollPayload{PendingInputID: pending.ID, Checkpoint: planningCheckpoint(pending), RerollIndices: indices}))
			}
		}
		if containsCommand(window.AllowedCommands, command.TypePlanningAbility) {
			for _, abilityID := range runtime.QualifiedAbilityIDs {
				ability := library.Abilities[abilityID]
				if ability.Usage.MaximumPerSegment > 0 && runtime.UsedAbilities[abilityID] >= ability.Usage.MaximumPerSegment {
					continue
				}
				for _, targets := range actorTargetChoices(battle, actorID, ability.Targeting) {
					tiers := []string{""}
					if abilityID == "needlefang" {
						tiers = nil
						for _, tier := range ability.Qualification.ActivationTiers {
							if requirementsMet(tier.Requirements, runtime.FinalDice) {
								tiers = append(tiers, tier.ID)
							}
						}
					}
					choices := [][]string{nil}
					if abilityID == "terminal_bite" || abilityID == "fever_spike" {
						n := 2
						tier, _ := qualifiedTier(ability, runtime.FinalDice)
						if abilityID == "fever_spike" && tier.ID == "base" {
							n = 1
						}
						choices = toxinChoices(battle, first(targets), n)
						if len(choices) == 0 {
							choices = [][]string{nil}
						}
					}
					for _, tierID := range tiers {
						for _, choice := range choices {
							actions = append(actions, legalCommand(battle.ID, actorID, command.TypePlanningAbility, command.PlanningAbilityPayload{PendingInputID: pending.ID, Checkpoint: planningCheckpoint(pending), AbilityID: abilityID, TierID: tierID, ToxinChoices: choice, TargetIDs: targets}))
						}
					}
				}
			}
		}
		if containsCommand(window.AllowedCommands, command.TypePlanningTargets) && runtime.SelectedAbilityID != "" {
			for _, targets := range actorTargetChoices(battle, actorID, library.Abilities[runtime.SelectedAbilityID].Targeting) {
				actions = append(actions, legalCommand(battle.ID, actorID, command.TypePlanningTargets, command.PlanningTargetsPayload{PendingInputID: pending.ID, Checkpoint: planningCheckpoint(pending), TargetIDs: targets}))
			}
		}
		if containsCommand(window.AllowedCommands, command.TypePlanningPass) {
			actions = append(actions, legalCommand(battle.ID, actorID, command.TypePlanningPass, command.PlanningPassPayload{PendingInputID: pending.ID, Checkpoint: planningCheckpoint(pending)}))
		}
	case stageDefenseSelect:
		runtime := battle.Settled.Actors[actorID]
		for _, abilityID := range runtime.DefensiveAbilityIDs {
			ability := library.Abilities[abilityID]
			if ability.Usage.MaximumPerSegment > 0 && defenseUses(battle, actorID, abilityID) >= ability.Usage.MaximumPerSegment {
				continue
			}
			if battle.Actors[actorID].Resources.EnergyPoints < ability.Cost.Energy {
				continue
			}
			for _, source := range battle.Settled.OffensiveSources {
				if source.TargetActorID == actorID && !defenseSourceChosen(battle, source.ID) {
					if abilityID == "barbed_mantle" && library.Abilities[source.SourceContentID].Type != "offensive" {
						continue
					}
					actions = append(actions, legalCommand(battle.ID, actorID, command.TypePlanningAbility, command.PlanningAbilityPayload{PendingInputID: pending.ID, Checkpoint: planningCheckpoint(pending), AbilityID: abilityID, TargetIDs: []string{source.ID}}))
					if abilityID == "shedskin" && stacks(battle, actorID, "catalyst") > 0 {
						actions = append(actions, legalCommand(battle.ID, actorID, command.TypePlanningAbility, command.PlanningAbilityPayload{PendingInputID: pending.ID, Checkpoint: planningCheckpoint(pending), AbilityID: abilityID, TargetIDs: []string{source.ID}, SpendCatalyst: true}))
					}
				}
			}
		}
		actions = append(actions, legalCommand(battle.ID, actorID, command.TypePlanningPass, command.PlanningPassPayload{PendingInputID: pending.ID, Checkpoint: planningCheckpoint(pending)}))
	case stageOngoingRoll:
		var unresolved []int
		ordinal := 0
		for _, roll := range battle.Settled.TriggerBatch.Rolls {
			if roll.ActorID != actorID {
				continue
			}
			if !roll.Resolved {
				unresolved = append(unresolved, ordinal)
			}
			ordinal++
		}
		for _, indices := range indexSubsets(unresolved, false) {
			actions = append(actions, legalCommand(battle.ID, actorID, command.TypeRollDice, command.RollDicePayload{PendingInputID: pending.ID, RerollIndices: indices}))
		}
	case stageDefenseRoll:
		actions = append(actions, legalCommand(battle.ID, actorID, command.TypeRollDice, command.RollDicePayload{PendingInputID: pending.ID, RequestID: pending.SourceID}))
	case stageHandLimit:
		actor := battle.Actors[actorID]
		need := len(actor.Cards.Hand) - battle.Settled.Actors[actorID].HandLimit
		for _, cards := range stringCombinations(actor.Cards.Hand, need) {
			actions = append(actions, legalCommand(battle.ID, actorID, command.TypeCommitInteraction, command.CommitInteractionPayload{PendingInputID: pending.ID, Checkpoint: interactionCheckpoint(pending), Commitment: command.InteractionCommitmentData{CardIDs: cards}}))
		}
	default:
		if containsCommand(window.AllowedCommands, command.TypeCommitInteraction) {
			actions = append(actions, reactionCardActions(battle, library, actorID, pending)...)
		}
		if containsCommand(window.AllowedCommands, command.TypePass) {
			actions = append(actions, legalCommand(battle.ID, actorID, command.TypePass, command.PassPayload{PendingInputID: pending.ID, Checkpoint: interactionCheckpoint(pending)}))
		}
	}
	actions = append(actions, venomCardActions(battle, library, actorID, pending)...)
	return actions
}

func planningCardActions(battle *state.Battle, library content.BattleLibrary, actorID string, pending state.PendingInput) []command.Command {
	if !containsCommand(battle.Settled.Window.AllowedCommands, command.TypePlanningCards) {
		return nil
	}
	actor := battle.Actors[actorID]
	runtime := battle.Settled.Actors[actorID]
	var actions []command.Command
	for _, instanceID := range actor.Cards.Hand {
		definition := library.Cards[runtime.CardInstances[instanceID].DefinitionID]
		if actor.Resources.EnergyPoints < definition.Cost.Energy || !cardPlayableDuring(definition, battle, "planning") {
			continue
		}
		base := command.PlanningCardsPayload{PendingInputID: pending.ID, Checkpoint: planningCheckpoint(pending), CardIDs: []string{instanceID}}
		switch definition.Targeting.Selector {
		case "self":
			payload := base
			payload.TargetIDs = []string{actorID}
			actions = append(actions, legalCommand(battle.ID, actorID, command.TypePlanningCards, payload))
		case "one_enemy":
			for _, targetID := range otherActorIDs(battle, actorID) {
				payload := base
				payload.TargetIDs = []string{targetID}
				actions = append(actions, legalCommand(battle.ID, actorID, command.TypePlanningCards, payload))
			}
		case "one_owned_combat_die":
			for index := range runtime.FinalDice {
				payload := base
				payload.DieIndex = index
				actions = append(actions, legalCommand(battle.ID, actorID, command.TypePlanningCards, payload))
			}
		case "one_owned_offensive_ability":
			for _, abilityID := range runtime.OffensiveAbilityIDs {
				payload := base
				payload.AbilityID = abilityID
				actions = append(actions, legalCommand(battle.ID, actorID, command.TypePlanningCards, payload))
			}
		}
	}
	return actions
}

func reactionCardActions(battle *state.Battle, library content.BattleLibrary, actorID string, pending state.PendingInput) []command.Command {
	actor := battle.Actors[actorID]
	runtime := battle.Settled.Actors[actorID]
	var actions []command.Command
	for _, instanceID := range actor.Cards.Hand {
		definition := library.Cards[runtime.CardInstances[instanceID].DefinitionID]
		if actor.Resources.EnergyPoints < definition.Cost.Energy || (!cardPlayableDuring(definition, battle, "reaction") && !(battle.Settled.Stage == stageVenomStatus && definition.Targeting.Selector == "one_negative_status_on_self")) {
			continue
		}
		if !reactionSelectorSupported(battle.Settled.Window.Stage, definition.Targeting.Selector) {
			continue
		}
		base := command.CommitInteractionPayload{PendingInputID: pending.ID, Checkpoint: interactionCheckpoint(pending), Commitment: command.InteractionCommitmentData{CardIDs: []string{instanceID}}}
		switch definition.Targeting.Selector {
		case "self":
			payload := base
			payload.Commitment.TargetIDs = []string{actorID}
			actions = append(actions, legalCommand(battle.ID, actorID, command.TypeCommitInteraction, payload))
		case "one_negative_status_on_self":
			for _, status := range actor.Statuses {
				if library.Statuses[status.DefinitionID].Polarity != "negative" {
					continue
				}
				payload := base
				payload.Commitment.ChoiceID = status.DefinitionID
				actions = append(actions, legalCommand(battle.ID, actorID, command.TypeCommitInteraction, payload))
			}
		case "one_incoming_damage_source":
			for _, source := range reactionDamageSources(battle) {
				if source.TargetActorID != actorID {
					continue
				}
				payload := base
				payload.Commitment.ProposalIDs = []string{source.ID}
				actions = append(actions, legalCommand(battle.ID, actorID, command.TypeCommitInteraction, payload))
			}
		case "one_owned_offensive_ability":
			for _, abilityID := range runtime.OffensiveAbilityIDs {
				payload := base
				payload.Commitment.ChoiceID = abilityID
				actions = append(actions, legalCommand(battle.ID, actorID, command.TypeCommitInteraction, payload))
			}
		case "selected_die":
			adjustmentFace, ok := selectedDieAdjustmentFace(definition)
			if !ok {
				continue
			}
			if battle.Settled.Window.Stage == stageBlindReact {
				pending := battle.Settled.PendingBlind
				if pending == nil || !selectedDieFaceEligible(definition.Targeting, pending.Face) {
					continue
				}
				payload := base
				payload.Commitment.PlanningAdjustments = []command.PlanningAdjustment{{Type: string(state.PlanningAdjustmentSetDieFace), ActorID: pending.ActorID, DieIndex: 0, Face: adjustmentFace}}
				actions = append(actions, legalCommand(battle.ID, actorID, command.TypeCommitInteraction, payload))
				continue
			}
			for _, targetActorID := range sortedSettledActorIDs(battle) {
				for dieIndex, die := range battle.Settled.Actors[targetActorID].FinalDice {
					if !selectedDieFaceEligible(definition.Targeting, die.Face) {
						continue
					}
					payload := base
					payload.Commitment.PlanningAdjustments = []command.PlanningAdjustment{{Type: string(state.PlanningAdjustmentSetDieFace), ActorID: targetActorID, DieIndex: dieIndex, Face: adjustmentFace}}
					actions = append(actions, legalCommand(battle.ID, actorID, command.TypeCommitInteraction, payload))
				}
			}
		}
	}
	return actions
}

func selectedDieFaceEligible(targeting content.TargetingDefinition, face int) bool {
	return targeting.RequiredFace == 0 || targeting.RequiredFace == face
}

func selectedDieAdjustmentFace(definition content.BattleCardDefinition) (int, bool) {
	for _, operation := range definition.Operations {
		if operation.Type == "modify_die" && operation.Target == "selected_die" && operation.Modification == "set_face" {
			return operation.Face, operation.Face > 0
		}
	}
	return 0, false
}

func reactionSelectorSupported(stage, selector string) bool {
	switch stage {
	case stageOffensiveReact, stageBlindReact:
		return selector == "selected_die"
	case stageVenomStatus, stageOngoingReact, stageDefenseReact:
		return selector == "one_negative_status_on_self" || selector == "self"
	case stageOngoingDamage, stageDamageReact:
		return selector == "one_incoming_damage_source"
	default:
		return false
	}
}

func legalCommand(battleID, actorID string, kind command.Type, payload any) command.Command {
	encoded, err := json.Marshal(payload)
	if err != nil {
		return command.Command{}
	}
	return command.Command{BattleID: battleID, ActorID: actorID, Type: kind, Payload: encoded}
}

func planningCheckpoint(pending state.PendingInput) command.PlanningCheckpoint {
	return command.PlanningCheckpoint{WindowID: pending.WindowID, Segment: string(pending.Segment), Stage: pending.Stage, Iteration: pending.Iteration, PlanningCycle: pending.PlanningCycle}
}

func interactionCheckpoint(pending state.PendingInput) command.InteractionCheckpoint {
	return command.InteractionCheckpoint{WindowID: pending.WindowID, Stage: pending.Stage, Iteration: pending.Iteration, ReactionRound: pending.ReactionRound, PlanningCycle: pending.PlanningCycle}
}

func actorTargetChoices(battle *state.Battle, actorID string, targeting *content.TargetingDefinition) [][]string {
	if targeting == nil || targeting.Maximum == 0 {
		return [][]string{nil}
	}
	switch targeting.Selector {
	case "self":
		return [][]string{{actorID}}
	case "one_enemy":
		var result [][]string
		for _, targetID := range otherActorIDs(battle, actorID) {
			result = append(result, []string{targetID})
		}
		return result
	default:
		return nil
	}
}

func otherActorIDs(battle *state.Battle, actorID string) []string {
	var result []string
	for targetID, actor := range battle.Actors {
		if targetID != actorID && actor.DefeatState != state.ActorDefeated && (actor.TeamID == "" || actor.TeamID != battle.Actors[actorID].TeamID) {
			result = append(result, targetID)
		}
	}
	sort.Strings(result)
	return result
}

func cardPlayableDuring(definition content.BattleCardDefinition, battle *state.Battle, purpose string) bool {
	if battle.Segment.Current == segment.OngoingEffects {
		return false
	}
	for _, timing := range definition.Play.PlayableDuring {
		immediateDamage := battle.Settled != nil && battle.Settled.Stage == stageOngoingDamage && battle.Settled.Venom != nil && battle.Settled.Venom.Active != nil && battle.Settled.Venom.Active.Kind == "damage"
		if (timing.Segment == string(battle.Segment.Current) || (immediateDamage && timing.Segment == "damage_resolution" && definition.Targeting.Selector == "one_incoming_damage_source")) && timing.Phase == "main" && timing.WindowPurpose == purpose {
			return true
		}
	}
	return false
}

func reactionDamageSources(battle *state.Battle) []state.SettledDamageSource {
	if battle.Settled.PendingDamage != nil {
		return battle.Settled.PendingDamage.Sources
	}
	return battle.Settled.OffensiveSources
}

func indexSubsets(values []int, includeEmpty bool) [][]int {
	if len(values) == 0 {
		if includeEmpty {
			return [][]int{{}}
		}
		return nil
	}
	start := 1
	if includeEmpty {
		start = 0
	}
	result := make([][]int, 0, (1<<len(values))-start)
	for mask := start; mask < 1<<len(values); mask++ {
		// Command payloads declare index selections as JSON arrays. Keep the
		// empty selection non-nil so the enumerated planning_keep candidate is
		// encoded as [] (and exactly matches clients), rather than null.
		subset := make([]int, 0, len(values))
		for index, value := range values {
			if mask&(1<<index) != 0 {
				subset = append(subset, value)
			}
		}
		result = append(result, subset)
	}
	return result
}

func stringCombinations(values []string, count int) [][]string {
	if count < 0 || count > len(values) {
		return nil
	}
	if count == 0 {
		return [][]string{{}}
	}
	var result [][]string
	var visit func(start int, current []string)
	visit = func(start int, current []string) {
		if len(current) == count {
			result = append(result, append([]string(nil), current...))
			return
		}
		for index := start; index <= len(values)-(count-len(current)); index++ {
			visit(index+1, append(current, values[index]))
		}
	}
	visit(0, nil)
	return result
}
