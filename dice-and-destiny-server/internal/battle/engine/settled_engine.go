package engine

import (
	"encoding/json"
	"errors"
	"fmt"
	"sort"

	"diceanddestiny/server/internal/battle/command"
	"diceanddestiny/server/internal/battle/event"
	"diceanddestiny/server/internal/battle/operation"
	battlerandom "diceanddestiny/server/internal/battle/random"
	"diceanddestiny/server/internal/battle/segment"
	"diceanddestiny/server/internal/battle/state"
	"diceanddestiny/server/internal/content"
)

const (
	stageOngoingCollect = "collect_statuses"
	stageOngoingReact   = "status_roll_reaction"
	stageOngoingDamage  = "status_damage_reaction"
	stageOffensivePlan  = "planning"
	stageOffensiveReact = "offensive_reaction"
	stageBlindReact     = "blind_reaction"
	stageDefenseSelect  = "defense_selection"
	stageDefenseRoll    = "defense_roll"
	stageDefenseReact   = "defense_reaction"
	stageDamageReact    = "damage_reaction"
	stageHandLimit      = "discard_to_hand_limit"
)

func (e Engine) namedIntn(battle *state.Battle, stream string, bound int) (int, error) {
	if e.namedRandom != nil {
		return e.namedRandom.IntnNamed(stream, bound)
	}
	return (battlerandom.NamedFallback{Source: battlerandom.BattleSource{Battle: battle}}).IntnNamed(stream, bound)
}

func settledLibrary(battle *state.Battle) (content.BattleLibrary, error) {
	var library content.BattleLibrary
	if battle == nil || battle.Settled == nil {
		return library, errors.New("settled battle state is required")
	}
	if err := json.Unmarshal(battle.SettledCatalog, &library); err != nil {
		return library, fmt.Errorf("decode pinned settled catalog: %w", err)
	}
	return library, nil
}

func (e Engine) progressSettled(battle *state.Battle) (ProgressionResult, error) {
	library, err := settledLibrary(battle)
	if err != nil {
		return ProgressionResult{}, err
	}
	var events []event.Event
	for steps := 0; steps < DefaultMaxAutomaticSteps; steps++ {
		if state.IsTerminalBattleStatus(battle.Status) {
			return ProgressionResult{Status: ProgressBattleComplete, Events: events}, nil
		}
		if battle.Settled.Window != nil {
			return ProgressionResult{Status: ProgressWaitingForInput, Events: events}, nil
		}
		var next []event.Event
		switch battle.Segment.Current {
		case segment.OngoingEffects:
			next, err = e.progressSettledOngoing(battle, library)
		case segment.Income:
			next, err = e.progressSettledIncome(battle, library)
		case segment.Offensive:
			next, err = e.progressSettledOffensive(battle, library)
		case segment.Defensive:
			next, err = e.progressSettledDefensive(battle, library)
		case segment.DamageResolution:
			next, err = e.progressSettledDamage(battle, library)
		default:
			err = fmt.Errorf("unknown settled segment %q", battle.Segment.Current)
		}
		if err != nil {
			return ProgressionResult{}, err
		}
		events = append(events, next...)
	}
	return ProgressionResult{}, ErrAutomaticStepLimit
}

func (e Engine) initializeSettled(battle *state.Battle) ([]event.Event, error) {
	if battle.Settled.Initialized {
		return nil, nil
	}
	var events []event.Event
	for _, actorID := range sortedSettledActorIDs(battle) {
		actor := battle.Actors[actorID]
		for i := 0; i < actor.Resources.StartingHandSize; i++ {
			drawn, err := e.drawSettledCard(battle, actorID, "card_draw")
			if err != nil {
				return nil, err
			}
			if drawn != "" {
				events = append(events, event.NewCardsDrawn(actorID, []string{drawn}, false))
			}
		}
	}
	battle.Settled.Initialized = true
	battle.Settled.Stage = ""
	battle.Flow = state.NewSegmentFlowState(battle.Segment)
	battle.Flow.Entered = true
	events = append([]event.Event{event.NewSegmentEntered(battle.Segment)}, events...)
	return events, nil
}

func (e Engine) progressSettledOngoing(battle *state.Battle, library content.BattleLibrary) ([]event.Event, error) {
	if !battle.Settled.Initialized {
		return e.initializeSettled(battle)
	}
	if battle.Settled.Stage == "" {
		battle.Settled.Stage = stageOngoingCollect
		battle.Flow.Stage = stageOngoingCollect
		batch, err := e.collectStatusTriggerBatch(battle, library, "ongoing_effects", "entry", stageOngoingCollect)
		if err != nil {
			return nil, err
		}
		battle.Settled.TriggerBatch = batch
		if batch == nil {
			return e.advanceSettledSegment(battle)
		}
		if batch.Reactable {
			openSettledWindow(battle, "status-roll", stageOngoingReact, "reaction", []command.Type{command.TypeCommitInteraction, command.TypePass})
			return []event.Event{settledEvent(event.TypeInteractionWindowOpened, battle, "", map[string]any{"batch_id": batch.ID, "rolls": batch.Rolls})}, nil
		}
		return e.finalizeTriggerBatch(battle, library)
	}
	if battle.Settled.Stage == stageOngoingCollect && battle.Settled.TriggerBatch != nil {
		return e.finalizeTriggerBatch(battle, library)
	}
	return e.advanceSettledSegment(battle)
}

func (e Engine) collectStatusTriggerBatch(battle *state.Battle, library content.BattleLibrary, segmentID, phase, stage string) (*state.SettledTriggerBatch, error) {
	batch := &state.SettledTriggerBatch{ID: fmt.Sprintf("trigger-r%d-%s-%d", battle.Segment.Round, segmentID, battle.Settled.Sequence+1)}
	battle.Settled.Sequence++
	for _, actorID := range sortedSettledActorIDs(battle) {
		actor := battle.Actors[actorID]
		statuses := append([]state.StatusState(nil), actor.Statuses...)
		sort.Slice(statuses, func(i, j int) bool {
			if statuses[i].DefinitionID == statuses[j].DefinitionID {
				return statuses[i].InstanceID < statuses[j].InstanceID
			}
			return statuses[i].DefinitionID < statuses[j].DefinitionID
		})
		for _, instance := range statuses {
			definition, ok := library.Statuses[instance.DefinitionID]
			if !ok {
				continue
			}
			for _, trigger := range definition.Triggers {
				if trigger.Segment != segmentID || trigger.Phase != phase || trigger.Stage != stage {
					continue
				}
				batch.StatusInstanceIDs = append(batch.StatusInstanceIDs, instance.InstanceID)
				for _, op := range trigger.Operations {
					repeat := 1
					if op.OnePerStatusStack || op.Repeat == "one_per_status_stack" {
						repeat = instance.Stacks
					}
					for n := 0; n < repeat; n++ {
						switch op.Type {
						case "roll_dice":
							die := library.Dice[op.DiceID]
							value, err := e.namedIntn(battle, "status_effect_dice", die.SideCount)
							if err != nil {
								return nil, err
							}
							face := die.Faces[value]
							batch.Rolls = append(batch.Rolls, state.SettledEffectRoll{ActorID: actorID, StatusInstanceID: instance.InstanceID, StatusID: instance.DefinitionID, Die: state.RolledDie{Index: n, DieID: die.ID, Face: face.Number, Value: face.Number, Symbols: []string{face.Symbol}}})
							if op.ReactionWindow != nil && op.ReactionWindow.Opens {
								batch.Reactable = true
							}
						case "deal_damage":
							amount, _ := operationAmount(op, 0)
							batch.Damage = append(batch.Damage, newSettledDamageSource(battle, actorID, actorID, instance.DefinitionID, amount))
						case "remove_status_stack":
							batch.RemoveStacks = append(batch.RemoveStacks, state.SettledStatusRemoval{ActorID: actorID, StatusID: op.StatusID, Stacks: max(1, op.StackCount)})
						case "adjust_max_rolls":
							runtime := battle.Settled.Actors[actorID]
							amount, _ := operationAmount(op, 0)
							runtime.MaxRolls = max(1, runtime.MaxRolls+amount)
							battle.Settled.Actors[actorID] = runtime
						}
					}
				}
				if definition.Lifecycle.ConsumeOnTriggerCheckpoint && len(trigger.Operations) == 0 {
					removeStatus(battle, actorID, definition.ID, 0)
				}
			}
		}
	}
	if len(batch.StatusInstanceIDs) == 0 {
		return nil, nil
	}
	return batch, nil
}

func (e Engine) finalizeTriggerBatch(battle *state.Battle, library content.BattleLibrary) ([]event.Event, error) {
	batch := battle.Settled.TriggerBatch
	if batch == nil {
		return nil, nil
	}
	for _, roll := range batch.Rolls {
		definition := library.Statuses[roll.StatusID]
		for _, trigger := range definition.Triggers {
			for _, op := range trigger.Operations {
				if op.Type != "roll_dice" {
					continue
				}
				for _, outcome := range op.Outcomes {
					if !containsInt(outcome.Faces, roll.Die.Face) {
						continue
					}
					for _, child := range outcome.Operations {
						switch child.Type {
						case "deal_damage":
							amount, _ := operationAmount(child, roll.Die.Face)
							batch.Damage = append(batch.Damage, newSettledDamageSource(battle, roll.ActorID, roll.ActorID, roll.StatusID, amount))
						case "remove_status_stack":
							batch.RemoveStacks = append(batch.RemoveStacks, state.SettledStatusRemoval{ActorID: roll.ActorID, StatusID: child.StatusID, Stacks: max(1, child.StackCount)})
						}
					}
				}
			}
		}
	}
	for _, removal := range batch.RemoveStacks {
		removeStatus(battle, removal.ActorID, removal.StatusID, removal.Stacks)
	}
	for _, instanceID := range batch.StatusInstanceIDs {
		actorID, statusID := findStatusInstance(battle, instanceID)
		if actorID == "" {
			continue
		}
		definition := library.Statuses[statusID]
		if definition.Lifecycle.RemoveAfterResolution {
			removeStatus(battle, actorID, statusID, 0)
		}
	}
	var events []event.Event
	events = append(events, settledEvent(event.TypeProposalBatchCommitted, battle, "", map[string]any{"batch_id": batch.ID, "rolls": batch.Rolls, "damage": batch.Damage, "status_removals": batch.RemoveStacks}))
	if len(batch.Damage) > 0 {
		damageBatch, err := e.buildDamageBatch(battle, batch.Damage)
		if err != nil {
			return nil, err
		}
		battle.Settled.PendingDamage = damageBatch
		batch.Finalized = true
		openSettledWindow(battle, "status-damage", stageOngoingDamage, "damage_response", []command.Type{command.TypeCommitInteraction, command.TypePass})
		events = append(events, damageRevealEvent(battle, damageBatch))
		return events, nil
	}
	battle.Settled.TriggerBatch = nil
	battle.Settled.Stage = "complete"
	advanced, err := e.advanceSettledSegment(battle)
	return append(events, advanced...), err
}

func (e Engine) progressSettledIncome(battle *state.Battle, _ content.BattleLibrary) ([]event.Event, error) {
	var events []event.Event
	for _, actorID := range sortedSettledActorIDs(battle) {
		runtime := battle.Settled.Actors[actorID]
		for i := 0; i < runtime.IncomeCards; i++ {
			card, err := e.drawSettledCard(battle, actorID, "card_draw")
			if err != nil {
				return nil, err
			}
			events = append(events, event.NewCardsDrawn(actorID, nonEmpty(card), card == ""))
		}
		actor := battle.Actors[actorID]
		actor.Resources.EnergyPoints += runtime.IncomeEnergy
		actor.EnergyPoints = actor.Resources.EnergyPoints
		battle.Actors[actorID] = actor
		if runtime.IncomeEnergy > 0 {
			events = append(events, event.Event{Type: event.TypeEnergyPointsGained, ActorID: actorID, EnergyPoints: actor.Resources.EnergyPoints})
		}
	}
	advanced, err := e.advanceSettledSegment(battle)
	return append(events, advanced...), err
}

func (e Engine) progressSettledOffensive(battle *state.Battle, library content.BattleLibrary) ([]event.Event, error) {
	if battle.Settled.Stage == "" {
		battle.Settled.Stage = stageOffensivePlan
		battle.Flow.Stage = stageOffensivePlan
		// Generic Offensive-entry triggers (Entangle) resolve before planning.
		if err := e.applyOffensiveEntryTriggers(battle, library); err != nil {
			return nil, err
		}
		for _, actorID := range sortedSettledActorIDs(battle) {
			runtime := battle.Settled.Actors[actorID]
			runtime.RollHistory = nil
			runtime.FinalDice = nil
			runtime.KeptIndices = nil
			runtime.RollsUsed = 0
			runtime.QualifiedAbilityIDs = nil
			runtime.SelectedAbilityID = ""
			runtime.SelectedTierID = ""
			runtime.SelectedTargetIDs = nil
			runtime.MaxRolls = max(1, runtime.MaxRolls)
			runtime.UsedAbilities = map[string]int{}
			battle.Settled.Actors[actorID] = runtime
			if battle.Actors[actorID].Controller == state.ControllerAI {
				if err := e.planSettledAI(battle, library, actorID); err != nil {
					return nil, err
				}
			}
		}
		player := humanActorID(battle)
		openSettledWindow(battle, "offensive-planning", stageOffensivePlan, "planning", []command.Type{command.TypePlanningCards, command.TypePlanningRoll, command.TypePlanningKeep, command.TypePlanningReroll, command.TypePlanningAbility, command.TypePlanningTargets, command.TypePlanningPass})
		return []event.Event{event.NewRollRequested(player, segment.Offensive, fmt.Sprintf("roll-r%d-%s", battle.Segment.Round, player), battle.Settled.Window.PendingInputID)}, nil
	}
	return nil, fmt.Errorf("settled offensive stalled at %q", battle.Settled.Stage)
}

func (e Engine) applyOffensiveEntryTriggers(battle *state.Battle, library content.BattleLibrary) error {
	for _, actorID := range sortedSettledActorIDs(battle) {
		actor := battle.Actors[actorID]
		for _, instance := range append([]state.StatusState(nil), actor.Statuses...) {
			definition := library.Statuses[instance.DefinitionID]
			for _, trigger := range definition.Triggers {
				if trigger.Segment != "offensive" || trigger.Phase != "entry" {
					continue
				}
				for _, op := range trigger.Operations {
					if op.Type == "adjust_max_rolls" {
						runtime := battle.Settled.Actors[actorID]
						amount, _ := operationAmount(op, 0)
						runtime.MaxRolls = max(1, runtime.MaxRolls+amount)
						battle.Settled.Actors[actorID] = runtime
					}
				}
				if definition.Lifecycle.ConsumeOnTriggerCheckpoint || definition.Lifecycle.RemoveAfterResolution {
					removeStatus(battle, actorID, definition.ID, 0)
				}
			}
		}
	}
	return nil
}

func (e Engine) planSettledAI(battle *state.Battle, library content.BattleLibrary, actorID string) error {
	definition := library.Combatants[battle.Actors[actorID].DefinitionID]
	runtime := battle.Settled.Actors[actorID]
	key := fmt.Sprintf("%d_rolls", runtime.MaxRolls)
	if runtime.MaxRolls == 1 {
		key = "1_roll"
	}
	chart, ok := definition.AI.OffensivePlanning.Charts[key]
	if !ok {
		return fmt.Errorf("AI %s has no %s chart", actorID, key)
	}
	roll, err := e.namedIntn(battle, "ai_d100", 100)
	if err != nil {
		return err
	}
	roll++
	runtime.AID100 = roll
	var selected string
	simulated := 0
	for _, entry := range chart.Abilities {
		ranges := []*content.D100Range{entry.ActivationRanges.FirstRoll, entry.ActivationRanges.SecondRoll, entry.ActivationRanges.ThirdRoll}
		for i, r := range ranges {
			if r != nil && roll >= r.Start && roll <= r.End {
				selected = entry.AbilityID
				simulated = i + 1
			}
		}
	}
	runtime.SelectedAbilityID = selected
	runtime.AISimulatedRolls = simulated
	runtime.RollsUsed = simulated
	runtime.MaxRolls = max(1, runtime.MaxRolls)
	if selected != "" {
		faces := definition.AI.RevealProfiles[selected]
		runtime.FinalDice = rolledFaces(library, "standard_d6", faces)
		runtime.SelectedTargetIDs = []string{humanActorID(battle)}
		tier, _ := qualifiedTier(library.Abilities[selected], runtime.FinalDice)
		runtime.SelectedTierID = tier.ID
	}
	battle.Settled.Actors[actorID] = runtime
	return nil
}

func (e Engine) progressSettledDefensive(battle *state.Battle, library content.BattleLibrary) ([]event.Event, error) {
	if battle.Settled.Stage == "" {
		battle.Settled.Stage = stageDefenseSelect
		battle.Flow.Stage = stageDefenseSelect
		battle.Settled.DefenseSelections = map[string]state.SettledDefense{}
		human := humanActorID(battle)
		if hasIncoming(battle, human) {
			openSettledWindow(battle, "defense-select", stageDefenseSelect, "defense_selection", []command.Type{command.TypePlanningAbility, command.TypePlanningPass})
			return nil, nil
		}
		if err := e.selectAIDefenses(battle, library); err != nil {
			return nil, err
		}
		return e.afterDefenseSelections(battle, library)
	}
	return nil, fmt.Errorf("settled defensive stalled at %q", battle.Settled.Stage)
}

func (e Engine) selectAIDefenses(battle *state.Battle, library content.BattleLibrary) error {
	for _, actorID := range sortedSettledActorIDs(battle) {
		if battle.Actors[actorID].Controller != state.ControllerAI || !hasIncoming(battle, actorID) {
			continue
		}
		if _, exists := battle.Settled.DefenseSelections[actorID]; exists {
			continue
		}
		runtime := battle.Settled.Actors[actorID]
		legal := runtime.DefensiveAbilityIDs
		if len(legal) == 0 {
			continue
		}
		choice := 0
		if len(legal) > 1 {
			value, err := e.namedIntn(battle, "ai_defense", len(legal))
			if err != nil {
				return err
			}
			choice = value
		}
		abilityID := legal[choice]
		source := firstIncoming(battle, actorID)
		ability := library.Abilities[abilityID]
		if battle.Actors[actorID].Resources.EnergyPoints < ability.Cost.Energy {
			return fmt.Errorf("AI cannot afford defense %q", abilityID)
		}
		spendEnergy(battle, actorID, ability.Cost.Energy)
		battle.Settled.DefenseSelections[actorID] = state.SettledDefense{ActorID: actorID, AbilityID: abilityID, SourceID: source.ID}
	}
	return nil
}

func (e Engine) afterDefenseSelections(battle *state.Battle, library content.BattleLibrary) ([]event.Event, error) {
	if err := e.selectAIDefenses(battle, library); err != nil {
		return nil, err
	}
	human := humanActorID(battle)
	selection, hasHuman := battle.Settled.DefenseSelections[human]
	if hasHuman {
		ability := library.Abilities[selection.AbilityID]
		if ability.Resolution.Roll != nil {
			battle.Settled.Stage = stageDefenseRoll
			openSettledWindow(battle, "defense-roll", stageDefenseRoll, "required_roll", []command.Type{command.TypeRollDice})
			return nil, nil
		}
	}
	return e.resolveDefenseRollsAndOpenReaction(battle, library)
}

func (e Engine) resolveDefenseRollsAndOpenReaction(battle *state.Battle, library content.BattleLibrary) ([]event.Event, error) {
	var events []event.Event
	reactable := false
	for _, actorID := range sortedSettledActorIDs(battle) {
		selection, exists := battle.Settled.DefenseSelections[actorID]
		if !exists {
			continue
		}
		ability := library.Abilities[selection.AbilityID]
		if ability.Resolution.Roll != nil && selection.RolledFace == 0 {
			die := library.Dice[ability.Resolution.Roll.DiceID]
			value, err := e.namedIntn(battle, "defense_dice", die.SideCount)
			if err != nil {
				return nil, err
			}
			selection.RolledFace = die.Faces[value].Number
			battle.Settled.DefenseSelections[actorID] = selection
			events = append(events, event.Event{Type: event.TypeDiceRolled, ActorID: actorID, Segment: segment.Defensive, Pool: state.RollPoolDefensive, SourceType: state.RollSourceAbility, SourceID: selection.AbilityID, Dice: rolledFaces(library, die.ID, []int{selection.RolledFace})})
			if ability.Resolution.ReactionWindow.Opens {
				reactable = true
			}
		}
	}
	if reactable {
		battle.Settled.Stage = stageDefenseReact
		openSettledWindow(battle, "defense-react", stageDefenseReact, "reaction", []command.Type{command.TypeCommitInteraction, command.TypePass})
		return events, nil
	}
	return append(events, e.finalizeDefenses(battle, library)...), nil
}

func (e Engine) finalizeDefenses(battle *state.Battle, library content.BattleLibrary) []event.Event {
	var events []event.Event
	for actorID, selection := range battle.Settled.DefenseSelections {
		ability := library.Abilities[selection.AbilityID]
		source := sourceBySettledID(battle, selection.SourceID)
		if source == nil {
			continue
		}
		for _, op := range ability.Resolution.Operations {
			switch op.Type {
			case "prevent_damage":
				amount, _ := operationAmount(op, selection.RolledFace)
				source.Prevention += amount
			case "scale_damage":
				source.ScaleNumerator = op.Numerator
				source.ScaleDenominator = op.Denominator
			}
		}
		selection.Finalized = true
		battle.Settled.DefenseSelections[actorID] = selection
		events = append(events, settledEvent(event.TypeDefenseSelected, battle, actorID, map[string]any{"ability_id": selection.AbilityID, "source_id": selection.SourceID, "rolled_face": selection.RolledFace}))
	}
	battle.Settled.Stage = "complete"
	advanced, _ := e.advanceSettledSegment(battle)
	return append(events, advanced...)
}

func (e Engine) progressSettledDamage(battle *state.Battle, library content.BattleLibrary) ([]event.Event, error) {
	if battle.Settled.Stage == "" {
		batch, err := e.buildDamageBatch(battle, battle.Settled.OffensiveSources)
		if err != nil {
			return nil, err
		}
		battle.Settled.PendingDamage = batch
		battle.Settled.Stage = stageDamageReact
		battle.Flow.Stage = stageDamageReact
		if len(batch.Removals) == 0 {
			return e.finishDamageBatch(battle, library)
		}
		if err := e.autoAIDamageResponse(battle, library, batch); err != nil {
			return nil, err
		}
		openSettledWindow(battle, "damage", stageDamageReact, "damage_response", []command.Type{command.TypeCommitInteraction, command.TypePass})
		return []event.Event{damageRevealEvent(battle, batch)}, nil
	}
	return nil, fmt.Errorf("settled damage stalled at %q", battle.Settled.Stage)
}

func (e Engine) buildDamageBatch(battle *state.Battle, sources []state.SettledDamageSource) (*state.SettledDamageBatch, error) {
	batch := &state.SettledDamageBatch{ID: fmt.Sprintf("damage-r%d-%d", battle.Segment.Round, battle.Settled.Sequence+1), Overage: map[string]int{}}
	battle.Settled.Sequence++
	for _, source := range sources {
		source.FinalAmount = source.BaseAmount - source.Prevention
		if source.FinalAmount < 0 {
			source.FinalAmount = 0
		}
		if source.ScaleDenominator > 0 {
			source.FinalAmount = source.FinalAmount * source.ScaleNumerator / source.ScaleDenominator
		}
		batch.Sources = append(batch.Sources, source)
		batch.Applications = append(batch.Applications, source.StatusApplications...)
	}
	byTarget := map[string]int{}
	sourceIDs := map[string][]string{}
	sourceActors := map[string][]string{}
	for _, source := range batch.Sources {
		byTarget[source.TargetActorID] += source.FinalAmount
		sourceIDs[source.TargetActorID] = append(sourceIDs[source.TargetActorID], source.ID)
		sourceActors[source.TargetActorID] = append(sourceActors[source.TargetActorID], source.SourceActorID)
	}
	targets := make([]string, 0, len(byTarget))
	for id := range byTarget {
		targets = append(targets, id)
	}
	sort.Strings(targets)
	for _, target := range targets {
		actor := battle.Actors[target]
		remaining := byTarget[target]
		sequence := 1
		for _, zone := range []operation.CardZone{operation.ZoneDeck, operation.ZoneDiscard, operation.ZoneHand} {
			cards := zoneCards(actor.Cards, zone)
			for remaining > 0 && len(cards) > 0 {
				index, err := e.namedIntn(battle, "damage_selection", len(cards))
				if err != nil {
					return nil, err
				}
				cardID := cards[index]
				cards = append(cards[:index], cards[index+1:]...)
				definitionID := battle.Settled.Actors[target].CardInstances[cardID].DefinitionID
				batch.Removals = append(batch.Removals, state.ProposedCardRemoval{ID: fmt.Sprintf("%s-removal-%d", batch.ID, len(batch.Removals)+1), TargetActorID: target, CardID: cardID, CardDefinitionID: definitionID, OriginalZone: zone, DamageProposalIDs: append([]string(nil), sourceIDs[target]...), SourceActorIDs: append([]string(nil), sourceActors[target]...), Sequence: sequence, Revealed: true, Accepted: true})
				sequence++
				remaining--
			}
		}
		if remaining > 0 {
			batch.Overage[target] = remaining
		}
	}
	batch.Revealed = true
	return batch, nil
}

func (e Engine) autoAIDamageResponse(battle *state.Battle, library content.BattleLibrary, batch *state.SettledDamageBatch) error {
	for _, actorID := range sortedSettledActorIDs(battle) {
		if battle.Actors[actorID].Controller != state.ControllerAI || damageForTarget(batch, actorID) == 0 {
			continue
		}
		choice, err := e.namedIntn(battle, "ai_damage_response", 2)
		if err != nil {
			return err
		}
		if choice == 0 {
			continue
		}
		cardID := findCardDefinitionInZone(battle, actorID, "emergency_ward", operation.ZoneHand)
		if cardID == "" {
			return fmt.Errorf("scripted AI damage response requested without emergency_ward")
		}
		source := firstBatchSourceForTarget(batch, actorID)
		if source == nil {
			return nil
		}
		if err := e.playSettledCard(battle, library, actorID, cardID, []string{source.ID}, "", 0, ""); err != nil {
			return err
		}
		source.ReactionPrevention += 3
		reconcileSettledDamage(batch, battle)
	}
	return nil
}

func (e Engine) finishDamageBatch(battle *state.Battle, library content.BattleLibrary) ([]event.Event, error) {
	batch := battle.Settled.PendingDamage
	if batch == nil {
		return e.advanceSettledSegment(battle)
	}
	var events []event.Event
	for _, removal := range batch.Removals {
		if !removal.Accepted || removal.Released {
			continue
		}
		actor := battle.Actors[removal.TargetActorID]
		moveCard(&actor.Cards, removal.CardID, removal.OriginalZone, operation.ZoneRemoved)
		battle.Actors[removal.TargetActorID] = actor
		events = append(events, event.NewCardPermanentlyRemoved(removal))
	}
	for _, application := range batch.Applications {
		applyStatus(battle, library, application.TargetActorID, application.StatusID, application.Stacks)
	}
	for actorID, actor := range battle.Actors {
		if actor.CurrentHealth() == 0 {
			actor.DefeatState = state.ActorPendingDefeat
			battle.Actors[actorID] = actor
		}
	}
	batch.Committed = true
	events = append(events, settledEvent(event.TypeDamageCommitted, battle, "", map[string]any{"batch_id": batch.ID, "sources": batch.Sources, "removals": batch.Removals, "overage": batch.Overage, "status_applications": batch.Applications}))
	if battle.Segment.Current == segment.OngoingEffects {
		battle.Settled.PendingDamage = nil
		battle.Settled.TriggerBatch = nil
		battle.Settled.Stage = "complete"
		advanced, err := e.advanceSettledSegment(battle)
		return append(events, advanced...), err
	}
	for actorID, actor := range battle.Actors {
		limit := battle.Settled.Actors[actorID].HandLimit
		if len(actor.Cards.Hand) > limit && actor.Controller == state.ControllerHuman {
			battle.Settled.Stage = stageHandLimit
			openSettledWindow(battle, "hand-limit", stageHandLimit, "choose_card", []command.Type{command.TypeCommitInteraction})
			return events, nil
		}
	}
	battle.Settled.PendingDamage = nil
	battle.Settled.Stage = "complete"
	advanced, err := e.advanceSettledSegment(battle)
	return append(events, advanced...), err
}

func (e Engine) advanceSettledSegment(battle *state.Battle) ([]event.Event, error) {
	// Final action of every Exit: mark zero-health actors defeated and evaluate.
	for actorID, actor := range battle.Actors {
		if actor.DefeatState == state.ActorPendingDefeat {
			actor.DefeatState = state.ActorDefeated
			battle.Actors[actorID] = actor
		}
	}
	completion, err := evaluateBattleCompletion(battle)
	if err != nil {
		return nil, err
	}
	if state.IsTerminalBattleStatus(battle.Status) {
		battle.Settled.CompletedRounds = battle.Segment.Round
		for actorID, runtime := range battle.Settled.Actors {
			runtime.AbilityModifiers = nil
			battle.Settled.Actors[actorID] = runtime
		}
		return completion, nil
	}
	next, advance, err := e.manager.Advance(battle.Segment)
	if err != nil {
		return nil, err
	}
	if advance.CompletedTurn {
		battle.Settled.CompletedRounds = battle.Segment.Round
	}
	battle.Segment = next
	battle.Flow = state.NewSegmentFlowState(next)
	battle.Flow.Entered = true
	battle.Settled.Stage = ""
	battle.Settled.Window = nil
	if next.Current == segment.OngoingEffects {
		for actorID, runtime := range battle.Settled.Actors {
			runtime.MaxRolls = 3
			battle.Settled.Actors[actorID] = runtime
		}
	}
	events := append(completion, event.NewSegmentAdvanced(advance), event.NewSegmentEntered(next))
	return events, nil
}

func (e Engine) handleSettledCommand(battle *state.Battle, cmd command.Command) ([]event.Event, error) {
	library, err := settledLibrary(battle)
	if err != nil {
		return nil, err
	}
	window := battle.Settled.Window
	if window == nil {
		return nil, errors.New("no pending human input")
	}
	if cmd.ActorID != window.RequiredActorID {
		return nil, errors.New("actor does not own pending input")
	}
	if !containsCommand(window.AllowedCommands, cmd.Type) {
		return nil, fmt.Errorf("command %q is not allowed", cmd.Type)
	}
	pending, ok := battle.Flow.PendingInput[cmd.ActorID]
	if !ok {
		return nil, errors.New("pending input is missing")
	}
	if err := validateSettledPending(cmd, pending); err != nil {
		return nil, err
	}
	var events []event.Event
	switch window.Stage {
	case stageOffensivePlan:
		events, err = e.handleOffensivePlanningCommand(battle, library, cmd)
	case stageOffensiveReact:
		events, err = e.handleOffensiveReactionCommand(battle, library, cmd)
	case stageOngoingReact:
		events, err = e.handleStatusReactionCommand(battle, library, cmd)
	case stageOngoingDamage, stageDamageReact:
		events, err = e.handleDamageReactionCommand(battle, library, cmd)
	case stageDefenseSelect:
		events, err = e.handleDefenseSelectionCommand(battle, library, cmd)
	case stageDefenseRoll:
		events, err = e.handleDefenseRollCommand(battle, library, cmd)
	case stageDefenseReact:
		if cmd.Type != command.TypePass {
			return nil, errors.New("only pass is supported in this reaction")
		}
		closeSettledWindow(battle)
		events = e.finalizeDefenses(battle, library)
	case stageBlindReact:
		events, err = e.handleBlindReactionCommand(battle, library, cmd)
	case stageHandLimit:
		events, err = e.handleHandLimitCommand(battle, cmd)
	default:
		err = fmt.Errorf("unsupported settled window stage %q", window.Stage)
	}
	return events, err
}

func (e Engine) handleOffensivePlanningCommand(battle *state.Battle, library content.BattleLibrary, cmd command.Command) ([]event.Event, error) {
	actorID := cmd.ActorID
	runtime := battle.Settled.Actors[actorID]
	switch cmd.Type {
	case command.TypePlanningCards:
		var payload command.PlanningCardsPayload
		if err := command.DecodePayload(cmd, &payload); err != nil {
			return nil, err
		}
		if len(payload.CardIDs) != 1 {
			return nil, errors.New("exactly one card instance is required")
		}
		if err := e.playSettledCard(battle, library, actorID, payload.CardIDs[0], payload.TargetIDs, payload.AbilityID, payload.DieIndex, payload.StatusID); err != nil {
			return nil, err
		}
		rotateSettledPending(battle)
		return []event.Event{settledEvent(event.TypeCardPlayed, battle, actorID, map[string]any{"card_instance_id": payload.CardIDs[0], "targets": payload.TargetIDs, "ability_id": payload.AbilityID})}, nil
	case command.TypePlanningRoll:
		if runtime.RollsUsed != 0 {
			return nil, errors.New("initial roll already used")
		}
		dice, err := e.rollCombatDice(battle, library, actorID, nil)
		if err != nil {
			return nil, err
		}
		runtime = battle.Settled.Actors[actorID]
		runtime.RollsUsed = 1
		runtime.FinalDice = dice
		runtime.RollHistory = append(runtime.RollHistory, state.RollBatch{Number: 1, RolledIndices: allDieIndices(len(dice)), Dice: cloneDice(dice)})
		runtime.QualifiedAbilityIDs = qualifiedAbilities(library, runtime.OffensiveAbilityIDs, dice, runtime.AbilityModifiers)
		battle.Settled.Actors[actorID] = runtime
		rotateSettledPending(battle)
		return []event.Event{diceEvent(battle, actorID, runtime, allDieIndices(len(dice)))}, nil
	case command.TypePlanningKeep:
		var payload command.PlanningKeepPayload
		if err := command.DecodePayload(cmd, &payload); err != nil {
			return nil, err
		}
		if runtime.RollsUsed == 0 {
			return nil, errors.New("roll before keeping dice")
		}
		if err := validateIndices(runtime.FinalDice, payload.KeptIndices); err != nil {
			return nil, err
		}
		runtime.KeptIndices = append([]int(nil), payload.KeptIndices...)
		battle.Settled.Actors[actorID] = runtime
		rotateSettledPending(battle)
		return nil, nil
	case command.TypePlanningReroll:
		var payload command.PlanningRerollPayload
		if err := command.DecodePayload(cmd, &payload); err != nil {
			return nil, err
		}
		if runtime.RollsUsed == 0 || runtime.RollsUsed >= runtime.MaxRolls {
			return nil, errors.New("no reroll is available")
		}
		if err := validateRerollIndices(runtime, payload.RerollIndices); err != nil {
			return nil, err
		}
		dice, err := e.rollCombatDice(battle, library, actorID, payload.RerollIndices)
		if err != nil {
			return nil, err
		}
		runtime = battle.Settled.Actors[actorID]
		runtime.RollsUsed++
		runtime.FinalDice = dice
		runtime.RollHistory = append(runtime.RollHistory, state.RollBatch{Number: runtime.RollsUsed, RolledIndices: append([]int(nil), payload.RerollIndices...), Dice: cloneDice(dice), KeptIndices: append([]int(nil), runtime.KeptIndices...)})
		runtime.QualifiedAbilityIDs = qualifiedAbilities(library, runtime.OffensiveAbilityIDs, dice, runtime.AbilityModifiers)
		battle.Settled.Actors[actorID] = runtime
		rotateSettledPending(battle)
		return []event.Event{diceEvent(battle, actorID, runtime, payload.RerollIndices)}, nil
	case command.TypePlanningAbility:
		var payload command.PlanningAbilityPayload
		if err := command.DecodePayload(cmd, &payload); err != nil {
			return nil, err
		}
		if !containsString(runtime.QualifiedAbilityIDs, payload.AbilityID) {
			return nil, fmt.Errorf("ability %q is not qualified", payload.AbilityID)
		}
		runtime.SelectedAbilityID = payload.AbilityID
		tier, _ := qualifiedTier(library.Abilities[payload.AbilityID], runtime.FinalDice)
		runtime.SelectedTierID = tier.ID
		runtime.SelectedTargetIDs = append([]string(nil), payload.TargetIDs...)
		battle.Settled.Actors[actorID] = runtime
		if len(payload.TargetIDs) > 0 {
			return e.finalizeOffensivePlanning(battle, library)
		}
		rotateSettledPending(battle)
		return []event.Event{settledEvent(event.TypeAbilitySelected, battle, actorID, map[string]any{"ability_id": payload.AbilityID, "qualified": runtime.QualifiedAbilityIDs})}, nil
	case command.TypePlanningTargets:
		var payload command.PlanningTargetsPayload
		if err := command.DecodePayload(cmd, &payload); err != nil {
			return nil, err
		}
		if runtime.SelectedAbilityID == "" || len(payload.TargetIDs) != 1 {
			return nil, errors.New("one target is required after selecting an ability")
		}
		if _, ok := battle.Actors[payload.TargetIDs[0]]; !ok || payload.TargetIDs[0] == actorID {
			return nil, errors.New("invalid offensive target")
		}
		runtime.SelectedTargetIDs = append([]string(nil), payload.TargetIDs...)
		battle.Settled.Actors[actorID] = runtime
		return e.finalizeOffensivePlanning(battle, library)
	case command.TypePlanningPass:
		runtime.SelectedAbilityID = ""
		runtime.SelectedTargetIDs = nil
		battle.Settled.Actors[actorID] = runtime
		return e.finalizeOffensivePlanning(battle, library)
	default:
		return nil, unsupportedCommand()
	}
}

func (e Engine) finalizeOffensivePlanning(battle *state.Battle, library content.BattleLibrary) ([]event.Event, error) {
	closeSettledWindow(battle)
	battle.Settled.Stage = stageOffensiveReact
	openSettledWindow(battle, "offensive-reaction", stageOffensiveReact, "reaction", []command.Type{command.TypeCommitInteraction, command.TypePass})
	reveals := map[string]any{}
	for _, actorID := range sortedSettledActorIDs(battle) {
		r := battle.Settled.Actors[actorID]
		reveals[actorID] = map[string]any{"dice": r.FinalDice, "roll_history": r.RollHistory, "rolls_used": r.RollsUsed, "max_rolls": r.MaxRolls, "ability_id": r.SelectedAbilityID, "tier_id": r.SelectedTierID, "targets": r.SelectedTargetIDs, "ai_d100": r.AID100, "simulated_rolls": r.AISimulatedRolls}
	}
	return []event.Event{settledEvent(event.TypeInteractionRevealed, battle, "", map[string]any{"commitments": reveals})}, nil
}

func (e Engine) handleOffensiveReactionCommand(battle *state.Battle, library content.BattleLibrary, cmd command.Command) ([]event.Event, error) {
	if cmd.Type == command.TypePass {
		closeSettledWindow(battle)
		return e.finalizeOffensiveSources(battle, library)
	}
	var payload command.CommitInteractionPayload
	if err := command.DecodePayload(cmd, &payload); err != nil {
		return nil, err
	}
	if len(payload.Commitment.CardIDs) != 1 || len(payload.Commitment.PlanningAdjustments) != 1 {
		return nil, errors.New("reaction needs one card and one die adjustment")
	}
	adjust := payload.Commitment.PlanningAdjustments[0]
	if err := e.playSettledCard(battle, library, cmd.ActorID, payload.Commitment.CardIDs[0], nil, "", adjust.DieIndex, ""); err != nil {
		return nil, err
	}
	runtime := battle.Settled.Actors[adjust.ActorID]
	if adjust.DieIndex < 0 || adjust.DieIndex >= len(runtime.FinalDice) {
		return nil, errors.New("reaction die index is invalid")
	}
	die := dieFace(library, runtime.FinalDice[adjust.DieIndex].DieID, adjust.Face)
	runtime.FinalDice[adjust.DieIndex] = state.RolledDie{Index: adjust.DieIndex, DieID: runtime.FinalDice[adjust.DieIndex].DieID, Face: die.Number, Value: die.Number, Symbols: []string{die.Symbol}}
	old := runtime.SelectedAbilityID
	valid := qualifiedAbilities(library, runtime.OffensiveAbilityIDs, runtime.FinalDice, runtime.AbilityModifiers)
	if containsString(valid, old) {
	} else if len(valid) == 1 {
		runtime.SelectedAbilityID = valid[0]
	} else if len(valid) > 1 {
		choice, err := e.namedIntn(battle, "fallback_selection", len(valid))
		if err != nil {
			return nil, err
		}
		runtime.SelectedAbilityID = valid[choice]
	} else {
		runtime.SelectedAbilityID = ""
		runtime.SelectedTierID = ""
	}
	if runtime.SelectedAbilityID != "" {
		tier, _ := qualifiedTier(library.Abilities[runtime.SelectedAbilityID], runtime.FinalDice)
		runtime.SelectedTierID = tier.ID
	}
	runtime.QualifiedAbilityIDs = valid
	battle.Settled.Actors[adjust.ActorID] = runtime
	openSettledWindow(battle, "offensive-reaction", stageOffensiveReact, "reaction", []command.Type{command.TypeCommitInteraction, command.TypePass})
	battle.Settled.Window.ReactionRound = 2
	return []event.Event{settledEvent(event.TypeCardPlayed, battle, cmd.ActorID, map[string]any{"card_instance_id": payload.Commitment.CardIDs[0], "actor_id": adjust.ActorID, "die_index": adjust.DieIndex, "face": adjust.Face, "old_ability": old, "new_ability": runtime.SelectedAbilityID, "valid_abilities": valid})}, nil
}

func (e Engine) finalizeOffensiveSources(battle *state.Battle, library content.BattleLibrary) ([]event.Event, error) {
	battle.Settled.OffensiveSources = nil
	var events []event.Event
	for _, actorID := range sortedSettledActorIDs(battle) {
		runtime := battle.Settled.Actors[actorID]
		if runtime.SelectedAbilityID == "" {
			continue
		}
		ability := library.Abilities[runtime.SelectedAbilityID]
		tier, ok := qualifiedTier(ability, runtime.FinalDice)
		if !ok {
			continue
		}
		ops := append([]content.BattleOperation(nil), tier.Operations...)
		for _, bonus := range ability.Qualification.ConditionalBonuses {
			if requirementsMet(bonus.Requirements, runtime.FinalDice) {
				ops = append(ops, bonus.Operations...)
			}
		}
		for _, modifier := range runtime.AbilityModifiers {
			if modifier.AbilityID != ability.ID {
				continue
			}
			card := library.Cards[runtime.CardInstances[modifier.SourceCardInstanceID].DefinitionID]
			for _, op := range card.Operations {
				if op.Modifier != nil && op.Modifier.AddConditionalBonus != nil && requirementsMet(op.Modifier.AddConditionalBonus.Requirements, runtime.FinalDice) {
					ops = append(ops, op.Modifier.AddConditionalBonus.Operations...)
				}
			}
		}
		source := state.SettledDamageSource{ID: fmt.Sprintf("source-r%d-%s-%s", battle.Segment.Round, actorID, ability.ID), SourceActorID: actorID, SourceContentID: ability.ID, TargetActorID: first(runtime.SelectedTargetIDs)}
		for _, op := range ops {
			switch op.Type {
			case "deal_damage":
				source.BaseAmount, _ = operationAmount(op, 0)
			case "apply_status":
				source.StatusApplications = append(source.StatusApplications, state.SettledStatusApplication{TargetActorID: source.TargetActorID, StatusID: op.StatusID, Stacks: max(1, op.StackCount)})
			case "gain_resource":
				amount, _ := operationAmount(op, 0)
				gainEnergy(battle, actorID, amount)
			}
		}
		if source.BaseAmount > 0 {
			battle.Settled.OffensiveSources = append(battle.Settled.OffensiveSources, source)
			events = append(events, event.NewDamageProposed(state.DamageSourceProposal{ID: source.ID, SourceActorID: actorID, SourceContentID: ability.ID, TargetActorID: source.TargetActorID, BaseAmount: source.BaseAmount}))
		}
	}
	blindEvents, opened, err := e.resolveBlindCheckpoint(battle, library)
	if err != nil {
		return nil, err
	}
	events = append(events, blindEvents...)
	if opened {
		return events, nil
	}
	battle.Settled.Stage = "complete"
	advanced, err := e.advanceSettledSegment(battle)
	return append(events, advanced...), err
}

func (e Engine) resolveBlindCheckpoint(battle *state.Battle, library content.BattleLibrary) ([]event.Event, bool, error) {
	for _, actorID := range sortedSettledActorIDs(battle) {
		for _, status := range append([]state.StatusState(nil), battle.Actors[actorID].Statuses...) {
			definition := library.Statuses[status.DefinitionID]
			for _, trigger := range definition.Triggers {
				if trigger.Segment != "offensive" || trigger.Phase != "exit" {
					continue
				}
				if battle.Settled.Actors[actorID].SelectedAbilityID == "" {
					removeStatus(battle, actorID, status.DefinitionID, 0)
					continue
				}
				for _, op := range trigger.Operations {
					if op.Type != "roll_dice" {
						continue
					}
					die := library.Dice[op.DiceID]
					value, err := e.namedIntn(battle, "status_effect_dice", die.SideCount)
					if err != nil {
						return nil, false, err
					}
					face := die.Faces[value].Number
					if op.ReactionWindow != nil && op.ReactionWindow.Opens {
						battle.Settled.PendingBlind = &state.SettledBlindResolution{ActorID: actorID, StatusID: status.DefinitionID, Face: face}
						battle.Settled.Stage = stageBlindReact
						openSettledWindow(battle, "blind", stageBlindReact, "reaction", []command.Type{command.TypeCommitInteraction, command.TypePass})
						return []event.Event{settledEvent(event.TypeDiceRolled, battle, actorID, map[string]any{"status_id": status.DefinitionID, "face": face})}, true, nil
					}
				}
			}
		}
	}
	return nil, false, nil
}

func (e Engine) handleBlindReactionCommand(battle *state.Battle, library content.BattleLibrary, cmd command.Command) ([]event.Event, error) {
	pending := battle.Settled.PendingBlind
	if pending == nil {
		return nil, errors.New("Blind resolution is missing")
	}
	if cmd.Type == command.TypeCommitInteraction {
		var payload command.CommitInteractionPayload
		if err := command.DecodePayload(cmd, &payload); err != nil {
			return nil, err
		}
		if len(payload.Commitment.CardIDs) != 1 || len(payload.Commitment.PlanningAdjustments) != 1 {
			return nil, errors.New("Blind reaction needs one card and die adjustment")
		}
		adjust := payload.Commitment.PlanningAdjustments[0]
		if err := e.playSettledCard(battle, library, cmd.ActorID, payload.Commitment.CardIDs[0], nil, "", adjust.DieIndex, ""); err != nil {
			return nil, err
		}
		face := dieFace(library, "standard_d6", adjust.Face)
		if face.Number == 0 {
			return nil, errors.New("Blind reaction face is invalid")
		}
		pending.Face = face.Number
		openSettledWindow(battle, "blind", stageBlindReact, "reaction", []command.Type{command.TypeCommitInteraction, command.TypePass})
		battle.Settled.Window.ReactionRound++
		return []event.Event{settledEvent(event.TypeCardPlayed, battle, cmd.ActorID, map[string]any{"card_instance_id": payload.Commitment.CardIDs[0], "blind_face": pending.Face})}, nil
	}
	if cmd.Type != command.TypePass {
		return nil, unsupportedCommand()
	}
	definition := library.Statuses[pending.StatusID]
	for _, trigger := range definition.Triggers {
		for _, op := range trigger.Operations {
			if op.Type != "roll_dice" {
				continue
			}
			for _, outcome := range op.Outcomes {
				if !containsInt(outcome.Faces, pending.Face) {
					continue
				}
				for _, child := range outcome.Operations {
					if child.Type == "cancel_source" {
						runtime := battle.Settled.Actors[pending.ActorID]
						runtime.SelectedAbilityID = ""
						battle.Settled.Actors[pending.ActorID] = runtime
						removeSourcesByActor(battle, pending.ActorID)
					}
				}
			}
		}
	}
	removeStatus(battle, pending.ActorID, pending.StatusID, 0)
	battle.Settled.PendingBlind = nil
	closeSettledWindow(battle)
	battle.Settled.Stage = "complete"
	return e.advanceSettledSegment(battle)
}

func removeSourcesByActor(battle *state.Battle, actorID string) {
	filtered := battle.Settled.OffensiveSources[:0]
	for _, source := range battle.Settled.OffensiveSources {
		if source.SourceActorID != actorID {
			filtered = append(filtered, source)
		}
	}
	battle.Settled.OffensiveSources = filtered
}

func (e Engine) handleStatusReactionCommand(battle *state.Battle, library content.BattleLibrary, cmd command.Command) ([]event.Event, error) {
	if cmd.Type == command.TypePass {
		closeSettledWindow(battle)
		battle.Settled.Stage = stageOngoingCollect
		return e.finalizeTriggerBatch(battle, library)
	}
	var payload command.CommitInteractionPayload
	if err := command.DecodePayload(cmd, &payload); err != nil {
		return nil, err
	}
	if len(payload.Commitment.CardIDs) != 1 {
		return nil, errors.New("status reaction needs one card")
	}
	statusID := payload.Commitment.ChoiceID
	if statusID == "" {
		return nil, errors.New("status reaction target is required")
	}
	if err := e.playSettledCard(battle, library, cmd.ActorID, payload.Commitment.CardIDs[0], nil, "", 0, statusID); err != nil {
		return nil, err
	}
	openSettledWindow(battle, "status-roll", stageOngoingReact, "reaction", []command.Type{command.TypeCommitInteraction, command.TypePass})
	battle.Settled.Window.ReactionRound++
	return []event.Event{settledEvent(event.TypeCardPlayed, battle, cmd.ActorID, map[string]any{"card_instance_id": payload.Commitment.CardIDs[0], "removed_status": statusID})}, nil
}

func (e Engine) handleDamageReactionCommand(battle *state.Battle, library content.BattleLibrary, cmd command.Command) ([]event.Event, error) {
	if cmd.Type == command.TypePass {
		closeSettledWindow(battle)
		return e.finishDamageBatch(battle, library)
	}
	var payload command.CommitInteractionPayload
	if err := command.DecodePayload(cmd, &payload); err != nil {
		return nil, err
	}
	if len(payload.Commitment.CardIDs) != 1 || len(payload.Commitment.ProposalIDs) != 1 {
		return nil, errors.New("damage response needs one card and source")
	}
	if err := e.playSettledCard(battle, library, cmd.ActorID, payload.Commitment.CardIDs[0], payload.Commitment.ProposalIDs, "", 0, ""); err != nil {
		return nil, err
	}
	source := batchSourceByID(battle.Settled.PendingDamage, payload.Commitment.ProposalIDs[0])
	if source == nil {
		return nil, errors.New("damage source was not found")
	}
	source.ReactionPrevention += 3
	reconcileSettledDamage(battle.Settled.PendingDamage, battle)
	openSettledWindow(battle, "damage", battle.Settled.Stage, "damage_response", []command.Type{command.TypeCommitInteraction, command.TypePass})
	battle.Settled.Window.ReactionRound++
	return []event.Event{settledEvent(event.TypeDamageModified, battle, cmd.ActorID, map[string]any{"source_id": source.ID, "prevention": 3})}, nil
}

func (e Engine) handleDefenseSelectionCommand(battle *state.Battle, library content.BattleLibrary, cmd command.Command) ([]event.Event, error) {
	if cmd.Type == command.TypePlanningPass {
		closeSettledWindow(battle)
		return e.afterDefenseSelections(battle, library)
	}
	var payload command.PlanningAbilityPayload
	if err := command.DecodePayload(cmd, &payload); err != nil {
		return nil, err
	}
	runtime := battle.Settled.Actors[cmd.ActorID]
	if !containsString(runtime.DefensiveAbilityIDs, payload.AbilityID) || len(payload.TargetIDs) != 1 {
		return nil, errors.New("select one legal defense and incoming source")
	}
	source := sourceBySettledID(battle, payload.TargetIDs[0])
	if source == nil || source.TargetActorID != cmd.ActorID {
		return nil, errors.New("defense target is not an incoming source")
	}
	ability := library.Abilities[payload.AbilityID]
	if battle.Actors[cmd.ActorID].Resources.EnergyPoints < ability.Cost.Energy {
		return nil, errors.New("insufficient energy")
	}
	spendEnergy(battle, cmd.ActorID, ability.Cost.Energy)
	battle.Settled.DefenseSelections[cmd.ActorID] = state.SettledDefense{ActorID: cmd.ActorID, AbilityID: payload.AbilityID, SourceID: source.ID}
	closeSettledWindow(battle)
	return e.afterDefenseSelections(battle, library)
}

func (e Engine) handleDefenseRollCommand(battle *state.Battle, library content.BattleLibrary, cmd command.Command) ([]event.Event, error) {
	human := cmd.ActorID
	selection := battle.Settled.DefenseSelections[human]
	ability := library.Abilities[selection.AbilityID]
	die := library.Dice[ability.Resolution.Roll.DiceID]
	value, err := e.namedIntn(battle, "defense_dice", die.SideCount)
	if err != nil {
		return nil, err
	}
	selection.RolledFace = die.Faces[value].Number
	battle.Settled.DefenseSelections[human] = selection
	closeSettledWindow(battle)
	return e.resolveDefenseRollsAndOpenReaction(battle, library)
}

func (e Engine) handleHandLimitCommand(battle *state.Battle, cmd command.Command) ([]event.Event, error) {
	var payload command.CommitInteractionPayload
	if err := command.DecodePayload(cmd, &payload); err != nil {
		return nil, err
	}
	actor := battle.Actors[cmd.ActorID]
	need := len(actor.Cards.Hand) - battle.Settled.Actors[cmd.ActorID].HandLimit
	if len(payload.Commitment.CardIDs) != need {
		return nil, fmt.Errorf("must discard exactly %d cards", need)
	}
	for _, id := range payload.Commitment.CardIDs {
		if !containsString(actor.Cards.Hand, id) {
			return nil, errors.New("discard card is not in hand")
		}
		moveCard(&actor.Cards, id, operation.ZoneHand, operation.ZoneDiscard)
	}
	battle.Actors[cmd.ActorID] = actor
	closeSettledWindow(battle)
	battle.Settled.PendingDamage = nil
	battle.Settled.Stage = "complete"
	return e.advanceSettledSegment(battle)
}

func validateSettledPending(cmd command.Command, pending state.PendingInput) error {
	var id string
	switch cmd.Type {
	case command.TypePlanningRoll:
		var p command.PlanningRollPayload
		if err := command.DecodePayload(cmd, &p); err != nil {
			return err
		}
		id = p.PendingInputID
	case command.TypePlanningKeep:
		var p command.PlanningKeepPayload
		if err := command.DecodePayload(cmd, &p); err != nil {
			return err
		}
		id = p.PendingInputID
	case command.TypePlanningReroll:
		var p command.PlanningRerollPayload
		if err := command.DecodePayload(cmd, &p); err != nil {
			return err
		}
		id = p.PendingInputID
	case command.TypePlanningCards:
		var p command.PlanningCardsPayload
		if err := command.DecodePayload(cmd, &p); err != nil {
			return err
		}
		id = p.PendingInputID
	case command.TypePlanningAbility:
		var p command.PlanningAbilityPayload
		if err := command.DecodePayload(cmd, &p); err != nil {
			return err
		}
		id = p.PendingInputID
	case command.TypePlanningTargets:
		var p command.PlanningTargetsPayload
		if err := command.DecodePayload(cmd, &p); err != nil {
			return err
		}
		id = p.PendingInputID
	case command.TypePlanningPass:
		var p command.PlanningPassPayload
		if err := command.DecodePayload(cmd, &p); err != nil {
			return err
		}
		id = p.PendingInputID
	case command.TypeRollDice:
		var p command.RollDicePayload
		if err := command.DecodePayload(cmd, &p); err != nil {
			return err
		}
		id = p.PendingInputID
	case command.TypeCommitInteraction:
		var p command.CommitInteractionPayload
		if err := command.DecodePayload(cmd, &p); err != nil {
			return err
		}
		id = p.PendingInputID
	case command.TypePass:
		var p command.PassPayload
		if err := command.DecodePayload(cmd, &p); err != nil {
			return err
		}
		id = p.PendingInputID
	}
	if id != pending.ID {
		return errors.New("stale pending input")
	}
	return nil
}

func openSettledWindow(battle *state.Battle, prefix, stage, purpose string, allowed []command.Type) {
	runtime := battle.Settled
	runtime.Sequence++
	player := humanActorID(battle)
	id := fmt.Sprintf("%s-r%d-%d", prefix, battle.Segment.Round, runtime.Sequence)
	pendingID := "input-" + id
	runtime.Window = &state.SettledWindow{ID: id, PendingInputID: pendingID, Purpose: purpose, Stage: stage, ReactionRound: 1, RequiredActorID: player, AllowedCommands: append([]command.Type(nil), allowed...), Passes: map[string]bool{}}
	runtime.Stage = stage
	battle.Flow.Stage = stage
	battle.Flow.Iteration++
	battle.Flow.Actors = map[string]state.ActorFlowState{}
	for actorID, actor := range battle.Actors {
		status := state.ActorResolvingAutomatic
		if actor.Controller == state.ControllerHuman {
			status = state.ActorNeedsInput
		}
		battle.Flow.Actors[actorID] = state.ActorFlowState{Status: status, ReasonCode: purpose}
	}
	battle.Flow.PendingInput = map[string]state.PendingInput{player: {ID: pendingID, ActorID: player, Segment: battle.Segment.Current, Phase: state.FlowPhaseInProgress, Stage: stage, Iteration: battle.Flow.Iteration, WindowID: id, ReactionRound: runtime.Window.ReactionRound, PlanningCycle: battle.Segment.Round, InputType: purpose, AllowedCommands: append([]command.Type(nil), allowed...)}}
}
func closeSettledWindow(battle *state.Battle) {
	battle.Settled.Window = nil
	battle.Flow.PendingInput = map[string]state.PendingInput{}
	for actorID, flow := range battle.Flow.Actors {
		flow.Status = state.ActorResolved
		battle.Flow.Actors[actorID] = flow
	}
}
func rotateSettledPending(battle *state.Battle) {
	window := battle.Settled.Window
	if window == nil {
		return
	}
	window.PendingInputID = fmt.Sprintf("input-%s-%d", window.ID, battle.Flow.Iteration+1)
	battle.Flow.Iteration++
	pending := battle.Flow.PendingInput[window.RequiredActorID]
	pending.ID = window.PendingInputID
	pending.Iteration = battle.Flow.Iteration
	battle.Flow.PendingInput[window.RequiredActorID] = pending
}

func (e Engine) rollCombatDice(battle *state.Battle, library content.BattleLibrary, actorID string, indices []int) ([]state.RolledDie, error) {
	runtime := battle.Settled.Actors[actorID]
	dice := cloneDice(runtime.FinalDice)
	if len(indices) == 0 {
		indices = allDieIndices(5)
		dice = make([]state.RolledDie, 5)
	}
	definition := library.Dice["standard_d6"]
	for _, index := range indices {
		value, err := e.namedIntn(battle, "combat_dice", definition.SideCount)
		if err != nil {
			return nil, err
		}
		face := definition.Faces[value]
		dice[index] = state.RolledDie{Index: index, DieID: definition.ID, Face: face.Number, Value: face.Number, Symbols: []string{face.Symbol}}
	}
	return dice, nil
}

func (e Engine) playSettledCard(battle *state.Battle, library content.BattleLibrary, actorID, instanceID string, targetIDs []string, abilityID string, dieIndex int, statusID string) error {
	runtime := battle.Settled.Actors[actorID]
	instance, ok := runtime.CardInstances[instanceID]
	if !ok {
		return errors.New("card instance does not exist")
	}
	if !containsString(battle.Actors[actorID].Cards.Hand, instanceID) {
		return errors.New("card is not in hand")
	}
	definition := library.Cards[instance.DefinitionID]
	purpose := ""
	if battle.Settled.Window != nil {
		purpose = battle.Settled.Window.Purpose
	}
	if purpose == "damage_response" || purpose == "status_response" {
		purpose = "reaction"
	}
	if purpose == "" && battle.Segment.Current == segment.DamageResolution {
		purpose = "reaction"
	}
	legal := false
	for _, timing := range definition.Play.PlayableDuring {
		if timing.Segment == string(battle.Segment.Current) && timing.Phase == "main" && timing.WindowPurpose == purpose {
			legal = true
			break
		}
	}
	if !legal {
		return fmt.Errorf("card %q is not legal during %s/%s", definition.ID, battle.Segment.Current, purpose)
	}
	if battle.Actors[actorID].Resources.EnergyPoints < definition.Cost.Energy {
		return errors.New("insufficient energy")
	}
	spendEnergy(battle, actorID, definition.Cost.Energy)
	actor := battle.Actors[actorID]
	moveCard(&actor.Cards, instanceID, operation.ZoneHand, operation.CardZone(definition.Play.Destination))
	battle.Actors[actorID] = actor
	for _, op := range definition.Operations {
		switch op.Type {
		case "remove_status":
			if statusID == "" {
				return errors.New("status target is required")
			}
			removeStatus(battle, actorID, statusID, 0)
		case "apply_ability_modifier":
			if abilityID == "" && len(targetIDs) > 0 {
				abilityID = targetIDs[0]
			}
			if !containsString(runtime.OffensiveAbilityIDs, abilityID) {
				return errors.New("ability modifier target is invalid")
			}
			runtime.AbilityModifiers = append(runtime.AbilityModifiers, state.RuntimeAbilityModifier{SourceCardInstanceID: instanceID, AbilityID: abilityID, BonusID: op.Modifier.AddConditionalBonus.ID})
		case "modify_die":
			if battle.Settled.Stage == stageOffensivePlan && len(runtime.FinalDice) > 0 {
				if dieIndex < 0 || dieIndex >= len(runtime.FinalDice) {
					return errors.New("die target is invalid")
				}
				face := dieFace(library, runtime.FinalDice[dieIndex].DieID, op.Face)
				runtime.FinalDice[dieIndex] = state.RolledDie{Index: dieIndex, DieID: runtime.FinalDice[dieIndex].DieID, Face: face.Number, Value: face.Number, Symbols: []string{face.Symbol}}
				runtime.QualifiedAbilityIDs = qualifiedAbilities(library, runtime.OffensiveAbilityIDs, runtime.FinalDice, runtime.AbilityModifiers)
			}
		case "draw_cards":
			amount, _ := operationAmount(op, 0)
			for i := 0; i < amount; i++ {
				if _, err := e.drawSettledCard(battle, actorID, "card_draw"); err != nil {
					return err
				}
			}
		case "gain_resource":
			amount, _ := operationAmount(op, 0)
			gainEnergy(battle, actorID, amount)
		}
	}
	battle.Settled.Actors[actorID] = runtime
	return nil
}

func qualifiedAbilities(library content.BattleLibrary, ids []string, dice []state.RolledDie, modifiers []state.RuntimeAbilityModifier) []string {
	var result []string
	for _, id := range ids {
		if _, ok := qualifiedTier(library.Abilities[id], dice); ok {
			result = append(result, id)
		}
	}
	sort.Strings(result)
	return result
}
func qualifiedTier(ability content.BattleAbilityDefinition, dice []state.RolledDie) (content.AbilityTier, bool) {
	if ability.Qualification == nil {
		return content.AbilityTier{}, false
	}
	for _, tier := range ability.Qualification.ActivationTiers {
		if requirementsMet(tier.Requirements, dice) {
			return tier, true
		}
	}
	return content.AbilityTier{}, false
}
func requirementsMet(group content.RequirementGroup, dice []state.RolledDie) bool {
	for _, requirement := range group.All {
		switch requirement.Type {
		case "symbol_count":
			count := 0
			for _, die := range dice {
				if containsString(die.Symbols, requirement.SymbolID) {
					count++
				}
			}
			if requirement.Exact != nil && count != *requirement.Exact {
				return false
			}
			if requirement.Minimum > 0 && count < requirement.Minimum {
				return false
			}
			if requirement.Maximum > 0 && count > requirement.Maximum {
				return false
			}
		case "exact_faces":
			faces := make([]int, len(dice))
			for i, die := range dice {
				faces[i] = die.Face
			}
			sort.Ints(faces)
			want := append([]int(nil), requirement.Faces...)
			sort.Ints(want)
			if !equalInts(faces, want) {
				return false
			}
		case "number_pattern":
			counts := map[int]int{}
			for _, die := range dice {
				counts[die.Face]++
			}
			found := false
			switch requirement.Pattern {
			case "three_of_a_kind":
				for _, count := range counts {
					if count >= 3 {
						found = true
					}
				}
			case "exact_pair":
				for _, count := range counts {
					if count == 2 {
						found = true
					}
				}
			default:
				return false
			}
			if !found {
				return false
			}
		default:
			return false
		}
	}
	return true
}

func operationAmount(op content.BattleOperation, rolled int) (int, error) {
	switch value := op.Amount.(type) {
	case int:
		return value, nil
	case float64:
		return int(value), nil
	case string:
		if value == "rolled_face" {
			return rolled, nil
		}
		return 0, fmt.Errorf("unsupported amount %q", value)
	case nil:
		return 0, nil
	default:
		return 0, fmt.Errorf("unsupported amount type %T", value)
	}
}
func newSettledDamageSource(battle *state.Battle, sourceActor, targetActor, contentID string, amount int) state.SettledDamageSource {
	battle.Settled.Sequence++
	return state.SettledDamageSource{ID: fmt.Sprintf("source-r%d-%s-%d", battle.Segment.Round, contentID, battle.Settled.Sequence), SourceActorID: sourceActor, SourceContentID: contentID, TargetActorID: targetActor, BaseAmount: amount}
}
func applyStatus(battle *state.Battle, library content.BattleLibrary, actorID, statusID string, stacks int) {
	actor := battle.Actors[actorID]
	for i, status := range actor.Statuses {
		if status.DefinitionID == statusID {
			status.Stacks = min(library.Statuses[statusID].Stacking.StackLimit, status.Stacks+stacks)
			actor.Statuses[i] = status
			battle.Actors[actorID] = actor
			return
		}
	}
	battle.Settled.Sequence++
	actor.Statuses = append(actor.Statuses, state.StatusState{InstanceID: fmt.Sprintf("%s-status-%s-%d", actorID, statusID, battle.Settled.Sequence), DefinitionID: statusID, Stacks: min(library.Statuses[statusID].Stacking.StackLimit, stacks)})
	battle.Actors[actorID] = actor
}
func removeStatus(battle *state.Battle, actorID, statusID string, stacks int) {
	actor := battle.Actors[actorID]
	for i, status := range actor.Statuses {
		if status.DefinitionID != statusID {
			continue
		}
		if stacks > 0 && status.Stacks > stacks {
			actor.Statuses[i].Stacks -= stacks
		} else {
			actor.Statuses = append(actor.Statuses[:i], actor.Statuses[i+1:]...)
		}
		battle.Actors[actorID] = actor
		return
	}
}
func findStatusInstance(battle *state.Battle, instanceID string) (string, string) {
	for actorID, actor := range battle.Actors {
		for _, status := range actor.Statuses {
			if status.InstanceID == instanceID {
				return actorID, status.DefinitionID
			}
		}
	}
	return "", ""
}
func moveCard(zones *state.CardZones, id string, from, to operation.CardZone) {
	source := zoneCards(zonesValue(zones), from)
	index := indexString(source, id)
	if index < 0 {
		return
	}
	source = append(source[:index], source[index+1:]...)
	setZone(zones, from, source)
	target := zoneCards(zonesValue(zones), to)
	target = append(target, id)
	setZone(zones, to, target)
}
func zonesValue(z *state.CardZones) state.CardZones { return *z }
func zoneCards(z state.CardZones, zone operation.CardZone) []string {
	switch zone {
	case operation.ZoneDeck:
		return append([]string(nil), z.Deck...)
	case operation.ZoneHand:
		return append([]string(nil), z.Hand...)
	case operation.ZoneDiscard:
		return append([]string(nil), z.Discard...)
	case operation.ZoneRemoved:
		return append([]string(nil), z.Removed...)
	}
	return nil
}
func setZone(z *state.CardZones, zone operation.CardZone, cards []string) {
	switch zone {
	case operation.ZoneDeck:
		z.Deck = cards
	case operation.ZoneHand:
		z.Hand = cards
	case operation.ZoneDiscard:
		z.Discard = cards
	case operation.ZoneRemoved:
		z.Removed = cards
	}
}
func (e Engine) drawSettledCard(battle *state.Battle, actorID, stream string) (string, error) {
	actor := battle.Actors[actorID]
	if len(actor.Cards.Deck) == 0 {
		if len(actor.Cards.Discard) == 0 {
			return "", nil
		}
		actor.Cards.Deck = append(actor.Cards.Deck, actor.Cards.Discard...)
		actor.Cards.Discard = nil
	}
	index, err := e.namedIntn(battle, stream, len(actor.Cards.Deck))
	if err != nil {
		return "", err
	}
	card := actor.Cards.Deck[index]
	actor.Cards.Deck = append(actor.Cards.Deck[:index], actor.Cards.Deck[index+1:]...)
	actor.Cards.Hand = append(actor.Cards.Hand, card)
	battle.Actors[actorID] = actor
	return card, nil
}
func spendEnergy(battle *state.Battle, actorID string, amount int) {
	actor := battle.Actors[actorID]
	actor.Resources.EnergyPoints -= amount
	actor.EnergyPoints = actor.Resources.EnergyPoints
	battle.Actors[actorID] = actor
}
func gainEnergy(battle *state.Battle, actorID string, amount int) {
	actor := battle.Actors[actorID]
	actor.Resources.EnergyPoints += amount
	actor.EnergyPoints = actor.Resources.EnergyPoints
	battle.Actors[actorID] = actor
}
func reconcileSettledDamage(batch *state.SettledDamageBatch, battle *state.Battle) {
	if batch == nil {
		return
	}
	for i := range batch.Sources {
		batch.Sources[i].FinalAmount = max(0, batch.Sources[i].BaseAmount-batch.Sources[i].Prevention)
		if batch.Sources[i].ScaleDenominator > 0 {
			batch.Sources[i].FinalAmount = batch.Sources[i].FinalAmount * batch.Sources[i].ScaleNumerator / batch.Sources[i].ScaleDenominator
		}
		batch.Sources[i].FinalAmount = max(0, batch.Sources[i].FinalAmount-batch.Sources[i].ReactionPrevention)
	}
	desired := map[string]int{}
	for _, source := range batch.Sources {
		desired[source.TargetActorID] += source.FinalAmount
	}
	for target, total := range desired {
		health := battle.Actors[target].CurrentHealth()
		overage := max(0, total-health)
		batch.Overage[target] = overage
		cards := min(total, health)
		count := 0
		for i := range batch.Removals {
			if batch.Removals[i].TargetActorID != target {
				continue
			}
			count++
			if count > cards {
				batch.Removals[i].Accepted = false
				batch.Removals[i].Released = true
			} else {
				batch.Removals[i].Accepted = true
				batch.Removals[i].Released = false
			}
		}
	}
}
func sourceBySettledID(battle *state.Battle, id string) *state.SettledDamageSource {
	for i := range battle.Settled.OffensiveSources {
		if battle.Settled.OffensiveSources[i].ID == id {
			return &battle.Settled.OffensiveSources[i]
		}
	}
	return nil
}
func batchSourceByID(batch *state.SettledDamageBatch, id string) *state.SettledDamageSource {
	if batch == nil {
		return nil
	}
	for i := range batch.Sources {
		if batch.Sources[i].ID == id {
			return &batch.Sources[i]
		}
	}
	return nil
}
func firstBatchSourceForTarget(batch *state.SettledDamageBatch, target string) *state.SettledDamageSource {
	if batch == nil {
		return nil
	}
	for i := range batch.Sources {
		if batch.Sources[i].TargetActorID == target {
			return &batch.Sources[i]
		}
	}
	return nil
}
func hasIncoming(battle *state.Battle, actorID string) bool {
	return firstIncoming(battle, actorID) != nil
}
func firstIncoming(battle *state.Battle, actorID string) *state.SettledDamageSource {
	for i := range battle.Settled.OffensiveSources {
		if battle.Settled.OffensiveSources[i].TargetActorID == actorID {
			return &battle.Settled.OffensiveSources[i]
		}
	}
	return nil
}
func damageForTarget(batch *state.SettledDamageBatch, actorID string) int {
	total := 0
	for _, s := range batch.Sources {
		if s.TargetActorID == actorID {
			total += s.FinalAmount
		}
	}
	return total
}
func findCardDefinitionInZone(battle *state.Battle, actorID, definitionID string, zone operation.CardZone) string {
	runtime := battle.Settled.Actors[actorID]
	for _, id := range zoneCards(battle.Actors[actorID].Cards, zone) {
		if runtime.CardInstances[id].DefinitionID == definitionID {
			return id
		}
	}
	return ""
}
func dieFace(library content.BattleLibrary, dieID string, number int) content.BattleDieFace {
	for _, face := range library.Dice[dieID].Faces {
		if face.Number == number {
			return face
		}
	}
	return content.BattleDieFace{}
}
func rolledFaces(library content.BattleLibrary, dieID string, faces []int) []state.RolledDie {
	result := make([]state.RolledDie, len(faces))
	for i, number := range faces {
		face := dieFace(library, dieID, number)
		result[i] = state.RolledDie{Index: i, DieID: dieID, Face: number, Value: number, Symbols: []string{face.Symbol}}
	}
	return result
}
func diceEvent(battle *state.Battle, actorID string, runtime state.SettledActorRuntime, indices []int) event.Event {
	return event.Event{Type: event.TypeDiceRolled, ActorID: actorID, Segment: segment.Offensive, Pool: state.RollPoolOffensive, Dice: cloneDice(runtime.FinalDice), RolledIndices: append([]int(nil), indices...), RollsUsed: runtime.RollsUsed, MaxRolls: runtime.MaxRolls, SymbolCounts: symbolCounts(runtime.FinalDice), Data: map[string]any{"roll_history": runtime.RollHistory, "qualified_abilities": runtime.QualifiedAbilityIDs, "kept_indices": runtime.KeptIndices}}
}
func damageRevealEvent(battle *state.Battle, batch *state.SettledDamageBatch) event.Event {
	return settledEvent(event.TypeDamageCardsRevealed, battle, "", map[string]any{"batch_id": batch.ID, "sources": batch.Sources, "cards": batch.Removals, "overage": batch.Overage})
}
func settledEvent(kind event.Type, battle *state.Battle, actorID string, data map[string]any) event.Event {
	return event.Event{Type: kind, ActorID: actorID, Segment: battle.Segment.Current, Round: battle.Segment.Round, Data: data}
}
func symbolCounts(dice []state.RolledDie) map[string]int {
	result := map[string]int{}
	for _, die := range dice {
		for _, symbol := range die.Symbols {
			result[symbol]++
		}
	}
	return result
}
func sortedSettledActorIDs(battle *state.Battle) []string {
	ids := make([]string, 0, len(battle.Actors))
	for id := range battle.Actors {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	return ids
}
func humanActorID(battle *state.Battle) string {
	for _, id := range sortedSettledActorIDs(battle) {
		if battle.Actors[id].Controller == state.ControllerHuman {
			return id
		}
	}
	return ""
}
func allDieIndices(count int) []int {
	result := make([]int, count)
	for i := range result {
		result[i] = i
	}
	return result
}
func cloneDice(values []state.RolledDie) []state.RolledDie {
	result := make([]state.RolledDie, len(values))
	for i, value := range values {
		result[i] = value
		result[i].Symbols = append([]string(nil), value.Symbols...)
	}
	return result
}
func validateIndices(dice []state.RolledDie, indices []int) error {
	seen := map[int]bool{}
	for _, index := range indices {
		if index < 0 || index >= len(dice) || seen[index] {
			return errors.New("invalid die indices")
		}
		seen[index] = true
	}
	return nil
}
func validateRerollIndices(runtime state.SettledActorRuntime, indices []int) error {
	if len(indices) == 0 {
		return errors.New("reroll indices are required")
	}
	if err := validateIndices(runtime.FinalDice, indices); err != nil {
		return err
	}
	for _, index := range indices {
		if containsInt(runtime.KeptIndices, index) {
			return errors.New("kept dice cannot be rerolled")
		}
	}
	return nil
}
func containsCommand(values []command.Type, target command.Type) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
}
func indexString(values []string, target string) int {
	for i, value := range values {
		if value == target {
			return i
		}
	}
	return -1
}
func containsInt(values []int, target int) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
}
func equalInts(a, b []int) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
func first(values []string) string {
	if len(values) == 0 {
		return ""
	}
	return values[0]
}
func nonEmpty(value string) []string {
	if value == "" {
		return nil
	}
	return []string{value}
}
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}
