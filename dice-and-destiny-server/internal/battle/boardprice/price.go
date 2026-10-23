package boardprice

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"math"
	"strings"
	"sync"

	"diceanddestiny/server/internal/content"
)

// Board is the abilities to price, one slot per ability.
type Board struct {
	Slots []Slot `json:"slots"`
}

// Slot is one ability: its cost (one tier per cost tier) and the rider every
// tier carries.
type Slot struct {
	ID      string                    `json:"id"`
	Job     string                    `json:"job,omitempty"`
	Cost    Cost                      `json:"-"`
	RiderID string                    `json:"rider,omitempty"`
	Rider   []content.BattleOperation `json:"-"`
}

// Options are the pricing method's numbers and the rider exchange rates.
type Options struct {
	Target        float64
	Alpha         float64
	Rolls         int
	Dice          int
	MinimumDamage int
	// LadderStep is how much more damage each harder tier of one ability
	// deals than the tier below it (0 in the reference script).
	LadderStep int
	// HalfBand: a damage this close to a half rounds whichever way lands the
	// board's average nearer its target.
	HalfBand float64
	// From and To are the staircase's targets; Scan is how far apart the
	// staircase is sampled before each change is pinned. Anchor is the
	// standard level (the menus' target): its step is the board rounded
	// closest to it, and no step above deals less than it (0: none).
	From, To, Scan, Anchor float64
	Rates                  Rates
}

// Options returns the menus' agreed pricing.
func (m Menus) Options() Options {
	p := m.Pricing
	return Options{Target: p.Target, Alpha: p.Alpha, Rolls: p.Rolls, Dice: p.Dice, MinimumDamage: p.MinimumDamage, LadderStep: p.LadderStep, HalfBand: p.HalfBand,
		From: p.Staircase.From, To: p.Staircase.To, Scan: p.Staircase.Scan, Anchor: p.Target, Rates: m.Rates}
}

// TierReport is one tier's price.
type TierReport struct {
	Slot string `json:"slot"`
	// Tier is the tier's cost in notation ("main3", "SS").
	Tier string `json:"tier"`
	// Chase is the chance to roll the tier when every keep chases it alone.
	Chase float64 `json:"chase"`
	// Relative is (1 / Chase) ^ alpha: a guaranteed hit is 1.
	Relative float64 `json:"relative"`
	// UseExact is the share of turns that end on this tier at exact prices.
	UseExact float64 `json:"use_exact"`
	// Value is Relative × multiplier: damage plus rider.
	Value float64 `json:"value"`
	// RiderValue is the rider at the exchange rates.
	RiderValue float64 `json:"rider_value"`
	// Damage is Value − RiderValue, never below the minimum damage, with
	// each ladder spread so every harder tier of one ability deals at least
	// the ladder step more than the tier below it. Damage is priced at
	// DamageMultiplier.
	Damage float64 `json:"damage"`
	// Floored reports a rider worth so much that the damage hit the minimum.
	Floored bool `json:"floored,omitempty"`
	// Stepped reports a tier the ladder step moved off its price.
	Stepped bool `json:"stepped,omitempty"`
	// WholeDamage is the whole damage the generated ability deals: the
	// staircase step's at the target.
	WholeDamage int `json:"whole_damage"`
	// UseWhole is the share of turns that end on this tier at whole damage.
	UseWhole float64 `json:"use_whole"`
}

// Report is a priced board.
type Report struct {
	Die   string       `json:"die"`
	Tiers []TierReport `json:"tiers"`
	// Sum is Σ(use × relative), the board's average at relative payoffs.
	Sum float64 `json:"sum"`
	// Multiplier is target ÷ Sum: what a guaranteed hit is worth here.
	Multiplier float64 `json:"multiplier"`
	// DamageMultiplier is what a guaranteed hit is worth once every ladder
	// is stepped: Multiplier, or the multiplier at which the stepped board
	// averages the target when the step moved a tier.
	DamageMultiplier float64 `json:"damage_multiplier"`
	// ExactAverage is the average per turn at exact prices (the target).
	ExactAverage float64 `json:"exact_average"`
	ExactNoHit   float64 `json:"exact_no_hit"`
	// WholeAverage is the average per turn with whole damage and riders.
	WholeAverage float64 `json:"whole_average"`
	WholeNoHit   float64 `json:"whole_no_hit"`
	// Drift is the share of turns that end on a different tier once
	// damage is whole (against the stepped board when a ladder was spread).
	Drift float64 `json:"drift"`
	// Step is the index of the staircase step in force at the target.
	Step int `json:"step"`
	// Policy is the keep after each roll of the whole-damage board.
	Policy KeepPolicy `json:"policy"`
}

// KeepPolicy is the keep after roll 1 (two rerolls left) and roll 2 (one
// left) for every hand.
type KeepPolicy struct {
	AfterRoll1 []KeepRow `json:"after_roll_1"`
	AfterRoll2 []KeepRow `json:"after_roll_2"`
}

// KeepRow is one hand's best keep: faces ascending; an empty keep rerolls
// everything and keeping every die stops.
type KeepRow struct {
	Hand     []int   `json:"hand"`
	Keep     []int   `json:"keep"`
	Expected float64 `json:"expected"`
}

var (
	cacheMu sync.Mutex
	cache   = map[string]Report{}
)

const cacheLimit = 512

// Price turns a die and a board into each tier's damage:
//
//  1. chase odds of each tier (5 dice, 3 rolls, every keep chasing it);
//  2. relative payoff (1 ÷ chase) ^ alpha, anchored on a guaranteed hit;
//  3. play the board exactly for each tier's use share, and scale every
//     payoff by target ÷ Σ(use × relative);
//  4. damage = value − rider (never below the minimum);
//  5. step every ladder: each harder tier of one ability deals at least the
//     ladder step more than the tier below it. A ladder too flat for that is
//     spread around its average (the closest stepped damages), and the
//     multiplier solved again so the stepped board still averages the
//     target;
//  6. take the whole damage from the board's staircase (PriceStaircase):
//     the step in force at the target. Each step rounds every tier to its
//     nearest whole damage, letting count ladders, tiers near a half and
//     tiers that would break the order go either way so the board lands
//     nearest its target, keeping the ladder steps and never paying a harder
//     tier less than an easier one; and of those rises it keeps only the
//     ones that last up to the top of the staircase, so no step ever lowers
//     a tier.
//
// Riders are counted in every play. Reports and staircases are cached by
// board key.
func Price(die Die, board Board, opts Options) (Report, error) {
	if err := opts.check(); err != nil {
		return Report{}, err
	}
	if opts.Target < opts.From-1e-9 || opts.Target > opts.To+1e-9 {
		return Report{}, fmt.Errorf("target %g is outside the staircase %g–%g", opts.Target, opts.From, opts.To)
	}
	key := boardKey(die, board, opts)
	cacheMu.Lock()
	if cached, ok := cache[key]; ok {
		cacheMu.Unlock()
		return cloneReport(cached), nil
	}
	cacheMu.Unlock()
	p, sc, err := staircaseFor(die, board, opts)
	if err != nil {
		return Report{}, err
	}
	k := sc.At(opts.Target)
	step := sc.Steps[k]
	reports := append([]TierReport(nil), p.reports...)
	multiplier := opts.Target / p.sum
	damageMultiplier, reference, referenceNoHit := p.s.stepped(p.relative, p.riders, p.ladders, multiplier, p.used, p.noHit, opts)
	damage, floored, stepped := pricedDamage(p.relative, p.riders, p.ladders, damageMultiplier, opts)
	played := make([]float64, len(reports))
	for i := range reports {
		reports[i].Value = p.relative[i] * multiplier
		reports[i].Damage, reports[i].Floored, reports[i].Stepped = damage[i], floored[i], stepped[i]
		reports[i].WholeDamage, reports[i].UseWhole = step.Damage[i], step.Use[i]
		played[i] = float64(step.Damage[i]) + p.riders[i]
	}
	_, _, _, policy := p.s.play(played, opts.Rolls, true)
	report := Report{Die: die.ID, Tiers: reports, Sum: p.sum, Multiplier: multiplier, DamageMultiplier: damageMultiplier, ExactAverage: opts.Target, ExactNoHit: p.noHit,
		WholeAverage: step.Average, WholeNoHit: step.NoHit, Drift: drift(reference, referenceNoHit, step.Use, step.NoHit), Step: k,
		Policy: p.s.keepPolicy(policy, played, opts.Rolls)}
	cacheMu.Lock()
	if len(cache) >= cacheLimit {
		cache = map[string]Report{}
	}
	cache[key] = report
	cacheMu.Unlock()
	return cloneReport(report), nil
}

func (o Options) check() error {
	if o.Target < 0 || o.Alpha < 0 || o.Rolls < 1 || o.Rolls > 5 || o.Dice < 1 || o.Dice > 5 || o.MinimumDamage < 1 || o.LadderStep < 0 ||
		o.HalfBand < 0 || o.HalfBand >= 0.5 || o.From <= 0 || o.To <= o.From || o.Scan <= 0 || o.Scan > o.To-o.From ||
		(o.Anchor != 0 && (o.Anchor < o.From || o.Anchor > o.To)) {
		return fmt.Errorf("invalid pricing options %+v", o)
	}
	return nil
}

// pricedDamage is each tier's damage at multiplier m: value − rider, never
// below the minimum, with every ladder stepped.
func pricedDamage(relative, riders []float64, ladders [][]int, m float64, opts Options) ([]float64, []bool, []bool) {
	damage := make([]float64, len(relative))
	floored := make([]bool, len(relative))
	stepped := make([]bool, len(relative))
	for i := range relative {
		damage[i] = relative[i]*m - riders[i]
		if damage[i] < float64(opts.MinimumDamage) {
			damage[i], floored[i] = float64(opts.MinimumDamage), true
		}
	}
	for _, ladder := range ladders {
		rung := make([]float64, len(ladder))
		for k, i := range ladder {
			rung[k] = damage[i]
		}
		for k, v := range stepLadder(rung, float64(opts.LadderStep), float64(opts.MinimumDamage)) {
			if i := ladder[k]; math.Abs(v-damage[i]) > 1e-12 {
				damage[i], stepped[i] = v, true
			}
		}
	}
	return damage, floored, stepped
}

// stepLadder is the closest damages (least squares), easiest tier first,
// where each tier deals at least step more than the one below and none less
// than the minimum: damage − k·step made non-decreasing by pooling the tiers
// that fall into their average, then held at the minimum. A ladder already
// stepped is unchanged; a flat one spreads around its average.
func stepLadder(damage []float64, step, minimum float64) []float64 {
	type pool struct {
		sum float64
		n   int
	}
	var pools []pool
	for k, v := range damage {
		pools = append(pools, pool{v - float64(k)*step, 1})
		for len(pools) > 1 {
			a, b := pools[len(pools)-2], pools[len(pools)-1]
			if a.sum/float64(a.n) <= b.sum/float64(b.n) {
				break
			}
			pools = append(pools[:len(pools)-2], pool{a.sum + b.sum, a.n + b.n})
		}
	}
	out := make([]float64, 0, len(damage))
	for _, p := range pools {
		for j := 0; j < p.n; j++ {
			out = append(out, math.Max(p.sum/float64(p.n), minimum)+float64(len(out))*step)
		}
	}
	return out
}

// stepped is the multiplier at which the stepped board averages the target,
// with that board's use shares. Stepping moves the average, so when it moves
// any tier the multiplier is found again by bisection (damage, and so the
// average, only grows with the multiplier); otherwise it is the multiplier
// and the exact play.
func (s *solver) stepped(relative, riders []float64, ladders [][]int, multiplier float64, used []float64, noHit float64, opts Options) (float64, []float64, float64) {
	_, _, moved := pricedDamage(relative, riders, ladders, multiplier, opts)
	step := false
	for _, m := range moved {
		step = step || m
	}
	if !step {
		return multiplier, used, noHit
	}
	play := func(m float64) (float64, []float64, float64) {
		damage, _, _ := pricedDamage(relative, riders, ladders, m, opts)
		for i := range damage {
			damage[i] += riders[i]
		}
		ev, u, nh, _ := s.play(damage, opts.Rolls, false)
		return ev, u, nh
	}
	low, high := 0.0, multiplier
	for k := 0; k < 30; k++ {
		if ev, _, _ := play(high); ev >= opts.Target {
			break
		}
		low, high = high, 2*high
	}
	for high-low > 1e-7*multiplier {
		mid := (low + high) / 2
		if ev, _, _ := play(mid); ev > opts.Target {
			high = mid
		} else {
			low = mid
		}
	}
	_, u, nh := play(high)
	return high, u, nh
}

// drift is the share of turns that end on a different tier (total variation
// distance, misses included).
func drift(a []float64, aNone float64, b []float64, bNone float64) float64 {
	total := math.Abs(aNone - bNone)
	for i := range a {
		total += math.Abs(a[i] - b[i])
	}
	return total / 2
}

func (s *solver) keepPolicy(policy []map[int]int, played []float64, rolls int) KeepPolicy {
	rows := func(k int) []KeepRow {
		if k >= len(policy) || policy[k] == nil {
			return nil
		}
		// Expected values of each keep with k rerolls left, recomputed from
		// the same play for the report.
		expected := s.keepValues(played, rolls, k)
		var out []KeepRow
		for _, hand := range s.hands.full {
			kept := policy[k][hand]
			out = append(out, KeepRow{Hand: s.hands.faces(hand), Keep: s.hands.faces(kept), Expected: expected[kept]})
		}
		return out
	}
	return KeepPolicy{AfterRoll1: rows(rolls - 1), AfterRoll2: rows(rolls - 2)}
}

// keepValues is the expected final value of every kept set with k rerolls
// left under optimal play.
func (s *solver) keepValues(values []float64, rolls, k int) []float64 {
	h := s.hands
	value := make([]float64, len(h.sets))
	for _, hand := range h.full {
		for i, ok := range s.qualify[hand] {
			if ok && values[i] > value[hand] {
				value[hand] = values[i]
			}
		}
	}
	expected := make([]float64, len(h.sets))
	for step := 1; step <= k; step++ {
		for i := range h.sets {
			if h.size[i] == h.dice {
				expected[i] = value[i]
				continue
			}
			sum := 0.0
			for o, next := range h.join[i] {
				sum += h.outs[h.dice-h.size[i]][o].prob * value[next]
			}
			expected[i] = sum
		}
		if step == k {
			break
		}
		next := make([]float64, len(h.sets))
		for _, hand := range h.full {
			best := math.Inf(-1)
			for _, kept := range h.keeps[hand] {
				best = math.Max(best, expected[kept])
			}
			next[hand] = best
		}
		value = next
	}
	return expected
}

func boardKey(die Die, board Board, opts Options) string {
	var sb strings.Builder
	sb.WriteString(die.ID)
	for _, f := range die.Faces {
		fmt.Fprintf(&sb, "|%d:%s", f.Number, f.Symbol)
	}
	for _, slot := range board.Slots {
		ops, _ := json.Marshal(slot.Rider)
		fmt.Fprintf(&sb, "#%s=%s:%s", slot.ID, slot.Cost, ops)
	}
	rates, _ := json.Marshal(opts.Rates)
	fmt.Fprintf(&sb, "@%g/%g/%d/%d/%d/%d/%g/%g/%g/%g/%g/%s", opts.Target, opts.Alpha, opts.Rolls, opts.Dice, opts.MinimumDamage, opts.LadderStep, opts.HalfBand, opts.From, opts.To, opts.Scan, opts.Anchor, rates)
	sum := sha256.Sum256([]byte(sb.String()))
	return fmt.Sprintf("%x", sum)
}

func cloneReport(r Report) Report {
	r.Tiers = append([]TierReport(nil), r.Tiers...)
	return r
}

// Evaluate plays a board with given tier values (damage plus rider, one per
// tier in board order), for comparing a printed board with its price: the
// average per turn, the no-hit share and each tier's use.
func Evaluate(die Die, board Board, values []float64, opts Options) (float64, float64, []float64, error) {
	if err := opts.check(); err != nil {
		return 0, 0, nil, err
	}
	var tiers []Tier
	for _, slot := range board.Slots {
		tiers = append(tiers, slot.Cost...)
	}
	if len(values) != len(tiers) {
		return 0, 0, nil, fmt.Errorf("board has %d tiers; got %d values", len(tiers), len(values))
	}
	ev, used, noHit, _ := newSolver(die, tiers, opts.Dice).play(values, opts.Rolls, false)
	return ev, noHit, used, nil
}
