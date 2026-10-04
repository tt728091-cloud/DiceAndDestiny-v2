package engine

import (
	"encoding/json"
	"fmt"
	"reflect"

	"diceanddestiny/server/internal/battle/command"
	"diceanddestiny/server/internal/battle/damage"
	"diceanddestiny/server/internal/battle/operation"
	"diceanddestiny/server/internal/battle/state"
	"diceanddestiny/server/internal/content"
)

// Choices contain identities, never client-provided effect amounts. Regenerate
// them on play so stale targets, forged prices and out-of-window plays fail.
type generalCardChoice struct {
	Kind       string `json:"kind"`
	Actor      string `json:"actor,omitempty"`
	Die        int    `json:"die"`
	Copy       int    `json:"copy"`
	Face       int    `json:"face,omitempty"`
	Indices    []int  `json:"indices,omitempty"`
	Card       string `json:"card,omitempty"`
	Definition string `json:"definition,omitempty"`
	Status     string `json:"status,omitempty"`
	Source     string `json:"source,omitempty"`
	Removal    string `json:"removal,omitempty"`
	Boost      bool   `json:"boost,omitempty"`
	Zone       string `json:"zone,omitempty"`
}

func (c generalCardChoice) key() string { raw, _ := json.Marshal(c); return string(raw) }
func (c generalCardChoice) targets() []string {
	if c.Source != "" {
		return []string{c.Source}
	}
	return []string{c.Actor}
}
func generalOperation(def content.BattleCardDefinition) (content.BattleOperation, bool) {
	if def.Targeting.Selector != "general_choice" || len(def.Operations) != 1 || def.Operations[0].Type != "general_card" {
		return content.BattleOperation{}, false
	}
	return def.Operations[0], true
}
func generalCardChoices(b *state.Battle, lib content.BattleLibrary, actor string, def content.BattleCardDefinition) []generalCardChoice {
	op, ok := generalOperation(def)
	if !ok || b.Settled == nil || b.Settled.Window == nil || b.Actors[actor].Resources.EnergyPoints < def.Cost.Energy {
		return nil
	}
	purpose := "reaction"
	if b.Settled.Stage == stageOffensivePlan {
		purpose = "planning"
	}
	if !cardPlayableDuring(def, b, purpose, actor) {
		return nil
	}
	var choices []generalCardChoice
	rt := b.Settled.Actors[actor]
	base := generalCardChoice{Kind: op.Modification, Actor: actor}
	switch op.Modification {
	case "copy_die", "flip_die":
		if b.Settled.Stage != stageOffensivePlan || rt.RollsUsed == 0 {
			break
		}
		for i, d := range rt.FinalDice {
			if op.Modification == "flip_die" {
				c := base
				c.Die = i
				c.Face = 7 - d.Face
				if dieFace(lib, d.DieID, c.Face).Number > 0 && c.Face != d.Face {
					choices = append(choices, c)
				}
			} else {
				for j, from := range rt.FinalDice {
					if i == j || d.Face == from.Face || dieFace(lib, d.DieID, from.Face).Number == 0 {
						continue
					}
					c := base
					c.Die = i
					c.Copy = j
					c.Face = from.Face
					choices = append(choices, c)
				}
			}
		}
	case "reroll_enemy_die":
		if b.Settled.Stage != stageOffensiveReact {
			break
		}
		for _, enemy := range otherActorIDs(b, actor) {
			for i, d := range b.Settled.Actors[enemy].FinalDice {
				c := base
				c.Actor = enemy
				c.Die = i
				c.Face = d.Face
				choices = append(choices, c)
			}
		}
	case "reroll_defense_dice":
		s, exists := b.Settled.DefenseSelections[actor]
		if b.Settled.Stage != stageDefenseReact || !exists || s.Finalized || len(s.RolledDice) == 0 {
			break
		}
		for _, indices := range indexSubsets(allDieIndices(len(s.RolledDice)), false) {
			c := base
			c.Source = s.SourceID
			c.Indices = indices
			choices = append(choices, c)
		}
	case "recover_discard":
		if b.Settled.Stage != stageOffensivePlan {
			break
		}
		for _, id := range b.Actors[actor].Cards.Discard {
			definition := rt.CardInstances[id].DefinitionID
			other, recovery := generalOperation(lib.Cards[definition])
			if recovery && other.Modification == "recover_discard" {
				continue
			}
			c := base
			c.Card = id
			c.Definition = definition
			choices = append(choices, c)
		}
	case "boost_prevention", "save_threatened_card":
		if !unifiedDefense(b) && b.Settled.Stage != stageDamageReact {
			break
		}
		for _, source := range reactionDamageSources(b) {
			if source.TargetActorID != actor || settledSourceAmount(source) <= 0 {
				continue
			}
			c := base
			c.Source = source.ID
			if op.Modification == "boost_prevention" {
				choices = append(choices, c)
				if b.Actors[actor].Resources.EnergyPoints >= def.Cost.Energy+op.ExtraEnergy {
					c.Boost = true
					choices = append(choices, c)
				}
			} else if unifiedDefense(b) {
				for _, r := range b.Settled.PendingDamage.Removals {
					if !r.Accepted || r.Released || !r.Revealed || r.TargetActorID != actor || !containsString(r.DamageProposalIDs, source.ID) {
						continue
					}
					c.Removal = r.ID
					c.Card = r.CardID
					c.Definition = r.CardDefinitionID
					c.Zone = string(currentRemovalZone(b, r))
					choices = append(choices, c)
				}
			}
		}
	case "dispel_positive":
		if b.Settled.Stage != stageOffensivePlan && b.Settled.Stage != stageDefenseSelect && b.Settled.Stage != stageDefenseReact {
			break
		}
		for _, enemy := range otherActorIDs(b, actor) {
			for _, s := range b.Actors[enemy].Statuses {
				if lib.Statuses[s.DefinitionID].Polarity != "positive" || lib.Statuses[s.DefinitionID].DispelImmune || s.Stacks <= 0 {
					continue
				}
				c := base
				c.Actor = enemy
				c.Status = s.DefinitionID
				choices = append(choices, c)
			}
		}
	}
	return choices
}
func generalCardActions(b *state.Battle, lib content.BattleLibrary, actor string, pending state.PendingInput) []command.Command {
	var actions []command.Command
	for _, id := range b.Actors[actor].Cards.Hand {
		def := lib.Cards[b.Settled.Actors[actor].CardInstances[id].DefinitionID]
		for _, c := range generalCardChoices(b, lib, actor, def) {
			if b.Settled.Stage == stageOffensivePlan && containsCommand(b.Settled.Window.AllowedCommands, command.TypePlanningCards) {
				actions = append(actions, legalCommand(b.ID, actor, command.TypePlanningCards, command.PlanningCardsPayload{PendingInputID: pending.ID, Checkpoint: planningCheckpoint(pending), CardIDs: []string{id}, TargetIDs: c.targets(), StatusID: c.key()}))
			} else if containsCommand(b.Settled.Window.AllowedCommands, command.TypeCommitInteraction) {
				actions = append(actions, legalCommand(b.ID, actor, command.TypeCommitInteraction, command.CommitInteractionPayload{PendingInputID: pending.ID, Checkpoint: interactionCheckpoint(pending), Commitment: command.InteractionCommitmentData{CardIDs: []string{id}, ProposalIDs: c.targets(), ChoiceID: c.key()}}))
			}
		}
	}
	return actions
}
func (e Engine) playGeneralCard(b *state.Battle, lib content.BattleLibrary, actor, instance string, def content.BattleCardDefinition, targets []string, key string) error {
	if !containsString(b.Actors[actor].Cards.Hand, instance) {
		return fmt.Errorf("card is not in hand")
	}
	var selected *generalCardChoice
	for _, c := range generalCardChoices(b, lib, actor, def) {
		if c.key() == key && reflect.DeepEqual(c.targets(), targets) {
			copy := c
			selected = &copy
			break
		}
	}
	if selected == nil {
		return fmt.Errorf("%s choice is no longer legal", def.Name)
	}
	c := *selected
	op, _ := generalOperation(def)
	extra := 0
	result := effectResult{SavedCardDestination: def.SavedCardDestination}
	switch c.Kind {
	case "copy_die", "flip_die":
		result.DieChanges = []effectDieChange{{ActorID: actor, Index: c.Die, Face: c.Face}}
	case "reroll_enemy_die":
		dice, err := e.rollCombatDice(b, lib, c.Actor, []int{c.Die})
		if err != nil {
			return err
		}
		result.DieChanges = []effectDieChange{{ActorID: c.Actor, Index: c.Die, Face: dice[c.Die].Face}}
	case "reroll_defense_dice":
		s := b.Settled.DefenseSelections[actor]
		for _, i := range c.Indices {
			// Reroll the same physical owned die, including its authored curse marks.
			d, err := e.ownedRoll(b, lib, actor, s.RolledDice[i].Index, "defense_dice", true)
			if err != nil {
				return err
			}
			s.RolledDice[i] = d
			s.RolledFaces[i] = d.Face
		}
		s.RolledFace = s.RolledFaces[0]
		b.Settled.DefenseSelections[actor] = s
	case "recover_discard":
		a := b.Actors[actor]
		moveCard(&a.Cards, c.Card, operation.ZoneDiscard, operation.ZoneHand)
		b.Actors[actor] = a
	case "boost_prevention":
		amount, err := operationAmount(op, 0)
		if err != nil {
			return err
		}
		if c.Boost {
			amount += op.BonusAmount
			extra = op.ExtraEnergy
		}
		result.Preventions = []effectPrevention{{ProposalID: c.Source, Amount: amount}}
	case "save_threatened_card":
		for i := range b.Settled.PendingDamage.Removals {
			r := &b.Settled.PendingDamage.Removals[i]
			if r.ID != c.Removal {
				continue
			}
			if def.SavedCardDestination == content.SavedCardsDiscard {
				damage.DiscardPreventedCard(b, r)
			} else {
				damage.RetainPreventedCard(b, r)
			}
			r.Accepted = false
			r.Released = true
			r.ProtectedFromSource = true
		}
		result.Preventions = []effectPrevention{{ProposalID: c.Source, Amount: 1}}
	case "dispel_positive":
		result.StatusRemovals = []state.SettledStatusRemoval{{ActorID: c.Actor, StatusID: c.Status, Stacks: 1}}
	}
	if err := e.applyEffectMutations(b, lib, instance, result); err != nil {
		return err
	}
	if c.Kind == "reroll_enemy_die" {
		if err := e.revalidateOffensiveSelection(b, lib, c.Actor); err != nil {
			return err
		}
	}
	spendEnergy(b, actor, def.Cost.Energy+extra)
	a := b.Actors[actor]
	moveCard(&a.Cards, instance, operation.ZoneHand, operation.CardZone(def.Play.Destination))
	b.Actors[actor] = a
	if batch := b.Settled.PendingDamage; batch != nil {
		for i := range batch.Removals {
			r := &batch.Removals[i]
			if r.CardID == instance && r.Released {
				damage.RetainPreventedCard(b, r)
			}
		}
	}
	return nil
}
