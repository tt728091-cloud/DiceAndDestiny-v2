package engine

import (
	"errors"
	"fmt"

	"diceanddestiny/server/internal/battle/state"
	"diceanddestiny/server/internal/content"
)

// effectContext describes activation-specific inputs. Cards, abilities, and
// statuses all feed the same operation language through this context.
type effectContext struct {
	SourceActorID      string
	SourceContentID    string
	SourceContentType  string
	TargetActorIDs     []string
	ProposalIDs        []string
	SelectedAbilityID  string
	SelectedStatusID   string
	StatusInstanceID   string
	TriggerID          string
	StatusStacks       int
	SelectedDieActorID string
	SelectedDieIndex   int
	RolledFace         int
	DeferReactableRoll bool
}

type effectResult struct {
	Damage             []state.SettledDamageSource
	StatusApplications []state.SettledStatusApplication
	StatusRemovals     []state.SettledStatusRemoval
	Rolls              []state.SettledEffectRoll
	ResourceDeltas     map[string]int
	DrawCounts         map[string]int
	MaxRollDeltas      map[string]int
	AbilityModifiers   []effectAbilityModifier
	DieChanges         []effectDieChange
	Preventions        []effectPrevention
	Scales             []effectScale
	CanceledActors     []string
	Reactable          bool
}

type effectAbilityModifier struct {
	ActorID, AbilityID string
	Modifier           *content.AbilityModifier
}

type effectDieChange struct {
	ActorID string
	Index   int
	Face    int
}

type effectPrevention struct {
	ProposalID string
	Amount     int
}

type effectScale struct {
	ProposalID             string
	Numerator, Denominator int
}

func (r *effectResult) merge(other effectResult) {
	r.Damage = append(r.Damage, other.Damage...)
	r.StatusApplications = append(r.StatusApplications, other.StatusApplications...)
	r.StatusRemovals = append(r.StatusRemovals, other.StatusRemovals...)
	r.Rolls = append(r.Rolls, other.Rolls...)
	r.AbilityModifiers = append(r.AbilityModifiers, other.AbilityModifiers...)
	r.DieChanges = append(r.DieChanges, other.DieChanges...)
	r.Preventions = append(r.Preventions, other.Preventions...)
	r.Scales = append(r.Scales, other.Scales...)
	r.CanceledActors = append(r.CanceledActors, other.CanceledActors...)
	r.Reactable = r.Reactable || other.Reactable
	mergeIntMap(&r.ResourceDeltas, other.ResourceDeltas)
	mergeIntMap(&r.DrawCounts, other.DrawCounts)
	mergeIntMap(&r.MaxRollDeltas, other.MaxRollDeltas)
}

func mergeIntMap(target *map[string]int, source map[string]int) {
	if len(source) == 0 {
		return
	}
	if *target == nil {
		*target = map[string]int{}
	}
	for key, value := range source {
		(*target)[key] += value
	}
}

// executeEffects is the single recursive interpreter for the battle_v1
// operation language. The caller decides when accumulated proposals are
// committed, but operation meaning is independent of its card/ability/status
// activation source.
func (e Engine) executeEffects(battle *state.Battle, library content.BattleLibrary, ctx effectContext, operations []content.BattleOperation) (effectResult, error) {
	result := effectResult{}
	for operationIndex, op := range operations {
		repeat := 1
		if op.Type != "roll_dice" && (op.OnePerStatusStack || op.Repeat == "one_per_status_stack") {
			repeat = ctx.StatusStacks
		}
		for n := 0; n < repeat; n++ {
			opResult, err := e.executeEffect(battle, library, ctx, op, operationIndex)
			if err != nil {
				return effectResult{}, fmt.Errorf("%s %q operation %d (%s): %w", ctx.SourceContentType, ctx.SourceContentID, operationIndex, op.Type, err)
			}
			result.merge(opResult)
		}
	}
	return result, nil
}

// executeResolvedEffects evaluates roll outcomes using a face already exposed
// through an interaction window. It uses the same child operation interpreter
// as immediate rolls and prevents defense/reaction code from redefining dice
// outcome semantics.
func (e Engine) executeResolvedEffects(battle *state.Battle, library content.BattleLibrary, ctx effectContext, operations []content.BattleOperation) (effectResult, error) {
	result := effectResult{}
	for _, op := range operations {
		if op.Type != "roll_dice" {
			part, err := e.executeEffects(battle, library, ctx, []content.BattleOperation{op})
			if err != nil {
				return effectResult{}, err
			}
			result.merge(part)
			continue
		}
		matched := false
		for _, outcome := range op.Outcomes {
			if !containsInt(outcome.Faces, ctx.RolledFace) {
				continue
			}
			matched = true
			part, err := e.executeEffects(battle, library, ctx, outcome.Operations)
			if err != nil {
				return effectResult{}, err
			}
			result.merge(part)
		}
		if !matched {
			return effectResult{}, fmt.Errorf("roll_dice has no outcome for face %d", ctx.RolledFace)
		}
	}
	return result, nil
}

func (e Engine) executeEffect(battle *state.Battle, library content.BattleLibrary, ctx effectContext, op content.BattleOperation, operationIndex int) (effectResult, error) {
	result := effectResult{}
	targets, err := effectTargets(battle, ctx, op.Target)
	if err != nil {
		return result, err
	}
	switch op.Type {
	case "noop":
		return result, nil
	case "deal_damage":
		amount, err := operationAmount(op, ctx.RolledFace)
		if err != nil {
			return result, err
		}
		for _, targetID := range targets {
			result.Damage = append(result.Damage, newSettledDamageSource(battle, ctx.SourceActorID, targetID, ctx.SourceContentID, amount))
		}
	case "apply_status":
		for _, targetID := range targets {
			result.StatusApplications = append(result.StatusApplications, state.SettledStatusApplication{TargetActorID: targetID, StatusID: op.StatusID, Stacks: max(1, op.StackCount)})
		}
	case "remove_status", "remove_status_stack":
		statusID := op.StatusID
		if statusID == "" {
			statusID = ctx.SelectedStatusID
		}
		if statusID == "" && ctx.SourceContentType == "status" {
			statusID = ctx.SourceContentID
		}
		if statusID == "" {
			return result, errors.New("status target is required")
		}
		stacks := 0
		if op.Type == "remove_status_stack" {
			stacks = max(1, op.StackCount)
		}
		for _, targetID := range targets {
			result.StatusRemovals = append(result.StatusRemovals, state.SettledStatusRemoval{ActorID: targetID, StatusID: statusID, Stacks: stacks})
		}
	case "gain_resource":
		if op.Resource != "energy" {
			return result, fmt.Errorf("unsupported resource %q", op.Resource)
		}
		amount, err := operationAmount(op, ctx.RolledFace)
		if err != nil {
			return result, err
		}
		result.ResourceDeltas = map[string]int{}
		for _, targetID := range targets {
			result.ResourceDeltas[targetID] += amount
		}
	case "draw_cards":
		amount, err := operationAmount(op, ctx.RolledFace)
		if err != nil {
			return result, err
		}
		result.DrawCounts = map[string]int{}
		for _, targetID := range targets {
			result.DrawCounts[targetID] += amount
		}
	case "adjust_max_rolls":
		amount, err := operationAmount(op, ctx.RolledFace)
		if err != nil {
			return result, err
		}
		result.MaxRollDeltas = map[string]int{}
		for _, targetID := range targets {
			result.MaxRollDeltas[targetID] += amount
		}
	case "apply_ability_modifier":
		abilityID := ctx.SelectedAbilityID
		if abilityID == "" {
			return result, errors.New("ability target is required")
		}
		result.AbilityModifiers = append(result.AbilityModifiers, effectAbilityModifier{ActorID: ctx.SourceActorID, AbilityID: abilityID, Modifier: op.Modifier})
	case "modify_die":
		actorID := ctx.SelectedDieActorID
		if actorID == "" {
			actorID = ctx.SourceActorID
		}
		result.DieChanges = append(result.DieChanges, effectDieChange{ActorID: actorID, Index: ctx.SelectedDieIndex, Face: op.Face})
	case "prevent_damage":
		amount, err := operationAmount(op, ctx.RolledFace)
		if err != nil {
			return result, err
		}
		for _, proposalID := range ctx.ProposalIDs {
			result.Preventions = append(result.Preventions, effectPrevention{ProposalID: proposalID, Amount: amount})
		}
	case "scale_damage":
		for _, proposalID := range ctx.ProposalIDs {
			result.Scales = append(result.Scales, effectScale{ProposalID: proposalID, Numerator: op.Numerator, Denominator: op.Denominator})
		}
	case "cancel_source":
		result.CanceledActors = append(result.CanceledActors, targets...)
	case "roll_dice":
		die, ok := library.Dice[op.DiceID]
		if !ok || die.SideCount < 1 || len(die.Faces) != die.SideCount {
			return result, fmt.Errorf("invalid dice %q", op.DiceID)
		}
		count := op.DiceCount
		if count == 0 {
			count = 1
		}
		if op.OnePerStatusStack || op.Repeat == "one_per_status_stack" {
			count = ctx.StatusStacks
		}
		rollActors := targets
		if len(rollActors) == 0 {
			rollActors = []string{ctx.SourceActorID}
		}
		for _, rollActorID := range rollActors {
			for rollIndex := 0; rollIndex < count; rollIndex++ {
				value, err := e.namedIntn(battle, "effect_dice", die.SideCount)
				if err != nil {
					return result, err
				}
				face := die.Faces[value]
				roll := state.SettledEffectRoll{
					ActorID: rollActorID, SourceContentType: ctx.SourceContentType,
					SourceContentID: ctx.SourceContentID, StatusInstanceID: ctx.StatusInstanceID,
					TriggerID:   ctx.TriggerID,
					OperationID: op.ID, OperationIndex: operationIndex,
					Die: state.RolledDie{Index: rollIndex, DieID: die.ID, Face: face.Number, Value: face.Number, Symbols: []string{face.Symbol}},
				}
				result.Rolls = append(result.Rolls, roll)
				if op.ReactionWindow != nil && op.ReactionWindow.Opens {
					result.Reactable = true
					if ctx.DeferReactableRoll {
						continue
					}
				}
				childContext := ctx
				childContext.RolledFace = face.Number
				for _, outcome := range op.Outcomes {
					if !containsInt(outcome.Faces, face.Number) {
						continue
					}
					children, err := e.executeEffects(battle, library, childContext, outcome.Operations)
					if err != nil {
						return result, err
					}
					result.merge(children)
				}
			}
		}
	default:
		return result, fmt.Errorf("unsupported operation type %q", op.Type)
	}
	return result, nil
}

func effectTargets(battle *state.Battle, ctx effectContext, target string) ([]string, error) {
	switch target {
	case "", "self", "source_actor":
		return []string{ctx.SourceActorID}, nil
	case "selected_targets", "target_actor":
		if len(ctx.TargetActorIDs) == 0 {
			return nil, errors.New("actor target is required")
		}
		for _, actorID := range ctx.TargetActorIDs {
			if _, ok := battle.Actors[actorID]; !ok {
				return nil, fmt.Errorf("actor target %q does not exist", actorID)
			}
		}
		return append([]string(nil), ctx.TargetActorIDs...), nil
	case "selected_proposal":
		if len(ctx.ProposalIDs) == 0 {
			return nil, errors.New("damage proposal target is required")
		}
		return nil, nil
	case "selected_ability", "selected_offensive_ability", "selected_die", "selected_status":
		return []string{ctx.SourceActorID}, nil
	default:
		return nil, fmt.Errorf("unsupported operation target %q", target)
	}
}

func (e Engine) applyEffectMutations(battle *state.Battle, library content.BattleLibrary, sourceCardInstanceID string, result effectResult) error {
	for actorID, delta := range result.ResourceDeltas {
		gainEnergy(battle, actorID, delta)
	}
	for actorID, count := range result.DrawCounts {
		for n := 0; n < count; n++ {
			if _, err := e.drawSettledCard(battle, actorID, "card_draw"); err != nil {
				return err
			}
		}
	}
	for actorID, delta := range result.MaxRollDeltas {
		runtime := battle.Settled.Actors[actorID]
		runtime.MaxRolls = max(1, runtime.MaxRolls+delta)
		battle.Settled.Actors[actorID] = runtime
	}
	for _, modifier := range result.AbilityModifiers {
		if modifier.Modifier == nil || modifier.Modifier.AddConditionalBonus == nil {
			return errors.New("ability modifier definition is missing")
		}
		runtime := battle.Settled.Actors[modifier.ActorID]
		if !containsString(runtime.OffensiveAbilityIDs, modifier.AbilityID) {
			return fmt.Errorf("ability modifier target %q is invalid", modifier.AbilityID)
		}
		runtime.AbilityModifiers = append(runtime.AbilityModifiers, state.RuntimeAbilityModifier{SourceCardInstanceID: sourceCardInstanceID, AbilityID: modifier.AbilityID, BonusID: modifier.Modifier.AddConditionalBonus.ID})
		battle.Settled.Actors[modifier.ActorID] = runtime
	}
	for _, change := range result.DieChanges {
		if battle.Settled.Stage == stageBlindReact && battle.Settled.PendingBlind != nil {
			face := dieFace(library, battle.Settled.PendingBlind.DieID, change.Face)
			if face.Number == 0 {
				return errors.New("effect die face is invalid")
			}
			battle.Settled.PendingBlind.Face = face.Number
			continue
		}
		runtime := battle.Settled.Actors[change.ActorID]
		if change.Index < 0 || change.Index >= len(runtime.FinalDice) {
			return errors.New("die target is invalid")
		}
		face := dieFace(library, runtime.FinalDice[change.Index].DieID, change.Face)
		if face.Number == 0 {
			return errors.New("die face is invalid")
		}
		runtime.FinalDice[change.Index] = state.RolledDie{Index: change.Index, DieID: runtime.FinalDice[change.Index].DieID, Face: face.Number, Value: face.Number, Symbols: []string{face.Symbol}}
		runtime.QualifiedAbilityIDs = qualifiedAbilities(library, runtime.OffensiveAbilityIDs, runtime.FinalDice, runtime.AbilityModifiers)
		battle.Settled.Actors[change.ActorID] = runtime
	}
	for _, application := range result.StatusApplications {
		applyStatus(battle, library, application.TargetActorID, application.StatusID, application.Stacks)
	}
	for _, removal := range result.StatusRemovals {
		removeStatus(battle, removal.ActorID, removal.StatusID, removal.Stacks)
	}
	for _, prevention := range result.Preventions {
		source := effectDamageSourceByID(battle, prevention.ProposalID)
		if source == nil {
			return fmt.Errorf("damage source %q was not found", prevention.ProposalID)
		}
		if sourceCardInstanceID != "" {
			source.ReactionPrevention += prevention.Amount
		} else {
			source.Prevention += prevention.Amount
		}
	}
	for _, scale := range result.Scales {
		source := effectDamageSourceByID(battle, scale.ProposalID)
		if source == nil {
			return fmt.Errorf("damage source %q was not found", scale.ProposalID)
		}
		source.ScaleNumerator, source.ScaleDenominator = scale.Numerator, scale.Denominator
	}
	for _, actorID := range result.CanceledActors {
		runtime := battle.Settled.Actors[actorID]
		runtime.SelectedAbilityID = ""
		battle.Settled.Actors[actorID] = runtime
		removeSourcesByActor(battle, actorID)
	}
	if len(result.Preventions) > 0 || len(result.Scales) > 0 {
		reconcileSettledDamage(battle.Settled.PendingDamage, battle)
	}
	return nil
}

func effectDamageSourceByID(battle *state.Battle, id string) *state.SettledDamageSource {
	if source := batchSourceByID(battle.Settled.PendingDamage, id); source != nil {
		return source
	}
	return sourceBySettledID(battle, id)
}

func consolidateEffectDamage(damage []state.SettledDamageSource, applications []state.SettledStatusApplication) []state.SettledDamageSource {
	var result []state.SettledDamageSource
	indices := map[string]int{}
	for _, source := range damage {
		key := source.SourceActorID + "\x00" + source.SourceContentID + "\x00" + source.TargetActorID
		if index, ok := indices[key]; ok {
			result[index].BaseAmount += source.BaseAmount
			continue
		}
		indices[key] = len(result)
		result = append(result, source)
	}
	for _, application := range applications {
		for i := range result {
			if result[i].TargetActorID == application.TargetActorID {
				result[i].StatusApplications = append(result[i].StatusApplications, application)
				break
			}
		}
	}
	return result
}
