package lookahead

import (
	"encoding/json"
	"fmt"
	"math"
	"sort"
	"strings"

	"diceanddestiny/server/internal/battle/engine"
	"diceanddestiny/server/internal/battle/state"
	"diceanddestiny/server/internal/content"
)

// DiceSolver plans offensive dice exactly. From the dice on the table, the
// rolls left and the dice cards in hand (a card that moves one die by a step,
// like Nudge, or rerolls one die without spending a roll, like Try Again,
// with the energy to pay for them) it computes the expected value of the
// best qualified ability under optimal play: reroll some dice, play a card on
// a die, or stop. Qualification uses the engine's own rules. An ability tier
// is worth its damage plus any active card bonus on it, plus a little for
// side benefits; every dice card played costs its energy at EnergyValue plus
// CardValue for the card itself, so a card is kept for later unless using it
// now is worth more. Only pools of identical dice are solved; mixed pools
// fall back to playouts.
//
// A DiceSolver caches its tables and is not safe for concurrent use.
type DiceSolver struct {
	// UseCards counts dice cards in hand. Without it the solver plans
	// rerolls only (the first version) and prices a card when asked.
	UseCards    bool
	EnergyValue float64
	CardValue   float64
	tables      map[string]*diceTable
}

func NewDiceSolver() *DiceSolver {
	return &DiceSolver{UseCards: true, EnergyValue: 0.1, CardValue: 0.25, tables: map[string]*diceTable{}}
}

// tierValue is one activation tier wrapped as a single-tier ability so the
// engine's qualification check can test it on its own.
type tierValue struct {
	ability string
	tier    string
	check   content.BattleAbilityDefinition
	value   float64
}

// diceTool is a dice card the solver can plan with.
type diceTool struct {
	card     string
	reroll   bool  // reroll one die for free; otherwise move it by a delta
	deltas   []int // for moves
	min, max int
	cost     int
	price    float64
}

const maxTools = 4

// toolState is how many of each dice card are in hand and the energy left
// to pay for them.
type toolState struct {
	counts [maxTools]int
	energy int
}

func (ts toolState) key(rolls int) int {
	k := rolls*512 + ts.energy
	for _, c := range ts.counts {
		k = k*16 + c
	}
	return k
}

func (ts toolState) use(i int, cost int) toolState {
	ts.counts[i]--
	ts.energy -= cost
	return ts
}

// multisets lists every multiset of up to n dice over the faces, indexed.
type multisets struct {
	n, sides int
	list     [][]int
	size     []int
	index    map[int]int
	radix    []int
}

func newMultisets(n, sides int) *multisets {
	m := &multisets{n: n, sides: sides, index: map[int]int{}, radix: make([]int, sides)}
	r := 1
	for s := 0; s < sides; s++ {
		m.radix[s] = r
		r *= n + 1
	}
	counts := make([]int, sides)
	var visit func(slot, left int)
	visit = func(slot, left int) {
		if slot == sides {
			total := 0
			for _, c := range counts {
				total += c
			}
			m.index[m.key(counts)] = len(m.list)
			m.list = append(m.list, append([]int(nil), counts...))
			m.size = append(m.size, total)
			return
		}
		for c := 0; c <= left; c++ {
			counts[slot] = c
			visit(slot+1, left-c)
		}
	}
	visit(0, n)
	return m
}

func (m *multisets) key(counts []int) int {
	k := 0
	for s, c := range counts {
		k += c * m.radix[s]
	}
	return k
}

func (m *multisets) lookup(counts []int) int {
	if i, ok := m.index[m.key(counts)]; ok {
		return i
	}
	return -1
}

// replace is multiset i with one die moved from slot `from` to slot `to`.
func (m *multisets) replace(i, from, to int) int {
	if from == to {
		return i
	}
	return m.index[m.key(m.list[i])-m.radix[from]+m.radix[to]]
}

type outcome struct {
	set  int // multiset index of the rolled dice
	prob float64
}

type diceTable struct {
	dieID       string
	n           int
	faces       []state.DiceFace
	tiers       []tierValue
	tools       []diceTool
	sets        *multisets
	full        []int         // multisets of all n dice
	keeps       map[int][]int // full multiset -> kept sub-multisets (at least one die rerolled)
	outs        map[int][]outcome
	join        map[int][]int // kept multiset -> result of each outcome of the rest
	stop        map[int]float64
	stopTier    map[int]tierValue
	layers      map[int][]float64
	waits       map[int][]float64
	energyValue float64
	cardValue   float64
}

// DicePlan is the solver's view of one offensive planning moment.
type DicePlan struct {
	Stop        float64 // value of selecting the best ability now
	BestAbility string
	BestTier    string
	RollsLeft   int
	table       *diceTable
	set         int
	tools       toolState
	slotAt      map[int]int // die index -> face slot
}

// Plan returns nil when the seat's dice cannot be solved exactly.
func (s *DiceSolver) Plan(b *state.Battle, seat string) (*DicePlan, error) {
	return s.PlanWith(b, seat, nil)
}

// PlanWith is Plan with extra damage on some abilities (a card being
// considered, such as Strong Swing).
func (s *DiceSolver) PlanWith(b *state.Battle, seat string, extra map[string]float64) (*DicePlan, error) {
	if b.Settled == nil {
		return nil, nil
	}
	rt := b.Settled.Actors[seat]
	if len(rt.FinalDice) == 0 {
		return nil, nil
	}
	dieID := rt.FinalDice[0].DieID
	for _, die := range rt.FinalDice {
		if die.DieID != dieID {
			return nil, nil
		}
	}
	definition, ok := b.DiceDefinitions[dieID]
	if !ok || len(definition.Faces) == 0 {
		return nil, nil
	}
	lib, err := library(b)
	if err != nil {
		return nil, err
	}
	tools, ts := s.diceTools(b, lib, seat, 0)
	table := s.table(definition, len(rt.FinalDice), offensiveTierValues(b, lib, seat, extra), tools)
	plan := &DicePlan{RollsLeft: rt.MaxRolls - rt.RollsUsed, table: table, tools: ts, slotAt: map[int]int{}}
	counts := make([]int, len(table.faces))
	for _, die := range rt.FinalDice {
		slot := table.slot(die.Face)
		if slot < 0 {
			return nil, nil
		}
		counts[slot]++
		plan.slotAt[die.Index] = slot
	}
	plan.set = table.sets.lookup(counts)
	plan.Stop = table.stop[plan.set]
	best := table.stopTier[plan.set]
	plan.BestAbility, plan.BestTier = best.ability, best.tier
	return plan, nil
}

// Best returns the value of optimal play from here.
func (p *DicePlan) Best() float64 {
	return p.table.layer(p.RollsLeft, p.tools)[p.set]
}

// Reroll returns the expected value of rerolling the dice at these indices
// and then playing on optimally.
func (p *DicePlan) Reroll(indices []int) float64 {
	if p.RollsLeft <= 0 || len(indices) == 0 {
		return p.Stop
	}
	kept := append([]int(nil), p.table.sets.list[p.set]...)
	for _, index := range indices {
		slot, ok := p.slotAt[index]
		if !ok || kept[slot] == 0 {
			return math.Inf(-1)
		}
		kept[slot]--
	}
	return p.table.wait(p.RollsLeft, p.tools)[p.table.sets.lookup(kept)]
}

// Nudge values playing dice card `card` (costing `cost` energy) to set the
// die at `index` to `face`, then playing on optimally, net of the card's
// price. A card the solver does not plan with is priced and the rest of the
// turn played without it.
func (p *DicePlan) Nudge(card string, cost, index, face int) float64 {
	from, ok := p.slotAt[index]
	to := p.table.slot(face)
	if !ok || to < 0 {
		return math.Inf(-1)
	}
	next := p.table.sets.replace(p.set, from, to)
	if i := p.table.toolIndex(card, false); i >= 0 && p.table.usable(p.tools, i) {
		return p.table.layer(p.RollsLeft, p.tools.use(i, p.table.tools[i].cost))[next] - p.table.tools[i].price
	}
	return p.table.layer(p.RollsLeft, p.tools)[next] - p.table.price(cost)
}

// TryAgain values playing dice card `card` to reroll the die at `index`
// without spending a roll, net of the card's price.
func (p *DicePlan) TryAgain(card string, cost, index int) float64 {
	from, ok := p.slotAt[index]
	if !ok {
		return math.Inf(-1)
	}
	values, price := p.table.layer(p.RollsLeft, p.tools), p.table.price(cost)
	if i := p.table.toolIndex(card, true); i >= 0 && p.table.usable(p.tools, i) {
		values, price = p.table.layer(p.RollsLeft, p.tools.use(i, p.table.tools[i].cost)), p.table.tools[i].price
	}
	total := 0.0
	for to := range p.table.faces {
		total += values[p.table.sets.replace(p.set, from, to)]
	}
	return total/float64(len(p.table.faces)) - price
}

// PlayCard values playing a dice card now on its best die (net of its
// price), and reports whether the card is a dice card at all.
func (p *DicePlan) PlayCard(card content.BattleCardDefinition, dice []state.RolledDie) (float64, bool) {
	if card.Program == nil || len(card.Program.Steps) != 1 {
		return 0, false
	}
	step := card.Program.Steps[0]
	if step.Effect != "adjust_die" && step.Effect != "reroll" {
		return 0, false
	}
	best := math.Inf(-1)
	for _, die := range dice {
		if step.Effect == "reroll" {
			best = math.Max(best, p.TryAgain(card.ID, card.Cost.Energy, die.Index))
			continue
		}
		for _, delta := range content.ProgramInts(step, "deltas") {
			face := die.Face + delta
			if face < content.ProgramInt(step, "minimum") || face > content.ProgramInt(step, "maximum") {
				continue
			}
			best = math.Max(best, p.Nudge(card.ID, card.Cost.Energy, die.Index, face))
		}
	}
	return best, true
}

// Fresh values a whole offensive roll that has not started yet: the
// expected best ability over all of the seat's rolls.
func (s *DiceSolver) Fresh(b *state.Battle, seat string) (float64, error) {
	return s.FreshWith(b, seat, nil, 0)
}

// FreshWith is Fresh with extra damage on some abilities and `spend` energy
// already committed (to the card being considered).
func (s *DiceSolver) FreshWith(b *state.Battle, seat string, extra map[string]float64, spend int) (float64, error) {
	if b.Settled == nil {
		return 0, nil
	}
	actor := b.Actors[seat]
	if len(actor.DiceLoadout) != 1 {
		return 0, nil
	}
	dieID, dice := actor.DiceLoadout[0].DiceID, actor.DiceLoadout[0].Count
	definition, ok := b.DiceDefinitions[dieID]
	if !ok || len(definition.Faces) == 0 || dice <= 0 {
		return 0, nil
	}
	lib, err := library(b)
	if err != nil {
		return 0, err
	}
	tools, ts := s.diceTools(b, lib, seat, spend)
	table := s.table(definition, dice, offensiveTierValues(b, lib, seat, extra), tools)
	rolls := b.Settled.Actors[seat].MaxRolls
	if rolls <= 0 {
		rolls = 3
	}
	return table.wait(rolls, ts)[table.sets.lookup(make([]int, len(table.faces)))], nil
}

// diceTools lists the seat's dice cards in hand and the energy to play them
// (less `spend`). Cards that need a condition, pick dice at random, wrap
// around or target other dice are left to the playouts.
func (s *DiceSolver) diceTools(b *state.Battle, lib content.BattleLibrary, seat string, spend int) ([]diceTool, toolState) {
	var ts toolState
	ts.energy = max(0, b.Actors[seat].Resources.EnergyPoints-spend)
	if !s.UseCards {
		return nil, ts
	}
	rt := b.Settled.Actors[seat]
	counts := map[string]int{}
	var tools []diceTool
	for _, instance := range b.Actors[seat].Cards.Hand {
		id := rt.CardInstances[instance].DefinitionID
		if counts[id] > 0 {
			counts[id]++
			continue
		}
		tool, ok := s.diceTool(lib.Cards[id])
		if !ok {
			continue
		}
		counts[id] = 1
		tools = append(tools, tool)
	}
	sort.Slice(tools, func(i, j int) bool { return tools[i].card < tools[j].card })
	if len(tools) > maxTools {
		tools = tools[:maxTools]
	}
	needed := 0
	for i, tool := range tools {
		n := counts[tool.card]
		if limit := lib.Cards[tool.card].Program.UsesPerRound; limit > 0 {
			n = min(n, max(0, limit-rt.CardUses[fmt.Sprintf("%d:%s", b.Segment.Round, tool.card)]))
		}
		ts.counts[i] = min(n, 15)
		needed += ts.counts[i] * tool.cost
	}
	ts.energy = min(ts.energy, needed, 511)
	return tools, ts
}

func (s *DiceSolver) diceTool(def content.BattleCardDefinition) (diceTool, bool) {
	if def.Program == nil || len(def.Program.Steps) != 1 {
		return diceTool{}, false
	}
	step := def.Program.Steps[0]
	after := false
	for _, w := range def.Program.Windows {
		after = after || w == "offensive_after_roll" || w == "offensive_planning"
	}
	if !after || step.Condition != nil || step.Target.Owner != "self" || step.Target.Selection == "random" || (step.Target.Count != 0 && step.Target.Count != 1) {
		return diceTool{}, false
	}
	tool := diceTool{card: def.ID, cost: def.Cost.Energy, price: s.EnergyValue*float64(def.Cost.Energy) + s.CardValue}
	switch step.Effect {
	case "reroll":
		tool.reroll = true
	case "adjust_die":
		if content.ProgramBool(step, "wrap") {
			return diceTool{}, false
		}
		tool.deltas = content.ProgramInts(step, "deltas")
		tool.min, tool.max = content.ProgramInt(step, "minimum"), content.ProgramInt(step, "maximum")
	default:
		return diceTool{}, false
	}
	return tool, true
}

func (s *DiceSolver) table(definition state.DiceDefinition, dice int, tiers []tierValue, tools []diceTool) *diceTable {
	key := tableKey(definition.ID, dice, tiers, tools, s.EnergyValue, s.CardValue)
	if t := s.tables[key]; t != nil {
		return t
	}
	t := newDiceTable(definition, dice, tiers, tools)
	t.energyValue, t.cardValue = s.EnergyValue, s.CardValue
	s.tables[key] = t
	return t
}

func newDiceTable(definition state.DiceDefinition, dice int, tiers []tierValue, tools []diceTool) *diceTable {
	faces := append([]state.DiceFace(nil), definition.Faces...)
	sort.Slice(faces, func(i, j int) bool { return faces[i].Face < faces[j].Face })
	t := &diceTable{dieID: definition.ID, n: dice, faces: faces, tiers: tiers, tools: tools,
		sets: newMultisets(dice, len(faces)), keeps: map[int][]int{}, outs: map[int][]outcome{}, join: map[int][]int{},
		stop: map[int]float64{}, stopTier: map[int]tierValue{}, layers: map[int][]float64{}, waits: map[int][]float64{}}
	for i, size := range t.sets.size {
		if size == dice {
			t.full = append(t.full, i)
			t.stop[i], t.stopTier[i] = t.best(t.sets.list[i])
		}
	}
	return t
}

func (t *diceTable) slot(face int) int {
	for i, f := range t.faces {
		if f.Face == face {
			return i
		}
	}
	return -1
}

func (t *diceTable) price(cost int) float64 {
	return t.energyValue*float64(cost) + t.cardValue
}

func (t *diceTable) toolIndex(card string, reroll bool) int {
	for i, tool := range t.tools {
		if tool.card == card && tool.reroll == reroll {
			return i
		}
	}
	return -1
}

func (t *diceTable) usable(ts toolState, i int) bool {
	return ts.counts[i] > 0 && ts.energy >= t.tools[i].cost
}

// best is the most valuable tier the dice qualify for.
func (t *diceTable) best(counts []int) (float64, tierValue) {
	dice := make([]state.RolledDie, 0, t.n)
	for slot, c := range counts {
		face := t.faces[slot]
		for i := 0; i < c; i++ {
			dice = append(dice, state.RolledDie{Index: len(dice), DieID: t.dieID, Face: face.Face, Value: face.Value, Symbols: face.Symbols})
		}
	}
	var best tierValue
	value := 0.0
	for _, tier := range t.tiers {
		if tier.value <= value {
			continue
		}
		if _, ok := engine.QualifiedAbilityTier(tier.check, dice); ok {
			value, best = tier.value, tier
		}
	}
	return value, best
}

// layer is the optimal value of every full multiset with `rolls` rerolls and
// the cards in `ts` left.
func (t *diceTable) layer(rolls int, ts toolState) []float64 {
	key := ts.key(rolls)
	if v, ok := t.layers[key]; ok {
		return v
	}
	var wait []float64
	if rolls > 0 {
		wait = t.wait(rolls, ts)
	}
	lower := make([][]float64, len(t.tools))
	for i := range t.tools {
		if t.usable(ts, i) {
			lower[i] = t.layer(rolls, ts.use(i, t.tools[i].cost))
		}
	}
	values := make([]float64, len(t.sets.list))
	for _, m := range t.full {
		v := t.stop[m]
		if wait != nil {
			for _, kept := range t.keepsOf(m) {
				if wait[kept] > v {
					v = wait[kept]
				}
			}
		}
		for i, tool := range t.tools {
			if lower[i] == nil {
				continue
			}
			if w := t.toolValue(m, tool, lower[i]); w > v {
				v = w
			}
		}
		values[m] = v
	}
	t.layers[key] = values
	return values
}

// toolValue is the best value of playing a dice card on some die of m, given
// the values once the card is spent.
func (t *diceTable) toolValue(m int, tool diceTool, lower []float64) float64 {
	best := math.Inf(-1)
	counts := t.sets.list[m]
	for from, c := range counts {
		if c == 0 {
			continue
		}
		if tool.reroll {
			total := 0.0
			for to := range t.faces {
				total += lower[t.sets.replace(m, from, to)]
			}
			best = math.Max(best, total/float64(len(t.faces)))
			continue
		}
		for _, delta := range tool.deltas {
			face := t.faces[from].Face + delta
			to := t.slot(face)
			if to < 0 || face < tool.min || face > tool.max {
				continue
			}
			best = math.Max(best, lower[t.sets.replace(m, from, to)])
		}
	}
	return best - tool.price
}

// wait is, for every kept multiset, the expected value of rolling the rest
// and playing on with rolls-1 rerolls.
func (t *diceTable) wait(rolls int, ts toolState) []float64 {
	key := ts.key(rolls)
	if w, ok := t.waits[key]; ok {
		return w
	}
	next := t.layer(rolls-1, ts)
	values := make([]float64, len(t.sets.list))
	for kept, size := range t.sets.size {
		if size >= t.n {
			continue
		}
		total := 0.0
		joined := t.joinOf(kept)
		for j, o := range t.outcomes(t.n - size) {
			total += o.prob * next[joined[j]]
		}
		values[kept] = total
	}
	t.waits[key] = values
	return values
}

func (t *diceTable) keepsOf(m int) []int {
	if k, ok := t.keeps[m]; ok {
		return k
	}
	counts := t.sets.list[m]
	kept := make([]int, len(counts))
	var out []int
	var visit func(slot int)
	visit = func(slot int) {
		if slot == len(counts) {
			if i := t.sets.lookup(kept); i >= 0 && t.sets.size[i] < t.n {
				out = append(out, i)
			}
			return
		}
		for c := 0; c <= counts[slot]; c++ {
			kept[slot] = c
			visit(slot + 1)
		}
	}
	visit(0)
	t.keeps[m] = out
	return out
}

func (t *diceTable) joinOf(kept int) []int {
	if j, ok := t.join[kept]; ok {
		return j
	}
	base := t.sets.list[kept]
	outs := t.outcomes(t.n - t.sets.size[kept])
	joined := make([]int, len(outs))
	sum := make([]int, len(base))
	for j, o := range outs {
		for s := range sum {
			sum[s] = base[s] + t.sets.list[o.set][s]
		}
		joined[j] = t.sets.lookup(sum)
	}
	t.join[kept] = joined
	return joined
}

// outcomes lists every multiset of k fair rolls with its probability.
func (t *diceTable) outcomes(k int) []outcome {
	if list, ok := t.outs[k]; ok {
		return list
	}
	sides := float64(len(t.faces))
	var list []outcome
	for i, size := range t.sets.size {
		if size != k {
			continue
		}
		ways := factorial(k)
		for _, c := range t.sets.list[i] {
			ways /= factorial(c)
		}
		list = append(list, outcome{set: i, prob: ways / math.Pow(sides, float64(k))})
	}
	t.outs[k] = list
	return list
}

func factorial(n int) float64 {
	out := 1.0
	for i := 2; i <= n; i++ {
		out *= float64(i)
	}
	return out
}

// offensiveTierValues values each tier of the seat's offensive abilities.
func offensiveTierValues(b *state.Battle, lib content.BattleLibrary, seat string, extra map[string]float64) []tierValue {
	rt := b.Settled.Actors[seat]
	energy := b.Actors[seat].Resources.EnergyPoints
	var out []tierValue
	for _, id := range rt.OffensiveAbilityIDs {
		def, ok := lib.Abilities[id]
		if !ok || def.Qualification == nil || def.Cost.Energy > energy {
			continue
		}
		bonus := cardDamageBonus(rt.AbilityModifiers, id) + extra[id]
		for _, tier := range def.Qualification.ActivationTiers {
			check := def
			q := *def.Qualification
			q.ActivationTiers = []content.AbilityTier{tier}
			check.Qualification = &q
			value := TierValue(tier.Operations, b.Actors[seat].Resources.MaxEnergyPoints) + bonus
			if value <= 0 {
				value = 0.5
			}
			out = append(out, tierValue{ability: id, tier: tier.ID, check: check, value: value})
		}
	}
	return out
}

// TierValue scores an ability tier's operations: damage counts in full, a
// status the actor gives itself half a point per stack, energy a tenth of a
// point per point gained.
func TierValue(ops []content.BattleOperation, _ int) float64 {
	value := 0.0
	for _, op := range ops {
		switch op.Type {
		case "deal_damage":
			value += float64(amount(op.Amount))
		case "apply_status":
			if op.Target == "self" {
				value += 0.5 * float64(max(op.StackCount, 1))
			}
		case "gain_resource":
			value += 0.1 * float64(amount(op.Amount))
		}
	}
	return value
}

// cardDamageBonus totals active card bonuses (such as Strong Swing) on one
// ability.
func cardDamageBonus(modifiers []state.RuntimeAbilityModifier, ability string) float64 {
	total := 0
	for _, m := range modifiers {
		if m.AbilityID != ability || len(m.ProgramBonus) == 0 {
			continue
		}
		var step content.CardStep
		if json.Unmarshal(m.ProgramBonus, &step) == nil {
			total += amount(step.Params["damage"])
		}
	}
	return float64(total)
}

func tableKey(dieID string, dice int, tiers []tierValue, tools []diceTool, energyValue, cardValue float64) string {
	var sb strings.Builder
	fmt.Fprintf(&sb, "%s/%d/%g/%g", dieID, dice, energyValue, cardValue)
	for _, t := range tiers {
		fmt.Fprintf(&sb, "|%s:%s=%g", t.ability, t.tier, t.value)
	}
	for _, t := range tools {
		fmt.Fprintf(&sb, "|tool %s %v %v %d-%d %d", t.card, t.reroll, t.deltas, t.min, t.max, t.cost)
	}
	return sb.String()
}
