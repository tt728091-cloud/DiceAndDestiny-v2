package valuenet

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"math"
	"strings"

	"diceanddestiny/server/internal/battle/command"
	"diceanddestiny/server/internal/battle/lookahead"
	"diceanddestiny/server/internal/battle/mlsim"
	"diceanddestiny/server/internal/content"
)

// OptionSchemaName names the option inputs: what one legal move does, read
// from the move and the seat's own view, never which card, ability or
// character it names.
const OptionSchemaName = "option-v1"

var optionKinds = []string{"reroll", "roll", "keep", "pass", "lock_in", "ability", "targets", "card_start", "card_step", "interaction", "interaction_card", "other"}

// option describes one move.
type option struct {
	kind                                [12]float64
	anchor                              float64
	rerolled, rerollValue, rerollGain   float64
	parts                               [5]float64
	partDamage                          float64
	tierValue, tierGain, abilityCost    float64
	tierEffect                          [7]float64
	defense                             [4]float64
	cardCost, affordable                float64
	cardEffect                          [10]float64
	cardTiming                          [4]float64
	diceCard, dieChoice, solverRelative float64
}

func (o *option) emit(put func(name string, v float64)) {
	for i, name := range optionKinds {
		put("option.kind."+name, o.kind[i])
	}
	put("option.greedy_pick", o.anchor)
	for _, field := range []struct {
		name string
		v    float64
	}{
		{"rerolled", o.rerolled}, {"reroll_value", o.rerollValue}, {"reroll_gain", o.rerollGain},
		{"parts_gain.miss", o.parts[0]}, {"parts_gain.damage", o.parts[1]}, {"parts_gain.self_status", o.parts[2]},
		{"parts_gain.enemy_status", o.parts[3]}, {"parts_gain.energy", o.parts[4]}, {"parts.damage", o.partDamage},
		{"tier_value", o.tierValue}, {"tier_gain", o.tierGain}, {"ability_cost", o.abilityCost},
	} {
		put("option."+field.name, field.v)
	}
	for i, name := range summaryNames {
		put("option.tier_effect."+name, o.tierEffect[i])
	}
	for i, name := range []string{"prevent", "damage", "status", "energy"} {
		put("option.defense."+name, o.defense[i])
	}
	put("option.card_cost", o.cardCost)
	put("option.card_affordable", o.affordable)
	for i, name := range cardEffects {
		put("option.card_effect."+name, o.cardEffect[i])
	}
	for i, name := range timings {
		put("option.card_timing."+name, o.cardTiming[i])
	}
	put("option.dice_card_gain", o.diceCard)
	put("option.die_choice_gain", o.dieChoice)
	put("option.solver_scored", o.solverRelative)
}

var optionFeatures, optionHash = func() ([]string, string) {
	var names []string
	(&option{}).emit(func(name string, _ float64) { names = append(names, name) })
	h := sha256.New()
	fmt.Fprintf(h, "%s\nrates %s card-value %g\n%s\n", OptionSchemaName, encoderRates, encoderCardValue, strings.Join(names, "\n"))
	return names, hex.EncodeToString(h.Sum(nil))
}()

// OptionFeatures lists the option inputs by name, in order.
func OptionFeatures() []string { return append([]string(nil), optionFeatures...) }

// OptionWidth is the number of inputs per option.
func OptionWidth() int { return len(optionFeatures) }

// OptionSchemaHash pins the option inputs.
func OptionSchemaHash() string { return optionHash }

// EncodeOptions writes OptionWidth() inputs for each listed option (indices
// into actions) into out, one row per option. anchor is the greedy player's
// choice.
func (e *Encoder) EncodeOptions(pos Position, actions []command.Command, options []int, anchor int, out []float32) error {
	width := OptionWidth()
	if len(out) != width*len(options) {
		return fmt.Errorf("%d options need %d inputs, got room for %d", len(options), width*len(options), len(out))
	}
	plan := e.plan(pos)
	lib := pos.Library
	own := pos.View.Actors[pos.Seat]
	bestTier := math.Inf(-1)
	tiers := make([]float64, len(options))
	for k, i := range options {
		tiers[k] = math.NaN()
		if actions[i].Type != command.TypePlanningAbility {
			continue
		}
		var p command.PlanningAbilityPayload
		if json.Unmarshal(actions[i].Payload, &p) != nil {
			continue
		}
		if def, ok := lib.Abilities[p.AbilityID]; ok && def.Qualification != nil {
			tiers[k] = tierValueOf(def, p.TierID) + damageBonus(own.AbilityModifiers, p.AbilityID)
			bestTier = math.Max(bestTier, tiers[k])
		}
	}
	extraDice := 0
	for _, st := range own.Statuses {
		if def := lib.Statuses[st.DefinitionID]; def.Modifiers != nil {
			extraDice += def.Modifiers.DefenseDiceBonus * st.Stacks
		}
	}
	for k, i := range options {
		var o option
		action := actions[i]
		o.kind[optionKind(action)] = 1
		if i == anchor {
			o.anchor = 1
		}
		switch action.Type {
		case command.TypePlanningReroll:
			var p command.PlanningRerollPayload
			if json.Unmarshal(action.Payload, &p) == nil && plan != nil {
				v := plan.plan.Reroll(p.RerollIndices)
				if !math.IsInf(v, 0) {
					o.rerolled = float64(len(p.RerollIndices)) / 5
					o.rerollValue, o.rerollGain = v/10, (v-plan.best)/5
					o.setParts(plan.plan.RerollParts(p.RerollIndices), plan.parts)
					o.solverRelative = 1
				}
			}
		case command.TypePlanningAbility:
			var p command.PlanningAbilityPayload
			if json.Unmarshal(action.Payload, &p) != nil {
				break
			}
			def, ok := lib.Abilities[p.AbilityID]
			if !ok {
				break
			}
			o.abilityCost = float64(def.Cost.Energy) / 10
			if !math.IsNaN(tiers[k]) {
				o.tierValue, o.tierGain = tiers[k]/10, (tiers[k]-bestTier)/5
				for _, tier := range def.Qualification.ActivationTiers {
					if tier.ID == p.TierID || (p.TierID == "" && tier.ID == def.Qualification.ActivationTiers[0].ID) {
						for j, v := range mlsim.V2OperationSummary(tier.Operations) {
							o.tierEffect[j] = float64(v) / 10
						}
					}
				}
				if plan != nil {
					o.solverRelative = 1
					o.rerollValue, o.rerollGain = tiers[k]/10, (tiers[k]-plan.best)/5
					parts := lookahead.TierParts(def, p.TierID, damageBonus(own.AbilityModifiers, p.AbilityID))
					o.setParts(lookahead.Parts{0, parts[0], parts[1], parts[2], parts[3]}, plan.parts)
				}
			}
			if def.Resolution != nil {
				effect := expectedEffect(def.Resolution.Operations, extraDice)
				o.defense = [4]float64{effect[1] * 20 / 5, effect[0] * 20 / 5, effect[2] * 10 / 3, effect[3] * 10 / 5}
			}
		}
		if cards := lookahead.CardIDs(action); len(cards) == 1 {
			if def, ok := lib.Cards[own.CardInstances[cards[0]].DefinitionID]; ok {
				o.cardCost = float64(def.Cost.Energy) / 10
				if def.Cost.Energy <= own.EnergyPoints {
					o.affordable = 1
				}
				effect := cardEffect(def)
				for j := range effect {
					o.cardEffect[j] = effect[j] / cardEffectScale[j]
				}
				for j, on := range cardTiming(def) {
					if on {
						o.cardTiming[j] = 1
					}
				}
				choice := lookahead.CardChoice(action)
				if plan != nil {
					switch {
					case choice.Kind == "offensive_die" && choice.Face != nil:
						o.dieChoice = (plan.plan.Nudge(def.ID, def.Cost.Energy, choice.Die, *choice.Face) - plan.best) / 5
						o.solverRelative = 1
					case choice.Kind == "offensive_die":
						o.dieChoice = (plan.plan.TryAgain(def.ID, def.Cost.Energy, choice.Die) - plan.best) / 5
						o.solverRelative = 1
					case choice.Verb == "start":
						if v, ok := plan.plan.PlayCard(def, own.Dice.Dice); ok && !math.IsInf(v, 0) {
							o.diceCard = (v - plan.best) / 5
							o.solverRelative = 1
						}
					}
				}
			}
		}
		if (action.Type == command.TypePlanningPass || action.Type == command.TypePlanningLockIn) && plan != nil {
			o.rerollValue, o.rerollGain, o.solverRelative = plan.plan.Stop/10, (plan.plan.Stop-plan.best)/5, 1
			o.setParts(plan.plan.StopParts(), plan.parts)
		}
		row := out[k*width : (k+1)*width]
		j := 0
		o.emit(func(_ string, v float64) {
			if math.IsNaN(v) || math.IsInf(v, 0) {
				v = 0
			}
			row[j] = float32(math.Max(-10, math.Min(10, v)))
			j++
		})
	}
	return nil
}

// setParts records an option's expected results against the plan's: the
// change in miss chance, damage, status stacks on self and on others and
// energy, and its damage.
func (o *option) setParts(parts, now lookahead.Parts) {
	scale := [5]float64{1, 5, 3, 3, 10}
	for k := range parts {
		o.parts[k] = (parts[k] - now[k]) / scale[k]
	}
	o.partDamage = parts[1] / 10
}

func optionKind(action command.Command) int {
	name := "other"
	switch action.Type {
	case command.TypePlanningReroll:
		name = "reroll"
	case command.TypePlanningRoll:
		name = "roll"
	case command.TypePlanningKeep:
		name = "keep"
	case command.TypePlanningPass, command.TypePass:
		name = "pass"
	case command.TypePlanningLockIn:
		name = "lock_in"
	case command.TypePlanningAbility:
		name = "ability"
	case command.TypePlanningTargets:
		name = "targets"
	case command.TypePlanningCards, command.TypeCommitInteraction:
		choice := lookahead.CardChoice(action)
		cards := lookahead.CardIDs(action)
		switch {
		case choice.Verb != "" && choice.Verb != "start":
			name = "card_step"
		case len(cards) > 0 && action.Type == command.TypePlanningCards:
			name = "card_start"
		case len(cards) > 0:
			name = "interaction_card"
		case action.Type == command.TypeCommitInteraction:
			name = "interaction"
		}
	}
	return indexOf(optionKinds, name)
}

func tierValueOf(def content.BattleAbilityDefinition, tierID string) float64 {
	for i, tier := range def.Qualification.ActivationTiers {
		if tier.ID == tierID || (tierID == "" && i == 0) {
			return encoderRates.TierValue(tier.Operations)
		}
	}
	return 0
}
