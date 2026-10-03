package engine

import (
	"diceanddestiny/server/internal/battle/command"
	"diceanddestiny/server/internal/battle/operation"
	"diceanddestiny/server/internal/battle/state"
	"diceanddestiny/server/internal/content"
	"errors"
	"fmt"
	"strconv"
	"strings"
)

func isCurseDiceCard(id string) bool { return id == "call_the_mark" || id == "no_safe_keep" }
func curseCardChoices(b *state.Battle, lib content.BattleLibrary, actor string, def content.BattleCardDefinition) []venomCardChoice {
	if b.Settled.Stage == stageCurseChoice || b.Settled.Curse != nil && b.Settled.Curse.Used[actor+":"+def.ID] == b.Segment.Round || b.Actors[actor].Resources.EnergyPoints < def.Cost.Energy {
		return nil
	}
	c := curseRuntime(b)
	var out []venomCardChoice
	add := func(key, target string) { out = append(out, venomCardChoice{Key: key, Targets: []string{target}}) }
	for _, enemy := range otherActorIDs(b, actor) {
		duplicate := isCurseMode(def.ID) && stacks(b, enemy, "grave_interest") > 0 || (def.ID == "second_knell" || def.ID == "maledictions_refusal" || def.ID == "black_dividend" || def.ID == "curse_bloom") && stacks(b, enemy, def.ID) > 0
		for _, p := range c.Preparations {
			if p.Target == enemy && (p.CardID == def.ID || isCurseMode(p.CardID) && isCurseMode(def.ID)) {
				duplicate = true
			}
		}
		if duplicate {
			continue
		}
		count := stacks(b, enemy, "curse_count")
		stage := b.Settled.Stage
		plan := stage == stageOffensivePlan && containsCommand(b.Settled.Window.AllowedCommands, command.TypePlanningCards)
		switch def.ID {
		case "mark_the_number", "black_fingerprint", "maledictions_refusal", "second_knell", "grave_interest", "black_dividend", "curse_bloom", "black_tax":
			if plan {
				add("apply", enemy)
			}
		case "three_knocks":
			if plan && stacks(b, enemy, "three_knocks_status") == 0 {
				add("apply", enemy)
			}
		case "stored_calamity":
			if plan && c.LastStored[enemy] != b.Segment.Round {
				add("store", enemy)
			}
		case "shared_misfortune", "rotten_numeral":
			if plan {
				for f := 1; f <= 6; f++ {
					if def.ID == "shared_misfortune" || len(missingCurseFace(b, enemy, f, false)) > 0 {
						add(fmt.Sprint(f), enemy)
					}
				}
			}
		case "widen_the_crack", "unquiet_hands", "chosen_instrument":
			if plan {
				kind := "cursed"
				if def.ID == "widen_the_crack" {
					kind = "partly"
				}
				for _, i := range curseDice(b, enemy, kind) {
					add(fmt.Sprint(i), enemy)
				}
			}
		case "tombs_choice":
			if plan && len(curseDice(b, enemy, "cursed")) > 0 {
				add("choose", enemy)
			}
		case "curse_eater":
			if plan && count >= 3 {
				add("draw", enemy)
				add("energy", enemy)
			}
		case "misfortunes_choice":
			if plan && count >= 3 {
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
			if stage == stageOffensiveReact && containsString(b.Settled.Actors[actor].SelectedTargetIDs, enemy) {
				ops, ok := resolvedOffensiveOperations(b, lib, actor)
				damaging := false
				for _, op := range ops {
					if op.Type == "deal_damage" {
						damaging = true
					}
				}
				if ok && damaging {
					if count >= 3 {
						add("3", enemy)
					}
					if count >= 6 {
						add("6", enemy)
					}
				}
			}
		case "blind_omen":
			if stage == stageOffensiveReact && count >= 6 && stacks(b, enemy, "blind") == 0 && b.Settled.Actors[enemy].SelectedAbilityID != "" {
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
	if !containsString(b.Actors[actor].Cards.Hand, instance) {
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
	spendEnergy(b, actor, def.Cost.Energy)
	a := b.Actors[actor]
	moveCard(&a.Cards, instance, operation.ZoneHand, operation.ZoneDiscard)
	b.Actors[actor] = a
	c := curseRuntime(b)
	c.Used[actor+":"+def.ID] = b.Segment.Round
	enemy := first(targets)
	n, _ := strconv.Atoi(key)
	prep := func() {
		c.Preparations = append(c.Preparations, state.CursePreparation{CardID: def.ID, Source: actor, Target: enemy, Die: n, Round: b.Segment.Round, ExpiresEffects: b.Segment.Round + 1})
	}
	switch def.ID {
	case "mark_the_number", "hexward_retort":
		return e.applyCurse(b, lib, enemy, def.ID, 1)
	case "grave_interest":
		if _, ok := lib.Statuses["grave_interest"]; ok {
			applyStatus(b, lib, enemy, "grave_interest", 1)
		} else {
			prep()
		}
	case "second_knell":
		if _, ok := lib.Statuses["second_knell"]; ok {
			applyStatus(b, lib, enemy, "second_knell", 1)
		} else {
			prep() // Keep old pinned battles playable.
		}
	case "black_dividend":
		if _, ok := lib.Statuses[def.ID]; ok {
			applyStatus(b, lib, enemy, def.ID, 1)
			if c.Dividends == nil {
				c.Dividends = map[string]state.DividendState{}
			}
			for _, status := range b.Actors[enemy].Statuses {
				if status.DefinitionID == def.ID {
					c.Dividends[enemy] = state.DividendState{Source: actor, StatusInstance: status.InstanceID, ExpiresEffects: b.Segment.Round + 1}
				}
			}
		} else {
			prep()
		} // Old pinned catalogs retain their original rules.
	case "curse_bloom":
		if _, ok := lib.Statuses[def.ID]; ok {
			applyStatus(b, lib, enemy, def.ID, 1)
		} else {
			prep()
		}
	case "black_fingerprint", "chosen_instrument", "black_tax", "stored_calamity":
		prep()
	case "maledictions_refusal":
		if err := e.applyCurse(b, lib, enemy, def.ID, 1); err != nil {
			return err
		}
		if _, ok := lib.Statuses["maledictions_refusal"]; ok {
			applyStatus(b, lib, enemy, "maledictions_refusal", 1)
		} else {
			prep() // Preserve the rules of old pinned catalogs.
		}
	case "shared_misfortune", "rotten_numeral":
		limit := 3
		clean := false
		if def.ID == "shared_misfortune" {
			limit = 2
			clean = true
		}
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
		_, err := e.ownedRoll(b, lib, enemy, n, "curse_dice", true)
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
		applyStatus(b, lib, enemy, "three_knocks_status", 1)
	case "curse_eater":
		removeStatus(b, enemy, "curse_count", 3)
		if key == "energy" {
			gainEnergy(b, actor, 2)
		} else {
			for i := 0; i < 2; i++ {
				if _, err := e.drawSettledCard(b, actor, "card_draw"); err != nil {
					return err
				}
			}
		}
	case "misfortunes_choice":
		removeStatus(b, enemy, "curse_count", 3)
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
		c.Bonus[curseBonusKey(b, actor)] = n / 3 * 2
	case "blind_omen":
		removeStatus(b, enemy, "curse_count", 3)
		applyStatus(b, lib, enemy, "blind", 1)
	case "spiteful_ward":
		s := effectDamageSourceByID(b, enemy)
		if s == nil {
			return errors.New("incoming damage source is missing")
		}
		c.Preparations = append(c.Preparations, state.CursePreparation{CardID: def.ID, Source: actor, Target: s.SourceActorID, SourceID: s.ID, Die: s.ReactionPrevention, Round: b.Segment.Round, ExpiresEffects: b.Segment.Round + 1})
		before := settledSourceAmount(*s)
		s.ReactionPrevention += 2
		setUnifiedSourceAmount(b, s, max(0, before-2))
		if unifiedDefense(b) {
			if err := e.reconcileUnifiedDamage(b, false); err != nil {
				return err
			}
		} else {
			reconcileSettledDamage(b.Settled.PendingDamage, b)
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
	for _, instance := range b.Actors[actor].Cards.Hand {
		def := lib.Cards[b.Settled.Actors[actor].CardInstances[instance].DefinitionID]
		if def.Targeting.Selector != "curse_choice" {
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
