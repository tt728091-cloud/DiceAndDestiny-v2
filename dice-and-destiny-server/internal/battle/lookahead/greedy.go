package lookahead

import (
	"encoding/json"
	"math"

	"diceanddestiny/server/internal/battle/command"
	"diceanddestiny/server/internal/battle/state"
	"diceanddestiny/server/internal/content"
)

// GreedyPolicy is the rules-aware heuristic player (level 0). It reads every
// effect from the battle's own content, never from card or ability names:
//
//   - Offensive dice follow the exact dice solver, which also plans the dice
//     cards in hand (Nudge, Try Again): it plays one only when that beats
//     rerolling or stopping with the card kept; it selects the ability tier
//     worth the most.
//   - Before rolling it plays a card only when the card's effect on the
//     solver's value beats its price.
//   - On defense it rolls its defensive ability first, then spends free
//     protection, then a prevention card if damage still gets through.
//   - At the hand limit it discards the card it values least.
//
// It is fast enough to drive the look-ahead player's simulated futures.
type GreedyPolicy struct {
	Solver *DiceSolver
	// EnergyValue is what one energy point is worth in damage, and CardValue
	// what holding a card for later is worth; a card must add more than its
	// energy at EnergyValue plus CardValue to be played.
	EnergyValue float64
	CardValue   float64
	// DrawValue is what one drawn card is worth.
	DrawValue float64
	// DiceCards lets the dice solver plan the dice cards in hand. Without it
	// (the first version) a dice card is judged against the dice as if it
	// did not exist.
	DiceCards bool
	// Bias adds value to chosen abilities, for "what if it favoured X"
	// experiments; empty in normal play.
	Bias map[string]float64
	// Label overrides the player's name in reports.
	Label string
}

// plan and fresh are the solver calls with Bias applied (plus extra).
func (p *GreedyPolicy) plan(b *state.Battle, seat string, extra map[string]float64) (*DicePlan, error) {
	p.syncSolver()
	return p.Solver.PlanWith(b, seat, p.withBias(extra))
}

func (p *GreedyPolicy) fresh(b *state.Battle, seat string, extra map[string]float64, spend int) (float64, error) {
	p.syncSolver()
	return p.Solver.FreshWith(b, seat, p.withBias(extra), spend)
}

func (p *GreedyPolicy) syncSolver() {
	p.Solver.UseCards, p.Solver.EnergyValue, p.Solver.CardValue = p.DiceCards, p.EnergyValue, p.CardValue
}

// price is what playing a card costing `energy` is worth giving up.
func (p *GreedyPolicy) price(energy int) float64 {
	return float64(energy)*p.EnergyValue + p.CardValue
}

func (p *GreedyPolicy) withBias(extra map[string]float64) map[string]float64 {
	if len(p.Bias) == 0 {
		return extra
	}
	out := map[string]float64{}
	for k, v := range p.Bias {
		out[k] += v
	}
	for k, v := range extra {
		out[k] += v
	}
	return out
}

func NewGreedyPolicy() *GreedyPolicy {
	return &GreedyPolicy{Solver: NewDiceSolver(), EnergyValue: 0.1, CardValue: 0.25, DrawValue: 0.25, DiceCards: true}
}

// NewGreedyPolicyV1 is the first version: dice cards are judged against the
// dice as if no card were in hand, and holding a card is worth nothing.
func NewGreedyPolicyV1() *GreedyPolicy {
	return &GreedyPolicy{Solver: NewDiceSolver(), EnergyValue: 0.1, DrawValue: 0.25}
}

func (p *GreedyPolicy) Name() string {
	if p.Label != "" {
		return p.Label
	}
	return "greedy"
}

func (p *GreedyPolicy) Choose(d *Decision) (int, error) {
	options := Useful(d.Battle, d.Seat, d.Actions)
	if len(options) == 1 {
		return options[0], nil
	}
	lib, err := library(d.Battle)
	if err != nil {
		return 0, err
	}
	rt := d.Battle.Settled.Actors[d.Seat]
	stage := ""
	if d.Battle.Settled.Window != nil {
		stage = d.Battle.Settled.Window.Stage
	}
	if rt.CardExecution != nil {
		return p.continueCard(d, options)
	}
	switch stage {
	case "planning":
		if d.Battle.Segment.Current == "offensive" {
			return p.offense(d, lib, options)
		}
	case "defense_selection", "damage_reaction":
		return p.defense(d, lib, options)
	case "discard_to_hand_limit":
		return p.discard(d, lib, options), nil
	}
	return fallback(d.Actions, options), nil
}

// offense plays the offensive planning step.
func (p *GreedyPolicy) offense(d *Decision, lib content.BattleLibrary, options []int) (int, error) {
	rt := d.Battle.Settled.Actors[d.Seat]
	if rt.AmbushChoose > 0 {
		return p.bestByApply(d, options, nil)
	}
	if rt.SelectedAbilityID != "" {
		return pick(d.Actions, options, command.TypePlanningPass), nil
	}
	if len(rt.FinalDice) == 0 {
		// Before the first roll: a card is worth its change in the value of
		// the whole roll.
		base, err := p.fresh(d.Battle, d.Seat, nil, 0)
		if err != nil {
			return 0, err
		}
		best, bestGain := -1, 0.0
		for _, i := range options {
			if d.Actions[i].Type != command.TypePlanningCards {
				continue
			}
			gain, err := p.cardGain(d, lib, i, func(b *state.Battle) (float64, error) { return p.fresh(b, d.Seat, nil, 0) }, base)
			if err != nil {
				return 0, err
			}
			if gain > bestGain {
				best, bestGain = i, gain
			}
		}
		if best >= 0 {
			return best, nil
		}
		return pick(d.Actions, options, command.TypePlanningRoll), nil
	}
	plan, err := p.plan(d.Battle, d.Seat, nil)
	if err != nil {
		return 0, err
	}
	if plan == nil {
		return p.selectBestAbility(d, lib, options), nil
	}
	now := plan.Best()
	best, bestValue := -1, plan.Stop
	for _, i := range options {
		switch d.Actions[i].Type {
		case command.TypePlanningReroll:
			var payload command.PlanningRerollPayload
			if json.Unmarshal(d.Actions[i].Payload, &payload) == nil {
				if v := plan.Reroll(payload.RerollIndices); v > bestValue+1e-9 {
					best, bestValue = i, v
				}
			}
		case command.TypePlanningCards:
			gain, err := p.cardGain(d, lib, i, func(b *state.Battle) (float64, error) {
				after, err := p.plan(b, d.Seat, nil)
				if err != nil || after == nil {
					return now, err
				}
				return after.Best(), nil
			}, now)
			if err != nil {
				return 0, err
			}
			// A dice card's value already counts keeping it, so it competes
			// with rerolling and stopping directly; any other card must gain.
			if v := now + gain; v > bestValue+1e-9 && (gain > 0 || (p.DiceCards && isDiceCard(d, lib, i))) {
				best, bestValue = i, v
			}
		}
	}
	if best >= 0 {
		return best, nil
	}
	return p.selectBestAbility(d, lib, options), nil
}

// selectBestAbility picks the qualified ability tier worth the most.
func (p *GreedyPolicy) selectBestAbility(d *Decision, lib content.BattleLibrary, options []int) int {
	rt := d.Battle.Settled.Actors[d.Seat]
	best, bestValue := -1, math.Inf(-1)
	for _, i := range options {
		if d.Actions[i].Type != command.TypePlanningAbility {
			continue
		}
		var payload command.PlanningAbilityPayload
		if json.Unmarshal(d.Actions[i].Payload, &payload) != nil {
			continue
		}
		v := abilityTierValue(lib, payload.AbilityID, payload.TierID) + cardDamageBonus(rt.AbilityModifiers, payload.AbilityID) + p.Bias[payload.AbilityID]
		if v > bestValue {
			best, bestValue = i, v
		}
	}
	if best >= 0 {
		return best
	}
	return fallback(d.Actions, options)
}

func abilityTierValue(lib content.BattleLibrary, abilityID, tierID string) float64 {
	def, ok := lib.Abilities[abilityID]
	if !ok || def.Qualification == nil {
		return 0
	}
	best := 0.0
	for i, tier := range def.Qualification.ActivationTiers {
		if tierID == tier.ID || (tierID == "" && i == 0) {
			if v := TierValue(tier.Operations, 0); v > best {
				best = v
			}
		}
	}
	return best
}

// cardGain values starting a card: its change in the solver's value (or in
// resources) minus its energy cost. Dice and ability-bonus effects are
// valued exactly from the card's own parameters; any other effect is tried
// on a reseeded copy.
func (p *GreedyPolicy) cardGain(d *Decision, lib content.BattleLibrary, index int, value func(*state.Battle) (float64, error), base float64) (float64, error) {
	cards := CardIDs(d.Actions[index])
	choice := CardChoice(d.Actions[index])
	if len(cards) != 1 || choice.Verb != "start" {
		return 0, nil
	}
	rt := d.Battle.Settled.Actors[d.Seat]
	definition := lib.Cards[rt.CardInstances[cards[0]].DefinitionID]
	cost := p.price(definition.Cost.Energy)
	if definition.Program == nil || len(definition.Program.Steps) != 1 {
		return p.appliedGain(d, index, value, base, cost)
	}
	step := definition.Program.Steps[0]
	switch step.Effect {
	case "energy", "draw":
		return p.resourceGain(d.Battle, d.Seat, definition) - cost, nil
	case "ability_bonus":
		if choice.Ability == "" {
			return 0, nil
		}
		extra := map[string]float64{choice.Ability: float64(content.ProgramInt(step, "damage"))}
		if len(rt.FinalDice) == 0 {
			v, err := p.fresh(d.Battle, d.Seat, extra, definition.Cost.Energy)
			return v - base - cost, err
		}
		plan, err := p.plan(d.Battle, d.Seat, extra)
		if err != nil || plan == nil {
			return 0, err
		}
		return plan.Best() - base - cost, nil
	case "adjust_die", "reroll":
		// PlayCard already charges the card's price.
		plan, err := p.plan(d.Battle, d.Seat, nil)
		if err != nil || plan == nil {
			return 0, err
		}
		v, _ := plan.PlayCard(definition, rt.FinalDice)
		return v - base, nil
	}
	return p.appliedGain(d, index, value, base, cost)
}

// appliedGain plays the card on a reseeded copy and values the result.
func (p *GreedyPolicy) appliedGain(d *Decision, index int, value func(*state.Battle) (float64, error), base, cost float64) (float64, error) {
	copy := Fork(d.Battle, d.Seat, evaluationSeed(d))
	if err := d.Sim.Apply(&copy, d.Actions[index]); err != nil {
		return 0, nil
	}
	after, err := p.finishCard(d, &copy, value)
	if err != nil {
		return 0, err
	}
	return after - base - cost, nil
}

// finishCard completes a started card's choices and values the result. A
// die choice is valued by the solver without applying it (a reroll in
// expectation); any other choice takes the first listed target.
func (p *GreedyPolicy) finishCard(d *Decision, b *state.Battle, value func(*state.Battle) (float64, error)) (float64, error) {
	for depth := 0; depth < 4 && b.Settled.Actors[d.Seat].CardExecution != nil; depth++ {
		seat, actions := d.Sim.Next(b)
		if seat != d.Seat || len(actions) == 0 {
			break
		}
		options := Useful(b, seat, actions)
		plan, err := p.plan(b, d.Seat, nil)
		if err != nil {
			return 0, err
		}
		if v, ok := bestDieChoice(plan, executingCard(b, d.Seat), actions, options); ok {
			return v, nil
		}
		if err := d.Sim.Apply(b, actions[options[0]]); err != nil {
			return math.Inf(-1), nil
		}
	}
	return value(b)
}

// bestDieChoice values the best offensive-die target among the options.
func bestDieChoice(plan *DicePlan, card content.BattleCardDefinition, actions []command.Command, options []int) (float64, bool) {
	if plan == nil {
		return 0, false
	}
	best, found := math.Inf(-1), false
	for _, i := range options {
		choice := CardChoice(actions[i])
		if choice.Kind != "offensive_die" {
			continue
		}
		found = true
		best = math.Max(best, dieChoiceValue(plan, card, choice))
	}
	return best, found
}

// dieChoiceValue values one die target of the dice card being played.
func dieChoiceValue(plan *DicePlan, card content.BattleCardDefinition, choice Choice) float64 {
	if choice.Face != nil {
		return plan.Nudge(card.ID, card.Cost.Energy, choice.Die, *choice.Face)
	}
	return plan.TryAgain(card.ID, card.Cost.Energy, choice.Die)
}

// executingCard is the definition of the card the seat is in the middle of
// playing.
func executingCard(b *state.Battle, seat string) content.BattleCardDefinition {
	rt := b.Settled.Actors[seat]
	if rt.CardExecution == nil {
		return content.BattleCardDefinition{}
	}
	lib, err := library(b)
	if err != nil {
		return content.BattleCardDefinition{}
	}
	return lib.Cards[rt.CardInstances[rt.CardExecution.CardID].DefinitionID]
}

// continueCard picks the next step of a card being played: for a die
// target it asks the solver (a set face exactly, a reroll in expectation);
// otherwise it takes the first listed target.
func (p *GreedyPolicy) continueCard(d *Decision, options []int) (int, error) {
	plan, err := p.plan(d.Battle, d.Seat, nil)
	if err != nil {
		return 0, err
	}
	best, bestValue := -1, math.Inf(-1)
	for _, i := range options {
		choice := CardChoice(d.Actions[i])
		if choice.Verb == "" {
			continue
		}
		v := 0.0
		if plan != nil && choice.Kind == "offensive_die" {
			v = dieChoiceValue(plan, executingCard(d.Battle, d.Seat), choice)
		}
		if v > bestValue {
			best, bestValue = i, v
		}
	}
	if best >= 0 {
		return best, nil
	}
	return options[0], nil
}

// resourceGain values energy and draw effects: energy at EnergyValue per
// point that fits under the cap, each card drawn at DrawValue.
func (p *GreedyPolicy) resourceGain(b *state.Battle, seat string, definition content.BattleCardDefinition) float64 {
	actor := b.Actors[seat]
	gain := 0.0
	for _, step := range definition.Program.Steps {
		switch step.Effect {
		case "energy":
			room := actor.Resources.EnergyPoints + content.ProgramInt(step, "amount")
			if limit := actor.Resources.MaxEnergyPoints; limit > 0 && room > limit {
				room = limit
			}
			gain += float64(room-actor.Resources.EnergyPoints) * p.EnergyValue
		case "draw":
			gain += p.DrawValue * float64(min(content.ProgramInt(step, "amount"), len(actor.Cards.Deck)))
		}
	}
	return gain
}

// defense rolls the defensive ability, then uses free protection, then a
// prevention card, then free energy; otherwise it passes.
func (p *GreedyPolicy) defense(d *Decision, lib content.BattleLibrary, options []int) (int, error) {
	rt := d.Battle.Settled.Actors[d.Seat]
	energy := d.Battle.Actors[d.Seat].Resources.EnergyPoints
	for _, i := range options {
		if d.Actions[i].Type == command.TypePlanningAbility {
			var payload command.PlanningAbilityPayload
			if json.Unmarshal(d.Actions[i].Payload, &payload) == nil && !payload.SpendCatalyst {
				return i, nil
			}
		}
	}
	for _, i := range options {
		if d.Actions[i].Type == command.TypeCommitInteraction && len(CardIDs(d.Actions[i])) == 0 {
			var payload command.CommitInteractionPayload
			if json.Unmarshal(d.Actions[i].Payload, &payload) == nil && len(payload.Commitment.ProposalIDs) > 0 {
				return i, nil
			}
		}
	}
	for _, effect := range []string{"prevent", "energy"} {
		for _, i := range options {
			cards := CardIDs(d.Actions[i])
			if len(cards) != 1 || CardChoice(d.Actions[i]).Verb != "start" {
				continue
			}
			definition := lib.Cards[rt.CardInstances[cards[0]].DefinitionID]
			if cardEffect(definition) != effect || definition.Cost.Energy > energy {
				continue
			}
			if effect == "energy" && p.resourceGain(d.Battle, d.Seat, definition) <= float64(definition.Cost.Energy)*p.EnergyValue {
				continue
			}
			return i, nil
		}
	}
	return fallback(d.Actions, options), nil
}

// discard drops the card it values least: extra copies first, then by what
// the card does.
func (p *GreedyPolicy) discard(d *Decision, lib content.BattleLibrary, options []int) int {
	rt := d.Battle.Settled.Actors[d.Seat]
	copies := map[string]int{}
	for _, id := range d.Battle.Actors[d.Seat].Cards.Hand {
		copies[rt.CardInstances[id].DefinitionID]++
	}
	worth := map[string]float64{"ability_bonus": 3, "energy": 2.5, "adjust_die": 2, "reroll": 2, "prevent": 1.5, "draw": 1}
	best, bestKeep := -1, math.Inf(1)
	for _, i := range options {
		cards := CardIDs(d.Actions[i])
		if len(cards) != 1 {
			continue
		}
		definition := rt.CardInstances[cards[0]].DefinitionID
		keep := worth[cardEffect(lib.Cards[definition])] - 0.5*float64(copies[definition]-1)
		if keep < bestKeep {
			best, bestKeep = i, keep
		}
	}
	if best >= 0 {
		return best
	}
	return options[0]
}

// bestByApply tries each option on a copy and keeps the one whose dice the
// solver values most (used for Ambush keeps).
func (p *GreedyPolicy) bestByApply(d *Decision, options []int, _ any) (int, error) {
	best, bestValue := options[0], math.Inf(-1)
	for _, i := range options {
		copy := Fork(d.Battle, d.Seat, evaluationSeed(d))
		if err := d.Sim.Apply(&copy, d.Actions[i]); err != nil {
			continue
		}
		plan, err := p.plan(&copy, d.Seat, nil)
		if err != nil || plan == nil {
			continue
		}
		if v := plan.Best(); v > bestValue {
			best, bestValue = i, v
		}
	}
	return best, nil
}

// evaluationSeed gives one-step evaluation copies their own randomness, so a
// copy never replays the real battle's future rolls.
func evaluationSeed(d *Decision) uint64 {
	return uint64(d.Rand.Int63()) | 1
}

func isDiceCard(d *Decision, lib content.BattleLibrary, index int) bool {
	cards := CardIDs(d.Actions[index])
	if len(cards) != 1 {
		return false
	}
	effect := cardEffect(lib.Cards[d.Battle.Settled.Actors[d.Seat].CardInstances[cards[0]].DefinitionID])
	return effect == "adjust_die" || effect == "reroll"
}

func cardEffect(definition content.BattleCardDefinition) string {
	if definition.Program == nil || len(definition.Program.Steps) == 0 {
		return ""
	}
	return definition.Program.Steps[0].Effect
}

// pick returns the first option of the wanted type, else a fallback.
func pick(actions []command.Command, options []int, kind command.Type) int {
	for _, i := range options {
		if actions[i].Type == kind {
			return i
		}
	}
	return fallback(actions, options)
}

// fallback prefers rolling, then passing.
func fallback(actions []command.Command, options []int) int {
	// planning_pass before pass: after a defensive roll, planning_pass applies
	// the roll and returns to defense selection, while pass is the main Pass
	// that gives up every remaining defense and card.
	for _, kind := range []command.Type{command.TypeRollDice, command.TypePlanningRoll, command.TypePlanningPass, command.TypePass} {
		for _, i := range options {
			if actions[i].Type == kind {
				return i
			}
		}
	}
	return options[0]
}
