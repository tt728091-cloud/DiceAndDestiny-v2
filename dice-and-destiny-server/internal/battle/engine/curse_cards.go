package engine

import (
	"diceanddestiny/server/internal/battle/command"
	"diceanddestiny/server/internal/battle/state"
	"diceanddestiny/server/internal/content"
	"errors"
	"fmt"
	"strconv"
	"strings"
)

func isCurseDiceCard(id string) bool { return id == "call_the_mark" || id == "no_safe_keep" }
func curseCardChoices(b *state.Battle, lib content.BattleLibrary, actor string, def content.BattleCardDefinition) []venomCardChoice {
	kind := content.MechanicKind(def)
	if !mechanicAvailable(b, actor, def) || b.Settled.Stage == stageCurseChoice || def.Mechanic == nil && b.Settled.Curse != nil && b.Settled.Curse.Used[actor+":"+def.ID] == b.Segment.Round || b.Actors[actor].Resources.EnergyPoints < def.Cost.Energy {
		return nil
	}
	c := curseRuntime(b)
	var out []venomCardChoice
	add := func(key, target string) { out = append(out, venomCardChoice{Key: key, Targets: []string{target}}) }
	for _, enemy := range otherActorIDs(b, actor) {
		duplicate := isCurseMode(kind) && mechanicStacks(b, enemy, "grave_interest") > 0 || (kind == "second_knell" || kind == "maledictions_refusal" || kind == "black_dividend" || kind == "curse_bloom") && mechanicStacks(b, enemy, kind) > 0
		for _, p := range c.Preparations {
			if p.Target == enemy && activePreparation(b, p) && (preparationKind(p) == kind || isCurseMode(preparationKind(p)) && isCurseMode(kind)) {
				duplicate = true
			}
		}
		if duplicate {
			continue
		}
		count := stacks(b, enemy, "curse_count")
		stage := b.Settled.Stage
		plan := stage == stageOffensivePlan && containsCommand(b.Settled.Window.AllowedCommands, command.TypePlanningCards)
		switch kind {
		case "mark_the_number", "black_fingerprint", "maledictions_refusal", "second_knell", "grave_interest", "black_dividend", "curse_bloom", "black_tax":
			if plan {
				add("apply", enemy)
			}
		case "three_knocks":
			if plan && mechanicStacks(b, enemy, "three_knocks") == 0 {
				add("apply", enemy)
			}
		case "stored_calamity":
			if plan && c.LastStored[enemy] != b.Segment.Round {
				add("store", enemy)
			}
		case "shared_misfortune", "rotten_numeral":
			if plan {
				for _, f := range content.MechanicInts(def, "faces") {
					if content.MechanicBool(def, "clean_only") || len(missingCurseFace(b, enemy, f, false)) > 0 {
						add(fmt.Sprint(f), enemy)
					}
				}
			}
		case "widen_the_crack", "unquiet_hands", "chosen_instrument":
			if plan {
				filter := "cursed"
				if kind == "widen_the_crack" {
					filter = "partly"
				}
				for _, i := range curseDice(b, enemy, filter) {
					add(fmt.Sprint(i), enemy)
				}
			}
		case "tombs_choice":
			if plan && len(curseDice(b, enemy, "cursed")) > 0 {
				add("choose", enemy)
			}
		case "curse_eater":
			if plan && count >= content.MechanicInt(def, "cost_count") {
				add("draw", enemy)
				add("energy", enemy)
			}
		case "misfortunes_choice":
			if plan && count >= content.MechanicInt(def, "cost_count") {
				add("choose", enemy)
			}
		case "call_the_mark":
			if stage == stageOffensiveReact {
				for _, target := range []string{actor, enemy} {
					for i, d := range b.Settled.Actors[target].FinalDice {
						for _, f := range c.Dice[target][i].CursedFaces {
							if f != d.Face {
								add(fmt.Sprintf("%s:%d:%d", target, i, f), target)
							}
						}
					}
				}
			}
		case "no_safe_keep":
			if stage == stageOffensiveReact {
				rt := b.Settled.Actors[enemy]
				for _, i := range rt.KeptIndices {
					if i < len(rt.FinalDice) && len(c.Dice[enemy][i].CursedFaces) > 0 && !rt.FinalDice[i].EffectRetried {
						add(fmt.Sprintf("%s:%d:0", enemy, i), enemy)
					}
				}
			}
		case "ruin_made_flesh":
			if stage == stageOffensiveReact && containsString(b.Settled.Actors[actor].SelectedTargetIDs, enemy) && ownQualifiedDamage(b, lib, actor) {
				for cost := content.MechanicInt(def, "cost_count"); cost <= min(count, content.MechanicInt(def, "maximum_cost")); cost += content.MechanicInt(def, "cost_count") {
					add(strconv.Itoa(cost), enemy)
				}
			}
		case "blind_omen":
			if stage == stageOffensiveReact && count >= content.MechanicInt(def, "required_count") && stacks(b, enemy, content.MechanicString(def, "status_id")) == 0 && b.Settled.Actors[enemy].SelectedAbilityID != "" {
				add("blind", enemy)
			}
		case "hexward_retort":
			if stage == stageDefenseReact {
				if d, ok := b.Settled.DefenseSelections[actor]; ok && d.RolledFace > 0 {
					if s := sourceBySettledID(b, d.SourceID); s != nil && s.SourceActorID == enemy && lib.Abilities[s.SourceContentID].Type == "offensive" {
						add("apply", enemy)
					}
				}
			}
		case "spiteful_ward":
			if stage == stageDamageReact || stage == stageOngoingDamage || (unifiedDefense(b) && stage == stageDefenseSelect) {
				for _, s := range reactionDamageSources(b) {
					if s.TargetActorID == actor && s.SourceActorID == enemy && lib.Abilities[s.SourceContentID].Type == "offensive" && settledSourceAmount(s) > 0 {
						add("prevent", s.ID)
					}
				}
			}
		}
	}
	return out
}
func missingCurseFace(b *state.Battle, actor string, face int, cleanOnly bool) []int {
	var out []int
	for i, d := range curseRuntime(b).Dice[actor] {
		if !containsInt(d.CursedFaces, face) && (!cleanOnly || len(d.CursedFaces) == 0) {
			out = append(out, i)
		}
	}
	return out
}
func (e Engine) playCurseCard(b *state.Battle, lib content.BattleLibrary, actor, instance string, def content.BattleCardDefinition, targets []string, key string) error {
	kind := content.MechanicKind(def)
	if !mechanicPlayable(b, actor, instance, def) {
		return errors.New("card is not in hand")
	}
	legal := false
	for _, v := range curseCardChoices(b, lib, actor, def) {
		if v.Key == key && strings.Join(v.Targets, "\x00") == strings.Join(targets, "\x00") {
			legal = true
			break
		}
	}
	if !legal {
		return fmt.Errorf("%s choice is no longer legal", def.Name)
	}
	beforeCards := mechanicLiveCards(b)
	defer recordMechanicRemovals(b, actor, def, beforeCards)
	payMechanic(b, actor, instance, def)
	c := curseRuntime(b)
	c.Used[actor+":"+def.ID] = b.Segment.Round
	enemy := first(targets)
	n, _ := strconv.Atoi(key)
	prep := func() {
		statusID := ""
		if def.Mechanic != nil {
			applyMechanicStatus(b, lib, enemy, def)
			statusID = mechanicStatus(b, enemy, kind)
		}
		c.Preparations = append(c.Preparations, state.CursePreparation{Kind: kind, StatusID: statusID, CardID: def.ID, Source: actor, Target: enemy, Die: n, Round: b.Segment.Round, ExpiresEffects: b.Segment.Round + 1})
	}
	switch kind {
	case "mark_the_number", "hexward_retort":
		return e.applyCurse(b, lib, enemy, def.ID, content.MechanicInt(def, "curses"))
	case "grave_interest":
		if _, ok := lib.Statuses["grave_interest"]; ok {
			applyMechanicStatus(b, lib, enemy, def)
		} else {
			prep()
		}
	case "second_knell":
		if _, ok := lib.Statuses["second_knell"]; ok {
			applyMechanicStatus(b, lib, enemy, def)
		} else {
			prep() // Keep old pinned battles playable.
		}
	case "black_dividend":
		if _, ok := lib.Statuses[kind]; ok {
			applyMechanicStatus(b, lib, enemy, def)
			if c.Dividends == nil {
				c.Dividends = map[string]state.DividendState{}
			}
			for _, status := range b.Actors[enemy].Statuses {
				if status.DefinitionID == mechanicStatus(b, enemy, kind) {
					c.Dividends[enemy] = state.DividendState{StatusID: mechanicStatus(b, enemy, kind), Energy: content.MechanicInt(def, "energy"), Limit: content.MechanicInt(def, "rewards"), Source: actor, StatusInstance: status.InstanceID, ExpiresEffects: b.Segment.Round + 1}
				}
			}
		} else {
			prep()
		} // Old pinned catalogs retain their original rules.
	case "curse_bloom":
		if _, ok := lib.Statuses[kind]; ok {
			applyMechanicStatus(b, lib, enemy, def)
		} else {
			prep()
		}
	case "black_fingerprint", "chosen_instrument", "black_tax", "stored_calamity":
		prep()
	case "maledictions_refusal":
		if err := e.applyCurse(b, lib, enemy, def.ID, content.MechanicInt(def, "curses")); err != nil {
			return err
		}
		if _, ok := lib.Statuses["maledictions_refusal"]; ok {
			applyMechanicStatus(b, lib, enemy, def)
		} else {
			prep() // Preserve the rules of old pinned catalogs.
		}
	case "shared_misfortune", "rotten_numeral":
		limit := content.MechanicInt(def, "dice")
		clean := content.MechanicBool(def, "clean_only")
		eligible := missingCurseFace(b, enemy, n, clean)
		placed := 0
		for placed < limit && len(eligible) > 0 {
			i, err := e.namedIntn(b, "curse_target", len(eligible))
			if err != nil {
				return err
			}
			markCurse(b, enemy, eligible[i], n)
			eligible = append(eligible[:i], eligible[i+1:]...)
			placed++
		}
		if clean {
			for ; placed < limit; placed++ {
				if err := e.lesserCurse(b, lib, enemy, def.ID, -1); err != nil {
					return err
				}
			}
		}
	case "widen_the_crack":
		queueCurse(b, state.CurseWork{Kind: "adjacent", Source: actor, Target: enemy, CardID: def.ID, Die: n})
	case "unquiet_hands":
		start := len(curseRuntime(b).Logs)
		var err error
		for roll := 0; roll < content.MechanicInt(def, "rolls"); roll++ {
			_, err = e.ownedRoll(b, lib, enemy, n, "curse_dice", true)
			if err != nil {
				return err
			}
		}
		for i := start; i < len(curseRuntime(b).Logs); i++ {
			log := &curseRuntime(b).Logs[i]
			if log.Data["kind"] == "owned_roll" || log.Data["kind"] == "second_knell_trigger" {
				log.Data["source_card_id"] = def.ID
				log.Data["source_actor_id"] = actor
			}
		}
		return err
	case "tombs_choice":
		queueCurse(b, state.CurseWork{Kind: "tomb", Source: actor, Target: enemy, CardID: def.ID})
	case "three_knocks":
		applyMechanicStatus(b, lib, enemy, def)
	case "curse_eater":
		removeStatus(b, enemy, "curse_count", content.MechanicInt(def, "cost_count"))
		if key == "energy" {
			gainEnergy(b, actor, content.MechanicInt(def, "energy"))
		} else {
			for i := 0; i < content.MechanicInt(def, "draw"); i++ {
				if _, err := e.drawSettledCard(b, actor, "card_draw"); err != nil {
					return err
				}
			}
		}
	case "misfortunes_choice":
		removeStatus(b, enemy, "curse_count", content.MechanicInt(def, "cost_count"))
		queueCurse(b, state.CurseWork{Kind: "misfortune", Source: actor, Target: enemy, CardID: def.ID})
	case "call_the_mark":
		target, index, face := parseDieChoice(key)
		before := b.Settled.Actors[target].FinalDice[index].Face
		if err := e.applyEffectMutations(b, lib, instance, effectResult{DieChanges: []effectDieChange{{ActorID: target, Index: index, Face: face}}}); err != nil {
			return err
		}
		curseLog(b, target, "offensive_face_set", map[string]any{"index": index, "face_before": before, "face": face, "card_id": def.ID, "source_actor_id": actor})
		return nil
	case "no_safe_keep":
		target, index, _ := parseDieChoice(key)
		rt := b.Settled.Actors[target]
		kept := []int{}
		for _, i := range rt.KeptIndices {
			if i != index {
				kept = append(kept, i)
			}
		}
		rt.KeptIndices = kept
		faceBefore := rt.FinalDice[index].Face
		logStart := len(curseRuntime(b).Logs)
		d, err := e.ownedRoll(b, lib, target, index, "curse_dice", false)
		if err != nil {
			return err
		}
		for _, log := range curseRuntime(b).Logs[logStart:] {
			if log.Data["kind"] == "owned_roll" || log.Data["kind"] == "second_knell_trigger" {
				log.Data["offensive_reroll"] = true
				log.Data["face_before"] = faceBefore
				log.Data["card_id"] = def.ID
				log.Data["source_actor_id"] = actor
			}
		}
		d.EffectRetried = true
		rt.FinalDice[index] = d
		rt.QualifiedAbilityIDs = qualifiedAbilities(lib, rt.OffensiveAbilityIDs, rt.FinalDice, rt.AbilityModifiers)
		b.Settled.Actors[target] = rt
	case "ruin_made_flesh":
		removeStatus(b, enemy, "curse_count", n)
		if def.Mechanic != nil {
			applyMechanicStatus(b, lib, actor, def)
		}
		c.Bonus[curseBonusKey(b, actor)] = n / content.MechanicInt(def, "cost_count") * content.MechanicInt(def, "damage")
	case "blind_omen":
		removeStatus(b, enemy, "curse_count", content.MechanicInt(def, "cost_count"))
		applyStatus(b, lib, enemy, content.MechanicString(def, "status_id"), content.MechanicInt(def, "stacks"))
	case "spiteful_ward":
		s := effectDamageSourceByID(b, enemy)
		if s == nil {
			return errors.New("incoming damage source is missing")
		}
		statusID := ""
		if def.Mechanic != nil {
			applyMechanicStatus(b, lib, s.SourceActorID, def)
			statusID = mechanicStatus(b, s.SourceActorID, kind)
		}
		c.Preparations = append(c.Preparations, state.CursePreparation{Kind: kind, StatusID: statusID, CardID: def.ID, Source: actor, Target: s.SourceActorID, SourceID: s.ID, Die: s.ReactionPrevention, Round: b.Segment.Round, ExpiresEffects: b.Segment.Round + 1})
		before := settledSourceAmount(*s)
		s.ReactionPrevention += content.MechanicInt(def, "prevent")
		setUnifiedSourceAmount(b, s, max(0, before-content.MechanicInt(def, "prevent")))
		if err := e.reconcilePreventionDestination(b, def.SavedCardDestination); err != nil {
			return err
		}
	}
	curseLog(b, actor, "card", map[string]any{"card_id": def.ID, "choice": key, "targets": targets})
	return nil
}
func curseCardActions(b *state.Battle, lib content.BattleLibrary, actor string, pending state.PendingInput) []command.Command {
	var out []command.Command
	if b.Settled.Stage == stageCurseChoice {
		if w := curseRuntime(b).Active; w != nil {
			for _, key := range w.Options {
				out = append(out, legalCommand(b.ID, actor, command.TypeCommitInteraction, command.CommitInteractionPayload{PendingInputID: pending.ID, Checkpoint: interactionCheckpoint(pending), Commitment: command.InteractionCommitmentData{ChoiceID: key}}))
			}
		}
		return out
	}
	for _, instance := range mechanicCards(b, actor) {
		def := lib.Cards[b.Settled.Actors[actor].CardInstances[instance].DefinitionID]
		if def.Targeting.Selector != "curse_choice" || !mechanicPlayable(b, actor, instance, def) {
			continue
		}
		for _, v := range curseCardChoices(b, lib, actor, def) {
			if b.Settled.Stage == stageOffensivePlan {
				out = append(out, legalCommand(b.ID, actor, command.TypePlanningCards, command.PlanningCardsPayload{PendingInputID: pending.ID, Checkpoint: planningCheckpoint(pending), CardIDs: []string{instance}, TargetIDs: v.Targets, StatusID: v.Key}))
			} else {
				out = append(out, legalCommand(b.ID, actor, command.TypeCommitInteraction, command.CommitInteractionPayload{PendingInputID: pending.ID, Checkpoint: interactionCheckpoint(pending), Commitment: command.InteractionCommitmentData{CardIDs: []string{instance}, ProposalIDs: v.Targets, ChoiceID: v.Key}}))
			}
		}
	}
	return out
}
