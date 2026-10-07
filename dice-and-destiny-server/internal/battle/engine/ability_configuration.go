package engine

import (
	"diceanddestiny/server/internal/battle/state"
	"diceanddestiny/server/internal/content"
)

func chooseAbilityTier(a content.BattleAbilityDefinition) bool {
	return a.Qualification != nil && (a.Qualification.ChooseTier || (a.ConfigurationVersion == 0 && (a.ID == "needlefang" || a.ID == "hexbrand" || a.ID == "adventurer_strike")))
}
func abilityPayment(a content.BattleAbilityDefinition) *content.AbilityPayment {
	if a.OptionalPayment != nil {
		return a.OptionalPayment
	}
	if a.ConfigurationVersion == 0 && a.ID == "shedskin" {
		return &content.AbilityPayment{StatusID: "catalyst", Stacks: 1, Prevention: 2}
	}
	return nil
}
func canPayAbility(b *state.Battle, actor string, a content.BattleAbilityDefinition) bool {
	p := abilityPayment(a)
	return p != nil && stacks(b, actor, p.StatusID) >= p.Stacks
}
func abilityOnlyDefense(a content.BattleAbilityDefinition) bool {
	return a.Selection != nil && (a.Selection.OffensiveAbilitiesOnly || (a.ConfigurationVersion == 0 && a.ID == "barbed_mantle"))
}
func provokeCount(ops []content.BattleOperation) int {
	n := 0
	for _, op := range ops {
		if op.Type == "provoke" {
			v, _ := operationAmount(op, 0)
			n += v
		}
	}
	return n
}
func (e Engine) runAbilityHooks(b *state.Battle, lib content.BattleLibrary, a content.BattleAbilityDefinition, timing, actor, target, source, tier string, faces []int, prevented bool) error {
	for _, h := range a.Hooks {
		if h.Timing != timing || (h.TierID != "" && h.TierID != tier) || (h.RequiresPrevention && !prevented) {
			continue
		}
		if len(h.FacesAny) > 0 {
			matched := false
			for _, f := range faces {
				matched = matched || containsInt(h.FacesAny, f)
			}
			if !matched {
				continue
			}
		}
		ctx := effectContext{SourceActorID: actor, SourceContentID: a.ID, SourceContentType: "ability", TargetActorIDs: []string{target}, ProposalIDs: []string{source}}
		ops := []content.BattleOperation{}
		for _, op := range h.Operations {
			if timing == "after_damage" && b.Actors[target].CurrentHealth() == 0 && op.Target != "self" && op.Target != "source_actor" && op.Target != "" {
				continue
			}
			ops = append(ops, op)
		}
		for i := range ops {
			if ops[i].Target == "enemy" {
				ops[i].Target = "selected_targets"
			}
		}
		result, err := e.executeEffects(b, lib, ctx, ops)
		if err != nil {
			return err
		}
		if err = e.applyEffectMutations(b, lib, "", result); err != nil {
			return err
		}
	}
	return nil
}
func (e Engine) executeSharedSpecial(b *state.Battle, lib content.BattleLibrary, ctx effectContext, targets []string, s *content.SharedSpecialEffect) error {
	for _, target := range targets {
		switch s.Kind {
		case "curse":
			if err := e.applyCurse(b, lib, target, ctx.SourceContentID, s.Amount); err != nil {
				return err
			}
		case "roll_cursed":
			eligible := curseDice(b, target, "cursed")
			if len(eligible) > 0 {
				if _, err := e.rollOwnedSubset(b, lib, target, ctx.SourceContentID, eligible, min(s.Amount, len(eligible))); err != nil {
					return err
				}
			}
		case "conditional_status":
			id, n := s.FallbackStatusID, s.FallbackStacks
			matched := stacks(b, target, s.StatusID) >= s.Threshold && stacks(b, target, s.ResultStatusID) < s.Limit
			if matched {
				id, n = s.ResultStatusID, s.Stacks
			}
			r := effectResult{StatusApplications: []state.SettledStatusApplication{{SourceActorID: ctx.SourceActorID, TargetActorID: target, StatusID: id, Stacks: n, RequirePoison: matched && s.StatusID == "poison" && id == "incubation"}}}
			if err := e.applyEffectMutations(b, lib, "", r); err != nil {
				return err
			}
		case "status_threshold":
			if stacks(b, target, s.StatusID) >= s.Threshold {
				if err := e.applyEffectMutations(b, lib, "", effectResult{StatusApplications: []state.SettledStatusApplication{{SourceActorID: ctx.SourceActorID, TargetActorID: target, StatusID: s.ResultStatusID, Stacks: s.Stacks}}}); err != nil {
					return err
				}
			}
		case "entomb_choice":
			queueCurse(b, state.CurseWork{Kind: "grasp", Source: ctx.SourceActorID, Target: target, CardID: ctx.SourceContentID, Amount: s.Amount, Limit: s.Limit, Configured: true})
		case "curse_face_choice":
			queueCurse(b, state.CurseWork{Kind: "eclipse", Source: ctx.SourceActorID, Target: target, CardID: ctx.SourceContentID, Amount: s.Amount, Configured: true})
		case "curse_die_choice":
			queueCurse(b, state.CurseWork{Kind: "repaid", Source: ctx.SourceActorID, Target: target, CardID: ctx.SourceContentID, Face: s.Face, DefenseSourceID: first(ctx.ProposalIDs), Configured: true})
		}
	}
	return nil
}

// Pure preview: never execute effects a second time to check a hook condition.
func configuredPrevention(ops []content.BattleOperation, face int) int {
	n := 0
	for _, op := range ops {
		if op.Type == "prevent_damage" {
			v, _ := operationAmount(op, face)
			n += v
		}
		if op.Type == "roll_dice" {
			for _, o := range op.Outcomes {
				if containsInt(o.Faces, face) {
					n += configuredPrevention(o.Operations, face)
				}
			}
		}
	}
	return n
}

// Reserve the cost on commitment. A reaction replan pays only the difference;
// returning to the same selection never charges twice.
func reserveOffensiveCost(b *state.Battle, lib content.BattleLibrary, actor string) bool {
	rt := b.Settled.Actors[actor]
	cost := 0
	if rt.SelectedAbilityID != "" {
		cost = lib.Abilities[rt.SelectedAbilityID].Cost.Energy
	}
	delta := cost - rt.PaidOffensiveEnergy
	if delta > b.Actors[actor].Resources.EnergyPoints {
		return false
	}
	if delta > 0 {
		spendEnergy(b, actor, delta)
	} else if delta < 0 {
		gainEnergy(b, actor, -delta)
	}
	rt.PaidOffensiveEnergy = cost
	b.Settled.Actors[actor] = rt
	return true
}

// AbilityFitsDice uses the real qualification evaluator, also used during play.
// Authoring uses it to reject a board whose abilities can never be activated.
func AbilityFitsDice(a content.BattleAbilityDefinition, loadout []content.DiceLoadoutEntry, lib content.BattleLibrary) bool {
	if a.Type == "defensive" {
		return true
	}
	dice := []state.RolledDie{}
	defs := []content.BattleDieDefinition{}
	for _, entry := range loadout {
		for i := 0; i < entry.Count; i++ {
			defs = append(defs, lib.Dice[entry.DiceID])
			dice = append(dice, state.RolledDie{})
		}
	}
	var visit func(int) bool
	visit = func(i int) bool {
		if i == len(dice) {
			_, ok := qualifiedTier(a, dice)
			return ok
		}
		for _, face := range defs[i].Faces {
			dice[i] = state.RolledDie{Face: face.Number, Symbols: []string{face.Symbol}}
			if visit(i + 1) {
				return true
			}
		}
		return false
	}
	return visit(0)
}
