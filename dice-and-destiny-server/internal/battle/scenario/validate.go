package scenario

import (
	"errors"
	"fmt"

	"diceanddestiny/server/internal/battle/operation"
	"diceanddestiny/server/internal/battle/participant"
	"diceanddestiny/server/internal/battle/repository"
	"diceanddestiny/server/internal/battle/segment"
	"diceanddestiny/server/internal/battle/state"
)

func ValidateSpec(spec Spec) error {
	if spec.SchemaVersion != SchemaVersion {
		return fmt.Errorf("unsupported scenario schema_version %d", spec.SchemaVersion)
	}
	if spec.BattleID != "" {
		if err := repository.ValidateBattleID(spec.BattleID); err != nil {
			return fmt.Errorf("battle_id: %w", err)
		}
	}
	if spec.Metadata.ID != "" {
		if err := ValidateScenarioID(spec.Metadata.ID); err != nil {
			return fmt.Errorf("metadata.id: %w", err)
		}
	}
	if spec.Entry.Round < 1 {
		return errors.New("entry.round must be at least 1")
	}
	if !segment.IsValid(spec.Entry.Segment) {
		return fmt.Errorf("entry.segment %q is invalid", spec.Entry.Segment)
	}
	if spec.Player.Controller != state.ControllerHuman {
		return errors.New("player.controller must be human")
	}
	if spec.Player.Source != participant.SourceRunPlayer &&
		spec.Player.Source != participant.SourceCharacterDefinition {
		return errors.New("player.source must be run_player or character_definition")
	}
	if len(spec.Enemies) == 0 {
		return errors.New("at least one enemy is required")
	}
	seen := make(map[string]struct{}, 1+len(spec.Enemies))
	for index, value := range append([]ParticipantSpec{spec.Player}, spec.Enemies...) {
		path := "player"
		if index > 0 {
			path = fmt.Sprintf("enemies[%d]", index-1)
		}
		if value.InstanceID == "" {
			return fmt.Errorf("%s.instance_id is required", path)
		}
		if value.DefinitionID == "" {
			return fmt.Errorf("%s.definition_id is required", path)
		}
		if _, ok := seen[value.InstanceID]; ok {
			return fmt.Errorf("duplicate participant instance_id %q", value.InstanceID)
		}
		seen[value.InstanceID] = struct{}{}
		if index > 0 && (value.Controller != state.ControllerAI ||
			value.Source != participant.SourceEnemyDefinition) {
			return fmt.Errorf("%s must use AI enemy_definition", path)
		}
	}
	for actorID := range spec.Actors {
		if _, ok := seen[actorID]; !ok {
			return fmt.Errorf("actors.%s does not reference a participant", actorID)
		}
	}
	switch spec.Random.Mode {
	case "", state.RandomModeNormal, state.RandomModeReproducible:
	default:
		return fmt.Errorf("random.mode %q is invalid", spec.Random.Mode)
	}
	if len(spec.SetupScript) > 32 {
		return errors.New("setup_script exceeds 32 steps")
	}
	for i, step := range spec.SetupScript {
		if _, ok := seen[step.ActorID]; !ok {
			return fmt.Errorf("setup_script[%d].actor_id %q is not a participant", i, step.ActorID)
		}
		if step.Type == "" {
			return fmt.Errorf("setup_script[%d].type is required", i)
		}
		if len(step.Payload) == 0 {
			return fmt.Errorf("setup_script[%d].payload is required", i)
		}
	}
	return nil
}

func ValidateBattle(battle state.Battle, spec Spec) error {
	if battle.Status != state.BattleActive {
		return errors.New("scenario battle must be active")
	}
	if battle.ActiveResolutionID != "" || len(battle.Flow.PendingInput) != 0 {
		return errors.New("segment-entry scenario cannot contain active resolution or pending input")
	}
	if battle.Flow.Entered || battle.Flow.Segment != spec.Entry.Segment ||
		battle.Flow.Round != spec.Entry.Round {
		return errors.New("segment-entry flow must be new and unentered")
	}
	for actorID, actor := range battle.Actors {
		if err := validateActor(actorID, actor, battle.Content, battle.DiceDefinitions); err != nil {
			return err
		}
	}
	return validatePrerequisites(battle)
}

func validateActor(
	actorID string,
	actor state.ActorState,
	content state.ContentCatalog,
	diceDefinitions map[string]state.DiceDefinition,
) error {
	counts := make(map[string]int)
	for _, entry := range actor.Decklist {
		if entry.CardID == "" || entry.Count <= 0 {
			return fmt.Errorf("actors.%s.decklist contains an invalid entry", actorID)
		}
		if _, ok := content.Cards[entry.CardID]; !ok {
			return fmt.Errorf("actors.%s.decklist references unknown card %q", actorID, entry.CardID)
		}
		counts[entry.CardID] += entry.Count
	}
	zoneCounts := make(map[string]int)
	for _, zone := range [][]string{
		actor.Cards.Deck,
		actor.Cards.Hand,
		actor.Cards.Discard,
		actor.Cards.Removed,
	} {
		for _, cardID := range zone {
			if _, ok := content.Cards[cardID]; !ok {
				return fmt.Errorf("actors.%s.card_zones references unknown card %q", actorID, cardID)
			}
			zoneCounts[cardID]++
		}
	}
	for cardID, count := range counts {
		if zoneCounts[cardID] != count {
			return fmt.Errorf(
				"actors.%s.card_zones has %d copies of %q, want %d",
				actorID,
				zoneCounts[cardID],
				cardID,
				count,
			)
		}
		delete(zoneCounts, cardID)
	}
	if len(zoneCounts) != 0 {
		return fmt.Errorf("actors.%s.card_zones contains cards outside the decklist", actorID)
	}
	deckSize := 0
	for _, entry := range actor.Decklist {
		deckSize += entry.Count
	}
	if actor.Health.MaxHealth != deckSize {
		return fmt.Errorf("actors.%s.health.max_health must match decklist size", actorID)
	}
	energy := actor.Resources.EnergyPoints
	if energy < 0 || energy > actor.Resources.MaxEnergyPoints {
		return fmt.Errorf("actors.%s.energy is outside 0..max", actorID)
	}
	statusIDs := make(map[string]struct{})
	for _, status := range actor.Statuses {
		if status.InstanceID == "" {
			return fmt.Errorf("actors.%s.statuses instance_id is required", actorID)
		}
		if _, ok := statusIDs[status.InstanceID]; ok {
			return fmt.Errorf("actors.%s.statuses duplicate instance_id %q", actorID, status.InstanceID)
		}
		statusIDs[status.InstanceID] = struct{}{}
		definition, ok := content.Statuses[status.DefinitionID]
		if !ok {
			return fmt.Errorf("actors.%s.statuses references unknown status %q", actorID, status.DefinitionID)
		}
		if status.Stacks < 1 || status.Stacks > definition.StackLimit {
			return fmt.Errorf("actors.%s.statuses %q stacks are outside 1..%d", actorID, status.DefinitionID, definition.StackLimit)
		}
	}
	tokenIDs := make(map[string]struct{})
	for _, token := range actor.Tokens {
		if token.ID == "" {
			return fmt.Errorf("actors.%s.tokens id is required", actorID)
		}
		if _, ok := tokenIDs[token.ID]; ok {
			return fmt.Errorf("actors.%s.tokens duplicate id %q", actorID, token.ID)
		}
		tokenIDs[token.ID] = struct{}{}
	}
	for _, abilityID := range actor.AbilityIDs {
		if _, ok := content.Abilities[abilityID]; !ok {
			return fmt.Errorf("actors.%s references unknown ability %q", actorID, abilityID)
		}
	}
	for _, entry := range actor.DiceLoadout {
		if entry.Count <= 0 {
			return fmt.Errorf("actors.%s dice count must be positive", actorID)
		}
		if _, ok := diceDefinitions[entry.DiceID]; !ok {
			return fmt.Errorf("actors.%s references unknown dice %q", actorID, entry.DiceID)
		}
	}
	switch actor.DefeatState {
	case state.ActorNotDefeated:
		if actor.CurrentHealth() == 0 {
			return fmt.Errorf("actors.%s has zero health but is not defeated", actorID)
		}
	case state.ActorPendingDefeat, state.ActorDefeated:
		if actor.CurrentHealth() != 0 {
			return fmt.Errorf("actors.%s is defeated with remaining health", actorID)
		}
	default:
		return fmt.Errorf("actors.%s has invalid defeat_state %q", actorID, actor.DefeatState)
	}
	return nil
}

func validatePrerequisites(battle state.Battle) error {
	if battle.Segment.Current == segment.Defensive {
		hasDefensible := false
		for _, proposal := range battle.OffensiveProposals {
			hasDefensible = hasDefensible || proposal.Defensible
		}
		if !hasDefensible {
			return errors.New("defensive entry requires a finalized defensible offensive proposal")
		}
	}
	if battle.Segment.Current == segment.DamageResolution && len(battle.OffensiveProposals) == 0 {
		return errors.New("damage_resolution entry requires finalized offensive proposals")
	}
	seen := make(map[string]struct{})
	for _, proposal := range append(
		append([]state.PlanningProposal(nil), battle.OffensiveProposals...),
		battle.DefensiveProposals...,
	) {
		if proposal.ID == "" {
			return errors.New("prerequisite proposal id is required")
		}
		if _, ok := seen[proposal.ID]; ok {
			return fmt.Errorf("duplicate prerequisite proposal id %q", proposal.ID)
		}
		seen[proposal.ID] = struct{}{}
		if _, ok := battle.Actors[proposal.ActorID]; !ok {
			return fmt.Errorf("proposal %q references unknown actor %q", proposal.ID, proposal.ActorID)
		}
		if proposal.Segment != segment.Offensive && proposal.Segment != segment.Defensive {
			return fmt.Errorf("proposal %q has invalid segment %q", proposal.ID, proposal.Segment)
		}
		if !proposal.Commitment.LockedIn {
			return fmt.Errorf("proposal %q is not finalized", proposal.ID)
		}
		for _, finalized := range proposal.Operations {
			if finalized.SourceActorID != proposal.ActorID {
				return fmt.Errorf("proposal %q operation source actor does not match", proposal.ID)
			}
			for _, targetID := range finalized.SelectedTargets {
				if _, ok := battle.Actors[targetID]; !ok {
					return fmt.Errorf("proposal %q references unknown target %q", proposal.ID, targetID)
				}
			}
			if finalized.Operation.Type == operation.TypeNoop {
				continue
			}
			switch finalized.ContentType {
			case "ability":
				if _, ok := battle.Content.Abilities[finalized.ContentID]; !ok {
					return fmt.Errorf("proposal %q references unknown ability %q", proposal.ID, finalized.ContentID)
				}
			case "card":
				if _, ok := battle.Content.Cards[finalized.ContentID]; !ok {
					return fmt.Errorf("proposal %q references unknown card %q", proposal.ID, finalized.ContentID)
				}
			}
		}
	}
	return nil
}
