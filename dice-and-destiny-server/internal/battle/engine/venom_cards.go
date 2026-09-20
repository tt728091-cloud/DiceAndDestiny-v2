package engine

import (
	"errors"
	"fmt"
	"strings"

	"diceanddestiny/server/internal/battle/command"
	"diceanddestiny/server/internal/battle/operation"
	"diceanddestiny/server/internal/battle/state"
	"diceanddestiny/server/internal/content"
)

type venomCardChoice struct {
	Key     string
	Targets []string
}

func venomCardChoices(b *state.Battle, lib content.BattleLibrary, actor string, def content.BattleCardDefinition) []venomCardChoice {
	var result []venomCardChoice
	seen := map[string]bool{}
	enemies := otherActorIDs(b, actor)
	if len(enemies) == 0 {
		enemies = []string{""}
	}
	for _, enemy := range enemies {
		for _, choice := range venomCardChoicesForEnemy(b, lib, actor, enemy, def) {
			key := choice.Key + "|" + strings.Join(choice.Targets, "|")
			if !seen[key] {
				seen[key] = true
				result = append(result, choice)
			}
		}
	}
	return result
}
func venomCardChoicesForEnemy(b *state.Battle, lib content.BattleLibrary, actor, enemy string, def content.BattleCardDefinition) []venomCardChoice {
	if b.Actors[actor].Resources.EnergyPoints < def.Cost.Energy {
		return nil
	}
	stage := b.Settled.Stage
	id := def.ID
	p := stacks(b, enemy, "poison")
	cat := stacks(b, actor, "catalyst")
	v := venomRuntime(b)
	var choices []venomCardChoice
	add := func(key string, targets ...string) {
		choices = append(choices, venomCardChoice{Key: key, Targets: targets})
	}
	switch id {
	case "pinprick", "twin_puncture":
		if stage == stageOffensivePlan && (p < 3 || stacks(b, enemy, "incubation") == 0) {
			add("apply", enemy)
		}
	case "culture_flask":
		if stage == stageOffensivePlan && cat < 3 {
			add("gain", actor)
		}
	case "slow_release", "incubate":
		if stage == stageOffensivePlan && p > 0 && stacks(b, enemy, "incubation") == 0 && (id != "incubate" || cat > 0) {
			add("incubate", enemy)
		}
	case "distill", "accelerant":
		if stage == stageOffensivePlan && p > 0 && cat > 0 && stacks(b, enemy, "volatile_poison") < 3 && (id != "accelerant" || v.Provoked[enemy] < 2) {
			add("convert", enemy)
		}
	case "agitate":
		if stage == stageOffensivePlan {
			for _, status := range []string{"poison", "volatile_poison"} {
				if stacks(b, enemy, status) > 0 {
					add(status, enemy)
				}
			}
		}
	case "fever_cycle":
		if stage == stageOffensivePlan {
			for _, row := range toxinChoices(b, enemy, 2) {
				add(strings.Join(row, ","), enemy)
			}
		}
	case "extract":
		if stage == stageOffensivePlan && p > 0 && cat < 3 {
			add("extract", enemy)
		}
	case "repurpose":
		if stage == stageOffensivePlan && cat > 0 {
			add("draw", actor)
		}
	case "venom_reserve":
		if stage == stageOffensivePlan && cat > 0 && !v.Used["reserve:"+actor] {
			add("energy", actor)
		}
	case "shock_dose":
		if stage == stageOffensivePlan && stacks(b, enemy, "volatile_poison") > 0 {
			add("spend", enemy)
		}
	case "venom_lens":
		if stage == stageOffensivePlan && !v.Used["battle:lens:"+actor] {
			add("upgrade", actor)
		}
	case "steady_hand":
		if stage == stageOffensivePlan && b.Settled.Actors[actor].RollsUsed > 0 {
			for i, d := range b.Settled.Actors[actor].FinalDice {
				for _, face := range []int{1, 4} {
					if face != d.Face {
						add(fmt.Sprintf("%s:%d:%d", actor, i, face), actor)
					}
				}
			}
		}
	case "forked_tongue":
		if stage == stageOffensiveReact {
			for _, target := range sortedSettledActorIDs(b) {
				for i, d := range b.Settled.Actors[target].FinalDice {
					for _, face := range []int{d.Face - 1, d.Face + 1} {
						if face >= 1 && face <= 6 {
							add(fmt.Sprintf("%s:%d:%d", target, i, face), target)
						}
					}
				}
			}
		}
	case "deep_puncture":
		if !containsString(b.Settled.Actors[actor].SelectedTargetIDs, enemy) {
			break
		}
		if stage == stageOffensiveReact && !v.Used["deep:"+actor] {
			ops, ok := resolvedOffensiveOperations(b, lib, actor)
			if ok {
				damage, poison := 0, 0
				for _, op := range ops {
					if op.Type == "deal_damage" {
						n, _ := operationAmount(op, 0)
						damage += n
					}
					if op.Type == "apply_status" && op.StatusID == "poison" {
						poison += op.StackCount
					}
				}
				if damage >= 2 && poison > 0 && (p < 3 || stacks(b, enemy, "incubation") == 0) {
					add("deepen", enemy)
				}
			}
		}
	case "terminal_formula":
		if !containsString(b.Settled.Actors[actor].SelectedTargetIDs, enemy) {
			break
		}
		if stage == stageOffensiveReact && b.Settled.Actors[actor].SelectedAbilityID == "terminal_bite" && !v.Used["formula:"+actor] && (p+stacks(b, enemy, "volatile_poison")) > 0 {
			add("formula", enemy)
		}
	case "bitter_reagent", "measured_dose":
		if stage == stageOffensivePlan && cat < 3 {
			add("gain", actor)
		}
	case "coagulate", "emergency_molt", "antivenom_draught", "spined_rebuttal":
		isDamage := stage == stageOngoingDamage || stage == stageDamageReact
		if id == "spined_rebuttal" {
			isDamage = stage == stageDefenseReact
		}
		if !isDamage || (id == "coagulate" && p == 0) {
			break
		}
		for _, source := range reactionDamageSources(b) {
			if source.TargetActorID != actor {
				continue
			}
			if id == "spined_rebuttal" && lib.Abilities[source.SourceContentID].Type != "offensive" {
				continue
			}
			if id == "coagulate" && len(otherActorIDs(b, actor)) > 1 {
				add("prevent|"+enemy, source.ID)
			} else {
				add("prevent", source.ID)
			}
			if id == "antivenom_draught" && cat > 0 {
				for _, status := range []string{"poison", "volatile_poison", "incubation"} {
					if stacks(b, actor, status) > 0 {
						add(status, source.ID)
					}
				}
			}
		}
	}
	return choices
}
func (e Engine) playVenomCard(b *state.Battle, lib content.BattleLibrary, actor, instance string, def content.BattleCardDefinition, targets []string, key string) error {
	if !containsString(b.Actors[actor].Cards.Hand, instance) {
		return errors.New("card is not in hand")
	}
	valid := false
	for _, choice := range venomCardChoices(b, lib, actor, def) {
		if choice.Key == key && strings.Join(choice.Targets, "\x00") == strings.Join(targets, "\x00") {
			valid = true
			break
		}
	}
	if !valid {
		return fmt.Errorf("%s choice is no longer legal", def.Name)
	}
	// Acceptance pays every explicit cost before the effect is proposed.
	spendEnergy(b, actor, def.Cost.Energy)
	a := b.Actors[actor]
	moveCard(&a.Cards, instance, operation.ZoneHand, operation.ZoneDiscard)
	b.Actors[actor] = a
	id := def.ID
	enemy := first(targets)
	if source := effectDamageSourceByID(b, enemy); source != nil {
		enemy = source.SourceActorID
	}
	if id == "coagulate" {
		if strings.HasPrefix(key, "prevent|") {
			enemy = strings.TrimPrefix(key, "prevent|")
		} else {
			enemy = enemyOf(b, actor)
		}
	}
	v := venomRuntime(b)
	switch id {
	case "distill", "accelerant", "incubate", "repurpose", "venom_reserve":
		removeStatus(b, actor, "catalyst", 1)
	}
	switch id {
	case "extract", "coagulate":
		removeStatus(b, enemy, "poison", 1)
	case "shock_dose":
		removeStatus(b, enemy, "volatile_poison", 1)
	}
	apply := func(target, status string, n int) {
		v.Queue = append(v.Queue, state.VenomWork{Kind: "application", SourceActorID: actor, TargetActorID: target, StatusID: status, Stacks: n, RequirePoison: status == "incubation"})
	}
	switch id {
	case "pinprick":
		apply(enemy, "poison", 1)
	case "twin_puncture":
		apply(enemy, "poison", 2)
	case "culture_flask", "extract":
		applyStatus(b, lib, actor, "catalyst", 2)
	case "slow_release", "incubate":
		apply(enemy, "incubation", 1)
	case "distill", "accelerant":
		v.Queue = append(v.Queue, state.VenomWork{Kind: "conversion", SourceActorID: actor, TargetActorID: enemy, Accelerant: id == "accelerant"})
		if id == "accelerant" {
			v.Provoked[enemy]++
		}
	case "agitate":
		// Agitate checks the whole chosen status, independently of Provoke's
		// per-round allowance. Capture now so later changes cannot add rolls.
		count := stacks(b, enemy, key)
		choices := make([]string, count)
		for i := range choices {
			choices[i] = key
		}
		rolls := captureToxins(b, lib, enemy, choices, count, false)
		v.Queue = append(v.Queue, state.VenomWork{Kind: "provoke", Rolls: rolls})
	case "fever_cycle":
		rolls := captureToxins(b, lib, enemy, strings.Split(key, ","), 2, true)
		v.Queue = append(v.Queue, state.VenomWork{Kind: "provoke", Rolls: rolls})
	case "repurpose":
		for n := 0; n < 2; n++ {
			if _, err := e.drawSettledCard(b, actor, "card_draw"); err != nil {
				return err
			}
		}
	case "venom_reserve":
		gainEnergy(b, actor, 1)
		v.Used["reserve:"+actor] = true
	case "shock_dose":
		v.Queue = append(v.Queue, state.VenomWork{Kind: "damage", SourceActorID: actor, TargetActorID: enemy, SourceContentID: id, Damage: 3})
	case "venom_lens":
		v.Used["battle:lens:"+actor] = true
	case "deep_puncture":
		v.Used["deep:"+actor] = true
	case "terminal_formula":
		v.Used["formula:"+actor] = true
	case "steady_hand", "forked_tongue":
		target, index, face := parseDieChoice(key)
		return e.applyEffectMutations(b, lib, instance, effectResult{DieChanges: []effectDieChange{{ActorID: target, Index: index, Face: face}}})
	case "bitter_reagent":
		applyStatus(b, lib, actor, "catalyst", 1)
	case "measured_dose":
		applyStatus(b, lib, actor, "catalyst", 2)
	case "coagulate", "emergency_molt", "antivenom_draught", "spined_rebuttal":
		source := effectDamageSourceByID(b, first(targets))
		amount := 2
		if id == "coagulate" {
			amount = 3
		}
		if id == "spined_rebuttal" {
			amount = 1
		}
		before := settledSourceAmount(*source)
		source.ReactionPrevention += amount
		after := settledSourceAmount(*source)
		if id == "emergency_molt" && before > after {
			v.MoltRewards = append(v.MoltRewards, state.VenomMoltReward{ActorID: actor, SourceID: source.ID, PriorPrevention: source.ReactionPrevention - amount})
		}
		if id == "spined_rebuttal" {
			apply(source.SourceActorID, "poison", 1)
		}
		if id == "antivenom_draught" && key != "prevent" {
			removeStatus(b, actor, "catalyst", 1)
			removeStatus(b, actor, key, 1)
		}
		reconcileSettledDamage(b.Settled.PendingDamage, b)
	}
	return nil
}
func settledSourceAmount(s state.SettledDamageSource) int {
	n := max(0, s.BaseAmount-s.Prevention)
	if s.ScaleDenominator > 0 {
		n = n * s.ScaleNumerator / s.ScaleDenominator
	}
	return max(0, n-s.ReactionPrevention)
}

func venomCardActions(b *state.Battle, lib content.BattleLibrary, actor string, pending state.PendingInput) []command.Command {
	var actions []command.Command
	for _, instance := range b.Actors[actor].Cards.Hand {
		def := lib.Cards[b.Settled.Actors[actor].CardInstances[instance].DefinitionID]
		if def.Targeting.Selector != "venom_choice" {
			continue
		}
		for _, choice := range venomCardChoices(b, lib, actor, def) {
			if b.Settled.Stage == stageOffensivePlan {
				actions = append(actions, legalCommand(b.ID, actor, command.TypePlanningCards, command.PlanningCardsPayload{PendingInputID: pending.ID, Checkpoint: planningCheckpoint(pending), CardIDs: []string{instance}, TargetIDs: choice.Targets, StatusID: choice.Key}))
			} else {
				actions = append(actions, legalCommand(b.ID, actor, command.TypeCommitInteraction, command.CommitInteractionPayload{PendingInputID: pending.ID, Checkpoint: interactionCheckpoint(pending), Commitment: command.InteractionCommitmentData{CardIDs: []string{instance}, ProposalIDs: choice.Targets, ChoiceID: choice.Key}}))
			}
		}
	}
	return actions
}
func defenseFaces(s state.SettledDefense) []int {
	if len(s.RolledFaces) > 0 {
		return s.RolledFaces
	}
	return []int{s.RolledFace}
}
