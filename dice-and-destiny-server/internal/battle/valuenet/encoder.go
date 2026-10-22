package valuenet

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"diceanddestiny/server/internal/battle/lookahead"
	"diceanddestiny/server/internal/battle/mlsim"
	"diceanddestiny/server/internal/battle/state"
	"diceanddestiny/server/internal/content"
)

// SchemaName names the encoder's inputs.
const SchemaName = "value-v1"

// The encoder values boards at these fixed rates, so its inputs never move
// when the players' rates are tuned.
var encoderRates = lookahead.Rates{Status: 0.5, EnergyGain: 0.1, EnergySpend: 0.1}

const encoderCardValue = 0.25

var (
	summaryNames = []string{"damage", "prevent", "status", "resource", "roll", "modify", "total"}
	cardEffects  = []string{"damage_bonus", "prevent", "energy", "draw", "dice", "extra_roll", "status_self", "status_enemy", "remove_status", "other"}
	// cardEffectScale divides each pooled card effect.
	cardEffectScale = []float64{5, 5, 10, 3, 3, 2, 3, 3, 2, 3}
	costBuckets     = []string{"cost_0", "cost_1_5", "cost_6_10", "cost_11_up"}
	timings         = []string{"offense_before_roll", "offense_after_roll", "defense", "damage_reaction"}
	polarities      = []string{"positive", "negative"}
)

// statusPool totals one side's statuses of one polarity.
type statusPool struct {
	count, stacks, fill, persistent, consumable, expiring float64
	effect                                                [7]float64
	defenseDice, attackDamage                             float64
}

// side is one actor's public state: health in each pile, resources, the
// value of its board, its defenses and its statuses.
type side struct {
	hand, deck, discard, removed, health, maxHealth float64
	handCount, deckCount, energy, handLimit, dice   float64
	fresh, miss                                     float64
	offense, affordable, bestTier, bonus            float64
	offenseEffect                                   [7]float64
	defenses, prevent, meanPrevent                  float64
	defenseEnergy, defenseDamage, defenseStatus     float64
	status                                          [2]statusPool
	tokens, cursed                                  float64
}

// handPool pools the viewer's own hand by cost, timing and effect.
type handPool struct {
	fresh, miss, cards, playable float64
	cost                         [4]float64
	timing                       [4]float64
	effect                       [10]float64
}

type features struct {
	round, actsFirst float64
	own, foe         side
	hand             handPool
}

// emit lists the features in their published order.
func (f *features) emit(put func(name string, v float64)) {
	put("round", f.round)
	put("acts_first", f.actsFirst)
	f.own.emit("own.", put)
	f.foe.emit("foe.", put)
	h := f.hand
	put("hand.fresh_with_cards", h.fresh)
	put("hand.miss_with_cards", h.miss)
	put("hand.cards", h.cards)
	put("hand.playable", h.playable)
	for i, name := range costBuckets {
		put("hand."+name, h.cost[i])
	}
	for i, name := range timings {
		put("hand.timing."+name, h.timing[i])
	}
	for i, name := range cardEffects {
		put("hand.effect."+name, h.effect[i])
	}
}

func (s *side) emit(prefix string, put func(name string, v float64)) {
	for _, field := range []struct {
		name string
		v    float64
	}{
		{"hand", s.hand}, {"deck", s.deck}, {"discard", s.discard}, {"removed", s.removed},
		{"health", s.health}, {"max_health", s.maxHealth}, {"hand_count", s.handCount}, {"deck_count", s.deckCount},
		{"energy", s.energy}, {"hand_limit", s.handLimit}, {"dice", s.dice},
		{"fresh", s.fresh}, {"miss", s.miss},
		{"offense", s.offense}, {"affordable", s.affordable}, {"best_tier", s.bestTier}, {"card_bonus", s.bonus},
		{"defenses", s.defenses}, {"defense_prevent", s.prevent}, {"defense_prevent_mean", s.meanPrevent},
		{"defense_energy", s.defenseEnergy}, {"defense_damage", s.defenseDamage}, {"defense_status", s.defenseStatus},
		{"tokens", s.tokens}, {"cursed_faces", s.cursed},
	} {
		put(prefix+field.name, field.v)
	}
	for i, name := range summaryNames {
		put(prefix+"offense_effect."+name, s.offenseEffect[i])
	}
	for k, polarity := range polarities {
		p := s.status[k]
		at := prefix + "status." + polarity + "."
		put(at+"count", p.count)
		put(at+"stacks", p.stacks)
		put(at+"fill", p.fill)
		put(at+"persistent", p.persistent)
		put(at+"consumable", p.consumable)
		put(at+"expiring", p.expiring)
		for i, name := range summaryNames {
			put(at+"effect."+name, p.effect[i])
		}
		put(at+"defense_dice", p.defenseDice)
		put(at+"attack_damage", p.attackDamage)
	}
}

var schemaFeatures, schemaHash = func() ([]string, string) {
	var names []string
	(&features{}).emit(func(name string, _ float64) { names = append(names, name) })
	h := sha256.New()
	fmt.Fprintf(h, "%s\nrates %s card-value %g\n%s\n", SchemaName, encoderRates, encoderCardValue, strings.Join(names, "\n"))
	return names, hex.EncodeToString(h.Sum(nil))
}()

// Features lists the encoder's inputs by name, in order.
func Features() []string { return append([]string(nil), schemaFeatures...) }

// Width is the number of inputs.
func Width() int { return len(schemaFeatures) }

// SchemaHash pins the inputs: their names, order and the encoder's rates.
func SchemaHash() string { return schemaHash }

// Encoder turns positions into value-v1 inputs. It caches dice solver tables
// and is not safe for concurrent use.
type Encoder struct {
	solver *lookahead.DiceSolver
}

func NewEncoder() *Encoder {
	solver := lookahead.NewDiceSolver()
	solver.Rates, solver.CardValue = encoderRates, encoderCardValue
	return &Encoder{solver: solver}
}

// Encode writes the position's Width() inputs into out.
func (e *Encoder) Encode(pos Position, out []float32) error {
	if len(out) != Width() {
		return fmt.Errorf("value-v1 needs %d inputs, got room for %d", Width(), len(out))
	}
	if _, ok := pos.View.Actors[pos.Seat]; !ok {
		return fmt.Errorf("seat %q is not in the battle", pos.Seat)
	}
	f := e.features(pos)
	i := 0
	f.emit(func(_ string, v float64) {
		out[i] = float32(v)
		i++
	})
	return nil
}

func (e *Encoder) features(pos Position) features {
	var f features
	f.round = float64(min(pos.View.Round, 12)) / 12
	if pos.Acting == pos.Seat {
		f.actsFirst = 1
	}
	f.own = e.side(pos, pos.Seat)
	if foe := pos.opponent(); foe != "" {
		f.foe = e.side(pos, foe)
	}
	f.hand = e.hand(pos)
	return f
}

// turn is what the solver needs to value the actor's next offensive turn,
// all of it public; cards are the viewer's own hand, or nil.
func (pos Position) turn(id string, cards map[string]int) lookahead.Turn {
	a := pos.View.Actors[id]
	setup := pos.Dice[id]
	rolls := 3
	if a.Dice != nil && a.Dice.Pool == "offensive" && a.Dice.MaxRolls > 0 {
		rolls = a.Dice.MaxRolls
	}
	return lookahead.Turn{Die: setup.Die, Dice: setup.Count, Rolls: rolls, Energy: a.EnergyPoints,
		Abilities: a.OffensiveAbilities, Modifiers: a.AbilityModifiers, Cards: cards}
}

func (e *Encoder) side(pos Position, id string) side {
	lib := pos.Library
	a := pos.View.Actors[id]
	var s side
	maxH := float64(max(allCards(a), 1))
	s.hand, s.deck = float64(a.HandCount)/maxH, float64(a.DeckCount)/maxH
	s.discard, s.removed = float64(a.DiscardCount)/maxH, float64(a.RemovedCount)/maxH
	s.health, s.maxHealth = float64(liveCards(a))/maxH, float64(allCards(a))/30
	s.handCount, s.deckCount = float64(a.HandCount)/6, float64(a.DeckCount)/20
	s.energy, s.handLimit, s.dice = float64(a.EnergyPoints)/20, float64(a.MaxHandSize)/6, float64(a.DiceCount)/5
	fresh, miss := e.solver.FreshTurn(lib, pos.turn(id, nil), nil)
	s.fresh, s.miss = fresh/10, miss

	for _, ability := range a.OffensiveAbilities {
		def, ok := lib.Abilities[ability]
		if !ok || def.Qualification == nil {
			continue
		}
		s.offense++
		if def.Cost.Energy > a.EnergyPoints {
			continue
		}
		s.affordable++
		bonus := damageBonus(a.AbilityModifiers, ability)
		for _, tier := range def.Qualification.ActivationTiers {
			s.bestTier = max(s.bestTier, encoderRates.TierValue(tier.Operations)+bonus)
			for i, v := range mlsim.V2OperationSummary(tier.Operations) {
				s.offenseEffect[i] = max(s.offenseEffect[i], float64(v))
			}
		}
	}
	s.offense, s.affordable, s.bestTier = s.offense/6, s.affordable/6, s.bestTier/10
	for _, m := range a.AbilityModifiers {
		s.bonus += damageBonus([]state.RuntimeAbilityModifier{m}, m.AbilityID)
	}
	s.bonus /= 5

	extraDice := 0
	for _, st := range a.Statuses {
		if def := lib.Statuses[st.DefinitionID]; def.Modifiers != nil {
			extraDice += def.Modifiers.DefenseDiceBonus * st.Stacks
		}
	}
	bestPrevent := -1.0
	for _, ability := range a.DefensiveAbilities {
		def, ok := lib.Abilities[ability]
		if !ok || def.Resolution == nil {
			continue
		}
		effect := expectedEffect(def.Resolution.Operations, extraDice)
		prevent := effect[1] * 20
		s.defenses++
		s.meanPrevent += prevent
		if prevent > bestPrevent {
			bestPrevent = prevent
			s.prevent, s.defenseEnergy, s.defenseDamage, s.defenseStatus = prevent/5, effect[3]*10/5, effect[0]*20/5, effect[2]*10/3
		}
	}
	if s.defenses > 0 {
		s.meanPrevent /= s.defenses * 5
	}
	s.defenses /= 3

	var capacity [2]float64
	for _, st := range a.Statuses {
		def, ok := lib.Statuses[st.DefinitionID]
		if !ok || st.Stacks <= 0 {
			continue
		}
		k := 1
		if def.Polarity == "positive" {
			k = 0
		}
		p := &s.status[k]
		stacks := float64(st.Stacks)
		p.count++
		p.stacks += stacks
		limit := float64(def.Stacking.StackLimit)
		if def.Stacking.Uncapped || limit <= 0 {
			limit = max(stacks, 1)
		}
		capacity[k] += limit
		if def.Lifecycle.Persistent {
			p.persistent += stacks
		}
		if def.Lifecycle.ConsumeOnPlay || def.Lifecycle.ConsumeOnTriggerCheckpoint {
			p.consumable += stacks
		}
		if expires(def) {
			p.expiring += stacks
		}
		var ops []content.BattleOperation
		for _, trigger := range def.Triggers {
			ops = append(ops, trigger.Operations...)
		}
		ops = append(ops, def.Operations...)
		for i, v := range mlsim.V2OperationSummary(ops) {
			p.effect[i] += float64(v) * stacks
		}
		if def.Modifiers != nil {
			p.defenseDice += float64(def.Modifiers.DefenseDiceBonus) * stacks
			p.attackDamage += float64(def.Modifiers.AttackDamageBonus) * stacks
		}
	}
	for k := range s.status {
		p := &s.status[k]
		if capacity[k] > 0 {
			p.fill = p.stacks / capacity[k]
		}
		p.count /= 3
		p.stacks /= 5
		p.persistent /= 5
		p.consumable /= 5
		p.expiring /= 5
		p.defenseDice /= 3
		p.attackDamage /= 3
	}
	for _, token := range a.Tokens {
		s.tokens += float64(token.Value)
	}
	s.tokens /= 10
	for _, die := range a.OwnedDice {
		s.cursed += float64(len(die.CursedFaces))
	}
	s.cursed /= 10
	return s
}

// expires reports a status that leaves on its own: not persistent, gone at
// zero duration, or removing itself in one of its triggers.
func expires(def content.BattleStatusDefinition) bool {
	if !def.Lifecycle.Persistent || def.Lifecycle.RemoveOnDurationZero || def.Lifecycle.RemoveAfterResolution {
		return true
	}
	for _, trigger := range def.Triggers {
		for _, op := range trigger.Operations {
			if op.Type == "remove_status" && (op.StatusID == def.ID || op.StatusID == "") && op.Target == "self" {
				return true
			}
		}
	}
	return false
}

// expectedEffect is the v2 effect summary of a defense's operations with
// every roll counted per die: each die of a roll resolves its own outcome.
// extraDice are added to every roll (statuses such as Fortified).
func expectedEffect(ops []content.BattleOperation, extraDice int) [7]float64 {
	var out [7]float64
	for _, op := range ops {
		summary := mlsim.V2OperationSummary([]content.BattleOperation{op})
		scale := 1.0
		if op.Type == "roll_dice" && len(op.Outcomes) > 0 {
			// The summary counts one die's outcome; every die resolves one.
			scale = float64(max(op.DiceCount, 1) + extraDice)
		}
		for i, v := range summary {
			out[i] += float64(v) * scale
		}
	}
	return out
}

// damageBonus totals active card bonuses (such as Strong Swing) on one
// ability, as the dice solver counts them.
func damageBonus(modifiers []state.RuntimeAbilityModifier, ability string) float64 {
	total := 0.0
	for _, m := range modifiers {
		if m.AbilityID != ability || len(m.ProgramBonus) == 0 {
			continue
		}
		var step content.CardStep
		if json.Unmarshal(m.ProgramBonus, &step) == nil {
			total += number(step.Params["damage"])
		}
	}
	return total
}

func number(v any) float64 {
	switch n := v.(type) {
	case int:
		return float64(n)
	case int64:
		return float64(n)
	case float64:
		return n
	case json.Number:
		f, _ := n.Float64()
		return f
	}
	return 0
}

// hand pools the viewer's own hand.
func (e *Encoder) hand(pos Position) handPool {
	lib := pos.Library
	own := pos.View.Actors[pos.Seat]
	var h handPool
	cards := lookahead.HandCards(lib, own.Hand, own.CardInstances, own.CardUses, pos.View.Round)
	fresh, miss := e.solver.FreshTurn(lib, pos.turn(pos.Seat, cards), nil)
	h.fresh, h.miss = fresh/10, miss
	for _, instance := range own.Hand {
		def, ok := lib.Cards[own.CardInstances[instance].DefinitionID]
		if !ok {
			continue
		}
		h.cards++
		if def.Cost.Energy <= own.EnergyPoints {
			h.playable++
		}
		switch cost := def.Cost.Energy; {
		case cost <= 0:
			h.cost[0]++
		case cost <= 5:
			h.cost[1]++
		case cost <= 10:
			h.cost[2]++
		default:
			h.cost[3]++
		}
		for i, on := range cardTiming(def) {
			if on {
				h.timing[i]++
			}
		}
		for i, v := range cardEffect(def) {
			h.effect[i] += v
		}
	}
	h.cards /= 6
	h.playable /= 6
	for i := range h.cost {
		h.cost[i] /= 6
		h.timing[i] /= 6
	}
	for i := range h.effect {
		h.effect[i] /= cardEffectScale[i]
	}
	return h
}

// cardTiming reports when a card can be played: before the offensive roll,
// after it, on defense, or in reaction to damage.
func cardTiming(def content.BattleCardDefinition) [4]bool {
	var out [4]bool
	windows := func(list []string, requirement string) {
		for _, w := range list {
			switch {
			case w == "offensive_before_roll":
				out[0] = true
			case w == "offensive_planning":
				out[0] = out[0] || requirement != "after_first"
				out[1] = out[1] || requirement != "before_first"
			case strings.HasPrefix(w, "offensive"):
				out[1] = true
			case strings.HasPrefix(w, "defense"):
				out[2] = true
			case strings.HasPrefix(w, "damage"):
				out[3] = true
			}
		}
	}
	switch {
	case def.Program != nil:
		windows(def.Program.Windows, def.Program.RollRequirement)
	case def.Mechanic != nil:
		windows(def.Mechanic.Windows, "any")
	default:
		for _, t := range def.Play.PlayableDuring {
			switch {
			case t.Segment == "offensive" && def.Play.BeforeFirstRoll:
				out[0] = true
			case t.Segment == "offensive":
				out[1] = true
			case t.Segment == "defensive":
				out[2] = true
			case strings.Contains(t.Segment, "damage") || strings.Contains(t.Stage, "damage"):
				out[3] = true
			}
		}
	}
	return out
}

// cardEffect summarizes what a card does, by effect kind (cardEffects).
// Programs are read step by step (a choice card counts its options' mean),
// configured mechanics by their numeric parameters, legacy cards by their
// operations.
func cardEffect(def content.BattleCardDefinition) [10]float64 {
	var out [10]float64
	switch {
	case def.Program != nil:
		programEffect(def.Program.Steps, 1, &out)
	case def.Mechanic != nil:
		out[9]++
		for _, key := range sortedKeys(def.Mechanic.Params) {
			v := number(def.Mechanic.Params[key])
			switch key {
			case "damage":
				out[0] += v
			case "energy":
				out[2] += v
			case "draw", "cards":
				out[3] += v
			case "stacks", "curses", "amount":
				out[7] += v
			}
		}
	default:
		for _, op := range def.Operations {
			v := number(op.Amount)
			switch op.Type {
			case "deal_damage":
				out[0] += v
			case "prevent_damage":
				out[1] += v
			case "gain_resource":
				out[2] += v
			case "modify_die", "roll_dice", "reroll_dice":
				out[4]++
			case "apply_status":
				if op.Target == "self" {
					out[6] += float64(max(op.StackCount, 1))
				} else {
					out[7] += float64(max(op.StackCount, 1))
				}
			case "remove_status":
				out[8]++
			default:
				out[9]++
			}
		}
	}
	return out
}

func programEffect(steps []content.CardStep, weight float64, out *[10]float64) {
	for _, step := range steps {
		switch step.Effect {
		case "ability_bonus":
			out[0] += weight * float64(content.ProgramInt(step, "damage"))
		case "prevent":
			out[1] += weight * float64(content.ProgramInt(step, "amount"))
		case "save_cards":
			out[1] += weight
		case "energy":
			out[2] += weight * float64(content.ProgramInt(step, "amount"))
		case "draw":
			out[3] += weight * float64(content.ProgramInt(step, "amount"))
		case "set_die", "adjust_die", "flip_die", "copy_die", "reroll":
			out[4] += weight
		case "extra_roll":
			out[5] += weight * float64(content.ProgramInt(step, "amount"))
		case "apply_status", "conditional_status", "status_threshold":
			stacks := float64(content.ProgramInt(step, "stacks"))
			if step.Target.Owner == "self" {
				out[6] += weight * stacks
			} else {
				out[7] += weight * stacks
			}
		case "curse", "roll_cursed":
			out[7] += weight * float64(content.ProgramInt(step, "amount"))
		case "remove_status":
			out[8] += weight
		case "choice":
			if n := len(step.Choices); n > 0 {
				for _, option := range step.Choices {
					programEffect(option.Steps, weight/float64(n), out)
				}
			}
		default:
			out[9] += weight
		}
	}
}

func sortedKeys(m map[string]any) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}
