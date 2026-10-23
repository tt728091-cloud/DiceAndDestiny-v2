package boardprice

import (
	"fmt"
	"math"
	"sort"
	"strconv"
	"strings"
	"sync"
)

// Staircase is a board's whole damage at every target from From to To: a
// list of steps, each in force from its target up to the next step's. No
// tier ever deals less on a later step, so moving up the staircase only
// ever adds damage.
type Staircase struct {
	From  float64 `json:"from"`
	To    float64 `json:"to"`
	Steps []Step  `json:"steps"`
}

// Step is one step of a staircase.
type Step struct {
	// At is the target from which the step is in force.
	At float64 `json:"at"`
	// Damage is each tier's whole damage, in board order.
	Damage []int `json:"damage"`
	// Average and NoHit are the step's average damage per turn and no-hit
	// share; Use is each tier's share of turns.
	Average float64   `json:"average"`
	NoHit   float64   `json:"no_hit"`
	Use     []float64 `json:"use"`
	// Raised is the tiers (board order indexes) this step deals more with
	// than the step before it.
	Raised []int `json:"raised"`
}

// At is the index of the step in force at a target (the first step below
// From, the last above To).
func (s Staircase) At(target float64) int {
	k := sort.Search(len(s.Steps), func(i int) bool { return s.Steps[i].At > target+1e-9 }) - 1
	return max(k, 0)
}

// maxOpen caps the tiers one rounding may move either way.
const maxOpen = 14

// gridScans is how many scan steps apart the exact multiplier is solved;
// in between it is interpolated (the board's exact average grows smoothly
// with the multiplier).
const gridScans = 10

// bisections pins each change of the board between two scanned targets.
const bisections = 16

// prepared is a board's target-free pricing: tiers, chase odds, relative
// payoffs and the exact relative play.
type prepared struct {
	opts     Options
	reports  []TierReport // slot, tier, rider value, chase, relative, exact use
	riders   []float64
	ladders  [][]int // every slot's tiers, easiest first
	count    []bool  // on a count ladder: one ability's tiers of more of the same symbols
	relative []float64
	sum      float64
	used     []float64
	noHit    float64
	s        *solver
}

func prepare(die Die, board Board, opts Options) (*prepared, error) {
	if len(die.Faces) != sides {
		return nil, fmt.Errorf("die %q has no faces", die.ID)
	}
	p := &prepared{opts: opts}
	var tiers []Tier
	seen := map[string]bool{}
	for _, slot := range board.Slots {
		if len(slot.Cost) == 0 {
			return nil, fmt.Errorf("slot %s has no cost", slot.ID)
		}
		if err := slot.Cost.Reachable(die.Layout, opts.Dice); err != nil {
			return nil, fmt.Errorf("slot %s: %w", slot.ID, err)
		}
		rider, err := opts.Rates.Value(slot.Rider)
		if err != nil {
			return nil, fmt.Errorf("slot %s: %w", slot.ID, err)
		}
		count := len(slot.Cost) > 1
		for _, tier := range slot.Cost {
			count = count && tier.Straight == ""
		}
		var ladder []int
		for _, tier := range slot.Cost {
			ladder = append(ladder, len(tiers))
			if seen[tier.String()] {
				return nil, fmt.Errorf("board repeats tier %s", tier)
			}
			seen[tier.String()] = true
			tiers = append(tiers, tier)
			p.riders = append(p.riders, rider)
			p.count = append(p.count, count)
			p.reports = append(p.reports, TierReport{Slot: slot.ID, Tier: tier.String(), RiderValue: rider})
		}
		p.ladders = append(p.ladders, ladder)
	}
	if len(tiers) == 0 {
		return nil, fmt.Errorf("board has no abilities")
	}
	p.s = newSolver(die, tiers, opts.Dice)
	p.relative = make([]float64, len(tiers))
	for i := range tiers {
		p.reports[i].Chase = p.s.chase(i, len(tiers), opts.Rolls)
		if p.reports[i].Chase <= 0 {
			return nil, fmt.Errorf("%s can never be rolled", tiers[i])
		}
		p.relative[i] = math.Pow(1/p.reports[i].Chase, opts.Alpha)
		p.reports[i].Relative = p.relative[i]
	}
	for _, ladder := range p.ladders {
		sort.SliceStable(ladder, func(a, b int) bool { return p.reports[ladder[a]].Chase > p.reports[ladder[b]].Chase })
	}
	p.sum, p.used, p.noHit, _ = p.s.play(p.relative, opts.Rolls, false)
	for i := range p.reports {
		p.reports[i].UseExact = p.used[i]
	}
	return p, nil
}

var (
	stairsMu sync.Mutex
	stairs   = map[string]*stairEntry{}
)

const stairLimit = 256

type stairEntry struct {
	once sync.Once
	p    *prepared
	sc   Staircase
	err  error
}

// PriceStaircase is a board's staircase over the pricing's From–To range.
// Staircases are cached by board key.
func PriceStaircase(die Die, board Board, opts Options) (Staircase, error) {
	if err := opts.check(); err != nil {
		return Staircase{}, err
	}
	_, sc, err := staircaseFor(die, board, opts)
	return sc, err
}

func staircaseFor(die Die, board Board, opts Options) (*prepared, Staircase, error) {
	free := opts
	free.Target = 0
	key := boardKey(die, board, free)
	stairsMu.Lock()
	e := stairs[key]
	if e == nil {
		if len(stairs) >= stairLimit {
			stairs = map[string]*stairEntry{}
		}
		e = &stairEntry{}
		stairs[key] = e
	}
	stairsMu.Unlock()
	e.once.Do(func() {
		if e.p, e.err = prepare(die, board, opts); e.err == nil {
			e.sc, e.err = e.p.build()
		}
	})
	return e.p, cloneStaircase(e.sc), e.err
}

func cloneStaircase(sc Staircase) Staircase {
	sc.Steps = append([]Step(nil), sc.Steps...)
	return sc
}

// played is a whole-damage board's play: its average, use and no-hit share.
type played struct {
	ev, noHit float64
	used      []float64
}

// rounder picks whole damage for the staircase, remembering every board it
// has played and every set of roundings it has weighed.
type rounder struct {
	p    *prepared
	memo map[string]played
	sets map[string][]candidate
}

type candidate struct {
	v   []int
	key string
}

func intsKey(v []int) string {
	var sb strings.Builder
	for i, x := range v {
		if i > 0 {
			sb.WriteByte(',')
		}
		sb.WriteString(strconv.Itoa(x))
	}
	return sb.String()
}

func (r *rounder) play(v []int, key string) played {
	if got, ok := r.memo[key]; ok {
		return got
	}
	values := make([]float64, len(v))
	for i := range v {
		values[i] = float64(v[i]) + r.p.riders[i]
	}
	ev, used, noHit, _ := r.p.s.play(values, r.p.opts.Rolls, false)
	got := played{ev: ev, noHit: noHit, used: used}
	r.memo[key] = got
	return got
}

// easier reports tier i as easier to roll than tier j on another ability.
func (p *prepared) easier(i, j int) bool {
	return p.reports[i].Slot != p.reports[j].Slot && p.reports[i].Chase > p.reports[j].Chase+1e-9
}

// keepsOrder reports whether no easier tier pays more (damage plus rider)
// than a harder one on another ability. Paying the same is allowed.
func (p *prepared) keepsOrder(v []int) bool {
	for i := range v {
		for j := range v {
			if p.easier(i, j) && float64(v[i])+p.riders[i] > float64(v[j])+p.riders[j] {
				return false
			}
		}
	}
	return true
}

// stepLadders lifts every harder tier of one ability to at least the ladder
// step above the tier below it.
func (p *prepared) stepLadders(v []int) {
	for _, ladder := range p.ladders {
		for k := 1; k < len(ladder); k++ {
			v[ladder[k]] = max(v[ladder[k]], v[ladder[k-1]]+p.opts.LadderStep)
		}
	}
}

// raiseHarder lifts harder tiers until none pays less than an easier one.
func (p *prepared) raiseHarder(v []int) {
	order := make([]int, len(v))
	for i := range order {
		order[i] = i
	}
	sort.SliceStable(order, func(a, b int) bool { return p.reports[order[a]].Chase > p.reports[order[b]].Chase })
	for k, j := range order {
		for _, i := range order[:k] {
			if p.easier(i, j) {
				if need := float64(v[i]) + p.riders[i] - p.riders[j]; float64(v[j]) < need {
					v[j] = int(math.Ceil(need))
				}
			}
		}
	}
}

// closest is the whole damage nearest the exact damage that lands the
// board's average closest to the target:
//
//   - every tier rounds to its nearest whole damage;
//   - a tier within the half band of a half, and every tier of a count
//     ladder, may go either way;
//   - so may two tiers where nearest would make the easier one pay more;
//   - every harder tier of one ability deals at least the ladder step more
//     than the tier below it, and no harder tier pays less than an easier
//     one (the same is allowed);
//
// and of the roundings that keep the rules the one whose average is closest
// to the target wins (failing any, harder tiers are raised to match).
func (r *rounder) closest(exact []float64, target float64) ([]int, error) {
	p := r.p
	n := len(exact)
	base := make([]int, n)
	open := make([]bool, n)
	for i, d := range exact {
		base[i] = int(math.Floor(d + 0.5))
		whole := d == math.Floor(d)
		if (p.count[i] && !whole) || math.Abs(d-math.Floor(d)-0.5) <= p.opts.HalfBand {
			base[i], open[i] = int(math.Floor(d)), !whole
		}
	}
	for i := range base {
		for j := range base {
			if p.easier(i, j) && float64(base[i])+p.riders[i] > float64(base[j])+p.riders[j] {
				for _, k := range []int{i, j} {
					if !open[k] && exact[k] != math.Floor(exact[k]) {
						open[k], base[k] = true, int(math.Floor(exact[k]))
					}
				}
			}
		}
	}
	var free []int
	for i, o := range open {
		if o {
			free = append(free, i)
		}
	}
	if len(free) > maxOpen {
		return nil, fmt.Errorf("%d tiers could round either way; at most %d", len(free), maxOpen)
	}
	sig := intsKey(base) + "|" + intsKey(free)
	set, ok := r.sets[sig]
	if !ok {
		seen := map[string]bool{}
		for _, raise := range []bool{false, true} {
			if len(set) > 0 {
				break
			}
			for mask := 0; mask < 1<<len(free); mask++ {
				v := append([]int(nil), base...)
				for k, i := range free {
					if mask&(1<<k) != 0 {
						v[i]++
					}
				}
				for i := range v {
					v[i] = max(v[i], p.opts.MinimumDamage)
				}
				p.stepLadders(v)
				if raise {
					// Raising only ever lifts tiers, so this settles.
					for !p.keepsOrder(v) {
						p.raiseHarder(v)
						p.stepLadders(v)
					}
				} else if !p.keepsOrder(v) {
					continue
				}
				key := intsKey(v)
				if seen[key] {
					continue
				}
				seen[key] = true
				set = append(set, candidate{v: v, key: key})
			}
		}
		r.sets[sig] = set
	}
	best, bestGap := -1, math.Inf(1)
	for k, c := range set {
		if gap := math.Abs(r.play(c.v, c.key).ev - target); gap < bestGap-1e-12 {
			best, bestGap = k, gap
		}
	}
	return set[best].v, nil
}

// build scans the targets From–To, rounding the exact board at each with
// closest, pins every change between scanned targets, and keeps only the
// rises that last: each tier's damage on a step is the least it deals at any
// target from there up to To, so no later step takes damage away. At the
// anchor (the standard level) the board is the one rounded closest to it,
// and no step above it deals less.
func (p *prepared) build() (Staircase, error) {
	o := p.opts
	r := &rounder{p: p, memo: map[string]played{}, sets: map[string][]candidate{}}
	exactAt := func(m float64) []float64 {
		d, _, _ := pricedDamage(p.relative, p.riders, p.ladders, m, o)
		return d
	}
	averageAt := func(m float64) float64 {
		d := exactAt(m)
		for i := range d {
			d[i] += p.riders[i]
		}
		ev, _, _, _ := p.s.play(d, o.Rolls, false)
		return ev
	}
	// The exact board's average grows with the multiplier: solve it on a
	// grid covering From–To and interpolate in between.
	low, high := o.From/p.sum, o.To/p.sum
	for k := 0; averageAt(low) > o.From && k < 60; k++ {
		low /= 1.25
	}
	for k := 0; averageAt(high) < o.To && k < 60; k++ {
		high *= 1.25
	}
	points := int(math.Ceil((o.To-o.From)/(o.Scan*gridScans))) + 2
	ms, ts := make([]float64, points), make([]float64, points)
	for k := range ms {
		ms[k] = low + (high-low)*float64(k)/float64(points-1)
		ts[k] = averageAt(ms[k])
	}
	multiplierFor := func(t float64) float64 {
		k := sort.SearchFloat64s(ts, t)
		k = min(max(k, 1), points-1)
		a, b := ts[k-1], ts[k]
		if b-a < 1e-12 {
			return ms[k]
		}
		return ms[k-1] + (ms[k]-ms[k-1])*(t-a)/(b-a)
	}
	at := func(t float64) ([]int, error) { return r.closest(exactAt(multiplierFor(t)), t) }
	same := func(a, b []int) bool {
		for i := range a {
			if a[i] != b[i] {
				return false
			}
		}
		return true
	}
	type state struct {
		at float64
		v  []int
	}
	first, err := at(o.From)
	if err != nil {
		return Staircase{}, err
	}
	states := []state{{o.From, first}}
	scans := int(math.Round((o.To - o.From) / o.Scan))
	for k := 1; k <= scans; k++ {
		x := math.Min(o.From+float64(k)*o.Scan, o.To)
		cur, err := at(x)
		if err != nil {
			return Staircase{}, err
		}
		for prev := states[len(states)-1]; !same(prev.v, cur); prev = states[len(states)-1] {
			a, b := prev.at, x
			for i := 0; i < bisections; i++ {
				mid := (a + b) / 2
				v, err := at(mid)
				if err != nil {
					return Staircase{}, err
				}
				if same(prev.v, v) {
					a = mid
				} else {
					b = mid
				}
			}
			v, err := at(b)
			if err != nil {
				return Staircase{}, err
			}
			states = append(states, state{b, v})
		}
	}
	// The anchor starts a state of its own.
	anchor := -1
	if o.Anchor > 0 {
		k := sort.Search(len(states), func(i int) bool { return states[i].at > o.Anchor+1e-9 }) - 1
		if math.Abs(states[k].at-o.Anchor) <= 1e-9 {
			anchor = k
		} else {
			states = append(states[:k+1], append([]state{{o.Anchor, states[k].v}}, states[k+1:]...)...)
			anchor = k + 1
		}
	}
	// Each tier's least damage from each state to the end; from the anchor
	// up, never less than the anchor's board.
	floor := append([]int(nil), states[len(states)-1].v...)
	lasting := make([][]int, len(states))
	for k := len(states) - 1; k >= 0; k-- {
		for i := range floor {
			floor[i] = min(floor[i], states[k].v[i])
		}
		lasting[k] = append([]int(nil), floor...)
		if anchor >= 0 && k >= anchor {
			for i := range lasting[k] {
				lasting[k][i] = max(lasting[k][i], states[anchor].v[i])
			}
		}
	}
	sc := Staircase{From: o.From, To: o.To}
	for k, v := range lasting {
		var raised []int
		if k > 0 {
			for i := range v {
				if v[i] > lasting[k-1][i] {
					raised = append(raised, i)
				}
			}
			if len(raised) == 0 {
				continue
			}
		}
		got := r.play(v, intsKey(v))
		sc.Steps = append(sc.Steps, Step{At: states[k].at, Damage: v, Average: got.ev, NoHit: got.noHit, Use: got.used, Raised: orEmptyInts(raised)})
	}
	return sc, nil
}

func orEmptyInts(v []int) []int {
	if v == nil {
		return []int{}
	}
	return v
}
