package learned

import (
	"encoding/json"
	"fmt"
	"path/filepath"
	"slices"
	"time"

	"diceanddestiny/server/internal/battle/command"
	"diceanddestiny/server/internal/battle/mlsim"
	"diceanddestiny/server/internal/content"
)

// singleAbilityPolicy uses only the acting seat's view and legal commands.
// Configuration lives in content, so additional minions need no card/creature code.
type singleAbilityPolicy struct {
	definition content.CombatantDefinition
}

func loadSingleAbilityPolicy(root, id string) (*singleAbilityPolicy, error) {
	lib, err := content.LoadBattleLibrary(filepath.Join(root, "battle_v1"))
	if err != nil {
		return nil, err
	}
	lib, err = content.LoadBattleExtension(lib, filepath.Join(root, "minions_v1"))
	if err != nil {
		return nil, err
	}
	def, ok := lib.Combatants[id]
	if !ok || def.SingleAbilityPolicy == nil {
		return nil, fmt.Errorf("unknown single-ability opponent %q", id)
	}
	return &singleAbilityPolicy{definition: def}, nil
}

func (p *singleAbilityPolicy) Metadata() PolicyMetadata {
	return PolicyMetadata{ModelID: p.definition.ID, Algorithm: "single_ability", Architecture: "keep-target-face", ObservationSchema: mlsim.ObservationSchemaVersion, EnvironmentSchema: mlsim.EnvironmentSchemaVersion, ActionSchema: mlsim.ActionSchemaVersion}
}

func (p *singleAbilityPolicy) Select(t mlsim.Transition) (int, time.Duration, error) {
	own := t.Result.Snapshot.Actors[t.ActorID]
	actions := t.Result.LegalActions
	config := p.definition.SingleAbilityPolicy
	attack := p.definition.AbilityBoard.Offensive[0]
	defense := p.definition.AbilityBoard.Defensive[0]
	// Identical minion cards need no discard evaluation. Only choose a
	// mandatory hand-limit commitment, never an optional reaction card.
	if t.Result.Snapshot.Stage == "discard_to_hand_limit" {
		for i, action := range actions {
			if action.Type == command.TypeCommitInteraction {
				return i, 0, nil
			}
		}
	}
	// Complete the initial roll and both available rerolls before spending cards.
	// An all-target roll stops early: rerolling a kept die would only hurt it.
	for i, action := range actions {
		if action.Type == command.TypePlanningRoll {
			return i, 0, nil
		}
	}
	if own.Dice != nil && own.Dice.RollsUsed < own.Dice.MaxRolls {
		reroll := []int{}
		for _, die := range own.Dice.Dice {
			if die.Face != config.TargetFace {
				reroll = append(reroll, die.Index)
			}
		}
		keep := []int{}
		for _, die := range own.Dice.Dice {
			if die.Face == config.TargetFace {
				keep = append(keep, die.Index)
			}
		}
		if !slices.Equal(keep, own.Dice.KeptIndices) {
			for i, action := range actions {
				if action.Type != command.TypePlanningKeep {
					continue
				}
				var payload command.PlanningKeepPayload
				if err := json.Unmarshal(action.Payload, &payload); err != nil {
					return 0, 0, err
				}
				if slices.Equal(payload.KeptIndices, keep) {
					return i, 0, nil
				}
			}
		}
		if len(reroll) > 0 {
			for i, action := range actions {
				if action.Type != command.TypePlanningReroll {
					continue
				}
				var payload command.PlanningRerollPayload
				if err := json.Unmarshal(action.Payload, &payload); err != nil {
					return 0, 0, err
				}
				if slices.Equal(payload.RerollIndices, reroll) {
					return i, 0, nil
				}
			}
		}
	}
	// A round modifier records the play even if the card is later removed by
	// damage. Reopened planning cannot double-play or carry the bonus forward.
	played := false
	for _, modifier := range own.AbilityModifiers {
		if modifier.ExpiresAfterRound == t.Result.Snapshot.Round {
			played = true
		}
	}
	if !played && slices.Contains(own.QualifiedAbilities, attack) {
		for i, action := range actions {
			if action.Type != command.TypePlanningCards {
				continue
			}
			var payload command.PlanningCardsPayload
			if err := json.Unmarshal(action.Payload, &payload); err != nil {
				return 0, 0, err
			}
			if payload.AbilityID == attack && len(payload.CardIDs) == 1 && own.CardInstances[payload.CardIDs[0]].DefinitionID == config.CardID {
				return i, 0, nil
			}
		}
	}
	for i, action := range actions {
		if action.Type != command.TypePlanningAbility {
			continue
		}
		var payload command.PlanningAbilityPayload
		if err := json.Unmarshal(action.Payload, &payload); err != nil {
			return 0, 0, err
		}
		if payload.AbilityID == attack || payload.AbilityID == defense {
			return i, 0, nil
		}
	}
	for _, kind := range []command.Type{command.TypeRollDice, command.TypePass, command.TypePlanningPass} {
		for i, action := range actions {
			if action.Type == kind {
				return i, 0, nil
			}
		}
	}
	return 0, 0, fmt.Errorf("single-ability policy has no supported legal action at %s", t.Result.Snapshot.Stage)
}
