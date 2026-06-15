package damage

import (
	"errors"
	"fmt"
	"sort"

	"diceanddestiny/server/internal/battle/operation"
	"diceanddestiny/server/internal/battle/state"
)

type RandomSource interface {
	Intn(maxExclusive int) (int, error)
}

type CardRemovalResult struct {
	Proposal state.ProposedCardRemoval
}

type CommitResult struct {
	Sources []state.DamageSourceProposal
	Totals  []state.AccumulatedDamageProposal
	Cards   []CardRemovalResult
}

func Collect(
	battle *state.Battle,
	registry *operation.RuntimeRegistry,
	resolution *state.DamageResolutionState,
) error {
	if battle == nil || resolution == nil {
		return errors.New("battle and damage resolution are required")
	}
	if registry == nil {
		return errors.New("operation runtime registry is required")
	}

	resolution.SourceProposals = nil
	resolution.ModifierProposals = nil
	resolution.PendingOperations = nil
	for _, planning := range appendPlanningProposals(battle.OffensiveProposals, battle.DefensiveProposals) {
		for _, finalized := range planning.Operations {
			if finalized.Operation.Type == operation.TypeNoop {
				continue
			}
			if !registry.Supports(finalized.Operation.Type) {
				resolution.PendingOperations = append(
					resolution.PendingOperations,
					cloneFinalizedOperation(finalized),
				)
				continue
			}
			runtimeProposals, err := registry.Execute(operation.RuntimeContext{
				ProposalID:               finalized.ID,
				SourcePlanningProposalID: planning.ID,
				SourceActorID:            finalized.SourceActorID,
				SourceContentType:        finalized.ContentType,
				SourceContentID:          finalized.ContentID,
				SelectedTargets:          finalized.SelectedTargets,
			}, finalized.Operation)
			if err != nil {
				return fmt.Errorf("execute operation proposal %q: %w", finalized.ID, err)
			}
			for _, proposal := range runtimeProposals {
				switch proposal.Type {
				case operation.TypeDealDamage:
					if _, ok := battle.Actors[proposal.TargetActorID]; !ok {
						return fmt.Errorf("damage target actor %q is not in battle", proposal.TargetActorID)
					}
					resolution.SourceProposals = append(resolution.SourceProposals, state.DamageSourceProposal{
						ID:                       proposal.ID,
						SourcePlanningProposalID: proposal.SourcePlanningProposalID,
						SourceActorID:            proposal.SourceActorID,
						SourceContentType:        proposal.SourceContentType,
						SourceContentID:          proposal.SourceContentID,
						TargetActorID:            proposal.TargetActorID,
						BaseAmount:               proposal.Amount,
						OriginatingOperation:     cloneFinalizedOperation(finalized),
					})
				case operation.TypePreventDamage:
					resolution.ModifierProposals = append(resolution.ModifierProposals, state.DamageModifierProposal{
						ID:                       proposal.ID,
						SourcePlanningProposalID: proposal.SourcePlanningProposalID,
						SourceActorID:            proposal.SourceActorID,
						SourceContentType:        proposal.SourceContentType,
						SourceContentID:          proposal.SourceContentID,
						TargetProposalIDs:        append([]string(nil), proposal.TargetProposalIDs...),
						Amount:                   proposal.Amount,
						OriginatingOperation:     cloneFinalizedOperation(finalized),
					})
				}
			}
		}
	}
	applyDefensiveModifiers(resolution)
	Recalculate(resolution)
	return nil
}

func Recalculate(resolution *state.DamageResolutionState) {
	if resolution == nil {
		return
	}
	previousPrevention := make(map[string]int, len(resolution.AccumulatedProposals))
	for _, proposal := range resolution.AccumulatedProposals {
		previousPrevention[proposal.ID] = proposal.ReactionPrevention
	}

	totals := make(map[string]*state.AccumulatedDamageProposal)
	targetOrder := make([]string, 0)
	for i := range resolution.SourceProposals {
		source := &resolution.SourceProposals[i]
		source.FinalAmount = clampNonNegative(
			source.BaseAmount -
				source.DefensivePrevention -
				source.ReactionPrevention +
				source.ReactionModification,
		)
		total, ok := totals[source.TargetActorID]
		if !ok {
			targetOrder = append(targetOrder, source.TargetActorID)
			total = &state.AccumulatedDamageProposal{
				ID:            accumulatedProposalID(source.TargetActorID),
				TargetActorID: source.TargetActorID,
			}
			total.ReactionPrevention = previousPrevention[total.ID]
			totals[source.TargetActorID] = total
		}
		total.SourceProposalIDs = append(total.SourceProposalIDs, source.ID)
		total.BaseAmount += source.BaseAmount
		total.FinalAmount += source.FinalAmount
	}
	sort.Strings(targetOrder)
	resolution.AccumulatedProposals = make([]state.AccumulatedDamageProposal, 0, len(targetOrder))
	for _, targetID := range targetOrder {
		total := totals[targetID]
		remaining := total.ReactionPrevention
		for i := range resolution.SourceProposals {
			source := &resolution.SourceProposals[i]
			if source.TargetActorID != targetID || remaining == 0 {
				continue
			}
			prevented := min(remaining, source.FinalAmount)
			source.FinalAmount -= prevented
			remaining -= prevented
		}
		total.FinalAmount = 0
		for _, source := range resolution.SourceProposals {
			if source.TargetActorID == targetID {
				total.FinalAmount += source.FinalAmount
			}
		}
		resolution.AccumulatedProposals = append(resolution.AccumulatedProposals, *total)
	}
	resolution.Revision++
}

func ReconcileCards(
	battle *state.Battle,
	resolution *state.DamageResolutionState,
	random RandomSource,
) ([]state.ProposedCardRemoval, error) {
	if battle == nil || resolution == nil {
		return nil, errors.New("battle and damage resolution are required")
	}
	if random == nil {
		return nil, errors.New("damage card random source is required")
	}

	var newlySelected []state.ProposedCardRemoval
	for _, total := range resolution.AccumulatedProposals {
		actor := battle.Actors[total.TargetActorID]
		desired := min(total.FinalAmount, actor.CurrentHealth())
		activeIndices := activeCardProposalIndices(resolution.CardProposals, total.TargetActorID)
		for len(activeIndices) > desired {
			index := activeIndices[len(activeIndices)-1]
			resolution.CardProposals[index].Accepted = false
			resolution.CardProposals[index].Released = true
			activeIndices = activeIndices[:len(activeIndices)-1]
		}
		if len(activeIndices) >= desired {
			continue
		}
		needed := desired - len(activeIndices)
		selected, err := selectAdditionalCards(
			actor.Cards,
			resolution.CardProposals,
			total,
			sourceActorsForTotal(resolution.SourceProposals, total),
			needed,
			random,
		)
		if err != nil {
			return nil, err
		}
		resolution.CardProposals = append(resolution.CardProposals, selected...)
		newlySelected = append(newlySelected, selected...)
	}
	return newlySelected, nil
}

func RevealCards(resolution *state.DamageResolutionState) []state.ProposedCardRemoval {
	if resolution == nil {
		return nil
	}
	var revealed []state.ProposedCardRemoval
	for i := range resolution.CardProposals {
		if !resolution.CardProposals[i].Accepted || resolution.CardProposals[i].Revealed {
			continue
		}
		resolution.CardProposals[i].Revealed = true
		revealed = append(revealed, resolution.CardProposals[i])
	}
	if len(revealed) > 0 {
		resolution.Revealed = true
	}
	return revealed
}

func ApplyReactions(
	battle *state.Battle,
	resolution *state.DamageResolutionState,
	reactions []state.DamageReaction,
) error {
	for _, reaction := range reactions {
		switch reaction.Type {
		case state.DamageReactionPreventAccumulated:
			proposal := accumulatedByID(resolution, reaction.ProposalID)
			if proposal == nil {
				return fmt.Errorf("accumulated damage proposal %q was not found", reaction.ProposalID)
			}
			if reaction.Amount < 1 {
				return errors.New("accumulated damage prevention must be positive")
			}
			proposal.ReactionPrevention += reaction.Amount
		case state.DamageReactionPreventSource:
			proposal := sourceByID(resolution, reaction.ProposalID)
			if proposal == nil {
				return fmt.Errorf("damage source proposal %q was not found", reaction.ProposalID)
			}
			if reaction.Amount < 1 {
				return errors.New("source damage prevention must be positive")
			}
			proposal.ReactionPrevention += reaction.Amount
		case state.DamageReactionModifySource:
			proposal := sourceByID(resolution, reaction.ProposalID)
			if proposal == nil {
				return fmt.Errorf("damage source proposal %q was not found", reaction.ProposalID)
			}
			if reaction.Amount == 0 {
				return errors.New("source damage modification must be non-zero")
			}
			proposal.ReactionModification += reaction.Amount
		case state.DamageReactionReplaceCard:
			if err := replaceCardProposal(battle, resolution, reaction); err != nil {
				return err
			}
		default:
			return fmt.Errorf("unknown damage reaction type %q", reaction.Type)
		}
	}
	Recalculate(resolution)
	return nil
}

func Commit(battle *state.Battle, resolution *state.DamageResolutionState) (CommitResult, error) {
	if battle == nil || resolution == nil {
		return CommitResult{}, errors.New("battle and damage resolution are required")
	}
	working := battle.Clone()
	result := CommitResult{
		Sources: append([]state.DamageSourceProposal(nil), resolution.SourceProposals...),
		Totals:  append([]state.AccumulatedDamageProposal(nil), resolution.AccumulatedProposals...),
	}

	accepted := acceptedCardProposals(resolution.CardProposals)
	if err := validateCardProposals(working, accepted); err != nil {
		return CommitResult{}, err
	}
	for _, proposal := range accepted {
		actor := working.Actors[proposal.TargetActorID]
		if !removeOneFromZone(&actor.Cards, proposal.OriginalZone, proposal.CardID) {
			return CommitResult{}, fmt.Errorf(
				"damage card %q is missing from actor %q zone %q",
				proposal.CardID,
				proposal.TargetActorID,
				proposal.OriginalZone,
			)
		}
		actor.Cards.Removed = append(actor.Cards.Removed, proposal.CardID)
		if actor.CurrentHealth() == 0 {
			actor.DefeatState = state.ActorPendingDefeat
		}
		working.Actors[proposal.TargetActorID] = actor
		result.Cards = append(result.Cards, CardRemovalResult{Proposal: proposal})
	}
	working.PendingOperations = append(
		working.PendingOperations,
		cloneFinalizedOperations(resolution.PendingOperations)...,
	)
	*battle = working
	return result, nil
}

func BuildProposalBatch(resolution *state.DamageResolutionState) state.ProposalBatch {
	batch := state.ProposalBatch{
		ID:           resolution.ID + "-proposal-batch",
		ResolutionID: resolution.ReactionResolutionID,
		Revealed:     true,
	}
	for _, total := range resolution.AccumulatedProposals {
		amount := total.FinalAmount
		batch.Proposals = append(batch.Proposals, state.Proposal{
			ID: total.ID,
			Source: state.SourceReference{
				Type: "accumulated_damage",
				ID:   total.ID,
			},
			Target: state.TargetReference{
				Type:    "actor",
				ID:      total.TargetActorID,
				ActorID: total.TargetActorID,
			},
			Operation: state.ProposalOperation("accumulated_damage"),
			Data:      state.ProposalData{Amount: &state.AmountData{Value: amount}},
		})
	}
	for _, source := range resolution.SourceProposals {
		amount := source.FinalAmount
		batch.Proposals = append(batch.Proposals, state.Proposal{
			ID: source.ID,
			Source: state.SourceReference{
				Type:         source.SourceContentType,
				ID:           source.SourceContentID,
				ActorID:      source.SourceActorID,
				DefinitionID: source.OriginatingOperation.Operation.ID,
			},
			Target: state.TargetReference{
				Type:    "actor",
				ID:      source.TargetActorID,
				ActorID: source.TargetActorID,
			},
			Operation: state.ProposalOperation("damage_source"),
			Data:      state.ProposalData{Amount: &state.AmountData{Value: amount}},
		})
	}
	for _, card := range resolution.CardProposals {
		if !card.Accepted {
			continue
		}
		batch.Proposals = append(batch.Proposals, state.Proposal{
			ID: card.ID,
			Source: state.SourceReference{
				Type: "damage_card_selection",
				ID:   accumulatedProposalID(card.TargetActorID),
			},
			Target: state.TargetReference{
				Type:    "actor",
				ID:      card.TargetActorID,
				ActorID: card.TargetActorID,
			},
			Operation: state.ProposalOperation("damage_card_removal"),
			Data: state.ProposalData{Selection: &state.SelectionData{
				OptionIDs: []string{card.CardID},
			}},
		})
	}
	for _, pending := range resolution.PendingOperations {
		batch.Proposals = append(batch.Proposals, state.Proposal{
			ID: pending.ID,
			Source: state.SourceReference{
				Type:    pending.ContentType,
				ID:      pending.ContentID,
				ActorID: pending.SourceActorID,
			},
			Target: state.TargetReference{
				Type: "pending_operation",
				ID:   pending.Operation.ID,
			},
			Operation: state.ProposalOperation("finalized_operation"),
			Data: state.ProposalData{Selection: &state.SelectionData{
				OptionIDs: append([]string(nil), pending.SelectedTargets...),
			}},
		})
	}
	return batch
}

func applyDefensiveModifiers(resolution *state.DamageResolutionState) {
	for i := range resolution.SourceProposals {
		resolution.SourceProposals[i].DefensivePrevention = 0
	}
	for _, modifier := range resolution.ModifierProposals {
		remaining := modifier.Amount
		for i := range resolution.SourceProposals {
			source := &resolution.SourceProposals[i]
			if remaining == 0 || !targetsSource(modifier.TargetProposalIDs, *source) {
				continue
			}
			prevented := min(remaining, source.BaseAmount-source.DefensivePrevention)
			source.DefensivePrevention += prevented
			remaining -= prevented
		}
	}
}

func targetsSource(targetIDs []string, source state.DamageSourceProposal) bool {
	for _, targetID := range targetIDs {
		if targetID == source.ID || targetID == source.SourcePlanningProposalID {
			return true
		}
	}
	return false
}

func appendPlanningProposals(groups ...[]state.PlanningProposal) []state.PlanningProposal {
	var result []state.PlanningProposal
	for _, group := range groups {
		result = append(result, group...)
	}
	return result
}

type cardCandidate struct {
	CardID string
	Zone   operation.CardZone
}

func selectAdditionalCards(
	zones state.CardZones,
	existing []state.ProposedCardRemoval,
	total state.AccumulatedDamageProposal,
	sourceActors []string,
	count int,
	random RandomSource,
) ([]state.ProposedCardRemoval, error) {
	excluded := acceptedCardCounts(existing, total.TargetActorID)
	primary := availableCandidates(zones, []operation.CardZone{operation.ZoneDeck, operation.ZoneDiscard}, excluded)
	selected, err := selectCandidates(primary, min(count, len(primary)), random)
	if err != nil {
		return nil, err
	}
	if len(selected) < count {
		hand := availableCandidates(zones, []operation.CardZone{operation.ZoneHand}, excluded)
		more, err := selectCandidates(hand, min(count-len(selected), len(hand)), random)
		if err != nil {
			return nil, err
		}
		selected = append(selected, more...)
	}

	nextSequence := nextCardSequence(existing, total.TargetActorID)
	proposals := make([]state.ProposedCardRemoval, len(selected))
	for i, candidate := range selected {
		sequence := nextSequence + i
		proposals[i] = state.ProposedCardRemoval{
			ID:                fmt.Sprintf("%s-card-%d", total.ID, sequence),
			TargetActorID:     total.TargetActorID,
			CardID:            candidate.CardID,
			OriginalZone:      candidate.Zone,
			DamageProposalIDs: append([]string(nil), total.SourceProposalIDs...),
			SourceActorIDs:    append([]string(nil), sourceActors...),
			Sequence:          sequence,
			Accepted:          true,
		}
	}
	return proposals, nil
}

func sourceActorsForTotal(
	sources []state.DamageSourceProposal,
	total state.AccumulatedDamageProposal,
) []string {
	seen := make(map[string]bool)
	var actorIDs []string
	for _, source := range sources {
		if source.TargetActorID != total.TargetActorID || seen[source.SourceActorID] {
			continue
		}
		seen[source.SourceActorID] = true
		actorIDs = append(actorIDs, source.SourceActorID)
	}
	sort.Strings(actorIDs)
	return actorIDs
}

func availableCandidates(
	zones state.CardZones,
	order []operation.CardZone,
	excluded map[operation.CardZone]map[string]int,
) []cardCandidate {
	var result []cardCandidate
	for _, zone := range order {
		var cards []string
		switch zone {
		case operation.ZoneDeck:
			cards = zones.Deck
		case operation.ZoneDiscard:
			cards = zones.Discard
		case operation.ZoneHand:
			cards = zones.Hand
		}
		skipped := make(map[string]int)
		for _, cardID := range cards {
			if skipped[cardID] < excluded[zone][cardID] {
				skipped[cardID]++
				continue
			}
			result = append(result, cardCandidate{CardID: cardID, Zone: zone})
		}
	}
	return result
}

func selectCandidates(
	candidates []cardCandidate,
	count int,
	random RandomSource,
) ([]cardCandidate, error) {
	pool := append([]cardCandidate(nil), candidates...)
	selected := make([]cardCandidate, 0, count)
	for len(selected) < count && len(pool) > 0 {
		index, err := random.Intn(len(pool))
		if err != nil {
			return nil, err
		}
		if index < 0 || index >= len(pool) {
			return nil, fmt.Errorf("damage random source returned index %d outside [0,%d)", index, len(pool))
		}
		selected = append(selected, pool[index])
		pool[index] = pool[len(pool)-1]
		pool = pool[:len(pool)-1]
	}
	return selected, nil
}

func replaceCardProposal(
	battle *state.Battle,
	resolution *state.DamageResolutionState,
	reaction state.DamageReaction,
) error {
	index := -1
	for i := range resolution.CardProposals {
		if resolution.CardProposals[i].ID == reaction.ProposalID &&
			resolution.CardProposals[i].Accepted {
			index = i
			break
		}
	}
	if index < 0 {
		return fmt.Errorf("damage card proposal %q was not found", reaction.ProposalID)
	}
	if reaction.ReplacementCardID == "" {
		return errors.New("replacement card id is required")
	}
	current := resolution.CardProposals[index]
	actor := battle.Actors[current.TargetActorID]
	zone, ok := findAvailableCardZone(actor.Cards, resolution.CardProposals, current.TargetActorID, reaction.ReplacementCardID)
	if !ok {
		return fmt.Errorf("replacement card %q is not available", reaction.ReplacementCardID)
	}
	resolution.CardProposals[index].Accepted = false
	resolution.CardProposals[index].Released = true
	sequence := nextCardSequence(resolution.CardProposals, current.TargetActorID)
	resolution.CardProposals = append(resolution.CardProposals, state.ProposedCardRemoval{
		ID:                       fmt.Sprintf("%s-card-%d", accumulatedProposalID(current.TargetActorID), sequence),
		TargetActorID:            current.TargetActorID,
		CardID:                   reaction.ReplacementCardID,
		OriginalZone:             zone,
		DamageProposalIDs:        append([]string(nil), current.DamageProposalIDs...),
		SourceActorIDs:           append([]string(nil), current.SourceActorIDs...),
		Sequence:                 sequence,
		Accepted:                 true,
		ReplacementForProposalID: current.ID,
	})
	return nil
}

func findAvailableCardZone(
	zones state.CardZones,
	existing []state.ProposedCardRemoval,
	actorID string,
	cardID string,
) (operation.CardZone, bool) {
	excluded := acceptedCardCounts(existing, actorID)
	for _, zone := range []operation.CardZone{operation.ZoneDeck, operation.ZoneDiscard, operation.ZoneHand} {
		available := 0
		var cards []string
		switch zone {
		case operation.ZoneDeck:
			cards = zones.Deck
		case operation.ZoneDiscard:
			cards = zones.Discard
		case operation.ZoneHand:
			cards = zones.Hand
		}
		for _, candidate := range cards {
			if candidate == cardID {
				available++
			}
		}
		if available > excluded[zone][cardID] {
			return zone, true
		}
	}
	return "", false
}

func validateCardProposals(battle state.Battle, proposals []state.ProposedCardRemoval) error {
	required := make(map[string]map[operation.CardZone]map[string]int)
	proposalIDs := make(map[string]bool, len(proposals))
	for _, proposal := range proposals {
		if proposal.ID == "" {
			return errors.New("damage card proposal id is required")
		}
		if proposalIDs[proposal.ID] {
			return fmt.Errorf("duplicate damage card proposal id %q", proposal.ID)
		}
		proposalIDs[proposal.ID] = true
		if _, ok := battle.Actors[proposal.TargetActorID]; !ok {
			return fmt.Errorf("damage card target actor %q is not in battle", proposal.TargetActorID)
		}
		if required[proposal.TargetActorID] == nil {
			required[proposal.TargetActorID] = make(map[operation.CardZone]map[string]int)
		}
		if required[proposal.TargetActorID][proposal.OriginalZone] == nil {
			required[proposal.TargetActorID][proposal.OriginalZone] = make(map[string]int)
		}
		required[proposal.TargetActorID][proposal.OriginalZone][proposal.CardID]++
	}
	for actorID, zones := range required {
		actor := battle.Actors[actorID]
		for zone, cards := range zones {
			for cardID, count := range cards {
				if cardCountInZone(actor.Cards, zone, cardID) < count {
					return fmt.Errorf(
						"damage card %q no longer exists %d time(s) in actor %q zone %q",
						cardID,
						count,
						actorID,
						zone,
					)
				}
			}
		}
	}
	return nil
}

func removeOneFromZone(zones *state.CardZones, zone operation.CardZone, cardID string) bool {
	var cards *[]string
	switch zone {
	case operation.ZoneDeck:
		cards = &zones.Deck
	case operation.ZoneDiscard:
		cards = &zones.Discard
	case operation.ZoneHand:
		cards = &zones.Hand
	default:
		return false
	}
	for i, candidate := range *cards {
		if candidate == cardID {
			*cards = append((*cards)[:i], (*cards)[i+1:]...)
			return true
		}
	}
	return false
}

func cardCountInZone(zones state.CardZones, zone operation.CardZone, cardID string) int {
	var cards []string
	switch zone {
	case operation.ZoneDeck:
		cards = zones.Deck
	case operation.ZoneDiscard:
		cards = zones.Discard
	case operation.ZoneHand:
		cards = zones.Hand
	}
	count := 0
	for _, candidate := range cards {
		if candidate == cardID {
			count++
		}
	}
	return count
}

func acceptedCardCounts(
	proposals []state.ProposedCardRemoval,
	actorID string,
) map[operation.CardZone]map[string]int {
	counts := map[operation.CardZone]map[string]int{
		operation.ZoneDeck:    {},
		operation.ZoneDiscard: {},
		operation.ZoneHand:    {},
	}
	for _, proposal := range proposals {
		if proposal.TargetActorID == actorID && proposal.Accepted {
			counts[proposal.OriginalZone][proposal.CardID]++
		}
	}
	return counts
}

func acceptedCardProposals(values []state.ProposedCardRemoval) []state.ProposedCardRemoval {
	var accepted []state.ProposedCardRemoval
	for _, proposal := range values {
		if proposal.Accepted {
			accepted = append(accepted, proposal)
		}
	}
	sort.SliceStable(accepted, func(i, j int) bool {
		if accepted[i].TargetActorID == accepted[j].TargetActorID {
			return accepted[i].Sequence < accepted[j].Sequence
		}
		return accepted[i].TargetActorID < accepted[j].TargetActorID
	})
	return accepted
}

func activeCardProposalIndices(values []state.ProposedCardRemoval, actorID string) []int {
	var indices []int
	for i, proposal := range values {
		if proposal.TargetActorID == actorID && proposal.Accepted {
			indices = append(indices, i)
		}
	}
	sort.SliceStable(indices, func(i, j int) bool {
		return values[indices[i]].Sequence < values[indices[j]].Sequence
	})
	return indices
}

func nextCardSequence(values []state.ProposedCardRemoval, actorID string) int {
	next := 1
	for _, proposal := range values {
		if proposal.TargetActorID == actorID && proposal.Sequence >= next {
			next = proposal.Sequence + 1
		}
	}
	return next
}

func accumulatedByID(
	resolution *state.DamageResolutionState,
	proposalID string,
) *state.AccumulatedDamageProposal {
	for i := range resolution.AccumulatedProposals {
		if resolution.AccumulatedProposals[i].ID == proposalID {
			return &resolution.AccumulatedProposals[i]
		}
	}
	return nil
}

func sourceByID(
	resolution *state.DamageResolutionState,
	proposalID string,
) *state.DamageSourceProposal {
	for i := range resolution.SourceProposals {
		if resolution.SourceProposals[i].ID == proposalID {
			return &resolution.SourceProposals[i]
		}
	}
	return nil
}

func accumulatedProposalID(actorID string) string {
	return "damage-total-" + actorID
}

func cloneFinalizedOperation(value state.FinalizedOperationProposal) state.FinalizedOperationProposal {
	value.Operation = operation.ClonePlans([]operation.Plan{value.Operation})[0]
	value.SelectedTargets = append([]string(nil), value.SelectedTargets...)
	return value
}

func cloneFinalizedOperations(values []state.FinalizedOperationProposal) []state.FinalizedOperationProposal {
	cloned := make([]state.FinalizedOperationProposal, len(values))
	for i, value := range values {
		cloned[i] = cloneFinalizedOperation(value)
	}
	return cloned
}

func clampNonNegative(value int) int {
	if value < 0 {
		return 0
	}
	return value
}

func min(left, right int) int {
	if left < right {
		return left
	}
	return right
}
