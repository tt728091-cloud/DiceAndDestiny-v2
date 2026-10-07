package engine

import (
	"errors"
	"fmt"
	"strings"

	"diceanddestiny/server/internal/battle/command"
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
	if !mechanicAvailable(b, actor, def) || b.Actors[actor].Resources.EnergyPoints < def.Cost.Energy {
		return nil
	}
	stage := b.Settled.Stage
	id := content.MechanicKind(def)
	p := stacks(b, enemy, "poison")
	cost := content.MechanicInt(def, "cost_stacks")
	costStatus := content.MechanicString(def, "cost_status")
	costSelf := stacks(b, actor, costStatus) >= cost
	costEnemy := stacks(b, enemy, costStatus) >= cost
	v := venomRuntime(b)
	var choices []venomCardChoice
	add := func(key string, targets ...string) {
		choices = append(choices, venomCardChoice{Key: key, Targets: targets})
	}
	switch id {
	case "pinprick", "twin_puncture":
		if stage == stageOffensivePlan && canReceiveMechanicStatus(b, lib, enemy, content.MechanicString(def, "status_id")) {
			add("apply", enemy)
		}
	case "culture_flask":
		if stage == stageOffensivePlan && stacks(b, actor, content.MechanicString(def, "status_id")) < statusCap(lib, content.MechanicString(def, "status_id")) {
			add("gain", actor)
		}
	case "slow_release", "incubate":
		if stage == stageOffensivePlan && p > 0 && stacks(b, enemy, "incubation") == 0 && costSelf {
			add("incubate", enemy)
		}
	case "distill", "accelerant":
		if stage == stageOffensivePlan && p >= content.MechanicInt(def, "convert_stacks") && costSelf && stacks(b, enemy, "volatile_poison") < statusCap(lib, "volatile_poison") && (id != "accelerant" || v.Provoked[enemy] < 2) {
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
			for _, row := range toxinChoices(b, enemy, content.MechanicInt(def, "checks")) {
				add(strings.Join(row, ","), enemy)
			}
		}
	case "extract":
		if stage == stageOffensivePlan && costEnemy && stacks(b, actor, content.MechanicString(def, "status_id")) < statusCap(lib, content.MechanicString(def, "status_id")) {
			add("extract", enemy)
		}
	case "repurpose":
		if stage == stageOffensivePlan && costSelf {
			add("draw", actor)
		}
	case "venom_reserve":
		if stage == stageOffensivePlan && costSelf && (def.Mechanic != nil || !v.Used["reserve:"+actor]) {
			add("energy", actor)
		}
	case "shock_dose":
		if stage == stageOffensivePlan && costEnemy {
			add("spend", enemy)
		}
	case "venom_lens":
		if stage == stageOffensivePlan && (def.Mechanic != nil && mechanicStacks(b, actor, id) == 0 || def.Mechanic == nil && !v.Used["battle:lens:"+actor]) {
			add("upgrade", actor)
		}
	case "steady_hand":
		if stage == stageOffensivePlan && b.Settled.Actors[actor].RollsUsed > 0 {
			for i, d := range b.Settled.Actors[actor].FinalDice {
				for _, face := range content.MechanicInts(def, "faces") {
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
					for _, delta := range content.MechanicInts(def, "deltas") {
						face := d.Face + delta
						if face >= content.MechanicInt(def, "minimum") && face <= content.MechanicInt(def, "maximum") {
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
		if stage == stageOffensiveReact && (def.Mechanic != nil && mechanicStacks(b, actor, id) == 0 || def.Mechanic == nil && !v.Used["deep:"+actor]) {
			ops, ok := resolvedOffensiveOperations(b, lib, actor)
			if ok {
				damage, poison := 0, 0
				for _, op := range ops {
					if op.Type == "deal_damage" {
						n, _ := operationAmount(op, 0)
						damage += n
					}
					if op.Type == "apply_status" && op.StatusID == content.MechanicString(def, "status_id") {
						poison += op.StackCount
					}
				}
				if damage >= max(content.MechanicInt(def, "minimum_damage"), content.MechanicInt(def, "damage_cost")) && poison > 0 && canReceiveMechanicStatus(b, lib, enemy, content.MechanicString(def, "status_id")) {
					add("deepen", enemy)
				}
			}
		}
	case "terminal_formula":
		if !containsString(b.Settled.Actors[actor].SelectedTargetIDs, enemy) {
			break
		}
		if stage == stageOffensiveReact && b.Settled.Actors[actor].SelectedAbilityID == content.MechanicString(def, "ability_id") && (def.Mechanic != nil && mechanicStacks(b, actor, id) == 0 || def.Mechanic == nil && !v.Used["formula:"+actor]) && (p+stacks(b, enemy, "volatile_poison")) > 0 {
			add("formula", enemy)
		}
	case "bitter_reagent", "measured_dose":
		if stage == stageOffensivePlan && stacks(b, actor, content.MechanicString(def, "status_id")) < statusCap(lib, content.MechanicString(def, "status_id")) {
			add("gain", actor)
		}
	case "coagulate", "emergency_molt", "antivenom_draught", "spined_rebuttal":
		isDamage := stage == stageOngoingDamage || stage == stageDamageReact || (unifiedDefense(b) && stage == stageDefenseSelect)
		if id == "spined_rebuttal" {
			isDamage = stage == stageDefenseReact || (unifiedDefense(b) && stage == stageDefenseSelect)
		}
		if !isDamage || (id == "coagulate" && !costEnemy) {
			break
		}
		for _, source := range reactionDamageSources(b) {
			if source.TargetActorID != actor || (unifiedDefense(b) && settledSourceAmount(source) == 0) {
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
			if id == "antivenom_draught" && costSelf {
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
	if !mechanicPlayable(b, actor, instance, def) {
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
	beforeCards := mechanicLiveCards(b)
	defer recordMechanicRemovals(b, actor, def, beforeCards)
	payMechanic(b, actor, instance, def)
	id := content.MechanicKind(def)
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
	case "distill", "accelerant", "slow_release", "incubate", "repurpose", "venom_reserve":
		spendMechanicStatus(b, actor, content.MechanicString(def, "cost_status"), content.MechanicInt(def, "cost_stacks"))
	}
	switch id {
	case "extract", "coagulate":
		spendMechanicStatus(b, enemy, content.MechanicString(def, "cost_status"), content.MechanicInt(def, "cost_stacks"))
	case "shock_dose":
		spendMechanicStatus(b, enemy, content.MechanicString(def, "cost_status"), content.MechanicInt(def, "cost_stacks"))
	}
	apply := func(target, status string, n int) {
		v.Queue = append(v.Queue, state.VenomWork{Kind: "application", SourceActorID: actor, TargetActorID: target, StatusID: status, Stacks: n, RequirePoison: status == "incubation"})
	}
	switch id {
	case "pinprick":
		apply(enemy, content.MechanicString(def, "status_id"), content.MechanicInt(def, "stacks"))
	case "twin_puncture":
		apply(enemy, content.MechanicString(def, "status_id"), content.MechanicInt(def, "stacks"))
	case "culture_flask", "extract":
		applyStatus(b, lib, actor, content.MechanicString(def, "status_id"), content.MechanicInt(def, "stacks"))
	case "slow_release", "incubate":
		apply(enemy, "incubation", content.MechanicInt(def, "stacks"))
	case "distill", "accelerant":
		v.Queue = append(v.Queue, state.VenomWork{Kind: "conversion", SourceContentID: def.ID, SourceActorID: actor, TargetActorID: enemy, Checks: min(content.MechanicInt(def, "gain_stacks"), 2-v.Provoked[enemy]), Accelerant: id == "accelerant"})
		if id == "accelerant" {
			v.Provoked[enemy] += min(content.MechanicInt(def, "gain_stacks"), 2-v.Provoked[enemy])
		}
	case "agitate":
		// Agitate checks the whole chosen status, independently of Provoke's
		// per-round allowance. Capture now so later changes cannot add rolls.
		count := stacks(b, enemy, key)
		if limit := content.MechanicInt(def, "checks"); limit > 0 {
			count = min(count, limit)
		}
		choices := make([]string, count)
		for i := range choices {
			choices[i] = key
		}
		rolls := captureToxins(b, lib, enemy, choices, count, false)
		v.Queue = append(v.Queue, state.VenomWork{Kind: "provoke", Rolls: rolls})
	case "fever_cycle":
		rolls := captureToxins(b, lib, enemy, strings.Split(key, ","), content.MechanicInt(def, "checks"), true)
		v.Queue = append(v.Queue, state.VenomWork{Kind: "provoke", Rolls: rolls})
	case "repurpose":
		for n := 0; n < content.MechanicInt(def, "amount"); n++ {
			if _, err := e.drawSettledCard(b, actor, "card_draw"); err != nil {
				return err
			}
		}
	case "venom_reserve":
		gainEnergy(b, actor, content.MechanicInt(def, "amount"))
		v.Used["reserve:"+actor] = true
	case "shock_dose":
		v.Queue = append(v.Queue, state.VenomWork{Kind: "damage", SourceActorID: actor, TargetActorID: enemy, SourceContentID: def.ID, Damage: content.MechanicInt(def, "damage")})
	case "venom_lens":
		if def.Mechanic != nil {
			applyMechanicStatus(b, lib, actor, def)
		} else {
			v.Used["battle:lens:"+actor] = true
		}
	case "deep_puncture":
		if def.Mechanic != nil {
			applyMechanicStatus(b, lib, actor, def)
		} else {
			v.Used["deep:"+actor] = true
		}
	case "terminal_formula":
		if def.Mechanic != nil {
			applyMechanicStatus(b, lib, actor, def)
		} else {
			v.Used["formula:"+actor] = true
		}
	case "steady_hand", "forked_tongue":
		target, index, face := parseDieChoice(key)
		return e.applyEffectMutations(b, lib, instance, effectResult{DieChanges: []effectDieChange{{ActorID: target, Index: index, Face: face}}})
	case "bitter_reagent":
		applyStatus(b, lib, actor, content.MechanicString(def, "status_id"), content.MechanicInt(def, "stacks"))
	case "measured_dose":
		applyStatus(b, lib, actor, content.MechanicString(def, "status_id"), content.MechanicInt(def, "stacks"))
	case "coagulate", "emergency_molt", "antivenom_draught", "spined_rebuttal":
		source := effectDamageSourceByID(b, first(targets))
		amount := content.MechanicInt(def, "prevent")
		before := settledSourceAmount(*source)
		source.ReactionPrevention += amount
		setUnifiedSourceAmount(b, source, max(0, before-amount))
		after := settledSourceAmount(*source)
		if id == "emergency_molt" && before > after {
			v.MoltRewards = append(v.MoltRewards, state.VenomMoltReward{CardID: def.ID, ActorID: actor, SourceID: source.ID, PriorPrevention: source.ReactionPrevention - amount})
		}
		if id == "spined_rebuttal" {
			apply(source.SourceActorID, content.MechanicString(def, "status_id"), content.MechanicInt(def, "stacks"))
		}
		if id == "antivenom_draught" && key != "prevent" {
			spendMechanicStatus(b, actor, content.MechanicString(def, "cost_status"), content.MechanicInt(def, "cost_stacks"))
			removeStatus(b, actor, key, content.MechanicInt(def, "cleanse_stacks"))
		}
		if err := e.reconcilePreventionDestination(b, def.SavedCardDestination); err != nil {
			return err
		}
	}
	return nil
}
func settledSourceAmount(s state.SettledDamageSource) int {
	n := max(0, s.BaseAmount-s.Prevention)
	if s.ScaleDenominator > 0 {
		n = n * s.ScaleNumerator / s.ScaleDenominator
	}
	return max(0, max(0, n-s.ReactionPrevention)+s.ResolutionAdjustment)
}

func venomCardActions(b *state.Battle, lib content.BattleLibrary, actor string, pending state.PendingInput) []command.Command {
	var actions []command.Command
	for _, instance := range mechanicCards(b, actor) {
		def := lib.Cards[b.Settled.Actors[actor].CardInstances[instance].DefinitionID]
		if def.Targeting.Selector != "venom_choice" || !mechanicPlayable(b, actor, instance, def) {
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
